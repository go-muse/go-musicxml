package validation

import (
	"context"
	"errors"
	"math"
	"testing"
)

func performanceRound1IndexFixture(t *testing.T) *timelineService {
	t.Helper()
	out := &timelineService{Facts: map[NodeID]timelineEventFact{}}
	for i, ordinal := range []uint64{0, 3, 4, 10, math.MaxUint64} {
		pos := statePosition(t, ordinal, 1, 1)
		out.Facts[NodeID(i+1)] = timelineEventFact{Origin: stateKnown(pos)}
		other := pos
		other.Part.DocumentID = "other"
		out.Facts[NodeID(i+20)] = timelineEventFact{Origin: stateKnown(other)}
		out.State.Divisions.Publications = append(out.State.Divisions.Publications, statePublication[Decimal]{Position: pos, Value: stateKnown(stateDecimal(t, 4, 1))})
	}
	for i, bounds := range [][2]uint64{{0, math.MaxUint64}, {3, 4}, {4, math.MaxUint64}, {10, 10}} {
		cause := &DecisionRef{Rule: RuleKey{"original-gap", uint32(i + 1)}, Subject: NodeRef{"d", NodeID(i + 100)}}
		out.State.Divisions.Gaps = append(out.State.Divisions.Gaps, stateGap{Part: NodeRef{"d", 1}, FirstMeasure: bounds[0], LastMeasure: bounds[1], Availability: Evidence[bool]{State: EvidenceInvalid, Reason: "division-gap", Cause: cause}})
	}
	return out
}
func TestPerformanceRound1IndexSparseGapPrecedenceAndRelease(t *testing.T) {
	service := performanceRound1IndexFixture(t)
	s := performanceRound1IndexSession(t, Budgets{})
	defer s.cleanup()
	before := rationalUsage(t, s, BudgetBufferedBytes)
	x, err := buildPerformanceAdviceIndex(context.Background(), s, service)
	if err != nil {
		t.Fatal(err)
	}
	for ordinal, revision := range map[uint64]uint32{0: 1, 3: 2, 4: 3, 10: 4, math.MaxUint64: 3} {
		got := x.movementGaps[performanceMeasure{NodeRef{"d", 1}, ordinal}]
		if got.State != EvidenceInvalid || got.Cause == nil || got.Cause.Rule.Revision != revision || got.Cause.Subject.Node != NodeID(revision+99) {
			t.Errorf("ordinal=%d cause=%+v", ordinal, got)
		}
		if _, ok := x.movementGaps[performanceMeasure{NodeRef{"other", 1}, ordinal}]; ok {
			t.Error("other document part contaminated")
		}
	}
	if len(x.movementGaps) != 5 {
		t.Errorf("expanded unobserved ordinal domain: %d", len(x.movementGaps))
	}
	x.release(s)
	x.release(s)
	if got := rationalUsage(t, s, BudgetBufferedBytes); got != before {
		t.Errorf("temporary reservation leaked: %d != %d", got, before)
	}
}
func TestPerformanceRound1IndexAdmissionAndEveryInterruption(t *testing.T) {
	service := performanceRound1IndexFixture(t)
	probe := performanceRound1IndexSession(t, Budgets{})
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	x, err := buildPerformanceAdviceIndex(counter, probe, service)
	if err != nil {
		t.Fatal(err)
	}
	bytes := rationalUsage(t, probe, BudgetBufferedBytes)
	steps := rationalUsage(t, probe, BudgetRuleSteps)
	polls := counter.calls
	x.release(probe)
	probe.cleanup()
	for _, limit := range []int64{1, bytes - 1, bytes} {
		s := performanceRound1IndexSession(t, Budgets{MaxBufferedBytes: limit})
		x, err := buildPerformanceAdviceIndex(context.Background(), s, service)
		if limit < bytes {
			if !errors.Is(err, ErrResourceLimit) || x != nil {
				t.Errorf("limit=%d index=%v err=%v", limit, x, err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if x != nil {
			x.release(s)
		}
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Errorf("failed build retained %d", got)
		}
		s.cleanup()
	}
	for at := 1; at <= polls; at++ {
		s := performanceRound1IndexSession(t, Budgets{})
		x, err := buildPerformanceAdviceIndex(&rationalCancelContext{Context: context.Background(), at: at}, s, service)
		if !errors.Is(err, context.Canceled) || x != nil {
			t.Fatalf("poll=%d/%d err=%v", at, polls, err)
		}
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Fatalf("poll=%d retained=%d", at, got)
		}
		s.cleanup()
	}
	for _, limit := range []int64{1, steps - 1, steps} {
		s := performanceRound1IndexSession(t, Budgets{MaxRuleSteps: limit})
		x, err := buildPerformanceAdviceIndex(context.Background(), s, service)
		if limit < steps {
			if !errors.Is(err, ErrResourceLimit) || x != nil {
				t.Errorf("steps=%d index=%v err=%v", limit, x, err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if x != nil {
			x.release(s)
		}
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Errorf("steps=%d retained=%d", limit, got)
		}
		s.cleanup()
	}
}

func TestPerformanceRound1IndexNilServiceAndPublicationWithoutFacts(t *testing.T) {
	for _, service := range []*timelineService{nil, {State: timelineState{Divisions: stateHistory[Decimal]{Publications: []statePublication[Decimal]{{Position: statePosition(t, 3, 0, 1), Value: stateKnown(stateDecimal(t, 4, 1))}}}}}} {
		s := performanceRound1IndexSession(t, Budgets{})
		defer s.cleanup()
		x, err := buildPerformanceAdviceIndex(context.Background(), s, service)
		if err != nil {
			t.Fatal(err)
		}
		if service == nil {
			if len(x.divisions) != 0 || len(x.movementDivisions) != 0 {
				t.Fatal("nil service invented state")
			}
		} else {
			if len(x.movementDivisions[performanceMeasure{NodeRef{"d", 1}, 3}]) != 1 || len(x.movementGaps) != 0 {
				t.Fatal("publication-only index lost state")
			}
		}
		x.release(s)
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Errorf("temporary bytes=%d", got)
		}
	}
}

func performanceRound1IndexSession(t *testing.T, budgets Budgets) *Session {
	s := rationalSession(t, budgets)
	s.sequence = []NodeID{1, 2, 3, 4, 5, 20, 21, 22, 23, 24}
	return s
}
