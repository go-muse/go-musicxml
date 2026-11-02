package musicxml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// windowedStride is the step through the corpus: every fourth file by default,
// which keeps the root package within its time limit on the slowest CI host,
// every twelfth with -short, and every file with MUSICXML_WINDOWED_FULL=1, the
// run each step of the windowed engine passes locally before it is pushed.
func windowedStride() int {
	switch {
	case os.Getenv("MUSICXML_WINDOWED_FULL") != "":
		return 1
	case testing.Short():
		return 12
	}
	return 4
}

// windowedRunIDs matches the identifiers a resource graph draws at random per
// run; they differ between any two runs, whatever the strategy.
var windowedRunIDs = regexp.MustCompile(`resource-run-[0-9a-f]+`)

// windowedReports assesses one document through every entry point family and
// returns the reports as JSON, with the per-run identifiers scrubbed.
func windowedReports(t *testing.T, name string, data []byte) map[string]string {
	t.Helper()
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34}
	out := map[string]string{}
	add := func(label string, value any, err error) {
		encoded, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatalf("%s %s: %v", name, label, marshalErr)
		}
		out[label] = windowedRunIDs.ReplaceAllString(string(encoded), "resource-run") + fmt.Sprintf(" error=%v", windowedRunIDs.ReplaceAllString(fmt.Sprint(err), "resource-run"))
	}
	for _, families := range []validation.ContextFamilies{0, validation.ContextFamilyAll} {
		suffix := fmt.Sprintf("/context=%b", families)
		result, err := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Lenient: true, Assessment: &SourceOptions{Budgets: big, Context: families}})
		add("DecodeAssessed"+suffix, []any{result.SourceLine, result.ModelLine, result.Conversion, result.Accepted}, err)
		packed, err := DecodeMXLAssessed(context.Background(), bytes.NewReader(ruleStepMXL(t, data)), DecodeMXLAssessmentOptions{Lenient: true, Assessment: MXLAssessmentOptions{Graph: MXLGraphOptions{Budgets: big, Context: families}}})
		add("DecodeMXLAssessed"+suffix, []any{packed.SourceLine, packed.ModelLine, packed.Conversion, packed.Accepted}, err)
	}
	// Tight diagnostic quotas truncate the stored findings; the default
	// rule-step limit makes any extra work after truncation visible.
	tight := validation.Budgets{MaxBufferedBytes: 1 << 34, MaxDiagnostics: 4, MaxDiagnosticBytes: 3000}
	result, err := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Lenient: true, Assessment: &SourceOptions{Budgets: tight}})
	add("DecodeAssessed/tight-diagnostics", []any{result.SourceLine, result.ModelLine, result.Conversion, result.Accepted}, err)
	packed, err := DecodeMXLAssessed(context.Background(), bytes.NewReader(ruleStepMXL(t, data)), DecodeMXLAssessmentOptions{Lenient: true, Assessment: MXLAssessmentOptions{Graph: MXLGraphOptions{Budgets: tight}}})
	add("DecodeMXLAssessed/tight-diagnostics", []any{packed.SourceLine, packed.ModelLine, packed.Conversion, packed.Accepted}, err)
	source, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceContext, Budgets: big, Context: validation.ContextFamilyAll})
	add("InspectXML/source-context", source, err)
	if document, err := Decode(bytes.NewReader(data)); err == nil {
		model, err := AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelContext, Budgets: big, Context: validation.ContextFamilyAll})
		add("AssessModel/model-context", model, err)
	}
	return out
}

// The windowed strategy assesses every document exactly as the buffered one:
// on every official and test-suite file, the strict loaders (XML and the same
// score packed as MXL, with and without the complete projection and under
// tight diagnostic quotas) and both context profiles return identical reports
// under both strategies. The two
// strategies share all code until the windowed engine lands; each streaming
// step is checked against this harness.
func TestWindowedStrategyMatchesBuffered(t *testing.T) {
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	var files []string
	for _, dir := range []string{"testdata/official", "testdata/corpus"} {
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".musicxml")) {
				files = append(files, path)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	if len(files) != 154 {
		t.Fatalf("corpus: %d files", len(files))
	}
	stride := windowedStride()
	compared := 0
	for i := 0; i < len(files); i += stride {
		data, err := os.ReadFile(files[i])
		if err != nil {
			t.Fatal(err)
		}
		previous := validationSetWindowedStrategy(false)
		buffered := windowedReports(t, files[i], data)
		validationSetWindowedStrategy(true)
		windowed := windowedReports(t, files[i], data)
		validationSetWindowedStrategy(previous)
		for label, want := range buffered {
			if windowed[label] != want {
				t.Errorf("%s %s: windowed report differs from buffered", files[i], label)
			}
			compared++
		}
	}
	if compared < 50 {
		t.Fatalf("compared %d reports", compared)
	}
}

