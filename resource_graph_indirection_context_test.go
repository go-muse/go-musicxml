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

func indirectionContextSource(kind, href string) string {
	switch kind {
	case "opus-score":
		return `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="` + href + `"/></opus>`
	case "work-opus":
		return strings.Replace(resourceScore(""), `<part-list>`, `<work><opus xlink:href="`+href+`"/></work><part-list>`, 1)
	case "part-link":
		return strings.Replace(resourceScore(""), `<part-name>`, `<part-link xlink:href="`+href+`"/><part-name>`, 1)
	default:
		panic("unknown indirection fixture link kind")
	}
}

func indirectionContextLinks(b validation.AssessmentBundle) []validation.Finding {
	var out []validation.Finding
	for _, f := range b.PackageReport().Findings {
		if f.Rule.ID == "MX40-PACKAGE-link-target" {
			out = append(out, f)
		}
	}
	return out
}

func requireIndirectionContextUnknown(t *testing.T, b validation.AssessmentBundle, count int) {
	t.Helper()
	findings := indirectionContextLinks(b)
	if len(findings) != count {
		t.Errorf("link findings=%d, want %d unresolved archive links: %+v", len(findings), count, findings)
	}
	for _, f := range findings {
		if f.Outcome != validation.OutcomeUnknown || f.Reason != "nested-link-indirection-unresolved" {
			t.Errorf("archive primary leaked into outer link interpretation: %+v", f)
		}
	}
	if b.Satisfied() || !b.EnumerationComplete() {
		t.Errorf("uncertainty/completeness lost: satisfied=%t enumeration=%t", b.Satisfied(), b.EnumerationComplete())
	}
}

func TestResourceGraphIndirectionContextLinkFamilies(t *testing.T) {
	for _, family := range []string{"opus-score", "work-opus", "part-link"} {
		for _, archive := range []bool{false, true} {
			for _, matching := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/archive-%t/matching-%t", family, archive, matching), func(t *testing.T) {
					target := resourceScore("")
					if (family == "work-opus") == matching {
						target = `<opus/>`
					}
					data := []byte(target)
					if archive {
						data = packageGraphBytes(target, "primary.xml", "")
					}
					// The same neutral extension prevents the filename from standing
					// in for observed representation evidence.
					root := indirectionContextSource(family, "target.dat")
					b, err := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(root, "main.xml", "", assessmentZIPFixture{name: "target.dat", body: string(data)})), packageGraphOptions())
					if err != nil {
						t.Fatal(err)
					}
					if archive {
						requireIndirectionContextUnknown(t, b, 1)
						if b.ResourceCount() != 4 || b.AdoptionCount() != 5 {
							t.Errorf("independent archive reports/adoptions lost: resources=%d adoptions=%d", b.ResourceCount(), b.AdoptionCount())
						}
						return
					}
					findings := indirectionContextLinks(b)
					if matching && len(findings) != 0 || !matching && (len(findings) != 1 || findings[0].Outcome != validation.OutcomeFail || findings[0].Reason != "wrong-linked-document-kind") {
						t.Errorf("ordinary XML archive member lost direct grading: %+v", findings)
					}
					// The role provider is intentionally unselected; its independent
					// uncertainty does not change the target-kind predicate.
					if !b.EnumerationComplete() || b.ResourceCount() != 3 || b.AdoptionCount() != 3 {
						t.Errorf("direct XML traversal changed: complete=%t resources=%d adoptions=%d", b.EnumerationComplete(), b.ResourceCount(), b.AdoptionCount())
					}
				})
			}
		}
	}
}

