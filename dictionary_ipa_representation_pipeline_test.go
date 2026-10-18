package musicxml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func dictionaryRepresentationSource(t *testing.T, calls *int) DictionarySource {
	raw := dictionaryRepresentationFixture(t)
	return dictionaryImportSourceFunc(func(_ context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
		*calls++
		if q.Manifest != validation.DictionaryManifestIPARepresentation2015Unicode13 {
			t.Fatalf("unrequested component %s", q.Manifest)
		}
		return DictionaryComponentResult{Request: q, State: validation.EvidenceKnown, Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})
}
func assessedRepresentationDictionaries(t *testing.T, calls *int) DictionaryOptions {
	t.Helper()
	seen := map[validation.DictionaryManifestID]bool{}
	return DictionaryOptions{
		Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical), dictionaryRepresentationTestSelection()},
		Source: dictionaryImportSourceFunc(func(_ context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
			*calls++
			if seen[request.Manifest] {
				t.Fatalf("component loaded more than once: %s", request.Manifest)
			}
			seen[request.Manifest] = true
			var raw []byte
			switch request.Manifest {
			case validation.DictionaryManifestSMuFL14Canonical:
				raw = dictionaryImportFixture(t, request.Manifest)
			case validation.DictionaryManifestIPARepresentation2015Unicode13:
				raw = dictionaryRepresentationFixture(t)
			default:
				t.Fatalf("unselected component requested: %s", request.Manifest)
			}
			return DictionaryComponentResult{Request: request, State: validation.EvidenceKnown, Body: io.NopCloser(bytes.NewReader(raw))}, nil
		}),
	}
}

