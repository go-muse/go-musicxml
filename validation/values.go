package validation

import "math/big"

// MaxExactValueBytes is the finite per-value constructor limit, in bytes. It
// independently bounds significant integer magnitude storage (the numerator
// plus denominator for a decimal) and canonical decimal output. Intermediate
// arithmetic is bounded by these inputs and the checked output scale. This
// constructor safety cap is separate from an assessment's scalar/buffer limits.
const MaxExactValueBytes = 1 << 20

// Integer is an immutable exact integer. Its zero value is uninitialized, not
// numeric zero. Initialized values can be copied and shared across goroutines.
type Integer struct {
	value *big.Int
	text  string
	bytes int64
}

// NewInteger copies an existing exact integer without parsing source text.
// Nil is invalid; excessive magnitude or output returns a ResourceError before
// copying or unbounded rendering. Do not mutate value during this call.
func NewInteger(value *big.Int) (Integer, error) {
	if value == nil {
		return Integer{}, &ConfigurationError{Field: "Integer", Reason: "nil exact value"}
	}
	magnitude := magnitudeBytes(value)
	if err := checkExactSize(magnitude); err != nil {
		return Integer{}, err
	}
	text, err := boundedIntegerText(value)
	if err != nil {
		return Integer{}, err
	}
	return Integer{value: new(big.Int).Set(value), text: text, bytes: magnitude + int64(len(text))}, nil
}

// Valid reports whether the integer was initialized by NewInteger.
func (n Integer) Valid() bool { return n.value != nil }

// Value returns an independent mutable copy, or nil for an uninitialized value.
func (n Integer) Value() *big.Int {
	if !n.Valid() {
		return nil
	}
	return new(big.Int).Set(n.value)
}

// Cmp compares initialized exact integers and returns -1, 0, or 1. An
// uninitialized operand is a configuration error, never an implicit zero.
func (n Integer) Cmp(other Integer) (int, error) {
	if !n.Valid() || !other.Valid() {
		return 0, &ConfigurationError{Field: "Integer.Cmp", Reason: "uninitialized operand"}
	}
	return n.value.Cmp(other.value), nil
}

// String returns the bounded canonical base-ten value, or "<uninitialized>".
func (n Integer) String() string {
	if !n.Valid() {
		return "<uninitialized>"
	}
	return n.text
}

// ByteSize returns retained canonical-text and significant-magnitude bytes. It
// excludes fixed Go-object overhead; zero is returned for an uninitialized value.
func (n Integer) ByteSize() int64 { return n.bytes }

// Decimal is an immutable exact finite decimal. Its normalized rational has a
// positive denominator containing only factors 2 and 5. Its zero value is
// uninitialized; initialized values can be copied and shared across goroutines.
type Decimal struct {
	value *big.Rat
	text  string
	bytes int64
}

// NewDecimal copies and normalizes an existing rational without parsing source
// text. Nil, an observably nonpositive denominator, or a nonterminating value is
// invalid. Excessive magnitude or expanded output returns ResourceError. Go's
// zero-value big.Rat is numeric zero. Do not mutate value during this call.
func NewDecimal(value *big.Rat) (Decimal, error) {
	if value == nil {
		return Decimal{}, &ConfigurationError{Field: "Decimal", Reason: "nil exact value"}
	}
	numerator, denominator := value.Num(), value.Denom()
	if denominator.Sign() <= 0 {
		return Decimal{}, &ConfigurationError{Field: "Decimal", Reason: "denominator must be positive"}
	}
	magnitude := magnitudeBytes(numerator) + magnitudeBytes(denominator)
	if err := checkExactSize(magnitude); err != nil {
		return Decimal{}, err
	}
	normalized := new(big.Rat).SetFrac(numerator, denominator)
	scale, finite := decimalScale(normalized.Denom())
	if !finite {
		return Decimal{}, &ConfigurationError{Field: "Decimal", Reason: "value has no finite decimal expansion"}
	}
	// The shortest nonintegral decimal needs "0." plus scale digits. Check it
	// before computing any decimal expansion or a power proportional to scale.
	if scale > 0 {
		if err := checkExactSize(int64(scale) + 2); err != nil {
			return Decimal{}, err
		}
	}
	integerPart := new(big.Int).Quo(normalized.Num(), normalized.Denom())
	integerText, err := boundedIntegerText(integerPart)
	if err != nil {
		return Decimal{}, err
	}
	outputBytes := int64(len(integerText))
	// Integer truncation erases the sign of a value strictly between -1 and 0.
	if normalized.Sign() < 0 && integerPart.Sign() == 0 {
		outputBytes++
	}
	if scale > 0 {
		outputBytes += int64(scale) + 1
	}
	if err := checkExactSize(outputBytes); err != nil {
		return Decimal{}, err
	}
	text := normalized.FloatString(scale)
	magnitude = magnitudeBytes(normalized.Num()) + magnitudeBytes(normalized.Denom())
	return Decimal{value: normalized, text: text, bytes: magnitude + int64(len(text))}, nil
}

