package validation

import (
	"context"
	"errors"
	"testing"
)

func independentRepeatContentRosterNotes() []contextRepeatNote {
	return []contextRepeatNote{
		{source: NodeRef{"d", 1}, part: stateKnown(NodeRef{"d", 10})},
		{source: NodeRef{"d", 2}, part: stateUnknown[NodeRef]("unplaced")},
		{source: NodeRef{"d", 3}, part: stateKnown(NodeRef{"other", 10})},
	}
}

func TestIndependentContextRepeatContentRosterAdmission(t *testing.T) {
	notes := independentRepeatContentRosterNotes()
	const indexBytes = 512 + 3*1024
	for _, limit := range []int64{indexBytes, indexBytes - 1} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		index, release, err := contextRepeatBuildContentRoster(context.Background(), s, notes)
		if limit == indexBytes {
			if err != nil || len(index.parts) != 2 || len(index.unknown) != 1 {
				t.Fatalf("exact roster admission: %v %+v", err, index)
			}
			release()
		} else if err == nil || index != nil || release != nil {
			t.Fatalf("partial index on refusal: %v", err)
		}
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Fatalf("build retained %d", got)
		}
		s.cleanup()
	}
	for _, limit := range []int64{indexBytes + 4096, indexBytes + 4096 - 1} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		index, releaseIndex, err := contextRepeatBuildContentRoster(context.Background(), s, notes)
		if err != nil {
			t.Fatal(err)
		}
		selected, release, err := index.partNotes(context.Background(), s, NodeRef{"d", 10})
		if limit == indexBytes+4096 {
			if err != nil || len(selected) != 2 || selected[0].source.Node != 1 || selected[1].source.Node != 2 || cap(selected) != 2 {
				t.Fatalf("part selection exact identity/order: %v %+v", err, selected)
			}
			release()
		} else if err == nil || selected != nil || release != nil {
			t.Fatalf("partial selection on refusal: %v", err)
		}
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != indexBytes {
			t.Fatalf("selection retained %d", got)
		}
		releaseIndex()
		s.cleanup()
	}
	for _, build := range []bool{true, false} {
		run := func(ctx context.Context) (error, int64) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			buildCtx := context.Background()
			if build {
				buildCtx = ctx
			}
			index, releaseIndex, err := contextRepeatBuildContentRoster(buildCtx, s, notes)
			if err != nil {
				return err, rationalUsage(t, s, BudgetBufferedBytes)
			}
			if build {
				releaseIndex()
				return nil, rationalUsage(t, s, BudgetBufferedBytes)
			}
			selected, release, err := index.partNotes(ctx, s, NodeRef{"d", 10})
			if err == nil {
				release()
			} else if selected != nil || release != nil {
				t.Fatal("cancellation published partial selection")
			}
			releaseIndex()
			return err, rationalUsage(t, s, BudgetBufferedBytes)
		}
		probe := &rationalCancelContext{Context: context.Background(), at: 1000000}
		if err, retained := run(probe); err != nil || retained != 0 {
			t.Fatalf("probe: %v %d", err, retained)
		}
		for at := 1; at <= probe.calls; at++ {
			if err, retained := run(&rationalCancelContext{Context: context.Background(), at: at}); !errors.Is(err, context.Canceled) || retained != 0 {
				t.Fatalf("build=%t poll=%d: %v retained=%d", build, at, err, retained)
			}
		}
		t.Logf("build=%t canceled all %d polls", build, probe.calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	index, release, err := contextRepeatBuildContentRoster(ctx, s, nil)
	if !errors.Is(err, context.Canceled) || index != nil || release != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatalf("empty pre-cancel admitted: %v", err)
	}
}
