package validation

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

func independentKeyPresenceFixture(t *testing.T, count int) (*Session, *timelineState, musicalPosition) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	h := &timelineState{}
	stateClose(t, s, h, 2)
	at := statePosition(t, 0, 0, 1)
	statePublish(t, s, &h.Keys, at, stateScope{AllStaves: true}, stateKnown(keyState{Traditional: true, Fifths: stateKnown(stateInt(t, 0)), Mode: stateAbsent[string](), Cancel: stateAbsent[Integer](), CancelLocation: stateAbsent[string]()}), 5)
	at.Offset = mustRational(t, 1, 1)
	cause := &DecisionRef{Rule: RuleKey{"independent-key-position", 1}, Subject: NodeRef{"d", 9}}
	for i := 0; i < count; i++ {
		g := stateGap{Part: at.Part, FirstMeasure: 0, LastMeasure: 0, Scope: stateScope{AllStaves: true}, Availability: Evidence[bool]{State: EvidenceUnsupported, Reason: "position-unavailable", Cause: cause}}
		if err := addStateGap(context.Background(), s, &h.Keys, g); err != nil {
			t.Fatal(err)
		}
		if err := h.certifyKeyPresenceGap(context.Background(), s, i, NodeRef{"d", NodeID(100 + i)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := stateIndex(context.Background(), s, &h.Keys); err != nil {
		t.Fatal(err)
	}
	return s, h, at
}

func TestIndependentTimelineKeyPresenceShapeAndCertificateAdmission(t *testing.T) {
	if need := 4 * int64(unsafe.Sizeof(int(0))+unsafe.Sizeof(stateProvenance{})); need > timelineKeyPresenceCertificateBytes {
		t.Fatalf("certificate map growth%d", need)
	}
	if need := int64(unsafe.Sizeof(stateGap{})); need > timelineKeyPresenceGapScratchBytes {
		t.Fatalf("filtered row%d", need)
	}
	node := NodeRef{"independent-source", 12}
	newHistory := func() *timelineState { return &timelineState{Keys: stateHistory[keyState]{Gaps: make([]stateGap, 2)}} }
	s := rationalSession(t, Budgets{})
	h := newHistory()
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if err := h.certifyKeyPresenceGap(probe, s, 0, node); err != nil {
		t.Fatal(err)
	}
	bytes := rationalUsage(t, s, BudgetBufferedBytes)
	if len(h.keyPresenceGaps) != 1 || rationalUsage(t, s, BudgetContextFacts) != 1 {
		t.Fatal("certificate admission missing")
	}
	s.cleanup()
	for _, limit := range []int64{timelineKeyPresenceCertificateBytes - 1, bytes - 1} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		h := newHistory()
		err := h.certifyKeyPresenceGap(context.Background(), s, 0, node)
		if !errors.Is(err, ErrResourceLimit) || len(h.keyPresenceGaps) != 0 {
			t.Fatalf("certificate limit%d published: %v", limit, err)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("certificate cleanup leaked")
		}
	}
	s = rationalSession(t, Budgets{MaxContextFacts: 1})
	h = newHistory()
	if err := h.certifyKeyPresenceGap(context.Background(), s, 0, node); err != nil {
		t.Fatal(err)
	}
	if err := h.certifyKeyPresenceGap(context.Background(), s, 1, node); !errors.Is(err, ErrResourceLimit) || len(h.keyPresenceGaps) != 1 {
		t.Fatalf("certificate quota bypass: %v", err)
	}
	s.cleanup()
	for poll := 1; poll <= probe.calls; poll++ {
		s := rationalSession(t, Budgets{})
		h := newHistory()
		err := h.certifyKeyPresenceGap(&rationalCancelContext{Context: context.Background(), at: poll}, s, 0, node)
		if !errors.Is(err, context.Canceled) || len(h.keyPresenceGaps) != 0 {
			t.Fatalf("certificate poll%d: %v", poll, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("certificate scratch leaked")
		}
		s.cleanup()
	}
	s = rationalSession(t, Budgets{MaxRuleSteps: 64})
	h = newHistory()
	err := h.certifyKeyPresenceGap(context.Background(), s, 0, NodeRef{strings.Repeat("source", 1024), 12})
	if !errors.Is(err, ErrResourceLimit) || len(h.keyPresenceGaps) != 0 {
		t.Fatalf("certificate copied before string work admission: %v", err)
	}
	s.cleanup()
	t.Logf("certificate%d bytes; all%d insertion cancellation polls", bytes, probe.calls)
}

func TestIndependentTimelineKeyPresenceQueryOwnershipAndCancellation(t *testing.T) {
	s, h, at := independentKeyPresenceFixture(t, 1)
	index := h.Keys.index
	beforeContext := rationalUsage(t, s, BudgetContextFacts)
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	got, err := h.queryKeyPresence(probe, s, at, stateKnown(stateOne()))
	if err != nil || got.Value.State != EvidenceKnown || !got.Value.Value || len(got.Provenance) != 2 {
		t.Fatalf("query %+v: %v", got, err)
	}
	got.Provenance[0].Node.Node = 999
	got.Provenance[1].Node.Node = 998
	again, err := h.queryKeyPresence(context.Background(), s, at, stateKnown(stateOne()))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range again.Provenance {
		if p.Node.Node == 999 || p.Node.Node == 998 {
			t.Fatal("returned provenance aliases state")
		}
	}
	if h.Keys.index != index || rationalUsage(t, s, BudgetContextFacts) != beforeContext || rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
		t.Fatal("query rebuilt index, published context or leaked scratch")
	}
	s.cleanup()
	for poll := 1; poll <= probe.calls; poll++ {
		s, h, at := independentKeyPresenceFixture(t, 1)
		index := h.Keys.index
		certs := maps.Clone(h.keyPresenceGaps)
		gaps := stateClone(h.Keys.Gaps)
		_, err := h.queryKeyPresence(&rationalCancelContext{Context: context.Background(), at: poll}, s, at, stateKnown(stateOne()))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("query poll%d: %v", poll, err)
		}
		if h.Keys.index != index || !reflect.DeepEqual(certs, h.keyPresenceGaps) || !reflect.DeepEqual(gaps, h.Keys.Gaps) || rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatalf("query poll%d mutated raw/index or leaked scratch", poll)
		}
		if _, err := h.queryKeyPresence(context.Background(), s, at, stateKnown(stateOne())); err != nil {
			t.Fatal(err)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("query cleanup leaked")
		}
	}
	t.Logf("all%d key-presence query cancellation polls", probe.calls)
}

