package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func finalQANestedOpus(depth int) string {
	return strings.Repeat("<opus>", depth) + strings.Repeat("</opus>", depth)
}
func TestFinalQAMXLXMLDepthIsIndependent(t *testing.T) {
	for _, surface := range []string{"raw", "current"} {
		for _, path := range []string{"primary", "local", "external", "nested"} {
			for _, cap := range []int{3, 4} {
				t.Run(fmt.Sprintf("%s/%s/cap%d", surface, path, cap), func(t *testing.T) {
					target := finalQANestedOpus(4)
					primary := target
					var extras []assessmentZIPFixture
					var resources []MXLResource
					if path != "primary" {
						href := "target.opus"
						if path == "external" {
							href = "https://final-qa.test/target.opus"
						}
						if path == "nested" {
							href = "target.mxl"
						}
						primary = `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="` + href + `"/></opus>`
						data := []byte(target)
						if path == "nested" {
							data = packageGraphBytes(target, "nested-primary.opus", "")
						}
						if path != "external" {
							extras = []assessmentZIPFixture{{name: href, body: string(data)}}
							resources = []MXLResource{{Path: href, Data: data}}
						}
					}
					calls := 0
					var body *resourceTestBody
					opts := MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSource, Budgets: validation.Budgets{MaxDepth: 20}}, MXL: MXLOptions{MaxXMLDepth: cap}, Identity: ResourceIdentity{RetrievalURI: "https://final-qa.test/root.mxl"}, Dependencies: []ResourceDependency{ResourceDocuments}}
					opts.Resolver = graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
						calls++
						body = &resourceTestBody{Reader: strings.NewReader(target)}
						return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "depth-target", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(target)))}, RetrievalURI: "https://final-qa.test/target.opus", Body: body}, nil
					})
					var b validation.AssessmentBundle
					var e error
					if surface == "raw" {
						b, e = InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(primary, "primary.opus", "", extras...)), opts)
					} else {
						d, x := Decode(strings.NewReader(primary))
						if x != nil {
							t.Fatal(x)
						}
						opts.Graph.Profile = ProfileMXLModel
						b, e = AssessMXLPackage(context.Background(), &MXLPackage{Document: d, RootFiles: []MXLRootFile{{FullPath: "primary.opus"}}, Resources: resources}, opts)
					}
					expectedCalls := 0
					if path == "external" {
						expectedCalls = 1
					}
					if calls != expectedCalls || body != nil && body.closed != 1 {
						t.Fatalf("I/O calls=%d/%d body=%+v", calls, expectedCalls, body)
					}
					if cap == 4 {
						if e != nil || !b.EnumerationComplete() {
							t.Fatalf("exact XML depth rejected: %v %+v", e, b.PackageReport())
						}
					} else {
						if !errors.Is(e, validation.ErrResourceLimit) || !errors.Is(e, ErrXMLTooDeep) || b.EnumerationComplete() || b.Satisfied() {
							t.Fatalf("independent cap ignored: err=%v complete=%v", e, b.EnumerationComplete())
						}
						if path == "local" || path == "external" {
							for i := 0; i < b.AdoptionCount(); i++ {
								a, _ := b.AdoptionContext(i)
								if a.ArchiveEntry == "target.opus" || a.BaseURI == "https://final-qa.test/target.opus" {
									t.Fatalf("XML depth error bypassed partial-target guard %+v", a)
								}
							}
						}
					}
				})
			}
		}
	}
}
func TestFinalQAMXLXMLDepthDoesNotBoundGraphHops(t *testing.T) {
	for _, surface := range []string{"raw", "current"} {
		for _, graphCap := range []int64{4, 8} {
			t.Run(fmt.Sprintf("%s/graph%d", surface, graphCap), func(t *testing.T) {
				link := func(h string) string {
					return `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="` + h + `"/></opus>`
				}
				root := link("a.opus")
				sources := []string{link("b.opus"), link("c.opus"), link("d.opus"), `<opus/>`}
				var extras []assessmentZIPFixture
				var resources []MXLResource
				for i, s := range sources {
					p := fmt.Sprintf("%c.opus", 'a'+i)
					extras = append(extras, assessmentZIPFixture{name: p, body: s})
					resources = append(resources, MXLResource{Path: p, Data: []byte(s)})
				}
				opts := MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSource, Budgets: validation.Budgets{MaxDepth: graphCap}}, MXL: MXLOptions{MaxXMLDepth: 3}}
				var b validation.AssessmentBundle
				var e error
				if surface == "raw" {
					b, e = InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(root, "primary.opus", "", extras...)), opts)
				} else {
					d, x := Decode(strings.NewReader(root))
					if x != nil {
						t.Fatal(x)
					}
					opts.Graph.Profile = ProfileMXLModel
					b, e = AssessMXLPackage(context.Background(), &MXLPackage{Document: d, RootFiles: []MXLRootFile{{FullPath: "primary.opus"}}, Resources: resources}, opts)
				}
				if graphCap == 8 {
					if e != nil || !b.EnumerationComplete() {
						t.Fatalf("XML cap incorrectly shortened graph: %v", e)
					}
				} else if !errors.Is(e, validation.ErrResourceLimit) || errors.Is(e, ErrXMLTooDeep) || b.EnumerationComplete() {
					t.Fatal("graph hop limit lost or attributed to XML", e)
				}
			})
		}
	}
}

