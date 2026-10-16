package finalreconciliation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These mutations reproduce review findings independently of the original
// blanket rejection of implemented-and-verified. They preserve unrelated data.
func TestFinalReconciliationRejectsReviewedPolicyDrift(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		mutate     func(map[string]any)
	}{
		{"limitation promoted", "reviewed requirement policy", func(m map[string]any) {
			r := rowWith(m, "requirements", "classification", "explicit-bounded-limitation")
			r["classification"] = "implemented-awaiting-current-verification"
			r["residual_scope"] = "None."
		}},
		{"unresolved decision promoted", "reviewed requirement policy", func(m map[string]any) {
			r := rowWith(m, "requirements", "classification", "unresolved-decision")
			r["classification"] = "implemented-awaiting-current-verification"
			r["residual_scope"] = "None."
		}},
		{"residual erased without promotion", "reviewed requirement policy", func(m map[string]any) {
			rowWith(m, "requirements", "classification", "explicit-bounded-limitation")["residual_scope"] = "None."
		}},
		{"Unknown alone substituted for closure", "reviewed requirement policy", func(m map[string]any) { first(m, "requirements")["closure_criterion"] = "Returns Unknown" }},
		{"partial promoted", "reviewed axis policy", func(m map[string]any) { promoteAxis(m, "partial") }},
		{"missing evidence promoted", "reviewed axis policy", func(m map[string]any) { promoteAxis(m, "missing-evidence") }},
		{"mapped basis replaced by Unknown", "reviewed axis policy", func(m map[string]any) {
			first(m, "test_contracts")["axes"].(map[string]any)["positive"].(map[string]any)["assessment_basis"] = "Returns Unknown"
		}},
		{"route outcome changed", "reviewed case policy", func(m map[string]any) { first(m, "cases")["expected"] = "Returns Unknown" }},
		{"PR guard relabeled baseline", "reviewed case policy", func(m map[string]any) {
			rowWith(m, "cases", "id", "TestFinalReconciliationRejectsHistoricalExecutionPromotion")["evidence_layer"] = "baseline"
		}},
		{"five source fingerprints removed", "preserved source set", func(m map[string]any) { m["preserved_sources"] = m["preserved_sources"].([]any)[:3] }},
		{"duplicate preserved source", "preserved source set", func(m map[string]any) { rows := m["preserved_sources"].([]any); rows[7] = rows[0] }},
		{"implementation references erased", "nonempty implementation_refs", func(m map[string]any) { first(m, "requirements")["implementation_refs"] = []any{} }},
		{"historical references erased", "nonempty historical_refs", func(m map[string]any) {
			first(m, "requirements")["execution"].(map[string]any)["historical_refs"] = []any{}
		}},
		{"decision references erased", "nonempty decision_refs", func(m map[string]any) { first(m, "requirements")["decision_refs"] = []any{} }},
		{"issue decisions erased", "nonempty decision_refs", func(m map[string]any) { first(m, "issues")["decision_refs"] = []any{} }},
		{"translation obligation removed", "retained obligation set", func(m map[string]any) { m["retained_obligations"] = m["retained_obligations"].([]any)[:1] }},
		{"translation obligation closed", "TRANSLATION-114-PORTABILITY", func(m map[string]any) {
			r := rowWith(m, "retained_obligations", "id", "TRANSLATION-114-PORTABILITY")
			r["state"] = "closed"
			r["closure_criterion"] = ""
		}},
		{"translation closure erased", "TRANSLATION-114-PORTABILITY", func(m map[string]any) {
			rowWith(m, "retained_obligations", "id", "TRANSLATION-114-PORTABILITY")["closure_criterion"] = ""
		}},
		{"unused route added", "unreferenced case", func(m map[string]any) {
			c := map[string]any{}
			for k, v := range first(m, "cases") {
				c[k] = v
			}
			c["id"] = "UNREFERENCED-REVIEW-ROUTE"
			m["cases"] = append(m["cases"].([]any), c)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) { root, m := loadLedger(t); tc.mutate(m); rejectLedger(t, root, m, tc.want) })
	}
}

func promoteAxis(m map[string]any, status string) {
	for _, raw := range m["test_contracts"].([]any) {
		for _, a := range raw.(map[string]any)["axes"].(map[string]any) {
			axis := a.(map[string]any)
			if axis["assessment"] == status {
				axis["assessment"] = "mapped"
				axis["gap"] = ""
				axis["case_ids"] = []any{first(m, "cases")["id"]}
				return
			}
		}
	}
	panic("missing axis status " + status)
}

