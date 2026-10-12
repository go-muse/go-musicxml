package validation

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"testing"
)

func TestIndependentTimelineDecimalQuotientOracleAndOwnership(t *testing.T) {
	values := []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(-1, 1), big.NewRat(2, 5), big.NewRat(3, 8), new(big.Rat).SetFloat64(.1), new(big.Rat).SetFrac(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)), new(big.Int).Lsh(big.NewInt(1), 128)), new(big.Rat).SetInt(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 4096), big.NewInt(1)))}
	pairs := 0
	for _, av := range values {
		for _, bv := range values {
			if bv.Sign() == 0 {
				continue
			}
			a, err := NewDecimal(av)
			if err != nil {
				t.Fatal(err)
			}
			b, err := NewDecimal(bv)
			if err != nil {
				t.Fatal(err)
			}
			aText, bText := a.String(), b.String()
			want := new(big.Rat).Quo(av, bv)
			s := rationalSession(t, Budgets{})
			got, err := s.rationalQuoDecimals(context.Background(), a, b)
			if err != nil || !got.Valid() || got.String() != want.RatString() {
				t.Fatalf("quotient pair%d: got%s want%s err%v", pairs, got.String(), want.RatString(), err)
			}
			if got.value == a.value || got.value == b.value || a.String() != aText || b.String() != bText || a.value.Cmp(av) != 0 || b.value.Cmp(bv) != 0 {
				t.Fatal("direct quotient aliased or mutated input")
			}
			copy := got.Value()
			copy.SetInt64(999)
			if got.String() != want.RatString() || got.value.Cmp(want) != 0 {
				t.Fatal("returned mutable numeric copy escaped")
			}
			if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
				t.Fatal("quotient scratch leaked")
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("cleanup leaked")
			}
			pairs++
		}
	}
	t.Logf("%d independent finite-decimal pairs through4096bits", pairs)
}

func TestIndependentTimelineDecimalQuotientLiteralLimits(t *testing.T) {
	a, b := stateDecimal(t, 1, 1), stateDecimal(t, 3, 1)
	// Independent width arithmetic: quotient bounds2/3bits, magnitude2B,text4B;
	// 8 logical units squared times4=256work; (4+2+4+1)*64+4096=4800scratch.
	const work, scalar, scratch int64 = 256, 6, 4800
	oracle := new(big.Rat).Quo(a.value, b.value)
	retained := int64(cap(oracle.Num().Bits())+cap(oracle.Denom().Bits()))*int64(strconv.IntSize/8) + int64(len(oracle.RatString())) + 128
	for _, tc := range []struct {
		name string
		b    Budgets
		pass bool
	}{{"exact", Budgets{MaxRuleSteps: work, MaxScalarBytes: scalar, MaxBufferedBytes: scratch + retained}, true}, {"work-minus1", Budgets{MaxRuleSteps: work - 1}, false}, {"scalar-minus1", Budgets{MaxScalarBytes: scalar - 1}, false}, {"scratch-minus1", Budgets{MaxBufferedBytes: scratch - 1}, false}, {"peak-minus1", Budgets{MaxBufferedBytes: scratch + retained - 1}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			s := rationalSession(t, tc.b)
			defer s.cleanup()
			got, err := s.rationalQuoDecimals(context.Background(), a, b)
			if tc.pass {
				if err != nil || got.String() != "1/3" || rationalUsage(t, s, BudgetRuleSteps) != work || rationalUsage(t, s, BudgetBufferedBytes) != retained {
					t.Fatalf("exact boundary got%s err%v work%d bytes%d", got.String(), err, rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetBufferedBytes))
				}
			} else if !errors.Is(err, ErrResourceLimit) || got.Valid() || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatalf("failed admission leaked/published: %+v %v", got, err)
			}
		})
	}
	huge, _ := NewDecimal(new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 4096)))
	s := rationalSession(t, Budgets{MaxScalarBytes: 8})
	defer s.cleanup()
	if got, err := s.rationalQuoDecimals(context.Background(), huge, huge); !errors.Is(err, ErrResourceLimit) || got.Valid() || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatalf("small result bypassed wide inputs: %s %v", got.String(), err)
	}
}

func TestIndependentTimelineDecimalQuotientEveryCancellationBoundary(t *testing.T) {
	a, b := stateDecimal(t, 1, 5), stateDecimal(t, 3, 8)
	s := rationalSession(t, Budgets{})
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if _, err := s.rationalQuoDecimals(probe, a, b); err != nil {
		t.Fatal(err)
	}
	s.cleanup()
	calls := probe.calls
	for at := 1; at <= calls; at++ {
		s := rationalSession(t, Budgets{})
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		got, err := s.rationalQuoDecimals(ctx, a, b)
		if !errors.Is(err, context.Canceled) || got.Valid() || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("poll%d leaked/published: %v", at, err)
		}
		if a.String() != "0.2" || b.String() != "0.375" {
			t.Fatal("cancelled quotient mutated inputs")
		}
		s.cleanup()
	}
	t.Logf("cancelled all %d direct quotient polls", calls)
}
