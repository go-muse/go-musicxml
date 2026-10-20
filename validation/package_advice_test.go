package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func packageAdviceFixture(t *testing.T) (*PackageSession, ResourceAdoptionID, ResourcePin) {
	return packageAdviceFixtureCheck(t, PackageCheckImageScaling)
}
func packageAdviceFixtureCheck(t *testing.T, check PackageCheck, budgets ...Budgets) (*PackageSession, ResourceAdoptionID, ResourcePin) {
	t.Helper()
	intrinsic := snapshotTestSession(t, "1", budgets...)
	_, snap, err := intrinsic.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	// A closed typed projection fixture avoids parser/I/O dependencies.
	snap.contexts = []ContextResult{{Subject: NodeRef{"source", 2}, Property: "resource.image", Resource: &ResourceContextResult{Owner: NodeRef{"source", 1}, Image: &ResourceImageContext{Source: ResourceAttribute{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "image.png"}}}, MediaType: ResourceAttribute{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "image/png"}}}, Width: ResourceAttribute{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicDecimal, Decimal: localDecimal(t, "10")}}}, Height: ResourceAttribute{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicDecimal, Decimal: localDecimal(t, "5")}}}}}}}
	rules, p := packageFixture(t, check)
	if check == PackageCheckImageScaling || check == PackageCheckFileMetadataAdvice || check == PackageCheckGroupMatch {
		rules[0].Rule.Class = ClassRecommendation
		rules[0].Rule.Role = RoleAdvisory
		rules[0].Rule.Category = CategoryAdvisory
	}
	h := snap.Header()
	p[0].Intrinsic = []IntrinsicProfileBinding{{Assembly: h.Assembly.Value, Target: h.Target, Profile: h.Profile, CatalogDigest: h.CatalogDigest}}
	c, err := NewPackageCatalog(rules, p)
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.begin(context.Background(), RunOptions{Context: ContextFamilyAll, DocumentID: "package", Profile: "test.package", Account: intrinsic.account})
	if err != nil {
		t.Fatal(err)
	}
	identity := packageTestIdentity()
	if err = s.openPackage(context.Background(), identity); err != nil {
		t.Fatal(err)
	}
	pin := ResourcePin{CanonicalID: "score", Representation: ResourceSourceXML, Digest: strings.Repeat("c", 64)}
	id, err := s.addResource(context.Background(), pin, snap, true)
	if err != nil {
		t.Fatal(err)
	}
	adoption, err := s.adoptResource(context.Background(), id, ResourceAdoptionOptions{Package: "outer", DocumentID: "source", Root: true})
	if err != nil {
		t.Fatal(err)
	}
	return s, adoption, pin
}
func TestPackageImageAdviceRatioAndOptionalUnknown(t *testing.T) {
	for _, known := range []bool{true, false} {
		s, a, pin := packageAdviceFixture(t)
		q := ImageMetadataRequest{Package: "outer", ID: "image", Source: pin, Subject: NodeRef{"source", 2}, Href: "image.png", MediaType: "image/png"}
		r := ResourceResult{RequestID: q.ID, State: EvidenceUnknown, Reason: "metadata-unavailable"}
		if known {
			m := ImageMetadataResult{Request: q, Resource: ResourcePin{CanonicalID: "image", Digest: strings.Repeat("d", 64), Representation: ResourceBinary}, Width: Evidence[Decimal]{State: EvidenceKnown, Value: localDecimal(t, "400")}, Height: Evidence[Decimal]{State: EvidenceKnown, Value: localDecimal(t, "200")}, Unit: ImageUnitPixel, PixelAspectRatio: Evidence[Decimal]{State: EvidenceKnown, Value: localDecimal(t, "1")}, Authority: EvidenceAuthority{ID: "image-reader", Revision: "1", Provenance: []SourceRef{{IDs: []string{"image"}, URL: "https://example.test/image", Locator: "dimensions"}}}}
			r = ResourceResult{RequestID: q.ID, State: EvidenceKnown, Image: &m}
		}
		if err := s.observeImageMetadata(context.Background(), a, 0, q, r); err != nil {
			t.Fatal(err)
		}
		sealFixtureExpansions(t, s)
		if err := s.sealEnumeration(context.Background(), completePackage("outer")); err != nil {
			t.Fatal(err)
		}
		b, err := s.finish(context.Background(), PackageCompletion{InputComplete: true})
		if err != nil {
			t.Fatal(err)
		}
		if !b.Satisfied() {
			t.Fatalf("optional advice blocked acceptance: %+v", b.PackageReport())
		}
		advice, ok := b.ImageAdviceAt(0)
		if !ok || b.ImageAdviceCount() != 1 {
			t.Fatal("missing advice")
		}
		if known && (advice.IntrinsicAspectRatio.State != EvidenceKnown || advice.IntrinsicAspectRatio.Value.String() != "2" || advice.PreservesAspectRatio.State != EvidenceKnown || !advice.PreservesAspectRatio.Value) {
			t.Fatalf("advice=%+v", advice)
		}
		if !known && advice.IntrinsicAspectRatio.State == EvidenceKnown {
			t.Fatal("invented dimensions")
		}
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("leaked %d", used)
		}
	}
}

