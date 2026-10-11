package xsd

import (
	"context"
	"errors"
	"testing"
)

type testMeter struct {
	steps, retained, ids int64
	limit                int64
}

func (m *testMeter) Step(ctx context.Context, n int64) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if m.limit > 0 && m.steps+n > m.limit {
		return errors.New("step limit")
	}
	m.steps += n
	return nil
}
func (m *testMeter) Reserve(n int64) error  { m.retained += n; return nil }
func (m *testMeter) Release(n int64) error  { m.retained -= n; return nil }
func (m *testMeter) Scalar(n int64) error   { return nil }
func (m *testMeter) Identity(n int64) error { m.ids += n; return nil }
func testRunSchema(t *testing.T) *Compiled {
	t.Helper()
	data := SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Particle: &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: 1, Max: 1}, Element: &ElementSchema{Name: QName{Local: "child"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}}}}}}}}
	c, e := Compile(data, CompileLimits{})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestRunBindsWithoutWalkingChildren(t *testing.T) {
	ctx := context.Background()
	c := testRunSchema(t)
	m := &testMeter{}
	r, e := NewRun(c, m)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	u, _ := c.Root(QName{Local: "root"})
	b, e := r.Bind(ctx, u, Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: true, Children: []Child{{ID: 2, Name: QName{Local: "child"}, Path: "/root/child"}}})
	if e != nil {
		t.Fatal(e)
	}
	w := r.Witnesses(b)
	if len(w) != 1 || w[0].Child != 2 {
		t.Fatalf("witnesses=%+v", w)
	}
	if !r.Results(b).Content.Valid {
		t.Fatal("child was prematurely assessed")
	}
	child, e := r.Bind(ctx, w[0].Use, Subject{ID: 2, Parent: 1, Name: QName{Local: "child"}, Path: "/root/child", Text: "bad", TextKnown: true, AttributesComplete: true, ChildrenComplete: true})
	if e != nil {
		t.Fatal(e)
	}
	if r.Results(child).Scalar.Valid {
		t.Fatal("invalid child scalar passed")
	}
	cc, e := r.Complete(ctx, child, nil)
	if e != nil {
		t.Fatal(e)
	}
	parent, e := r.Complete(ctx, b, []Completion{cc})
	if e != nil {
		t.Fatal(e)
	}
	if parent.TypeValid || parent.DeclarationValid {
		t.Fatal("child failure lost")
	}
}
func TestRunUnknownScalarIsNotEmpty(t *testing.T) {
	c, e := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}}}}, CompileLimits{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := NewRun(c, &testMeter{})
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	u, _ := c.Root(QName{Local: "n"})
	b, e := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, AttributesComplete: true, ChildrenComplete: true})
	if e != nil {
		t.Fatal(e)
	}
	s := r.Results(b).Scalar
	if s.Complete || !s.Valid || len(s.Issues) != 0 {
		t.Fatalf("unknown fabricated invalidity: %+v", s)
	}
}

func TestRunInvalidNominationKeepsKnownFallback(t *testing.T) {
	c, e := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}}}}, CompileLimits{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := NewRun(c, &testMeter{})
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	u, _ := c.Root(QName{Local: "n"})
	b, e := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, Path: "/n", Text: "bad", TextKnown: true, AttributesComplete: true, ChildrenComplete: true, Attributes: []Attribute{{Attr: Attr{Name: QName{Space: XSINamespace, Local: "type"}, Value: "missing"}, ValueKnown: true}}})
	if e != nil {
		t.Fatal(e)
	}
	v := r.Results(b)
	if !v.Binding.Valid || !v.Binding.Complete || len(v.Binding.Issues) != 0 || v.Declaration.Valid || v.Scalar.Valid {
		t.Fatalf("fallback results: %+v", v)
	}
	typ, ok := r.ActualType(b)
	if !ok || typ.Name().Local != "integer" {
		t.Fatalf("fallback actual type: %+v", typ)
	}
}
func TestRunUnknownChildrenKeepIndependentAttributes(t *testing.T) {
	c := testRunSchema(t)
	r, e := NewRun(c, &testMeter{})
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	u, _ := c.Root(QName{Local: "root"})
	b, e := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: false, Attributes: []Attribute{{Attr: Attr{Name: QName{Local: "bad"}, Value: "x"}, ValueKnown: true}}})
	if e != nil {
		t.Fatal(e)
	}
	v := r.Results(b)
	if v.Attributes.Valid || !v.Attributes.Complete || !v.Content.Valid || v.Content.Complete || len(v.Content.Issues) != 0 {
		t.Fatalf("incomplete children invented/lost verdict: %+v", v)
	}
}
func TestRunUnknownAttributesCannotProveRequiredMissing(t *testing.T) {
	c, e := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Attributes: []AttributeSchema{{Name: QName{Local: "a"}, Use: AttributeRequired, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}}}}}}}}, CompileLimits{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := NewRun(c, &testMeter{})
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	u, _ := c.Root(QName{Local: "n"})
	b, e := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "n"}, TextKnown: true, ChildrenComplete: true})
	if e != nil {
		t.Fatal(e)
	}
	v := r.Results(b).Attributes
	if !v.Valid || v.Complete || len(v.Issues) != 0 {
		t.Fatalf("unknown attributes became absence: %+v", v)
	}
}
func TestRunFinishAndCleanupAreSingleUse(t *testing.T) {
	c := testRunSchema(t)
	m := &testMeter{}
	r, e := NewRun(c, m)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.IdentityPreview(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, e = r.Finalize(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	if _, e = r.Finalize(context.Background(), true); e == nil {
		t.Fatal("second finalize accepted")
	}
	if e = r.Close(); e != nil {
		t.Fatal(e)
	}
	if m.retained != 0 {
		t.Fatalf("live retention=%d", m.retained)
	}
	if e = r.Close(); e == nil {
		t.Fatal("second close accepted")
	}
}

func TestRunIncompleteEnumerationRetainsSafeFullMatchWitness(t *testing.T) {
	c := testRunSchema(t)
	r, err := NewRun(c, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	u, _ := c.Root(QName{Local: "root"})
	b, err := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", AttributesComplete: true, Children: []Child{{ID: 2, Name: QName{Local: "child"}, Path: "/root/child"}}})
	if err != nil {
		t.Fatal(err)
	}
	v := r.Results(b).Content
	if !v.Valid || v.Complete || len(v.Issues) != 0 {
		t.Fatalf("unknown suffix invented invalidity: %+v", v)
	}
	w := r.Witnesses(b)
	if len(w) != 1 || w[0].Child != 2 || w[0].Mode != WitnessDeclared {
		t.Fatalf("safe full-match prefix lost: %+v", w)
	}
	child, err := r.Bind(context.Background(), w[0].Use, Subject{ID: 2, Parent: 1, Name: QName{Local: "child"}, Path: "/root/child", Text: "bad", TextKnown: true, AttributesComplete: true, ChildrenComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Results(child).Scalar.Valid {
		t.Fatal("closed invalid child was lost")
	}
}
