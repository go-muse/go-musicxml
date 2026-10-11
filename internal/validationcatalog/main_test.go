package main

import (
	"bytes"
	"encoding/json"
	"os"
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
	committed, err := os.ReadFile(filepath.Join("..", "..", "validation", "zz_generated_inventory.go"))
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
