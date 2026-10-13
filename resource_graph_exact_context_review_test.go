package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"testing"
)

func TestIndependentReviewArchivePrimaryExactBase(t *testing.T) {
	for _, limit := range []int64{3, 8} {
		t.Run(fmt.Sprintf("depth-%d", limit), func(t *testing.T) {
			testIndependentReviewArchivePrimaryExactBase(t, limit)
		})
	}
}

func testIndependentReviewArchivePrimaryExactBase(t *testing.T, limit int64) {
	primary := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://e.test/b.mxl"/><opus-link xlink:href="child.opus"/></opus>`
	data := packageGraphBytes(primary, "main.opus", "", assessmentZIPFixture{name: "child.opus", body: depthLink("main.opus")})
	sum := sha256.Sum256(data)
	calls := 0
	var bodies []*resourceTestBody
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		calls++
		body := &resourceTestBody{Reader: bytes.NewReader(data)}
		bodies = append(bodies, body)
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "root-archive", Representation: validation.ResourceRawMXL, Digest: fmt.Sprintf("%x", sum)}, RetrievalURI: "https://e.test/b.mxl", Body: body}, nil
	})
	opts := packageGraphOptions()
	opts.Graph.Budgets.MaxDepth = limit
	opts.Identity = ResourceIdentity{CanonicalID: "root-archive", RetrievalURI: "https://e.test/a.mxl"}
	opts.Resolver = resolver
	opts.Dependencies = []ResourceDependency{ResourceDocuments}
	b, err := InspectMXL(context.Background(), bytes.NewReader(data), opts)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || b.AdoptionCount() != 8 || b.ResourceCount() != 3 {
		t.Fatalf("alias identities/admissions changed: calls=%d adoptions=%d resources=%d", calls, b.AdoptionCount(), b.ResourceCount())
	}
	for _, body := range bodies {
		if body.closed != 1 {
			t.Errorf("alias body closes=%d", body.closed)
		}
	}
	last, ok := b.AdoptionContext(b.AdoptionCount() - 1)
	if !ok || last.ArchiveEntry != "main.opus" || last.BaseURI != "https://e.test/b.mxl" {
		t.Fatalf("return changed exact retrieval context: %+v", last)
	}
	owner, ok := b.AdoptionContext(2)
	if !ok || last.ExpansionOwner != owner.Adoption || last.Adoption == owner.Adoption || last.Assessment != owner.Assessment || last.Package != owner.Package {
		t.Fatalf("alias lost its exact expansion owner or fresh adoption: alias=%+v owner=%+v", last, owner)
	}
}

func TestReviewArchivePrimaryDifferentBaseNeedsOwnExpansion(t *testing.T) {
	g, err := startResourceGraph(context.Background(), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxDepth: 3}}}, ProfileResourceGraph)
	if err != nil {
		t.Fatal(err)
	}
	primary := &graphResource{id: "primary"}
	original := resourceURIBase{RetrievalURI: "https://e.test/a.mxl", Archive: true, Entry: "main.opus"}
	alias := original
	alias.RetrievalURI = "https://e.test/b.mxl"
	g.packages["root"] = &graphPackage{id: "root", primary: primary, primaryPath: "main.opus", primaryBase: original, primaryPackage: "root"}
	g.expanded[graphExpansionKey{primary.id, "root", original}] = true
	g.queue = []graphExpansion{{resource: primary, pkg: "root", base: original, depth: 1}}
	child := graphExpansion{resource: &graphResource{id: "child"}, pkg: "root", base: resourceURIBase{RetrievalURI: alias.RetrievalURI, Archive: true, Entry: "child.opus"}, depth: 3}
	if target, ok, err := g.knownArchiveTarget(child, "main.opus"); err != nil || ok {
		t.Errorf("unadmitted alias base reused old owner: target=%+v ok=%t err=%v", target, ok, err)
	}
	g.expanded[graphExpansionKey{primary.id, "root", alias}] = true
	if target, ok, err := g.knownArchiveTarget(child, "main.opus"); err != nil || !ok || target.resource != primary || target.pkg != "root" || target.base != alias {
		t.Errorf("exact admitted alias was not retained: target=%+v ok=%t err=%v", target, ok, err)
	}
	_, base, pkg, _, err := g.loadArchiveDocument(child, "main.opus")
	if err != nil || base != alias || pkg != "root" {
		t.Errorf("normal primary selection lost alias context: base=%+v pkg=%s err=%v", base, pkg, err)
	}
	g.finish(nil)
	reviewGraphNoLiveLeases(t, g)
}
