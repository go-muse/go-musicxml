package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func representationFixture(t *testing.T, budgets Budgets, selected bool) *Session {
	t.Helper()
	rules, profiles, plans := catalogFixture(t)
	rules[0].Targets = []Target{TargetModel}
	profiles[0].Target = TargetModel
	profiles[0].ID = "test.model"
	if selected {
		assembly := AssemblyMusicXML
		plan, err := NewModelRepresentationPlan(assembly)
		if err != nil {
			t.Fatal(err)
		}
		key := RuleKey{ID: "test.model-representation", Revision: 1}
		rules = append(rules, RuleDefinition{Key: key, Class: ClassMandatory, Role: RolePredicate, Category: CategoryModel, Targets: []Target{TargetModel}, Scope: ScopeNode, Selector: Selector{Assembly: &assembly}, Predicate: "representation", Sources: rules[0].Sources, Text: RuleText{Title: "Current representation", Explanation: "The current Go representation must be representable.", Message: "Invalid model representation."}})
		profiles[0].Required = append(profiles[0].Required, key)
		profiles[0].Support = append(profiles[0].Support, RuleSupport{Rule: key, State: ImplementationAvailable})
		plans = append(plans, PlanBinding{ID: "representation", Plan: plan})
	}
	catalog, err := NewCatalog(rules, profiles, plans)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(catalog)
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: "test.model", Budgets: budgets})
	if err != nil {
		t.Fatal(err)
	}
	return session
}
func representationHeader(id, parent NodeID, occurrence, name string) NodeHeader {
	h := factHeader(id, parent, occurrence, name)
	h.Location.Target = TargetModel
	h.Location.Path = "Root." + name
	return h
}
func representationEnd(id NodeID) NodeEnd {
	end := factEnd(id)
	end.Order.Value = OrderModelFields
	end.Scalar.Lexeme = Evidence[string]{State: EvidenceUnsupported, Reason: "model-source-lexeme-unavailable"}
	return end
}
func representationFact(id NodeID) ModelRepresentationFact {
	return ModelRepresentationFact{Owner: id, Complete: Evidence[bool]{State: EvidenceKnown, Value: true}}
}
func representationProblem(axis ModelAxis) ModelProblem {
	return ModelProblem{Code: "model-nonfinite", Axis: axis, Location: Location{Target: TargetModel, DocumentID: "doc", Path: "Root.root.Value"}, Expected: "finite decimal", Actual: "NaN"}
}
func representationOpen(t *testing.T, s *Session) {
	t.Helper()
	if err := s.Open(context.Background(), representationHeader(1, 0, "test.root", "root")); err != nil {
		t.Fatal(err)
	}
}
func representationFinish(t *testing.T, s *Session) Report {
	t.Helper()
	r, err := s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxNotAssessed})
	if err != nil {
		t.Fatal(err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("buffer leaked: %d", used)
	}
	if used, _ := s.account.Usage(BudgetDepth); used != 0 {
		t.Fatalf("depth leaked: %d", used)
	}
	return r
}

func TestModelRepresentationRequiredEvidenceAndCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		selected, present, complete bool
		problems                    bool
	}{
		{name: "complete", selected: true, present: true, complete: true},
		{name: "missing", selected: true},
		{name: "incomplete", selected: true, present: true},
		{name: "legacy-generic"},
		{name: "fail-complete", selected: true, present: true, complete: true, problems: true},
		{name: "fail-incomplete", selected: true, present: true, problems: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := representationFixture(t, Budgets{}, tc.selected)
			representationOpen(t, s)
			end := representationEnd(1)
			if tc.present {
				f := representationFact(1)
				f.Complete.Value = tc.complete
				if tc.problems {
					f.Problems = []ModelProblem{representationProblem(ModelAxisScalar)}
					end.Scalar.Value = Evidence[Atomic]{State: EvidenceUnsupported, Reason: "model-nonfinite"}
				}
				if err := s.ModelRepresentation(context.Background(), f); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Close(context.Background(), end); err != nil {
				t.Fatal(err)
			}
			r := representationFinish(t, s)
			if r.Counts.Fail != 1 || r.Conformance != ConformanceViolated {
				t.Fatalf("independent normative failure lost: %+v", r)
			}
			wantComplete := !tc.selected || tc.present && tc.complete
			if r.AssessmentComplete != wantComplete {
				t.Fatalf("assessment complete=%v want %v: %+v", r.AssessmentComplete, wantComplete, r)
			}
			wantBlock := tc.selected && (!tc.present || !tc.complete || tc.problems)
			if (r.BlockingProblems > 0) != wantBlock {
				t.Fatalf("blocking axes: %+v", r)
			}
			if r.Syntax != SyntaxNotAssessed || r.Satisfied() {
				t.Fatalf("invalid report: %+v", r)
			}
		})
	}
}

