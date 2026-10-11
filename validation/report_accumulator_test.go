package validation

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func reportTestProfile() Profile {
	return Profile{ID: "report-test", Target: TargetModel, Required: []RuleKey{{ID: "report.rule", Revision: 1}}}
}

func reportTestRule() RuleDefinition {
	return RuleDefinition{Key: RuleKey{ID: "report.rule", Revision: 1}, Category: CategoryNormative, Class: ClassMandatory, Role: RolePredicate,
		Targets: []Target{TargetModel}, Scope: ScopeNode, Predicate: "test",
		Selector: Selector{Component: &ComponentID{Assembly: AssemblyMusicXML, Name: QName{Local: "test"}, Kind: ComponentType}},
		Text:     RuleText{Title: "Required test", Explanation: "A required test must succeed.", Message: "Test failed"},
		Sources:  []SourceRef{{IDs: []string{"source"}, URL: "https://example.org/spec", Locator: "section"}}}
}

func reportTestDecision(outcome Outcome) Decision {
	return Decision{Rule: reportTestRule().Key, Subject: NodeRef{DocumentID: "doc", Node: 1}, Outcome: outcome, Reason: "test", Expected: "expected", Actual: "actual"}
}

func TestReportAccumulatorIndependentAxesAndTruncation(t *testing.T) {
	tests := []struct {
		name                string
		outcomes            []Outcome
		conformance         ConformanceState
		complete, satisfied bool
		counts              Counts
	}{
		{"pass", []Outcome{OutcomePass}, ConformanceConformant, true, true, Counts{Pass: 1}},
		{"excluded", []Outcome{OutcomeNotApplicable}, ConformanceConformant, true, true, Counts{NotApplicable: 1}},
		{"fail", []Outcome{OutcomeFail}, ConformanceViolated, true, false, Counts{Fail: 1}},
		{"unknown", []Outcome{OutcomeUnknown}, ConformanceUndetermined, false, false, Counts{Unknown: 1}},
		{"unsupported", []Outcome{OutcomeUnsupported}, ConformanceUndetermined, false, false, Counts{Unsupported: 1}},
		{"blocked", []Outcome{OutcomeBlocked}, ConformanceUndetermined, false, false, Counts{Blocked: 1}},
		{"fail and unknown", []Outcome{OutcomeFail, OutcomeUnknown}, ConformanceViolated, false, false, Counts{Fail: 1, Unknown: 1}},
	}
	for _, tc := range tests {
		for _, tiny := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/retained", true: "/truncated"}[tiny], func(t *testing.T) {
				limits := Budgets{}
				if tiny {
					limits.MaxDiagnostics, limits.MaxDiagnosticBytes = 1, 1
				}
				account, _ := NewBudgetAccount(limits)
				acc := newReportAccumulator(reportTestProfile(), account)
				for _, outcome := range tc.outcomes {
					if err := acc.record(reportTestRule(), reportTestDecision(outcome), Location{Target: TargetModel, DocumentID: "doc", Path: "/root"}); err != nil {
						t.Fatal(err)
					}
				}
				r := acc.finish(true, SyntaxNotAssessed)
				if r.Conformance != tc.conformance || r.AssessmentComplete != tc.complete || r.Satisfied() != tc.satisfied || r.Counts != tc.counts || r.BlockingProblems != 0 {
					t.Fatalf("report axes = %+v", r)
				}
				if r.DiagnosticsTruncated != tiny {
					t.Fatalf("truncation = %v", r.DiagnosticsTruncated)
				}
				if tiny && (len(r.Findings) != 0 || len(r.Rules) != 0) {
					t.Fatal("oversized diagnostic retained")
				}
			})
		}
	}
}

