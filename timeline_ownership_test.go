package musicxml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func timelineOwnershipJSON(t *testing.T, path string) map[string]any {
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

func timelineOwnershipRows(t *testing.T, value any, key string) map[string]map[string]any {
	t.Helper()
	rows, ok := value.([]any)
	if !ok {
		t.Fatalf("missing row array for %s", key)
	}
	result := map[string]map[string]any{}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("invalid row for %s", key)
		}
		id, ok := row[key].(string)
		if !ok || id == "" || result[id] != nil {
			t.Fatalf("missing/duplicate %s: %v", key, row[key])
		}
		result[id] = row
	}
	return result
}

func timelineOwnershipNonempty(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

// These are provenance/ownership checks, not executable musical coverage.
// The original research statuses and five-axis recipes must survive new work.
func TestTimelineOwnershipPreservesProgram(t *testing.T) {
	ledger := timelineOwnershipJSON(t, "docs/validation/timeline-assessment-ownership.json")
	previous := timelineOwnershipJSON(t, "docs/validation/model-assessment-ownership.json")
	snapshot := timelineOwnershipJSON(t, "docs/validation/plan/external-snapshot.json")
	if ledger["format"] != "musicxml-timeline-assessment-ownership-1" || ledger["registry_commit"] != snapshot["registry_commit"] {
		t.Fatal("unrecognized timeline ownership format or registry pin")
	}
	for _, key := range []string{"retained_counts", "source_contracts"} {
		if !reflect.DeepEqual(ledger[key], previous[key]) {
			t.Fatalf("retained program %s changed", key)
		}
	}
	for section, idKey := range map[string]string{"requirements": "id", "issue_gates": "id", "candidate_dispositions": "research_id"} {
		got := timelineOwnershipRows(t, ledger[section], idKey)
		want := timelineOwnershipRows(t, previous[section], idKey)
		expectedCount := map[string]int{"requirements": 95, "issue_gates": 28, "candidate_dispositions": 29}[section]
		if len(got) != len(want) || len(got) != expectedCount {
			t.Fatalf("%s inventory changed", section)
		}
		for id, prior := range want {
			row, ok := got[id]
			if !ok {
				t.Fatalf("missing %s/%s", section, id)
			}
			if !reflect.DeepEqual(row["previous_ownership"], prior) {
				t.Errorf("original ownership or planning evidence changed: %s", id)
			}
			owner, ok := row["batch6_ownership"].(map[string]any)
			if !ok || owner["status"] != "planned" || !timelineOwnershipNonempty(owner["scope"]) || !reflect.DeepEqual(owner["remaining_batches"], prior["remaining_batches"]) {
				t.Errorf("unqualified closure or lost owner: %s", id)
			}
		}
	}
}

func TestTimelineOwnershipCompleteClauseSelection(t *testing.T) {
	ledger := timelineOwnershipJSON(t, "docs/validation/timeline-assessment-ownership.json")
	context := timelineOwnershipJSON(t, "docs/validation/plan/context-dispositions.json")
	prose := timelineOwnershipJSON(t, "docs/validation/plan/prose-contracts.json")
	previous := timelineOwnershipJSON(t, "docs/validation/model-assessment-ownership.json")
	selection := ledger["selection"].(map[string]any)
	seeds := map[string]bool{}
	for _, v := range selection["dependency_seed_labels"].([]any) {
		seeds[v.(string)] = true
	}
	expected := map[string]bool{}
	for _, raw := range context["dispositions"].([]any) {
		d := raw.(map[string]any)
		for _, dep := range d["dependencies"].([]any) {
			if seeds[dep.(string)] {
				expected[d["research_id"].(string)] = true
			}
		}
	}
	for _, raw := range previous["candidate_dispositions"].([]any) {
		row := raw.(map[string]any)
		for _, b := range row["remaining_batches"].([]any) {
			if b == float64(6) {
				expected[row["research_id"].(string)] = true
			}
		}
	}
	for _, id := range selection["manual_adjacent_clause_ids"].([]any) {
		expected[id.(string)] = true
	}
	clauses := timelineOwnershipRows(t, ledger["clauses"], "research_id")
	if len(clauses) != 121 || len(expected) != len(clauses) {
		t.Fatalf("selected scope: got %d, expected %d", len(clauses), len(expected))
	}
	var selectedIDs []string
	for id := range clauses {
		selectedIDs = append(selectedIDs, id)
	}
	sort.Strings(selectedIDs)
	selectedSum := sha256.Sum256([]byte(strings.Join(selectedIDs, "\n") + "\n"))
	const reviewedSelection = "592084e72d5942802db4fbe590aa9b5d5d975672dc18f6c2c2812c9ec4687118"
	if hex.EncodeToString(selectedSum[:]) != reviewedSelection {
		t.Fatal("reviewed selected record set changed")
	}
	checks := timelineOwnershipRows(t, context["checks"], "id")
	tests := timelineOwnershipRows(t, context["test_contracts"], "id")
	for id, row := range timelineOwnershipRows(t, prose["checks"], "id") {
		checks[id] = row
	}
	for id, row := range timelineOwnershipRows(t, prose["test_contracts"], "id") {
		tests[id] = row
	}
	records := timelineOwnershipRows(t, ledger["source_records"], "research_id")
	checkCopies := func(id string, row map[string]any) {
		t.Helper()
		for _, pair := range []struct {
			key       string
			originals map[string]map[string]any
		}{{"checks_original", checks}, {"test_contracts_original", tests}} {
			for k, copy := range timelineOwnershipRows(t, row[pair.key], "id") {
				if !reflect.DeepEqual(copy, pair.originals[k]) {
					t.Errorf("original %s changed for %s", k, id)
				}
			}
		}
	}
	dispositions := timelineOwnershipRows(t, context["dispositions"], "research_id")
	candidates := timelineOwnershipRows(t, previous["candidate_dispositions"], "research_id")
	for id := range expected {
		row, ok := clauses[id]
		if !ok || records[id] == nil {
			t.Fatalf("selected clause/source missing: %s", id)
		}
		checkCopies(id, row)
		if original := dispositions[id]; original != nil && !reflect.DeepEqual(row["context_disposition_original"], original) {
			t.Errorf("original contextual status/contract changed: %s", id)
		}
		if original := candidates[id]; original != nil && !reflect.DeepEqual(row["candidate_ownership_original"], original) {
			t.Errorf("original candidate ownership changed: %s", id)
		}
		actualTests := timelineOwnershipRows(t, row["test_contracts_original"], "id")
		actualChecks := timelineOwnershipRows(t, row["checks_original"], "id")
		for _, check := range actualChecks {
			for _, testID := range check["test_contracts"].([]any) {
				if actualTests[testID.(string)] == nil {
					t.Errorf("missing original test contract: %s/%s", id, testID)
				}
			}
		}
		for checkID, c := range checks {
			for _, rawID := range c["research_ids"].([]any) {
				if rawID == id && actualChecks[checkID] == nil {
					t.Errorf("selected shared check missing: %s/%s", id, checkID)
				}
			}
		}
		ownership := row["ownership"].(map[string]any)
		if ownership["status"] != "planned" || !timelineOwnershipNonempty(ownership["implementation_target"]) || !timelineOwnershipNonempty(ownership["batch6_scope"]) || !timelineOwnershipNonempty(ownership["closure_evidence_required"]) {
			t.Errorf("missing planned scope/evidence: %s", id)
		}
		remaining, ok := ownership["remaining_batches"].([]any)
		if !ok || !timelineOwnershipNonempty(ownership["remaining_owner"]) {
			t.Errorf("remaining clause owner omitted: %s", id)
		}
		kind := row["batch6_proposal"].(map[string]any)["classification"]
		if kind != "existing-shared-prerequisite" {
			containsSix := false
			for _, batch := range remaining {
				containsSix = containsSix || batch == float64(6)
			}
			if !containsSix {
				t.Errorf("uncompleted batch-6 contribution removed: %s", id)
			}
		}
		if len(ownership["runtime_bindings"].([]any)) != 0 || len(ownership["executable_tests"].([]any)) != 0 {
			t.Errorf("unreconciled runtime evidence: %s", id)
		}
		if !reflect.DeepEqual(ownership["targets"], []any{"source", "model"}) {
			t.Errorf("target omitted: %s", id)
		}
	}
	split := timelineOwnershipRows(t, ledger["split_shared_checks"], "id")
	if len(split) != 20 {
		t.Fatalf("split/shared checks = %d", len(split))
	}
	// Identify the complete original split/shared inventory independently by its
	// record-specific planning contract. Generic source checks are not substitutes.
	for id, check := range timelineOwnershipRows(t, context["checks"], "id") {
		if _, isSplit := check["semantic_status"]; !isSplit {
			continue
		}
		if check["expected_outcomes"] != nil || check["role"] != "context_fact" {
			continue
		}
		row := split[id]
		if row == nil || !reflect.DeepEqual(row["check_original"], check) {
			t.Errorf("split/shared contract missing or changed: %s", id)
			continue
		}
		for _, sourceID := range check["research_ids"].([]any) {
			if records[sourceID.(string)] == nil {
				t.Errorf("split/shared source missing: %s", sourceID)
			}
		}
	}
	for id, row := range split {
		if !reflect.DeepEqual(row["check_original"], checks[id]) || !timelineOwnershipNonempty(row["remaining_owner"]) || !timelineOwnershipNonempty(row["implementation_target"]) {
			t.Errorf("split/shared owner lost: %s", id)
		}
		copies := timelineOwnershipRows(t, row["test_contracts_original"], "id")
		declared := checks[id]["test_contracts"].([]any)
		if len(copies) != len(declared) {
			t.Errorf("split original tests missing: %s", id)
		}
		for _, rawID := range declared {
			testID := rawID.(string)
			if !reflect.DeepEqual(copies[testID], tests[testID]) {
				t.Errorf("split test missing or changed: %s", testID)
			}
		}
		withinTimeline := false
		for _, sourceID := range checks[id]["research_ids"].([]any) {
			withinTimeline = withinTimeline || clauses[sourceID.(string)] != nil
		}
		wantOwners := []any{float64(7), float64(9)}
		if withinTimeline {
			wantOwners = []any{float64(6), float64(7)}
		}
		if id == "CHECK-context-xlink-show-default" || id == "CHECK-context-image-resource-type" {
			wantOwners = []any{float64(8), float64(9)}
		}
		if row["status"] != "planned" || !timelineOwnershipNonempty(row["closure_evidence_required"]) || !reflect.DeepEqual(row["remaining_batches"], wantOwners) {
			t.Errorf("split check closure or ownership drift: %s", id)
		}
	}
}

func TestTimelineOwnershipOriginalSourceAndTranslation(t *testing.T) {
	ledger := timelineOwnershipJSON(t, "docs/validation/timeline-assessment-ownership.json")
	records := timelineOwnershipRows(t, ledger["source_records"], "research_id")
	if len(records) != 135 {
		t.Fatalf("source union = %d, want 135", len(records))
	}
	originals := map[string]any{}
	english := map[string]any{}
	research := timelineOwnershipRows(t, timelineOwnershipJSON(t, "docs/validation/plan/research-map.json")["research_records"], "research_id")
	for id, row := range records {
		original := row["source_record_original"].(map[string]any)
		originals[id] = original
		if original["id"] != id || row["registry_pointer"] != research[id]["registry_pointer"] {
			t.Errorf("lost original identity: %s", id)
		}
		for _, field := range []string{"description_ru", "short_explanation_ru", "conditions_ru"} {
			if _, ok := original[field].(string); !ok {
				t.Errorf("lost Russian field: %s/%s", id, field)
			}
		}
		translation := row["reviewed_english_translation"].(map[string]any)
		ru := translation["source_fields"].(map[string]any)
		for _, field := range []string{"description_ru", "short_explanation_ru", "conditions_ru"} {
			if ru[field] != original[field] {
				t.Errorf("translation source drift: %s/%s", id, field)
			}
		}
		detail := translation["review_detail"].(map[string]any)
		if detail["normative_or_implementation_approval"] != false || !timelineOwnershipNonempty(detail["approval_boundary"]) {
			t.Errorf("translation review boundary lost: %s", id)
		}
		englishFields := map[string]any{}
		for _, field := range []string{"description_en", "short_explanation_en", "conditions_en"} {
			englishFields[field] = translation[field]
			if _, ok := translation[field].(string); !ok {
				t.Errorf("lost English field: %s/%s", id, field)
			}
		}
		english[id] = englishFields
		if translation["translation_review"] != "reviewed-textual-equivalence-only" || translation["normative_approval"] != false {
			t.Errorf("translation mistaken for semantic approval: %s", id)
		}
		sources := original["sources"].([]any)
		if len(sources) == 0 {
			t.Errorf("missing sources: %s", id)
		}
		for _, raw := range sources {
			source := raw.(map[string]any)
			fields := []string{"source_id", "source_url"}
			switch source["location_kind"] {
			case "schema_annotation":
				fields = append(fields, "source_xpath", "source_text_sha256")
			case "html_section":
				fields = append(fields, "section_label", "source_document_sha256", "retrieval_source_url")
			default:
				t.Errorf("unrecognized exact source location for %s", id)
			}
			for _, field := range fields {
				if !timelineOwnershipNonempty(source[field]) {
					t.Errorf("missing pinned source field: %s/%s", id, field)
				}
			}
		}
	}
	data, err := json.Marshal(originals)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	// Independent pinned registry extraction, not a digest supplied by the ledger.
	const want = "29e85d041eb891b64e375747cb06a428bfec2c40350476f47b21b4cabc1017d6"
	if hex.EncodeToString(sum[:]) != want {
		t.Fatal("original registry source bytes changed")
	}
	data, err = json.Marshal(english)
	if err != nil {
		t.Fatal(err)
	}
	sum = sha256.Sum256(data)
	// Full-field translation review is separately pinned; pending original
	// status preservation alone cannot catch an altered English condition.
	const reviewedEnglish = "75f9ed7cbed57294620e0239f1ac1db9756ff5920c4cda2c73f8fdf7fdf7fdbc"
	if hex.EncodeToString(sum[:]) != reviewedEnglish {
		t.Fatal("reviewed English fields changed without renewed review")
	}

}

func TestTimelineOwnershipKeepsOpenGatesAndFrozenProfiles(t *testing.T) {
	ledger := timelineOwnershipJSON(t, "docs/validation/timeline-assessment-ownership.json")
	profiles := timelineOwnershipRows(t, ledger["frozen_profiles"], "id")
	expected := map[string]float64{"musicxml-4.0/source-xsd-preview": 12, "musicxml-4.0/model-xsd": 14, "musicxml-4.0/model-local": 36, "musicxml-4.0/source-local": 34}
	if len(profiles) != len(expected) {
		t.Fatal("frozen profile inventory changed")
	}
	for id, count := range expected {
		if profiles[id]["required_keys"] != count || profiles[id]["status"] != "frozen" {
			t.Errorf("frozen profile changed: %s", id)
		}
	}
	gates := timelineOwnershipRows(t, ledger["decision_gates"], "id")
	gate := gates["B6-DIVISIONS-REWIND"]
	if gate["status"] != "documented-semantics-research-remaining" || gate["onset_departure_approved"] != false || gate["all_solutions_approved"] != false || gate["blanket_incomplete_approved"] != false || gate["documented_uncertainty_preserved"] != true {
		t.Fatal("unapproved divisions interpretation promoted")
	}
	perf := ledger["performance_obligation"].(map[string]any)
	if perf["id"] != "B5-PERF-01" || perf["status"] != "outstanding" || perf["owner"] != "shared validation kernel/session workstream" || perf["final_reconciliation_batch"] != float64(9) {
		t.Fatal("retained performance owner changed")
	}
	evidence := ledger["execution_evidence"].(map[string]any)
	if evidence["status"] != "not-yet-reconciled" || evidence["implementation_commit"] != nil {
		t.Fatal("unverified implementation claim")
	}
	data, err := os.ReadFile("docs/validation/TIMELINE_ASSESSMENT_SCOPE.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(data)), " ")
	for _, required := range []string{"121", "135", "20 split/shared", "82", "B5-PERF-01", "batch 9", "evidence/research gate", "not approved", "not a scope ceiling", "does not close", "divisions", "key", "time", "clef", "transpose", "instrument"} {
		if !strings.Contains(text, required) {
			t.Errorf("scope lost %q", required)
		}
	}
	// Every affected original ID must be discoverable in the readable ledger.
	var ids []string
	for id := range timelineOwnershipRows(t, ledger["clauses"], "research_id") {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !strings.Contains(text, id) {
			t.Errorf("readable clause omitted: %s", id)
		}
	}
}

// The full selected and split-check capability union remains an unmodified
// planning contract, including capabilities outside the new timeline claim.
func TestTimelineOwnershipCapabilityAndSelectionIndex(t *testing.T) {
	ledger := timelineOwnershipJSON(t, "docs/validation/timeline-assessment-ownership.json")
	originals := timelineOwnershipRows(t, timelineOwnershipJSON(t, "docs/validation/plan/capabilities.json")["capability_gates"], "id")
	caps := timelineOwnershipRows(t, ledger["capabilities_original"], "id")
	if len(caps) != 144 {
		t.Fatalf("full capability union = %d, want 144", len(caps))
	}
	for id, row := range caps {
		if !reflect.DeepEqual(row, originals[id]) {
			t.Errorf("original capability changed: %s", id)
		}
	}
	for _, raw := range ledger["clauses"].([]any) {
		row := raw.(map[string]any)
		for _, c := range row["checks_original"].([]any) {
			for _, id := range c.(map[string]any)["capability_ids"].([]any) {
				if caps[id.(string)] == nil {
					t.Errorf("clause capability omitted: %s", id)
				}
			}
		}
	}
	for _, raw := range ledger["split_shared_checks"].([]any) {
		row := raw.(map[string]any)
		for _, id := range row["check_original"].(map[string]any)["capability_ids"].([]any) {
			if caps[id.(string)] == nil {
				t.Errorf("split capability omitted: %s", id)
			}
		}
	}
	index := timelineOwnershipRows(t, ledger["full_prose_review_index"], "research_id")
	selected := timelineOwnershipRows(t, ledger["clauses"], "research_id")
	research := timelineOwnershipRows(t, timelineOwnershipJSON(t, "docs/validation/plan/research-map.json")["research_records"], "research_id")
	if len(index) != 342 {
		t.Fatalf("full prose selection index = %d", len(index))
	}
	for id, row := range index {
		if research[id] == nil || row["included"] != (selected[id] != nil) || !timelineOwnershipNonempty(row["reason"]) {
			t.Errorf("selection evidence drift: %s", id)
		}
	}
	for id := range research {
		if strings.HasPrefix(id, "MX40-PROSE-") && index[id] == nil {
			t.Errorf("prose selection omitted: %s", id)
		}
	}
	context := timelineOwnershipRows(t, timelineOwnershipJSON(t, "docs/validation/plan/context-dispositions.json")["dispositions"], "research_id")
	for id, row := range timelineOwnershipRows(t, ledger["source_records"], "research_id") {
		translation := row["reviewed_english_translation"].(map[string]any)
		if d := context[id]; d != nil && translation["original_translation_status"] != d["translation"] {
			t.Errorf("original pending translation status changed: %s", id)
		}
	}
}
