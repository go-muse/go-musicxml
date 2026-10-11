package validation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

func schemaDiagnosticSession(t *testing.T, budgets Budgets) (*Session, RuleDefinition) {
	t.Helper()
	s, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "source-schema-test", Budgets: budgets})
	if err != nil {
		t.Fatal(err)
	}
	// Isolate the shared result-to-report seam. Results are already established;
	// source binding and its own independent tests are not rerun by this fixture.
	s.schema.bound = true
	return s, s.catalog.rules[s.profile.Required[0]]
}

func schemaDiagnosticNode(s *Session, id NodeID) *factNode {
	n := &factNode{header: NodeHeader{ID: id, Location: Location{Target: TargetSource, DocumentID: "source", Path: "/occurrence"}}, closed: true}
	s.nodes[id] = n
	if s.root == nil {
		s.root = n
	}
	return n
}

func TestSourceSchemaDocumentLimitationDefersToNormativeDetails(t *testing.T) {
	for _, tc := range []struct {
		name      string
		budgets   Budgets
		cancel    bool
		findings  int
		truncated bool
	}{
		{name: "summarized", findings: 2},
		{name: "one-slot", budgets: Budgets{MaxDiagnostics: 1}, findings: 1, truncated: true},
		{name: "no-fitting-detail", budgets: Budgets{MaxDiagnosticBytes: 1}, findings: 0, truncated: true},
		{name: "canceled-finish", cancel: true, findings: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rule := schemaDiagnosticSession(t, tc.budgets)
			account := s.account
			unavailable := xsd.Result{Applicable: true, Valid: true, Complete: false, Limitations: []xsd.Limitation{{Reason: "dtd-infoset-unavailable"}}}
			for id := NodeID(1); id <= 10; id++ {
				n := schemaDiagnosticNode(s, id)
				if err := s.recordXSDResult(context.Background(), n, rule, unavailable); err != nil {
					t.Fatal(err)
				}
				if err := s.recordXSDResult(context.Background(), n, rule, unavailable); err != nil {
					t.Fatal(err)
				} // no replay double-count
			}
			n := schemaDiagnosticNode(s, 11)
			violation := xsd.Result{Applicable: true, Valid: false, Complete: true, Issues: []xsd.Detail{{Issue: xsd.Issue{Constraint: "integer", Message: "invalid integer"}, Subject: 11}}}
			if err := s.recordXSDResult(context.Background(), n, rule, violation); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if (err != nil) != tc.cancel || tc.cancel && !errors.Is(err, context.Canceled) {
				t.Fatalf("finish error: %v", err)
			}
			if report.Counts != (Counts{Unsupported: 10, Fail: 1}) || report.Conformance != ConformanceViolated || report.AssessmentComplete || report.Satisfied() {
				t.Fatalf("axes lost: %+v", report)
			}
			if len(report.Findings) != tc.findings || report.DiagnosticsTruncated != tc.truncated {
				t.Fatalf("stored findings=%d truncated=%t, want %d/%t: %+v", len(report.Findings), report.DiagnosticsTruncated, tc.findings, tc.truncated, report.Findings)
			}
			if tc.findings > 0 && (report.Findings[0].Outcome != OutcomeFail || report.Findings[0].Subject.Node != 11) {
				t.Fatalf("later violation lost: %+v", report.Findings)
			}
			if tc.findings == 2 && (!strings.Contains(report.Findings[1].Actual, "10") || report.Findings[1].Subject.Node != 1) {
				t.Fatalf("summary lacks count/first occurrence: %+v", report.Findings[1])
			}
			if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("retention leaked: %d", used)
			}
		})
	}
}

func TestSourceSchemaOptionalSummaryRetentionDoesNotStopAssessment(t *testing.T) {
	s, rule := schemaDiagnosticSession(t, Budgets{})
	initial := s.retained
	if _, err := s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete}); err != nil {
		t.Fatal(err)
	}
	mandatory := int64(64 + 384 + len(rule.Key.ID) + len("source") + len("dtd-infoset-unavailable"))
	s, rule = schemaDiagnosticSession(t, Budgets{MaxBufferedBytes: initial + mandatory})
	account := s.account
	n := schemaDiagnosticNode(s, 1)
	result := xsd.Result{Applicable: true, Valid: true, Complete: false, Limitations: []xsd.Limitation{{Reason: "dtd-infoset-unavailable"}}}
	if err := s.recordXSDResult(context.Background(), n, rule, result); err != nil {
		t.Fatalf("optional summary stopped semantic work: %v", err)
	}
	if s.accumulator.report.DiagnosticsTruncated {
		t.Fatal("early truncation would suppress later failure details")
	}
	report, err := s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil || report.Counts.Unsupported != 1 || !report.DiagnosticsTruncated || len(report.Findings) != 0 || report.Terminal != nil {
		t.Fatalf("optional omission: %+v %v", report, err)
	}
	if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("retention leaked: %d", used)
	}
}

func TestSourceSchemaSummaryEligibilityIsNarrow(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		blocked      bool
	}{
		{name: "other-unsupported", reason: "other-unavailable"},
		{name: "causal-blocker", reason: "dtd-infoset-unavailable", blocked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rule := schemaDiagnosticSession(t, Budgets{})
			s.schema.nodes = make(map[NodeID]schemaNode)
			for id := NodeID(1); id <= 2; id++ {
				n := schemaDiagnosticNode(s, id)
				if tc.blocked {
					s.schema.nodes[id] = schemaNode{cause: &DecisionRef{Rule: rule.Key, Subject: NodeRef{DocumentID: "source", Node: 99}}}
				}
				result := xsd.Result{Applicable: true, Valid: true, Complete: false, Limitations: []xsd.Limitation{{Reason: tc.reason}}}
				if err := s.recordXSDResult(context.Background(), n, rule, result); err != nil {
					t.Fatal(err)
				}
			}
			report, err := s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if err != nil || len(report.Findings) != 2 || report.DiagnosticsTruncated {
				t.Fatalf("unrelated detail was coalesced: %+v %v", report, err)
			}
			if report.Findings[0].Subject.Node != 1 || report.Findings[1].Subject.Node != 2 {
				t.Fatal("distinct occurrence locations lost")
			}
			if tc.blocked && (report.Counts.Blocked != 2 || report.Findings[0].Cause == nil) {
				t.Fatal("causal evidence lost")
			}
		})
	}
}
