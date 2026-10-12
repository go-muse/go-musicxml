package validation

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
)

func TestTimelineStateSparseTrajectoryCosts(t *testing.T) {
	for _, count := range []int{32, 128, 256} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			h := &timelineState{}
			stateClose(t, s, h, 0)
			scope := stateScope{AllStaves: true}
			staff := stateKnown(stateInt(t, 1))
			for i := range count {
				statePublish(t, s, &h.Staves, statePosition(t, 0, int64(i), 1), scope, stateKnown(stateInt(t, int64(i))), NodeID(i+100))
			}
			before := rationalUsage(t, s, BudgetBufferedBytes)
			for i := range count {
				got := stateQuery(t, s, h, &h.Staves, statePosition(t, 0, int64(i), 1), staff)
				if got.Value.State != EvidenceKnown || got.Value.Value.String() != fmt.Sprint(i) {
					t.Fatalf("query%d=%+v", i, got)
				}
			}
			t.Logf("publications=%d queries=%d steps=%d retained_total=%d query_bytes=%d", count, count, rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetBufferedBytes), rationalUsage(t, s, BudgetBufferedBytes)-before)
		})
	}
}
func TestTimelineStateTrajectoryBudgetExhaustionIsIncomplete(t *testing.T) {
	s := rationalSession(t, Budgets{MaxRuleSteps: 20000})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	for i := range 16 {
		statePublish(t, s, &h.Staves, statePosition(t, 0, int64(i), 1), stateScope{AllStaves: true}, stateKnown(stateInt(t, int64(i))), NodeID(i+100))
	}
	interrupted := false
	for i := range 16 {
		_, e := queryState(context.Background(), s, h, &h.Staves, statePosition(t, 0, int64(i), 1), stateKnown(stateInt(t, 1)), stateWhole[Integer])
		if e != nil {
			if !errors.Is(e, ErrResourceLimit) || !errors.Is(e, ErrIncompleteAssessment) {
				t.Fatalf("wrong quota error %v", e)
			}
			interrupted = true
			break
		}
	}
	if !interrupted {
		t.Fatal("small work budget failed to bound repeated scans")
	}
}
func BenchmarkTimelineStateSparseHistory(b *testing.B) {
	for _, count := range []int{32, 128, 512} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			one, _ := NewInteger(big.NewInt(1))
			ctx := context.Background()
			for range b.N {
				account, _ := NewBudgetAccount(Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 30})
				s := &Session{account: account}
				h := &timelineState{}
				part := NodeRef{DocumentID: "benchmark", Node: 1}
				measure := NodeRef{DocumentID: "benchmark", Node: 2}
				if e := h.closePart(ctx, s, part, 0, Evidence[bool]{State: EvidenceKnown, Value: true}); e != nil {
					b.Fatal(e)
				}
				for i := range count {
					offset, _ := NewRational(big.NewRat(int64(i), 1))
					if e := publishState(ctx, s, &h.Staves, statePublication[Integer]{Position: musicalPosition{Part: part, Measure: measure, Offset: offset}, Scope: stateScope{AllStaves: true}, Value: Evidence[Integer]{State: EvidenceKnown, Value: one}}); e != nil {
						b.Fatal(e)
					}
				}
				for i := range count {
					offset, _ := NewRational(big.NewRat(int64(i), 1))
					if _, e := queryState(ctx, s, h, &h.Staves, musicalPosition{Part: part, Measure: measure, Offset: offset}, Evidence[Integer]{State: EvidenceKnown, Value: one}, stateWhole[Integer]); e != nil {
						b.Fatal(e)
					}
				}
				s.cleanup()
			}
		})
	}
}
