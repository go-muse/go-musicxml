package validation

import "testing"

func TestLayoutSelectorExactPartialAndUnknown(t *testing.T) {
	absent := Evidence[Atomic]{State: EvidenceAbsent}
	known := func(s string) Evidence[Atomic] {
		return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: s}}
	}
	for _, tc := range []struct {
		name           string
		pn, pv, ln, lv Evidence[Atomic]
		want           EvidenceState
		match          bool
	}{
		{"unrestricted", absent, absent, absent, absent, EvidenceKnown, true},
		{"exact-name", known("verse"), absent, known("verse"), known("01"), EvidenceKnown, true},
		{"number-not-numeric", absent, known("1"), known("verse"), known("01"), EvidenceKnown, false},
		{"partial-conjunction", known("verse"), known("2"), known("verse"), known("1"), EvidenceKnown, false},
		{"unknown-publisher", Evidence[Atomic]{State: EvidenceUnknown}, absent, known("verse"), absent, EvidenceUnknown, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := layoutSelectorMatch(tc.pn, tc.pv, tc.ln, tc.lv)
			if got.State != tc.want || got.Value != tc.match {
				t.Fatalf("got %+v", got)
			}
		})
	}
}
func TestLayoutComponentConsensusFiveAxes(t *testing.T) {
	a := Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "A"}}
	b := Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "B"}}
	unknown := Evidence[Atomic]{State: EvidenceUnknown, Reason: "unobserved"}
	absent := Evidence[Atomic]{State: EvidenceAbsent}
	for _, tc := range []struct {
		name   string
		values []Evidence[Atomic]
		closed bool
		state  EvidenceState
		text   string
	}{
		{"positive", []Evidence[Atomic]{a, a}, true, EvidenceKnown, "A"},
		{"negative", []Evidence[Atomic]{a, b}, true, EvidenceUnknown, ""},
		{"boundary", []Evidence[Atomic]{absent, a}, true, EvidenceKnown, "A"},
		{"missing-fact", []Evidence[Atomic]{a, unknown}, true, EvidenceUnknown, ""},
		{"interaction", []Evidence[Atomic]{a}, false, EvidenceUnknown, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := layoutConsensus(tc.values, tc.closed)
			if got.State != tc.state || got.Value.Text != tc.text {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

// Every static layout rule is exercised independently. These tests concern
// already assessed evidence; adapter fixtures separately prove exact use sites.
func TestLayoutEveryConstantFiveAxes(t *testing.T) {
	for _, rule := range layoutConstantRules {
		t.Run(rule.Property, func(t *testing.T) {
			explicit := Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "explicit"}}
			for _, target := range []Target{TargetSource, TargetModel} {
				for _, hierarchy := range []string{"partwise", "timewise"} {
					t.Run(string(target)+"-"+hierarchy, func(t *testing.T) {
						t.Run("positive", func(t *testing.T) {
							v := localConstantEffective(rule, explicit)
							if v.State != EvidenceKnown || v.Value.Text != "explicit" {
								t.Fatalf("explicit %+v", v)
							}
						})
						t.Run("negative", func(t *testing.T) {
							v := localConstantEffective(rule, Evidence[Atomic]{State: EvidenceAbsent})
							if rule.Unknown != "" {
								if v.State != EvidenceUnknown {
									t.Fatalf("invented application default %+v", v)
								}
							} else if v.State != EvidenceKnown {
								t.Fatalf("lost known default %+v", v)
							}
						})
						t.Run("boundary", func(t *testing.T) {
							v := localConstantEffective(rule, Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: ""}})
							if !rule.Empty && v.State != EvidenceKnown {
								t.Fatalf("empty explicit %+v", v)
							}
							if rule.Empty && rule.Unknown != "" && v.State != EvidenceUnknown {
								t.Fatalf("empty symbol %+v", v)
							}
						})
						t.Run("missing_fact", func(t *testing.T) {
							v := localConstantEffective(rule, Evidence[Atomic]{State: EvidenceUnknown, Reason: "adapter-field-unavailable"})
							if v.State != EvidenceUnknown || v.Reason != "adapter-field-unavailable" {
								t.Fatalf("unknown became default %+v", v)
							}
						})
						t.Run("interaction", func(t *testing.T) {
							cause := &DecisionRef{Rule: RuleKey{"xsd-field", 1}, Subject: NodeRef{"score", 1}}
							v := localConstantEffective(rule, Evidence[Atomic]{State: EvidenceInvalid, Reason: "xsd", Cause: cause})
							if v.State != EvidenceInvalid || v.Cause != cause {
								t.Fatalf("invalid became default %+v", v)
							}
						})
					})
				}
			}
		})
	}
}