func TestIndependentTimelineKeyPresenceFilteredScratchBeforeAllocation(t *testing.T) {
	s, h, at := independentKeyPresenceFixture(t, 32)
	defer s.cleanup()
	before := rationalUsage(t, s, BudgetBufferedBytes)
	if _, err := queryState(context.Background(), s, h, &h.Keys, at, stateKnown(stateOne()), func(v keyState) Evidence[bool] { return v.encodingEvidence() }); err != nil {
		t.Fatal(err)
	}
	originalBytes := rationalUsage(t, s, BudgetBufferedBytes) - before
	before = rationalUsage(t, s, BudgetBufferedBytes)
	scratch := int64(len(h.Keys.Gaps))*timelineKeyPresenceGapScratchBytes + timelineKeyPresenceScratchFixedBytes
	held := s.account.Limits().MaxBufferedBytes - before - originalBytes - scratch + 1
	if err := s.account.Charge(BudgetBufferedBytes, held); err != nil {
		t.Fatal(err)
	}
	defer s.account.Release(BudgetBufferedBytes, held)
	_, err := h.queryKeyPresence(context.Background(), s, at, stateKnown(stateOne()))
	var resource *ResourceError
	if !errors.As(err, &resource) || resource.Kind != BudgetBufferedBytes || resource.Requested != scratch {
		t.Fatalf("filtered rows lacked upfront exact admission: %v; want request%d", err, scratch)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != s.retained+held {
		t.Fatal("failed filtered scratch was not released")
	}
}
