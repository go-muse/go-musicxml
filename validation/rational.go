package validation

import (
	"context"
	"math"
	"math/big"
	"strconv"
)

// Rational is an immutable exact fraction, distinct from the terminating
// Decimal and the XSD Atomic value space. Its zero value is uninitialized.
// Initialized values may be copied and shared across goroutines.
type Rational struct {
	value *big.Rat
	text  string
	bytes int64
}

// NewRational copies and reduces an exact fraction with a positive denominator.
// Nil or an observably nonpositive denominator is invalid. The finite
// MaxExactValueBytes cap applies independently to significant numerator plus
// denominator magnitude and canonical fraction text, including pre-normalization
// inputs. Excesses return ResourceError, even if reduction could make them small.
// Do not mutate value during this call. Session arithmetic has additional shared
// work, scalar and live-buffer accounting; this constructor is not that service.
func NewRational(value *big.Rat) (Rational, error) {
	if value == nil {
		return Rational{}, rationalConfiguration("nil exact value")
	}
	if value.Denom().Sign() <= 0 {
		return Rational{}, rationalConfiguration("denominator must be positive")
	}
	magnitude, err := rationalCheckedAdd(BudgetScalarBytes, magnitudeBytes(value.Num()), magnitudeBytes(value.Denom()))
	if err != nil {
		return Rational{}, err
	}
	if err = checkExactSize(magnitude); err != nil {
		return Rational{}, err
	}
	// Refuse excessive raw text growth before normalization/copying. The lower
	// digit bound is within two digits per integer at the static magnitude cap.
	if err = rationalTextPrecheck(value); err != nil {
		return Rational{}, err
	}
	normalized := new(big.Rat).SetFrac(value.Num(), value.Denom())
	text, err := boundedRationalText(normalized)
	if err != nil {
		return Rational{}, err
	}
	return rationalOwned(normalized, text), nil
}

// Valid reports whether the fraction was initialized by NewRational or a
// successful exact-arithmetic service.
func (r Rational) Valid() bool { return r.value != nil }

// Value returns an independent mutable copy, or nil when uninitialized.
func (r Rational) Value() *big.Rat {
	if !r.Valid() {
		return nil
	}
	return new(big.Rat).Set(r.value)
}

// String returns a bounded reduced fraction with a positive denominator, an
// integer when its denominator is one, or "<uninitialized>".
func (r Rational) String() string {
	if !r.Valid() {
		return "<uninitialized>"
	}
	return r.text
}

// ByteSize returns canonical-text plus significant numerator/denominator bytes,
// excluding fixed Go-object overhead. Uninitialized values return zero.
func (r Rational) ByteSize() int64 { return r.bytes }

func rationalConfiguration(reason string) error {
	return &ConfigurationError{Field: "Rational", Reason: reason}
}
func rationalOwned(value *big.Rat, text string) Rational {
	return Rational{value: value, text: text, bytes: magnitudeBytes(value.Num()) + magnitudeBytes(value.Denom()) + int64(len(text))}
}
func rationalTextPrecheck(value *big.Rat) error {
	minimum := rationalMinimumDigits(value.Num())
	if !value.IsInt() {
		minimum += 1 + rationalMinimumDigits(value.Denom())
	}
	return checkExactSize(minimum)
}
func rationalMinimumDigits(value *big.Int) int64 {
	bits := int64(value.BitLen())
	if bits == 0 {
		return 1
	}
	// Called only after significant magnitude is bounded to MaxExactValueBytes.
	n := (bits-1)*301029995/1_000_000_000 + 1
	if value.Sign() < 0 {
		n++
	}
	return n
}
func boundedRationalText(value *big.Rat) (string, error) {
	if err := rationalTextPrecheck(value); err != nil {
		return "", err
	}
	n, err := boundedIntegerText(value.Num())
	if err != nil {
		return "", err
	}
	if value.IsInt() {
		return n, nil
	}
	d, err := boundedIntegerText(value.Denom())
	if err != nil {
		return "", err
	}
	if err = checkExactSize(int64(len(n)) + 1 + int64(len(d))); err != nil {
		return "", err
	}
	return n + "/" + d, nil
}

type rationalOperation uint8

