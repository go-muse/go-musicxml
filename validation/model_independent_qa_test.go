package validation

import (
	"context"
	"math/big"
	"testing"
)

func independentModelEngine(t *testing.T, schema *CompiledSchema) *Engine {
	t.Helper()
	old := sourceSchemaEngineFor(t, schema, XSDCheckBinding, XSDCheckDeclaration, XSDCheckAttributes, XSDCheckScalar, XSDCheckContent, XSDCheckIdentity)
	profile, _ := old.catalog.Profile("source-schema-test")
	profile.Target = TargetModel
	profile.ID = "independent-model"
	profile.CatalogDigest = ""
	var rules []RuleDefinition
	var plans []PlanBinding
	for _, key := range profile.Required {
		r, _ := old.catalog.Rule(key)
		r.Targets = []Target{TargetModel}
		rules = append(rules, r)
		plans = append(plans, PlanBinding{ID: r.Predicate, Plan: old.catalog.plans[r.Predicate]})
	}
	assembly := AssemblyMusicXML
	key := RuleKey{ID: "independent.model-representation", Revision: 1}
	p, err := NewModelRepresentationPlan(assembly)
	if err != nil {
		t.Fatal(err)
	}
	r := RuleDefinition{Key: key, Targets: []Target{TargetModel}, Scope: ScopeNode, Category: CategoryModel, Class: ClassMandatory, Role: RolePredicate, Selector: Selector{Assembly: &assembly}, Predicate: string(key.ID), Text: RuleText{Title: "Model representation", Explanation: "Check current representation.", Message: "Invalid representation."}, Sources: rules[0].Sources}
	rules = append(rules, r)
	plans = append(plans, PlanBinding{ID: r.Predicate, Plan: p})
	profile.Required = append(profile.Required, key)
	profile.Support = append(profile.Support, RuleSupport{Rule: key, State: ImplementationAvailable})
	catalog, err := NewCatalog(rules, []Profile{profile}, plans)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}
func independentModelHeader(id, parent NodeID, name, path string) NodeHeader {
	h := schemaPendingHeader()
	h.ID = id
	h.Parent = parent
	h.Name = QName{Local: name}
	h.Location = Location{Target: TargetModel, DocumentID: "model", Path: path}
	return h
}
func independentModelEnd(id NodeID) NodeEnd {
	return NodeEnd{ID: id, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceUnsupported, Reason: "model-no-source"}, Value: Evidence[Atomic]{State: EvidenceAbsent}}, ChildrenComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, AttributesComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, Order: Evidence[OrderKind]{State: EvidenceKnown, Value: OrderModelFields}}
}
func independentModelAttribute(id NodeID, name, text string) AttributeFact {
	return AttributeFact{Owner: id, Name: QName{Local: name}, Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "schema-binding-pending"}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceUnsupported, Reason: "model-no-source"}, Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: text}}}, Location: Location{Target: TargetModel, DocumentID: "model", Path: "Root." + name}}
}

