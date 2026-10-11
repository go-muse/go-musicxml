package validation

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

func TestModelSchemaBridgeKeepsCurrentValue(t *testing.T) {
	ctx := context.Background()
	account, _ := NewBudgetAccount(Budgets{})
	s := &Session{profile: Profile{Target: TargetModel}, account: account}
	exact, err := NewDecimal(new(big.Rat).SetFloat64(0.1))
	if err != nil {
		t.Fatal(err)
	}
	n := &factNode{header: NodeHeader{ID: 1, Name: QName{Local: "root"}}, closed: true, end: NodeEnd{Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceUnsupported, Reason: "no-source"}, Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicDecimal, Decimal: exact}}}, AttributesComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, ChildrenComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, Order: Evidence[OrderKind]{State: EvidenceKnown, Value: OrderModelFields}}}
	got, err := s.schemaSubject(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	if !got.CurrentModel || !got.Current.Known || got.TextKnown || got.Current.Value.Domain != xsd.NumberDomain || !got.ChildrenComplete || !got.ChildrenOrderKnown {
		t.Fatalf("subject=%+v", got)
	}
	if got.Current.Value.Number.Compare(xsd.Decimal{Fraction: "1"}) <= 0 {
		t.Fatal("binary-exact current 0.1 was rounded")
	}
	n.end.Order = Evidence[OrderKind]{State: EvidenceUnsupported, Reason: "content-hole"}
	got, err = s.schemaSubject(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	if got.ChildrenOrderKnown || got.ChildrenComplete {
		t.Fatalf("hole was compacted into ordered input: %+v", got)
	}
}

func modelSchemaSession(t *testing.T, schema *CompiledSchema, budgets Budgets) *Session {
	t.Helper()
	source := sourceSchemaEngineFor(t, schema, XSDCheckBinding, XSDCheckDeclaration, XSDCheckAttributes, XSDCheckScalar, XSDCheckContent, XSDCheckIdentity)
	profile, _ := source.catalog.Profile("source-schema-test")
	profile.ID = "model-schema-test"
	profile.Target = TargetModel
	profile.CatalogDigest = ""
	var rules []RuleDefinition
	var plans []PlanBinding
	for _, key := range profile.Required {
		rule, _ := source.catalog.Rule(key)
		rule.Targets = []Target{TargetModel}
		rules = append(rules, rule)
		plans = append(plans, PlanBinding{ID: rule.Predicate, Plan: source.catalog.plans[rule.Predicate]})
	}
	assembly := AssemblyMusicXML
	key := RuleKey{ID: "test.model-representation", Revision: 1}
	plan, err := NewModelRepresentationPlan(assembly)
	if err != nil {
		t.Fatal(err)
	}
	rules = append(rules, RuleDefinition{Key: key, Targets: []Target{TargetModel}, Scope: ScopeNode, Class: ClassMandatory, Role: RolePredicate, Category: CategoryModel, Selector: Selector{Assembly: &assembly}, Predicate: "model-representation", Sources: rules[0].Sources, Text: RuleText{Title: "Model", Explanation: "Assess current representation.", Message: "Model representation issue."}})
	plans = append(plans, PlanBinding{ID: "model-representation", Plan: plan})
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
	s, err := engine.Begin(context.Background(), RunOptions{DocumentID: "model", Profile: profile.ID, Budgets: budgets})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func modelSchemaHeader(id, parent NodeID, name, path string) NodeHeader {
	return NodeHeader{ID: id, Parent: parent, Name: QName{Local: name}, Type: Evidence[ComponentID]{State: EvidenceUnsupported, Reason: "model-schema-witness-required"}, Declaration: Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "model-schema-witness-required"}, Location: Location{Target: TargetModel, DocumentID: "model", Path: path}}
}
func modelSchemaEnd(id NodeID, value Evidence[Atomic], order OrderKind) NodeEnd {
	return NodeEnd{ID: id, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceUnsupported, Reason: "model-source-lexeme-unavailable"}, Value: value}, ChildrenComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, AttributesComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, Order: Evidence[OrderKind]{State: EvidenceKnown, Value: order}}
}
func modelSchemaClose(t *testing.T, s *Session, end NodeEnd, problems ...ModelProblem) {
	t.Helper()
	if err := s.ModelRepresentation(context.Background(), ModelRepresentationFact{Owner: end.ID, Complete: Evidence[bool]{State: EvidenceKnown, Value: true}, Problems: problems}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(context.Background(), end); err != nil {
		t.Fatal(err)
	}
}
func TestModelSchemaBindsPendingCurrentRoot(t *testing.T) {
	s := modelSchemaSession(t, miniatureSourceSchema(t), Budgets{})
	ctx := context.Background()
	if err := s.Open(ctx, modelSchemaHeader(1, 0, "root", "Root")); err != nil {
		t.Fatal(err)
	}
	value, _ := NewInteger(big.NewInt(0))
	input := Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: value}}
	modelSchemaClose(t, s, modelSchemaEnd(1, input, OrderModelFields))
	if s.nodes[1].header.Declaration.State != EvidenceKnown {
		t.Fatalf("no exact root binding: %+v", s.nodes[1].header)
	}
	if got := s.nodes[1].end.Scalar; got.Lexeme.State != EvidenceUnsupported || got.Value.Value.Integer.String() != "0" {
		t.Fatalf("current input overwritten: %+v", got)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxNotAssessed})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Satisfied() || report.Syntax != SyntaxNotAssessed {
		t.Fatalf("model report=%+v", report)
	}
}

