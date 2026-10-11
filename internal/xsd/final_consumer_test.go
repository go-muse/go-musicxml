package xsd

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type builderAdmissionMeter struct {
	testMeter
	reservations []int64
	denyText     bool
}

func (m *builderAdmissionMeter) Reserve(n int64) error {
	m.reservations = append(m.reservations, n)
	if m.denyText && len(m.reservations) == 3 {
		return errMeterStop
	}
	return m.testMeter.Reserve(n)
}
func textRun(t *testing.T, decl *ElementSchema, m Meter) (*Run, Use) {
	t.Helper()
	c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: decl}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRun(c, m)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	u, _ := r.Root(QName{Local: "n"})
	return r, u
}
func TestRunTextBuilderCapacityAdmittedBeforeGrowth(t *testing.T) {
	for _, size := range []int{0, 1, 9, 20000, 65537} {
		m := &builderAdmissionMeter{}
		r, u := textRun(t, &ElementSchema{Name: QName{Local: "n"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}}, m)
		b, err := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, Text: strings.Repeat("a", size), TextKnown: true, ChildrenComplete: true, AttributesComplete: true})
		if err != nil {
			t.Fatal(err)
		}
		if admitted := m.reservations[2]; admitted < int64(b.state.node.Text.Cap()) {
			t.Errorf("size=%d admitted=%d capacity=%d", size, admitted, b.state.node.Text.Cap())
		}
		if size == 0 && b.state.node.Text.Cap() != 0 {
			t.Fatal("empty text allocated buffer")
		}
	}
	m := &builderAdmissionMeter{denyText: true}
	r, u := textRun(t, &ElementSchema{Name: QName{Local: "n"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}}, m)
	_, err := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, Text: strings.Repeat("a", 20000), TextKnown: true, ChildrenComplete: true, AttributesComplete: true})
	if !errors.Is(err, errMeterStop) || len(r.nodes) != 0 {
		t.Fatalf("growth preceded admission: %v, nodes=%d", err, len(r.nodes))
	}
}
func TestRunUnknownChildrenDoNotEstablishElementConstraintEmptiness(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		value := "7"
		decl := &ElementSchema{Name: QName{Local: "n"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}}
		if fixed {
			decl.Fixed = &value
		} else {
			decl.Default = &value
		}
		r, u := textRun(t, decl, &testMeter{})
		b, err := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, TextKnown: true, AttributesComplete: true})
		if err != nil {
			t.Fatal(err)
		}
		result := r.Results(b)
		if result.Declaration.Complete || result.Scalar.Complete || !result.Declaration.Valid || !result.Scalar.Valid || r.Scalar(b).Known || r.Scalar(b).Defaulted {
			t.Errorf("unknown emptiness fabricated value: fixed=%v results=%+v scalar=%+v", fixed, result, r.Scalar(b))
		}
	}
}
func TestRunUnknownChildrenKeepFixedFailureWithoutClosure(t *testing.T) {
	value := "7"
	r, u := textRun(t, &ElementSchema{Name: QName{Local: "n"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}, Fixed: &value}, &testMeter{})
	b, err := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, Text: "8", TextKnown: true, AttributesComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	v := r.Results(b).Declaration
	if v.Valid || v.Complete {
		t.Fatalf("fixed known failure/unknown child absence conflated: %+v", v)
	}
}

func TestRunIncompleteAttributesDoNotInventXSIControlledFacts(t *testing.T) {
	value := "7"
	r, u := textRun(t, &ElementSchema{Name: QName{Local: "n"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}, Default: &value, Nillable: true}, &testMeter{})
	b, err := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, TextKnown: true, ChildrenComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	results := r.Results(b)
	for _, result := range []Result{results.Binding, results.Declaration, results.Attributes, results.Scalar, results.Content} {
		if result.Complete || !result.Valid {
			t.Errorf("unknown attributes fabricated assessment: %+v", result)
		}
	}
	if _, known := r.ActualType(b); known || r.Scalar(b).Known || r.Scalar(b).Defaulted {
		t.Fatalf("hidden xsi:type/nil invented scalar: %+v", r.Scalar(b))
	}
}
func TestRunIncompleteAttributesDoNotProveDeclaredAttributeProhibition(t *testing.T) {
	r, u := textRun(t, &ElementSchema{Name: QName{Local: "n"}, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect}}}, &testMeter{})
	b, err := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, TextKnown: true, ChildrenComplete: true, Attributes: []Attribute{{Attr: Attr{Name: QName{Local: "extension"}, Value: "x"}, ValueKnown: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if v := r.Results(b).Attributes; !v.Valid || v.Complete {
		t.Fatalf("hidden derived xsi:type could permit attribute: %+v", v)
	}
}
func TestRunUnknownSchemaHintValueIsIncomplete(t *testing.T) {
	r, u := textRun(t, &ElementSchema{Name: QName{Local: "n"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}}, &testMeter{})
	b, err := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, TextKnown: true, ChildrenComplete: true, AttributesComplete: true, Attributes: []Attribute{{Attr: Attr{Name: QName{Space: XSINamespace, Local: "schemaLocation"}, Value: "http://%invalid"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if v := r.Results(b).Declaration; !v.Valid || v.Complete {
		t.Fatalf("unknown hint used lexical payload as evidence: %+v", v)
	}
	if !r.Results(b).Binding.Complete || !r.Scalar(b).Known {
		t.Fatal("independent known type/scalar lost")
	}
}
