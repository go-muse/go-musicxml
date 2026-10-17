package validation

import (
	"context"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"
)

func TestStateMeasureGuardInterfaceNilAndJointFrontiers(t *testing.T) {
	number, err := NewInteger(big.NewInt(123))
	if err != nil {
		t.Fatal(err)
	}
	var nilInteger *Integer
	var nilDecimal *Decimal
	var nilRational *Rational
	var nilCause *DecisionRef
	var method interface{ ByteSize() int64 } = nilInteger
	fixtures := []struct {
		name  string
		value any
	}{
		{"integer-typed-nil", nilInteger}, {"decimal-typed-nil", nilDecimal}, {"rational-typed-nil", nilRational},
		{"allowed-pointer-typed-nil", nilCause}, {"integer-pointer", &number}, {"method-pointer-nil", method},
		{"embedded-number", struct{ Integer }{number}},
		{"private-numeric-zero", struct{ number Integer }{}},
	}
	failure := errors.New("independent exact cancellation")
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			wrapped := fixture.value
			for _, value := range []reflect.Value{reflect.ValueOf(fixture.value), reflect.ValueOf(&wrapped).Elem()} {
				probe := stateMeasureGuardCompare(t, value, 0, math.MaxInt, failure, false)
				for limit := int64(1); limit <= probe.Usage[9]+1; limit++ {
					for at := 1; at <= probe.Calls+1; at++ {
						stateMeasureGuardCompare(t, value, limit, at, failure, false)
					}
				}
				for _, fn := range []stateMeasureGuardFunc{legacyStateMeasureForGuard, stateMeasure} {
					s := rationalSession(t, Budgets{MaxRuleSteps: math.MaxInt64})
					s.account.usage[9] = math.MaxInt64
					size, err := fn(context.Background(), s, value)
					if size != 0 || !errors.Is(err, ErrResourceLimit) || s.account.usage[9] != math.MaxInt64 {
						t.Fatalf("overflow admission changed: size=%d err=%v usage=%d", size, err, s.account.usage[9])
					}
				}
			}
		})
	}
	for _, ptr := range []any{nilInteger, nilDecimal, nilRational, nilCause} {
		wrapped := ptr
		for _, fn := range []stateMeasureGuardFunc{legacyStateMeasureForGuard, stateMeasure} {
			s := rationalSession(t, Budgets{})
			size, err := fn(context.Background(), s, reflect.ValueOf(ptr))
			if size != 32 || err != nil {
				t.Fatalf("direct typed nil size=%d error=%v", size, err)
			}
			s = rationalSession(t, Budgets{})
			size, err = fn(context.Background(), s, reflect.ValueOf(&wrapped).Elem())
			if size != 0 || !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("interface typed nil size=%d error=%v", size, err)
			}
		}
	}
}

// The fast guard intentionally spells out this closed set rather than paying
// for a map lookup per struct. A registry addition must update the guard and this
// contract together; it cannot silently pass the existing three-wrapper tests.
func TestStateMeasureNumericRegistryClosedSet(t *testing.T) {
	want := map[reflect.Type]bool{
		reflect.TypeFor[Integer]():  true,
		reflect.TypeFor[Decimal]():  true,
		reflect.TypeFor[Rational](): true,
	}
	if len(resourceNumericFields) != len(want) {
		t.Fatalf("numeric registry has %d entries, want %d; review stateMeasure guard", len(resourceNumericFields), len(want))
	}
	for typ, byteField := range resourceNumericFields {
		value := reflect.Zero(typ)
		wrapped := value.Interface()
		for _, input := range []reflect.Value{value, reflect.ValueOf(&wrapped).Elem()} {
			s := rationalSession(t, Budgets{MaxRuleSteps: 1})
			size, err := stateMeasure(context.Background(), s, input)
			if err != nil || size != value.Field(byteField).Int()+64 || s.account.usage[9] != 1 {
				t.Fatalf("registry type %v is not a terminal numeric value: size=%d err=%v steps=%d", typ, size, err, s.account.usage[9])
			}
		}
		if !want[typ] {
			t.Fatalf("unhandled numeric registry type %v; update stateMeasure guard", typ)
		}
	}
	for typ := range want {
		if _, ok := resourceNumericFields[typ]; !ok {
			t.Fatalf("numeric wrapper %v missing from registry", typ)
		}
	}
}
