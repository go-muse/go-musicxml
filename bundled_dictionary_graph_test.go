package musicxml

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestBundledDictionaryGraphAndMXLParity(t *testing.T) {
	ids := []validation.DictionaryManifestID{
		validation.DictionaryManifestSMuFL14Canonical,
		validation.DictionaryManifestSMuFL14Ranges,
		validation.DictionaryManifestStandardSounds40,
		validation.DictionaryManifestIPAPositive2015Unicode13,
		validation.DictionaryManifestIPARepresentation2015Unicode13,
	}
	var selections []validation.DictionarySelection
	var totalBytes int64
	for _, id := range ids {
		selections = append(selections, dictionaryImportSelection(id))
		m, _ := validation.LookupDictionaryManifest(id)
		totalBytes += m.Component.Bytes
	}
	// Exercise all five components, including the bounded IPA witness and gap.
	input := resourceScore(`<note><rest/><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental><notations><ornaments><tremolo type="unmeasured">0</tremolo></ornaments></notations><play><ipa>a</ipa><ipa>a̧</ipa></play></note>`)
	input = strings.Replace(input, "</score-part>", `<score-instrument id="I1"><instrument-name>Piano</instrument-name><instrument-sound>keyboard.piano</instrument-sound></score-instrument></score-part>`, 1)
	doc, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	archive := packageGraphBytes(input, "main.musicxml", "")
	fixture := dictionaryImportSourceFunc(func(_ context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
		var raw []byte
		if q.Manifest == validation.DictionaryManifestIPARepresentation2015Unicode13 {
			raw = dictionaryRepresentationFixture(t)
		} else {
			raw = dictionaryImportFixture(t, q.Manifest)
		}
		return DictionaryComponentResult{Request: q, State: validation.EvidenceKnown, Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})
	for _, mode := range []string{"graph source", "graph model", "MXL source", "MXL model"} {
		t.Run(mode, func(t *testing.T) {
			run := func(dictionaries DictionaryOptions, limit int64) (validation.AssessmentBundle, error) {
				ctx := context.Background()
				graph := GraphOptions{Profile: ProfileResourceGraphDictionaries, Budgets: validation.Budgets{MaxExternalBytes: limit}}
				identity := ResourceIdentity{CanonicalID: "bundled-parity-root"}
				switch mode {
				case "graph source":
					return inspectXMLWithResourcesAndDictionaries(ctx, strings.NewReader(input), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: graph, Identity: identity}.internal(), Dictionaries: dictionaries})
				case "graph model":
					return assessModelWithResourcesAndDictionaries(ctx, doc, linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: graph, Identity: identity}.internal(), Dictionaries: dictionaries})
				case "MXL source":
					mxlGraph := MXLGraphOptions{Profile: ProfileMXLSourceDictionaries, Budgets: graph.Budgets}
					return inspectMXLWithDictionaries(ctx, bytes.NewReader(archive), mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: mxlGraph, Identity: identity}.internal(), Dictionaries: dictionaries})
				default:
					mxlGraph := MXLGraphOptions{Profile: ProfileMXLModelDictionaries, Budgets: graph.Budgets}
					return assessMXLPackageWithDictionaries(ctx, &MXLPackage{Document: doc}, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: mxlGraph, Identity: identity}.internal(), Dictionaries: dictionaries})
				}
			}
			t.Run("all five components", func(t *testing.T) {
				var baseline []validation.Report
				for _, source := range []DictionarySource{fixture, BundledDictionarySource()} {
					calls := map[validation.DictionaryManifestID]int{}
					counted := dictionaryImportSourceFunc(func(ctx context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
						calls[q.Manifest]++
						return source.Open(ctx, q)
					})
					bundle, err := run(DictionaryOptions{Selections: selections, Source: counted}, 0)
					if err != nil || !bundle.EnumerationComplete() {
						t.Fatalf("enumeration=%v err=%v", bundle.EnumerationComplete(), err)
					}
					for _, id := range ids {
						if calls[id] != 1 {
							t.Fatalf("%s opened %d times", id, calls[id])
						}
					}
					reports := bundledParityReports(t, bundle)
					wantPackage, wantIntrinsic, wantTarget := ProfileResourceGraphDictionaries.ID(), ProfileSourceDictionaries.ID(), validation.TargetSource
					if strings.HasSuffix(mode, "model") {
						wantIntrinsic, wantTarget = ProfileModelDictionaries.ID(), validation.TargetModel
					}
					switch mode {
					case "MXL source":
						wantPackage = ProfileMXLSourceDictionaries.ID()
					case "MXL model":
						wantPackage = ProfileMXLModelDictionaries.ID()
					}
					if reports[0].Profile.ID != wantPackage {
						t.Fatalf("package profile=%s want=%s", reports[0].Profile.ID, wantPackage)
					}
					intrinsic := 0
					for _, report := range reports[1:] {
						if mode == "MXL source" && report.Profile.ID == ProfileSourceContainer.ID() {
							continue
						}
						intrinsic++
						if report.Profile.ID != wantIntrinsic || report.Target != wantTarget || report.Conformance == validation.ConformanceViolated {
							t.Fatalf("intrinsic profile=%s target=%s conformance=%s", report.Profile.ID, report.Target, report.Conformance)
						}
					}
					if intrinsic != 1 {
						t.Fatalf("intrinsic dictionary reports=%d", intrinsic)
					}
					verified := map[validation.DictionaryManifestID]bool{}
					knownRepresentation, unknownRepresentation := false, false
					for _, report := range reports {
						for _, row := range report.Context {
							d := row.Dictionary
							if d == nil {
								continue
							}
							verified[d.Manifest] = verified[d.Manifest] || d.ComponentVerified
							if d.Manifest == validation.DictionaryManifestIPARepresentation2015Unicode13 {
								knownRepresentation = knownRepresentation || d.Input.Value.Text == "a" && d.Result.State == validation.EvidenceKnown && d.Result.Value
								unknownRepresentation = unknownRepresentation || d.Input.Value.Text == "a̧" && d.Result.State == validation.EvidenceUnknown
							}
						}
					}
					for _, id := range ids {
						if !verified[id] {
							t.Fatalf("missing verified evidence for %s", id)
						}
					}
					if !knownRepresentation || !unknownRepresentation {
						t.Fatalf("bounded representation evidence: known=%v gap=%v", knownRepresentation, unknownRepresentation)
					}
					if baseline != nil && !reflect.DeepEqual(baseline, reports) {
						t.Fatalf("bundled and caller-supplied reports differ:\ncaller=%+v\nbundled=%+v", baseline, reports)
					}
					baseline = reports
				}
			})
			for _, state := range []string{"unselected", "nil source"} {
				t.Run(state, func(t *testing.T) {
					dictionaries := DictionaryOptions{Selections: selections}
					if state == "unselected" {
						dictionaries = DictionaryOptions{Source: dictionaryImportSourceFunc(func(ctx context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
							t.Fatal("unselected bundled source was invoked")
							return BundledDictionarySource().Open(ctx, q)
						})}
					}
					bundle, err := run(dictionaries, 0)
					if err != nil {
						t.Fatal(err)
					}
					canonical := 0
					for _, report := range bundledParityReports(t, bundle) {
						for _, row := range report.Context {
							if row.Property == validation.ContextDictionaryCanonical && row.Dictionary != nil {
								canonical++
								if row.Dictionary.ComponentVerified || row.Dictionary.Result.State != validation.EvidenceUnknown {
									t.Fatalf("unavailable canonical evidence=%+v", row.Dictionary)
								}
							}
						}
					}
					if canonical != 1 {
						t.Fatalf("canonical evidence count=%d", canonical)
					}
				})
			}
			t.Run("external byte limit", func(t *testing.T) {
				_, err := run(DictionaryOptions{Selections: selections, Source: BundledDictionarySource()}, totalBytes-1)
				if !errors.Is(err, validation.ErrResourceLimit) {
					t.Fatalf("bundled imports escaped external byte accounting: %v", err)
				}
			})
		})
	}
}

// Compare complete public reports, replacing only fresh per-run document IDs.
// Keep distinct documents distinct, and preserve all component/data/profile IDs.
func bundledParityReports(t *testing.T, bundle validation.AssessmentBundle) []validation.Report {
	t.Helper()
	reports := []validation.Report{bundle.PackageReport()}
	for i := 0; i < bundle.ResourceCount(); i++ {
		_, report, ok := bundle.ResourceReport(i)
		if !ok {
			t.Fatalf("missing resource report %d", i)
		}
		reports = append(reports, report)
	}
	ids := map[string]string{}
	var normalize func(reflect.Value)
	normalize = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if !v.IsNil() {
				normalize(v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				if v.Type().Field(i).Name == "DocumentID" && field.CanSet() && field.String() != "" {
					id := field.String()
					if ids[id] == "" {
						ids[id] = fmt.Sprintf("document-%d", len(ids)+1)
					}
					field.SetString(ids[id])
				} else {
					normalize(field)
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				normalize(v.Index(i))
			}
		}
	}
	normalize(reflect.ValueOf(reports))
	return reports
}