func TestModelRepresentationOwnsProblemsAndPublishesRealCause(t *testing.T) {
	s := representationFixture(t, Budgets{}, true)
	representationOpen(t, s)
	f := representationFact(1)
	f.Problems = []ModelProblem{representationProblem(ModelAxisScalar)}
	if err := s.ModelRepresentation(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	f.Problems[0].Code = "changed"
	f.Problems[0].Location.Path = "changed"
	f.Problems[0].Actual = "changed"
	end := representationEnd(1)
	end.Scalar.Value = Evidence[Atomic]{State: EvidenceUnsupported, Reason: "model-nonfinite"}
	if err := s.Close(context.Background(), end); err != nil {
		t.Fatal(err)
	}
	scalar := s.nodes[1].end.Scalar.Value
	if scalar.State != EvidenceInvalid || scalar.Cause == nil {
		t.Fatalf("missing causal invalidation: %+v", scalar)
	}
	decision, ok := s.lookupDecision(decisionID{rule: scalar.Cause.Rule, node: scalar.Cause.Subject.Node})
	if !ok || decision.Outcome != OutcomeFail || scalar.Cause.Subject.DocumentID != "doc" {
		t.Fatalf("invented cause: %+v %+v", scalar, decision)
	}
	if s.nodes[1].end.ChildrenComplete.State != EvidenceKnown || s.nodes[1].end.AttributesComplete.State != EvidenceKnown {
		t.Fatal("unrelated axes were invalidated")
	}
	r := representationFinish(t, s)
	found := false
	for _, finding := range r.Findings {
		if finding.Category == CategoryModel {
			found = true
			if finding.Reason != "model-nonfinite" || finding.Actual != "NaN" || finding.Location.Path != "Root.root.Value" {
				t.Fatalf("aliased diagnostic: %+v", finding)
			}
		}
	}
	if !found {
		t.Fatal("missing model diagnostic")
	}
}

func TestModelRepresentationLifecycleAndContracts(t *testing.T) {
	tests := map[string]func(*Session) error{
		"wrong-owner": func(s *Session) error {
			f := representationFact(2)
			return s.ModelRepresentation(context.Background(), f)
		},
		"duplicate": func(s *Session) error {
			if err := s.ModelRepresentation(context.Background(), representationFact(1)); err != nil {
				return err
			}
			return s.ModelRepresentation(context.Background(), representationFact(1))
		},
		"after-close": func(s *Session) error {
			if err := s.Close(context.Background(), representationEnd(1)); err != nil {
				return err
			}
			return s.ModelRepresentation(context.Background(), representationFact(1))
		},
		"absent-completeness": func(s *Session) error {
			f := representationFact(1)
			f.Complete = Evidence[bool]{State: EvidenceAbsent}
			return s.ModelRepresentation(context.Background(), f)
		},
		"invented-cause": func(s *Session) error {
			f := representationFact(1)
			f.Complete = Evidence[bool]{State: EvidenceInvalid, Reason: "bad", Cause: &DecisionRef{Rule: RuleKey{ID: "invented", Revision: 1}, Subject: NodeRef{"doc", 1}}}
			return s.ModelRepresentation(context.Background(), f)
		},
		"wrong-location": func(s *Session) error {
			f := representationFact(1)
			p := representationProblem(ModelAxisNode)
			p.Location.DocumentID = "other"
			f.Problems = []ModelProblem{p}
			return s.ModelRepresentation(context.Background(), f)
		},
		"source-position": func(s *Session) error {
			f := representationFact(1)
			p := representationProblem(ModelAxisNode)
			p.Location.Position = &SourcePosition{}
			f.Problems = []ModelProblem{p}
			return s.ModelRepresentation(context.Background(), f)
		},
		"oversized-detail": func(s *Session) error {
			f := representationFact(1)
			p := representationProblem(ModelAxisNode)
			p.Actual = strings.Repeat("x", 4097)
			f.Problems = []ModelProblem{p}
			return s.ModelRepresentation(context.Background(), f)
		},
		"unknown-axis": func(s *Session) error {
			f := representationFact(1)
			f.Problems = []ModelProblem{representationProblem("other")}
			return s.ModelRepresentation(context.Background(), f)
		},
		"missing-attribute": func(s *Session) error {
			f := representationFact(1)
			p := representationProblem(ModelAxisAttribute)
			p.Attribute = QName{Local: "amount"}
			f.Problems = []ModelProblem{p}
			return s.ModelRepresentation(context.Background(), f)
		},
		"known-scalar-contradiction": func(s *Session) error {
			f := representationFact(1)
			f.Problems = []ModelProblem{representationProblem(ModelAxisScalar)}
			if err := s.ModelRepresentation(context.Background(), f); err != nil {
				return err
			}
			end := representationEnd(1)
			end.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "known"}}
			return s.Close(context.Background(), end)
		},
		"children-contradiction": func(s *Session) error {
			f := representationFact(1)
			f.Problems = []ModelProblem{representationProblem(ModelAxisChildren)}
			if err := s.ModelRepresentation(context.Background(), f); err != nil {
				return err
			}
			return s.Close(context.Background(), representationEnd(1))
		},
		"child-after-seal": func(s *Session) error {
			if err := s.ModelRepresentation(context.Background(), representationFact(1)); err != nil {
				return err
			}
			return s.Open(context.Background(), representationHeader(2, 1, "test.child", "child"))
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s := representationFixture(t, Budgets{}, true)
			representationOpen(t, s)
			err := test(s)
			if err == nil {
				t.Fatal("contradiction accepted")
			}
			r, finishErr := s.Finish(context.Background(), Completion{Syntax: SyntaxNotAssessed})
			if finishErr == nil || r.Satisfied() || r.AssessmentComplete {
				t.Fatalf("failure disappeared: %+v %v", r, finishErr)
			}
		})
	}
	t.Run("source-target", func(t *testing.T) {
		_, s := sessionFixture(t, Budgets{})
		if err := s.Open(context.Background(), factHeader(1, 0, "test.root", "root")); err != nil {
			t.Fatal(err)
		}
		if err := s.ModelRepresentation(context.Background(), representationFact(1)); err == nil {
			t.Fatal("source accepted model event")
		}
		_, _ = s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
	})
}

