package validation

import (
	"errors"
	"math/big"
	"testing"
)

func TestNumericWrappersEqualAndCmp(t *testing.T) {
	t.Parallel()

	third, err := NewRational(big.NewRat(1, 3))
	if err != nil {
		t.Fatal(err)
	}
	twoSixths, err := NewRational(big.NewRat(2, 6))
	if err != nil {
		t.Fatal(err)
	}
	half, err := NewRational(big.NewRat(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if !third.Equal(twoSixths) || third.Equal(half) {
		t.Fatalf("Rational.Equal: 1/3 vs 2/6 = %v, 1/3 vs 1/2 = %v", third.Equal(twoSixths), third.Equal(half))
	}
	if got, err := third.Cmp(half); err != nil || got != -1 {
		t.Fatalf("Rational.Cmp(1/3, 1/2) = %d, %v", got, err)
	}
	if got, err := half.Cmp(third); err != nil || got != 1 {
		t.Fatalf("Rational.Cmp(1/2, 1/3) = %d, %v", got, err)
	}
	if got, err := third.Cmp(twoSixths); err != nil || got != 0 {
		t.Fatalf("Rational.Cmp(1/3, 2/6) = %d, %v", got, err)
	}
	if _, err := third.Cmp(Rational{}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("Rational.Cmp with uninitialized operand = %v", err)
	}
	if !(Rational{}).Equal(Rational{}) || third.Equal(Rational{}) || (Rational{}).Equal(third) {
		t.Fatal("Rational.Equal must treat only two uninitialized values as equal")
	}

	one, err := NewInteger(big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	otherOne, err := NewInteger(big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	two, err := NewInteger(big.NewInt(2))
	if err != nil {
		t.Fatal(err)
	}
	if !one.Equal(otherOne) || one.Equal(two) || one.Equal(Integer{}) || !(Integer{}).Equal(Integer{}) {
		t.Fatal("Integer.Equal compares exact values and uninitialized state")
	}

	decimalOne, err := NewDecimal(big.NewRat(10, 10))
	if err != nil {
		t.Fatal(err)
	}
	otherDecimalOne, err := NewDecimal(big.NewRat(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	decimalHalf, err := NewDecimal(big.NewRat(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if !decimalOne.Equal(otherDecimalOne) || decimalOne.Equal(decimalHalf) ||
		decimalOne.Equal(Decimal{}) || !(Decimal{}).Equal(Decimal{}) {
		t.Fatal("Decimal.Equal compares exact values and uninitialized state")
	}
}
