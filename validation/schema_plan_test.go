package validation

import (
	"context"
	"errors"
	"github.com/go-muse/go-musicxml/internal/xsd"
	"testing"
)

func miniatureSourceSchema(t *testing.T) *CompiledSchema {
	t.Helper()
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[QName]*SchemaElement{
		{Local: "root"}: {Name: QName{Local: "root"}, Type: SchemaTypeRef{Name: QName{Space: "http://www.w3.org/2001/XMLSchema", Local: "integer"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func sourceSchemaEngine(t *testing.T, checks ...XSDCheck) *Engine {
	t.Helper()
	return sourceSchemaEngineFor(t, miniatureSourceSchema(t), checks...)
}
func sourceSchemaEngineFor(t *testing.T, schema *CompiledSchema, checks ...XSDCheck) *Engine {
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

func schemaPendingHeader() NodeHeader {
	return NodeHeader{ID: 1, Name: QName{Local: "root"},
		Type:        Evidence[ComponentID]{State: EvidenceUnknown, Reason: "schema-binding-pending"},
		Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "schema-binding-pending"},
		Location:    Location{Target: TargetSource, DocumentID: "source", Path: "/root"}}
}

func schemaLexemeEnd(value string) NodeEnd {
	return NodeEnd{ID: 1, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceKnown, Value: value}, Value: Evidence[Atomic]{State: EvidenceUnknown, Reason: "schema-binding-pending"}},
		ChildrenComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, AttributesComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, Order: Evidence[OrderKind]{State: EvidenceKnown, Value: OrderSource}}
}

func TestSourceSchemaPendingBindingExecutesSharedScalar(t *testing.T) {
	for _, value := range []string{"18446744073709551616", "1.5"} {
		t.Run(value, func(t *testing.T) {
			ctx := context.Background()
			session, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
			if err != nil {
				t.Fatal(err)
			}
			if err := session.OpenNode(ctx, schemaPendingHeader()); err != nil {
				t.Fatal(err)
			}
			if err := session.CloseNode(ctx, schemaLexemeEnd(value)); err != nil {
				t.Fatal(err)
			}
			report, err := session.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if err != nil {
				t.Fatal(err)
			}
			if value == "1.5" {
				if report.Counts.Fail != 1 || report.Conformance != ConformanceViolated || len(report.Findings) == 0 {
					t.Fatalf("missing scalar failure: %+v", report)
				}
			} else if !report.Satisfied() || report.Counts.Pass != 1 || report.Counts.Unknown != 0 {
				t.Fatalf("pending binding became permanent unknown: %+v", report)
			}
		})
	}
}

func TestSourceSchemaRootClosedFailureSurvivesTailError(t *testing.T) {
	ctx := context.Background()
	session, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	if err := session.CloseNode(ctx, schemaLexemeEnd("bad")); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("malformed trailing source")
	report, err := session.Finish(ctx, Completion{Syntax: SyntaxMalformed, Cause: cause})
	if !errors.Is(err, cause) || report.Counts.Fail != 1 || report.Conformance != ConformanceViolated || report.AssessmentComplete || report.InputComplete || report.Terminal == nil || report.Terminal.Category != CategorySyntax {
		t.Fatalf("lost pre-tail assessment: %+v / %v", report, err)
	}
}

func TestXSDPlanSchemaOwnershipAndCatalogBoundary(t *testing.T) {
	schema := miniatureSourceSchema(t)
	if schema.Digest() == "" {
		t.Fatal("missing immutable schema digest")
	}
	if _, ok := schema.Root(QName{Local: "root"}); !ok {
		t.Fatal("missing exact root declaration")
	}
	if _, err := NewXSDPlan(nil, XSDCheckScalar); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("nil schema: %v", err)
	}
	if _, err := NewXSDPlan(schema, XSDCheck("invalid")); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("invalid family: %v", err)
	}
	if _, err := CompileSchema(Assembly("unknown"), SchemaData{}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("invalid assembly: %v", err)
	}
}

func TestSourceSchemaEOFAndSyntaxIncompleteAreIndependent(t *testing.T) {
	ctx := context.Background()
	session, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	header := schemaPendingHeader()
	header.Type = Evidence[ComponentID]{State: EvidenceUnsupported, Reason: "dtd-infoset-unavailable"}
	header.Declaration = Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "dtd-infoset-unavailable"}
	if err := session.OpenNode(ctx, header); err != nil {
		t.Fatal(err)
	}
	if err := session.CloseNode(ctx, schemaLexemeEnd("1")); err != nil {
		t.Fatal(err)
	}
	report, err := session.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxIncomplete})
	if err != nil || !report.InputComplete || report.Syntax != SyntaxIncomplete || report.AssessmentComplete || report.Conformance != ConformanceUndetermined || report.Counts.Unsupported != 1 {
		t.Fatalf("EOF/syntax axes conflated: %+v / %v", report, err)
	}
}

