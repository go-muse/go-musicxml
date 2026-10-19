package validation

import (
	"context"
	"testing"
)

func qaSchemaEngine(t *testing.T, schema *CompiledSchema, checks ...XSDCheck) *Engine {
	t.Helper()
	var rules []RuleDefinition
	var bindings []PlanBinding
	profile := Profile{ID: "source-schema-test", Target: TargetSource, MusicXMLVersion: "4.0", RegistryCommit: "3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c", SupportedRoots: []QName{{Local: "root"}}, NormativeClosure: "Only the explicitly selected source XSD composite families on this miniature schema."}
	for _, check := range checks {
		plan, err := NewXSDPlan(schema, check)
		if err != nil {
			t.Fatal(err)
		}
		key := RuleKey{ID: RuleID("test.xsd." + string(check)), Revision: 1}
		assembly := AssemblyMusicXML
		rule := RuleDefinition{Key: key, Targets: []Target{TargetSource}, Scope: ScopeNode, Category: CategoryNormative, Class: ClassMandatory, Role: RolePredicate,
			Selector: Selector{Assembly: &assembly}, Predicate: string(key.ID), Text: RuleText{Title: "Source XSD", Explanation: "Assess the selected XSD composite operation.", Message: "XSD source assessment."}, Sources: []SourceRef{{IDs: []string{"XSD-1.0"}, URL: "https://www.w3.org/TR/xmlschema-1/", Locator: string(check)}}}
		if check == XSDCheckIdentity {
			rule.Scope = ScopeDocument
		}
		rules = append(rules, rule)
		bindings = append(bindings, PlanBinding{ID: rule.Predicate, Plan: plan})
		profile.Required = append(profile.Required, key)
		profile.Support = append(profile.Support, RuleSupport{Rule: key, State: ImplementationAvailable})
	}
	catalog, err := NewCatalog(rules, []Profile{profile}, bindings)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func qaDuplicateIDReport(t *testing.T, requires bool) Report {
	t.Helper()
	ctx := context.Background()
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Particle: &SchemaParticle{Kind: SchemaParticleElement, Occurrence: SchemaOccurrence{Min: 2, Max: 2}, Element: &SchemaElement{Name: SchemaName{Local: "child"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{{Name: SchemaName{Local: "id"}, Type: SchemaTypeRef{Name: SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "ID"}}, Use: SchemaAttributeRequired}}}}}}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	engine := qaSchemaEngine(t, schema, XSDCheckIdentity)
	if requires {
		key := RuleKey{ID: "test.xsd.identity", Revision: 1}
		rule, _ := engine.catalog.Rule(key)
		rule.Requires = []Dependency{{Kind: DependencyExternal, Key: "identity-service", Relation: RelationDocument}}
		profile, _ := engine.catalog.Profile("source-schema-test")
		profile.CatalogDigest = ""
		plan, err := NewXSDPlan(schema, XSDCheckIdentity)
		if err != nil {
			t.Fatal(err)
		}
		catalog, err := NewCatalog([]RuleDefinition{rule}, []Profile{profile}, []PlanBinding{{ID: rule.Predicate, Plan: plan}})
		if err != nil {
			t.Fatal(err)
		}
		engine, err = NewEngine(catalog)
		if err != nil {
			t.Fatal(err)
		}
	}
	s, err := engine.Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	for id := NodeID(2); id <= 3; id++ {
		h := schemaPendingHeader()
		h.ID = id
		h.Parent = 1
		h.Name = QName{Local: "child"}
		h.Location.Path = "/root/child"
		if err = s.OpenNode(ctx, h); err != nil {
			t.Fatal(err)
		}
		a := AttributeFact{Owner: id, Name: QName{Local: "id"}, Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "schema-binding-pending"}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceKnown, Value: "same"}, Value: Evidence[Atomic]{State: EvidenceUnknown, Reason: "schema-binding-pending"}}, Location: h.Location}
		if err = s.Attribute(ctx, a); err != nil {
			t.Fatal(err)
		}
		end := schemaLexemeEnd("")
		end.ID = id
		if err = s.CloseNode(ctx, end); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.CloseNode(ctx, schemaLexemeEnd("")); err != nil {
		t.Fatal(err)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	return report
}
func TestIndependentDuplicateIDDoesNotInventIncompleteWork(t *testing.T) {
	report := qaDuplicateIDReport(t, false)
	if report.Counts.Fail != 1 || !report.AssessmentComplete || report.BlockingProblems != 0 {
		t.Fatalf("duplicate has known completed verdict, got %+v", report)
	}
}
func TestIndependentIdentityPreviewHonorsRequires(t *testing.T) {
	report := qaDuplicateIDReport(t, true)
	if report.Counts.Fail != 0 || report.Counts.Unsupported != 1 || report.AssessmentComplete {
		t.Fatalf("identity Requires was bypassed: %+v", report)
	}
}