func TestReportAccumulatorNonNormativeClassesAndProviders(t *testing.T) {
	for _, tc := range []struct {
		name     string
		category DiagnosticCategory
		class    RuleClass
		role     RuleRole
		outcome  Outcome
		complete bool
		blockers uint64
	}{
		{"model failure", CategoryModel, ClassMandatory, RolePredicate, OutcomeFail, true, 1},
		{"policy failure", CategoryPolicy, ClassPolicy, RolePredicate, OutcomeFail, true, 1},
		{"model unknown", CategoryModel, ClassMandatory, RolePredicate, OutcomeUnknown, false, 1},
		{"policy unsupported", CategoryPolicy, ClassPolicy, RolePredicate, OutcomeUnsupported, false, 1},
		{"context unknown", CategoryNormative, ClassDefault, RoleContextProvider, OutcomeUnknown, false, 1},
		{"context pass", CategoryNormative, ClassMandatory, RoleContextProvider, OutcomePass, true, 0},
		{"default pass", CategoryNormative, ClassDefault, RolePredicate, OutcomePass, true, 0},
		{"advisory failure", CategoryAdvisory, ClassMandatory, RolePredicate, OutcomeFail, true, 0},
		{"advisory role", CategoryNormative, ClassMandatory, RoleAdvisory, OutcomeFail, true, 0},
		{"recommendation failure", CategoryNormative, ClassRecommendation, RolePredicate, OutcomeFail, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, tiny := range []bool{false, true} {
				limits := Budgets{}
				if tiny {
					limits.MaxDiagnostics, limits.MaxDiagnosticBytes = 1, 1
				}
				account, _ := NewBudgetAccount(limits)
				acc := newReportAccumulator(reportTestProfile(), account)
				rule := reportTestRule()
				rule.Category, rule.Class, rule.Role = tc.category, tc.class, tc.role
				if err := acc.record(rule, reportTestDecision(tc.outcome), Location{}); err != nil {
					t.Fatal(err)
				}
				r := acc.finish(true, SyntaxNotAssessed)
				if r.Counts != (Counts{}) || r.Conformance != ConformanceConformant || r.AssessmentComplete != tc.complete || r.BlockingProblems != tc.blockers || r.Satisfied() != (tc.complete && tc.blockers == 0) {
					t.Fatalf("non-normative axes = %+v", r)
				}
			}
		})
	}
}

func TestReportAccumulatorEnumerationSyntaxAndTerminal(t *testing.T) {
	for _, outcome := range []Outcome{OutcomePass, OutcomeFail} {
		for _, syntax := range []SyntaxState{SyntaxMalformed, SyntaxIncomplete, SyntaxNotAssessed} {
			account, _ := NewBudgetAccount(Budgets{MaxDiagnosticBytes: 1})
			profile := reportTestProfile()
			profile.Target = TargetSource
			acc := newReportAccumulator(profile, account)
			if err := acc.record(reportTestRule(), reportTestDecision(outcome), Location{}); err != nil {
				t.Fatal(err)
			}
			acc.terminal(errors.New("do not stringify arbitrary errors"), CategoryResource, Code(strings.Repeat("stopped", 10000)))
			r := acc.finish(false, syntax)
			if r.AssessmentComplete || r.Satisfied() || r.BlockingProblems == 0 || r.Terminal == nil || len(r.Terminal.Reason) > 1024 {
				t.Fatalf("unsafe terminal report: %+v", r)
			}
			if outcome == OutcomeFail && r.Conformance != ConformanceViolated {
				t.Fatal("lost proven failure")
			}
			if outcome == OutcomePass && r.Conformance != ConformanceUndetermined {
				t.Fatal("incomplete source claimed conformance")
			}
			if len(r.Findings) != 0 || len(r.Rules) != 0 {
				t.Fatal("terminal bypassed ordinary storage quota")
			}
		}
	}
	account, _ := NewBudgetAccount(Budgets{})
	acc := newReportAccumulator(reportTestProfile(), account)
	acc.incomplete()
	if r := acc.finish(true, SyntaxNotAssessed); r.AssessmentComplete || r.Conformance != ConformanceUndetermined || r.Satisfied() {
		t.Fatal("aborted enumeration accepted")
	}
}