func TestSourceSchemaProvedFailurePrecedesBufferCache(t *testing.T) {
	ctx := context.Background()
	session, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	used, _ := session.account.Usage(BudgetBufferedBytes)
	remaining := session.account.Limits().MaxBufferedBytes - used
	if err := session.account.Charge(BudgetBufferedBytes, remaining); err != nil {
		t.Fatal(err)
	}
	rule := session.catalog.rules[session.profile.Required[0]]
	result := xsd.Result{Applicable: true, Complete: true, Valid: false, Issues: []xsd.Detail{{Issue: xsd.Issue{Constraint: "integer", Message: "proved failure"}, Subject: 1}}}
	err = session.recordXSDResult(ctx, session.root, rule, result)
	if !errors.Is(err, ErrResourceLimit) || session.accumulator.report.Counts.Fail != 1 {
		t.Fatalf("proved failure erased by storage: count=%+v error=%v", session.accumulator.report.Counts, err)
	}
	_ = session.account.Release(BudgetBufferedBytes, remaining)
	_, _ = session.Finish(ctx, Completion{Syntax: SyntaxIncomplete, Cause: err})
}

func TestSourceSchemaClosedChildFailureSurvivesUnclosedParent(t *testing.T) {
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[QName]*SchemaElement{{Local: "root"}: {Name: QName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Particle: &SchemaParticle{Kind: SchemaParticleSequence, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Children: []*SchemaParticle{{Kind: SchemaParticleElement, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Element: &SchemaElement{Name: QName{Local: "child"}, Type: SchemaTypeRef{Name: QName{Space: "http://www.w3.org/2001/XMLSchema", Local: "integer"}}}}}}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	session, err := sourceSchemaEngineFor(t, schema, XSDCheckScalar).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	root := schemaPendingHeader()
	if err := session.OpenNode(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := session.CompleteAttributes(ctx, root.ID, Evidence[bool]{State: EvidenceKnown, Value: true}); err != nil {
		t.Fatal(err)
	}
	child := schemaPendingHeader()
	child.ID = 2
	child.Parent = 1
	child.Name = QName{Local: "child"}
	child.Location.Path = "/root/child"
	if err := session.OpenNode(ctx, child); err != nil {
		t.Fatal(err)
	}
	end := schemaLexemeEnd("bad")
	end.ID = 2
	if err := session.CloseNode(ctx, end); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("unclosed root at end")
	report, err := session.Finish(ctx, Completion{Syntax: SyntaxMalformed, Cause: cause})
	if !errors.Is(err, cause) || report.Counts.Fail != 1 || report.Conformance != ConformanceViolated || report.AssessmentComplete {
		t.Fatalf("lost witnessed child failure: %+v / %v", report, err)
	}
}

func TestSourceSchemaCompositeAppendsDetailsWithoutRecount(t *testing.T) {
	ctx := context.Background()
	session, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	rule := session.catalog.rules[session.profile.Required[0]]
	result := xsd.Result{Applicable: true, Complete: true, Valid: false, Issues: []xsd.Detail{{Issue: xsd.Issue{Constraint: "first", Message: "first proof"}, Subject: 1}}}
	if err := session.recordXSDResult(ctx, session.root, rule, result); err != nil {
		t.Fatal(err)
	}
	result.Issues = append(result.Issues, xsd.Detail{Issue: xsd.Issue{Constraint: "second", Message: "later proof"}, Subject: 1})
	result.Complete = false
	if err := session.recordXSDResult(ctx, session.root, rule, result); err != nil {
		t.Fatal(err)
	}
	if err := session.recordXSDResult(ctx, session.root, rule, result); err != nil {
		t.Fatal(err)
	}
	report := session.accumulator.report
	if report.Counts.Fail != 1 || len(report.Findings) != 2 || report.AssessmentComplete || report.BlockingProblems != 1 {
		t.Fatalf("composite replay: %+v", report)
	}
	_, _ = session.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
}

func TestSourceSchemaKnownProofPrecedesLaterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	run, err := xsd.NewRun(session.schema.active.kernel, schemaMeter{session})
	if err != nil {
		t.Fatal(err)
	}
	session.schema.run = run
	use, _ := session.schema.active.kernel.Root(xsd.QName{Local: "root"})
	binding, err := run.Bind(ctx, use, xsd.Subject{ID: 1, Name: xsd.QName{Local: "root"}, Text: "bad", TextKnown: true, AttributesComplete: true, ChildrenComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	session.schema.nodes = map[NodeID]schemaNode{1: {binding: binding, available: true}}
	cancel()
	err = session.recordXSDProvedFailures(ctx, session.root)
	if !errors.Is(err, context.Canceled) || session.accumulator.report.Counts.Fail != 1 {
		t.Fatalf("known proof erased by later cancellation: count=%+v error=%v", session.accumulator.report.Counts, err)
	}
	_, _ = session.Finish(ctx, Completion{Syntax: SyntaxIncomplete, Cause: err})
}
