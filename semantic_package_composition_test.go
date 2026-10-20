package musicxml

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSemanticPackageCompositionEntryPoints(t *testing.T) {
	ctx := context.Background()
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		profile validation.ProfileID
		run     func() (validation.AssessmentBundle, error)
		model   bool
	}{
		{"source", validation.ProfileID("musicxml-4.0/resource-graph-semantics"), func() (validation.AssessmentBundle, error) {
			return inspectXMLWithResourcesAndDictionaries(ctx, strings.NewReader(`<!DOCTYPE opus [<!ATTLIST opus version CDATA "4.0">]><opus/>`), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: GraphProfile(validation.ProfileID("musicxml-4.0/resource-graph-semantics"))}}.internal()})
		}, false},
		{"model", validation.ProfileID("musicxml-4.0/resource-graph-semantics"), func() (validation.AssessmentBundle, error) {
			return assessModelWithResourcesAndDictionaries(ctx, doc, linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: GraphProfile(validation.ProfileID("musicxml-4.0/resource-graph-semantics"))}}.internal()})
		}, true},
		{"raw MXL", validation.ProfileID("musicxml-4.0/mxl-source-semantics"), func() (validation.AssessmentBundle, error) {
			return inspectMXLWithDictionaries(ctx, bytes.NewReader(packageGraphBytes(`<opus/>`, "main.opus", "")), mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: MXLProfile(validation.ProfileID("musicxml-4.0/mxl-source-semantics"))}}.internal()})
		}, false},
		{"current MXL", validation.ProfileID("musicxml-4.0/mxl-model-semantics"), func() (validation.AssessmentBundle, error) {
			return assessMXLPackageWithDictionaries(ctx, &MXLPackage{Document: doc}, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: MXLProfile(validation.ProfileID("musicxml-4.0/mxl-model-semantics"))}}.internal()})
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, e := tc.run()
			if e != nil {
				t.Fatal(e)
			}
			if b.PackageReport().Profile.ID != tc.profile || !b.EnumerationComplete() {
				t.Fatalf("wrong composition: %+v", b.PackageReport())
			}
			found := false
			for i := 0; i < b.ResourceCount(); i++ {
				_, r, _ := b.ResourceReport(i)
				if r.Profile.ID == dtdContainerProfile {
					continue
				}
				found = true
				want := ProfileSourceSemantics.ID()
				if tc.model {
					want = ProfileModelSemantics.ID()
				}
				if r.Profile.ID != want {
					t.Errorf("intrinsic=%s want=%s", r.Profile.ID, want)
				}
				for _, row := range r.Context {
					if tc.model && row.SourceOrigin != nil {
						t.Fatal("current model invented source provenance")
					}
				}
			}
			if !found {
				t.Fatal("no primary")
			}
		})
	}
}

func TestSemanticPackageCompositionFragments(t *testing.T) {
	for _, model := range []bool{false, true} {
		t.Run(map[bool]string{false: "source", true: "model"}[model], func(t *testing.T) {
			const root = `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://semantics.test/child.opus#element(/1)"/></opus>`
			fixture := &graphFixtureResolver{resources: map[string]string{"https://semantics.test/child.opus#element(/1)": `<opus/>`}, pins: map[string]string{"https://semantics.test/child.opus#element(/1)": "child"}}
			options := linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphSemantics}, Identity: ResourceIdentity{RetrievalURI: "https://semantics.test/root.opus"}, Resolver: fixture, Dependencies: []ResourceDependency{ResourceDocuments}}.internal()}
			var b validation.AssessmentBundle
			var err error
			if model {
				doc, e := Decode(strings.NewReader(root))
				if e != nil {
					t.Fatal(e)
				}
				b, err = assessModelWithResourcesAndDictionaries(context.Background(), doc, options)
			} else {
				b, err = inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(root), options)
			}
			if err != nil || (!model && !b.Satisfied()) || b.ResourceCount() != 2 || b.AdoptionCount() != 2 {
				t.Fatalf("err=%v resources=%d package=%+v", err, b.ResourceCount(), b.PackageReport())
			}
			for _, finding := range b.PackageReport().Findings {
				if finding.Rule.ID == "MX40-PACKAGE-link-target" && finding.Outcome != validation.OutcomePass {
					t.Fatalf("fragment unresolved: %+v", finding)
				}
			}
			for _, body := range fixture.bodies {
				if body.closed != 1 {
					t.Fatalf("close=%d", body.closed)
				}
			}
		})
	}
}

