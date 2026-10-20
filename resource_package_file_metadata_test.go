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

func TestPackageFileMetadataBeforeFailedParse(t *testing.T) {
	for _, body := range []string{`<sounds/>`, `<opus><broken`} {
		b, err := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(body, "wrong.bad", "application/x-wrong")), packageGraphOptions())
		if err == nil {
			t.Fatal("fixture must fail parsing")
		}
		if b.FileMetadataCount() != 1 {
			t.Fatalf("advice lost before parse: %v count%d", err, b.FileMetadataCount())
		}
		m, _ := b.FileMetadataAt(0)
		if m.Name.Value != "wrong.bad" || m.MediaType.Value != "application/x-wrong" || m.Entry == 0 || m.ArchiveEntry != "wrong.bad" || m.Pin.Digest == "" {
			t.Fatalf("metadata %+v", m)
		}
		if b.EnumerationComplete() || b.Satisfied() {
			t.Fatal("metadata fabricated conformance")
		}
	}
}
func TestPackageFileMetadataExternalAliasesAndEscapedName(t *testing.T) {
	data := []byte(`<opus/>`)
	source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://e.test/a%2Fb.musicxml"/><opus-link xlink:href="https://e.test/second.bad"/></opus>`
	opts := LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		media := musicXMLMIMEType
		if strings.Contains(q.Document.Href, "second") {
			media = "application/pdf"
		}
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "same", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256(data))}, RetrievalURI: q.Document.Href, MediaType: media, Body: &resourceTestBody{Reader: bytes.NewReader(data)}}, nil
	})}
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), opts)
	if err != nil || !b.Satisfied() || b.FileMetadataCount() != 2 {
		t.Fatal(err, b.FileMetadataCount(), b.PackageReport().Findings)
	}
	first, _ := b.FileMetadataAt(0)
	second, _ := b.FileMetadataAt(1)
	if first.Name.Value != "a%2Fb.musicxml" || second.Name.Value != "second.bad" || first.BaseURI == second.BaseURI || first.Template == second.Template || first.Pin != second.Pin || first.MediaType.Value == second.MediaType.Value {
		t.Fatalf("alias metadata collapsed: %+v %+v", first, second)
	}
	first.Location.Path = "mutated"
	first.Name.Value = "mutated"
	again, _ := b.FileMetadataAt(0)
	if again.Location.Path == "mutated" || again.Name.Value == "mutated" {
		t.Fatal("getter aliases")
	}
}
func TestPackageFileMetadataNestedSameNamesAndCurrent(t *testing.T) {
	inner := packageGraphBytes(`<opus/>`, "same.bad", "")
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="a.mxl"/><opus-link xlink:href="b.mxl"/></opus>`
	b, err := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(root, "same.bad", "", assessmentZIPFixture{name: "a.mxl", body: string(inner)}, assessmentZIPFixture{name: "b.mxl", body: string(inner)})), packageGraphOptions())
	if err != nil {
		t.Fatal(err)
	}
	scopes := map[validation.PackageID]bool{}
	for i := 0; i < b.FileMetadataCount(); i++ {
		m, _ := b.FileMetadataAt(i)
		if m.Name.Value == "same.bad" {
			scopes[m.Package] = true
		}
	}
	if len(scopes) != 3 {
		t.Fatalf("same-named members collapsed: %v", scopes)
	}
	d, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	opts := packageGraphOptions()
	opts.Graph.Profile = ProfileMXLModel
	b, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: d}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if b.FileMetadataCount() != 1 {
		t.Fatal(b.FileMetadataCount())
	}
	m, _ := b.FileMetadataAt(0)
	if m.Name.Value != "score.musicxml" || m.Pin.Representation != validation.ResourceCurrentModel || m.Entry != 0 || m.Location.Path != "RootFiles[0]" {
		t.Fatalf("current provenance %+v", m)
	}
}
func TestPackageFileMetadataOptionalAndInvalidMIME(t *testing.T) {
	g := packageBodyRun(t, context.Background(), MXLOptions{})
	defer g.close()
	for _, u := range []string{"urn:opaque", "custom://host/path.musicxml"} {
		n, err := g.fileNameFromRetrieval(u)
		if err != nil || n.State != validation.EvidenceUnknown {
			t.Fatal(n, err)
		}
	}
	absent, err := g.externalFileMedia("")
	if err != nil || absent.State != validation.EvidenceAbsent {
		t.Fatal(absent, err)
	}
	invalid, err := g.externalFileMedia("not a MIME")
	if err != nil || invalid.State != validation.EvidenceUnknown {
		t.Fatal(invalid, err)
	}
}
func TestPackageFileMetadataSurvivesResourceAdmissionRefusal(t *testing.T) {
	wire := packageGraphBytes(`<opus/>`, "primary.bad", "")
	for cap := int64(1); cap <= 500; cap++ {
		opts := packageGraphOptions()
		opts.Graph.Budgets.MaxContextFacts = cap
		b, err := InspectMXL(context.Background(), bytes.NewReader(wire), opts)
		if b.FileMetadataCount() == 1 && b.ResourceCount() == 1 {
			if !errors.Is(err, validation.ErrResourceLimit) || b.AdoptionCount() != 1 || b.EnumerationComplete() {
				t.Fatalf("metadata promoted target: err%v files%d resources%d adoptions%d", err, b.FileMetadataCount(), b.ResourceCount(), b.AdoptionCount())
			}
			t.Logf("observed file retained before context/export refusal at budget %d", cap)
			return
		}
	}
	t.Fatal("no refusal boundary after metadata and before primary snapshot admission")
}
func TestPackageFileMetadataExcludesContainerAndPreservesEmpty(t *testing.T) {
	for _, media := range []string{"", ` media-type=""`} {
		container := `<container><rootfiles><rootfile full-path="main.musicxml"` + media + `/></rootfiles></container>`
		wire := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType}, {name: mxlContainerPath, body: container}, {name: "main.musicxml", body: `<opus/>`}}, nil, "")
		b, err := InspectMXL(context.Background(), bytes.NewReader(wire), packageGraphOptions())
		if err != nil {
			t.Fatal(err)
		}
		if b.FileMetadataCount() != 1 {
			t.Fatalf("container acquired musical advice: %d", b.FileMetadataCount())
		}
		m, _ := b.FileMetadataAt(0)
		state := validation.EvidenceAbsent
		if media != "" {
			state = validation.EvidenceKnown
		}
		if m.MediaType.State != state || m.MediaType.Value != "" {
			t.Fatal(m.MediaType)
		}
	}
}
