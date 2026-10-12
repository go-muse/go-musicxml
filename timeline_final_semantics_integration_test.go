package musicxml

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// These are production InspectXML/AssessModel regressions. Both targets use
// ordinary integers; the model is decoded from the same bytes, without claiming
// exact decimal/binary equality for arbitrary source values.
func finalReviewDisplacedKeyScore(form string, unitGap, localKey bool) string {
	key := ""
	if localKey {
		key = semanticQALocalKey
	}
	first := semanticQADivisions + timelineNote("1", "", "") + timelineNumeral(key, "") + timelineNote("1", "", "")
	second := `<backup><duration>2</duration></backup>`
	if unitGap {
		second = timelineNote("2", "", "") + `<attributes><divisions>2</divisions></attributes><backup><duration>8</duration></backup>`
	}
	second += `<attributes><key><fifths>0</fifths></key></attributes>`
	return semanticQAScore(form, [][2]string{{first, ""}, {second, ""}})
}

func TestTimelineFinalReviewDisplacedKeyMandatoryConsumerBothAdapters(t *testing.T) {
	for _, form := range []string{"partwise", "timewise"} {
		for _, tc := range []struct {
			name              string
			unitGap, localKey bool
			want              validation.Outcome
		}{
			{"known-floating-key", false, false, validation.OutcomeUnknown},
			{"unit-gated-key", true, false, validation.OutcomeUnsupported},
			{"own-key-independent", true, true, ""},
		} {
			t.Run(form+"/"+tc.name, func(t *testing.T) {
				assessTimelineBoth(t, finalReviewDisplacedKeyScore(form, tc.unitGap, tc.localKey), func(t *testing.T, report validation.Report) {
					for _, finding := range report.Findings {
						if strings.HasPrefix(string(finding.Rule.ID), "MX40-XSD-") && finding.Outcome == validation.OutcomeFail {
							t.Fatalf("unintended schema failure: %s", localFindings(report))
						}
					}
					count := 1
					if tc.localKey {
						count = 0
					}
					semanticQAExpect(t, report, "MX40-TIMELINE-numeral-requires-key", tc.want, count)
					if tc.localKey {
						if !report.Satisfied() {
							t.Fatalf("own local analysis key lost independence: %s", localFindings(report))
						}
						return
					}
					if report.AssessmentComplete || report.Conformance == validation.ConformanceViolated {
						t.Fatalf("unresolved key applicability fabricated a complete rejection: %s", localFindings(report))
					}
				})
			})
		}
	}
}

func TestTimelineFinalReviewDisplacedKeySchemaControls(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint required for fixture controls")
		}
		t.Skip("xmllint is not installed")
	}
	schema, err := filepath.Abs(filepath.Join("schema", "musicxml-4.0"))
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range []string{"partwise", "timewise"} {
		t.Run(form, func(t *testing.T) {
			assertXMLLintOutcome(t, xmllint, filepath.Join(schema, "musicxml.xsd"), filepath.Join(schema, "catalog.xml"), finalReviewDisplacedKeyScore(form, true, false), true)
		})
	}
}
