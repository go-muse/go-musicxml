package xsd

import (
	"reflect"
	"slices"
	"testing"
)

// isFreshResult and isZeroScalar name every field of the results they
// compact. A new field must be added there too, or a completed subject would
// drop it; this pin fails until it is.
func TestCompactedResultFieldsArePinned(t *testing.T) {
	names := func(v any) []string {
		var out []string
		for i := range reflect.TypeOf(v).NumField() {
			out = append(out, reflect.TypeOf(v).Field(i).Name)
		}
		return out
	}
	for _, tc := range []struct {
		value any
		want  []string
	}{
		{Result{}, []string{"Applicable", "Valid", "Complete", "Issues", "Limitations"}},
		{ScalarResult{}, []string{"Known", "Complete", "Defaulted", "Lexeme", "EffectiveLexeme", "Builtin", "Value", "Atomic"}},
		{SimpleValue{}, []string{"Domain", "Primitive", "Boolean", "Text", "Number", "Items"}},
	} {
		if got := names(tc.value); !slices.Equal(got, tc.want) {
			t.Errorf("%T fields %v, want %v", tc.value, got, tc.want)
		}
	}
	for _, v := range []ScalarResult{{Known: true}, {Lexeme: "x"}, {Value: SimpleValue{Items: []SimpleValue{}}}, {Value: SimpleValue{Number: Decimal{Negative: true}}}} {
		if isZeroScalar(&v) {
			t.Errorf("nonzero scalar %+v reads as zero", v)
		}
	}
	fresh := freshResults().Binding
	if !isFreshResult(&fresh) {
		t.Fatal("fresh result")
	}
	for _, r := range []Result{{Applicable: true, Valid: true, Complete: true, Issues: []Detail{}}, {Applicable: true, Valid: true}} {
		if isFreshResult(&r) {
			t.Errorf("nonfresh result %+v reads as fresh", r)
		}
	}
}
