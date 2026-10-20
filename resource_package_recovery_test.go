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

func TestRecoveryPackageLongInputIsProcessingRefusal(t *testing.T) {
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("x", 4097)
	for _, tc := range []struct {
		name    string
		raw     []byte
		current *MXLPackage
	}{
		{name: "raw-name", raw: packageGraphBytes(`<opus/>`, "main.opus", "", assessmentZIPFixture{name: long, body: "x"})},
		{name: "raw-nonutf8-name", raw: packageGraphBytes(`<opus/>`, "main.opus", "", assessmentZIPFixture{name: long + "\xff", body: "x"})},
		{name: "raw-root-token", raw: assessmentZIPBytes([]assessmentZIPFixture{{name: mxlContainerPath, body: `<container><rootfiles><rootfile full-path="` + long + `"/></rootfiles></container>`}}, nil, "")},
		{name: "raw-media-token", raw: packageGraphBytes(`<opus/>`, "main.opus", long)},
		{name: "current-root", current: &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: long}}}},
		{name: "current-media", current: &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "main.opus", MediaType: Ptr(long)}}}},
		{name: "current-resource", current: &MXLPackage{Document: doc, Resources: []MXLResource{{Path: long, Data: []byte("x")}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var e error
			var b validation.AssessmentBundle
			if tc.current != nil {
				b, e = AssessMXLPackage(context.Background(), tc.current, MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel}})
			} else {
				b, e = InspectMXL(context.Background(), bytes.NewReader(tc.raw), packageGraphOptions())
			}
			if !errors.Is(e, ErrMXLTooLarge) || errors.Is(e, validation.ErrInvalidConfiguration) || b.EnumerationComplete() {
				t.Fatalf("error=%v complete=%v", e, b.EnumerationComplete())
			}
			if !strings.Contains(e.Error(), "4096") {
				t.Fatalf("fixed evidence bound omitted: %v", e)
			}
		})
	}
}

func TestRecoveryPackageNULNameIsPathPolicy(t *testing.T) {
	_, err := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(`<opus/>`, "main.opus", "", assessmentZIPFixture{name: "a\x00b", body: "x"})), packageGraphOptions())
	if !errors.Is(err, ErrMXLInvalidPath) || errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatal(err)
	}
}

func TestRecoveryPackageCurrentDuplicateRoots(t *testing.T) {
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, roots := range [][]MXLRootFile{{{FullPath: "main.opus"}, {FullPath: "main.opus"}}, {{FullPath: "main.opus"}, {FullPath: "r.opus"}, {FullPath: "r.opus"}}} {
		_, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: doc, RootFiles: roots, Resources: []MXLResource{{Path: "r.opus", Data: []byte(`<opus/>`)}}}, MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel}})
		if !errors.Is(err, ErrMXLDuplicateEntry) || errors.Is(err, ErrMXLRootFileNotFound) {
			t.Fatal(err)
		}
	}
}

func TestRecoveryPackageLaterMediaControlsParsing(t *testing.T) {
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, media := range []string{"application/octet-stream", "text/plain", "application/x-custom", "not a MIME", "application/vnd.recordare.musicxml+xml; version=4"} {
		t.Run(media, func(t *testing.T) {
			knownXML := media == "application/vnd.recordare.musicxml+xml; version=4"
			container := `<container><rootfiles><rootfile full-path="main.opus"/><rootfile full-path="alt.bin" media-type="` + media + `"/></rootfiles></container>`
			wire := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlContainerPath, body: container}, {name: "main.opus", body: `<opus/>`}, {name: "alt.bin", body: "\x00binary"}}, nil, "")
			b, err := InspectMXL(context.Background(), bytes.NewReader(wire), packageGraphOptions())
			if (err != nil) != knownXML || b.ResourceCount() != 2 {
				t.Fatalf("raw error=%v resources=%d", err, b.ResourceCount())
			}
			b, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "main.opus"}, {FullPath: "alt.bin", MediaType: Ptr(media)}}, Resources: []MXLResource{{Path: "alt.bin", Data: []byte("\x00binary")}}}, MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel}})
			if (err != nil) != knownXML || b.ResourceCount() != 1 {
				t.Fatalf("current error=%v resources=%d", err, b.ResourceCount())
			}
		})
	}
}

