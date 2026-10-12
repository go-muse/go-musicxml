package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestLayoutPublisherAllocationAdmittedAndCanceled(t *testing.T) {
	for _, tc := range []struct {
		name   string
		budget Budgets
		cancel bool
	}{{"quota", Budgets{MaxBufferedBytes: 1}, false}, {"cancellation", Budgets{}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			s := rationalSession(t, tc.budget)
			defer s.cleanup()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			_, err := newLayoutFormatting(&contextReader{ctx: ctx, s: s}, RuleKey{"layout", 1})
			if tc.cancel {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("got %v", err)
				}
			} else if !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("got %v", err)
			}
			if s.retained != 0 {
				t.Fatalf("retained %d before admission", s.retained)
			}
		})
	}
}
func TestLayoutOwnedProjectionAndLongTextWork(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	cause := &DecisionRef{Rule: RuleKey{"cause", 1}, Subject: NodeRef{"doc", 2}}
	in := &LayoutContextResult{Scope: NodeRef{"doc", 1}, Origin: NodeRef{"doc", 2}, Raw: Evidence[Atomic]{State: EvidenceUnknown, Cause: cause}, Applicability: Evidence[bool]{State: EvidenceUnknown, Cause: cause}, Basis: strings.Repeat("basis", 100)}
	out, err := cloneLayoutContext(context.Background(), s, in)
	if err != nil {
		t.Fatal(err)
	}
	cause.Rule.ID = "mutated"
	if out.Raw.Cause.Rule.ID != "cause" || out.Applicability.Cause.Rule.ID != "cause" {
		t.Fatal("projection aliases cause")
	}
}
