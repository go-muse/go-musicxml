package dictionarydata

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
