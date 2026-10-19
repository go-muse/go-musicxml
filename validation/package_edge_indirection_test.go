package validation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func edgeIndirectionFixture(t *testing.T, check PackageCheck, root, text string, budgets ...Budgets) (*PackageSession, PackageEdgeObservation) {
	t.Helper()
	s, source, _ := packageAdviceFixtureCheck(t, check, budgets...)
	sourceSnapshot := s.adoptions[source].resource.snapshot
	sourceSnapshot.contexts = []ContextResult{{Subject: NodeRef{"source", 2}, Resource: &ResourceContextResult{Link: &ResourceLinkContext{Kind: ResourceOpusLink, Href: ResourceAttribute{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "inner.mxl"}}}}}}}
	targetSnapshot, _ := interruptionSnapshot(t, s.account, "inner", text)
	targetSnapshot.header.Name = Evidence[QName]{State: EvidenceKnown, Value: QName{Local: root}}
	targetResource, err := s.addResource(context.Background(), testResourcePin("inner"), targetSnapshot, true)
	if err != nil {
		t.Fatal(err)
	}
	target, err := s.adoptResource(context.Background(), targetResource, ResourceAdoptionOptions{Package: "outer", DocumentID: "inner-view"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !s.finished {
			_, _ = s.finish(context.Background(), PackageCompletion{})
		}
	})
	return s, PackageEdgeObservation{ID: 1, Source: source, Target: Evidence[ResourceAdoptionID]{State: EvidenceKnown, Value: target}, Indirection: Evidence[bool]{State: EvidenceKnown, Value: true}, Fragment: Evidence[string]{State: EvidenceAbsent}, FragmentTarget: Evidence[NodeRef]{State: EvidenceAbsent}}
}

func finishIndirectionFixture(t *testing.T, s *PackageSession) AssessmentBundle {
	t.Helper()
	sealFixtureExpansions(t, s)
	if err := s.sealEnumeration(context.Background(), completePackage("outer")); err != nil {
		t.Fatal(err)
	}
	b, err := s.finish(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("retained %d bytes after Finish", used)
	}
	return b
}

func assertIndirectionReport(t *testing.T, r Report, want Outcome, reason Code) {
	t.Helper()
	var got uint64
	switch want {
	case OutcomePass:
		got = r.Counts.Pass
	case OutcomeFail:
		got = r.Counts.Fail
	case OutcomeUnknown:
		got = r.Counts.Unknown
	case OutcomeUnsupported:
		got = r.Counts.Unsupported
	case OutcomeBlocked:
		got = r.Counts.Blocked
	case OutcomeNotApplicable:
		got = r.Counts.NotApplicable
	}
	if got != 1 {
		t.Fatalf("want one %s: %+v", want, r)
	}
	if want != OutcomePass && want != OutcomeNotApplicable && (len(r.Findings) != 1 || r.Findings[0].Reason != reason) {
		t.Fatalf("want reason %q: %+v", reason, r)
	}
}

func TestPackageEdgeIndirectionDoesNotJudgeInnerPrimary(t *testing.T) {
	for _, kind := range []ResourceLinkKind{ResourceOpusLink, ResourceOpusScore, ResourcePartLink, ResourceScoreWorkOpus, ResourceScoreLink} {
		for _, root := range []string{"opus", "score-partwise", "score-timewise"} {
			for _, indirect := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/indirect=%t", kind, root, indirect), func(t *testing.T) {
					s, e := edgeIndirectionFixture(t, PackageCheckLinkTarget, root, "1")
					s.adoptions[e.Source].resource.snapshot.contexts[0].Resource.Link.Kind = kind
					e.Indirection.Value = indirect
					if err := s.observeEdge(context.Background(), e); err != nil {
						t.Fatal(err)
					}
					b := finishIndirectionFixture(t, s)
					want, reason := OutcomePass, Code("linked-target-kind-established")
					wantScore := kind == ResourceOpusScore || kind == ResourcePartLink
					wantOpus := kind == ResourceOpusLink || kind == ResourceScoreWorkOpus
					if wantScore && root == "opus" || wantOpus && root != "opus" {
						want, reason = OutcomeFail, "wrong-linked-document-kind"
					}
					if indirect {
						want, reason = OutcomeUnknown, "nested-link-indirection-unresolved"
					}
					assertIndirectionReport(t, b.PackageReport(), want, reason)
					if indirect && b.Satisfied() {
						t.Fatal("indirect edge certified satisfaction")
					}
					if b.ResourceCount() != 2 {
						t.Fatal("inner intrinsic assessment lost")
					}
				})
			}
		}
	}
}

