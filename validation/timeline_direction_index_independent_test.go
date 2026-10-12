package validation

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func independentDirectionIndexFixture(t *testing.T) (*Session, *timelineService, *timelineBindings, []timelineMeasureBlock) {
	t.Helper()
	s, v, b, blocks := directionOriginFixture(t, OrderSource, Budgets{})
	nodes := blocks[0].content.children[:1]
	for i := 0; i < 32; i++ {
		id := NodeID(100 + i)
		nodes = append(nodes, stateNode(b, timelineRoleNote, id, stateAbsent[Atomic]()))
		pos := timelineAt(blocks[0], stateKnown(mustRational(t, int64((i*13)%29), 3)))
		v.Facts[id] = timelineEventFact{Origin: pos, Position: pos, Part: blocks[0].part, Staff: stateAbsent[Integer](), Voice: stateKnown(fmt.Sprintf("v%d", i%3))}
	}
	blocks[0].content.children = nodes
	return s, v, b, blocks
}

func TestIndependentTimelineDirectionIndexMatchesCausalScan(t *testing.T) {
	for _, mode := range []string{"known", "unknown-position", "unknown-voice", "unknown-owner", "duplicate-domain"} {
		t.Run(mode, func(t *testing.T) {
			s, v, b, blocks := independentDirectionIndexFixture(t)
			switch mode {
			case "unknown-position":
				f := v.Facts[104]
				f.Position = stateUnknown[musicalPosition]("unplaced")
				v.Facts[104] = f
			case "unknown-voice":
				f := v.Facts[104]
				f.Voice = stateUnknown[string]("unvoiced")
				v.Facts[104] = f
			case "unknown-owner":
				blocks[0].part = stateUnknown[NodeRef]("owner")
			case "duplicate-domain":
				blocks = append(blocks, blocks[0])
			}
			before := rationalUsage(t, s, BudgetBufferedBytes)
			index, release, err := v.buildDirectionNoteIndex(&stateReader{ctx: context.Background(), s: s, b: b}, blocks)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { release() }()
			for numerator := int64(0); numerator < 31; numerator++ {
				for _, voice := range []Evidence[string]{stateAbsent[string](), stateKnown("v1"), stateKnown("missing")} {
					origin := v.Facts[10]
					origin.Origin.Value.Offset = mustRational(t, numerator, 3)
					origin.Voice = voice
					got, fact, handled, err := index.anchor(context.Background(), s, v, origin)
					if err != nil {
						t.Fatal(err)
					}
					if mode == "known" && !handled {
						t.Fatal("fully proved candidates unnecessarily bypassed index")
					}
					if !handled {
						continue
					}
					r := &stateReader{ctx: context.Background(), s: s, b: b}
					want, wantFact := v.directionAnchor(r, origin, blocks)
					if r.err != nil {
						t.Fatal(r.err)
					}
					if got.State != want.State || got.Value != want.Value || got.Reason != want.Reason {
						t.Fatalf("q=%d/3 voice=%+v fast=%+v slow=%+v", numerator, voice, got, want)
					}
					if got.State == EvidenceKnown && fact.Position.Value.Offset.String() != wantFact.Position.Value.Offset.String() {
						t.Fatal("selected fact differs")
					}
				}
			}
			release()
			release = timelineNoScratch
			if rationalUsage(t, s, BudgetBufferedBytes) != before {
				t.Fatal("index scratch or scan buffers retained unexpectedly")
			}
		})
	}
}

func TestIndependentTimelineDirectionIndexEveryCancellationBoundary(t *testing.T) {
	s, v, b, blocks := independentDirectionIndexFixture(t)
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	_, release, err := v.buildDirectionNoteIndex(&stateReader{ctx: probe, s: s, b: b}, blocks)
	if err != nil {
		t.Fatal(err)
	}
	release()
	calls := probe.calls
	for at := 1; at <= calls; at++ {
		s, v, b, blocks := independentDirectionIndexFixture(t)
		before := rationalUsage(t, s, BudgetBufferedBytes)
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		got, release, err := v.buildDirectionNoteIndex(&stateReader{ctx: ctx, s: s, b: b}, blocks)
		release()
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatalf("poll%d published index or wrong error: %v", at, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != before {
			t.Fatalf("poll%d leaked temporary index", at)
		}
		work := rationalUsage(t, s, BudgetRuleSteps)
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || rationalUsage(t, s, BudgetRuleSteps) != work {
			t.Fatal("cleanup corrupted gauges")
		}
	}
	t.Logf("interrupted all %d direction-index polls", calls)
}
