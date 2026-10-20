package musicxml

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

type resourceReviewCountingReader struct {
	*strings.Reader
	reads int
}

func (r *resourceReviewCountingReader) Read(p []byte) (int, error) {
	r.reads++
	return r.Reader.Read(p)
}

func TestResourceGraphReturnedURIAdmissionBeforeRead(t *testing.T) {
	for _, uri := range []string{"relative/child.opus", "https://e.test/%zz"} {
		t.Run(uri, func(t *testing.T) {
			source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="child.opus"/></opus>`
			data := `<opus/>`
			reader := &resourceReviewCountingReader{Reader: strings.NewReader(data)}
			body := &resourceTestBody{Reader: reader}
			resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "child", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(data)))}, RetrievalURI: uri, Body: body}, nil
			})
			_, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: resolver})
			if !errors.Is(err, validation.ErrInvalidConfiguration) || reader.reads != 0 || body.closed != 1 {
				t.Fatalf("err=%v reads=%d closed=%d", err, reader.reads, body.closed)
			}
		})
	}
}
func TestResourceGraphImageCallbackCannotMutateRequest(t *testing.T) {
	source := resourceScore(`<direction><direction-type><image source="plot.dat" type="image/png"/></direction-type></direction>`)
	calls := 0
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		calls++
		q.Image.Href = "forged.dat"
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "metadata-unavailable"}, nil
	})
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Dependencies: []ResourceDependency{ResourceImages}, Resolver: resolver})
	if err != nil || calls != 1 || b.ImageAdviceCount() != 1 {
		t.Fatalf("err=%v calls=%d advice=%d", err, calls, b.ImageAdviceCount())
	}
}
func TestResourceGraphSelectedMissingResolverReason(t *testing.T) {
	source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="child.opus"/></opus>`
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Dependencies: []ResourceDependency{ResourceDocuments}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range b.PackageReport().Findings {
		if d.Reason == "resolver-unavailable" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing reason: %+v", b.PackageReport())
	}
}

func TestResourceGraphCallbackCopyAdmittedBeforeCall(t *testing.T) {
	q := validation.ResourceRequest{ID: "image", Image: &validation.ImageMetadataRequest{ID: "image", Source: validation.ResourcePin{CanonicalID: "source", Digest: strings.Repeat("a", 64), Representation: validation.ResourceSourceXML}, Subject: validation.NodeRef{DocumentID: "source", Node: 1}, Href: "plot.png", MediaType: "image/png"}}
	measure, _ := validation.NewBudgetAccount(validation.Budgets{})
	release, err := resourceWorkingLease(context.Background(), measure, q)
	if err != nil {
		t.Fatal(err)
	}
	one, _ := measure.Usage(validation.BudgetBufferedBytes)
	release()
	a, _ := validation.NewBudgetAccount(validation.Budgets{MaxBufferedBytes: one})
	calls := 0
	g := resourceGraphRun{ctx: context.Background(), options: resourceRunOptions{account: a, resolver: graphResolverFunc(func(_ context.Context, r validation.ResourceRequest) (validation.ResourceResult, error) {
		calls++
		return validation.ResourceResult{RequestID: r.ID, State: validation.EvidenceUnknown, Reason: "unavailable"}, nil
	})}}
	_, err = g.resolveMetadata(q)
	if !errors.Is(err, validation.ErrResourceLimit) || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 0 {
		t.Fatalf("live=%d", live)
	}
}
