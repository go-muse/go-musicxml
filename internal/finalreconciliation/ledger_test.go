// Package finalreconciliation checks only the additive final-readiness ledger.
// These are reference/inventory guards, not runtime conformance or execution proof.
package finalreconciliation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ledgerPath = "docs/validation/final-requirement-reconciliation.json"

func loadLedger(t *testing.T) (string, map[string]any) {
	t.Helper()
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, ledgerPath))
	if err != nil {
		t.Fatal(err)
	}
	var ledger map[string]any
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatal(err)
	}
	return root, ledger
}

func TestFinalRequirementReconciliation(t *testing.T) {
	root, ledger := loadLedger(t)
	if err := validateLedger(root, ledger); err != nil {
		t.Fatal(err)
	}
}

func TestFinalReconciliationRejectsUnsupportedClaims(t *testing.T) {
	tests := []struct {
		name, want string
		mutate     func(map[string]any)
	}{
		{"missing requirement", "requirement identity set", func(m map[string]any) { m["requirements"] = m["requirements"].([]any)[1:] }},
		{"duplicate requirement", "duplicate", func(m map[string]any) { rows := m["requirements"].([]any); rows[1] = rows[0] }},
		{"missing test contract", "test contract identity set", func(m map[string]any) { m["test_contracts"] = m["test_contracts"].([]any)[1:] }},
		{"duplicate test contract", "duplicate", func(m map[string]any) { rows := m["test_contracts"].([]any); rows[1] = rows[0] }},
		{"changed original status", "original requirement", func(m map[string]any) { first(m, "requirements")["original"].(map[string]any)["status"] = "complete" }},
		{"changed original axis", "original test contract", func(m map[string]any) { first(m, "test_contracts")["original"].(map[string]any)["status"] = "complete" }},
		{"broken original pointer", "reference", func(m map[string]any) {
			first(m, "requirements")["original_ref"] = "docs/validation/plan/implementation-plan.json#/requirements/99999"
		}},
		{"wrong inherited requirement", "inherited reference", func(m map[string]any) {
			first(m, "requirements")["inherited_refs"].([]any)[0] = "docs/validation/source-assessment-ownership.json#/requirements/1"
		}},
		{"missing linked collection", "linked inventory", func(m map[string]any) { m["linked_inventories"] = m["linked_inventories"].([]any)[1:] }},
		{"broken route", "test function", func(m map[string]any) { first(m, "cases")["function"] = "TestNonexistentFinalLedgerCase" }},
		{"broken case anchor", "anchor", func(m map[string]any) {
			first(m, "cases")["anchors"].([]any)[0].(map[string]any)["text"] = "MISSING_CONCRETE_FIXTURE_6260"
		}},
		{"unknown case reference", "unknown case", func(m map[string]any) {
			first(m, "test_contracts")["axes"].(map[string]any)["positive"].(map[string]any)["case_ids"] = []any{"CASE-NONEXISTENT"}
		}},
		{"missing axis", "five case axes", func(m map[string]any) { delete(first(m, "test_contracts")["axes"].(map[string]any), "interaction") }},
		{"unsupported closure", "unsupported closure", func(m map[string]any) { first(m, "requirements")["classification"] = "implemented-and-verified" }},
		{"blanket completion", "classification", func(m map[string]any) { first(m, "requirements")["classification"] = "complete" }},
		{"missing issue", "issue identity set", func(m map[string]any) { m["issues"] = m["issues"].([]any)[1:] }},
		{"duplicate candidate", "duplicate", func(m map[string]any) { rows := m["candidates"].([]any); rows[1] = rows[0] }},
		{"translation silently retired", "translation disposition", func(m map[string]any) {
			for _, raw := range m["translations"].([]any) {
				r := raw.(map[string]any)
				if r["disposition"] == "missing-portable-evidence" {
					r["disposition"] = "complete"
					break
				}
			}
		}},
		{"performance silently closed", "B5-PERF-01", func(m map[string]any) { m["retained_obligations"].([]any)[0].(map[string]any)["state"] = "closed" }},
		{"invented linked inventory ID", "linked inventory", func(m map[string]any) { first(m, "linked_inventories")["ids"].([]any)[0] = "UNKNOWN-LINKED-ID" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, ledger := loadLedger(t)
			test.mutate(ledger)
			err := validateLedger(root, ledger)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("wanted rejection containing %q; got %v", test.want, err)
			}
		})
	}
}

func first(m map[string]any, key string) map[string]any { return m[key].([]any)[0].(map[string]any) }

func TestFinalReconciliationRejectsUnlinkedContracts(t *testing.T) {
	for _, test := range []struct{ name, collection, key, want string }{
		{"requirement without test", "requirements", "test_contract_ids", "lost test contract links"},
		{"test without requirement", "test_contracts", "requirement_ids", "changed requirement links"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, ledger := loadLedger(t)
			first(ledger, test.collection)[test.key] = []any{}
			err := validateLedger(root, ledger)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("unlinked contract accepted or wrong rejection: %v", err)
			}
		})
	}
}

func TestFinalReconciliationRejectsHistoricalExecutionPromotion(t *testing.T) {
	root, ledger := loadLedger(t)
	row := first(ledger, "requirements")
	row["classification"] = "implemented-and-verified"
	// Existing named cases and historic receipt links are still present. They
	// cannot promote an unexecuted final-tree obligation into verified coverage.
	err := validateLedger(root, ledger)
	if err == nil || !strings.Contains(err.Error(), "unsupported closure") {
		t.Fatalf("historical evidence promoted into final verification: %v", err)
	}
}
