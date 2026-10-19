package validation

import (
	"context"
	"testing"
)

func TestQARound2DeferredDuplicateSurvivesUnrelatedIncompleteEnumeration(t *testing.T) {
	declare, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	provider, _ := NewContextValuePlan("ready", RelationSameNode, Atomic{Kind: AtomicString, Text: "yes"})
	s := qaSession(t, qaSpec{id: "a-provider", occurrence: "test.decl", p: provider, role: RoleContextProvider}, qaSpec{id: "z-declaration", occurrence: "test.decl", p: declare, requires: []Dependency{{Kind: DependencyFact, Key: "ready", Relation: RelationSameNode}}})
	qaOpenRoot(t, s)
	ciStreamNode(t, s, 2, 1, "test.decl", "K")
	ciStreamNode(t, s, 3, 1, "test.decl", "K")
	end := factEnd(1)
	end.ChildrenComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "unrelated-unseen-children"}
	qaClose(t, s, end)
	r := qaFinish(t, s, true)
	if r.Counts.Fail != 1 || r.Conformance != ConformanceViolated {
		t.Fatalf("known duplicate lost solely to unrelated incomplete enumeration: counts=%+v conformance=%s findings=%+v", r.Counts, r.Conformance, r.Findings)
	}
}

func TestQARound2DeferredCrossRuleDeclarationsAndReference(t *testing.T) {
	declare, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	reference, _ := NewIdentityReferencePlan("ids", ciSelector())
	provider, _ := NewContextValuePlan("ready", RelationDocument, Atomic{Kind: AtomicString, Text: "yes"})
	deps := []Dependency{{Kind: DependencyFact, Key: "ready", Relation: RelationDocument}}
	s := qaSession(t, qaSpec{id: "a-declaration", occurrence: "test.decl-a", p: declare, requires: deps}, qaSpec{id: "b-declaration", occurrence: "test.decl-b", p: declare, requires: deps}, qaSpec{id: "c-reference", occurrence: "test.ref", p: reference}, qaSpec{id: "z-provider", occurrence: "test.root", p: provider, role: RoleContextProvider, scope: ScopeDocument})
	qaOpenRoot(t, s)
	ciStreamNode(t, s, 2, 1, "test.ref", "K")
	ciStreamNode(t, s, 3, 1, "test.decl-a", "K")
	ciStreamNode(t, s, 4, 1, "test.decl-b", "K")
	qaClose(t, s, factEnd(1))
	r := qaFinish(t, s, true)
	for _, want := range []struct {
		id      string
		node    NodeID
		outcome Outcome
	}{{"a-declaration", 3, OutcomeBlocked}, {"b-declaration", 4, OutcomeFail}, {"c-reference", 2, OutcomeBlocked}} {
		f := qaFinding(t, r, want.id, want.node)
		if f.Outcome != want.outcome {
			t.Fatalf("%s verdict=%s want=%s", want.id, f.Outcome, want.outcome)
		}
		if want.outcome == OutcomeBlocked && (f.Cause == nil || f.Cause.Rule.ID != "b-declaration" || f.Cause.Subject.Node != 4) {
			t.Fatalf("bad first collision cause: %+v", f)
		}
	}
	if r.Counts.Fail != 1 {
		t.Fatalf("duplicate failure should occur once: %+v", r.Counts)
	}
}

func TestQARound2DefaultDoesNotReplacePresentMissingScalar(t *testing.T) {
	fallback := Atomic{Kind: AtomicString, Text: "fallback"}
	provider, _ := NewContextDefaultPlan("effective", RelationSameNode, ciSelector(), fallback)
	consumer, _ := NewContextComparisonPlan("effective", RelationSameNode, ComparisonEqual, fallback)
	for _, state := range []EvidenceState{EvidenceAbsent, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		t.Run(string(state), func(t *testing.T) {
			s := qaSession(t, qaSpec{id: "a-default", occurrence: "test.root", p: provider, role: RoleContextProvider}, qaSpec{id: "z-consumer", occurrence: "test.root", p: consumer})
			qaOpenRoot(t, s)
			a := ciStringNode(1, "K").attributes[0]
			a.Scalar.Value = Evidence[Atomic]{State: state}
			if state != EvidenceAbsent {
				a.Scalar.Value.Reason = "unavailable"
			}
			if state == EvidenceInvalid {
				a.Scalar.Value.Cause = &DecisionRef{Rule: RuleKey{ID: "upstream", Revision: 1}, Subject: NodeRef{DocumentID: "doc", Node: 1}}
			}
			if err := s.Attribute(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			qaClose(t, s, factEnd(1))
			r := qaFinish(t, s, true)
			f := qaFinding(t, r, "z-consumer", 1)
			if f.Outcome == OutcomePass || f.Outcome == OutcomeFail || r.Satisfied() {
				t.Fatalf("present unavailable scalar defaulted: %+v", f)
			}
		})
	}
}

func TestQARound2DeferredLocalFailureSurvivesUnrelatedEnumeration(t *testing.T) {
	provider, _ := NewContextValuePlan("ready", RelationSameNode, Atomic{Kind: AtomicString, Text: "yes"})
	predicate, _ := NewAtomicComparisonPlan(ComparisonEqual, Atomic{Kind: AtomicString, Text: "expected"})
	s := qaSession(t, qaSpec{id: "a-provider", occurrence: "test.child", p: provider, role: RoleContextProvider}, qaSpec{id: "z-predicate", occurrence: "test.child", p: predicate, requires: []Dependency{{Kind: DependencyFact, Key: "ready", Relation: RelationSameNode}}})
	qaOpenRoot(t, s)
	if err := s.OpenNode(context.Background(), factHeader(2, 1, "test.child", "child")); err != nil {
		t.Fatal(err)
	}
	childEnd := factEnd(2)
	childEnd.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "actual"}}
	qaClose(t, s, childEnd)
	rootEnd := factEnd(1)
	rootEnd.ChildrenComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "other-children-unavailable"}
	qaClose(t, s, rootEnd)
	r := qaFinish(t, s, true)
	f := qaFinding(t, r, "z-predicate", 2)
	if f.Outcome != OutcomeFail {
		t.Fatalf("independent closed local predicate suppressed by unrelated scope gap: %+v", f)
	}
}
