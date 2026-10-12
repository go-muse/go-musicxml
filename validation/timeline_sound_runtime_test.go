package validation

import (
	"context"
	"testing"
)

func TestIndependentReviewStandaloneSoundOffset(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 2, 1)))
	sound := stateNode(b, timelineRoleSound, 50, stateAbsent[Atomic](), stateNode(b, timelineRoleSoundPlay, 51, stateAbsent[Atomic](), stateNode(b, timelineRolePlayIpa, 52, stateAtomicText("a"))), stateNode(b, timelineRoleSoundOffset, 53, stateAtomicDecimal(t, 2, 1)))
	if err := svc.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, attrs, runtimeNote(t, b, 30, 4, false), sound)); err != nil {
		t.Fatal(err)
	}
	if len(svc.State.Play) != 1 {
		t.Fatalf("raw sound records=%d", len(svc.State.Play))
	}
	got := svc.State.Play[0].Position
	if got.State != EvidenceKnown || got.Value.Offset.String() != "3" {
		t.Fatalf("sound offset ignored: %+v", got)
	}
}
func TestIndependentReviewNestedSound(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 2, 1)))
	direction := stateNode(b, timelineRoleDirection, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleDirectionSound, 50, stateAbsent[Atomic](), stateNode(b, timelineRoleSoundPlay, 51, stateAbsent[Atomic](), stateNode(b, timelineRolePlayIpa, 52, stateAtomicText("a")))))
	if err := svc.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, attrs, direction)); err != nil {
		t.Fatal(err)
	}
	if len(svc.State.Play) != 1 {
		t.Fatalf("nested sound records=%d; want 1", len(svc.State.Play))
	}
}
