package validation

import (
	"context"
	"errors"
	"testing"
)

func independentContextFallbackFixture(t *testing.T) (*Session, *contextBindings) {
	t.Helper()
	xs := SchemaTypeRef{Name: SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"}}
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{
		Types: map[SchemaName]*SchemaType{
			{Local: "A"}: {Complex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{{Name: SchemaName{Local: "id"}, Type: xs}}}},
			{Local: "B"}: {Complex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{{Name: SchemaName{Local: "id"}, Type: xs}}}},
		},
		Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Particle: &SchemaParticle{Kind: SchemaParticleSequence, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Children: []*SchemaParticle{
			{Kind: SchemaParticleElement, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Element: &SchemaElement{Name: SchemaName{Local: "a"}, Type: SchemaTypeRef{Name: SchemaName{Local: "A"}}}},
			{Kind: SchemaParticleElement, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Element: &SchemaElement{Name: SchemaName{Local: "b"}, Type: SchemaTypeRef{Name: SchemaName{Local: "B"}}}},
		}}}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := compileContextBindingSpecs(schema, []contextBindingSpec{{Role: "a-id", Owner: "A", Name: "id", Type: "xs:string", Attribute: true, Count: 1}, {Role: "b-id", Owner: "B", Name: "id", Type: "xs:string", Attribute: true, Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	s, err := sourceSchemaEngineFor(t, schema, XSDCheckAttributes, XSDCheckContent).Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Open(context.Background(), schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"a", "b"} {
		h := schemaPendingHeader()
		h.ID = NodeID(i + 2)
		h.Parent = 1
		h.Name.Local = name
		if err = s.Open(context.Background(), h); err != nil {
			t.Fatal(err)
		}
		end := schemaLexemeEnd("")
		end.ID = h.ID
		if err = s.Close(context.Background(), end); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Close(context.Background(), schemaLexemeEnd("")); err != nil {
		t.Fatal(err)
	}
	// Force the authoritative fallback instead of its static owner fast path.
	// Equal public type evidence must not merge two distinct exact use bindings.
	for _, id := range []NodeID{2, 3} {
		s.nodes[id].header.Type = Evidence[ComponentID]{State: EvidenceUnknown, Reason: "independent-fallback-probe"}
	}
	return s, bindings
}

func TestIndependentContextAttributeFallbackBindingIsolation(t *testing.T) {
	s, bindings := independentContextFallbackFixture(t)
	defer s.cleanup()
	reader := &contextReader{ctx: context.Background(), s: s, b: bindings}
	for round := 0; round < 3; round++ {
		for _, tc := range []struct {
			id   NodeID
			role contextRole
			want bool
		}{{2, "b-id", false}, {2, "a-id", true}, {3, "a-id", false}, {3, "b-id", true}} {
			got, ok := reader.attributeSelector(s.nodes[tc.id], tc.role)
			if reader.err != nil {
				t.Fatal(reader.err)
			}
			if ok != tc.want {
				t.Fatalf("round%d node%d role%s: got%t want%t", round, tc.id, tc.role, ok, tc.want)
			}
			if ok && got.Declaration != bindings.Roles[tc.role][0].Declaration {
				t.Fatal("same-named owner selector replaced exact declaration")
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader.ctx = ctx
	_, _ = reader.attributeSelector(s.nodes[2], "a-id")
	if !errors.Is(reader.err, context.Canceled) {
		t.Fatalf("cached positive lookup lost cancellation: %v", reader.err)
	}
}

func TestIndependentContextEmptyAdviceIndexCancellation(t *testing.T) {
	for _, service := range []*timelineService{nil, {}} {
		s := rationalSession(t, Budgets{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := buildPerformanceAdviceIndex(ctx, s, service)
		if !errors.Is(err, context.Canceled) || s.retained != 0 {
			t.Errorf("pre-canceled empty performance advice index: err=%v retained=%d", err, s.retained)
		}
		s.cleanup()
	}
}

func TestIndependentContextAttributeCacheNeverPublishesInterruptedEnumeration(t *testing.T) {
	s, b := independentContextFallbackFixture(t)
	defer s.cleanup()
	c := &rationalCancelContext{Context: context.Background(), at: 1000000}
	probe := &contextReader{ctx: c, s: s, b: b}
	if _, ok := probe.attributeSelector(s.nodes[2], "a-id"); !ok || probe.err != nil {
		t.Fatalf("reference fallback: %v", probe.err)
	}
	polls := c.calls
	for at := 1; at <= polls; at++ {
		r := &contextReader{ctx: &rationalCancelContext{Context: context.Background(), at: at}, s: s, b: b}
		_, _ = r.attributeSelector(s.nodes[2], "a-id")
		if !errors.Is(r.err, context.Canceled) || len(r.attributeCache) != 0 {
			t.Fatalf("poll%d cached incomplete attribute enumeration: err=%v cache=%d", at, r.err, len(r.attributeCache))
		}
		r.ctx = context.Background()
		_, _ = r.attributeSelector(s.nodes[2], "a-id")
		if !errors.Is(r.err, context.Canceled) || len(r.attributeCache) != 0 {
			t.Fatal("first interruption was reset or cached")
		}
	}
	t.Logf("interrupted all %d fallback cache polls without ready publication", polls)
}
