package validation

import (
	"context"
	"errors"
	"testing"
)

func TestPerformanceRound1UnrelatedStaticOwnerCost(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	r := &contextReader{ctx: context.Background(), s: s, b: &contextBindings{Attributes: map[contextRole]map[ComponentID]AttributeSelector{}}}
	n := localNode(1, 0, "step")
	n.header.Type = testEvidence(stateKnown(contextNamedType("step")))
	for i := 0; i < 100; i++ {
		performanceStatic(r, RuleKey{"performance", 1}, n)
	}
	if r.err != nil {
		t.Fatal(r.err)
	}
	if len(s.contextResults) != 0 {
		t.Fatal("unrelated type published static defaults")
	}
	if steps := rationalUsage(t, s, BudgetRuleSteps); steps > 100*100 {
		t.Errorf("unrelated type pays every default role: %d", steps)
	}
}

// A minimal exact-bound scalar role exercises the real provider control flow;
// no timeline service or state role participates in this isolated fixture.
func performanceRound1ScalarProviderFixture(t *testing.T, budgets Budgets) (*Session, *contextPlan) {
	t.Helper()
	schema := localSchema(t)
	bindings, err := compileContextBindingSpecs(schema, []contextBindingSpec{{Role: "perf-prefix", Owner: "root", Name: "relation", Type: "string", Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	cache := &timelineBindingsCache{bindings: &timelineBindings{}}
	cache.once.Do(func() {})
	schema.timeline = cache
	s := rationalSession(t, budgets)
	s.documentID = "d"
	n := localNode(1, 0, "relation")
	n.header.Declaration = testEvidence(stateKnown(bindings.Roles["perf-prefix"][0].Declaration))
	n.header.Type = testEvidence(stateKnown(contextNamedType("string")))
	n.end.Scalar.Value = testEvidence(stateKnown(localString("6")))
	s.nodes = map[NodeID]*factNode{1: n}
	s.sequence = []NodeID{1}
	return s, &contextPlan{Schema: schema, Bindings: bindings}
}
func TestPerformanceRound1ScalarOnlyDispatchAndRoleRefusal(t *testing.T) {
	for _, tight := range []bool{false, true} {
		budget := Budgets{}
		if tight {
			budget.MaxRuleSteps = 2
		}
		s, p := performanceRound1ScalarProviderFixture(t, budget)
		defer s.cleanup()
		_, pending, err := evaluateContextPerformance(context.Background(), s, nil, RuleKey{"performance", 1}, p, true)
		if tight {
			if !errors.Is(err, ErrResourceLimit) || pending || len(s.contextResults) != 0 {
				t.Fatalf("role refusal lost: err=%v pending=%v rows=%d", err, pending, len(s.contextResults))
			}
			if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
				t.Errorf("refusal leaked index: %d", got)
			}
			continue
		}
		if err != nil || pending || len(s.contextResults) != 1 {
			t.Fatalf("scalar-only provider: %v/%v/%d", err, pending, len(s.contextResults))
		}
		got := s.contextResults[0]
		if got.Timing != nil || got.Property != "performance-figure-prefix" || got.Value.Value.Text != "6" {
			t.Errorf("scalar-only output=%+v", got)
		}
	}
}
