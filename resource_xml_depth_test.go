package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func depthModel(depth int, leaf bool) *OpusDocument {
	root := &OpusDocument{}
	next := root
	for i := 1; i < depth; i++ {
		child := &OpusDocument{}
		next.Content = OpusDocumentContents{{Opus: child}}
		next = child
	}
	if leaf {
		title := "leaf"
		next.Title = &title
	}
	return root
}
func requireXMLDepth(t *testing.T, b validation.AssessmentBundle, err error, limit int64) {
	t.Helper()
	var resource *validation.ResourceError
	if !errors.As(err, &resource) || resource.Kind != validation.BudgetDepth || resource.Limit != limit || b.Satisfied() {
		t.Fatalf("depth=%d err=%v resource=%+v bundle=%+v", limit, err, resource, b.PackageReport())
	}
	found := false
	for i := 0; i < b.ResourceCount(); i++ {
		_, r, _ := b.ResourceReport(i)
		found = found || r.Terminal != nil && r.Terminal.Category == validation.CategoryResource
	}
	if f, ok := b.Failure(); ok {
		found = found || f.Terminal.Category == validation.CategoryResource
	}
	if !found {
		t.Fatal("lost intrinsic resource-depth report")
	}
}
func TestMXLAssessmentXMLDepthRawPrimaryAndContainer(t *testing.T) {
	for _, limit := range []int{2, 3, 4} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			opts := packageGraphOptions()
			opts.MXL.MaxXMLDepth = limit
			opts.Graph.Budgets.MaxDepth = 16
			b, err := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(nestedOpusXML(4), "main.xml", "")), opts)
			if limit < 4 {
				requireXMLDepth(t, b, err, int64(limit))
			} else if err != nil {
				t.Fatal(err, b.PackageReport())
			}
		})
	}
}
func TestMXLAssessmentXMLDepthCurrentScalarLeaf(t *testing.T) {
	for _, limit := range []int{1, 2} {
		b, err := AssessMXLPackage(context.Background(), &MXLPackage{Document: depthModel(1, true)}, MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel, Budgets: validation.Budgets{MaxDepth: 16}}, MXL: MXLOptions{MaxXMLDepth: limit}})
		if limit == 1 {
			requireXMLDepth(t, b, err, 1)
		} else if err != nil {
			t.Fatal(err, b.PackageReport())
		}
	}
}
func TestMXLAssessmentXMLDepthLocalAndNested(t *testing.T) {
	deep := nestedOpusXML(4)
	for _, nested := range []bool{false, true} {
		for _, current := range []bool{false, true} {
			t.Run(fmt.Sprintf("nested=%t/current=%t", nested, current), func(t *testing.T) {
				extra := []byte(deep)
				name := "extra.xml"
				if nested {
					extra = packageGraphBytes(deep, "main.xml", "")
					name = "extra.mxl"
				}
				var b validation.AssessmentBundle
				var err error
				opts := MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSource, Budgets: validation.Budgets{MaxDepth: 32}}, MXL: MXLOptions{MaxXMLDepth: 3}}
				if current {
					opts.Graph.Profile = ProfileMXLModel
					b, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: depthModel(1, false), RootFiles: []MXLRootFile{{FullPath: "main.xml"}, {FullPath: name}}, Resources: []MXLResource{{Path: name, Data: extra}}}, opts)
				} else {
					container := `<container><rootfiles><rootfile full-path="main.xml"/><rootfile full-path="` + name + `"/></rootfiles></container>`
					data := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType}, {name: mxlContainerPath, body: container}, {name: "main.xml", body: `<opus/>`}, {name: name, body: string(extra)}}, nil, "")
					b, err = InspectMXL(context.Background(), bytes.NewReader(data), opts)
				}
				requireXMLDepth(t, b, err, 3)
			})
		}
	}
}

type xmlDepthResolver struct {
	data   map[string][]byte
	bodies []*resourceTestBody
}

