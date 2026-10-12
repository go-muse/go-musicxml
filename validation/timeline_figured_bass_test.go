package validation

import (
	"context"
	"errors"
	"testing"
)

func figuredOriginFixture(t *testing.T, order OrderKind) (*Session, *timelineService, *timelineBindings, timelineMeasureBlock) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	t.Cleanup(s.cleanup)
	b := noteOriginBindings()
	// Musical order is 20, 40, 30. XML/model order is deliberately different.
	figure := stateNode(b, timelineRoleFiguredBass, 10, stateAbsent[Atomic](), stateNode(b, timelineRoleFiguredBassFigure, 11, stateAbsent[Atomic]()))
	early := runtimeNote(t, b, 20, 3, false)
	late := runtimeNote(t, b, 30, 3, false)
	nearest := runtimeNote(t, b, 40, 3, false)
	block := runtimeBlock(b, 0, figure, late, early, nearest)
	block.content.end.Order = stateKnown(order)
	service := &timelineService{Facts: map[NodeID]timelineEventFact{}}
	for id, offset := range map[NodeID]int64{10: 1, 20: 0, 30: 3, 40: 2} {
		service.Facts[id] = timelineEventFact{Position: timelineAt(block, stateKnown(mustRational(t, offset, 1))), Part: block.part, Staff: stateAbsent[Integer](), Voice: stateAbsent[string]()}
	}
	return s, service, b, block
}
func TestFiguredBassOriginUsesExactScoreOrderBothRepresentations(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		t.Run(string(order), func(t *testing.T) {
			s, service, b, block := figuredOriginFixture(t, order)
			if err := service.resolveFiguredBassOrigins(context.Background(), s, b, block); err != nil {
				t.Fatal(err)
			}
			got := service.FiguredBassOrigins[10]
			if got.Anchor.State != EvidenceKnown || got.Anchor.Value.Node != 40 {
				t.Fatalf("anchor = %+v", got)
			}
			noteOriginCheckPosition(t, got.CursorOrigin, "1")
			noteOriginCheckPosition(t, got.Position, "2")
			if got.Staff.State != EvidenceAbsent || len(got.Figures) != 1 || got.Figures[0].Node != 11 {
				t.Fatalf("invented staff or missing top-down figure identity: %+v", got)
			}
		})
	}
}
func TestFiguredBassOriginSkipsGraceChordAndEarlierNotes(t *testing.T) {
	s, service, b, block := figuredOriginFixture(t, OrderSource)
	for _, tc := range []struct {
		id   NodeID
		role timelineRole
	}{{50, timelineRoleNoteGrace}, {60, timelineRoleNoteChord}} {
		n := stateNode(b, timelineRoleNote, tc.id, stateAbsent[Atomic](), stateNode(b, tc.role, tc.id+1, stateAbsent[Atomic]()))
		block.content.children = append(block.content.children, n)
		service.Facts[tc.id] = timelineEventFact{Position: timelineAt(block, stateKnown(mustRational(t, 1, 1))), Part: block.part, Staff: stateAbsent[Integer]()}
	}
	if err := service.resolveFiguredBassOrigins(context.Background(), s, b, block); err != nil {
		t.Fatal(err)
	}
	if got := service.FiguredBassOrigins[10].Anchor; got.State != EvidenceKnown || got.Value.Node != 40 {
		t.Fatalf("grace/chord stole anchor: %+v", got)
	}
}
func TestFiguredBassOriginAmbiguityAndMissingFactsStayUnknown(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Session, *timelineService, *timelineBindings, *timelineMeasureBlock)
	}{
		{"simultaneous", func(_ *Session, v *timelineService, _ *timelineBindings, _ *timelineMeasureBlock) {
			f := v.Facts[30]
			f.Position = v.Facts[40].Position
			v.Facts[30] = f
		}},
		{"unplaced-note", func(_ *Session, v *timelineService, _ *timelineBindings, _ *timelineMeasureBlock) {
			f := v.Facts[30]
			f.Position = stateUnknown[musicalPosition]("unplaced-note")
			v.Facts[30] = f
		}},
		{"missing-note", func(_ *Session, v *timelineService, _ *timelineBindings, _ *timelineMeasureBlock) {
			delete(v.Facts, 30)
		}},
		{"incomplete-measure", func(_ *Session, _ *timelineService, _ *timelineBindings, b *timelineMeasureBlock) {
			b.complete = stateUnknown[bool]("missing-events")
		}},
		{"no-following-note", func(_ *Session, v *timelineService, _ *timelineBindings, _ *timelineMeasureBlock) {
			f := v.Facts[10]
			f.Position.Value.Offset = mustRational(t, 9, 1)
			v.Facts[10] = f
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, service, b, block := figuredOriginFixture(t, OrderSource)
			tc.change(s, service, b, &block)
			if err := service.resolveFiguredBassOrigins(context.Background(), s, b, block); err != nil {
				t.Fatal(err)
			}
			got := service.FiguredBassOrigins[10]
			if got.Anchor.State != EvidenceUnknown || got.Position.State != EvidenceUnknown {
				t.Fatalf("unproved association fabricated: %+v", got)
			}
		})
	}
}
func TestFiguredBassOriginScopeAndCancellation(t *testing.T) {
	s, service, b, block := figuredOriginFixture(t, OrderSource)
	// A nearer fact belonging to another measure/part cannot leak into the domain.
	f := service.Facts[20]
	f.Position.Value.Offset = mustRational(t, 1, 1)
	f.Position.Value.Measure.Node = 999
	service.Facts[20] = f
	f = service.Facts[30]
	f.Position.Value.Offset = mustRational(t, 1, 1)
	f.Position.Value.Part.Node = 999
	service.Facts[30] = f
	if err := service.resolveFiguredBassOrigins(context.Background(), s, b, block); err != nil {
		t.Fatal(err)
	}
	if got := service.FiguredBassOrigins[10].Anchor; got.State != EvidenceKnown || got.Value.Node != 40 {
		t.Fatalf("scope leak: %+v", got)
	}
	s2, v2, b2, block2 := figuredOriginFixture(t, OrderSource)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := v2.resolveFiguredBassOrigins(ctx, s2, b2, block2); !errors.Is(err, context.Canceled) || len(v2.FiguredBassOrigins) != 0 {
		t.Fatalf("canceled publication: %v", err)
	}
}