func TestPackageEdgeIndirectionPrecedesInnerFragmentAndRootEvidence(t *testing.T) {
	for _, fragment := range []string{"", "missing", "xpointer(/1)"} {
		t.Run(fragment, func(t *testing.T) {
			s, e := edgeIndirectionFixture(t, PackageCheckLinkTarget, "opus", "1")
			e.Fragment = Evidence[string]{State: EvidenceKnown, Value: fragment}
			s.adoptions[e.Target.Value].resource.snapshot.header.Name = Evidence[QName]{State: EvidenceUnsupported, Reason: "inner-root-unavailable"}
			if err := s.observeEdge(context.Background(), e); err != nil {
				t.Fatal(err)
			}
			assertIndirectionReport(t, finishIndirectionFixture(t, s).PackageReport(), OutcomeUnknown, "nested-link-indirection-unresolved")
		})
	}
}

func TestPackageEdgeIndirectionRetainsDirectFragmentSemantics(t *testing.T) {
	for _, fragment := range []string{"", "missing", "xpointer(/1)"} {
		for _, indirect := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/indirect=%t", fragment, indirect), func(t *testing.T) {
				s, e := edgeIndirectionFixture(t, PackageCheckLinkTarget, "opus", "1")
				e.Indirection.Value = indirect
				e.Fragment = Evidence[string]{State: EvidenceKnown, Value: fragment}
				if err := s.observeEdge(context.Background(), e); err != nil {
					t.Fatal(err)
				}
				want, reason := OutcomePass, Code("linked-target-kind-established")
				switch fragment {
				case "missing":
					want, reason = OutcomeFail, "fragment-id-absent"
				case "xpointer(/1)":
					want, reason = OutcomeUnsupported, "fragment-grammar-unimplemented"
				}
				if indirect {
					want, reason = OutcomeUnknown, "nested-link-indirection-unresolved"
				}
				assertIndirectionReport(t, finishIndirectionFixture(t, s).PackageReport(), want, reason)
			})
		}
	}
}

func TestPackageEdgeIndirectionEvidenceStatesAndOwnedCause(t *testing.T) {
	cause := DecisionRef{Rule: RuleKey{ID: "transport-proof", Revision: 1}, Subject: NodeRef{"source", 2}}
	for _, tc := range []struct {
		name   string
		value  Evidence[bool]
		want   Outcome
		reason Code
	}{
		{"direct", Evidence[bool]{State: EvidenceKnown}, OutcomePass, "linked-target-kind-established"},
		{"nested", Evidence[bool]{State: EvidenceKnown, Value: true}, OutcomeUnknown, "nested-link-indirection-unresolved"},
		{"absent", Evidence[bool]{State: EvidenceAbsent}, OutcomeUnknown, "required-evidence-unavailable"},
		{"unknown", Evidence[bool]{State: EvidenceUnknown, Reason: "transport-unavailable"}, OutcomeUnknown, "transport-unavailable"},
		{"unsupported", Evidence[bool]{State: EvidenceUnsupported, Reason: "transport-unsupported"}, OutcomeUnsupported, "transport-unsupported"},
		{"invalid", Evidence[bool]{State: EvidenceInvalid, Reason: "transport-invalid", Cause: &cause}, OutcomeBlocked, "transport-invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := edgeIndirectionFixture(t, PackageCheckLinkTarget, "opus", "1")
			e.Indirection = stateClone(tc.value)
			if err := s.observeEdge(context.Background(), e); err != nil {
				t.Fatal(err)
			}
			if e.Indirection.Cause != nil {
				e.Indirection.Cause.Subject.Node = 99
			}
			got := s.edges[e.ID].Indirection
			if !reflect.DeepEqual(got, tc.value) {
				t.Fatalf("edge evidence was not owned: %+v", got)
			}
			b := finishIndirectionFixture(t, s)
			assertIndirectionReport(t, b.PackageReport(), tc.want, tc.reason)
			if tc.want == OutcomeBlocked {
				f := b.PackageReport().Findings[0]
				if f.Cause == nil || *f.Cause != cause {
					t.Fatalf("lost causal reference: %+v", f)
				}
			}
		})
	}
}

