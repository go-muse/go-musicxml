package dictionarygen

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

var componentPaths = []string{
	"smufl-1.4/glyphnames.json", "smufl-1.4/ranges.json", "musicxml-4.0/sounds.xml",
	"ipa-positive/runtime-positive-bundle.json", "ipa-representation/runtime-representation-bundle.json",
}
var noticePaths = []string{
	"NOTICES.txt", "ipa-positive/NOTICES.txt", "ipa-positive/LICENSE-UNICODE-V3.txt",
	"ipa-representation/NOTICES.txt", "ipa-representation/LICENSE-UNICODE-V3.txt",
}

func inputRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, path := range append(append([]string(nil), componentPaths...), noticePaths...) {
		raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "dictionaries", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(root, "testdata", "dictionaries", filepath.FromSlash(path)), raw)
	}
	return root
}
func writeTestFile(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
}
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[path] = string(raw)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestGeneratePinnedDictionaryAssetsOfflineAndDeterministic(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	root := inputRoot(t)
	if err := Generate(root); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "internal", "dictionarydata", "data")
	for _, path := range append(append([]string(nil), componentPaths...), noticePaths...) {
		want, err := os.ReadFile(filepath.Join(root, "testdata", "dictionaries", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(path)))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s changed: %v", path, err)
		}
	}
	before := snapshot(t, output)
	if len(before) != len(componentPaths)+len(noticePaths) {
		t.Fatalf("generated files=%d", len(before))
	}
	if err := Generate(root); err != nil {
		t.Fatal(err)
	}
	after := snapshot(t, output)
	if len(after) != len(before) {
		t.Fatal("nondeterministic output count")
	}
	for path, raw := range before {
		if after[path] != raw {
			t.Fatalf("nondeterministic %s", path)
		}
	}
}
func TestGenerateRejectsInputBeforeMutatingAssets(t *testing.T) {
	for _, path := range componentPaths {
		for _, change := range []string{"digest", "short", "long", "missing"} {
			t.Run(path+"/"+change, func(t *testing.T) {
				root := inputRoot(t)
				if err := Generate(root); err != nil {
					t.Fatal(err)
				}
				output := filepath.Join(root, "internal", "dictionarydata", "data")
				// Force an earlier output to need replacement; validation must finish first.
				writeTestFile(t, filepath.Join(output, filepath.FromSlash(componentPaths[0])), []byte("keep unchanged on failure"))
				before := snapshot(t, output)
				input := filepath.Join(root, "testdata", "dictionaries", filepath.FromSlash(path))
				raw, err := os.ReadFile(input)
				if err != nil {
					t.Fatal(err)
				}
				switch change {
				case "digest":
					raw[len(raw)/2] ^= 1
				case "short":
					raw = raw[:len(raw)-1]
				case "long":
					raw = append(raw, '\n')
				case "missing":
					if err := os.Remove(input); err != nil {
						t.Fatal(err)
					}
				}
				if change != "missing" {
					writeTestFile(t, input, raw)
				}
				if err := Generate(root); err == nil {
					t.Fatal("accepted input outside pin")
				}
				after := snapshot(t, output)
				if len(after) != len(before) {
					t.Fatal("failure changed output count")
				}
				for path, raw := range before {
					if after[path] != raw {
						t.Fatalf("failure mutated %s", path)
					}
				}
			})
		}
	}
}
func TestGenerateMissingNoticeLeavesOutputAbsent(t *testing.T) {
	root := inputRoot(t)
	if err := os.Remove(filepath.Join(root, "testdata", "dictionaries", "ipa-representation", "NOTICES.txt")); err != nil {
		t.Fatal(err)
	}
	if err := Generate(root); err == nil {
		t.Fatal("generated data without its notices")
	}
	if _, err := os.Stat(filepath.Join(root, "internal")); !os.IsNotExist(err) {
		t.Fatalf("created outputs on rejected input: %v", err)
	}
}
