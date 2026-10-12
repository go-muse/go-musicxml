package validation

import (
	"context"
	"testing"
)

func TestLocalFontGenericCategoriesPreservePreferenceOrder(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	raw := localKnownText("music, engraved, handwritten, text, serif, Custom Font, fantasy")
	got, err := localFontCategories(context.Background(), s, raw)
	if err != nil || got.State != EvidenceKnown || got.Value.Text != "music,music,music,text,text,specific,text" {
		t.Fatalf("%+v %v", got, err)
	}
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		raw.State = state
		raw.Cause = &DecisionRef{Rule: RuleKey{"font-prerequisite", 1}, Subject: NodeRef{"score", 1}}
		got, err = localFontCategories(context.Background(), s, raw)
		if err != nil || got.State != state || got.Cause == nil || got.Cause.Rule.ID != "font-prerequisite" {
			t.Fatalf("%+v %v", got, err)
		}
	}
}
