package musicxml

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// The literal permits the pre-implementation regression to compile: the RED
// result is an unavailable explicitly selected profile, not a missing symbol.
func TestSourceInternalDTDProfilePublicAcceptance(t *testing.T) {
	const profile validation.ProfileID = "musicxml-4.0/source-internal-dtd"
	base := assessedScore("1", "")
	for _, tc := range []struct{ name, input string }{
		{"physical", base},
		{"default-version", `<!DOCTYPE score-partwise [<!ATTLIST score-partwise version CDATA "4.0">]>` + strings.Replace(base, ` version="4.0"`, "", 1)},
		{"internal-text", `<!DOCTYPE score-partwise [<!ENTITY title "Bounded title">]>` + strings.Replace(base, "<part-list>", "<movement-title>&title;</movement-title><part-list>", 1)},
		{"default-xlink-namespace", `<!DOCTYPE opus [<!ATTLIST opus xmlns:xlink CDATA "http://www.w3.org/1999/xlink">]><opus version="4.0"><score xlink:href="a.xml"/></opus>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &assessmentCountingReader{source: strings.NewReader(tc.input)}
			report, err := InspectXML(context.Background(), r, AssessmentOptions{Profile: profile})
			if err != nil || !report.Satisfied() || !report.InputComplete || report.Syntax != validation.SyntaxWellFormed {
				t.Fatalf("selected internal-DTD profile: error=%v syntax=%s complete=%t satisfied=%t findings=%+v", err, report.Syntax, report.InputComplete, report.Satisfied(), report.Findings)
			}
			if r.closed || r.bytes != len(tc.input) {
				t.Fatalf("input ownership: closed=%t bytes=%d want=%d", r.closed, r.bytes, len(tc.input))
			}
			origins := 0
			for _, fact := range report.Context {
				if fact.Property == "source.origin" {
					origins++
				}
			}
			if origins == 0 {
				t.Fatal("selected source-origin provider published no occurrence evidence")
			}
		})
	}
}

func TestSourceInternalDTDDefaultProvenanceAndExplicitSuppression(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		input := assessedScore("1", "")
		if !explicit {
			input = strings.Replace(input, ` version="4.0"`, "", 1)
		}
		input = `<!DOCTYPE score-partwise [<!ATTLIST score-partwise version CDATA "4.0">]>` + input
		report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTD})
		if err != nil || !report.Satisfied() {
			t.Fatalf("explicit=%t error=%v report=%+v", explicit, err, report)
		}
		found := 0
		for _, row := range report.Context {
			origin := row.SourceOrigin
			if origin == nil || row.Subject.Node != 1 || origin.Attribute == nil || *origin.Attribute != (validation.QName{Local: "version"}) {
				continue
			}
			found++
			if origin.Specified.State != validation.EvidenceKnown || origin.Specified.Value != explicit {
				t.Fatalf("specified=%+v explicit=%t", origin.Specified, explicit)
			}
			if explicit {
				if origin.Kind != validation.SourceOriginPhysical || origin.Physical.State != validation.EvidenceKnown || origin.Physical.Value.Position == nil {
					t.Fatalf("physical origin=%+v", origin)
				}
			} else if origin.Kind != validation.SourceOriginDefault || origin.Physical.State != validation.EvidenceAbsent || origin.Declaration.State != validation.EvidenceKnown || origin.Use.State != validation.EvidenceKnown {
				t.Fatalf("default origin=%+v", origin)
			}
		}
		if found != 1 {
			t.Fatalf("version origin count=%d", found)
		}
	}
}

func TestSourceInternalDTDDefaultExpandedNameCollision(t *testing.T) {
	input := `<!DOCTYPE score-partwise [<!ATTLIST score-partwise q:x CDATA "default">]><score-partwise xmlns:p="urn:same" xmlns:q="urn:same" p:x="explicit"/>`
	report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTD})
	if err == nil || report.Syntax != validation.SyntaxMalformed || report.Satisfied() {
		t.Fatalf("error=%v syntax=%s satisfied=%t", err, report.Syntax, report.Satisfied())
	}
}

func TestSourceInternalDTDDecodeUsesSameEffectiveStream(t *testing.T) {
	input := `<!DOCTYPE score-partwise [<!ENTITY title "Expanded title"><!ATTLIST score-partwise version CDATA "4.0">]>` + strings.Replace(strings.Replace(assessedScore("1", ""), ` version="4.0"`, "", 1), "<part-list>", "<movement-title>&title;</movement-title><part-list>", 1)
	reader := &assessmentCountingReader{source: strings.NewReader(input)}
	result, err := DecodeAssessed(context.Background(), reader, DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: ProfileSourceLine}})
	if err != nil || !result.SourceLine.Satisfied() || !result.ModelLine.Satisfied() || result.Conversion.State != validation.ConversionConverted {
		t.Fatalf("error=%v syntax=%s conversion=%s", err, result.SourceLine.Syntax, result.Conversion.State)
	}
	score, ok := result.Document.(*ScorePartwise)
	if !ok || score.Version == nil || *score.Version != "4.0" || score.MovementTitle == nil || *score.MovementTitle != "Expanded title" {
		t.Fatalf("document=%+v", result.Document)
	}
	if reader.closed || reader.bytes != len(input) {
		t.Fatalf("reader=%+v", reader)
	}
}

func TestSourceInternalDTDModePreflightAndLegacyIsolation(t *testing.T) {
	input := `<!DOCTYPE score-partwise [<!ENTITY title "Expanded title">]>` + strings.Replace(assessedScore("1", ""), "<part-list>", "<movement-title>&title;</movement-title><part-list>", 1)
	if _, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview}); !errors.Is(err, validation.ErrIncompleteAssessment) {
		t.Fatalf("legacy entity expansion unexpectedly elected: %v", err)
	}
	if _, err := AssessModel(context.Background(), &ScorePartwise{}, AssessmentOptions{Profile: ProfileSourceInternalDTD}); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatalf("model accepted source profile: %v", err)
	}
	oldEngine, err := newSourceAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	account, err := validation.NewBudgetAccount(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	reader := &assessmentCountingReader{source: strings.NewReader(input)}
	_, err = newSourceAssessmentRunWithConfig(context.Background(), reader, DecodeOptions{}, &sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: account, DocumentID: "source", Profile: ProfileSourceInternalDTD, Engine: oldEngine}, ByteOwner: sourceBytesPrimary}, false)
	if !errors.Is(err, validation.ErrInvalidConfiguration) || reader.reads != 0 {
		t.Fatalf("mismatched engine: error=%v reads=%d", err, reader.reads)
	}
}

func TestSourceInternalDTDUnverifiedDeclarationClosureIsIncomplete(t *testing.T) {
	for _, subset := range []string{`<!ENTITY unused '&unused;'>`, `<!ENTITY amp 'wrong'>`} {
		t.Run(subset, func(t *testing.T) {
			input := `<!DOCTYPE score-partwise [` + subset + `]>` + assessedScore("1", "")
			report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTD})
			// InspectXML records ordinary unsupported assessment in the report;
			// nil error is not acceptance and does not imply syntax closure.
			if err != nil || report.Satisfied() || report.Syntax != validation.SyntaxIncomplete || !report.InputComplete {
				t.Fatalf("unverified declaration closure: err=%v syntax=%s complete=%t satisfied=%t", err, report.Syntax, report.InputComplete, report.Satisfied())
			}
		})
	}
}
