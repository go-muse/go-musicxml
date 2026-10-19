package validation

import (
	"context"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"
)

// Independent regression: a type fact is not evidence that an unsupported
// element/group/attribute component selector is excluded.
func TestQAUnsupportedComponentSelectorCannotBecomeSatisfied(t *testing.T) {
	for _, kind := range []ComponentKind{ComponentElement, ComponentGroup, ComponentAttribute} {
		t.Run(string(kind), func(t *testing.T) {
			rules, profiles, _ := catalogFixture(t)
			rules[0].Selector = Selector{Component: &ComponentID{Assembly: AssemblyMusicXML, Kind: kind, Name: QName{Local: "root"}}}
			profiles[0].Support[0] = RuleSupport{Rule: rules[0].Key, State: ImplementationUnsupported, Reason: "component-binding-unavailable"}
			c, err := NewCatalog(rules, profiles, nil)
			if err != nil {
				t.Fatal(err)
			}
			e, err := NewEngine(c)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			s, err := e.Begin(ctx, RunOptions{DocumentID: "doc", Profile: profiles[0].ID})
			if err != nil {
				t.Fatal(err)
			}
			h := factHeader(1, 0, "test.root", "root")
			h.Type = Evidence[ComponentID]{State: EvidenceKnown, Value: ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType, Name: QName{Local: "root"}}}
			if err = s.OpenNode(ctx, h); err != nil {
				t.Fatal(err)
			}
			if err = s.CloseNode(ctx, factEnd(1)); err != nil {
				t.Fatal(err)
			}
			report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if err != nil {
				t.Fatal(err)
			}
			if report.Satisfied() || report.Counts.Unsupported != 1 || report.AssessmentComplete {
				t.Fatalf("unsupported component was excluded using unrelated type evidence: Satisfied=%v Counts=%+v Complete=%v Findings=%+v", report.Satisfied(), report.Counts, report.AssessmentComplete, report.Findings)
			}
		})
	}
}

func qaContextCatalog(t *testing.T, consumerID string, cycle bool) (*CompiledCatalog, ProfileID) {
	t.Helper()
	rules, profiles, _ := catalogFixture(t)
	base := rules[0]
	constant := Atomic{Kind: AtomicString, Text: "value"}
	seed, _ := NewContextValuePlan("seed", RelationSameNode, constant)
	published, _ := NewContextValuePlan("published", RelationSameNode, constant)
	compare, _ := NewContextComparisonPlan("published", RelationSameNode, ComparisonEqual, constant)
	seedRule, provider, consumer := cloneRule(base), cloneRule(base), cloneRule(base)
	seedRule.Key.ID = "seed-provider"
	seedRule.Predicate = "seed"
	seedRule.Role = RoleContextProvider
	provider.Key.ID = "z-provider"
	provider.Predicate = "published"
	provider.Role = RoleContextProvider
	provider.Requires = []Dependency{{Kind: DependencyFact, Key: "seed", Relation: RelationSameNode}}
	consumer.Key.ID = RuleID(consumerID)
	consumer.Predicate = "compare"
	consumer.Requires = []Dependency{{Kind: DependencyFact, Key: "published", Relation: RelationSameNode}}
	if cycle {
		seedRule.Requires = []Dependency{{Kind: DependencyFact, Key: "published", Relation: RelationSameNode}}
	}
	rules = []RuleDefinition{seedRule, provider, consumer}
	profiles[0].Required = nil
	profiles[0].Support = nil
	for _, rule := range rules {
		profiles[0].Required = append(profiles[0].Required, rule.Key)
		profiles[0].Support = append(profiles[0].Support, RuleSupport{Rule: rule.Key, State: ImplementationAvailable})
	}
	catalog, err := NewCatalog(rules, profiles, []PlanBinding{{ID: "seed", Plan: seed}, {ID: "published", Plan: published}, {ID: "compare", Plan: compare}})
	if cycle {
		if err == nil {
			t.Error("catalog accepted a runtime fact-provider cycle: seed -> published -> seed")
		}
		return catalog, profiles[0].ID
	}
	if err != nil {
		t.Fatal(err)
	}
	return catalog, profiles[0].ID
}

