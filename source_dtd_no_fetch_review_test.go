package musicxml

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourceInternalDTDPublicExternalIdentifiersStayUnread(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`<!ATTLIST opus version CDATA "4.0">`))
	}))
	defer server.Close()
	local := filepath.Join(t.TempDir(), "external.dtd")
	// A readable local resource deliberately contains malformed grammar. Reading
	// and applying it would change the result; this is a content-use control,
	// whereas the HTTP counter directly detects attempted transport access.
	if err := os.WriteFile(local, []byte(`<!ATTLIST malformed`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, subset string }{
		{"system-http", `SYSTEM "` + server.URL + `/external.dtd"`},
		{"public-http", `PUBLIC "-//MusicXML//DTD Opus 4.0//EN" "` + server.URL + `/external.dtd"`},
		{"system-local", `SYSTEM "` + filepath.ToSlash(local) + `"`},
		{"external-parameter", `[<!ENTITY % p SYSTEM "` + server.URL + `/parameter.ent">%p;]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte(`<!DOCTYPE opus ` + tc.subset + `><opus version="4.0"/>`)
			for _, decode := range []bool{false, true} {
				var report validation.Report
				var err error
				if decode {
					var result DecodeResult
					result, err = DecodeAssessed(context.Background(), bytes.NewReader(input), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: ProfileSourceSemantics}})
					report = result.Source
				} else {
					report, err = InspectXML(context.Background(), bytes.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTD})
				}
				if (!decode && err != nil) || (decode && !errors.Is(err, validation.ErrIncompleteAssessment)) || report.Syntax == validation.SyntaxMalformed || !report.InputComplete || report.Satisfied() {
					t.Fatalf("decode=%t err=%v syntax=%s complete=%t satisfied=%t", decode, err, report.Syntax, report.InputComplete, report.Satisfied())
				}
				if got := requests.Load(); got != 0 {
					t.Fatalf("external fetch attempts=%d", got)
				}
			}
		})
	}
}

func TestSourceInternalDTDOfficialDOCTYPEsRemainIncomplete(t *testing.T) {
	for _, name := range []string{"MozartTrio.musicxml", "MozaChloSample.musicxml", "Chant.musicxml"} {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "official", name))
			if err != nil {
				t.Fatal(err)
			}
			// Keep the original bytes, including the UTF-16 BOM in MozaChlo.
			if !bytes.Contains(input, []byte("DOCTYPE")) && !bytes.Contains(input, []byte{'D', 0, 'O', 0, 'C', 0, 'T', 0, 'Y', 0, 'P', 0, 'E', 0}) {
				t.Fatal("fixture lost its original DOCTYPE")
			}
			for _, decode := range []bool{false, true} {
				var report validation.Report
				if decode {
					var result DecodeResult
					if name == "MozaChloSample.musicxml" {
						result, err = decodeMozaWithDeclaredCapacityControl(t, input)
					} else {
						result, err = DecodeAssessed(context.Background(), bytes.NewReader(input), strictSourceOptions())
					}
					if result.Document != nil || result.Model.Requested || result.Conversion.State != validation.ConversionConverted {
						t.Fatalf("unaccepted DTD exposed a model or lost completed conversion: document=%T modelRequested=%t conversion=%s", result.Document, result.Model.Requested, result.Conversion.State)
					}
					report = result.Source
				} else {
					report, err = InspectXML(context.Background(), bytes.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTD})
				}
				if (!decode && err != nil) || (decode && !errors.Is(err, validation.ErrIncompleteAssessment)) || report.Syntax == validation.SyntaxMalformed || !report.InputComplete || report.Satisfied() {
					t.Fatalf("decode=%t err=%v syntax=%s complete=%t satisfied=%t", decode, err, report.Syntax, report.InputComplete, report.Satisfied())
				}
				unavailable := false
				for _, f := range report.Findings {
					if strings.Contains(string(f.Reason), "dtd-infoset-unavailable") {
						unavailable = true
					}
				}
				if !unavailable {
					t.Fatal("missing causal unread-DTD evidence")
				}
			}
		})
	}
}

// The original UTF-16 Moza sample already has a reviewed 96 MiB source-semantics
// control in TestReviewOfficialOriginalDOCTYPEBufferAndEvidence. Mandatory
// loading additionally keeps the converted candidate and both pinned dictionary
// components live. Its default boundary is asserted before this configured
// control, rather than increasing the default limit to recover the old result.
func decodeMozaWithDeclaredCapacityControl(t *testing.T, input []byte) (DecodeResult, error) {
	t.Helper()
	var result DecodeResult
	var err error
	// This frontier is an amd64 receipt, not a portable storage-size promise.
	// Smaller architectures still run the configured DTD authority control.
	if runtime.GOARCH == "amd64" {
		result, err = DecodeAssessed(context.Background(), bytes.NewReader(input), strictSourceOptions())
		var resource *validation.ResourceError
		if !errors.As(err, &resource) || resource.Kind != validation.BudgetBufferedBytes || resource.Limit != validation.DefaultBudgets().MaxBufferedBytes || resource.Used > resource.Limit || resource.Requested <= resource.Limit-resource.Used || !errors.Is(err, ErrIncompleteAssessment) || errors.Is(err, ErrInvalidDocument) {
			t.Fatalf("original Moza default capacity changed: resource=%+v error=%v", resource, err)
		}
		if result.Document != nil || result.Model.Requested || result.Conversion.State != validation.ConversionIncomplete || !result.Source.Requested || result.Source.InputComplete || result.Source.AssessmentComplete || result.Source.Syntax != validation.SyntaxIncomplete {
			t.Fatalf("default capacity refusal leaked acceptance: document=%T modelRequested=%t conversion=%s sourceRequested=%t inputComplete=%t assessmentComplete=%t syntax=%s", result.Document, result.Model.Requested, result.Conversion.State, result.Source.Requested, result.Source.InputComplete, result.Source.AssessmentComplete, result.Source.Syntax)
		}
		t.Logf("original Moza default buffered-byte checkpoint: used=%d requested=%d limit=%d", resource.Used, resource.Requested, resource.Limit)
	}
	options := strictSourceOptions()
	options.Assessment.Budgets.MaxBufferedBytes = 96 << 20
	result, err = DecodeAssessed(context.Background(), bytes.NewReader(input), options)
	if !errors.Is(err, ErrIncompleteAssessment) || errors.Is(err, ErrInvalidDocument) || errors.Is(err, validation.ErrResourceLimit) || result.Document != nil || result.Model.Requested || result.Conversion.State != validation.ConversionConverted || !result.Source.InputComplete || result.Source.AssessmentComplete || result.Source.Satisfied() {
		t.Fatalf("configured-capacity DTD control: document=%T modelRequested=%t conversion=%s inputComplete=%t assessmentComplete=%t error=%v", result.Document, result.Model.Requested, result.Conversion.State, result.Source.InputComplete, result.Source.AssessmentComplete, err)
	}
	for _, finding := range result.Source.Findings {
		if strings.Contains(string(finding.Reason), "dtd-infoset-unavailable") {
			return result, err
		}
	}
	t.Fatal("configured-capacity control lost the original unread external-DTD evidence")
	return result, err
}
