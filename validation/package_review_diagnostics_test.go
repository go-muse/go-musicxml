package validation

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestPackageReviewFindingSurvivesDecisionCacheRefusal(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	r := s.catalog.rules[s.catalog.orders[s.profile.ID][0]].Rule
	before, _ := s.account.Usage(BudgetBufferedBytes)
	fill := s.account.Limits().MaxBufferedBytes - before
	if err := s.account.Charge(BudgetBufferedBytes, fill); err != nil {
		t.Fatal(err)
	}
	d := s.baseDecision(r.Key, 1)
	d.Outcome = OutcomeFail
	d.Reason = "observed-compressed"
	err := s.record(context.Background(), r, 1, d, s.location("outer"), true)
	if !errors.Is(err, ErrResourceLimit) {
		t.Errorf("err=%v", err)
	}
	_ = s.account.Release(BudgetBufferedBytes, fill)
	b, _ := s.finish(context.Background(), PackageCompletion{})
	report := b.PackageReport()
	if report.Counts.Fail != 1 || len(report.Findings) != 1 || report.Findings[0].Reason != d.Reason {
		t.Errorf("authoritative finding lost at cache admission: %+v", report)
	}
}
func TestPackageReviewFailurePreservesOriginalTerminal(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	child := snapshotTestSession(t, "1")
	report, err := child.Finish(context.Background(), Completion{InputComplete: false, Syntax: SyntaxMalformed, Cause: errors.New("malformed child")})
	if err == nil || report.Terminal == nil || report.Terminal.Category != CategorySyntax {
		t.Fatalf("bad fixture: %+v %v", report, err)
	}
	if err := s.recordResourceFailure(context.Background(), interruptionPin("malformed"), report, errors.New("snapshot unavailable")); err == nil {
		t.Error("failure ingress lost error")
	}
	b, _ := s.finish(context.Background(), PackageCompletion{})
	summary, ok := b.Failure()
	if !ok || summary.Terminal != *report.Terminal {
		t.Errorf("summary=%+v original=%+v", summary, report.Terminal)
	}
}
func TestPackageReviewTerminalCauseIsReadOnly(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	reader, ok := any(s).(interface{ terminalCause() error })
	if !ok {
		_, _ = s.finish(context.Background(), PackageCompletion{})
		t.Fatal("missing read-only terminal cause accessor")
	}
	if reader.terminalCause() != nil {
		t.Error("healthy session has terminal cause")
	}
	stop := errors.New("original stop")
	s.fail(stop)
	used, _ := s.account.Usage(BudgetRuleSteps)
	if !errors.Is(reader.terminalCause(), stop) {
		t.Error("terminal accessor lost original cause")
	}
	after, _ := s.account.Usage(BudgetRuleSteps)
	if after != used {
		t.Error("read-only accessor charged steps")
	}
	if err := s.openPackage(context.Background(), packageTestIdentity()); !errors.Is(err, stop) {
		t.Errorf("ordinary event after terminal=%v", err)
	}
	_, _ = s.finish(context.Background(), PackageCompletion{})
	if !errors.Is(reader.terminalCause(), stop) {
		t.Error("finish lost original cause")
	}
}
func TestPackageReviewChildDiagnosticsCannotHidePackageBlocker(t *testing.T) {
	account, _ := NewBudgetAccount(Budgets{MaxDiagnostics: 1})
	rules, profiles := packageFixture(t, PackageCheckMimetypeStored)
	catalog, err := NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	s, err := catalog.begin(context.Background(), RunOptions{Context: ContextFamilyAll, DocumentID: "package", Profile: "test.package", Account: account})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, child := interruptionSnapshot(t, account, "child", "invalid")
	if len(child.Findings) != 1 {
		t.Fatalf("child did not consume shared diagnostic: %+v", child)
	}
	if err := snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	r := rules[0].Rule
	for _, id := range []PackageFactID{7, 8} {
		s.subjectPackages[id] = "outer"
		d := s.baseDecision(r.Key, id)
		d.Outcome = OutcomeFail
		d.Reason = "original-blocking-proof"
		if err := s.record(context.Background(), r, id, d, s.location("outer"), true); err != nil {
			t.Fatal(err)
		}
	}
	b, err := s.finish(context.Background(), PackageCompletion{})
	if err != nil {
		t.Fatal(err)
	}
	report := b.PackageReport()
	if report.Counts.Fail != 2 || len(report.Findings) != 0 || !report.DiagnosticsTruncated {
		t.Errorf("unexpected authoritative report: %+v", report)
	}
	method := reflect.ValueOf(b).MethodByName("OmittedPackageDiagnostics")
	if !method.IsValid() {
		t.Fatal("shared child quota hid package blocking evidence without an independent summary")
	}
	out := method.Call(nil)
	if !out[1].Bool() {
		t.Fatal("missing omitted package summary")
	}
	summary := out[0]
	if summary.FieldByName("Omitted").Uint() != 2 || summary.FieldByName("Subject").Interface() != (NodeRef{"package", 7}) || summary.FieldByName("Package").String() != "outer" || summary.FieldByName("Reason").String() != "original-blocking-proof" {
		t.Errorf("summary=%+v", summary.Interface())
	}
	if used, _ := account.Usage(BudgetDiagnostics); used != 1 {
		t.Errorf("summary bypassed hard diagnostic quota: %d", used)
	}
	if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
		t.Errorf("leaked %d bytes", used)
	}
}

