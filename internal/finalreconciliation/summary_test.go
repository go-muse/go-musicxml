package finalreconciliation

import (
	"strings"
	"testing"
)

func TestFinalReconciliationRejectsStaleEvidenceSummary(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		mutate     func(map[string]any)
	}{
		{"missing summary", "missing evidence_summary", func(m map[string]any) { delete(m, "evidence_summary") }},
		{"invented routes", "evidence_summary.anchored_routes", func(m map[string]any) { m["evidence_summary"].(map[string]any)["anchored_routes"] = float64(200) }},
		{"invented layer counts", "evidence_summary.routes_by_evidence_layer", func(m map[string]any) {
			m["evidence_summary"].(map[string]any)["routes_by_evidence_layer"] = map[string]any{"baseline": float64(127), "pr42-added": float64(0)}
		}},
		{"invented mapped and partial counts", "evidence_summary.axis_dispositions", func(m map[string]any) {
			counts := m["evidence_summary"].(map[string]any)["axis_dispositions"].(map[string]any)
			counts["mapped"] = float64(120)
			counts["partial"] = float64(0)
		}},
		{"retired missing translations", "evidence_summary.missing_portable_triplets", func(m map[string]any) {
			m["evidence_summary"].(map[string]any)["missing_portable_triplets"] = float64(0)
		}},
		{"invented execution count", "evidence_summary.individually_executed_final_requirements", func(m map[string]any) {
			m["evidence_summary"].(map[string]any)["individually_executed_final_requirements"] = float64(95)
		}},
		{"invented requirement count", "evidence_summary.requirements", func(m map[string]any) { m["evidence_summary"].(map[string]any)["requirements"] = float64(100) }},
		{"invented contract count", "evidence_summary.test_contracts", func(m map[string]any) { m["evidence_summary"].(map[string]any)["test_contracts"] = float64(100) }},
		{"invented issue count", "evidence_summary.issues", func(m map[string]any) { m["evidence_summary"].(map[string]any)["issues"] = float64(29) }},
		{"invented candidate count", "evidence_summary.candidates", func(m map[string]any) { m["evidence_summary"].(map[string]any)["candidates"] = float64(28) }},
		{"invented translation identities", "evidence_summary.translation_identities", func(m map[string]any) {
			m["evidence_summary"].(map[string]any)["translation_identities"] = float64(177)
		}},
		{"invented committed triplets", "evidence_summary.committed_reviewed_triplets", func(m map[string]any) {
			m["evidence_summary"].(map[string]any)["committed_reviewed_triplets"] = float64(291)
		}},
		{"invented classifications", "evidence_summary.requirement_classifications", func(m map[string]any) {
			m["evidence_summary"].(map[string]any)["requirement_classifications"] = map[string]any{"implemented-and-verified": float64(95)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) { root, m := loadLedger(t); tc.mutate(m); rejectLedger(t, root, m, tc.want) })
	}
}