func TestModelSchemaEffectiveValuesRemainDistinctFromInput(t *testing.T) {
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{
		{Name: SchemaName{Local: "text"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: "token"}}},
		{Name: SchemaName{Local: "number"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: "integer"}}},
		{Name: SchemaName{Local: "default"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: "integer"}}, Constraint: &SchemaValueConstraint{Kind: "default", Value: "1"}},
		{Name: SchemaName{Local: "absent"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: "string"}}},
	}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	s := modelSchemaSession(t, schema, Budgets{})
	ctx := context.Background()
	h := modelSchemaHeader(1, 0, "root", "Root")
	if err := s.Open(ctx, h); err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct{ name, text string }{{"text", " a\t b "}, {"number", "bad"}} {
		if err := s.Attribute(ctx, AttributeFact{Owner: 1, Name: QName{Local: pair.name}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Declaration: Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "model-schema-witness-required"}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceUnsupported, Reason: "no-source"}, Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: pair.text}}}, Location: h.Location}); err != nil {
			t.Fatal(err)
		}
	}
	modelSchemaClose(t, s, modelSchemaEnd(1, Evidence[Atomic]{State: EvidenceAbsent}, OrderModelFields))
	if s.nodes[1].attributes[0].Scalar.Value.Value.Text != " a\t b " {
		t.Fatal("raw current text normalized in place")
	}
	for _, pair := range []struct {
		name  string
		state EvidenceState
		text  string
	}{{"text", EvidenceKnown, "a b"}, {"number", EvidenceInvalid, ""}, {"default", EvidenceKnown, "1"}, {"absent", EvidenceAbsent, ""}} {
		got, err := s.schemaEffectiveScalar(ctx, 1, QName{Local: pair.name})
		if err != nil {
			t.Fatal(err)
		}
		if got.State != pair.state {
			t.Fatalf("%s effective=%+v", pair.name, got)
		}
		if pair.state == EvidenceKnown {
			text := got.Value.Text
			if got.Value.Kind == AtomicInteger {
				text = got.Value.Integer.String()
			}
			if text != pair.text {
				t.Errorf("%s=%q", pair.name, text)
			}
		}
		if pair.state == EvidenceInvalid && got.Cause == nil {
			t.Fatal("invalid without actual scalar cause")
		}
	}
	if len(s.nodes[1].attributes) != 2 {
		t.Fatal("effective default mutated model fact attributes")
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxNotAssessed})
	if err != nil {
		t.Fatal(err)
	}
	if report.Conformance != ConformanceViolated {
		t.Fatalf("invalid value disappeared: %+v", report)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("leaked %d", used)
	}
}

