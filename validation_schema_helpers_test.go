package musicxml

import (
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
	"github.com/go-muse/go-musicxml/validation"
)

// publicValidationSchema converts a generated schema into the validation
// package's metadata types for tests that edit it before compiling.
func publicValidationSchema(t testing.TB, data validationSchemaSet) validation.SchemaData {
	t.Helper()
	var public validation.SchemaData
	if err := xsd.ConvertGraph(&public, data); err != nil {
		t.Fatal(err)
	}
	return public
}

// Generated schemas are converted once; a variant of the same assembly is
// converted on its own and keeps its own digest.
func TestCompileValidationSchemaCachesOnlyGeneratedSchemas(t *testing.T) {
	first, err := compileValidationSchema(validation.AssemblySounds, soundsValidationSchema)
	if err != nil {
		t.Fatal(err)
	}
	again, err := compileValidationSchema(validation.AssemblySounds, soundsValidationSchema)
	if err != nil {
		t.Fatal(err)
	}
	kernel, err := xsd.Compile(soundsValidationSchema, xsd.CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() != kernel.Digest() || again.Digest() != first.Digest() {
		t.Fatal("the converted generated schema changed its digest")
	}
	variant := validationSchemaSet{Types: map[validationQName]*xsd.TypeSchema{}, Elements: map[validationQName]*xsd.ElementSchema{}}
	for name, element := range soundsValidationSchema.Elements {
		variant.Elements[name] = element
	}
	for name, typ := range soundsValidationSchema.Types {
		variant.Types[name] = typ
	}
	delete(variant.Elements, validationQName{Local: "sounds"})
	other, err := compileValidationSchema(validation.AssemblySounds, variant)
	if err != nil {
		t.Fatal(err)
	}
	if other.Digest() == first.Digest() {
		t.Fatal("a variant of a generated schema reused its cached conversion")
	}
}