func TestReportAccumulatorCopiesAndChargesAllRetainedMetadata(t *testing.T) {
	account, _ := NewBudgetAccount(Budgets{})
	profile := reportTestProfile()
	acc := newReportAccumulator(profile, account)
	profile.Required[0].ID = "changed"
	rule := reportTestRule()
	decision := reportTestDecision(OutcomeFail)
	decision.Cause = &DecisionRef{Rule: rule.Key, Subject: decision.Subject}
	decision.Related = []Location{{DocumentID: "related", Path: "/first", Position: &SourcePosition{Line: 2}}}
	location := Location{DocumentID: "doc", Path: "/second", Position: &SourcePosition{Line: 3}}
	if err := acc.record(rule, decision, location); err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := account.Usage(BudgetDiagnosticBytes)
	if firstBytes < int64(len(rule.Text.Explanation)+len(rule.Sources[0].URL)+len(decision.Related[0].Path)) {
		t.Fatal("nested metadata was not charged")
	}
	if err := acc.record(rule, decision, location); err != nil {
		t.Fatal(err)
	}
	secondBytes, _ := account.Usage(BudgetDiagnosticBytes)
	if secondBytes-firstBytes >= firstBytes {
		t.Fatal("rule metadata charged again for same key")
	}
	rule.Sources[0].IDs[0] = "changed"
	rule.Selector.Component.Name.Local = "changed"
	rule.Targets[0] = TargetSource
	decision.Cause.Rule.ID = "changed"
	decision.Related[0].Position.Line = 99
	location.Position.Line = 99
	r := acc.finish(true, SyntaxNotAssessed)
	if len(r.Rules) != 1 || r.Profile.Required[0].ID != "report.rule" || r.Rules[0].Sources[0].IDs[0] != "source" || r.Rules[0].Selector.Component.Name.Local != "test" || r.Rules[0].Targets[0] != TargetModel || r.Findings[0].Cause.Rule.ID != "report.rule" || r.Findings[0].Related[0].Position.Line != 2 || r.Findings[0].Location.Position.Line != 3 {
		t.Fatalf("retained alias: %+v", r)
	}
	r.Findings[0].Cause.Rule.ID = "report-mutated"
	r.Rules[0].Sources[0].IDs[0] = "report-mutated"
	r.Profile.Required[0].ID = "report-mutated"
	again := acc.finish(true, SyntaxNotAssessed)
	if reflect.DeepEqual(r, again) || again.Rules[0].Sources[0].IDs[0] != "source" || again.Findings[0].Cause.Rule.ID != "report.rule" || again.Profile.Required[0].ID != "report.rule" {
		t.Fatal("final reports alias accumulator")
	}
}

func TestReportAccumulatorAtomicStorageReservationAndBoundedValues(t *testing.T) {
	for _, limits := range []Budgets{{MaxDiagnosticBytes: 1}, {MaxDiagnostics: 1}} {
		account, _ := NewBudgetAccount(limits)
		if limits.MaxDiagnostics == 1 {
			if err := account.Charge(BudgetDiagnostics, 1); err != nil {
				t.Fatal(err)
			}
		}
		beforeCount, _ := account.Usage(BudgetDiagnostics)
		beforeBytes, _ := account.Usage(BudgetDiagnosticBytes)
		acc := newReportAccumulator(reportTestProfile(), account)
		if err := acc.record(reportTestRule(), reportTestDecision(OutcomePass), Location{}); err != nil {
			t.Fatal(err)
		}
		afterCount, _ := account.Usage(BudgetDiagnostics)
		afterBytes, _ := account.Usage(BudgetDiagnosticBytes)
		if afterCount != beforeCount || afterBytes != beforeBytes {
			t.Fatal("partial diagnostic reservation consumed quota")
		}
	}
	account, _ := NewBudgetAccount(Budgets{})
	acc := newReportAccumulator(reportTestProfile(), account)
	d := reportTestDecision(OutcomeFail)
	d.Expected = strings.Repeat("x", 1<<20)
	d.Actual = strings.Repeat("y", 1<<20)
	if err := acc.record(reportTestRule(), d, Location{}); err != nil {
		t.Fatal(err)
	}
	r := acc.finish(true, SyntaxNotAssessed)
	if len(r.Findings) != 1 || len(r.Findings[0].Expected) > 4096 || len(r.Findings[0].Actual) > 4096 {
		t.Fatal("actual/expected were not independently bounded before retention")
	}
}

func TestReportAccumulatorRejectsInvalidInputsBeforeMutation(t *testing.T) {
	for _, account := range []*Account{nil, new(Account)} {
		acc := newReportAccumulator(reportTestProfile(), account)
		if err := acc.record(reportTestRule(), reportTestDecision(OutcomePass), Location{}); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("invalid account hidden as truncation: %v", err)
		}
		if r := acc.finish(true, SyntaxNotAssessed); r.Counts != (Counts{}) || len(r.Findings) != 0 {
			t.Fatal("invalid account mutated report")
		}
	}
	for _, category := range []DiagnosticCategory{CategoryNormative, CategoryPolicy, CategoryAdvisory} {
		account, _ := NewBudgetAccount(Budgets{})
		acc := newReportAccumulator(reportTestProfile(), account)
		rule := reportTestRule()
		rule.Category = category
		if err := acc.record(rule, reportTestDecision("bogus"), Location{}); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("invalid %s outcome accepted: %v", category, err)
		}
		if r := acc.finish(true, SyntaxNotAssessed); r.Counts != (Counts{}) || r.BlockingProblems != 0 || len(r.Findings) != 0 {
			t.Fatal("invalid outcome mutated report")
		}
	}
}

