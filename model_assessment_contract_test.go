package musicxml

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestAssessModelFiniteResourcesAndIndependentAxes(t *testing.T) {
	for _, budget := range []validation.Budgets{{MaxNodes: 1}, {MaxAttributes: 1}, {MaxEvents: 1}, {MaxRuleSteps: 1}, {MaxBufferedBytes: 1}, {MaxScalarBytes: 1}, {MaxDepth: 1}} {
		report, err := AssessModel(context.Background(), modelMinimalScore(), AssessmentOptions{Profile: ProfileModelLocal, Budgets: budget})
		if !errors.Is(err, validation.ErrResourceLimit) || errors.Is(err, ErrInvalidDocument) || report.Satisfied() || report.AssessmentComplete || (report.Requested && report.Syntax != validation.SyntaxNotAssessed) {
			t.Fatalf("budget %+v: err=%v axes=%+v", budget, err, report)
		}
	}
	if report, err := AssessModel(context.Background(), modelMinimalScore(), AssessmentOptions{Profile: ProfileModelXSD, Budgets: validation.Budgets{MaxInputBytes: 1}}); err != nil || !report.Satisfied() {
		t.Fatalf("current graph charged as fabricated source bytes: %+v %v", report, err)
	}
	doc := modelMinimalScore()
	doc.Part[0].Measure[0].Content[0].Note.Duration = Ptr(PositiveDivisions(math.NaN()))
	report, err := AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelXSD, Budgets: validation.Budgets{MaxDiagnostics: 1, MaxDiagnosticBytes: 1}})
	if err != nil || !report.InputComplete || report.Satisfied() || report.BlockingProblems == 0 || !report.DiagnosticsTruncated || len(report.Findings) != 0 {
		t.Fatalf("truncated model problem erased: %+v %v", report, err)
	}
	cycle := NewOpusDocument()
	cycle.AddOpus(cycle)
	cycle.AddScore(&OpusScore{Href: "after-cycle.musicxml"})
	report, err = AssessModel(context.Background(), cycle, AssessmentOptions{Profile: ProfileModelXSD})
	if err != nil || !report.InputComplete || report.Satisfied() || report.BlockingProblems == 0 || report.Syntax != validation.SyntaxNotAssessed {
		t.Fatalf("bounded cycle result: %+v %v", report, err)
	}
}

func TestBoundedLocalStrictAccordionIntegration(t *testing.T) {
	source := (localFixture{body: localDirection(`<accordion-registration/>`)}).xml()
	doc, err := Decode(strings.NewReader(source))
	if err != nil || doc == nil {
		t.Fatalf("permissive transport: %v", err)
	}
	if err = Validate(doc); err != nil {
		t.Fatalf("XSD-only facade unexpectedly selected local prose: %v", err)
	}
	preview, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceXSDPreview})
	if err != nil || !preview.Satisfied() {
		t.Fatalf("XSD preview: %+v %v", preview, err)
	}
	result, err := DecodeAssessed(context.Background(), strings.NewReader(source), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: ProfileSourceLine}})
	if !errors.Is(err, ErrInvalidDocument) || result.Document != nil || result.Conversion.State != validation.ConversionConverted || !result.SourceLine.InputComplete || !result.SourceLine.AssessmentComplete {
		t.Fatalf("strict local outcome: %+v %v", result, err)
	}
	report, err := AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelLocal})
	if err != nil || report.Conformance != validation.ConformanceViolated {
		t.Fatalf("current model local outcome: %+v %v", report, err)
	}
}