func TestModelRepresentationTruncationPreservesBlockersAndCauses(t *testing.T) {
	s := representationFixture(t, Budgets{MaxDiagnosticBytes: 1}, true)
	representationOpen(t, s)
	f := representationFact(1)
	f.Complete.Value = false
	f.Problems = []ModelProblem{representationProblem(ModelAxisScalar)}
	if err := s.ModelRepresentation(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	end := representationEnd(1)
	end.Scalar.Value = Evidence[Atomic]{State: EvidenceUnsupported, Reason: "model-nonfinite"}
	if err := s.Close(context.Background(), end); err != nil {
		t.Fatal(err)
	}
	if s.nodes[1].end.Scalar.Value.Cause == nil {
		t.Fatal("diagnostic truncation lost causal result")
	}
	r := representationFinish(t, s)
	if len(r.Findings) != 0 || !r.DiagnosticsTruncated || r.BlockingProblems == 0 || r.AssessmentComplete || r.Counts.Fail != 1 || r.Conformance != ConformanceViolated {
		t.Fatalf("truncated proof lost: %+v", r)
	}
}

func TestModelRepresentationEventBudgetAndCleanup(t *testing.T) {
	s := representationFixture(t, Budgets{MaxEvents: 1}, true)
	representationOpen(t, s)
	err := s.ModelRepresentation(context.Background(), representationFact(1))
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("event did not consume budget: %v", err)
	}
	r, finishErr := s.Finish(context.Background(), Completion{Syntax: SyntaxNotAssessed})
	if !errors.Is(finishErr, ErrResourceLimit) || r.Terminal == nil || r.Terminal.Category != CategoryResource {
		t.Fatalf("resource classification: %+v %v", r, finishErr)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("buffer leaked: %d", used)
	}
	if used, _ := s.account.Usage(BudgetDepth); used != 0 {
		t.Fatalf("depth leaked: %d", used)
	}
}

