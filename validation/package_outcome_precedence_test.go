package validation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func packageHintPrecedenceFixture(t *testing.T, values ...Evidence[bool]) (*PackageSession, PackageRuleBinding) {
	t.Helper()
	intrinsic := snapshotTestSession(t, "1")
	_, snapshot, err := intrinsic.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	rules, profiles := packageFixture(t, PackageCheckSchemaHintPairs)
	rules[0].Rule.Class, rules[0].Rule.Role, rules[0].Rule.Category = ClassPolicy, RolePredicate, CategoryCapability
	h := snapshot.Header()
	profiles[0].Intrinsic = []IntrinsicProfileBinding{{Assembly: h.Assembly.Value, Target: h.Target, Profile: h.Profile, CatalogDigest: h.CatalogDigest}}
	catalog, err := NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	s, err := catalog.Begin(context.Background(), RunOptions{DocumentID: "package", Profile: "test.package", Account: intrinsic.account})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OpenPackage(context.Background(), packageTestIdentity()); err != nil {
		t.Fatal(err)
	}
	resource, err := s.AddResource(context.Background(), testResourcePin("source"), snapshot, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdoptResource(context.Background(), resource, ResourceAdoptionOptions{Package: "outer", DocumentID: "source", Root: true}); err != nil {
		t.Fatal(err)
	}
	snapshot.contexts = nil
	for i, value := range values {
		snapshot.contexts = append(snapshot.contexts, ContextResult{
			Provider: RuleKey{ID: "resource-schema-hints", Revision: 1},
			Subject:  NodeRef{"source", NodeID(i + 2)},
			Resource: &ResourceContextResult{SchemaHints: &ResourceSchemaHintsContext{
				Raw: Evidence[string]{State: EvidenceKnown, Value: "observed hint"}, Complete: value,
			}},
		})
	}
	t.Cleanup(func() {
		if !s.finished {
			_, _ = s.Finish(context.Background(), PackageCompletion{})
		}
	})
	return s, rules[0]
}

func TestPackageSchemaHintConclusiveFailurePrecedence(t *testing.T) {
	invalidCause := &DecisionRef{Rule: RuleKey{ID: "hint-datatype", Revision: 1}, Subject: NodeRef{"source", 90}}
	odd := Evidence[bool]{State: EvidenceKnown, Reason: "schema-location-unmatched-token"}
	even := Evidence[bool]{State: EvidenceKnown, Value: true}
	for _, missing := range []Evidence[bool]{
		{State: EvidenceAbsent},
		{State: EvidenceUnknown, Reason: "hint-unknown"},
		{State: EvidenceUnsupported, Reason: "hint-unsupported"},
		{State: EvidenceInvalid, Reason: "hint-invalid", Cause: invalidCause},
	} {
		for _, order := range []string{"missing-odd", "odd-missing", "missing-even", "even-missing", "missing-only"} {
			t.Run(string(missing.State)+"/"+order, func(t *testing.T) {
				values := []Evidence[bool]{missing}
				oddNode := NodeID(0)
				switch order {
				case "missing-odd":
					values, oddNode = append(values, odd), 3
				case "odd-missing":
					values, oddNode = []Evidence[bool]{odd, missing}, 2
				case "missing-even":
					values = append(values, even)
				case "even-missing":
					values = []Evidence[bool]{even, missing}
				}
				s, _ := packageHintPrecedenceFixture(t, values...)
				r := finishIndirectionFixture(t, s).PackageReport()
				want, reason, cause := OutcomeUnknown, Code("schema-discovery-pairing-unavailable"), missing.Cause
				if missing.State == EvidenceUnsupported {
					want = OutcomeUnsupported
				} else if missing.State == EvidenceInvalid {
					want = OutcomeBlocked
				}
				if oddNode != 0 {
					want, reason = OutcomeFail, odd.Reason
					cause = &DecisionRef{Rule: RuleKey{ID: "resource-schema-hints", Revision: 1}, Subject: NodeRef{"source", oddNode}}
				}
				if len(r.Findings) != 1 || r.Findings[0].Outcome != want || r.Findings[0].Reason != reason || !reflect.DeepEqual(r.Findings[0].Cause, cause) {
					t.Fatalf("want %s/%s cause=%+v, got %+v", want, reason, cause, r.Findings)
				}
				if r.Counts.Fail != 0 || r.Conformance == ConformanceViolated || r.BlockingProblems != 1 || r.AssessmentComplete != (oddNode != 0) {
					t.Errorf("discovery policy changed normative/completion axes: %+v", r)
				}
			})
		}
	}
}

func TestPackageSchemaHintUnavailableFallbackKeepsFirstCause(t *testing.T) {
	for _, firstInvalid := range []bool{false, true} {
		t.Run(fmt.Sprintf("first-invalid=%t", firstInvalid), func(t *testing.T) {
			cause := &DecisionRef{Rule: RuleKey{ID: "hint-datatype", Revision: 1}, Subject: NodeRef{"source", 7}}
			first, second := Evidence[bool]{State: EvidenceUnknown, Reason: "hint-unknown"}, Evidence[bool]{State: EvidenceInvalid, Reason: "hint-invalid", Cause: cause}
			want := OutcomeUnknown
			if firstInvalid {
				first, second, want = second, first, OutcomeBlocked
			}
			s, _ := packageHintPrecedenceFixture(t, first, second, Evidence[bool]{State: EvidenceKnown, Value: true})
			r := finishIndirectionFixture(t, s).PackageReport()
			if len(r.Findings) != 1 || r.Findings[0].Outcome != want || r.Findings[0].Reason != "schema-discovery-pairing-unavailable" || !reflect.DeepEqual(r.Findings[0].Cause, first.Cause) {
				t.Fatalf("first unavailable decision changed: %+v", r.Findings)
			}
		})
	}
}

func TestPackageSchemaHintContinuedScanHonorsInterruption(t *testing.T) {
	for _, mode := range []string{"cancel", "budget"} {
		t.Run(mode, func(t *testing.T) {
			values := make([]Evidence[bool], 32)
			for i := range values {
				values[i] = Evidence[bool]{State: EvidenceUnknown, Reason: "hint-unknown"}
			}
			values[len(values)-1] = Evidence[bool]{State: EvidenceKnown, Reason: "schema-location-unmatched-token"}
			s, rule := packageHintPrecedenceFixture(t, values...)
			var ctx context.Context = &rationalCancelContext{Context: context.Background(), at: 8}
			want := error(context.Canceled)
			if mode == "budget" {
				ctx, want = context.Background(), ErrResourceLimit
				used, _ := s.account.Usage(BudgetRuleSteps)
				if err := s.account.Charge(BudgetRuleSteps, s.account.Limits().MaxRuleSteps-used-8); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.evaluateResourceRule(ctx, rule); !errors.Is(err, want) {
				t.Fatalf("continued scan ignored interruption: %v", err)
			}
			b, err := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
			if !errors.Is(err, want) || b.Satisfied() || b.EnumerationComplete() || b.PackageReport().AssessmentComplete || len(b.PackageReport().Findings) != 0 {
				t.Errorf("interrupted scan published a premature fallback or success: %v %+v", err, b.PackageReport())
			}
			if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
				t.Errorf("retained %d bytes after interrupted Finish", used)
			}
		})
	}
}

func TestPackageNoProviderAssociationDirectTargetPrecedence(t *testing.T) {
	targetCause := &DecisionRef{Rule: RuleKey{ID: "target-proof", Revision: 1}, Subject: NodeRef{"source", 3}}
	for _, target := range []Evidence[ResourceAdoptionID]{
		{State: EvidenceAbsent},
		{State: EvidenceUnknown, Reason: "resolver-not-selected"},
		{State: EvidenceUnsupported, Reason: "target-unsupported"},
		{State: EvidenceInvalid, Reason: "target-invalid", Cause: targetCause},
	} {
		for _, indirection := range []Evidence[bool]{{State: EvidenceKnown}, {State: EvidenceKnown, Value: true}, {State: EvidenceAbsent}, {State: EvidenceUnknown, Reason: "transport-unknown"}} {
			for _, local := range []string{"valid", "instrument", "href"} {
				t.Run(fmt.Sprintf("%s/%s/%t/%s", target.State, indirection.State, indirection.Value, local), func(t *testing.T) {
					s, _ := packageAssociationFixture(t, indirection)
					edge := s.edges[1]
					edge.Target = target
					s.edges[1] = edge
					snapshot := s.adoptions[edge.Source].resource.snapshot
					want := packageRequiredEvidenceDecision(Decision{}, target)
					if indirection.State == EvidenceKnown && !indirection.Value {
						want = Decision{Outcome: OutcomeUnknown, Reason: "instrument-association-evidence-unavailable"}
					}
					if local != "valid" {
						cause := &DecisionRef{Rule: RuleKey{ID: "local-" + RuleID(local), Revision: 1}, Subject: NodeRef{"source", 4}}
						want = Decision{Outcome: OutcomeBlocked, Reason: Code("local-" + local + "-invalid"), Cause: cause}
						if local == "instrument" {
							snapshot.contexts[1].Resource.Instrument.Target = Evidence[NodeRef]{State: EvidenceInvalid, Reason: want.Reason, Cause: cause}
						} else {
							snapshot.contexts[0].Resource.Link.Href.Value = Evidence[Atomic]{State: EvidenceInvalid, Reason: want.Reason, Cause: cause}
						}
					}
					r := finishIndirectionFixture(t, s).PackageReport()
					if len(r.Findings) != 1 || r.Findings[0].Outcome != want.Outcome || r.Findings[0].Reason != want.Reason || !reflect.DeepEqual(r.Findings[0].Cause, want.Cause) {
						t.Fatalf("no-provider association precedence changed: want %+v got %+v", want, r.Findings)
					}
				})
			}
		}
	}
}