func TestResourceGraphIndirectionContextFragmentsKeepDirectSemantics(t *testing.T) {
	for _, fragment := range []string{"P1", "missing", "xpointer(/)"} {
		t.Run(fragment, func(t *testing.T) {
			inner := packageGraphBytes(resourceScore(""), "primary.xml", "")
			root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="inner.mxl#` + fragment + `"/><score xlink:href="direct.xml#P1"/><score xlink:href="direct.xml#missing"/></opus>`
			b, err := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(root, "main.opus", "", assessmentZIPFixture{name: "inner.mxl", body: string(inner)}, assessmentZIPFixture{name: "direct.xml", body: resourceScore("")})), packageGraphOptions())
			if err != nil {
				t.Fatal(err)
			}
			unknown, missing := 0, 0
			findings := indirectionContextLinks(b)
			for _, f := range findings {
				switch {
				case f.Outcome == validation.OutcomeUnknown && f.Reason == "nested-link-indirection-unresolved":
					unknown++
					if len(f.Related) != 1 || f.Related[0].Entry != "main.opus" || f.Related[0].Path != "/opus/score" {
						t.Errorf("outer uncertainty lost its own link provenance: %+v", f.Related)
					}
				case f.Outcome == validation.OutcomeFail && f.Reason == "fragment-id-absent":
					missing++
					if len(f.Related) != 1 || f.Related[0].Entry != "main.opus" || f.Related[0].Path != "/opus/score[3]" {
						t.Errorf("direct missing fragment lost sibling provenance: %+v", f.Related)
					}
				default:
					t.Errorf("unexpected fragment interpretation: %+v", f)
				}
			}
			if unknown != 1 || missing != 1 || len(findings) != 2 || b.Satisfied() || !b.EnumerationComplete() {
				t.Errorf("indirect fragment used inner IDs or changed direct sibling: unknown=%d missing=%d complete=%t findings=%+v", unknown, missing, b.EnumerationComplete(), findings)
			}
			// Public report copies cannot mutate the retained outer uncertainty.
			copy := b.PackageReport()
			for i := range copy.Findings {
				copy.Findings[i].Reason = "caller-edit"
				copy.Findings[i].Location.Path = "caller-edit"
				for j := range copy.Findings[i].Related {
					copy.Findings[i].Related[j].Path = "caller-edit"
				}
			}
			for _, f := range b.PackageReport().Findings {
				if f.Reason == "caller-edit" || f.Location.Path == "caller-edit" {
					t.Error("report retained caller-owned provenance")
				}
				for _, related := range f.Related {
					if related.Path == "caller-edit" {
						t.Error("report retained caller-owned related provenance")
					}
				}
			}
		})
	}
}

func TestResourceGraphIndirectionContextInnerFailuresSurvive(t *testing.T) {
	innerRoot := `<opus xmlns:xlink="http://www.w3.org/1999/xlink" wrong="yes"><opus-link xlink:href="wrong-kind.xml"/><opus-link xlink:href="absent.xml"/></opus>`
	inner := packageGraphBytes(innerRoot, "primary.opus", "", assessmentZIPFixture{name: "wrong-kind.xml", body: resourceScore("")})
	root := depthLink("inner.mxl")
	b, err := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(root, "main.opus", "", assessmentZIPFixture{name: "inner.mxl", body: string(inner)})), packageGraphOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := map[validation.Code]validation.Outcome{"nested-link-indirection-unresolved": validation.OutcomeUnknown, "wrong-linked-document-kind": validation.OutcomeFail, "linked-resource-authoritatively-absent": validation.OutcomeFail}
	seen := map[validation.Code]int{}
	for _, f := range indirectionContextLinks(b) {
		if outcome, ok := want[f.Reason]; !ok || outcome != f.Outcome {
			t.Errorf("unexpected direct or indirect result: %+v", f)
		}
		seen[f.Reason]++
	}
	for reason := range want {
		if seen[reason] != 1 {
			t.Errorf("independent finding %s count=%d, want 1", reason, seen[reason])
		}
	}
	intrinsicFailures := 0
	for i := 0; i < b.ResourceCount(); i++ {
		_, r, _ := b.ResourceReport(i)
		if r.Counts.Fail > 0 {
			intrinsicFailures++
		}
	}
	if intrinsicFailures != 1 || b.ResourceCount() != 5 || b.Satisfied() || !b.EnumerationComplete() {
		t.Errorf("inner evidence lost: failingReports=%d resources=%d satisfied=%t enumeration=%t", intrinsicFailures, b.ResourceCount(), b.Satisfied(), b.EnumerationComplete())
	}
}

func indirectionContextInstrumentScore(part, instrument, href, body string) string {
	return `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink"><part-list><score-part id="` + part + `"><part-link xlink:href="` + href + `"><instrument-link id="` + instrument + `"/></part-link><part-name>P</part-name><score-instrument id="` + instrument + `"><instrument-name>P</instrument-name></score-instrument></score-part></part-list><part id="` + part + `"><measure number="1">` + body + `</measure></part></score-partwise>`
}

func TestResourceGraphIndirectionContextAssociationIsolation(t *testing.T) {
	innerXML := indirectionContextInstrumentScore("P2", "I2", "direct.xml", `<direction><direction-type><image source="inner.png" type="image/png"/></direction-type></direction>`)
	inner := packageGraphBytes(innerXML, "primary.xml", "", assessmentZIPFixture{name: "direct.xml", body: resourceScore("")})
	outer := indirectionContextInstrumentScore("P1", "I1", "inner.mxl", "")
	documentCalls, associationCalls, imageCalls := 0, 0, 0
	var bodies []*resourceTestBody
	var innerAssociation validation.AssociationRequest
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		switch {
		case q.Document != nil:
			documentCalls++
			if q.Document.Href != "inner.mxl" {
				t.Errorf("archive-local direct edge escaped to resolver: %+v", q.Document)
			}
			body := &resourceTestBody{Reader: bytes.NewReader(inner)}
			bodies = append(bodies, body)
			return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "inner", Representation: validation.ResourceRawMXL, Digest: fmt.Sprintf("%x", sha256.Sum256(inner))}, RetrievalURI: "https://e.test/inner.mxl", Body: body}, nil
		case q.Image != nil:
			imageCalls++
			if q.Image.Href != "inner.png" || q.Image.ArchiveEntry != "primary.xml" || q.Image.BaseURI != "https://e.test/inner.mxl" {
				t.Errorf("inner image lost independent context: %+v", q.Image)
			}
			return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "fixture-image-unavailable"}, nil
		case q.Association != nil:
			associationCalls++
			if q.Association.PartID != "P2" || q.Association.InstrumentID != "I2" {
				t.Errorf("outer indirect part-link invoked association provider: %+v", q.Association)
			} else {
				innerAssociation = *q.Association
			}
			result := validation.AssociationResult{Request: *q.Association, Membership: packageKnown(true), Authority: validation.EvidenceAuthority{ID: "indirection-fixture", ExactRelationComplete: true, Provenance: []validation.SourceRef{{IDs: []string{"inner-direct-membership"}, URL: "https://e.test/association", Locator: "inner direct relation"}}}, TargetPartID: validation.Evidence[string]{State: validation.EvidenceAbsent}, TargetInstrumentID: validation.Evidence[string]{State: validation.EvidenceAbsent}}
			return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Association: &result}, nil
		default:
			t.Fatalf("unexpected callback: %+v", q)
			return validation.ResourceResult{}, nil
		}
	})
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(outer), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{CanonicalID: "outer", RetrievalURI: "https://e.test/outer.xml"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments, ResourceImages, ResourceAssociations}})
	if err != nil {
		t.Fatal(err)
	}
	requireIndirectionContextUnknown(t, b, 1)
	outerAssociations := 0
	for _, finding := range b.PackageReport().Findings {
		if finding.Rule.ID == "MX40-PACKAGE-instrument-association" {
			outerAssociations++
			if finding.Outcome != validation.OutcomeUnknown || finding.Reason != "nested-link-indirection-unresolved" {
				t.Errorf("outer association lost indirection cause: %+v", finding)
			}
		}
	}
	if outerAssociations != 1 {
		t.Errorf("outer association findings=%d want=1", outerAssociations)
	}
	if documentCalls != 1 || associationCalls != 1 || imageCalls != 1 || b.ImageAdviceCount() != 1 || b.ResourceCount() != 4 {
		t.Errorf("independent inner work changed: documents=%d associations=%d images=%d advice=%d resources=%d", documentCalls, associationCalls, imageCalls, b.ImageAdviceCount(), b.ResourceCount())
	}
	for _, body := range bodies {
		if body.closed != 1 {
			t.Errorf("body closes=%d, want 1", body.closed)
		}
	}
	advice, ok := b.ImageAdviceAt(0)
	if !ok || advice.Request.Subject.DocumentID != innerAssociation.SourcePart.DocumentID || advice.Availability.Reason != "fixture-image-unavailable" {
		t.Errorf("inner advice/association provenance changed: advice=%+v association=%+v", advice, innerAssociation)
	}
	ownerFound := false
	for i := 0; i < b.AdoptionCount(); i++ {
		a, _ := b.AdoptionContext(i)
		if a.Adoption == advice.Source {
			ownerFound = a.DocumentID == innerAssociation.SourcePart.DocumentID && a.ArchiveEntry == "primary.xml" && a.Package != "root" && a.ExpansionOwner == a.Adoption
		}
	}
	if !ownerFound {
		t.Error("inner metadata lost its actual expansion owner")
	}
	advice.Request.ArchiveEntry = "caller-edit"
	retained, _ := b.ImageAdviceAt(0)
	if retained.Request.ArchiveEntry != "primary.xml" {
		t.Error("image advice retains caller-owned context")
	}
}

