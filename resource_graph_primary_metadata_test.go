package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestResourceGraphPrimaryRetrievalMetadata(t *testing.T) {
	for _, model := range []bool{false, true} {
		options := LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/primary.bad"}}
		var b validation.AssessmentBundle
		var err error
		if model {
			d, e := Decode(strings.NewReader(`<opus/>`))
			if e != nil {
				t.Fatal(e)
			}
			b, err = AssessModelWithResources(context.Background(), d, options)
		} else {
			b, err = InspectXMLWithResources(context.Background(), strings.NewReader(`<opus/>`), options)
		}
		if err != nil || b.FileMetadataCount() != 1 {
			t.Fatalf("model%v err%v metadata%d", model, err, b.FileMetadataCount())
		}
		m, _ := b.FileMetadataAt(0)
		if m.Name.State != validation.EvidenceKnown || m.Name.Value != "primary.bad" || m.BaseURI != options.Identity.RetrievalURI || m.MediaType.State != validation.EvidenceAbsent || m.Entry != 0 || m.Source != "" {
			t.Fatalf("metadata %+v", m)
		}
		if b.ResourceCount() != 1 || b.AdoptionCount() != 1 || (!model && !b.Satisfied()) {
			t.Fatal("optional advice changed parsed capabilities", b.PackageReport())
		}
	}
}
