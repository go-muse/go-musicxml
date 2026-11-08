package musicxml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// typedReleaseReports adds to windowedReports the profiles whose rules read no
// typed XSD result once binding completes, which therefore release them: the
// source line, the XSD preview, the internal-DTD source profile, model XSD
// and Validate.
func typedReleaseReports(t *testing.T, name string, data []byte) map[string]string {
	t.Helper()
	out := windowedReports(t, name, data)
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34}
	add := func(label string, value any, err error) {
		encoded, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		out[label] = string(encoded) + fmt.Sprintf(" error=%v", err)
	}
	for _, profile := range []SourceProfile{ProfileSourceLine, ProfileSourceXSDPreview, ProfileSourceInternalDTD} {
		report, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: profile, Budgets: big})
		add("InspectXML/"+string(profile), report, err)
	}
	if document, err := Decode(bytes.NewReader(data)); err == nil {
		report, err := AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelXSD, Budgets: big})
		add("AssessModel/"+string(ProfileModelXSD), report, err)
		add("Validate", nil, Validate(document))
	}
	return out
}

// typedReleaseCases are documents chosen for what typed XSD results carry:
// schema hints on the root, whose readers read the XSD results of the root,
// attribute values that fail their type, defaulted and absent attributes,
// element content that fails its type, and IDs.
func typedReleaseCases() map[string][]byte {
	base := map[string]string{"CREDITS": "", "ROOTATTR": "", "PART1ATTR": "", "M2ID": "m2", "DIV2": "1", "DUR2": "4", "MIDDLE": "", "TRAILING": "", "PART2ID": "P2", "NOTEID": "o1", "ROOTTRAILING": ""}
	render := func(changes map[string]string) []byte {
		text := windowedRiskScore
		for key, value := range base {
			if changed, ok := changes[key]; ok {
				value = changed
			}
			text = strings.ReplaceAll(text, key, value)
		}
		return []byte(text)
	}
	xsi := `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" `
	return map[string][]byte{
		"valid":                    render(nil),
		"schema location hint":     render(map[string]string{"ROOTATTR": xsi + `xsi:schemaLocation="http://www.musicxml.org/ns musicxml.xsd"`}),
		"no-namespace hint":        render(map[string]string{"ROOTATTR": xsi + `xsi:noNamespaceSchemaLocation="musicxml.xsd"`}),
		"malformed hint":           render(map[string]string{"ROOTATTR": xsi + `xsi:schemaLocation="only-one-token"`}),
		"invalid attribute value":  render(map[string]string{"MIDDLE": `<note default-x="wide" print-object="maybe"><rest/><duration>1</duration></note>`}),
		"defaulted attribute":      render(map[string]string{"MIDDLE": `<direction><direction-type><dynamics><p/></dynamics></direction-type><sound tempo="120"/></direction>`}),
		"invalid element content":  render(map[string]string{"DUR2": "four"}),
		"duplicate and dangling":   render(map[string]string{"M2ID": "w1", "PART2ID": "P9"}),
		"content error in measure": render(map[string]string{"MIDDLE": "<bogus/>"}),
	}
}

// Releasing the typed XSD results of completed subjects changes no report:
// on the official and test-suite files and on documents chosen for what the
// typed results carry, every entry point the windowed harness covers and the
// profiles that release them return the reports of buffered sessions that
// keep them, whether the sessions that release them are buffered or windowed.
// A session that read a released typed result would fail Finish with a
// contract error instead.
func TestReleasingTypedXSDResultsChangesNoReport(t *testing.T) {
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	inputs := typedReleaseCases()
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
	for i := 0; i < len(files); i += windowedStride() {
		data, err := os.ReadFile(files[i])
		if err != nil {
			t.Fatal(err)
		}
		inputs[files[i]] = data
	}
	names := slices.Sorted(maps.Keys(inputs))
	// Each phase sets the switches once and assesses every input in parallel.
	phase := func(group string, keep, windowed bool) []map[string]string {
		previousKeep := validationSetKeepTypedXSD(keep)
		previousStrategy := validationSetWindowedStrategy(windowed)
		defer validationSetKeepTypedXSD(previousKeep)
		defer validationSetWindowedStrategy(previousStrategy)
		out := make([]map[string]string, len(names))
		inParallel(t, group, len(names), func(i int) string { return filepath.Base(names[i]) }, func(t *testing.T, i int) {
			out[i] = typedReleaseReports(t, names[i], inputs[names[i]])
		})
		return out
	}
	kept, released, windowed := phase("kept", true, false), phase("released", false, false), phase("released windowed", false, true)
	compared := 0
	for i, name := range names {
		for label, want := range kept[i] {
			for strategy, reports := range map[string]map[string]string{"buffered": released[i], "windowed": windowed[i]} {
				if got := reports[label]; got != want {
					t.Errorf("%s %s: %s report differs when typed results are released%s", name, label, strategy, windowedDifference(want, got))
				}
				if strings.Contains(reports[label], "read after they were released") {
					t.Errorf("%s %s: a %s reader read released typed results", name, label, strategy)
				}
			}
			compared++
		}
	}
	if compared < 100 {
		t.Fatalf("compared %d reports", compared)
	}
}

// The source line releases the typed XSD results of completed subjects and
// the model line keeps them: on MozartTrio the source line's buffered charge
// at the end of Finish drops by at least 300 bytes per element when they are
// released, and the model line's does not change.
func TestSourceLineReleasesTypedXSDResults(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "official", "MozartTrio.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	data := reviewBudgetRemoveDOCTYPE(t, raw)
	document, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	const elements = 2516
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34}
	charged := func(keep bool, source bool) int64 {
		previous := validationSetKeepTypedXSD(keep)
		defer validationSetKeepTypedXSD(previous)
		var used int64
		observer := validationSetFinishObserver(func(stage string, a *validation.Account) {
			if stage == "finished" {
				used, _ = a.Usage(validation.BudgetBufferedBytes)
			}
		})
		defer validationSetFinishObserver(observer)
		if source {
			_, err = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceLine, Budgets: big})
		} else {
			_, err = AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelSemantics, Budgets: big})
		}
		if err != nil {
			t.Fatal(err)
		}
		return used
	}
	if kept, released := charged(true, true), charged(false, true); kept-released < 300*elements {
		t.Errorf("source line: releasing typed results saves %d bytes, want at least %d", kept-released, 300*elements)
	}
	if kept, released := charged(true, false), charged(false, false); kept != released {
		t.Errorf("model line: charge %d with typed results kept, %d by default", kept, released)
	}
}
