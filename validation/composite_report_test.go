package validation

import "testing"

// Composite XSD operators have one predicate verdict but can retain several
// independently located errors. Missing assessment work is another acceptance
// axis, including when a real violation already settles that verdict.
func TestCompositeReportPreservesFailureAndIncompleteDetails(t *testing.T) {
	for _, tiny := range []bool{false, true} {
		limits := Budgets{}
		if tiny {
			limits.MaxDiagnostics, limits.MaxDiagnosticBytes = 1, 1
		}
		account, err := NewBudgetAccount(limits)
		if err != nil {
			t.Fatal(err)
		}
		acc := newReportAccumulator(reportTestProfile(), account)
		rule := reportTestRule()
		summary := reportTestDecision(OutcomeFail)
		details := []Finding{
			{Decision: summary, Location: Location{Target: TargetModel, DocumentID: "doc", Path: "/root/@first"}},
			{Decision: reportTestDecision(OutcomeFail), Location: Location{Target: TargetModel, DocumentID: "doc", Path: "/root/@second"}},
			{Decision: reportTestDecision(OutcomeUnsupported), Location: Location{Target: TargetModel, DocumentID: "doc", Path: "/root/@third"}},
		}
		if err := acc.recordCompositeOutcome(rule, summary, false); err != nil {
			t.Fatal(err)
		}
		for _, detail := range details {
			if err := acc.storeDiagnostic(rule, detail.Decision, detail.Location); err != nil {
				t.Fatal(err)
			}
		}
		report := acc.finish(true, SyntaxNotAssessed)
		if report.Counts != (Counts{Fail: 1}) || report.Conformance != ConformanceViolated || report.AssessmentComplete || report.BlockingProblems != 1 || report.Satisfied() {
			t.Fatalf("composite axes: %+v", report)
		}
		if tiny {
			if !report.DiagnosticsTruncated || len(report.Findings) != 0 {
				t.Fatalf("tiny diagnostics: %+v", report)
			}
		} else {
			if len(report.Findings) != 3 || len(report.Rules) != 1 || report.DiagnosticsTruncated {
				t.Fatalf("details: %+v", report)
			}
			details[0].Location.Path = "mutated"
			if report.Findings[0].Location.Path != "/root/@first" || report.Findings[2].Outcome != OutcomeUnsupported {
				t.Fatal("detail ownership/outcome lost")
			}
		}
	}
}

func TestCompositeReportCompleteAndUnresolvedCounts(t *testing.T) {
	for _, tc := range []struct {
		outcome     Outcome
		complete    bool
		counts      Counts
		blockers    uint64
		conformance ConformanceState
	}{
		{OutcomePass, true, Counts{Pass: 1}, 0, ConformanceConformant},
		{OutcomeNotApplicable, true, Counts{NotApplicable: 1}, 0, ConformanceConformant},
		{OutcomeFail, true, Counts{Fail: 1}, 0, ConformanceViolated},
		{OutcomeUnsupported, false, Counts{Unsupported: 1}, 1, ConformanceUndetermined},
		{OutcomeUnknown, false, Counts{Unknown: 1}, 1, ConformanceUndetermined},
		{OutcomeBlocked, false, Counts{Blocked: 1}, 1, ConformanceUndetermined},
	} {
		account, _ := NewBudgetAccount(Budgets{})
		acc := newReportAccumulator(reportTestProfile(), account)
		if err := acc.recordCompositeOutcome(reportTestRule(), reportTestDecision(tc.outcome), tc.complete); err != nil {
			t.Fatal(err)
		}
		if err := acc.storeDiagnostic(reportTestRule(), reportTestDecision(tc.outcome), Location{}); err != nil {
			t.Fatal(err)
		}
		r := acc.finish(true, SyntaxNotAssessed)
		if r.Counts != tc.counts || r.AssessmentComplete != tc.complete || r.BlockingProblems != tc.blockers || r.Conformance != tc.conformance {
			t.Fatalf("%s: %+v", tc.outcome, r)
		}
		if tc.outcome == OutcomePass || tc.outcome == OutcomeNotApplicable {
			if len(r.Findings) != 0 || !r.Satisfied() {
				t.Fatalf("successful composite: %+v", r)
			}
		} else if len(r.Findings) != 1 {
			t.Fatal("missing fallback summary finding")
		}
	}
}
