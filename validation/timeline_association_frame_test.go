package validation

import (
	"context"
	"testing"
)

func TestTimelineAssociationFrameCandidatesAndFastPath(t *testing.T) {
	for _, family := range []string{"direction", "figure"} {
		for _, mode := range []string{"later-negative", "earlier-positive", "later-contained", "later-positive-excluded", "same-frame-before", "same-frame-nearest"} {
			t.Run(family+"/"+mode, func(t *testing.T) {
				ctx := context.Background()
				s, v, b, blocks := directionOriginFixture(t, OrderSource, Budgets{})
				origin := v.Facts[10]
				extra := runtimeNote(t, b, 70, 1, false)
				block := runtimeBlock(b, 1, extra)
				block.coverage = stateKnown(true)
				at := timelineAt(block, stateKnown(mustRational(t, -2, 1)))
				at.Value.UnprovedFrame = true
				wantKnown, wantID, wantHandled := false, NodeID(40), false
				switch mode {
				case "earlier-positive":
					later := runtimeBlock(b, 1, blocks[0].content.children...)
					later.coverage = stateKnown(true)
					for id, fact := range v.Facts {
						fact.Position.Value.Measure, fact.Position.Value.MeasureOrdinal = NodeRef{"d", later.measure.header.ID}, 1
						fact.Origin = fact.Position
						v.Facts[id] = fact
					}
					origin = v.Facts[10]
					block = runtimeBlock(b, 0, extra)
					at = timelineAt(block, stateKnown(mustRational(t, 10, 1)))
					at.Value.UnprovedFrame = true
					blocks = []timelineMeasureBlock{block, later}
				case "later-contained":
					at.Value.Offset, at.Value.UnprovedFrame = timelineZero, false
					wantKnown, wantHandled = true, true
				case "later-positive-excluded":
					at.Value.Offset = mustRational(t, 10, 1)
					wantKnown, wantHandled = true, true
				case "same-frame-before", "same-frame-nearest":
					block = blocks[0]
					block.content.children = append(block.content.children, extra)
					at = timelineAt(block, stateKnown(mustRational(t, -2, 1)))
					at.Value.UnprovedFrame = true
					wantKnown, wantHandled = true, true
					if mode == "same-frame-nearest" {
						at.Value.Offset = mustRational(t, 3, 2)
						wantID, wantHandled = 70, false
					}
				}
				if mode != "earlier-positive" && mode != "same-frame-before" && mode != "same-frame-nearest" {
					blocks = append(blocks, block)
				}
				v.Facts[70] = timelineEventFact{Part: block.part, Origin: at, Position: at, Voice: stateAbsent[string](), Staff: stateAbsent[Integer]()}
				stateClose(t, s, &v.State, 1)
				reader := &stateReader{ctx: ctx, s: s, b: b}
				var got Evidence[NodeRef]
				var handled bool
				if family == "direction" {
					index, release, err := v.buildDirectionNoteIndex(reader, blocks)
					if err != nil {
						t.Fatal(err)
					}
					defer release()
					got, _, handled, err = index.anchor(ctx, s, v, origin)
					if err != nil {
						t.Fatal(err)
					}
					if !handled {
						got, _ = v.directionAnchor(reader, origin, blocks)
					}
				} else {
					domains := make([]figuredBassDomain, len(blocks))
					for i, block := range blocks {
						domains[i] = figuredBassDomain{block: block, complete: stateKnown(true)}
					}
					index, release, err := v.buildFiguredBassNoteIndex(reader, domains)
					if err != nil {
						t.Fatal(err)
					}
					defer release()
					got, _, handled, err = index.anchor(ctx, s, v, origin)
					if err != nil {
						t.Fatal(err)
					}
					if !handled {
						got, _ = v.figuredBassAnchor(reader, domains, origin, stateKnown(true))
					}
				}
				if reader.err != nil {
					t.Fatal(reader.err)
				}
				if handled != wantHandled {
					t.Errorf("indexed proof = %v, want %v", handled, wantHandled)
				}
				if wantKnown {
					if got.State != EvidenceKnown || got.Value.Node != wantID {
						t.Fatalf("exact control lost: %+v", got)
					}
				} else if got.State == EvidenceKnown {
					t.Fatalf("unproved competing frame selected: %+v", got)
				}
			})
		}
	}
}

