package dictionarydata

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

var noticePaths = []string{
	"NOTICES.txt",
	"ipa-positive/NOTICES.txt",
	"ipa-positive/LICENSE-UNICODE-V3.txt",
	"ipa-representation/NOTICES.txt",
	"ipa-representation/LICENSE-UNICODE-V3.txt",
}

func TestVendoringRetainsExactDictionaryNotices(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	module, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var goVersion string
	for _, line := range strings.Split(string(module), "\n") {
		if strings.HasPrefix(line, "go ") {
			goVersion = strings.TrimPrefix(line, "go ")
		}
	}
	if goVersion == "" {
		t.Fatal("module has no Go version")
	}
	consumer := t.TempDir()
	files := map[string]string{
		"go.mod": fmt.Sprintf("module example.com/bundled-dictionary-consumer\n\ngo %s\n\nrequire github.com/go-muse/go-musicxml v0.0.0\n\nreplace github.com/go-muse/go-musicxml => %q\n", goVersion, root),
		// Even a consumer that never calls BundledDictionarySource vendors its data.
		"main.go": "package main\n\nimport _ \"github.com/go-muse/go-musicxml\"\n\nfunc main() {}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(consumer, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "mod", "vendor")
	cmd.Dir = consumer
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod vendor: %v\n%s", err, output)
	}
	for _, path := range noticePaths {
		want, err := os.ReadFile(filepath.Join(root, "testdata", "dictionaries", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		vendored := filepath.Join(consumer, "vendor", "github.com", "go-muse", "go-musicxml", "internal", "dictionarydata", "data", filepath.FromSlash(path))
		got, err := os.ReadFile(vendored)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("vendored notice %s differs from its original bytes: %v", path, err)
		}
	}
}

func TestEmbeddedDictionaryNoticesMatchOriginalBytes(t *testing.T) {
	for _, path := range noticePaths {
		want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "dictionaries", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		got, err := notices.ReadFile("data/" + path)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("embedded notice %s differs from its original bytes: %v", path, err)
		}
	}
}

// Notice names are deliberately discovered from the generated directory rather
// than another hand-maintained list. Every non-component output must be covered
// by the notice embed patterns, or go mod vendor would silently omit it.
func checkGeneratedDictionaryFilesEmbedded(data fs.FS) error {
	components := map[string]validation.DictionaryComponentID{
		"smufl-1.4/glyphnames.json":                             validation.DictionaryComponentSMuFL14Glyphnames,
		"smufl-1.4/ranges.json":                                 validation.DictionaryComponentSMuFL14Ranges,
		"musicxml-4.0/sounds.xml":                               validation.DictionaryComponentStandardSounds40,
		"ipa-positive/runtime-positive-bundle.json":             validation.DictionaryComponentIPAPositive2015Unicode13,
		"ipa-representation/runtime-representation-bundle.json": validation.DictionaryComponentIPARepresentation2015Unicode13,
	}
	return fs.WalkDir(data, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := fs.ReadFile(data, path)
		if err != nil {
			return err
		}
		var embedded []byte
		if id, ok := components[path]; ok {
			component, ok := Component(id)
			if !ok {
				return fmt.Errorf("generated dictionary file %s has no component %s", path, id)
			}
			embedded = []byte(component)
		} else {
			embedded, err = notices.ReadFile("data/" + path)
			if err != nil {
				return fmt.Errorf("generated dictionary file %s is not embedded: %w", path, err)
			}
		}
		if !bytes.Equal(raw, embedded) {
			return fmt.Errorf("generated dictionary file %s differs from embedded bytes", path)
		}
		return nil
	})
}

func TestGeneratedDictionaryFilesAreEmbedded(t *testing.T) {
	if err := checkGeneratedDictionaryFilesEmbedded(os.DirFS("data")); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedDictionaryFileGuardRejectsDrift(t *testing.T) {
	for _, tc := range []struct {
		name, path, want string
	}{
		{"unembedded notice", "ipa-positive/EXTRA-NOTICE.txt", "is not embedded"},
		{"notice bytes", "ipa-positive/NOTICES.txt", "differs from embedded bytes"},
		{"component bytes", "smufl-1.4/ranges.json", "differs from embedded bytes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("data")); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(tc.path)), []byte("temporary generated-file drift\n"), 0600); err != nil {
				t.Fatal(err)
			}
			err := checkGeneratedDictionaryFilesEmbedded(os.DirFS(root))
			if err == nil || !strings.Contains(err.Error(), tc.path+" "+tc.want) {
				t.Fatalf("generated-file drift was not rejected: %v", err)
			}
			t.Log(err)
		})
	}
}
