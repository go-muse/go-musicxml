package validation_test

import (
	"reflect"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestPerformanceRound3UnestablishedBendInvalidConsequentAdapters(t *testing.T) {
	// NaN survives the model's float decoder but is invalid in the XSD decimal
	// value space, so both adapters independently retain a causal scalar defect.
	repeatAssessBoth(t, `<attributes><divisions>4</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>2</duration><notations><technical><bend><bend-alter>NaN</bend-alter><release/></bend></technical></notations></note>`, func(t *testing.T, r validation.Report) {
		var release *validation.TimingContextResult
		var subject validation.NodeRef
		for _, row := range r.Context {
			if row.Timing != nil && row.Timing.Kind == "bend" && row.Timing.BendRelease.Value {
				release, subject = row.Timing, row.Subject
			}
		}
		if release == nil {
			t.Fatal("missing release proof")
		}
		if release.BendNegativeApplicable.State != validation.EvidenceUnknown || release.BendNegativeSatisfied.State != validation.EvidenceInvalid || release.BendNegativeSatisfied.Cause == nil {
			t.Fatalf("unexpected proof: applicable=%+v satisfied=%+v", release.BendNegativeApplicable, release.BendNegativeSatisfied)
		}
		found := false
		for _, finding := range r.Findings {
			if finding.Rule.ID == "MX40-CONTEXT-bend-release-negative" && finding.Subject == subject {
				found = true
				if finding.Outcome != validation.OutcomeBlocked || finding.Reason != release.BendNegativeSatisfied.Reason || !reflect.DeepEqual(finding.Cause, release.BendNegativeSatisfied.Cause) {
					t.Errorf("consumer lost consequent cause: %+v", finding)
				}
			}
		}
		if !found {
			t.Error("missing causal consumer decision")
		}
	})
}
