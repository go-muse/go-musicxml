package validation

import (
	"context"
	"errors"
	"testing"
)

func orderedRepeatNotes(t *testing.T, count int) ([]contextRepeatNote, []contextRepeatNote) {
	t.Helper()
	source, target := make([]contextRepeatNote, count), make([]contextRepeatNote, count)
	for i := range source {
		source[i] = contextRepeatNote{
			pos: statePosition(t, 0, int64(i), 1), duration: mustRational(t, 1, 1),
			available: stateKnown(true), part: stateKnown(NodeRef{"d", 1}),
			voice: stateKnown("1"), step: stateKnown("C"), octave: stateKnown(stateInt(t, 4)),
			staff: stateAbsent[Integer](), alter: stateAbsent[Decimal](), instrument: stateAbsent[string](),
		}
		target[i] = source[i]
		target[i].pos.MeasureOrdinal = 1
	}
	return source, target
}

func TestContextRepeatOrderedProofBoundary(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func([]contextRepeatNote)
		want bool
	}{
		{"same", func([]contextRepeatNote) {}, true},
		{"natural-alter", func(v []contextRepeatNote) { v[0].alter = stateKnown(stateDecimal(t, 0, 1)) }, true},
		{"offset", func(v []contextRepeatNote) { v[0].pos.Offset = mustRational(t, 1, 2) }, false},
		{"duration", func(v []contextRepeatNote) { v[0].duration = mustRational(t, 1, 2) }, false},
		{"pitch", func(v []contextRepeatNote) { v[0].step = stateKnown("D") }, false},
		{"voice", func(v []contextRepeatNote) { v[0].voice = stateKnown("2") }, false},
		{"staff", func(v []contextRepeatNote) { v[0].staff = stateKnown(stateInt(t, 1)) }, false},
		{"instrument", func(v []contextRepeatNote) { v[0].instrument = stateKnown("I2") }, false},
		{"chord", func(v []contextRepeatNote) { v[0].chord = true }, false},
		{"rest", func(v []contextRepeatNote) { v[0].rest = true }, false},
		{"unpitched", func(v []contextRepeatNote) { v[0].unpitched = true }, false},
		{"unavailable", func(v []contextRepeatNote) { v[0].available = stateUnknown[bool]("unavailable") }, false},
		{"reordered", func(v []contextRepeatNote) { v[0], v[1] = v[1], v[0] }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, target := orderedRepeatNotes(t, 2)
			tc.edit(target)
			for _, measure := range []bool{false, true} {
				s := rationalSession(t, Budgets{})
				var equal bool
				var err error
				if measure {
					proof, e := contextRepeatSelectedMeasureEqual(context.Background(), s, source, target)
					equal, err = proof.State == EvidenceKnown && proof.Value, e
				} else {
					equal, err = contextRepeatOrderedEqual(context.Background(), s, source, target)
				}
				if err != nil || equal != (tc.want || measure && tc.name == "reordered") {
					t.Fatalf("measure=%t: equal=%t, err=%v", measure, equal, err)
				}
				if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
					t.Fatalf("measure=%t retained %d scratch bytes", measure, got)
				}
				s.cleanup()
			}
		})
	}
	for _, lengths := range [][2]int{{0, 0}, {0, 1}, {1, 0}, {1, 2}, {2, 1}} {
		source, target := orderedRepeatNotes(t, 2)
		source, target = source[:lengths[0]], target[:lengths[1]]
		s := rationalSession(t, Budgets{})
		if equal, err := contextRepeatOrderedEqual(context.Background(), s, source, target); err != nil || equal {
			t.Fatalf("lengths %v: beat equal=%t err=%v", lengths, equal, err)
		}
		if proof, err := contextRepeatSelectedMeasureEqual(context.Background(), s, source, target); err != nil || proof.State != EvidenceUnknown {
			t.Fatalf("lengths %v: measure proof=%+v err=%v", lengths, proof, err)
		}
		s.cleanup()
	}
}

func TestContextRepeatOrderedProofFallback(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		source, target := orderedRepeatNotes(t, 2)
		target[0], target[1] = target[1], target[0]
		cause := &DecisionRef{Rule: RuleKey{ID: "ordered-fallback-cause"}, Subject: NodeRef{"d", 42}}
		if unavailable {
			target[0].available = Evidence[bool]{State: EvidenceInvalid, Reason: "preserve-original-fallback", Cause: cause}
		}
		s := rationalSession(t, Budgets{})
		equal, err := contextRepeatOrderedEqual(context.Background(), s, source, target)
		if err != nil || equal {
			t.Fatalf("unavailable=%t fast path did not decline: %t %v", unavailable, equal, err)
		}
		proof, err := contextRepeatSelectedMeasureEqual(context.Background(), s, source, target)
		if err != nil {
			t.Fatal(err)
		}
		if unavailable {
			if proof.State != EvidenceInvalid || proof.Reason != "preserve-original-fallback" || proof.Cause != cause {
				t.Fatalf("fallback changed unavailable cause: %+v", proof)
			}
		} else if proof.State != EvidenceKnown || !proof.Value {
			t.Fatalf("fallback lost reordered equivalence: %+v", proof)
		}
		s.cleanup()
	}
}

