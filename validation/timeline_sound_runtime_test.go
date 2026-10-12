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

func TestTimelineRuntimeDirectionOffsetsKeepIndependentOrigins(t *testing.T) {
	for _, kind := range []timelineRole{timelineRoleDirectionSound, timelineRoleDirectionListening} {
		for _, tc := range []struct {
			name, flag string
			own        bool
			want       string
		}{{"default", "", false, "2"}, {"no", "no", false, "2"}, {"yes", "yes", false, "3"}, {"override-default", "", true, "4"}, {"override-no", "no", true, "4"}, {"override-yes", "yes", true, "4"}} {
			t.Run(string(rune(kind))+tc.name, func(t *testing.T) {
				s, svc, b := runtimeFixture(t)
				attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 2, 1)))
				offset := stateNode(b, timelineRoleDirectionOffset, 41, stateAtomicDecimal(t, 2, 1))
				if tc.flag != "" {
					stateAttribute(b, offset, timelineRoleOffsetSound, Atomic{Kind: AtomicString, Text: tc.flag})
				}
				child := stateNode(b, kind, 50, stateAbsent[Atomic]())
				if tc.own {
					role := timelineRoleSoundOffset
					if kind == timelineRoleDirectionListening {
						role = timelineRoleListeningOffset
					}
					child.children = []*factNode{stateNode(b, role, 51, stateAtomicDecimal(t, 4, 1))}
				}
				direction := stateNode(b, timelineRoleDirection, 40, stateAbsent[Atomic](), offset, child)
				if err := svc.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, attrs, runtimeNote(t, b, 30, 4, false), direction, runtimeNote(t, b, 60, 2, false))); err != nil {
					t.Fatal(err)
				}
				for node, want := range map[NodeID]string{40: "3", 50: tc.want, 60: "2"} {
					got := svc.Facts[node].Position
					if got.State != EvidenceKnown || got.Value.Offset.String() != want {
						t.Fatalf("node %d=%+v want %s", node, got, want)
					}
				}
			})
		}
	}
}

func TestTimelineRuntimePublishesNoteAndFiguredBassOrigins(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 2, 1)))
	figure := stateNode(b, timelineRoleFiguredBass, 25, stateAbsent[Atomic](), stateNode(b, timelineRoleFiguredBassFigure, 26, stateAbsent[Atomic]()))
	note := runtimeNote(t, b, 30, 4, false)
	stateAttribute(b, note, timelineRoleNoteAttack, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, -1, 1)})
	stateAttribute(b, note, timelineRoleNoteRelease, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 1, 1)})
	if err := svc.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, attrs, figure, note)); err != nil {
		t.Fatal(err)
	}
	origin, ok := svc.NoteOrigins[30]
	if !ok {
		t.Fatal("missing note timing origins")
	}
	for label, p := range map[string]Evidence[musicalPosition]{"onset": origin.ScoreOnset, "end": origin.ScoreEnd, "attack": origin.Attack.Position, "release": origin.Release.Position} {
		want := map[string]string{"onset": "0", "end": "2", "attack": "-1/2", "release": "5/2"}[label]
		if p.State != EvidenceKnown || p.Value.Offset.String() != want {
			t.Fatalf("%s=%+v want%s", label, p, want)
		}
	}
	bass, ok := svc.FiguredBassOrigins[25]
	if !ok || bass.Anchor.State != EvidenceKnown || bass.Anchor.Value.Node != 30 {
		t.Fatalf("figured bass=%+v exists=%v", bass, ok)
	}
}

func TestTimelineRuntimePublishesPlaybackAndFigureUnits(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	ctx := context.Background()
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 2, 1)))
	sound := stateNode(b, timelineRoleSound, 23, stateAbsent[Atomic]())
	stateAttribute(b, sound, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 120, 1)})
	figure := stateNode(b, timelineRoleFiguredBass, 25, stateAbsent[Atomic](), stateNode(b, timelineRoleFiguredBassDuration, 26, stateAtomicDecimal(t, 1, 1)))
	note := runtimeNote(t, b, 30, 4, false)
	barline := stateNode(b, timelineRoleBarline, 40, stateAbsent[Atomic]())
	if err := svc.readMeasure(ctx, s, b, runtimeBlock(b, 0, attrs, sound, figure, note, barline)); err != nil {
		t.Fatal(err)
	}
	bass := svc.FiguredBassOrigins[25]
	if bass.Duration.State != EvidenceKnown || bass.Duration.Value.String() != "1/2" {
		t.Fatalf("figure=%+v", bass)
	}
	if len(svc.Playback.Notes) != 1 || len(svc.Playback.Sounds) != 1 || len(svc.Playback.Barlines) != 1 {
		t.Fatalf("missing playback maps notes=%d sounds=%d bars=%d", len(svc.Playback.Notes), len(svc.Playback.Sounds), len(svc.Playback.Barlines))
	}
	at := svc.Facts[30].Position.Value
	at.Offset = mustRational(t, 1, 1)
	tempo, err := svc.Playback.effectiveTempo(ctx, s, &svc.State, at)
	if err != nil || tempo.Value.State != EvidenceKnown || tempo.Value.Value.String() != "120" {
		t.Fatalf("tempo=%+v %v", tempo, err)
	}
}

func TestTimelineRuntimeUnknownPublisherGapsOnlyPotentialFamilies(t *testing.T) {
	for _, role := range []timelineRole{timelineRoleSound, timelineRoleDirection, timelineRoleAttributes} {
		t.Run(string(rune(role)), func(t *testing.T) {
			s, svc, b := runtimeFixture(t)
			ctx := context.Background()
			typ := ComponentID{Kind: ComponentComplexType, Name: QName{Local: "test-publisher"}}
			b.Roles[role][0].DeclaredTypeKnown = true
			b.Roles[role][0].DeclaredType = typ
			unknown := stateNode(b, role, 50, stateAbsent[Atomic]())
			unknown.header.Declaration = stateUnknown[DeclarationID]("unbound-publisher")
			unknown.header.Type = stateKnown(typ)
			if err := svc.readMeasure(ctx, s, b, runtimeBlock(b, 0, unknown)); err != nil {
				t.Fatal(err)
			}
			if role == timelineRoleAttributes {
				if len(svc.State.InstrumentGaps) != 0 || len(svc.Playback.Tempo.Gaps) != 0 {
					t.Fatal("attribute-only publisher damaged sound/instrument state")
				}
				return
			}
			if len(svc.State.Keys.Gaps) != 0 || len(svc.State.Divisions.Gaps) != 0 {
				t.Fatal("sound-only publisher damaged attribute state")
			}
			if len(svc.State.InstrumentGaps) != 1 || len(svc.Playback.Tempo.Gaps) != 1 {
				t.Fatalf("missing possible sound gap: instruments=%d tempo=%d", len(svc.State.InstrumentGaps), len(svc.Playback.Tempo.Gaps))
			}
		})
	}
}
