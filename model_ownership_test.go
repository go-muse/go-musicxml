package musicxml

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func modelOwnershipJSON(t *testing.T, path string) map[string]any {
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

func modelOwnershipRows(t *testing.T, value any, key string) map[string]map[string]any {
	t.Helper()
	rows, ok := value.([]any)
	if !ok {
		t.Fatalf("missing row array for %s", key)
	}
	result := make(map[string]map[string]any, len(rows))
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("invalid row for %s", key)
		}
		id, ok := row[key].(string)
		if !ok || id == "" || result[id] != nil {
			t.Fatalf("missing or duplicate %s: %v", key, row[key])
		}
		result[id] = row
	}
	return result
}

// A merge-time performance acceptance does not discharge the retained program
// obligation. Keep its owner, evidence and closure criteria outside the batch's
// historical pre-acceptance checklist. This is documentation integrity only,
// not a benchmark threshold or executable performance-coverage claim.
func TestModelAssessmentRemainingPerformanceOwner(t *testing.T) {
	data, err := os.ReadFile("docs/validation/model-assessment-scope.md")
	if err != nil {
		t.Fatal(err)
	}
	remaining, ok := modelRemainingProgramSection(data)
	if !ok {
		t.Fatal("missing distinct remaining-program section")
	}
	for _, required := range []string{
		"B5-PERF-01", "shared validation kernel/session", "batch 9",
		"outstanding", "Run.Bind", "child witnesses", "recordXSDResult",
		"Open", "header copies", "cacheDecision", "27.02 ms", "100.73 ms",
		"11,705,376", "33,374,378", "123,632", "181,857",
		"matched", "reviewed", "resource", "causal",
	} {
		if !strings.Contains(remaining, required) {
			t.Errorf("remaining performance obligation lost %q", required)
		}
	}
}

func modelRemainingProgramSection(data []byte) (string, bool) {
	// Git may check Markdown out with CRLF on Windows. The heading's logical
	// line boundary, rather than a platform-specific byte spelling, matters.
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	_, remaining, ok := strings.Cut(text, "## Remaining program\n")
	return strings.Join(strings.Fields(remaining), " "), ok
}

func TestModelRemainingProgramSectionLineEndings(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		t.Run(fmt.Sprintf("newline-%q", newline), func(t *testing.T) {
			input := strings.Join([]string{"Historical acceptance", "", "## Remaining program", "", "Owned obligation", ""}, newline)
			got, ok := modelRemainingProgramSection([]byte(input))
			if !ok || got != "Owned obligation" {
				t.Fatalf("section = %q, found=%v", got, ok)
			}
		})
	}
}

