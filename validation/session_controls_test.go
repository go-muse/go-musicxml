package validation

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestQAControlLifecycleMatrix(t *testing.T) {
	cases := map[string]func(*Session) error{
		"zero-node": func(s *Session) error { return s.Open(context.Background(), factHeader(0, 0, "test.root", "root")) },
		"cross-document": func(s *Session) error {
			h := factHeader(1, 0, "test.root", "root")
			h.Location.DocumentID = "foreign"
			return s.Open(context.Background(), h)
		},
		"bad-target": func(s *Session) error {
			h := factHeader(1, 0, "test.root", "root")
			h.Location.Target = TargetModel
			return s.Open(context.Background(), h)
		},
		"wrong-parent": func(s *Session) error {
			qaOpenRoot(t, s)
			return s.Open(context.Background(), factHeader(2, 9, "test.child", "child"))
		},
		"duplicate-id": func(s *Session) error {
			qaOpenRoot(t, s)
			return s.Open(context.Background(), factHeader(1, 1, "test.child", "child"))
		},
		"duplicate-root": func(s *Session) error {
			qaOpenRoot(t, s)
			qaClose(t, s, factEnd(1))
			return s.Open(context.Background(), factHeader(2, 0, "test.root", "root"))
		},
		"wrong-close": func(s *Session) error { qaOpenRoot(t, s); return s.Close(context.Background(), factEnd(2)) },
		"wrong-attribute-owner": func(s *Session) error {
			qaOpenRoot(t, s)
			a := ciStringNode(2, "K").attributes[0]
			return s.Attribute(context.Background(), a)
		},
		"attribute-after-child": func(s *Session) error {
			qaOpenRoot(t, s)
			ciStreamNode(t, s, 2, 1, "test.child", "")
			a := ciStringNode(1, "K").attributes[0]
			return s.Attribute(context.Background(), a)
		},
		"nil-event-context": func(s *Session) error { return s.Open(nil, factHeader(1, 0, "test.root", "root")) },
	}
	for name, invalid := range cases {
		t.Run(name, func(t *testing.T) {
			_, s := sessionFixture(t, Budgets{})
			first := invalid(s)
			if first == nil {
				t.Fatal("invalid event accepted")
			}
			used, _ := s.account.Usage(BudgetEvents)
			count := len(s.decisions)
			if err := s.Open(context.Background(), factHeader(99, 0, "test.root", "root")); !errors.Is(err, first) {
				t.Fatalf("later event lost cause: %v", err)
			}
			after, _ := s.account.Usage(BudgetEvents)
			if after != used || len(s.decisions) != count {
				t.Fatal("terminal input mutated state")
			}
			a := s.account
			r, err := s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
			if !errors.Is(err, first) || r.Satisfied() {
				t.Fatalf("first cause lost: report=%+v err=%v", r, err)
			}
			for _, k := range []BudgetKind{BudgetDepth, BudgetBufferedBytes} {
				u, _ := a.Usage(k)
				if u != 0 {
					t.Fatalf("live gauge %s leaked %d", k, u)
				}
			}
		})
	}
}
func TestQAControlCancellationJoiningAndCleanup(t *testing.T) {
	_, s := sessionFixture(t, Budgets{})
	qaOpenRoot(t, s)
	qaClose(t, s, factEnd(1))
	transport := errors.New("transport error")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := s.account
	r, err := s.Finish(ctx, Completion{Syntax: SyntaxIncomplete, Cause: transport})
	if !errors.Is(err, transport) || !errors.Is(err, context.Canceled) || r.Counts.Fail != 1 || r.Conformance != ConformanceViolated {
		t.Fatalf("joined partial lost: report=%+v err=%v", r, err)
	}
	for _, k := range []BudgetKind{BudgetDepth, BudgetBufferedBytes} {
		u, _ := a.Usage(k)
		if u != 0 {
			t.Fatalf("leak %s:%d", k, u)
		}
	}
	if _, err := s.Finish(context.Background(), Completion{}); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("Finish twice not rejected")
	}
	if err := s.Attribute(context.Background(), AttributeFact{}); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("post-Finish attribute not rejected")
	}
	if err := s.Close(context.Background(), factEnd(1)); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("post-Finish close not rejected")
	}
}
func TestQAControlFailedPartialAcquisitionDoesNotReleaseSibling(t *testing.T) {
	e, _ := sessionFixture(t, Budgets{})
	a, _ := NewBudgetAccount(Budgets{MaxBufferedBytes: 1 << 20})
	one, err := e.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: "test.source", Account: a})
	if err != nil {
		t.Fatal(err)
	}
	qaOpenRoot(t, one)
	baseline, _ := a.Usage(BudgetBufferedBytes)
	depth, _ := a.Usage(BudgetDepth)
	two, err := e.Begin(context.Background(), RunOptions{DocumentID: "other", Profile: "test.source", Account: a})
	if err != nil {
		t.Fatal(err)
	}
	used, _ := a.Usage(BudgetBufferedBytes)
	spare := a.Limits().MaxBufferedBytes - used
	if err := a.Charge(BudgetBufferedBytes, spare); err != nil {
		t.Fatal(err)
	}
	h := factHeader(1, 0, "test.root", "root")
	h.Location.DocumentID = "other"
	if err := two.Open(context.Background(), h); !errors.Is(err, ErrResourceLimit) {
		t.Fatal("expected header buffer acquisition failure")
	}
	_, _ = two.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
	after, _ := a.Usage(BudgetBufferedBytes)
	afterDepth, _ := a.Usage(BudgetDepth)
	if after != baseline+spare || afterDepth != depth {
		t.Fatalf("child cleanup released sibling: bytes=%d want=%d depth=%d want=%d", after, baseline+spare, afterDepth, depth)
	}
	_ = a.Release(BudgetBufferedBytes, spare)
	_, _ = one.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
	after, _ = a.Usage(BudgetBufferedBytes)
	afterDepth, _ = a.Usage(BudgetDepth)
	if after != 0 || afterDepth != 0 {
		t.Fatalf("final gauges leaked %d/%d", after, afterDepth)
	}
}
func TestQAControlConcurrentSessionIsolation(t *testing.T) {
	e, s := sessionFixture(t, Budgets{})
	_, _ = s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := e.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: "test.source"})
			if err != nil {
				t.Error(err)
				return
			}
			ctx := context.Background()
			if err := s.Open(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
				t.Error(err)
				return
			}
			if err := s.Close(ctx, factEnd(1)); err != nil {
				t.Error(err)
				return
			}
			r, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if err != nil || r.Counts.Fail != 1 {
				t.Errorf("cross-run state: %+v %v", r, err)
			}
		}()
	}
	wg.Wait()
}

