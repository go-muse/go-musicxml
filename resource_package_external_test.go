package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

type packageResolverFixture struct {
	data      map[string][]byte
	canonical map[string]string
	revision  map[string]string
	bodies    []*resourceTestBody
	calls     int
	closeErr  error
}

func (r *packageResolverFixture) Resolve(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
	if q.Document == nil {
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "metadata-not-selected"}, nil
	}
	r.calls++
	data, ok := r.data[q.Document.Href]
	if !ok {
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "fixture-missing"}, nil
	}
	body := &resourceTestBody{Reader: bytes.NewReader(data), closeErr: r.closeErr}
	r.bodies = append(r.bodies, body)
	sum := sha256.Sum256(data)
	pin := validation.ResourcePin{CanonicalID: r.canonical[q.Document.Href], Revision: r.revision[q.Document.Href], Digest: hex.EncodeToString(sum[:]), Representation: validation.ResourceRawMXL}
	return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: pin, RetrievalURI: q.Document.Href, Body: body}, nil
}
func TestPackageGraphExternalRawMXLAndAliasContexts(t *testing.T) {
	archive := packageGraphBytes(`<opus/>`, "main.opus", "")
	source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://e.test/a.mxl"/><opus-link xlink:href="https://e.test/b.mxl"/></opus>`
	resolver := &packageResolverFixture{data: map[string][]byte{"https://e.test/a.mxl": archive, "https://e.test/b.mxl": archive}, canonical: map[string]string{"https://e.test/a.mxl": "same", "https://e.test/b.mxl": "same"}}
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
	if err != nil || b.Satisfied() || !b.EnumerationComplete() || b.ResourceCount() != 3 || resolver.calls != 2 {
		t.Fatalf("err=%v resources=%d calls=%d findings=%v", err, b.ResourceCount(), resolver.calls, b.PackageReport().Findings)
	}
	requireIndirectionContextUnknown(t, b, 2)
	requireIndirectionIntrinsicSatisfied(t, b)
	contexts := map[string]validation.PackageID{}
	for i := 0; i < b.AdoptionCount(); i++ {
		a, _ := b.AdoptionContext(i)
		if a.ArchiveEntry == "main.opus" {
			contexts[a.BaseURI] = a.Package
		}
	}
	if contexts["https://e.test/a.mxl"] == "" || contexts["https://e.test/b.mxl"] == "" || contexts["https://e.test/a.mxl"] == contexts["https://e.test/b.mxl"] {
		t.Fatal("aliases lost encapsulation contexts", contexts)
	}
	for _, body := range resolver.bodies {
		if body.closed != 1 {
			t.Fatal(body.closed)
		}
	}
}
func TestPackageGraphExternalRawMXLRejectsRevisionConflict(t *testing.T) {
	archive := packageGraphBytes(`<opus/>`, "main.opus", "")
	source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://e.test/a.mxl"/><opus-link xlink:href="https://e.test/b.mxl"/></opus>`
	resolver := &packageResolverFixture{data: map[string][]byte{"https://e.test/a.mxl": archive, "https://e.test/b.mxl": archive}, canonical: map[string]string{"https://e.test/a.mxl": "same", "https://e.test/b.mxl": "same"}, revision: map[string]string{"https://e.test/a.mxl": "one", "https://e.test/b.mxl": "two"}}
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
	if !errors.Is(err, validation.ErrInvalidConfiguration) || b.EnumerationComplete() {
		t.Fatal("accepted canonical revision conflict", err, b.PackageReport().Findings)
	}
	for _, body := range resolver.bodies {
		if body.closed != 1 {
			t.Fatal(body.closed)
		}
	}
}
func TestPackageGraphExternalRawArchiveSelfReference(t *testing.T) {
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://e.test/a.mxl"/></opus>`
	archive := packageGraphBytes(root, "main.opus", "")
	resolver := &packageResolverFixture{data: map[string][]byte{"https://e.test/a.mxl": archive}, canonical: map[string]string{"https://e.test/a.mxl": "archive"}}
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(root), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
	if err != nil || b.ResourceCount() != 3 || resolver.calls != 2 {
		t.Fatalf("archive URI was mistaken for contained XML: %v resources%d calls%d", err, b.ResourceCount(), resolver.calls)
	}
	for _, body := range resolver.bodies {
		if body.closed != 1 {
			t.Fatal(body.closed)
		}
	}
}
func TestPackageGraphExternalRawBodyCloseError(t *testing.T) {
	archive := packageGraphBytes(`<opus/>`, "main.opus", "")
	boom := errors.New("body close failed")
	resolver := &packageResolverFixture{data: map[string][]byte{"https://e.test/a.mxl": archive}, canonical: map[string]string{"https://e.test/a.mxl": "archive"}, closeErr: boom}
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://e.test/a.mxl"/></opus>`
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(root), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
	if !errors.Is(err, boom) || b.EnumerationComplete() || len(resolver.bodies) != 1 || resolver.bodies[0].closed != 1 {
		t.Fatal(err, b.PackageReport().Findings)
	}
}
func TestPackageGraphWrongRootRetainsIndependentViolation(t *testing.T) {
	original, originalErr := InspectXML(context.Background(), strings.NewReader(`<sounds/>`), SourceOptions{Profile: ProfileSourceLinks})
	if originalErr == nil || original.Terminal == nil {
		t.Fatalf("missing rejected-root fixture: %+v %v", original, originalErr)
	}
	b, err := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(`<sounds/>`, "main.musicxml", "")), packageGraphOptions())
	if !errors.Is(err, ErrUnsupportedRoot) || b.ResourceCount() != 1 || b.Satisfied() || b.EnumerationComplete() {
		t.Fatal(err, b.ResourceCount(), b.PackageReport().Findings)
	}
	// The original rejected-root result remains independent; an unknown
	// assembly cannot be transferred to invent a package rootfile predicate.
	failure, ok := b.Failure()
	if !ok || failure.Target != original.Target || failure.Profile != original.Profile.ID || failure.CatalogDigest != original.Profile.CatalogDigest || failure.Counts != original.Counts || failure.Conformance != original.Conformance || failure.Terminal != *original.Terminal {
		t.Fatalf("unadmitted original root result lost: %+v", failure)
	}
}