func TestIndependentModelScopedAttributeCauseAndDefault(t *testing.T) {
	integer := SchemaTypeRef{Name: SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "integer"}}
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{
		{Name: SchemaName{Local: "broken"}, Type: integer}, {Name: SchemaName{Local: "other"}, Type: integer}, {Name: SchemaName{Local: "good"}, Type: integer}, {Name: SchemaName{Local: "defaulted"}, Type: integer, Constraint: &SchemaValueConstraint{Kind: "default", Value: "7"}},
	}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s, err := independentModelEngine(t, schema).Begin(ctx, RunOptions{DocumentID: "model", Profile: "independent-model"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Open(ctx, independentModelHeader(1, 0, "root", "Root")); err != nil {
		t.Fatal(err)
	}
	broken := independentModelAttribute(1, "broken", "")
	broken.Scalar.Value = Evidence[Atomic]{State: EvidenceUnsupported, Reason: "model-nonfinite"}
	for _, a := range []AttributeFact{broken, independentModelAttribute(1, "other", "bad"), independentModelAttribute(1, "good", "0004")} {
		if err := s.Attribute(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	fact := ModelRepresentationFact{Owner: 1, Complete: Evidence[bool]{State: EvidenceKnown, Value: true}, Problems: []ModelProblem{{Code: "model-nonfinite", Axis: ModelAxisAttribute, Attribute: QName{Local: "broken"}, Location: broken.Location, Actual: "NaN"}}}
	if err := s.ModelRepresentation(ctx, fact); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(ctx, independentModelEnd(1)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"broken", "other", "good", "defaulted"} {
		got, err := s.schemaEffectiveScalar(ctx, 1, QName{Local: name})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %+v", name, got)
		switch name {
		case "broken":
			if got.State != EvidenceInvalid || got.Cause == nil || got.Cause.Rule.ID != "independent.model-representation" {
				t.Errorf("wrong scoped representation cause: %+v", got)
			}
		case "other":
			if got.State != EvidenceInvalid || got.Cause == nil || got.Cause.Rule.ID != "test.xsd.attributes" {
				t.Errorf("independent attribute failure lost: %+v", got)
			}
		case "good", "defaulted":
			want := "4"
			if name == "defaulted" {
				want = "7"
			}
			if got.State != EvidenceKnown || got.Value.Kind != AtomicInteger || got.Value.Integer.String() != want {
				t.Errorf("independent valid/default fact lost: %+v", got)
			}
		}
	}
	for _, a := range s.nodes[1].attributes {
		if a.Name.Local == "good" && (a.Scalar.Value.Value.Text != "0004" || a.Scalar.Lexeme.State == EvidenceKnown) {
			t.Errorf("current raw fact overwritten: %+v", a)
		}
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxNotAssessed})
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts.Fail == 0 || report.Conformance != ConformanceViolated || report.BlockingProblems == 0 {
		t.Errorf("independent model/XSD axes lost: %+v", report)
	}
}

func TestIndependentModelKnownAttributeFailureWithPatternLimitKeepsCause(t *testing.T) {
	pattern := &SchemaSimple{Form: SchemaSimpleRestriction, Base: &SchemaSimpleMember{Name: SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "integer"}}, Patterns: []string{"[0-9]+"}, HasMaxInclusive: true, MaxInclusive: "3"}
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{{Name: SchemaName{Local: "n"}, Type: SchemaTypeRef{InlineSimple: pattern}}}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s, err := independentModelEngine(t, schema).Begin(ctx, RunOptions{DocumentID: "model", Profile: "independent-model"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Open(ctx, independentModelHeader(1, 0, "root", "Root")); err != nil {
		t.Fatal(err)
	}
	integer, err := NewInteger(big.NewInt(4))
	if err != nil {
		t.Fatal(err)
	}
	a := independentModelAttribute(1, "n", "")
	a.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: integer}}
	if err := s.Attribute(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := s.ModelRepresentation(ctx, ModelRepresentationFact{Owner: 1, Complete: Evidence[bool]{State: EvidenceKnown, Value: true}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(ctx, independentModelEnd(1)); err != nil {
		t.Fatal(err)
	}
	got, err := s.schemaEffectiveScalar(ctx, 1, QName{Local: "n"})
	if err != nil {
		t.Fatal(err)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxNotAssessed})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("effective=%+v; counts=%+v; findings=%+v", got, report.Counts, report.Findings)
	if report.Counts.Fail == 0 || report.AssessmentComplete {
		t.Fatalf("known failure/incompleteness control failed: %+v", report)
	}
	if got.State != EvidenceInvalid || got.Cause == nil || got.Cause.Rule.ID != "test.xsd.attributes" {
		t.Errorf("known attribute failure lost causal evidence when independent lexical facet was unavailable: %+v", got)
	}
}
