package musicxml

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/go-muse/go-musicxml/validation"
)

type assessmentCountingReader struct {
	source       io.Reader
	reads, bytes int
	closed       bool
}

func (r *assessmentCountingReader) Read(p []byte) (int, error) {
	r.reads++
	n, e := r.source.Read(p)
	r.bytes += n
	return n, e
}
func (r *assessmentCountingReader) Close() error { r.closed = true; return nil }

func TestSourceAssessmentPreflightDoesNotRead(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name    string
		ctx     context.Context
		options DecodeAssessmentOptions
		want    error
	}{
		{"nil-context", nil, DecodeAssessmentOptions{}, validation.ErrInvalidConfiguration},
		{"canceled", canceled, DecodeAssessmentOptions{}, context.Canceled},
		{"depth", context.Background(), DecodeAssessmentOptions{Decode: DecodeOptions{-1}}, ErrInvalidDecodeOptions},
		{"empty-profile", context.Background(), DecodeAssessmentOptions{Assessment: &AssessmentOptions{}}, validation.ErrInvalidConfiguration},
		{"unknown-profile", context.Background(), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: "unknown"}}, validation.ErrInvalidConfiguration},
		{"model-profile", context.Background(), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: "musicxml-4.0/model"}}, validation.ErrInvalidConfiguration},
		{"negative-budget", context.Background(), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: ProfileSourceXSDPreview, Budgets: validation.Budgets{MaxNodes: -1}}}, validation.ErrInvalidConfiguration},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &assessmentCountingReader{source: strings.NewReader(`<score-partwise/>`)}
			result, err := DecodeAssessed(tt.ctx, r, tt.options)
			if !errors.Is(err, tt.want) {
				t.Errorf("error=%v; want %v", err, tt.want)
			}
			if r.reads != 0 || r.closed || result.Document != nil {
				t.Errorf("preflight read=%d closed=%t document=%T", r.reads, r.closed, result.Document)
			}
		})
	}
	if _, err := InspectXML(context.Background(), nil, AssessmentOptions{Profile: ProfileSourceXSDPreview}); !errors.Is(err, ErrNilReader) {
		t.Errorf("nil reader: %v", err)
	}
}

func TestDecodeAssessedPermissiveAndStrictSourceAreSeparate(t *testing.T) {
	input := `<score-partwise/>`
	result, err := DecodeAssessed(context.Background(), strings.NewReader(input), DecodeAssessmentOptions{})
	if err != nil || result.Document == nil || result.Source.Requested || result.Source.Satisfied() || result.Conversion.State != validation.ConversionConverted {
		t.Fatalf("permissive result=%+v error=%v", result, err)
	}
	report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceXSDPreview})
	if err != nil || !report.InputComplete || report.Syntax != validation.SyntaxWellFormed || report.Conformance != validation.ConformanceViolated {
		t.Fatalf("inspection result=%+v error=%v", report, err)
	}
	result, err = DecodeAssessed(context.Background(), strings.NewReader(input), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: ProfileSourceXSDPreview}})
	if !errors.Is(err, ErrInvalidDocument) || result.Document != nil || result.Conversion.State != validation.ConversionConverted || result.Source.Conformance != validation.ConformanceViolated {
		t.Fatalf("strict result=%+v error=%v", result, err)
	}
}

func TestSourceAssessmentDTDCompletenessAxes(t *testing.T) {
	valid := assessedScore("1", "")
	for _, tt := range []struct {
		name, prefix string
		syntax       validation.SyntaxState
		complete     bool
	}{
		{"empty-subset", `<!DOCTYPE score-partwise [<!-- checked -->]>`, validation.SyntaxWellFormed, true},
		{"different-doctype-name", `<!DOCTYPE different>`, validation.SyntaxWellFormed, true},
		{"external-subset", `<!DOCTYPE score-partwise SYSTEM "file:///definitely-not-read.dtd">`, validation.SyntaxWellFormed, false},
		{"attribute-declaration", `<!DOCTYPE score-partwise [<!ATTLIST score-partwise hidden CDATA #IMPLIED>]>`, validation.SyntaxIncomplete, false},
		{"entity-declaration", `<!DOCTYPE score-partwise [<!ENTITY unused "value">]>`, validation.SyntaxIncomplete, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report, err := InspectXML(context.Background(), strings.NewReader(tt.prefix+valid), AssessmentOptions{Profile: ProfileSourceXSDPreview})
			if err != nil || !report.InputComplete || report.Syntax != tt.syntax || report.Satisfied() != tt.complete {
				t.Fatalf("DTD axes report=%+v error=%v", report, err)
			}
			result, err := DecodeAssessed(context.Background(), strings.NewReader(tt.prefix+valid), strictSourceOptions())
			if result.Conversion.State != validation.ConversionConverted || (err == nil) != tt.complete || errors.Is(err, ErrInvalidDocument) || (result.Document != nil) != tt.complete {
				t.Errorf("DTD strict decode=%+v error=%v", result, err)
			}
			if !tt.complete && (!errors.Is(err, ErrIncompleteAssessment) || report.Counts.Unsupported == 0) {
				t.Errorf("DTD missing required unsupported blocker: %+v %v", report, err)
			}
		})
	}
}

