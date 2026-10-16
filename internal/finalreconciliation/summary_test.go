package finalreconciliation

import "testing"

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
