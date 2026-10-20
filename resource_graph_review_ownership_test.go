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

func reviewGraphSource(t *testing.T, ctx context.Context, source string, options LinkedAssessmentOptions) (*resourceGraphRun, resourceIntrinsicResult, validation.ResourcePin, string) {
	t.Helper()
	g, err := startResourceGraph(ctx, options.internal(), ProfileResourceGraph.ID())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := sourceAssessmentEngineForProfile(ProfileSourceLinks.ID())
	if err != nil {
		t.Fatal(err)
	}
	id := g.next("source")
	result := assessResourceSource(ctx, strings.NewReader(source), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Context: validation.ContextFamilyAll, Account: g.options.account, DocumentID: id, Profile: ProfileSourceLinks.ID(), Engine: engine}, ByteOwner: sourceBytesPrimary})
	if result.Err != nil || !result.DigestComplete {
		t.Fatal(result.Err)
	}
	return g, result, g.primaryPin(validation.ResourceSourceXML, hex.EncodeToString(result.Digest[:])), id
}

func reviewGraphNoLiveLeases(t *testing.T, g *resourceGraphRun) {
	t.Helper()
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
		used, err := g.options.account.Usage(kind)
		if err != nil || used != 0 {
			t.Errorf("cleanup %s=%d err=%v", kind, used, err)
		}
	}
}

func TestReviewGraphDepthRefusesBeforeResolverIO(t *testing.T) {
	for _, returnedKind := range []validation.ResourceRepresentation{validation.ResourceSourceXML, validation.ResourceRawMXL} {
		t.Run(string(returnedKind), func(t *testing.T) {
			source := depthLink("one.opus")
			one := depthLink("two.opus")
			calls := 0
			var bodies []*resourceTestBody
			resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				calls++
				data := []byte(one)
				kind := validation.ResourceSourceXML
				if calls > 1 {
					data = []byte(`<opus/>`)
					kind = returnedKind
					if kind == validation.ResourceRawMXL {
						data = packageGraphBytes(`<opus/>`, "main.opus", "")
					}
				}
				body := &resourceTestBody{Reader: bytes.NewReader(data)}
				bodies = append(bodies, body)
				sum := sha256.Sum256(data)
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: q.Document.Href, Representation: kind, Digest: hex.EncodeToString(sum[:])}, RetrievalURI: "https://e.test/" + q.Document.Href, Body: body}, nil
			})
			g, result, pin, id := reviewGraphSource(t, context.Background(), source, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxDepth: 2}}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
			b, err := g.finishPrimary(result, pin, id)
			var depth *validation.ResourceError
			if !errors.As(err, &depth) || depth.Kind != validation.BudgetDepth || depth.Limit != 2 || depth.Used != 2 || depth.Requested != 1 {
				t.Errorf("incorrect depth witness: %v", err)
			}
			if calls != 1 || b.ResourceCount() != 2 || b.AdoptionCount() != 2 {
				t.Errorf("over-depth target performed I/O/admission: calls=%d resources=%d adoptions=%d", calls, b.ResourceCount(), b.AdoptionCount())
			}
			used, _ := g.options.account.Usage(validation.BudgetExternalBytes)
			if used != int64(len(one)) {
				t.Errorf("over-depth bytes charged: %d, want %d", used, len(one))
			}
			linked, _ := g.options.account.Usage(validation.BudgetLinkedDocuments)
			if linked != 1 {
				t.Errorf("over-depth adoption charged: %d", linked)
			}
			for _, body := range bodies {
				if body.closed != 1 {
					t.Errorf("body closes=%d", body.closed)
				}
			}
			reviewGraphNoLiveLeases(t, g)
		})
	}
}

func TestReviewGraphDepthKeepsKnownSameEntityAdmission(t *testing.T) {
	source := depthLink("child.opus")
	child := depthLink("#")
	resolver := &graphFixtureResolver{resources: map[string]string{"https://e.test/child.opus": child}, pins: map[string]string{"https://e.test/child.opus": "child"}}
	g, result, pin, id := reviewGraphSource(t, context.Background(), source, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxDepth: 2}}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
	b, err := g.finishPrimary(result, pin, id)
	linked, _ := g.options.account.Usage(validation.BudgetLinkedDocuments)
	if err != nil || !b.Satisfied() || b.AdoptionCount() != 3 || linked != 2 || resolver.calls != 1 {
		t.Fatalf("same-entity alias lost fresh admission: %v adoptions=%d linked=%d calls=%d", err, b.AdoptionCount(), linked, resolver.calls)
	}
	reviewGraphNoLiveLeases(t, g)
}

