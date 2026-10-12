package validation

import (
	"context"
	"fmt"
	"testing"
)

// Exclusion strings can be unbounded input, but N memberships need no N*V
// whole-list rescans. Index construction is inside the measured production call.
func TestIndependentContextRepeatVoiceExclusionLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		timeline := &timelineService{Facts: make(map[NodeID]timelineEventFact), Playback: timelinePlaybackContext{Notes: make(map[NodeID]notePlaybackContext)}}
		part := NodeRef{"d", 10000}
		pos := statePosition(t, 0, 0, 1)
		pos.Part = part
		rows := []contextRepeatRecord{{source: NodeRef{"d", 20000}, position: stateKnown(pos), value: RepeatContextResult{Kind: RepeatStyleSlash, Part: stateKnown(part), AllStaves: stateKnown(true), Type: stateKnown("start"), Stop: stateAbsent[NodeRef](), Complete: stateKnown(true)}}}
		for i := 0; i < count; i++ {
			id := NodeID(i + 1)
			s.sequence = append(s.sequence, id)
			pos := statePosition(t, 0, 1, 1)
			pos.Part = part
			timeline.Playback.Notes[id] = notePlaybackContext{Origin: playbackOrigin{Source: NodeRef{"d", id}, Part: stateKnown(part)}}
			timeline.Facts[id] = timelineEventFact{Position: stateKnown(pos), Voice: stateKnown("active")}
			rows[0].value.ExceptVoices = append(rows[0].value.ExceptVoices, stateKnown(fmt.Sprintf("%06d", i)))
		}
		if err := contextRepeatNoteScopes(context.Background(), s, timeline, rows); err != nil {
			t.Fatal(err)
		}
		if len(rows[0].value.Notes) != count {
			t.Fatal("membership lost")
		}
		for _, note := range rows[0].value.Notes {
			if note.Applies.State != EvidenceKnown || !note.Applies.Value {
				t.Fatalf("unrelated exclusion contaminated active voice: %+v", note.Applies)
			}
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("100 notes/exclusions: %d steps; 1000: %d steps; ratio %.2f", small, large, float64(large)/float64(small))
	if large > small*15 {
		t.Fatalf("voice membership rescanned full exclusions per note: %d > 15*%d", large, small)
	}
}
