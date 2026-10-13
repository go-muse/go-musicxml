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
	for _, fragment := range []string{"", "#", "#missing"} {
		t.Run(fragment, func(t *testing.T) {
			calls := 0
			r := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				calls++
				body := `<sounds/>`
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "bad-target", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(body)))}, Body: &resourceTestBody{Reader: strings.NewReader(body)}, RetrievalURI: "https://e.test/bad.xml"}, nil
			})
			source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="bad.xml` + fragment + `"/><score xlink:href="never.xml"/></opus>`
			b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Resolver: r, Dependencies: []ResourceDependency{ResourceDocuments}})
			if err == nil || errors.Is(err, validation.ErrInvalidConfiguration) || b.Satisfied() || calls != 1 || b.ResourceCount() != 2 || b.AdoptionCount() != 2 {
				t.Fatalf("err=%v calls=%d resources=%d adoptions=%d", err, calls, b.ResourceCount(), b.AdoptionCount())
			}
			found := false
			for _, f := range b.PackageReport().Findings {
				if f.Reason == "wrong-linked-document-kind" && f.Outcome == validation.OutcomeFail {
					found = true
				}
			}
			if !found {
				t.Fatalf("known rejected root kind lost: %+v", b.PackageReport().Findings)
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
