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

func TestResourceGraphChecksLinkedQuotaBeforeResolver(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run(fmt.Sprint(known), func(t *testing.T) {
			source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="one"/><opus-link xlink:href="two"/></opus>`
			calls := 0
			var bodies []*resourceTestBody
			resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				calls++
				if !known {
					return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "unavailable"}, nil
				}
				data := `<opus/>`
				body := &resourceTestBody{Reader: strings.NewReader(data)}
				bodies = append(bodies, body)
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: q.Document.Href, Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(data)))}, RetrievalURI: "https://e.test/" + q.Document.Href, Body: body}, nil
			})
			b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxLinkedDocuments: 1}}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
			if !errors.Is(err, validation.ErrResourceLimit) || b.Satisfied() {
				t.Fatal(err, b.Satisfied())
			}
			if calls != 1 {
				t.Errorf("resolver calls=%d despite linked-document request budget=1", calls)
			}
			for _, body := range bodies {
				if body.closed != 1 {
					t.Errorf("body closes=%d", body.closed)
				}
			}
		})
	}
}
