package validation

import (
	"context"
	"errors"
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

func schemaAttributeScratchFixture(t *testing.T) (*Session, *factNode, xsd.Use, xsd.Binding) {
	t.Helper()
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{
		{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{
			{Name: SchemaName{Local: "a"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: "string"}}},
			{Name: SchemaName{Local: "b"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: "string"}}},
		}}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	s, err := sourceSchemaEngineFor(t, schema, XSDCheckAttributes).Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.cleanup)
	if err := s.Open(context.Background(), schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ name, value string }{{"a", "x"}, {"b", "yy"}} {
		if err := s.Attribute(context.Background(), AttributeFact{Owner: 1, Name: QName{Local: item.name}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "schema-binding-pending"}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceKnown, Value: item.value}, Value: Evidence[Atomic]{State: EvidenceUnknown, Reason: "schema-binding-pending"}}, Location: s.root.header.Location}); err != nil {
			t.Fatal(err)
		}
	}
	n := s.root
	n.closed, n.end = true, schemaLexemeEnd("")
	run, err := xsd.NewRun(s.schema.active.kernel, schemaMeter{s})
	if err != nil {
		t.Fatal(err)
	}
	s.schema.run = run
	u, _ := s.schema.active.kernel.Root(xsd.QName{Local: "root"})
	subject, err := s.schemaSubject(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := run.Bind(context.Background(), u, subject)
	if err != nil {
		t.Fatal(err)
	}
	return s, n, u, binding
}

func TestSchemaAttributeScratchRetainsOnlyPublishedScalars(t *testing.T) {
	s, n, u, b := schemaAttributeScratchFixture(t)
	before := s.retained
	if err := s.publishSchemaFacts(context.Background(), n, u, b); err != nil {
		t.Fatal(err)
	}
	want := int64(128 + 1 + 128 + 2)
	if got := s.retained - before; got != want {
		t.Errorf("temporary attribute index retained: got %d, want real scalar storage %d", got, want)
	}
	for i, value := range []string{"x", "yy"} {
		if n.attributes[i].Declaration.State != EvidenceKnown || n.attributes[i].Scalar.Value.State != EvidenceKnown || n.attributes[i].Scalar.Value.Value.Text != value {
			t.Fatalf("published attribute %d lost: %+v", i, n.attributes[i])
		}
	}
	schemaScratchAssertReleased(t, s)
}

func TestSchemaAttributeScratchCancellationDropsIndex(t *testing.T) {
	s, n, u, b := schemaAttributeScratchFixture(t)
	before := s.retained
	used, _ := s.account.Usage(BudgetBufferedBytes)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.publishSchemaFacts(ctx, n, u, b); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if s.retained != before {
		t.Errorf("canceled index retained: got %d, want %d", s.retained, before)
	}
	if got, _ := s.account.Usage(BudgetBufferedBytes); got != used {
		t.Errorf("canceled index lease remained live: got %d, want %d", got, used)
	}
	schemaScratchAssertReleased(t, s)
}
