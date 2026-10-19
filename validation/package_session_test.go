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
	s, e := c.begin(context.Background(), RunOptions{DocumentID: "package", Profile: "test.package"})
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
		if e := s.openPackage(context.Background(), packageTestIdentity()); e != nil {
			t.Fatal(e)
		}
		if complete {
			if e := s.sealEnumeration(context.Background(), completePackage("outer")); e != nil {
				t.Fatal(e)
			}
		}
		b, e := s.finish(context.Background(), PackageCompletion{InputComplete: true})
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
	_ = s.openPackage(context.Background(), packageTestIdentity())
	entry := packageTestEntry(1, "mimetype")
	entry.Local.Method.Value = 8
	entry.Central.Present = Evidence[bool]{State: EvidenceUnknown, Reason: "later-directory-failure"}
	entry.Central.Method = Evidence[uint16]{State: EvidenceUnknown, Reason: "unavailable"}
	entry.HeaderAssociation = Evidence[bool]{State: EvidenceUnknown, Reason: "unavailable"}
	if e := s.observeEntry(context.Background(), entry); e != nil {
		t.Fatal(e)
	}
	b, e := s.finish(context.Background(), PackageCompletion{InputComplete: false})
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
	_ = s.openPackage(context.Background(), outer)
	inner := outer
	inner.ID = "inner"
	inner.Parent = "outer"
	inner.Pin.CanonicalID = "inner-archive"
	_ = s.openPackage(context.Background(), inner)
	e := packageTestEntry(1, "META-INF/container.xml")
	e.Package = "inner"
	if err := s.observeEntry(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	_ = s.sealEnumeration(context.Background(), completePackage("outer"))
	_ = s.sealEnumeration(context.Background(), completePackage("inner"))
	b, err := s.finish(context.Background(), PackageCompletion{InputComplete: true})
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
	_ = s.openPackage(context.Background(), packageTestIdentity())
	entry := packageTestEntry(1, "mimetype")
	entry.Local.Method.Value = 8
	entry.Central.Method.Value = 8
	if e := s.observeEntry(context.Background(), entry); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b, err := s.finish(ctx, PackageCompletion{InputComplete: false})
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

func TestPackageFailureSummaryUnpinnedAndWriteOnce(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	child := snapshotTestSession(t, "bad")
	report, snap, err := child.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	pin := ResourcePin{CanonicalID: "failed-input", Representation: ResourceSourceXML}
	if err = s.recordResourceFailure(context.Background(), pin, report, context.Canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("unpinned failure=%v", err)
	}
	later := report
	later.Counts = Counts{}
	later.Conformance = ConformanceConformant
	if err = s.recordResourceFailure(context.Background(), pin, later, context.Canceled); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("repeat summary=%v", err)
	}
	b, _ := s.finish(context.Background(), PackageCompletion{})
	failure, ok := b.Failure()
	if !ok || failure.Counts.Fail != 1 || failure.PinComplete.State == EvidenceKnown || failure.Pin.Digest != "" {
		t.Fatalf("failure=%+v", failure)
	}
}

func TestPackageUnassociatedHeadersCannotProveNoExtra(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeNoExtra)
	if err := s.openPackage(context.Background(), packageTestIdentity()); err != nil {
		t.Fatal(err)
	}
	entry := packageTestEntry(1, "mimetype")
	entry.HeaderAssociation = Evidence[bool]{State: EvidenceKnown, Value: false}
	err := s.observeEntry(context.Background(), entry)
	if err != nil {
		_, _ = s.finish(context.Background(), PackageCompletion{})
		return
	}
	_ = s.sealEnumeration(context.Background(), completePackage("outer"))
	b, _ := s.finish(context.Background(), PackageCompletion{InputComplete: true})
	if b.Satisfied() || b.PackageReport().Counts.Pass != 0 {
		t.Fatalf("unassociated headers proved no-extra: %+v", b.PackageReport())
	}
}

func TestPackageDocumentDependencyUsesOwningBoundary(t *testing.T) {
	r, p := packageFixture(t, PackageCheckContainerPath, PackageCheckMimetypeStored)
	r[1].Rule.Requires = []Dependency{{Kind: DependencyRule, Rule: r[0].Rule.Key, Relation: RelationDocument}}
	c, err := NewPackageCatalog(r, p)
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
	if err = s.observeEntry(context.Background(), packageTestEntry(1, "META-INF/container.xml")); err != nil {
		t.Fatal(err)
	}
	if err = s.observeEntry(context.Background(), packageTestEntry(2, "mimetype")); err != nil {
		t.Fatal(err)
	}
	_ = s.sealEnumeration(context.Background(), completePackage("outer"))
	b, _ := s.finish(context.Background(), PackageCompletion{InputComplete: true})
	if !b.Satisfied() {
		t.Fatalf("passed boundary prerequisite not found by entry: %+v", b.PackageReport())
	}
}