func (r *xmlDepthResolver) Resolve(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
	if q.Document == nil {
		return validation.ResourceResult{}, errors.New("unexpected metadata")
	}
	data := r.data[q.Document.Href]
	representation := validation.ResourceSourceXML
	if bytes.HasPrefix(data, []byte("PK")) {
		representation = validation.ResourceRawMXL
	}
	body := &resourceTestBody{Reader: bytes.NewReader(data)}
	r.bodies = append(r.bodies, body)
	return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: q.Document.Href, Representation: representation, Digest: fmt.Sprintf("%x", sha256.Sum256(data))}, RetrievalURI: q.Document.Href, Body: body}, nil
}
func depthLink(href string) string {
	return `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="` + href + `"/></opus>`
}
func TestMXLAssessmentXMLDepthExternalAndGraphHops(t *testing.T) {
	for _, deep := range []bool{false, true} {
		t.Run(fmt.Sprint(deep), func(t *testing.T) {
			r := &xmlDepthResolver{data: map[string][]byte{}}
			for i := 1; i < 6; i++ {
				r.data[fmt.Sprintf("https://e.test/%d.xml", i)] = []byte(depthLink(fmt.Sprintf("https://e.test/%d.xml", i+1)))
			}
			r.data["https://e.test/6.xml"] = []byte(`<opus/>`)
			if deep {
				r.data["https://e.test/6.xml"] = []byte(nestedOpusXML(4))
			}
			opts := packageGraphOptions()
			opts.MXL.MaxXMLDepth = 3
			opts.Graph.Budgets.MaxDepth = 16
			opts.Dependencies = []ResourceDependency{ResourceDocuments}
			opts.Resolver = r
			b, err := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(depthLink("https://e.test/1.xml"), "main.xml", "")), opts)
			if deep {
				requireXMLDepth(t, b, err, 3)
			} else if err != nil {
				t.Fatal("XML limit incorrectly restricted graph hops", err)
			}
			if len(r.bodies) != 6 {
				t.Fatal("wrong graph traversal", len(r.bodies))
			}
			for _, body := range r.bodies {
				if body.closed != 1 {
					t.Fatal("close count", body.closed)
				}
			}
		})
	}
}
func TestMXLAssessmentXMLDepthDefaultAndPlainSibling(t *testing.T) {
	deep := nestedOpusXML(defaultMaxXMLDepth + 1)
	for _, current := range []bool{false, true} {
		var b validation.AssessmentBundle
		var err error
		if current {
			b, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: depthModel(defaultMaxXMLDepth+1, false)}, MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel, Budgets: validation.Budgets{MaxDepth: 512}}})
		} else {
			opts := packageGraphOptions()
			opts.Graph.Budgets.MaxDepth = 512
			b, err = InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(deep, "main.xml", "")), opts)
		}
		requireXMLDepth(t, b, err, defaultMaxXMLDepth)
	}
	// A nested archive in a plain XML run gets the MXL default; it must not narrow
	// the later plain XML sibling or mutate the shared graph/account depth limit.
	r := &xmlDepthResolver{data: map[string][]byte{"https://e.test/a.mxl": packageGraphBytes(`<opus/>`, "main.xml", ""), "https://e.test/deep.xml": []byte(deep)}}
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://e.test/a.mxl"/><opus-link xlink:href="https://e.test/deep.xml"/></opus>`
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(root), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxDepth: 512}}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: r})
	if err != nil || !b.EnumerationComplete() || b.PackageReport().Counts.Fail != 0 {
		t.Fatal("MXL cap leaked into plain sibling", err, b.PackageReport())
	}
	requireIndirectionContextUnknown(t, b, 1)
	requireIndirectionIntrinsicSatisfied(t, b)
	r = &xmlDepthResolver{data: map[string][]byte{"https://e.test/a.mxl": packageGraphBytes(deep, "main.xml", "")}}
	b, err = InspectXMLWithResources(context.Background(), strings.NewReader(depthLink("https://e.test/a.mxl")), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxDepth: 512}}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: r})
	requireXMLDepth(t, b, err, defaultMaxXMLDepth)
}

