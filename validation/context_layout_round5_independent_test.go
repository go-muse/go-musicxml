package validation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func layoutRound5IndependentPublisher(t *testing.T, budget Budgets) (*layoutScopes, *factNode) {
	t.Helper()
	s := rationalSession(t, budget)
	t.Cleanup(s.cleanup)
	s.documentID = "score"
	n := localNode(7, 0, "measure")
	return &layoutScopes{r: &contextReader{ctx: context.Background(), s: s}, key: RuleKey{"layout", 1}}, n
}

func TestLayoutRound5IndependentOrdinaryAndExplicitPublication(t *testing.T) {
	for _, rawState := range []EvidenceState{EvidenceAbsent, EvidenceKnown, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		for _, valueState := range []EvidenceState{EvidenceKnown, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
			for _, explicit := range []bool{false, true} {
				t.Run(fmt.Sprintf("raw=%s/value=%s/explicit=%t", rawState, valueState, explicit), func(t *testing.T) {
					l, n := layoutRound5IndependentPublisher(t, Budgets{})
					control, _ := layoutRound5IndependentPublisher(t, Budgets{})
					cause := &DecisionRef{Rule: RuleKey{"fixture", 2}, Subject: NodeRef{"score", 19}}
					raw := Evidence[Atomic]{State: rawState}
					if rawState != EvidenceAbsent {
						raw.Reason, raw.Cause = "raw-fixture", cause
					}
					value := Evidence[Atomic]{State: valueState, Reason: "value-fixture", Cause: cause}
					active := Evidence[bool]{State: EvidenceUnknown, Reason: "active-fixture", Cause: cause}
					part := Evidence[NodeRef]{State: EvidenceUnsupported, Reason: "part-fixture", Cause: cause}
					frame := layoutFrame{
						page:   Evidence[NodeRef]{State: EvidenceInvalid, Reason: "page-fixture", Cause: cause},
						system: Evidence[NodeRef]{State: EvidenceUnknown, Reason: "system-fixture", Cause: cause},
					}
					provenance := ContextExplicit
					if !explicit && rawState == EvidenceAbsent {
						provenance = ContextInherited
						if valueState == EvidenceUnknown {
							provenance = ContextApplicationChoice
						}
					}
					scope := NodeRef{"score", 3}
					origin := localNode(9, 0, "page-layout")
					want := ContextResult{Provider: l.key, Subject: NodeRef{"score", 7}, Property: "layout.width-outcome", Value: value,
						Layout: &LayoutContextResult{Staff: Evidence[Integer]{State: EvidenceAbsent}, Conversion: Evidence[Rational]{State: EvidenceAbsent}, Part: part, Scope: scope, Origin: NodeRef{"score", 9}, Raw: raw, Applicability: active, Unit: ContextUnitTenths, Page: frame.page, System: frame.system, Provenance: provenance}}
					if err := control.r.s.publishContextResult(control.r.ctx, want); err != nil {
						t.Fatal(err)
					}
					var err error
					if explicit {
						err = l.publishWithProvenance(n, "width-outcome", raw, value, active, scope, frame, origin, part, ContextExplicit)
					} else {
						err = l.publish(n, "width-outcome", raw, value, active, scope, frame, origin, part)
					}
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(l.r.s.contextResults, control.r.s.contextResults) {
						t.Fatalf("publication changed fields: got=%+v want=%+v", l.r.s.contextResults, control.r.s.contextResults)
					}
					for _, kind := range []BudgetKind{BudgetContextFacts, BudgetRuleSteps, BudgetBufferedBytes} {
						if got, want := rationalUsage(t, l.r.s, kind), rationalUsage(t, control.r.s, kind); got != want {
							t.Errorf("%s accounting=%d want=%d", kind, got, want)
						}
					}
					row := l.r.s.contextResults[0]
					causes := []*DecisionRef{row.Value.Cause, row.Layout.Part.Cause, row.Layout.Page.Cause, row.Layout.System.Cause, row.Layout.Applicability.Cause}
					if rawState != EvidenceAbsent {
						causes = append(causes, row.Layout.Raw.Cause)
					}
					for i, got := range causes {
						if got == nil || got == cause || *got != *cause {
							t.Fatalf("cause %d not independently owned: %+v", i, got)
						}
						for _, other := range causes[:i] {
							if got == other {
								t.Fatal("editable payload causes alias each other")
							}
						}
					}
					cause.Rule.ID = "caller-mutation"
					row.Value.Cause.Rule.ID = "projection-mutation"
					for _, got := range causes[1:] {
						if got.Rule.ID != "fixture" {
							t.Error("mutating one cause contaminated independent evidence")
						}
					}
				})
			}
		}
	}
}

