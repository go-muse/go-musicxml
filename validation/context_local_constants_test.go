package validation

import "testing"

func TestLocalConstantTableNormativeDefaults(t *testing.T) {
	for _, tt := range []struct{ property, want string }{{"score-version", "1.0"}, {"winged", "none"}, {"horizontal-turn-slash", "no"}, {"multiple-rest-use-symbols", "no"}, {"first-fret", "1"}, {"notehead-parentheses", "no"}, {"strong-accent-type", "up"}, {"empty-caesura", "normal"}, {"fermata-shape", "normal"}, {"fermata-orientation", "upright"}} {
		found := false
		for _, r := range localConstantRules {
			if r.Property == tt.property {
				found = true
				if r.Default != tt.want {
					t.Errorf("%s default=%q want %q", tt.property, r.Default, tt.want)
				}
			}
		}
		if !found {
			t.Errorf("missing normative default %s", tt.property)
		}
	}
}

func TestEveryLocalConstantPreservesMissingInvalidAndExplicitEvidence(t *testing.T) {
	for _, rule := range localConstantRules {
		t.Run(rule.Property, func(t *testing.T) {
			for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
				raw := Evidence[Atomic]{State: state, Reason: "fixture-missing", Cause: &DecisionRef{Rule: RuleKey{"fixture-invalid", 1}, Subject: NodeRef{"score", 8}}}
				got := localConstantEffective(rule, raw)
				if got.State != state || got.Cause == nil || got.Cause.Rule.ID != "fixture-invalid" {
					t.Fatalf("state %s replaced: %+v", state, got)
				}
			}
			raw := Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "explicit-distinct"}}
			got := localConstantEffective(rule, raw)
			if got.Value.Text != raw.Value.Text || got.State != EvidenceKnown {
				t.Fatal("explicit field overwritten")
			}
			got = localConstantEffective(rule, Evidence[Atomic]{State: EvidenceAbsent})
			if got.State != EvidenceKnown {
				t.Fatalf("proven absence failed: %+v", got)
			}
		})
	}
}