func TestMXLAssessmentXMLDepthCountsScalarLeavesInBothTargets(t *testing.T) {
	source := strings.Repeat(`<opus>`, 3) + `<title>leaf</title>` + strings.Repeat(`</opus>`, 3)
	for _, current := range []bool{false, true} {
		for _, limit := range []int{3, 4} {
			var b validation.AssessmentBundle
			var err error
			opts := MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSource, Budgets: validation.Budgets{MaxDepth: 16}}, MXL: MXLOptions{MaxXMLDepth: limit}}
			if current {
				opts.Graph.Profile = ProfileMXLModel
				b, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: depthModel(3, true)}, opts)
			} else {
				b, err = InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(source, "main.xml", "")), opts)
			}
			if limit == 3 {
				requireXMLDepth(t, b, err, 3)
				if !errors.Is(err, ErrXMLTooDeep) {
					t.Fatal("private cap lost XML error identity", err)
				}
			} else if err != nil {
				t.Fatal(err, b.PackageReport())
			}
		}
	}
}
func TestMXLAssessmentGraphDepthRemainsIndependentlyStricter(t *testing.T) {
	for _, current := range []bool{false, true} {
		opts := MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSource, Budgets: validation.Budgets{MaxDepth: 3}}, MXL: MXLOptions{MaxXMLDepth: 4}}
		var b validation.AssessmentBundle
		var err error
		if current {
			opts.Graph.Profile = ProfileMXLModel
			b, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: depthModel(4, false)}, opts)
		} else {
			b, err = InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(nestedOpusXML(4), "main.xml", "")), opts)
		}
		requireXMLDepth(t, b, err, 3)
		if errors.Is(err, ErrXMLTooDeep) {
			t.Fatal("shared account limit mislabeled XML-specific", err)
		}
	}
}
func TestIntrinsicXMLDepthConfigurationDoesNotAlterAccount(t *testing.T) {
	for _, limit := range []int{-1, maximumXMLDepth + 1, 0, maximumXMLDepth} {
		a, err := validation.NewBudgetAccount(validation.Budgets{MaxDepth: 12})
		if err != nil {
			t.Fatal(err)
		}
		cfg := adapterSourceConfig(t, a, "depth", "", ProfileSourceLinks.ID(), sourceBytesPrimary)
		cfg.XMLDepth = limit
		input := bytes.NewBufferString(`<opus/>`)
		run, err := newSourceAssessmentRunWithConfig(context.Background(), input, DecodeOptions{}, &cfg, false)
		if limit < 0 || limit > maximumXMLDepth {
			if err == nil || input.Len() != len(`<opus/>`) {
				t.Fatal("invalid private depth read input", err, input.Len())
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if run.account != a || a.Limits().MaxDepth != 12 {
			t.Fatal("private XML cap changed shared account")
		}
		inputErr := run.drain()
		_, err = run.finish(inputErr)
		if err != nil {
			t.Fatal(err)
		}
		if used, _ := a.Usage(validation.BudgetBufferedBytes); used != 0 {
			t.Fatal("retained source state", used)
		}
	}
}

func TestMXLAssessmentXMLDepthCachedPlainResourceCannotBypassPackageCap(t *testing.T) {
	deep := nestedOpusXML(defaultMaxXMLDepth + 1)
	r := &xmlDepthResolver{data: map[string][]byte{
		"https://e.test/deep.xml": []byte(deep),
		"https://e.test/a.mxl":    packageGraphBytes(depthLink("https://e.test/deep.xml"), "main.xml", ""),
	}}
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://e.test/deep.xml"/><opus-link xlink:href="https://e.test/a.mxl"/></opus>`
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(root), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxDepth: 512}}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: r})
	var depth *validation.ResourceError
	if !errors.Is(err, ErrXMLTooDeep) || !errors.As(err, &depth) || depth.Kind != validation.BudgetDepth || depth.Limit != defaultMaxXMLDepth || b.Satisfied() {
		t.Fatalf("cached plain resource bypassed MXL cap: %v %+v", err, b.PackageReport())
	}
	count := 0
	var deepID validation.ResourceAssessmentID
	for i := 0; i < b.AdoptionCount(); i++ {
		c, _ := b.AdoptionContext(i)
		if c.BaseURI == "https://e.test/deep.xml" {
			count++
			deepID = c.Assessment
		}
	}
	if count != 1 {
		t.Fatal("context refusal admitted cached deep target", count)
	}
	count = 0
	for i := 0; i < b.ResourceCount(); i++ {
		id, report, _ := b.ResourceReport(i)
		if id == deepID {
			count++
			if report.Terminal != nil || !report.InputComplete {
				t.Fatal("context refusal changed original intrinsic report", report)
			}
		}
	}
	if count != 1 || b.ResourceCount() != 4 {
		t.Fatal("canonical XML was reparsed", count, b.ResourceCount())
	}
	for _, body := range r.bodies {
		if body.closed != 1 {
			t.Fatal("body close", body.closed)
		}
	}
}

