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