func TestResourceGraphIndirectionContextRootfilePrimaryAndDepthReuse(t *testing.T) {
	inner := packageGraphBytes(`<opus/>`, "primary.opus", "")
	container := `<container><rootfiles><rootfile full-path="inner.mxl" media-type="application/vnd.recordare.musicxml"/><rootfile full-path="sibling.opus"/></rootfiles></container>`
	sibling := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="inner.mxl"/><opus-link xlink:href="middle.opus"/></opus>`
	outer := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType}, {name: mxlContainerPath, body: container}, {name: "inner.mxl", body: string(inner)}, {name: "sibling.opus", body: sibling}, {name: "middle.opus", body: depthLink("at-limit.opus")}, {name: "at-limit.opus", body: depthLink("inner.mxl")}}, nil, "")
	for _, limit := range []int64{3, 8} {
		t.Run(fmt.Sprintf("depth-%d", limit), func(t *testing.T) {
			opts := packageGraphOptions()
			opts.Graph.Budgets.MaxDepth = limit
			opts.Graph.Budgets.MaxLinkedDocuments = 8
			b, err := InspectMXL(context.Background(), bytes.NewReader(outer), opts)
			if err != nil {
				t.Fatal(err)
			}
			requireIndirectionContextUnknown(t, b, 2)
			rootGate := 0
			for _, f := range b.PackageReport().Findings {
				if f.Reason == "nested-rootfile-indirection-unresolved" && f.Outcome == validation.OutcomeUnknown {
					rootGate++
				}
			}
			if rootGate != 2 || b.ResourceCount() != 6 || b.AdoptionCount() != 8 {
				t.Errorf("primary or exact-depth reuse lost evidence: rootGate=%d resources=%d adoptions=%d", rootGate, b.ResourceCount(), b.AdoptionCount())
			}
			var primary []validation.ResourceAdoptionContext
			for i := 0; i < b.AdoptionCount(); i++ {
				a, _ := b.AdoptionContext(i)
				if a.ArchiveEntry == "primary.opus" {
					primary = append(primary, a)
				}
			}
			if len(primary) != 3 {
				t.Fatalf("primary occurrence count=%d, want independent owner plus two aliases", len(primary))
			}
			for _, a := range primary[1:] {
				if a.Assessment != primary[0].Assessment || a.Package != primary[0].Package || a.ExpansionOwner != primary[0].Adoption || a.Adoption == primary[0].Adoption || a.ExpansionComplete.State != validation.EvidenceKnown || !a.ExpansionComplete.Value {
					t.Errorf("outer uncertainty discarded inner exact-context identity: alias=%+v owner=%+v", a, primary[0])
				}
			}
		})
	}
	t.Run("one-under-adoption-cap", func(t *testing.T) {
		opts := packageGraphOptions()
		opts.Graph.Budgets.MaxDepth = 3
		opts.Graph.Budgets.MaxLinkedDocuments = 7
		b, err := InspectMXL(context.Background(), bytes.NewReader(outer), opts)
		var limit *validation.ResourceError
		if !errors.As(err, &limit) || limit.Kind != validation.BudgetLinkedDocuments || limit.Limit != 7 || b.AdoptionCount() != 7 || b.Satisfied() || b.EnumerationComplete() {
			t.Errorf("indirection lost alias admission charging: err=%v adoptions=%d satisfied=%t complete=%t", err, b.AdoptionCount(), b.Satisfied(), b.EnumerationComplete())
		}
	})
}

func TestResourceGraphIndirectionContextExternalAncestorAlias(t *testing.T) {
	primary := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://e.test/b.mxl"/><opus-link xlink:href="child.opus"/></opus>`
	data := packageGraphBytes(primary, "main.opus", "", assessmentZIPFixture{name: "child.opus", body: depthLink("main.opus")})
	for _, limit := range []int64{3, 8} {
		t.Run(fmt.Sprintf("depth-%d", limit), func(t *testing.T) {
			calls := 0
			var bodies []*resourceTestBody
			resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				calls++
				if q.Document == nil || q.Document.Href != "https://e.test/b.mxl" {
					t.Fatalf("unexpected request: %+v", q)
				}
				body := &resourceTestBody{Reader: bytes.NewReader(data)}
				bodies = append(bodies, body)
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "root-archive", Representation: validation.ResourceRawMXL, Digest: fmt.Sprintf("%x", sha256.Sum256(data))}, RetrievalURI: "https://e.test/b.mxl", Body: body}, nil
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
			requireIndirectionContextUnknown(t, b, 2)
			if calls != 2 || b.AdoptionCount() != 8 || b.ResourceCount() != 3 {
				t.Errorf("ancestor reuse changed: calls=%d adoptions=%d resources=%d", calls, b.AdoptionCount(), b.ResourceCount())
			}
			for _, body := range bodies {
				if body.closed != 1 {
					t.Errorf("alias closes=%d", body.closed)
				}
			}
			last, _ := b.AdoptionContext(b.AdoptionCount() - 1)
			owner, _ := b.AdoptionContext(2)
			if last.ArchiveEntry != "main.opus" || last.BaseURI != "https://e.test/b.mxl" || last.ExpansionOwner != owner.Adoption || last.Adoption == owner.Adoption || last.Assessment != owner.Assessment || last.Package != owner.Package {
				t.Errorf("indirection changed exact owner or base: last=%+v owner=%+v", last, owner)
			}
		})
	}
}

func requireIndirectionIntrinsicSatisfied(t *testing.T, b validation.AssessmentBundle) {
	t.Helper()
	if failure, ok := b.Failure(); ok {
		t.Errorf("unexpected failed resource: %+v", failure)
	}
	if b.PackageReport().Counts.Fail != 0 {
		t.Errorf("unexpected package failure: %+v", b.PackageReport())
	}
	for i := 0; i < b.ResourceCount(); i++ {
		id, report, ok := b.ResourceReport(i)
		if !ok || !report.Satisfied() {
			t.Errorf("intrinsic %s not satisfied: %+v", id, report)
		}
	}
}
