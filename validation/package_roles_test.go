package validation

import (
	"context"
	"strings"
	"testing"
)

func roleMemberUnsealedFixture(t *testing.T) (*PackageSession, ResourceRoleRequest, ResourceRoleResult) {
	t.Helper()
	a, _ := NewBudgetAccount(Budgets{})
	snap, _ := interruptionSnapshot(t, a, "canonical", "1")
	rules, profiles := packageFixture(t, PackageCheckScoreBeforeParts)
	h := snap.Header()
	profiles[0].Intrinsic = []IntrinsicProfileBinding{{Assembly: h.Assembly.Value, Target: h.Target, Profile: h.Profile, CatalogDigest: h.CatalogDigest}}
	c, err := NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.Begin(context.Background(), RunOptions{DocumentID: "package", Profile: "test.package", Account: a})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.OpenPackage(context.Background(), packageTestIdentity()); err != nil {
		t.Fatal(err)
	}
	pin := testResourcePin("same-content")
	rid, err := s.AddResource(context.Background(), pin, snap, true)
	if err != nil {
		t.Fatal(err)
	}
	for i, path := range []string{"part.xml", "score.xml"} {
		doc := path
		if i == 0 {
			doc = "canonical"
		}
		aid, err := s.AdoptResource(context.Background(), rid, ResourceAdoptionOptions{Package: "outer", DocumentID: doc, ArchiveEntry: path, Root: i == 0})
		if err != nil {
			t.Fatal(err)
		}
		if err = s.SealResourceExpansion(context.Background(), aid, Evidence[bool]{State: EvidenceKnown, Value: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.ObserveEntry(context.Background(), packageTestEntry(1, "part.xml")); err != nil {
		t.Fatal(err)
	}
	if err = s.ObserveRootfile(context.Background(), PackageRootfileObservation{ID: 2, Package: "outer", Source: NodeRef{"container", 1}, Location: Location{Target: TargetPackage, DocumentID: "package", Path: "/rootfile"}, Ordinal: 1, FullPath: Evidence[string]{State: EvidenceKnown, Value: "part.xml"}, MediaType: Evidence[string]{State: EvidenceAbsent}, Entry: Evidence[PackageFactID]{State: EvidenceKnown, Value: 1}, Resource: Evidence[ResourceAssessmentID]{State: EvidenceKnown, Value: rid}, Indirection: Evidence[bool]{State: EvidenceKnown}}); err != nil {
		t.Fatal(err)
	}
	if err = s.ObserveEntry(context.Background(), packageTestEntry(3, "score.xml")); err != nil {
		t.Fatal(err)
	}
	members := []ResourceRoleMember{{Resource: pin, ArchiveEntry: "part.xml", BaseURI: Evidence[string]{State: EvidenceAbsent}}, {Resource: pin, ArchiveEntry: "score.xml", BaseURI: Evidence[string]{State: EvidenceAbsent}}}
	q := ResourceRoleRequest{ID: "roles", Package: "outer", Resources: []ResourcePin{pin}, Members: members, ScopeComplete: Evidence[bool]{State: EvidenceKnown, Value: true}}
	r := ResourceRoleResult{Request: q, ScopeComplete: q.ScopeComplete, Authority: EvidenceAuthority{ID: "table", ExactRelationComplete: true, Provenance: []SourceRef{{IDs: []string{"roles"}, URL: "https://example.test/roles", Locator: "scope"}}}}
	for i, m := range members {
		for _, role := range []ResourceRole{ResourceRoleScore, ResourceRoleIndividualPart} {
			member := m
			r.Resources = append(r.Resources, ResourceRoleFact{Resource: pin, Member: &member, Role: role, Membership: Evidence[bool]{State: EvidenceKnown, Value: (i == 1) == (role == ResourceRoleScore)}})
		}
	}
	return s, q, r
}
func roleMemberFixture(t *testing.T) (*PackageSession, ResourceRoleRequest, ResourceRoleResult) {
	s, q, r := roleMemberUnsealedFixture(t)
	if err := s.SealResourceMembers(context.Background(), "outer", Evidence[bool]{State: EvidenceKnown, Value: true}); err != nil {
		t.Fatal(err)
	}
	return s, q, r
}
func TestPackageRolesPreserveDistinctMembersWithSamePin(t *testing.T) {
	s, _, r := roleMemberFixture(t)
	// Result facts may arrive in any order. The echoed request remains ordered.
	for i, j := 0, len(r.Resources)-1; i < j; i, j = i+1, j-1 {
		r.Resources[i], r.Resources[j] = r.Resources[j], r.Resources[i]
	}
	if err := s.ObserveRoles(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b, _ := s.Finish(ctx, PackageCompletion{})
	if b.PackageReport().Counts.Fail != 1 {
		t.Fatalf("member identity collapsed: %+v", b.PackageReport())
	}
}
func TestPackageRoleRequestEchoAndMemberValidation(t *testing.T) {
	for _, kind := range []string{"reordered-echo", "forged-member", "extra-member", "missing-echo", "unknown-base", "duplicate-member", "missing-pin"} {
		t.Run(kind, func(t *testing.T) {
			s, q, r := roleMemberFixture(t)
			defer s.Finish(context.Background(), PackageCompletion{})
			q = stateClone(q)
			r = stateClone(r)
			switch kind {
			case "reordered-echo":
				r.Request.Members[0], r.Request.Members[1] = r.Request.Members[1], r.Request.Members[0]
			case "forged-member":
				r.Resources[0].Member.ArchiveEntry = "forged.xml"
			case "extra-member":
				r.Request.Members = append(r.Request.Members, ResourceRoleMember{Resource: q.Resources[0], ArchiveEntry: "extra.xml", BaseURI: Evidence[string]{State: EvidenceAbsent}})
			case "missing-echo":
				r.Request.Members = r.Request.Members[:1]
			case "unknown-base":
				q.Members[0].BaseURI = Evidence[string]{State: EvidenceUnknown, Reason: "missing"}
				r.Request = q
			case "duplicate-member":
				q.Members[1] = q.Members[0]
				r.Request = q
			case "missing-pin":
				q.Resources = nil
				r.Request = q
			}
			if err := ValidateResourceResult(ResourceRequest{ID: q.ID, Roles: &q}, ResourceResult{RequestID: q.ID, State: EvidenceKnown, Roles: &r}); err == nil {
				t.Fatal("invalid member request/result accepted")
			}
		})
	}
}
func TestPackageRoleMemberMustMatchAdmittedContext(t *testing.T) {
	for _, kind := range []string{"path", "known-empty-base", "sibling-scope"} {
		t.Run(kind, func(t *testing.T) {
			s, _, r := roleMemberFixture(t)
			switch kind {
			case "path":
				r.Request.Members[0].ArchiveEntry = "forged.xml"
			case "known-empty-base":
				r.Request.Members[0].BaseURI = Evidence[string]{State: EvidenceKnown}
			case "sibling-scope":
				p := packageTestIdentity()
				p.ID = "sibling"
				p.Parent = "outer"
				p.Pin.CanonicalID = "sibling"
				p.Pin.Digest = strings.Repeat("d", 64)
				if err := s.OpenPackage(context.Background(), p); err != nil {
					t.Fatal(err)
				}
				r.Request.Package = "sibling"
			}
			for i := range r.Resources {
				if r.Resources[i].Member.ArchiveEntry == "part.xml" {
					*r.Resources[i].Member = r.Request.Members[0]
				}
			}
			if err := s.ObserveRoles(context.Background(), r); err == nil {
				t.Fatal("unadmitted member context accepted")
			}
			s.Finish(context.Background(), PackageCompletion{})
		})
	}
}
func TestPackageMissingRoleClassificationCannotProveExclusion(t *testing.T) {
	s, _, r := roleMemberFixture(t)
	r.Resources = nil
	if err := s.ObserveRoles(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := s.SealEnumeration(context.Background(), completePackage("outer")); err != nil {
		t.Fatal(err)
	}
	b, err := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if b.Satisfied() || b.PackageReport().Counts.NotApplicable != 0 {
		t.Fatalf("missing classifications became exclusion: %+v", b.PackageReport())
	}
}

func TestPackageLegacyPinRolesCannotClassifyAmbiguousMemberContexts(t *testing.T) {
	s, q, r := roleMemberFixture(t)
	q.Members = nil
	r.Request = q
	r.Resources = []ResourceRoleFact{{Resource: q.Resources[0], Role: ResourceRoleScore, Membership: Evidence[bool]{State: EvidenceKnown, Value: false}}, {Resource: q.Resources[0], Role: ResourceRoleIndividualPart, Membership: Evidence[bool]{State: EvidenceKnown, Value: true}}}
	if err := s.ObserveRoles(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := s.SealEnumeration(context.Background(), completePackage("outer")); err != nil {
		t.Fatal(err)
	}
	b, err := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if b.Satisfied() {
		t.Fatal("legacy pin inferred membership across multiple contexts")
	}
}

func TestPackageCompleteRoleRequestCannotOmitAdmittedMembers(t *testing.T) {
	s, q, r := roleMemberFixture(t)
	q.Members = q.Members[:1]
	r.Request = q
	r.Resources = r.Resources[:2]
	// The listed member is authoritatively neither a score nor a part; the other
	// admitted member is outside this response and cannot become negative space.
	for i := range r.Resources {
		r.Resources[i].Membership.Value = false
	}
	if err := s.ObserveRoles(context.Background(), r); err != nil {
		s.Finish(context.Background(), PackageCompletion{})
		return
	}
	if err := s.SealEnumeration(context.Background(), completePackage("outer")); err != nil {
		t.Fatal(err)
	}
	b, err := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if b.Satisfied() {
		t.Fatal("subset role scope certified whole package")
	}
}

func TestPackageMemberSealPreventsNewContextsButAllowsAliases(t *testing.T) {
	s, _, _ := roleMemberFixture(t)
	rid := s.resourceOrder[0]
	if _, err := s.AdoptResource(context.Background(), rid, ResourceAdoptionOptions{Package: "outer", DocumentID: "alias", ArchiveEntry: "part.xml"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdoptResource(context.Background(), rid, ResourceAdoptionOptions{Package: "outer", DocumentID: "late", ArchiveEntry: "late.xml"}); err == nil {
		t.Fatal("new member after seal")
	}
	s.Finish(context.Background(), PackageCompletion{})
}
func TestPackagePartialRolesWaitForExplicitMemberSeal(t *testing.T) {
	s, _, r := roleMemberUnsealedFixture(t)
	if err := s.ObserveRoles(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if len(s.decisions) != 0 {
		t.Fatal("unsealed member inventory certified roles")
	}
	if err := s.SealResourceMembers(context.Background(), "outer", Evidence[bool]{State: EvidenceKnown, Value: true}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b, _ := s.Finish(ctx, PackageCompletion{})
	if b.PackageReport().Counts.Fail != 1 {
		t.Fatal("seal did not preserve independently proved failure")
	}
}
func TestPackageCanceledMemberSealCannotCertifyInventory(t *testing.T) {
	s, _, _ := roleMemberUnsealedFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.SealResourceMembers(ctx, "outer", Evidence[bool]{State: EvidenceKnown, Value: true}); err == nil {
		t.Fatal("canceled seal succeeded")
	}
	if s.boundaries["outer"].membersSealed {
		t.Fatal("canceled seal changed inventory")
	}
	s.Finish(context.Background(), PackageCompletion{})
}
