package validation

import (
	"context"
	"errors"
	"testing"
)

func TestCompleteAttributesSealsItsOwnEventPhase(t *testing.T) {
	ctx := context.Background()
	s, err := sourceSchemaEngine(t, XSDCheckAttributes).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteAttributes(ctx, 1, Evidence[bool]{State: EvidenceKnown, Value: true}); err != nil {
		t.Fatal(err)
	}
	a := AttributeFact{Owner: 1, Name: QName{Local: "a"}, Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "schema-binding-pending"}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceKnown, Value: "value"}, Value: Evidence[Atomic]{State: EvidenceUnknown, Reason: "schema-binding-pending"}}, Location: schemaPendingHeader().Location}
	if err := s.Attribute(ctx, a); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("attribute after closure accepted: %v", err)
	}
	_, _ = s.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
}

func TestCompleteAttributesKeepsUnknownAsUnknown(t *testing.T) {
	ctx := context.Background()
	s, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteAttributes(ctx, 1, Evidence[bool]{State: EvidenceUnsupported, Reason: "attribute-enumeration-unavailable"}); err != nil {
		t.Fatal(err)
	}
	subject, err := s.schemaSubject(ctx, s.root)
	if err != nil {
		t.Fatal(err)
	}
	if subject.AttributesComplete {
		t.Fatal("unknown enumeration became complete")
	}
	_, _ = s.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
}

func TestCompleteAttributesLifecycleAndContradiction(t *testing.T) {
	for _, mode := range []string{"wrong-owner", "repeated", "contradictory-close", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			s, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
			if err != nil {
				t.Fatal(err)
			}
			if err := s.OpenNode(ctx, schemaPendingHeader()); err != nil {
				t.Fatal(err)
			}
			complete := Evidence[bool]{State: EvidenceKnown, Value: true}
			switch mode {
			case "wrong-owner":
				err = s.CompleteAttributes(ctx, 2, complete)
			case "repeated":
				if err = s.CompleteAttributes(ctx, 1, complete); err != nil {
					t.Fatal(err)
				}
				err = s.CompleteAttributes(ctx, 1, complete)
			case "contradictory-close":
				if err = s.CompleteAttributes(ctx, 1, complete); err != nil {
					t.Fatal(err)
				}
				end := schemaLexemeEnd("1")
				end.AttributesComplete = Evidence[bool]{State: EvidenceUnsupported, Reason: "contradiction"}
				err = s.CloseNode(ctx, end)
			case "canceled":
				cctx, cancel := context.WithCancel(ctx)
				cancel()
				err = s.CompleteAttributes(cctx, 1, complete)
			}
			if err == nil {
				t.Fatal("invalid lifecycle accepted")
			}
			_, _ = s.Finish(ctx, Completion{Syntax: SyntaxIncomplete, Cause: err})
			for _, kind := range []BudgetKind{BudgetBufferedBytes, BudgetDepth} {
				used, _ := s.account.Usage(kind)
				if used != 0 {
					t.Fatalf("%s retained %d", kind, used)
				}
			}
		})
	}
}
