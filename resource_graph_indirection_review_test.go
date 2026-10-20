package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestResourceGraphArchivePrimaryIndirectionPublic(t *testing.T) {
	for _, inner := range []struct{ name, xml string }{
		{"matching-opus", `<opus/>`},
		{"different-score", `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list><part id="P1"><measure number="1"/></part></score-partwise>`},
	} {
		for _, mode := range []string{"raw-package", "current-package", "external-source", "external-model"} {
			for _, count := range []int{1, 2} {
				t.Run(inner.name+"/"+mode+"/"+strings.Repeat("alias", count), func(t *testing.T) {
					archive := packageGraphBytes(inner.xml, "primary.xml", "")
					source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink">` + strings.Repeat(`<opus-link xlink:href="inner.mxl"/>`, count) + `</opus>`
					var bundle validation.AssessmentBundle
					var err error
					calls := 0
					var bodies []*resourceTestBody
					resolver := graphResolverFunc(func(_ context.Context, request validation.ResourceRequest) (validation.ResourceResult, error) {
						calls++
						sum := sha256.Sum256(archive)
						body := &resourceTestBody{Reader: bytes.NewReader(archive)}
						bodies = append(bodies, body)
						return validation.ResourceResult{RequestID: request.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "inner", Representation: validation.ResourceRawMXL, Digest: hex.EncodeToString(sum[:])}, RetrievalURI: "https://example.test/inner.mxl", Body: body}, nil
					})
					switch mode {
					case "raw-package":
						outer := packageGraphBytes(source, "main.opus", "", assessmentZIPFixture{name: "inner.mxl", body: string(archive)})
						bundle, err = InspectMXL(context.Background(), bytes.NewReader(outer), packageGraphOptions())
					case "current-package":
						var doc Document
						doc, err = Decode(strings.NewReader(source))
						if err == nil {
							bundle, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "main.opus"}}, Resources: []MXLResource{{Path: "inner.mxl", Data: archive}}}, MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel}})
						}
					default:
						options := LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://example.test/main.opus"}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: resolver}
						if mode == "external-source" {
							bundle, err = InspectXMLWithResources(context.Background(), strings.NewReader(source), options)
						} else {
							var doc Document
							doc, err = Decode(strings.NewReader(source))
							if err == nil {
								bundle, err = AssessModelWithResources(context.Background(), doc, options)
							}
						}
					}
					if err != nil {
						t.Fatal(err)
					}
					findings := 0
					for _, finding := range bundle.PackageReport().Findings {
						if finding.Rule.ID != "MX40-PACKAGE-link-target" {
							continue
						}
						findings++
						if finding.Outcome != validation.OutcomeUnknown || finding.Reason != "nested-link-indirection-unresolved" {
							t.Errorf("inner primary promoted to direct target: %+v", finding)
						}
					}
					if findings != count || bundle.Satisfied() || !bundle.EnumerationComplete() {
						t.Errorf("outer uncertainty lost: findings=%d want=%d satisfied=%t enumeration=%t", findings, count, bundle.Satisfied(), bundle.EnumerationComplete())
					}
					if bundle.ResourceCount() < 3 {
						t.Errorf("inner reports discarded: resources=%d", bundle.ResourceCount())
					}
					if strings.HasPrefix(mode, "external") && calls != count {
						t.Errorf("alias resolution count=%d want=%d", calls, count)
					}
					for _, body := range bodies {
						if body.closed != 1 {
							t.Errorf("body closed %d times", body.closed)
						}
					}
				})
			}
		}
	}
}