func TestQAFactDependencyClosureCannotDependOnRuleSpelling(t *testing.T) {
	for _, id := range []string{"a-consumer", "zz-consumer"} {
		t.Run(id, func(t *testing.T) {
			c, p := qaContextCatalog(t, id, false)
			e, _ := NewEngine(c)
			ctx := context.Background()
			s, err := e.Begin(ctx, RunOptions{DocumentID: "doc", Profile: p})
			if err != nil {
				t.Fatal(err)
			}
			if err = s.OpenNode(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
				t.Fatal(err)
			}
			if err = s.CloseNode(ctx, factEnd(1)); err != nil {
				t.Fatal(err)
			}
			r, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if err != nil {
				t.Fatal(err)
			}
			if !r.Satisfied() {
				t.Fatalf("complete acyclic fact dependency chain did not run provider before consumer: Counts=%+v Findings=%+v", r.Counts, r.Findings)
			}
		})
	}
}
func TestQARejectRuntimeFactProviderCycle(t *testing.T) { qaContextCatalog(t, "consumer", true) }

func TestQADependencySetDigestMustNotHideOutcomeChanges(t *testing.T) {
	var digest string
	var prior Outcome
	for _, reverse := range []bool{false, true} {
		rules, profiles, bindings := catalogFixture(t)
		base := rules[0]
		consumer, unknown, failed := cloneRule(base), cloneRule(base), cloneRule(base)
		consumer.Key.ID = "consumer"
		consumer.Predicate = "okay"
		unknown.Key.ID = "unknown"
		unknown.Predicate = "atomic"
		failed.Key.ID = "failed"
		okay, _ := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}}, 0, -1)
		atomic, _ := NewAtomicComparisonPlan(ComparisonEqual, Atomic{Kind: AtomicString, Text: "expected"})
		bindings = append(bindings, PlanBinding{ID: "okay", Plan: okay}, PlanBinding{ID: "atomic", Plan: atomic})
		consumer.Requires = []Dependency{{Kind: DependencyRule, Rule: unknown.Key, Relation: RelationSameNode}, {Kind: DependencyRule, Rule: failed.Key, Relation: RelationSameNode}}
		if reverse {
			consumer.Requires[0], consumer.Requires[1] = consumer.Requires[1], consumer.Requires[0]
		}
		rules = []RuleDefinition{consumer, unknown, failed}
		profiles[0].Required = []RuleKey{consumer.Key}
		profiles[0].Support = nil
		for _, r := range rules {
			profiles[0].Support = append(profiles[0].Support, RuleSupport{Rule: r.Key, State: ImplementationAvailable})
		}
		c, err := NewCatalog(rules, profiles, bindings)
		if err != nil {
			t.Fatal(err)
		}
		p, _ := c.Profile(profiles[0].ID)
		if digest != "" && digest != p.CatalogDigest {
			t.Fatal("dependency set order changed digest")
		}
		digest = p.CatalogDigest
		e, _ := NewEngine(c)
		ctx := context.Background()
		s, err := e.Begin(ctx, RunOptions{DocumentID: "doc", Profile: p.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err = s.OpenNode(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
			t.Fatal(err)
		}
		if err = s.CloseNode(ctx, factEnd(1)); err != nil {
			t.Fatal(err)
		}
		report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range report.Findings {
			if f.Rule == consumer.Key {
				if prior != "" && prior != f.Outcome {
					t.Fatalf("same canonical digest %s yields consumer %s vs %s after swapping set-like dependency rows", digest, prior, f.Outcome)
				}
				prior = f.Outcome
			}
		}
	}
}

