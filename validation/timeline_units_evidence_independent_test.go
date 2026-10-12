package validation

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestIndependentTimelinePrimitiveUnitsParityAndOwnership(t *testing.T) {
	for _, mode := range []string{"absent", "known", "equal-tie", "conflict", "invalid-tie", "gap", "closure"} {
		t.Run(mode, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			h := &timelineState{}
			at := statePosition(t, 0, 1, 3)
			cause := &DecisionRef{Rule: RuleKey{"units-upstream", 1}, Subject: NodeRef{"d", 77}}
			if mode == "closure" {
				if err := h.closePart(context.Background(), s, at.Part, 0, Evidence[bool]{State: EvidenceInvalid, Cause: cause, Reason: "bad-closure"}); err != nil {
					t.Fatal(err)
				}
			} else {
				stateClose(t, s, h, 0)
			}
			if mode != "absent" && mode != "closure" {
				p := statePublication[Decimal]{Position: at, Scope: stateScope{AllStaves: true}, Value: stateKnown(stateDecimal(t, 2, 1))}
				if err := publishState(context.Background(), s, &h.Divisions, p); err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "equal-tie", "conflict", "invalid-tie":
					if mode == "conflict" {
						p.Value = stateKnown(stateDecimal(t, 3, 1))
					}
					if mode == "invalid-tie" {
						p.Value = Evidence[Decimal]{State: EvidenceInvalid, Cause: cause, Reason: "bad-units"}
					}
					if err := publishState(context.Background(), s, &h.Divisions, p); err != nil {
						t.Fatal(err)
					}
				case "gap":
					if err := addStateGap(context.Background(), s, &h.Divisions, stateGap{Part: at.Part, FirstMeasure: 0, LastMeasure: 0, Scope: stateScope{AllStaves: true}, Availability: Evidence[bool]{State: EvidenceInvalid, Cause: cause, Reason: "bad-gap"}}); err != nil {
						t.Fatal(err)
					}
				}
			}
			full, err := h.effectiveDivisions(context.Background(), s, at)
			if err != nil {
				t.Fatal(err)
			}
			before := rationalUsage(t, s, BudgetBufferedBytes)
			got, err := h.effectiveDivisionsEvidence(context.Background(), s, at)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, full.Value) {
				t.Fatalf("primitive %+v differs from full %+v", got, full.Value)
			}
			delta := rationalUsage(t, s, BudgetBufferedBytes) - before
			if got.Cause == nil && delta != 0 {
				t.Fatalf("cause-free warmed primitive query retained%d bytes", delta)
			}
			if got.Cause != nil {
				if delta != 128 || got.Cause == full.Value.Cause || got.Cause == cause {
					t.Fatalf("cause was not independently admitted/owned: delta%d", delta)
				}
				got.Cause.Subject.Node = 999
				again, err := h.effectiveDivisionsEvidence(context.Background(), s, at)
				if err != nil || again.Cause == nil || again.Cause.Subject.Node != 77 || full.Value.Cause.Subject.Node != 77 {
					t.Fatalf("primitive cause leaked mutation: %+v %v", again, err)
				}
			}
		})
	}
}

func TestIndependentTimelinePrimitiveModeRejectsCompoundEscape(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	at := statePosition(t, 0, 0, 1)
	_, err := queryStateAtMode(context.Background(), s, h, &h.Keys, at, stateKnown(stateInt(t, 1)), stateWhole[keyState], true, false)
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("borrowed compound mode accepted: %v", err)
	}
}