func TestFiguredBassOriginLaterUnknownMarkersDoNotMaskEarlierAnchor(t *testing.T) {
	for _, laterFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "later-last", true: "later-first"}[laterFirst], func(t *testing.T) {
			s, service, b, block := figuredOriginFixture(t, OrderSource)
			late := block.content.children[1]
			late.end.ChildrenComplete = stateUnknown[bool]("later-note-branches-unavailable")
			if !laterFirst {
				block.content.children[1], block.content.children[3] = block.content.children[3], block.content.children[1]
			}
			if err := service.resolveFiguredBassOrigins(context.Background(), s, b, block); err != nil {
				t.Fatal(err)
			}
			if got := service.FiguredBassOrigins[10].Anchor; got.State != EvidenceKnown || got.Value.Node != 40 {
				t.Fatalf("later unknown branch poisoned earlier anchor: %+v", got)
			}
		})
	}
}

func TestFiguredBassOriginBudgetFailureCannotPublish(t *testing.T) {
	_, service, b, block := figuredOriginFixture(t, OrderSource)
	s := rationalSession(t, Budgets{MaxRuleSteps: 5})
	defer s.cleanup()
	if err := service.resolveFiguredBassOrigins(context.Background(), s, b, block); !errors.Is(err, ErrResourceLimit) || len(service.FiguredBassOrigins) != 0 {
		t.Fatalf("interrupted anchor publication: %v", err)
	}
}
