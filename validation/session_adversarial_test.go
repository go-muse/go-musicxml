package validation

import (
	"context"
	"errors"
	"testing"
)

type qaSpec struct {
	id, occurrence string
	p              Plan
	role           RuleRole
	scope          ScopeKind
	requires       []Dependency
}

func qaSession(t *testing.T, specs ...qaSpec) *Session {
	t.Helper()
	base, ps, _ := catalogFixture(t)
	ps[0].Required = nil
	ps[0].Support = nil
	var rules []RuleDefinition
	var plans []PlanBinding
	for _, x := range specs {
		r := cloneRule(base[0])
		r.Key = RuleKey{ID: RuleID(x.id), Revision: 1}
		r.Predicate = x.id
		r.Selector.Declaration.Occurrence = x.occurrence
		r.Requires = x.requires
		if x.role != "" {
			r.Role = x.role
		}
		if x.scope != "" {
			r.Scope = x.scope
		}
		rules = append(rules, r)
		plans = append(plans, PlanBinding{ID: x.id, Plan: x.p})
		ps[0].Required = append(ps[0].Required, r.Key)
		ps[0].Support = append(ps[0].Support, RuleSupport{Rule: r.Key, State: ImplementationAvailable})
	}
	c, err := NewCatalog(rules, ps, plans)
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEngine(c)
	if err != nil {
		t.Fatal(err)
	}
	s, err := e.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: ps[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func qaFinish(t *testing.T, s *Session, complete bool) Report {
	t.Helper()
	syntax := SyntaxIncomplete
	if complete {
		syntax = SyntaxWellFormed
	}
	r, err := s.Finish(context.Background(), Completion{InputComplete: complete, Syntax: syntax})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func qaFinding(t *testing.T, r Report, id string, node NodeID) Finding {
	t.Helper()
	for _, f := range r.Findings {
		if f.Rule.ID == RuleID(id) && f.Subject.Node == node {
			return f
		}
	}
	t.Fatalf("missing finding %s@%d: %+v", id, node, r)
	return Finding{}
}
func qaOpenRoot(t *testing.T, s *Session) {
	t.Helper()
	if err := s.Open(context.Background(), factHeader(1, 0, "test.root", "root")); err != nil {
		t.Fatal(err)
	}
}
func qaClose(t *testing.T, s *Session, end NodeEnd) {
	t.Helper()
	if err := s.Close(context.Background(), end); err != nil {
		t.Fatal(err)
	}
}

func TestQAContextConsumerSeesLateDocumentConflict(t *testing.T) {
	one := Atomic{Kind: AtomicString, Text: "one"}
	two := Atomic{Kind: AtomicString, Text: "two"}
	compare, _ := NewContextComparisonPlan("value", RelationDocument, ComparisonEqual, one)
	early, _ := NewContextValuePlan("value", RelationDocument, one)
	late, _ := NewContextValuePlan("value", RelationDocument, two)
	s := qaSession(t, qaSpec{id: "a-consumer", occurrence: "test.root", p: compare}, qaSpec{id: "b-early-provider", occurrence: "test.root", p: early, role: RoleContextProvider}, qaSpec{id: "z-late-provider", occurrence: "test.root", p: late, role: RoleContextProvider, scope: ScopeDocument})
	qaOpenRoot(t, s)
	qaClose(t, s, factEnd(1))
	r := qaFinish(t, s, true)
	f := qaFinding(t, r, "a-consumer", 1)
	if f.Outcome != OutcomeUnknown || r.Satisfied() {
		t.Fatalf("late conflicting publication must prevent pass; outcome=%s satisfied=%t counts=%+v", f.Outcome, r.Satisfied(), r.Counts)
	}
}
func TestQAIdentityReferenceWaitsForDeferredDeclaration(t *testing.T) {
	declare, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	ref, _ := NewIdentityReferencePlan("ids", ciSelector())
	base, _ := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}}, 0, -1)
	s := qaSession(t, qaSpec{id: "a-reference", occurrence: "test.ref", p: ref}, qaSpec{id: "m-root", occurrence: "test.root", p: base, scope: ScopeDocument}, qaSpec{id: "z-declaration", occurrence: "test.decl", p: declare, requires: []Dependency{{Kind: DependencyRule, Rule: RuleKey{ID: "m-root", Revision: 1}, Relation: RelationDocument}}})
	qaOpenRoot(t, s)
	ciStreamNode(t, s, 2, 1, "test.ref", "K")
	ciStreamNode(t, s, 3, 1, "test.decl", "K")
	qaClose(t, s, factEnd(1))
	r := qaFinish(t, s, true)
	if r.Counts != (Counts{Pass: 3, NotApplicable: 4}) || !r.Satisfied() || len(r.Findings) != 0 || len(r.Rules) != 0 || r.DiagnosticsTruncated {
		t.Fatalf("forward reference must await deferred declaration, with all three selected obligations passing: %+v", r)
	}
}
func TestQAIdentityCannotCloseIncompleteChildEnumeration(t *testing.T) {
	for _, hasTarget := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "existing"}[hasTarget], func(t *testing.T) {
			declare, _ := NewIdentityDeclarationPlan("ids", ciSelector())
			ref, _ := NewIdentityReferencePlan("ids", ciSelector())
			s := qaSession(t, qaSpec{id: "a-reference", occurrence: "test.ref", p: ref}, qaSpec{id: "z-declaration", occurrence: "test.decl", p: declare})
			qaOpenRoot(t, s)
			ciStreamNode(t, s, 2, 1, "test.ref", "K")
			if hasTarget {
				ciStreamNode(t, s, 3, 1, "test.decl", "K")
			}
			end := factEnd(1)
			end.ChildrenComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "adapter-could-not-enumerate"}
			qaClose(t, s, end)
			r := qaFinish(t, s, true)
			f := qaFinding(t, r, "a-reference", 2)
			if f.Outcome != OutcomeUnknown || r.Satisfied() {
				t.Fatalf("incomplete child enumeration cannot prove missing/unique target; outcome=%s reason=%s satisfied=%t counts=%+v", f.Outcome, f.Reason, r.Satisfied(), r.Counts)
			}
		})
	}
}
func TestQADocumentPrerequisiteCannotProveScopeOnPrefix(t *testing.T) {
	pass, _ := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}}, 0, -1)
	fail, _ := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}}, 1, -1)
	s := qaSession(t, qaSpec{id: "a-prerequisite", occurrence: "test.provider", p: pass}, qaSpec{id: "z-consumer", occurrence: "test.root", p: fail, requires: []Dependency{{Kind: DependencyRule, Rule: RuleKey{ID: "a-prerequisite", Revision: 1}, Relation: RelationDocument}}})
	qaOpenRoot(t, s)
	ciStreamNode(t, s, 2, 1, "test.provider", "")
	qaClose(t, s, factEnd(1))
	r := qaFinish(t, s, false)
	f := qaFinding(t, r, "z-consumer", 1)
	if f.Outcome == OutcomeFail {
		t.Fatalf("incomplete document prerequisite cannot justify dependent failure: %+v", f)
	}
}