func TestPackageUnpinnedRawPrefixPreservesFailureWithoutCertification(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	p := packageTestIdentity()
	p.PinUnavailable = true
	p.Pin.Digest = ""
	if err := s.openPackage(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	e := packageTestEntry(1, "mimetype")
	e.Local.Method.Value = 8
	if err := s.observeEntry(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if err := s.sealEnumeration(context.Background(), completePackage("outer")); err != nil {
		t.Fatal(err)
	}
	b, err := s.finish(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if b.Satisfied() || b.EnumerationComplete() || b.PackageReport().Counts.Fail != 1 || b.PackageReport().AssessmentComplete {
		t.Fatalf("partial pin certified or failure lost: %+v", b.PackageReport())
	}
}
func TestPackageCurrentRootfileHasNoInventedXMLSource(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckFirstRootMusicXML)
	p := packageTestIdentity()
	p.Representation = ResourceCurrentMXL
	p.Pin.Representation = ResourceCurrentMXL
	p.Pin.Digest = ""
	p.Pin.SnapshotID = "current-run"
	if err := s.openPackage(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	r := PackageRootfileObservation{ID: 1, Package: "outer", Location: Location{Target: TargetPackage, DocumentID: "package", Path: "/rootfiles[1]"}, Ordinal: 1, FullPath: Evidence[string]{State: EvidenceKnown, Value: "score.xml"}, MediaType: Evidence[string]{State: EvidenceAbsent}, Entry: Evidence[PackageFactID]{State: EvidenceUnknown, Reason: "unresolved"}, Resource: Evidence[ResourceAssessmentID]{State: EvidenceUnknown, Reason: "unresolved"}, Indirection: Evidence[bool]{State: EvidenceKnown, Value: false}}
	if err := s.observeRootfile(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	_, _ = s.finish(context.Background(), PackageCompletion{})
}

func TestPackageCurrentContainerExcludesOriginalEntryClaim(t *testing.T) {
	for _, current := range []bool{false, true} {
		s := beginPackageFixture(t, PackageCheckContainerPath)
		p := packageTestIdentity()
		if current {
			p.Representation = ResourceCurrentMXL
			p.Pin.Representation = ResourceCurrentMXL
			p.Pin.Digest = ""
			p.Pin.SnapshotID = "run"
		}
		if err := s.openPackage(context.Background(), p); err != nil {
			t.Fatal(err)
		}
		if err := s.sealEnumeration(context.Background(), completePackage("outer")); err != nil {
			t.Fatal(err)
		}
		b, err := s.finish(context.Background(), PackageCompletion{InputComplete: true})
		if err != nil {
			t.Fatal(err)
		}
		if current {
			if !b.Satisfied() || b.PackageReport().Counts.NotApplicable != 1 {
				t.Fatalf("current original claim: %+v", b.PackageReport())
			}
		} else if b.PackageReport().Counts.Fail != 1 || b.Satisfied() {
			t.Fatal("raw obligation lost")
		}
	}
}

func TestPackageFailureReportOwnedWhenAffordableAndSummaryOnRefusal(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		s := beginPackageFixture(t, PackageCheckMimetypeStored)
		child := snapshotTestSession(t, "bad")
		report, snapshot, err := child.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
		if err != nil {
			t.Fatal(err)
		}
		_ = snapshot.Close()
		want := report.Findings[0].Reason
		var hold int64
		if refuse {
			used, _ := s.account.Usage(BudgetBufferedBytes)
			hold = s.account.Limits().MaxBufferedBytes - used
			if err = s.account.Charge(BudgetBufferedBytes, hold); err != nil {
				t.Fatal(err)
			}
		}
		stop := errors.New("intrinsic-operation-stop")
		if err = s.recordResourceFailure(context.Background(), ResourcePin{CanonicalID: "unread-source", Representation: ResourceSourceXML}, report, stop); !errors.Is(err, stop) {
			t.Fatal(err)
		}
		report.Findings[0].Reason = "mutated-input"
		bundle, err := s.finish(context.Background(), PackageCompletion{})
		if !errors.Is(err, stop) {
			t.Fatal(err)
		}
		full, ok := bundle.FailureReport()
		summary, present := bundle.Failure()
		if !present || summary.Counts.Fail != 1 || bundle.Satisfied() {
			t.Fatal("failure truth lost")
		}
		if refuse {
			if ok || !summary.DiagnosticsTruncated || !summary.MetadataTruncated {
				t.Fatal("refused full report represented as complete")
			}
			_ = s.account.Release(BudgetBufferedBytes, hold)
		} else {
			if !ok || full.Findings[0].Reason != want || summary.DiagnosticsTruncated || summary.MetadataTruncated {
				t.Fatalf("affordable best report lost: %t %+v", ok, summary)
			}
			full.Findings[0].Reason = "mutated-getter"
			again, _ := bundle.FailureReport()
			if again.Findings[0].Reason != want {
				t.Fatal("failure getter aliases backing report")
			}
		}
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("refuse=%t leaked=%d", refuse, used)
		}
	}
}
