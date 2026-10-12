package validation

import (
	"reflect"
	"testing"
)

// This audits the provider's raw/effective distinction. Whether a hidden staff
// actually reserves vertical space remains a graphical-layout consumer query.
func TestTimelineStaffPrintDefaultsRetainRawAndCausalEvidence(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{ID: "staff-print", Revision: 1}, Subject: NodeRef{"d", 31}}
	for _, tc := range []struct {
		name string
		raw  Evidence[bool]
	}{
		{"absent", stateAbsent[bool]()},
		{"yes", stateKnown(true)},
		{"no", stateKnown(false)},
		{"unknown", stateUnknown[bool]("staff-print-unavailable")},
		{"invalid", Evidence[bool]{State: EvidenceInvalid, Reason: "staff-print-invalid", Cause: cause}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			staff := staffDetailsState{PrintObject: tc.raw, PrintSpacing: tc.raw}
			line := lineDetailState{PrintObject: tc.raw}
			for label, got := range map[string]stateDefault[bool]{
				"staff-object":  staff.effectivePrintObject(),
				"staff-spacing": staff.effectivePrintSpacing(),
				"line-object":   line.effectivePrintObject(),
			} {
				if !reflect.DeepEqual(got.Raw, tc.raw) {
					t.Fatalf("%s changed raw evidence: %+v", label, got)
				}
				if tc.raw.State == EvidenceAbsent {
					if got.Effective.State != EvidenceKnown || !got.Effective.Value || got.Provenance != stateDocumentedDefault || got.Clause == "" {
						t.Fatalf("%s lost documented yes default: %+v", label, got)
					}
				} else if !reflect.DeepEqual(got.Effective, tc.raw) || got.Provenance == stateDocumentedDefault {
					t.Fatalf("%s defaulted present/unavailable evidence: %+v", label, got)
				}
			}
		})
	}
	cutaway := staffDetailsState{PrintObject: stateKnown(false), PrintSpacing: stateKnown(true)}
	if cutaway.effectivePrintObject().Effective.Value || !cutaway.effectivePrintSpacing().Effective.Value {
		t.Fatal("hidden staff erased independent explicit spacing statement")
	}
}
