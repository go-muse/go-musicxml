package musicxml

import (
	"crypto/sha256"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// This ledger protects original scope and evidence provenance. Its counts do
// not measure executable conformance coverage or promote planning statuses.
func TestContextOwnershipPreservesOriginalScope(t *testing.T) {
	ledger := timelineOwnershipJSON(t, "docs/validation/context-assessment-ownership.json")
	original := timelineOwnershipJSON(t, "docs/validation/plan/context-dispositions.json")
	if ledger["format"] != "musicxml-context-ownership-v1" {
		t.Fatal("unknown ownership format")
	}
	checks := timelineOwnershipRows(t, ledger["context_checks"], "check_id")
	source := timelineOwnershipRows(t, original["checks"], "id")
	if len(checks) != 307 || len(source) != 307 {
		t.Fatalf("context check inventory narrowed: %d/%d", len(checks), len(source))
	}
	for id, want := range source {
		got, ok := checks[id]
		if !ok {
			t.Errorf("missing original check %s", id)
			continue
		}
		if got["original_role"] != want["role"] || !reflect.DeepEqual(got["research_ids"], want["research_ids"]) {
			t.Errorf("original identity changed for %s", id)
		}
		if !timelineOwnershipNonempty(got["source_contract"]) || !timelineOwnershipNonempty(got["workstream"]) {
			t.Errorf("missing ownership/provenance for %s", id)
		}
	}
	if !reflect.DeepEqual(ledger["retained_context_dispositions"], original["dispositions"]) {
		t.Fatal("291 original dispositions changed")
	}
	for _, v := range []struct {
		key, id string
		count   int
	}{{"retained_requirements", "id", 95}, {"retained_issue_gates", "id", 28}, {"retained_split_checks", "id", 20}, {"retained_context_dispositions", "research_id", 291}} {
		if n := len(timelineOwnershipRows(t, ledger[v.key], v.id)); n != v.count {
			t.Errorf("%s=%d want%d", v.key, n, v.count)
		}
	}
	hashes, ok := ledger["source_hashes"].(map[string]any)
	if !ok || len(hashes) < 4 {
		t.Fatal("missing authoritative source hashes")
	}
	for path, want := range hashes {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Errorf("source %s changed: %s", path, got)
		}
	}
}

func TestContextTranslationsPreserveFidelityBoundary(t *testing.T) {
	ledger := timelineOwnershipJSON(t, "docs/validation/context-assessment-ownership.json")
	meta := ledger["linguistic_review"].(map[string]any)
	path := meta["artifact"].(string)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != meta["artifact_sha256"] {
		t.Fatal("reviewed translation artifact drift")
	}
	translations := timelineOwnershipJSON(t, path)
	records := timelineOwnershipRows(t, translations["records"], "research_id")
	if len(records) != 177 {
		t.Fatalf("reviewed triplets=%d", len(records))
	}
	research := timelineOwnershipRows(t, timelineOwnershipJSON(t, "docs/validation/plan/research-map.json")["research_records"], "research_id")
	for id, row := range records {
		if row["registry_pointer"] != research[id]["registry_pointer"] {
			t.Errorf("source pointer changed: %s", id)
		}
		if row["review_status"] != "field-for-field linguistic fidelity reviewed; no normative, implementation or closure status promotion" {
			t.Errorf("review boundary lost: %s", id)
		}
		ru, ok := row["source_triplet_ru"].(map[string]any)
		if !ok {
			t.Fatalf("missing originals: %s", id)
		}
		en, ok := row["reviewed_translation_en"].(map[string]any)
		if !ok {
			t.Fatalf("missing translation: %s", id)
		}
		for _, field := range []string{"description", "short_explanation", "conditions"} {
			if !timelineOwnershipNonempty(ru[field+"_ru"]) || !timelineOwnershipNonempty(en[field+"_en"]) {
				t.Errorf("missing triplet field: %s/%s", id, field)
			}
		}
	}
	prior := timelineOwnershipRows(t, timelineOwnershipJSON(t, "docs/validation/timeline-assessment-ownership.json")["source_records"], "research_id")
	original := timelineOwnershipRows(t, timelineOwnershipJSON(t, "docs/validation/plan/context-dispositions.json")["dispositions"], "research_id")
	for id := range original {
		if records[id] == nil && prior[id] == nil {
			t.Errorf("context source lacks a retained English triplet: %s", id)
		}
	}
}

// These links identify concrete contribution records, not a numerical coverage
// claim. Untouched later-owner rows must never masquerade as external blockers
// for an unimplemented batch7 contribution.
func TestContextOwnershipEvidenceLinksResolve(t *testing.T) {
	ledger := timelineOwnershipJSON(t, "docs/validation/context-assessment-ownership.json")
	cache := map[string]map[string]any{}
	for _, raw := range ledger["context_checks"].([]any) {
		row := raw.(map[string]any)
		evidence := row["evidence"].([]any)
		if len(evidence) == 0 {
			for _, owner := range row["retained_owner_batches"].([]any) {
				if owner.(float64) == 7 {
					t.Errorf("unmapped batch7 row %s", row["check_id"])
				}
			}
		}
		for _, item := range evidence {
			link := item.(map[string]any)
			path := link["artifact"].(string)
			doc := cache[path]
			if doc == nil {
				doc = timelineOwnershipJSON(t, path)
				cache[path] = doc
			}
			var value any = doc
			for _, part := range strings.Split(strings.TrimPrefix(link["pointer"].(string), "#/"), "/") {
				switch node := value.(type) {
				case map[string]any:
					value = node[part]
				case []any:
					index, err := strconv.Atoi(part)
					if err != nil || index < 0 || index >= len(node) {
						t.Fatalf("invalid evidence pointer %v", link)
					}
					value = node[index]
				default:
					t.Fatalf("unresolved evidence pointer %v", link)
				}
			}
			record, ok := value.(map[string]any)
			if !ok {
				t.Fatalf("evidence is not record: %v", link)
			}
			id := record["check_id"]
			if id == nil {
				id = record["id"]
			}
			if id != row["check_id"] {
				t.Errorf("evidence joined wrong original check: %v versus %v", id, row["check_id"])
			}
		}
	}
	for _, raw := range ledger["retained_issue_gates"].([]any) {
		row := raw.(map[string]any)
		if row["hard_error_basis"] != false {
			t.Errorf("issue became hard-error authority: %s", row["id"])
		}
		for _, owner := range row["remaining_batches"].([]any) {
			if owner.(float64) == 7 && row["batch7_reconciliation"] == nil {
				t.Errorf("missing gate reconciliation: %s", row["id"])
			}
		}
	}
}
