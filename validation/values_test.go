package validation

import (
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func TestIntegerCopiesAndComparesExactValues(t *testing.T) {
	source, _ := new(big.Int).SetString("900719925474099312345678901234567890", 10)
	want := source.String()
	number, err := NewInteger(source)
	if err != nil {
		t.Fatal(err)
	}
	source.SetInt64(0)
	if !number.Valid() || number.String() != want {
		t.Fatalf("source alias or bad exact value: %s", number)
	}
	returned := number.Value()
	returned.SetInt64(-7)
	if number.String() != want {
		t.Fatal("mutable accessor alias")
	}
	same, _ := NewInteger(number.Value())
	low, _ := NewInteger(new(big.Int).Sub(number.Value(), big.NewInt(1)))
	for _, tc := range []struct {
		a, b Integer
		want int
	}{{number, same, 0}, {number, low, 1}, {low, number, -1}} {
		got, err := tc.a.Cmp(tc.b)
		if err != nil || got != tc.want {
			t.Errorf("Cmp = %d, %v; want %d", got, err, tc.want)
		}
	}
	zero, err := NewInteger(new(big.Int))
	if err != nil || zero.String() != "0" || !zero.Valid() {
		t.Fatalf("initialized zero: %v, %v", zero, err)
	}
}

func TestUninitializedExactValuesRejected(t *testing.T) {
	if _, err := NewInteger(nil); !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("nil integer = %v", err)
	}
	if _, err := NewDecimal(nil); !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("nil decimal = %v", err)
	}
	var integer Integer
	var decimal Decimal
	if integer.Valid() || decimal.Valid() || integer.Value() != nil || decimal.Value() != nil {
		t.Error("zero wrapper accepted")
	}
	if integer.String() != "<uninitialized>" || decimal.String() != "<uninitialized>" {
		t.Error("zero wrapper disguised as numeric zero")
	}
	one, _ := NewInteger(big.NewInt(1))
	fraction, _ := NewDecimal(big.NewRat(1, 2))
	for _, tc := range []struct{ a, b Integer }{{integer, one}, {one, integer}, {integer, integer}} {
		if _, err := tc.a.Cmp(tc.b); !errors.Is(err, ErrInvalidConfiguration) {
			t.Errorf("uninitialized integer Cmp = %v", err)
		}
	}
	for _, tc := range []struct{ a, b Decimal }{{decimal, fraction}, {fraction, decimal}, {decimal, decimal}} {
		if _, err := tc.a.Cmp(tc.b); !errors.Is(err, ErrInvalidConfiguration) {
			t.Errorf("uninitialized decimal Cmp = %v", err)
		}
	}
}

func TestFiniteDecimalCanonicalAndCopied(t *testing.T) {
	for _, tc := range []struct {
		numerator, denominator int64
		want                   string
	}{{0, 1, "0"}, {1, 2, "0.5"}, {1, 5, "0.2"}, {1, 8, "0.125"}, {-1, 40, "-0.025"}, {1000, 8, "125"}, {1200, 1000, "1.2"}, {1, 1024, "0.0009765625"}, {25, 100, "0.25"}} {
		source := big.NewRat(tc.numerator, tc.denominator)
		number, err := NewDecimal(source)
		if err != nil {
			t.Fatal(err)
		}
		source.SetInt64(42)
		if !number.Valid() || number.String() != tc.want {
			t.Errorf("%d/%d = %s want %s", tc.numerator, tc.denominator, number.String(), tc.want)
		}
		copy := number.Value()
		copy.SetInt64(13)
		if number.String() != tc.want {
			t.Error("decimal accessor alias")
		}
		same, _ := NewDecimal(number.Value())
		if got, err := number.Cmp(same); got != 0 || err != nil {
			t.Errorf("decimal equal = %d,%v", got, err)
		}
	}
	left, _ := NewDecimal(big.NewRat(9007199254740993, 10))
	right, _ := NewDecimal(big.NewRat(9007199254740992, 10))
	if got, err := left.Cmp(right); got != 1 || err != nil {
		t.Errorf("exact decimal comparison = %d,%v", got, err)
	}
	if got, err := right.Cmp(left); got != -1 || err != nil {
		t.Errorf("reverse decimal comparison = %d,%v", got, err)
	}
}

