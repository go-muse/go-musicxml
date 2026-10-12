package validation

import (
	"context"
	"errors"
	"math"
	"math/big"
	"strconv"
	"sync"
	"testing"
)

func mustRational(t *testing.T, n, d int64) Rational {
	t.Helper()
	v, err := NewRational(big.NewRat(n, d))
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func rationalSession(t *testing.T, budgets Budgets) *Session {
	t.Helper()
	a, err := NewBudgetAccount(budgets)
	if err != nil {
		t.Fatal(err)
	}
	return &Session{account: a}
}
func rationalUsage(t *testing.T, s *Session, kind BudgetKind) int64 {
	t.Helper()
	n, err := s.account.Usage(kind)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRationalConstructorCanonicalImmutable(t *testing.T) {
	for _, tc := range []struct {
		n, d int64
		want string
	}{
		{0, 1, "0"}, {1, 3, "1/3"}, {-1, 3, "-1/3"}, {6, 15, "2/5"}, {8, 4, "2"},
	} {
		raw := big.NewRat(1, 1)
		raw.Num().SetInt64(tc.n)
		raw.Denom().SetInt64(tc.d)
		v, err := NewRational(raw)
		if err != nil {
			t.Fatal(err)
		}
		raw.SetInt64(91)
		if !v.Valid() || v.String() != tc.want {
			t.Fatalf("canonical copy = %s; want %s", v, tc.want)
		}
		copy := v.Value()
		copy.Num().SetInt64(17)
		copy.Denom().SetInt64(19)
		if v.String() != tc.want {
			t.Fatal("accessor aliases immutable value")
		}
		if v.Value().RatString() != tc.want || v.Value().Denom().Sign() <= 0 {
			t.Fatal("retained value not canonical")
		}
		wantBytes := magnitudeBytes(v.Value().Num()) + magnitudeBytes(v.Value().Denom()) + int64(len(tc.want))
		if v.ByteSize() != wantBytes {
			t.Fatalf("bytes=%d want %d", v.ByteSize(), wantBytes)
		}
	}
	z, err := NewRational(new(big.Rat))
	if err != nil || z.String() != "0" {
		t.Fatalf("numeric zero = %v,%v", z, err)
	}
	if _, err := NewDecimal(big.NewRat(1, 3)); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("Decimal widened: %v", err)
	}
}
func TestRationalConstructorUninitializedAndMalformed(t *testing.T) {
	var z Rational
	if z.Valid() || z.Value() != nil || z.ByteSize() != 0 || z.String() != "<uninitialized>" {
		t.Fatal("zero value became numeric zero")
	}
	if _, err := NewRational(nil); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("nil=%v", err)
	}
	raw := big.NewRat(1, 3)
	raw.Denom().SetInt64(-3)
	if _, err := NewRational(raw); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("negative denominator=%v", err)
	}
}
func TestRationalConstructorBoundsMagnitudeAndFractionText(t *testing.T) {
	huge := new(big.Int).Lsh(big.NewInt(1), 8*MaxExactValueBytes)
	if _, err := NewRational(new(big.Rat).SetInt(huge)); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("magnitude=%v", err)
	}
	wide := new(big.Int).Lsh(big.NewInt(1), 4*MaxExactValueBytes)
	if _, err := NewRational(new(big.Rat).SetFrac(big.NewInt(1), wide)); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("fraction text=%v", err)
	}
	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(MaxExactValueBytes-3), nil)
	raw := new(big.Rat).SetFrac(big.NewInt(1), denominator)
	exact, err := NewRational(raw)
	if err != nil || len(exact.String()) != MaxExactValueBytes {
		t.Fatalf("exact boundary: len=%d err=%v", len(exact.String()), err)
	}
	raw.Neg(raw)
	if _, err := NewRational(raw); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("boundary+1=%v", err)
	}
	denominator.Quo(denominator, big.NewInt(10))
	below, err := NewRational(new(big.Rat).SetFrac(big.NewInt(1), denominator))
	if err != nil || len(below.String()) != MaxExactValueBytes-1 {
		t.Fatalf("boundary-1: len=%d err=%v", len(below.String()), err)
	}
}
func TestRationalSessionExactOperations(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	ctx := context.Background()
	a, b := mustRational(t, 1, 3), mustRational(t, 1, 6)
	for _, tc := range []struct {
		name string
		fn   func(context.Context, Rational, Rational) (Rational, error)
		a, b Rational
		want string
	}{
		{"add", s.rationalAdd, a, b, "1/2"},
		{"sub", s.rationalSub, b, a, "-1/6"},
		{"mul", s.rationalMul, mustRational(t, -2, 3), mustRational(t, 9, 4), "-3/2"},
		{"quo", s.rationalQuo, a, mustRational(t, -2, 1), "-1/6"},
		{"cancel", s.rationalAdd, a, mustRational(t, -1, 3), "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			av, bv := tc.a.String(), tc.b.String()
			v, err := tc.fn(ctx, tc.a, tc.b)
			if err != nil || v.String() != tc.want {
				t.Fatalf("got %v,%v want %s", v, err, tc.want)
			}
			if tc.a.String() != av || tc.b.String() != bv {
				t.Fatal("input mutated")
			}
		})
	}
	for _, tc := range []struct {
		a, b Rational
		want int
	}{{a, b, 1}, {b, a, -1}, {a, a, 0}, {mustRational(t, -1, 2), b, -1}} {
		got, err := s.rationalCmp(ctx, tc.a, tc.b)
		if err != nil || got != tc.want {
			t.Fatalf("cmp=%d,%v want %d", got, err, tc.want)
		}
	}
	decimal, _ := NewDecimal(big.NewRat(1, 10))
	divisions, _ := NewDecimal(big.NewRat(3, 1))
	duration, err := s.rationalFromDecimal(ctx, decimal)
	if err != nil {
		t.Fatal(err)
	}
	divisor, err := s.rationalFromDecimal(ctx, divisions)
	if err != nil {
		t.Fatal(err)
	}
	position, err := s.rationalQuo(ctx, duration, divisor)
	if err != nil || position.String() != "1/30" {
		t.Fatalf("decimal division = %v,%v", position, err)
	}
	if decimal.String() != "0.1" || divisions.String() != "3" {
		t.Fatal("decimal input mutated")
	}
	if rationalUsage(t, s, BudgetRuleSteps) == 0 || s.retained == 0 {
		t.Fatal("arithmetic unmetered")
	}
	if got := rationalUsage(t, s, BudgetBufferedBytes); got != s.retained {
		t.Fatalf("scratch leaked: %d vs retained %d", got, s.retained)
	}
}
func TestRationalSessionRejectsInvalidOperands(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	ctx := context.Background()
	one, zero := mustRational(t, 1, 1), mustRational(t, 0, 1)
	for _, fn := range []func(context.Context, Rational, Rational) (Rational, error){s.rationalAdd, s.rationalSub, s.rationalMul, s.rationalQuo} {
		for _, pair := range [][2]Rational{{{}, one}, {one, {}}} {
			if _, err := fn(ctx, pair[0], pair[1]); !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("uninitialized=%v", err)
			}
		}
	}
	if _, err := s.rationalQuo(ctx, one, zero); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("zero division=%v", err)
	}
	if _, err := s.rationalCmp(ctx, Rational{}, one); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("uninitialized compare=%v", err)
	}
	if _, err := s.rationalFromDecimal(ctx, Decimal{}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("uninitialized decimal=%v", err)
	}
	if _, err := s.rationalAdd(nil, one, one); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("nil context=%v", err)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("invalid operation retained scratch")
	}
}

