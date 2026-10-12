package validation

import (
	"context"
	"reflect"
	"testing"
)

func TestIndependentContextVoiceFilterPreservesLoopEvidence(t *testing.T) {
	causeA := &DecisionRef{Rule: RuleKey{ID: "a", Revision: 1}, Subject: NodeRef{"d", 1}}
	causeB := &DecisionRef{Rule: RuleKey{ID: "b", Revision: 1}, Subject: NodeRef{"d", 2}}
	variants := []Evidence[string]{stateKnown("x"), stateKnown("y"), stateAbsent[string](), {State: EvidenceUnknown, Reason: "unknown-a", Cause: causeA}, {State: EvidenceInvalid, Reason: "invalid-a", Cause: causeA}, {State: EvidenceInvalid, Reason: "invalid-b", Cause: causeB}, {State: EvidenceUnsupported, Reason: "unsupported-b", Cause: causeB}}
	s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34})
	defer s.cleanup()
	checks := 0
	check := func(values []Evidence[string]) {
		filter, release, err := contextRepeatBuildVoiceFilter(context.Background(), s, values)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		for _, voice := range variants {
			want := stateKnown(true)
			for _, except := range values {
				if except.State == EvidenceKnown && voice.State == EvidenceKnown && except.Value == voice.Value {
					want = stateKnown(false)
					break
				}
				if want.State == EvidenceKnown && !want.Value {
					continue
				}
				if except.State != EvidenceKnown {
					want = stateMergeGap(want, stateUnavailable[string, bool](except))
				} else if voice.State != EvidenceKnown {
					gap := stateUnavailable[string, bool](voice)
					if gap.State == EvidenceAbsent {
						gap = stateUnknown[bool]("context-repeat-note-voice-unavailable")
					}
					want = stateMergeGap(want, gap)
				}
			}
			got, err := contextRepeatVoiceApplies(context.Background(), s, filter, voice)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("voice=%+v values=%+v: got=%+v want=%+v err=%v", voice, values, got, want, err)
			}
			checks++
		}
	}
	check(nil)
	for _, a := range variants {
		check([]Evidence[string]{a})
		for _, b := range variants {
			check([]Evidence[string]{a, b})
			for _, c := range variants {
				check([]Evidence[string]{a, b, c})
			}
		}
	}
	if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
		t.Fatalf("filter scratch retained: %d", got)
	}
	t.Logf("%d independent original-loop evidence comparisons", checks)
}