// These synthetic changes deliberately bypass policy validation. Stale counts
// must fail, and corrected counts must pass without changing any policy pin.
// They neither modify the committed ledger nor authorize evidence promotion.
func TestFinalReconciliationSummaryDerivesChangedRows(t *testing.T) {
	for _, tc := range []struct {
		name, key     string
		changeRows    func(map[string]any)
		correctCounts func(map[string]any)
	}{
		{"requirement inventory", "requirements", func(m map[string]any) { appendSummaryRow(m, "requirements") }, func(s map[string]any) {
			adjustSummaryCount(s, "requirements", 1)
			adjustSummaryCount(s["requirement_classifications"].(map[string]any), "implemented-awaiting-current-verification", 1)
		}},
		{"contract inventory", "test_contracts", func(m map[string]any) { appendSummaryRow(m, "test_contracts") }, func(s map[string]any) {
			adjustSummaryCount(s, "test_contracts", 1)
			adjustSummaryCount(s["axis_dispositions"].(map[string]any), "mapped", 5)
		}},
		{"issue inventory", "issues", func(m map[string]any) { appendSummaryRow(m, "issues") }, func(s map[string]any) { adjustSummaryCount(s, "issues", 1) }},
		{"candidate inventory", "candidates", func(m map[string]any) { appendSummaryRow(m, "candidates") }, func(s map[string]any) { adjustSummaryCount(s, "candidates", 1) }},
		{"translation inventory", "translation_identities", func(m map[string]any) { appendSummaryRow(m, "translations") }, func(s map[string]any) {
			adjustSummaryCount(s, "translation_identities", 1)
			adjustSummaryCount(s, "missing_portable_triplets", 1)
		}},
		{"route inventory", "anchored_routes", func(m map[string]any) { appendSummaryRow(m, "cases") }, func(s map[string]any) {
			adjustSummaryCount(s, "anchored_routes", 1)
			adjustSummaryCount(s["routes_by_evidence_layer"].(map[string]any), "baseline", 1)
		}},
		{"route layer", "routes_by_evidence_layer", func(m map[string]any) { first(m, "cases")["evidence_layer"] = "pr42-added" }, func(s map[string]any) {
			counts := s["routes_by_evidence_layer"].(map[string]any)
			adjustSummaryCount(counts, "baseline", -1)
			adjustSummaryCount(counts, "pr42-added", 1)
		}},
		{"axis disposition", "axis_dispositions", func(m map[string]any) {
			first(m, "test_contracts")["axes"].(map[string]any)["positive"].(map[string]any)["assessment"] = "partial"
		}, func(s map[string]any) {
			counts := s["axis_dispositions"].(map[string]any)
			adjustSummaryCount(counts, "mapped", -1)
			adjustSummaryCount(counts, "partial", 1)
		}},
		{"translation disposition", "committed_reviewed_triplets", func(m map[string]any) {
			rowWith(m, "translations", "disposition", "missing-portable-evidence")["disposition"] = "committed-reviewed-triplet"
		}, func(s map[string]any) {
			adjustSummaryCount(s, "committed_reviewed_triplets", 1)
			adjustSummaryCount(s, "missing_portable_triplets", -1)
		}},
		{"requirement classification", "requirement_classifications", func(m map[string]any) { first(m, "requirements")["classification"] = "explicit-bounded-limitation" }, func(s map[string]any) {
			counts := s["requirement_classifications"].(map[string]any)
			adjustSummaryCount(counts, "implemented-awaiting-current-verification", -1)
			adjustSummaryCount(counts, "explicit-bounded-limitation", 1)
		}},
		{"execution records", "individually_executed_final_requirements", func(m map[string]any) {
			first(m, "requirements")["execution"].(map[string]any)["state"] = "executed-on-final-tree"
		}, func(s map[string]any) { adjustSummaryCount(s, "individually_executed_final_requirements", 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, m := loadLedger(t)
			tc.changeRows(m)
			err := validateEvidenceSummary(m)
			if err == nil || !strings.Contains(err.Error(), "evidence_summary."+tc.key+" must match derived row counts") {
				t.Fatalf("stale %s counts accepted or wrong rejection: %v", tc.key, err)
			}
			tc.correctCounts(m["evidence_summary"].(map[string]any))
			if err := validateEvidenceSummary(m); err != nil {
				t.Fatalf("corrected counts rejected: %v", err)
			}
		})
	}
}

func appendSummaryRow(m map[string]any, collection string) {
	row := map[string]any{}
	for key, value := range first(m, collection) {
		row[key] = value
	}
	row["id"] = row["id"].(string) + "-SUMMARY-COUNT-CONTROL"
	m[collection] = append(m[collection].([]any), row)
}
func adjustSummaryCount(counts map[string]any, key string, delta float64) {
	counts[key] = counts[key].(float64) + delta
}
