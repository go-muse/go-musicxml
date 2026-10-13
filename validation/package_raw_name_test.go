package validation

import (
	"context"
	"testing"
)

func TestPackageRawNameOwnedObservationOnly(t *testing.T) {
	s, _, _ := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
	raw := []byte{'x', 0x82}
	e := packageTestEntry(10, "")
	e.RawName = raw
	e.UTF8Name = Evidence[bool]{State: EvidenceKnown}
	e.Location.Path = "raw-entry-name-hex[7882]"
	if err := s.ObserveEntry(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	raw[0] = 'y'
	if s.entries[10].RawName[0] != 'x' {
		t.Fatal("raw name aliases caller")
	}
	if len(s.entryNames["outer"]) != 0 || len(s.rootfiles) != 0 || len(s.roles) != 0 {
		t.Fatal("raw name granted path/root/role capability")
	}
	duplicate := e
	duplicate.ID = 11
	duplicate.RawName = []byte{'x', 0x82}
	if err := s.ObserveEntry(context.Background(), duplicate); err == nil {
		t.Fatal("duplicate raw byte identity accepted")
	}
	_, _ = s.Finish(context.Background(), PackageCompletion{})
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatal(used)
	}
}
func TestPackageRawNameRejectsTextAndCurrentClaims(t *testing.T) {
	for _, mode := range []string{"text", "valid-utf8", "wrong-evidence", "current"} {
		s, _, _ := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
		e := packageTestEntry(10, "")
		e.RawName = []byte{'x', 0x82}
		e.UTF8Name = Evidence[bool]{State: EvidenceKnown}
		switch mode {
		case "text":
			e.Name = "invented.xml"
		case "valid-utf8":
			e.RawName = []byte("valid.xml")
		case "wrong-evidence":
			e.UTF8Name.Value = true
		case "current":
			s.boundaries["outer"].identity.Representation = ResourceCurrentMXL
		}
		if err := s.ObserveEntry(context.Background(), e); err == nil {
			t.Fatal("accepted", mode)
		}
		_, _ = s.Finish(context.Background(), PackageCompletion{})
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatal(used)
		}
	}
}
func TestPackageRawNameCannotCompleteRoleUniverse(t *testing.T) {
	s, _, pin := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
	e := packageTestEntry(10, "")
	e.RawName = []byte{0x82}
	e.UTF8Name = Evidence[bool]{State: EvidenceKnown}
	if err := s.ObserveEntry(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	b := s.boundaries["outer"]
	b.sealed = true
	b.enumeration = completePackage("outer")
	roles := ResourceRoleResult{Request: ResourceRoleRequest{Resources: []ResourcePin{pin}, Members: []ResourceRoleMember{{Resource: pin, ArchiveEntry: "", BaseURI: Evidence[string]{State: EvidenceAbsent}}}}}
	if s.roleUniverseComplete(context.Background(), b, roles) {
		t.Fatal("empty textual name covered raw byte entry")
	}
	_, _ = s.Finish(context.Background(), PackageCompletion{})
}
func TestPackageRawNameCannotBindRootfile(t *testing.T) {
	s, _, _ := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
	e := packageTestEntry(10, "")
	e.RawName = []byte{0x82}
	e.UTF8Name = Evidence[bool]{State: EvidenceKnown}
	if err := s.ObserveEntry(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	r := PackageRootfileObservation{ID: 11, Package: "outer", Source: NodeRef{DocumentID: "source", Node: 1}, Location: Location{Target: TargetPackage, DocumentID: "package"}, Ordinal: 1, FullPath: Evidence[string]{State: EvidenceKnown}, MediaType: Evidence[string]{State: EvidenceAbsent}, Entry: Evidence[PackageFactID]{State: EvidenceKnown, Value: 10}, Resource: Evidence[ResourceAssessmentID]{State: EvidenceUnknown, Reason: "unavailable"}, Indirection: Evidence[bool]{State: EvidenceKnown}}
	if err := s.ObserveRootfile(context.Background(), r); err == nil {
		t.Fatal("raw byte entry became rootfile path")
	}
	_, _ = s.Finish(context.Background(), PackageCompletion{})
}
