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
	block.content.end.Order = testEvidence(stateKnown(order))
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
			late.end.ChildrenComplete = testEvidence(stateUnknown[bool]("later-note-branches-unavailable"))
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

func TestFiguredBassDurationUsesFinalizedAnchorUnits(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		for _, mode := range []string{"equal", "different", "missing-cursor", "invalid-anchor", "absent-duration"} {
			t.Run(string(order)+"/"+mode, func(t *testing.T) {
				s, service, b, block := figuredOriginFixture(t, order)
				figure := block.content.children[0]
				if mode != "absent-duration" {
					figure.children = append(figure.children, stateNode(b, timelineRoleFiguredBassDuration, 12, stateAtomicDecimal(t, 1, 1)))
				}
				f := service.Facts[10]
				f.Units = stateKnown(stateDecimal(t, 3, 1))
				service.Facts[10] = f
				f = service.Facts[40]
				f.Units = stateKnown(stateDecimal(t, 3, 1))
				cause := &DecisionRef{Rule: RuleKey{"units", 1}, Subject: NodeRef{"d", 40}}
				if mode == "different" {
					f.Units = stateKnown(stateDecimal(t, 4, 1))
				}
				if mode == "invalid-anchor" {
					f.Units = Evidence[Decimal]{State: EvidenceInvalid, Reason: "bad-units", Cause: cause}
				}
				service.Facts[40] = f
				if mode == "missing-cursor" {
					f = service.Facts[10]
					f.Units = stateAbsent[Decimal]()
					service.Facts[10] = f
				}
				stateClose(t, s, &service.State, 0)
				if mode != "missing-cursor" {
					statePublish(t, s, &service.State.Divisions, service.Facts[10].Position.Value, stateScope{AllStaves: true}, service.Facts[10].Units, 100)
				}
				statePublish(t, s, &service.State.Divisions, service.Facts[40].Position.Value, stateScope{AllStaves: true}, service.Facts[40].Units, 101)
				if err := service.resolveFiguredBassOrigins(context.Background(), s, b, block); err != nil {
					t.Fatal(err)
				}
				got := service.FiguredBassOrigins[10]
				switch mode {
				case "equal":
					if got.DurationUnits.State != EvidenceKnown || got.Duration.Value.String() != "1/3" {
						t.Fatalf("duration = %+v", got)
					}
					noteOriginCheckPosition(t, got.End, "7/3")
				case "different":
					if got.CursorUnits.Value.String() != "3" || got.AnchorUnits.Value.String() != "4" || got.DurationUnits.Value.String() != "4" || got.Duration.Value.String() != "1/4" {
						t.Fatalf("duration did not use actual anchor position: %+v", got)
					}
					noteOriginCheckPosition(t, got.End, "9/4")
				case "missing-cursor":
					if got.CursorUnits.State != EvidenceAbsent || got.DurationUnits.State != EvidenceKnown || got.Duration.Value.String() != "1/3" {
						t.Fatalf("missing lexical cursor units blocked known anchor: %+v", got)
					}
				case "absent-duration":
					if got.Duration.State != EvidenceAbsent || got.End.State != EvidenceAbsent {
						t.Fatalf("invented omitted duration: %+v", got)
					}
				case "invalid-anchor":
					if got.Duration.State != EvidenceInvalid || got.Duration.Cause == nil || *got.Duration.Cause != *cause || got.Duration.Cause == cause {
						t.Fatalf("cause ownership: %+v", got)
					}
				default:
					if got.Duration.State != EvidenceUnknown || got.End.State != EvidenceUnknown {
						t.Fatalf("selected disputed units: %+v", got)
					}
				}
			})
		}
	}
}

