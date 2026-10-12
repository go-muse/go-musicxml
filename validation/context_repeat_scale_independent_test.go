package validation

import (
	"context"
	"testing"
)

func TestIndependentContextNavigationEdgesLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		var rows []contextNavigationNode
		for i := 0; i < count; i++ {
			rows = append(rows, contextNavigationNode{event: navigationEvent{Source: NodeRef{"d", NodeID(3*i + 1)}, Kind: NavigationForward}}, contextNavigationNode{event: navigationEvent{Source: NodeRef{"d", NodeID(3*i + 2)}, Kind: NavigationNote}}, contextNavigationNode{event: navigationEvent{Source: NodeRef{"d", NodeID(3*i + 3)}, Kind: NavigationBackward}, value: NavigationContextResult{Direction: stateKnown("backward")}})
		}
		if err := navigationBuildEdges(context.Background(), s, rows); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < count; i++ {
			if rows[3*i+2].event.Target.State != EvidenceKnown || rows[3*i+2].event.Target.Value != 3*i {
				t.Fatal("ordinary region association lost")
			}
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("100 regions: %d steps; 1000: %d steps; ratio %.2f", small, large, float64(large)/float64(small))
	if large > small*15 {
		t.Fatalf("ordinary navigation edges rescanned rows: %d > 15*%d", large, small)
	}
}

func TestIndependentContextRepeatSparseRangesLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		var rows []contextRepeatRecord
		for i := 0; i < count; i++ {
			part := NodeRef{"d", NodeID(i + 10000)}
			for j, role := range []string{"start", "stop"} {
				pos := statePosition(t, 0, int64(j), 1)
				pos.Part = part
				rows = append(rows, contextRepeatRecord{source: NodeRef{"d", NodeID(2*i + j + 1)}, position: stateKnown(pos), value: RepeatContextResult{Kind: RepeatStyleSlash, Part: stateKnown(part), AllStaves: stateKnown(true), Staff: stateAbsent[Integer](), Type: stateKnown(role)}})
			}
		}
		if err := contextRepeatPairRanges(context.Background(), s, &timelineService{}, rows); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < count; i++ {
			if rows[2*i].value.Stop.State != EvidenceKnown || rows[2*i].value.Stop.Value != rows[2*i+1].source {
				t.Fatal("independent part range lost")
			}
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("100 independent parts/ranges: %d steps; 1000: %d steps; ratio %.2f", small, large, float64(large)/float64(small))
	if large > small*15 {
		t.Fatalf("independent part ranges rescanned all rows: %d > 15*%d", large, small)
	}
}

func TestIndependentContextRepeatSparseNoteScopesLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		timeline := &timelineService{Facts: make(map[NodeID]timelineEventFact), Playback: timelinePlaybackContext{Notes: make(map[NodeID]notePlaybackContext)}}
		var rows []contextRepeatRecord
		for i := 0; i < count; i++ {
			part := NodeRef{"d", NodeID(i + 10000)}
			id := NodeID(i + 1)
			s.sequence = append(s.sequence, id)
			pos := statePosition(t, 0, 1, 1)
			pos.Part = part
			timeline.Playback.Notes[id] = notePlaybackContext{Origin: playbackOrigin{Source: NodeRef{"d", id}, Part: stateKnown(part)}}
			timeline.Facts[id] = timelineEventFact{Position: stateKnown(pos)}
			pos.Offset = timelineZero
			rows = append(rows, contextRepeatRecord{source: NodeRef{"d", NodeID(i + 20000)}, position: stateKnown(pos), value: RepeatContextResult{Kind: RepeatStyleSlash, Part: stateKnown(part), AllStaves: stateKnown(true), Type: stateKnown("start"), Stop: stateAbsent[NodeRef](), Complete: stateKnown(true)}})
		}
		if err := contextRepeatNoteScopes(context.Background(), s, timeline, rows); err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if len(row.value.Notes) != 1 || row.value.Notes[0].InRange.State != EvidenceKnown || !row.value.Notes[0].InRange.Value {
				t.Fatal("independent part membership lost")
			}
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("100 unrelated parts/notes: %d steps; 1000: %d steps; ratio %.2f", small, large, float64(large)/float64(small))
	if large > small*15 {
		t.Fatalf("unrelated parts' notes rescanned for each graphical range: %d > 15*%d", large, small)
	}
}
