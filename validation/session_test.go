package validation

import (
	"context"
	"errors"
	"testing"
)

func sessionFixture(t *testing.T, limits Budgets) (*Engine, *Session) {
	t.Helper()
	r, p, b := catalogFixture(t)
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEngine(c)
	if err != nil {
		t.Fatal(err)
	}
	s, err := e.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: "test.source", Budgets: limits})
	if err != nil {
		t.Fatal(err)
	}
	return e, s
}
func factHeader(id, parent NodeID, occurrence, name string) NodeHeader {
	return NodeHeader{ID: id, Parent: parent, Name: QName{Local: name}, Type: Evidence[ComponentID]{State: EvidenceUnknown, Reason: "not-bound"}, Declaration: Evidence[DeclarationID]{State: EvidenceKnown, Value: DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: occurrence}}, Location: Location{Target: TargetSource, DocumentID: "doc", Path: "/" + name}}
}
func factEnd(id NodeID) NodeEnd {
	return NodeEnd{ID: id, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceAbsent}, Value: Evidence[Atomic]{State: EvidenceAbsent}}, ChildrenComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, AttributesComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, Order: Evidence[OrderKind]{State: EvidenceKnown, Value: OrderSource}}
}
func TestSessionRunsRealExactDeclarationPlan(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			_, s := sessionFixture(t, Budgets{})
			ctx := context.Background()
			if err := s.Open(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < count; i++ {
				id := NodeID(i + 2)
				if err := s.Open(ctx, factHeader(id, 1, "test.child", "child")); err != nil {
					t.Fatal(err)
				}
				if err := s.Close(ctx, factEnd(id)); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Close(ctx, factEnd(1)); err != nil {
				t.Fatal(err)
			}
			r, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if err != nil {
				t.Fatal(err)
			}
			if count == 0 {
				if r.Conformance != ConformanceViolated || !r.AssessmentComplete || r.Satisfied() {
					t.Fatalf("missing child report=%+v", r)
				}
			} else if !r.Satisfied() {
				t.Fatalf("valid report=%+v", r)
			}
		})
	}
}
func TestSessionMissingSelectionNeverDisappears(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported} {
		t.Run(string(state), func(t *testing.T) {
			_, s := sessionFixture(t, Budgets{})
			h := factHeader(1, 0, "test.root", "root")
			h.Declaration = Evidence[DeclarationID]{State: state, Reason: "binding-missing"}
			if err := s.Open(context.Background(), h); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(context.Background(), factEnd(1)); err != nil {
				t.Fatal(err)
			}
			r, err := s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if err != nil {
				t.Fatal(err)
			}
			if r.Satisfied() || r.AssessmentComplete || r.Conformance != ConformanceUndetermined {
				t.Fatalf("missing evidence passed:%+v", r)
			}
		})
	}
}
func TestSessionTerminalCancellationPreservesEarlierFailure(t *testing.T) {
	_, s := sessionFixture(t, Budgets{})
	ctx := context.Background()
	if err := s.Open(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(ctx, factEnd(1)); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	r, err := s.Finish(canceled, Completion{InputComplete: false, Syntax: SyntaxIncomplete})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrIncompleteAssessment) {
		t.Fatalf("lost cause:%v", err)
	}
	if r.Conformance != ConformanceViolated || r.AssessmentComplete || r.Counts.Fail != 1 || r.Terminal == nil {
		t.Fatalf("failure lost:%+v", r)
	}
	if _, err := s.Finish(ctx, Completion{}); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("second finish:%v", err)
	}
	if err := s.Open(ctx, factHeader(2, 0, "test.root", "root")); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("postfinish:%v", err)
	}
}
func TestSessionLifecycleViolationIsTerminal(t *testing.T) {
	_, s := sessionFixture(t, Budgets{})
	ctx := context.Background()
	if err := s.Open(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
		t.Fatal(err)
	}
	original := s.Close(ctx, factEnd(2))
	if original == nil {
		t.Fatal("wrong close accepted")
	}
	if err := s.Close(ctx, factEnd(1)); err == nil {
		t.Fatal("failed session resumed")
	}
	r, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if !errors.Is(err, original) || r.Satisfied() || r.AssessmentComplete {
		t.Fatalf("lost failed cause:%+v %v", r, err)
	}
}
func TestSessionSharesAccountWithoutResetOrLimitOverride(t *testing.T) {
	e, _ := sessionFixture(t, Budgets{})
	a, err := NewBudgetAccount(Budgets{MaxNodes: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Begin(context.Background(), RunOptions{DocumentID: "bad", Profile: "test.source", Account: a, Budgets: Budgets{MaxNodes: 2}}); err == nil {
		t.Fatal("override accepted")
	}
	first, err := e.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: "test.source", Account: a})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Open(context.Background(), factHeader(1, 0, "test.root", "root")); err != nil {
		t.Fatal(err)
	}
	_, _ = first.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
	second, err := e.Begin(context.Background(), RunOptions{DocumentID: "doc2", Profile: "test.source", Account: a})
	if err != nil {
		t.Fatal(err)
	}
	h := factHeader(1, 0, "test.root", "root")
	h.Location.DocumentID = "doc2"
	if err := second.Open(context.Background(), h); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("quota reset:%v", err)
	}
	_, _ = second.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
	used, _ := a.Usage(BudgetDepth)
	if used != 0 {
		t.Fatalf("depth leak:%d", used)
	}
	used, _ = a.Usage(BudgetBufferedBytes)
	if used != 0 {
		t.Fatalf("buffer leak:%d", used)
	}
}

func TestSessionRejectsKnownPayloadOnProvenAbsentAttribute(t *testing.T) {
	for _, lexical := range []bool{false, true} {
		_, s := sessionFixture(t, Budgets{})
		ctx := context.Background()
		if err := s.Open(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
			t.Fatal(err)
		}
		attribute := AttributeFact{Owner: 1, Name: QName{Local: "id"}, Declaration: Evidence[DeclarationID]{State: EvidenceKnown, Value: DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.id"}}, Presence: Evidence[bool]{State: EvidenceAbsent}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceAbsent}, Value: Evidence[Atomic]{State: EvidenceAbsent}}, Location: Location{Target: TargetSource, DocumentID: "doc", Path: "/root/@id"}}
		if lexical {
			attribute.Scalar.Lexeme = Evidence[string]{State: EvidenceKnown, Value: ""}
		} else {
			attribute.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: ""}}
		}
		if err := s.Attribute(ctx, attribute); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("absent attribute carried known payload: %v", err)
		}
		r, err := s.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
		if !errors.Is(err, ErrIncompleteAssessment) || r.Satisfied() {
			t.Fatalf("contradiction did not terminate: %+v %v", r, err)
		}
	}
}
