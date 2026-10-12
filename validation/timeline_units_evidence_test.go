package validation

import (
	"context"
	"testing"
)

func TestTimelineDivisionsValueQueryMatchesOwnedQueryWithoutUnusedProvenance(t *testing.T) {
	for _, kind := range []string{"known", "absent", "conflict", "invalid", "gap"} {
		t.Run(kind, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			h := &timelineState{}
			stateClose(t, s, h, 1)
			ctx := context.Background()
			at := statePosition(t, 0, 0, 1)
			cause := &DecisionRef{Rule: RuleKey{"bad-units", 1}, Subject: NodeRef{"d", 9}}
			if kind != "absent" {
				value := stateKnown(stateDecimal(t, 3, 1))
				if kind == "invalid" {
					value = Evidence[Decimal]{State: EvidenceInvalid, Cause: cause}
				}
				statePublish(t, s, &h.Divisions, at, stateScope{AllStaves: true}, value, 9)
			}
			if kind == "conflict" {
				statePublish(t, s, &h.Divisions, at, stateScope{AllStaves: true}, stateKnown(stateDecimal(t, 4, 1)), 10)
			}
			if kind == "gap" {
				if err := addStateGap(ctx, s, &h.Divisions, stateGap{Part: at.Part, FirstMeasure: 0, LastMeasure: 0, Scope: stateScope{AllStaves: true}, Availability: stateUnknown[bool]("missing-publisher")}); err != nil {
					t.Fatal(err)
				}
			}
			expected, err := h.effectiveDivisions(ctx, s, at)
			if err != nil {
				t.Fatal(err)
			}
			before := rationalUsage(t, s, BudgetBufferedBytes)
			got, err := h.effectiveDivisionsEvidence(ctx, s, at)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != expected.Value.State || got.Reason != expected.Value.Reason || got.Value.String() != expected.Value.Value.String() {
				t.Fatalf("got=%+v expected=%+v", got, expected)
			}
			if expected.Value.Cause != nil {
				if got.Cause == nil || *got.Cause != *expected.Value.Cause || got.Cause == expected.Value.Cause {
					t.Fatalf("causal ownership=%+v %+v", got, expected)
				}
			}
			if got.Cause == nil && rationalUsage(t, s, BudgetBufferedBytes) != before {
				t.Fatalf("primitive query retained unused provenance: before=%d after=%d", before, rationalUsage(t, s, BudgetBufferedBytes))
			}
		})
	}
}
