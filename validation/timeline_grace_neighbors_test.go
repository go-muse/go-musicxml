package validation

import (
	"context"
	"errors"
	"testing"
)

func graceNeighborFixture(t *testing.T, order OrderKind, voice Evidence[string]) (*Session, *timelineService, *timelineBindings, []timelineMeasureBlock) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	t.Cleanup(s.cleanup)
	b := noteOriginBindings()
	p := runtimeNote(t, b, 10, 3, false)
	f := runtimeNote(t, b, 40, 6, false)
	nodes := []*factNode{p}
	for _, id := range []NodeID{20, 30} {
		g := stateNode(b, timelineRoleNoteGrace, id+1, stateAbsent[Atomic]())
		stateAttribute(b, g, timelineRoleGraceStealPrevious, stateAtomicDecimal(t, 25, 1).Value)
		stateAttribute(b, g, timelineRoleGraceStealFollowing, stateAtomicDecimal(t, 10, 1).Value)
		nodes = append(nodes, stateNode(b, timelineRoleNote, id, stateAbsent[Atomic](), g))
	}
	nodes = append(nodes, f)
	block := runtimeBlock(b, 0, nodes...)
	block.content.end.Order = stateKnown(order)
	service := &timelineService{Facts: map[NodeID]timelineEventFact{}}
	for _, n := range nodes {
		onset, end, duration := int64(1), int64(1), int64(0)
		if n.header.ID == 10 {
			onset, end, duration = 0, 1, 1
		}
		if n.header.ID == 40 {
			onset, end, duration = 1, 3, 2
		}
		fact := timelineEventFact{Position: timelineAt(block, stateKnown(mustRational(t, onset, 1))), End: timelineAt(block, stateKnown(mustRational(t, end, 1))), Duration: stateKnown(mustRational(t, duration, 1)), Part: block.part, Staff: stateKnown(stateInt(t, 1)), Voice: voice}
		if duration == 0 {
			fact.Duration = stateAbsent[Rational]()
			fact.End = stateUnknown[musicalPosition]("grace-no-score-duration")
			fact.Staff = stateKnown(stateInt(t, 2))
		}
		service.Facts[n.header.ID] = fact
		if err := service.readNoteOrigins(context.Background(), s, b, n, fact, stateKnown(stateDecimal(t, 3, 1))); err != nil {
			t.Fatal(err)
		}
	}
	return s, service, b, []timelineMeasureBlock{block}
}

func TestGraceNeighborsGroupsExactAndNoStaffFilter(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent, OrderModelFields} {
		for _, voice := range []Evidence[string]{stateKnown("melody"), stateAbsent[string]()} {
			s, v, b, blocks := graceNeighborFixture(t, order, voice)
			if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
				t.Fatal(err)
			}
			for _, id := range []NodeID{20, 30} {
				got := v.GraceNeighbors[id]
				if got.Previous.Note.State != EvidenceKnown || got.Previous.Note.Value.Node != 10 || got.Following.Note.State != EvidenceKnown || got.Following.Note.Value.Node != 40 {
					t.Fatalf("group neighbor: %+v", got)
				}
				if got.Previous.Fraction.Value.String() != "1/4" || got.Previous.StolenDuration.Value.String() != "1/4" || got.Following.StolenDuration.Value.String() != "1/5" {
					t.Fatalf("individual fractions: %+v", got)
				}
				if v.NoteOrigins[id].Duration.State != EvidenceAbsent {
					t.Fatal("grace duration fabricated")
				}
				if v.NoteOrigins[id].GraceTiming.Value.PreviousNote.State != EvidenceUnknown {
					t.Fatal("published origin mutated")
				}
			}
		}
	}
}

func TestGraceNeighborsAmbiguityRetainsCausalEvidence(t *testing.T) {
	for _, mode := range []string{"mixed-voice", "unknown-voice", "overlap", "chord-donor", "incomplete", "missing-fact"} {
		t.Run(mode, func(t *testing.T) {
			s, v, b, blocks := graceNeighborFixture(t, OrderSource, stateKnown("a"))
			cause := &DecisionRef{Rule: RuleKey{"voice", 1}, Subject: NodeRef{"d", 10}}
			n := v.NoteOrigins[10]
			switch mode {
			case "mixed-voice":
				n.Voice = stateAbsent[string]()
			case "unknown-voice":
				n.Voice = Evidence[string]{State: EvidenceInvalid, Reason: "invalid-voice", Cause: cause}
			case "overlap":
				n.ScoreEnd = timelineAt(blocks[0], stateKnown(mustRational(t, 2, 1)))
			case "chord-donor":
				n.Chord = stateKnown(true)
			case "incomplete":
				blocks[0].complete = stateUnknown[bool]("incomplete")
			case "missing-fact":
				delete(v.NoteOrigins, 10)
			}
			if mode != "missing-fact" {
				v.NoteOrigins[10] = n
			}
			if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
				t.Fatal(err)
			}
			got := v.GraceNeighbors[20].Previous
			if got.Note.State == EvidenceKnown || got.StolenDuration.State == EvidenceKnown {
				t.Fatalf("invented donor: %+v", got)
			}
			if mode == "unknown-voice" && (got.Note.State != EvidenceInvalid || got.Note.Cause == nil || *got.Note.Cause != *cause || got.Note.Cause == cause) {
				t.Fatalf("causal ownership: %+v", got)
			}
		})
	}
}

