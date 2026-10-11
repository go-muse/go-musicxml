package musicxml

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// The packaging ledger carries original acceptance/five-axis contracts; it is
// not allowed to quietly shrink the already accepted engineering program.
func TestSourceAssessmentOwnershipPreservesProgram(t *testing.T) {
	read := func(path string) map[string]any {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	plan := read("docs/validation/plan/implementation-plan.json")
	questions := read("docs/validation/plan/open-questions.json")
	ledger := read("docs/validation/source-assessment-ownership.json")
	original := map[string]map[string]any{}
	tests := map[string]any{}
	for _, value := range plan["requirements"].([]any) {
		r := value.(map[string]any)
		original[r["id"].(string)] = r
	}
	for _, value := range plan["test_contracts"].([]any) {
		r := value.(map[string]any)
		tests[r["id"].(string)] = r
	}
	requirements := ledger["requirements"].([]any)
	if len(original) != 95 || len(requirements) != 95 {
		t.Fatalf("requirements original=%d ledger=%d", len(original), len(requirements))
	}
	seen := map[string]bool{}
	for _, value := range requirements {
		r := value.(map[string]any)
		id := r["id"].(string)
		base, exists := original[id]
		if !exists || seen[id] {
			t.Fatalf("unexpected/duplicate requirement %s", id)
		}
		seen[id] = true
		if r["planning_status"] != base["status"] || r["stage"] != base["stage"] || !reflect.DeepEqual(r["acceptance"], base["acceptance"]) || !reflect.DeepEqual(r["test_contract_ids"], base["test_contracts"]) {
			t.Fatalf("original contract changed for %s", id)
		}
		copies := r["test_contracts"].([]any)
		if len(copies) != len(base["test_contracts"].([]any)) {
			t.Fatalf("missing five-axis contract for %s", id)
		}
		for _, copy := range copies {
			v := copy.(map[string]any)
			if !reflect.DeepEqual(copy, tests[v["id"].(string)]) {
				t.Fatalf("test contract changed for %s", id)
			}
		}
		if (len(r["remaining_batches"].([]any)) == 0 && r["planning_status"] != "optional") || r["batch4_scope"] == "" {
			t.Fatalf("missing exact ownership for %s", id)
		}
	}
	gates := ledger["issue_gates"].([]any)
	source := questions["open_questions"].([]any)
	if len(gates) != 28 || len(source) != 28 {
		t.Fatalf("issue gates source=%d ledger=%d", len(source), len(gates))
	}
	known := map[string]map[string]any{}
	for _, value := range source {
		r := value.(map[string]any)
		known[r["id"].(string)] = r
	}
	seen = map[string]bool{}
	for _, value := range gates {
		r := value.(map[string]any)
		id := r["id"].(string)
		base, exists := known[id]
		if !exists || seen[id] {
			t.Fatalf("unexpected/duplicate gate %s", id)
		}
		seen[id] = true
		if r["disposition"] != base["disposition"] || r["hard_error_basis"] != base["hard_error_basis"] || !reflect.DeepEqual(r["requirements"], base["requirements"]) {
			t.Fatalf("issue gate changed for %s", id)
		}
	}
	if len(ledger["source_contracts"].([]any)) != 42 {
		t.Fatal("expanded source contract omitted")
	}
}