func requireRepresentationReport(t *testing.T, r validation.Report, value string, known bool) {
	t.Helper()
	rows := dictionaryResults(r, validation.ContextDictionaryIPARepresentation)
	if len(rows) != 1 {
		t.Fatalf("representation rows=%d in %s", len(rows), localFindings(r))
	}
	row := rows[0]
	if (row.Result.State == validation.EvidenceKnown) != known || row.Input.Value.Text != value || len(row.Spans) != 1 || row.Spans[0].Text != value || row.Spans[0].End != uint64(len(value)) || !row.ComponentVerified {
		t.Fatalf("projection=%+v", row)
	}
	if row.Spans[0].RowID != "" || row.Spans[0].Category != "whole-representation" || row.WholeSequence != row.Result {
		t.Fatalf("composite evidence=%+v", row)
	}
}
func TestIPARepresentationPublicSourceModelAndAssessedReader(t *testing.T) {
	for _, value := range []string{"ḉ\u0334", "a\u0327"} {
		for _, mode := range []string{"source", "model", "assessed-reader"} {
			t.Run(mode+value, func(t *testing.T) {
				ctx := context.Background()
				text := contextFixtureXML(`<note><rest/><duration>1</duration><play><ipa>`+value+`</ipa></play></note>`, false)
				calls := 0
				options := dictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries}, Dictionaries: DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryRepresentationTestSelection()}, Source: dictionaryRepresentationSource(t, &calls)}}
				var report validation.Report
				var err error
				switch mode {
				case "source":
					report, err = inspectXMLWithDictionaries(ctx, strings.NewReader(text), options)
				case "model":
					doc, e := Decode(strings.NewReader(text))
					if e != nil {
						t.Fatal(e)
					}
					options.Assessment.Profile = ProfileModelDictionaries
					report, err = assessModelWithDictionaries(ctx, doc, options)
				case "assessed-reader":
					options.Assessment.Profile = ProfileSourceSemantics
					options.Dictionaries = assessedRepresentationDictionaries(t, &calls)
					result, e := decodeAssessedWithDictionaries(ctx, strings.NewReader(text), decodeDictionaryTestOptions{Assessment: &options})
					report, err = result.Source, e
					if value == "a\u0327" {
						if result.Document != nil || !errors.Is(err, ErrIncompleteAssessment) || result.Model.Requested {
							t.Fatalf("unproved witness acceptance: %v %v", result.Document, err)
						}
						err = nil
					} else if result.Document == nil || !result.Source.Satisfied() || !result.Model.Satisfied() {
						t.Fatalf("accepted representation source/model assessment lost: %+v %v", result, err)
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				requireRepresentationReport(t, report, value, value != "a\u0327")
				wantCalls := 1
				if mode == "assessed-reader" {
					wantCalls = 2
				}
				if calls != wantCalls {
					t.Fatalf("component loads=%d want=%d", calls, wantCalls)
				}
			})
		}
	}
}
func TestIPARepresentationGraphAliasesAndMXLUseSameWitness(t *testing.T) {
	for _, mode := range []string{"graph-aliases", "raw-mxl", "model-mxl", "dtd-raw-mxl"} {
		t.Run(mode, func(t *testing.T) {
			value := "d\u0334\u0322\u0301"
			text := resourceScore(`<note><rest/><duration>1</duration><play><ipa>` + value + `</ipa></play></note>`)
			calls := 0
			dictionaries := DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryRepresentationTestSelection()}, Source: dictionaryRepresentationSource(t, &calls)}
			ctx := context.Background()
			var bundle validation.AssessmentBundle
			var err error
			switch mode {
			case "graph-aliases":
				root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="a.musicxml"/><score xlink:href="b.musicxml"/></opus>`
				fixture := &graphFixtureResolver{resources: map[string]string{"https://ipa.test/a.musicxml": text, "https://ipa.test/b.musicxml": text}, pins: map[string]string{"https://ipa.test/a.musicxml": "same", "https://ipa.test/b.musicxml": "same"}}
				bundle, err = inspectXMLWithResourcesAndDictionaries(ctx, strings.NewReader(root), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphDictionaries}, Identity: ResourceIdentity{RetrievalURI: "https://ipa.test/root.opus"}, Resolver: fixture, Dependencies: []ResourceDependency{ResourceDocuments}}, Dictionaries: dictionaries})
				if fixture.calls != 2 || bundle.ResourceCount() != 2 || bundle.AdoptionCount() != 3 {
					t.Fatalf("graph counts %d %d %d", fixture.calls, bundle.ResourceCount(), bundle.AdoptionCount())
				}
			case "raw-mxl", "dtd-raw-mxl":
				profile := ProfileMXLSourceDictionaries
				if mode == "dtd-raw-mxl" {
					profile = ProfileMXLSourceInternalDTDDictionaries
				}
				bundle, err = inspectMXLWithDictionaries(ctx, bytes.NewReader(packageGraphBytes(text, "main.musicxml", "")), mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: profile}}, Dictionaries: dictionaries})
			case "model-mxl":
				doc, e := Decode(strings.NewReader(text))
				if e != nil {
					t.Fatal(e)
				}
				bundle, err = assessMXLPackageWithDictionaries(ctx, &MXLPackage{Document: doc}, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLModelDictionaries}}, Dictionaries: dictionaries})
			}
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			for i := 0; i < bundle.ResourceCount(); i++ {
				_, r, ok := bundle.ResourceReport(i)
				if ok && len(dictionaryResults(r, validation.ContextDictionaryIPARepresentation)) > 0 {
					requireRepresentationReport(t, r, value, true)
					seen++
				}
			}
			if seen != 1 || calls != 1 {
				t.Fatalf("reports=%d dictionary calls=%d", seen, calls)
			}
		})
	}
}

func TestIPARepresentationManyShortMarkSegmentsFitDefaultBudget(t *testing.T) {
	value := strings.Repeat("a\u0301", 31438)
	text := contextFixtureXML(`<note><rest/><duration>1</duration><play><ipa>`+value+`</ipa></play></note>`, false)
	calls := 0
	options := dictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries}, Dictionaries: DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryRepresentationTestSelection()}, Source: dictionaryRepresentationSource(t, &calls)}}
	report, err := inspectXMLWithDictionaries(context.Background(), strings.NewReader(text), options)
	if err != nil {
		t.Fatal(err)
	}
	requireRepresentationReport(t, report, value, true)
	if calls != 1 || !report.Satisfied() {
		t.Fatalf("short mark runs: loads=%d satisfied=%v findings=%s", calls, report.Satisfied(), localFindings(report))
	}
}

func TestIPARepresentationWholeMarksAndHalfPairScope(t *testing.T) {
	// The pinned language concatenates complete encoding atoms. Whole ties are
	// independent mark atoms; their membership does not assert endpoint scope,
	// rendering, or phonotactics. Half ties have a separate adjacent-pair rule.
	for _, tc := range []struct {
		name, value string
		known       bool
	}{
		{"whole-dangling", "t\u0361", true},
		{"whole-leading", "\u0361", true},
		{"whole-repeated", "t\u0361\u0361s", true},
		{"whole-mixed", "t\u0361\u035cs", true},
		{"whole-on-sign", "\u02c8\u0361s", true},
		{"whole-before-sign", "t\u0361\u02c8", true},
		{"whole-shared-middle", "t\u0361s\u0361a", true},
		{"half-pair", "t\ufe20s\ufe21", true},
		{"half-dangling", "t\ufe20", false},
		{"half-repeated", "t\ufe20\ufe20s\ufe21\ufe21", false},
		{"half-on-sign", "\u02c8\ufe20s\ufe21", false},
		{"half-shared-middle", "t\ufe20s\ufe21\ufe20a\ufe21", false},
		{"whole-half-overlap", "t\u0361\ufe20s\ufe21", false},
	} {
		for _, mode := range []string{"source", "model"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				text := contextFixtureXML(`<note><rest/><duration>1</duration><play><ipa>`+tc.value+`</ipa></play></note>`, false)
				calls := 0
				options := dictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries}, Dictionaries: DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryRepresentationTestSelection()}, Source: dictionaryRepresentationSource(t, &calls)}}
				var report validation.Report
				var err error
				if mode == "source" {
					report, err = inspectXMLWithDictionaries(context.Background(), strings.NewReader(text), options)
				} else {
					doc, e := Decode(strings.NewReader(text))
					if e != nil {
						t.Fatal(e)
					}
					options.Assessment.Profile = ProfileModelDictionaries
					report, err = assessModelWithDictionaries(context.Background(), doc, options)
				}
				if err != nil {
					t.Fatal(err)
				}
				requireRepresentationReport(t, report, tc.value, tc.known)
				if calls != 1 || report.Satisfied() != tc.known {
					t.Fatalf("loads=%d satisfied=%v want%v: %s", calls, report.Satisfied(), tc.known, localFindings(report))
				}
				row := dictionaryResults(report, validation.ContextDictionaryIPARepresentation)[0]
				if tc.known {
					if !row.Result.Value || !row.WholeSequence.Value {
						t.Fatal("established witness is not affirmative")
					}
				} else if row.Result.State != validation.EvidenceUnknown || row.WholeSequence.State != validation.EvidenceUnknown {
					t.Fatal("unproved half shape became a negative or absent result")
				}
			})
		}
	}
}
