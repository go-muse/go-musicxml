package musicxml

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func timelineNote(duration, prefix, tail string) string {
	return `<note>` + prefix + `<pitch><step>C</step><octave>4</octave></pitch><duration>` + duration + `</duration>` + tail + `</note>`
}

func timelineNumeral(localKey, tail string) string {
	return `<harmony><numeral><numeral-root>1</numeral-root>` + localKey + `</numeral><kind>major</kind>` + tail + `</harmony>`
}

func assessTimelineBoth(t *testing.T, source string, check func(*testing.T, validation.Report)) {
	t.Helper()
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatalf("fixture decode: %v", err)
	}
	for _, target := range []validation.Target{validation.TargetSource, validation.TargetModel} {
		t.Run(string(target), func(t *testing.T) {
			var report validation.Report
			var err error
			if target == validation.TargetSource {
				report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceTimeline})
			} else {
				report, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelTimeline})
			}
			if err != nil {
				t.Fatalf("operation: %v; %s", err, localFindings(report))
			}
			if !report.InputComplete {
				t.Fatalf("fixture was not fully assessed: %s", localFindings(report))
			}
			check(t, report)
		})
	}
}

func TestTimelineMandatoryMilestonesBothTargets(t *testing.T) {
	key := `<attributes><divisions>3</divisions><key><fifths>0</fifths></key></attributes>`
	localKey := `<numeral-key><numeral-fifths>0</numeral-fifths><numeral-mode>major</numeral-mode></numeral-key>`
	for _, tt := range []struct {
		name, body, failure string
	}{
		{"principal-four-one-three", key + timelineNote("4", "", "") + timelineNote("1", `<chord/>`, "") + timelineNote("3", `<chord/>`, ""), ""},
		{"tone-exceeds-principal", key + timelineNote("1", "", "") + timelineNote("2", `<chord/>`, ""), "MX40-TIMELINE-chord-duration-bound"},
		{"explicit-zero-fifths", key + timelineNumeral("", ""), ""},
		{"local-key-needs-no-divisions", timelineNumeral(localKey, ""), ""},
		{"complete-absent-key", timelineNumeral("", ""), "MX40-TIMELINE-numeral-requires-key"},
		{"fractional-duration", key + timelineNote("0.5", "", "") + timelineNote("0.25", `<chord/>`, ""), ""},
		{"cross-staff-chord", `<attributes><divisions>3</divisions><staves>2</staves></attributes>` + timelineNote("4", "", `<voice>upper</voice><staff>2</staff>`) + timelineNote("3", `<chord/>`, `<voice>upper</voice><staff>1</staff>`), ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assessTimelineBoth(t, (localFixture{body: tt.body}).xml(), func(t *testing.T, report validation.Report) {
				if !report.AssessmentComplete {
					t.Fatalf("unexpected incomplete result: %s", localFindings(report))
				}
				if tt.failure == "" {
					if !report.Satisfied() {
						t.Fatalf("positive rejected: %s", localFindings(report))
					}
					return
				}
				found := false
				for _, finding := range report.Findings {
					if finding.Rule.ID == validation.RuleID(tt.failure) && finding.Outcome == validation.OutcomeFail {
						found = true
					}
					if finding.Outcome == validation.OutcomeFail && strings.HasPrefix(string(finding.Rule.ID), "MX40-XSD-") {
						t.Errorf("unintended schema failure: %s", localFindings(report))
					}
				}
				if !found || report.Satisfied() || report.Conformance != validation.ConformanceViolated {
					t.Fatalf("missing intended failure %s: %s", tt.failure, localFindings(report))
				}
			})
		})
	}
}

func TestTimelineProfilesAreExplicitAndLegacyRemainsXSDOnly(t *testing.T) {
	source := (localFixture{body: timelineNumeral("", "")}).xml()
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(doc); err != nil {
		t.Fatalf("legacy XSD scope broadened: %v", err)
	}
	for _, profile := range []validation.ProfileID{ProfileModelXSD, ProfileModelLocal} {
		r, err := AssessModel(context.Background(), doc, AssessmentOptions{Profile: profile})
		if err != nil || !r.Satisfied() {
			t.Fatalf("frozen model profile broadened: %s %v %s", profile, err, localFindings(r))
		}
	}
	for _, profile := range []validation.ProfileID{ProfileSourceXSDPreview, ProfileSourceLocal} {
		r, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: profile})
		if err != nil || !r.Satisfied() {
			t.Fatalf("frozen source profile broadened: %s %v %s", profile, err, localFindings(r))
		}
	}
	result, err := DecodeAssessed(context.Background(), strings.NewReader(source), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: ProfileSourceSemantics}})
	if !errors.Is(err, ErrInvalidDocument) || result.Document != nil || result.Conversion.State != validation.ConversionConverted {
		t.Fatalf("strict timeline acceptance: %+v %v", result, err)
	}
}

func TestTimelinePreservesFrozenCatalogDigests(t *testing.T) {
	engine, err := buildModelAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []validation.ProfileID{ProfileModelXSD, ProfileModelLocal, ProfileSourceLocal} {
		if got := catalogProfile(t, engine, id).CatalogDigest; got != decimalContractCatalogAfter {
			t.Errorf("frozen profile %s digest changed: %s", id, got)
		}
	}
	engine, err = buildSourceAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	if got := catalogProfile(t, engine, ProfileSourceXSDPreview).CatalogDigest; got != "e6139cbdde4b7eeb977fab3c4c33213300c536a902b5bb313ffa3483ff880760" {
		t.Errorf("frozen source preview digest changed: %s", got)
	}
}
