package validation

import (
	"reflect"
	"testing"
)

// Parent enumeration does not prove every child's declaration. A gap may be
// the kind that closed the preceding chord, or the next chord's choice.
func TestLocalHarmonyNoneUnavailableDeclarationDoesNotReusePriorChord(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceAbsent, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		for _, tc := range []struct {
			name, afterGap string
			priorKind      bool
			wantKnown      bool
			wantNumeral    bool
			wantGap        bool
		}{
			{"missing-kind", "", false, false, false, true},
			{"missing-choice", "", true, false, false, true},
			{"missing-kind-before-function", "function", false, false, false, true},
			{"later-root-is-independent", "root", false, true, false, false},
			{"later-numeral-is-independent", "numeral", false, true, true, false},
			{"later-kind-closes-gap", "kind", false, false, false, false},
		} {
			t.Run(string(state)+"/"+tc.name, func(t *testing.T) {
				s, n, r := localEvidenceFixture(t, EvidenceKnown, localKind)
				defer s.cleanup()
				n.end.Scalar.Value = testEvidence(localKnownText("none"))
				parent := localNode(1, 0, "harmony")
				prior := localNode(3, 1, "root")
				gap := localNode(4, 1, "kind")
				cause := &DecisionRef{Rule: RuleKey{"child-declaration", 1}, Subject: NodeRef{"doc", 4}}
				if state == EvidenceAbsent {
					cause = nil
				}
				gap.header.Declaration = testEvidence(Evidence[DeclarationID]{State: state, Value: localDecl("kind"), Reason: "child-declaration-gap", Cause: cause})
				parent.children = []*factNode{prior}
				if tc.priorKind {
					parent.children = append(parent.children, localNode(5, 1, "kind"))
				}
				parent.children = append(parent.children, gap)
				if tc.afterGap != "" {
					parent.children = append(parent.children, localNode(6, 1, tc.afterGap))
				}
				parent.children = append(parent.children, n)
				s.nodes[1] = parent
				r.b.Roles[localKind] = append(r.b.Roles[localKind], SchemaUse{Declaration: localDecl("kind")})
				r.b.Roles[localAdvisoryRole("root")] = []SchemaUse{{Declaration: localDecl("root")}}
				r.b.Roles[localAdvisoryRole("numeral")] = []SchemaUse{{Declaration: localDecl("numeral")}}
				if err := localAdvisoryNode(r.ctx, s, RuleKey{"local-test", 1}, n, r); err != nil {
					t.Fatal(err)
				}
				row := harmonyScopeAdvice(t, s)
				want := stateUnknown[bool]("harmony-root-or-numeral-association-unavailable")
				if tc.wantKnown {
					want = stateKnown(true)
				} else if tc.wantGap {
					want = Evidence[bool]{State: state, Reason: "child-declaration-gap", Cause: cause}
					if state == EvidenceAbsent {
						want.State = EvidenceUnknown
						want.Reason = "harmony-child-declaration-unavailable"
					}
				}
				if !reflect.DeepEqual(row.Layout.Applicability, want) {
					t.Errorf("applicability = %+v; want %+v", row.Layout.Applicability, want)
				}
				advice := "set-root-step-text-to-empty-to-suppress-root"
				if tc.wantNumeral {
					advice = "set-numeral-root-text-to-empty-to-suppress-numeral"
				}
				if row.Value.State != EvidenceKnown || row.Value.Value.Text != advice || row.Value.Cause != nil {
					t.Errorf("independent advice = %+v; want %q", row.Value, advice)
				}
			})
		}
	}
}

func TestLocalHarmonyNoneStackedParentCompletenessRemainsPrerequisite(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		t.Run(string(state), func(t *testing.T) {
			s, n, r := localEvidenceFixture(t, EvidenceKnown, localKind)
			defer s.cleanup()
			n.end.Scalar.Value = testEvidence(localKnownText("none"))
			parent := localNode(1, 0, "harmony")
			cause := &DecisionRef{Rule: RuleKey{"parent-content", 1}, Subject: NodeRef{"doc", 1}}
			want := Evidence[bool]{State: state, Reason: "parent-content-gap", Cause: cause}
			parent.end.ChildrenComplete = testEvidence(want)
			parent.children = []*factNode{localNode(3, 1, "root"), localNode(4, 1, "kind"), localNode(5, 1, "numeral"), n}
			s.nodes[1] = parent
			r.b.Roles[localKind] = append(r.b.Roles[localKind], SchemaUse{Declaration: localDecl("kind")})
			r.b.Roles[localAdvisoryRole("root")] = []SchemaUse{{Declaration: localDecl("root")}}
			r.b.Roles[localAdvisoryRole("numeral")] = []SchemaUse{{Declaration: localDecl("numeral")}}
			if err := localAdvisoryNode(r.ctx, s, RuleKey{"local-test", 1}, n, r); err != nil {
				t.Fatal(err)
			}
			if got := harmonyScopeAdvice(t, s).Layout.Applicability; !reflect.DeepEqual(got, want) {
				t.Errorf("applicability = %+v; want %+v", got, want)
			}
		})
	}
}

func harmonyScopeAdvice(t *testing.T, s *Session) ContextResult {
	t.Helper()
	var rows []ContextResult
	for _, row := range s.contextResults {
		if row.Property == "local.advisory-harmony-none-root" {
			rows = append(rows, row)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("advice rows = %d; want 1", len(rows))
	}
	return rows[0]
}
