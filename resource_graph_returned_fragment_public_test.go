package musicxml_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
	"testing"

	musicxml "github.com/go-muse/go-musicxml"
	"github.com/go-muse/go-musicxml/validation"
)

type returnedFragmentResolver func(context.Context, validation.ResourceRequest) (validation.ResourceResult, error)

func (f returnedFragmentResolver) Resolve(ctx context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
	return f(ctx, q)
}

type returnedFragmentBody struct {
	*strings.Reader
	closes int
}

func (b *returnedFragmentBody) Close() error {
	b.closes++
	return nil
}

func TestResourceGraphReturnedRetrievalFragmentPublic(t *testing.T) {
	const (
		rootURI   = "https://origin.test/root.opus"
		childURI  = "https://cdn.test/scores/child.musicxml?edition=1"
		leafURI   = "https://cdn.test/leaf.musicxml"
		childHref = "../leaf.musicxml#P2"
		childXML  = `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink"><part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list><part id="P1"><measure number="1"><link xlink:href="` + childHref + `"/></measure></part></score-partwise>`
		leafXML   = `<score-partwise><part-list><score-part id="P2"><part-name>P</part-name></score-part></part-list><part id="P2"><measure number="1"/></part></score-partwise>`
	)
	for _, tc := range []struct {
		name, fragment string
		missing        bool
	}{
		{name: "absent"},
		{name: "empty", fragment: "#"},
		{name: "schema-id", fragment: "#P1"},
		{name: "missing-id", fragment: "#missing", missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rootHref := "requested.musicxml" + tc.fragment
			rootXML := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="` + rootHref + `"/></opus>`
			calls := 0
			var bodies []*returnedFragmentBody
			resolver := returnedFragmentResolver(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				calls++
				if q.Document == nil {
					t.Fatalf("unexpected non-document request: %+v", q)
				}
				var canonical, retrieval, source string
				switch calls {
				case 1:
					if q.Document.BaseURI != rootURI || q.Document.Href != rootHref {
						t.Errorf("root request lost its link fragment: %+v", q.Document)
					}
					// This transport fragment deliberately names no schema ID. It
					// must neither become the child's base nor replace the href fragment.
					canonical, retrieval, source = "child", childURI+"#transport-only", childXML
				case 2:
					if q.Document.BaseURI != childURI || q.Document.Href != childHref {
						t.Errorf("child request did not retain the normalized returned base and exact href: %+v", q.Document)
					}
					base, err := url.Parse(q.Document.BaseURI)
					if err != nil {
						t.Fatal(err)
					}
					ref, err := url.Parse(q.Document.Href)
					if err != nil {
						t.Fatal(err)
					}
					if got := base.ResolveReference(ref).String(); got != leafURI+"#P2" {
						t.Errorf("child href resolved to %q, want %q", got, leafURI+"#P2")
					}
					canonical, retrieval, source = "leaf", leafURI, leafXML
				default:
					t.Fatalf("unexpected extra resolver request: %+v", q.Document)
				}
				body := &returnedFragmentBody{Reader: strings.NewReader(source)}
				bodies = append(bodies, body)
				return validation.ResourceResult{
					RequestID: q.ID, State: validation.EvidenceKnown,
					Pin:          validation.ResourcePin{CanonicalID: canonical, Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(source)))},
					RetrievalURI: retrieval, Body: body,
				}, nil
			})
			bundle, err := musicxml.InspectXMLWithResources(context.Background(), strings.NewReader(rootXML), musicxml.LinkedAssessmentOptions{
				Graph:        musicxml.AssessmentOptions{Profile: musicxml.ProfileResourceGraph},
				Identity:     musicxml.ResourceIdentity{CanonicalID: "root", RetrievalURI: rootURI},
				Dependencies: []musicxml.ResourceDependency{musicxml.ResourceDocuments}, Resolver: resolver,
			})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 2 || bundle.ResourceCount() != 3 || bundle.AdoptionCount() != 3 || !bundle.EnumerationComplete() {
				t.Fatalf("incomplete graph: calls=%d resources=%d adoptions=%d complete=%t", calls, bundle.ResourceCount(), bundle.AdoptionCount(), bundle.EnumerationComplete())
			}
			for i, body := range bodies {
				if body.closes != 1 {
					t.Errorf("body %d closed %d times, want 1", i, body.closes)
				}
			}
			var adoptions []validation.ResourceAdoptionContext
			for i, want := range []string{rootURI, childURI, leafURI} {
				adoption, ok := bundle.AdoptionContext(i)
				if !ok || adoption.BaseURI != want || adoption.ArchiveEntry != "" {
					t.Errorf("adoption %d retained the wrong retrieval context: %+v, want base %q", i, adoption, want)
				}
				adoptions = append(adoptions, adoption)
			}
			if bundle.FileMetadataCount() != 3 {
				t.Fatalf("file metadata count=%d, want 3", bundle.FileMetadataCount())
			}
			seen := map[string]bool{}
			for i := 0; i < bundle.FileMetadataCount(); i++ {
				metadata, ok := bundle.FileMetadataAt(i)
				if !ok {
					t.Fatalf("missing file metadata %d", i)
				}
				var base, name, href string
				var source validation.ResourceAdoptionID
				switch metadata.Pin.CanonicalID {
				case "root":
					base, name = rootURI, "root.opus"
				case "child":
					base, name, href, source = childURI, "child.musicxml", rootHref, adoptions[0].Adoption
				case "leaf":
					base, name, href, source = leafURI, "leaf.musicxml", childHref, adoptions[1].Adoption
				default:
					t.Fatalf("unexpected file pin: %+v", metadata.Pin)
				}
				if seen[metadata.Pin.CanonicalID] {
					t.Errorf("duplicate metadata for %q", metadata.Pin.CanonicalID)
				}
				seen[metadata.Pin.CanonicalID] = true
				if metadata.BaseURI != base || metadata.Name.State != validation.EvidenceKnown || metadata.Name.Value != name || metadata.Href != href || metadata.Source != source {
					t.Errorf("file metadata lost fragment-free retrieval or encoded link context: %+v", metadata)
				}
			}
			missing := 0
			for _, finding := range bundle.PackageReport().Findings {
				if finding.Reason == "fragment-id-absent" && finding.Outcome == validation.OutcomeFail {
					missing++
				}
			}
			wantMissing := 0
			if tc.missing {
				wantMissing = 1
			}
			if missing != wantMissing || bundle.Satisfied() == tc.missing {
				t.Fatalf("href fragment semantics changed: missing=%d want=%d satisfied=%t report=%+v", missing, wantMissing, bundle.Satisfied(), bundle.PackageReport())
			}
		})
	}
}
