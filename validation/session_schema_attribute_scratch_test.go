package validation

import (
	"context"
	"errors"
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

func schemaAttributeScratchFixture(t *testing.T, invalid ...bool) (*Session, *factNode, xsd.Use, xsd.Binding) {
	t.Helper()
	firstType := "string"
	if len(invalid) != 0 && invalid[0] {
		firstType = "integer"
	}
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[QName]*SchemaElement{
		{Local: "root"}: {Name: QName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{
			{Name: QName{Local: "a"}, Type: SchemaTypeRef{Name: QName{Space: xsd.XSDNamespace, Local: firstType}}},
			{Name: QName{Local: "b"}, Type: SchemaTypeRef{Name: QName{Space: xsd.XSDNamespace, Local: "string"}}},
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
	if err := s.OpenNode(context.Background(), schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ name, value string }{{"a", "x"}, {"b", "yy"}} {
		if err := s.Attribute(context.Background(), AttributeFact{Owner: 1, Name: QName{Local: item.name}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "schema-binding-pending"}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceKnown, Value: item.value}, Value: Evidence[Atomic]{State: EvidenceUnknown, Reason: "schema-binding-pending"}}, Location: s.root.header.Location}); err != nil {
			t.Fatal(err)
		}
	}
	n := s.root
	n.closed, n.end = true, testRetainedEnd(schemaLexemeEnd(""))
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
	// Warm only the exact immutable header records, not the root scalar or
	// any attributes. This isolates the temporary map's lifetime from the
	// two session caches while leaving their real ownership fully charged.
	beforeHeaders := s.retained
	typ, known := run.ActualType(binding)
	if !known {
		t.Fatal("fixture actual type unavailable")
	}
	declaration := s.schema.active.declaration(u)
	component := s.schema.active.component(typ)
	if !validDeclaration(declaration) || !validComponent(component) {
		t.Fatal("fixture header identity unavailable")
	}
	if _, err := s.retainSchemaDeclaration(context.Background(), u.CacheIdentity(), Evidence[DeclarationID]{State: EvidenceKnown, Value: declaration}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.retainSchemaType(context.Background(), typ.CacheIdentity(), Evidence[ComponentID]{State: EvidenceKnown, Value: component}); err != nil {
		t.Fatal(err)
	}
	// Independently itemized: two 256-byte map headers, eight 64-byte slots
	// per map, and one 96-byte declaration plus one 128-byte type record.
	const headerOwnership = 2*(256+8*64) + 96 + 128
	if got := s.retained - beforeHeaders; got != headerOwnership {
		t.Fatalf("fixture header ownership: got %d, want maps + exact records %d", got, headerOwnership)
	}
	return s, n, u, binding
}

// The complex root has no typed scalar. Each publication replaces its
// immutable unsupported evidence record, retaining 192 bytes until cleanup.
// This is real evidence storage, separate from the temporary attribute map.
const schemaAttributeScratchRootRecordBytes int64 = 192

func schemaAttributeScratchRootReplacement(t *testing.T, n *factNode, before *Evidence[Atomic]) int64 {
	t.Helper()
	if n.end.Scalar.Value == before {
		return 0
	}
	if got := n.end.Scalar.Value; got.State != EvidenceUnsupported || got.Reason != "typed-scalar-unavailable" || got.Cause != nil {
		t.Fatalf("unexpected root scalar replacement: %+v", got)
	}
	return schemaAttributeScratchRootRecordBytes
}

func TestSchemaAttributeScratchRetainsOnlyPublishedScalars(t *testing.T) {
	s, n, u, b := schemaAttributeScratchFixture(t)
	before := s.retained
	if err := s.publishSchemaFacts(context.Background(), n, u, b); err != nil {
		t.Fatal(err)
	}
	want := schemaAttributeScratchRootRecordBytes + int64(128+1+128+2)
	if got := s.retained - before; got != want {
		t.Errorf("temporary attribute index retained: got %d, want root evidence plus attribute scalar storage %d", got, want)
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
	rootBefore := n.end.Scalar.Value
	// The first two polls are exact declaration/type cache lookups. Poll
	// three is the original map-population phase, after root publication.
	ctx := &schemaScratchCancelContext{Context: context.Background(), at: 3}
	if err := s.publishSchemaFacts(ctx, n, u, b); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if ctx.calls != 3 || schemaAttributeScratchRootReplacement(t, n, rootBefore) != schemaAttributeScratchRootRecordBytes {
		t.Fatal("cancellation did not reach the original map-population phase")
	}
	if s.retained != before+schemaAttributeScratchRootRecordBytes {
		t.Errorf("canceled index retained beyond root evidence: got %d, want %d", s.retained, before+schemaAttributeScratchRootRecordBytes)
	}
	if got, _ := s.account.Usage(BudgetBufferedBytes); got != used+schemaAttributeScratchRootRecordBytes {
		t.Errorf("canceled index lease remained live: got %d, want root-only delta %d", got, used+schemaAttributeScratchRootRecordBytes)
	}
	schemaScratchAssertReleased(t, s)
}
