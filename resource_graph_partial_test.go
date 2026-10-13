package musicxml

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestResourceGraphRetainsRejectedTargetKindWithoutExpansion(t *testing.T) {
	original, originalErr := InspectXML(context.Background(), strings.NewReader(`<sounds/>`), AssessmentOptions{Profile: ProfileSourceLinks})
	if originalErr == nil || original.Terminal == nil {
		t.Fatalf("missing rejected-child fixture: %+v %v", original, originalErr)
	}
	for _, fragment := range []string{"", "#", "#missing"} {
		t.Run(fragment, func(t *testing.T) {
			calls := 0
			var returnedBody *resourceTestBody
			r := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				calls++
				body := `<sounds/>`
				returnedBody = &resourceTestBody{Reader: strings.NewReader(body)}
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "bad-target", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(body)))}, Body: returnedBody, RetrievalURI: "https://e.test/bad.xml"}, nil
			})
			source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="bad.xml` + fragment + `"/><score xlink:href="never.xml"/></opus>`
			b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Resolver: r, Dependencies: []ResourceDependency{ResourceDocuments}})
			if err == nil || !errors.Is(err, ErrUnsupportedRoot) || b.Satisfied() || calls != 1 || b.ResourceCount() != 1 || b.AdoptionCount() != 1 {
				t.Fatalf("err=%v calls=%d resources=%d adoptions=%d", err, calls, b.ResourceCount(), b.AdoptionCount())
			}
			// An unknown assembly is refused rather than adopted under an
			// arbitrary intrinsic row. Preserve its original independent truth
			// in the failure channel without manufacturing a new edge predicate.
			failure, ok := b.Failure()
			if !ok || failure.Target != original.Target || failure.Profile != original.Profile.ID || failure.CatalogDigest != original.Profile.CatalogDigest || failure.Counts != original.Counts || failure.Conformance != original.Conformance || failure.Terminal != *original.Terminal {
				t.Fatalf("unadmitted original child result lost: %+v", failure)
			}
			if returnedBody == nil || returnedBody.closed != 1 {
				t.Fatalf("resolver body closed %v times", returnedBody)
			}
			if b.EnumerationComplete() {
				t.Fatal("partial failed target certified complete")
			}
			for i := 0; i < b.AdoptionCount(); i++ {
				a, _ := b.AdoptionContext(i)
				if a.ExpansionComplete.State == validation.EvidenceKnown && a.ExpansionComplete.Value {
					t.Fatal("interrupted expansion certified complete")
				}
			}
		})
	}
}
