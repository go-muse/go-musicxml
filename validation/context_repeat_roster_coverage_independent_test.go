package validation

import (
	"context"
	"testing"
)

func TestIndependentContextRepeatRosterPreservesCoverageCause(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	cause := &DecisionRef{Rule: RuleKey{ID: "content-coverage", Revision: 1}, Subject: NodeRef{"d", 1}}
	coverage := Evidence[bool]{State: EvidenceInvalid, Reason: "coverage-invalid", Cause: cause}
	row := contextRepeatRecord{value: RepeatContextResult{Kind: RepeatStyleMeasure, Type: stateKnown("start"), Part: stateUnknown[NodeRef]("part-unavailable")}}
	direct := row
	if err := contextRepeatProveRow(context.Background(), s, &timelineService{}, nil, nil, &direct, coverage); err != nil {
		t.Fatal(err)
	}
	if direct.value.ContentEquivalent.State != EvidenceInvalid || direct.value.ContentEquivalent.Cause != cause {
		t.Fatal("invalid baseline fixture")
	}
	if err := contextRepeatProveRosterRow(context.Background(), s, &timelineService{}, &contextRepeatContentRoster{}, nil, &row, coverage); err != nil {
		t.Fatal(err)
	}
	if row.value.ContentEquivalent.State != EvidenceInvalid || row.value.ContentEquivalent.Cause == nil || *row.value.ContentEquivalent.Cause != *cause {
		t.Fatalf("roster wrapper downgraded coverage cause: %+v", row.value.ContentEquivalent)
	}
}