func TestGraceNeighborsCrossMeasureProvenPrevious(t *testing.T) {
	s, v, b, blocks := graceNeighborFixture(t, OrderSource, stateKnown("a"))
	first := blocks[0]
	second := runtimeBlock(b, 1, first.content.children[1:]...)
	first.content.children = first.content.children[:1]
	for _, id := range []NodeID{20, 30, 40} {
		n := v.NoteOrigins[id]
		n.ScoreOnset = timelineAt(second, stateKnown(mustRational(t, 0, 1)))
		if id == 40 {
			n.ScoreEnd = timelineAt(second, stateKnown(mustRational(t, 2, 1)))
		}
		v.NoteOrigins[id] = n
	}
	if err := v.resolveGraceNeighbors(context.Background(), s, b, []timelineMeasureBlock{first, second}); err != nil {
		t.Fatal(err)
	}
	got := v.GraceNeighbors[20]
	if got.Previous.Note.State != EvidenceKnown || got.Previous.Note.Value.Node != 10 || got.Following.Note.State != EvidenceKnown || got.Following.Note.Value.Node != 40 {
		t.Fatalf("barline donor: %+v", got)
	}
}

func TestGraceNeighborsOmissionAndBudgetCancellation(t *testing.T) {
	s, v, b, blocks := graceNeighborFixture(t, OrderSource, stateKnown("a"))
	n := v.NoteOrigins[20]
	n.GraceTiming.Value.StealPrevious = stateAbsent[Decimal]()
	v.NoteOrigins[20] = n
	if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
		t.Fatal(err)
	}
	got := v.GraceNeighbors[20].Previous
	if got.Raw.State != EvidenceAbsent || got.StolenDuration.State != EvidenceAbsent || got.Fraction.State != EvidenceAbsent {
		t.Fatalf("invented percentage default: %+v", got)
	}
	for _, cancelled := range []bool{false, true} {
		_, v, b, blocks := graceNeighborFixture(t, OrderSource, stateKnown("a"))
		s := rationalSession(t, Budgets{MaxRuleSteps: 5})
		defer s.cleanup()
		ctx, cancel := context.WithCancel(context.Background())
		if cancelled {
			cancel()
		} else {
			defer cancel()
		}
		err := v.resolveGraceNeighbors(ctx, s, b, blocks)
		want := ErrResourceLimit
		if cancelled {
			want = context.Canceled
		}
		if !errors.Is(err, want) || len(v.GraceNeighbors) != 0 {
			t.Fatalf("partial publication %v %+v", err, v.GraceNeighbors)
		}
	}
}

func TestGraceNeighborsSeparateExplicitVoicesAndCrossMeasureFollowing(t *testing.T) {
	s, v, b, blocks := graceNeighborFixture(t, OrderSource, stateKnown("a"))
	other := runtimeNote(t, b, 50, 6, false)
	blocks[0].content.children = append(blocks[0].content.children, other)
	n := v.NoteOrigins[40]
	n.Note.Node = 50
	n.Voice = stateKnown("b")
	v.NoteOrigins[50] = n
	if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
		t.Fatal(err)
	}
	if got := v.GraceNeighbors[20].Following.Note; got.State != EvidenceKnown || got.Value.Node != 40 {
		t.Fatalf("another voice interfered: %+v", got)
	}
	s, v, b, blocks = graceNeighborFixture(t, OrderSource, stateKnown("a"))
	second := runtimeBlock(b, 1, blocks[0].content.children[3])
	blocks[0].content.children = blocks[0].content.children[:3]
	n = v.NoteOrigins[40]
	n.ScoreOnset = timelineAt(second, stateKnown(timelineZero))
	n.ScoreEnd = timelineAt(second, stateKnown(mustRational(t, 2, 1)))
	v.NoteOrigins[40] = n
	if err := v.resolveGraceNeighbors(context.Background(), s, b, append(blocks, second)); err != nil {
		t.Fatal(err)
	}
	if got := v.GraceNeighbors[20].Following.Note; got.State != EvidenceKnown || got.Value.Node != 40 {
		t.Fatalf("following barline donor: %+v", got)
	}
}