func TestBoundedLocalSourceCapabilityAndPartialFailure(t *testing.T) {
	source := (localFixture{body: `<sound dacapo="no"/>`}).xml()
	report, err := InspectXML(context.Background(), strings.NewReader(source+`<broken`), AssessmentOptions{Profile: ProfileSourceLocal})
	found := false
	for _, f := range report.Findings {
		if f.Rule.ID == "MX40-LOCAL-sound-dacapo-yes" && f.Outcome == validation.OutcomeFail {
			found = true
		}
	}
	if err == nil || !found || report.Conformance != validation.ConformanceViolated || report.InputComplete {
		t.Fatalf("earlier local failure lost on malformed tail: %v %s", err, localFindings(report))
	}
	valid := (localFixture{}).xml()
	input := `<!DOCTYPE score-partwise SYSTEM "https://not-fetched.invalid/musicxml.dtd">` + valid
	report, err = InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceLocal})
	if err != nil || !report.InputComplete || report.Satisfied() || report.AssessmentComplete || report.Syntax != validation.SyntaxWellFormed || report.Counts.Unsupported == 0 {
		t.Fatalf("DTD closure silently changed: %+v %v", report, err)
	}
	result, err := DecodeAssessed(context.Background(), strings.NewReader(input), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: ProfileSourceLine}})
	if !errors.Is(err, ErrIncompleteAssessment) || errors.Is(err, ErrInvalidDocument) || result.Document != nil || result.Conversion.State != validation.ConversionConverted {
		t.Fatalf("DTD strict closure: %+v %v", result, err)
	}
	for _, profile := range []validation.ProfileID{ProfileModelXSD, ProfileModelLocal} {
		reader := &assessmentCountingReader{source: strings.NewReader(valid)}
		if _, err := InspectXML(context.Background(), reader, AssessmentOptions{Profile: profile}); !errors.Is(err, validation.ErrInvalidConfiguration) || reader.reads != 0 {
			t.Fatalf("wrong source target consumed input: %s %d %v", profile, reader.reads, err)
		}
	}
}

func TestAssessModelOfficialCorpusAndSourceDTDRemainSeparate(t *testing.T) {
	for _, name := range []string{"Chant.musicxml", "MozartTrio.musicxml", "MozaChloSample.musicxml", "OpusLink.musicxml", "OpusScore.musicxml"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "official", name))
			if err != nil {
				t.Fatal(err)
			}
			doc, err := Decode(strings.NewReader(string(data)))
			if err != nil {
				t.Fatal(err)
			}
			for _, profile := range []validation.ProfileID{ProfileModelXSD, ProfileModelLocal} {
				model, err := AssessModel(context.Background(), doc, AssessmentOptions{Profile: profile})
				if err != nil || !model.Satisfied() {
					t.Fatalf("current corpus %s: %v %s", profile, err, localFindings(model))
				}
			}
			source, err := InspectXML(context.Background(), strings.NewReader(string(data)), AssessmentOptions{Profile: ProfileSourceLocal})
			if err != nil {
				t.Fatal(err)
			}
			score := !strings.HasPrefix(name, "Opus")
			if score && (source.AssessmentComplete || source.Satisfied() || source.Counts.Unsupported == 0) {
				t.Fatal("unread official external DTD accepted")
			}
			if !score && !source.Satisfied() {
				t.Fatalf("DOCTYPE-free opus: %s", localFindings(source))
			}
		})
	}
}

func TestBoundedDateReusesScalarConsequenceBothTargets(t *testing.T) {
	for _, date := range []string{"2024-02-29", "2026-02-30", "0000-01-01", "2024-01-01Z"} {
		source := (localFixture{header: `<identification><encoding><encoding-date>` + date + `</encoding-date></encoding></identification>`}).xml()
		doc, err := Decode(strings.NewReader(source))
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range []validation.Target{validation.TargetSource, validation.TargetModel} {
			var report validation.Report
			if target == validation.TargetSource {
				report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceLocal})
			} else {
				report, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelLocal})
			}
			if err != nil || report.Satisfied() != (date == "2024-02-29") {
				t.Fatalf("%s %s: %v %s", target, date, err, localFindings(report))
			}
			scalarFailures := 0
			for _, f := range report.Findings {
				if f.Outcome == validation.OutcomeFail {
					if strings.HasPrefix(string(f.Rule.ID), "MX40-LOCAL-date") {
						t.Fatal("duplicate prose date predicate")
					}
					if f.Rule.ID == "MX40-XSD-musicxml-scalar" {
						scalarFailures++
					}
				}
			}
			if date != "2024-02-29" && scalarFailures != 1 {
				t.Fatalf("%s %s: date shared scalar failures=%d; %s", target, date, scalarFailures, localFindings(report))
			}
		}
	}
}

func TestBoundedProfilesConcurrentIndependentReports(t *testing.T) {
	doc := modelMinimalScore()
	baseline, err := AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelLocal})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			t.Parallel()
			report, err := AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelLocal})
			if err != nil || !reflect.DeepEqual(baseline, report) {
				t.Errorf("non-independent model report: %v", err)
			}
		})
	}
}
