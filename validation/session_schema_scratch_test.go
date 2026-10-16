package validation

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

// These generated facts isolate binding ownership from parser/model storage.
// They are test-owned inputs; Begin and all storage allocated by binding use
// the normal bounded Account. No caller corpus fixture is needed.
func schemaScratchFixture(t *testing.T, children int, scalar string, budgets Budgets) *Session {
	t.Helper()
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{
		{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{
			Form:     SchemaComplexDirect,
			Particle: &SchemaParticle{Kind: SchemaParticleElement, Occurrence: SchemaOccurrence{Min: uint64(children), Max: uint64(children)}, Element: &SchemaElement{Name: SchemaName{Local: "child"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: scalar}}}},
		}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	s, err := sourceSchemaEngineFor(t, schema, XSDCheckScalar, XSDCheckContent).Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "source-schema-test", Budgets: budgets})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ensureFactPool(); err != nil {
		t.Fatal(err)
	}
	s.schema.active = schema
	s.root = &factNode{header: testRetainedHeader(schemaPendingHeader()), end: testRetainedEnd(schemaLexemeEnd("")), closed: true}
	s.nodes[1] = s.root
	s.sequence = append(s.sequence, 1)
	for i := 0; i < children; i++ {
		id := NodeID(i + 2)
		h := schemaPendingHeader()
		h.ID, h.Parent = id, 1
		h.Name.Local = "child"
		h.Location.Path = fmt.Sprintf("/root/child[%d]", i+1)
		end := schemaLexemeEnd("x")
		end.ID = id
		child := &factNode{header: testRetainedHeader(h), end: testRetainedEnd(end), closed: true}
		s.nodes[id] = child
		s.sequence = append(s.sequence, id)
		s.root.children = append(s.root.children, child)
	}
	t.Cleanup(s.cleanup)
	return s
}

// Schema publication now owns compact evidence records and two per-session
// identity caches in addition to the traversal leases asserted below. Known
// scalar payload bodies remain in the original scalar charge; only their new
// evidence wrapper is added here. The root's unavailable scalar needs its full
// record. This generated schema has two distinct types and declarations.
func testSchemaScratchRetainedFacts(declaredChildren int) int64 {
	const cacheHeaders = 2 * 256
	const typeSlotsAndRecords = 8*64 + 2*128
	const declarationSlotsAndRecords = 8*64 + 2*96
	const unavailableRootScalar = 192
	const knownScalarEvidence = 64
	return cacheHeaders + typeSlotsAndRecords + declarationSlotsAndRecords + unavailableRootScalar + int64(declaredChildren)*knownScalarEvidence
}

func TestSchemaBindingScratchCanceledBeforeTraversal(t *testing.T) {
	for _, children := range []int{1, 32, 129} {
		t.Run(fmt.Sprint(children), func(t *testing.T) {
			s := schemaScratchFixture(t, children, "string", Budgets{})
			before := s.retained
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := s.bindSourceSchema(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("binding cancellation: %v", err)
			}
			// Only the persistent schema-node map may be newly session-owned.
			want := before + 128 + int64(len(s.nodes))*128
			if s.retained != want {
				t.Errorf("DFS scratch retained until Finish: got %d, want %d", s.retained, want)
			}
			// NewRun's 1024-byte state remains owned until session cleanup.
			used, _ := s.account.Usage(BudgetBufferedBytes)
			if used != want+1024 {
				t.Errorf("canceled traversal scratch still live: got %d, want %d", used, want+1024)
			}
			s.cleanup()
			if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("cleanup leaked %d bytes", used)
			}
		})
	}
}

func TestSchemaBindingScratchReleasesCompletedTraversal(t *testing.T) {
	for _, children := range []int{1, 32, 129} {
		t.Run(fmt.Sprint(children), func(t *testing.T) {
			s := schemaScratchFixture(t, children, "string", Budgets{})
			before := s.retained
			if err := s.bindSourceSchema(context.Background()); err != nil {
				t.Fatal(err)
			}
			// Persistent map + subject projections + owned scalar strings.
			// Borrowed witnesses remain charged by Run; consumed completion
			// arrays and the depth-bounded DFS stack must not outlive traversal.
			want := before + 128 + int64(children+1)*(128+256) + int64(children)*(64+129) + testSchemaScratchRetainedFacts(children)
			if s.retained != want {
				t.Errorf("completed traversal retained scratch: got %d, want %d", s.retained, want)
			}
			root := s.schema.nodes[1]
			if witnesses := s.schema.run.Witnesses(root.binding); len(witnesses) != children {
				t.Fatalf("lost retained witnesses: %d, want %d", len(witnesses), children)
			}
			if result := s.schema.run.Results(root.binding).Content; !result.Valid || !result.Complete {
				t.Fatalf("lost content result: %+v", result)
			}
			if err := s.schema.run.Close(); err != nil {
				t.Fatal(err)
			}
			s.schema.run = nil
			if used, _ := s.account.Usage(BudgetBufferedBytes); used != want {
				t.Errorf("borrowed witnesses or traversal scratch outlived owner: got %d, want %d", used, want)
			}
		})
	}
}
