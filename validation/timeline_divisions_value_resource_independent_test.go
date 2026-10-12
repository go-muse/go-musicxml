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

func independentDivisionsValueFixture(t *testing.T, count int) (*Session, *timelineState, musicalPosition) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	h := &timelineState{}
	stateClose(t, s, h, 2)
	at := statePosition(t, 0, 0, 1)
	statePublish(t, s, &h.Divisions, at, stateScope{AllStaves: true}, stateKnown(stateDecimal(t, 1, 1)), 5)
	at.Offset = mustRational(t, 1, 1)
	cause := &DecisionRef{Rule: RuleKey{"independent-divisions-position", 1}, Subject: NodeRef{"d", 9}}
	for i := 0; i < count; i++ {
		g := stateGap{Part: at.Part, FirstMeasure: 0, LastMeasure: 0, Scope: stateScope{AllStaves: true}, Availability: Evidence[bool]{State: EvidenceUnsupported, Reason: "position-unavailable", Cause: cause}}
		if err := addStateGap(context.Background(), s, &h.Divisions, g); err != nil {
			t.Fatal(err)
		}
		if err := h.certifyDivisionsValueGap(context.Background(), s, i, stateDecimal(t, 1, 1), NodeRef{"d", NodeID(100 + i)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := stateIndex(context.Background(), s, &h.Divisions); err != nil {
		t.Fatal(err)
	}
	return s, h, at
}

func TestIndependentTimelineDivisionsValueShapeAndCertificateAdmission(t *testing.T) {
	if need := 4 * int64(unsafe.Sizeof(int(0))+unsafe.Sizeof(divisionsValueCertificate{})); need > timelineDivisionsValueCertificateBytes {
		t.Fatalf("certificate map growth%d", need)
	}
	if need := int64(unsafe.Sizeof(stateGap{})); need > timelineDivisionsValueGapScratchBytes {
		t.Fatalf("filtered row%d", need)
	}
	node := NodeRef{"independent-source", 12}
	newHistory := func() *timelineState {
		return &timelineState{Divisions: stateHistory[Decimal]{Gaps: make([]stateGap, 2)}}
	}
	s := rationalSession(t, Budgets{})
	h := newHistory()
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if err := h.certifyDivisionsValueGap(probe, s, 0, stateDecimal(t, 1, 1), node); err != nil {
		t.Fatal(err)
	}
	bytes := rationalUsage(t, s, BudgetBufferedBytes)
	if len(h.divisionsValueGaps) != 1 || rationalUsage(t, s, BudgetContextFacts) != 1 {
		t.Fatal("certificate admission missing")
	}
	s.cleanup()
	for _, limit := range []int64{timelineDivisionsValueCertificateBytes - 1, bytes - 1} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		h := newHistory()
		err := h.certifyDivisionsValueGap(context.Background(), s, 0, stateDecimal(t, 1, 1), node)
		if !errors.Is(err, ErrResourceLimit) || len(h.divisionsValueGaps) != 0 {
			t.Fatalf("certificate limit%d published: %v", limit, err)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("certificate cleanup leaked")
		}
	}
	s = rationalSession(t, Budgets{MaxContextFacts: 1})
	h = newHistory()
	if err := h.certifyDivisionsValueGap(context.Background(), s, 0, stateDecimal(t, 1, 1), node); err != nil {
		t.Fatal(err)
	}
	if err := h.certifyDivisionsValueGap(context.Background(), s, 1, stateDecimal(t, 1, 1), node); !errors.Is(err, ErrResourceLimit) || len(h.divisionsValueGaps) != 1 {
		t.Fatalf("certificate quota bypass: %v", err)
	}
	s.cleanup()
	for poll := 1; poll <= probe.calls; poll++ {
		s := rationalSession(t, Budgets{})
		h := newHistory()
		err := h.certifyDivisionsValueGap(&rationalCancelContext{Context: context.Background(), at: poll}, s, 0, stateDecimal(t, 1, 1), node)
		if !errors.Is(err, context.Canceled) || len(h.divisionsValueGaps) != 0 {
			t.Fatalf("certificate poll%d: %v", poll, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("certificate scratch leaked")
		}
		s.cleanup()
	}
	s = rationalSession(t, Budgets{MaxRuleSteps: 64})
	h = newHistory()
	err := h.certifyDivisionsValueGap(context.Background(), s, 0, stateDecimal(t, 1, 1), NodeRef{strings.Repeat("source", 1024), 12})
	if !errors.Is(err, ErrResourceLimit) || len(h.divisionsValueGaps) != 0 {
		t.Fatalf("certificate copied before string work admission: %v", err)
	}
	s.cleanup()
	t.Logf("certificate%d bytes; all%d insertion cancellation polls", bytes, probe.calls)
}

func TestIndependentTimelineDivisionsValueQueryOwnershipAndCancellation(t *testing.T) {
	s, h, at := independentDivisionsValueFixture(t, 1)
	index := h.Divisions.index
	beforeContext := rationalUsage(t, s, BudgetContextFacts)
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	got, err := h.queryDivisions(probe, s, at, true)
	if err != nil || got.Value.State != EvidenceKnown || got.Value.Value.String() != "1" || len(got.Provenance) != 2 {
		t.Fatalf("query %+v: %v", got, err)
	}
	got.Provenance[0].Node.Node = 999
	got.Provenance[1].Node.Node = 998
	again, err := h.queryDivisions(context.Background(), s, at, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range again.Provenance {
		if p.Node.Node == 999 || p.Node.Node == 998 {
			t.Fatal("returned provenance aliases state")
		}
	}
	if h.Divisions.index != index || rationalUsage(t, s, BudgetContextFacts) != beforeContext || rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
		t.Fatal("query rebuilt index, published context or leaked scratch")
	}
	s.cleanup()
	for poll := 1; poll <= probe.calls; poll++ {
		s, h, at := independentDivisionsValueFixture(t, 1)
		index := h.Divisions.index
		certs := maps.Clone(h.divisionsValueGaps)
		gaps := stateClone(h.Divisions.Gaps)
		_, err := h.queryDivisions(&rationalCancelContext{Context: context.Background(), at: poll}, s, at, true)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("query poll%d: %v", poll, err)
		}
		if h.Divisions.index != index || !reflect.DeepEqual(certs, h.divisionsValueGaps) || !reflect.DeepEqual(gaps, h.Divisions.Gaps) || rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatalf("query poll%d mutated raw/index or leaked scratch", poll)
		}
		if _, err := h.queryDivisions(context.Background(), s, at, true); err != nil {
			t.Fatal(err)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("query cleanup leaked")
		}
	}
	t.Logf("all%d divisions-value query cancellation polls", probe.calls)
}

