package validation

import (
	"context"
	"testing"
)

func TestIndependentContextRepeatSequentialBeatWindowsLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		part := NodeRef{"d", 1}
		notes := make([]contextRepeatNote, count)
		beat := statePosition(t, 0, 1, 1).Offset
		for i := range notes {
			pos := statePosition(t, 0, int64(i), 1)
			pos.Part = part
			notes[i] = contextRepeatNote{part: stateKnown(part), pos: pos, duration: beat, rest: true, available: stateKnown(true), staff: stateAbsent[Integer](), voice: stateAbsent[string](), step: stateAbsent[string](), alter: stateAbsent[Decimal](), octave: stateAbsent[Integer](), instrument: stateAbsent[string]()}
		}
		start := statePosition(t, 0, 1, 1)
		start.Part = part
		end := statePosition(t, 0, int64(count), 1)
		end.Part = part
		stop := NodeRef{"d", 3}
		row := contextRepeatRecord{source: NodeRef{"d", 2}, position: stateKnown(start), value: RepeatContextResult{Kind: RepeatStyleBeat, Type: stateKnown("start"), Part: stateKnown(part), AllStaves: stateKnown(true), Beat: stateKnown(beat), Complete: stateKnown(true), Stop: stateKnown(stop)}}
		rows := []contextRepeatRecord{row, {source: stop, position: stateKnown(end)}}
		if err := contextRepeatBeatProof(context.Background(), s, notes, rows, &row); err != nil {
			t.Fatal(err)
		}
		if row.value.ContentEquivalent.State != EvidenceKnown || !row.value.ContentEquivalent.Value {
			t.Fatalf("sequential beat equivalence lost: %+v", row.value.ContentEquivalent)
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("100 sequential beat windows: %d steps; 1000: %d steps; ratio %.2f", small, large, float64(large)/float64(small))
	if large > small*15 {
		t.Fatalf("each simple beat window rescanned whole measure: %d > 15*%d", large, small)
	}
}