func TestQAControlExactDeclarationIsolation(t *testing.T) {
	selected := ciSelector()
	plan, _ := NewAttributePresencePlan(selected, true)
	for _, variant := range []string{"exact", "other-assembly", "other-owner", "other-use", "unknown"} {
		t.Run(variant, func(t *testing.T) {
			s := qaSession(t, qaSpec{id: "presence", occurrence: "test.root", p: plan})
			qaOpenRoot(t, s)
			a := ciStringNode(1, "K").attributes[0]
			switch variant {
			case "other-assembly":
				a.Declaration.Value.Assembly = AssemblyOpus
			case "other-owner":
				a.Declaration.Value.OwnerOccurrence = "foreign.owner"
			case "other-use":
				a.Declaration.Value.Occurrence = "foreign.use"
			case "unknown":
				a.Declaration = Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "binding-unresolved"}
			}
			if err := s.Attribute(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			qaClose(t, s, factEnd(1))
			r := qaFinish(t, s, true)
			f := qaFinding(t, r, "presence", 1)
			want := OutcomeFail
			if variant == "exact" {
				want = OutcomePass
			}
			if variant == "unknown" {
				want = OutcomeUnknown
			}
			if f.Outcome != want {
				t.Fatalf("exact identity collapsed for %s: %+v", variant, f)
			}
		})
	}
}
func TestQAControlAttributeIncompleteCannotInventAbsence(t *testing.T) {
	plan, _ := NewAttributePresencePlan(ciSelector(), true)
	for _, state := range []EvidenceState{EvidenceKnown, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		t.Run(string(state), func(t *testing.T) {
			s := qaSession(t, qaSpec{id: "presence", occurrence: "test.root", p: plan})
			qaOpenRoot(t, s)
			end := factEnd(1)
			end.AttributesComplete = Evidence[bool]{State: state, Reason: "unavailable"}
			if state == EvidenceInvalid {
				end.AttributesComplete.Cause = &DecisionRef{Rule: RuleKey{ID: "upstream", Revision: 1}, Subject: NodeRef{DocumentID: "other-doc", Node: 23}}
			}
			qaClose(t, s, end)
			r := qaFinish(t, s, true)
			f := qaFinding(t, r, "presence", 1)
			want := OutcomeUnknown
			if state == EvidenceUnsupported {
				want = OutcomeUnsupported
			}
			if state == EvidenceInvalid {
				want = OutcomeBlocked
			}
			if f.Outcome != want || r.Satisfied() {
				t.Fatalf("invented absence: %+v", f)
			}
			if state == EvidenceInvalid && (f.Cause == nil || f.Cause.Subject.DocumentID != "other-doc") {
				t.Fatal("cross-document cause lost")
			}
		})
	}
}
func TestQAControlReportDoesNotAliasPositionOrCausalInput(t *testing.T) {
	_, s := sessionFixture(t, Budgets{})
	h := factHeader(1, 0, "test.root", "root")
	h.Location.Position = &SourcePosition{Line: 12, Column: 4, CoordinateSpace: "source"}
	cause := &DecisionRef{Rule: RuleKey{ID: "upstream", Revision: 2}, Subject: NodeRef{DocumentID: "other-doc", Node: 3}}
	h.Declaration = Evidence[DeclarationID]{State: EvidenceInvalid, Reason: "invalid-binding", Cause: cause}
	if err := s.Open(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	h.Location.Position.Line = 999
	cause.Rule.ID = "mutated"
	qaClose(t, s, factEnd(1))
	r := qaFinish(t, s, true)
	f := qaFinding(t, r, "test.child-required", 1)
	if f.Location.Position.Line != 12 || f.Cause.Rule.ID != "upstream" || f.Cause.Subject.DocumentID != "other-doc" {
		t.Fatalf("input alias retained: %+v", f)
	}
}

func TestQAControlChildCountBoundsAndIncompleteEvidence(t *testing.T) {
	decl := DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}
	p, _ := NewChildCountPlan([]DeclarationID{decl}, 1, 2)
	for _, complete := range []bool{false, true} {
		for count := 0; count <= 3; count++ {
			s := qaSession(t, qaSpec{id: "count", occurrence: "test.root", p: p})
			qaOpenRoot(t, s)
			for i := 0; i < count; i++ {
				ciStreamNode(t, s, NodeID(i+2), 1, "test.child", "")
			}
			end := factEnd(1)
			end.ChildrenComplete = Evidence[bool]{State: EvidenceKnown, Value: complete}
			qaClose(t, s, end)
			r := qaFinish(t, s, true)
			f := qaFinding(t, r, "count", 1)
			want := OutcomePass
			if !complete {
				want = OutcomeUnknown
			}
			if count > 2 || (complete && count == 0) {
				want = OutcomeFail
			}
			if f.Outcome != want {
				t.Fatalf("count=%d complete=%t outcome=%s want=%s", count, complete, f.Outcome, want)
			}
		}
	}
}
func TestQAControlParentDependencyDefersAndPreservesCause(t *testing.T) {
	pass, _ := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}}, 0, -1)
	fail, _ := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "other.child"}}, 1, -1)
	s := qaSession(t, qaSpec{id: "a-parent", occurrence: "test.root", p: fail}, qaSpec{id: "z-child", occurrence: "test.child", p: pass, requires: []Dependency{{Kind: DependencyRule, Rule: RuleKey{ID: "a-parent", Revision: 1}, Relation: RelationParent}}})
	qaOpenRoot(t, s)
	ciStreamNode(t, s, 2, 1, "test.child", "")
	qaClose(t, s, factEnd(1))
	r := qaFinish(t, s, true)
	f := qaFinding(t, r, "z-child", 2)
	if f.Outcome != OutcomeBlocked || f.Cause == nil || f.Cause.Subject != (NodeRef{DocumentID: "doc", Node: 1}) || r.Counts.Fail != 1 {
		t.Fatalf("parent dependency/cause broken: %+v %+v", f, r.Counts)
	}
}
