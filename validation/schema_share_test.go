package validation

import "testing"

// A shared handle uses the same immutable compiled graph with its own, empty
// construction memos.
func TestCompiledSchemaShareKeepsTheGraphAndSeparatesMemos(t *testing.T) {
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[QName]*SchemaElement{{Local: "root"}: {Name: QName{Local: "root"}, Type: localBuiltin("string")}}})
	if err != nil {
		t.Fatal(err)
	}
	shared := schema.share()
	if shared == schema || shared.kernel != schema.kernel || shared.assembly != schema.assembly || shared.Digest() != schema.Digest() {
		t.Fatalf("share did not keep the graph: %p %p", shared.kernel, schema.kernel)
	}
	if shared.timeline == nil || shared.context == nil || shared.timeline == schema.timeline || shared.context == schema.context {
		t.Fatal("share must give the handle its own memos")
	}
	if (*CompiledSchema)(nil).share() != nil {
		t.Fatal("nil handle")
	}
}
