package finalreconciliation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

var axes = []string{"positive", "negative", "boundary", "missing_fact", "interaction"}

func validateLedger(root string, ledger map[string]any) error {
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
		file, name := stringField(c, "file"), stringField(c, "function")
		data, err := readLocal(root, file)
		if err != nil {
			return err
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, data, 0)
		if err != nil {
			return err
		}
		found := false
		for _, d := range parsed.Decls {
			if f, ok := d.(*ast.FuncDecl); ok && f.Name.Name == name && f.Recv == nil && strings.HasPrefix(name, "Test") {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s missing test function %s in %s", id, name, file)
		}
		if stringField(c, "input") == "" || stringField(c, "expected") == "" || stringField(c, "scope") == "" {
			return fmt.Errorf("%s lacks substantive input/outcome/scope", id)
		}
		anchors, ok := c["anchors"].([]any)
		if !ok || len(anchors) < 2 {
			return fmt.Errorf("%s needs fixture and assertion anchors", id)
		}
		for _, v := range anchors {
			a, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("%s malformed anchor", id)
			}
			b, e := readLocal(root, stringField(a, "file"))
			if e != nil {
				return e
			}
			s := stringField(a, "text")
			if s == "" || !strings.Contains(normalize(b), s) {
				return fmt.Errorf("%s broken anchor in %s: %q", id, a["file"], s)
			}
		}
	}
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
		for _, key := range []string{"implementation_refs", "decision_refs", "inherited_refs", "crosswalk_refs"} {
			if err := refs(root, row[key]); err != nil {
				return fmt.Errorf("%s %s: %w", id, key, err)
			}
		}
		execution, ok := row["execution"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s missing execution disposition", id)
		}
		if stringField(execution, "state") != "not-executed-on-final-tree" {
			return fmt.Errorf("%s unsupported execution state; a separate exact-tree receipt is required", id)
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
	perf := obligations["B5-PERF-01"]
	if perf == nil || perf["state"] != "open-later-separate-authorization" || stringField(perf, "closure_criterion") == "" {
		return fmt.Errorf("B5-PERF-01 must remain open with a closure criterion")
	}
	inventories, ok := ledger["linked_inventories"].([]any)
	if !ok || len(inventories) < 8 {
		return fmt.Errorf("missing linked inventory identity sets")
	}
	for _, v := range inventories {
		row, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("malformed linked inventory")
		}
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
	frozen, ok := ledger["preserved_sources"].([]any)
	if !ok || len(frozen) < 3 {
		return fmt.Errorf("missing original source fingerprints")
	}
	for _, v := range frozen {
		r, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("bad source fingerprint")
		}
		data, e := readLocal(root, stringField(r, "path"))
		if e != nil {
			return e
		}
		sum := sha256.Sum256([]byte(normalize(data)))
		if hex.EncodeToString(sum[:]) != r["sha256_lf"] {
			return fmt.Errorf("preserved source changed: %s", r["path"])
		}
	}
	return nil
}

func stringField(m map[string]any, key string) string { s, _ := m[key].(string); return s }
func normalize(b []byte) string                       { return strings.ReplaceAll(string(b), "\r\n", "\n") }
func readLocal(root, path string) ([]byte, error) {
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") || strings.HasPrefix(filepath.Clean(path), "..") {
		return nil, fmt.Errorf("invalid local reference %q", path)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return nil, fmt.Errorf("reference %s: %w", path, err)
	}
	return data, nil
}
func readJSON(root, path string) (map[string]any, error) {
	b, e := readLocal(root, path)
	if e != nil {
		return nil, e
	}
	var m map[string]any
	e = json.Unmarshal(b, &m)
	return m, e
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
func refs(root string, value any) error {
	list, e := stringList(value)
	if e != nil {
		return e
	}
	for _, ref := range list {
		if _, err := resolveRef(root, ref); err != nil {
			return err
		}
	}
	return nil
}
func refEquals(root, ref string, want any) error {
	got, e := resolveRef(root, ref)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("reference %s does not resolve to preserved record", ref)
	}
	return nil
}
func resolveRef(root, ref string) (any, error) {
	path, pointer, has := strings.Cut(ref, "#")
	data, e := readLocal(root, path)
	if e != nil {
		return nil, e
	}
	if !has {
		return string(data), nil
	}
	var value any
	if e = json.Unmarshal(data, &value); e != nil {
		return nil, fmt.Errorf("reference %s: %w", ref, e)
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
