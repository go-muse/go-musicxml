package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/go-muse/go-musicxml/validation"
)

// The first input is the exact minimized CI corpus 08f45434a0f307bc. Its
// original-byte SHA256 is b09ab738d312e5bde61c6fae4e60c16314e558aa3e4f606d5734d74705822456.
func TestSourceTruncatedMarkupTerminal(t *testing.T) {
	for _, internalDTD := range []bool{false, true} {
		name := "legacy"
		if internalDTD {
			name = "internal-dtd"
		}
		t.Run(name, func(t *testing.T) {
			for _, source := range []string{
				`<?A`, `<?A `, `<?xml `, `<?xml version="1.0"`,
				`<?xml version="1.0" `, `<r><?A`, `<r/><?A`,
			} {
				for encoding, data := range map[string][]byte{
					"utf8":     []byte(source),
					"utf8-bom": append([]byte{0xef, 0xbb, 0xbf}, []byte(source)...),
					"utf16le":  encodeUTF16(source, binary.LittleEndian),
					"utf16be":  encodeUTF16(source, binary.BigEndian),
				} {
					for _, chunked := range []bool{false, true} {
						name := source + "/" + encoding
						if chunked {
							name += "/one-byte"
						}
						t.Run(name, func(t *testing.T) {
							var input io.Reader = bytes.NewReader(data)
							if chunked {
								input = iotest.OneByteReader(input)
							}
							a, err := validation.NewBudgetAccount(validation.Budgets{})
							if err != nil {
								t.Fatal(err)
							}
							r, err := newSourceXMLTokenReader(input, sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: internalDTD})
							if err != nil {
								t.Fatal(err)
							}
							defer r.Close()
							for err == nil {
								_, err = r.Token()
							}
							var syntax *xml.SyntaxError
							if !errors.As(err, &syntax) || errors.Is(err, io.EOF) || errors.Is(err, ErrEmptyDocument) {
								t.Errorf("truncated markup lost its syntax cause: %T: %v", err, err)
							}
							s := r.Summary()
							if s.InputComplete || s.DigestComplete || s.SyntaxComplete || s.BytesRead != int64(len(data)) || s.BytesConsumed != int64(len(data)) || s.ReadPrefixSHA256 != sha256.Sum256(data) {
								t.Errorf("truncated source summary=%+v, original length=%d", s, len(data))
							}
							if invariant := sourceDTDFrameFuzzInvariant(string(data), s, err, r.entity != nil, len(r.frames)); invariant != nil {
								t.Error(invariant)
							}
							if _, repeated := r.Token(); repeated != err || r.Summary() != s {
								t.Error("terminal error or original-byte summary changed on repeated Token")
							}
							if err := r.Close(); err != nil {
								t.Fatal(err)
							}
							if used, err := a.Usage(validation.BudgetBufferedBytes); err != nil || used != 0 {
								t.Errorf("retained bytes=%d error=%v", used, err)
							}
						})
					}
				}
			}
		})
	}
}

func TestSourceTruncatedMarkupPublic(t *testing.T) {
	for _, source := range []string{
		`<?A`, `<?A `, `<?xml `, `<?xml version="1.0"`,
		`<?xml version="1.0" `, assessedScore("1", "") + `<?A`,
	} {
		for _, api := range []struct {
			name    string
			profile validation.ProfileID
		}{
			{"decode", ""}, {"decode-assessed-default", ""},
			{"inspect-preview", ProfileSourceXSDPreview.ID()}, {"decode-source-line", ProfileSourceLine.ID()}, {"inspect-semantics", ProfileSourceSemantics.ID()},
			{"inspect-internal-dtd", ProfileSourceInternalDTD.ID()},
		} {
			t.Run(api.name+"/"+source, func(t *testing.T) {
				input := &onceEOFReader{input: source}
				var report validation.Report
				var err error
				if api.name == "decode" {
					var document Document
					document, err = Decode(input)
					if document != nil {
						t.Error("truncated source exposed a document")
					}
				} else if strings.HasPrefix(api.name, "inspect") {
					report, err = InspectXML(context.Background(), input, SourceOptions{Profile: SourceProfile(api.profile)})
				} else {
					options := DecodeAssessmentOptions{}
					if api.profile != "" {
						options.Assessment = &SourceOptions{Profile: SourceProfile(api.profile)}
					}
					result, decodeErr := DecodeAssessed(context.Background(), input, options)
					report, err = result.SourceLine, decodeErr
					if result.Document != nil {
						t.Error("truncated source exposed a document")
					}
				}
				var syntax *xml.SyntaxError
				if !errors.As(err, &syntax) || errors.Is(err, io.EOF) || errors.Is(err, ErrEmptyDocument) {
					t.Errorf("truncated markup lost its syntax cause: %v", err)
				}
				if api.name != "decode" {
					if !errors.Is(err, ErrIncompleteAssessment) || report.Syntax != validation.SyntaxMalformed || report.InputComplete || report.AssessmentComplete || report.Satisfied() || report.Terminal == nil || report.Terminal.Category != validation.CategorySyntax || report.Terminal.Reason != "malformed-source" {
						t.Errorf("error=%v syntax=%s inputComplete=%t assessmentComplete=%t terminal=%+v", err, report.Syntax, report.InputComplete, report.AssessmentComplete, report.Terminal)
					}
				}
				if input.offset != len(source) || input.afterEOF != 0 {
					t.Errorf("input replayed or read after EOF: %+v", input)
				}
			})
		}
	}
}