func TestPackageUnknownRoleCannotProveExclusion(t *testing.T) {
	s, _, pin := packageAdviceFixtureCheck(t, PackageCheckScoreBeforeParts)
	q := ResourceRoleRequest{ID: "roles", Package: "outer", Resources: []ResourcePin{pin}, ScopeComplete: Evidence[bool]{State: EvidenceKnown, Value: true}}
	r := ResourceRoleResult{Request: q, ScopeComplete: q.ScopeComplete, Resources: []ResourceRoleFact{{Resource: pin, Role: ResourceRoleScore, Membership: Evidence[bool]{State: EvidenceUnknown, Reason: "partial-membership"}}, {Resource: pin, Role: ResourceRoleIndividualPart, Membership: Evidence[bool]{State: EvidenceKnown, Value: true}}}, Authority: EvidenceAuthority{ID: "role-table", ExactRelationComplete: true, Provenance: []SourceRef{{IDs: []string{"scope"}, URL: "https://example.test/roles", Locator: "all"}}}}
	if err := s.observeRoles(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	sealFixtureExpansions(t, s)
	if err := s.sealEnumeration(context.Background(), completePackage("outer")); err != nil {
		t.Fatal(err)
	}
	b, err := s.finish(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if b.Satisfied() || b.PackageReport().Counts.Unknown != 1 {
		t.Fatalf("unknown role proved exclusion: %+v", b.PackageReport())
	}
}
func TestPackageUnresolvedEdgeChargesLinkedAdmission(t *testing.T) {
	s, a, _ := packageAdviceFixture(t)
	snap := s.adoptions[a].resource.snapshot
	snap.contexts[0].Resource.Image = nil
	snap.contexts[0].Resource.Link = &ResourceLinkContext{Kind: ResourceScoreLink, Href: ResourceAttribute{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "unavailable"}}}}
	err := s.observeEdge(context.Background(), PackageEdgeObservation{ID: 1, Source: a, Target: Evidence[ResourceAdoptionID]{State: EvidenceUnknown, Reason: "unavailable"}, Indirection: Evidence[bool]{State: EvidenceKnown, Value: false}, Fragment: Evidence[string]{State: EvidenceAbsent}, FragmentTarget: Evidence[NodeRef]{State: EvidenceAbsent}})
	if err != nil {
		t.Fatal(err)
	}
	if used, _ := s.account.Usage(BudgetLinkedDocuments); used != 1 {
		t.Fatalf("unresolved edge admission=%d", used)
	}
	_, _ = s.finish(context.Background(), PackageCompletion{})
}
func TestPackageFileAdviceOwnedAndNonblocking(t *testing.T) {
	s, _, pin := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
	v := PackageFileMetadataObservation{ID: 1, Package: "outer", Pin: pin, Name: Evidence[string]{State: EvidenceKnown, Value: "piece.bad"}, MediaType: Evidence[string]{State: EvidenceKnown, Value: "application/vnd.recordare.musicxml+xml"}}
	if err := s.observeFileMetadata(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	sealFixtureExpansions(t, s)
	_ = s.sealEnumeration(context.Background(), completePackage("outer"))
	b, err := s.finish(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil || !b.Satisfied() {
		t.Fatalf("advice blocked: %v %+v", err, b.PackageReport())
	}
	got, ok := b.FileMetadataAt(0)
	if !ok || got.Name.Value != "piece.bad" {
		t.Fatal("missing metadata")
	}
	got.Name.Value = "changed"
	again, _ := b.FileMetadataAt(0)
	if again.Name.Value != "piece.bad" {
		t.Fatal("getter aliases metadata")
	}
}

func TestPackageImagePartialMetadataChecksEveryScalar(t *testing.T) {
	s, a, pin := packageAdviceFixtureCheck(t, PackageCheckImageScaling, Budgets{MaxScalarBytes: 512})
	q := ImageMetadataRequest{Package: "outer", ID: "image", Source: pin, Subject: NodeRef{"source", 2}, Href: "image.png", MediaType: "image/png"}
	metadata := ImageMetadataResult{Request: q, Resource: ResourcePin{CanonicalID: "image", Digest: strings.Repeat("d", 64), Representation: ResourceBinary}, Width: Evidence[Decimal]{State: EvidenceKnown, Value: localDecimal(t, strings.Repeat("9", 1000))}, Height: Evidence[Decimal]{State: EvidenceUnknown, Reason: "partial-size"}, Unit: ImageUnitPoint, PixelAspectRatio: Evidence[Decimal]{State: EvidenceAbsent}, Authority: EvidenceAuthority{ID: "image-reader", Provenance: []SourceRef{{IDs: []string{"size"}, URL: "https://example.test/image", Locator: "dimensions"}}}}
	err := s.observeImageMetadata(context.Background(), a, 0, q, ResourceResult{RequestID: q.ID, State: EvidenceKnown, Image: &metadata})
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("known scalar bypassed budget because peer unavailable: %v", err)
	}
	_, _ = s.finish(context.Background(), PackageCompletion{})
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("leaked %d", used)
	}
}
