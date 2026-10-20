package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func reviewUnadoptedResource(t *testing.T) (*PackageSession, ResourceAssessmentID) {
	t.Helper()
	child := snapshotTestSession(t, "1")
	_, snapshot, err := child.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	rules, profiles := packageFixture(t, PackageCheckMimetypeStored)
	h := snapshot.Header()
	profiles[0].Intrinsic = []IntrinsicProfileBinding{{Assembly: h.Assembly.Value, Target: h.Target, Profile: h.Profile, CatalogDigest: h.CatalogDigest}}
	catalog, err := NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	s, err := catalog.begin(context.Background(), RunOptions{Context: ContextFamilyAll, DocumentID: "package", Profile: "test.package", Account: child.account})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.openPackage(context.Background(), packageTestIdentity()); err != nil {
		t.Fatal(err)
	}
	id, err := s.addResource(context.Background(), interruptionPin("unadopted"), snapshot, false)
	if err != nil {
		t.Fatal(err)
	}
	return s, id
}

func TestPackageReviewRootfileSourceIsProvenanceNotAdoptionProof(t *testing.T) {
	for _, valid := range []bool{true, false} {
		s := beginPackageFixture(t, PackageCheckFirstRootMusicXML)
		if err := s.openPackage(context.Background(), packageTestIdentity()); err != nil {
			t.Fatal(err)
		}
		source := NodeRef{"adapter-provenance", 77}
		if !valid {
			source.Node = 0
		}
		v := PackageRootfileObservation{ID: 1, Package: "outer", Source: source, Location: Location{Target: TargetPackage, DocumentID: "package", Path: "/rootfile[1]"}, Ordinal: 1, FullPath: Evidence[string]{State: EvidenceKnown, Value: "score.musicxml"}, MediaType: Evidence[string]{State: EvidenceAbsent}, Entry: Evidence[PackageFactID]{State: EvidenceUnknown, Reason: "unavailable"}, Resource: Evidence[ResourceAssessmentID]{State: EvidenceUnknown, Reason: "unavailable"}, Indirection: Evidence[bool]{State: EvidenceKnown}}
		err := s.observeRootfile(context.Background(), v)
		if valid && err != nil {
			t.Errorf("well-formed provenance refused: %v", err)
		}
		if !valid {
			var contract *ConfigurationError
			if !errors.As(err, &contract) || contract.Reason != "well-formed source reference required" {
				t.Errorf("source shape error promises unsupported occurrence proof: %v", err)
			}
		}
		b, _ := s.finish(context.Background(), PackageCompletion{})
		if b.Satisfied() {
			t.Error("source provenance certified missing resource")
		}
	}
}

func TestPackageReviewUnknownAssemblyIsConsistentlyRefused(t *testing.T) {
	for _, valid := range []bool{false, true} {
		for _, multiple := range []bool{false, true} {
			text := "bad"
			if valid {
				text = "1"
			}
			child := snapshotTestSession(t, text)
			original, snapshot, err := child.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if err != nil {
				t.Fatal(err)
			}
			snapshot.header.Assembly = Evidence[Assembly]{State: EvidenceUnknown, Reason: "profile-has-no-established-assembly"}
			rules, profiles := packageFixture(t, PackageCheckMimetypeStored)
			h := snapshot.Header()
			profiles[0].Intrinsic = []IntrinsicProfileBinding{{Assembly: AssemblyOpus, Target: h.Target, Profile: h.Profile, CatalogDigest: h.CatalogDigest}}
			if multiple {
				profiles[0].Intrinsic = append(profiles[0].Intrinsic, IntrinsicProfileBinding{Assembly: AssemblyMusicXML, Target: h.Target, Profile: h.Profile, CatalogDigest: h.CatalogDigest})
			}
			catalog, err := NewPackageCatalog(rules, profiles)
			if err != nil {
				t.Fatal(err)
			}
			s, err := catalog.begin(context.Background(), RunOptions{Context: ContextFamilyAll, DocumentID: "package", Profile: "test.package", Account: child.account})
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.addResource(context.Background(), interruptionPin("unknown-assembly"), snapshot, true)
			var invalid *ConfigurationError
			if !errors.As(err, &invalid) || invalid.Reason != "established intrinsic assembly required" {
				t.Errorf("satisfied=%t multiple=%t unknown assembly was not explicitly refused: %v", valid, multiple, err)
			}
			if len(s.resources) != 0 || snapshot.owner != nil || snapshot.closed {
				t.Error("rejected transfer changed canonical ownership")
			}
			if snapshot.header.Assembly.State != EvidenceUnknown {
				t.Error("assembly guessed from matching row")
			}
			_ = snapshot.Close()
			failureErr := s.recordResourceFailure(context.Background(), interruptionPin("unknown-assembly"), original, err)
			if !errors.Is(failureErr, ErrInvalidConfiguration) || s.failure == nil || s.failure.Counts != original.Counts || s.failure.Conformance != original.Conformance {
				t.Errorf("refused assembly lost original results: %+v error=%v", s.failure, failureErr)
			}
			_, _ = s.finish(context.Background(), PackageCompletion{})
		}
	}
}