func TestLayoutRound5IndependentOrdinaryWidthOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		width, sum Evidence[Rational]
		complete   bool
		state      EvidenceState
		reason     Code
	}{
		{"consistent", Evidence[Rational]{State: EvidenceKnown, Value: mustRational(t, 80, 1)}, Evidence[Rational]{State: EvidenceKnown, Value: mustRational(t, 80, 1)}, true, EvidenceKnown, ""},
		{"mismatch", Evidence[Rational]{State: EvidenceKnown, Value: mustRational(t, 80, 1)}, Evidence[Rational]{State: EvidenceKnown, Value: mustRational(t, 81, 1)}, true, EvidenceUnknown, "system-width-disagreement-rendering-unspecified"},
		{"incomplete", Evidence[Rational]{State: EvidenceKnown, Value: mustRational(t, 80, 1)}, Evidence[Rational]{State: EvidenceKnown, Value: mustRational(t, 80, 1)}, false, EvidenceUnknown, "system-width-prerequisite-unavailable"},
		{"width-unavailable", Evidence[Rational]{State: EvidenceUnsupported}, Evidence[Rational]{State: EvidenceKnown, Value: mustRational(t, 80, 1)}, true, EvidenceUnknown, "system-width-prerequisite-unavailable"},
		{"sum-unavailable", Evidence[Rational]{State: EvidenceKnown, Value: mustRational(t, 80, 1)}, Evidence[Rational]{State: EvidenceInvalid}, true, EvidenceUnknown, "system-width-prerequisite-unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, n := layoutRound5IndependentPublisher(t, Budgets{})
			l.r.s.sequence = []NodeID{n.header.ID}
			frame := layoutFrame{page: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 1}}, system: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 7}}}
			l.widths = map[NodeID]layoutSystemWidth{n.header.ID: {Node: n, Frame: frame, Width: tc.width, Sum: tc.sum, Complete: tc.complete, Part: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 2}}}}
			if err := l.publishWidths(); err != nil {
				t.Fatal(err)
			}
			if len(l.r.s.contextResults) != 1 {
				t.Fatalf("width rows=%d", len(l.r.s.contextResults))
			}
			row := l.r.s.contextResults[0]
			provenance := ContextApplicationChoice
			if tc.state == EvidenceKnown {
				provenance = ContextInherited
				if row.Value.Value.Text != "consistent" {
					t.Errorf("lost consistency result: %+v", row.Value)
				}
			}
			if row.Value.State != tc.state || row.Value.Reason != tc.reason || row.Layout.Provenance != provenance || row.Layout.Raw.State != EvidenceAbsent || row.Layout.Origin != row.Subject {
				t.Errorf("ordinary width semantics changed: value=%+v layout=%+v", row.Value, row.Layout)
			}
		})
	}
}

func layoutRound5IndependentPublishConflict(l *layoutScopes, n *factNode) error {
	cause := &DecisionRef{Rule: RuleKey{"fixture", 1}, Subject: NodeRef{"score", 19}}
	gap := Evidence[Atomic]{State: EvidenceInvalid, Reason: "encoded-break-unavailable", Cause: cause}
	frame := layoutFrame{page: Evidence[NodeRef]{State: EvidenceInvalid, Reason: gap.Reason, Cause: cause}, system: Evidence[NodeRef]{State: EvidenceInvalid, Reason: gap.Reason, Cause: cause}}
	return l.publishWithProvenance(n, "width-outcome", Evidence[Atomic]{State: EvidenceAbsent}, gap, Evidence[bool]{State: EvidenceKnown, Value: true}, NodeRef{"score", n.header.ID}, frame, nil, Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"score", 2}}, ContextExplicit)
}