// The canonical finding key reproduces the buffered order: on every official
// and test-suite file, through every entry point, the buffered engine stores
// findings in strictly increasing key order. The windowed engine sorts by this
// key.
func TestFindingKeysFollowInsertionOrder(t *testing.T) {
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	var files []string
	for _, dir := range []string{"testdata/official", "testdata/corpus"} {
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".musicxml")) {
				files = append(files, path)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	stride := windowedStride()
	reports, unordered, findings := 0, 0, 0
	t.Cleanup(func() { validationObserveFindingKeyOrder(nil) })
	for i := 0; i < len(files); i += stride {
		data, err := os.ReadFile(files[i])
		if err != nil {
			t.Fatal(err)
		}
		bad := 0
		validationObserveFindingKeyOrder(func(ordered bool, n int) {
			reports++
			findings += n
			if !ordered {
				unordered++
				bad++
			}
		})
		windowedReports(t, files[i], data)
		validationObserveFindingKeyOrder(nil)
		if bad != 0 {
			t.Errorf("%s: %d reports stored findings out of key order", files[i], bad)
		}
	}
	if reports < 100 || findings < 100 {
		t.Fatalf("observed %d reports with %d findings", reports, findings)
	}
	t.Logf("%d reports, %d findings, %d out of order", reports, findings, unordered)
}

// Under the windowed strategy every measure is one evaluation window: through
// both validation lines, on the official and test-suite files, the windows
// close in document order without overlapping, there is one per measure of a
// partwise score, and the nodes outside them, the skeleton that stays until
// Finish, are the header and the parts. The buffered strategy opens none.
func TestWindowsCoverMeasures(t *testing.T) {
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	var files []string
	for _, dir := range []string{"testdata/official", "testdata/corpus"} {
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".musicxml")) {
				files = append(files, path)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	measure := regexp.MustCompile(`<measure[\s>/]`)
	partwise := regexp.MustCompile(`<score-partwise[\s>]`)
	type window struct{ first, end int }
	var windows []window
	total := -1
	t.Cleanup(func() { validationSetWindowObserver(nil) })
	observe := func(root validation.NodeID, first, end int) {
		if root == 0 {
			total = end
			return
		}
		windows = append(windows, window{first, end})
	}
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34}
	checked, nodes, skeleton := 0, 0, 0
	for i := 0; i < len(files); i += windowedStride() {
		data, err := os.ReadFile(files[i])
		if err != nil {
			t.Fatal(err)
		}
		document, err := Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		measures := len(measure.FindAll(data, -1))
		for _, line := range []string{"source", "model"} {
			for _, windowed := range []bool{false, true} {
				windows, total = nil, -1
				previousStrategy := validationSetWindowedStrategy(windowed)
				validationSetWindowObserver(observe)
				if line == "source" {
					_, err = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceLine, Budgets: big})
				} else {
					_, err = AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelSemantics, Budgets: big})
				}
				validationSetWindowObserver(nil)
				validationSetWindowedStrategy(previousStrategy)
				if err != nil {
					t.Fatalf("%s %s: %v", files[i], line, err)
				}
				if !windowed {
					if len(windows) != 0 || total != -1 {
						t.Fatalf("%s %s: buffered strategy observed %d windows", files[i], line, len(windows))
					}
					continue
				}
				if partwise.Match(data) && len(windows) != measures {
					t.Errorf("%s %s: %d windows for %d measures", files[i], line, len(windows), measures)
				}
				covered := 0
				for k, w := range windows {
					if w.first >= w.end || (k > 0 && w.first < windows[k-1].end) || w.end > total {
						t.Fatalf("%s %s: window %d [%d,%d) out of order (total %d)", files[i], line, k, w.first, w.end, total)
					}
					covered += w.end - w.first
				}
				nodes += total
				skeleton += total - covered
				checked++
			}
		}
	}
	if checked < 20 {
		t.Fatalf("checked %d runs", checked)
	}
	t.Logf("%d windowed runs, %d nodes, %d in the skeleton", checked, nodes, skeleton)
}
