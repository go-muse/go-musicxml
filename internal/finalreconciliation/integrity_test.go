package finalreconciliation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

var axes = []string{"positive", "negative", "boundary", "missing_fact", "interaction"}

func validateLedger(root string, ledger map[string]any) error {
	references := &ledgerReferences{files: make(map[string][]byte), documents: make(map[string]any)}
	readJSON, readLocal := references.readJSON, references.readLocal
	refs, refEquals, resolveRef := references.refs, references.refEquals, references.resolveRef
	if ledger["format"] != "musicxml-final-requirement-reconciliation-1" {
		return fmt.Errorf("unexpected format")
	}
	plan, err := readJSON(root, "docs/validation/plan/implementation-plan.json")
	if err != nil {
		return err
	}
	originals, err := indexed(plan["requirements"], "id")
	if err != nil {
		return err
	}
	originalTests, err := indexed(plan["test_contracts"], "id")
	if err != nil {
		return err
	}
	requirements, err := indexed(ledger["requirements"], "id")
	if err != nil {
		return err
	}
	tests, err := indexed(ledger["test_contracts"], "id")
	if err != nil {
		return err
	}
	if len(originals) != 95 || !sameKeys(requirements, originals) {
		return fmt.Errorf("requirement identity set must match original 95")
	}
	if len(originalTests) != 95 || !sameKeys(tests, originalTests) {
		return fmt.Errorf("test contract identity set must match original 95")
	}
	cases, err := indexed(ledger["cases"], "id")
	if err != nil {
		return err
	}
	if len(cases) == 0 {
		return fmt.Errorf("missing substantive cases")
	}
	for id, c := range cases {
		if err := references.validateCase(root, id, c); err != nil {
			return err
		}
	}
	if err := validateMatrixEdges(originals, originalTests); err != nil {
		return err
	}
	requirementPolicy := map[string]any{}
	axisPolicy := map[string]any{}
	usedCases := map[string]bool{}
	for id, row := range requirements {
		if !reflect.DeepEqual(row["original"], originals[id]) {
			return fmt.Errorf("original requirement changed: %s", id)
		}
		if err := refEquals(root, stringField(row, "original_ref"), originals[id]); err != nil {
			return err
		}
		if !reflect.DeepEqual(row["test_contract_ids"], originals[id]["test_contracts"]) {
			return fmt.Errorf("%s lost test contract links", id)
		}
		class := stringField(row, "classification")
		switch class {
		case "implemented-and-verified", "implemented-awaiting-current-verification", "explicit-bounded-limitation", "unresolved-decision", "missing-implementation", "optional-not-elected":
		default:
			return fmt.Errorf("%s invalid classification %q", id, class)
		}
		if stringField(row, "implementation") == "" || stringField(row, "residual_scope") == "" || stringField(row, "closure_criterion") == "" {
			return fmt.Errorf("%s lacks implementation/residual/closure", id)
		}
		for _, key := range []string{"implementation_refs", "decision_refs"} {
			if list, err := stringList(row[key]); err != nil || len(list) == 0 {
				return fmt.Errorf("%s needs nonempty %s", id, key)
			}
		}
		for _, key := range []string{"implementation_refs", "decision_refs", "inherited_refs", "crosswalk_refs"} {
			if err := refs(root, row[key]); err != nil {
				return fmt.Errorf("%s %s: %w", id, key, err)
			}
		}
		inherited, err := stringList(row["inherited_refs"])
		if err != nil || len(inherited) != 4 {
			return fmt.Errorf("%s missing inherited ownership references", id)
		}
		for _, ref := range inherited {
			value, err := resolveRef(root, ref)
			if err != nil {
				return err
			}
			record, ok := value.(map[string]any)
			if !ok || record["id"] != id {
				return fmt.Errorf("%s inherited reference names another requirement", id)
			}
		}
		execution, ok := row["execution"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s missing execution disposition", id)
		}
		if stringField(execution, "state") != "not-executed-on-final-tree" {
			return fmt.Errorf("%s unsupported execution state; a separate exact-tree receipt is required", id)
		}
		if list, err := stringList(execution["historical_refs"]); err != nil || len(list) == 0 {
			return fmt.Errorf("%s needs nonempty historical_refs", id)
		}
		if err := refs(root, execution["historical_refs"]); err != nil {
			return err
		}
		// This ledger is a development reconciliation at a frozen baseline.
		// None of its historic receipts certifies the final integration tree.
		if class == "implemented-and-verified" {
			return fmt.Errorf("%s unsupported closure: final-tree execution is pending", id)
		}
		if class == "optional-not-elected" && originals[id]["status"] != "optional" {
			return fmt.Errorf("%s mandatory requirement labeled optional", id)
		}
		requirementPolicy[id] = map[string]any{
			"classification": class, "residual_scope": row["residual_scope"],
			"closure_criterion": row["closure_criterion"], "crosswalk_refs": row["crosswalk_refs"],
		}
	}
	for id, row := range tests {
		if !reflect.DeepEqual(row["original"], originalTests[id]) {
			return fmt.Errorf("original test contract changed: %s", id)
		}
		if err := refEquals(root, stringField(row, "original_ref"), originalTests[id]); err != nil {
			return err
		}
		if !reflect.DeepEqual(row["requirement_ids"], originalTests[id]["requirement_ids"]) {
			return fmt.Errorf("%s changed requirement links", id)
		}
		axisMap, ok := row["axes"].(map[string]any)
		if !ok || len(axisMap) != 5 {
			return fmt.Errorf("%s needs five case axes", id)
		}
		for _, axis := range axes {
			a, ok := axisMap[axis].(map[string]any)
			if !ok {
				return fmt.Errorf("%s needs five case axes", id)
			}
			if !reflect.DeepEqual(a["original_cases"], originalTests[id]["cases"].(map[string]any)[axis]) {
				return fmt.Errorf("%s original test contract axis changed", id)
			}
			ids, ok := a["case_ids"].([]any)
			if !ok {
				return fmt.Errorf("%s/%s missing explicit case list", id, axis)
			}
			seen := map[string]bool{}
			for _, v := range ids {
				s, ok := v.(string)
				if !ok || cases[s] == nil {
					return fmt.Errorf("%s/%s unknown case %v", id, axis, v)
				}
				if seen[s] {
					return fmt.Errorf("%s/%s duplicate case", id, axis)
				}
				seen[s] = true
				usedCases[s] = true
			}
			status := stringField(a, "assessment")
			switch status {
			case "mapped", "partial", "decision-only", "missing-evidence", "not-elected":
			default:
				return fmt.Errorf("%s/%s invalid assessment", id, axis)
			}
			if status == "mapped" && len(ids) == 0 {
				return fmt.Errorf("%s/%s mapped without a concrete case", id, axis)
			}
			if status != "mapped" && stringField(a, "gap") == "" {
				return fmt.Errorf("%s/%s unproved axis lacks explicit gap", id, axis)
			}
			if stringField(a, "assessment_basis") == "" {
				return fmt.Errorf("%s/%s missing substantive assessment", id, axis)
			}
			axisPolicy[id+"/"+axis] = map[string]any{
				"assessment": status, "case_ids": ids, "assessment_basis": a["assessment_basis"],
				"gap": a["gap"], "gap_kind": a["gap_kind"],
			}
		}
	}
	for id := range cases {
		if !usedCases[id] {
			return fmt.Errorf("unreferenced case %s", id)
		}
	}
	questionDoc, err := readJSON(root, "docs/validation/plan/open-questions.json")
	if err != nil {
		return err
	}
	questions, err := indexed(questionDoc["open_questions"], "id")
	if err != nil {
		return err
	}
	issues, err := indexed(ledger["issues"], "id")
	if err != nil {
		return err
	}
	if len(issues) != 28 || !sameKeys(issues, questions) {
		return fmt.Errorf("issue identity set must match original 28")
	}
	for id, row := range issues {
		if !reflect.DeepEqual(row["original"], questions[id]) {
			return fmt.Errorf("original issue changed: %s", id)
		}
		if err := refEquals(root, stringField(row, "original_ref"), questions[id]); err != nil {
			return err
		}
		if stringField(row, "current_disposition") == "" || stringField(row, "residual_scope") == "" {
			return fmt.Errorf("%s missing issue disposition", id)
		}
		if list, err := stringList(row["decision_refs"]); err != nil || len(list) == 0 {
			return fmt.Errorf("%s needs nonempty decision_refs", id)
		}
		if err := refs(root, row["decision_refs"]); err != nil {
			return err
		}
	}
	model, err := readJSON(root, "docs/validation/model-assessment-ownership.json")
	if err != nil {
		return err
	}
	oldCandidates, err := indexed(model["candidate_dispositions"], "research_id")
	if err != nil {
		return err
	}
	candidates, err := indexed(ledger["candidates"], "id")
	if err != nil {
		return err
	}
	if len(candidates) != 29 || !sameKeys(candidates, oldCandidates) {
		return fmt.Errorf("candidate identity set must match original 29")
	}
	for id, row := range candidates {
		if err := refEquals(root, stringField(row, "original_ref"), oldCandidates[id]); err != nil {
			return err
		}
		if stringField(row, "current_disposition") == "" || stringField(row, "residual_scope") == "" {
			return fmt.Errorf("%s missing candidate disposition", id)
		}
		if list, err := stringList(row["decision_refs"]); err != nil || len(list) == 0 {
			return fmt.Errorf("%s needs nonempty decision_refs", id)
		}
		if err := refs(root, row["decision_refs"]); err != nil {
			return err
		}
	}
	context, err := readJSON(root, "docs/validation/plan/context-dispositions.json")
	if err != nil {
		return err
	}
	originalTranslations, err := indexed(context["dispositions"], "research_id")
	if err != nil {
		return err
	}
	translatedDoc, err := readJSON(root, "docs/validation/evidence/context-reviewed-translations.json")
	if err != nil {
		return err
	}
	translated, err := indexed(translatedDoc["records"], "research_id")
	if err != nil {
		return err
	}
	translations, err := indexed(ledger["translations"], "id")
	if err != nil {
		return err
	}
	if len(translations) != 291 || len(translated) != 177 || !sameKeys(translations, originalTranslations) {
		return fmt.Errorf("translation identity set must retain 291 and committed 177")
	}
	missing := 0
	for id, row := range translations {
		if err := refEquals(root, stringField(row, "original_ref"), originalTranslations[id]); err != nil {
			return err
		}
		if old := translated[id]; old != nil {
			if row["disposition"] != "committed-reviewed-triplet" {
				return fmt.Errorf("%s translation disposition must retain committed evidence", id)
			}
			if err := refEquals(root, stringField(row, "evidence_ref"), old); err != nil {
				return err
			}
		} else {
			missing++
			if row["disposition"] != "missing-portable-evidence" || row["evidence_ref"] != nil {
				return fmt.Errorf("%s translation disposition cannot silently close missing evidence", id)
			}
		}
	}
	if missing != 114 {
		return fmt.Errorf("translation portability residual must remain 114")
	}
	obligations, err := indexed(ledger["retained_obligations"], "id")
	if err != nil {
		return err
	}
	wantObligations := map[string]string{
		"B5-PERF-01":                  "open-later-separate-authorization",
		"TRANSLATION-114-PORTABILITY": "open-no-waiver-no-recovery",
	}
	if len(obligations) != len(wantObligations) {
		return fmt.Errorf("retained obligation set must contain exactly B5-PERF-01 and TRANSLATION-114-PORTABILITY")
	}
	for id, state := range wantObligations {
		row := obligations[id]
		if row == nil || row["state"] != state || stringField(row, "closure_criterion") == "" {
			return fmt.Errorf("%s must remain open with a closure criterion", id)
		}
		if _, err := resolveRef(root, stringField(row, "source")); err != nil {
			return err
		}
	}
	inventories, ok := ledger["linked_inventories"].([]any)
	if !ok || len(inventories) < 8 {
		return fmt.Errorf("missing linked inventory identity sets")
	}
	// These inventories overlap, but none can disappear from the reconciliation.
	wantInventories := map[string]bool{}
	for _, ref := range []string{
		"plan/context-dispositions.json#/checks", "plan/context-dispositions.json#/test_contracts", "plan/context-dispositions.json#/overlap_reviews",
		"plan/prose-contracts.json#/checks", "plan/prose-contracts.json#/test_contracts", "plan/prose-contracts.json#/overlap_reviews",
		"plan/xsd-contracts.json#/checks", "plan/xsd-contracts.json#/instances", "plan/xsd-test-contracts.json#/test_contracts",
		"source-assessment-ownership.json#/source_contracts", "functional-completion-crosswalk.json#/external_rows", "functional-completion-crosswalk.json#/musical_rows",
	} {
		wantInventories["docs/validation/"+ref] = true
	}
	if len(inventories) != len(wantInventories) {
		return fmt.Errorf("missing linked inventory collection")
	}
	for _, v := range inventories {
		row, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("malformed linked inventory")
		}
		ref := stringField(row, "ref")
		if !wantInventories[ref] {
			return fmt.Errorf("unknown or duplicate linked inventory %s", ref)
		}
		delete(wantInventories, ref)
		source, e := resolveRef(root, stringField(row, "ref"))
		if e != nil {
			return e
		}
		rows, e := indexed(source, stringField(row, "id_key"))
		if e != nil {
			return e
		}
		ids, e := stringList(row["ids"])
		if e != nil {
			return e
		}
		want := make([]string, 0, len(rows))
		for id := range rows {
			want = append(want, id)
		}
		sort.Strings(want)
		sort.Strings(ids)
		if !reflect.DeepEqual(ids, want) {
			return fmt.Errorf("linked inventory identity set changed: %s", row["ref"])
		}
	}
	frozen, err := indexed(ledger["preserved_sources"], "path")
	if err != nil || len(frozen) != len(preservedSourcePins) {
		return fmt.Errorf("preserved source set must retain the exact eight paths")
	}
	for path, want := range preservedSourcePins {
		r := frozen[path]
		if r == nil || r["sha256_lf"] != want {
			return fmt.Errorf("preserved source set or fingerprint changed: %s", path)
		}
		data, e := readLocal(root, path)
		if e != nil {
			return e
		}
		sum := sha256.Sum256([]byte(normalize(data)))
		if hex.EncodeToString(sum[:]) != want {
			return fmt.Errorf("preserved source changed: %s", path)
		}
	}
	if err := checkReviewedPolicy("requirement", requirementPolicy, reviewedRequirementPolicySHA256); err != nil {
		return err
	}
	if err := checkReviewedPolicy("axis", axisPolicy, reviewedAxisPolicySHA256); err != nil {
		return err
	}
	if err := checkReviewedPolicy("case", cases, reviewedCasePolicySHA256); err != nil {
		return err
	}
	if err := validateEvidenceSummary(ledger); err != nil {
		return err
	}
	return nil
}