func TestIndependentTimelineDivisionsValueFilteredScratchBeforeAllocation(t *testing.T) {
	s, h, at := independentDivisionsValueFixture(t, 32)
	defer s.cleanup()
	before := rationalUsage(t, s, BudgetBufferedBytes)
	if _, err := queryState(context.Background(), s, h, &h.Divisions, at, stateKnown(stateOne()), stateWhole[Decimal]); err != nil {
		t.Fatal(err)
	}
	originalBytes := rationalUsage(t, s, BudgetBufferedBytes) - before
	before = rationalUsage(t, s, BudgetBufferedBytes)
	scratch := int64(len(h.Divisions.Gaps))*timelineDivisionsValueGapScratchBytes + timelineDivisionsValueScratchFixedBytes
	held := s.account.Limits().MaxBufferedBytes - before - originalBytes - scratch + 1
	if err := s.account.Charge(BudgetBufferedBytes, held); err != nil {
		t.Fatal(err)
	}
	defer s.account.Release(BudgetBufferedBytes, held)
	_, err := h.queryDivisions(context.Background(), s, at, true)
	var resource *ResourceError
	if !errors.As(err, &resource) || resource.Kind != BudgetBufferedBytes || resource.Requested != scratch {
		t.Fatalf("filtered rows lacked upfront exact admission: %v; want request%d", err, scratch)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != s.retained+held {
		t.Fatal("failed filtered scratch was not released")
	}
}

func TestIndependentTimelineDivisionsValuePrimitiveParityAndCancellation(t *testing.T) {
	for _, same := range []bool{true, false} {
		fixture := func() (*Session, *timelineState, musicalPosition) {
			s, h, at := independentDivisionsValueFixture(t, 1)
			if !same {
				c := h.divisionsValueGaps[0]
				c.Value = stateDecimal(t, 2, 1)
				h.divisionsValueGaps[0] = c
			}
			return s, h, at
		}
		s, h, at := fixture()
		owned, err := h.queryDivisions(context.Background(), s, at, true)
		if err != nil {
			t.Fatal(err)
		}
		before := rationalUsage(t, s, BudgetBufferedBytes)
		probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
		got, err := h.queryDivisions(probe, s, at, false)
		if err != nil || !reflect.DeepEqual(got.Value, owned.Value) || len(got.Provenance) != 0 {
			t.Fatalf("same%t primitive parity: %+v %+v %v", same, got, owned, err)
		}
		// Only the discarded original causal result needs independent ownership.
		// Primitive known consensus retains no numeric or witness copies.
		if delta := rationalUsage(t, s, BudgetBufferedBytes) - before; delta != 128 {
			t.Fatalf("same%t primitive copy retained%d, want one owned causal record", same, delta)
		}
		if !same {
			if got.Value.Cause == nil || got.Value.Cause == h.Divisions.Gaps[0].Availability.Cause {
				t.Fatal("primitive fallback aliases raw cause")
			}
			got.Value.Cause.Subject.Node = 999
			if h.Divisions.Gaps[0].Availability.Cause.Subject.Node == 999 {
				t.Fatal("returned cause mutated history")
			}
		}
		s.cleanup()
		for poll := 1; poll <= probe.calls; poll++ {
			s, h, at := fixture()
			index := h.Divisions.index
			certs := maps.Clone(h.divisionsValueGaps)
			gaps := stateClone(h.Divisions.Gaps)
			_, err := h.queryDivisions(&rationalCancelContext{Context: context.Background(), at: poll}, s, at, false)
			if !errors.Is(err, context.Canceled) || h.Divisions.index != index || !reflect.DeepEqual(certs, h.divisionsValueGaps) || !reflect.DeepEqual(gaps, h.Divisions.Gaps) || rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
				t.Fatalf("same%t primitive poll%d: %v", same, poll, err)
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("primitive cleanup leaked")
			}
		}
		t.Logf("same%t all%d primitive query cancellation polls;128 bytes causal ownership only", same, probe.calls)
	}
}
