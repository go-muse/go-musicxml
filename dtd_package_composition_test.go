package musicxml

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Literal IDs keep the red phase executable without implementation symbols.
const (
	dtdContainerProfile validation.ProfileID = "musicxml-4.0/source-internal-dtd-container"
	dtdSoundsProfile    validation.ProfileID = "musicxml-4.0/source-internal-dtd-sounds-dictionaries"
	dtdGraphProfile     validation.ProfileID = "musicxml-4.0/resource-graph-internal-dtd-dictionaries"
	dtdMXLSourceProfile validation.ProfileID = "musicxml-4.0/mxl-source-internal-dtd-dictionaries"
	dtdMXLModelProfile  validation.ProfileID = "musicxml-4.0/mxl-model-internal-dtd-dictionaries"
)

func TestDTDPackageCompositionAuxiliaryProfiles(t *testing.T) {
	for _, tc := range []struct {
		id     validation.ProfileID
		source string
	}{
		{dtdContainerProfile, `<!DOCTYPE container [<!ATTLIST rootfile full-path CDATA "main.opus">]><container><rootfiles><rootfile/></rootfiles></container>`},
		{dtdSoundsProfile, `<!DOCTYPE sounds [<!ATTLIST sound id CDATA "custom.sound">]><sounds><sound><any>Library</any></sound></sounds>`},
	} {
		t.Run(string(tc.id), func(t *testing.T) {
			var r validation.Report
			var err error
			if tc.id == dtdSoundsProfile {
				r, err = inspectXMLWithDictionaries(context.Background(), strings.NewReader(tc.source), dictionaryAssessmentOptions{Assessment: assessmentOptions{Profile: tc.id}})
			} else {
				r, err = InspectXML(context.Background(), strings.NewReader(tc.source), SourceOptions{Profile: SourceProfile(tc.id)})
			}
			if err != nil || r.Conformance == validation.ConformanceViolated || !r.InputComplete {
				t.Fatalf("err=%v report=%+v", err, r)
			}
			found := false
			for _, row := range r.Context {
				if row.SourceOrigin != nil {
					found = true
				}
			}
			if !found {
				t.Fatal("missing auxiliary source origins")
			}
		})
	}
}

func TestDTDPackageCompositionEntryPoints(t *testing.T) {
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
		{"source", dtdGraphProfile, func() (validation.AssessmentBundle, error) {
			return inspectXMLWithResourcesAndDictionaries(ctx, strings.NewReader(`<!DOCTYPE opus [<!ATTLIST opus version CDATA "4.0">]><opus/>`), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: GraphProfile(dtdGraphProfile)}}.internal()})
		}, false},
		{"model", dtdGraphProfile, func() (validation.AssessmentBundle, error) {
			return assessModelWithResourcesAndDictionaries(ctx, doc, linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: GraphProfile(dtdGraphProfile)}}.internal()})
		}, true},
		{"raw MXL", dtdMXLSourceProfile, func() (validation.AssessmentBundle, error) {
			return inspectMXLWithDictionaries(ctx, bytes.NewReader(packageGraphBytes(`<opus/>`, "main.opus", "")), mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: MXLProfile(dtdMXLSourceProfile)}}.internal()})
		}, false},
		{"current MXL", dtdMXLModelProfile, func() (validation.AssessmentBundle, error) {
			return assessMXLPackageWithDictionaries(ctx, &MXLPackage{Document: doc}, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: MXLProfile(dtdMXLModelProfile)}}.internal()})
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
				want := ProfileSourceInternalDTDDictionaries.ID()
				if tc.model {
					want = ProfileModelDictionaries.ID()
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