const (
	rationalCopyOp rationalOperation = iota
	rationalAddOp
	rationalSubOp
	rationalMulOp
	rationalQuoOp
	rationalCmpOp
)

type rationalWidths struct{ num, den int64 }
type rationalCost struct{ scalar, scratch, work int64 }

func rationalWidthsOf(v *big.Rat) rationalWidths {
	return rationalWidths{int64(v.Num().BitLen()), int64(v.Denom().BitLen())}
}

func rationalCheckedAdd(kind BudgetKind, a, b int64) (int64, error) {
	if a < 0 || b < 0 {
		return 0, rationalConfiguration("negative size")
	}
	if a > math.MaxInt64-b {
		return 0, &ResourceError{Kind: kind, Limit: math.MaxInt64, Used: a, Requested: b}
	}
	return a + b, nil
}
func rationalCheckedMul(kind BudgetKind, a, b int64) (int64, error) {
	if a < 0 || b < 0 {
		return 0, rationalConfiguration("negative size")
	}
	if b != 0 && a > math.MaxInt64/b {
		return 0, &ResourceError{Kind: kind, Limit: math.MaxInt64, Used: a, Requested: b}
	}
	return a * b, nil
}
func rationalSum(kind BudgetKind, values ...int64) (int64, error) {
	var sum int64
	for _, n := range values {
		var err error
		sum, err = rationalCheckedAdd(kind, sum, n)
		if err != nil {
			return 0, err
		}
	}
	return sum, nil
}
func rationalAllocation(bytes int64) (int, error) { return AllocationSize(bytes) }
func rationalMagnitude(bits int64) int64 {
	n := bits / 8
	if bits%8 != 0 {
		n++
	}
	return n
}
func rationalUpperDigits(bits int64) (int64, error) {
	if bits == 0 {
		return 1, nil
	}
	n, err := rationalCheckedMul(BudgetScalarBytes, bits, 30103)
	if err != nil {
		return 0, err
	}
	n, err = rationalCheckedAdd(BudgetScalarBytes, n, 99999)
	if err != nil {
		return 0, err
	}
	return n / 100000, nil
}

