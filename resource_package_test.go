package musicxml

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func packageGraphBytes(root, name, media string, extra ...assessmentZIPFixture) []byte {
	container := `<container><rootfiles><rootfile full-path="` + name + `"`
	if media != "" {
		container += ` media-type="` + media + `"`
	}
	container += `/></rootfiles></container>`
	entries := []assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType}, {name: mxlContainerPath, body: container}, {name: name, body: root}}
	return assessmentZIPBytes(append(entries, extra...), nil, "")
}
func packageGraphOptions() MXLAssessmentOptions {
	return MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLSource}}
}
func TestPackageGraphRawPrimaryAndSourceContainer(t *testing.T) {
	b, err := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(`<opus/>`, "main.opus", "")), packageGraphOptions())
	if err != nil || !b.Satisfied() || b.ResourceCount() != 2 || !b.EnumerationComplete() {
		t.Fatalf("err=%v resources=%d report=%+v", err, b.ResourceCount(), b.PackageReport().Findings)
	}
	seen := map[validation.ProfileID]bool{}
	for i := 0; i < b.ResourceCount(); i++ {
		_, r, _ := b.ResourceReport(i)
		seen[r.Profile.ID] = true
	}
	if !seen[ProfileSourceContainer] || !seen[ProfileSourceLinks] {
		t.Fatal(seen)
	}
}
func TestPackageGraphRawMandatoryIndependentNegative(t *testing.T) {
	for _, tc := range []struct{ name, root, container, mime string }{
		{"bad-child", `<opus wrong="yes"/>`, `<container><rootfiles><rootfile full-path="main.opus"/></rootfiles></container>`, mxlMIMEType},
		{"bad-container", `<opus/>`, `<container wrong="yes"><rootfiles><rootfile full-path="main.opus"/></rootfiles></container>`, mxlMIMEType},
		{"bad-prefix", `<opus/>`, `<container><rootfiles><rootfile full-path="main.opus"/></rootfiles></container>`, "\xef\xbb\xbf" + mxlMIMEType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlMIMETypePath, body: tc.mime}, {name: mxlContainerPath, body: tc.container}, {name: "main.opus", body: tc.root}}, nil, "")
			b, err := InspectMXL(context.Background(), bytes.NewReader(data), packageGraphOptions())
			if err != nil || b.Satisfied() {
				t.Fatal(err, b.PackageReport().Findings)
			}
		})
	}
}
func TestPackageGraphRawPrefixFailureSurvivesBrokenEnvelope(t *testing.T) {
	data := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType, localExtra: "1234"}}, nil, "")
	data = data[:len(data)-22]
	b, err := InspectMXL(context.Background(), bytes.NewReader(data), packageGraphOptions())
	if err == nil || b.PackageReport().Conformance != validation.ConformanceViolated || b.Satisfied() {
		t.Fatal(err, b.PackageReport().Findings)
	}
}
func TestPackageGraphCurrentDirectModelAndNoInputMutation(t *testing.T) {
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	value := &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "main.opus"}, {FullPath: "selected.opus"}}, Resources: []MXLResource{{Path: "selected.opus", Data: []byte(`<opus/>`)}}}
	b, err := AssessMXLPackage(context.Background(), value, MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLModel}})
	if err != nil || b.ResourceCount() != 2 {
		t.Fatal(err, b.PackageReport().Findings)
	}
	_, r, _ := b.ResourceReport(0)
	if r.Target != validation.TargetModel || r.Profile.ID != ProfileModelLinks {
		t.Fatal(r)
	}
	if len(value.RootFiles) != 2 || value.RootFiles[0].FullPath != "main.opus" || value.RootFiles[1].FullPath != "selected.opus" || value.Resources[0].Path != "selected.opus" || string(value.Resources[0].Data) != `<opus/>` {
		t.Fatal("mutated current package")
	}
}
func TestPackageGraphNestedRawKeepsIndirectionGate(t *testing.T) {
	inner := packageGraphBytes(`<opus/>`, "score.musicxml", "")
	outer := packageGraphBytes(string(inner), "inner.mxl", mxlMIMEType)
	b, err := InspectMXL(context.Background(), bytes.NewReader(outer), packageGraphOptions())
	if err != nil || b.Satisfied() || b.ResourceCount() != 3 || !b.EnumerationComplete() {
		t.Fatal(err, b.ResourceCount(), b.PackageReport().Findings)
	}
	found := false
	for _, d := range b.PackageReport().Findings {
		if d.Reason == "nested-rootfile-indirection-unresolved" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing retained outer gate", b.PackageReport().Findings)
	}
}
func TestPackageGraphLocalCycleParsesOnce(t *testing.T) {
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="child.opus"/><opus-link xlink:href="child.opus"/></opus>`
	child := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="main.opus"/></opus>`
	b, err := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(root, "main.opus", "", assessmentZIPFixture{name: "child.opus", body: child})), packageGraphOptions())
	if err != nil || b.ResourceCount() != 3 || b.AdoptionCount() != 5 || !b.EnumerationComplete() {
		t.Fatal(err, b.ResourceCount(), b.AdoptionCount(), b.PackageReport().Findings)
	}
}
func TestPackageGraphRejectConfigurationBeforeRead(t *testing.T) {
	_, err := InspectMXL(context.Background(), nil, packageGraphOptions())
	if !errors.Is(err, ErrNilReader) {
		t.Fatal(err)
	}
	_, err = AssessMXLPackage(context.Background(), nil, MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLModel}})
	if !errors.Is(err, ErrNilMXLPackage) {
		t.Fatal(err)
	}
}

