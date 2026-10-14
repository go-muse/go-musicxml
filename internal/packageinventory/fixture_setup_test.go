package packageinventory

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestMaintenanceClonePreservesIndependentNestedValues(t *testing.T) {
	original := map[string]any{
		"unknown": map[string]any{
			"values": []any{
				map[string]any{"number": json.Number("123456789012345678901234567890.123456789e+42")},
				[]any{true, nil, "text", json.Number("-0.00e-003")},
				[]byte{1, 2, 3},
			},
		},
		"empty_map":   map[string]any{},
		"empty_array": []any{},
		"nil_map":     map[string]any(nil),
		"nil_array":   []any(nil),
		"nil_bytes":   []byte(nil),
	}
	clone := maintenanceClone(t, original)
	if !reflect.DeepEqual(clone, original) {
		t.Fatalf("clone changed types, nil values, or unknown numeric tokens: got %#v", clone)
	}
	values := ma(mo(clone["unknown"])["values"])
	mo(values[0])["number"] = json.Number("7")
	ma(values[1])[0] = false
	values[2].([]byte)[0] = 9
	mo(clone["unknown"])["added"] = "clone only"
	mo(clone["empty_map"])["added"] = true
	before := ma(mo(original["unknown"])["values"])
	maintenanceEqual(t, mo(before[0])["number"], json.Number("123456789012345678901234567890.123456789e+42"))
	maintenanceEqual(t, ma(before[1]), []any{true, nil, "text", json.Number("-0.00e-003")})
	maintenanceEqual(t, before[2], []byte{1, 2, 3})
	if _, exists := mo(original["unknown"])["added"]; exists || len(mo(original["empty_map"])) != 0 {
		t.Fatal("cloned maps alias the original")
	}
}

func TestMaintenanceClonePreservesUnknownNumberTokens(t *testing.T) {
	original := maintenanceJSON(t, []byte(`{"unknown":{"values":[9007199254740993,1.2300e+004,-0.00e-003]}}`))
	clone := maintenanceClone(t, original)
	maintenanceEqual(t, clone, original)
	maintenanceEqual(t, maintenanceEncode(t, clone), maintenanceEncode(t, original))
}

func TestMaintenanceFixtureIsolation(t *testing.T) {
	first, second := newMaintenanceFixture(t), newMaintenanceFixture(t)
	if first.root == second.root {
		t.Fatal("fixtures share their temporary filesystem")
	}
	manifest, evidence := second.read(maintenanceManifest), second.read(maintenanceEvidence)
	manifestValues, evidenceValues := maintenanceClone(t, second.m), maintenanceClone(t, second.e)
	path := "link_intent_test.go"
	source := bytes.Clone(second.originals[path])
	mo(ma(mo(first.m["selected"])["requirements"])[0])["owner"] = "fixture-local owner"
	mo(mo(first.e["tests"])[maintenanceFirst(mo(first.e["tests"]))])["file_sha256"] = "fixture-local hash"
	first.originals[path][0] ^= 1
	first.write(path, []byte("fixture-local source file"))
	first.save()
	maintenanceEqual(t, second.m, manifestValues)
	maintenanceEqual(t, second.e, evidenceValues)
	maintenanceEqual(t, second.originals[path], source)
	maintenanceEqual(t, second.read(path), source)
	maintenanceEqual(t, second.read(maintenanceManifest), manifest)
	maintenanceEqual(t, second.read(maintenanceEvidence), evidence)
	third := newMaintenanceFixture(t)
	maintenanceEqual(t, third.m, manifestValues)
	maintenanceEqual(t, third.e, evidenceValues)
	maintenanceEqual(t, third.originals[path], source)
	maintenanceEqual(t, third.read(path), source)
	maintenanceEqual(t, third.read(maintenanceManifest), manifest)
	maintenanceEqual(t, third.read(maintenanceEvidence), evidence)
	// Initial cached encodings must be identical to writing each decoded value.
	maintenanceEqual(t, manifest, maintenanceEncode(t, second.m))
	maintenanceEqual(t, evidence, maintenanceEncode(t, second.e))
	maintenanceEqual(t, maintenanceRead(t, filepath.Join(maintenanceRepo(t), path)), source)
}

