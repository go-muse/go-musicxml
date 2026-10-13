package musicxml

import (
	"bytes"
	"context"
	"errors"
	"github.com/go-muse/go-musicxml/validation"
	"testing"
)

func packageRoleData(extra ...assessmentZIPFixture) []byte {
	entries := []assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType}, {name: mxlContainerPath, body: `<container><rootfiles><rootfile full-path="part.xml"/><rootfile full-path="score.xml"/></rootfiles></container>`}, {name: "part.xml", body: `<opus/>`}, {name: "score.xml", body: `<opus/>`}}
	return assessmentZIPBytes(append(entries, extra...), nil, "")
}
func packageMetadataGraph(t *testing.T, data []byte, options MXLAssessmentOptions) *resourceGraphRun {
	t.Helper()
	g, err := startResourceGraph(context.Background(), options.linked(), ProfileMXLSource)
	if err != nil {
		t.Fatal(err)
	}
	a, err := openMXLAssessmentArchive(g.ctx, bytes.NewReader(data), MXLOptions{}, g.options.account)
	if err != nil {
		t.Fatal(err)
	}
	if err = g.ownArchive(a); err != nil {
		t.Fatal(err)
	}
	digest, err := g.packageDigest(a.data)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = g.importRawPackage(a, g.primaryPin(validation.ResourceRawMXL, digest), "", options.Identity.RetrievalURI, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = g.walkResources(); err != nil {
		t.Fatal(err)
	}
	return g
}
func packageRoleReply(q validation.ResourceRequest) validation.ResourceResult {
	r := validation.ResourceRoleResult{Request: *q.Roles, ScopeComplete: q.Roles.ScopeComplete, Authority: validation.EvidenceAuthority{ID: "fixture-authority", ExactRelationComplete: true, Provenance: []validation.SourceRef{{IDs: []string{"roles"}, URL: "https://example.test/roles", Locator: "exact-member-scope"}}}}
	for _, m := range q.Roles.Members {
		for _, role := range []validation.ResourceRole{validation.ResourceRoleScore, validation.ResourceRoleIndividualPart} {
			member := m
			r.Resources = append(r.Resources, validation.ResourceRoleFact{Resource: m.Resource, Member: &member, Role: role, Membership: packageKnown(m.ArchiveEntry == "score.xml" && role == validation.ResourceRoleScore || m.ArchiveEntry == "part.xml" && role == validation.ResourceRoleIndividualPart)})
		}
	}
	return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Roles: &r}
}
func TestPackageMetadataRoleProviderExactScope(t *testing.T) {
	calls := 0
	options := packageGraphOptions()
	options.Dependencies = []ResourceDependency{ResourceRoles}
	options.Identity.RetrievalURI = "https://example.test/archive.mxl"
	options.Resolver = graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		calls++
		if q.Roles == nil || len(q.Roles.Members) != 3 || q.Roles.ScopeComplete.State != validation.EvidenceKnown || !q.Roles.ScopeComplete.Value {
			t.Fatalf("scope=%+v", q)
		}
		for _, m := range q.Roles.Members {
			if m.BaseURI.State != validation.EvidenceKnown || m.BaseURI.Value != options.Identity.RetrievalURI {
				t.Fatalf("base=%+v", m)
			}
		}
		return packageRoleReply(q), nil
	})
	g := packageMetadataGraph(t, packageRoleData(), options)
	err := g.observePackageMetadata()
	if err == nil {
		err = g.sealPackages()
	}
	b, err := g.finish(err)
	if err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	found := false
	for _, f := range b.PackageReport().Findings {
		found = found || f.Reason == "score-must-precede-parts"
	}
	if !found {
		t.Fatalf("no authoritative member order failure: %+v", b.PackageReport())
	}
	if used, _ := g.options.account.Usage(validation.BudgetBufferedBytes); used != 0 {
		t.Fatal("leaked", used)
	}
}
func TestPackageMetadataSelectionAndIncompleteScope(t *testing.T) {
	for _, selected := range []bool{false, true} {
		t.Run(map[bool]string{false: "unselected", true: "selected"}[selected], func(t *testing.T) {
			calls := 0
			options := packageGraphOptions()
			if selected {
				options.Dependencies = []ResourceDependency{ResourceRoles}
			}
			options.Resolver = graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				calls++
				if q.Roles.ScopeComplete.State != validation.EvidenceUnknown {
					t.Fatal("unread member falsely complete")
				}
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "partial-role-table"}, nil
			})
			g := packageMetadataGraph(t, packageRoleData(assessmentZIPFixture{name: "unread.dat", body: "unread"}), options)
			err := g.observePackageMetadata()
			if err == nil {
				err = g.sealPackages()
			}
			b, err := g.finish(err)
			if err != nil || calls != map[bool]int{false: 0, true: 1}[selected] || b.Satisfied() {
				t.Fatal(err, calls, b.PackageReport())
			}
		})
	}
}
func TestPackageMetadataRejectsMutatedResolverScope(t *testing.T) {
	options := packageGraphOptions()
	options.Dependencies = []ResourceDependency{ResourceRoles}
	options.Resolver = graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		q.Roles.Members[0].ArchiveEntry = "forged.xml"
		return packageRoleReply(q), nil
	})
	g := packageMetadataGraph(t, packageRoleData(), options)
	err := g.observePackageMetadata()
	if err == nil {
		t.Fatal("resolver mutation replaced retained request")
	}
	g.finish(err)
	if used, _ := g.options.account.Usage(validation.BudgetBufferedBytes); used != 0 {
		t.Fatal("leaked", used)
	}
}
func TestPackageMetadataRefusesBeforeResolverAllocation(t *testing.T) {
	calls := 0
	options := packageGraphOptions()
	options.Dependencies = []ResourceDependency{ResourceRoles}
	options.Resolver = graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		calls++
		return packageRoleReply(q), nil
	})
	g := packageMetadataGraph(t, packageRoleData(), options)
	used, _ := g.options.account.Usage(validation.BudgetBufferedBytes)
	remaining := g.options.account.Limits().MaxBufferedBytes - used
	if err := g.options.account.Charge(validation.BudgetBufferedBytes, remaining); err != nil {
		t.Fatal(err)
	}
	err := g.observePackageMetadata()
	g.options.account.Release(validation.BudgetBufferedBytes, remaining)
	var limit *validation.ResourceError
	if !errors.As(err, &limit) || calls != 0 {
		t.Fatal(err, calls)
	}
	g.finish(err)
	if used, _ := g.options.account.Usage(validation.BudgetBufferedBytes); used != 0 {
		t.Fatal("leaked", used)
	}
}

func TestPackageMetadataGatesUnusableLocalScope(t *testing.T) {
	for _, kind := range []string{"partial-inventory", "metadata-only"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			options := packageGraphOptions()
			options.Dependencies = []ResourceDependency{ResourceRoles}
			options.Resolver = graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				calls++
				return packageRoleReply(q), nil
			})
			g := packageMetadataGraph(t, packageRoleData(), options)
			if kind == "partial-inventory" {
				g.packageList[0].entriesComplete = packageUnknown[bool]("partial-fixture")
			} else {
				g.queue = g.queue[:1]
			}
			err := g.observePackageRoles()
			if err != nil || calls != 0 {
				t.Fatal(err, calls)
			}
			g.finish(err)
		})
	}
}

func TestPackageMetadataPlainGraphHasNoRoleRequest(t *testing.T) {
	calls := 0
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		calls++
		return packageRoleReply(q), nil
	})
	options := LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}, Dependencies: []ResourceDependency{ResourceRoles}, Resolver: resolver}
	b, err := InspectXMLWithResources(context.Background(), bytes.NewBufferString(`<opus/>`), options)
	if err != nil || !b.Satisfied() || calls != 0 {
		t.Fatal(err, calls, b.PackageReport())
	}
}
