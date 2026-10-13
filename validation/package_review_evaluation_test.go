package validation

import (
	"context"
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
				if err := s.OpenPackage(context.Background(), packageTestIdentity()); err != nil {
					t.Fatal(err)
				}
				v := PackageRootfileObservation{ID: 1, Package: "outer", Source: NodeRef{"container", 1}, Location: Location{Target: TargetPackage, DocumentID: "package", Path: "/rootfile[1]"}, Ordinal: 1, FullPath: Evidence[string]{State: EvidenceUnknown, Reason: "unavailable"}, MediaType: Evidence[string]{State: EvidenceKnown, Value: "application/pdf"}, Entry: Evidence[PackageFactID]{State: EvidenceUnknown, Reason: "unavailable"}, Resource: Evidence[ResourceAssessmentID]{State: EvidenceUnknown, Reason: "unavailable"}, Indirection: Evidence[bool]{State: EvidenceUnknown, Reason: "unavailable"}}
				if err := s.ObserveRootfile(context.Background(), v); err != nil {
					t.Fatal(err)
				}
				rule := s.catalog.rules[s.catalog.orders[s.profile.ID][0]]
				repeat = func() error { return s.evaluateRootfileRule(context.Background(), rule, v, true) }
			case "entry":
				s = beginPackageFixture(t, PackageCheckMimetypeStored)
				if err := s.OpenPackage(context.Background(), packageTestIdentity()); err != nil {
					t.Fatal(err)
				}
				v := packageTestEntry(1, "mimetype")
				v.Local.Method.Value = 8
				if err := s.ObserveEntry(context.Background(), v); err != nil {
					t.Fatal(err)
				}
				repeat = func() error { return s.evaluateEntry(context.Background(), v, true) }
			case "edge":
				var source, target ResourceAdoptionID
				s, source, target = reviewEdgeFixture(t)
				v := PackageEdgeObservation{ID: 1, Source: source, Target: Evidence[ResourceAdoptionID]{State: EvidenceKnown, Value: target}, Fragment: Evidence[string]{State: EvidenceAbsent}, FragmentTarget: Evidence[NodeRef]{State: EvidenceAbsent}}
				if err := s.ObserveEdge(context.Background(), v); err != nil {
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
			_, _ = s.Finish(context.Background(), PackageCompletion{})
		})
	}
}

func TestPackageReviewMinimalVacuityRequiresSelectedLinks(t *testing.T) {
	for _, selected := range []bool{false, true} {
		s, adoption, _ := packageAdviceFixtureCheck(t, PackageCheckScoreBeforeParts)
		resource := s.adoptions[adoption].resource
		resource.snapshot.header.Name = Evidence[QName]{State: EvidenceKnown, Value: QName{Local: "score-partwise"}}
		resource.snapshot.header.LinkProjectionSelected = selected
		resource.snapshot.contexts = nil
		for i, name := range []string{"META-INF/container.xml", "score.musicxml"} {
			if err := s.ObserveEntry(context.Background(), packageTestEntry(PackageFactID(i+1), name)); err != nil {
				t.Fatal(err)
			}
		}
		root := PackageRootfileObservation{ID: 3, Package: "outer", Source: NodeRef{"container", 1}, Location: Location{Target: TargetPackage, DocumentID: "package", Path: "/rootfile[1]"}, Ordinal: 1, FullPath: Evidence[string]{State: EvidenceKnown, Value: "score.musicxml"}, MediaType: Evidence[string]{State: EvidenceAbsent}, Entry: Evidence[PackageFactID]{State: EvidenceKnown, Value: 2}, Resource: Evidence[ResourceAssessmentID]{State: EvidenceKnown, Value: resource.id}, Indirection: Evidence[bool]{State: EvidenceKnown}}
		if err := s.ObserveRootfile(context.Background(), root); err != nil {
			t.Fatal(err)
		}
		sealFixtureExpansions(t, s)
		if err := s.SealEnumeration(context.Background(), completePackage("outer")); err != nil {
			t.Fatal(err)
		}
		b, err := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
		if err != nil {
			t.Fatal(err)
		}
		r := b.PackageReport()
		if b.Satisfied() != selected {
			t.Errorf("link projection selected=%t report=%+v", selected, r)
		}
		if !selected && (r.Counts.Unknown != 1 || r.Counts.NotApplicable != 0) {
			t.Errorf("unselected links inferred absent: %+v", r)
		}
	}
}