func TestModelRepresentationAttributeAndChildrenAreScoped(t *testing.T) {
	for _, axis := range []ModelAxis{ModelAxisAttribute, ModelAxisChildren} {
		t.Run(string(axis), func(t *testing.T) {
			s := representationFixture(t, Budgets{}, true)
			representationOpen(t, s)
			end := representationEnd(1)
			end.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "independent"}}
			problem := representationProblem(axis)
			if axis == ModelAxisAttribute {
				problem.Attribute = QName{Local: "amount"}
				problem.Location.Path = "Root.root.Amount"
				attribute := AttributeFact{Owner: 1, Name: problem.Attribute, Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "unbound"}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceUnsupported, Reason: "model-lexeme"}, Value: Evidence[Atomic]{State: EvidenceUnsupported, Reason: "nonfinite"}}, Location: problem.Location}
				if err := s.Attribute(context.Background(), attribute); err != nil {
					t.Fatal(err)
				}
			} else {
				problem.Code = "model-choice-cardinality"
				problem.Location.Path = "Root.root.Content[1]"
				end.ChildrenComplete = Evidence[bool]{State: EvidenceKnown, Value: false}
				end.Order = Evidence[OrderKind]{State: EvidenceUnsupported, Reason: "model-sequence-hole"}
			}
			f := representationFact(1)
			f.Problems = []ModelProblem{problem}
			if err := s.ModelRepresentation(context.Background(), f); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(context.Background(), end); err != nil {
				t.Fatal(err)
			}
			n := s.nodes[1]
			if n.end.Scalar.Value.State != EvidenceKnown || n.end.Scalar.Value.Value.Text != "independent" || n.end.AttributesComplete.State != EvidenceKnown {
				t.Fatal("independent facts lost")
			}
			if axis == ModelAxisAttribute {
				a := n.attributes[0]
				if a.Presence.State != EvidenceKnown || !a.Presence.Value || a.Scalar.Value.State != EvidenceInvalid || a.Scalar.Value.Cause == nil || n.end.ChildrenComplete.State != EvidenceKnown {
					t.Fatalf("attribute scoping: %+v", a)
				}
			} else if n.end.ChildrenComplete.State != EvidenceInvalid || n.end.Order.State != EvidenceInvalid || n.end.Order.Cause == nil {
				t.Fatalf("sequence hole compacted: %+v", n.end)
			}
			r := representationFinish(t, s)
			if r.BlockingProblems == 0 || r.Satisfied() {
				t.Fatalf("lost model blocker: %+v", r)
			}
			if axis == ModelAxisChildren && (r.AssessmentComplete || r.Counts.Fail != 0 || r.Counts.Blocked != 1) {
				t.Fatalf("hole fabricated missing-child failure: %+v", r)
			}
		})
	}
}

func TestModelRepresentationCompleteSuccessAndAssemblyScope(t *testing.T) {
	s := representationFixture(t, Budgets{}, true)
	representationOpen(t, s)
	if err := s.Open(context.Background(), representationHeader(2, 1, "test.child", "child")); err != nil {
		t.Fatal(err)
	}
	for _, id := range []NodeID{2, 1} {
		if err := s.ModelRepresentation(context.Background(), representationFact(id)); err != nil {
			t.Fatal(err)
		}
		if err := s.Close(context.Background(), representationEnd(id)); err != nil {
			t.Fatal(err)
		}
	}
	r := representationFinish(t, s)
	if !r.Satisfied() || r.Counts.Pass != 1 || r.Counts.Fail != 0 || r.BlockingProblems != 0 {
		t.Fatalf("complete model rejected: %+v", r)
	}
	t.Run("unbound-descendant", func(t *testing.T) {
		s := representationFixture(t, Budgets{}, true)
		representationOpen(t, s)
		h := representationHeader(2, 1, "test.child", "child")
		h.Declaration = Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "schema-unbound"}
		if err := s.Open(context.Background(), h); err != nil {
			t.Fatal(err)
		}
		if err := s.ModelRepresentation(context.Background(), representationFact(2)); err != nil {
			t.Fatal(err)
		}
		if err := s.Close(context.Background(), representationEnd(2)); err != nil {
			t.Fatal(err)
		}
		d, ok := s.lookupDecision(decisionID{rule: RuleKey{ID: "test.model-representation", Revision: 1}, node: 2})
		if !ok || d.Outcome != OutcomePass {
			t.Fatalf("model check requires bound child: %+v", d)
		}
		_, _ = s.Finish(context.Background(), Completion{Syntax: SyntaxNotAssessed})
	})
}