func TestDecimalRejectsNonTerminatingAndMalformedRationals(t *testing.T) {
	for _, denominator := range []int64{3, 6, 7, 11, 15, 21, 101} {
		if _, err := NewDecimal(big.NewRat(1, denominator)); !errors.Is(err, ErrInvalidConfiguration) {
			t.Errorf("denominator %d accepted: %v", denominator, err)
		}
	}
	for _, denominator := range []int64{-2} {
		value := big.NewRat(1, 2)
		value.Denom().SetInt64(denominator)
		if _, err := NewDecimal(value); !errors.Is(err, ErrInvalidConfiguration) {
			t.Errorf("malformed denominator %d = %v", denominator, err)
		}
	}
	// big.Rat exposes mutable numerator/denominator pointers: normalize a valid
	// positive, unreduced rational before deciding whether its value terminates.
	raw := big.NewRat(1, 2)
	raw.Num().SetInt64(6)
	raw.Denom().SetInt64(15)
	number, err := NewDecimal(raw)
	if err != nil || number.String() != "0.4" {
		t.Errorf("unreduced decimal = %s,%v", number, err)
	}
}

func TestExactConstructorsRejectOversizeBeforeCopyOrExpansion(t *testing.T) {
	huge := new(big.Int).Lsh(big.NewInt(1), 8*MaxExactValueBytes)
	if _, err := NewInteger(huge); !errors.Is(err, ErrResourceLimit) {
		t.Errorf("oversize integer = %v", err)
	}
	if _, err := NewDecimal(new(big.Rat).SetInt(huge)); !errors.Is(err, ErrResourceLimit) {
		t.Errorf("oversize decimal numerator = %v", err)
	}
	// Storage can fit the magnitude cap while canonical integer output does not.
	wide := new(big.Int).Lsh(big.NewInt(1), 4*MaxExactValueBytes)
	if _, err := NewInteger(wide); !errors.Is(err, ErrResourceLimit) {
		t.Errorf("wide integer output = %v", err)
	}
	if _, err := NewDecimal(new(big.Rat).SetInt(wide)); !errors.Is(err, ErrResourceLimit) {
		t.Errorf("wide decimal integer output = %v", err)
	}
	// Its denominator fits the byte cap, but a terminating decimal expansion
	// needs more than the cap. Refuse it before generating the expansion.
	denominator := new(big.Int).Lsh(big.NewInt(1), 2*MaxExactValueBytes)
	if _, err := NewDecimal(new(big.Rat).SetFrac(big.NewInt(1), denominator)); !errors.Is(err, ErrResourceLimit) {
		t.Errorf("oversize expansion = %v", err)
	}
}

func TestDecimalLargeFivePowerAndNonDecimalFactor(t *testing.T) {
	denominator := new(big.Int).Exp(big.NewInt(5), big.NewInt(16384), nil)
	finite, err := NewDecimal(new(big.Rat).SetFrac(big.NewInt(1), denominator))
	if err != nil {
		t.Fatal(err)
	}
	if finite.String() == "" || finite.Value().Cmp(new(big.Rat).SetFrac(big.NewInt(1), denominator)) != 0 {
		t.Error("large finite value changed")
	}
	denominator.Mul(denominator, big.NewInt(7))
	if _, err := NewDecimal(new(big.Rat).SetFrac(big.NewInt(1), denominator)); !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("nondecimal factor accepted: %v", err)
	}
}

func TestExactValueByteSizeSupportsBoundedRetention(t *testing.T) {
	integer, _ := NewInteger(big.NewInt(256))
	if integer.ByteSize() != 5 {
		t.Errorf("integer bytes = %d want 5", integer.ByteSize())
	}
	decimal, _ := NewDecimal(big.NewRat(1, 2))
	if decimal.ByteSize() != 5 {
		t.Errorf("decimal bytes = %d want 5", decimal.ByteSize())
	}
	if (Integer{}).ByteSize() != 0 || (Decimal{}).ByteSize() != 0 {
		t.Error("zero wrapper has retained numeric payload")
	}
	zero, err := NewDecimal(new(big.Rat))
	if err != nil || zero.String() != "0" {
		t.Errorf("big.Rat zero value = %s,%v", zero, err)
	}
}