// Original planning and five-axis texts are retained evidence, not statuses that
// an implementation or a successful helper test is allowed to promote.
func TestModelAssessmentOwnershipPreservesProgram(t *testing.T) {
	ledger := modelOwnershipJSON(t, "docs/validation/model-assessment-ownership.json")
	source := modelOwnershipJSON(t, "docs/validation/source-assessment-ownership.json")
	plan := modelOwnershipJSON(t, "docs/validation/plan/implementation-plan.json")
	prose := modelOwnershipJSON(t, "docs/validation/plan/prose-contracts.json")
	snapshot := modelOwnershipJSON(t, "docs/validation/plan/external-snapshot.json")
	if ledger["format"] != "musicxml-model-assessment-ownership-1" || ledger["registry_commit"] != snapshot["registry_commit"] {
		t.Fatal("ownership format or source pin changed")
	}
	if !reflect.DeepEqual(ledger["retained_counts"], source["retained_counts"]) || !reflect.DeepEqual(ledger["source_contracts"], source["source_contracts"]) {
		t.Fatal("original inventory totals or expanded source contracts changed")
	}
	requirements := modelOwnershipRows(t, ledger["requirements"], "id")
	original := modelOwnershipRows(t, plan["requirements"], "id")
	prior := modelOwnershipRows(t, source["requirements"], "id")
	plannedTests := modelOwnershipRows(t, plan["test_contracts"], "id")
	if len(requirements) != 95 || len(original) != len(requirements) || len(prior) != len(requirements) {
		t.Fatal("engineering requirement inventory changed")
	}
	compare := func(id string, got, want any) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("original contract changed: %s", id)
		}
	}
	retainsLaterOwners := func(id string, current, previous any) {
		t.Helper()
		owners, ok := current.([]any)
		if !ok {
			t.Fatalf("missing remaining owners: %s", id)
		}
		for _, old := range previous.([]any) {
			if old.(float64) < 6 {
				continue
			}
			found := false
			for _, owner := range owners {
				found = found || owner == old
			}
			if !found {
				t.Fatalf("later batch %v silently removed from %s", old, id)
			}
		}
	}
	for id, row := range requirements {
		base, exists := original[id]
		if !exists {
			t.Fatalf("unknown requirement %s", id)
		}
		for got, want := range map[string]string{"planning_status": "status", "stage": "stage", "acceptance": "acceptance", "test_contract_ids": "test_contracts"} {
			compare(id+"/"+got, row[got], base[want])
		}
		copies := modelOwnershipRows(t, row["test_contracts"], "id")
		if len(copies) != len(base["test_contracts"].([]any)) {
			t.Fatalf("missing original five-axis test contract: %s", id)
		}
		for _, raw := range base["test_contracts"].([]any) {
			name := raw.(string)
			compare(id+"/"+name, copies[name], plannedTests[name])
		}
		for got, want := range map[string]string{"previous_scope": "batch4_scope", "previous_remaining_batches": "remaining_batches", "previous_remaining_scope": "remaining_scope"} {
			compare(id+"/"+got, row[got], prior[id][want])
		}
		retainsLaterOwners(id, row["remaining_batches"], prior[id]["remaining_batches"])
		if row["batch5_scope"] == "" || row["remaining_scope"] == "" || row["evidence_scope"] == "" {
			t.Fatalf("missing scoped ownership/evidence: %s", id)
		}
		status, _ := row["implementation_status"].(string)
		if status == "" || status == "completed" || status == "implemented" {
			t.Fatalf("unqualified whole-program completion: %s", id)
		}
		if len(row["remaining_batches"].([]any)) != 0 && strings.HasPrefix(status, "implemented-") {
			t.Fatalf("broader requirement marked implemented despite retained owners: %s", id)
		}
	}
	gates := modelOwnershipRows(t, ledger["issue_gates"], "id")
	oldGates := modelOwnershipRows(t, source["issue_gates"], "id")
	if len(gates) != 28 || len(gates) != len(oldGates) {
		t.Fatal("issue gate inventory changed")
	}
	for id, row := range gates {
		base, exists := oldGates[id]
		if !exists {
			t.Fatalf("unknown gate %s", id)
		}
		for key, want := range base {
			if key == "remaining_batches" {
				compare(id+"/previous_remaining_batches", row["previous_remaining_batches"], want)
				retainsLaterOwners(id, row[key], want)
			} else {
				compare(id+"/"+key, row[key], want)
			}
		}
		if row["review_status"] != "retained-not-promoted" || row["hard_error_basis"] != false {
			t.Fatalf("issue gate promoted into rejection authority: %s", id)
		}
	}
	candidates := modelOwnershipRows(t, ledger["candidate_dispositions"], "research_id")
	checks := modelOwnershipRows(t, prose["checks"], "id")
	clauseTests := modelOwnershipRows(t, prose["test_contracts"], "id")
	candidateIDs := snapshot["candidate_prose_ids"].([]any)
	if len(candidates) != 29 || len(candidates) != len(candidateIDs) {
		t.Fatal("candidate inventory changed")
	}
	for _, raw := range candidateIDs {
		id := raw.(string)
		row, exists := candidates[id]
		if !exists {
			t.Fatalf("candidate omitted: %s", id)
		}
		base := checks[row["planning_check"].(string)]
		for got, want := range map[string]string{"planning_revision": "revision", "planning_status": "status", "selector_contract": "selector", "applicability_contract": "applicability", "semantics_contract": "semantics", "normative_provenance": "normative_provenance"} {
			compare(id+"/"+got, row[got], base[want])
		}
		compare(id+"/research identity", base["research_ids"], []any{id})
		copies := modelOwnershipRows(t, row["test_contracts"], "id")
		if len(copies) != len(base["test_contracts"].([]any)) {
			t.Fatalf("missing clause five-axis contract: %s", id)
		}
		for _, raw := range base["test_contracts"].([]any) {
			name := raw.(string)
			compare(id+"/"+name, copies[name], clauseTests[name])
		}
	}
}

