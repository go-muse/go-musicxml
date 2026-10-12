package validation

import "testing"

func TestLayoutStateConditionalSemantics(t *testing.T) {
	k := func(v bool) Evidence[bool] { return Evidence[bool]{State: EvidenceKnown, Value: v} }
	for _, c := range []struct {
		object, spacing bool
		want            bool
	}{{true, true, false}, {true, false, false}, {false, true, true}, {false, false, false}} {
		got := layoutStateCutaway(k(c.object), k(c.spacing))
		if got.State != EvidenceKnown || got.Value != c.want {
			t.Fatalf("cutaway %+v: %+v", c, got)
		}
	}
	unknown := Evidence[bool]{State: EvidenceUnknown, Reason: "missing"}
	if got := layoutStateCutaway(k(true), unknown); got.State != EvidenceKnown || got.Value {
		t.Fatal(got)
	}
	if got := layoutStateCutaway(k(false), unknown); got.State != EvidenceUnknown || got.Reason != "missing" {
		t.Fatal(got)
	}
	for _, c := range []struct {
		size  string
		grace bool
		want  string
	}{{"full", true, ""}, {"cue", true, "grace"}, {"cue", false, "cue"}, {"grace-cue", false, "grace-cue"}, {"large", true, "large"}} {
		got := layoutStateNoteCategory(Evidence[string]{State: EvidenceKnown, Value: c.size}, k(c.grace))
		if c.want == "" {
			if got.State != EvidenceAbsent {
				t.Fatal(got)
			}
		} else if got.State != EvidenceKnown || got.Value != c.want {
			t.Fatalf("%+v %+v", c, got)
		}
	}
	if got := layoutStateNoteCategory(Evidence[string]{State: EvidenceKnown, Value: "cue"}, unknown); got.State != EvidenceUnknown {
		t.Fatal(got)
	}
}

func TestLayoutStateMissingAndBlockedComponents(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		cause := &DecisionRef{Rule: RuleKey{"fixture", 1}, Subject: NodeRef{"score", 3}}
		missing := Evidence[bool]{State: state, Reason: "missing-component", Cause: cause}
		got := layoutStateCutaway(Evidence[bool]{State: EvidenceKnown, Value: false}, missing)
		if got.State != state || got.Cause != cause {
			t.Fatal(got)
		}
		size := layoutStateNoteCategory(Evidence[string]{State: EvidenceKnown, Value: "cue"}, missing)
		if size.State != state || size.Cause != cause {
			t.Fatal(size)
		}
		fixed := layoutStateNoteCategory(Evidence[string]{State: EvidenceKnown, Value: "large"}, missing)
		if fixed.State != EvidenceKnown || fixed.Value != "large" {
			t.Fatal(fixed)
		}
	}
}