func TestRationalCostChecksOverflowAndHostWidth(t *testing.T) {
	for _, tc := range []struct {
		op   rationalOperation
		a, b rationalWidths
	}{
		{rationalAddOp, rationalWidths{math.MaxInt64, 1}, rationalWidths{1, 1}},
		{rationalMulOp, rationalWidths{math.MaxInt64 / 2, 1}, rationalWidths{math.MaxInt64 / 2, 1}},
		{rationalCmpOp, rationalWidths{1, math.MaxInt64}, rationalWidths{1, math.MaxInt64}},
	} {
		if _, err := rationalPlan(tc.op, tc.a, tc.b); !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("overflow plan=%v", err)
		}
	}
	if _, err := rationalCheckedMul(BudgetRuleSteps, math.MaxInt64, 2); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("work multiply overflow=%v", err)
	}
	if _, err := rationalCheckedAdd(BudgetBufferedBytes, math.MaxInt64, 1); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("allocation add overflow=%v", err)
	}
	if strconv.IntSize == 32 {
		if _, err := rationalAllocation(int64(math.MaxInt32) + 1); !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("host overflow=%v", err)
		}
	} else if _, err := rationalAllocation(int64(math.MaxInt32) + 1); err != nil {
		t.Fatalf("64-bit allocation guard=%v", err)
	}
}
func TestRationalMeterBoundariesAndCleanup(t *testing.T) {
	a, b := mustRational(t, 1, 3), mustRational(t, 1, 6)
	cost, err := rationalPlan(rationalAddOp, rationalWidthsOf(a.value), rationalWidthsOf(b.value))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		limits Budgets
		ok     bool
	}{
		{"work below", Budgets{MaxRuleSteps: cost.work - 1}, false},
		{"work exact", Budgets{MaxRuleSteps: cost.work}, true},
		{"work above", Budgets{MaxRuleSteps: cost.work + 1}, true},
		{"scalar below", Budgets{MaxScalarBytes: cost.scalar - 1}, false},
		{"scalar exact", Budgets{MaxScalarBytes: cost.scalar}, true},
		{"scalar above", Budgets{MaxScalarBytes: cost.scalar + 1}, true},
		{"scratch below", Budgets{MaxBufferedBytes: cost.scratch - 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := rationalSession(t, tc.limits)
			v, err := s.rationalAdd(context.Background(), a, b)
			if tc.ok {
				if err != nil || v.String() != "1/2" {
					t.Fatalf("boundary=%v,%v", v, err)
				}
			} else {
				if !errors.Is(err, ErrResourceLimit) || !errors.Is(err, ErrIncompleteAssessment) || v.Valid() {
					t.Fatalf("resource rejection=%v,%v", v, err)
				}
			}
			if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
				t.Fatal("scratch leaked")
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("retained payload leaked")
			}
		})
	}
	// A mathematically tiny cancellation still has large unreduced intermediates.
	numerator := new(big.Int).Lsh(big.NewInt(1), 200)
	large, _ := NewRational(new(big.Rat).SetFrac(numerator, big.NewInt(3)))
	negative, _ := NewRational(new(big.Rat).Neg(large.Value()))
	s := rationalSession(t, Budgets{MaxScalarBytes: large.ByteSize()})
	if _, err := s.rationalAdd(context.Background(), large, negative); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("cancellation escaped intermediate cap: %v", err)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("intermediate rejection leaked")
	}
}