func TestIntrinsicXMLDepthWitnessAndInterruptedCache(t *testing.T) {
	ctx := context.Background()
	for _, model := range []bool{false, true} {
		for _, limit := range []int{3, 4} {
			a, err := validation.NewBudgetAccount(validation.Budgets{MaxDepth: 16})
			if err != nil {
				t.Fatal(err)
			}
			var result resourceIntrinsicResult
			if model {
				engine, _ := modelAssessmentEngineForProfile(ProfileModelLinks.ID())
				result = assessResourceModel(ctx, depthModel(3, true), assessmentRunConfig{Account: a, DocumentID: "depth", Profile: ProfileModelLinks.ID(), Engine: engine, XMLDepth: limit})
			} else {
				cfg := adapterSourceConfig(t, a, "depth", "", ProfileSourceLinks.ID(), sourceBytesPrimary)
				cfg.XMLDepth = limit
				result = assessResourceSource(ctx, strings.NewReader(`<opus><opus><opus><title>leaf</title></opus></opus></opus>`), cfg)
			}
			if limit == 4 && (result.Err != nil || result.XMLDepth != 4) {
				t.Fatal("complete peak witness", result.XMLDepth, result.Err)
			}
			if limit == 3 && (!errors.Is(result.Err, ErrXMLTooDeep) || result.XMLDepth != 3) {
				t.Fatal("interrupted peak witness", result.XMLDepth, result.Err)
			}
			if result.Snapshot != nil {
				if err := result.Snapshot.Close(); err != nil {
					t.Fatal(err)
				}
			}
			g := &resourceGraphRun{ctx: ctx, options: resourceRunOptions{account: a}, cache: map[graphResourceKey]*graphResource{}, pins: map[graphCanonicalKey]validation.ResourcePin{}}
			pin := validation.ResourcePin{CanonicalID: "depth", Representation: validation.ResourceSourceXML, Digest: strings.Repeat("a", 64)}
			_, err = g.cacheResource(pin, ProfileSourceLinks.ID(), "intrinsic", result.XMLDepth, result.Err == nil)
			if err != nil {
				t.Fatal(err)
			}
			if (len(g.cache) == 1) != (limit == 4) {
				t.Fatal("interrupted intrinsic became reusable", len(g.cache))
			}
			g.close()
			for _, kind := range []validation.BudgetKind{validation.BudgetDepth, validation.BudgetBufferedBytes} {
				if used, _ := a.Usage(kind); used != 0 {
					t.Fatal("leaked live gauge", kind, used)
				}
			}
		}
	}
}