func TestPackageReviewRootCannotAliasEarlierLinkedOccurrence(t *testing.T) {
	s, id := reviewUnadoptedResource(t)
	first, err := s.adoptResource(context.Background(), id, ResourceAdoptionOptions{Package: "outer", DocumentID: "linked-first"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.adoptResource(context.Background(), id, ResourceAdoptionOptions{Package: "outer", DocumentID: "source", Root: true})
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("root aliased an earlier linked context: %v", err)
	}
	if len(s.adoptions) != 1 || s.rootAdoption || s.resources[id].required || s.adoptions[first].expansion.owner != first {
		t.Error("rejected root changed adoption ownership or requirements")
	}
	_, _ = s.finish(context.Background(), PackageCompletion{})
}
func TestPackageReviewAliasRefusalsExplainExpansionOwnership(t *testing.T) {
	for _, kind := range []string{"edge", "association", "image"} {
		t.Run(kind, func(t *testing.T) {
			var s *PackageSession
			var err error
			switch kind {
			case "edge":
				var source ResourceAdoptionID
				s, source, _ = reviewEdgeFixture(t)
				a := s.adoptions[source]
				alias, e := s.adoptResource(context.Background(), a.resource.id, ResourceAdoptionOptions{Package: "outer", DocumentID: "alias", BaseURI: a.expansion.key.baseURI})
				if e != nil {
					t.Fatal(e)
				}
				err = s.observeEdge(context.Background(), PackageEdgeObservation{ID: 10, Source: alias})
			case "association":
				var q AssociationRequest
				s, q = packageAssociationFixture(t)
				a := s.adoptionForPin(context.Background(), q.Source, q.SourcePart.DocumentID)
				if _, e := s.adoptResource(context.Background(), a.resource.id, ResourceAdoptionOptions{Package: "outer", DocumentID: "alias"}); e != nil {
					t.Fatal(e)
				}
				q.SourcePart.DocumentID = "alias"
				q.SourceInstrument.DocumentID = "alias"
				q.PartLink.DocumentID = "alias"
				q.InstrumentLink.DocumentID = "alias"
				err = s.observeAssociation(context.Background(), q, packageAssociationReply(q, Evidence[bool]{State: EvidenceKnown, Value: true}))
			case "image":
				var source ResourceAdoptionID
				var pin ResourcePin
				s, source, pin = packageAdviceFixture(t)
				a := s.adoptions[source]
				alias, e := s.adoptResource(context.Background(), a.resource.id, ResourceAdoptionOptions{Package: "outer", DocumentID: "alias"})
				if e != nil {
					t.Fatal(e)
				}
				q := ImageMetadataRequest{ID: "image", Package: "outer", Source: pin, Subject: NodeRef{"alias", 2}, Href: "image.png", MediaType: "image/png"}
				err = s.observeImageMetadata(context.Background(), alias, 0, q, ResourceResult{RequestID: q.ID, State: EvidenceUnknown, Reason: "unavailable"})
			}
			var contract *ConfigurationError
			if !errors.As(err, &contract) || contract.Reason != "source adoption is not the expansion owner" {
				t.Errorf("alias refusal misdescribed: %v", err)
			}
			_, _ = s.finish(context.Background(), PackageCompletion{})
		})
	}
}

func TestPackageReviewFailureIngressRejectsContradictionsTransactionally(t *testing.T) {
	for _, kind := range []string{"nil-context", "conformance", "profile-target", "pin-target", "binary-pin", "count-conformance", "canonical-conflict", "already-transferred", "unrequested", "terminal-category", "terminal-reason"} {
		t.Run(kind, func(t *testing.T) {
			s, id := reviewUnadoptedResource(t)
			pin := interruptionPin("new-failure")
			report := stateClone(s.resources[id].snapshot.report)
			ctx := context.Background()
			switch kind {
			case "nil-context":
				ctx = nil
			case "conformance":
				report.Conformance = "bogus"
			case "profile-target":
				report.Profile.Target = TargetModel
			case "pin-target":
				report.Target = TargetModel
				report.Profile.Target = TargetModel
			case "binary-pin":
				pin.Representation = ResourceBinary
			case "count-conformance":
				report.Counts.Fail = 1
			case "canonical-conflict":
				pin = s.resources[id].pin
				pin.Digest = strings.Repeat("c", 64)
			case "already-transferred":
				pin = s.resources[id].pin
			case "unrequested":
				report.Requested = false
			case "terminal-category":
				report.Terminal = &TerminalSummary{Category: "bogus", Reason: "original"}
			case "terminal-reason":
				report.Terminal = &TerminalSummary{Category: CategorySyntax, Reason: Code(strings.Repeat("x", reportTerminalBytes+1))}
			}
			err := s.recordResourceFailure(ctx, pin, report, errors.New("later failure"))
			if !errors.Is(err, ErrInvalidConfiguration) {
				t.Errorf("contradictory failure admitted: %v", err)
			}
			if s.failure != nil || s.output.failureReport != nil {
				t.Error("rejected failure left summary or full report")
			}
			if len(s.resources) != 1 || s.resources[id].required {
				t.Error("rejected failure altered canonical resource")
			}
			_, _ = s.finish(context.Background(), PackageCompletion{})
		})
	}
}
func TestPackageReviewFailureIngressAfterTerminalPreservesIndependentFacts(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	child := snapshotTestSession(t, "bad")
	report, err := child.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	stop := errors.New("package already stopped")
	s.fail(stop)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before, _ := s.account.Usage(BudgetRuleSteps)
	err = s.recordResourceFailure(ctx, interruptionPin("failed-original"), report, stop)
	after, _ := s.account.Usage(BudgetRuleSteps)
	if !errors.Is(err, stop) || after != before || s.failure == nil || s.failure.Counts.Fail != report.Counts.Fail {
		t.Errorf("prepaid terminal capture lost independent facts: %v summary=%+v steps=%d", err, s.failure, after-before)
	}
	b, _ := s.finish(context.Background(), PackageCompletion{})
	if b.Satisfied() {
		t.Error("failure-only ingress certified success")
	}
}

func TestPackageReviewFailureAfterTerminalDoesNoNewRetentionWork(t *testing.T) {
	for _, exhausted := range []bool{false, true} {
		s := beginPackageFixture(t, PackageCheckMimetypeStored)
		child := snapshotTestSession(t, "bad")
		report, err := child.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
		if err != nil {
			t.Fatal(err)
		}
		var held int64
		if exhausted {
			used, _ := s.account.Usage(BudgetRuleSteps)
			if err = s.account.Charge(BudgetRuleSteps, s.account.Limits().MaxRuleSteps-used); err != nil {
				t.Fatal(err)
			}
			used, _ = s.account.Usage(BudgetBufferedBytes)
			held = s.account.Limits().MaxBufferedBytes - used
			if err = s.account.Charge(BudgetBufferedBytes, held); err != nil {
				t.Fatal(err)
			}
		}
		stop := errors.New("original terminal")
		s.fail(stop)
		before, _ := s.account.Usage(BudgetRuleSteps)
		err = s.recordResourceFailure(context.Background(), interruptionPin("failed-original"), report, stop)
		after, _ := s.account.Usage(BudgetRuleSteps)
		if !errors.Is(err, stop) || before != after || s.output.failureReport != nil {
			t.Errorf("exhausted=%t post-terminal work changed: steps %d fullReport=%t error=%v", exhausted, after-before, s.output.failureReport != nil, err)
		}
		if s.failure == nil || s.failure.Counts.Fail != 1 {
			t.Errorf("failure facts lost: %+v", s.failure)
		}
		if held != 0 {
			_ = s.account.Release(BudgetBufferedBytes, held)
		}
		_, _ = s.finish(context.Background(), PackageCompletion{})
	}
}
func TestPackageReviewInvalidFailureCannotOverwriteProvenBlocker(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	if err := s.openPackage(context.Background(), packageTestIdentity()); err != nil {
		t.Fatal(err)
	}
	entry := packageTestEntry(1, "mimetype")
	entry.Local.Method.Value = 8
	if err := s.observeEntry(context.Background(), entry); err != nil {
		t.Fatal(err)
	}
	stop := errors.New("original terminal")
	s.fail(stop)
	child := snapshotTestSession(t, "bad")
	report, err := child.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	report.Conformance = "bogus"
	if err = s.recordResourceFailure(context.Background(), interruptionPin("unadmitted"), report, stop); !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("invalid metadata accepted: %v", err)
	}
	b, err := s.finish(context.Background(), PackageCompletion{})
	if !errors.Is(err, stop) || !errors.Is(err, ErrInvalidConfiguration) || b.PackageReport().Counts.Fail != 1 || len(b.PackageReport().Findings) != 1 {
		t.Errorf("original blocker lost: %+v error=%v", b.PackageReport(), err)
	}
	if _, ok := b.Failure(); ok {
		t.Error("invalid summary stored")
	}
}

