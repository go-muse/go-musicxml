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
	block := runtimeBlock(b, 0, attrs, figure, note)
	if err := svc.readMeasure(context.Background(), s, b, block); err != nil {
		t.Fatal(err)
	}
	if err := svc.finalizeTimingOrigins(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
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
	block := runtimeBlock(b, 0, attrs, sound, figure, note, barline)
	if err := svc.readMeasure(ctx, s, b, block); err != nil {
		t.Fatal(err)
	}
	if err := svc.finalizeTimingOrigins(ctx, s, b, []timelineMeasureBlock{block}); err != nil {
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
			typ := ComponentID{Kind: ComponentType, Name: QName{Local: "test-publisher"}}
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

func TestTimelineRuntimeOffsetsUseFinalOriginUnits(t *testing.T) {
	for _, role := range []timelineRole{timelineRoleHarmony, timelineRoleDirection, timelineRoleSound, timelineRoleListening} {
		t.Run(string(rune(role)), func(t *testing.T) {
			s, svc, b := runtimeFixture(t)
			d2 := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 2, 1)))
			d4 := stateNode(b, timelineRoleAttributes, 60, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 61, stateAtomicDecimal(t, 4, 1)))
			offrole := map[timelineRole]timelineRole{timelineRoleHarmony: timelineRoleHarmonyOffset, timelineRoleDirection: timelineRoleDirectionOffset, timelineRoleSound: timelineRoleSoundOffset, timelineRoleListening: timelineRoleListeningOffset}[role]
			off := stateNode(b, offrole, 41, stateAtomicDecimal(t, 2, 1))
			event := stateNode(b, role, 40, stateAbsent[Atomic](), off)
			if role == timelineRoleDirection {
				stateAttribute(b, off, timelineRoleOffsetSound, Atomic{Kind: AtomicString, Text: "yes"})
				event.children = append(event.children, stateNode(b, timelineRoleDirectionSound, 50, stateAbsent[Atomic]()), stateNode(b, timelineRoleDirectionListening, 55, stateAbsent[Atomic]()))
			}
			if err := svc.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, d2, runtimeNote(t, b, 30, 2, false), event, d4, runtimeNote(t, b, 70, 4, false))); err != nil {
				t.Fatal(err)
			}
			ids := []NodeID{40}
			if role == timelineRoleDirection {
				ids = append(ids, 50, 55)
			}
			for _, id := range ids {
				fact := svc.Facts[id]
				if fact.Position.State != EvidenceKnown || fact.Position.Value.Offset.String() != "3/2" || fact.Origin.State != EvidenceKnown || fact.Origin.Value.Offset.String() != "1" || fact.Units.State != EvidenceKnown || fact.Units.Value.String() != "4" {
					t.Fatalf("node%d=%+v", id, fact)
				}
			}
			if got := svc.Facts[70].Position; got.State != EvidenceKnown || got.Value.Offset.String() != "1" {
				t.Fatalf("offset moved cursor %+v", got)
			}
		})
	}
}

func TestTimelineRuntimeScalarOffsetCanCrossKnownUnitBoundary(t *testing.T) {
	for _, forward := range []bool{false, true} {
		t.Run(map[bool]string{true: "forward", false: "backward"}[forward], func(t *testing.T) {
			s, svc, b := runtimeFixture(t)
			d2 := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 2, 1)))
			d4 := stateNode(b, timelineRoleAttributes, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 41, stateAtomicDecimal(t, 4, 1)))
			offset, want := int64(-3), "3/4"
			if forward {
				offset, want = 2, "3/2"
			}
			harmony := stateNode(b, timelineRoleHarmony, 60, stateAbsent[Atomic](), stateNode(b, timelineRoleHarmonyOffset, 61, stateAtomicDecimal(t, offset, 1)))
			events := []*factNode{d2, runtimeNote(t, b, 30, 2, false), d4, runtimeNote(t, b, 50, 2, false), harmony}
			if forward {
				events = []*factNode{d2, runtimeNote(t, b, 30, 1, false), harmony, runtimeNote(t, b, 35, 1, false), d4}
			}
			if err := svc.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, events...)); err != nil {
				t.Fatal(err)
			}
			got := svc.Facts[60].Position
			if got.State != EvidenceKnown || got.Value.Offset.String() != want {
				t.Fatalf("crossing scalar %+v want%s", got, want)
			}
		})
	}
}
