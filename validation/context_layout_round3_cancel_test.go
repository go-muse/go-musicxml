package validation

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func layoutRound3CancellationFixture(t *testing.T, duplicate bool, limit int64) (*layoutScopes, timelineMeasureBlock) {
	t.Helper()
	measure := localNode(1, 0, "measure")
	nodes := []*factNode{measure}
	if duplicate {
		for id := NodeID(2); id <= 3; id++ {
			print := localNode(id, 1, "print")
			measure.children = append(measure.children, print)
			nodes = append(nodes, print)
		}
	}
	r := layoutRound1Reader(t, nodes...)
	account, err := NewBudgetAccount(Budgets{MaxBufferedBytes: limit})
	if err != nil {
		t.Fatal(err)
	}
	r.s.account = account
	l := &layoutScopes{r: r, key: RuleKey{"layout", 1}, staffOverrides: map[NodeID]map[string]layoutStaffBucket{100: {}}, staffGaps: map[NodeID]Evidence[bool]{}}
	return l, timelineMeasureBlock{measure: measure, content: measure, part: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 100}}}
}

func TestLayoutRound3StaffSelectionCancellationPrecedesAllocation(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		baseline, b := layoutRound3CancellationFixture(t, duplicate, 64<<20)
		counter := &rationalCancelContext{Context: context.Background(), at: int(^uint(0) >> 1)}
		baseline.r.ctx = counter
		if err := baseline.publishStaffLayouts(b, Evidence[layoutStaffRef]{State: EvidenceAbsent}, layoutFrame{}); err != nil {
			t.Fatal(err)
		}
		polls := counter.calls
		baseline.r.s.cleanup()
		for _, limit := range []int64{1, 511, 512, 768, 1024, 64 << 20} {
			for at := 1; at <= polls+1; at++ {
				t.Run(fmt.Sprintf("duplicate=%t/limit=%d/poll=%d", duplicate, limit, at), func(t *testing.T) {
					l, b := layoutRound3CancellationFixture(t, duplicate, limit)
					ctx := &rationalCancelContext{Context: context.Background(), at: at}
					l.r.ctx = ctx
					err := l.publishStaffLayouts(b, Evidence[layoutStaffRef]{State: EvidenceAbsent}, layoutFrame{})
					if ctx.calls >= at {
						if !errors.Is(err, context.Canceled) {
							t.Errorf("observed cancellation replaced: err=%v sticky=%v", err, l.r.err)
						}
						if len(l.r.s.contextResults) != 0 {
							t.Error("canceled output published")
						}
						if errors.Is(l.r.err, context.Canceled) && len(l.staffGaps) != 0 {
							t.Errorf("interrupted selection manufactured staff gap: %+v", l.staffGaps)
						}
						if !duplicate && l.r.s.retained != 0 {
							t.Errorf("empty interrupted selection retained %d", l.r.s.retained)
						}
					} else if err != nil && !errors.Is(err, ErrResourceLimit) {
						t.Errorf("unexpected pre-cancellation error: %v", err)
					}
					l.r.s.cleanup()
					if got := rationalUsage(t, l.r.s, BudgetBufferedBytes); got != 0 {
						t.Errorf("cleanup retained %d", got)
					}
				})
			}
		}
	}
}

func TestLayoutRound3NilStaffOwnerPreservesStickyInterruption(t *testing.T) {
	l, _ := layoutRound3CancellationFixture(t, false, 512)
	l.r.err = context.Canceled
	if _, err := l.readStaffLayouts(nil, map[string]layoutStaffBucket{}, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("nil owner discarded interruption: %v", err)
	}
	if l.r.s.retained != 0 || rationalUsage(t, l.r.s, BudgetBufferedBytes) != 0 {
		t.Error("nil owner retained work")
	}
}