// rationalPlan bounds unreduced intermediates, not only the eventual reduced
// result. Add/Sub may create Na*Db +/- Nb*Da and Da*Db. Mul creates Na*Nb and
// Da*Db; Quo swaps the nonzero divisor's widths. Cmp reserves both cross-products.
// Rejecting an oversized intermediate is incomplete assessment, even when the
// eventual mathematical result would be small after cancellation.
func rationalPlan(op rationalOperation, a, b rationalWidths) (rationalCost, error) {
	if a.num < 0 || a.den <= 0 || b.num < 0 || b.den < 0 {
		return rationalCost{}, rationalConfiguration("invalid operand widths")
	}
	if op != rationalCopyOp && b.den == 0 {
		return rationalCost{}, rationalConfiguration("invalid operand widths")
	}
	var n, d int64
	var err error
	switch op {
	case rationalCopyOp:
		n, d = a.num, a.den
	case rationalAddOp, rationalSubOp, rationalCmpOp:
		left, e := rationalCheckedAdd(BudgetScalarBytes, a.num, b.den)
		if e != nil {
			return rationalCost{}, e
		}
		right, e := rationalCheckedAdd(BudgetScalarBytes, b.num, a.den)
		if e != nil {
			return rationalCost{}, e
		}
		n = max(left, right)
		if op == rationalCmpOp {
			d = n
		} else {
			n, err = rationalCheckedAdd(BudgetScalarBytes, n, 1)
			if err != nil {
				return rationalCost{}, err
			}
			d, err = rationalCheckedAdd(BudgetScalarBytes, a.den, b.den)
		}
	case rationalMulOp:
		n, err = rationalCheckedAdd(BudgetScalarBytes, a.num, b.num)
		if err == nil {
			d, err = rationalCheckedAdd(BudgetScalarBytes, a.den, b.den)
		}
	case rationalQuoOp:
		if b.num == 0 {
			return rationalCost{}, rationalConfiguration("division by zero")
		}
		n, err = rationalCheckedAdd(BudgetScalarBytes, a.num, b.den)
		if err == nil {
			d, err = rationalCheckedAdd(BudgetScalarBytes, a.den, b.num)
		}
	default:
		return rationalCost{}, rationalConfiguration("unknown arithmetic operation")
	}
	if err != nil {
		return rationalCost{}, err
	}
	magnitude, err := rationalSum(BudgetScalarBytes, rationalMagnitude(n), rationalMagnitude(d))
	if err != nil {
		return rationalCost{}, err
	}
	if err = checkExactSize(magnitude); err != nil {
		return rationalCost{}, err
	}
	nd, err := rationalUpperDigits(n)
	if err != nil {
		return rationalCost{}, err
	}
	dd, err := rationalUpperDigits(d)
	if err != nil {
		return rationalCost{}, err
	}
	// Include a possible minus sign. A one-bit denominator can only be one.
	text := nd + 1
	if d > 1 {
		text, err = rationalSum(BudgetScalarBytes, nd, dd, 2)
		if err != nil {
			return rationalCost{}, err
		}
	}
	if err = checkExactSize(text); err != nil {
		return rationalCost{}, err
	}
	scalar, err := rationalCheckedAdd(BudgetScalarBytes, magnitude, text)
	if err != nil {
		return rationalCost{}, err
	}
	totalBits, err := rationalSum(BudgetRuleSteps, a.num, a.den, b.num, b.den, n, d, text, 1)
	if err != nil {
		return rationalCost{}, err
	}
	// A conservative quadratic charge covers the operand arithmetic, Rat's
	// internal GCD/normalization, copying and decimal integer rendering. Charge
	// it before any big-number operation, including comparison and conversion.
	work, err := rationalCheckedMul(BudgetRuleSteps, totalBits, totalBits)
	if err != nil {
		return rationalCost{}, err
	}
	work, err = rationalCheckedMul(BudgetRuleSteps, work, 4)
	if err != nil {
		return rationalCost{}, err
	}
	live, err := rationalSum(BudgetBufferedBytes, rationalMagnitude(a.num), rationalMagnitude(a.den), rationalMagnitude(b.num), rationalMagnitude(b.den), magnitude, text, 1)
	if err != nil {
		return rationalCost{}, err
	}
	// Conservatively cover nat capacity/word rounding, cross-products, GCD and
	// division workspaces, temporary rendering strings and the pending output.
	// The fixed allowance includes Go object and small-word overhead.
	scratch, err := rationalCheckedMul(BudgetBufferedBytes, live, 64)
	if err != nil {
		return rationalCost{}, err
	}
	scratch, err = rationalCheckedAdd(BudgetBufferedBytes, scratch, 4096)
	if err != nil {
		return rationalCost{}, err
	}
	if _, err = rationalAllocation(scratch); err != nil {
		return rationalCost{}, err
	}
	return rationalCost{scalar: scalar, scratch: scratch, work: work}, nil
}

func rationalPoll(ctx context.Context) error {
	if ctx == nil {
		return rationalConfiguration("nil context")
	}
	if err := ctx.Err(); err != nil {
		return &IncompleteError{Reason: "exact-rational-arithmetic", Cause: err}
	}
	return nil
}

// rationalReserve checks all quotas before arithmetic and lends scratch from the
// shared account. It does not add scratch to Session.retained: callers release
// it on every exit. math/big calls are synchronous and cannot be interrupted in
// flight; finite operand/work bounds and before/after polling bound that honest
// cooperative-cancellation limitation, rather than promising preemption.
func (s *Session) rationalReserve(ctx context.Context, op rationalOperation, a, b *big.Rat, aBytes, bBytes int64) (func(), error) {
	if err := rationalPoll(ctx); err != nil {
		return nil, err
	}
	if s == nil || s.account == nil {
		return nil, rationalConfiguration("uninitialized session account")
	}
	if a == nil || (op != rationalCopyOp && b == nil) {
		return nil, rationalConfiguration("uninitialized operand")
	}
	if op == rationalQuoOp && b.Sign() == 0 {
		return nil, rationalConfiguration("division by zero")
	}
	aw, bw := rationalWidthsOf(a), rationalWidths{}
	if b != nil {
		bw = rationalWidthsOf(b)
	}
	cost, err := rationalPlan(op, aw, bw)
	if err != nil {
		return nil, err
	}
	for _, size := range []int64{aBytes, bBytes, cost.scalar} {
		if err = s.account.Check(BudgetScalarBytes, size); err != nil {
			return nil, err
		}
	}
	if err = s.step(ctx, cost.work); err != nil {
		if ctx.Err() != nil {
			return nil, &IncompleteError{Reason: "exact-rational-arithmetic", Cause: err}
		}
		return nil, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, cost.scratch); err != nil {
		return nil, err
	}
	return func() { _ = s.account.Release(BudgetBufferedBytes, cost.scratch) }, nil
}

