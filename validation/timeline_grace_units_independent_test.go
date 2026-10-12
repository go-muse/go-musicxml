package validation

import (
	"context"
	"testing"
)

func TestIndependentTimelineGraceUsesLateSameOriginUnits(t *testing.T) {
	s, v, b := runtimeFixture(t)
	d2 := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 2, 1)))
	grace := stateNode(b, timelineRoleNoteGrace, 41, stateAbsent[Atomic]())
	stateAttribute(b, grace, timelineRoleGraceMakeTime, stateAtomicDecimal(t, 2, 1).Value)
	n := stateNode(b, timelineRoleNote, 40, stateAbsent[Atomic](), grace)
	stateAttribute(b, n, timelineRoleNoteAttack, stateAtomicDecimal(t, 2, 1).Value)
	d4 := stateNode(b, timelineRoleAttributes, 50, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 51, stateAtomicDecimal(t, 4, 1)))
	if err := v.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, d2, runtimeNote(t, b, 30, 2, false), n, d4)); err != nil {
		t.Fatal(err)
	}
	out := v.NoteOrigins[40]
	units, err := v.State.effectiveDivisions(context.Background(), s, out.ScoreOnset.Value)
	if err != nil || units.Value.State != EvidenceKnown || units.Value.Value.String() != "4" {
		t.Fatalf("final effective units=%+v %v", units, err)
	}
	if got := out.GraceTiming.Value.MakeTime.Delta; got.State != EvidenceKnown || got.Value.String() != "1/2" {
		t.Errorf("grace make-time used stale units: %+v", got)
	}
	if got := out.Attack.Position; got.State != EvidenceKnown || got.Value.Offset.String() != "3/2" {
		t.Errorf("grace attack used stale units: %+v", got)
	}
}