func TestPackageGraphRawReadFailureRetainsObservedLocalViolation(t *testing.T) {
	boom := errors.New("caller transport")
	wire := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType, localExtra: "1234"}}, nil, "")
	b, err := InspectMXL(context.Background(), &resourceReadError{data: wire[:50], err: boom}, packageGraphOptions())
	if !errors.Is(err, boom) || b.PackageReport().Conformance != validation.ConformanceViolated || b.Satisfied() {
		t.Fatal(err, b.PackageReport().Findings)
	}
}

func TestPackageGraphNestedExpandedCapIsAggregate(t *testing.T) {
	inner := packageGraphBytes(`<opus/>`, "main.opus", "")
	outer := packageGraphBytes(string(inner), "inner.mxl", mxlMIMEType)
	// Discover exact selected-byte accounting through the same private run, then
	// apply the independently known sum at public exact/one-under boundaries.
	selected := int64(len(mxlMIMEType) + len(`<container><rootfiles><rootfile full-path="inner.mxl" media-type="`+mxlMIMEType+`"/></rootfiles></container>`) + len(inner) + len(mxlMIMEType) + len(`<container><rootfiles><rootfile full-path="main.opus"/></rootfiles></container>`) + len(`<opus/>`))
	for _, delta := range []int64{0, -1} {
		options := packageGraphOptions()
		options.MXL.MaxResourcesBytes = selected + delta
		b, err := InspectMXL(context.Background(), bytes.NewReader(outer), options)
		if delta == 0 && err != nil {
			t.Fatal("exact aggregate cap", err)
		}
		if delta < 0 && (!errors.Is(err, ErrMXLTooLarge) || b.Satisfied()) {
			t.Fatal("one below", err)
		}
	}
}

func TestPackageGraphMXLArchiveContainerDocumentCaps(t *testing.T) {
	container := `<container><rootfiles><rootfile full-path="main.opus"/></rootfiles></container>`
	data := packageGraphBytes(`<opus/>`, "main.opus", "")
	for _, tc := range []struct {
		name    string
		options MXLOptions
	}{
		{"archive", MXLOptions{MaxArchiveBytes: int64(len(data)) - 1}},
		{"metadata", MXLOptions{MaxMetadataBytes: int64(len(container)) - 1}},
		{"document", MXLOptions{MaxDocumentBytes: 6}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := packageGraphOptions()
			options.MXL = tc.options
			_, err := InspectMXL(context.Background(), bytes.NewReader(data), options)
			if !errors.Is(err, ErrMXLTooLarge) {
				t.Fatal(err)
			}
		})
	}
}
