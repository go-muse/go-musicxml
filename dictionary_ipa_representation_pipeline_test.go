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
				options := DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries}, Dictionaries: DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryRepresentationTestSelection()}, Source: dictionaryRepresentationSource(t, &calls)}}
				var report validation.Report
				var err error
				switch mode {
				case "source":
					report, err = InspectXMLWithDictionaries(ctx, strings.NewReader(text), options)
				case "model":
					doc, e := Decode(strings.NewReader(text))
					if e != nil {
						t.Fatal(e)
					}
					options.Assessment.Profile = ProfileModelDictionaries
					report, err = AssessModelWithDictionaries(ctx, doc, options)
				case "assessed-reader":
					result, e := DecodeAssessedWithDictionaries(ctx, strings.NewReader(text), DecodeDictionaryAssessmentOptions{Assessment: &options})
					report, err = result.Source, e
					if value == "a\u0327" {
						if result.Document != nil || !errors.Is(err, ErrIncompleteAssessment) {
							t.Fatalf("unproved witness acceptance: %v %v", result.Document, err)
						}
						err = nil
					} else if result.Document == nil {
						t.Fatal("accepted representation conversion lost")
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				requireRepresentationReport(t, report, value, value != "a\u0327")
				if calls != 1 {
					t.Fatalf("component loads=%d", calls)
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
				bundle, err = InspectXMLWithResourcesAndDictionaries(ctx, strings.NewReader(root), LinkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphDictionaries}, Identity: ResourceIdentity{RetrievalURI: "https://ipa.test/root.opus"}, Resolver: fixture, Dependencies: []ResourceDependency{ResourceDocuments}}, Dictionaries: dictionaries})
				if fixture.calls != 2 || bundle.ResourceCount() != 2 || bundle.AdoptionCount() != 3 {
					t.Fatalf("graph counts %d %d %d", fixture.calls, bundle.ResourceCount(), bundle.AdoptionCount())
				}
			case "raw-mxl", "dtd-raw-mxl":
				profile := ProfileMXLSourceDictionaries
				if mode == "dtd-raw-mxl" {
					profile = ProfileMXLSourceInternalDTDDictionaries
				}
				bundle, err = InspectMXLWithDictionaries(ctx, bytes.NewReader(packageGraphBytes(text, "main.musicxml", "")), MXLDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: profile}}, Dictionaries: dictionaries})
			case "model-mxl":
				doc, e := Decode(strings.NewReader(text))
				if e != nil {
					t.Fatal(e)
				}
				bundle, err = AssessMXLPackageWithDictionaries(ctx, &MXLPackage{Document: doc}, MXLDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLModelDictionaries}}, Dictionaries: dictionaries})
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
	options := DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries}, Dictionaries: DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryRepresentationTestSelection()}, Source: dictionaryRepresentationSource(t, &calls)}}
	report, err := InspectXMLWithDictionaries(context.Background(), strings.NewReader(text), options)
	if err != nil {
		t.Fatal(err)
	}
	requireRepresentationReport(t, report, value, true)
	if calls != 1 || !report.Satisfied() {
		t.Fatalf("short mark runs: loads=%d satisfied=%v findings=%s", calls, report.Satisfied(), localFindings(report))
	}
}