func TestPackageEdgeIndirectionRejectsMalformedEvidence(t *testing.T) {
	validCause := &DecisionRef{Rule: RuleKey{ID: "transport", Revision: 1}, Subject: NodeRef{"source", 2}}
	for name, value := range map[string]Evidence[bool]{
		"zero": {}, "invented-state": {State: "direct"},
		"known-cause":       {State: EvidenceKnown, Cause: validCause},
		"absent-value":      {State: EvidenceAbsent, Value: true},
		"unknown-no-reason": {State: EvidenceUnknown},
		"unknown-value":     {State: EvidenceUnknown, Value: true, Reason: "unavailable"},
		"unsupported-cause": {State: EvidenceUnsupported, Reason: "unavailable", Cause: validCause},
		"invalid-no-cause":  {State: EvidenceInvalid, Reason: "invalid"},
		"invalid-bad-cause": {State: EvidenceInvalid, Reason: "invalid", Cause: &DecisionRef{}},
	} {
		t.Run(name, func(t *testing.T) {
			s, e := edgeIndirectionFixture(t, PackageCheckLinkTarget, "opus", "1")
			e.Indirection = value
			if err := s.observeEdge(context.Background(), e); !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("malformed evidence admitted: %v", err)
			}
			if len(s.edges) != 0 {
				t.Fatal("malformed edge partially admitted")
			}
		})
	}
}

func TestPackageEdgeIndirectionPreservesPrerequisiteEvidence(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{ID: "local-href", Revision: 1}, Subject: NodeRef{"source", 2}}
	for _, tc := range []struct {
		name   string
		href   bool
		state  EvidenceState
		want   Outcome
		reason Code
	}{
		{"local-invalid", true, EvidenceInvalid, OutcomeBlocked, "local-invalid"},
		{"target-unknown", false, EvidenceUnknown, OutcomeUnknown, "target-unknown"},
		{"target-unsupported", false, EvidenceUnsupported, OutcomeUnsupported, "target-unsupported"},
		{"target-invalid", false, EvidenceInvalid, OutcomeBlocked, "target-invalid"},
		{"target-absent", false, EvidenceAbsent, OutcomeFail, "linked-resource-authoritatively-absent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := edgeIndirectionFixture(t, PackageCheckLinkTarget, "opus", "1")
			if tc.href {
				s.adoptions[e.Source].resource.snapshot.contexts[0].Resource.Link.Href.Value = Evidence[Atomic]{State: tc.state, Reason: tc.reason, Cause: cause}
			} else {
				e.Target = Evidence[ResourceAdoptionID]{State: tc.state, Reason: tc.reason}
				if tc.state == EvidenceInvalid {
					e.Target.Cause = cause
				}
			}
			if err := s.observeEdge(context.Background(), e); err != nil {
				t.Fatal(err)
			}
			assertIndirectionReport(t, finishIndirectionFixture(t, s).PackageReport(), tc.want, tc.reason)
		})
	}
}