func TestTimelineGraceFrameCompetitorAndSideLocality(t *testing.T) {
	for _, mode := range []string{"later-negative", "later-positive-excluded", "same-frame-before", "earlier-positive"} {
		t.Run(mode, func(t *testing.T) {
			s, v, b, blocks := graceNeighborFixture(t, OrderSource, stateKnown("a"))
			extra := runtimeNote(t, b, 70, 1, false)
			block := runtimeBlock(b, 1, extra)
			at := timelineAt(block, stateKnown(mustRational(t, -2, 1)))
			at.Value.UnprovedFrame = true
			if mode == "later-positive-excluded" {
				at.Value.Offset = mustRational(t, 10, 1)
			}
			if mode == "same-frame-before" {
				block = blocks[0]
				block.content.children = append(block.content.children, extra)
				at = timelineAt(block, stateKnown(mustRational(t, -2, 1)))
				at.Value.UnprovedFrame = true
			} else if mode == "earlier-positive" {
				later := runtimeBlock(b, 1, blocks[0].content.children...)
				for id, n := range v.NoteOrigins {
					n.ScoreOnset.Value.Measure = NodeRef{"d", later.measure.header.ID}
					n.ScoreOnset.Value.MeasureOrdinal = 1
					if n.ScoreEnd.State == EvidenceKnown {
						n.ScoreEnd.Value.Measure = n.ScoreOnset.Value.Measure
						n.ScoreEnd.Value.MeasureOrdinal = 1
					}
					v.NoteOrigins[id] = n
				}
				block = runtimeBlock(b, 0, extra)
				blocks = []timelineMeasureBlock{block, later}
				at = timelineAt(block, stateKnown(mustRational(t, 10, 1)))
				at.Value.UnprovedFrame = true
			}
			if mode != "same-frame-before" && mode != "earlier-positive" {
				blocks = append(blocks, block)
			}
			n := v.NoteOrigins[40]
			n.Note.Node = 70
			n.ScoreOnset = at
			n.ScoreEnd = at
			n.Duration = stateKnown(mustRational(t, 1, 1))
			v.NoteOrigins[70] = n
			if err := v.resolveGraceNeighbors(context.Background(), s, b, blocks); err != nil {
				t.Fatal(err)
			}
			got := v.GraceNeighbors[20]
			wantKnown := mode == "later-positive-excluded" || mode == "same-frame-before"
			for _, side := range []graceDonorTiming{got.Previous, got.Following} {
				if (side.Note.State == EvidenceKnown) != wantKnown {
					t.Fatalf("wrong donor chronology for %s: %+v", mode, got)
				}
			}
		})
	}
}

func TestTimelineFigureChainDisplacedGapKeepsActualCause(t *testing.T) {
	for _, offset := range []int64{-2, 2} {
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		s.documentID = "d"
		b := noteOriginBindings()
		first := stateNode(b, timelineRoleFiguredBass, 10, stateAbsent[Atomic]())
		bad := stateNode(b, timelineRoleFiguredBass, 20, stateAbsent[Atomic]())
		block := runtimeBlock(b, 0, first)
		later := runtimeBlock(b, 1, bad)
		at := timelineAt(block, stateKnown(mustRational(t, 1, 1)))
		gapAt := timelineAt(later, stateKnown(mustRational(t, offset, 1)))
		gapAt.Value.UnprovedFrame = true
		cause := &DecisionRef{Rule: RuleKey{"floating-figure", 1}, Subject: NodeRef{"d", 20}}
		pending := map[NodeID]figuredBassTimingOrigin{
			10: {FiguredBass: NodeRef{"d", 10}, Part: block.part, Anchor: stateKnown(NodeRef{"d", 30}), AnchorPosition: at, Position: at, DurationRaw: stateAbsent[Decimal](), Duration: stateAbsent[Rational]()},
			20: {FiguredBass: NodeRef{"d", 20}, Part: block.part, Anchor: Evidence[NodeRef]{State: EvidenceInvalid, Reason: "invalid-floating-figure", Cause: cause}, CursorOrigin: gapAt},
		}
		r := &stateReader{ctx: context.Background(), s: s, b: b}
		if err := resolveFiguredBassChains(r, []figuredBassDomain{{block: block, complete: stateKnown(true)}, {block: later, complete: stateKnown(true)}}, pending, []NodeID{10, 20}); err != nil {
			t.Fatal(err)
		}
		got := pending[10].GroupStart
		if offset < 0 {
			if got.State != EvidenceInvalid || got.Cause != cause {
				t.Fatalf("possible displaced chain member lost cause: %+v", got)
			}
		} else if got.State != EvidenceKnown {
			t.Fatalf("definitely later displaced chain gap erased known group: %+v", got)
		}
	}
}
