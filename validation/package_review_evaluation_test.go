package validation

import (
	"context"
	"strings"
	"testing"
)

func TestPackageReviewSettledDecisionsDoNotRepeatWork(t *testing.T) {
	for _, kind := range []string{"rootfile", "entry", "edge"} {
		t.Run(kind, func(t *testing.T) {
			var s *PackageSession
			var repeat func() error
			switch kind {
			case "rootfile":
				s = beginPackageFixture(t, PackageCheckFirstMediaType)
				if err := s.openPackage(context.Background(), packageTestIdentity()); err != nil {
					t.Fatal(err)
				}
				v := PackageRootfileObservation{ID: 1, Package: "outer", Source: NodeRef{"container", 1}, Location: Location{Target: TargetPackage, DocumentID: "package", Path: "/rootfile[1]"}, Ordinal: 1, FullPath: Evidence[string]{State: EvidenceUnknown, Reason: "unavailable"}, MediaType: Evidence[string]{State: EvidenceKnown, Value: "application/pdf"}, Entry: Evidence[PackageFactID]{State: EvidenceUnknown, Reason: "unavailable"}, Resource: Evidence[ResourceAssessmentID]{State: EvidenceUnknown, Reason: "unavailable"}, Indirection: Evidence[bool]{State: EvidenceUnknown, Reason: "unavailable"}}
				if err := s.observeRootfile(context.Background(), v); err != nil {
					t.Fatal(err)
				}
				rule := s.catalog.rules[s.catalog.orders[s.profile.ID][0]]
				repeat = func() error { return s.evaluateRootfileRule(context.Background(), rule, v, true) }
			case "entry":
				s = beginPackageFixture(t, PackageCheckMimetypeStored)
				if err := s.openPackage(context.Background(), packageTestIdentity()); err != nil {
					t.Fatal(err)
				}
				v := packageTestEntry(1, "mimetype")
				v.Local.Method.Value = 8
				if err := s.observeEntry(context.Background(), v); err != nil {
					t.Fatal(err)
				}
				repeat = func() error { return s.evaluateEntry(context.Background(), v, true) }
			case "edge":
				var source, target ResourceAdoptionID
				s, source, target = reviewEdgeFixture(t)
				v := PackageEdgeObservation{ID: 1, Source: source, Target: Evidence[ResourceAdoptionID]{State: EvidenceKnown, Value: target}, Indirection: Evidence[bool]{State: EvidenceKnown, Value: false}, Fragment: Evidence[string]{State: EvidenceAbsent}, FragmentTarget: Evidence[NodeRef]{State: EvidenceAbsent}}
				if err := s.observeEdge(context.Background(), v); err != nil {
					t.Fatal(err)
				}
				repeat = func() error { return s.evaluateEdge(context.Background(), v, true) }
			}
			before, _ := s.account.Usage(BudgetRuleSteps)
			if err := repeat(); err != nil {
				t.Fatal(err)
			}
			after, _ := s.account.Usage(BudgetRuleSteps)
			if after != before {
				t.Errorf("settled %s decision charged another %d steps", kind, after-before)
			}
			_, _ = s.finish(context.Background(), PackageCompletion{})
		})
	}
}

func TestPackageReviewMinimalVacuityRequiresSelectedLinks(t *testing.T) {
	for _, tc := range []struct {
		name     string
		kind     planKind
		support  ImplementationState
		selected bool
	}{
		{"schema-only", "", "", false},
		{"available-links", planContextMusicXMLLinks, ImplementationAvailable, true},
		{"unsupported-links", planContextMusicXMLLinks, ImplementationUnsupported, false},
		{"missing-support", planContextMusicXMLLinks, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := selectionWitnessSnapshot(t, tc.kind, tc.support)
			// Supply the established musical root kind for this miniature schema;
			// retain the real finalized support witness and empty projection.
			snapshot.header.Name = Evidence[QName]{State: EvidenceKnown, Value: QName{Local: "score-partwise"}}
			rules, profiles := packageFixture(t, PackageCheckScoreBeforeParts)
			h := snapshot.Header()
			profiles[0].Intrinsic = []IntrinsicProfileBinding{{Assembly: h.Assembly.Value, Target: h.Target, Profile: h.Profile, CatalogDigest: h.CatalogDigest}}
			catalog, err := NewPackageCatalog(rules, profiles)
			if err != nil {
				t.Fatal(err)
			}
			s, err := catalog.begin(context.Background(), RunOptions{Context: ContextFamilyAll, DocumentID: "package", Profile: "test.package", Account: snapshot.account})
			if err != nil {
				t.Fatal(err)
			}
			if err = s.openPackage(context.Background(), packageTestIdentity()); err != nil {
				t.Fatal(err)
			}
			pin := ResourcePin{CanonicalID: "score", Representation: ResourceSourceXML, Digest: strings.Repeat("c", 64)}
			resourceID, err := s.addResource(context.Background(), pin, snapshot, true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.adoptResource(context.Background(), resourceID, ResourceAdoptionOptions{Package: "outer", DocumentID: "source", Root: true}); err != nil {
				t.Fatal(err)
			}
			for i, name := range []string{"META-INF/container.xml", "score.musicxml"} {
				if err := s.observeEntry(context.Background(), packageTestEntry(PackageFactID(i+1), name)); err != nil {
					t.Fatal(err)
				}
			}
			root := PackageRootfileObservation{ID: 3, Package: "outer", Source: NodeRef{"container", 1}, Location: Location{Target: TargetPackage, DocumentID: "package", Path: "/rootfile[1]"}, Ordinal: 1, FullPath: Evidence[string]{State: EvidenceKnown, Value: "score.musicxml"}, MediaType: Evidence[string]{State: EvidenceAbsent}, Entry: Evidence[PackageFactID]{State: EvidenceKnown, Value: 2}, Resource: Evidence[ResourceAssessmentID]{State: EvidenceKnown, Value: resourceID}, Indirection: Evidence[bool]{State: EvidenceKnown}}
			if err := s.observeRootfile(context.Background(), root); err != nil {
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
			r := b.PackageReport()
			// Check the package report as well as the bundle: an intrinsic
			// Unsupported result must not mask a false package-level proof.
			if b.Satisfied() != tc.selected || r.Satisfied() != tc.selected {
				t.Errorf("link projection selected=%t report=%+v", tc.selected, r)
			}
			if !tc.selected && (r.Counts.Unknown != 1 || r.Counts.NotApplicable != 0 || len(r.Findings) != 1 || r.Findings[0].Reason != "package-score-part-role-evidence-unavailable") {
				t.Errorf("unselected links inferred absent: %+v", r)
			}
			if tc.selected && r.Counts.NotApplicable != 1 {
				t.Errorf("available link projection did not prove minimal vacuity: %+v", r)
			}
		})
	}
}
