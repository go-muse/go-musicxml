package musicxml

import (
	"bytes"
	"context"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
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
			return InspectXMLWithResourcesAndDictionaries(ctx, strings.NewReader(`<!DOCTYPE opus [<!ATTLIST opus version CDATA "4.0">]><opus/>`), LinkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: validation.ProfileID("musicxml-4.0/resource-graph-semantics")}}})
		}, false},
		{"model", validation.ProfileID("musicxml-4.0/resource-graph-semantics"), func() (validation.AssessmentBundle, error) {
			return AssessModelWithResourcesAndDictionaries(ctx, doc, LinkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: validation.ProfileID("musicxml-4.0/resource-graph-semantics")}}})
		}, true},
		{"raw MXL", validation.ProfileID("musicxml-4.0/mxl-source-semantics"), func() (validation.AssessmentBundle, error) {
			return InspectMXLWithDictionaries(ctx, bytes.NewReader(packageGraphBytes(`<opus/>`, "main.opus", "")), MXLDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: validation.ProfileID("musicxml-4.0/mxl-source-semantics")}}})
		}, false},
		{"current MXL", validation.ProfileID("musicxml-4.0/mxl-model-semantics"), func() (validation.AssessmentBundle, error) {
			return AssessMXLPackageWithDictionaries(ctx, &MXLPackage{Document: doc}, MXLDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: validation.ProfileID("musicxml-4.0/mxl-model-semantics")}}})
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
				want := ProfileSourceSemantics
				if tc.model {
					want = ProfileModelSemantics
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