func TestPackageEdgeIndirectionAdmissionHonorsCancellationAndRetentionBudget(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprintf("canceled=%t", canceled), func(t *testing.T) {
			s, e := edgeIndirectionFixture(t, PackageCheckLinkTarget, "opus", "1")
			used, _ := s.account.Usage(BudgetBufferedBytes)
			held := s.account.Limits().MaxBufferedBytes - used
			if err := s.account.Charge(BudgetBufferedBytes, held); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if canceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			err := s.observeEdge(ctx, e)
			if canceled {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation: %v", err)
				}
			} else {
				var budget *ResourceError
				if !errors.As(err, &budget) || budget.Kind != BudgetBufferedBytes {
					t.Fatalf("retention budget: %v", err)
				}
			}
			if len(s.edges) != 0 {
				t.Fatal("refused evidence partially admitted")
			}
			if err := s.account.Release(BudgetBufferedBytes, held); err != nil {
				t.Fatal(err)
			}
			_, _ = s.finish(context.Background(), PackageCompletion{})
			if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("leaked %d bytes", used)
			}
		})
	}
}

func TestPackageEdgeIndirectionRetainsInnerIntrinsicFailure(t *testing.T) {
	s, e := edgeIndirectionFixture(t, PackageCheckLinkTarget, "score-partwise", "not-a-number")
	if err := s.observeEdge(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	b := finishIndirectionFixture(t, s)
	assertIndirectionReport(t, b.PackageReport(), OutcomeUnknown, "nested-link-indirection-unresolved")
	_, inner, ok := b.ResourceReport(1)
	if !ok || inner.Counts.Fail == 0 || inner.Conformance != ConformanceViolated {
		t.Fatalf("inner failure hidden: %+v", inner)
	}
}

func TestPackageEdgeIndirectionGatesGroupTargetOnly(t *testing.T) {
	for _, mode := range []string{"match", "mismatch", "no-group", "no-group-incomplete", "local-invalid"} {
		for _, indirect := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/indirect=%t", mode, indirect), func(t *testing.T) {
				s, e := edgeIndirectionFixture(t, PackageCheckGroupMatch, "opus", "1")
				e.Indirection.Value = indirect
				source, target := s.adoptions[e.Source].resource.snapshot, s.adoptions[e.Target.Value].resource.snapshot
				if mode == "no-group-incomplete" {
					source.header.ScopeComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "source-incomplete"}
				}
				if mode != "no-group" && mode != "no-group-incomplete" {
					value := Evidence[string]{State: EvidenceKnown, Value: "group-one"}
					if mode == "local-invalid" {
						value = Evidence[string]{State: EvidenceInvalid, Reason: "local-group-invalid", Cause: &DecisionRef{Rule: RuleKey{ID: "local-group", Revision: 1}, Subject: NodeRef{"source", 3}}}
					}
					source.contexts = append(source.contexts, ContextResult{Subject: NodeRef{"source", 3}, Resource: &ResourceContextResult{Owner: NodeRef{"source", 2}, Group: &ResourceGroupContext{Linked: true, Value: value}}})
					other := "group-one"
					if mode == "mismatch" {
						other = "group-two"
					}
					target.contexts = []ContextResult{{Subject: NodeRef{"inner", 2}, Resource: &ResourceContextResult{Group: &ResourceGroupContext{Value: Evidence[string]{State: EvidenceKnown, Value: other}}}}}
				}
				if err := s.observeEdge(context.Background(), e); err != nil {
					t.Fatal(err)
				}
				want, reason := OutcomePass, Code("group-link-strings-match")
				if mode == "mismatch" {
					want, reason = OutcomeFail, "group-link-string-mismatch"
				}
				if indirect {
					want, reason = OutcomeUnknown, "nested-link-indirection-unresolved"
				}
				if mode == "no-group" {
					want, reason = OutcomeNotApplicable, "no-group-link-advice"
				}
				if mode == "no-group-incomplete" {
					want, reason = OutcomeUnknown, "group-scope-incomplete"
				}
				if mode == "local-invalid" {
					want, reason = OutcomeBlocked, "local-group-invalid"
				}
				if err := s.evaluateEdge(context.Background(), e, true); err != nil {
					t.Fatal(err)
				}
				key := packageDecisionKey{RuleKey{ID: RuleID(PackageCheckGroupMatch), Revision: 1}, e.ID}
				if got := s.decisions[key]; got.Outcome != want || got.Reason != reason {
					t.Fatalf("want %s/%s, got %+v", want, reason, got)
				}
				if mode == "no-group-incomplete" {
					// Incomplete intrinsic scope cannot be sealed as a complete
					// expansion; the already-recorded advice remains independently
					// checkable without inventing that completion witness.
					b, err := s.finish(context.Background(), PackageCompletion{})
					if err != nil || len(b.PackageReport().Findings) != 1 || b.PackageReport().Findings[0].Reason != reason {
						t.Fatalf("incomplete local scope: %v %+v", err, b.PackageReport())
					}
					return
				}
				b := finishIndirectionFixture(t, s)
				if mode == "no-group" && len(b.PackageReport().Findings) != 0 {
					t.Fatal("no group-link created group advice")
				}
			})
		}
	}
}

