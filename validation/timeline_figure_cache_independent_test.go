package validation

import (
	"context"
	"errors"
	"testing"
	"unsafe"
)

func TestIndependentTimelineFigureClosedScratchShapes(t *testing.T) {
	// Each map's bound includes old/new tables and conservative spare capacity.
	anchor := int64(unsafe.Sizeof(figuredBassAnchorKey{})) + int64(unsafe.Sizeof(Evidence[NodeRef]{}))
	units := int64(unsafe.Sizeof(figuredBassAnchorKey{})) + int64(unsafe.Sizeof(Evidence[Decimal]{}))
	durations := int64(unsafe.Sizeof(figuredBassDurationKey{})) + int64(unsafe.Sizeof(Evidence[Rational]{}))
	wantCaches := 4*int64(timelineFigureCacheEntries)*(anchor+units+durations) + 3*512
	if timelineFigureCacheReserveBytes < wantCaches {
		t.Fatalf("cache allowance%d below closed-shape bound%d", timelineFigureCacheReserveBytes, wantCaches)
	}
	// A figure contributes to either known-anchor counts or unknown-anchor partGaps.
	// The latter is the wider record; both counts are bounded by input figures.
	gap := int64(unsafe.Sizeof(NodeID(0))) + int64(unsafe.Sizeof(Evidence[NodeRef]{}))
	count := int64(unsafe.Sizeof(NodeID(0))) + int64(unsafe.Sizeof(int(0)))
	if count > gap {
		gap = count
	}
	wantVariable := 4*gap + int64(unsafe.Sizeof(NodeID(0)))
	if timelineFigureVariableScratchBytes < wantVariable {
		t.Fatalf("per-figure allowance%d below gap/map/ID bound%d", timelineFigureVariableScratchBytes, wantVariable)
	}
	if timelineFigureReserveBytes < int64(unsafe.Sizeof(figuredBassTimingOrigin{}))+512 {
		t.Fatal("retained figure lost indirect map headroom")
	}
	domain := 3*int64(unsafe.Sizeof(figuredBassDomain{})) + 4*(int64(unsafe.Sizeof(timelineDomain{}))+int64(unsafe.Sizeof(int(0))))
	if 2048 < domain {
		t.Fatalf("per-domain scratch below%d", domain)
	}
	t.Logf("cache bound%d allowance%d; variable bound%d allowance%d; domain bound%d", wantCaches, timelineFigureCacheReserveBytes, wantVariable, timelineFigureVariableScratchBytes, domain)
}

func TestIndependentTimelineFigureCacheBoundedAndReused(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	cache := map[int]int{}
	for i := 0; i < 4096; i++ {
		if err := timelineFigureCachePut(context.Background(), s, cache, i, i); err != nil {
			t.Fatal(err)
		}
		if len(cache) > timelineFigureCacheEntries || cache[i] != i {
			t.Fatalf("cache escaped cap at%d len%d", i, len(cache))
		}
	}
	for len(cache) < timelineFigureCacheEntries {
		key := 10000 + len(cache)
		if err := timelineFigureCachePut(context.Background(), s, cache, key, key); err != nil {
			t.Fatal(err)
		}
	}
	key := 0
	for existing := range cache {
		key = existing
		break
	}
	if err := timelineFigureCachePut(context.Background(), s, cache, key, -1); err != nil {
		t.Fatal(err)
	}
	if len(cache) != timelineFigureCacheEntries || cache[key] != -1 {
		t.Fatal("existing-key update unnecessarily discarded cache")
	}
}

func TestIndependentTimelineFigureCacheEvictionEveryCancellationBoundary(t *testing.T) {
	fresh := func() map[int]int {
		m := map[int]int{}
		for i := 0; i < timelineFigureCacheEntries; i++ {
			m[i] = i
		}
		return m
	}
	s := rationalSession(t, Budgets{})
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if err := timelineFigureCachePut(probe, s, fresh(), -1, 1); err != nil {
		t.Fatal(err)
	}
	s.cleanup()
	for at := 1; at <= probe.calls; at++ {
		s := rationalSession(t, Budgets{})
		cache := fresh()
		err := timelineFigureCachePut(&rationalCancelContext{Context: context.Background(), at: at}, s, cache, -1, 1)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll%d error%v", at, err)
		}
		if _, published := cache[-1]; published {
			t.Fatalf("poll%d inserted after cancellation", at)
		}
		for key, value := range cache {
			if key != value {
				t.Fatal("surviving immutable cache entries mutated")
			}
		}
		s.cleanup()
	}
	s = rationalSession(t, Budgets{MaxRuleSteps: int64(timelineFigureCacheEntries)})
	defer s.cleanup()
	cache := fresh()
	if err := timelineFigureCachePut(context.Background(), s, cache, -1, 1); !errors.Is(err, ErrResourceLimit) || len(cache) != timelineFigureCacheEntries {
		t.Fatalf("eviction traversal not admitted before clearing: %v", err)
	}
	t.Logf("all%d eviction cancellation polls", probe.calls)
}