func stringField(m map[string]any, key string) string { s, _ := m[key].(string); return s }
func normalize(b []byte) string                       { return strings.ReplaceAll(string(b), "\r\n", "\n") }

type ledgerReferences struct {
	files     map[string][]byte
	documents map[string]any
}

func (r *ledgerReferences) readLocal(root, path string) ([]byte, error) {
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") || strings.HasPrefix(filepath.Clean(path), "..") {
		return nil, fmt.Errorf("invalid local reference %q", path)
	}
	key := filepath.Join(root, filepath.FromSlash(path))
	if data, ok := r.files[key]; ok {
		return data, nil
	}
	data, err := os.ReadFile(key)
	if err != nil {
		return nil, fmt.Errorf("reference %s: %w", path, err)
	}
	r.files[key] = data
	return data, nil
}
func (r *ledgerReferences) readJSON(root, path string) (map[string]any, error) {
	value, err := r.resolveRef(root, path+"#")
	if err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("reference %s is not a JSON object", path)
	}
	return object, nil
}

func indexed(value any, key string) (map[string]map[string]any, error) {
	rows, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("missing %s row array", key)
	}
	result := map[string]map[string]any{}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("malformed %s row", key)
		}
		id := stringField(row, key)
		if id == "" || result[id] != nil {
			return nil, fmt.Errorf("missing or duplicate %s: %s", key, id)
		}
		result[id] = row
	}
	return result, nil
}
func sameKeys(a, b map[string]map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if b[k] == nil {
			return false
		}
	}
	return true
}
func stringList(v any) ([]string, error) {
	a, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("missing string list")
	}
	r := make([]string, 0, len(a))
	for _, v := range a {
		s, ok := v.(string)
		if !ok || s == "" {
			return nil, fmt.Errorf("invalid string list")
		}
		r = append(r, s)
	}
	return r, nil
}
func (r *ledgerReferences) refs(root string, value any) error {
	list, e := stringList(value)
	if e != nil {
		return e
	}
	for _, ref := range list {
		if _, err := r.resolveRef(root, ref); err != nil {
			return err
		}
	}
	return nil
}
func (r *ledgerReferences) refEquals(root, ref string, want any) error {
	got, e := r.resolveRef(root, ref)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("reference %s does not resolve to preserved record", ref)
	}
	return nil
}
func (r *ledgerReferences) resolveRef(root, ref string) (any, error) {
	path, pointer, has := strings.Cut(ref, "#")
	data, e := r.readLocal(root, path)
	if e != nil {
		return nil, e
	}
	if !has {
		return string(data), nil
	}
	key := filepath.Join(root, filepath.FromSlash(path))
	value, cached := r.documents[key]
	if !cached {
		if e = json.Unmarshal(data, &value); e != nil {
			return nil, fmt.Errorf("reference %s: %w", ref, e)
		}
		r.documents[key] = value
	}
	if pointer == "" {
		return value, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("invalid JSON reference %s", ref)
	}
	for _, part := range strings.Split(pointer[1:], "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch v := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = v[part]
			if !ok {
				return nil, fmt.Errorf("broken reference %s", ref)
			}
		case []any:
			n, e := strconv.Atoi(part)
			if e != nil || n < 0 || n >= len(v) {
				return nil, fmt.Errorf("broken reference %s", ref)
			}
			value = v[n]
		default:
			return nil, fmt.Errorf("broken reference %s", ref)
		}
	}
	return value, nil
}