func TestFinalQANestedXMLDepthCacheContext(t *testing.T) {
	for _, mode := range []string{"plain-after-package", "cached-inside-package"} {
		t.Run(mode, func(t *testing.T) {
			deep := finalQANestedOpus(257)
			inner := `<opus/>`
			refs := []string{"https://final-qa.test/nested.mxl", "https://final-qa.test/deep.opus"}
			if mode == "cached-inside-package" {
				inner = `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://final-qa.test/deep.opus"/></opus>`
				refs[0], refs[1] = refs[1], refs[0]
			}
			root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink">`
			for _, h := range refs {
				root += `<opus-link xlink:href="` + h + `"/>`
			}
			root += `</opus>`
			calls := 0
			var bodies []*resourceTestBody
			r := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				calls++
				data := []byte(deep)
				rep := validation.ResourceSourceXML
				id := "shared-deep"
				if strings.Contains(q.Document.Href, "nested.mxl") {
					data = packageGraphBytes(inner, "main.opus", "")
					rep = validation.ResourceRawMXL
					id = "nested-package"
				}
				body := &resourceTestBody{Reader: bytes.NewReader(data)}
				bodies = append(bodies, body)
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: id, Representation: rep, Digest: fmt.Sprintf("%x", sha256.Sum256(data))}, RetrievalURI: q.Document.Href, Body: body}, nil
			})
			opts := finalQAOptions(r)
			opts.Graph.Budgets.MaxDepth = 1024
			b, e := InspectXMLWithResources(context.Background(), strings.NewReader(root), opts)
			wantCalls := 2
			if mode == "cached-inside-package" {
				wantCalls = 3
			}
			if calls != wantCalls {
				t.Fatalf("callbacks%d want%d err=%v", calls, wantCalls, e)
			}
			for _, body := range bodies {
				if body.closed != 1 {
					t.Fatal("close", body.closed)
				}
			}
			if mode == "plain-after-package" {
				if e != nil || !b.EnumerationComplete() || b.PackageReport().Counts.Fail != 0 {
					t.Fatalf("nested cap leaked into plain sibling: %v %+v", e, b.PackageReport())
				}
				requireIndirectionContextUnknown(t, b, 1)
				requireIndirectionIntrinsicSatisfied(t, b)
			} else {
				if !errors.Is(e, validation.ErrResourceLimit) || !errors.Is(e, ErrXMLTooDeep) || b.EnumerationComplete() {
					t.Fatalf("canonical cache bypassed contextual XML cap: %v complete%v", e, b.EnumerationComplete())
				}
				for i := 0; i < b.AdoptionCount(); i++ {
					a, _ := b.AdoptionContext(i)
					if a.BaseURI == "https://final-qa.test/deep.opus" && a.Package != "root" {
						t.Fatalf("limited cached body adopted in nested context %+v", a)
					}
				}
			}
		})
	}
}
