package validation

import (
	"reflect"
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

// sharedSchemaGraph has a shared particle, pointers into embedded fields and a
// cycle through an inline complex type.
func sharedSchemaGraph() xsd.SchemaData {
	element := &xsd.ElementSchema{Name: xsd.QName{Local: "root"}}
	element.Fixed = &element.Name.Local
	complex := &xsd.ComplexSchema{Form: xsd.ComplexDirect, Attributes: []xsd.AttributeSchema{{Name: xsd.QName{Local: "a"}}}}
	complex.Base = &complex.Attributes[0].Type
	shared := &xsd.ParticleSchema{Kind: xsd.ParticleElement, Element: &xsd.ElementSchema{Name: xsd.QName{Local: "self"}, Type: xsd.TypeRef{InlineComplex: complex}}}
	complex.Particle = &xsd.ParticleSchema{Kind: xsd.ParticleSequence, Children: []*xsd.ParticleSchema{shared, shared}}
	element.Type.InlineComplex = complex
	return xsd.SchemaData{Elements: map[xsd.QName]*xsd.ElementSchema{element.Name: element}}
}

func TestSchemaTypesMirrorTheKernel(t *testing.T) {
	kernel, err := xsd.Compile(sharedSchemaGraph(), xsd.CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	var public SchemaData
	if err := xsd.ConvertGraph(&public, sharedSchemaGraph()); err != nil {
		t.Fatal(err)
	}
	root := public.Elements[QName{Local: "root"}]
	complex := root.Type.InlineComplex
	if root.Fixed != &root.Name.Local || complex.Base != &complex.Attributes[0].Type {
		t.Fatal("conversion lost pointers into embedded fields")
	}
	if complex.Particle.Children[0] != complex.Particle.Children[1] || complex.Particle.Children[0].Element.Type.InlineComplex != complex {
		t.Fatal("conversion lost sharing or the cycle")
	}
	compiled, err := CompileSchema(AssemblyMusicXML, public)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Digest() != kernel.Digest() {
		t.Fatalf("digest %s differs from the kernel's %s", compiled.Digest(), kernel.Digest())
	}

	var back xsd.SchemaData
	if err := xsd.ConvertGraph(&back, public); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, sharedSchemaGraph()) {
		t.Fatal("round trip changed the graph")
	}
}

func TestConvertGraphRejectsOtherShapes(t *testing.T) {
	type renamed struct {
		Types    map[QName]*SchemaType
		Elements map[QName]*SchemaElement
		Extra    bool
	}
	var target SchemaData
	if err := xsd.ConvertGraph(&target, renamed{}); err == nil {
		t.Fatal("an extra field was accepted")
	}
	if err := xsd.ConvertGraph(target, sharedSchemaGraph()); err == nil {
		t.Fatal("a non-pointer target was accepted")
	}
	if err := xsd.ConvertGraph(&target, nil); err == nil {
		t.Fatal("a nil source was accepted")
	}
	if _, err := xsd.CompileGraph(nil, xsd.CompileLimits{}); err == nil {
		t.Fatal("a nil graph was compiled")
	}
	stale := SchemaData{Types: map[QName]*SchemaType{{Local: "stale"}: {}}}
	if err := xsd.ConvertGraph(&stale, xsd.SchemaData{}); err != nil || stale.Types != nil {
		t.Fatalf("conversion kept the target's previous maps: %v %v", err, stale.Types)
	}
	huge := xsd.SchemaData{Types: map[xsd.QName]*xsd.TypeSchema{{Local: "large"}: {Simple: &xsd.SimpleSchema{Members: make([]*xsd.SimpleMember, 65537)}}}}
	if err := xsd.ConvertGraph(&target, huge); err == nil {
		t.Fatal("conversion ignored the compile limits")
	}
}

// The Patterns contract: alternatives within one restriction layer, an
// intersection across layers.
func TestSchemaSimplePatternsAreAlternativesWithinALayer(t *testing.T) {
	str := &SchemaSimpleMember{Name: QName{Space: SchemaXSDNamespace, Local: "string"}}
	public := SchemaData{Types: map[QName]*SchemaType{
		{Local: "word"}:  {Simple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: str, Patterns: []string{"[0-9]+", "[a-z]+"}}},
		{Local: "short"}: {Simple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: &SchemaSimpleMember{Name: QName{Local: "word"}}, Patterns: []string{".{3}"}}},
		{Local: "free"}:  {Simple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: &SchemaSimpleMember{Name: QName{Local: "word"}}}},
	}}
	var kernel xsd.SchemaData
	if err := xsd.ConvertGraph(&kernel, public); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileSchema(AssemblyMusicXML, public); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		typ, value string
		valid      bool
	}{
		{"word", "123", true}, {"word", "abc", true}, {"word", "12a", false},
		{"short", "123", true}, {"short", "abc", true}, {"short", "1234", false}, {"short", "12a", false},
		{"free", "1234", true}, {"free", "12a", false},
	} {
		c := &xsd.Context{Schema: &kernel}
		_, failure := c.AssessSimpleValue(kernel.Types[xsd.QName{Local: test.typ}].Simple, test.value, test.value)
		if (failure == nil) != test.valid {
			t.Errorf("%s %q: failure %+v, want valid=%t", test.typ, test.value, failure, test.valid)
		}
	}
}