func TestQAKnownFailedPrerequisiteWinsOverUnrelatedUnknown(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "unknown-first", true: "failure-first"}[reverse], func(t *testing.T) {
			pass, _ := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}}, 0, -1)
			fail, _ := NewAtomicComparisonPlan(ComparisonEqual, Atomic{Kind: AtomicString, Text: "expected"})
			deps := []Dependency{{Kind: DependencyRule, Rule: RuleKey{ID: "a-unknown", Revision: 1}, Relation: RelationSameNode}, {Kind: DependencyRule, Rule: RuleKey{ID: "b-failure", Revision: 1}, Relation: RelationSameNode}}
			if reverse {
				deps[0], deps[1] = deps[1], deps[0]
			}
			s := qaSession(t, qaSpec{id: "a-unknown", occurrence: "test.root", p: pass}, qaSpec{id: "b-failure", occurrence: "test.root", p: fail}, qaSpec{id: "z-consumer", occurrence: "test.root", p: pass, requires: deps})
			qaOpenRoot(t, s)
			end := factEnd(1)
			end.ChildrenComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "unknown-children"}
			end.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "wrong"}}
			qaClose(t, s, end)
			r := qaFinish(t, s, true)
			f := qaFinding(t, r, "z-consumer", 1)
			if f.Outcome != OutcomeBlocked || f.Cause == nil || f.Cause.Rule.ID != "b-failure" {
				t.Fatalf("known failing prerequisite must causally block regardless of dependency order: %+v", f)
			}
		})
	}
}

