package validation

import (
	"context"
	"strings"
	"testing"
)

func observedPinFixture(t *testing.T, representation ResourceRepresentation) (*PackageSession, PackageFileMetadataObservation) {
	t.Helper()
	s, _, _ := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
	if err := s.ObserveEntry(context.Background(), packageTestEntry(10, "member.xml")); err != nil {
		t.Fatal(err)
	}
	v := PackageFileMetadataObservation{ID: 11, Package: "outer", Pin: ResourcePin{CanonicalID: "observed", Representation: representation, Digest: strings.Repeat("e", 64)}, Entry: 10, ArchiveEntry: "member.xml", Location: Location{Target: TargetPackage, DocumentID: "package", Entry: "member.xml"}, Name: Evidence[string]{State: EvidenceKnown, Value: "member.xml"}, MediaType: Evidence[string]{State: EvidenceAbsent}}
	if err := s.ObserveFileMetadata(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	return s, v
}
func TestPackageObservedFilePinsRejectConflictingMetadata(t *testing.T) {
	s, v := observedPinFixture(t, ResourceSourceXML)
	v.ID = 12
	v.Pin.Digest = strings.Repeat("f", 64)
	if err := s.ObserveFileMetadata(context.Background(), v); err == nil {
		t.Fatal("conflicting observed pin accepted")
	}
	b, _ := s.Finish(context.Background(), PackageCompletion{})
	if b.FileMetadataCount() != 1 {
		t.Fatal("retained conflicting metadata")
	}
	if n, _ := s.account.Usage(BudgetBufferedBytes); n != 0 {
		t.Fatal("leaked", n)
	}
}
func TestPackageObservedFilePinsConstrainResourcePromotion(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "same-pin", true: "conflict"}[conflict], func(t *testing.T) {
			s, v := observedPinFixture(t, ResourceSourceXML)
			resources, adoptions := len(s.resources), len(s.adoptions)
			snap, _ := interruptionSnapshot(t, s.account, "promoted", "1")
			pin := v.Pin
			if conflict {
				pin.Digest = strings.Repeat("f", 64)
			}
			id, err := s.AddResource(context.Background(), pin, snap, true)
			if conflict {
				if err == nil {
					t.Fatal("conflicting parsed resource accepted")
				}
				if snap.owner != nil {
					t.Fatal("refused transfer changed ownership")
				}
				snap.Close()
			} else {
				if err != nil || id == "" || len(s.resources) != resources+1 || len(s.adoptions) != adoptions {
					t.Fatal("same-pin promotion failed", err)
				}
			}
			s.Finish(context.Background(), PackageCompletion{})
			if n, _ := s.account.Usage(BudgetBufferedBytes); n != 0 {
				t.Fatal("leaked", n)
			}
		})
	}
}
func TestPackageObservedFilePinsConstrainNestedBoundaries(t *testing.T) {
	s, v := observedPinFixture(t, ResourceRawMXL)
	pin := v.Pin
	pin.Digest = strings.Repeat("f", 64)
	if err := s.OpenPackage(context.Background(), PackageIdentity{ID: "nested", Parent: "outer", Representation: ResourceRawMXL, Pin: pin, ArchiveEntry: "member.xml"}); err == nil {
		t.Fatal("conflicting nested boundary accepted")
	}
	s.Finish(context.Background(), PackageCompletion{})
}
func TestPackageRefusedMetadataDoesNotRegisterPin(t *testing.T) {
	s, _, _ := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
	if err := s.ObserveEntry(context.Background(), packageTestEntry(10, "member.xml")); err != nil {
		t.Fatal(err)
	}
	used, _ := s.account.Usage(BudgetBufferedBytes)
	reserve := s.account.Limits().MaxBufferedBytes - used
	if err := s.account.Charge(BudgetBufferedBytes, reserve); err != nil {
		t.Fatal(err)
	}
	pin := ResourcePin{CanonicalID: "refused", Representation: ResourceSourceXML, Digest: strings.Repeat("e", 64)}
	err := s.ObserveFileMetadata(context.Background(), PackageFileMetadataObservation{ID: 11, Package: "outer", Pin: pin, Entry: 10, ArchiveEntry: "member.xml", Location: Location{Target: TargetPackage, DocumentID: "package", Entry: "member.xml"}, Name: Evidence[string]{State: EvidenceKnown, Value: "member.xml"}, MediaType: Evidence[string]{State: EvidenceAbsent}})
	s.account.Release(BudgetBufferedBytes, reserve)
	if err == nil {
		t.Fatal("expected refusal")
	}
	if _, ok := s.pins[packageCanonicalKey{pin.CanonicalID, pin.Revision, pin.Representation}]; ok {
		t.Fatal("refusal pinned content")
	}
	s.Finish(context.Background(), PackageCompletion{})
}

func TestPackageExistingResourcePinRejectsConflictingFileObservation(t *testing.T) {
	s, _, pin := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
	if err := s.ObserveEntry(context.Background(), packageTestEntry(10, "member.xml")); err != nil {
		t.Fatal(err)
	}
	pin.Digest = strings.Repeat("f", 64)
	err := s.ObserveFileMetadata(context.Background(), PackageFileMetadataObservation{ID: 11, Package: "outer", Pin: pin, Entry: 10, ArchiveEntry: "member.xml", Location: Location{Target: TargetPackage, DocumentID: "package", Entry: "member.xml"}, Name: Evidence[string]{State: EvidenceKnown, Value: "member.xml"}, MediaType: Evidence[string]{State: EvidenceAbsent}})
	if err == nil {
		t.Fatal("observed file bypassed prior resource content pin")
	}
	s.Finish(context.Background(), PackageCompletion{})
}