func TestSemanticPackageCompositionWrongEntryPointBeforeIO(t *testing.T) {
	for _, profile := range []validation.ProfileID{ProfileMXLSourceSemantics.ID(), ProfileMXLModelSemantics.ID(), ProfileSourceSemantics.ID(), ProfileModelSemantics.ID()} {
		calls := 0
		reader := &assessmentCountingReader{source: strings.NewReader(`<opus/>`)}
		provider := dictionaryImportSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
			calls++
			return DictionaryComponentResult{}, nil
		})
		_, err := inspectXMLWithResourcesAndDictionaries(context.Background(), reader, linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: GraphProfile(profile)}}.internal(), Dictionaries: DictionaryOptions{Source: provider, Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical)}}})
		if err == nil || calls != 0 || reader.reads != 0 {
			t.Fatalf("profile=%s err=%v calls=%d reader=%+v", profile, err, calls, reader)
		}
	}
}

func TestSemanticPackageCompositionFragmentAbsence(t *testing.T) {
	const href = "https://semantics.test/child.xml#element(/2)"
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="` + href + `"/></opus>`
	resolver := &graphFixtureResolver{resources: map[string]string{href: `<opus/>`}, pins: map[string]string{href: "child"}}
	b, err := inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(root), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphSemantics}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}}.internal()})
	if err != nil || b.ResourceCount() != 2 || !b.EnumerationComplete() {
		t.Fatalf("err=%v resources=%d", err, b.ResourceCount())
	}
	findings := indirectionContextLinks(b)
	if len(findings) != 1 || findings[0].Outcome != validation.OutcomeFail {
		t.Fatalf("complete absent element must fail: %+v", findings)
	}
}

func TestSemanticPackageCompositionRetainsEncodedFailure(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		input := barlineLocationScore(timewise, barlineDivisions+barlineNote("1")+`<barline location="left"/>`, "")
		doc, err := Decode(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []string{"source", "model", "raw", "current"} {
			t.Run(mode+map[bool]string{false: "/partwise", true: "/timewise"}[timewise], func(t *testing.T) {
				var b validation.AssessmentBundle
				var e error
				graph := linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphSemantics}}.internal()}
				switch mode {
				case "source":
					b, e = inspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(input), graph)
				case "model":
					b, e = assessModelWithResourcesAndDictionaries(context.Background(), doc, graph)
				case "raw":
					b, e = inspectMXLWithDictionaries(context.Background(), bytes.NewReader(packageGraphBytes(input, "main.musicxml", "")), mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSourceSemantics}}.internal()})
				case "current":
					b, e = assessMXLPackageWithDictionaries(context.Background(), &MXLPackage{Document: doc}, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModelSemantics}}.internal()})
				}
				if e != nil || b.Satisfied() {
					t.Fatalf("err=%v satisfied=%t", e, b.Satisfied())
				}
				found := false
				for i := 0; i < b.ResourceCount(); i++ {
					_, r, _ := b.ResourceReport(i)
					for _, f := range r.Findings {
						if f.Rule.ID == "MX40-CONTEXT-barline-location" && f.Outcome == validation.OutcomeFail {
							found = true
							if r.Conformance != validation.ConformanceViolated {
								t.Fatal("lost intrinsic violation")
							}
						}
					}
				}
				if !found {
					t.Fatal("encoded failure was lost during composition")
				}
			})
		}
	}
}
