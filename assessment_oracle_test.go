package musicxml

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// This oracle receives the identical original bytes as InspectXML. Model
// re-encoding is deliberately absent: it would erase occurrence/order evidence.
func TestSourceAssessmentAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	valid := assessedScore("1", `<instruments>1</instruments>`)
	note := strings.Replace(valid, `</measure>`, `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note></measure>`, 1)
	tests := []struct {
		name, input string
		valid       bool
	}{
		{"ordinary", valid, true},
		{"go-uint64-boundary", assessedScore("18446744073709551615", ""), true},
		{"source-unbounded-2-to-64", assessedScore("18446744073709551616", ""), true},
		{"negative-staves", assessedScore("-1", ""), false},
		{"fractional-integer", assessedScore("1.5", ""), false},
		{"unknown-simple-attribute", strings.Replace(valid, `<staves>`, `<staves rubbish="x">`, 1), false},
		{"repeated-source-occurrence", assessedScore("1", `<staves>2</staves>`), false},
		{"physical-pitch-order", strings.Replace(note, `<step>C</step><octave>4</octave>`, `<octave>4</octave><step>C</step>`, 1), false},
		{"ordered-pitch", note, true},
		{"foreign-extension", assessedScore("1", `<f:extension xmlns:f="urn:test"><staves>2</staves></f:extension>`), false},
		{"foreign-attribute", strings.Replace(valid, `<measure number="1">`, `<measure number="1" xmlns:f="urn:test" f:implicit="yes">`, 1), false},
		{"odd-schema-location-datatype", strings.Replace(valid, ` version="4.0"`, ` version="4.0" xmlns:i="http://www.w3.org/2001/XMLSchema-instance" i:schemaLocation="urn:odd"`, 1), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			report, err := InspectXML(context.Background(), strings.NewReader(tt.input), AssessmentOptions{Profile: ProfileSourceXSDPreview})
			if err != nil || !report.InputComplete || report.Syntax != validation.SyntaxWellFormed || report.Satisfied() != tt.valid {
				t.Errorf("source assessment=%+v error=%v expected-valid=%t", report, err, tt.valid)
			}
			assertXMLLintOutcome(t, xmllint, filepath.Join("schema", "musicxml-4.0", "musicxml.xsd"), filepath.Join("schema", "musicxml-4.0", "catalog.xml"), tt.input, tt.valid)
		})
	}
	// These unchanged original files intentionally exercise both the usable
	// DTD-independent scope and the explicit external-subset capability boundary.
	// xmllint's XSD result does not establish unread DTD infoset neutrality.
	for _, fixture := range []struct {
		name, schema string
		externalDTD  bool
	}{
		{"Chant.musicxml", "musicxml.xsd", true},
		{"MozartTrio.musicxml", "musicxml.xsd", true},
		{"MozaChloSample.musicxml", "musicxml.xsd", true},
		{"OpusLink.musicxml", "opus.xsd", false},
		{"OpusScore.musicxml", "opus.xsd", false},
	} {
		t.Run("official/"+fixture.name, func(t *testing.T) {
			t.Parallel()
			input, err := os.ReadFile(filepath.Join("testdata", "official", fixture.name))
			if err != nil {
				t.Fatal(err)
			}
			report, err := InspectXML(context.Background(), bytes.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview})
			if err != nil || !report.InputComplete || report.Syntax != validation.SyntaxWellFormed {
				t.Fatalf("original source: %+v %v", report, err)
			}
			if fixture.externalDTD {
				if report.Satisfied() || report.AssessmentComplete || report.Conformance != validation.ConformanceUndetermined || report.Counts.Unsupported == 0 || report.Counts.Fail != 0 {
					t.Fatalf("unread external subset became trusted: %+v", report)
				}
				found := false
				for _, finding := range report.Findings {
					if finding.Reason == "dtd-infoset-unavailable" {
						found = true
					}
				}
				if !found {
					t.Fatal("missing explicit DTD capability explanation")
				}
				var result DecodeResult
				var decodeErr error
				if fixture.name == "MozaChloSample.musicxml" {
					result, decodeErr = decodeMozaWithDeclaredCapacityControl(t, input)
				} else {
					result, decodeErr = DecodeAssessed(context.Background(), bytes.NewReader(input), strictSourceOptions())
				}
				if result.Document != nil || result.Conversion.State != validation.ConversionConverted || !errors.Is(decodeErr, ErrIncompleteAssessment) || errors.Is(decodeErr, ErrInvalidDocument) || !result.Source.InputComplete || result.Model.Requested {
					t.Fatalf("strict external-DTD result: %+v %v", result, decodeErr)
				}
			} else if !report.Satisfied() {
				t.Fatalf("DOCTYPE-free official opus fixture: %+v", report)
			}
			assertXMLLintOutcome(t, xmllint, filepath.Join("schema", "musicxml-4.0", fixture.schema), filepath.Join("schema", "musicxml-4.0", "catalog.xml"), string(input), true)
		})
	}
}

func TestSourceAssessmentRetainsOrderErasedByModels(t *testing.T) {
	input := strings.Replace(assessedScore("1", ""), `</measure>`, `<note><pitch><octave>4</octave><step>C</step></pitch><duration>1</duration></note></measure>`, 1)
	document, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if err = Validate(document); err != nil {
		t.Fatalf("fixture must demonstrate reordered model loses original evidence: %v", err)
	}
	report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview})
	if err != nil || report.Conformance != validation.ConformanceViolated {
		t.Fatalf("physical source ordering vanished: %+v %v", report, err)
	}
}
