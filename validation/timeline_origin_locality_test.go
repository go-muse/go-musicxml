package validation

import (
	"context"
	"testing"
)

func TestFiguredBassLaterAnchorGapPreservesEarlierChain(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		for _, before := range []bool{false, true} {
			s, v, b, block := figuredOriginFixture(t, order)
			first := block.content.children[0]
			first.children = append(first.children, stateNode(b, timelineRoleFiguredBassDuration, 12, stateAtomicDecimal(t, 1, 1)))
			trailing := stateNode(b, timelineRoleFiguredBass, 80, stateAbsent[Atomic](), stateNode(b, timelineRoleFiguredBassFigure, 81, stateAbsent[Atomic]()))
			v.Facts[80] = timelineEventFact{Position: timelineAt(block, stateKnown(mustRational(t, 4, 1))), Part: block.part, Staff: stateAbsent[Integer](), Voice: stateAbsent[string]()}
			if before {
				block.content.children = append([]*factNode{trailing, stateNode(b, timelineRoleBackup, 90, stateAbsent[Atomic]())}, block.content.children...)
			} else {
				block.content.children = append(block.content.children, stateNode(b, timelineRoleForward, 90, stateAbsent[Atomic]()), trailing)
			}
			stateClose(t, s, &v.State, 0)
			statePublish(t, s, &v.State.Divisions, timelineAt(block, stateKnown(timelineZero)).Value, stateScope{AllStaves: true}, stateKnown(stateDecimal(t, 3, 1)), 100)
			if err := v.resolveFiguredBassOriginsAll(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
				t.Fatal(err)
			}
			early, late := v.FiguredBassOrigins[10], v.FiguredBassOrigins[80]
			if early.Anchor.State != EvidenceKnown || late.Anchor.State != EvidenceUnknown {
				t.Fatalf("invalid fixture: %+v %+v", early, late)
			}
			noteOriginCheckPosition(t, early.ChainPosition, "2")
			noteOriginCheckPosition(t, early.ChainEnd, "7/3")
			if early.GroupStart.State != EvidenceKnown {
				t.Fatalf("later gap erased earlier group: %+v", early)
			}
		}
	}
}

func TestFiguredBassChainGapUsesExactScoreLowerBound(t *testing.T) {
	for _, mode := range []string{"earlier", "equal", "later", "later-measure", "unknown-position", "unknown-owner", "foreign-part"} {
		for _, before := range []bool{false, true} {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			s.documentID = "d"
			b := noteOriginBindings()
			first := stateNode(b, timelineRoleFiguredBass, 10, stateAbsent[Atomic]())
			bad := stateNode(b, timelineRoleFiguredBass, 20, stateAbsent[Atomic]())
			block := runtimeBlock(b, 0, first, bad)
			if before {
				block.content.children[0], block.content.children[1] = bad, first
			}
			at := timelineAt(block, stateKnown(mustRational(t, 2, 1)))
			cause := &DecisionRef{Rule: RuleKey{"anchor", 1}, Subject: NodeRef{"d", 20}}
			pending := map[NodeID]figuredBassTimingOrigin{10: {FiguredBass: NodeRef{"d", 10}, Part: block.part, Anchor: stateKnown(NodeRef{"d", 30}), AnchorPosition: at, Position: at, End: timelineAt(block, stateKnown(mustRational(t, 3, 1))), Duration: stateKnown(mustRational(t, 1, 1)), DurationRaw: stateKnown(stateDecimal(t, 1, 1))}}
			gap := figuredBassTimingOrigin{FiguredBass: NodeRef{"d", 20}, Part: block.part, Anchor: Evidence[NodeRef]{State: EvidenceInvalid, Reason: "unavailable-anchor", Cause: cause}, CursorOrigin: at}
			switch mode {
			case "earlier":
				gap.CursorOrigin.Value.Offset = mustRational(t, 1, 1)
			case "later":
				gap.CursorOrigin.Value.Offset = mustRational(t, 3, 1)
			case "later-measure":
				gap.CursorOrigin.Value.MeasureOrdinal++
				gap.CursorOrigin.Value.Measure.Node++
			case "unknown-position":
				gap.CursorOrigin = stateUnknown[musicalPosition]("unplaced")
			case "unknown-owner":
				gap.Part = stateUnknown[NodeRef]("owner")
			case "foreign-part":
				gap.Part = stateKnown(NodeRef{"d", 2})
				gap.CursorOrigin.Value.Part = gap.Part.Value
			}
			pending[20] = gap
			r := &stateReader{ctx: context.Background(), s: s, b: b}
			if err := resolveFiguredBassChains(r, []figuredBassDomain{{block: block, complete: stateKnown(true)}}, pending, []NodeID{10, 20}); err != nil {
				t.Fatal(err)
			}
			got := pending[10]
			unrelated := mode == "later" || mode == "later-measure" || mode == "foreign-part"
			if unrelated {
				if got.GroupStart.State != EvidenceKnown || got.ChainPosition.State != EvidenceKnown || got.ChainEnd.State != EvidenceKnown {
					t.Fatalf("%s before=%v erased known chain: %+v", mode, before, got)
				}
			} else if got.GroupStart.State == EvidenceKnown || got.ChainPosition.State == EvidenceKnown {
				t.Fatalf("%s before=%v ignored possible dependency: %+v", mode, before, got)
			}
			if !unrelated && mode != "unknown-owner" && (got.GroupStart.State != EvidenceInvalid || got.GroupStart.Cause != cause) {
				t.Fatalf("required gap cause lost: %+v", got)
			}
		}
	}
}