func TestRecoveryPackageExternalMalformedMediaIsOptional(t *testing.T) {
	for _, media := range []string{"not a MIME", "text/plain; charset"} {
		t.Run(media, func(t *testing.T) {
			body := `<opus/>`
			resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "child", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(body)))}, RetrievalURI: "https://example.test/child.opus", MediaType: media, Body: &resourceTestBody{Reader: strings.NewReader(body)}}, nil
			})
			b, err := InspectXMLWithResources(context.Background(), strings.NewReader(`<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://example.test/child.opus"/></opus>`), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
			if err != nil || !b.Satisfied() {
				t.Fatalf("error=%v report=%+v", err, b.PackageReport())
			}
			if b.FileMetadataCount() != 1 {
				t.Fatal(b.FileMetadataCount())
			}
			m, _ := b.FileMetadataAt(0)
			if m.MediaType.State != validation.EvidenceUnknown {
				t.Fatal(m.MediaType)
			}
		})
	}
}

func TestRecoveryPackageCurrentMetadataBeforeIntrinsicFailure(t *testing.T) {
	doc, err := Decode(strings.NewReader(`<opus><opus><opus><opus/></opus></opus></opus>`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := AssessMXLPackage(context.Background(), &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "primary.bad"}}}, MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel}, MXL: MXLOptions{MaxXMLDepth: 3}})
	if !errors.Is(err, ErrXMLTooDeep) || b.FileMetadataCount() != 1 {
		t.Fatalf("error=%v metadata=%d", err, b.FileMetadataCount())
	}
	m, _ := b.FileMetadataAt(0)
	if m.Pin.SnapshotID == "" || m.Location.Path != "RootFiles[0]" {
		t.Fatal(m)
	}
	found := false
	for _, f := range b.PackageReport().Findings {
		found = found || strings.Contains(string(f.Rule.ID), "file-metadata")
	}
	if !found {
		t.Fatal(b.PackageReport())
	}
}

func TestRecoveryPackageDigestBoundedWork(t *testing.T) {
	for _, n := range []int{0, 1, 4096, 4097, 8192} {
		g := packageBodyRun(t, context.Background(), MXLOptions{})
		data := bytes.Repeat([]byte{'x'}, n)
		got, err := g.packageDigest(data)
		used, _ := g.options.account.Usage(validation.BudgetRuleSteps)
		if err != nil || got != fmt.Sprintf("%x", sha256.Sum256(data)) || used != int64(1+(n+4095)/4096) {
			t.Fatalf("size=%d digest=%s steps=%d error=%v", n, got, used, err)
		}
		g.close()
	}
}

func TestRecoveryPackageCurrentCopyBoundedWorkAndBytes(t *testing.T) {
	for _, n := range []int{0, 1, 4096, 4097, 8192} {
		g := packageBodyRun(t, context.Background(), MXLOptions{})
		data := bytes.Repeat([]byte{'x'}, n)
		p := &graphPackage{current: map[string][]byte{"x": data}, copied: map[string][]byte{}}
		copied, err := g.currentPackageEntryBytes(p, "x", mxlAssessmentResource)
		steps, _ := g.options.account.Usage(validation.BudgetRuleSteps)
		external, _ := g.options.account.Usage(validation.BudgetExternalBytes)
		live, _ := g.options.account.Usage(validation.BudgetBufferedBytes)
		if err != nil || !bytes.Equal(copied, data) || steps != int64(1+(n+4095)/4096) || external != int64(n) || live < int64(cap(copied)) {
			t.Fatalf("size=%d steps=%d bytes=%d live=%d err=%v", n, steps, external, live, err)
		}
		g.close()
		live, _ = g.options.account.Usage(validation.BudgetBufferedBytes)
		if live != 0 {
			t.Fatal(live)
		}
	}
}

