package musicxml

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSemanticIPARepresentationPublicSourceModelAndAssessedReader(t *testing.T) {
	for _, value := range []string{"ḉ\u0334", "a\u0327"} {
		for _, mode := range []string{"source", "model", "assessed-reader"} {
			t.Run(mode+value, func(t *testing.T) {
				ctx := context.Background()
				text := contextFixtureXML(`<note><rest/><duration>1</duration><play><ipa>`+value+`</ipa></play></note>`, false)
				calls := 0
				options := dictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceSemantics}, Dictionaries: DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryRepresentationTestSelection()}, Source: dictionaryRepresentationSource(t, &calls)}}
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
					options.Assessment.Profile = ProfileModelSemantics
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
func TestSemanticIPARepresentationGraphAliasesAndMXLUseSameWitness(t *testing.T) {
	for _, mode := range []string{"graph-aliases", "raw-mxl", "model-mxl"} {
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
				bundle, err = inspectXMLWithResourcesAndDictionaries(ctx, strings.NewReader(root), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphSemantics}, Identity: ResourceIdentity{RetrievalURI: "https://ipa.test/root.opus"}, Resolver: fixture, Dependencies: []ResourceDependency{ResourceDocuments}}, Dictionaries: dictionaries})
				if fixture.calls != 2 || bundle.ResourceCount() != 2 || bundle.AdoptionCount() != 3 {
					t.Fatalf("graph counts %d %d %d", fixture.calls, bundle.ResourceCount(), bundle.AdoptionCount())
				}
			case "raw-mxl":
				bundle, err = inspectMXLWithDictionaries(ctx, bytes.NewReader(packageGraphBytes(text, "main.musicxml", "")), mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLSourceSemantics}}, Dictionaries: dictionaries})
			case "model-mxl":
				doc, e := Decode(strings.NewReader(text))
				if e != nil {
					t.Fatal(e)
				}
				bundle, err = assessMXLPackageWithDictionaries(ctx, &MXLPackage{Document: doc}, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLModelSemantics}}, Dictionaries: dictionaries})
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
