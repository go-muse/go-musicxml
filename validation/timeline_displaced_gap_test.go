package validation

import (
	"context"
	"testing"
)

func TestTimelineDisplacedGapCanAffectEarlierFrame(t *testing.T) {
	ctx := context.Background()
	s, svc, b := runtimeFixture(t)
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	initial := stateNode(b, timelineRoleSound, 30, stateAbsent[Atomic]())
	stateAttribute(b, initial, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 60, 1)})
	cause := &DecisionRef{Rule: RuleKey{"unavailable-signed-offset", 1}, Subject: NodeRef{"d", 41}}
	offset := stateNode(b, timelineRoleSoundOffset, 41, Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-signed-offset", Cause: cause})
	shifted := stateNode(b, timelineRoleSound, 40, stateAbsent[Atomic](), offset)
	stateAttribute(b, shifted, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 120, 1)})
	for _, block := range []timelineMeasureBlock{runtimeBlock(b, 0, attrs, initial, runtimeNote(t, b, 50, 1, false)), runtimeBlock(b, 1, shifted)} {
		if err := svc.readMeasure(ctx, s, b, block); err != nil {
			t.Fatal(err)
		}
	}
	got, err := svc.Playback.effectiveTempo(ctx, s, &svc.State, svc.Facts[50].Position.Value)
	if err != nil || got.Value.State != EvidenceInvalid || got.Value.Cause == nil || *got.Value.Cause != *cause {
		t.Fatalf("unconfined signed-offset gap excluded by measure label: %+v %v", got, err)
	}
}
func TestTimelineZeroOffsetDoesNotNeedUnits(t *testing.T) {
	ctx := context.Background()
	s, svc, b := runtimeFixture(t)
	sound := stateNode(b, timelineRoleSound, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleSoundOffset, 41, stateAtomicDecimal(t, 0, 1)))
	stateAttribute(b, sound, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 120, 1)})
	if err := svc.readMeasure(ctx, s, b, runtimeBlock(b, 0, sound)); err != nil {
		t.Fatal(err)
	}
	if got := svc.Facts[40]; got.Origin.State != EvidenceKnown || got.Position.State != EvidenceKnown || got.Position.Value.Offset.String() != "0" {
		t.Fatalf("zero displacement lost independent origin: %+v", got)
	}
}