func TestRecoveryPackageNestedDiagnosticIdentity(t *testing.T) {
	inner := packageGraphBytes(`<opus/>`, "score.musicxml", "application/pdf")
	container := `<container><rootfiles><rootfile full-path="main.opus"/><rootfile full-path="a.mxl" media-type="` + mxlMIMEType + `"/><rootfile full-path="b.mxl" media-type="` + mxlMIMEType + `"/></rootfiles></container>`
	wire := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlContainerPath, body: container}, {name: "main.opus", body: `<opus/>`}, {name: "a.mxl", body: string(inner)}, {name: "b.mxl", body: string(inner)}}, nil, "")
	b, err := InspectMXL(context.Background(), bytes.NewReader(wire), packageGraphOptions())
	if err != nil {
		t.Fatal(err)
	}
	scopes := map[string]bool{}
	for _, f := range b.PackageReport().Findings {
		if strings.HasSuffix(string(f.Rule.ID), "first-media-type") {
			scopes[f.Location.Entry+"|"+f.Location.Path] = true
		}
	}
	if len(scopes) != 2 {
		t.Fatalf("indistinguishable nested locations: %v", scopes)
	}
	for scope := range scopes {
		if !strings.Contains(scope, "a.mxl") && !strings.Contains(scope, "b.mxl") {
			t.Fatalf("missing containing archive: %s", scope)
		}
	}
}

func TestRecoveryPackageInvalidParentCannotInventRootContexts(t *testing.T) {
	for _, content := range []string{`<rootfiles><rootfile full-path="main.opus"/></rootfiles><extra/>`, `<rootfiles><rootfile full-path="main.opus"/></rootfiles><rootfiles><rootfile full-path="other.opus"/></rootfiles>`} {
		data := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlContainerPath, body: `<container>` + content + `</container>`}, {name: "main.opus", body: `<opus/>`}, {name: "other.opus", body: `<opus/>`}}, nil, "")
		g := packageMetadataGraph(t, data, packageGraphOptions())
		if len(g.packageList[0].rootfiles) != 0 || len(g.queue) != 1 {
			t.Fatal("invented package child identity")
		}
		adoption, err := validationPackageSessionAdoption(g.session, g.ctx, g.queue[0].adoption)
		if err != nil {
			t.Fatal(err)
		}
		rootFailure := false
		for i := 0; i < adoption.ContextCount(); i++ {
			row, ok, err := adoption.ContextAt(g.ctx, i)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || row.Container == nil {
				continue
			}
			c := row.Container
			if c.Role == validation.ContainerRoot {
				rootFailure = c.Content.State != validation.EvidenceKnown || !c.Content.Value
			}
			if c.Role == validation.ContainerRootfiles || c.Role == validation.ContainerRootfile {
				t.Fatalf("fixture unexpectedly admitted typed child %+v", c)
			}
		}
		if !rootFailure {
			t.Fatal("missing invalid parent proof")
		}
		b, err := g.finish(nil)
		if err != nil || b.ResourceCount() != 1 || b.FileMetadataCount() != 0 || b.EnumerationComplete() {
			t.Fatalf("err=%v resources=%d metadata=%d complete=%v", err, b.ResourceCount(), b.FileMetadataCount(), b.EnumerationComplete())
		}
	}
}

