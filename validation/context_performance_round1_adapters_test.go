package validation_test

import (
	"github.com/go-muse/go-musicxml/validation"
	"testing"
)

func TestPerformanceRound1BendImplicationAdapters(t *testing.T) {
	for _, tc := range []struct {
		name, technical string
		satisfied       validation.EvidenceState
		value           bool
		outcome         validation.Outcome
	}{
		{"lone-negative", `<bend><bend-alter>-2</bend-alter><release/></bend>`, validation.EvidenceKnown, true, validation.OutcomePass},
		{"prebend-negative", `<bend><bend-alter>-2</bend-alter><pre-bend/></bend><bend><bend-alter>-2</bend-alter><release/></bend>`, validation.EvidenceKnown, true, validation.OutcomePass},
		{"lone-zero", `<bend><bend-alter>0</bend-alter><release/></bend>`, validation.EvidenceKnown, false, validation.OutcomeUnknown},
		{"proved-positive", `<bend><bend-alter>2</bend-alter></bend><bend><bend-alter>2</bend-alter><release/></bend>`, validation.EvidenceKnown, false, validation.OutcomeFail},
		{"nonpositive-first", `<bend><bend-alter>0</bend-alter></bend><bend><bend-alter>2</bend-alter><release/></bend>`, validation.EvidenceKnown, false, validation.OutcomeNotApplicable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repeatAssessBoth(t, `<attributes><divisions>4</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>2</duration><notations><technical>`+tc.technical+`</technical></notations></note>`, func(t *testing.T, r validation.Report) {
				var release *validation.TimingContextResult
				var subject validation.NodeRef
				for _, c := range r.Context {
					if c.Timing != nil && c.Timing.Kind == "bend" && c.Timing.BendRelease.Value {
						release = c.Timing
						subject = c.Subject
					}
				}
				if release == nil {
					t.Fatal("missing release fact")
				}
				if release.BendNegativeSatisfied.State != tc.satisfied || release.BendNegativeSatisfied.Value != tc.value {
					t.Errorf("consequent=%+v", release.BendNegativeSatisfied)
				}
				got := validation.OutcomePass
				for _, f := range r.Findings {
					if f.Rule.ID == "MX40-CONTEXT-bend-release-negative" && f.Subject == subject {
						got = f.Outcome
					}
				}
				// NotApplicable and Pass have no finding; the false antecedent distinguishes them.
				if tc.outcome == validation.OutcomeNotApplicable {
					if release.BendNegativeApplicable.State != validation.EvidenceKnown || release.BendNegativeApplicable.Value {
						t.Errorf("antecedent=%+v", release.BendNegativeApplicable)
					}
				} else if got != tc.outcome {
					t.Errorf("outcome=%s want %s", got, tc.outcome)
				}
			})
		})
	}
}
func TestPerformanceRound1AbsentEncodedTypeExcludesSwingAdapters(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>4</divisions></attributes><sound><swing><first>2</first><second>1</second></swing></sound><note><pitch><step>C</step><octave>4</octave></pitch><duration>2</duration></note>`, func(t *testing.T, r validation.Report) {
		for _, c := range r.Context {
			if c.Timing != nil && c.Timing.Kind == "note" {
				if c.Timing.SwingEligible.State != validation.EvidenceKnown || c.Timing.SwingEligible.Value || c.Timing.SwingRealized.State != validation.EvidenceKnown || c.Timing.SwingRealized.Value {
					t.Errorf("bounded missing-type exclusion=%+v/%+v", c.Timing.SwingEligible, c.Timing.SwingRealized)
				}
				return
			}
		}
		t.Fatal("missing note")
	})
}

func TestPerformanceRound1UnplacedEventRetainsTimelineCauseAdapters(t *testing.T) {
	repeatAssessHeader(t, `<attributes><divisions>4</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>2</duration><type>eighth</type></note>`, `<part-list><score-part id="P2"><part-name>Other</part-name></score-part></part-list>`, func(t *testing.T, r validation.Report) {
		for _, c := range r.Context {
			if c.Timing != nil && c.Timing.Kind == "note" {
				p := c.Timing.Position
				if p.State != validation.EvidenceInvalid || p.Reason != "identity-missing" || p.Cause == nil {
					t.Errorf("event=%+v", p)
				}
				if p.Cause != nil {
					matched := false
					for _, f := range r.Findings {
						if f.Rule == p.Cause.Rule && f.Subject == p.Cause.Subject && f.Outcome == validation.OutcomeFail {
							matched = true
						}
					}
					if !matched {
						t.Errorf("lost exact failing identity prerequisite: %+v", p.Cause)
					}
				}
				return
			}
		}
		t.Fatal("missing performance note")
	})
}
func TestPerformanceRound1SeparateTechnicalNegativeAdapters(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>4</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>2</duration><notations><technical><bend><bend-alter>2</bend-alter></bend></technical><technical><bend><bend-alter>-2</bend-alter><release/></bend></technical></notations></note>`, func(t *testing.T, r validation.Report) {
		var count int
		for _, c := range r.Context {
			if c.Timing != nil && c.Timing.Kind == "bend" && c.Timing.BendRelease.Value {
				count++
				if c.Timing.BendNegativeApplicable.State != validation.EvidenceUnknown || c.Timing.BendNegativeSatisfied.State != validation.EvidenceKnown || !c.Timing.BendNegativeSatisfied.Value {
					t.Errorf("cross-parent relation=%+v/%+v", c.Timing.BendNegativeApplicable, c.Timing.BendNegativeSatisfied)
				}
			}
		}
		if count != 1 {
			t.Fatalf("release count=%d", count)
		}
		for _, f := range r.Findings {
			if f.Rule.ID == "MX40-CONTEXT-bend-release-negative" {
				t.Errorf("proved negative must satisfy implication: %+v", f)
			}
		}
	})
}