func rowWith(m map[string]any, collection, key, value string) map[string]any {
	for _, raw := range m[collection].([]any) {
		r := raw.(map[string]any)
		if r[key] == value {
			return r
		}
	}
	panic("missing row " + value)
}

func rejectLedger(t *testing.T, root string, m map[string]any, want string) {
	t.Helper()
	if err := validateLedger(root, m); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("wanted rejection containing %q; got %v", want, err)
	}
}

// The source record, ledger copy and source fingerprint change together. A
// DeepEqual/fingerprint-only check therefore cannot pass these controls.
func TestFinalReconciliationRejectsConsistentlyUnlinkedMatrix(t *testing.T) {
	for _, tc := range []struct{ name, collection, sourceKey, ledgerKey string }{
		{"mandatory requirement without test", "requirements", "test_contracts", "test_contract_ids"},
		{"test without requirement", "test_contracts", "requirement_ids", "requirement_ids"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, m := loadLedger(t)
			const path = "docs/validation/plan/implementation-plan.json"
			raw, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				t.Fatal(err)
			}
			var plan map[string]any
			if err = json.Unmarshal(raw, &plan); err != nil {
				t.Fatal(err)
			}
			first(plan, tc.collection)[tc.sourceKey] = []any{}
			r := first(m, tc.collection)
			r["original"] = first(plan, tc.collection)
			r[tc.ledgerKey] = []any{}
			raw, err = json.MarshalIndent(plan, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(raw)
			rowWith(m, "preserved_sources", "path", path)["sha256_lf"] = hex.EncodeToString(sum[:])
			rejectLedger(t, overlayRoot(t, root, map[string][]byte{path: raw}), m, "matrix edge")
		})
	}
}

func TestFinalReconciliationRejectsUnboundAnchors(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		mutate     func(map[string]any)
	}{
		{"unrelated file", "anchor must name its test file", func(m map[string]any) {
			first(m, "cases")["anchors"] = []any{map[string]any{"file": "README.md", "text": "MusicXML"}, map[string]any{"file": "README.md", "text": "Go"}}
		}},
		{"generic repeated substrings", "anchor must occur exactly once", func(m map[string]any) {
			c := first(m, "cases")
			c["anchors"] = []any{map[string]any{"file": c["file"], "text": "t"}, map[string]any{"file": c["file"], "text": "func"}}
		}},
		{"another test in same file", "anchor is outside its test and directly referenced fixtures", func(m map[string]any) {
			c := first(m, "cases")
			c["anchors"] = []any{map[string]any{"file": c["file"], "text": "func TestXMLAttributeCheckDoesNotInventNamespaceBindings(t *testing.T) {"}, map[string]any{"file": c["file"], "text": "assert.NotContains(t, err.Error(), \"duplicate XML attribute\")"}}
		}},
		{"helper anchors only", "needs an anchor in the named test body", func(m map[string]any) {
			c := rowWith(m, "cases", "id", "TestValidateXSIType")
			c["anchors"].([]any)[1] = map[string]any{"file": c["file"], "text": `add("unknown named type", "text", ` + "`" + ` i:type="t:Missing"` + "`" + `, "ok", xsi)`}
		}},
		{"repeated anchor entry", "duplicate fixture/assertion anchor", func(m map[string]any) {
			c := rowWith(m, "cases", "id", "TestValidateXSIType")
			assertion := c["anchors"].([]any)[1]
			c["anchors"] = []any{assertion, assertion}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) { root, m := loadLedger(t); tc.mutate(m); rejectLedger(t, root, m, tc.want) })
	}
	t.Run("package fixture no longer referenced", func(t *testing.T) {
		root, m := loadLedger(t)
		const path = "validation_xsi_type_test.go"
		raw := mutateTestBody(t, root, path, "TestValidateXSIType", func(body string) string {
			// Keep the assertion anchor byte-for-byte intact. Only remove the
			// reference that binds the package-level fixture-building helper.
			return strings.Replace(body, "validationXSICases()", "[]validationSourceCase{}", 1)
		})
		c := rowWith(m, "cases", "id", "TestValidateXSIType")
		for _, a := range c["anchors"].([]any) {
			if !strings.Contains(string(raw), strings.TrimSpace(a.(map[string]any)["text"].(string))) {
				t.Fatal("fixture-reference mutation damaged an anchor")
			}
		}
		rejectLedger(t, overlayRoot(t, root, map[string][]byte{path: raw}), m, "anchor is outside its test and directly referenced fixtures")
	})
	t.Run("package variable no longer referenced", func(t *testing.T) {
		root, m := loadLedger(t)
		const path = "xml_wellformedness_test.go"
		c := rowWith(m, "cases", "id", "TestXMLWellFormednessDecodePaths")
		raw := mutateTestBody(t, root, path, c["function"].(string), func(body string) string {
			return strings.Replace(body, "xmlWellFormednessCases", "nil", 1)
		})
		c["anchors"].([]any)[1].(map[string]any)["text"] = "runXMLReadingDecodePaths(t, nil, xmlReadingEncodingVariants, assertXMLReadingError)"
		// Exercise the structural guard directly, so the deliberately changed
		// anchor text cannot fall through to the independent case-policy pin.
		references := &ledgerReferences{files: make(map[string][]byte), documents: make(map[string]any)}
		err := references.validateCase(overlayRoot(t, root, map[string][]byte{path: raw}), c["id"].(string), c)
		if err == nil || !strings.Contains(err.Error(), "anchor is outside its test and directly referenced fixtures") {
			t.Fatalf("unreferenced package variable accepted or wrong rejection: %v", err)
		}
	})
}