func TestPackageReviewRootOwnsFreshContextAfterDifferentLinkedContext(t *testing.T) {
	s, id := reviewUnadoptedResource(t)
	linked, err := s.adoptResource(context.Background(), id, ResourceAdoptionOptions{Package: "outer", DocumentID: "linked-first", BaseURI: "https://example.test/linked/"})
	if err != nil {
		t.Fatal(err)
	}
	root, err := s.adoptResource(context.Background(), id, ResourceAdoptionOptions{Package: "outer", DocumentID: "source", Root: true, BaseURI: "https://example.test/primary/"})
	if err != nil {
		t.Fatal(err)
	}
	if s.adoptions[root].expansion.owner != root || s.adoptions[linked].expansion.owner != linked || !s.rootAdoption {
		t.Error("different exact contexts lost independent ownership")
	}
	_, _ = s.finish(context.Background(), PackageCompletion{})
}

func TestPackageReviewTerminalFailureCaptureDoesNotScaleWithReport(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	child := snapshotTestSession(t, "bad")
	small, err := child.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil || len(small.Findings) == 0 {
		t.Fatal("missing original failure fixture", err)
	}
	large := small
	large.Findings = make([]Finding, 20000)
	for i := range large.Findings {
		large.Findings[i] = small.Findings[0]
	}
	large.Counts.Fail = uint64(len(large.Findings))
	stop := errors.New("already terminal")
	pin := interruptionPin("failed-original")
	beforeSteps, _ := s.account.Usage(BudgetRuleSteps)
	beforeBytes, _ := s.account.Usage(BudgetBufferedBytes)
	capture := func(report Report) float64 {
		return testing.AllocsPerRun(25, func() {
			s.failure = nil
			s.terminal = stop
			if err := s.recordResourceFailure(context.Background(), pin, report, stop); !errors.Is(err, stop) {
				t.Fatal(err)
			}
		})
	}
	smallAllocs, largeAllocs := capture(small), capture(large)
	afterSteps, _ := s.account.Usage(BudgetRuleSteps)
	afterBytes, _ := s.account.Usage(BudgetBufferedBytes)
	if smallAllocs != largeAllocs || afterSteps != beforeSteps || afterBytes != beforeBytes || s.output.failureReport != nil {
		t.Fatalf("post-terminal work scaled with report: allocations=%v/%v steps=%d bytes=%d full=%t", smallAllocs, largeAllocs, afterSteps-beforeSteps, afterBytes-beforeBytes, s.output.failureReport != nil)
	}
	if s.failure == nil || s.failure.Counts != large.Counts || s.failure.Conformance != large.Conformance || !s.failure.DiagnosticsTruncated || !s.failure.MetadataTruncated {
		t.Fatalf("large report independent truth lost: %+v", s.failure)
	}
	_, _ = s.finish(context.Background(), PackageCompletion{})
}