func TestFiguredBassFinalDivisionsOverrideEventRegister(t *testing.T) {
	s, v, b, block := figuredOriginFixture(t, OrderSource)
	block.content.children[0].children = append(block.content.children[0].children, stateNode(b, timelineRoleFiguredBassDuration, 12, stateAtomicDecimal(t, 1, 1)))
	for _, id := range []NodeID{10, 40} {
		f := v.Facts[id]
		f.Units = stateKnown(stateDecimal(t, 3, 1))
		v.Facts[id] = f
	}
	stateClose(t, s, &v.State, 0)
	statePublish(t, s, &v.State.Divisions, timelineAt(block, stateKnown(timelineZero)).Value, stateScope{AllStaves: true}, stateKnown(stateDecimal(t, 3, 1)), 100)
	statePublish(t, s, &v.State.Divisions, v.Facts[10].Position.Value, stateScope{AllStaves: true}, stateKnown(stateDecimal(t, 4, 1)), 101)
	if err := v.resolveFiguredBassOriginsAll(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
		t.Fatal(err)
	}
	got := v.FiguredBassOrigins[10]
	if got.CursorUnits.Value.String() != "4" || got.AnchorUnits.Value.String() != "4" || got.Duration.Value.String() != "1/4" {
		t.Fatalf("stale lexical units: %+v", got)
	}
	noteOriginCheckPosition(t, got.End, "9/4")
}

func TestFiguredBassFollowingNoteCanCrossCompleteMeasures(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		s, v, b, first := figuredOriginFixture(t, order)
		second := runtimeBlock(b, 1, first.content.children[3])
		second.content.end.Order = testEvidence(stateKnown(order))
		first.content.children = first.content.children[:1]
		first.content.children[0].children = append(first.content.children[0].children, stateNode(b, timelineRoleFiguredBassDuration, 12, stateAtomicDecimal(t, 1, 1)))
		f := v.Facts[40]
		f.Position = timelineAt(second, stateKnown(timelineZero))
		v.Facts[40] = f
		stateClose(t, s, &v.State, 1)
		statePublish(t, s, &v.State.Divisions, timelineAt(first, stateKnown(timelineZero)).Value, stateScope{AllStaves: true}, stateKnown(stateDecimal(t, 3, 1)), 100)
		if err := v.resolveFiguredBassOriginsAll(context.Background(), s, b, []timelineMeasureBlock{first, second}); err != nil {
			t.Fatal(err)
		}
		got := v.FiguredBassOrigins[10]
		if got.Anchor.State != EvidenceKnown || got.Anchor.Value.Node != 40 || got.Position.Value.Measure.Node != second.measure.header.ID || got.End.Value.Measure.Node != second.measure.header.ID {
			t.Fatalf("barline anchor: %+v", got)
		}
		noteOriginCheckPosition(t, got.End, "1/3")
	}
}

func figuredChainFixture(t *testing.T, order OrderKind) (*Session, *timelineService, *timelineBindings, timelineMeasureBlock) {
	t.Helper()
	s, v, b, block := figuredOriginFixture(t, order)
	first := block.content.children[0]
	first.children = append(first.children, stateNode(b, timelineRoleFiguredBassDuration, 12, stateAtomicDecimal(t, 2, 1)))
	nodes := []*factNode{first}
	for _, id := range []NodeID{60, 70} {
		nodes = append(nodes, stateNode(b, timelineRoleFiguredBass, id, stateAbsent[Atomic](), stateNode(b, timelineRoleFiguredBassFigure, id+1, stateAbsent[Atomic]()), stateNode(b, timelineRoleFiguredBassDuration, id+2, stateAtomicDecimal(t, 1, 1))))
		v.Facts[id] = v.Facts[10]
	}
	block.content.children = append(nodes, block.content.children[1:]...)
	stateClose(t, s, &v.State, 0)
	statePublish(t, s, &v.State.Divisions, timelineAt(block, stateKnown(timelineZero)).Value, stateScope{AllStaves: true}, stateKnown(stateDecimal(t, 3, 1)), 100)
	return s, v, b, block
}

func TestFiguredBassConsecutiveChainKeepsIndependentAnchorAndDuration(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		s, v, b, block := figuredChainFixture(t, order)
		if err := v.resolveFiguredBassOriginsAll(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
			t.Fatal(err)
		}
		for i, id := range []NodeID{10, 60, 70} {
			got := v.FiguredBassOrigins[id]
			noteOriginCheckPosition(t, got.AnchorPosition, "2")
			noteOriginCheckPosition(t, got.ChainPosition, []string{"2", "8/3", "3"}[i])
			noteOriginCheckPosition(t, got.ChainEnd, []string{"8/3", "3", "10/3"}[i])
			if got.GroupStart.State != EvidenceKnown || got.GroupStart.Value.Node != 10 || got.ChainInterpretation != "consecutive-figured-bass-duration-chain" {
				t.Fatalf("chain provenance: %+v", got)
			}
		}
		noteOriginCheckPosition(t, v.FiguredBassOrigins[60].End, "7/3")
	}
}

