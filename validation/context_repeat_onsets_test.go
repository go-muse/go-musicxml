package validation

import (
	"context"
	"errors"
	"testing"
)

func repeatOnsetProofScope(t *testing.T, count int) *contextRepeatProofScope {
	t.Helper()
	notes, _ := orderedRepeatNotes(t, count)
	return &contextRepeatProofScope{notes: notes, complete: stateKnown(true), buckets: map[uint64][]contextRepeatNote{0: notes}}
}

func TestContextRepeatMissingOnsetResources(t *testing.T) {
	ctx := context.Background()
	part := NodeRef{"d", 1}
	probe := &rationalCancelContext{Context: ctx, at: 1000000}
	s := rationalSession(t, Budgets{})
	scope := repeatOnsetProofScope(t, 3)
	got, err := contextRepeatEmptyMeasure(probe, s, &timelineService{}, scope, part, 0, 1)
	if err != nil || got.State != EvidenceKnown || got.Value {
		t.Fatalf("probe: %+v %v", got, err)
	}
	steps := rationalUsage(t, s, BudgetRuleSteps)
	bytes := rationalUsage(t, s, BudgetBufferedBytes)
	if bytes != 512+3*1024 {
		t.Fatalf("witness map reservation=%d", bytes)
	}
	scope.closeOnsets()
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("scope release leaked")
	}
	s.cleanup()
	for _, limit := range []int64{steps, steps - 1} {
		s := rationalSession(t, Budgets{MaxRuleSteps: limit})
		scope := repeatOnsetProofScope(t, 3)
		got, err := contextRepeatEmptyMeasure(ctx, s, &timelineService{}, scope, part, 0, 1)
		if limit == steps {
			if err != nil || got.State != EvidenceKnown || got.Value {
				t.Fatalf("exact steps: %+v %v", got, err)
			}
		} else if !errors.Is(err, ErrResourceLimit) || got.State != "" || scope.onsetComplete.State != "" || scope.sourceOnsets != nil || scope.releaseOnsets != nil {
			t.Fatalf("one-under steps published: %+v %+v %v", got, scope, err)
		}
		scope.closeOnsets()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("step failure leaked")
		}
		s.cleanup()
	}
	for _, limit := range []int64{bytes, bytes - 1} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		scope := repeatOnsetProofScope(t, 3)
		got, err := contextRepeatEmptyMeasure(ctx, s, &timelineService{}, scope, part, 0, 1)
		if limit == bytes {
			if err != nil || got.State != EvidenceKnown || got.Value {
				t.Fatalf("exact bytes: %+v %v", got, err)
			}
		} else if !errors.Is(err, ErrResourceLimit) || got.State != "" || scope.onsetComplete.State != "" {
			t.Fatalf("one-under bytes published: %+v %v", got, err)
		}
		scope.closeOnsets()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("byte failure leaked")
		}
		s.cleanup()
	}
	for at := 1; at <= probe.calls; at++ {
		s := rationalSession(t, Budgets{})
		scope := repeatOnsetProofScope(t, 3)
		got, err := contextRepeatEmptyMeasure(&rationalCancelContext{Context: ctx, at: at}, s, &timelineService{}, scope, part, 0, 1)
		if !errors.Is(err, context.Canceled) || got.State != "" || scope.onsetComplete.State != "" || scope.sourceOnsets != nil || scope.releaseOnsets != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("poll%d published/leaked: %+v %+v %v", at, got, scope, err)
		}
		s.cleanup()
	}
	t.Logf("exact admission %d steps / %d bytes; all %d cancellation polls atomic", steps, bytes, probe.calls)
}

