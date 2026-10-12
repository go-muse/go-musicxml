package validation

import (
	"context"
	"testing"
)

func TestIndependentContextRepeatSequentialMeasureRangeLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		part := NodeRef{"d", 1}
		notes := make([]contextRepeatNote, count)
		for i := range notes {
			pos := statePosition(t, uint64(i), 0, 1)
			pos.Part = part
			notes[i] = contextRepeatNote{part: stateKnown(part), pos: pos, duration: statePosition(t, 0, 1, 1).Offset, rest: true, available: stateKnown(true), staff: stateAbsent[Integer](), voice: stateAbsent[string](), step: stateAbsent[string](), alter: stateAbsent[Decimal](), octave: stateAbsent[Integer](), instrument: stateAbsent[string]()}
		}
		roster, release, err := contextRepeatBuildContentRoster(context.Background(), s, notes)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		timeline := &timelineService{}
		timeline.State.Closures = []stateClosure{{Part: part, Through: uint64(count - 1), Complete: stateKnown(true)}}
		pos := statePosition(t, 1, 0, 1)
		pos.Part = part
		row := contextRepeatRecord{source: NodeRef{"d", 2}, position: stateKnown(pos), value: RepeatContextResult{Kind: RepeatStyleMeasure, Type: stateKnown("start"), Part: stateKnown(part), AllStaves: stateKnown(true), PatternMeasures: stateKnown(localInteger(t, 1).Integer), Complete: stateKnown(true), Stop: stateAbsent[NodeRef]()}}
		if err := contextRepeatProveRosterRow(context.Background(), s, timeline, roster, nil, &row, stateKnown(true)); err != nil {
			t.Fatal(err)
		}
		if row.value.ContentEquivalent.State != EvidenceKnown || !row.value.ContentEquivalent.Value {
			t.Fatalf("sequential repeat equivalence lost: %+v", row.value.ContentEquivalent)
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("100 sequential measures: %d steps; 1000: %d steps; ratio %.2f", small, large, float64(large)/float64(small))
	if large > small*15 {
		t.Fatalf("each simple measure rescanned whole part: %d > 15*%d", large, small)
	}
}
