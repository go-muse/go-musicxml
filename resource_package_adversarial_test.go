package musicxml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestPackageGraphPartialReadKeepsKnownHeaderFailure(t *testing.T) {
	data := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType, localExtra: "1234"}}, nil, "")
	data = data[:45]
	boom := errors.New("caller stream failed")
	reader := &assessmentZIPReader{data: data, err: boom}
	bundle, err := InspectMXL(context.Background(), reader, packageGraphOptions())
	if !errors.Is(err, boom) || bundle.PackageReport().Conformance != validation.ConformanceViolated || bundle.EnumerationComplete() || reader.closed != 0 {
		t.Fatalf("err=%v conformance=%v complete=%v closed=%d findings=%v", err, bundle.PackageReport().Conformance, bundle.EnumerationComplete(), reader.closed, bundle.PackageReport().Findings)
	}
}
func TestPackageGraphContainerGatesAndLiteralPaths(t *testing.T) {
	for _, tc := range []struct {
		name, container, entry string
		complete               bool
		reports                int
	}{
		{"literal percent", `<container><rootfiles><rootfile full-path="a%20b.opus"/></rootfiles></container>`, "a%20b.opus", true, 2},
		{"collapsed token", `<container><rootfiles><rootfile full-path="  a b.opus&#x9; "/></rootfiles></container>`, "a b.opus", true, 2},
		{"unbound child", `<container><unknown/><rootfiles><rootfile full-path="main.opus"/></rootfiles></container>`, "main.opus", false, 1},
		{"wrong container root", `<sounds/>`, "main.opus", false, 0},
		{"missing path", `<container><rootfiles><rootfile/></rootfiles></container>`, "main.opus", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlContainerPath, body: tc.container}, {name: tc.entry, body: `<opus/>`}}, nil, "")
			b, err := InspectMXL(context.Background(), bytes.NewReader(data), packageGraphOptions())
			if (err != nil && tc.name != "wrong container root") || b.ResourceCount() != tc.reports || b.EnumerationComplete() != tc.complete {
				t.Fatalf("err=%v resources=%d enum=%v findings=%v", err, b.ResourceCount(), b.EnumerationComplete(), b.PackageReport().Findings)
			}
			if tc.name == "wrong container root" {
				original, originalErr := InspectXML(context.Background(), strings.NewReader(tc.container), SourceOptions{Profile: ProfileSourceContainer})
				failure, ok := b.Failure()
				if !errors.Is(err, ErrUnsupportedRoot) || originalErr == nil || original.Terminal == nil || !ok || failure.Target != original.Target || failure.Profile != original.Profile.ID || failure.CatalogDigest != original.Profile.CatalogDigest || failure.Counts != original.Counts || failure.Conformance != original.Conformance || failure.Terminal != *original.Terminal {
					t.Fatalf("unadmitted original container failure lost: %+v error=%v", failure, err)
				}
			}
		})
	}
}
func TestPackageGraphLaterAlternateAndCurrentPrimaryBackreference(t *testing.T) {
	container := `<container><rootfiles><rootfile full-path="main.opus"/><rootfile full-path="view.pdf" media-type="application/pdf"/></rootfiles></container>`
	data := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlContainerPath, body: container}, {name: "main.opus", body: `<opus/>`}, {name: "view.pdf", body: "%PDF-not-XML"}}, nil, "")
	b, err := InspectMXL(context.Background(), bytes.NewReader(data), packageGraphOptions())
	if err != nil || b.ResourceCount() != 2 || !b.EnumerationComplete() {
		t.Fatalf("alternate parsed or inventory lost: %v %d %v", err, b.ResourceCount(), b.PackageReport().Findings)
	}
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="child.opus"/></opus>`
	child := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="main.opus"/></opus>`
	doc, err := Decode(strings.NewReader(root))
	if err != nil {
		t.Fatal(err)
	}
	value := &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "main.opus"}}, Resources: []MXLResource{{Path: "child.opus", Data: []byte(child)}}}
	b, err = AssessMXLPackage(context.Background(), value, MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel}})
	if err != nil || b.ResourceCount() != 2 || b.AdoptionCount() != 3 {
		t.Fatalf("current backreference reparsed primary: %v %d %d %v", err, b.ResourceCount(), b.AdoptionCount(), b.PackageReport().Findings)
	}
	var models int
	for i := 0; i < b.ResourceCount(); i++ {
		_, r, _ := b.ResourceReport(i)
		if r.Target == validation.TargetModel {
			models++
		}
	}
	if models != 1 {
		t.Fatal("primary model was replaced by source", models)
	}
}
func TestPackageGraphSiblingArchivesHaveDistinctNamespaces(t *testing.T) {
	first := packageGraphBytes(`<opus/>`, "main.opus", "")
	second := packageGraphBytes(`<opus wrong="yes"/>`, "main.opus", "")
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="first.mxl"/><opus-link xlink:href="second.mxl"/></opus>`
	data := packageGraphBytes(root, "main.opus", "", assessmentZIPFixture{name: "first.mxl", body: string(first)}, assessmentZIPFixture{name: "second.mxl", body: string(second)})
	b, err := InspectMXL(context.Background(), bytes.NewReader(data), packageGraphOptions())
	if err != nil || b.ResourceCount() != 6 || b.Satisfied() {
		t.Fatalf("namespace collision: %v %d %v", err, b.ResourceCount(), b.PackageReport().Findings)
	}
	var violated int
	for i := 0; i < b.ResourceCount(); i++ {
		_, r, _ := b.ResourceReport(i)
		if r.Conformance == validation.ConformanceViolated {
			violated++
		}
	}
	if violated != 1 {
		t.Fatal("lost exact sibling report", violated)
	}
}
func TestPackageGraphNestedAggregateByteCaps(t *testing.T) {
	innerContainer := `<container><rootfiles><rootfile full-path="main.opus"/></rootfiles></container>`
	inner := packageGraphBytes(`<opus/>`, "main.opus", "")
	outerContainer := `<container><rootfiles><rootfile full-path="inner.mxl" media-type="` + mxlMIMEType + `"/></rootfiles></container>`
	outer := packageGraphBytes(string(inner), "inner.mxl", mxlMIMEType)
	total := int64(2*len(mxlMIMEType) + len(outerContainer) + len(inner) + len(innerContainer) + len(`<opus/>`))
	for _, delta := range []int64{0, -1} {
		o := packageGraphOptions()
		o.MXL.MaxResourcesBytes = total + delta
		b, err := InspectMXL(context.Background(), bytes.NewReader(outer), o)
		if delta == 0 && (err != nil || b.ResourceCount() != 3 || !b.EnumerationComplete()) {
			t.Fatalf("exact aggregate %d: %v reports=%d", total, err, b.ResourceCount())
		}
		if delta < 0 && !errors.Is(err, ErrMXLTooLarge) {
			t.Fatalf("nested reset aggregate %d: %v", total-1, err)
		}
	}
}
func TestPackageGraphRawExactByteOwnership(t *testing.T) {
	root := `<opus/>`
	container := `<container><rootfiles><rootfile full-path="main.opus"/></rootfiles></container>`
	data := packageGraphBytes(root, "main.opus", "")
	total := int64(len(mxlMIMEType) + len(container) + len(root))
	tests := []struct {
		name  string
		exact int64
		set   func(*MXLAssessmentOptions, int64)
		want  error
	}{
		{"archive", int64(len(data)), func(o *MXLAssessmentOptions, n int64) { o.MXL.MaxArchiveBytes = n }, ErrMXLTooLarge},
		{"metadata", int64(len(container)), func(o *MXLAssessmentOptions, n int64) { o.MXL.MaxMetadataBytes = n }, ErrMXLTooLarge},
		{"document", int64(len(root)), func(o *MXLAssessmentOptions, n int64) { o.MXL.MaxDocumentBytes = n }, ErrMXLTooLarge},
		{"expanded", total, func(o *MXLAssessmentOptions, n int64) { o.MXL.MaxResourcesBytes = n }, ErrMXLTooLarge},
		{"input account", int64(len(data)), func(o *MXLAssessmentOptions, n int64) { o.Graph.Budgets.MaxInputBytes = n }, validation.ErrResourceLimit},
		{"external account", total, func(o *MXLAssessmentOptions, n int64) { o.Graph.Budgets.MaxExternalBytes = n }, validation.ErrResourceLimit},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, delta := range []int64{0, -1} {
				o := packageGraphOptions()
				tc.set(&o, tc.exact+delta)
				b, err := InspectMXL(context.Background(), bytes.NewReader(data), o)
				if delta == 0 && (err != nil || !b.Satisfied()) {
					t.Fatalf("exact: %v %v", err, b.PackageReport().Findings)
				}
				if delta < 0 && !errors.Is(err, tc.want) {
					t.Fatalf("one-under: %v", err)
				}
			}
		})
	}
}

type packageRequestedReader struct{ requested int }

func (r *packageRequestedReader) Read(p []byte) (int, error) { r.requested = len(p); return 0, io.EOF }
func TestPackageSharedExpandedReaderBoundsReadBeforeIO(t *testing.T) {
	account := assessmentZIPAccount(t, validation.Budgets{})
	shared := int64(8)
	a := &mxlAssessmentArchive{ctx: context.Background(), account: account, limits: mxlLimits{resourcesSize: 10}, sharedExpanded: &shared}
	probe := &packageRequestedReader{}
	r := &mxlAssessmentExpandedReader{archive: a, body: io.NopCloser(probe), limit: 100}
	_, err := r.Read(make([]byte, 100))
	if err != io.EOF || probe.requested != 2 {
		t.Fatalf("shared remaining admitted only2, requested=%d err=%v", probe.requested, err)
	}
}

func packageGraphMeasuredRun(t *testing.T, ctx context.Context, data []byte) (*resourceGraphRun, error) {
	t.Helper()
	g, err := startResourceGraph(ctx, packageGraphOptions().internal().linked(), ProfileMXLSource.ID())
	if err != nil {
		return nil, err
	}
	a, readErr := openMXLAssessmentArchive(ctx, bytes.NewReader(data), MXLOptions{}, g.options.account)
	if a == nil {
		_, err = g.finish(readErr)
		return g, err
	}
	if err = g.ownArchive(a); err != nil {
		a.Close()
		_, err = g.finish(errors.Join(readErr, err))
		return g, err
	}
	digest := ""
	if a.inputComplete {
		digest, err = g.packageDigest(a.data)
	}
	if err == nil {
		_, _, _, err = g.importRawPackage(a, g.primaryPin(validation.ResourceRawMXL, digest), "", "", true)
	}
	err = errors.Join(readErr, err)
	if err == nil {
		err = g.walkResources()
	}
	if err == nil {
		err = g.sealPackages()
	}
	_, err = g.finish(err)
	return g, err
}
func TestPackageGraphCleanupAtCancellationBoundaries(t *testing.T) {
	data := packageGraphBytes(`<opus/>`, "main.opus", "")
	full := &assessmentZIPCountingContext{}
	run, err := packageGraphMeasuredRun(t, full, data)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
		used, _ := run.options.account.Usage(kind)
		if used != 0 {
			t.Fatal("baseline leak", kind, used)
		}
	}
	positions := []int{1, 2, 5, 10, 20, 40, 80, 120, full.polls - 1}
	for i := 1; i <= 9; i++ {
		positions = append(positions, full.polls*i/10)
	}
	for _, at := range positions {
		run, err := packageGraphMeasuredRun(t, &assessmentZIPCountingContext{fail: at}, data)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel poll%d/%d: %v", at, full.polls, err)
		}
		if run == nil {
			continue
		}
		for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
			used, _ := run.options.account.Usage(kind)
			if used != 0 {
				t.Fatalf("poll%d leak %s=%d", at, kind, used)
			}
		}
		if run.archives != nil || run.packages != nil || run.resourceBodies != nil {
			t.Fatal("retained package owners after cleanup")
		}
	}
}
func TestPackageGraphCurrentResourceCopyIsIndependent(t *testing.T) {
	g, err := startResourceGraph(context.Background(), MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel}}.internal().linked(), ProfileMXLModel.ID())
	if err != nil {
		t.Fatal(err)
	}
	data := []byte(`<opus/>`)
	p, err := g.newPackage(g.primaryPin(validation.ResourceCurrentMXL, ""), "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	p.current = map[string][]byte{"child.opus": data}
	p.copied = make(map[string][]byte)
	copied, err := g.currentPackageEntryBytes(p, "child.opus", mxlAssessmentResource)
	if err != nil {
		t.Fatal(err)
	}
	data[1] = 'X'
	if string(copied) != `<opus/>` {
		t.Fatal("selected copy aliases caller resource")
	}
	again, err := g.currentPackageEntryBytes(p, "child.opus", mxlAssessmentResource)
	if err != nil || string(again) != `<opus/>` || g.mxlExpanded != int64(len(copied)) {
		t.Fatal("copy cache mismatch or duplicate charge", err, g.mxlExpanded)
	}
	_, _ = g.finish(errors.New("test complete"))
	used, _ := g.options.account.Usage(validation.BudgetBufferedBytes)
	if used != 0 {
		t.Fatal(used)
	}
}

func TestPackageGraphAdditionalRootfileResourceCap(t *testing.T) {
	child := `<opus>` + strings.Repeat(" ", 80) + `</opus>`
	container := `<container><rootfiles><rootfile full-path="main.opus"/><rootfile full-path="extra.opus"/></rootfiles></container>`
	data := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlContainerPath, body: container}, {name: "main.opus", body: `<opus/>`}, {name: "extra.opus", body: child}}, nil, "")
	for _, delta := range []int64{0, -1} {
		o := packageGraphOptions()
		o.MXL.MaxResourceBytes = int64(len(child)) + delta
		b, err := InspectMXL(context.Background(), bytes.NewReader(data), o)
		if delta == 0 && (err != nil || b.ResourceCount() != 3) {
			t.Fatalf("exact: %v resources%d", err, b.ResourceCount())
		}
		if delta < 0 && !errors.Is(err, ErrMXLTooLarge) {
			t.Fatalf("additional rootfile bypassed resource cap: %v", err)
		}
	}
}
