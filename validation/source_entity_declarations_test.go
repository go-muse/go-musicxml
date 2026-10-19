package validation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func entityDeclarationSchema(t *testing.T, ref SchemaTypeRef) *CompiledSchema {
	t.Helper()
	s, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[QName]*SchemaElement{{Local: "root"}: {Name: QName{Local: "root"}, Type: ref}}})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func entityBuiltin(name string) SchemaTypeRef {
	return SchemaTypeRef{Name: QName{Space: "http://www.w3.org/2001/XMLSchema", Local: name}}
}
func entityMember(name string) *SchemaSimpleMember {
	return &SchemaSimpleMember{Name: entityBuiltin(name).Name}
}
func entityDeclarationDefinitions(t *testing.T, schema *CompiledSchema, checks ...XSDCheck) ([]RuleDefinition, []Profile, []PlanBinding) {
	t.Helper()
	old := sourceSchemaEngineFor(t, schema, checks...)
	p, _ := old.catalog.Profile("source-schema-test")
	p.CatalogDigest = ""
	var rules []RuleDefinition
	var bindings []PlanBinding
	for _, k := range p.Required {
		r := old.catalog.rules[k]
		rules = append(rules, r)
		plan, err := newXSDPlanWithSourceDeclarations(schema, old.catalog.plans[r.Predicate].xsdCheck)
		if err != nil {
			t.Fatal(err)
		}
		bindings = append(bindings, PlanBinding{ID: r.Predicate, Plan: plan})
	}
	return rules, []Profile{p}, bindings
}
func entityDeclarationEngine(t *testing.T, schema *CompiledSchema, checks ...XSDCheck) *Engine {
	t.Helper()
	r, p, b := entityDeclarationDefinitions(t, schema, checks...)
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEngine(c)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func entitySnapshot(complete bool, declarations ...SourceEntityDeclaration) SourceEntityDeclarations {
	evidence := Evidence[bool]{State: EvidenceKnown, Value: complete}
	if !complete {
		evidence.Reason = "unread-external-subset"
	}
	return SourceEntityDeclarations{DocumentID: "source", Declarations: declarations, Complete: evidence}
}
func entityRow(name string, unparsed bool, sequence uint64) SourceEntityDeclaration {
	return SourceEntityDeclaration{Name: name, Unparsed: unparsed, Sequence: sequence, Origin: Evidence[Location]{State: EvidenceUnknown, Reason: "replacement-declaration-location-unavailable"}}
}
func entityDeclarationSession(t *testing.T, e *Engine) *Session {
	t.Helper()
	s, err := e.Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func entityDeclarationAssess(t *testing.T, e *Engine, value string, facts *SourceEntityDeclarations) Report {
	t.Helper()
	ctx := context.Background()
	s := entityDeclarationSession(t, e)
	if err := s.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	if facts != nil {
		if err := s.sourceEntityDeclarations(ctx, *facts); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CloseNode(ctx, schemaLexemeEnd(value)); err != nil {
		t.Fatal(err)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	return report
}
func ptrEntitySnapshot(f SourceEntityDeclarations) *SourceEntityDeclarations { return &f }

func TestSourceEntityDeclarationsCatalogModesAndLegacyDigest(t *testing.T) {
	schema := entityDeclarationSchema(t, entityBuiltin("ENTITY"))
	r, p, b := entityDeclarationDefinitions(t, schema, XSDCheckScalar, XSDCheckAttributes)
	legacy, _ := NewXSDPlan(schema, XSDCheckScalar)
	modern, _ := newXSDPlanWithSourceDeclarations(schema, XSDCheckScalar)
	oldJSON, _ := json.Marshal(legacy.digestValue())
	expected, _ := json.Marshal(struct {
		Kind           planKind
		SchemaDigest   string
		SchemaAssembly Assembly
		Check          XSDCheck
	}{planXSD, schema.Digest(), schema.Assembly(), XSDCheckScalar})
	if string(oldJSON) != string(expected) {
		t.Fatalf("legacy plan encoding changed: %s", oldJSON)
	}
	newJSON, _ := json.Marshal(modern.digestValue())
	if string(newJSON) == string(oldJSON) {
		t.Fatal("mode missing from digest")
	}
	for _, unsupported := range []bool{false, true} {
		bb := append([]PlanBinding(nil), b...)
		bb[0].Plan = legacy
		pp := cloneProfile(p[0])
		if unsupported {
			pp.Support[0].State = ImplementationUnsupported
			pp.Support[0].Reason = "reviewed-unavailable"
		}
		if _, err := NewCatalog(r, []Profile{pp}, bb); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("mixed selected modes unsupported=%v: %v", unsupported, err)
		}
	}
	for _, targets := range [][]Target{{TargetModel}, {TargetSource, TargetModel}} {
		rr := append([]RuleDefinition(nil), r...)
		rr[0] = cloneRule(rr[0])
		rr[0].Targets = targets
		if _, err := NewCatalog(rr, p, b); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("new mode accepted target %v: %v", targets, err)
		}
	}
	oldEngine := sourceSchemaEngineFor(t, schema, XSDCheckScalar)
	if report := entityDeclarationAssess(t, oldEngine, "asset", nil); report.Counts.Unsupported != 1 {
		t.Fatalf("legacy behavior changed: %+v", report)
	}
}

func TestSourceEntityDeclarationsAdmissionPhase(t *testing.T) {
	ctx := context.Background()
	schema := entityDeclarationSchema(t, entityBuiltin("ENTITY"))
	e := entityDeclarationEngine(t, schema, XSDCheckScalar)
	for _, phase := range []string{"before-root", "duplicate", "after-attribute", "after-attribute-seal", "after-child", "closed", "finished", "legacy", "unsupported-only", "unselected"} {
		t.Run(phase, func(t *testing.T) {
			chosen := e
			if phase == "legacy" {
				chosen = sourceSchemaEngineFor(t, schema, XSDCheckScalar)
			}
			if phase == "unselected" {
				r, p, b := entityDeclarationDefinitions(t, schema, XSDCheckScalar)
				b[0].Plan, _ = NewXSDPlan(schema, XSDCheckScalar)
				extra, _ := newXSDPlanWithSourceDeclarations(schema, XSDCheckScalar)
				b = append(b, PlanBinding{ID: "unselected", Plan: extra})
				c, err := NewCatalog(r, p, b)
				if err != nil {
					t.Fatal(err)
				}
				chosen, _ = NewEngine(c)
			}
			if phase == "unsupported-only" {
				r, p, b := entityDeclarationDefinitions(t, schema, XSDCheckScalar)
				p[0].Support[0].State = ImplementationUnsupported
				p[0].Support[0].Reason = "unavailable"
				c, err := NewCatalog(r, p, b)
				if err != nil {
					t.Fatal(err)
				}
				chosen, _ = NewEngine(c)
			}
			s := entityDeclarationSession(t, chosen)
			f := entitySnapshot(true, entityRow("asset", true, 1))
			if phase != "before-root" {
				if err := s.OpenNode(ctx, schemaPendingHeader()); err != nil {
					t.Fatal(err)
				}
			}
			switch phase {
			case "duplicate":
				if err := s.sourceEntityDeclarations(ctx, f); err != nil {
					t.Fatal(err)
				}
			case "after-attribute":
				a := AttributeFact{Owner: 1, Name: QName{Local: "x"}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "pending"}, Scalar: schemaLexemeEnd("x").Scalar, Location: schemaPendingHeader().Location}
				if err := s.Attribute(ctx, a); err != nil {
					t.Fatal(err)
				}
			case "after-attribute-seal":
				if err := s.CompleteAttributes(ctx, 1, Evidence[bool]{State: EvidenceKnown, Value: true}); err != nil {
					t.Fatal(err)
				}
			case "after-child":
				h := schemaPendingHeader()
				h.ID = 2
				h.Parent = 1
				if err := s.OpenNode(ctx, h); err != nil {
					t.Fatal(err)
				}
			case "closed":
				if err := s.CloseNode(ctx, schemaLexemeEnd("asset")); err != nil {
					t.Fatal(err)
				}
			case "finished":
				_, _ = s.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
			}
			err := s.sourceEntityDeclarations(ctx, f)
			if err == nil {
				t.Fatal("late/wrong-mode facts admitted")
			}
			if phase != "finished" {
				_, _ = s.Finish(ctx, Completion{Syntax: SyntaxIncomplete, Cause: err})
			}
		})
	}
}
func TestSourceEntityDeclarationNamesUseSourceLimits(t *testing.T) {
	e := entityDeclarationEngine(t, entityDeclarationSchema(t, entityBuiltin("string")), XSDCheckScalar)
	for _, name := range []string{strings.Repeat("x", 2048), "\U00010000"} {
		s := entityDeclarationSession(t, e)
		if err := s.OpenNode(context.Background(), schemaPendingHeader()); err != nil {
			t.Fatal(err)
		}
		f := entitySnapshot(true, entityRow(name, true, 1))
		if err := s.sourceEntityDeclarations(context.Background(), f); err != nil {
			t.Fatalf("legal source name: %v", err)
		}
		_, _ = s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
	}
}

func TestSourceEntityDeclarationsAdmissionBudgetsAndCancellation(t *testing.T) {
	ctx := context.Background()
	e := entityDeclarationEngine(t, entityDeclarationSchema(t, entityBuiltin("ENTITY")), XSDCheckScalar)
	facts := entitySnapshot(true, entityRow(strings.Repeat("a", 2048), true, 1), entityRow("second", false, 2))
	baseline := entityDeclarationSession(t, e)
	if err := baseline.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	beforeBytes, _ := baseline.account.Usage(BudgetBufferedBytes)
	beforeSteps, _ := baseline.account.Usage(BudgetRuleSteps)
	if err := baseline.sourceEntityDeclarations(ctx, facts); err != nil {
		t.Fatal(err)
	}
	afterBytes, _ := baseline.account.Usage(BudgetBufferedBytes)
	afterSteps, _ := baseline.account.Usage(BudgetRuleSteps)
	_, _ = baseline.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
	for _, kind := range []BudgetKind{BudgetBufferedBytes, BudgetRuleSteps} {
		for _, short := range []int64{0, 1} {
			s := entityDeclarationSession(t, e)
			if err := s.OpenNode(ctx, schemaPendingHeader()); err != nil {
				t.Fatal(err)
			}
			delta := afterBytes - beforeBytes
			limit := s.account.Limits().MaxBufferedBytes
			if kind == BudgetRuleSteps {
				delta = afterSteps - beforeSteps
				limit = s.account.Limits().MaxRuleSteps
			}
			used, _ := s.account.Usage(kind)
			held := limit - used - delta + short
			if err := s.account.Charge(kind, held); err != nil {
				t.Fatal(err)
			}
			err := s.sourceEntityDeclarations(ctx, facts)
			if (short == 0 && err != nil) || (short == 1 && !errors.Is(err, ErrResourceLimit)) {
				t.Fatalf("%s short=%d: %v", kind, short, err)
			}
			if kind == BudgetBufferedBytes {
				_ = s.account.Release(kind, held)
			}
			_, _ = s.Finish(ctx, Completion{Syntax: SyntaxIncomplete, Cause: err})
			if live, _ := s.account.Usage(BudgetBufferedBytes); live != 0 {
				t.Fatalf("admission leaked %d", live)
			}
		}
	}
	s := entityDeclarationSession(t, e)
	if err := s.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	err := s.sourceEntityDeclarations(cancelled, facts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	_, err = s.Finish(ctx, Completion{Syntax: SyntaxIncomplete, Cause: err})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cause lost: %v", err)
	}
	if live, _ := s.account.Usage(BudgetBufferedBytes); live != 0 {
		t.Fatalf("cancel leak %d", live)
	}
}
