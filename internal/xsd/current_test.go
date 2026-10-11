package xsd

import (
	"context"
	"testing"
)

func currentNumber(t *testing.T, text string) CurrentScalar {
	t.Helper()
	n, ok := ParseDecimal(text)
	if !ok {
		t.Fatal(text)
	}
	return CurrentScalar{Known: true, Value: SimpleValue{Domain: NumberDomain, Number: n}}
}
func bindCurrent(t *testing.T, typ TypeRef, value CurrentScalar, fixed *string) (*Run, Binding, *testMeter) {
	t.Helper()
	name := QName{Local: "n"}
	compiled, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{name: {Name: name, Type: typ, Fixed: fixed}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	meter := &testMeter{}
	run, err := NewRun(compiled, meter)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := run.Close(); err != nil {
			t.Error(err)
		}
		if meter.retained != 0 {
			t.Errorf("retained=%d", meter.retained)
		}
	})
	use, _ := compiled.Root(name)
	bound, err := run.Bind(context.Background(), use, Subject{ID: 1, Name: name, Path: "N", CurrentModel: true, Current: value, ChildrenOrderKnown: true, ChildrenComplete: true, AttributesComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	return run, bound, meter
}
func currentRestriction(base string) *SimpleSchema {
	return &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: base}}}
}
func TestCurrentExactValues(t *testing.T) {
	cases := []struct {
		name, builtin, value string
		valid                bool
	}{
		{"zero", "integer", "0", true}, {"large integer", "integer", "9007199254740993", true}, {"unsigned max", "unsignedLong", "18446744073709551615", true}, {"unsigned overflow", "unsignedLong", "18446744073709551616", false}, {"fraction integer", "integer", "0.5", false}, {"wrong string domain", "string", "1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, b, _ := bindCurrent(t, TypeRef{Name: QName{Space: XSDNamespace, Local: tc.builtin}}, currentNumber(t, tc.value), nil)
			got := r.Results(b).Scalar
			if got.Valid != tc.valid || !got.Complete {
				t.Fatalf("result=%+v", got)
			}
			if tc.valid {
				s := r.Scalar(b)
				if !s.Known || s.Lexeme != "" || s.EffectiveLexeme != "" {
					t.Fatalf("scalar=%+v", s)
				}
			}
		})
	}
}
func TestCurrentNumericFacetsAndPattern(t *testing.T) {
	cases := []struct {
		name            string
		schema          *SimpleSchema
		value           string
		valid, complete bool
		constraint      string
	}{
		{name: "exact decimal boundary", schema: &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "decimal"}}, HasMaxInclusive: true, MaxInclusive: "0.1"}, value: "0.1000000000000000055511151231257827021181583404541015625", constraint: "maxInclusive", complete: true},
		{name: "binary fraction digits", schema: &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "decimal"}}, HasFractionDigits: true, FractionDigits: 2}, value: "0.1000000000000000055511151231257827021181583404541015625", constraint: "fractionDigits", complete: true},
		{name: "pattern unavailable", schema: &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "integer"}}, Patterns: []string{"[0-9]+"}}, value: "4", valid: true},
		{name: "pattern keeps bound failure", schema: &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "integer"}}, Patterns: []string{"[0-9]+"}, HasMaxInclusive: true, MaxInclusive: "3"}, value: "4", constraint: "maxInclusive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, b, _ := bindCurrent(t, TypeRef{InlineSimple: tc.schema}, currentNumber(t, tc.value), nil)
			got := r.Results(b).Scalar
			if got.Valid != tc.valid || got.Complete != tc.complete {
				t.Fatalf("result=%+v", got)
			}
			if tc.constraint != "" && (len(got.Issues) == 0 || got.Issues[0].Constraint != tc.constraint) {
				t.Fatalf("issues=%+v", got.Issues)
			}
		})
	}
}
func TestCurrentStringUnionAndFixed(t *testing.T) {
	numeric := currentRestriction("decimal")
	union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Inline: numeric}, {Name: QName{Space: XSDNamespace, Local: "string"}}}}
	r, b, _ := bindCurrent(t, TypeRef{InlineSimple: union}, CurrentScalar{Known: true, Value: SimpleValue{Domain: StringDomain, Text: " 0.1 "}}, nil)
	if got := r.Scalar(b); !got.Known || got.Value.Domain != NumberDomain || got.Lexeme != "" {
		t.Fatalf("scalar=%+v", got)
	}
	fixed := "0.1"
	r, b, _ = bindCurrent(t, TypeRef{Name: QName{Space: XSDNamespace, Local: "decimal"}}, currentNumber(t, "0.1000000000000000055511151231257827021181583404541015625"), &fixed)
	if r.Results(b).Declaration.Valid {
		t.Fatal("binary decimal equated with decimal fixed")
	}
	zero := "0"
	r, b, _ = bindCurrent(t, TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}, currentNumber(t, "0"), &zero)
	if !r.Results(b).Declaration.Valid || r.Scalar(b).Defaulted {
		t.Fatal("numeric zero treated as empty")
	}
}