func TestGraceLaterUnavailableEvidencePreservesBoundedDonors(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		for _, mode := range []string{"later-voice", "earlier-voice", "same-voice-position", "between-voice", "later-absent-voice", "same-absent-voice", "later-kind", "later-position-unknown", "same-position-unknown", "later-overlap"} {
			for _, before := range []bool{false, true} {
				t.Run(string(order)+"/"+mode+map[bool]string{false: "/last", true: "/first"}[before], func(t *testing.T) {
					s, v, b, blocks := graceNeighborFixture(t, order, stateKnown("a"))
					block := blocks[0]
					extra := runtimeNote(t, b, 60, 3, false)
					n := v.NoteOrigins[40]
					n.Note.Node = 60
					n.ScoreOnset = timelineAt(block, stateKnown(mustRational(t, 5, 1)))
					n.ScoreEnd = timelineAt(block, stateKnown(mustRational(t, 6, 1)))
					cause := &DecisionRef{Rule: RuleKey{"later", 1}, Subject: NodeRef{"d", 60}}
					unavailable := Evidence[string]{State: EvidenceInvalid, Reason: "unavailable-voice", Cause: cause}
					n.Voice = unavailable
					switch mode {
					case "earlier-voice":
						n.ScoreOnset.Value.Offset = mustRational(t, -1, 1)
					case "same-voice-position":
						n.ScoreOnset.Value.Offset = mustRational(t, 1, 1)
					case "between-voice":
						n.ScoreOnset.Value.Offset = mustRational(t, 1, 2)
					case "later-absent-voice":
						n.Voice = stateAbsent[string]()
					case "same-absent-voice":
						n.Voice = stateAbsent[string]()
						n.ScoreOnset.Value.Offset = mustRational(t, 1, 1)
					case "later-kind":
						n.Voice = stateKnown("a")
						n.Grace = Evidence[bool]{State: EvidenceInvalid, Reason: "unavailable-kind", Cause: cause}
					case "later-position-unknown":
						later := runtimeBlock(b, 1, extra)
						n.Voice = stateKnown("a")
						n.ScoreOnset = Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "unplaced-later-note", Cause: cause}
						blocks = append(blocks, later)
					case "same-position-unknown":
						n.Voice = stateKnown("a")
						n.ScoreOnset = Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "unplaced-note", Cause: cause}
					case "later-overlap":
						n.Voice = stateKnown("a")
						n.ScoreEnd.Value.Offset = mustRational(t, 8, 1)
						other := n
						other.Note.Node = 70
						other.ScoreOnset.Value.Offset = mustRational(t, 6, 1)
						other.ScoreEnd.Value.Offset = mustRational(t, 7, 1)
						v.NoteOrigins[70] = other
						block.content.children = append(block.content.children, runtimeNote(t, b, 70, 3, false))
					}
					v.NoteOrigins[60] = n
					if mode != "later-position-unknown" {
						if before {
							block.content.children = append([]*factNode{extra, stateNode(b, timelineRoleBackup, 80, stateAbsent[Atomic]())}, block.content.children...)
						} else {
							block.content.children = append(block.content.children, stateNode(b, timelineRoleForward, 80, stateAbsent[Atomic]()), extra)
						}
					}
					if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
						t.Fatal(err)
					}
					got := v.GraceNeighbors[20]
					prevGap := mode == "same-voice-position" || mode == "same-absent-voice" || mode == "same-position-unknown" || mode == "between-voice"
					nextGap := mode == "same-voice-position" || mode == "same-absent-voice" || mode == "same-position-unknown"
					for _, side := range []struct {
						value  graceDonorTiming
						gap    bool
						id     NodeID
						amount string
					}{{got.Previous, prevGap, 10, "1/4"}, {got.Following, nextGap, 40, "1/5"}} {
						if side.gap {
							if side.value.Note.State == EvidenceKnown {
								t.Fatalf("ignored relevant candidate: %+v", got)
							}
						} else if side.value.Note.State != EvidenceKnown || side.value.Note.Value.Node != side.id || side.value.StolenDuration.Value.String() != side.amount {
							t.Fatalf("unrelated gap erased bounded donor: %+v", got)
						}
						if side.gap && mode != "same-absent-voice" && (side.value.Note.State != EvidenceInvalid || side.value.Note.Cause == nil || *side.value.Note.Cause != *cause || side.value.Note.Cause == cause) {
							t.Fatalf("relevant gap cause not owned: %+v", got)
						}
					}
				})
			}
		}
	}
}

