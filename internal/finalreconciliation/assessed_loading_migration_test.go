package finalreconciliation

import "testing"

// The mandatory-loader route asserts the opposite of these original opt-in
// clauses. Historical labels and reviewed hashes cannot make it their proof.
func TestFinalReconciliationRejectsSupersededOptInCoverage(t *testing.T) {
	for _, axis := range []string{"boundary", "missing_fact"} {
		t.Run(axis, func(t *testing.T) {
			root, ledger := loadLedger(t)
			contract := rowWith(ledger, "test_contracts", "id", "TEST-DEF-ADAPT-OPTIN")
			item := contract["axes"].(map[string]any)[axis].(map[string]any)
			item["assessment"] = "mapped"
			item["gap"] = ""
			item["gap_kind"] = "none"
			rejectLedger(t, root, ledger, "superseded opt-in axis")
		})
	}
}

func TestFinalReconciliationRejectsMigratedRouteAsBaseline(t *testing.T) {
	root, ledger := loadLedger(t)
	route := rowWith(ledger, "cases", "id", "TestDecodeAssessedPermissiveAndStrictSourceAreSeparate")
	route["evidence_layer"] = "baseline"
	rejectLedger(t, root, ledger, "current assessed-loading route")
}