func TestLayoutRound5IndependentExplicitAdmissionAndCancellation(t *testing.T) {
	baseline, node := layoutRound5IndependentPublisher(t, Budgets{})
	counter := &rationalCancelContext{Context: context.Background(), at: int(^uint(0) >> 1)}
	baseline.r.ctx = counter
	if err := layoutRound5IndependentPublishConflict(baseline, node); err != nil {
		t.Fatal(err)
	}
	bytes := rationalUsage(t, baseline.r.s, BudgetBufferedBytes)
	steps := rationalUsage(t, baseline.r.s, BudgetRuleSteps)
	for _, limit := range []int64{1, bytes - 1, bytes} {
		t.Run(fmt.Sprintf("bytes=%d", limit), func(t *testing.T) {
			l, n := layoutRound5IndependentPublisher(t, Budgets{MaxBufferedBytes: limit})
			err := layoutRound5IndependentPublishConflict(l, n)
			if limit == bytes {
				if err != nil || len(l.r.s.contextResults) != 1 {
					t.Fatalf("exact admission failed: %v", err)
				}
			} else if !errors.Is(err, ErrResourceLimit) || len(l.r.s.contextResults) != 0 || l.r.s.retained != internCharge(l.r.s) || rationalUsage(t, l.r.s, BudgetBufferedBytes) != internCharge(l.r.s) {
				t.Fatalf("failed admission leaked: err=%v rows=%d bytes=%d", err, len(l.r.s.contextResults), l.r.s.retained)
			}
		})
	}
	for at := 1; at <= counter.calls; at++ {
		t.Run(fmt.Sprintf("cancel-poll=%d", at), func(t *testing.T) {
			l, n := layoutRound5IndependentPublisher(t, Budgets{})
			l.r.ctx = &rationalCancelContext{Context: context.Background(), at: at}
			if err := layoutRound5IndependentPublishConflict(l, n); !errors.Is(err, context.Canceled) || len(l.r.s.contextResults) != 0 || l.r.s.retained != internCharge(l.r.s) || rationalUsage(t, l.r.s, BudgetBufferedBytes) != internCharge(l.r.s) {
				t.Fatalf("canceled publication leaked: err=%v rows=%d bytes=%d", err, len(l.r.s.contextResults), l.r.s.retained)
			}
		})
	}
	for _, sticky := range []error{context.Canceled, ErrResourceLimit} {
		l, n := layoutRound5IndependentPublisher(t, Budgets{})
		l.r.err = sticky
		if err := layoutRound5IndependentPublishConflict(l, n); err != sticky || len(l.r.s.contextResults) != 0 || l.r.s.retained != internCharge(l.r.s) || rationalUsage(t, l.r.s, BudgetContextFacts) != 0 || rationalUsage(t, l.r.s, BudgetRuleSteps) != 0 {
			t.Fatalf("sticky error changed or charged work: %v", err)
		}
	}
	for _, budget := range []Budgets{{MaxContextFacts: 1}, {MaxRuleSteps: steps}} {
		l, n := layoutRound5IndependentPublisher(t, budget)
		if err := layoutRound5IndependentPublishConflict(l, n); err != nil {
			t.Fatal(err)
		}
		before := l.r.s.retained
		if err := layoutRound5IndependentPublishConflict(l, n); !errors.Is(err, ErrResourceLimit) || len(l.r.s.contextResults) != 1 || l.r.s.retained != before || rationalUsage(t, l.r.s, BudgetContextFacts) != 1 {
			t.Fatalf("second publication escaped quota or damaged first: err=%v rows=%d bytes=%d", err, len(l.r.s.contextResults), l.r.s.retained)
		}
	}
}
