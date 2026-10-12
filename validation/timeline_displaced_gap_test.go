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

func TestTimelineDisplacedGapSelectedOffsetPrecedence(t *testing.T) {
	for _, ownZero := range []bool{false, true} {
		t.Run(map[bool]string{false: "ignored-parent-sound-no", true: "own-zero-overrides-parent"}[ownZero], func(t *testing.T) {
			ctx := context.Background()
			s, svc, b := runtimeFixture(t)
			attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
			offset := stateNode(b, timelineRoleDirectionOffset, 41, Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-visual-offset"})
			flag := "no"
			if ownZero {
				flag = "yes"
			}
			stateAttribute(b, offset, timelineRoleOffsetSound, Atomic{Kind: AtomicString, Text: flag})
			sound := stateNode(b, timelineRoleDirectionSound, 50, stateAbsent[Atomic]())
			stateAttribute(b, sound, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 120, 1)})
			if ownZero {
				sound.children = append(sound.children, stateNode(b, timelineRoleSoundOffset, 51, stateAtomicDecimal(t, 0, 1)))
			}
			direction := stateNode(b, timelineRoleDirection, 40, stateAbsent[Atomic](), offset, sound)
			if err := svc.readMeasure(ctx, s, b, runtimeBlock(b, 0, attrs, runtimeNote(t, b, 30, 1, false), direction)); err != nil {
				t.Fatal(err)
			}
			at := svc.Facts[50].Origin.Value
			got, err := svc.Playback.effectiveTempo(ctx, s, &svc.State, at)
			if err != nil || got.Value.State != EvidenceKnown || got.Value.Value.String() != "120" || len(svc.Playback.Tempo.Gaps) != 0 {
				t.Fatalf("irrelevant parent displacement tainted sound: %+v %v", got, err)
			}
		})
	}
}

func TestTimelineDisplacedGapRetainsKnownFlagAlternatives(t *testing.T) {
	ctx := context.Background()
	s, svc, b := runtimeFixture(t)
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	initial := stateNode(b, timelineRoleSound, 25, stateAbsent[Atomic]())
	stateAttribute(b, initial, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 60, 1)})
	offset := stateNode(b, timelineRoleDirectionOffset, 41, stateAtomicDecimal(t, 2, 1))
	stateAttribute(b, offset, timelineRoleOffsetSound, Atomic{Kind: AtomicString, Text: "yes"})
	cause := &DecisionRef{Rule: RuleKey{"invalid-sound-flag", 1}, Subject: NodeRef{"d", 41}}
	offset.attributes[0].Scalar.Value = Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-sound-flag", Cause: cause}
	sound := stateNode(b, timelineRoleDirectionSound, 50, stateAbsent[Atomic]())
	stateAttribute(b, sound, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 120, 1)})
	direction := stateNode(b, timelineRoleDirection, 40, stateAbsent[Atomic](), offset, sound)
	repair := stateNode(b, timelineRoleSound, 80, stateAbsent[Atomic]())
	stateAttribute(b, repair, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 90, 1)})
	block := runtimeBlock(b, 0, attrs, initial, runtimeNote(t, b, 30, 1, false), direction, runtimeNote(t, b, 60, 1, false), runtimeNote(t, b, 70, 2, false), repair)
	if err := svc.readMeasure(ctx, s, b, block); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		q     int64
		want  EvidenceState
		value string
	}{{0, EvidenceKnown, "60"}, {2, EvidenceInvalid, ""}, {4, EvidenceKnown, "90"}} {
		at := svc.Facts[30].Position.Value
		at.Offset = mustRational(t, tc.q, 1)
		got, err := svc.Playback.effectiveTempo(ctx, s, &svc.State, at)
		if err != nil || got.Value.State != tc.want || tc.want == EvidenceKnown && got.Value.Value.String() != tc.value {
			t.Fatalf("q%d bounded alternative=%+v %v", tc.q, got, err)
		}
	}
	raw := svc.Playback.Sounds[50]
	if raw.Origin.Placement.Lower == nil || raw.Origin.Placement.Upper == nil || raw.Origin.Placement.Lower.Offset.String() != "1" || raw.Origin.Placement.Upper.Offset.String() != "3" {
		t.Fatalf("lost exact flag alternatives: %+v", raw.Origin.Placement)
	}
	raw.Origin.Placement.Lower.Offset = mustRational(t, 99, 1)
	if svc.Facts[50].Placement.Lower.Offset.String() != "1" || svc.Playback.Tempo.Gaps[0].Placement.Lower.Offset.String() != "1" {
		t.Fatal("placement witnesses alias independent fact owners")
	}
}
