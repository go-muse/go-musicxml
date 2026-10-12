package musicxml

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

// Hash the canonical text represented by a Git checkout. Preserve every byte
// except the CR in a CRLF line ending; bare CR and content edits are significant.
func contextEvidenceTextSHA256(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))))
}

func TestContextEvidenceHashesSurviveCheckoutLineEndings(t *testing.T) {
	ledger := timelineOwnershipJSON(t, "docs/validation/context-assessment-ownership.json")
	hashes := ledger["source_hashes"].(map[string]any)
	translation := ledger["linguistic_review"].(map[string]any)
	hashes[translation["artifact"].(string)] = translation["artifact_sha256"]
	for path, rawWant := range hashes {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			lf := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
			want := rawWant.(string)
			for name, input := range map[string][]byte{"LF": lf, "mixed": bytes.Replace(lf, []byte("\n"), []byte("\r\n"), 1), "CRLF": bytes.ReplaceAll(lf, []byte("\n"), []byte("\r\n"))} {
				if got := contextEvidenceTextSHA256(input); got != want {
					t.Errorf("%s canonical digest = %s, want %s", name, got, want)
				}
			}
			changed := append([]byte(nil), lf...)
			for i, c := range changed {
				if c >= 'a' && c <= 'z' {
					changed[i] = c - 'a' + 'A'
					break
				}
			}
			if contextEvidenceTextSHA256(changed) == want {
				t.Error("content mutation lost")
			}
			if contextEvidenceTextSHA256(append(append([]byte(nil), lf...), '\r')) == want {
				t.Error("bare carriage return lost")
			}
		})
	}
}

func TestContextEvidenceHashPreservesEscapedJSONWhitespace(t *testing.T) {
	lf := []byte("{\"escaped\":\"\\r\\n\"}\n")
	crlf := bytes.ReplaceAll(lf, []byte("\n"), []byte("\r\n"))
	want := fmt.Sprintf("%x", sha256.Sum256(lf))
	if got := contextEvidenceTextSHA256(crlf); got != want {
		t.Fatalf("escaped JSON bytes changed: %s", got)
	}
}

func TestContextEvidenceHashPreservesUnmatchedCRBeforeCRLF(t *testing.T) {
	if contextEvidenceTextSHA256([]byte("\r\r\n")) == contextEvidenceTextSHA256([]byte("\n")) {
		t.Fatal("unmatched CR erased by repeated normalization")
	}
}
