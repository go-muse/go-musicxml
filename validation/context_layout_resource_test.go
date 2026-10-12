package validation

import (
	"context"
	"errors"
	"math/big"
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

func TestLayoutGroupDisplayEnumerationGate(t *testing.T) {
	b := &contextBindings{Roles: map[contextRole][]SchemaUse{}}
	for _, name := range []string{"group-name", "group-abbreviation"} {
		b.Roles[layoutRole(name)] = []SchemaUse{{Declaration: localDecl(name)}}
	}
	for _, name := range []string{"group-name", "group-abbreviation"} {
		t.Run(name, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			s.documentID = "score"
			parent := localNode(1, 0, "part-group")
			parent.closed = false
			n := localNode(2, 1, name)
			use := b.Roles[layoutRole(name)][0]
			n.header.Declaration = Evidence[DeclarationID]{State: EvidenceKnown, Value: use.Declaration}
			s.nodes = map[NodeID]*factNode{1: parent, 2: n}
			r := &contextReader{ctx: context.Background(), s: s, b: b}
			if err := layoutStatic(r, RuleKey{"layout", 1}, n, &contextPlan{}); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, v := range s.contextResults {
				if v.Property == "layout.display-active" {
					found = true
					if v.Layout.Applicability.State != EvidenceUnknown {
						t.Fatalf("unsealed display presence: %+v", v.Layout.Applicability)
					}
				}
			}
			if !found {
				t.Fatal("no applicability")
			}
		})
	}
}

func TestLayoutZeroStaffHasNoImplicitCandidate(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	l := &layoutScopes{r: &contextReader{ctx: context.Background(), s: s}, candidateCache: map[NodeID][]Integer{}}
	zero, err := NewInteger(big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	got, err := l.staffCandidates(NodeRef{"score", 1}, zero)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatal("invented staff one for zero-staff part")
	}
	got, err = l.staffCandidates(NodeRef{"score", 1}, localOne)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].String() != "1" {
		t.Fatal("lost ordinary staff one")
	}
}