func TestRecoveryPackageRoleScopeFixedBound(t *testing.T) {
	data := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType, localExtra: "1234"}, {name: mxlContainerPath, body: `<container><rootfiles><rootfile full-path="main.opus"/></rootfiles></container>`}, {name: "main.opus", body: `<opus/>`}}, nil, "")
	g := packageMetadataGraph(t, data, packageGraphOptions())
	// Construct the otherwise costly context count directly; the refused pass
	// must neither allocate request state nor erase earlier independent truth.
	item := g.queue[0]
	g.queue = make([]graphExpansion, 65537)
	for i := range g.queue {
		g.queue[i] = item
	}
	before, _ := g.options.account.Usage(validation.BudgetBufferedBytes)
	err := g.observePackageRoleScope(g.packageList[0])
	var incomplete *validation.IncompleteError
	var configured *validation.ResourceError
	if !errors.As(err, &incomplete) || incomplete.Reason != "package-role-scope-too-large" || errors.As(err, &configured) || errors.Is(err, validation.ErrInvalidConfiguration) || !errors.Is(err, validation.ErrResourceLimit) || !strings.Contains(err.Error(), "65536") {
		t.Fatal(err)
	}
	after, _ := g.options.account.Usage(validation.BudgetBufferedBytes)
	if before != after {
		t.Fatalf("refusal allocated request state: before=%d after=%d", before, after)
	}
	b, err := g.finish(err)
	if !errors.Is(err, validation.ErrResourceLimit) || b.PackageReport().Conformance != validation.ConformanceViolated || b.EnumerationComplete() {
		t.Fatalf("error=%v report=%+v", err, b.PackageReport())
	}
	after, _ = g.options.account.Usage(validation.BudgetBufferedBytes)
	if after != 0 {
		t.Fatalf("live refusal leases: %d", after)
	}
}

func TestRecoveryPackageExactEvidenceTextBound(t *testing.T) {
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	exact := strings.Repeat("x", 4096)
	for _, surface := range []string{"raw", "current"} {
		t.Run(surface, func(t *testing.T) {
			var e error
			if surface == "raw" {
				_, e = InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(`<opus/>`, exact, exact)), packageGraphOptions())
			} else {
				_, e = AssessMXLPackage(context.Background(), &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: exact, MediaType: Ptr(exact)}}}, MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel}})
			}
			if e != nil {
				t.Fatalf("exact 4096-byte bound refused: %v", e)
			}
		})
	}
}

func TestRecoveryPackageVerifiedPinBorrowAvoidsSecondDigest(t *testing.T) {
	g := packageMetadataGraph(t, packageGraphBytes(`<opus/>`, "main.opus", ""), packageGraphOptions())
	defer g.finish(errors.New("fixture complete"))
	p := g.packageList[0]
	var pin validation.ResourcePin
	for i := range g.queue {
		if g.queue[i].base.Entry == "main.opus" {
			pin = g.queue[i].resource.pin
		}
	}
	if pin.Digest == "" {
		t.Fatal("no verified member pin")
	}
	before, _ := g.options.account.Usage(validation.BudgetRuleSteps)
	resource, _, err := g.assessVerifiedPackageSource(p, "main.opus", []byte(`<opus/>`), ProfileSourceLinks.ID(), pin)
	after, _ := g.options.account.Usage(validation.BudgetRuleSteps)
	// The sole charge is contextual XML-depth admission, not another hash.
	if err != nil || resource == nil || after-before != 1 {
		t.Fatalf("error=%v resource=%v extra steps=%d", err, resource, after-before)
	}
}

func TestRecoveryPackageOptionalRolePassDoesNotClaimAuthority(t *testing.T) {
	for _, selected := range []bool{false, true} {
		options := packageGraphOptions()
		if selected {
			options.Dependencies = []ResourceDependency{ResourceRoles}
		}
		options.Resolver = graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
			if len(q.Roles.Members) != 3 {
				t.Fatal("all admitted member contract changed", q.Roles.Members)
			}
			return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "role-authority-unavailable"}, nil
		})
		b, err := InspectMXL(context.Background(), bytes.NewReader(packageRoleData()), options)
		if err != nil || !b.EnumerationComplete() || b.Satisfied() {
			t.Fatalf("selected=%v err=%v enumeration=%v satisfied=%v", selected, err, b.EnumerationComplete(), b.Satisfied())
		}
	}
}
