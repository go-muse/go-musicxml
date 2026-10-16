package validation

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// The fixture includes same-named uses and changed inherited requiredness. Only
// the exact active owner can prove an omitted required slot without a runtime
// AttributeResult; inherited slots deliberately retain conservative evidence.
func contextRequiredFixture(t *testing.T, actual string, attrs map[QName]string, complete bool, checks ...XSDCheck) (*Session, *contextBindings) {
	t.Helper()
	integer := SchemaTypeRef{Name: SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "integer"}}
	base := SchemaTypeRef{Name: SchemaName{Local: "A"}}
	required := SchemaAttribute{Name: SchemaName{Local: "type"}, Type: integer, Use: SchemaAttributeRequired}
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{
		Types: map[SchemaName]*SchemaType{
			{Local: "A"}: {Complex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{required,
				{Name: SchemaName{Local: "number"}, Type: integer, Constraint: &SchemaValueConstraint{Kind: "default", Value: "1"}},
				{Name: SchemaName{Local: "other"}, Type: integer},
			}}},
			{Local: "B"}:          {Complex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{required}}},
			{Local: "inherited"}:  {Complex: &SchemaComplex{Form: SchemaComplexExtension, Base: &base}},
			{Local: "optional"}:   {Complex: &SchemaComplex{Form: SchemaComplexRestriction, Base: &base, Attributes: []SchemaAttribute{{Name: required.Name, Type: integer, Use: SchemaAttributeOptional}}}},
			{Local: "prohibited"}: {Complex: &SchemaComplex{Form: SchemaComplexRestriction, Base: &base, Attributes: []SchemaAttribute{{Name: required.Name, Type: integer, Use: SchemaAttributeProhibited}}}},
		},
		Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{Name: SchemaName{Local: actual}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := compileContextBindingSpecs(schema, []contextBindingSpec{
		{Role: "required", Owner: "A", Name: "type", Type: "xs:integer", Attribute: true, Count: 1},
		{Role: "different", Owner: "B", Name: "type", Type: "xs:integer", Attribute: true, Count: 1},
		{Role: "number", Owner: "A", Name: "number", Type: "xs:integer", Attribute: true, Count: 1},
		{Role: "other", Owner: "A", Name: "other", Type: "xs:integer", Attribute: true, Count: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := sourceSchemaEngineFor(t, schema, checks...).Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.cleanup)
	if err = s.Open(context.Background(), schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	for name, value := range attrs {
		a := AttributeFact{Owner: 1, Name: name, Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "schema-binding-pending"}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceKnown, Value: value}, Value: Evidence[Atomic]{State: EvidenceUnknown, Reason: "schema-binding-pending"}}, Location: Location{Target: TargetSource, DocumentID: "source", Path: "/root/@" + name.Local}}
		if err = s.Attribute(context.Background(), a); err != nil {
			t.Fatal(err)
		}
	}
	end := schemaLexemeEnd("")
	end.AttributesComplete.Value = complete
	if err = s.Close(context.Background(), end); err != nil {
		t.Fatal(err)
	}
	return s, bindings
}

func TestContextRequiredAttributeExactOwnerCause(t *testing.T) {
	for _, attrs := range []map[QName]string{nil, {{Local: "other"}: "bad"}, {{Space: "urn:foreign", Local: "type"}: "1"}} {
		s, b := contextRequiredFixture(t, "A", attrs, true, XSDCheckAttributes)
		r := &contextReader{ctx: context.Background(), s: s, b: b}
		got := r.attribute(s.nodes[1], "required")
		want := DecisionRef{Rule: RuleKey{ID: "test.xsd.attributes", Revision: 1}, Subject: NodeRef{DocumentID: "source", Node: 1}}
		if r.err != nil || got.State != EvidenceInvalid || got.Cause == nil || *got.Cause != want {
			t.Errorf("exact missing required cause: %+v / %v", got, r.err)
		}
		optional := r.attribute(s.nodes[1], "number")
		if optional.State != EvidenceKnown || optional.Value.Integer.String() != "1" {
			t.Errorf("optional default lost: %+v", optional)
		}
	}
}

func TestContextRequiredAttributeConservativeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, actual    string
		complete, proof bool
		change          func(*Session, *contextBindings)
	}{
		{name: "inherited", actual: "inherited", complete: true, proof: true},
		{name: "derived-optional", actual: "optional", complete: true, proof: true},
		{name: "derived-prohibited", actual: "prohibited", complete: true, proof: true},
		{name: "forged-derived-owner", actual: "inherited", complete: true, proof: true, change: func(s *Session, b *contextBindings) {
			s.nodes[1].header.Type = testEvidence(Evidence[ComponentID]{State: EvidenceKnown, Value: contextNamedType("A")})
		}},

		{name: "different-schema-graph", actual: "A", complete: true, proof: true, change: func(s *Session, b *contextBindings) {
			_, different := contextRequiredFixture(t, "B", nil, true, XSDCheckAttributes)
			*b = *different
		}},
		{name: "uninspected", actual: "A", proof: true},
		{name: "no-attribute-proof", actual: "A", complete: true},
		{name: "wrong-selector", actual: "A", complete: true, proof: true, change: func(s *Session, b *contextBindings) {
			b.Attributes["required"][contextNamedType("A")] = b.Attributes["different"][contextNamedType("B")]
		}},
		{name: "wrong-assembly", actual: "A", complete: true, proof: true, change: func(s *Session, b *contextBindings) {
			copy := *s.schema.active
			copy.assembly = AssemblyOpus
			s.schema.active = &copy
		}},
		{name: "no-runtime-binding", actual: "A", complete: true, proof: true, change: func(s *Session, b *contextBindings) { delete(s.schema.nodes, 1) }},
		{name: "same-name-present-wrong-declaration", actual: "A", complete: true, proof: true, change: func(s *Session, b *contextBindings) {
			a := b.Attributes["different"][contextNamedType("B")]
			s.nodes[1].attributes = append(s.nodes[1].attributes, AttributeFact{Name: a.Name, Declaration: Evidence[DeclarationID]{State: EvidenceKnown, Value: a.Declaration}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}})
		}},
		{name: "same-name-unknown-presence", actual: "A", complete: true, proof: true, change: func(s *Session, b *contextBindings) {
			a := b.Attributes["different"][contextNamedType("B")]
			s.nodes[1].attributes = append(s.nodes[1].attributes, AttributeFact{Name: a.Name, Declaration: Evidence[DeclarationID]{State: EvidenceKnown, Value: a.Declaration}, Presence: Evidence[bool]{State: EvidenceUnknown, Reason: "unknown"}})
		}},
		{name: "unclosed", actual: "A", complete: true, proof: true, change: func(s *Session, b *contextBindings) { s.nodes[1].closed = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checks := []XSDCheck{XSDCheckContent}
			if tc.proof {
				checks = append(checks, XSDCheckAttributes)
			}
			s, b := contextRequiredFixture(t, tc.actual, nil, tc.complete, checks...)
			if tc.change != nil {
				tc.change(s, b)
			}
			r := &contextReader{ctx: context.Background(), s: s, b: b}
			got := r.attribute(s.nodes[1], "required")
			if r.err != nil || got.State == EvidenceInvalid || got.State == EvidenceKnown {
				t.Errorf("unproved required absence upgraded: %+v / %v", got, r.err)
			}
		})
	}
}

func TestContextRequiredAttributeCancellation(t *testing.T) {
	s, b := contextRequiredFixture(t, "A", nil, true, XSDCheckAttributes)
	probe := &rationalCancelContext{Context: context.Background(), at: 1000000}
	r := &contextReader{ctx: probe, s: s, b: b}
	_ = r.attribute(s.nodes[1], "required")
	if r.err != nil {
		t.Fatal(r.err)
	}
	for at := 1; at <= probe.calls; at++ {
		r = &contextReader{ctx: &rationalCancelContext{Context: context.Background(), at: at}, s: s, b: b}
		_ = r.attribute(s.nodes[1], "required")
		if !errors.Is(r.err, context.Canceled) {
			t.Fatalf("poll %d: %v", at, r.err)
		}
		r.ctx = context.Background()
		_ = r.attribute(s.nodes[1], "required")
		if !errors.Is(r.err, context.Canceled) {
			t.Fatal("lost first cancellation")
		}
	}
}