func TestPackageEdgeIndirectionGatesAssociationMembership(t *testing.T) {
	for _, indirect := range []bool{false, true} {
		for _, member := range []bool{false, true} {
			t.Run(fmt.Sprintf("indirect=%t/member=%t", indirect, member), func(t *testing.T) {
				s, q := packageAssociationFixture(t, Evidence[bool]{State: EvidenceKnown, Value: indirect})
				r := packageAssociationReply(q, Evidence[bool]{State: EvidenceKnown, Value: member})
				if err := s.observeAssociation(context.Background(), q, r); err != nil {
					t.Fatal(err)
				}
				if len(s.associations) != 1 || s.associations[0].result.Membership != r.Membership {
					t.Fatal("provider evidence discarded")
				}
				key := packageDecisionKey{RuleKey{ID: RuleID(PackageCheckInstrumentAssociation), Revision: 1}, s.associations[0].id}
				_, settled := s.decisions[key]
				if settled == indirect {
					t.Fatalf("immediate membership settled=%t, indirect=%t", settled, indirect)
				}
				want, reason := OutcomePass, Code("")
				if !member {
					want, reason = OutcomeFail, "instrument-not-in-linked-part"
				}
				if indirect {
					want, reason = OutcomeUnknown, "nested-link-indirection-unresolved"
				}
				assertIndirectionReport(t, finishIndirectionFixture(t, s).PackageReport(), want, reason)
			})
		}
	}
}

func TestPackageEdgeIndirectionAssociationUnavailableEvidence(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{ID: "transport", Revision: 1}, Subject: NodeRef{"source", 3}}
	for _, tc := range []struct {
		value  Evidence[bool]
		want   Outcome
		reason Code
	}{
		{Evidence[bool]{State: EvidenceAbsent}, OutcomeUnknown, "required-evidence-unavailable"},
		{Evidence[bool]{State: EvidenceUnknown, Reason: "transport-unknown"}, OutcomeUnknown, "transport-unknown"},
		{Evidence[bool]{State: EvidenceUnsupported, Reason: "transport-unsupported"}, OutcomeUnsupported, "transport-unsupported"},
		{Evidence[bool]{State: EvidenceInvalid, Reason: "transport-invalid", Cause: cause}, OutcomeBlocked, "transport-invalid"},
	} {
		t.Run(string(tc.value.State), func(t *testing.T) {
			original := stateClone(tc.value)
			s, q := packageAssociationFixture(t, tc.value)
			if err := s.observeAssociation(context.Background(), q, packageAssociationReply(q, Evidence[bool]{State: EvidenceKnown, Value: true})); err != nil {
				t.Fatal(err)
			}
			key := packageDecisionKey{RuleKey{ID: RuleID(PackageCheckInstrumentAssociation), Revision: 1}, s.associations[0].id}
			if _, settled := s.decisions[key]; settled {
				t.Fatal("unavailable transport evidence immediately certified membership")
			}
			if tc.value.Cause != nil {
				tc.value.Cause.Subject.Node = 99
			}
			r := finishIndirectionFixture(t, s).PackageReport()
			assertIndirectionReport(t, r, tc.want, tc.reason)
			if !reflect.DeepEqual(r.Findings[0].Cause, original.Cause) {
				t.Fatalf("association indirection cause was not retained independently: %+v", r.Findings[0])
			}
		})
	}
}

