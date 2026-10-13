package validation

import (
	"context"
	"errors"
	"testing"
)

func fixtureLink(s *PackageSession, a ResourceAdoptionID) {
	v := &s.adoptions[a].resource.snapshot.contexts[0]
	v.Resource.Image = nil
	v.Resource.Link = &ResourceLinkContext{Kind: ResourceScoreLink, Href: ResourceAttribute{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "next.xml"}}}, Fragment: Evidence[string]{State: EvidenceAbsent}}
}
func sealFixtureExpansions(t *testing.T, s *PackageSession) {
	t.Helper()
	for _, id := range s.adoptionOrder {
		owner, err := s.ExpansionOwner(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if owner != id {
			continue
		}
		if err = s.SealResourceExpansion(context.Background(), owner, Evidence[bool]{State: EvidenceKnown, Value: true}); err != nil {
			t.Fatal(err)
		}
	}
}
func TestPackageSelfCycleExpansionHasOneOwnerAndFreshAdoptions(t *testing.T) {
	s, owner, _ := packageAdviceFixtureCheck(t, PackageCheckLinkTarget)
	fixtureLink(s, owner)
	resource := s.adoptions[owner].resource.id
	alias, err := s.AdoptResource(context.Background(), resource, ResourceAdoptionOptions{Package: "outer", DocumentID: "cycle-view"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ExpansionOwner(context.Background(), alias)
	if err != nil || got != owner {
		t.Fatalf("alias owner=%s err=%v", got, err)
	}
	if err = s.ObserveEdge(context.Background(), PackageEdgeObservation{ID: 1, Source: owner, Target: Evidence[ResourceAdoptionID]{State: EvidenceKnown, Value: alias}, Fragment: Evidence[string]{State: EvidenceAbsent}, FragmentTarget: Evidence[NodeRef]{State: EvidenceAbsent}}); err != nil {
		t.Fatal(err)
	}
	sealFixtureExpansions(t, s)
	_ = s.SealEnumeration(context.Background(), completePackage("outer"))
	b, err := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil || !b.Satisfied() {
		t.Fatalf("self cycle=%v %+v", err, b.PackageReport())
	}
	if b.AdoptionCount() != 2 || b.PackageReport().Counts.Pass != 1 {
		t.Fatal("multiplied expansion")
	}
}
func TestPackageExpansionCannotSealOmittedTemplate(t *testing.T) {
	s, a, _ := packageAdviceFixtureCheck(t, PackageCheckLinkTarget)
	fixtureLink(s, a)
	if err := s.SealResourceExpansion(context.Background(), a, Evidence[bool]{State: EvidenceKnown, Value: true}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("omitted template certified: %v", err)
	}
	b, _ := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
	if b.Satisfied() {
		t.Fatal("missing expansion accepted")
	}
}
func TestPackageExpansionSeparatesBaseAndPackageContext(t *testing.T) {
	s, a, _ := packageAdviceFixture(t)
	resource := s.adoptions[a].resource.id
	other, err := s.AdoptResource(context.Background(), resource, ResourceAdoptionOptions{Package: "outer", DocumentID: "other-base", BaseURI: "https://example.test/other/", ArchiveEntry: "score.xml"})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.ExpansionOwner(context.Background(), other)
	if err != nil || owner != other {
		t.Fatal("different base collapsed")
	}
	inner := packageTestIdentity()
	inner.ID = "inner"
	inner.Parent = "outer"
	inner.Pin.CanonicalID = "inner"
	if err = s.OpenPackage(context.Background(), inner); err != nil {
		t.Fatal(err)
	}
	nested, err := s.AdoptResource(context.Background(), resource, ResourceAdoptionOptions{Package: "inner", DocumentID: "nested"})
	if err != nil {
		t.Fatal(err)
	}
	owner, err = s.ExpansionOwner(context.Background(), nested)
	if err != nil || owner != nested {
		t.Fatal("sibling package context collapsed")
	}
	_, _ = s.Finish(context.Background(), PackageCompletion{})
}
func TestPackageUnsealedExpansionCannotCertify(t *testing.T) {
	s, _, _ := packageAdviceFixture(t)
	_ = s.SealEnumeration(context.Background(), completePackage("outer"))
	b, _ := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
	if b.Satisfied() || b.EnumerationComplete() {
		t.Fatal("in-progress expansion certified")
	}
}
