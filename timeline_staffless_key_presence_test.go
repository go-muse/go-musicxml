package musicxml

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Omission of harmony/staff does not obscure a key that applies independently
// of that choice. The same source is assessed through both public adapters.
func TestTimelineStafflessKeyPresenceBothAdapters(t *testing.T) {
	const allKey = `<key><fifths>0</fifths></key>`
	const oneKey = `<key number="1"><fifths>0</fifths></key>`
	for _, form := range []string{"partwise", "timewise"} {
		for _, tc := range []struct {
			name, keys, staves, staff, local, after string
			want                                    validation.Outcome
			count                                   int
		}{
			{name: "all-staves", keys: allKey},
			{name: "all-staves-explicit-one", keys: allKey, staff: `<staff>1</staff>`},
			{name: "all-staves-explicit-two", keys: allKey, staff: `<staff>2</staff>`},
			{name: "all-staves-large-count", keys: allKey, staves: "18446744073709551615"},
			{name: "all-staves-conflicting-content", keys: allKey, after: `<attributes><key><fifths>3</fifths></key></attributes>`},
			{name: "all-staves-with-staff-specific-content", keys: allKey, after: `<attributes><key number="2"><fifths>3</fifths></key></attributes>`},
			{name: "staff-specific-ambiguous", keys: oneKey, want: validation.OutcomeUnknown, count: 1},
			{name: "staff-specific-explicit-match", keys: oneKey, staff: `<staff>1</staff>`},
			{name: "staff-specific-explicit-missing", keys: oneKey, staff: `<staff>2</staff>`, want: validation.OutcomeFail, count: 1},
			{name: "local-key-independent", keys: oneKey, local: semanticQALocalKey},
			{name: "no-key-any-staff", want: validation.OutcomeFail, count: 1},
		} {
			t.Run(form+"/"+tc.name, func(t *testing.T) {
				staves := tc.staves
				if staves == "" {
					staves = "2"
				}
				body := `<attributes><divisions>4</divisions>` + tc.keys + `<staves>` + staves + `</staves></attributes>` + tc.after + timelineNumeral(tc.local, tc.staff)
				assessTimelineBoth(t, semanticQAScore(form, [][2]string{{body, ""}}), func(t *testing.T, report validation.Report) {
					semanticQAExpect(t, report, "MX40-TIMELINE-numeral-requires-key", tc.want, tc.count)
					if tc.count == 0 && !report.Satisfied() {
						t.Fatalf("staff-independent key rejected: %s", localFindings(report))
					}
					for _, finding := range report.Findings {
						if strings.HasPrefix(string(finding.Rule.ID), "MX40-XSD-") && finding.Outcome == validation.OutcomeFail {
							t.Fatalf("unintended schema failure: %s", localFindings(report))
						}
					}
					if tc.want == validation.OutcomeUnknown && report.AssessmentComplete {
						t.Fatal("staff-dependent result incorrectly completed")
					}
				})
			})
		}
	}
}

func TestTimelineStafflessKeyPresenceUnplacedAlternativesBothAdapters(t *testing.T) {
	for _, form := range []string{"partwise", "timewise"} {
		for _, scope := range []string{"", ` number="2"`} {
			for _, baseline := range []bool{false, true} {
				for _, local := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/scope=%s/baseline=%t/local=%t", form, scope, baseline, local), func(t *testing.T) {
						key, own := "", ""
						if baseline {
							key = `<key><fifths>0</fifths></key>`
						}
						if local {
							own = semanticQALocalKey
						}
						first := `<attributes><divisions>1</divisions>` + key + `<staves>2</staves></attributes>` + timelineNote("1", "", "")
						second := timelineNote("2", "", "") + timelineNumeral(own, "") + `<attributes><divisions>2</divisions></attributes><backup><duration>8</duration></backup><attributes><key` + scope + `><fifths>3</fifths></key></attributes>`
						assessTimelineBoth(t, semanticQAScore(form, [][2]string{{first, ""}, {second, ""}}), func(t *testing.T, report validation.Report) {
							if baseline || local {
								semanticQAExpect(t, report, "MX40-TIMELINE-numeral-requires-key", "", 0)
								if !report.Satisfied() {
									t.Fatalf("staff/position-independent key lost: %s", localFindings(report))
								}
							} else {
								semanticQAExpect(t, report, "MX40-TIMELINE-numeral-requires-key", validation.OutcomeUnsupported, 1)
								if report.AssessmentComplete || report.Conformance == validation.ConformanceViolated {
									t.Fatalf("missing baseline became complete: %s", localFindings(report))
								}
							}
						})
					})
				}
			}
		}
	}
}

func TestTimelineStafflessKeyPresenceSchemaControls(t *testing.T) {
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
		body := `<attributes><divisions>4</divisions><key><fifths>0</fifths></key><staves>18446744073709551615</staves></attributes>` + timelineNumeral("", "")
		assertXMLLintOutcome(t, xmllint, filepath.Join(schema, "musicxml.xsd"), filepath.Join(schema, "catalog.xml"), semanticQAScore(form, [][2]string{{body, ""}}), true)
	}
}