type rationalCancelContext struct {
	context.Context
	calls, at int
}

func (c *rationalCancelContext) Err() error {
	c.calls++
	if c.calls >= c.at {
		return context.Canceled
	}
	return nil
}
func TestRationalCancellationCleansScratchAndRetainsNoOutput(t *testing.T) {
	for _, at := range []int{1, 3, 4} {
		s := rationalSession(t, Budgets{})
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		v, err := s.rationalAdd(ctx, mustRational(t, 1, 3), mustRational(t, 1, 6))
		if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrIncompleteAssessment) || v.Valid() {
			t.Fatalf("cancel check %d: %v,%v", at, v, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || s.retained != 0 {
			t.Fatalf("cancel check %d leaked scratch", at)
		}
	}
}
func TestRationalSharedAndIndependentAccounts(t *testing.T) {
	a, b := mustRational(t, 1, 3), mustRational(t, 1, 6)
	cost, err := rationalPlan(rationalAddOp, rationalWidthsOf(a.value), rationalWidthsOf(b.value))
	if err != nil {
		t.Fatal(err)
	}
	first := rationalSession(t, Budgets{MaxRuleSteps: cost.work})
	child := &Session{account: first.account}
	if _, err := first.rationalAdd(context.Background(), a, b); err != nil {
		t.Fatal(err)
	}
	if _, err := child.rationalAdd(context.Background(), a, b); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("child reset=%v", err)
	}
	independent := rationalSession(t, Budgets{MaxRuleSteps: cost.work})
	if _, err := independent.rationalAdd(context.Background(), a, b); err != nil {
		t.Fatalf("independent quota affected=%v", err)
	}
	child.cleanup()
	if rationalUsage(t, first, BudgetBufferedBytes) != first.retained {
		t.Fatal("child released another session's output")
	}
	first.cleanup()
	independent.cleanup()
}
func TestRationalConcurrentImmutableReads(t *testing.T) {
	v := mustRational(t, 1, 3)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 64 {
				if v.Value().RatString() != "1/3" || v.String() != "1/3" {
					t.Error("concurrent value changed")
				}
			}
		})
	}
	wg.Wait()
}

