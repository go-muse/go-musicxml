package xsd

import (
	"reflect"
	"strings"
	"sync"
	"testing"
)

func compilerFixture() SchemaData {
	text := "kept"
	simple := &SimpleSchema{Form: SimpleForm("restriction"), Enumerations: []string{"first", "second"}, Patterns: []string{}, WhiteSpace: "collapse"}
	member := &SimpleMember{Inline: simple}
	simple.Base = member
	shared := &ElementSchema{Name: QName{Local: "item"}, Type: TypeRef{InlineSimple: simple}, Default: &text, Fixed: &text}
	particle := &ParticleSchema{Kind: ParticleKind("sequence")}
	particle.Children = []*ParticleSchema{{Kind: ParticleKind("element"), Element: shared}, {Kind: ParticleKind("element"), Element: shared}, particle}
	complex := &ComplexSchema{Form: ComplexForm("direct"), Particle: particle, Attributes: []AttributeSchema{{Name: QName{Local: "a"}, Type: TypeRef{InlineSimple: simple}, Constraint: &ValueConstraint{Kind: "fixed", Value: "one"}}}, AnyAttribute: &AnyAttributeSchema{Namespace: "##other", ProcessContents: "lax"}}
	return SchemaData{Types: map[QName]*TypeSchema{{Local: "T"}: {Complex: complex}, {Local: "unused"}: nil}, Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: TypeRef{Name: QName{Local: "T"}}}, {Local: "nil"}: nil}}
}

func TestCompileOwnsCompleteGraph(t *testing.T) {
	input := compilerFixture()
	compiled, err := Compile(input, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	owned := compiled.data
	if !reflect.DeepEqual(input, owned) {
		t.Fatal("snapshot changed metadata")
	}
	p := owned.Types[QName{Local: "T"}].Complex.Particle
	if p == input.Types[QName{Local: "T"}].Complex.Particle || p.Children[2] != p || p.Children[0].Element != p.Children[1].Element {
		t.Fatal("copy did not preserve graph cycles and sharing")
	}
	s := p.Children[0].Element.Type.InlineSimple
	if s.Base.Inline != s || s.Patterns == nil || s.Members != nil {
		t.Fatal("simple graph or nil/empty state changed")
	}
	if p.Children[0].Element.Default != p.Children[0].Element.Fixed {
		t.Fatal("shared string pointer changed")
	}
	oldDigest := compiled.Digest()
	input.Types[QName{Local: "T"}].Complex.Attributes[0].Constraint.Value = "changed"
	input.Types[QName{Local: "T"}].Complex.Particle.Children[0].Element.Type.InlineSimple.Enumerations[0] = "changed"
	*input.Types[QName{Local: "T"}].Complex.Particle.Children[0].Element.Default = "changed"
	delete(input.Elements, QName{Local: "root"})
	if owned.Types[QName{Local: "T"}].Complex.Attributes[0].Constraint.Value != "one" || s.Enumerations[0] != "first" || *p.Children[0].Element.Default != "kept" || compiled.Digest() != oldDigest {
		t.Fatal("caller mutation reached snapshot")
	}
	if _, present := owned.Elements[QName{Local: "nil"}]; !present {
		t.Fatal("nil map entry lost")
	}
}

func TestCompileDigestDeterministicAndStructural(t *testing.T) {
	a, err := Compile(compilerFixture(), CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Compile(compilerFixture(), CompileLimits{})
	if err != nil || a.Digest() != b.Digest() {
		t.Fatalf("equivalent graphs differ: %v", err)
	}
	for _, change := range []func(*SchemaData){
		func(d *SchemaData) { delete(d.Types, QName{Local: "unused"}) },
		func(d *SchemaData) { d.Types[QName{Local: "T"}].Complex.Attributes[0].Constraint.Value = "two" },
		func(d *SchemaData) {
			d.Types[QName{Local: "T"}].Complex.Particle.Children[0].Element.Type.InlineSimple.Patterns = nil
		},
		func(d *SchemaData) {
			p := d.Types[QName{Local: "T"}].Complex.Particle
			other := *p.Children[1].Element
			p.Children[1].Element = &other
		},
	} {
		d := compilerFixture()
		change(&d)
		c, err := Compile(d, CompileLimits{})
		if err != nil {
			t.Fatal(err)
		}
		if c.Digest() == a.Digest() {
			t.Fatal("digest omitted graph state")
		}
	}
}

func TestCompileLimits(t *testing.T) {
	for _, limits := range []CompileLimits{{MaxBytes: 1}, {MaxRecords: 1}, {MaxEdges: 1}, {MaxLiteralBytes: 1}, {MaxBytes: -1}, {MaxBytes: 1<<63 - 1}} {
		if _, err := Compile(compilerFixture(), limits); err == nil {
			t.Errorf("accepted exceeded limits %+v", limits)
		}
	}
	compiled, err := Compile(compilerFixture(), CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(compilerFixture(), CompileLimits{MaxBytes: compiled.LogicalBytes()}); err != nil {
		t.Fatalf("exact byte boundary: %v", err)
	}
	if _, err := Compile(compilerFixture(), CompileLimits{MaxBytes: compiled.LogicalBytes() - 1}); err == nil {
		t.Fatal("accepted byte boundary + 1")
	}
	if _, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: strings.Repeat("x", 1025)}: nil}}, CompileLimits{MaxLiteralBytes: 1024}); err == nil {
		t.Fatal("map key escaped literal limit")
	}
}