func TestPackageEdgeIndirectionWithoutAssociationProvider(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{ID: "local-proof", Revision: 1}, Subject: NodeRef{"source", 3}}
	for _, tc := range []struct {
		name        string
		indirection Evidence[bool]
		change      string
		want        Outcome
		reason      Code
	}{
		{"direct", Evidence[bool]{State: EvidenceKnown}, "", OutcomeUnknown, "instrument-association-evidence-unavailable"},
		{"nested", Evidence[bool]{State: EvidenceKnown, Value: true}, "", OutcomeUnknown, "nested-link-indirection-unresolved"},
		{"absent", Evidence[bool]{State: EvidenceAbsent}, "", OutcomeUnknown, "required-evidence-unavailable"},
		{"unknown", Evidence[bool]{State: EvidenceUnknown, Reason: "transport-unknown"}, "", OutcomeUnknown, "transport-unknown"},
		{"unsupported", Evidence[bool]{State: EvidenceUnsupported, Reason: "transport-unsupported"}, "", OutcomeUnsupported, "transport-unsupported"},
		{"invalid", Evidence[bool]{State: EvidenceInvalid, Reason: "transport-invalid", Cause: cause}, "", OutcomeBlocked, "transport-invalid"},
		{"local-instrument-invalid", Evidence[bool]{State: EvidenceKnown, Value: true}, "instrument", OutcomeBlocked, "local-instrument-invalid"},
		{"local-href-invalid", Evidence[bool]{State: EvidenceKnown, Value: true}, "href", OutcomeBlocked, "local-href-invalid"},
		{"unrelated-owner", Evidence[bool]{State: EvidenceKnown, Value: true}, "owner", OutcomeUnknown, "instrument-association-evidence-unavailable"},
		{"unrelated-owner-document", Evidence[bool]{State: EvidenceKnown, Value: true}, "document", OutcomeUnknown, "instrument-association-evidence-unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := packageAssociationFixture(t, tc.indirection)
			source := s.adoptions[s.edges[1].Source].resource.snapshot
			switch tc.change {
			case "instrument":
				source.contexts[1].Resource.Instrument.Target = Evidence[NodeRef]{State: EvidenceInvalid, Reason: tc.reason, Cause: cause}
			case "href":
				source.contexts[0].Resource.Link.Href.Value = Evidence[Atomic]{State: EvidenceInvalid, Reason: tc.reason, Cause: cause}
			case "owner":
				source.contexts[1].Resource.Owner.Node = 99
			case "document":
				source.contexts[1].Resource.Owner.DocumentID = "unrelated-source"
			}
			if len(s.associations) != 0 {
				t.Fatal("test unexpectedly supplied association provider evidence")
			}
			b := finishIndirectionFixture(t, s)
			assertIndirectionReport(t, b.PackageReport(), tc.want, tc.reason)
			if tc.want == OutcomeBlocked && !reflect.DeepEqual(b.PackageReport().Findings[0].Cause, cause) {
				t.Fatalf("lost prerequisite cause: %+v", b.PackageReport())
			}
		})
	}
}