func TestSourceTruncatedMarkupNonEOFCause(t *testing.T) {
	for _, tc := range []struct {
		name     string
		terminal error
		category validation.DiagnosticCategory
		reason   validation.Code
	}{
		{"unexpected-eof", io.ErrUnexpectedEOF, validation.CategoryCapability, "assessment-interrupted"},
		{"read-failure", errors.New("source read failed"), validation.CategoryCapability, "assessment-interrupted"},
		{"canceled", context.Canceled, validation.CategoryResource, "canceled"},
		{"joined-canceled", errors.Join(io.EOF, context.Canceled), validation.CategoryResource, "canceled"},
		{"joined-resource", errors.Join(io.EOF, &validation.ResourceError{Kind: validation.BudgetInputBytes, Limit: 3, Used: 3, Requested: 1}), validation.CategoryResource, "resource-limit"},
	} {
		for _, profile := range []validation.ProfileID{ProfileSourceXSDPreview.ID(), ProfileSourceInternalDTD.ID(), ProfileSourceSemantics.ID(), ProfileSourceLine.ID()} {
			for _, decode := range []bool{false, true} {
				if decode && profile != ProfileSourceLine.ID() {
					continue // Narrow profiles remain inspection-only contracts.
				}
				name := tc.name + "/" + string(profile) + "/inspect"
				if decode {
					name = tc.name + "/" + string(profile) + "/decode"
				}
				t.Run(name, func(t *testing.T) {
					input := &dtdAuditFinalReader{data: []byte(`<?A`), final: tc.terminal}
					options := SourceOptions{Profile: SourceProfile(profile)}
					var report validation.Report
					var err error
					if decode {
						result, decodeErr := DecodeAssessed(context.Background(), input, DecodeAssessmentOptions{Assessment: &options})
						report, err = result.SourceLine, decodeErr
						if result.Document != nil || result.Conversion.State != validation.ConversionIncomplete {
							t.Errorf("failed source conversion: document=%T state=%s", result.Document, result.Conversion.State)
						}
					} else {
						report, err = InspectXML(context.Background(), input, options)
					}
					var syntax *xml.SyntaxError
					if !errors.Is(err, tc.terminal) || errors.As(err, &syntax) || report.Syntax != validation.SyntaxIncomplete || report.InputComplete || report.Terminal == nil || report.Terminal.Category != tc.category || report.Terminal.Reason != tc.reason {
						t.Errorf("non-EOF cause reclassified: error=%v syntax=%s terminal=%+v", err, report.Syntax, report.Terminal)
					}
				})
			}
		}
	}
}

func TestSourceCompletedInstructionEOF(t *testing.T) {
	for _, internalDTD := range []bool{false, true} {
		name := "legacy"
		if internalDTD {
			name = "internal-dtd"
		}
		t.Run(name, func(t *testing.T) {
			for _, source := range []string{
				`<?A?><r/>`, `<?A ?><r/>`, `<?xml version="1.0"?><r/>`, `<r/><?A?>`,
				`<!DOCTYPE r SYSTEM 'unread.dtd'><r/><?A?>`,
			} {
				t.Run(source, func(t *testing.T) {
					a, err := validation.NewBudgetAccount(validation.Budgets{})
					if err != nil {
						t.Fatal(err)
					}
					r, err := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: internalDTD})
					if err != nil {
						t.Fatal(err)
					}
					defer r.Close()
					for err == nil {
						_, err = r.Token()
					}
					s := r.Summary()
					if err != io.EOF || !s.InputComplete || !s.DigestComplete {
						t.Errorf("completed input changed: error=%v summary=%+v", err, s)
					}
					if invariant := sourceDTDFrameFuzzInvariant(source, s, err, r.entity != nil, len(r.frames)); invariant != nil {
						t.Error(invariant)
					}
				})
			}
		})
	}
}
