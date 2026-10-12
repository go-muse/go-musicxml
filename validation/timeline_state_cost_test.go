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

func stateIndexFixture(t *testing.T, budgets Budgets) (*Session, *stateHistory[Integer]) {
	t.Helper()
	s := rationalSession(t, budgets)
	h := &stateHistory[Integer]{}
	for i := range 8 {
		statePublish(t, s, h, statePosition(t, 0, int64(8-i), 1), stateScope{AllStaves: true}, stateKnown(stateInt(t, int64(i))), NodeID(i+100))
	}
	return s, h
}
func TestTimelineStateIndexInterruptionAndInvalidation(t *testing.T) {
	probe, pubs := stateIndexFixture(t, Budgets{})
	base := rationalUsage(t, probe, BudgetBufferedBytes)
	indexBytes := int64(len(pubs.Publications))*256 + 256
	probe.cleanup()
	limited, h := stateIndexFixture(t, Budgets{MaxBufferedBytes: base + indexBytes - 1})
	defer limited.cleanup()
	if _, e := stateIndex(context.Background(), limited, h); !errors.Is(e, ErrResourceLimit) || h.index != nil || rationalUsage(t, limited, BudgetBufferedBytes) != base {
		t.Fatalf("partial index published or leaked: %v", e)
	}
	for _, at := range []int{1, 4, 12, 30} {
		s, h := stateIndexFixture(t, Budgets{})
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		before := rationalUsage(t, s, BudgetBufferedBytes)
		_, e := stateIndex(ctx, s, h)
		if !errors.Is(e, context.Canceled) || h.index != nil || rationalUsage(t, s, BudgetBufferedBytes) != before {
			t.Fatalf("cancel at%d published or leaked index: %v", at, e)
		}
		s.cleanup()
	}
	s, h := stateIndexFixture(t, Budgets{})
	defer s.cleanup()
	index, e := stateIndex(context.Background(), s, h)
	if e != nil {
		t.Fatal(e)
	}
	statePublish(t, s, h, statePosition(t, 0, 9, 1), stateScope{AllStaves: true}, stateKnown(stateInt(t, 99)), 200)
	if h.index != nil {
		t.Fatal("publication did not invalidate index")
	}
	rebuilt, e := stateIndex(context.Background(), s, h)
	if e != nil || rebuilt == index || rebuilt.Count != 9 {
		t.Fatalf("stale index retained %+v,%v", rebuilt, e)
	}
	selection, e := selectState(context.Background(), s, h, statePosition(t, 0, 9, 1), stateInt(t, 1), true)
	if e != nil || selection.Best < 0 || h.Publications[selection.Best].Value.Value.String() != "99" {
		t.Fatalf("rebuilt selection %+v,%v", selection, e)
	}
}
