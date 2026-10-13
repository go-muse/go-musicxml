package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func beginPackageFixture(t *testing.T, checks ...PackageCheck) *PackageSession {
	t.Helper()
	r, p := packageFixture(t, checks...)
	c, e := NewPackageCatalog(r, p)
	if e != nil {
		t.Fatal(e)
	}
	s, e := c.Begin(context.Background(), RunOptions{DocumentID: "package", Profile: "test.package"})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func packageTestIdentity() PackageIdentity {
	return PackageIdentity{ID: "outer", Representation: ResourceRawMXL, Pin: ResourcePin{CanonicalID: "archive", Revision: "1", Digest: strings.Repeat("a", 64), Representation: ResourceRawMXL}}
}
func completePackage(id PackageID) PackageEnumeration {
	return PackageEnumeration{Package: id, Entries: Evidence[bool]{State: EvidenceKnown, Value: true}, Rootfiles: Evidence[bool]{State: EvidenceKnown, Value: true}, Edges: Evidence[bool]{State: EvidenceKnown, Value: true}, Roles: Evidence[bool]{State: EvidenceKnown, Value: true}}
}
func packageTestEntry(id PackageFactID, name string) PackageEntryObservation {
	return PackageEntryObservation{ID: id, Package: "outer", Name: name, Location: Location{Target: TargetPackage, DocumentID: "package", Entry: name}, Local: ZIPHeaderObservation{Present: Evidence[bool]{State: EvidenceKnown, Value: true}, Method: Evidence[uint16]{State: EvidenceKnown}, Flags: Evidence[uint16]{State: EvidenceKnown}, ExtraLength: Evidence[uint64]{State: EvidenceKnown}}, Central: ZIPHeaderObservation{Present: Evidence[bool]{State: EvidenceKnown, Value: true}, Method: Evidence[uint16]{State: EvidenceKnown}, Flags: Evidence[uint16]{State: EvidenceKnown}, ExtraLength: Evidence[uint64]{State: EvidenceKnown}}, HeaderAssociation: Evidence[bool]{State: EvidenceKnown, Value: true}, UTF8Name: Evidence[bool]{State: EvidenceKnown, Value: true}, PhysicalOrder: Evidence[uint64]{State: EvidenceKnown}}
}
func TestPackageCompleteVacuityDiffersFromUnknown(t *testing.T) {
	for _, complete := range []bool{true, false} {
		s := beginPackageFixture(t, PackageCheckMimetypeStored)
		if e := s.OpenPackage(context.Background(), packageTestIdentity()); e != nil {
			t.Fatal(e)
		}
		if complete {
			if e := s.SealEnumeration(context.Background(), completePackage("outer")); e != nil {
				t.Fatal(e)
			}
		}
		b, e := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
		if e != nil {
			t.Fatal(e)
		}
		if b.Satisfied() != complete {
			t.Fatalf("complete=%t package=%+v", complete, b.PackageReport())
		}
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("leaked %d", used)
		}
	}
}
func TestPackageKnownFailureSurvivesIncompleteEnumeration(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	_ = s.OpenPackage(context.Background(), packageTestIdentity())
	entry := packageTestEntry(1, "mimetype")
	entry.Local.Method.Value = 8
	entry.Central.Present = Evidence[bool]{State: EvidenceUnknown, Reason: "later-directory-failure"}
	entry.Central.Method = Evidence[uint16]{State: EvidenceUnknown, Reason: "unavailable"}
	entry.HeaderAssociation = Evidence[bool]{State: EvidenceUnknown, Reason: "unavailable"}
	if e := s.ObserveEntry(context.Background(), entry); e != nil {
		t.Fatal(e)
	}
	b, e := s.Finish(context.Background(), PackageCompletion{InputComplete: false})
	if e != nil {
		t.Fatal(e)
	}
	r := b.PackageReport()
	if r.Counts.Fail != 1 || r.Conformance != ConformanceViolated || r.AssessmentComplete || b.Satisfied() {
		t.Fatalf("lost local proof: %+v", r)
	}
}
func TestPackageContainerScopeNotSatisfiedBySiblingPackage(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckContainerPath)
	outer := packageTestIdentity()
	_ = s.OpenPackage(context.Background(), outer)
	inner := outer
	inner.ID = "inner"
	inner.Parent = "outer"
	inner.Pin.CanonicalID = "inner-archive"
	_ = s.OpenPackage(context.Background(), inner)
	e := packageTestEntry(1, "META-INF/container.xml")
	e.Package = "inner"
	if err := s.ObserveEntry(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	_ = s.SealEnumeration(context.Background(), completePackage("outer"))
	_ = s.SealEnumeration(context.Background(), completePackage("inner"))
	b, err := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	r := b.PackageReport()
	if r.Counts.Fail != 1 || r.Counts.Pass != 1 {
		t.Fatalf("scope leaked: %+v", r)
	}
}
func TestPackageCanceledFinishPreservesPriorFailureAndCleans(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	_ = s.OpenPackage(context.Background(), packageTestIdentity())
	entry := packageTestEntry(1, "mimetype")
	entry.Local.Method.Value = 8
	entry.Central.Method.Value = 8
	if e := s.ObserveEntry(context.Background(), entry); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b, err := s.Finish(ctx, PackageCompletion{InputComplete: false})
	if !errors.Is(err, context.Canceled) || b.Satisfied() {
		t.Fatalf("cancel=%v bundle=%+v", err, b.PackageReport())
	}
	if b.PackageReport().Counts.Fail != 1 {
		t.Fatal("early known failure lost")
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("leaked %d", used)
	}
}
