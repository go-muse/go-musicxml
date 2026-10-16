package validation

import (
	"context"
	"fmt"
	"slices"
	"testing"
)

func representationScheduleCatalog(t *testing.T, target Target, unrelated int) (*CompiledCatalog, ProfileID, []RuleKey) {
	t.Helper()
	rules, profiles, plans := catalogFixture(t)
	base := rules[0]
	rules = nil
	profiles[0].Target = target
	profiles[0].Required = nil
	profiles[0].Support = nil
	add := func(rule RuleDefinition) {
		rules = append(rules, rule)
		profiles[0].Required = append(profiles[0].Required, rule.Key)
		profiles[0].Support = append(profiles[0].Support, RuleSupport{Rule: rule.Key, State: ImplementationAvailable})
	}
	for i := 0; i < unrelated; i++ {
		rule := base
		rule.Key = RuleKey{ID: RuleID(fmt.Sprintf("ordinary.%04d", i)), Revision: 1}
		rule.Targets = []Target{target}
		add(rule)
	}
	var represented []RuleKey
	if target == TargetModel {
		assembly := AssemblyMusicXML
		plan, err := NewModelRepresentationPlan(assembly)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, PlanBinding{ID: "representation", Plan: plan})
		// Deliberately supply reverse order: the compiled schedule is authoritative.
		for _, id := range []RuleID{"representation.z", "representation.a"} {
			rule := base
			rule.Key = RuleKey{ID: id, Revision: 1}
			rule.Targets = []Target{target}
			rule.Category = CategoryModel
			rule.Selector = Selector{Assembly: &assembly}
			rule.Predicate = "representation"
			add(rule)
			represented = append(represented, rule.Key)
		}
		slices.Reverse(represented)
	}
	catalog, err := NewCatalog(rules, profiles, plans)
	if err != nil {
		t.Fatal(err)
	}
	// Callers cannot alter the compiled classification or schedule through inputs.
	for i := range rules {
		rules[i].Predicate = "changed"
	}
	for i := range profiles[0].Required {
		profiles[0].Required[i].ID = "changed"
	}
	schedule, _ := catalog.Schedule(profiles[0].ID)
	for i := range schedule {
		schedule[i].ID = "changed"
	}
	return catalog, profiles[0].ID, represented
}

func TestCloseUsesCompiledRepresentationSchedule(t *testing.T) {
	ctx := context.Background()
	for _, target := range []Target{TargetSource, TargetModel} {
		var baseline int64 = -1
		for _, unrelated := range []int{1, 64, 512} {
			t.Run(fmt.Sprintf("%s/rules=%d", target, unrelated), func(t *testing.T) {
				catalog, profile, represented := representationScheduleCatalog(t, target, unrelated)
				engine, err := NewEngine(catalog)
				if err != nil {
					t.Fatal(err)
				}
				s, err := engine.Begin(ctx, RunOptions{DocumentID: "doc", Profile: profile})
				if err != nil {
					t.Fatal(err)
				}
				defer s.cleanup()
				header, end := factHeader, factEnd
				if target == TargetModel {
					header, end = representationHeader, representationEnd
				}
				if err := s.Open(ctx, header(1, 0, "test.root", "root")); err != nil {
					t.Fatal(err)
				}
				// White-box fault injection makes each repeated classification observable
				// without wall-clock timing or a production-only instrumentation hook.
				// Public catalogs are immutable. Only this isolated test mutates one.
				catalog.plans["child-count"], err = NewModelRepresentationPlan(AssemblyMusicXML)
				if err != nil {
					t.Fatal(err)
				}
				// Deferred schema binding isolates Close's preparation pass from the
				// later full assessment, which must still visit every selected rule.
				s.schema = &schemaSession{}
				for id := NodeID(2); id <= 4; id++ {
					if err := s.Open(ctx, header(id, 1, "test.child", "child")); err != nil {
						t.Fatal(err)
					}
					if target == TargetModel {
						fact := representationFact(id)
						fact.Problems = []ModelProblem{representationProblem(ModelAxisScalar)}
						if err := s.ModelRepresentation(ctx, fact); err != nil {
							t.Fatal(err)
						}
					}
					closure := end(id)
					if target == TargetModel {
						closure.Scalar.Value = Evidence[Atomic]{State: EvidenceUnsupported, Reason: "model-nonfinite"}
					}
					before, _ := s.account.Usage(BudgetRuleSteps)
					if err := s.Close(ctx, closure); err != nil {
						t.Fatal(err)
					}
					after, _ := s.account.Usage(BudgetRuleSteps)
					if baseline < 0 {
						baseline = after - before
					}
					if got := after - before; got != baseline {
						t.Errorf("preparation grew with unrelated rules: steps=%d want=%d", got, baseline)
					}
					for _, key := range catalog.orders[profile] {
						_, found := s.lookupDecision(decisionID{key, id})
						if found != slices.Contains(represented, key) {
							t.Fatalf("Close reclassified a plan instead of using the compiled representation subset: rule=%s decision=%v", key.ID, found)
						}
					}
					if target == TargetModel {
						v := *s.nodes[id].end.Scalar.Value
						if v.State != EvidenceInvalid || v.Cause == nil || v.Cause.Rule != represented[len(represented)-1] {
							t.Fatalf("ordered causal preparation lost: %+v", v)
						}
					}
				}
				s.cleanup()
				for _, kind := range []BudgetKind{BudgetBufferedBytes, BudgetDepth} {
					if used, _ := s.account.Usage(kind); used != 0 {
						t.Errorf("%s leaked: %d", kind, used)
					}
				}
			})
		}
	}
}
