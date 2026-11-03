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
// partwise score, every window is bound to the schema when it closes, and the
// nodes outside them, the skeleton that stays until Finish, are the header and
// the parts. The buffered strategy opens and binds none.
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
	var bound int
	t.Cleanup(func() { validationSetWindowObserver(nil); validationSetWindowBindObserver(nil) })
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
				windows, total, bound = nil, -1, 0
				previousStrategy := validationSetWindowedStrategy(windowed)
				validationSetWindowObserver(observe)
				validationSetWindowBindObserver(func(validation.NodeID) { bound++ })
				if line == "source" {
					_, err = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceLine, Budgets: big})
				} else {
					_, err = AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelSemantics, Budgets: big})
				}
				validationSetWindowObserver(nil)
				validationSetWindowBindObserver(nil)
				validationSetWindowedStrategy(previousStrategy)
				if err != nil {
					t.Fatalf("%s %s: %v", files[i], line, err)
				}
				if !windowed {
					if len(windows) != 0 || total != -1 || bound != 0 {
						t.Fatalf("%s %s: buffered strategy observed %d windows", files[i], line, len(windows))
					}
					continue
				}
				if partwise.Match(data) && len(windows) != measures {
					t.Errorf("%s %s: %d windows for %d measures", files[i], line, len(windows), measures)
				}
				if bound != len(windows) {
					t.Errorf("%s %s: %d of %d windows bound when they closed", files[i], line, bound, len(windows))
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

// windowedRiskScore is a two-part partwise score whose elements carry IDs.
const windowedRiskScore = `<?xml version="1.0" encoding="UTF-8"?>
<score-partwise version="4.0" ROOTATTR>
  <work><work-title id="w1">Risk</work-title></work>
  <part-list><score-part id="P1"><part-name>A</part-name></score-part><score-part id="P2"><part-name>B</part-name></score-part></part-list>
  <part id="P1" PART1ATTR>
    <measure number="1" id="m1"><attributes><divisions>1</divisions><time><beats>4</beats><beat-type>4</beat-type></time></attributes><note id="n1"><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration><type>whole</type></note></measure>
    <measure number="2" id="M2ID"><attributes><divisions>DIV2</divisions></attributes><note id="n2"><rest/><duration>DUR2</duration></note>MIDDLE</measure>
    <measure number="3" id="m3"><note id="n3"><rest/><duration>4</duration></note></measure>
    TRAILING
  </part>
  <part id="PART2ID">
    <measure number="1" id="q1"><attributes><divisions>2</divisions></attributes><note id="NOTEID"><rest/><duration>8</duration></note></measure>
    <measure number="2" id="q2"><note id="o2"><rest/><duration>8</duration></note></measure>
    <measure number="3" id="q3"><note id="o3"><rest/><duration>8</duration></note></measure>
  </part>
  ROOTTRAILING
</score-partwise>`

// windowedTimewiseScore is a two-part timewise score.
const windowedTimewiseScore = `<?xml version="1.0" encoding="UTF-8"?>
<score-timewise version="4.0">
  <part-list><score-part id="P1"><part-name>A</part-name></score-part><score-part id="P2"><part-name>B</part-name></score-part></part-list>
  <measure number="1" id="m1"><part id="P1"><attributes><divisions>1</divisions></attributes><note id="a"><rest/><duration>4</duration></note></part><part id="P2"><attributes><divisions>2</divisions></attributes><note id="b"><rest/><duration>8</duration></note></part></measure>
  <measure number="2" id="m2"><part id="P1"><note id="TWID"><rest/><duration>4</duration></note></part><part id="P2"><attributes><divisions>4</divisions></attributes><note id="d"><rest/><duration>16</duration></note>TWMIDDLE</part></measure>
  TWTRAILING
</score-timewise>`

// The windowed strategy matches the buffered one on documents chosen for the
// risks of binding windows before the root closes: IDs on every level with
// duplicates across the header, measures and parts, a dangling part IDREF,
// divisions changing inside a part, content-model errors inside a measure,
// after the last measure of a part and after the last part, a schema-instance
// attribute on the root or a part, a timewise score, and input cut off
// inside a measure. The buffered engine stores their findings in key order.
func TestWindowedStrategyMatchesBufferedOnRiskCases(t *testing.T) {
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	base := map[string]string{"ROOTATTR": "", "PART1ATTR": "", "M2ID": "m2", "DIV2": "1", "DUR2": "4", "MIDDLE": "", "TRAILING": "", "PART2ID": "P2", "NOTEID": "o1", "ROOTTRAILING": ""}
	timewise := map[string]string{"TWID": "c", "TWMIDDLE": "", "TWTRAILING": ""}
	render := func(text string, values map[string]string, changes map[string]string) []byte {
		for key, value := range values {
			if changed, ok := changes[key]; ok {
				value = changed
			}
			text = strings.ReplaceAll(text, key, value)
		}
		return []byte(text)
	}
	cases := map[string][]byte{
		"valid":                         render(windowedRiskScore, base, nil),
		"duplicate IDs across levels":   render(windowedRiskScore, base, map[string]string{"M2ID": "w1", "NOTEID": "n1"}),
		"duplicate ID of a score part":  render(windowedRiskScore, base, map[string]string{"M2ID": "P1", "NOTEID": "P2"}),
		"dangling part IDREF":           render(windowedRiskScore, base, map[string]string{"PART2ID": "P9"}),
		"divisions change":              render(windowedRiskScore, base, map[string]string{"DIV2": "4", "DUR2": "16"}),
		"error inside a measure":        render(windowedRiskScore, base, map[string]string{"MIDDLE": "<bogus/>"}),
		"error after the last measure":  render(windowedRiskScore, base, map[string]string{"TRAILING": "<bogus/>"}),
		"measure-level element in part": render(windowedRiskScore, base, map[string]string{"TRAILING": "<note><rest/><duration>4</duration></note><measure number=\"4\"/>"}),
		"error after the last part":     render(windowedRiskScore, base, map[string]string{"ROOTTRAILING": "<bogus/>"}),
		"schema instance on the root":   render(windowedRiskScore, base, map[string]string{"ROOTATTR": `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="musicxml.xsd"`}),
		"schema instance on a part":     render(windowedRiskScore, base, map[string]string{"ROOTATTR": `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`, "PART1ATTR": `xsi:type="part"`}),
		"timewise":                      render(windowedTimewiseScore, timewise, nil),
		"timewise duplicate ID":         render(windowedTimewiseScore, timewise, map[string]string{"TWID": "m1"}),
		"timewise errors":               render(windowedTimewiseScore, timewise, map[string]string{"TWMIDDLE": "<bogus/>", "TWTRAILING": "<bogus/>"}),
	}
	valid := cases["valid"]
	cases["cut inside a measure"] = valid[:bytes.Index(valid, []byte(`<measure number="2" id="q2">`))+20]
	t.Cleanup(func() { validationObserveFindingKeyOrder(nil) })
	for name, data := range cases {
		previous := validationSetWindowedStrategy(false)
		unordered := 0
		validationObserveFindingKeyOrder(func(ordered bool, _ int) {
			if !ordered {
				unordered++
			}
		})
		buffered := windowedReports(t, name, data)
		validationObserveFindingKeyOrder(nil)
		if unordered != 0 {
			t.Errorf("%s: %d buffered reports stored findings out of key order", name, unordered)
		}
		validationSetWindowedStrategy(true)
		windowed := windowedReports(t, name, data)
		validationSetWindowedStrategy(previous)
		for label, want := range buffered {
			if got := windowed[label]; got != want {
				t.Errorf("%s %s: windowed report differs from buffered at %s", name, label, windowedDifference(want, got))
			}
		}
	}
}

// windowedDifference shows both texts around their first difference.
func windowedDifference(want, got string) string {
	i := 0
	for i < len(want) && i < len(got) && want[i] == got[i] {
		i++
	}
	from := max(0, i-200)
	return fmt.Sprintf("byte %d:\nbuffered ...%s\nwindowed ...%s", i, want[from:min(len(want), i+200)], got[from:min(len(got), i+200)])
}
