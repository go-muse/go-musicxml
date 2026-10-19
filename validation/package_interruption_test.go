package validation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func interruptionSnapshot(t *testing.T, a *Account, doc, text string) (*ResourceSnapshot, Report) {
	t.Helper()
	s, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(context.Background(), RunOptions{DocumentID: doc, Profile: "source-schema-test", Account: a})
	if err != nil {
		t.Fatal(err)
	}
	h := schemaPendingHeader()
	h.Location.DocumentID = doc
	if err = s.OpenNode(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	if err = s.CloseNode(context.Background(), schemaLexemeEnd(text)); err != nil {
		t.Fatal(err)
	}
	r, snap, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil || snap == nil {
		t.Fatalf("snapshot=%v err=%v", snap, err)
	}
	return snap, r
}
func interruptionPin(name string) ResourcePin {
	return ResourcePin{CanonicalID: name, Digest: strings.Repeat("b", 64), Representation: ResourceSourceXML}
}
func interruptionAdopt(t *testing.T, s *PackageSession, id ResourceAssessmentID, doc string, root bool, base string) ResourceAdoptionID {
	t.Helper()
	a, err := s.adoptResource(context.Background(), id, ResourceAdoptionOptions{Package: "outer", DocumentID: doc, Root: root, BaseURI: base})
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func TestPackageEstablishedRoleFailureSurvivesCancellation(t *testing.T) {
	for _, rolesFirst := range []bool{false, true} {
		for _, canceled := range []bool{false, true} {
			t.Run(fmt.Sprintf("roles-first=%t/canceled=%t", rolesFirst, canceled), func(t *testing.T) {
				a, _ := NewBudgetAccount(Budgets{})
				part, _ := interruptionSnapshot(t, a, "part", "1")
				score, _ := interruptionSnapshot(t, a, "score", "1")
				rules, profiles := packageFixture(t, PackageCheckScoreBeforeParts)
				h := part.Header()
				profiles[0].Intrinsic = []IntrinsicProfileBinding{{Assembly: h.Assembly.Value, Target: h.Target, Profile: h.Profile, CatalogDigest: h.CatalogDigest}}
				catalog, err := NewPackageCatalog(rules, profiles)
				if err != nil {
					t.Fatal(err)
				}
				s, err := catalog.begin(context.Background(), RunOptions{DocumentID: "package", Profile: "test.package", Account: a})
				if err != nil {
					t.Fatal(err)
				}
				if err = s.openPackage(context.Background(), packageTestIdentity()); err != nil {
					t.Fatal(err)
				}
				partID, err := s.addResource(context.Background(), interruptionPin("part"), part, true)
				if err != nil {
					t.Fatal(err)
				}
				scoreID, err := s.addResource(context.Background(), interruptionPin("score"), score, true)
				if err != nil {
					t.Fatal(err)
				}
				interruptionAdopt(t, s, partID, "part", true, "")
				interruptionAdopt(t, s, scoreID, "score-view", false, "")
				sealFixtureExpansions(t, s)
				if err = s.sealResourceMembers(context.Background(), "outer", Evidence[bool]{State: EvidenceKnown, Value: true}); err != nil {
					t.Fatal(err)
				}
				q := ResourceRoleRequest{ID: "roles", Package: "outer", Resources: []ResourcePin{interruptionPin("part"), interruptionPin("score")}, ScopeComplete: Evidence[bool]{State: EvidenceKnown, Value: true}}
				r := ResourceRoleResult{Request: q, ScopeComplete: q.ScopeComplete, Resources: []ResourceRoleFact{
					{Resource: interruptionPin("part"), Role: ResourceRoleIndividualPart, Membership: Evidence[bool]{State: EvidenceKnown, Value: true}},
					{Resource: interruptionPin("part"), Role: ResourceRoleScore, Membership: Evidence[bool]{State: EvidenceKnown, Value: false}},
					{Resource: interruptionPin("score"), Role: ResourceRoleScore, Membership: Evidence[bool]{State: EvidenceKnown, Value: true}},
				}, Authority: EvidenceAuthority{ID: "role-table", ExactRelationComplete: true, Provenance: []SourceRef{{IDs: []string{"scope"}, URL: "https://example.test/roles", Locator: "all"}}}}
				observeRoles := func() {
					if err := s.observeRoles(context.Background(), r); err != nil {
						t.Fatal(err)
					}
				}
				if rolesFirst {
					observeRoles()
				}
				if err = s.observeEntry(context.Background(), packageTestEntry(1, "part.xml")); err != nil {
					t.Fatal(err)
				}
				root := PackageRootfileObservation{ID: 2, Package: "outer", Source: NodeRef{"container", 1}, Location: Location{Target: TargetPackage, DocumentID: "package", Path: "/rootfiles/rootfile[1]"}, Ordinal: 1, FullPath: Evidence[string]{State: EvidenceKnown, Value: "part.xml"}, MediaType: Evidence[string]{State: EvidenceAbsent}, Entry: Evidence[PackageFactID]{State: EvidenceKnown, Value: 1}, Resource: Evidence[ResourceAssessmentID]{State: EvidenceKnown, Value: partID}, Indirection: Evidence[bool]{State: EvidenceKnown}}
				if err = s.observeRootfile(context.Background(), root); err != nil {
					t.Fatal(err)
				}
				if !rolesFirst {
					observeRoles()
				}
				ctx := context.Background()
				if canceled {
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				}
				bundle, err := s.finish(ctx, PackageCompletion{InputComplete: false})
				if canceled && !errors.Is(err, context.Canceled) {
					t.Fatalf("err=%v", err)
				}
				if !canceled && err != nil {
					t.Fatal(err)
				}
				if report := bundle.PackageReport(); report.Counts.Fail != 1 || report.Conformance != ConformanceViolated {
					t.Fatalf("lost already-proven score order failure: %+v", report)
				}
				if n, _ := a.Usage(BudgetBufferedBytes); n != 0 {
					t.Fatalf("leaked %d", n)
				}
			})
		}
	}
}

func TestPackageEstablishedAdviceFailureSurvivesCancellation(t *testing.T) {
	for _, kind := range []string{"mimetype-order", "mimetype-content", "image", "file"} {
		for _, canceled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/canceled=%t", kind, canceled), func(t *testing.T) {
				var s *PackageSession
				if strings.HasPrefix(kind, "mimetype") {
					rules, profiles := packageFixture(t, PackageCheckMimetypeAdvice)
					rules[0].Rule.Class, rules[0].Rule.Role, rules[0].Rule.Category = ClassRecommendation, RoleAdvisory, CategoryAdvisory
					catalog, err := NewPackageCatalog(rules, profiles)
					if err != nil {
						t.Fatal(err)
					}
					s, err = catalog.begin(context.Background(), RunOptions{DocumentID: "package", Profile: "test.package"})
					if err != nil {
						t.Fatal(err)
					}
					if err := s.openPackage(context.Background(), packageTestIdentity()); err != nil {
						t.Fatal(err)
					}
					e := packageTestEntry(1, "mimetype")
					if kind == "mimetype-order" {
						e.PhysicalOrder.Value = 1
					}
					if err := s.observeEntry(context.Background(), e); err != nil {
						t.Fatal(err)
					}
					if kind == "mimetype-content" {
						if err := s.observeMimetype(context.Background(), PackageMimetypeObservation{Entry: 1, Prefix: []byte("wrong"), ByteLength: Evidence[uint64]{State: EvidenceKnown, Value: 5}, ContentComplete: Evidence[bool]{State: EvidenceKnown, Value: true}}); err != nil {
							t.Fatal(err)
						}
					}
				} else if kind == "file" {
					var pin ResourcePin
					s, _, pin = packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
					if err := s.observeFileMetadata(context.Background(), PackageFileMetadataObservation{ID: 1, Package: "outer", Pin: pin, Name: Evidence[string]{State: EvidenceKnown, Value: "score.bad"}, MediaType: Evidence[string]{State: EvidenceAbsent}}); err != nil {
						t.Fatal(err)
					}
				} else {
					var adoption ResourceAdoptionID
					var pin ResourcePin
					s, adoption, pin = packageAdviceFixture(t)
					q := ImageMetadataRequest{Package: "outer", ID: "image", Source: pin, Subject: NodeRef{"source", 2}, Href: "image.png", MediaType: "image/png"}
					m := ImageMetadataResult{Request: q, Resource: ResourcePin{CanonicalID: "image", Digest: strings.Repeat("d", 64), Representation: ResourceBinary}, Width: Evidence[Decimal]{State: EvidenceKnown, Value: localDecimal(t, "400")}, Height: Evidence[Decimal]{State: EvidenceKnown, Value: localDecimal(t, "400")}, Unit: ImageUnitPixel, PixelAspectRatio: Evidence[Decimal]{State: EvidenceKnown, Value: localDecimal(t, "1")}, Authority: EvidenceAuthority{ID: "image-reader", Revision: "1", Provenance: []SourceRef{{IDs: []string{"image"}, URL: "https://example.test/image", Locator: "dimensions"}}}}
					if err := s.observeImageMetadata(context.Background(), adoption, 0, q, ResourceResult{RequestID: q.ID, State: EvidenceKnown, Image: &m}); err != nil {
						t.Fatal(err)
					}
				}
				ctx := context.Background()
				if canceled {
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				}
				bundle, err := s.finish(ctx, PackageCompletion{InputComplete: false})
				if canceled && !errors.Is(err, context.Canceled) {
					t.Fatalf("err=%v", err)
				}
				if !canceled && err != nil {
					t.Fatal(err)
				}
				found := false
				for _, f := range bundle.PackageReport().Findings {
					found = found || f.Outcome == OutcomeFail
				}
				if !found {
					t.Fatalf("lost known %s advice: %+v", kind, bundle.PackageReport())
				}
				if n, _ := s.account.Usage(BudgetBufferedBytes); n != 0 {
					t.Fatalf("leaked %d", n)
				}
			})
		}
	}
}

