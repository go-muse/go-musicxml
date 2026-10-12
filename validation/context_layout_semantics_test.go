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

// Every static layout rule is exercised independently against assessed
// evidence. These target-neutral transformations are not adapter fixtures.
func TestLayoutEveryConstantFiveAxes(t *testing.T) {
	for _, rule := range layoutConstantRules {
		t.Run(rule.Property, func(t *testing.T) {
			t.Run("positive", func(t *testing.T) {
				raw := Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "explicit"}}
				got := localConstantEffective(rule, raw)
				if got.State != EvidenceKnown || got.Value.Text != "explicit" {
					t.Fatalf("explicit %+v", got)
				}
			})
			t.Run("negative", func(t *testing.T) {
				got := localConstantEffective(rule, Evidence[Atomic]{State: EvidenceAbsent})
				want := EvidenceKnown
				if rule.Unknown != "" {
					want = EvidenceUnknown
				}
				if got.State != want {
					t.Fatalf("absence %+v", got)
				}
			})
			t.Run("boundary", func(t *testing.T) {
				got := localConstantEffective(rule, Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: ""}})
				want := EvidenceKnown
				if rule.Empty && rule.Unknown != "" {
					want = EvidenceUnknown
				}
				if got.State != want {
					t.Fatalf("empty %+v", got)
				}
			})
			t.Run("missing_fact", func(t *testing.T) {
				got := localConstantEffective(rule, Evidence[Atomic]{State: EvidenceUnknown, Reason: "field-unavailable"})
				if got.State != EvidenceUnknown || got.Reason != "field-unavailable" {
					t.Fatalf("missing %+v", got)
				}
			})
			t.Run("interaction", func(t *testing.T) {
				cause := &DecisionRef{Rule: RuleKey{"xsd-field", 1}, Subject: NodeRef{"score", 1}}
				got := localConstantEffective(rule, Evidence[Atomic]{State: EvidenceInvalid, Reason: "xsd", Cause: cause})
				if got.State != EvidenceInvalid || got.Cause != cause {
					t.Fatalf("invalid %+v", got)
				}
			})
		})
	}
}
