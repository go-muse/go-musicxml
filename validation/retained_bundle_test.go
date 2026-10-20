package validation

import (
	"context"
	"errors"
	"testing"
)

// retainedBundleFixture builds a package session on a, with one required
// adopted resource whose snapshot was charged to a.
func retainedBundleFixture(t *testing.T, a *Account) *PackageSession {
	t.Helper()
	part, _ := interruptionSnapshot(t, a, "part", "1")
	rules, profiles := packageFixture(t, PackageCheckMimetypeStored)
	h := part.Header()
	profiles[0].Intrinsic = []IntrinsicProfileBinding{{Assembly: h.Assembly.Value, Target: h.Target, Profile: h.Profile, CatalogDigest: h.CatalogDigest}}
	catalog, err := NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	s, err := catalog.begin(context.Background(), RunOptions{Context: ContextFamilyAll, DocumentID: "package", Profile: "test.package", Account: a})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.openPackage(context.Background(), packageTestIdentity()); err != nil {
		t.Fatal(err)
	}
	id, err := s.addResource(context.Background(), interruptionPin("part"), part, true)
	if err != nil {
		t.Fatal(err)
	}
	interruptionAdopt(t, s, id, "part", true, "")
	return s
}

func TestFinishWithRetainedBundleChargesMeasuredStorage(t *testing.T) {
	a, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	s := retainedBundleFixture(t, a)
	b, lease, err := s.finishWithRetainedBundle(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil || lease == nil || b.ResourceCount() != 1 {
		t.Fatalf("lease=%v resources=%d err=%v", lease, b.ResourceCount(), err)
	}
	reference, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := bundleRetainedBytes(context.Background(), reference, b.data)
	if err != nil {
		t.Fatal(err)
	}
	// The package and resource reports keep several kilobytes.
	if want < 1024 {
		t.Fatalf("measured only %d bytes", want)
	}
	if used, _ := a.Usage(BudgetBufferedBytes); used != want+reportLeaseControlBytes {
		t.Fatalf("live charge=%d want bundle %d + control %d", used, want, reportLeaseControlBytes)
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("closed lease left %d bytes", used)
	}
}

func TestFinishWithRetainedBundleRefusalHasNoLease(t *testing.T) {
	// The measurement charges rule steps after Finish, so a step budget can
	// admit Finish and refuse the retention. Find the smallest remaining
	// step budget that retains; one step less is refused.
	run := func(remaining int64) (AssessmentBundle, *ReportLease, error, *Account) {
		a, err := NewBudgetAccount(Budgets{MaxRuleSteps: 1 << 40})
		if err != nil {
			t.Fatal(err)
		}
		s := retainedBundleFixture(t, a)
		used, _ := a.Usage(BudgetRuleSteps)
		if err = a.Charge(BudgetRuleSteps, 1<<40-used-remaining); err != nil {
			t.Fatal(err)
		}
		b, lease, err := s.finishWithRetainedBundle(context.Background(), PackageCompletion{InputComplete: true})
		return b, lease, err, a
	}
	low, high := int64(0), int64(1<<20)
	if _, lease, err, _ := run(high); err != nil || lease == nil {
		t.Fatalf("generous budget: lease=%v err=%v", lease, err)
	}
	for low+1 < high {
		mid := (low + high) / 2
		if _, lease, err, _ := run(mid); err == nil && lease != nil {
			high = mid
		} else {
			low = mid
		}
	}
	finished, _ := beginRetainedFinishOnly(t, high)
	b, lease, err, a := run(high - 1)
	var resource *ResourceError
	if !errors.As(err, &resource) || resource.Kind != BudgetRuleSteps || lease != nil {
		t.Fatalf("lease=%v err=%v", lease, err)
	}
	if b.ResourceCount() != finished || b.PackageReport().Terminal != nil {
		t.Fatalf("refused retention lost the bundle: resources=%d want %d terminal=%v", b.ResourceCount(), finished, b.PackageReport().Terminal)
	}
	if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("refused retention left %d bytes", used)
	}
}

