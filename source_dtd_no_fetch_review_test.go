package musicxml

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
					result, err = DecodeAssessed(context.Background(), bytes.NewReader(input), DecodeAssessmentOptions{Assessment: &SourceOptions{Profile: ProfileSourceLine}})
					report = result.SourceLine
				} else {
					report, err = InspectXML(context.Background(), bytes.NewReader(input), SourceOptions{Profile: ProfileSourceInternalDTD})
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

// officialDTDCapacity is a configured capacity at which the two larger
// official scores complete source and model assessment with their bundled
// external DTD; they also complete with the default budgets.
var officialDTDCapacity = validation.Budgets{MaxBufferedBytes: 256 << 20, MaxRuleSteps: 100_000_000}

func TestSourceInternalDTDOfficialDOCTYPEsResolveBundledDTD(t *testing.T) {
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
			budgets := officialDTDCapacity
			if name == "Chant.musicxml" {
				budgets = validation.Budgets{}
			}
			report, err := InspectXML(context.Background(), bytes.NewReader(input), SourceOptions{Profile: ProfileSourceInternalDTD, Budgets: budgets})
			if err != nil || !report.InputComplete || !report.AssessmentComplete || !report.Satisfied() {
				t.Fatalf("bundled external DTD inspection: err=%v syntax=%s complete=%t satisfied=%t counts=%+v", err, report.Syntax, report.AssessmentComplete, report.Satisfied(), report.Counts)
			}
			for _, f := range report.Findings {
				if strings.Contains(string(f.Reason), "dtd-infoset-unavailable") {
					t.Fatalf("resolved external DTD still reported unavailable: %+v", f)
				}
			}
			var result DecodeResult
			if name == "Chant.musicxml" {
				result, err = DecodeAssessed(context.Background(), bytes.NewReader(input), strictSourceOptions())
			} else {
				result, err = decodeOfficialWithDeclaredCapacityControl(t, input, name)
			}
			if err != nil || result.Document == nil || !result.SourceLine.Satisfied() || !result.ModelLine.Satisfied() || result.Conversion.State != validation.ConversionConverted {
				t.Fatalf("strict loading with the bundled external DTD: document=%T source=%t model=%t conversion=%s error=%v", result.Document, result.SourceLine.Satisfied(), result.ModelLine.Satisfied(), result.Conversion.State, err)
			}
		})
	}
}

// Both larger official scores load with the default budgets once their
// external DTD is read, although source and model assessment then do their
// full work: the default rule-step limit grows with the document. Until it did,
// line 3 reached the fixed 20 million steps. The configured capacity is checked
// as well.
func decodeOfficialWithDeclaredCapacityControl(t *testing.T, input []byte, name string) (DecodeResult, error) {
	t.Helper()
	result, err := DecodeAssessed(context.Background(), bytes.NewReader(input), strictSourceOptions())
	if err != nil || result.Document == nil || !result.SourceLine.Satisfied() || !result.ModelLine.Satisfied() {
		t.Fatalf("original %s with default budgets: document=%T source=%t model=%t error=%v", name, result.Document, result.SourceLine.Satisfied(), result.ModelLine.Satisfied(), err)
	}
	options := strictSourceOptions()
	options.Assessment.Budgets = officialDTDCapacity
	return DecodeAssessed(context.Background(), bytes.NewReader(input), options)
}