func TestIndependentCatalogOwnsSchemaHandle(t *testing.T) {
	ctx := context.Background()
	schema := miniatureSourceSchema(t)
	engine := qaSchemaEngine(t, schema, XSDCheckScalar)
	before, _ := engine.catalog.Profile("source-schema-test")
	replacement, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{Name: SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	// No private field mutation: callers can assign any public opaque handle value.
	*schema = *replacement
	session, err := engine.Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	if err := session.CloseNode(ctx, schemaLexemeEnd("not an integer")); err != nil {
		t.Fatal(err)
	}
	report, err := session.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts.Fail != 1 || report.Profile.CatalogDigest != before.CatalogDigest {
		t.Fatalf("catalog behavior changed through caller schema handle assignment: %+v", report)
	}
}

func TestIndependentAbsentAttributeIsNotInventedBySchemaBridge(t *testing.T) {
	ctx := context.Background()
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect}}}}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := qaSchemaEngine(t, schema, XSDCheckAttributes).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	h := schemaPendingHeader()
	if err := session.OpenNode(ctx, h); err != nil {
		t.Fatal(err)
	}
	a := AttributeFact{Owner: 1, Name: QName{Local: "absent"}, Declaration: Evidence[DeclarationID]{State: EvidenceAbsent}, Presence: Evidence[bool]{State: EvidenceAbsent}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceAbsent}, Value: Evidence[Atomic]{State: EvidenceAbsent}}, Location: h.Location}
	if err := session.Attribute(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := session.CloseNode(ctx, schemaLexemeEnd("")); err != nil {
		t.Fatal(err)
	}
	report, err := session.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Satisfied() || report.Counts.Fail != 0 {
		t.Fatalf("absent attribute was treated as present: %+v", report)
	}
}

func TestIndependentUnknownOrderDoesNotProveSequenceViolation(t *testing.T) {
	ctx := context.Background()
	particle := &SchemaParticle{Kind: SchemaParticleSequence, Occurrence: SchemaOccurrence{Min: 1, Max: 1}}
	for _, name := range []string{"a", "b"} {
		particle.Children = append(particle.Children, &SchemaParticle{Kind: SchemaParticleElement, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Element: &SchemaElement{Name: SchemaName{Local: name}, Type: SchemaTypeRef{Name: SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"}}}})
	}
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Particle: particle}}}}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := qaSchemaEngine(t, schema, XSDCheckContent).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"b", "a"} {
		h := schemaPendingHeader()
		h.ID = NodeID(i + 2)
		h.Parent = 1
		h.Name = QName{Local: name}
		h.Location.Path = "/root/" + name
		if err := session.OpenNode(ctx, h); err != nil {
			t.Fatal(err)
		}
		end := schemaLexemeEnd("")
		end.ID = h.ID
		if err := session.CloseNode(ctx, end); err != nil {
			t.Fatal(err)
		}
	}
	end := schemaLexemeEnd("")
	end.Order = Evidence[OrderKind]{State: EvidenceUnsupported, Reason: "original-order-unavailable"}
	if err := session.CloseNode(ctx, end); err != nil {
		t.Fatal(err)
	}
	report, err := session.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts.Fail != 0 || report.AssessmentComplete {
		t.Fatalf("unavailable source order invented a sequence failure: %+v", report)
	}
}
