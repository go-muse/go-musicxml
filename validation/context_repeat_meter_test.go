package validation

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func repeatMeterValue(beats, unit string) timeState {
	return timeState{SenzaMisura: stateAbsent[string](), Pairs: []timePair{{Beats: stateKnown(beats), BeatType: stateKnown(unit)}}}
}
func repeatUniformFixture(t *testing.T, count Integer) (*Session, *timelineService, musicalPosition) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	v := &timelineService{}
	stateClose(t, s, &v.State, 2)
	statePublish(t, s, &v.State.Staves, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(count), 90)
	return s, v, statePosition(t, 0, 2, 1)
}
func TestContextRepeatUniformMeterSparseHugeDomain(t *testing.T) {
	huge, e := NewInteger(new(big.Int).Lsh(big.NewInt(1), 256))
	if e != nil {
		t.Fatal(e)
	}
	s, v, pos := repeatUniformFixture(t, huge)
	defer s.cleanup()
	statePublish(t, s, &v.State.Times, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(repeatMeterValue("4", "4")), 91)
	statePublish(t, s, &v.State.Times, statePosition(t, 0, 1, 1), stateScope{Staff: huge}, stateKnown(repeatMeterValue("4", "4")), 92)
	before := rationalUsage(t, s, BudgetRuleSteps)
	r := &contextReader{ctx: context.Background(), s: s}
	got := contextRepeatUniformBeat(r, v, pos)
	if r.err != nil || got.State != EvidenceKnown || got.Value.String() != "1" {
		t.Fatalf("huge sparse domain %+v %v", got, r.err)
	}
	if work := rationalUsage(t, s, BudgetRuleSteps) - before; work > 100000 {
		t.Fatalf("count-expanded work %d", work)
	}
}
func TestContextRepeatUniformMeterChronologyAndMissingClass(t *testing.T) {
	for _, mode := range []string{"missing", "conflict", "stale-repaired", "future", "outside-count"} {
		t.Run(mode, func(t *testing.T) {
			s, v, pos := repeatUniformFixture(t, stateInt(t, 2))
			defer s.cleanup()
			statePublish(t, s, &v.State.Times, statePosition(t, 0, 0, 1), stateScope{Staff: stateInt(t, 1)}, stateKnown(repeatMeterValue("4", "4")), 91)
			if mode != "missing" {
				statePublish(t, s, &v.State.Times, statePosition(t, 0, 0, 1), stateScope{Staff: stateInt(t, 2)}, stateKnown(repeatMeterValue("4", "4")), 92)
			}
			switch mode {
			case "conflict":
				statePublish(t, s, &v.State.Times, statePosition(t, 0, 1, 1), stateScope{Staff: stateInt(t, 2)}, stateKnown(repeatMeterValue("2", "2")), 93)
			case "stale-repaired":
				statePublish(t, s, &v.State.Times, statePosition(t, 0, 0, 1), stateScope{Staff: stateInt(t, 2)}, stateKnown(repeatMeterValue("2", "2")), 93)
				statePublish(t, s, &v.State.Times, statePosition(t, 0, 1, 1), stateScope{Staff: stateInt(t, 2)}, stateKnown(repeatMeterValue("4", "4")), 94)
			case "future":
				statePublish(t, s, &v.State.Times, statePosition(t, 0, 3, 1), stateScope{Staff: stateInt(t, 2)}, stateKnown(repeatMeterValue("2", "2")), 93)
			case "outside-count":
				statePublish(t, s, &v.State.Times, statePosition(t, 0, 1, 1), stateScope{Staff: stateInt(t, 3)}, stateKnown(repeatMeterValue("2", "2")), 93)
			}
			r := &contextReader{ctx: context.Background(), s: s}
			got := contextRepeatUniformBeat(r, v, pos)
			if r.err != nil {
				t.Fatal(r.err)
			}
			known := mode != "missing" && mode != "conflict"
			if (got.State == EvidenceKnown) != known || known && got.Value.String() != "1" {
				t.Fatalf("mode=%s %+v", mode, got)
			}
		})
	}
}
func TestContextRepeatUniformMeterFieldLocalCauses(t *testing.T) {
	for _, field := range []string{"print", "beat", "count"} {
		s, v, pos := repeatUniformFixture(t, stateInt(t, 2))
		cause := &DecisionRef{Rule: RuleKey{RuleID("bad-" + field), 1}, Subject: NodeRef{"d", 99}}
		meter := repeatMeterValue("4", "4")
		if field == "print" {
			meter.PrintObject = Evidence[bool]{State: EvidenceInvalid, Cause: cause}
		}
		if field == "beat" {
			meter.Pairs[0].BeatType = Evidence[string]{State: EvidenceInvalid, Cause: cause}
		}
		if field == "count" {
			statePublish(t, s, &v.State.Staves, statePosition(t, 0, 1, 1), stateScope{AllStaves: true}, Evidence[Integer]{State: EvidenceInvalid, Cause: cause}, 93)
		}
		statePublish(t, s, &v.State.Times, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(meter), 91)
		r := &contextReader{ctx: context.Background(), s: s}
		got := contextRepeatUniformBeat(r, v, pos)
		if r.err != nil {
			t.Fatal(r.err)
		}
		if field == "print" {
			if got.State != EvidenceKnown || got.Value.String() != "1" {
				t.Fatalf("display gap poisoned beat %+v", got)
			}
		} else if got.State != EvidenceInvalid || got.Cause == nil || got.Cause.Subject != cause.Subject {
			t.Fatalf("field=%s causal gap lost %+v", field, got)
		}
		s.cleanup()
	}
}
func TestContextRepeatUniformMeterCancellationAndQuota(t *testing.T) {
	build := func() (*Session, *timelineService, musicalPosition) {
		s, v, pos := repeatUniformFixture(t, stateInt(t, 2))
		for i := int64(1); i <= 2; i++ {
			statePublish(t, s, &v.State.Times, statePosition(t, 0, 0, 1), stateScope{Staff: stateInt(t, i)}, stateKnown(repeatMeterValue("4", "4")), NodeID(90+i))
		}
		gapAt := statePosition(t, 0, 1, 2)
		v.State.Times.Gaps = append(v.State.Times.Gaps, stateGap{Part: pos.Part, Position: &gapAt, Availability: stateUnknown[bool]("unknown-selector-before-repair")})
		statePublish(t, s, &v.State.Times, statePosition(t, 0, 1, 1), stateScope{AllStaves: true}, stateKnown(repeatMeterValue("4", "4")), 99)
		return s, v, pos
	}
	s, v, pos := build()
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	r := &contextReader{ctx: counter, s: s}
	got := contextRepeatUniformBeat(r, v, pos)
	if r.err != nil || got.State != EvidenceKnown {
		t.Fatalf("probe %+v %v", got, r.err)
	}
	calls := counter.calls
	s.cleanup()
	for at := 1; at <= calls; at++ {
		s, v, pos := build()
		r := &contextReader{ctx: &rationalCancelContext{Context: context.Background(), at: at}, s: s}
		_ = contextRepeatUniformBeat(r, v, pos)
		if !errors.Is(r.err, context.Canceled) {
			t.Fatalf("poll%d %v", at, r.err)
		}
		work, facts := rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetContextFacts)
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || rationalUsage(t, s, BudgetRuleSteps) != work || rationalUsage(t, s, BudgetContextFacts) != facts {
			t.Fatalf("poll%d leaked/refunded", at)
		}
	}
	t.Logf("interrupted every one of %d uniform-meter polls", calls)
}