func TestPackageDiagnosticSummaryCaptureIsPrepaidAndDeterministic(t *testing.T) {
	rules, _ := packageFixture(t, PackageCheckMimetypeStored)
	rule := rules[0].Rule
	first := Decision{Rule: rule.Key, Subject: NodeRef{"original-source", 9}, Outcome: OutcomeBlocked, Reason: "original-prerequisite", Cause: &DecisionRef{Rule: RuleKey{ID: "original-cause", Revision: 1}, Subject: NodeRef{"original-child", 3}}}
	// Exhaust both live scratch and shared diagnostic quotas after Begin has
	// already allocated and charged the fixed reserve.
	session := beginPackageFixture(t, PackageCheckMimetypeStored)
	summary := session.output.omittedDiagnostics
	before, _ := session.account.Usage(BudgetBufferedBytes)
	fill := session.account.Limits().MaxBufferedBytes - before
	if err := session.account.Charge(BudgetBufferedBytes, fill); err != nil {
		t.Fatal(err)
	}
	if err := session.account.Charge(BudgetDiagnostics, session.account.Limits().MaxDiagnostics); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = session.account.Release(BudgetBufferedBytes, fill)
		_, _ = session.finish(context.Background(), PackageCompletion{})
	}()
	if allocations := testing.AllocsPerRun(1000, func() { summary.omitted = 0; summary.capture(rule, first, "original-package") }); allocations != 0 {
		t.Errorf("capture allocates after exhaustion: %g", allocations)
	}
	later := first
	later.Subject = NodeRef{"later-source", 10}
	later.Reason = "later-proof"
	summary.capture(rule, later, "later-package")
	b := AssessmentBundle{data: &assessmentBundleData{omittedDiagnostics: summary}}
	got, ok := b.OmittedPackageDiagnostics()
	if !ok || got.Omitted != 2 || got.Rule != first.Rule || got.Subject != first.Subject || got.Package != "original-package" || got.Reason != first.Reason || got.Outcome != first.Outcome || got.Category != rule.Category {
		t.Fatalf("first observed identity changed: %+v", got)
	}
	got.Subject.DocumentID = "mutated"
	again, _ := b.OmittedPackageDiagnostics()
	if again.Subject != first.Subject {
		t.Error("getter aliases summary")
	}
	summary.omitted = ^uint64(0) - 1
	summary.capture(rule, later, "later-package")
	summary.capture(rule, later, "later-package")
	got, _ = b.OmittedPackageDiagnostics()
	if got.Omitted != ^uint64(0) || got.Subject != first.Subject {
		t.Errorf("saturation changed summary: %+v", got)
	}
}
func TestPackageDiagnosticSummaryDoesNotCountOptionalOrSuccessfulOutcomes(t *testing.T) {
	rules, _ := packageFixture(t, PackageCheckMimetypeStored)
	rule := rules[0].Rule
	var summary packageDiagnosticSummaryData
	for _, outcome := range []Outcome{OutcomePass, OutcomeNotApplicable} {
		summary.capture(rule, Decision{Outcome: outcome}, "")
	}
	rule.Category = CategoryAdvisory
	summary.capture(rule, Decision{Outcome: OutcomeFail}, "")
	if summary.omitted != 0 {
		t.Errorf("nonblocking omissions counted: %d", summary.omitted)
	}
}
