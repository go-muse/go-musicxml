package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGenerateInventoryReproducible(t *testing.T) {
	planDir := filepath.Join("..", "..", "docs", "validation", "plan")
	first, err := generate(planDir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generate(planDir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("generation is not deterministic")
	}
	committed, err := os.ReadFile(filepath.Join("..", "..", "validation", "inventory", "zz_generated_inventory.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, committed) {
		t.Fatal("compiled inventory is stale; run go generate ./...")
	}
}

func TestGenerateInventoryRejectsInvalidPlans(t *testing.T) {
	tests := []struct {
		name, file string
		mutate     func(map[string]any)
	}{
		{"registry_repository", "external-snapshot.json", func(d map[string]any) { d["registry_repository"] = "https://example.invalid/registry" }},
		{"upstream_pin", "external-snapshot.json", func(d map[string]any) { d["upstream_musicxml_commit"] = "main" }},
		{"external_digest", "external-snapshot.json", func(d map[string]any) { d["files"].([]any)[0].(map[string]any)["sha256"] = "changed" }},

		{"registry_pin", "external-snapshot.json", func(d map[string]any) { d["registry_commit"] = "main" }},
		{"missing_record", "research-map.json", func(d map[string]any) { d["research_records"] = d["research_records"].([]any)[1:] }},
		{"swapped_pointer", "research-map.json", func(d map[string]any) {
			d["research_records"].([]any)[0].(map[string]any)["registry_pointer"] = "/records/1/record"
		}},
		{"dangling_capability", "prose-contracts.json", func(d map[string]any) { d["checks"].([]any)[0].(map[string]any)["capability_ids"] = []any{"missing"} }},
		{"provider_not_unbound", "capabilities.json", func(d map[string]any) {
			d["capability_gates"].([]any)[0].(map[string]any)["provider_status"] = "implemented"
		}},
		{"schema_edge_lost", "xsd-contracts.json", func(d map[string]any) {
			d["instances"].([]any)[0].(map[string]any)["plan_dependencies"].(map[string]any)["language_semantics"] = []any{}
		}},
		{"claimed_execution", "xsd-test-contracts.json", func(d map[string]any) { d["test_contracts"].([]any)[0].(map[string]any)["status"] = "passed" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyInventoryPlan(t)
			path := filepath.Join(dir, tc.file)
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var d map[string]any
			if err = json.Unmarshal(b, &d); err != nil {
				t.Fatal(err)
			}
			tc.mutate(d)
			b, err = json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = generate(dir); err == nil {
				t.Fatal("invalid source plan accepted")
			}
		})
	}
}

func TestGenerateInventoryRejectsDuplicateJSONKeys(t *testing.T) {
	dir := copyInventoryPlan(t)
	path := filepath.Join(dir, "research-map.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b = bytes.Replace(b, []byte(`"research_id":`), []byte(`"research_id":"invented", "research_id":`), 1)
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = generate(dir); err == nil {
		t.Fatal("duplicate JSON keys accepted")
	}
}

func copyInventoryPlan(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "plan")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join("..", "..", "docs", "validation", "plan", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, filepath.Base(path)), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "validation", "deferred-inventory.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "deferred-inventory.md"), b, 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPlanningCommandIsLocalAndOffline(t *testing.T) {
	cmd, err := planningCommand(filepath.Join("..", "..", "docs", "validation", "plan"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Args) != 5 || cmd.Args[1] != "run" || cmd.Args[2] != "./internal/validationplan" || !filepath.IsAbs(cmd.Args[4]) {
		t.Fatalf("validator command is not local and path-stable: %#v", cmd.Args)
	}
	if !filepath.IsAbs(cmd.Dir) {
		t.Fatalf("validator module root is not absolute: %q", cmd.Dir)
	}
	for _, setting := range []string{"GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local"} {
		found := false
		for _, actual := range cmd.Env {
			found = found || actual == setting
		}
		if !found {
			t.Errorf("validator missing offline setting %s", setting)
		}
	}
}

// This uses Git's checkout conversion, rather than manually replacing newlines:
// exact-byte source hashes must survive the checkout policy used on Windows.
func TestGenerateInventoryAutocrlfCheckout(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("Git is required to exercise real checkout conversion")
	}
	root := t.TempDir()
	runGit := func(args ...string) []byte {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v: %v; output: %s", args, err, out)
		}
		return out
	}
	runGit("init", "--quiet")
	write := func(name string, data []byte) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	copySource := func(name string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		write(name, data)
	}
	copySource(".gitattributes")
	copySource("docs/validation/deferred-inventory.md")
	files, err := filepath.Glob(filepath.Join("..", "..", "docs", "validation", "plan", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 11 {
		t.Fatalf("expected 11 reviewed planning inputs, got %d", len(files))
	}
	var paths []string
	for _, file := range files {
		name := "docs/validation/plan/" + filepath.Base(file)
		paths = append(paths, name)
		copySource(name)
	}
	write("checkout-control.txt", []byte("unprotected text\nsecond line\n"))
	// Populate an index without creating a commit or changing the real repository.
	runGit("-c", "core.autocrlf=false", "add", "--all")
	indexed := map[string][]byte{}
	for _, path := range paths {
		indexed[path] = runGit("show", ":"+path)
	}
	// Recreate every tracked file: checkout-index can otherwise skip a file
	// whose index stat information still matches, even with --force.
	for _, path := range append(append([]string{}, paths...), "checkout-control.txt", ".gitattributes", "docs/validation/deferred-inventory.md") {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatal(err)
		}
	}
	runGit("-c", "core.autocrlf=true", "-c", "core.safecrlf=false", "checkout-index", "--all", "--force")
	control, err := os.ReadFile(filepath.Join(root, "checkout-control.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(control, []byte("unprotected text\r\nsecond line\r\n")) {
		t.Fatalf("checkout control did not undergo CRLF conversion: %q", control)
	}
	for _, path := range paths {
		actual, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(actual, indexed[path]) {
			t.Errorf("autocrlf changed committed planning bytes: %s (index %d bytes, checkout %d)", path, len(indexed[path]), len(actual))
		}
	}
	generated, err := generate(filepath.Join(root, "docs", "validation", "plan"))
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(filepath.Join("..", "..", "validation", "inventory", "zz_generated_inventory.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, committed) {
		t.Error("autocrlf checkout changed generated inventory; preserve exact source bytes instead of normalizing provenance hashes")
	}
}