func TestSourceAssessmentConcurrentIndependentReports(t *testing.T) {
	for worker := 0; worker < 8; worker++ {
		t.Run(strconv.Itoa(worker), func(t *testing.T) {
			t.Parallel()
			for repeat := 0; repeat < 4; repeat++ {
				report, err := InspectXML(context.Background(), strings.NewReader(`<score-partwise/>`), AssessmentOptions{Profile: ProfileSourceXSDPreview})
				if err != nil || report.Conformance != validation.ConformanceViolated || len(report.Findings) == 0 {
					t.Fatalf("independent report: %+v %v", report, err)
				}
				if report.Findings[0].Location.Path == "caller-mutated" {
					t.Fatal("report storage aliased another call")
				}
				report.Findings[0].Location.Path = "caller-mutated"
				report.Profile.Required[0].ID = "caller-mutated"
			}
		})
	}
}

func TestSourceAssessmentSyntaxUnderSkippedContent(t *testing.T) {
	for _, input := range []string{
		`<score-partwise><unknown><p:child/></unknown></score-partwise>`,
		`<score-partwise><f:unknown xmlns:f="urn:f"><x a="1"a="2"/></f:unknown></score-partwise>`,
		`<score-partwise><unknown><![CDATA[missing-end</unknown></score-partwise>`,
	} {
		for _, strict := range []bool{false, true} {
			options := DecodeAssessmentOptions{}
			if strict {
				options = strictSourceOptions()
			}
			result, err := DecodeAssessed(context.Background(), strings.NewReader(input), options)
			var syntax *xml.SyntaxError
			if !errors.As(err, &syntax) || result.Document != nil {
				t.Errorf("skipped source escaped XML checks: strict=%t %+v %v", strict, result, err)
			}
		}
	}
	result, err := DecodeAssessed(context.Background(), strings.NewReader(`<score-partwise><𐀀/></score-partwise>`), DecodeAssessmentOptions{})
	if err != nil || result.Document == nil {
		t.Errorf("valid Fifth Edition skipped name rejected: %+v %v", result, err)
	}
}

func TestSourceAssessmentDiagnosticOriginalPosition(t *testing.T) {
	text := strings.Replace(assessedScore("1", ""), "<staves>", "\r\n<staves rubbish=\"x\">", 1)
	prefix := text[:strings.Index(text, "rubbish")]
	variants := []struct {
		name   string
		input  []byte
		offset int64
	}{
		{"utf8-bom", append([]byte{0xef, 0xbb, 0xbf}, []byte(text)...), int64(3 + len(prefix))},
		{"utf16be", encodeUTF16(text, binary.BigEndian), int64(2 + 2*len(utf16.Encode([]rune(prefix))))},
		{"utf16le", encodeUTF16(text, binary.LittleEndian), int64(2 + 2*len(utf16.Encode([]rune(prefix))))},
	}
	for _, tt := range variants {
		t.Run(tt.name, func(t *testing.T) {
			report, err := InspectXML(context.Background(), bytes.NewReader(tt.input), AssessmentOptions{Profile: ProfileSourceXSDPreview})
			if err != nil || report.Conformance != validation.ConformanceViolated {
				t.Fatalf("invalid attribute not assessed: %+v %v", report, err)
			}
			found := false
			for _, finding := range report.Findings {
				if strings.HasSuffix(finding.Location.Path, "/@rubbish") {
					found = true
					p := finding.Location.Position
					if p == nil || p.CoordinateSpace != "original-bytes" || p.ByteOffset != tt.offset || p.Line != 2 || p.Column != 9 {
						t.Errorf("diagnostic position=%+v want byte%d line2 column9", p, tt.offset)
					}
				}
			}
			if !found {
				t.Errorf("physical attribute diagnostic missing: %+v", report.Findings)
			}
		})
	}
}