// Valid reports whether the decimal was initialized by NewDecimal.
func (n Decimal) Valid() bool { return n.value != nil }

// Value returns an independent mutable rational copy, or nil if uninitialized.
func (n Decimal) Value() *big.Rat {
	if !n.Valid() {
		return nil
	}
	return new(big.Rat).Set(n.value)
}

// Cmp compares initialized exact decimals and returns -1, 0, or 1. An
// uninitialized operand is a configuration error, never an implicit zero.
func (n Decimal) Cmp(other Decimal) (int, error) {
	if !n.Valid() || !other.Valid() {
		return 0, &ConfigurationError{Field: "Decimal.Cmp", Reason: "uninitialized operand"}
	}
	return n.value.Cmp(other.value), nil
}

// String returns the bounded canonical decimal expansion, or "<uninitialized>".
func (n Decimal) String() string {
	if !n.Valid() {
		return "<uninitialized>"
	}
	return n.text
}

// ByteSize returns retained canonical-text and significant numerator/denominator
// bytes, excluding fixed Go-object overhead. Uninitialized values return zero.
func (n Decimal) ByteSize() int64 { return n.bytes }

func magnitudeBytes(value *big.Int) int64 { return (int64(value.BitLen()) + 7) / 8 }

func checkExactSize(bytes int64) error {
	if bytes > MaxExactValueBytes {
		return &ResourceError{Kind: BudgetScalarBytes, Limit: MaxExactValueBytes, Requested: bytes}
	}
	return nil
}

func boundedIntegerText(value *big.Int) (string, error) {
	bits := int64(value.BitLen())
	if bits > 0 {
		// 0.301029995 is a strict lower bound for log10(2). For a magnitude
		// already bounded to 8 Mi bits, this estimate is within two digits. Thus
		// even the boundary conversion below has a finite cap plus two-byte
		// scratch allowance, and grossly oversized output is never rendered.
		minimum := (bits-1)*301029995/1_000_000_000 + 1
		if value.Sign() < 0 {
			minimum++
		}
		if err := checkExactSize(minimum); err != nil {
			return "", err
		}
	}
	text := value.String()
	if err := checkExactSize(int64(len(text))); err != nil {
		return "", err
	}
	return text, nil
}

func decimalScale(denominator *big.Int) (int, bool) {
	twos := denominator.TrailingZeroBits()
	remainder := new(big.Int).Rsh(denominator, twos)
	one := big.NewInt(1)
	if remainder.Cmp(one) == 0 {
		return int(twos), true
	}
	// Squared powers 5^(2^k) keep both the number of divisions and the number
	// of stored factors logarithmic in the exponent. The largest temporary
	// square has at most twice the bounded denominator's significant bits.
	powers := []*big.Int{big.NewInt(5)}
	for powers[len(powers)-1].Cmp(remainder) <= 0 {
		previous := powers[len(powers)-1]
		next := new(big.Int).Mul(previous, previous)
		if next.Cmp(remainder) > 0 {
			break
		}
		powers = append(powers, next)
	}
	fives := 0
	quotient, modulus := new(big.Int), new(big.Int)
	for i := len(powers) - 1; i >= 0; i-- {
		quotient.QuoRem(remainder, powers[i], modulus)
		if modulus.Sign() == 0 {
			remainder.Set(quotient)
			fives += 1 << uint(i)
		}
	}
	if remainder.Cmp(one) != 0 {
		return 0, false
	}
	if int(twos) > fives {
		return int(twos), true
	}
	return fives, true
}
