package finalreconciliation

import (
	"fmt"
	"strings"
)

// This reviewed migration supersedes two concrete original opt-in clauses.
// Pins preserve edited data, but cannot turn a contrary current test into
// evidence for those clauses. Keep that semantic exclusion independently.
func validateAssessedLoadingSupersession(tests, cases map[string]map[string]any) error {
	const contractID = "TEST-DEF-ADAPT-OPTIN"
	const routeID = "TestDecodeAssessedPermissiveAndStrictSourceAreSeparate"
	const decision = "docs/validation/assessed-loading-pipeline.md"
	axisMap, _ := tests[contractID]["axes"].(map[string]any)
	for _, name := range []string{"boundary", "missing_fact"} {
		axis, _ := axisMap[name].(map[string]any)
		status := stringField(axis, "assessment")
		if (status != "decision-only" && status != "partial") || stringField(axis, "gap_kind") != "superseded-api-contract" || !strings.Contains(stringField(axis, "gap"), decision) {
			return fmt.Errorf("superseded opt-in axis %s/%s requires an explicit migration gap, not mapped opposite-behavior evidence", contractID, name)
		}
	}
	if stringField(cases[routeID], "evidence_layer") != "pr46-assessed-loading" {
		return fmt.Errorf("current assessed-loading route %s must not be labeled baseline evidence", routeID)
	}
	return nil
}