func TestContextRepeatUniformMeterUnknownSelectorChronology(t *testing.T) {
	for _, future := range []bool{false, true} {
		s, v, pos := repeatUniformFixture(t, stateInt(t, 2))
		cause := &DecisionRef{Rule: RuleKey{"bad-meter-staff", 1}, Subject: NodeRef{"d", 99}}
		statePublish(t, s, &v.State.Times, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(repeatMeterValue("4", "4")), 91)
		at := statePosition(t, 0, 1, 1)
		if future {
			at = statePosition(t, 0, 3, 1)
		}
		// Deliberately unclassified selector in a retained source gap. Its known
		// future-only position must be excluded before selector-domain reasoning.
		v.State.Times.Gaps = append(v.State.Times.Gaps, stateGap{Part: pos.Part, Position: &at, Scope: stateScope{}, Availability: Evidence[bool]{State: EvidenceInvalid, Cause: cause}})
		r := &contextReader{ctx: context.Background(), s: s}
		got := contextRepeatUniformBeat(r, v, pos)
		if r.err != nil {
			t.Fatal(r.err)
		}
		if future {
			if got.State != EvidenceKnown || got.Value.String() != "1" {
				t.Fatalf("future selector poisoned beat %+v", got)
			}
		} else if got.State != EvidenceInvalid || got.Cause == nil || got.Cause.Subject != cause.Subject {
			t.Fatalf("relevant unknown selector lost cause %+v", got)
		}
		s.cleanup()
	}
}

