package validation

import "testing"

func TestLocalAdvisoryKnownFalseAndUnavailableStaySeparate(t *testing.T) {
	if got := localAdvisoryCondition(localKnownText("none"), "none"); got.State != EvidenceKnown || !got.Value {
		t.Fatal(got)
	}
	if got := localAdvisoryCondition(localKnownText("G"), "none"); got.State != EvidenceKnown || got.Value {
		t.Fatal(got)
	}
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		raw := Evidence[Atomic]{State: state, Reason: "missing", Cause: &DecisionRef{Rule: RuleKey{"advisory-input", 1}, Subject: NodeRef{"score", 1}}}
		got := localAdvisoryCondition(raw, "none")
		if got.State != state || got.Cause == nil {
			t.Fatal(got)
		}
	}
}

func TestLocalSemanticLabelsRequireValidPrerequisites(t *testing.T) {
	for _, label := range []string{"octave-four-begins-at-middle-c", "one-is-bottom-line-unbounded-by-line-count", "unlisted-strings-muted"} {
		for _, state := range []EvidenceState{EvidenceAbsent, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
			raw := Evidence[Atomic]{State: state, Reason: "source-unavailable", Cause: &DecisionRef{Rule: RuleKey{"fixture-source", 1}, Subject: NodeRef{"score", 1}}}
			got := localSemanticLabel(raw, label)
			if got.State != state || got.Cause != raw.Cause {
				t.Fatalf("%s %+v", label, got)
			}
		}
	}
}