// rationalResult transfers the immutable output to this session's retention
// ownership until Finish. Callers retain container overhead separately, but
// must not charge the numeric payload again. Scratch still covers the pending
// output if rendering, cancellation or output retention fails. Actual nat
// capacities (including excess capacity after reduction) remain charged.
func (s *Session) rationalResult(ctx context.Context, value *big.Rat) (Rational, error) {
	if err := rationalPoll(ctx); err != nil {
		return Rational{}, err
	}
	text, err := boundedRationalText(value)
	if err != nil {
		return Rational{}, err
	}
	if err = rationalPoll(ctx); err != nil {
		return Rational{}, err
	}
	result := rationalOwned(value, text)
	if err = s.account.Check(BudgetScalarBytes, result.ByteSize()); err != nil {
		return Rational{}, err
	}
	capacity, err := rationalSum(BudgetBufferedBytes, int64(cap(value.Num().Bits())), int64(cap(value.Denom().Bits())))
	if err != nil {
		return Rational{}, err
	}
	capacity, err = rationalCheckedMul(BudgetBufferedBytes, capacity, strconv.IntSize/8)
	if err != nil {
		return Rational{}, err
	}
	retained, err := rationalSum(BudgetBufferedBytes, capacity, int64(len(text)), 128)
	if err != nil {
		return Rational{}, err
	}
	if _, err = rationalAllocation(retained); err != nil {
		return Rational{}, err
	}
	if err = s.retain(retained); err != nil {
		return Rational{}, err
	}
	return result, nil
}
func (s *Session) rationalFromDecimal(ctx context.Context, value Decimal) (Rational, error) {
	release, err := s.rationalReserve(ctx, rationalCopyOp, value.value, nil, value.ByteSize(), 0)
	if err != nil {
		return Rational{}, err
	}
	defer release()
	return s.rationalResult(ctx, new(big.Rat).Set(value.value))
}
func (s *Session) rationalArithmetic(ctx context.Context, op rationalOperation, a, b Rational) (Rational, error) {
	release, err := s.rationalReserve(ctx, op, a.value, b.value, a.ByteSize(), b.ByteSize())
	if err != nil {
		return Rational{}, err
	}
	defer release()
	value := new(big.Rat)
	switch op {
	case rationalAddOp:
		value.Add(a.value, b.value)
	case rationalSubOp:
		value.Sub(a.value, b.value)
	case rationalMulOp:
		value.Mul(a.value, b.value)
	case rationalQuoOp:
		value.Quo(a.value, b.value)
	default:
		return Rational{}, rationalConfiguration("unknown arithmetic operation")
	}
	return s.rationalResult(ctx, value)
}
func (s *Session) rationalAdd(ctx context.Context, a, b Rational) (Rational, error) {
	return s.rationalArithmetic(ctx, rationalAddOp, a, b)
}
func (s *Session) rationalSub(ctx context.Context, a, b Rational) (Rational, error) {
	return s.rationalArithmetic(ctx, rationalSubOp, a, b)
}
func (s *Session) rationalMul(ctx context.Context, a, b Rational) (Rational, error) {
	return s.rationalArithmetic(ctx, rationalMulOp, a, b)
}
func (s *Session) rationalQuo(ctx context.Context, a, b Rational) (Rational, error) {
	return s.rationalArithmetic(ctx, rationalQuoOp, a, b)
}
func (s *Session) rationalCmp(ctx context.Context, a, b Rational) (int, error) {
	release, err := s.rationalReserve(ctx, rationalCmpOp, a.value, b.value, a.ByteSize(), b.ByteSize())
	if err != nil {
		return 0, err
	}
	defer release()
	result := a.value.Cmp(b.value)
	if err = rationalPoll(ctx); err != nil {
		return 0, err
	}
	return result, nil
}