func TestGraceNeighborsLongGroupDefaultBudgetsAndAtomicFailure(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		s, v, b, blocks := graceNeighborFixture(t, order, stateKnown("a"))
		prototype := v.NoteOrigins[20]
		nodes := []*factNode{blocks[0].content.children[0]}
		delete(v.NoteOrigins, 20)
		delete(v.NoteOrigins, 30)
		for i := 0; i < 1000; i++ {
			id := NodeID(1000 + i*10)
			n := prototype
			n.Note.Node = id
			n.GraceTiming.Value.Grace.Node = id + 1
			v.NoteOrigins[id] = n
			nodes = append(nodes, stateNode(b, timelineRoleNote, id, stateAbsent[Atomic]()))
		}
		nodes = append(nodes, blocks[0].content.children[3])
		blocks[0].content.children = nodes
		if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
			t.Fatal(err)
		}
		if len(v.GraceNeighbors) != 1000 {
			t.Fatalf("records: %d", len(v.GraceNeighbors))
		}
		t.Logf("%s steps=%d bytes=%d", order, rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetBufferedBytes))
		s.cleanup()
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Fatalf("retained after cleanup: %d", got)
		}
	}
	for _, at := range []int{10, 50, 100, 250} {
		_, v, b, blocks := graceNeighborFixture(t, OrderSource, stateKnown("a"))
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		if err := v.resolveGraceNeighbors(ctx, s, b, blocks); !errors.Is(err, context.Canceled) || len(v.GraceNeighbors) != 0 {
			t.Fatalf("partial publication at %d: %v", at, err)
		}
	}
	_, v, b, blocks := graceNeighborFixture(t, OrderSource, stateKnown("a"))
	s := rationalSession(t, Budgets{MaxContextFacts: 1})
	defer s.cleanup()
	if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); !errors.Is(err, ErrResourceLimit) || len(v.GraceNeighbors) != 0 {
		t.Fatalf("context quota published subset: %v", err)
	}
}

func TestGraceNeighborsScratchReleasedAndOrdinaryFastPath(t *testing.T) {
	s, v, b, blocks := graceNeighborFixture(t, OrderSource, stateKnown("a"))
	for _, id := range []NodeID{20, 30} {
		n := v.NoteOrigins[id]
		n.GraceTiming.Value.StealPrevious = stateAbsent[Decimal]()
		n.GraceTiming.Value.StealFollowing = stateAbsent[Decimal]()
		v.NoteOrigins[id] = n
	}
	before := rationalUsage(t, s, BudgetBufferedBytes)
	if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
		t.Fatal(err)
	}
	if got := rationalUsage(t, s, BudgetBufferedBytes) - before; got != 8192 {
		t.Fatalf("scratch retained: delta %d, want two 4096-byte records", got)
	}
	s, v, b, blocks = graceNeighborFixture(t, OrderSource, stateKnown("a"))
	for _, id := range []NodeID{20, 30} {
		n := v.NoteOrigins[id]
		n.Grace = stateKnown(false)
		v.NoteOrigins[id] = n
	}
	before = rationalUsage(t, s, BudgetBufferedBytes)
	if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
		t.Fatal(err)
	}
	if got := rationalUsage(t, s, BudgetBufferedBytes); got != before {
		t.Fatalf("ordinary score allocated %d bytes", got-before)
	}
}

func TestGraceNeighborIdentityDoesNotClaimContinuousTime(t *testing.T) {
	s, v, b, blocks := graceNeighborFixture(t, OrderSource, stateKnown("a"))
	// A forward gap separates the ordinary note from the following grace run.
	// The previous note identity is still known; this is not performed scheduling.
	for _, id := range []NodeID{20, 30, 40} {
		n := v.NoteOrigins[id]
		n.ScoreOnset = timelineAt(blocks[0], stateKnown(mustRational(t, 3, 1)))
		if id == 40 {
			n.ScoreEnd = timelineAt(blocks[0], stateKnown(mustRational(t, 5, 1)))
		}
		v.NoteOrigins[id] = n
	}
	forward := stateNode(b, timelineRoleForward, 60, stateAbsent[Atomic]())
	blocks[0].content.children = append(blocks[0].content.children[:1], append([]*factNode{forward}, blocks[0].content.children[1:]...)...)
	if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
		t.Fatal(err)
	}
	if got := v.GraceNeighbors[20].Previous; got.Note.State != EvidenceKnown || got.Note.Value.Node != 10 || got.StolenDuration.Value.String() != "1/4" {
		t.Fatalf("chronological donor lost across forward gap: %+v", got)
	}
}

func TestGraceNeighborsForeignPositionIsGapNotProviderContract(t *testing.T) {
	s, v, b, blocks := graceNeighborFixture(t, OrderSource, stateKnown("a"))
	n := v.NoteOrigins[10]
	n.ScoreOnset.Value.Part.Node = 999
	v.NoteOrigins[10] = n
	if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
		t.Fatalf("assessed foreign position escaped as provider contract: %v", err)
	}
	if got := v.GraceNeighbors[20].Previous.Note; got.State != EvidenceUnknown {
		t.Fatalf("foreign donor: %+v", got)
	}
}
