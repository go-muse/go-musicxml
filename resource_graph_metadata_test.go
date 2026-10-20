package musicxml

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"math/big"
	"strings"
	"testing"
)

type graphResolverFunc func(context.Context, validation.ResourceRequest) (validation.ResourceResult, error)

func (f graphResolverFunc) Resolve(c context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
	return f(c, q)
}
func TestResourceGraphImageMetadataIsExplicitOwnedAdvice(t *testing.T) {
	source := resourceScore(`<direction><direction-type><image source="plot.dat" type="image/png" width="20" height="10"/></direction-type></direction>`)
	calls := 0
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		calls++
		if q.Image == nil {
			t.Fatal("unexpected document fetch")
		}
		w, _ := validation.NewDecimal(big.NewRat(400, 1))
		h, _ := validation.NewDecimal(big.NewRat(200, 1))
		one, _ := validation.NewDecimal(big.NewRat(1, 1))
		metadata := validation.ImageMetadataResult{Request: *q.Image, Resource: validation.ResourcePin{CanonicalID: "image", Representation: validation.ResourceBinary, Digest: strings.Repeat("a", 64)}, Width: validation.Evidence[validation.Decimal]{State: validation.EvidenceKnown, Value: w}, Height: validation.Evidence[validation.Decimal]{State: validation.EvidenceKnown, Value: h}, Unit: validation.ImageUnitPixel, PixelAspectRatio: validation.Evidence[validation.Decimal]{State: validation.EvidenceKnown, Value: one}, Authority: validation.EvidenceAuthority{ID: "test-image-metadata", ExactRelationComplete: true, Provenance: []validation.SourceRef{{IDs: []string{"fixture-metadata"}, URL: "https://example.test/image-metadata", Locator: "exact test fixture"}}}}
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Image: &metadata}, nil
	})
	for _, selected := range []bool{false, true} {
		opts := LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Resolver: resolver}
		if selected {
			opts.Dependencies = []ResourceDependency{ResourceImages}
		}
		b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), opts)
		if err != nil || !b.Satisfied() || b.ImageAdviceCount() != 1 {
			t.Fatalf("selected%v: %v advice%d %+v", selected, err, b.ImageAdviceCount(), b.PackageReport())
		}
		advice, ok := b.ImageAdviceAt(0)
		if !ok {
			t.Fatal("missing advice")
		}
		if selected {
			if calls != 1 || advice.IntrinsicAspectRatio.State != validation.EvidenceKnown || !advice.PreservesAspectRatio.Value {
				t.Fatal(calls, advice)
			}
		} else if calls != 0 || advice.Availability.State != validation.EvidenceUnknown {
			t.Fatal(calls, advice)
		}
	}
}

func TestResourceGraphInstrumentAssociationRequiresExactProvider(t *testing.T) {
	source := `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink"><part-list><score-part id="P1"><part-link xlink:href="part.musicxml"><instrument-link id="I1"/></part-link><part-name>P</part-name><score-instrument id="I1"><instrument-name>P</instrument-name></score-instrument></score-part></part-list><part id="P1"><measure number="1"/></part></score-partwise>`
	target := resourceScore("")
	for _, selected := range []bool{false, true} {
		associationCalls := 0
		resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
			if q.Document != nil {
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "part", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(target)))}, RetrievalURI: "https://e.test/part.musicxml", Body: &resourceTestBody{Reader: strings.NewReader(target)}}, nil
			}
			if q.Association == nil {
				t.Fatal("unexpected metadata")
			}
			associationCalls++
			if q.Association.PartID != "P1" || q.Association.InstrumentID != "I1" || q.Association.SourcePart.DocumentID != q.Association.InstrumentLink.DocumentID {
				t.Fatal(q.Association)
			}
			a := validation.AssociationResult{Request: *q.Association, Membership: validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true}, Authority: validation.EvidenceAuthority{ID: "fixture-authority", ExactRelationComplete: true, Provenance: []validation.SourceRef{{IDs: []string{"membership"}, URL: "https://example.test/association", Locator: "test exact membership"}}}, TargetPartID: validation.Evidence[string]{State: validation.EvidenceAbsent}, TargetInstrumentID: validation.Evidence[string]{State: validation.EvidenceAbsent}}
			return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Association: &a}, nil
		})
		opts := LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/score.musicxml"}}
		if selected {
			opts.Dependencies = append(opts.Dependencies, ResourceAssociations)
		}
		b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), opts)
		if err != nil || b.Satisfied() != selected || associationCalls != map[bool]int{false: 0, true: 1}[selected] {
			t.Fatalf("selected%v calls%d err%v report%+v", selected, associationCalls, err, b.PackageReport())
		}
	}
}