func TestRationalOutputRetentionPeakAndFinishCleanup(t *testing.T) {
	a, b := mustRational(t, 1, 3), mustRational(t, 1, 6)
	cost, err := rationalPlan(rationalAddOp, rationalWidthsOf(a.value), rationalWidthsOf(b.value))
	if err != nil {
		t.Fatal(err)
	}
	probe := rationalSession(t, Budgets{})
	if _, err := probe.rationalAdd(context.Background(), a, b); err != nil {
		t.Fatal(err)
	}
	peak := cost.scratch + probe.retained
	probe.cleanup()
	for _, delta := range []int64{-1, 0, 1} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: peak + delta})
		v, err := s.rationalAdd(context.Background(), a, b)
		if delta < 0 {
			if !errors.Is(err, ErrResourceLimit) || v.Valid() || s.retained != 0 {
				t.Fatalf("output reservation failure=%v,%v retained=%d", v, err, s.retained)
			}
		} else if err != nil || v.String() != "1/2" {
			t.Fatalf("output peak %+d=%v,%v", delta, v, err)
		}
		if rationalUsage(t, s, BudgetRuleSteps) != cost.work {
			t.Fatal("work was refunded or not precharged")
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("scratch survived output reservation")
		}
		s.cleanup()
	}
	_, s := sessionFixture(t, Budgets{})
	if _, err := s.rationalAdd(context.Background(), a, b); err != nil {
		t.Fatal(err)
	}
	_, _ = s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("Finish did not release rational outputs")
	}
}

func TestRationalComparisonAndConversionMetering(t *testing.T) {
	a, b := mustRational(t, 1, 3), mustRational(t, 1, 6)
	cost, err := rationalPlan(rationalCmpOp, rationalWidthsOf(a.value), rationalWidthsOf(b.value))
	if err != nil {
		t.Fatal(err)
	}
	for _, delta := range []int64{-1, 0, 1} {
		s := rationalSession(t, Budgets{MaxRuleSteps: cost.work + delta})
		got, err := s.rationalCmp(context.Background(), a, b)
		if delta < 0 {
			if !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("comparison work cap=%v", err)
			}
		} else if err != nil || got != 1 {
			t.Fatalf("comparison boundary=%d,%v", got, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || s.retained != 0 {
			t.Fatal("comparison retained memory")
		}
	}
	decimal, _ := NewDecimal(big.NewRat(7, 20))
	cost, err = rationalPlan(rationalCopyOp, rationalWidthsOf(decimal.value), rationalWidths{})
	if err != nil {
		t.Fatal(err)
	}
	s := rationalSession(t, Budgets{MaxRuleSteps: cost.work - 1})
	if _, err := s.rationalFromDecimal(context.Background(), decimal); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("conversion skipped work cap=%v", err)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("failed conversion retained memory")
	}
	s = rationalSession(t, Budgets{MaxRuleSteps: cost.work})
	v, err := s.rationalFromDecimal(context.Background(), decimal)
	if err != nil || v.String() != "7/20" {
		t.Fatalf("conversion exact work=%v,%v", v, err)
	}
	s.cleanup()
}

