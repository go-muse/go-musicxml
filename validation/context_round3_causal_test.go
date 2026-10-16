package validation

import (
	"context"
	"reflect"
	"testing"
)

func TestContextRound3BendImplicationEvidenceMatrix(t *testing.T) {
	antecedentCause := &DecisionRef{Rule: RuleKey{"gesture", 1}, Subject: NodeRef{"d", 1}}
	consequentCause := &DecisionRef{Rule: RuleKey{"bend-alter", 1}, Subject: NodeRef{"d", 3}}
	type axis struct {
		name  string
		proof Evidence[bool]
		gap   Decision
	}
	axes := func(cause *DecisionRef) []axis {
		return []axis{
			{"true", stateKnown(true), Decision{}},
			{"false", stateKnown(false), Decision{}},
			{"absent", stateAbsent[bool](), Decision{Outcome: OutcomeUnknown, Reason: "required-value-absent"}},
			{"unknown", Evidence[bool]{State: EvidenceUnknown, Reason: "input-unknown", Cause: cause}, Decision{Outcome: OutcomeUnknown, Reason: "input-unknown", Cause: cause}},
			{"unsupported", Evidence[bool]{State: EvidenceUnsupported, Reason: "input-unsupported", Cause: cause}, Decision{Outcome: OutcomeUnsupported, Reason: "input-unsupported", Cause: cause}},
			{"invalid", Evidence[bool]{State: EvidenceInvalid, Reason: "input-invalid", Cause: cause}, Decision{Outcome: OutcomeBlocked, Reason: "input-invalid", Cause: cause}},
		}
	}
	for _, app := range axes(antecedentCause) {
		for _, satisfied := range axes(consequentCause) {
			t.Run(app.name+"/"+satisfied.name, func(t *testing.T) {
				s := rationalSession(t, Budgets{})
				defer s.cleanup()
				key := RuleKey{"performance", 1}
				n := localNode(2, 0, "bend")
				s.contextServices = map[RuleKey]*contextService{key: {Final: true, Performance: &contextPerformanceService{Bends: map[NodeID]performanceBendProof{2: {Applicable: app.proof, Satisfied: satisfied.proof}}}}}
				want := Decision{Outcome: OutcomeFail, Reason: "bend-release-must-be-negative", Expected: "negative semitone displacement for an established release after a bend", Actual: "nonnegative bend-alter"}
				switch {
				case app.name == "false":
					want = Decision{Outcome: OutcomeNotApplicable, Reason: "bend-release-condition-false"}
				case satisfied.name == "true":
					want = Decision{Outcome: OutcomePass}
				case satisfied.name == "invalid":
					want = satisfied.gap
				case app.proof.State != EvidenceKnown:
					want = app.gap
				case satisfied.proof.State != EvidenceKnown:
					want = satisfied.gap
				}
				got, pending, err := evaluateBendReleaseSign(context.Background(), s, n, &contextPlan{Provider: key}, true)
				if err != nil || pending || !reflect.DeepEqual(got, want) {
					t.Errorf("decision=%+v pending=%v err=%v; want %+v", got, pending, err, want)
				}
			})
		}
	}
}

func TestContextRound3HarmonyAdvisoryParentEvidence(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{"parent-content", 1}, Subject: NodeRef{"doc", 1}}
	for _, tc := range []struct {
		name   string
		parent *factNode
		want   Evidence[bool]
	}{
		{"missing", nil, stateUnknown[bool]("harmony-parent-unavailable")},
		{"open", &factNode{header: testRetainedHeader(NodeHeader{}), end: testRetainedEnd(NodeEnd{})}, stateUnknown[bool]("timeline-node-open")},
		{"unknown", &factNode{closed: true, end: testRetainedEnd(NodeEnd{ChildrenComplete: Evidence[bool]{State: EvidenceUnknown, Reason: "parent-content-unknown", Cause: cause}}), header: testRetainedHeader(NodeHeader{})}, Evidence[bool]{State: EvidenceUnknown, Reason: "parent-content-unknown", Cause: cause}},
		{"invalid", &factNode{closed: true, end: testRetainedEnd(NodeEnd{ChildrenComplete: Evidence[bool]{State: EvidenceInvalid, Reason: "parent-content-invalid", Cause: cause}}), header: testRetainedHeader(NodeHeader{})}, Evidence[bool]{State: EvidenceInvalid, Reason: "parent-content-invalid", Cause: cause}},
		{"unsupported", &factNode{closed: true, end: testRetainedEnd(NodeEnd{ChildrenComplete: Evidence[bool]{State: EvidenceUnsupported, Reason: "parent-content-unsupported", Cause: cause}}), header: testRetainedHeader(NodeHeader{})}, Evidence[bool]{State: EvidenceUnsupported, Reason: "parent-content-unsupported", Cause: cause}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, n, r := localEvidenceFixture(t, EvidenceKnown, localKind)
			defer s.cleanup()
			n.end.Scalar.Value = testEvidence(localKnownText("none"))
			if tc.parent != nil {
				s.nodes[n.header.Parent] = tc.parent
			}
			if err := localAdvisoryNode(r.ctx, s, RuleKey{"local-test", 1}, n, r); err != nil {
				t.Fatal(err)
			}
			found := 0
			for _, row := range s.contextResults {
				if row.Property == "local.advisory-harmony-none-root" {
					found++
					if !reflect.DeepEqual(row.Layout.Applicability, tc.want) {
						t.Errorf("applicability=%+v; want %+v", row.Layout.Applicability, tc.want)
					}
				}
			}
			if found != 1 {
				t.Fatalf("advisory rows=%d", found)
			}
		})
	}
}