func TestPackageMimetypeAdviceRequiresKnownOrderForPass(t *testing.T) {
	rules, profiles := packageFixture(t, PackageCheckMimetypeAdvice)
	rules[0].Rule.Class, rules[0].Rule.Role, rules[0].Rule.Category = ClassRecommendation, RoleAdvisory, CategoryAdvisory
	c, err := NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.begin(context.Background(), RunOptions{DocumentID: "package", Profile: "test.package"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.openPackage(context.Background(), packageTestIdentity()); err != nil {
		t.Fatal(err)
	}
	e := packageTestEntry(1, "mimetype")
	e.PhysicalOrder = Evidence[uint64]{State: EvidenceUnknown, Reason: "order-unavailable"}
	if err = s.observeEntry(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	content := []byte("application/vnd.recordare.musicxml")
	if err = s.observeMimetype(context.Background(), PackageMimetypeObservation{Entry: 1, Prefix: content, ByteLength: Evidence[uint64]{State: EvidenceKnown, Value: uint64(len(content))}, ContentComplete: Evidence[bool]{State: EvidenceKnown, Value: true}}); err != nil {
		t.Fatal(err)
	}
	if err = s.sealEnumeration(context.Background(), completePackage("outer")); err != nil {
		t.Fatal(err)
	}
	b, err := s.finish(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if !b.Satisfied() {
		t.Fatal("optional advice blocked bundle")
	}
	found := false
	for _, f := range b.PackageReport().Findings {
		found = found || f.Outcome == OutcomeUnknown
	}
	if !found {
		t.Fatal("unknown order advice erased")
	}
}