func TestReviewGraphDepthDoesNotInferExternalIdentityFromURI(t *testing.T) {
	source := depthLink("child.opus")
	child := depthLink("root.opus")
	resolver := &graphFixtureResolver{resources: map[string]string{"https://e.test/child.opus": child, "https://e.test/root.opus": source}, pins: map[string]string{"https://e.test/child.opus": "child", "https://e.test/root.opus": "different-canonical-root"}}
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxDepth: 2}}, Identity: ResourceIdentity{CanonicalID: "root", RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
	if !errors.Is(err, validation.ErrResourceLimit) || resolver.calls != 1 || b.ResourceCount() != 2 {
		t.Fatalf("unknown same-URI target crossed depth boundary: %v calls=%d resources=%d", err, resolver.calls, b.ResourceCount())
	}
}

func TestReviewGraphAliasMXLReleasesUnusedOwnership(t *testing.T) {
	archive := packageGraphBytes(`<opus/>`, "main.opus", "", assessmentZIPFixture{name: "large.bin", body: strings.Repeat("x", 128<<10)})
	source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink">` + strings.Repeat(`<opus-link xlink:href="same.mxl"/>`, 4) + `</opus>`
	var g *resourceGraphRun
	var bodies []*resourceTestBody
	var bodyOwners, archiveOwners []int
	var buffered []int64
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		used, _ := g.options.account.Usage(validation.BudgetBufferedBytes)
		buffered = append(buffered, used)
		bodyOwners = append(bodyOwners, len(g.resourceBodies))
		archiveOwners = append(archiveOwners, len(g.archives))
		body := &resourceTestBody{Reader: bytes.NewReader(archive)}
		bodies = append(bodies, body)
		sum := sha256.Sum256(archive)
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "one-archive", Representation: validation.ResourceRawMXL, Digest: hex.EncodeToString(sum[:])}, RetrievalURI: "https://e.test/same.mxl", Body: body}, nil
	})
	var result resourceIntrinsicResult
	var pin validation.ResourcePin
	var id string
	g, result, pin, id = reviewGraphSource(t, context.Background(), source, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
	b, err := g.finishPrimary(result, pin, id)
	if err != nil || !b.EnumerationComplete() {
		t.Fatal(err, b.PackageReport())
	}
	for i, want := range []int{0, 1, 1, 1} {
		if bodyOwners[i] != want || archiveOwners[i] != want {
			t.Errorf("alias retained discarded representation: bodies=%v archives=%v", bodyOwners, archiveOwners)
			break
		}
	}
	for i := 2; i < len(buffered); i++ {
		if buffered[i]-buffered[i-1] >= int64(len(archive))/2 {
			t.Errorf("discarded alias retained byte lease: %v", buffered)
		}
	}
	used, _ := g.options.account.Usage(validation.BudgetExternalBytes)
	if used < 4*int64(len(archive)) {
		t.Errorf("alias external quota refunded: %d", used)
	}
	for _, body := range bodies {
		if body.closed != 1 {
			t.Errorf("body closes=%d", body.closed)
		}
	}
	reviewGraphNoLiveLeases(t, g)
}

func TestReviewGraphQueueCancellationBeforeWorkCharge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	g, err := startResourceGraph(ctx, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph}}.internal(), ProfileResourceGraph.ID())
	if err != nil {
		t.Fatal(err)
	}
	g.queue = []graphExpansion{{}}
	before, _ := g.options.account.Usage(validation.BudgetRuleSteps)
	cancel()
	err = g.walkResources()
	after, _ := g.options.account.Usage(validation.BudgetRuleSteps)
	if !errors.Is(err, context.Canceled) || after != before {
		t.Errorf("canceled queue charged work: %v before=%d after=%d", err, before, after)
	}
	g.finish(err)
	reviewGraphNoLiveLeases(t, g)
}

func TestReviewGraphFinishDoesNotDuplicateCause(t *testing.T) {
	g, err := startResourceGraph(context.Background(), LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph}}.internal(), ProfileResourceGraph.ID())
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("unique transport failure")
	_, err = g.finish(boom)
	if !errors.Is(err, boom) || strings.Count(err.Error(), boom.Error()) != 1 {
		t.Fatalf("duplicate finish cause: %v", err)
	}
	reviewGraphNoLiveLeases(t, g)
}

func TestReviewGraphCanonicalRevisionReasonAndDepthWitness(t *testing.T) {
	g, err := startResourceGraph(context.Background(), LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph}}.internal(), ProfileResourceGraph.ID())
	if err != nil {
		t.Fatal(err)
	}
	pin := validation.ResourcePin{CanonicalID: "resource", Revision: "old", Representation: validation.ResourceSourceXML}
	if _, err = g.cacheResource(pin, ProfileSourceLinks.ID(), "id", 9, true); err != nil {
		t.Fatal(err)
	}
	pin.Revision = "new"
	_, err = g.cacheResource(pin, ProfileSourceLinks.ID(), "other-id", 9, true)
	if !errors.Is(err, validation.ErrInvalidConfiguration) || !strings.Contains(err.Error(), "revision") {
		t.Errorf("revision-only mismatch blamed content: %v", err)
	}
	g.packages["mxl"] = &graphPackage{}
	g.mxlOptions.MaxXMLDepth = 3
	err = g.checkIntrinsicXMLDepth(&graphResource{xmlDepth: 9}, "mxl")
	var depth *validation.ResourceError
	if !errors.As(err, &depth) || depth.Used != 9 || depth.Requested != 0 || depth.Limit != 3 {
		t.Errorf("depth witness was fabricated: %v", err)
	}
	g.finish(err)
	reviewGraphNoLiveLeases(t, g)
}