func TestQAReportAxesIndependentAcrossPermutationsAndCaps(t *testing.T) {
	outcomes := []Outcome{OutcomePass, OutcomeFail, OutcomeNotApplicable, OutcomeUnknown, OutcomeUnsupported, OutcomeBlocked}
	for shift := range outcomes {
		for _, category := range []DiagnosticCategory{CategoryNormative, CategoryModel, CategoryPolicy, CategoryAdvisory} {
			var reference Report
			for limitIndex, limit := range []int64{1 << 20, 1, 100, 2000} {
				account, _ := NewBudgetAccount(Budgets{MaxDiagnosticBytes: limit, MaxDiagnostics: 2})
				acc := newReportAccumulator(reportTestProfile(), account)
				rule := reportTestRule()
				rule.Category = category
				for i := range outcomes {
					if err := acc.record(rule, reportTestDecision(outcomes[(i+shift)%len(outcomes)]), Location{Target: TargetModel, DocumentID: "doc"}); err != nil {
						t.Fatal(err)
					}
				}
				report := acc.finish(true, SyntaxNotAssessed)
				if limitIndex == 0 {
					reference = report
				} else if reference.Counts != report.Counts || reference.Conformance != report.Conformance || reference.AssessmentComplete != report.AssessmentComplete || reference.BlockingProblems != report.BlockingProblems || reference.Satisfied() != report.Satisfied() {
					t.Fatalf("diagnostics affected authoritative axes: reference=%+v actual=%+v", reference, report)
				}
			}
		}
	}
}
func TestQAExactNumbersIndependentOracleAndMutableAccessors(t *testing.T) {
	for numerator := int64(-51); numerator <= 51; numerator++ {
		for exponent := int64(0); exponent <= 12; exponent++ {
			den := new(big.Int).Exp(big.NewInt(10), big.NewInt(exponent), nil)
			rat := new(big.Rat).SetFrac(big.NewInt(numerator), den)
			exact, err := NewDecimal(rat)
			if err != nil {
				t.Fatal(err)
			}
			textRat, ok := new(big.Rat).SetString(exact.String())
			if !ok || textRat.Cmp(rat) != 0 {
				t.Fatalf("canonical decimal changed value: %s", exact.String())
			}
			rat.SetInt64(999)
			copy := exact.Value()
			copy.SetInt64(998)
			if exact.Value().Cmp(textRat) != 0 {
				t.Fatal("mutable input/accessor changed decimal")
			}
		}
	}
	huge := new(big.Int).Lsh(big.NewInt(1), 10000)
	n, err := NewInteger(huge)
	if err != nil {
		t.Fatalf("exact integer acquired a machine/domain bound: %v", err)
	}
	copy := n.Value()
	copy.SetInt64(0)
	huge.SetInt64(0)
	if n.Value().BitLen() != 10001 {
		t.Fatal("integer ownership lost")
	}
	if _, err = (Integer{}).Cmp(n); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatal("uninitialized integer treated as zero")
	}
}
func TestQABudgetExtremesAndFailureAtomicity(t *testing.T) {
	for _, kind := range []BudgetKind{BudgetInputBytes, BudgetExternalBytes, BudgetBufferedBytes, BudgetDepth, BudgetNodes, BudgetAttributes, BudgetEvents, BudgetIDs, BudgetRuleSteps, BudgetContextFacts, BudgetLinkedDocuments, BudgetDiagnostics, BudgetDiagnosticBytes} {
		all := Budgets{}
		v := reflect.ValueOf(&all).Elem()
		for i := 0; i < v.NumField(); i++ {
			v.Field(i).SetInt(math.MaxInt64)
		}
		a, err := NewBudgetAccount(all)
		if err != nil {
			t.Fatal(err)
		}
		if err = a.Charge(kind, math.MaxInt64); err != nil {
			t.Fatal(err)
		}
		if err = a.Charge(kind, 1); !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("%s overflow not rejected: %v", kind, err)
		}
		used, _ := a.Usage(kind)
		if used != math.MaxInt64 {
			t.Fatalf("%s refused operation mutated usage", kind)
		}
		if err = a.Charge(kind, -1); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("%s accepted negative charge", kind)
		}
	}
}