func TestCompileUseIndexAndConcurrentReaders(t *testing.T) {
	compiled, err := Compile(compilerFixture(), CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	root, ok := compiled.Root(QName{Local: "root"})
	if !ok || root.Declaration() != (QName{Local: "root"}) || root.TypeName() != (QName{Local: "T"}) || root.Occurrence() == "" {
		t.Fatalf("wrong root use: %+v", root)
	}
	if _, ok := compiled.Root(QName{Local: "nil"}); ok {
		t.Fatal("nil root became usable")
	}
	owner := compiled.complexTypes[compiled.data.Types[QName{Local: "T"}].Complex]
	p := compiled.data.Types[QName{Local: "T"}].Complex.Particle
	first := compiled.particleUses[particleUseKey{owner: owner, particle: p.Children[0]}]
	second := compiled.particleUses[particleUseKey{owner: owner, particle: p.Children[1]}]
	if len(first) != 1 || len(second) != 1 || first[0].Occurrence() == second[0].Occurrence() || first[0].OwnerOccurrence() != owner.Occurrence() {
		t.Fatal("particle use slots lost")
	}
	roots := compiled.Roots()
	roots[0].Local = "changed"
	if _, ok := compiled.Root(QName{Local: "root"}); !ok {
		t.Fatal("roots accessor leaked mutable state")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				compiled.Roots()
				compiled.Root(QName{Local: "root"})
				compiled.Digest()
			}
		})
	}
	wg.Wait()
}

func TestCompileIndexesBuiltinTypes(t *testing.T) {
	compiled, err := Compile(SchemaData{}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"anyType", "anySimpleType", "string", "ENTITIES", "unsignedLong", "dateTime"} {
		id, ok := compiled.builtinTypes[name]
		if !ok || id.Name() != (QName{Space: XSDNamespace, Local: name}) || id.Occurrence() == "" {
			t.Errorf("missing builtin %s", name)
		}
	}
}

