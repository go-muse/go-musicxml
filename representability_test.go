package musicxml

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func representabilityScore(attributes, notes string) string {
	return `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name>` +
		`<score-instrument id="I1"><instrument-name>Piano</instrument-name><ensemble>99999999999999999999999</ensemble></score-instrument></score-part></part-list>` +
		`<part id="P1"><measure number="1"><attributes><divisions>1</divisions>` + attributes + `</attributes>` + notes + `</measure></part></score-partwise>`
}

func representabilityNote(duration, extra string) string {
	return `<note` + extra + `><pitch><step>C</step><octave>4</octave></pitch><duration>` + duration + `</duration></note>`
}

// The source line rejects exactly the numeric values the model cannot hold
// unchanged, on elements and attributes; union values, held as strings, and
// ordinary decimals pass.
func TestSourceRepresentabilityRule(t *testing.T) {
	tiny := "0." + strings.Repeat("0", 399) + "1"
	for _, tc := range []struct {
		name, score string
		paths       []string
	}{
		{"ordinary values and a huge union value", representabilityScore("", representabilityNote("0.1", "")+representabilityNote("0.25", ` default-x="12.5"`)), nil},
		{"element decimal precision", representabilityScore("", representabilityNote("1.00000000000000000001", "")), []string{"/score-partwise/part/measure/note/duration"}},
		{"attribute decimal precision", representabilityScore("", representabilityNote("1", ` default-x="1.000000000000000000001"`)), []string{"/score-partwise/part/measure/note/@default-x"}},
		{"decimal underflow", representabilityScore("", representabilityNote(tiny, "")), []string{"/score-partwise/part/measure/note/duration"}},
		{"integer beyond uint64", representabilityScore("<staves>18446744073709551616</staves>", representabilityNote("1", "")), []string{"/score-partwise/part/measure/attributes/staves"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, err := InspectXML(context.Background(), strings.NewReader(tc.score), AssessmentOptions{Profile: ProfileSourceSemantics})
			if err != nil {
				t.Fatal(err)
			}
			var paths []string
			for _, f := range report.Findings {
				if strings.HasPrefix(string(f.Rule.ID), "MX40-SOURCE-REPRESENTABLE-") && f.Outcome == validation.OutcomeFail {
					paths = append(paths, f.Location.Path)
				}
			}
			if strings.Join(paths, ",") != strings.Join(tc.paths, ",") {
				t.Fatalf("representability failures at %v, want %v", paths, tc.paths)
			}
			if report.Conformance != validation.ConformanceConformant {
				t.Errorf("a representability limit changed conformance: %s", report.Conformance)
			}
			if report.Satisfied() != (len(tc.paths) == 0) {
				t.Errorf("satisfied=%t with failures %v", report.Satisfied(), paths)
			}
			result, err := DecodeAssessed(context.Background(), strings.NewReader(tc.score), DecodeAssessmentOptions{})
			if len(tc.paths) == 0 {
				if err != nil || result.Document == nil {
					t.Fatalf("representable document rejected: %v", err)
				}
				return
			}
			if result.Document != nil || !errors.Is(err, ErrInvalidDocument) {
				t.Fatalf("unrepresentable document: document=%T error=%v", result.Document, err)
			}
			if errors.Is(err, ErrIncompleteAssessment) && !strings.Contains(tc.name, "integer") {
				t.Errorf("a representability failure reported as incomplete: %v", err)
			}
		})
	}
}

// Model decimals are the shortest decimal of each float64: 0.1 + 0.2 equals
// 0.3 in the model, as in the source, so timeline rules agree across lines.
func TestModelDecimalsAgreeWithTheSource(t *testing.T) {
	note := func(duration string) string {
		return `<note><pitch><step>C</step><octave>4</octave></pitch><duration>` + duration + `</duration></note>`
	}
	for _, tc := range []struct {
		name, notes, rule string
		fails             bool
	}{
		{"numeral key after binary sums", note("0.1") + note("0.2") + `<attributes><key><fifths>0</fifths></key></attributes><backup><duration>0.1</duration></backup><backup><duration>0.2</duration></backup><forward><duration>0.3</duration></forward><harmony><numeral><numeral-root>1</numeral-root></numeral><kind>major</kind></harmony>`, "MX40-TIMELINE-numeral-requires-key", false},
		{"barline location after binary sums", note("0.1") + note("0.2") + `<backup><duration>0.3</duration></backup><barline location="middle"/>` + note("0.3"), "MX40-CONTEXT-barline-location", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			score := mandatoryScore("1", tc.notes)
			source, err := InspectXML(context.Background(), strings.NewReader(score), AssessmentOptions{Profile: ProfileSourceSemantics})
			if err != nil {
				t.Fatal(err)
			}
			document, err := Decode(bytes.NewReader([]byte(score)))
			if err != nil {
				t.Fatal(err)
			}
			model, err := AssessModel(context.Background(), document, AssessmentOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if got := ruleFailed(source, tc.rule); got != tc.fails {
				t.Fatalf("source %s failed=%t, want %t", tc.rule, got, tc.fails)
			}
			if got := ruleFailed(model, tc.rule); got != tc.fails {
				t.Errorf("model %s failed=%t, want %t as in the source", tc.rule, got, tc.fails)
			}
		})
	}
}

func ruleFailed(r validation.Report, rule string) bool {
	for _, f := range r.Findings {
		if string(f.Rule.ID) == rule && f.Outcome == validation.OutcomeFail {
			return true
		}
	}
	return false
}
