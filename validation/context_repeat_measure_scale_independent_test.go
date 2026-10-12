package validation

import (
	"context"
	"testing"
)

// Identically ordered sequential content is the common sufficient equality
// proof; reordered/ambiguous content may retain a bounded fallback.
func TestIndependentContextRepeatOrderedMeasureLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		part := NodeRef{"d", 1}
		notes := make([]contextRepeatNote, 0, 2*count)
		for m := uint64(0); m < 2; m++ {
			for i := 0; i < count; i++ {
				pos := statePosition(t, m, int64(i), 1)
				pos.Part = part
				notes = append(notes, contextRepeatNote{part: stateKnown(part), pos: pos, duration: statePosition(t, 0, 1, 1).Offset, rest: true, available: stateKnown(true), staff: stateAbsent[Integer](), voice: stateAbsent[string](), step: stateAbsent[string](), alter: stateAbsent[Decimal](), octave: stateAbsent[Integer](), instrument: stateAbsent[string]()})
			}
		}
		same, err := contextRepeatMeasureEqual(context.Background(), s, notes, part, 0, 1)
		if err != nil || same.State != EvidenceKnown || !same.Value {
			t.Fatalf("ordered exact equivalence lost: %+v %v", same, err)
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("100 notes per ordered measure: %d steps; 1000: %d steps; ratio %.2f", small, large, float64(large)/float64(small))
	if large > small*15 {
		t.Fatalf("ordinary ordered repeat equality is superlinear: %d > 15*%d", large, small)
	}
}