func TestMaintenanceParallelFixtureIsolation(t *testing.T) {
	for i := 0; i < 4; i++ {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			t.Parallel()
			f := newMaintenanceFixture(t)
			f.m["fixture_local"] = t.Name()
			f.e["fixture_local"] = t.Name()
			for path, data := range f.originals {
				if len(data) > 0 {
					data[0] ^= 1
					f.write(path, data)
				}
			}
			f.save()
			maintenanceEqual(t, maintenanceJSON(t, f.read(maintenanceManifest))["fixture_local"], t.Name())
		})
	}
}

func maintenanceTinyBaseline(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	maintenanceWrite(t, filepath.Join(repo, maintenanceManifest), []byte(`{"source_files":{"source.json":{}},"reviewed_source_corrections":[{"reconciliation":"review.json"}],"unknown":1.2300e+004}`))
	maintenanceWrite(t, filepath.Join(repo, maintenanceEvidence), []byte(`{"tests":{"first":{"file":"input_test.go"},"second":{"file":"input_test.go"}}}`))
	for _, path := range []string{"source.json", "review.json", "input_test.go"} {
		maintenanceWrite(t, filepath.Join(repo, path), []byte(path))
	}
	return repo
}

func TestMaintenanceBaselineLoadsOriginalsOnlyOnce(t *testing.T) {
	repo := maintenanceTinyBaseline(t)
	var cache maintenanceFixtureCache
	baseline, err := cache.load(repo)
	if err != nil {
		t.Fatal(err)
	}
	maintenanceEqual(t, baseline.m["unknown"], json.Number("1.2300e+004"))
	maintenanceEqual(t, len(baseline.originals), 3)
	for path, data := range baseline.originals {
		maintenanceEqual(t, data, []byte(path))
	}
	// Removing the originals after the first load proves later fixtures need
	// neither to reread nor reparse the repository's snapshot.
	if err := os.RemoveAll(repo); err != nil {
		t.Fatal(err)
	}
	again, err := cache.load(repo)
	if err != nil || again != baseline {
		t.Fatalf("baseline was reloaded: got %p, want %p; error %v", again, baseline, err)
	}
	maintenanceEqual(t, baseline.manifest, maintenanceEncode(t, baseline.m))
	maintenanceEqual(t, baseline.evidence, maintenanceEncode(t, baseline.e))
}

func TestMaintenanceBaselineLoadFailureRemainsAnError(t *testing.T) {
	for _, tc := range []struct{ name, path, content string }{
		{"missing manifest", maintenanceManifest, ""},
		{"invalid manifest", maintenanceManifest, "{"},
		{"null manifest", maintenanceManifest, "null"},
		{"invalid evidence", maintenanceEvidence, "{"},
		{"missing source", "source.json", ""},
		{"invalid source map", maintenanceManifest, `{"source_files":[]}`},
		{"invalid corrections", maintenanceManifest, `{"source_files":{},"reviewed_source_corrections":{}}`},
		{"invalid correction record", maintenanceManifest, `{"source_files":{},"reviewed_source_corrections":[null]}`},
		{"invalid correction path", maintenanceManifest, `{"source_files":{},"reviewed_source_corrections":[{"reconciliation":3}]}`},
		{"invalid tests map", maintenanceEvidence, `{"tests":[]}`},
		{"invalid test record", maintenanceEvidence, `{"tests":{"x":false}}`},
		{"empty test path", maintenanceEvidence, `{"tests":{"x":{"file":""}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := maintenanceTinyBaseline(t)
			path := filepath.Join(repo, tc.path)
			original := maintenanceRead(t, path)
			if tc.content == "" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				maintenanceWrite(t, path, []byte(tc.content))
			}
			var cache maintenanceFixtureCache
			baseline, err := cache.load(repo)
			if err == nil || baseline != nil {
				t.Fatalf("failed load = (%p, %v), want (nil, error)", baseline, err)
			}
			maintenanceWrite(t, path, original)
			again, againErr := cache.load(repo)
			if again != nil || againErr != err {
				t.Fatalf("repeated failed load = (%p, %v), want same error %v", again, againErr, err)
			}
		})
	}
}

func TestMaintenanceBaselineConcurrentFirstLoad(t *testing.T) {
	repo := maintenanceTinyBaseline(t)
	var cache maintenanceFixtureCache
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Go(func() {
			baseline, err := cache.load(repo)
			if err != nil {
				t.Error(err)
				return
			}
			if len(baseline.originals) != 3 || baseline.m["unknown"] != json.Number("1.2300e+004") {
				t.Error("concurrent load observed incomplete baseline")
			}
		})
	}
	group.Wait()
}
