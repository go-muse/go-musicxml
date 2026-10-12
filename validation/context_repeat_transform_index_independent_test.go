package validation

import (
	"context"
	"errors"
	"testing"
)

func TestIndependentContextRepeatTransformIndexAdmission(t *testing.T) {
	timeline := &timelineService{}
	timeline.State.Transpose.Publications = []statePublication[transposeState]{{Position: musicalPosition{Part: NodeRef{"a", 1}}}, {Position: musicalPosition{Part: NodeRef{"a", 1}}}}
	timeline.State.Transpose.Gaps = []stateGap{{Part: NodeRef{"b", 1}}}
	const bytes = 512 + 3*1024
	for _, limit := range []int64{bytes, bytes - 1} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		out, release, err := contextRepeatTransformParts(context.Background(), s, timeline)
		if limit == bytes {
			if err != nil || len(out) != 2 || !out[NodeRef{"a", 1}] || !out[NodeRef{"b", 1}] || out[NodeRef{"a", 2}] {
				t.Fatalf("exact admission/identity: %v %+v", err, out)
			}
			if got := rationalUsage(t, s, BudgetBufferedBytes); got != bytes {
				t.Fatalf("reserve: %d", got)
			}
			release()
		} else if err == nil || out != nil || release != nil {
			t.Fatalf("one-under admission published partial index: %+v %v", out, err)
		}
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Fatalf("retained after release/failure: %d", got)
		}
		s.cleanup()
	}
	probe := &rationalCancelContext{Context: context.Background(), at: 1000000}
	s := rationalSession(t, Budgets{})
	_, release, err := contextRepeatTransformParts(probe, s, timeline)
	if err != nil {
		t.Fatal(err)
	}
	release()
	s.cleanup()
	for at := 1; at <= probe.calls; at++ {
		s := rationalSession(t, Budgets{})
		out, release, err := contextRepeatTransformParts(&rationalCancelContext{Context: context.Background(), at: at}, s, timeline)
		if !errors.Is(err, context.Canceled) || out != nil || release != nil {
			t.Fatalf("poll %d published canceled index: %v", at, err)
		}
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Fatalf("poll %d retained %d", at, got)
		}
		s.cleanup()
	}
	t.Logf("all %d index cancellation polls preserve zero scratch", probe.calls)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s = rationalSession(t, Budgets{})
	defer s.cleanup()
	out, release, err := contextRepeatTransformParts(ctx, s, &timelineService{})
	if !errors.Is(err, context.Canceled) || out != nil || release != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatalf("empty pre-cancel admitted: %v", err)
	}
}
