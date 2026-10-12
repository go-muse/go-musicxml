package validation

import (
	"context"
	"errors"
	"testing"
)

func TestLocalProjectionOwnsRawCausalAndLocationEvidence(t *testing.T) {
	s := rationalSession(t, Budgets{})
	s.documentID = "score"
	defer s.cleanup()
	n := localNode(3, 1, "note")
	n.header.Location = Location{Target: TargetSource, DocumentID: "score", Path: "/note", Position: &SourcePosition{Line: 4}}
	raw := Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-print-object", Cause: &DecisionRef{Rule: RuleKey{"xsd-enumeration", 1}, Subject: NodeRef{"score", 3}}}
	if err := localPublish(context.Background(), s, RuleKey{"local-test", 1}, n, ContextPrintObject, raw, raw, Evidence[bool]{State: EvidenceKnown, Value: true}, NodeRef{}, ContextUnitNone); err != nil {
		t.Fatal(err)
	}
	raw.Cause.Rule.ID = "changed"
	n.header.Location.Position.Line = 999
	got := s.contextResults[0]
	if got.Value.Cause.Rule.ID != "xsd-enumeration" || got.Layout.Raw.Cause.Rule.ID != "xsd-enumeration" || got.Origins[0].Position.Line != 4 {
		t.Fatalf("borrowed evidence escaped: %+v", got)
	}
}
func TestLocalProjectionBudgetAndCancellationAreNotNormativeFailures(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		limits := Budgets{MaxBufferedBytes: 1}
		if cancel {
			limits = Budgets{}
		}
		s := rationalSession(t, limits)
		s.documentID = "score"
		ctx, cancelFn := context.WithCancel(context.Background())
		if cancel {
			cancelFn()
		}
		n := localNode(1, 0, "note")
		err := localPublish(ctx, s, RuleKey{"local-test", 1}, n, ContextPrintObject, Evidence[Atomic]{State: EvidenceAbsent}, localKnownText("yes"), Evidence[bool]{State: EvidenceKnown, Value: true}, NodeRef{}, ContextUnitNone)
		if cancel {
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v", err)
			}
		} else if !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("got %v", err)
		}
		if len(s.contextResults) != 0 {
			t.Fatal("partial publication escaped")
		}
		s.cleanup()
		used, _ := s.account.Usage(BudgetBufferedBytes)
		if used != 0 {
			t.Fatalf("leaked %d", used)
		}
		cancelFn()
	}
}
func TestLocalNoteheadMissingTypeAndEnclosureAreFieldLocal(t *testing.T) {
	if got := localNoteheadFill(Evidence[Atomic]{State: EvidenceAbsent}); got.State != EvidenceUnknown {
		t.Fatalf("missing type is not absence of fill: %+v", got)
	}
	if got := localEnclosedNotehead(Evidence[string]{State: EvidenceKnown, Value: "other"}); got.State != EvidenceUnknown {
		t.Fatalf("custom glyph geometry guessed: %+v", got)
	}
	if got := localEnclosedNotehead(Evidence[string]{State: EvidenceKnown, Value: "x"}); got.State != EvidenceKnown || got.Value {
		t.Fatalf("open x treated enclosed: %+v", got)
	}
}
