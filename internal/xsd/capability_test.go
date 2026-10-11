package xsd

import (
	"context"
	"testing"
)

func bindSingle(t *testing.T, typ TypeRef, text string, attrs ...Attribute) (*Run, Binding) {
	t.Helper()
	c, e := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: typ}}}, CompileLimits{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := NewRun(c, &testMeter{})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close() })
	u, _ := c.Root(QName{Local: "n"})
	b, e := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, Path: "/n", Text: text, TextKnown: true, Attributes: attrs, AttributesComplete: true, ChildrenComplete: true})
	if e != nil {
		t.Fatal(e)
	}
	return r, b
}
func TestRunOpaqueAnyTypeIsIncomplete(t *testing.T) {
	r, b := bindSingle(t, TypeRef{Name: QName{Space: XSDNamespace, Local: "anyType"}}, "")
	v := r.Results(b).Content
	if v.Complete || !v.Valid {
		t.Fatalf("opaque type claimed closure: %+v", v)
	}
}
func TestRunUnverifiedEqualityDoesNotBecomeFacetFailure(t *testing.T) {
	simple := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "date"}}, Enumerations: []string{"2026-10-11"}}
	r, b := bindSingle(t, TypeRef{InlineSimple: simple}, "2026-10-12")
	v := r.Results(b).Scalar
	if v.Complete || !v.Valid || len(v.Issues) != 0 {
		t.Fatalf("unverified equality invented validity: %+v", v)
	}
}
func TestRunAbsentDefaultIdentityBlocksClosure(t *testing.T) {
	complex := &ComplexSchema{Form: ComplexDirect, Attributes: []AttributeSchema{{Name: QName{Local: "ref"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "IDREF"}}, Use: AttributeOptional, Constraint: &ValueConstraint{Kind: "default", Value: "missing"}}}}
	r, b := bindSingle(t, TypeRef{InlineComplex: complex}, "")
	v := r.Results(b).Attributes
	if v.Complete || !v.Valid {
		t.Fatalf("absent default identity claimed closure: %+v", v)
	}
	a := r.Attributes(b)
	if len(a) != 1 || a[0].Present || !a[0].Scalar.Defaulted || !a[0].Scalar.Known {
		t.Fatalf("default facts: %+v", a)
	}
}
func TestRunIndependentAttributeFactsRemainUsable(t *testing.T) {
	complex := &ComplexSchema{Form: ComplexDirect, Attributes: []AttributeSchema{{Name: QName{Local: "bad"}, Type: TypeRef{Name: QName{Local: "missing"}}}, {Name: QName{Local: "ok"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}}}}
	r, b := bindSingle(t, TypeRef{InlineComplex: complex}, "", Attribute{Attr: Attr{Name: QName{Local: "bad"}, Value: "x"}, ValueKnown: true}, Attribute{Attr: Attr{Name: QName{Local: "ok"}, Value: "7"}, ValueKnown: true})
	a := r.Attributes(b)
	if len(a) != 2 || !a[1].TypeValid || !a[1].Complete || !a[1].Scalar.Known || a[1].Use.Occurrence() == "" {
		t.Fatalf("unrelated incomplete attribute erased known slot: %+v", a)
	}
}

func TestRunWildcardModesRemainDistinct(t *testing.T) {
	for _, mode := range []string{"skip", "lax", "strict"} {
		t.Run(mode, func(t *testing.T) {
			typ := &ComplexSchema{Form: ComplexDirect, Particle: &ParticleSchema{Kind: ParticleAny, Occurrence: Occurrence{Min: 1, Max: 1}, Any: &AnySchema{Namespace: "##any", ProcessContents: mode}}}
			c, e := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{InlineComplex: typ}}}}, CompileLimits{})
			if e != nil {
				t.Fatal(e)
			}
			r, e := NewRun(c, &testMeter{})
			if e != nil {
				t.Fatal(e)
			}
			defer r.Close()
			u, _ := c.Root(QName{Local: "n"})
			b, e := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, TextKnown: true, AttributesComplete: true, ChildrenComplete: true, Children: []Child{{ID: 2, Name: QName{Local: "foreign"}}}})
			if e != nil {
				t.Fatal(e)
			}
			v := r.Results(b).Content
			if !v.Valid || v.Complete != (mode == "skip") {
				t.Fatalf("wildcard local closure=%+v", v)
			}
			identity, err := r.IdentityPreview(context.Background())
			if err != nil || identity.Complete {
				t.Fatalf("wildcard identity closure=%+v, %v", identity, err)
			}
			w := r.Witnesses(b)
			if len(w) != 1 || string(w[0].Mode) != map[string]string{"skip": "skip", "lax": "unsupported-lax", "strict": "unsupported-strict"}[mode] {
				t.Fatalf("wildcard mode=%+v", w)
			}
			if _, e = r.Complete(context.Background(), b, nil); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestRunIdentityOriginsDoNotUseDisplayPaths(t *testing.T) {
	leaf := func(name string) *ParticleSchema {
		return &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: 1, Max: 1}, Element: &ElementSchema{Name: QName{Space: name, Local: "item"}, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Attributes: []AttributeSchema{{Name: QName{Local: "id"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "ID"}}}}}}}}
	}
	typ := &ComplexSchema{Form: ComplexDirect, Particle: &ParticleSchema{Kind: ParticleSequence, Occurrence: Occurrence{Min: 1, Max: 1}, Children: []*ParticleSchema{leaf("a"), leaf("b")}}}
	c, e := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: TypeRef{InlineComplex: typ}}}}, CompileLimits{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := NewRun(c, &testMeter{})
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	u, _ := c.Root(QName{Local: "root"})
	root, e := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: true, Children: []Child{{ID: 2, Name: QName{Space: "a", Local: "item"}, Path: "/same"}, {ID: 3, Name: QName{Space: "b", Local: "item"}, Path: "/same"}}})
	if e != nil {
		t.Fatal(e)
	}
	var completions []Completion
	for i, w := range r.Witnesses(root) {
		b, e := r.Bind(context.Background(), w.Use, Subject{ID: w.Child, Parent: 1, Name: QName{Space: []string{"a", "b"}[i], Local: "item"}, Path: "/same", TextKnown: true, AttributesComplete: true, ChildrenComplete: true, Attributes: []Attribute{{Attr: Attr{Name: QName{Local: "id"}, Value: "same"}, ValueKnown: true}}})
		if e != nil {
			t.Fatal(e)
		}
		cc, e := r.Complete(context.Background(), b, nil)
		if e != nil {
			t.Fatal(e)
		}
		completions = append(completions, cc)
	}
	if _, e = r.Complete(context.Background(), root, completions); e != nil {
		t.Fatal(e)
	}
	got, e := r.IdentityPreview(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	details := got.BySubject[3].Issues
	if len(details) != 1 || details[0].Subject != 3 || details[0].Attribute.Local != "id" {
		t.Fatalf("duplicate attributed by ambiguous path: %+v", got)
	}
}