func TestGraceBoundedDonorsUseScoreOrderDespiteEarlierSerialization(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		s, v, b, blocks := graceNeighborFixture(t, order, stateKnown("a"))
		block := blocks[0]
		following := v.NoteOrigins[40]
		following.ScoreEnd = timelineAt(block, stateKnown(mustRational(t, 2, 1)))
		following.Duration = stateKnown(mustRational(t, 1, 1))
		v.NoteOrigins[40] = following
		later := following
		later.Note.Node = 60
		later.ScoreOnset = timelineAt(block, stateKnown(mustRational(t, 2, 1)))
		later.ScoreEnd = timelineAt(block, stateKnown(mustRational(t, 3, 1)))
		v.NoteOrigins[60] = later
		block.content.children = append([]*factNode{
			stateNode(b, timelineRoleForward, 70, stateAbsent[Atomic]()), runtimeNote(t, b, 60, 3, false),
			stateNode(b, timelineRoleBackup, 80, stateAbsent[Atomic]()),
		}, block.content.children...)
		if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
			t.Fatal(err)
		}
		got := v.GraceNeighbors[20]
		if got.Previous.Note.State != EvidenceKnown || got.Previous.Note.Value.Node != 10 || got.Following.Note.State != EvidenceKnown || got.Following.Note.Value.Node != 40 {
			t.Fatalf("serialization hid exact score neighbors: %+v", got)
		}
	}
}

