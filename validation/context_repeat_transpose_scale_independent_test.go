package validation

import (
	"context"
	"testing"
)

// The part-wide presence answer is invariant while the note snapshot is built.
// Include sparse publications and gaps, then ask from both affected and unrelated
// parts. Tenfold ordinary input permits 15x work, including any index build.
func TestIndependentContextRepeatTransposePresenceLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		s.nodes = make(map[NodeID]*factNode)
		timeline := &timelineService{Facts: make(map[NodeID]timelineEventFact), Playback: timelinePlaybackContext{Notes: make(map[NodeID]notePlaybackContext)}}
		bindings := &contextBindings{Elements: map[contextRole]map[DeclarationID]struct{}{contextRepeatNoteRest: {localDecl("rest"): {}}}}
		for i := 0; i < count; i++ {
			pos := statePosition(t, 0, 0, 1)
			pos.Part = NodeRef{"d", NodeID(10000 + 2*i)}
			timeline.State.Transpose.Publications = append(timeline.State.Transpose.Publications, statePublication[transposeState]{Position: pos})
			timeline.State.Transpose.Gaps = append(timeline.State.Transpose.Gaps, stateGap{Part: pos.Part})
			id := NodeID(i + 1)
			n := localNode(id, 0, "note")
			n.children = []*factNode{localNode(NodeID(100000+i), id, "rest")}
			s.nodes[id] = n
			pos.Part = NodeRef{"d", NodeID(10000 + i)}
			timeline.Facts[id] = timelineEventFact{Part: stateKnown(pos.Part), Position: stateKnown(pos), Duration: stateKnown(timelineZero), Staff: stateAbsent[Integer](), Voice: stateAbsent[string]()}
			timeline.Playback.Notes[id] = notePlaybackContext{Chord: stateKnown(false), TimeOnly: stateAbsent[string]()}
		}
		transformed, release, err := contextRepeatTransformParts(context.Background(), s, timeline)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		r := &contextReader{ctx: context.Background(), s: s, b: bindings}
		for i := 0; i < count; i++ {
			note, err := contextRepeatReadNote(r, timeline, transformed, NodeID(i+1))
			if err != nil {
				t.Fatal(err)
			}
			if i%2 == 0 {
				if note.available.State != EvidenceUnknown || note.available.Reason != "context-repeat-transposed-equivalence-unresolved" {
					t.Fatalf("affected part lost conservative evidence: %+v", note.available)
				}
			} else if note.available.State != EvidenceKnown || !note.available.Value {
				t.Fatalf("unrelated part contaminated: %+v", note.available)
			}
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("100 notes/publications/gaps: %d steps; 1000: %d steps; ratio %.2f", small, large, float64(large)/float64(small))
	if large > small*15 {
		t.Fatalf("part-wide transpose presence rescanned per note: %d > 15*%d", large, small)
	}
}