func TestDecimalExactOutputBoundaryAndNegativeSign(t *testing.T) {
	denominator := new(big.Int).Lsh(big.NewInt(1), MaxExactValueBytes-2)
	value := new(big.Rat).SetFrac(big.NewInt(1), denominator)
	number, err := NewDecimal(value)
	if err != nil {
		t.Fatal(err)
	}
	if len(number.String()) != MaxExactValueBytes {
		t.Fatalf("boundary output = %d bytes", len(number.String()))
	}
	value.Neg(value)
	if _, err := NewDecimal(value); !errors.Is(err, ErrResourceLimit) {
		t.Errorf("sign exceeds exact boundary: %v", err)
	}
}

func TestDecimalFactorPropertyAndCopiedComparisons(t *testing.T) {
	for twos := 0; twos <= 16; twos++ {
		for fives := 0; fives <= 16; fives++ {
			denominator := new(big.Int).Exp(big.NewInt(5), big.NewInt(int64(fives)), nil)
			denominator.Lsh(denominator, uint(twos))
			for _, numerator := range []int64{-125, -7, 0, 1, 20, 113} {
				rational := new(big.Rat).SetFrac(big.NewInt(numerator), denominator)
				decimal, err := NewDecimal(rational)
				if err != nil {
					t.Fatal(err)
				}
				rendered, ok := new(big.Rat).SetString(decimal.String())
				if !ok || rendered.Cmp(rational) != 0 {
					t.Fatalf("2^%d*5^%d numerator %d changed value", twos, fives, numerator)
				}
				before := decimal.String()
				other, _ := NewDecimal(big.NewRat(3, 4))
				got, err := decimal.Cmp(other)
				if err != nil || got != rational.Cmp(big.NewRat(3, 4)) || decimal.String() != before {
					t.Fatalf("comparison changed value or order: %d,%v", got, err)
				}
			}
		}
	}
}

func BenchmarkNewInteger(b *testing.B) {
	value := new(big.Int).Lsh(big.NewInt(1), 255)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewInteger(value); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewDecimalFivePower(b *testing.B) {
	denominator := new(big.Int).Exp(big.NewInt(5), big.NewInt(16384), nil)
	value := new(big.Rat).SetFrac(big.NewInt(1), denominator)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewDecimal(value); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRejectOversizedExactMagnitude(b *testing.B) {
	value := new(big.Int).Lsh(big.NewInt(1), 8*MaxExactValueBytes)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewInteger(value); !errors.Is(err, ErrResourceLimit) {
			b.Fatal(err)
		}
	}
}

func TestIntegerExactOutputBoundaryAndNegativeSign(t *testing.T) {
	value := new(big.Int).Exp(big.NewInt(10), big.NewInt(MaxExactValueBytes-1), nil)
	number, err := NewInteger(value)
	if err != nil {
		t.Fatal(err)
	}
	if len(number.String()) != MaxExactValueBytes {
		t.Fatalf("boundary integer = %d bytes", len(number.String()))
	}
	value.Mul(value, big.NewInt(10))
	if _, err := NewInteger(value); !errors.Is(err, ErrResourceLimit) {
		t.Errorf("one extra integer digit = %v", err)
	}
	value.Quo(value, big.NewInt(10))
	value.Neg(value)
	if _, err := NewInteger(value); !errors.Is(err, ErrResourceLimit) {
		t.Errorf("integer sign exceeds exact boundary: %v", err)
	}
}

func TestImmutableNumbersSupportConcurrentReads(t *testing.T) {
	integer, _ := NewInteger(big.NewInt(9007199254740993))
	decimal, _ := NewDecimal(big.NewRat(7, 20))
	failures := make(chan string, 8)
	for worker := 0; worker < 8; worker++ {
		go func() {
			for i := 0; i < 100; i++ {
				integer.Value().SetInt64(-1)
				decimal.Value().SetInt64(-1)
				if got, err := integer.Cmp(integer); got != 0 || err != nil || integer.String() != "9007199254740993" {
					failures <- "integer"
					return
				}
				if got, err := decimal.Cmp(decimal); got != 0 || err != nil || decimal.String() != "0.35" {
					failures <- "decimal"
					return
				}
			}
			failures <- ""
		}()
	}
	for worker := 0; worker < 8; worker++ {
		if failure := <-failures; failure != "" {
			t.Error("concurrent immutable", failure)
		}
	}
}

func TestExactValuesAreNotComparable(t *testing.T) {
	for _, value := range []any{Integer{}, Decimal{}, Rational{}, Atomic{}, ContextPosition{}} {
		if reflect.TypeOf(value).Comparable() {
			t.Errorf("%T is comparable with ==; it must use Equal or Cmp", value)
		}
	}
}