// The graph owns each transferred archive body and every retained child, even
// when cancellation interrupts the body before its package can be imported.
func TestResourceGraphIndirectionResourceCleanup(t *testing.T) {
	for _, cancelAt := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("cancel-body=%d", cancelAt), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			archive := packageGraphBytes(`<opus wrong="yes"/>`, "primary.opus", "")
			var bodies []*resourceTestBody
			resolver := graphResolverFunc(func(_ context.Context, request validation.ResourceRequest) (validation.ResourceResult, error) {
				var reader io.Reader = bytes.NewReader(archive)
				if cancelAt == len(bodies)+1 {
					reader = &indirectionCancelReader{reader: reader, cancel: cancel}
				}
				body := &resourceTestBody{Reader: reader}
				bodies = append(bodies, body)
				return validation.ResourceResult{RequestID: request.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "inner", Representation: validation.ResourceRawMXL, Digest: fmt.Sprintf("%x", sha256.Sum256(archive))}, RetrievalURI: "https://e.test/inner.mxl", Body: body}, nil
			})
			g, err := startResourceGraph(ctx, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}}.internal(), ProfileResourceGraph.ID())
			if err != nil {
				t.Fatal(err)
			}
			engine, err := sourceAssessmentEngineForProfile(ProfileSourceLinks.ID())
			if err != nil {
				t.Fatal(err)
			}
			id := g.next("source")
			source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="inner.mxl"/><opus-link xlink:href="inner.mxl"/></opus>`
			result := assessResourceSource(ctx, strings.NewReader(source), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Context: validation.ContextFamilyAll, Account: g.options.account, DocumentID: id, Profile: ProfileSourceLinks.ID(), Engine: engine}, ByteOwner: sourceBytesPrimary})
			if result.Err != nil || result.Snapshot == nil {
				t.Fatalf("root fixture: %v", result.Err)
			}
			bundle, err := g.finishPrimary(result, g.primaryPin(validation.ResourceSourceXML, hex.EncodeToString(result.Digest[:])), id)
			wantBodies := 2
			if cancelAt != 0 {
				wantBodies = cancelAt
				if !errors.Is(err, context.Canceled) || bundle.EnumerationComplete() || bundle.Satisfied() {
					t.Fatalf("cancellation outcome lost: %v %+v", err, bundle.PackageReport())
				}
			} else if err != nil || bundle.Satisfied() || !bundle.EnumerationComplete() {
				t.Fatalf("indirect alias outcome lost: %v %+v", err, bundle.PackageReport())
			}
			if cancelAt != 1 && !qaBundleHasKnownChildFailure(bundle) {
				t.Error("retained inner failure was lost")
			}
			if len(bodies) != wantBodies {
				t.Errorf("bodies=%d want=%d", len(bodies), wantBodies)
			}
			for _, body := range bodies {
				if body.closed != 1 {
					t.Errorf("body closed=%d", body.closed)
				}
			}
			for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
				used, e := g.options.account.Usage(kind)
				if e != nil || used != 0 {
					t.Errorf("cleanup %s=%d err=%v", kind, used, e)
				}
			}
		})
	}
}

type indirectionCancelReader struct {
	reader io.Reader
	cancel context.CancelFunc
}

func (r *indirectionCancelReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.cancel()
	return n, err
}

func TestResourceGraphIndirectionEdgeAdmissionPreservesInnerFailure(t *testing.T) {
	ctx := &qaPostExportBufferContext{Context: context.Background(), stage: "ObserveEdge", ancestor: ".(*resourceGraphRun).walkLink"}
	archive := packageGraphBytes(`<opus wrong="yes"/>`, "primary.opus", "")
	calls := 0
	var body *resourceTestBody
	resolver := graphResolverFunc(func(_ context.Context, request validation.ResourceRequest) (validation.ResourceResult, error) {
		calls++
		body = &resourceTestBody{Reader: bytes.NewReader(archive)}
		return validation.ResourceResult{RequestID: request.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "inner", Representation: validation.ResourceRawMXL, Digest: fmt.Sprintf("%x", sha256.Sum256(archive))}, RetrievalURI: "https://e.test/inner.mxl", Body: body}, nil
	})
	g, err := startResourceGraph(ctx, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}}.internal(), ProfileResourceGraph.ID())
	if err != nil {
		t.Fatal(err)
	}
	ctx.account = g.options.account
	engine, err := sourceAssessmentEngineForProfile(ProfileSourceLinks.ID())
	if err != nil {
		t.Fatal(err)
	}
	id := g.next("source")
	source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="inner.mxl#missing"/><opus-link xlink:href="later.mxl"/></opus>`
	result := assessResourceSource(ctx, strings.NewReader(source), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Context: validation.ContextFamilyAll, Account: g.options.account, DocumentID: id, Profile: ProfileSourceLinks.ID(), Engine: engine}, ByteOwner: sourceBytesPrimary})
	if result.Err != nil || result.Snapshot == nil {
		t.Fatalf("root fixture: %v", result.Err)
	}
	bundle, err := g.finishPrimary(result, g.primaryPin(validation.ResourceSourceXML, hex.EncodeToString(result.Digest[:])), id)
	if !ctx.fired || !errors.Is(err, validation.ErrResourceLimit) || calls != 1 || body == nil || body.closed != 1 {
		t.Fatalf("edge refusal cause/lifetime: fired=%t calls=%d body=%+v err=%v", ctx.fired, calls, body, err)
	}
	if !qaBundleHasKnownChildFailure(bundle) || bundle.Satisfied() || bundle.EnumerationComplete() {
		t.Fatalf("inner failure lost: %+v", bundle.PackageReport())
	}
	if used, e := ctx.account.Usage(validation.BudgetBufferedBytes); e != nil || used != ctx.held {
		t.Errorf("retained=%d foreign=%d err=%v", used, ctx.held, e)
	}
	if used, e := ctx.account.Usage(validation.BudgetDepth); e != nil || used != 0 {
		t.Errorf("depth=%d err=%v", used, e)
	}
	if err := ctx.account.Release(validation.BudgetBufferedBytes, ctx.held); err != nil {
		t.Fatal(err)
	}
}
