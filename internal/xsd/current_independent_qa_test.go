package xsd

import (
	"context"
	"testing"
)

func TestIndependentCurrentFixedUnionPatternUnavailable(t *testing.T) {
	patterned := currentRestriction("integer")
	patterned.Patterns = []string{"[0-9]+"}
	union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Inline: patterned}, {Name: QName{Space: XSDNamespace, Local: "decimal"}}}}
	fixed := "4"
	r, b, _ := bindCurrent(t, TypeRef{InlineSimple: union}, currentNumber(t, "4"), &fixed)
	got := r.Results(b)
	t.Logf("declaration=%+v scalar=%+v", got.Declaration, got.Scalar)
	if !got.Declaration.Valid {
		t.Errorf("unavailable numeric union member selection fabricated a fixed-value mismatch: %+v", got.Declaration)
	}
	if got.Declaration.Complete || got.Scalar.Complete {
		t.Errorf("lexical pattern unexpectedly assessed: %+v", got)
	}
}

func TestIndependentCurrentAttributePatternCompleteness(t *testing.T) {
	patterned := currentRestriction("integer")
	patterned.Patterns = []string{"[0-9]+"}
	name := QName{Local: "root"}
	schema, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{name: {Name: name, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Attributes: []AttributeSchema{{Name: QName{Local: "n"}, Type: TypeRef{InlineSimple: patterned}}}}}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRun(schema, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	use, _ := schema.Root(name)
	b, err := r.Bind(context.Background(), use, Subject{ID: 1, Name: name, Path: "Root", CurrentModel: true, Current: CurrentScalar{Absent: true}, ChildrenOrderKnown: true, ChildrenComplete: true, AttributesComplete: true, Attributes: []Attribute{{Attr: Attr{Name: QName{Local: "n"}}, Current: currentNumber(t, "4")}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("aggregate=%+v attribute=%+v", r.Results(b).Attributes, r.Attributes(b))
	if r.Results(b).Attributes.Complete {
		t.Fatal("missing aggregate limitation")
	}
	for _, attribute := range r.Attributes(b) {
		if attribute.Complete || attribute.Scalar.Complete {
			t.Errorf("unavailable lexical pattern reported complete: %+v", attribute)
		}
	}
}