func TestReviewGraphDepthLocalSelectionAndKnownArchiveCycle(t *testing.T) {
	for _, backref := range []bool{false, true} {
		t.Run(map[bool]string{false: "unknown-entry", true: "known-cycle"}[backref], func(t *testing.T) {
			last := "never.opus"
			if backref {
				last = "one.opus"
			}
			doc, err := Decode(strings.NewReader(depthLink("one.opus")))
			if err != nil {
				t.Fatal(err)
			}
			g, err := startResourceGraph(context.Background(), MXLAssessmentOptions{Graph: MXLGraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileMXLModel, Budgets: validation.Budgets{MaxDepth: 3}}}.internal().linked(), ProfileMXLModel.ID())
			if err != nil {
				t.Fatal(err)
			}
			p, err := g.newPackage(g.primaryPin(validation.ResourceCurrentMXL, ""), "", "https://e.test/root.mxl", nil)
			if err != nil {
				t.Fatal(err)
			}
			value := &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "root.opus"}}, Resources: []MXLResource{{Path: "one.opus", Data: []byte(depthLink("two.opus"))}, {Path: "two.opus", Data: []byte(depthLink(last))}, {Path: "never.opus", Data: []byte(`<opus/>`)}}}
			if err = g.importCurrentPackage(p, value, "https://e.test/root.mxl"); err != nil {
				t.Fatal(err)
			}
			err = g.walkResources()
			if backref && err != nil {
				t.Fatalf("known exact archive cycle refused: %v", err)
			}
			if !backref && !errors.Is(err, validation.ErrResourceLimit) {
				t.Fatalf("unknown over-depth archive target accepted: %v", err)
			}
			if _, copied := p.copied["never.opus"]; copied {
				t.Error("over-depth archive member was copied/read")
			}
			linked, _ := g.options.account.Usage(validation.BudgetLinkedDocuments)
			want := int64(2)
			if backref {
				want = 3
			}
			if linked != want {
				t.Errorf("incoming cycle/new-target charge=%d want=%d", linked, want)
			}
			g.finish(err)
			reviewGraphNoLiveLeases(t, g)
		})
	}
}

func TestReviewGraphJoinPreservesUnrelatedTransportCauses(t *testing.T) {
	one, two := errors.New("first unique cause"), errors.New("second unique cause")
	for _, input := range []error{one, errors.Join(one, two)} {
		_, err := InspectXMLWithResources(context.Background(), &resourceReadError{data: []byte(`<opus>`), err: input}, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph}})
		if !errors.Is(err, one) || strings.Count(err.Error(), one.Error()) != 1 {
			t.Errorf("first cause repeated/lost: %v", err)
		}
		if errors.Is(input, two) && (!errors.Is(err, two) || strings.Count(err.Error(), two.Error()) != 1) {
			t.Errorf("second cause repeated/lost: %v", err)
		}
	}
}

func TestReviewGraphRawArchiveFailureClosesPartialOwners(t *testing.T) {
	for _, mode := range []string{"malformed-primary", "broken-archive", "linked-quota"} {
		t.Run(mode, func(t *testing.T) {
			data := packageGraphBytes(`<opus/>`, "main.opus", "")
			budgets := validation.Budgets{}
			switch mode {
			case "malformed-primary":
				data = packageGraphBytes(`<opus>`, "main.opus", "")
			case "broken-archive":
				data = data[:len(data)-22]
			case "linked-quota":
				budgets.MaxLinkedDocuments = 1
			}
			body := &resourceTestBody{Reader: bytes.NewReader(data)}
			sum := sha256.Sum256(data)
			resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "archive", Representation: validation.ResourceRawMXL, Digest: hex.EncodeToString(sum[:])}, RetrievalURI: "https://e.test/archive.mxl", Body: body}, nil
			})
			g, result, pin, id := reviewGraphSource(t, context.Background(), depthLink("archive.mxl"), LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph, Budgets: budgets}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
			b, err := g.finishPrimary(result, pin, id)
			if err == nil || b.EnumerationComplete() || body.closed != 1 {
				t.Errorf("failed import result=%v complete=%v closes=%d", err, b.EnumerationComplete(), body.closed)
			}
			reviewGraphNoLiveLeases(t, g)
		})
	}
}