func TestContextRepeatMissingOnsetCacheIsLazyAndLinear(t *testing.T) {
	part := NodeRef{"d", 1}
	ctx := context.Background()
	for _, count := range []int{100, 1000} {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34})
		scope := repeatOnsetProofScope(t, count)
		scope.buckets[1] = scope.notes[:1]
		got, err := contextRepeatEmptyMeasure(ctx, s, &timelineService{}, scope, part, 0, 1)
		if err != nil || got.State == EvidenceKnown || scope.onsetComplete.State != "" || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("nonempty target allocated proof: %+v %v", got, err)
		}
		delete(scope.buckets, 1)
		if got, err = contextRepeatEmptyMeasure(ctx, s, &timelineService{}, scope, part, 0, 1); err != nil || got.State != EvidenceKnown || got.Value {
			t.Fatalf("initial proof %+v %v", got, err)
		}
		before := rationalUsage(t, s, BudgetRuleSteps)
		bytes := rationalUsage(t, s, BudgetBufferedBytes)
		for i := 0; i < 1000; i++ {
			if got, err = contextRepeatEmptyMeasure(ctx, s, &timelineService{}, scope, part, 0, 1); err != nil || got.State != EvidenceKnown || got.Value {
				t.Fatalf("cached proof %+v %v", got, err)
			}
		}
		delta := rationalUsage(t, s, BudgetRuleSteps) - before
		if delta != 1000 || rationalUsage(t, s, BudgetBufferedBytes) != bytes {
			t.Fatalf("count%d rescanned/allocated cached proof: %d steps", count, delta)
		}
		scope.closeOnsets()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cache release leaked")
		}
		s.cleanup()
		t.Logf("%d notes, 1000 repeated checks: %d cached steps", count, delta)
	}
}

func TestContextRepeatMissingOnsetPreservesGaps(t *testing.T) {
	for _, kind := range []string{"unavailable", "known-false-availability", "empty-availability", "absent-availability", "uninitialized-position", "unconfined-future", "no-principal", "rest-only", "zero-duration"} {
		t.Run(kind, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			scope := repeatOnsetProofScope(t, 2)
			defer scope.closeOnsets()
			cause := &DecisionRef{Rule: RuleKey{ID: "onset-cause", Revision: 1}, Subject: NodeRef{"d", 42}}
			switch kind {
			case "unavailable":
				scope.notes[1].available = Evidence[bool]{State: EvidenceInvalid, Reason: "onset-prerequisite", Cause: cause}
			case "known-false-availability":
				scope.notes[1].available = stateKnown(false)
			case "empty-availability":
				scope.notes[1].available = Evidence[bool]{}
			case "absent-availability":
				scope.notes[1].available = stateAbsent[bool]()
			case "uninitialized-position":
				scope.notes[1].pos = musicalPosition{}
			case "unconfined-future":
				scope.notes[1].pos.MeasureOrdinal = 2
				scope.notes[1].pos.UnprovedFrame = true
				scope.notes[1].pos.Offset = mustRational(t, -1, 1)
			case "no-principal":
				for i := range scope.notes {
					scope.notes[i].chord = true
				}
			case "rest-only":
				for i := range scope.notes {
					scope.notes[i].rest = true
				}
			case "zero-duration":
				for i := range scope.notes {
					scope.notes[i].duration = timelineZero
				}
			}
			got, err := contextRepeatEmptyMeasure(context.Background(), s, &timelineService{}, scope, NodeRef{"d", 1}, 0, 1)
			if err != nil || got.State == EvidenceKnown {
				t.Fatalf("gap became proof %+v %v", got, err)
			}

			bytes := rationalUsage(t, s, BudgetBufferedBytes)
			again, e := contextRepeatEmptyMeasure(context.Background(), s, &timelineService{}, scope, NodeRef{"d", 1}, 0, 1)
			if e != nil || again.State != got.State || rationalUsage(t, s, BudgetBufferedBytes) != bytes {
				t.Fatalf("gap cache rebuilt/leaked %+v %v", again, e)
			}
			if kind == "unavailable" && (got.State != EvidenceInvalid || got.Reason != "onset-prerequisite" || got.Cause != cause) {
				t.Fatalf("cause changed %+v", got)
			}
		})
	}
}