// Resolve implementation claims against real Go declarations and the compiled
// Required inventories. This checks traceability, not whether tests ran or the
// completeness of their assertions against every planned case.
func TestModelAssessmentOwnershipExecutableEvidence(t *testing.T) {
	ledger := modelOwnershipJSON(t, "docs/validation/model-assessment-ownership.json")
	engine, err := buildModelAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	required := map[validation.RuleKey]bool{}
	localRequired := map[validation.RuleKey]bool{}
	for _, id := range []validation.ProfileID{ProfileModelXSD, ProfileModelLocal, ProfileSourceLocal} {
		for _, key := range catalogProfile(t, engine, id).Required {
			required[key] = true
			if strings.HasPrefix(string(key.ID), "MX40-LOCAL-") {
				localRequired[key] = true
			}
		}
	}
	parsed := map[string]map[string]bool{}
	checkTest := func(owner string, evidence map[string]any) {
		t.Helper()
		path, _ := evidence["file"].(string)
		name, _ := evidence["name"].(string)
		rel := filepath.FromSlash(path)
		if path == "" || filepath.IsAbs(rel) || filepath.Clean(rel) != rel || strings.HasPrefix(rel, "..") || !strings.HasSuffix(path, "_test.go") || (!strings.HasPrefix(name, "Test") && !strings.HasPrefix(name, "Fuzz") && !strings.HasPrefix(name, "Example")) {
			t.Fatalf("invalid executable evidence for %s: %q %q", owner, path, name)
		}
		if parsed[path] == nil {
			file, err := parser.ParseFile(token.NewFileSet(), rel, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			parsed[path] = map[string]bool{}
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil {
					parsed[path][fn.Name.Name] = true
				}
			}
		}
		if !parsed[path][name] || evidence["scope"] == "" {
			t.Fatalf("nonexistent or unscoped executable test for %s: %s/%s", owner, path, name)
		}
	}
	boundLocals := map[validation.RuleKey]bool{}
	selected, split, deferred := 0, 0, 0
	deferredOwners := map[string][]any{
		"MX40-PROSE-barline-location":       {float64(6), float64(7)},
		"MX40-PROSE-beat-repeat-content":    {float64(6), float64(7)},
		"MX40-PROSE-bend-release-negative":  {float64(7)},
		"MX40-PROSE-chord-duration-bound":   {float64(6)},
		"MX40-PROSE-measure-repeat-content": {float64(6), float64(7)},
		"MX40-PROSE-numeral-requires-key":   {float64(6)},
		"MX40-PROSE-pitch-alter-explicit":   {float64(7)},
		"MX40-PROSE-time-only-ascending":    {float64(7)},
		"MX40-PROSE-written-transposition":  {float64(6), float64(7)},
	}
	for _, section := range []string{"requirements", "candidate_dispositions"} {
		for _, raw := range ledger[section].([]any) {
			row := raw.(map[string]any)
			id, _ := row["id"].(string)
			if section == "candidate_dispositions" {
				id = row["research_id"].(string)
			}
			bindings, ok := row["runtime_bindings"].([]any)
			if !ok {
				t.Fatalf("missing explicit runtime bindings: %s", id)
			}
			seen := map[validation.RuleKey]bool{}
			for _, raw := range bindings {
				binding := raw.(map[string]any)
				revision, ok := binding["revision"].(float64)
				if !ok || revision != 1 {
					t.Fatalf("invalid frozen revision: %s", id)
				}
				key := validation.RuleKey{ID: validation.RuleID(binding["id"].(string)), Revision: uint32(revision)}
				if !required[key] || seen[key] {
					t.Fatalf("unbound/duplicate runtime claim for %s: %+v", id, key)
				}
				seen[key] = true
				if section == "candidate_dispositions" && localRequired[key] {
					boundLocals[key] = true
				}
			}
			tests, ok := row["executable_tests"].([]any)
			if !ok {
				t.Fatalf("missing explicit executable evidence array: %s", id)
			}
			for _, raw := range tests {
				checkTest(id, raw.(map[string]any))
			}
			if section != "candidate_dispositions" {
				continue
			}
			switch row["disposition"] {
			case "selected-local-clause", "selected-shared-XSD", "selected-shared-typed-reference":
				selected++
				if len(bindings) == 0 || len(tests) == 0 {
					t.Fatalf("selected clause lacks concrete binding/evidence: %s", id)
				}
			case "split-local-selected":
				split++
				if id != "MX40-PROSE-instrument-link-target" || len(bindings) != 1 || len(tests) == 0 || !reflect.DeepEqual(row["remaining_batches"], []any{float64(8)}) {
					t.Fatalf("local/external split changed: %s", id)
				}
			case "deferred-outside-bounded-local-profile":
				deferred++
				if len(bindings) != 0 || len(tests) != 0 || !reflect.DeepEqual(row["remaining_batches"], deferredOwners[id]) || row["remaining_scope"] == "" {
					t.Fatalf("deferred clause mislabeled as runtime coverage: %s", id)
				}
			default:
				t.Fatalf("unreviewed candidate disposition: %s", id)
			}
		}
	}
	if selected != 19 || split != 1 || deferred != 9 || !reflect.DeepEqual(boundLocals, localRequired) || len(boundLocals) != 22 {
		t.Fatalf("frozen scope mismatch: selected=%d split=%d deferred=%d local=%d", selected, split, deferred, len(boundLocals))
	}
}