func TestFiguredBassChainAmbiguityAndAbsentDurationAreIndependent(t *testing.T) {
	for _, mode := range []string{"interleaved", "absent-first-duration"} {
		s, v, b, block := figuredChainFixture(t, OrderSource)
		if mode == "interleaved" {
			nodes := block.content.children
			block.content.children = append([]*factNode{nodes[0], stateNode(b, timelineRoleDirection, 80, stateAbsent[Atomic]())}, nodes[1:]...)
		} else {
			block.content.children[0].children = block.content.children[0].children[:1]
		}
		if err := v.resolveFiguredBassOriginsAll(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
			t.Fatal(err)
		}
		got := v.FiguredBassOrigins[60]
		if got.Anchor.State != EvidenceKnown || got.Duration.State != EvidenceKnown || got.ChainPosition.State != EvidenceUnknown || got.ChainEnd.State != EvidenceUnknown {
			t.Fatalf("chain ambiguity hid independent facts: %+v", got)
		}
	}
}

func TestFiguredBassAllTransactionalScratchAndLongChain(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		s, v, b, block := figuredChainFixture(t, order)
		base := block.content.children[0]
		nodes := make([]*factNode, 0, 1001)
		for i := 0; i < 1000; i++ {
			id := NodeID(1000 + i*10)
			n := stateNode(b, timelineRoleFiguredBass, id, stateAbsent[Atomic](), stateNode(b, timelineRoleFiguredBassFigure, id+1, stateAbsent[Atomic]()), stateNode(b, timelineRoleFiguredBassDuration, id+2, stateAtomicDecimal(t, 1, 1)))
			nodes = append(nodes, n)
			v.Facts[id] = v.Facts[base.header.ID]
		}
		nodes = append(nodes, block.content.children[len(block.content.children)-1])
		block.content.children = nodes
		if err := v.resolveFiguredBassOriginsAll(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
			t.Fatal(err)
		}
		if len(v.FiguredBassOrigins) != 1000 {
			t.Fatal(len(v.FiguredBassOrigins))
		}
		noteOriginCheckPosition(t, v.FiguredBassOrigins[10990].ChainEnd, "1006/3")
		t.Logf("%s steps=%d bytes=%d", order, rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetBufferedBytes))
		s.cleanup()
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Fatalf("cleanup retained %d", got)
		}
	}
	for _, at := range []int{10, 100, 250, 500} {
		_, v, b, block := figuredChainFixture(t, OrderSource)
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		if err := v.resolveFiguredBassOriginsAll(ctx, s, b, []timelineMeasureBlock{block}); !errors.Is(err, context.Canceled) || len(v.FiguredBassOrigins) != 0 {
			t.Fatalf("partial finalization at %d: %v", at, err)
		}
	}
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	b := noteOriginBindings()
	v := &timelineService{}
	block := runtimeBlock(b, 0, runtimeNote(t, b, 10, 1, false))
	before := rationalUsage(t, s, BudgetBufferedBytes)
	if err := v.resolveFiguredBassOriginsAll(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
		t.Fatal(err)
	}
	if got := rationalUsage(t, s, BudgetBufferedBytes); got != before {
		t.Fatalf("ordinary score allocated %d", got-before)
	}
}

func TestFiguredBassThousandDistinctOriginsDefaultBudget(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		s := rationalSession(t, Budgets{})
		s.documentID = "d"
		defer s.cleanup()
		b := noteOriginBindings()
		v := &timelineService{Facts: map[NodeID]timelineEventFact{}}
		block := runtimeBlock(b, 0)
		block.content.end.Order = testEvidence(stateKnown(order))
		for i := 0; i < 1000; i++ {
			id := NodeID(1000 + i*10)
			figure := stateNode(b, timelineRoleFiguredBass, id, stateAbsent[Atomic](), stateNode(b, timelineRoleFiguredBassFigure, id+1, stateAbsent[Atomic]()), stateNode(b, timelineRoleFiguredBassDuration, id+2, stateAtomicDecimal(t, 1, 1)))
			note := runtimeNote(t, b, id+5, 3, false)
			block.content.children = append(block.content.children, figure, note)
			for _, node := range []*factNode{figure, note} {
				v.Facts[node.header.ID] = timelineEventFact{Position: timelineAt(block, stateKnown(mustRational(t, int64(i), 1))), Part: block.part, Staff: stateAbsent[Integer](), Voice: stateAbsent[string]()}
			}
		}
		stateClose(t, s, &v.State, 0)
		statePublish(t, s, &v.State.Divisions, timelineAt(block, stateKnown(timelineZero)).Value, stateScope{AllStaves: true}, stateKnown(stateDecimal(t, 3, 1)), 100)
		if err := v.resolveFiguredBassOriginsAll(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
			t.Fatal(err)
		}
		if len(v.FiguredBassOrigins) != 1000 {
			t.Fatal(len(v.FiguredBassOrigins))
		}
		t.Logf("%s steps=%d bytes=%d", order, rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetBufferedBytes))
	}
}

