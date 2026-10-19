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
	huge := xsd.SchemaData{Types: map[xsd.QName]*xsd.TypeSchema{{Local: "large"}: {Simple: &xsd.SimpleSchema{Members: make([]*xsd.SimpleMember, 65537)}}}}
	if err := xsd.ConvertGraph(&target, huge); err == nil {
		t.Fatal("conversion ignored the compile limits")
	}
}
