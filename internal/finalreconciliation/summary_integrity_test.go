package finalreconciliation

import (
	"fmt"
	"reflect"
)

// Summary arithmetic uses the rows, not the reviewed policy hashes or the
// published counts. Structural/policy validation remains a separate obligation.
func validateEvidenceSummary(ledger map[string]any) error {
	summary, ok := ledger["evidence_summary"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing evidence_summary")
	}
	collections := map[string]map[string]map[string]any{}
	for _, name := range []string{"requirements", "test_contracts", "issues", "candidates", "translations", "cases"} {
		rows, err := indexed(ledger[name], "id")
		if err != nil {
			return fmt.Errorf("evidence_summary source %s: %w", name, err)
		}
		collections[name] = rows
	}
	classifications, layers, dispositions := map[string]any{}, map[string]any{}, map[string]any{}
	increment := func(counts map[string]any, label string) { n, _ := counts[label].(float64); counts[label] = n + 1 }
	executed := 0
	for _, row := range collections["requirements"] {
		increment(classifications, stringField(row, "classification"))
		execution, _ := row["execution"].(map[string]any)
		// This counts explicit execution records only. The independent snapshot
		// execution-state guard disallows promoting this ledger to that state.
		if stringField(execution, "state") == "executed-on-final-tree" {
			executed++
		}
	}
	for _, row := range collections["cases"] {
		increment(layers, stringField(row, "evidence_layer"))
	}
	for _, row := range collections["test_contracts"] {
		axisMap, ok := row["axes"].(map[string]any)
		if !ok {
			return fmt.Errorf("evidence_summary source test contract lacks axes")
		}
		for _, axis := range axes {
			item, ok := axisMap[axis].(map[string]any)
			if !ok {
				return fmt.Errorf("evidence_summary source test contract lacks %s axis", axis)
			}
			increment(dispositions, stringField(item, "assessment"))
		}
	}
	committed, missing := 0, 0
	for _, row := range collections["translations"] {
		switch row["disposition"] {
		case "committed-reviewed-triplet":
			committed++
		case "missing-portable-evidence":
			missing++
		}
	}
	derived := map[string]any{
		"requirements":                float64(len(collections["requirements"])),
		"test_contracts":              float64(len(collections["test_contracts"])),
		"issues":                      float64(len(collections["issues"])),
		"candidates":                  float64(len(collections["candidates"])),
		"translation_identities":      float64(len(collections["translations"])),
		"committed_reviewed_triplets": float64(committed),
		"missing_portable_triplets":   float64(missing),
		"individually_executed_final_requirements": float64(executed),
		"anchored_routes":                          float64(len(collections["cases"])),
		"routes_by_evidence_layer":                 layers,
		"axis_dispositions":                        dispositions,
		"requirement_classifications":              classifications,
	}
	for _, key := range []string{"requirements", "test_contracts", "issues", "candidates", "translation_identities", "committed_reviewed_triplets", "missing_portable_triplets", "individually_executed_final_requirements", "anchored_routes", "routes_by_evidence_layer", "axis_dispositions", "requirement_classifications"} {
		actual, present := summary[key]
		if !present || !reflect.DeepEqual(actual, derived[key]) {
			return fmt.Errorf("evidence_summary.%s must match derived row counts: got %v, want %v", key, actual, derived[key])
		}
	}
	return nil
}