func TestCompilePreservesEmbeddedPointerSharing(t *testing.T) {
	element := &ElementSchema{Name: QName{Local: "root"}}
	element.Fixed = &element.Name.Local
	complex := &ComplexSchema{Attributes: []AttributeSchema{{Name: QName{Local: "a"}}}}
	complex.Base = &complex.Attributes[0].Type
	element.Type.InlineComplex = complex
	input := SchemaData{Elements: map[QName]*ElementSchema{element.Name: element}}
	compiled, err := Compile(input, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	owned := compiled.data.Elements[QName{Local: "root"}]
	if owned.Fixed != &owned.Name.Local {
		t.Error("pointer to embedded string lost identity")
	}
	if owned.Type.InlineComplex.Base != &owned.Type.InlineComplex.Attributes[0].Type {
		t.Error("pointer to embedded type reference lost identity")
	}
}

func TestCompileEveryLimitBoundary(t *testing.T) {
	data := compilerFixture()
	compiled, err := Compile(data, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		amount int64
		limits func(int64) CompileLimits
	}{
		{"records", compiled.compileRecordCount, func(n int64) CompileLimits { return CompileLimits{MaxRecords: n} }},
		{"edges", compiled.compileEdgeCount, func(n int64) CompileLimits { return CompileLimits{MaxEdges: n} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Compile(data, test.limits(test.amount)); err != nil {
				t.Fatalf("exact boundary: %v", err)
			}
			if _, err := Compile(data, test.limits(test.amount-1)); err == nil {
				t.Fatal("accepted exceeded boundary")
			}
		})
	}
	literal := SchemaData{Elements: map[QName]*ElementSchema{{Local: strings.Repeat("x", 1024)}: nil}}
	if _, err := Compile(literal, CompileLimits{MaxLiteralBytes: 1024}); err != nil {
		t.Fatalf("exact literal boundary: %v", err)
	}
	if _, err := Compile(literal, CompileLimits{MaxLiteralBytes: 1023}); err == nil {
		t.Fatal("accepted exceeded literal boundary")
	}
	data = SchemaData{Types: map[QName]*TypeSchema{{Local: "large"}: {Simple: &SimpleSchema{Members: make([]*SimpleMember, 65537)}}}}
	if _, err := Compile(data, CompileLimits{}); err == nil {
		t.Fatal("unbounded collection accepted")
	}
}

func TestCompileRetainsExactReferenceUseSlots(t *testing.T) {
	name := QName{Space: "urn:test", Local: "global"}
	declaration := &ElementSchema{Name: name, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}}
	reference := &ElementSchema{Reference: name, Nillable: true}
	sharedParticle := &ParticleSchema{Kind: ParticleElement, Element: reference}
	complex := &ComplexSchema{Particle: &ParticleSchema{Kind: ParticleSequence, Children: []*ParticleSchema{sharedParticle, sharedParticle}}}
	compiled, err := Compile(SchemaData{Types: map[QName]*TypeSchema{{Local: "owner"}: {Complex: complex}}, Elements: map[QName]*ElementSchema{name: declaration}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	owned := compiled.data.Types[QName{Local: "owner"}].Complex
	owner := compiled.complexTypes[owned]
	uses := compiled.particleUses[particleUseKey{owner, owned.Particle.Children[0]}]
	if len(uses) != 2 {
		t.Fatalf("shared particle slots: %d", len(uses))
	}
	if uses[0].Occurrence() == uses[1].Occurrence() || uses[0].original == uses[0].declaration || uses[0].original.Reference != name || uses[0].Declaration() != name {
		t.Fatal("reference/use distinction lost")
	}
	if uses[0].TypeName() != (QName{Space: XSDNamespace, Local: "string"}) {
		t.Fatal("reference use did not resolve declared type")
	}
	if owned.Particle.Children[0] != owned.Particle.Children[1] {
		t.Fatal("shared particle object lost")
	}
}

func TestCompileDeepGraphIsIterative(t *testing.T) {
	particle := &ParticleSchema{Element: &ElementSchema{Name: QName{Local: "leaf"}}}
	for range 3000 {
		particle = &ParticleSchema{Children: []*ParticleSchema{particle}}
	}
	compiled, err := Compile(SchemaData{Types: map[QName]*TypeSchema{{Local: "deep"}: {Complex: &ComplexSchema{Particle: particle}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, use := range compiled.Uses() {
		if len(use.Occurrence()) > 128 {
			t.Fatalf("recursive path expansion: %d", len(use.Occurrence()))
		}
	}
}

func TestCompileEmbeddedAliasAffectsDigest(t *testing.T) {
	makeData := func(alias bool) SchemaData {
		e := &ElementSchema{Name: QName{Local: "root"}}
		if alias {
			e.Fixed = &e.Name.Local
		} else {
			fixed := "root"
			e.Fixed = &fixed
		}
		return SchemaData{Elements: map[QName]*ElementSchema{e.Name: e}}
	}
	a, err := Compile(makeData(true), CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Compile(makeData(true), CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := Compile(makeData(false), CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Digest() != b.Digest() || a.Digest() == c.Digest() {
		t.Fatal("embedded alias topology not canonical")
	}
}

func TestCompileNilHandlesAndReturnedCopies(t *testing.T) {
	var compiled *Compiled
	if compiled.Digest() != "" || compiled.LogicalBytes() != 0 || compiled.Roots() != nil || compiled.Components() != nil || compiled.Uses() != nil {
		t.Fatal("nil handle access")
	}
	if _, ok := compiled.Root(QName{}); ok {
		t.Fatal("nil handle root")
	}
	var use Use
	if use.Declaration() != (QName{}) || use.TypeName() != (QName{}) || use.Occurrence() != "" || use.OwnerOccurrence() != "" {
		t.Fatal("zero use")
	}
	compiled, err := Compile(compilerFixture(), CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	components, uses := compiled.Components(), compiled.Uses()
	components[0] = TypeID{}
	uses[0] = Use{}
	if compiled.Components()[0].Occurrence() == "" || compiled.Uses()[0].Occurrence() == "" {
		t.Fatal("mutable accessor escaped")
	}
}
