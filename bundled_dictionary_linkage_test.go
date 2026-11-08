package musicxml

import (
	"bytes"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestBundledDictionaryLinkage backs the documented list of entry points whose
// binaries embed the bundled dictionary data: each is linked into its own
// program, and a window of an embedded component must appear exactly in the
// binaries that reference one of them.
func TestBundledDictionaryLinkage(t *testing.T) {
	if testing.Short() {
		t.Skip("builds programs")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool unavailable")
	}
	// One window of each of the five embedded components.
	var markers [][]byte
	for _, file := range []string{"smufl-1.4/glyphnames.json", "smufl-1.4/ranges.json", "musicxml-4.0/sounds.xml", "ipa-positive/runtime-positive-bundle.json", "ipa-representation/runtime-representation-bundle.json"} {
		component, err := os.ReadFile(filepath.Join("internal", "dictionarydata", "data", filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		markers = append(markers, component[len(component)/2:len(component)/2+64])
	}

	calls := map[string]bool{
		`musicxml.DecodeAssessed(ctx, r, musicxml.DecodeAssessmentOptions{})`:                                    true,
		`musicxml.DecodeMXLAssessed(ctx, r, musicxml.DecodeMXLAssessmentOptions{})`:                              true,
		`musicxml.InspectXML(ctx, r, musicxml.SourceOptions{})`:                                                  true,
		`musicxml.AssessModel(ctx, musicxml.NewOpusDocument(), musicxml.ModelOptions{})`:                         true,
		`musicxml.InspectXMLWithResources(ctx, r, musicxml.LinkedAssessmentOptions{})`:                           true,
		`musicxml.AssessModelWithResources(ctx, musicxml.NewOpusDocument(), musicxml.LinkedAssessmentOptions{})`: true,
		`musicxml.InspectMXL(ctx, r, musicxml.MXLAssessmentOptions{})`:                                           true,
		`musicxml.AssessMXLPackage(ctx, &musicxml.MXLPackage{}, musicxml.MXLAssessmentOptions{})`:                true,
		`musicxml.Decode(r)`:    false,
		`musicxml.DecodeMXL(r)`: false,
		`musicxml.Validate(musicxml.NewOpusDocument()), error(nil)`:           false,
		`musicxml.Encode(io.Discard, musicxml.NewOpusDocument()), error(nil)`: false,
	}
	root, err := os.MkdirTemp(".", "_linkage_probe_")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	// The programs build in parallel; the build cache is safe for that.
	ordered := slices.Sorted(maps.Keys(calls))
	inParallel(t, "programs", len(ordered), func(i int) string { name, _, _ := strings.Cut(ordered[i], "("); return name }, func(t *testing.T, i int) {
		call, embeds := ordered[i], calls[ordered[i]]
		dir := filepath.Join(root, "p"+strings.Repeat("x", i+1))
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		program := "package main\n\nimport (\n\t\"context\"\n\t\"io\"\n\t\"strings\"\n\n\t\"github.com/go-muse/go-musicxml\"\n)\n\n" +
			"func main() {\n\tctx := context.Background()\n\tr := strings.NewReader(\"<opus/>\")\n\t_, _, _ = ctx, r, io.Discard\n\t_, _ = " + call + "\n}\n"
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(program), 0o644); err != nil {
			t.Fatal(err)
		}
		binary := filepath.Join(t.TempDir(), "probe")
		output, err := exec.Command("go", "build", "-o", binary, "./"+filepath.ToSlash(dir)).CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", call, err, output)
		}
		linked, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		for component, marker := range markers {
			if got := bytes.Contains(linked, marker); got != embeds {
				t.Errorf("%s: embeds bundled dictionary component %d = %v, want %v", call, component, got, embeds)
			}
		}
	})
}
