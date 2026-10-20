package musicxml

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"net/url"
	"strings"
	"testing"
)

func TestResourceGraphPrimaryReports(t *testing.T) {
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(`<opus/>`), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}})
	if err != nil || !b.Satisfied() || b.ResourceCount() != 1 || b.AdoptionCount() != 1 || !b.EnumerationComplete() {
		t.Fatalf("err=%v package=%+v resources=%d adoptions=%d", err, b.PackageReport(), b.ResourceCount(), b.AdoptionCount())
	}
	_, r, ok := b.ResourceReport(0)
	if !ok || r.Target != validation.TargetSource || r.Profile.ID != ProfileSourceLinks.ID() {
		t.Fatal(r)
	}
	b, err = InspectXMLWithResources(context.Background(), strings.NewReader(`<opus wrong="yes"/>`), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}})
	if err != nil || b.Satisfied() {
		t.Fatal(err, b.PackageReport())
	}
	_, r, _ = b.ResourceReport(0)
	if r.Conformance != validation.ConformanceViolated {
		t.Fatal(r)
	}
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	b, err = AssessModelWithResources(context.Background(), doc, LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}})
	if err != nil || b.ResourceCount() != 1 {
		t.Fatal(err, b.PackageReport())
	}
	_, r, _ = b.ResourceReport(0)
	if r.Target != validation.TargetModel || r.Profile.ID != ProfileModelLinks.ID() {
		t.Fatal(r)
	}
}
func TestResourceGraphConfigurationNoInput(t *testing.T) {
	_, err := InspectXMLWithResources(context.Background(), nil, LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}})
	if !errors.Is(err, ErrNilReader) {
		t.Fatal(err)
	}
	_, err = InspectXMLWithResources(context.Background(), strings.NewReader(`<opus/>`), LinkedAssessmentOptions{Graph: GraphOptions{Profile: "test/unknown-profile"}})
	if !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatal(err)
	}
}

type graphFixtureResolver struct {
	calls     int
	bodies    []*resourceTestBody
	resources map[string]string
	pins      map[string]string
}

func (r *graphFixtureResolver) Resolve(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
	r.calls++
	if q.Document == nil {
		return validation.ResourceResult{}, errors.New("unexpected metadata request")
	}
	u, err := url.Parse(q.Document.BaseURI)
	if err != nil {
		return validation.ResourceResult{}, err
	}
	ref, err := url.Parse(q.Document.Href)
	if err != nil {
		return validation.ResourceResult{}, err
	}
	target := u.ResolveReference(ref).String()
	data, ok := r.resources[target]
	if !ok {
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "fixture-unavailable"}, nil
	}
	body := &resourceTestBody{Reader: strings.NewReader(data)}
	r.bodies = append(r.bodies, body)
	return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: r.pins[target], Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(data)))}, RetrievalURI: target, Body: body}, nil
}
func TestResourceGraphFiniteCyclesAndVerifiedParseReuse(t *testing.T) {
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="child.opus"/><opus-link xlink:href="child.opus"/></opus>`
	child := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="root.opus"/></opus>`
	resolver := &graphFixtureResolver{resources: map[string]string{"https://e.test/root.opus": root, "https://e.test/child.opus": child}, pins: map[string]string{"https://e.test/root.opus": "root", "https://e.test/child.opus": "child"}}
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(root), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{CanonicalID: "root", RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
	if err != nil || !b.Satisfied() || b.ResourceCount() != 2 || b.AdoptionCount() != 4 || resolver.calls != 3 {
		t.Fatalf("err %v resources %d adoptions %d calls %d report %+v", err, b.ResourceCount(), b.AdoptionCount(), resolver.calls, b.PackageReport())
	}
	for _, body := range resolver.bodies {
		if body.closed != 1 {
			t.Fatal(body.closed)
		}
	}
}
func TestResourceGraphNoResolverForUnselectedOrInvalidHref(t *testing.T) {
	for _, tc := range []struct {
		input    string
		selected bool
	}{
		{`<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="child.musicxml"/></opus>`, false},
		{`<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="child.musicxml" xlink:actuate="onLoad" xlink:show="embed"/></opus>`, false},
		{`<opus><score/></opus>`, true},
		{`<opus xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="https://e.test/schema.xsd"/>`, true},
	} {
		resolver := &graphFixtureResolver{}
		options := LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Resolver: resolver, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}}
		if tc.selected {
			options.Dependencies = []ResourceDependency{ResourceDocuments}
		}
		_, err := InspectXMLWithResources(context.Background(), strings.NewReader(tc.input), options)
		if err != nil || resolver.calls != 0 {
			t.Fatal(err, resolver.calls)
		}
	}
}

func TestResourceGraphRejectsUnrequestedRevisionChurn(t *testing.T) {
	source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="a.opus"/><opus-link xlink:href="b.opus"/></opus>`
	calls := 0
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		calls++
		body := `<opus/>`
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "same-resource", Revision: fmt.Sprint(calls), Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(body)))}, Body: &resourceTestBody{Reader: strings.NewReader(body)}, RetrievalURI: "https://e.test/child.opus"}, nil
	})
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
	if !errors.Is(err, validation.ErrInvalidConfiguration) || b.Satisfied() {
		t.Fatal(err, b.ResourceCount(), b.Satisfied())
	}
}
