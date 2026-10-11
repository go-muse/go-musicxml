package validation

import (
	"context"
	"testing"
)

func TestReportSessionUnrelatedDecisionsCannotHideViolation(t *testing.T) {
	for _, diagnosticLimit := range []int64{0, 1} {
		t.Run(map[int64]string{0: "default-budget", 1: "one-diagnostic"}[diagnosticLimit], func(t *testing.T) {
			child := DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}
			pass, _ := NewChildCountPlan([]DeclarationID{child}, 0, -1)
			fail, _ := NewChildCountPlan([]DeclarationID{child}, 1, -1)
			s := qaSession(t,
				qaSpec{id: "a-prerequisite", occurrence: "test.root", p: pass},
				qaSpec{id: "z-required-child", occurrence: "test.root", p: fail, requires: []Dependency{{Kind: DependencyRule, Rule: RuleKey{ID: "a-prerequisite", Revision: 1}, Relation: RelationSameNode}}},
			)
			account, err := NewBudgetAccount(Budgets{MaxDiagnostics: diagnosticLimit})
			if err != nil {
				t.Fatal(err)
			}
			// Use the same catalog with a fresh account so diagnostic usage can be
			// inspected independently of assessment and dependency-cache storage.
			engine, err := NewEngine(s.catalog)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
			s, err = engine.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: "test.source", Account: account})
			if err != nil {
				t.Fatal(err)
			}
			qaOpenRoot(t, s)
			for i := 0; i < 1200; i++ {
				id := NodeID(i + 2)
				if err := s.Open(context.Background(), factHeader(id, 1, "test.unrelated", "unrelated")); err != nil {
					t.Fatal(err)
				}
				qaClose(t, s, factEnd(id))
			}
			for _, kind := range []BudgetKind{BudgetDiagnostics, BudgetDiagnosticBytes} {
				used, err := account.Usage(kind)
				if err != nil || used != 0 {
					t.Errorf("unrelated selector decisions consumed %s: %d (%v)", kind, used, err)
				}
			}
			qaClose(t, s, factEnd(1))
			r := qaFinish(t, s, true)
			if r.Counts != (Counts{Pass: 1, Fail: 1, NotApplicable: 2400}) || r.DiagnosticsTruncated || !r.AssessmentComplete || r.Conformance != ConformanceViolated || r.Satisfied() || r.Terminal != nil {
				t.Errorf("successful/excluded decisions changed report axes: counts=%+v truncated=%t complete=%t conformance=%s terminal=%+v", r.Counts, r.DiagnosticsTruncated, r.AssessmentComplete, r.Conformance, r.Terminal)
			}
			if len(r.Findings) != 1 || len(r.Rules) != 1 {
				t.Fatalf("retained %d findings and %d rules; want only the genuine violation", len(r.Findings), len(r.Rules))
			}
			f := qaFinding(t, r, "z-required-child", 1)
			if f.Outcome != OutcomeFail || f.Reason != "child-count-minimum" || f.Actual != "0" || r.Rules[0].Key != f.Rule {
				t.Errorf("missing-child violation was obscured: %+v", f)
			}
			if used, _ := account.Usage(BudgetDiagnostics); used != 1 {
				t.Errorf("diagnostic count = %d, want 1", used)
			}
		})
	}
}

func TestReportSessionSuccessCountsPreserveDependencyCauses(t *testing.T) {
	child := DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}
	pass, _ := NewChildCountPlan([]DeclarationID{child}, 0, -1)
	fail, _ := NewChildCountPlan([]DeclarationID{child}, 1, -1)
	s := qaSession(t,
		qaSpec{id: "a-pass", occurrence: "test.root", p: pass},
		qaSpec{id: "b-fail", occurrence: "test.root", p: fail, requires: []Dependency{{Kind: DependencyRule, Rule: RuleKey{ID: "a-pass", Revision: 1}, Relation: RelationSameNode}}},
		qaSpec{id: "c-blocked", occurrence: "test.root", p: pass, requires: []Dependency{{Kind: DependencyRule, Rule: RuleKey{ID: "b-fail", Revision: 1}, Relation: RelationSameNode}}},
	)
	qaOpenRoot(t, s)
	qaClose(t, s, factEnd(1))
	r := qaFinish(t, s, true)
	if r.Counts != (Counts{Pass: 1, Fail: 1, Blocked: 1}) || len(r.Findings) != 2 || len(r.Rules) != 2 || r.DiagnosticsTruncated || r.AssessmentComplete || r.Conformance != ConformanceViolated {
		t.Fatalf("success filtering changed dependency outcomes: %+v", r)
	}
	f := qaFinding(t, r, "c-blocked", 1)
	wantCause := DecisionRef{Rule: RuleKey{ID: "b-fail", Revision: 1}, Subject: NodeRef{DocumentID: "doc", Node: 1}}
	if f.Outcome != OutcomeBlocked || f.Reason != "failed-prerequisite" || f.Cause == nil || *f.Cause != wantCause {
		t.Errorf("failed prerequisite cause lost: %+v", f)
	}
}