func TestContextRepeatOrderedProofFallbackResources(t *testing.T) {
	source, target := orderedRepeatNotes(t, 3)
	target[0], target[2] = target[2], target[0]
	probe := &rationalCancelContext{Context: context.Background(), at: 1000000}
	s := rationalSession(t, Budgets{})
	proof, err := contextRepeatSelectedMeasureEqual(probe, s, source, target)
	if err != nil || proof.State != EvidenceKnown || !proof.Value {
		t.Fatalf("fallback probe: %+v %v", proof, err)
	}
	steps := rationalUsage(t, s, BudgetRuleSteps)
	s.cleanup()
	for at := 1; at <= probe.calls; at++ {
		s := rationalSession(t, Budgets{})
		proof, err := contextRepeatSelectedMeasureEqual(&rationalCancelContext{Context: context.Background(), at: at}, s, source, target)
		if proof.State != "" || !errors.Is(err, context.Canceled) || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("fallback poll %d published/retained canceled proof: %+v %v", at, proof, err)
		}
		s.cleanup()
	}
	for _, limit := range []int64{steps, steps - 1} {
		s := rationalSession(t, Budgets{MaxRuleSteps: limit})
		proof, err := contextRepeatSelectedMeasureEqual(context.Background(), s, source, target)
		if limit == steps && (err != nil || proof.State != EvidenceKnown || !proof.Value) || limit < steps && (err == nil || proof.State != "") || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("fallback step limit %d: %+v %v", limit, proof, err)
		}
		s.cleanup()
	}
	// An unavailable source declines before rational arithmetic, isolating the
	// fallback mask's exact allocation from temporary numeric scratch.
	source[0].available = stateUnknown[bool]("mask-boundary")
	for _, limit := range []int64{3, 2} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		proof, err := contextRepeatSelectedMeasureEqual(context.Background(), s, source, target)
		if limit == 3 && (err != nil || proof.State != EvidenceUnknown || proof.Reason != "mask-boundary") || limit == 2 && (err == nil || proof.State != "") || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("fallback byte limit %d: %+v %v", limit, proof, err)
		}
		s.cleanup()
	}
	t.Logf("fallback exact admission %d steps; all %d cancellation polls and exact/one-under mask bytes preserve zero scratch", steps, probe.calls)
}

func TestContextRepeatOrderedProofResourceBoundaries(t *testing.T) {
	source, target := orderedRepeatNotes(t, 3)
	for _, measure := range []bool{false, true} {
		call := func(ctx context.Context, s *Session) (bool, error) {
			if measure {
				proof, err := contextRepeatSelectedMeasureEqual(ctx, s, source, target)
				return proof.State == EvidenceKnown && proof.Value, err
			}
			return contextRepeatOrderedEqual(ctx, s, source, target)
		}
		probe := &rationalCancelContext{Context: context.Background(), at: 1000000}
		s := rationalSession(t, Budgets{})
		if equal, err := call(probe, s); err != nil || !equal {
			t.Fatalf("measure=%t probe: %t %v", measure, equal, err)
		}
		steps := rationalUsage(t, s, BudgetRuleSteps)
		s.cleanup()
		for _, limit := range []int64{steps, steps - 1} {
			s := rationalSession(t, Budgets{MaxRuleSteps: limit})
			equal, err := call(context.Background(), s)
			if limit == steps && (err != nil || !equal) || limit < steps && (err == nil || equal) {
				t.Fatalf("measure=%t limit=%d: equal=%t err=%v", measure, limit, equal, err)
			}
			if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
				t.Fatalf("measure=%t budget boundary retained %d", measure, got)
			}
			s.cleanup()
		}
		for at := 1; at <= probe.calls; at++ {
			s := rationalSession(t, Budgets{})
			equal, err := call(&rationalCancelContext{Context: context.Background(), at: at}, s)
			if equal || !errors.Is(err, context.Canceled) || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatalf("measure=%t poll=%d published/retained canceled proof: %t %v", measure, at, equal, err)
			}
			s.cleanup()
		}
		t.Logf("measure=%t: exact admission %d steps; all %d cancellation polls preserve zero scratch", measure, steps, probe.calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	if equal, err := contextRepeatOrderedEqual(ctx, s, nil, nil); equal || !errors.Is(err, context.Canceled) {
		t.Fatalf("empty beat cancellation: %t %v", equal, err)
	}
	if proof, err := contextRepeatSelectedMeasureEqual(ctx, s, nil, nil); proof.State == EvidenceKnown || !errors.Is(err, context.Canceled) {
		t.Fatalf("empty measure cancellation: %+v %v", proof, err)
	}
}

func TestContextRepeatOrderedProofLinearWork(t *testing.T) {
	for _, measure := range []bool{false, true} {
		work := func(count int) int64 {
			source, target := orderedRepeatNotes(t, count)
			s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34})
			defer s.cleanup()
			var equal bool
			var err error
			if measure {
				proof, e := contextRepeatSelectedMeasureEqual(context.Background(), s, source, target)
				equal, err = proof.State == EvidenceKnown && proof.Value, e
			} else {
				equal, err = contextRepeatOrderedEqual(context.Background(), s, source, target)
			}
			if err != nil || !equal || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatalf("measure=%t count=%d: equal=%t err=%v", measure, count, equal, err)
			}
			return rationalUsage(t, s, BudgetRuleSteps)
		}
		small, large := work(100), work(1000)
		t.Logf("measure=%t: 100 atoms %d steps; 1000 atoms %d steps; ratio %.2f", measure, small, large, float64(large)/float64(small))
		if large > small*15 {
			t.Fatalf("measure=%t ordered proof grew superlinearly: %d > 15*%d", measure, large, small)
		}
	}
}
