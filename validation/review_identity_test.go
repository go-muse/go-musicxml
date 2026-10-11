package validation

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func reviewIdentityNode(t *testing.T, s *Session, id NodeID, occurrence string, value Atomic) {
	t.Helper()
	ctx := context.Background()
	if err := s.Open(ctx, factHeader(id, 1, occurrence, "slot")); err != nil {
		t.Fatal(err)
	}
	a := ciStringNode(id, "unused").attributes[0]
	a.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: value}
	if err := s.Attribute(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(ctx, factEnd(id)); err != nil {
		t.Fatalf("valid typed family must remain a per-rule outcome: %v", err)
	}
}

func TestReviewIdentityFamilyMismatchIsUnsupportedWithoutAborting(t *testing.T) {
	integer, _ := NewInteger(big.NewInt(7))
	decimal, _ := NewDecimal(big.NewRat(3, 2))
	for _, value := range []Atomic{{Kind: AtomicInteger, Integer: integer}, {Kind: AtomicDecimal, Decimal: decimal}, {Kind: AtomicBoolean, Text: "true"}, {Kind: AtomicQName, Name: QName{Local: "key"}}} {
		t.Run(string(value.Kind), func(t *testing.T) {
			declare, _ := NewIdentityDeclarationPlan("ids", ciSelector())
			reference, _ := NewIdentityReferencePlan("ids", ciSelector())
			predicate, _ := NewAtomicComparisonPlan(ComparisonEqual, Atomic{Kind: AtomicString, Text: "expected"})
			s := qaSession(t, qaSpec{id: "declare", occurrence: "test.decl", p: declare}, qaSpec{id: "reference", occurrence: "test.ref", p: reference}, qaSpec{id: "independent", occurrence: "test.root", p: predicate})
			qaOpenRoot(t, s)
			reviewIdentityNode(t, s, 2, "test.decl", value)
			reviewIdentityNode(t, s, 3, "test.ref", Atomic{Kind: AtomicString, Text: "missing"})
			end := factEnd(1)
			end.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "actual"}}
			qaClose(t, s, end)
			r := qaFinish(t, s, true)
			if d := qaFinding(t, r, "declare", 2); d.Outcome != OutcomeUnsupported || d.Reason != "identity-family-unavailable" {
				t.Fatalf("mismatch=%+v", d)
			}
			if d := qaFinding(t, r, "reference", 3); d.Outcome != OutcomeUnknown {
				t.Fatalf("unsupported declaration fabricated absence: %+v", d)
			}
			if r.Terminal != nil || r.Counts.Fail != 1 || r.Conformance != ConformanceViolated || r.Satisfied() {
				t.Fatalf("independent assessment was lost: %+v", r)
			}
		})
	}
}

func TestReviewUnsupportedReferenceDoesNotPoisonDeclarationIndex(t *testing.T) {
	declare, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	reference, _ := NewIdentityReferencePlan("ids", ciSelector())
	s := qaSession(t, qaSpec{id: "declare", occurrence: "test.decl", p: declare}, qaSpec{id: "reference", occurrence: "test.ref", p: reference})
	qaOpenRoot(t, s)
	reviewIdentityNode(t, s, 2, "test.decl", Atomic{Kind: AtomicString, Text: "K"})
	reviewIdentityNode(t, s, 3, "test.ref", Atomic{Kind: AtomicBoolean, Text: "true"})
	reviewIdentityNode(t, s, 4, "test.ref", Atomic{Kind: AtomicString, Text: "K"})
	qaClose(t, s, factEnd(1))
	r := qaFinish(t, s, true)
	if r.Terminal != nil || r.Counts.Unsupported != 1 || r.Counts.Unknown != 0 || r.Counts.Fail != 0 || r.Counts.Pass != 2 {
		t.Fatalf("unsupported reference tainted valid index: %+v", r)
	}
	if d := qaFinding(t, r, "reference", 3); d.Outcome != OutcomeUnsupported {
		t.Fatalf("reference mismatch=%+v", d)
	}
}

func TestReviewMalformedAtomicPayloadRemainsContractFailure(t *testing.T) {
	declare, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	s := qaSession(t, qaSpec{id: "declare", occurrence: "test.root", p: declare})
	qaOpenRoot(t, s)
	a := ciStringNode(1, "unused").attributes[0]
	a.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Text: "not-an-integer-wrapper"}}
	if err := s.Attribute(context.Background(), a); !errors.Is(err, ErrInvalidConfiguration) || !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("malformed fact was accepted: %v", err)
	}
	r, err := s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
	if !errors.Is(err, ErrIncompleteAssessment) || r.Terminal == nil || r.Satisfied() {
		t.Fatalf("invalid payload did not terminate: %+v %v", r, err)
	}
}