// beginRetainedFinishOnly shows that plain Finish fits in remaining steps
// minus one, so the refusal above comes from the retention alone.
func beginRetainedFinishOnly(t *testing.T, remaining int64) (int, error) {
	t.Helper()
	a, err := NewBudgetAccount(Budgets{MaxRuleSteps: 1 << 40})
	if err != nil {
		t.Fatal(err)
	}
	s := retainedBundleFixture(t, a)
	used, _ := a.Usage(BudgetRuleSteps)
	if err = a.Charge(BudgetRuleSteps, 1<<40-used-(remaining-1)); err != nil {
		t.Fatal(err)
	}
	b, err := s.finish(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil || b.PackageReport().Terminal != nil {
		t.Fatalf("plain finish needs the last step: terminal=%v err=%v", b.PackageReport().Terminal, err)
	}
	return b.ResourceCount(), err
}

func TestAssessmentBundleFailures(t *testing.T) {
	a, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	_, ok := interruptionSnapshot(t, a, "part", "1")
	if !ok.Satisfied() {
		t.Fatalf("fixture report not satisfied: %+v", ok)
	}
	failed := Finding{Decision: Decision{Outcome: OutcomeFail, Reason: "bad", Related: []Location{{Path: "/a"}}}, Category: CategoryNormative}
	violated := ok
	violated.Conformance, violated.Counts.Fail, violated.Findings = ConformanceViolated, 1, []Finding{failed}
	truncated := violated
	truncated.Findings, truncated.DiagnosticsTruncated = nil, true
	undetermined := ok
	undetermined.Conformance = ConformanceUndetermined
	violatedIncomplete := violated
	violatedIncomplete.AssessmentComplete = false
	okTruncated := ok
	okTruncated.DiagnosticsTruncated = true
	bundle := func(report Report, resources ...bundleResource) AssessmentBundle {
		return AssessmentBundle{data: &assessmentBundleData{report: report, resources: resources, enumeration: true}}
	}
	required := func(r Report) bundleResource { return bundleResource{id: "r", report: r, required: true} }
	tests := []struct {
		name   string
		bundle AssessmentBundle
		want   BundleFailures
	}{
		{"satisfied", bundle(ok, required(ok)), BundleFailures{}},
		{"zero bundle", AssessmentBundle{}, BundleFailures{Incomplete: true}},
		{"violation", bundle(ok, required(violated)), BundleFailures{Violated: true, Failed: 1, Findings: []Finding{failed}}},
		{"truncated violation", bundle(ok, required(truncated)), BundleFailures{Violated: true, Failed: 1, Truncated: true}},
		{"undetermined", bundle(undetermined, required(ok)), BundleFailures{Incomplete: true}},
		{"violation and incompleteness", bundle(ok, required(violatedIncomplete)), BundleFailures{Violated: true, Incomplete: true, Failed: 1, Findings: []Finding{failed}}},
		{"truncation outside the violation", bundle(okTruncated, required(violated), required(undetermined)), BundleFailures{Violated: true, Incomplete: true, Failed: 1, Findings: []Finding{failed}}},
		{"optional resource", bundle(undetermined, bundleResource{id: "r", report: violated}), BundleFailures{Incomplete: true}},
	}
	for _, tc := range tests {
		got := tc.bundle.Failures()
		if got.Violated != tc.want.Violated || got.Incomplete != tc.want.Incomplete || got.Failed != tc.want.Failed || got.Truncated != tc.want.Truncated || len(got.Findings) != len(tc.want.Findings) {
			t.Errorf("%s: got %+v want %+v", tc.name, got, tc.want)
		}
	}

	open := bundle(ok, required(ok))
	open.data.enumeration = false
	if got := open.Failures(); got.Violated || !got.Incomplete {
		t.Errorf("incomplete enumeration: %+v", got)
	}
	summary := bundle(ok, required(ok))
	summary.data.failure = &ResourceFailureSummary{Conformance: ConformanceViolated, Counts: Counts{Fail: 2}, DiagnosticsTruncated: true}
	if got := summary.Failures(); !got.Violated || !got.Incomplete || got.Failed != 2 || !got.Truncated {
		t.Errorf("violated failure summary: %+v", got)
	}
	summary.data.failure = &ResourceFailureSummary{Conformance: ConformanceUndetermined}
	if got := summary.Failures(); got.Violated || !got.Incomplete {
		t.Errorf("undetermined failure summary: %+v", got)
	}
	// A retained failed report replaces the summary's counts with its own
	// diagnostics.
	retained := violated
	summary.data.failure = &ResourceFailureSummary{Conformance: ConformanceViolated, Counts: Counts{Fail: 1}}
	summary.data.failureReport = &retained
	if got := summary.Failures(); !got.Violated || !got.Incomplete || got.Failed != 1 || got.Truncated || len(got.Findings) != 1 {
		t.Errorf("retained failed report: %+v", got)
	}
	retained.Findings, retained.DiagnosticsTruncated = nil, true
	if got := summary.Failures(); !got.Violated || got.Failed != 1 || !got.Truncated || len(got.Findings) != 0 {
		t.Errorf("retained truncated failed report: %+v", got)
	}
	// Findings are deep copies.
	got := bundle(ok, required(violated)).Failures()
	got.Findings[0].Related[0].Path = "/changed"
	if violated.Findings[0].Related[0].Path != "/a" {
		t.Error("Failures shares findings with the bundle")
	}
}