type reportPanicError struct{}

func (reportPanicError) Error() string {
	panic("terminal must not stringify an arbitrary operation error")
}

func TestReportTerminalIsReservedBoundedAndKeepsFirstStatus(t *testing.T) {
	account, _ := NewBudgetAccount(Budgets{MaxDiagnostics: 1})
	acc := newReportAccumulator(reportTestProfile(), account)
	if err := acc.record(reportTestRule(), reportTestDecision(OutcomeFail), Location{}); err != nil {
		t.Fatal(err)
	}
	acc.terminal(reportPanicError{}, CategoryResource, "first-stop")
	acc.terminal(reportPanicError{}, CategoryCapability, "later-stop")
	r := acc.finish(false, SyntaxIncomplete)
	if r.Terminal == nil || r.Terminal.Category != CategoryResource || r.Terminal.Reason != "first-stop" || r.BlockingProblems != 1 || r.Counts.Fail != 1 || r.Conformance != ConformanceViolated {
		t.Fatalf("terminal status lost: %+v", r)
	}
	if len(r.Findings) != 1 || len(r.Rules) != 1 {
		t.Fatal("terminal corrupted full ordinary diagnostics")
	}
	r.Terminal.Reason = "mutated"
	if again := acc.finish(false, SyntaxIncomplete); again.Terminal.Reason != "first-stop" {
		t.Fatal("terminal snapshot aliased accumulator")
	}
}

func TestReportTruncationWithoutTerminalDoesNotStopCounting(t *testing.T) {
	account, _ := NewBudgetAccount(Budgets{MaxDiagnostics: 1})
	acc := newReportAccumulator(reportTestProfile(), account)
	for _, outcome := range []Outcome{OutcomePass, OutcomeFail, OutcomeUnknown} {
		if err := acc.record(reportTestRule(), reportTestDecision(outcome), Location{}); err != nil {
			t.Fatal(err)
		}
	}
	r := acc.finish(true, SyntaxNotAssessed)
	if !r.DiagnosticsTruncated || len(r.Findings) != 1 || r.Counts != (Counts{Pass: 1, Fail: 1, Unknown: 1}) || r.Conformance != ConformanceViolated || r.AssessmentComplete || r.Terminal != nil || r.BlockingProblems != 0 {
		t.Fatalf("storage truncation changed assessment: %+v", r)
	}
}

func TestReportDiagnosticBytesIncludeLargeNestedMetadata(t *testing.T) {
	for _, change := range []func(*RuleDefinition, *Decision, *Location){
		func(rule *RuleDefinition, _ *Decision, _ *Location) {
			rule.Sources[0].IDs = []string{strings.Repeat("x", 1<<20)}
		},
		func(rule *RuleDefinition, _ *Decision, _ *Location) {
			rule.Selector.Component.Occurrence = strings.Repeat("x", 1<<20)
		},
		func(_ *RuleDefinition, decision *Decision, _ *Location) {
			decision.Related = []Location{{Path: strings.Repeat("x", 1<<20)}}
		},
		func(_ *RuleDefinition, decision *Decision, _ *Location) {
			decision.Cause = &DecisionRef{Subject: NodeRef{DocumentID: strings.Repeat("x", 1<<20)}}
		},
		func(_ *RuleDefinition, _ *Decision, location *Location) {
			location.Position = &SourcePosition{CoordinateSpace: strings.Repeat("x", 1<<20)}
		},
	} {
		account, _ := NewBudgetAccount(Budgets{MaxDiagnosticBytes: 4096})
		acc := newReportAccumulator(reportTestProfile(), account)
		rule, decision, location := reportTestRule(), reportTestDecision(OutcomeFail), Location{}
		change(&rule, &decision, &location)
		if err := acc.record(rule, decision, location); err != nil {
			t.Fatal(err)
		}
		r := acc.finish(true, SyntaxNotAssessed)
		if !r.DiagnosticsTruncated || len(r.Findings) != 0 || len(r.Rules) != 0 || r.Counts.Fail != 1 || r.Conformance != ConformanceViolated || !r.AssessmentComplete {
			t.Fatalf("nested diagnostic payload escaped quota: %+v", r)
		}
		if count, _ := account.Usage(BudgetDiagnostics); count != 0 {
			t.Fatal("rejected metadata consumed count quota")
		}
		if size, _ := account.Usage(BudgetDiagnosticBytes); size != 0 {
			t.Fatal("rejected metadata consumed byte quota")
		}
	}
}