func TestGraceUnplacedMixedVoiceGapKeepsCanonicalMeasureScope(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		for _, explicit := range []bool{false, true} {
			for _, ordinal := range []uint64{0, 1} {
				voice, otherVoice := stateAbsent[string](), stateKnown("a")
				if explicit {
					voice, otherVoice = otherVoice, voice
				}
				s, v, b, blocks := graceNeighborFixture(t, order, voice)
				extra := runtimeNote(t, b, 60, 3, false)
				bad := v.NoteOrigins[40]
				bad.Note.Node, bad.Voice = 60, otherVoice
				bad.ScoreOnset = stateUnknown[musicalPosition]("unplaced-mixed-note")
				v.NoteOrigins[60] = bad
				if ordinal == 0 {
					blocks[0].content.children = append(blocks[0].content.children, extra)
				} else {
					blocks = append(blocks, runtimeBlock(b, ordinal, extra))
				}
				if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
					t.Fatal(err)
				}
				got := v.GraceNeighbors[20]
				for _, side := range []graceDonorTiming{got.Previous, got.Following} {
					if (side.Note.State == EvidenceKnown) != (ordinal == 1) {
						t.Fatalf("wrong mixed-presence scope, explicit=%v ordinal=%d: %+v", explicit, ordinal, got)
					}
				}
			}
		}
	}
}

func TestGraceDuplicateCanonicalMeasureDomainIsCausalNotContract(t *testing.T) {
	s, v, b, blocks := graceNeighborFixture(t, OrderSource, stateKnown("a"))
	extra := runtimeNote(t, b, 60, 3, false)
	duplicate := runtimeBlock(b, 0, extra)
	duplicate.measure.header.ID = 900
	n := v.NoteOrigins[40]
	n.Note.Node = 60
	n.ScoreOnset = timelineAt(duplicate, stateKnown(timelineZero))
	n.ScoreEnd = timelineAt(duplicate, stateKnown(mustRational(t, 1, 1)))
	v.NoteOrigins[60] = n
	if err := v.resolveGraceNeighbors(context.Background(), s, b, append(blocks, duplicate)); err != nil {
		t.Fatal(err)
	}
	got := v.GraceNeighbors[20]
	if got.Previous.Note.State == EvidenceKnown || got.Following.Note.State == EvidenceKnown {
		t.Fatalf("duplicate canonical domain fabricated donor: %+v", got)
	}
}

func TestGracePotentialDeclarationGapsAggregateWithinCanonicalBlock(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		for _, later := range []bool{false, true} {
			_, v, b, blocks := graceNeighborFixture(t, order, stateKnown("a"))
			s := rationalSession(t, Budgets{MaxBufferedBytes: 64 << 10})
			s.documentID = "d"
			defer s.cleanup()
			block := blocks[0]
			if later {
				block = runtimeBlock(b, 1)
				blocks = append(blocks, block)
			}
			cause := &DecisionRef{Rule: RuleKey{"declaration", 1}, Subject: NodeRef{"d", 9000}}
			for i := 0; i < 512; i++ {
				node := stateNode(b, timelineRoleNote, NodeID(10000+i), stateAbsent[Atomic]())
				node.header.Declaration = stateUnknown[DeclarationID]("unavailable-note-declaration")
				if i == 511 {
					node.header.Declaration = Evidence[DeclarationID]{State: EvidenceInvalid, Reason: "invalid-note-declaration", Cause: cause}
				}
				block.content.children = append(block.content.children, node)
			}
			if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
				t.Fatal(err)
			}
			got := v.GraceNeighbors[20]
			for _, side := range []graceDonorTiming{got.Previous, got.Following} {
				if later {
					if side.Note.State != EvidenceKnown {
						t.Fatalf("later declarations erased local donor: %+v", got)
					}
				} else if side.Note.State != EvidenceInvalid || side.Note.Cause == nil || *side.Note.Cause != *cause || side.Note.Cause == cause {
					t.Fatalf("aggregated strongest cause lost ownership: %+v", got)
				}
			}
			s.cleanup()
			if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
				t.Fatalf("scratch retained after cleanup: %d", got)
			}
		}
	}
}