func TestRationalDefaultBudgetOrdinaryModelTrajectory(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	ctx := context.Background()
	durationDecimal, err := NewDecimal(new(big.Rat).SetFloat64(0.1))
	if err != nil {
		t.Fatal(err)
	}
	divisionsDecimal, err := NewDecimal(big.NewRat(3, 1))
	if err != nil {
		t.Fatal(err)
	}
	divisions, err := s.rationalFromDecimal(ctx, divisionsDecimal)
	if err != nil {
		t.Fatal(err)
	}
	position := mustRational(t, 0, 1)
	for note := 1; note <= 1000; note++ {
		duration, err := s.rationalFromDecimal(ctx, durationDecimal)
		if err != nil {
			t.Fatalf("note %d duration: %v", note, err)
		}
		increment, err := s.rationalQuo(ctx, duration, divisions)
		if err != nil {
			t.Fatalf("note %d quotient: %v", note, err)
		}
		position, err = s.rationalAdd(ctx, position, increment)
		if err != nil {
			t.Fatalf("note %d cursor: %v", note, err)
		}
	}
	// 1000 * exact float64(0.1) / 3, reduced by hand. This intentionally differs
	// from the source decimal 0.1 trajectory's 100/3.
	if position.String() != "450359962737049625/13510798882111488" {
		t.Fatalf("exact final position=%s", position)
	}
	if work := rationalUsage(t, s, BudgetRuleSteps); work > 10_000_000 {
		t.Fatalf("ordinary model arithmetic leaves insufficient default-budget headroom: %d", work)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
		t.Fatal("trajectory leaked scratch")
	}
}

func TestRationalWorkScalingKeepsLargeFractionsBounded(t *testing.T) {
	for _, op := range []rationalOperation{rationalCopyOp, rationalAddOp, rationalSubOp, rationalMulOp, rationalQuoOp, rationalCmpOp} {
		var previous int64
		for _, bits := range []int64{256, 512, 1024, 2048, 4096, 8192} {
			other := rationalWidths{bits, bits}
			if op == rationalCopyOp {
				other = rationalWidths{}
			}
			cost, err := rationalPlan(op, rationalWidths{bits, bits}, other)
			if err != nil {
				t.Fatal(err)
			}
			if previous != 0 && (cost.work < 3*previous || cost.work > 5*previous) {
				t.Fatalf("operation %d doubling widths does not retain conservative quadratic scale: %d => %d", op, previous, cost.work)
			}
			previous = cost.work
		}
	}
	// Coprime Mersenne numerator and power-of-two denominator are large exact
	// fractions. Dividing equal values reduces to one, but cannot skip precharged
	// magnitude-dependent normalization work.
	numerator := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 8192), big.NewInt(1))
	denominator := new(big.Int).Lsh(big.NewInt(1), 8191)
	value, err := NewRational(new(big.Rat).SetFrac(numerator, denominator))
	if err != nil {
		t.Fatal(err)
	}
	s := rationalSession(t, Budgets{})
	result, err := s.rationalQuo(context.Background(), value, value)
	if !errors.Is(err, ErrResourceLimit) || !errors.Is(err, ErrIncompleteAssessment) || result.Valid() {
		t.Fatalf("large cancellation bypassed work cap: %v,%v", result, err)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("large rejection retained scratch")
	}
}
