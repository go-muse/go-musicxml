package musicxml

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// MusicXML deliberately retained style-text rather than the proposed integer
// restriction: https://github.com/w3c-cg/musicxml/issues/94#issuecomment-290221733.
// These checks establish conformance to that open text declaration, not numeric
// meaning for arbitrary notation.
func TestFigureNumberUnrestrictedTextConformance(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		for _, value := range []string{"6", "10", "0006", "0", "+6", "-6", "6.0", "6/5", "6½", "VI", "Ⅵ", "٦", "６", "not-a-number", "", " ", "\t\n", " 6 "} {
			source := contextFixtureXML(`<figured-bass><figure><prefix>sharp</prefix><figure-number>`+value+`</figure-number><suffix>slash</suffix></figure></figured-bass>`, timewise)
			doc, err := Decode(strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			for _, model := range []bool{false, true} {
				t.Run(fmt.Sprintf("timewise=%t/model=%t/text=%q", timewise, model, value), func(t *testing.T) {
					var report validation.Report
					if model {
						report, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
					} else {
						report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceContext})
					}
					if err != nil || !report.Satisfied() || !report.AssessmentComplete || report.Counts.Unsupported != 0 {
						t.Fatalf("unrestricted style-text incurred a conformance penalty: %v; %s", err, localFindings(report))
					}
					assertNoFigureNumberNumericRequirement(t, report)
				})
			}
		}
	}
}

func TestFigureNumberUnrestrictedTextPreservesXSDConstraints(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		source := contextFixtureXML(`<figured-bass><figure><figure-number font-style="invalid">Ⅵ</figure-number></figure></figured-bass>`, timewise)
		doc, err := Decode(strings.NewReader(source))
		if err != nil {
			t.Fatal(err)
		}
		for _, model := range []bool{false, true} {
			var report validation.Report
			if model {
				report, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
			} else {
				report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceContext})
			}
			if err != nil || report.Satisfied() || report.Conformance != validation.ConformanceViolated {
				t.Fatalf("invalid style-text attribute accepted: model=%t timewise=%t: %v; %s", model, timewise, err, localFindings(report))
			}
			failed := false
			for _, finding := range report.Findings {
				failed = failed || strings.HasPrefix(string(finding.Rule.ID), "MX40-XSD-") && finding.Outcome == validation.OutcomeFail
			}
			if !failed {
				t.Fatalf("independent XSD cause lost: %s", localFindings(report))
			}
			assertNoFigureNumberNumericRequirement(t, report)
		}
	}
}

func TestFigureNumberUnrestrictedTextStrictDecode(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		source := contextFixtureXML(`<figured-bass><figure><figure-number>Ⅵ</figure-number></figure></figured-bass>`, timewise)
		result, err := DecodeAssessed(context.Background(), strings.NewReader(source), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: ProfileSourceContext}})
		if err != nil || result.Document == nil {
			t.Fatalf("valid open notation rejected by strict decode: %v", err)
		}
	}
}

func assertNoFigureNumberNumericRequirement(t *testing.T, report validation.Report) {
	t.Helper()
	const retired = "MX40-CONTEXT-figure-number-numeric"
	for _, rule := range report.Profile.Required {
		if rule.ID == retired {
			t.Error("built-in profile still selects the withdrawn numeric overconstraint")
		}
	}
	for _, support := range report.Profile.Support {
		if support.Rule.ID == retired {
			t.Error("built-in support still advertises the withdrawn numeric overconstraint")
		}
	}
	for _, finding := range report.Findings {
		if finding.Rule.ID == retired {
			t.Errorf("withdrawn numeric overconstraint produced a finding: %+v", finding)
		}
	}
}
