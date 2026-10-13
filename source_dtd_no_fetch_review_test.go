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
					result, err = DecodeAssessed(context.Background(), bytes.NewReader(input), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: ProfileSourceInternalDTD}})
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
					result, err = DecodeAssessed(context.Background(), bytes.NewReader(input), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: ProfileSourceInternalDTD}})
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
