package validation

import (
	"context"
	"errors"
	"testing"
)

func independentFloatingAssociationFixture(t *testing.T) (*Session, *timelineService, *timelineBindings, []timelineMeasureBlock) {
	t.Helper()
	s, v, b, blocks := independentDirectionIndexFixture(t)
	var displaced *factNode
	nodes := blocks[0].content.children[:0]
	for _, node := range blocks[0].content.children {
		if node.header.ID == 104 {
			displaced = node
		} else {
			nodes = append(nodes, node)
		}
	}
	blocks[0].content.children = nodes
	later := runtimeBlock(b, 1, displaced)
	later.coverage = stateKnown(true)
	blocks = append(blocks, later)
	fact := v.Facts[104]
	fact.Position = timelineAt(later, stateKnown(mustRational(t, -1, 3)))
	fact.Position.Value.UnprovedFrame = true
	v.Facts[104] = fact
	stateClose(t, s, &v.State, 1)
	return s, v, b, blocks
}

func TestIndependentTimelineFloatingAssociationEveryCancellationBoundary(t *testing.T) {
	for _, figure := range []bool{false, true} {
		run := func(ctx context.Context, s *Session, v *timelineService, b *timelineBindings, blocks []timelineMeasureBlock) error {
			r := &stateReader{ctx: ctx, s: s, b: b}
			origin := v.Facts[10]
			if figure {
				domains := make([]figuredBassDomain, len(blocks))
				for i, block := range blocks {
					domains[i] = figuredBassDomain{block: block, complete: stateKnown(true)}
				}
				v.figuredBassAnchor(r, domains, origin, stateKnown(true))
			} else {
				v.directionAnchor(r, origin, blocks)
			}
			return r.err
		}
		s, v, b, blocks := independentFloatingAssociationFixture(t)
		before := rationalUsage(t, s, BudgetBufferedBytes)
		probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
		if err := run(probe, s, v, b, blocks); err != nil {
			t.Fatal(err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != before {
			t.Fatal("two-pass search retained unescaped scratch")
		}
		s.cleanup()
		for at := 1; at <= probe.calls; at++ {
			s, v, b, blocks := independentFloatingAssociationFixture(t)
			before := rationalUsage(t, s, BudgetBufferedBytes)
			err := run(&rationalCancelContext{Context: context.Background(), at: at}, s, v, b, blocks)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("figure%t poll%d lost cancellation: %v", figure, at, err)
			}
			if rationalUsage(t, s, BudgetBufferedBytes) != before || s.retained != before {
				t.Fatalf("figure%t poll%d scratch retained", figure, at)
			}
			if v.Facts[104].Position.Value.Offset.String() != "-1/3" || !v.Facts[104].Position.Value.UnprovedFrame {
				t.Fatal("search mutated raw candidate")
			}
			if err := run(context.Background(), s, v, b, blocks); err != nil {
				t.Fatal(err)
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("cleanup leaked")
			}
		}
		t.Logf("figure%t: all%d two-pass association cancellation polls", figure, probe.calls)
	}
}