func TestFiguredBassCacheCannotAliasForeignDocumentCoordinates(t *testing.T) {
	s, v, b, block := figuredChainFixture(t, OrderSource)
	f := v.Facts[60]
	f.Position.Value.Part.DocumentID = "foreign"
	f.Position.Value.Measure.DocumentID = "foreign"
	v.Facts[60] = f
	if err := v.resolveFiguredBassOriginsAll(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
		t.Fatal(err)
	}
	if got := v.FiguredBassOrigins[60]; got.Anchor.State == EvidenceKnown || got.DurationUnits.State == EvidenceKnown {
		t.Fatalf("cross-document cache alias: %+v", got)
	}
}

func TestFiguredBassChainKeepsAnchorBasedDurationsAcrossLaterDivisions(t *testing.T) {
	s, v, b, block := figuredChainFixture(t, OrderSource)
	// First figure starts at 2 and lasts 2/3. A divisions change at its end does
	// not rewrite the common anchor-based duration of the second figure.
	at := timelineAt(block, stateKnown(mustRational(t, 8, 3))).Value
	statePublish(t, s, &v.State.Divisions, at, stateScope{AllStaves: true}, stateKnown(stateDecimal(t, 4, 1)), 110)
	if err := v.resolveFiguredBassOriginsAll(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
		t.Fatal(err)
	}
	got := v.FiguredBassOrigins[60]
	if got.AnchorUnits.Value.String() != "3" || got.Duration.Value.String() != "1/3" || got.ChainEnd.State != EvidenceKnown {
		t.Fatalf("chain rewrote anchored duration units: %+v", got)
	}
	noteOriginCheckPosition(t, got.ChainPosition, "8/3")
	noteOriginCheckPosition(t, got.End, "7/3")
	noteOriginCheckPosition(t, got.ChainEnd, "3")
	noteOriginCheckPosition(t, v.FiguredBassOrigins[70].ChainPosition, "3")
}

func TestFiguredBassUnavailableAnchorCauseAndRawAbsence(t *testing.T) {
	for _, present := range []bool{false, true} {
		s, v, b, block := figuredOriginFixture(t, OrderSource)
		cause := &DecisionRef{Rule: RuleKey{"measure", 1}, Subject: NodeRef{"d", block.measure.header.ID}}
		block.complete = Evidence[bool]{State: EvidenceInvalid, Reason: "invalid-measure-domain", Cause: cause}
		if present {
			block.content.children[0].children = append(block.content.children[0].children, stateNode(b, timelineRoleFiguredBassDuration, 12, stateAtomicDecimal(t, 1, 1)))
		}
		if err := v.resolveFiguredBassOriginsAll(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
			t.Fatal(err)
		}
		got := v.FiguredBassOrigins[10]
		if got.Anchor.State != EvidenceInvalid || got.Anchor.Cause == nil || *got.Anchor.Cause != *cause || got.Anchor.Cause == cause {
			t.Fatalf("anchor cause not owned: %+v", got)
		}
		if present {
			if got.Duration.State != EvidenceInvalid || got.Duration.Cause == nil || *got.Duration.Cause != *cause || got.Duration.Cause == cause {
				t.Fatalf("duration anchor cause lost: %+v", got)
			}
		} else if got.Duration.State != EvidenceAbsent || got.End.State != EvidenceAbsent {
			t.Fatalf("missing duration fabricated from bad anchor: %+v", got)
		}
	}
}