func TestContextRepeatUniformMeterSharedQuota(t *testing.T) {
	s, v, pos := repeatUniformFixture(t, stateInt(t, 2))
	statePublish(t, s, &v.State.Times, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(repeatMeterValue("4", "4")), 91)
	fill := s.account.Limits().MaxBufferedBytes - rationalUsage(t, s, BudgetBufferedBytes) - 1
	if err := s.account.Charge(BudgetBufferedBytes, fill); err != nil {
		t.Fatal(err)
	}
	before := rationalUsage(t, s, BudgetBufferedBytes)
	r := &contextReader{ctx: context.Background(), s: s}
	_ = contextRepeatUniformBeat(r, v, pos)
	if !errors.Is(r.err, ErrResourceLimit) {
		t.Fatalf("unbounded uniform query %v", r.err)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != before {
		t.Fatal("failed uniform admission leaked scratch")
	}
	work, facts := rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetContextFacts)
	_ = s.account.Release(BudgetBufferedBytes, fill)
	s.cleanup()
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 || rationalUsage(t, s, BudgetRuleSteps) != work || rationalUsage(t, s, BudgetContextFacts) != facts {
		t.Fatal("quota cleanup refunded cumulative work")
	}
}

func TestContextRepeatUniformMeterCollectsLaterCausalGap(t *testing.T) {
	s, v, pos := repeatUniformFixture(t, stateInt(t, 2))
	defer s.cleanup()
	statePublish(t, s, &v.State.Times, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(repeatMeterValue("4", "4")), 91)
	at := statePosition(t, 0, 1, 1)
	cause := &DecisionRef{Rule: RuleKey{"late-invalid-selector", 1}, Subject: NodeRef{"d", 99}}
	v.State.Times.Gaps = append(v.State.Times.Gaps, stateGap{Part: pos.Part, Position: &at, Availability: stateUnknown[bool]("earlier-unknown-selector")}, stateGap{Part: pos.Part, Position: &at, Availability: Evidence[bool]{State: EvidenceInvalid, Cause: cause}})
	r := &contextReader{ctx: context.Background(), s: s}
	got := contextRepeatUniformBeat(r, v, pos)
	if r.err != nil || got.State != EvidenceInvalid || got.Cause == nil || got.Cause.Subject != cause.Subject {
		t.Fatalf("later causal gap discarded %+v %v", got, r.err)
	}
}

func TestContextRepeatUniformMeterZeroStaffDomain(t *testing.T) {
	s, v, pos := repeatUniformFixture(t, stateInt(t, 0))
	defer s.cleanup()
	statePublish(t, s, &v.State.Times, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(repeatMeterValue("4", "4")), 91)
	r := &contextReader{ctx: context.Background(), s: s}
	got := contextRepeatUniformBeat(r, v, pos)
	if r.err != nil || got.State != EvidenceUnknown {
		t.Fatalf("zero staff manufactured common beat %+v %v", got, r.err)
	}
}

func TestContextRepeatUniformMeterUnknownSelectorRepair(t *testing.T) {
	for _, mode := range []string{"all-staves", "exhaustive-numbered", "one-unrepaired", "unconfined"} {
		t.Run(mode, func(t *testing.T) {
			s, v, pos := repeatUniformFixture(t, stateInt(t, 2))
			defer s.cleanup()
			cause := &DecisionRef{Rule: RuleKey{"invalid-selector", 1}, Subject: NodeRef{"d", 99}}
			statePublish(t, s, &v.State.Times, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(repeatMeterValue("4", "4")), 91)
			at := statePosition(t, 0, 1, 2)
			gap := stateGap{Part: pos.Part, Position: &at, Scope: stateScope{}, Availability: Evidence[bool]{State: EvidenceInvalid, Cause: cause}}
			if mode == "unconfined" {
				gap.Position = nil
				gap.Placement.Displaced = true
			}
			v.State.Times.Gaps = append(v.State.Times.Gaps, gap)
			if mode == "all-staves" || mode == "unconfined" {
				statePublish(t, s, &v.State.Times, statePosition(t, 0, 1, 1), stateScope{AllStaves: true}, stateKnown(repeatMeterValue("4", "4")), 92)
			} else {
				statePublish(t, s, &v.State.Times, statePosition(t, 0, 1, 1), stateScope{Staff: stateInt(t, 1)}, stateKnown(repeatMeterValue("4", "4")), 92)
				if mode == "exhaustive-numbered" {
					statePublish(t, s, &v.State.Times, statePosition(t, 0, 1, 1), stateScope{Staff: stateInt(t, 2)}, stateKnown(repeatMeterValue("4", "4")), 93)
				}
			}
			r := &contextReader{ctx: context.Background(), s: s}
			got := contextRepeatUniformBeat(r, v, pos)
			if r.err != nil {
				t.Fatal(r.err)
			}
			repaired := mode == "all-staves" || mode == "exhaustive-numbered"
			if repaired {
				if got.State != EvidenceKnown || got.Value.String() != "1" {
					t.Fatalf("proved repair lost %+v", got)
				}
			} else if got.State != EvidenceInvalid || got.Cause == nil || got.Cause.Subject != cause.Subject {
				t.Fatalf("unrepaired class lost cause %+v", got)
			}
			if v.State.Times.Gaps[0].Scope.AllStaves || v.State.Times.Gaps[0].Availability.Cause != cause {
				t.Fatal("public gap mutated")
			}
		})
	}
}
