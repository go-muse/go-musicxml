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

func TestResourceGraphReviewUnknownAssemblyCauseOnce(t *testing.T) {
	target := `<sounds/>`
	sum := sha256.Sum256([]byte(target))
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "unsupported-child", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sum)}, RetrievalURI: "https://e.test/child.xml", Body: &resourceTestBody{Reader: strings.NewReader(target)}}, nil
	})
	_, err := InspectXMLWithResources(context.Background(), strings.NewReader(`<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="https://e.test/child.xml"/></opus>`), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
	if !errors.Is(err, ErrUnsupportedRoot) || !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatal("original identities lost", err)
	}
	n := strings.Count(err.Error(), "established intrinsic assembly required")
	if n != 1 {
		t.Fatalf("transfer cause repeated %d times: %v", n, err)
	}
}
