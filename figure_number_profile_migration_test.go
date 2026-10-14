package musicxml

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestFigureNumberHistoricalProfileSnapshotPreserved(t *testing.T) {
	data, err := os.ReadFile("testdata/validation/frozen-eight-profiles-batch8.json")
	if err != nil {
		t.Fatal(err)
	}
	const want = "cabd7e95094b7cf696fc6bc12816b5ed65ec3e7fdab58a854209599ef23260b1"
	// Repository checkout conventions permit CRLF without changing the record.
	canonical := strings.ReplaceAll(string(data), "\r\n", "\n")
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(canonical))); got != want {
		t.Fatalf("historical profile snapshot changed: %s", got)
	}
}

// The batch-8 snapshot remains historical, including the removed overconstraint.
// Permit exactly this source-backed correction for the two context profiles;
// every other report field and each earlier profile still compare unchanged.
func migrateFrozenFigureNumberReport(t *testing.T, id validation.ProfileID, value any) {
	t.Helper()
	if id != ProfileSourceContext && id != ProfileModelContext {
		return
	}
	report := value.(map[string]any)
	profile := report["Profile"].(map[string]any)
	const before = "84279f72d5bda73adca5e340d52a9ba8532df43de036c910eae6b64ad1c47866"
	const after = "c3e4d66f82ce776a6f8cfdd62295c124e352bc2ade1b0218ef70c8837c7f1506"
	if profile["CatalogDigest"] != before {
		t.Fatalf("unexpected historical context digest: %v", profile["CatalogDigest"])
	}
	profile["CatalogDigest"] = after
	const oldText = "Equal-adjacent passes, unresolved musical-content equivalence, and the unreviewed figure-number numeric grammar remain explicitly incomplete."
	const newText = "Equal-adjacent passes and unresolved musical-content equivalence remain explicitly incomplete. Figure-number retains the deliberately unrestricted style-text declaration; no additional numeric lexical requirement is selected."
	closure := profile["NormativeClosure"].(string)
	if strings.Count(closure, oldText) != 1 {
		t.Fatal("historical figure-number closure text changed")
	}
	profile["NormativeClosure"] = strings.Replace(closure, oldText, newText, 1)
	const retired = "MX40-CONTEXT-figure-number-numeric"
	for _, field := range []string{"Required", "Support"} {
		items := profile[field].([]any)
		kept := make([]any, 0, len(items)-1)
		removed := 0
		for _, item := range items {
			key := item.(map[string]any)
			if field == "Support" {
				key = key["Rule"].(map[string]any)
			}
			if key["ID"] == retired {
				if key["Revision"] != float64(1) {
					t.Fatal("unexpected historical figure-number revision")
				}
				removed++
				continue
			}
			kept = append(kept, item)
		}
		if removed != 1 {
			t.Fatalf("removed %d figure-number entries from %s", removed, field)
		}
		profile[field] = kept
	}
	// The retained Opus fixture has no figure-number; the withdrawn selected
	// rule contributed one document-level NotApplicable result and no finding.
	counts := report["Counts"].(map[string]any)
	if counts["NotApplicable"] != float64(42) {
		t.Fatalf("unexpected historical inapplicable count: %v", counts)
	}
	counts["NotApplicable"] = float64(41)
}

// The source-composition guard hashes each complete Profile, rather than only
// the catalog. Retain its historical baseline and enumerate this correction.
func migrateFigureNumberProfileHash(t *testing.T, id validation.ProfileID, historical string) string {
	t.Helper()
	var before, after string
	switch id {
	case ProfileSourceContext:
		before = "f2547f7b087b4551e47f1a4d1f1b8799cf955ec3f2cb2f56da96257dae6e41fc"
		after = "43a78f538defd0fbbc4e5331eecf41688204f12742a5900e49672128cd320155"
	case ProfileModelContext:
		before = "37a63c9325973e67ae03328df45be262101fd83b216562641770e7ddf6ca93bb"
		after = "ea6744501e0555255420471e8b8d5a0e4fb9371d915332777bfffdac77fd2511"
	default:
		return historical
	}
	if historical != before {
		t.Fatalf("unexpected historical profile hash for %s: %s", id, historical)
	}
	return after
}