func TestQAComputedViolationSurvivesDecisionCacheExhaustion(t *testing.T) {
	_, s := sessionFixture(t, Budgets{})
	qaOpenRoot(t, s)
	a := s.account
	used, _ := a.Usage(BudgetBufferedBytes)
	end := factEnd(1)
	reservation := a.Limits().MaxBufferedBytes - used - measuredRetainedEndBytes(t, end) - 1
	if err := a.Charge(BudgetBufferedBytes, reservation); err != nil {
		t.Fatal(err)
	}
	err := s.Close(context.Background(), end)
	var refused *ResourceError
	if !errors.As(err, &refused) || !s.nodes[1].closed {
		t.Fatalf("expected decision-cache resource failure after end retention: %v", err)
	}
	if left := refused.Limit - refused.Used; left != 1 {
		t.Fatalf("decision-cache refusal has %d bytes left, want exactly 1", left)
	}
	r, _ := s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	_ = a.Release(BudgetBufferedBytes, reservation)
	if r.Counts.Fail != 1 || r.Conformance != ConformanceViolated {
		t.Fatalf("computed child-minimum violation lost on decision-cache allocation failure: counts=%+v conformance=%s", r.Counts, r.Conformance)
	}
}

func TestQACatalogRejectsImplicitContextFactCycle(t *testing.T) {
	rules, profiles, _ := catalogFixture(t)
	base := rules[0]
	rules = nil
	profiles[0].Required = nil
	profiles[0].Support = nil
	var plans []PlanBinding
	for _, pair := range [][2]string{{"a", "b"}, {"b", "a"}} {
		r := cloneRule(base)
		r.Key = RuleKey{ID: RuleID(pair[0]), Revision: 1}
		r.Predicate = pair[0]
		r.Role = RoleContextProvider
		r.Requires = []Dependency{{Kind: DependencyFact, Key: pair[1], Relation: RelationSameNode}}
		p, _ := NewContextValuePlan(pair[0], RelationSameNode, Atomic{Kind: AtomicString, Text: "value"})
		rules = append(rules, r)
		plans = append(plans, PlanBinding{ID: pair[0], Plan: p})
		profiles[0].Required = append(profiles[0].Required, r.Key)
		profiles[0].Support = append(profiles[0].Support, RuleSupport{Rule: r.Key, State: ImplementationAvailable})
	}
	if _, err := NewCatalog(rules, profiles, plans); err == nil {
		t.Fatal("runtime context fact dependency cycle accepted")
	}
}

func TestQADeferredIdentityDeclarationsCollectBeforeUniqueVerdict(t *testing.T) {
	declare, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	base, _ := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}}, 0, -1)
	s := qaSession(t, qaSpec{id: "a-root", occurrence: "test.root", p: base, scope: ScopeDocument}, qaSpec{id: "z-declaration", occurrence: "test.decl", p: declare, requires: []Dependency{{Kind: DependencyRule, Rule: RuleKey{ID: "a-root", Revision: 1}, Relation: RelationDocument}}})
	qaOpenRoot(t, s)
	ciStreamNode(t, s, 2, 1, "test.decl", "K")
	ciStreamNode(t, s, 3, 1, "test.decl", "K")
	qaClose(t, s, factEnd(1))
	r := qaFinish(t, s, true)
	f := qaFinding(t, r, "z-declaration", 2)
	if f.Outcome != OutcomeBlocked || f.Cause == nil || f.Cause.Subject.Node != 3 {
		t.Fatalf("first declaration cannot claim unique before deferred peer registered: %+v", f)
	}
}

func TestQAContextCannotCloseIncompleteProviderEnumeration(t *testing.T) {
	value := Atomic{Kind: AtomicString, Text: "known"}
	for _, relation := range []RelationKind{RelationParent, RelationDocument} {
		t.Run(string(relation), func(t *testing.T) {
			provider, _ := NewContextValuePlan("effective", relation, value)
			consumer, _ := NewContextComparisonPlan("effective", relation, ComparisonEqual, value)
			s := qaSession(t, qaSpec{id: "a-provider", occurrence: "test.provider", p: provider, role: RoleContextProvider}, qaSpec{id: "z-consumer", occurrence: "test.consumer", p: consumer})
			qaOpenRoot(t, s)
			ciStreamNode(t, s, 2, 1, "test.provider", "")
			ciStreamNode(t, s, 3, 1, "test.consumer", "")
			end := factEnd(1)
			end.ChildrenComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "unseen-provider-possible"}
			qaClose(t, s, end)
			r := qaFinish(t, s, true)
			f := qaFinding(t, r, "z-consumer", 3)
			if f.Outcome != OutcomeUnknown || r.Satisfied() {
				t.Fatalf("incomplete provider enumeration cannot prove settled context: outcome=%s satisfied=%t", f.Outcome, r.Satisfied())
			}
		})
	}
}
