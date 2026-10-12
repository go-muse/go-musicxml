package validation

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestCrossReviewSparseGapRandomOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(120102026))
	states := []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid}
	ords := []uint64{0, 1, 3, 7, 10, 100, 1 << 40, math.MaxUint64}
	parts := []NodeRef{{"a", 1}, {"b", 1}, {"a", 2}}
	for trial := 0; trial < 250; trial++ {
		s := rationalSession(t, Budgets{})
		service := &timelineService{Facts: map[NodeID]timelineEventFact{}}
		queries := map[performanceMeasure]bool{}
		for i := 0; i < rng.Intn(80)+1; i++ {
			id := NodeID(i + 1)
			pos := statePosition(t, ords[rng.Intn(len(ords))], 0, 1)
			pos.Part = parts[rng.Intn(len(parts))]
			service.Facts[id] = timelineEventFact{Origin: stateKnown(pos)}
			s.sequence = append(s.sequence, id)
			queries[performanceMeasure{pos.Part, pos.MeasureOrdinal}] = true
		}
		for i := 0; i < rng.Intn(90)+1; i++ {
			cause := &DecisionRef{Rule: RuleKey{"gap", uint32(i + 1)}, Subject: parts[rng.Intn(len(parts))]}
			service.State.Divisions.Gaps = append(service.State.Divisions.Gaps, stateGap{Part: parts[rng.Intn(len(parts))], FirstMeasure: ords[rng.Intn(len(ords))], LastMeasure: ords[rng.Intn(len(ords))], Availability: Evidence[bool]{State: states[rng.Intn(len(states))], Reason: "gap", Cause: cause}})
		}
		expected := map[performanceMeasure]Evidence[bool]{}
		for query := range queries {
			for _, g := range service.State.Divisions.Gaps {
				if query.Part == g.Part && query.Ordinal >= g.FirstMeasure && query.Ordinal <= g.LastMeasure {
					expected[query] = g.Availability
				}
			}
		}
		x, err := buildPerformanceAdviceIndex(context.Background(), s, service)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(x.movementGaps, expected) {
			t.Fatalf("trial=%d got=%+v expected=%+v", trial, x.movementGaps, expected)
		}
		x.release(s)
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Fatalf("trial=%d leak=%d", trial, got)
		}
		s.cleanup()
	}
}

func TestCrossReviewScalarProviderAllInterruptions(t *testing.T) {
	s, p := performanceRound1ScalarProviderFixture(t, Budgets{})
	probe := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	_, _, err := evaluateContextPerformance(probe, s, nil, RuleKey{"performance", 1}, p, true)
	if err != nil {
		t.Fatal(err)
	}
	steps := rationalUsage(t, s, BudgetRuleSteps)
	polls := probe.calls
	s.cleanup()
	for at := 1; at <= polls; at++ {
		s, p = performanceRound1ScalarProviderFixture(t, Budgets{})
		_, _, err = evaluateContextPerformance(&rationalCancelContext{Context: context.Background(), at: at}, s, nil, RuleKey{"performance", 1}, p, true)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll=%d/%d err=%v", at, polls, err)
		}
		s.cleanup()
	}
	for limit := int64(1); limit < steps; limit++ {
		s, p = performanceRound1ScalarProviderFixture(t, Budgets{MaxRuleSteps: limit})
		_, _, err = evaluateContextPerformance(context.Background(), s, nil, RuleKey{"performance", 1}, p, true)
		if !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("limit=%d/%d err=%v", limit, steps, err)
		}
		s.cleanup()
	}
	t.Logf("checked all %d cancellation polls and %d quota cutoffs", polls, steps-1)
}

func TestCrossReviewDivisionAdviceScopeAndCause(t *testing.T) {
	part := NodeRef{"d", 1}
	cause := &DecisionRef{Rule: RuleKey{"division", 1}, Subject: NodeRef{"d", 99}}
	for _, st := range []EvidenceState{EvidenceKnown, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		for _, samePart := range []bool{false, true} {
			s := rationalSession(t, Budgets{})
			service := &timelineService{}
			service.State.Divisions.Publications = []statePublication[Decimal]{{Position: statePosition(t, 0, 0, 1), Value: stateKnown(stateDecimal(t, 4, 1))}}
			if st != EvidenceKnown {
				gapPart := part
				if !samePart {
					gapPart.DocumentID = "other"
				}
				service.State.Divisions.Gaps = []stateGap{{Part: gapPart, FirstMeasure: 0, LastMeasure: math.MaxUint64, Availability: Evidence[bool]{State: st, Reason: "division-gap", Cause: cause}}}
			}
			x, err := buildPerformanceAdviceIndex(context.Background(), s, service)
			if err != nil {
				t.Fatal(err)
			}
			out := newTimingContext("event")
			out.Part = stateKnown(part)
			if err = performanceStateAdvice(context.Background(), s, x, localNode(1, 0, "note"), &out); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, a := range out.Advisories {
				if a.Clause == "one-initial-divisions-per-part" {
					found = true
					if st == EvidenceKnown || !samePart {
						if a.Satisfied.State != EvidenceKnown || !a.Satisfied.Value {
							t.Fatalf("valid/other-part state=%s: %+v", st, a)
						}
					} else if a.Satisfied.State != st || a.Satisfied.Cause != cause || a.Satisfied.Reason != "division-gap" {
						t.Fatalf("state=%s lost cause: %+v", st, a)
					}
				}
			}
			if !found {
				t.Fatal("no division advice")
			}
			x.release(s)
			s.cleanup()
		}
	}
}

func TestCrossReviewMissingEventCausalMatrix(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{"timeline", 1}, Subject: NodeRef{"d", 1}}
	key := RuleKey{"performance", 1}
	for _, st := range []EvidenceState{EvidenceKnown, EvidenceAbsent, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		s := rationalSession(t, Budgets{})
		s.contextServices = map[RuleKey]*contextService{key: {Final: true, Timeline: Evidence[bool]{State: st, Reason: "timeline-gap", Cause: cause}}}
		out := newTimingContext("note")
		performanceUnavailableEvent(s, key, &out)
		want := st
		if st == EvidenceKnown {
			want = EvidenceUnknown
		}
		if st == EvidenceAbsent {
			want = EvidenceUnsupported
		}
		if out.Position.State != want || out.Duration.State != want || out.Part.State != want || out.PlayComplete.State != want {
			t.Fatalf("state=%s out=%+v", st, out)
		}
		if st != EvidenceKnown && st != EvidenceAbsent && (out.Position.Cause != cause || out.Duration.Cause != cause || out.Part.Cause != cause || out.PlayComplete.Cause != cause) {
			t.Fatalf("lost cause state=%s", st)
		}
		s.cleanup()
	}
}