func TestModelRepresentationPlanCatalogContract(t *testing.T) {
	if _, err := NewModelRepresentationPlan(""); err == nil {
		t.Fatal("empty assembly accepted")
	}
	changes := map[string]func(*RuleDefinition){
		"normative-category":  func(r *RuleDefinition) { r.Category = CategoryNormative },
		"optional-class":      func(r *RuleDefinition) { r.Class = ClassRecommendation },
		"source-target":       func(r *RuleDefinition) { r.Targets = []Target{TargetSource} },
		"document-scope":      func(r *RuleDefinition) { r.Scope = ScopeDocument },
		"mismatched-assembly": func(r *RuleDefinition) { a := AssemblyOpus; r.Selector.Assembly = &a },
		"exact-selector": func(r *RuleDefinition) {
			r.Selector = Selector{Declaration: &DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "x", Occurrence: "y"}}
		},
		"prerequisite": func(r *RuleDefinition) {
			r.Requires = []Dependency{{Kind: DependencyExternal, Relation: RelationSameNode, Key: "pending"}}
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			s := representationFixture(t, Budgets{}, true)
			p := s.catalog.plans["representation"]
			r := s.catalog.rules[RuleKey{ID: "test.model-representation", Revision: 1}]
			change(&r)
			if err := p.validateFor(r); err == nil {
				t.Fatal("unsafe model plan binding accepted")
			}
			_, _ = s.Finish(context.Background(), Completion{Syntax: SyntaxNotAssessed})
		})
	}
	first, _ := NewModelRepresentationPlan(AssemblyMusicXML)
	second, _ := NewModelRepresentationPlan(AssemblyOpus)
	if first.modelAssembly == second.modelAssembly {
		t.Fatal("plan assembly lost")
	}
}

func TestModelRepresentationBufferAndWorkBudgets(t *testing.T) {
	for _, kind := range []BudgetKind{BudgetBufferedBytes, BudgetRuleSteps} {
		t.Run(string(kind), func(t *testing.T) {
			s := representationFixture(t, Budgets{}, true)
			representationOpen(t, s)
			used, _ := s.account.Usage(kind)
			limit := s.account.Limits().MaxBufferedBytes
			if kind == BudgetRuleSteps {
				limit = s.account.Limits().MaxRuleSteps
			}
			external := limit - used
			if err := s.account.Charge(kind, external); err != nil {
				t.Fatal(err)
			}
			f := representationFact(1)
			f.Problems = []ModelProblem{representationProblem(ModelAxisNode)}
			if err := s.ModelRepresentation(context.Background(), f); !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("missing %s charge: %v", kind, err)
			}
			_, err := s.Finish(context.Background(), Completion{Syntax: SyntaxNotAssessed})
			if !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("resource cause lost: %v", err)
			}
			if kind == BudgetBufferedBytes {
				if err := s.account.Release(kind, external); err != nil {
					t.Fatal(err)
				}
			}
			if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("buffer leak: %d", used)
			}
		})
	}
}

func TestModelRepresentationVerdictSurvivesCacheFailure(t *testing.T) {
	s := representationFixture(t, Budgets{}, true)
	representationOpen(t, s)
	f := representationFact(1)
	f.Problems = []ModelProblem{representationProblem(ModelAxisScalar)}
	if err := s.ModelRepresentation(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	end := representationEnd(1)
	end.Scalar.Value = Evidence[Atomic]{State: EvidenceUnsupported, Reason: "model-nonfinite"}
	used, _ := s.account.Usage(BudgetBufferedBytes)
	external := s.account.Limits().MaxBufferedBytes - used - endLogicalBytes(end)
	if err := s.account.Charge(BudgetBufferedBytes, external); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(context.Background(), end); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("expected cache exhaustion: %v", err)
	}
	r, err := s.Finish(context.Background(), Completion{Syntax: SyntaxNotAssessed})
	if !errors.Is(err, ErrResourceLimit) || r.BlockingProblems < 2 || r.AssessmentComplete || r.Satisfied() {
		t.Fatalf("established verdict erased: %+v %v", r, err)
	}
	if err := s.account.Release(BudgetBufferedBytes, external); err != nil {
		t.Fatal(err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("buffer leak: %d", used)
	}
}
