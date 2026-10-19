package musicxml

import (
	"context"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Variables, so that the sum is computed in binary64 rather than as an exact
// constant expression.
var tenth, fifth = .1, .2

func TestDirectModelScalarExactCurrentValues(t *testing.T) {
	tests := []struct {
		name  string
		value any
		kind  modelScalarKind
		want  string
	}{
		{"signed-min", int64(math.MinInt64), modelSigned, "-9223372036854775808"},
		{"unsigned-max", uint64(math.MaxUint64), modelUnsigned, "18446744073709551615"},
		{"half", float64(.5), modelDecimal, "1/2"},
		{"tenth", float64(.1), modelDecimal, "1/10"},
		{"sum", tenth + fifth, modelDecimal, "7500000000000001/25000000000000000"},
		{"negative-zero", math.Copysign(0, -1), modelDecimal, "0"},
		{"current-string", " 17 ", modelString, " 17 "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _ := validation.NewBudgetAccount(validation.Budgets{})
			got, err := readModelScalar(context.Background(), a, reflect.ValueOf(tt.value), tt.kind)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Release(validation.BudgetBufferedBytes, got.retained)
			if got.problem != "" {
				t.Fatal(got.problem)
			}
			if got.scalar.Lexeme.State != validation.EvidenceUnsupported {
				t.Fatal("fabricated lexeme")
			}
			v := got.scalar.Value.Value
			actual := v.Text
			if v.Kind == validation.AtomicInteger {
				actual = v.Integer.String()
			}
			if v.Kind == validation.AtomicDecimal {
				actual = v.Decimal.Value().RatString()
			}
			if actual != tt.want {
				t.Errorf("got %q, want %q", actual, tt.want)
			}
		})
	}
	for _, value := range []float64{math.MaxFloat64, math.SmallestNonzeroFloat64} {
		a, _ := validation.NewBudgetAccount(validation.Budgets{})
		got, err := readModelScalar(context.Background(), a, reflect.ValueOf(value), modelDecimal)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := new(big.Rat).SetString(strconv.FormatFloat(value, 'f', -1, 64))
		if got.scalar.Value.Value.Decimal.Value().Cmp(want) != 0 {
			t.Error("finite boundary is not its shortest decimal")
		}
		a.Release(validation.BudgetBufferedBytes, got.retained)
	}
}

// The bound covers the shortest decimal of every binary64 class: zero, signs,
// subnormals, the largest finite values and ordinary fractions.
func TestShortestDecimalBoundCoversFormatFloat(t *testing.T) {
	values := []float64{0, math.Copysign(0, -1), math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64, math.MaxFloat64, -math.MaxFloat64, .1, .1 + .2, 1, 10, 123.456, 1e-7, 9.999999999999999e22, 1e300, 2.2250738585072014e-308}
	for exponent := -330; exponent <= 310; exponent++ {
		values = append(values, math.Pow(10, float64(exponent)), math.Nextafter(math.Pow(10, float64(exponent)), 0), -1.2345678901234567*math.Pow(10, float64(exponent)))
	}
	for _, value := range values {
		if math.IsInf(value, 0) {
			continue
		}
		if text := strconv.FormatFloat(value, 'f', -1, 64); len(text) > shortestDecimalBound(value) {
			t.Errorf("%v: %d bytes, bound %d", value, len(text), shortestDecimalBound(value))
		}
	}
}
