package validation

import (
	"context"
	"math/big"
	"testing"
)

// One exact staff per unrelated part has one selected note, so scanning every
// other part for every graphical record is avoidable, not output-sized work.
func TestIndependentContextRepeatContentScopeLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		staff, err := NewInteger(big.NewInt(1))
		if err != nil {
			t.Fatal(err)
		}
		notes := make([]contextRepeatNote, count)
		for i := range notes {
			notes[i] = contextRepeatNote{source: NodeRef{"d", NodeID(i + 1)}, part: stateKnown(NodeRef{"d", NodeID(10000 + i)}), staff: stateKnown(staff), available: stateKnown(true)}
		}
		roster, freeRoster, err := contextRepeatBuildContentRoster(context.Background(), s, notes)
		if err != nil {
			t.Fatal(err)
		}
		defer freeRoster()
		for i := range notes {
			row := RepeatContextResult{AllStaves: stateKnown(false), Staff: stateKnown(staff), Part: notes[i].part}
			partNotes, freePart, err := roster.partNotes(context.Background(), s, row.Part.Value)
			if err != nil {
				t.Fatal(err)
			}
			selected, complete, release, err := contextRepeatScopedNotes(context.Background(), s, partNotes, row)
			if err != nil {
				t.Fatal(err)
			}
			if complete.State != EvidenceKnown || !complete.Value || len(selected) != 1 || selected[0].source != notes[i].source {
				t.Fatalf("exact unrelated part scope changed: %+v %d", complete, len(selected))
			}
			release()
			freePart()
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("100 unrelated parts/staff rows: %d steps; 1000: %d steps; ratio %.2f", small, large, float64(large)/float64(small))
	if large > small*15 {
		t.Fatalf("staff-scoped content rescanned unrelated parts: %d > 15*%d", large, small)
	}
}