func TestFinalReconciliationRejectsUnconditionalSkip(t *testing.T) {
	for _, skip := range []string{`t.Skip("disabled")`, `t.Skipf("disabled: %s", "fixture")`, `t.SkipNow()`} {
		t.Run(skip, func(t *testing.T) {
			root, m := loadLedger(t)
			const path = "validation_xsi_type_test.go"
			raw := mutateTestBody(t, root, path, "TestValidateXSIType", func(body string) string {
				return "{\n\t" + skip + body[1:]
			})
			rejectLedger(t, overlayRoot(t, root, map[string][]byte{path: raw}), m, "unconditional top-level test skip")
		})
	}
	t.Run("conditional skips remain permitted", func(t *testing.T) {
		root, m := loadLedger(t)
		const path = "validation_xsi_type_test.go"
		raw := mutateTestBody(t, root, path, "TestValidateXSIType", func(body string) string {
			return "{\n\tif false { t.Skip(\"optional tool unavailable\") }" + body[1:]
		})
		if err := validateLedger(overlayRoot(t, root, map[string][]byte{path: raw}), m); err != nil {
			t.Fatalf("conditional skip rejected: %v", err)
		}
	})
}

func mutateTestBody(t *testing.T, root, path, name string, mutate func(string) string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, path, raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Name.Name == name {
			start, end := fs.Position(fn.Body.Pos()).Offset, fs.Position(fn.Body.End()).Offset
			changed := mutate(string(raw[start:end]))
			if changed == string(raw[start:end]) {
				t.Fatal("test body mutation made no change")
			}
			return append(append(append([]byte{}, raw[:start]...), []byte(changed)...), raw[end:]...)
		}
	}
	t.Fatalf("missing mutation target %s", name)
	return nil
}

// Only replacement paths are copied. Other files remain read-only symlinks to
// the checkout, so mutations cannot change the author's working tree.
func overlayRoot(t *testing.T, root string, replacements map[string][]byte) string {
	t.Helper()
	absolute, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	overlay := t.TempDir()
	var populate func(string)
	populate = func(relative string) {
		entries, err := os.ReadDir(filepath.Join(absolute, relative))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.Name() == ".git" {
				continue
			}
			path := filepath.Join(relative, entry.Name())
			destination := filepath.Join(overlay, path)
			if data, ok := replacements[filepath.ToSlash(path)]; ok {
				if err := os.WriteFile(destination, data, 0600); err != nil {
					t.Fatal(err)
				}
				continue
			}
			nested := false
			for replacement := range replacements {
				if strings.HasPrefix(replacement, filepath.ToSlash(path)+"/") {
					nested = true
					break
				}
			}
			if nested {
				if err := os.Mkdir(destination, 0700); err != nil {
					t.Fatal(err)
				}
				populate(path)
			} else if err := os.Symlink(filepath.Join(absolute, path), destination); err != nil {
				t.Fatal(err)
			}
		}
	}
	populate("")
	return overlay
}
