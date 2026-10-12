package validation

import (
	"context"
	"errors"
	"math"
	"testing"
)

func navigationResourceEvents(t *testing.T) []navigationEvent {
	t.Helper()
	return []navigationEvent{
		{Source: NodeRef{"d", 1}, Kind: NavigationForward},
		{Source: NodeRef{"d", 2}, Kind: NavigationSound, Tempo: stateKnown(stateDecimal(t, 80, 1)), Guard: stateKnown("1"), PassDomain: navigationSingleRepeatPass},
		{Source: NodeRef{"d", 3}, Kind: NavigationNote, PassDomain: navigationSingleRepeatPass},
		{Source: NodeRef{"d", 4}, Kind: NavigationBackward, Target: stateKnown(0), Times: stateKnown(stateInt(t, 3))},
	}
}
func TestContextNavigationEveryCancellationBoundary(t *testing.T) {
	events := navigationResourceEvents(t)
	s := rationalSession(t, Budgets{})
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	if _, err := runNavigation(counter, s, events); err != nil {
		t.Fatal(err)
	}
	calls := counter.calls
	s.cleanup()
	for at := 1; at <= calls; at++ {
		s := rationalSession(t, Budgets{})
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		_, err := runNavigation(ctx, s, events)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll%d err=%v", at, err)
		}
		work, facts := rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetContextFacts)
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || rationalUsage(t, s, BudgetRuleSteps) != work || rationalUsage(t, s, BudgetContextFacts) != facts {
			t.Fatalf("poll%d leaked or refunded cumulative budget", at)
		}
		if events[1].Tempo.Value.String() != "80" || events[3].Times.Value.String() != "3" {
			t.Fatal("input mutated")
		}
	}
	t.Logf("interrupted all %d navigation polls", calls)
}
func TestContextNavigationSharedRetentionAndFactLimits(t *testing.T) {
	for _, budgets := range []Budgets{{MaxBufferedBytes: 512}, {MaxContextFacts: 2}, {MaxRuleSteps: 20}} {
		s := rationalSession(t, budgets)
		_, err := runNavigation(context.Background(), s, navigationResourceEvents(t))
		if !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("limits %+v err=%v", budgets, err)
		}
		work, facts := rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetContextFacts)
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || rationalUsage(t, s, BudgetRuleSteps) != work || rationalUsage(t, s, BudgetContextFacts) != facts {
			t.Fatal("resource ownership broken")
		}
	}
}
func TestContextRepeatOwnedScopeProjection(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	source := RepeatContextResult{Kind: RepeatStyleSlash, Notes: []RepeatNoteScope{{Source: NodeRef{"d", 2}, Applies: Evidence[bool]{State: EvidenceInvalid, Cause: &DecisionRef{Rule: RuleKey{"x", 1}, Subject: NodeRef{"d", 3}}}}}, ExceptVoices: []Evidence[string]{stateKnown("1")}}
	got, err := cloneRepeatContext(context.Background(), s, &source)
	if err != nil {
		t.Fatal(err)
	}
	source.Notes[0].Source.Node = 99
	source.Notes[0].Applies.Cause.Subject.Node = 99
	source.ExceptVoices[0].Value = "9"
	if got.Notes[0].Source.Node != 2 || got.Notes[0].Applies.Cause.Subject.Node != 3 || got.ExceptVoices[0].Value != "1" {
		t.Fatal("scope clone aliased")
	}
}
