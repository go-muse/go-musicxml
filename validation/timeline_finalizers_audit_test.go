package validation

import (
	"context"
	"testing"
)

func TestTimelineFinalizersPublishGraceDonorsAfterCompleteMeasures(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		s, service, b := runtimeFixture(t)
		attr := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
		grace := stateNode(b, timelineRoleNoteGrace, 41, stateAbsent[Atomic]())
		stateAttribute(b, grace, timelineRoleGraceStealPrevious, stateAtomicDecimal(t, 25, 1).Value)
		stateAttribute(b, grace, timelineRoleGraceStealFollowing, stateAtomicDecimal(t, 50, 1).Value)
		blocks := []timelineMeasureBlock{
			runtimeBlock(b, 0, attr, runtimeNote(t, b, 30, 1, false), stateNode(b, timelineRoleNote, 40, stateAbsent[Atomic](), grace)),
			runtimeBlock(b, 1, runtimeNote(t, b, 50, 2, false)),
		}
		for _, block := range blocks {
			block.content.end.Order = stateKnown(order)
			if err := service.readMeasure(context.Background(), s, b, block); err != nil {
				t.Fatal(err)
			}
		}
		if len(service.GraceNeighbors) != 0 {
			t.Fatal("grace donors published before complete-measure finalization")
		}
		if err := service.finalizeTimingOrigins(context.Background(), s, b, blocks); err != nil {
			t.Fatal(err)
		}
		got, exists := service.GraceNeighbors[40]
		if !exists || got.Previous.Note.State != EvidenceKnown || got.Previous.Note.Value.Node != 30 || got.Following.Note.State != EvidenceKnown || got.Following.Note.Value.Node != 50 {
			t.Fatalf("finalized cross-measure grace donor identities: %+v exists=%v", got, exists)
		}
		if got.Previous.StolenDuration.State != EvidenceKnown || got.Previous.StolenDuration.Value.String() != "1/4" || got.Following.StolenDuration.State != EvidenceKnown || got.Following.StolenDuration.Value.String() != "1" {
			t.Fatalf("finalized donor duration bases: %+v", got)
		}
		if service.NoteOrigins[40].Duration.State != EvidenceAbsent || service.NoteOrigins[40].GraceTiming.Value.PreviousNote.State != EvidenceUnknown {
			t.Fatal("finalization fabricated grace duration or mutated immutable raw origin")
		}
	}
}