// Catalog interning may select either independently compiled equal-digest
// schema. Required-use evidence must not depend on private kernel addresses.
func TestContextRequiredAttributeEquivalentSchemaInterning(t *testing.T) {
	for _, contextFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("context-first-%t", contextFirst), func(t *testing.T) {
			original, _ := contextRequiredFixture(t, "A", nil, true, XSDCheckAttributes)
			other, b := contextRequiredFixture(t, "A", nil, true, XSDCheckAttributes)
			if original.schema.active.kernel == other.schema.active.kernel || original.schema.active.Digest() != other.schema.active.Digest() {
				t.Fatal("fixture needs independently compiled identical schemas")
			}
			catalog := original.catalog
			profile, _ := catalog.Profile("source-schema-test")
			profile.CatalogDigest = ""
			var rules []RuleDefinition
			var plans []PlanBinding
			for _, key := range profile.Required {
				rule, _ := catalog.Rule(key)
				rules = append(rules, rule)
				plans = append(plans, PlanBinding{ID: rule.Predicate, Plan: catalog.plans[rule.Predicate]})
			}
			key := RuleKey{ID: "required-context", Revision: 1}
			assembly := AssemblyMusicXML
			rule := RuleDefinition{Key: key, Targets: []Target{TargetSource}, Scope: ScopeDocument, Role: RoleContextProvider, Class: ClassMandatory, Category: CategoryNormative, Selector: Selector{Assembly: &assembly}, Predicate: string(key.ID), Text: RuleText{Title: "Required context", Explanation: "Exact required-use context interning.", Message: "Context unavailable."}, Sources: rules[0].Sources}
			rules = append(rules, rule)
			binding := PlanBinding{ID: rule.Predicate, Plan: Plan{kind: planContextLocal, context: &contextPlan{Schema: other.schema.active, Bindings: b, revision: contextSemanticRevision}}}
			if contextFirst {
				plans = append([]PlanBinding{binding}, plans...)
			} else {
				plans = append(plans, binding)
			}
			profile.Required = append(profile.Required, key)
			profile.Support = append(profile.Support, RuleSupport{Rule: key, State: ImplementationAvailable})
			compiled, err := NewCatalog(rules, []Profile{profile}, plans)
			if err != nil {
				t.Fatal(err)
			}
			contextPlan := compiled.plans[rule.Predicate]
			wantSchema := original.schema.active.kernel
			if contextFirst {
				wantSchema = other.schema.active.kernel
			}
			if contextPlan.context.Schema.kernel != wantSchema {
				t.Fatal("catalog did not canonicalize expected schema")
			}
			engine, err := NewEngine(compiled)
			if err != nil {
				t.Fatal(err)
			}
			s, err := engine.Begin(context.Background(), RunOptions{DocumentID: "source", Profile: profile.ID})
			if err != nil {
				t.Fatal(err)
			}
			defer s.cleanup()
			if err = s.Open(context.Background(), schemaPendingHeader()); err != nil {
				t.Fatal(err)
			}
			if err = s.Close(context.Background(), schemaLexemeEnd("")); err != nil {
				t.Fatal(err)
			}
			r := &contextReader{ctx: context.Background(), s: s, b: contextPlan.context.Bindings}
			got := r.attribute(s.nodes[1], "required")
			if r.err != nil || got.State != EvidenceInvalid || got.Cause == nil {
				t.Errorf("equivalent-schema interning lost required cause: %+v / %v", got, r.err)
			}
		})
	}
}