func TestModelSchemaContentHoleKeepsIndependentAttributeFailure(t *testing.T) {
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{{Name: SchemaName{Local: "number"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: "integer"}}}}, Particle: &SchemaParticle{Kind: SchemaParticleElement, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Element: &SchemaElement{Name: SchemaName{Local: "child"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: "integer"}}}}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	s := modelSchemaSession(t, schema, Budgets{})
	ctx := context.Background()
	h := modelSchemaHeader(1, 0, "root", "Root")
	if err := s.Open(ctx, h); err != nil {
		t.Fatal(err)
	}
	if err := s.Attribute(ctx, AttributeFact{Owner: 1, Name: QName{Local: "number"}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Declaration: Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "pending"}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceUnsupported, Reason: "no-source"}, Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "bad"}}}, Location: h.Location}); err != nil {
		t.Fatal(err)
	}
	ch := modelSchemaHeader(2, 1, "child", "Root.Content[1]")
	if err := s.Open(ctx, ch); err != nil {
		t.Fatal(err)
	}
	number, _ := NewInteger(big.NewInt(1))
	modelSchemaClose(t, s, modelSchemaEnd(2, Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: number}}, OrderModelFields))
	end := modelSchemaEnd(1, Evidence[Atomic]{State: EvidenceAbsent}, OrderModelContent)
	end.Order = Evidence[OrderKind]{State: EvidenceUnsupported, Reason: "content-hole"}
	end.ChildrenComplete = Evidence[bool]{State: EvidenceUnsupported, Reason: "content-hole"}
	modelSchemaClose(t, s, end, ModelProblem{Code: "model-content-hole", Axis: ModelAxisChildren, Location: h.Location, Expected: "valid content variant", Actual: "nil variant"})
	bound := s.schema.nodes[1]
	if len(s.schema.run.Witnesses(bound.binding)) != 0 {
		t.Fatal("compacted hole produced child witness")
	}
	if got := s.schemaResult(s.nodes[1], XSDCheckAttributes); got.Valid || !got.Complete {
		t.Fatalf("independent attribute failure lost: %+v", got)
	}
	if got := s.nodes[2].header.Declaration; got.State != EvidenceInvalid || got.Cause == nil || got.Cause.Rule.ID != "test.model-representation" {
		t.Fatalf("child lost representation cause: %+v", got)
	}
	if got := s.schemaResult(s.nodes[1], XSDCheckContent); !got.Valid || got.Complete || len(got.Issues) != 0 {
		t.Fatalf("hole invented grammar failure: %+v", got)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxNotAssessed})
	if err != nil {
		t.Fatal(err)
	}
	if report.Conformance != ConformanceViolated || report.AssessmentComplete {
		t.Fatalf("report=%+v", report)
	}
}

func TestModelSchemaBridgeExtremeFloatValues(t *testing.T) {
	for _, number := range []float64{math.SmallestNonzeroFloat64, math.MaxFloat64, math.Copysign(0, -1)} {
		exact, err := NewDecimal(new(big.Rat).SetFloat64(number))
		if err != nil {
			t.Fatal(err)
		}
		account, _ := NewBudgetAccount(Budgets{})
		s := &Session{profile: Profile{Target: TargetModel}, account: account}
		value, err := s.schemaCurrent(context.Background(), Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicDecimal, Decimal: exact}})
		if err != nil {
			t.Fatal(err)
		}
		parsed, ok := xsd.ParseDecimal(exact.String())
		if !ok || value.Value.Number.Compare(parsed) != 0 {
			t.Fatalf("float %g was rounded: %+v", number, value)
		}
		if number == 0 && value.Value.Number.Negative {
			t.Fatal("negative zero preserved as distinct numeric value")
		}
	}
}

func TestModelSchemaRepresentationHoleKeepsIndependentReference(t *testing.T) {
	for _, broken := range []string{"number", "id"} {
		t.Run(broken, func(t *testing.T) {
			schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{
				{Name: SchemaName{Local: "id"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: "ID"}}}, {Name: SchemaName{Local: "ref"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: "IDREF"}}}, {Name: SchemaName{Local: "number"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: "decimal"}}},
			}}}}}})
			if err != nil {
				t.Fatal(err)
			}
			s := modelSchemaSession(t, schema, Budgets{})
			ctx := context.Background()
			h := modelSchemaHeader(1, 0, "root", "Root")
			if err := s.Open(ctx, h); err != nil {
				t.Fatal(err)
			}
			for _, pair := range []struct{ name, text string }{{"id", "target"}, {"ref", "target"}, {"number", "4"}} {
				value := Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: pair.text}}
				if pair.name == broken {
					value = Evidence[Atomic]{State: EvidenceUnsupported, Reason: "invalid-representation"}
				}
				if err := s.Attribute(ctx, AttributeFact{Owner: 1, Name: QName{Local: pair.name}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Declaration: Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "pending"}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceUnsupported, Reason: "no-source"}, Value: value}, Location: h.Location}); err != nil {
					t.Fatal(err)
				}
			}
			modelSchemaClose(t, s, modelSchemaEnd(1, Evidence[Atomic]{State: EvidenceAbsent}, OrderModelFields), ModelProblem{Code: "model-value-invalid", Axis: ModelAxisAttribute, Attribute: QName{Local: broken}, Location: h.Location, Actual: "invalid current representation"})
			if _, err := s.schema.run.Identity(ctx, true); err != nil {
				t.Fatal(err)
			}
			result, cause, err := s.schemaResolveIdentity(ctx, 1, QName{Local: "ref"})
			if err != nil {
				t.Fatal(err)
			}
			want := xsd.IdentityUnique
			if broken == "id" {
				want = xsd.IdentityIncomplete
			}
			if result.Status != want || cause != nil {
				t.Fatalf("%s hole resolution=%+v cause=%+v", broken, result, cause)
			}
			report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxNotAssessed})
			if err != nil {
				t.Fatal(err)
			}
			if report.AssessmentComplete || report.BlockingProblems == 0 {
				t.Fatalf("value hole itself was lost: %+v", report)
			}
		})
	}
}
