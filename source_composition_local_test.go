package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourceDeclarationCompositionEffectiveLocalPresence(t *testing.T) {
	for _, tc := range []struct {
		name, declarations, body, rule string
		violated                       bool
	}{
		{"default-dacapo-no", `<!ATTLIST sound dacapo (yes|no) "no">`, `<sound/>`, "MX40-LOCAL-sound-dacapo-yes", true},
		{"default-dacapo-yes", `<!ATTLIST sound dacapo (yes|no) "yes">`, `<sound/>`, "MX40-LOCAL-sound-dacapo-yes", false},
		{"explicit-suppresses-dacapo-default", `<!ATTLIST sound dacapo (yes|no) "no">`, `<sound dacapo="yes"/>`, "MX40-LOCAL-sound-dacapo-yes", false},
		{"default-forward-repeat-no", `<!ATTLIST sound forward-repeat (yes|no) "no">`, `<sound/>`, "MX40-LOCAL-sound-forward-repeat-yes", true},
		{"default-second-control-start", `<!ATTLIST slur bezier-x2 CDATA "0">`, localNote(`<notations><slur type="start"/></notations>`), "MX40-LOCAL-bezier-second-control/slur", true},
		{"default-second-control-continue", `<!ATTLIST slur bezier-x2 CDATA "0">`, localNote(`<notations><slur type="continue"/></notations>`), "MX40-LOCAL-bezier-second-control/slur", false},
		{"entity-generated-sound", `<!ENTITY nav '<sound dacapo="no"/>'>`, `&nav;`, "MX40-LOCAL-sound-dacapo-yes", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := "<!DOCTYPE score-partwise [" + tc.declarations + "]>" + (localFixture{body: tc.body}).xml()
			report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTDLocal})
			if err != nil || !report.InputComplete || report.Syntax != validation.SyntaxWellFormed {
				t.Fatalf("error=%v syntax=%s inputComplete=%t findings=%s", err, report.Syntax, report.InputComplete, localFindings(report))
			}
			found := false
			for _, finding := range report.Findings {
				if finding.Outcome == validation.OutcomeFail && strings.HasPrefix(string(finding.Rule.ID), "MX40-XSD-") {
					t.Fatalf("unintended XSD failure: %s", localFindings(report))
				}
				if string(finding.Rule.ID) == tc.rule && finding.Outcome == validation.OutcomeFail {
					if finding.Rule.Revision != 2 {
						t.Fatalf("effective-source rule did not receive a semantic revision: %+v", finding.Rule)
					}
					found = true
				}
			}
			if found != tc.violated || report.Satisfied() == tc.violated {
				t.Fatalf("violated=%t found=%t satisfied=%t findings=%s", tc.violated, found, report.Satisfied(), localFindings(report))
			}
		})
	}
}
