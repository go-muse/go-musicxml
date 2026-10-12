package validation

import (
	"context"
	"testing"
)

// Filtering by exact part may skip unrelated notes, but it may not reorder the
// first equally ranked cause from unknown-part versus selected-part evidence.
func TestIndependentContextRepeatContentScopeCauseOrder(t *testing.T) {
	part := NodeRef{"d", 10}
	unknownCause := &DecisionRef{Rule: RuleKey{ID: "unknown-part", Revision: 1}, Subject: NodeRef{"d", 1}}
	staffCause := &DecisionRef{Rule: RuleKey{ID: "selected-staff", Revision: 1}, Subject: NodeRef{"d", 2}}
	unknown := contextRepeatNote{source: NodeRef{"d", 1}, part: Evidence[NodeRef]{State: EvidenceInvalid, Reason: "first-or-last-unknown-part", Cause: unknownCause}}
	selected := contextRepeatNote{source: NodeRef{"d", 2}, part: stateKnown(part), staff: Evidence[Integer]{State: EvidenceInvalid, Reason: "first-or-last-selected-staff", Cause: staffCause}}
	unrelated := contextRepeatNote{source: NodeRef{"d", 3}, part: stateKnown(NodeRef{"d", 20}), staff: Evidence[Integer]{State: EvidenceInvalid, Reason: "unrelated-must-not-win", Cause: &DecisionRef{Rule: RuleKey{ID: "unrelated"}}}}
	for _, unknownFirst := range []bool{true, false} {
		s := rationalSession(t, Budgets{})
		notes := []contextRepeatNote{unrelated, selected, unknown}
		want := staffCause
		if unknownFirst {
			notes = []contextRepeatNote{unknown, unrelated, selected}
			want = unknownCause
		}
		row := RepeatContextResult{Part: stateKnown(part), AllStaves: stateKnown(false), Staff: stateKnown(localInteger(t, 1).Integer)}
		roster, freeRoster, err := contextRepeatBuildContentRoster(context.Background(), s, notes)
		if err != nil {
			t.Fatal(err)
		}
		local, freePart, err := roster.partNotes(context.Background(), s, part)
		if err != nil {
			t.Fatal(err)
		}
		_, status, release, err := contextRepeatScopedNotes(context.Background(), s, local, row)
		if err != nil {
			t.Fatal(err)
		}
		release()
		freePart()
		freeRoster()
		s.cleanup()
		if status.State != EvidenceInvalid || status.Cause == nil || *status.Cause != *want {
			t.Fatalf("unknownFirst=%t causal order changed: %+v", unknownFirst, status)
		}
	}
}
