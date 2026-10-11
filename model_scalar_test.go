package musicxml

import (
	"context"
	"math"
	"math/big"
	"reflect"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

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
		{"tenth", float64(.1), modelDecimal, "3602879701896397/36028797018963968"},
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
		if got.scalar.Value.Value.Decimal.Value().Cmp(new(big.Rat).SetFloat64(value)) != 0 {
			t.Error("finite boundary changed")
		}
		a.Release(validation.BudgetBufferedBytes, got.retained)
	}
}
