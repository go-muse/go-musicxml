package validation

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReportAcceptanceSeparatesAxes(t *testing.T) {
	base := Report{Requested: true, Profile: Profile{ID: "test", Target: TargetModel}, Target: TargetModel, Syntax: SyntaxNotAssessed, Conformance: ConformanceConformant, AssessmentComplete: true, InputComplete: true}
	if !base.Satisfied() {
		t.Fatal("complete model report rejected")
	}
	if (Report{}).Satisfied() {
		t.Fatal("zero report accepted")
	}
	tests := map[string]func(*Report){"unrequested": func(r *Report) { r.Requested = false }, "incomplete input": func(r *Report) { r.InputComplete = false }, "incomplete assessment": func(r *Report) { r.AssessmentComplete = false }, "unknown": func(r *Report) { r.Conformance = ConformanceUndetermined }, "violated": func(r *Report) { r.Conformance = ConformanceViolated }, "blocker": func(r *Report) { r.BlockingProblems = 1 }, "source without syntax": func(r *Report) { r.Target = TargetSource; r.Profile.Target = TargetSource }, "target mismatch": func(r *Report) { r.Profile.Target = TargetSource }}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			r := base
			change(&r)
			if r.Satisfied() {
				t.Fatal("unsafe acceptance")
			}
		})
	}
	base.DiagnosticsTruncated = true
	if !base.Satisfied() {
		t.Fatal("storage truncation alone changed acceptance")
	}
}

func TestReportFormatsOfflineExplanationAndSource(t *testing.T) {
	key := RuleKey{ID: "test.rule", Revision: 7}
	report := Report{Findings: []Finding{{Decision: Decision{Rule: key, Subject: NodeRef{DocumentID: "doc", Node: 1}, Outcome: OutcomeFail, Reason: "missing-child", Expected: "one", Actual: "zero\x1b[31m"}, Category: CategoryNormative, Location: Location{Target: TargetModel, DocumentID: "doc", Path: "/root"}}}, Rules: []RuleDefinition{{Key: key, Text: RuleText{Title: "Required child", Explanation: "This child is required by the scoped rule.", Message: "Missing child"}, Sources: []SourceRef{{IDs: []string{"source-id"}, URL: "https://example.org/spec#child"}}}}}
	var output bytes.Buffer
	if err := report.Format(&output); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"test.rule", "7", "/root", "This child is required", "https://example.org/spec#child"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("missing %q from %q", want, output.String())
		}
	}
	if strings.Contains(output.String(), "\x1b") {
		t.Fatal("raw control sequence escaped formatter")
	}
	failure := errors.New("writer failed")
	if err := report.Format(failingReportWriter{failure}); !errors.Is(err, failure) {
		t.Fatalf("writer error lost: %v", err)
	}
	if err := report.Format(shortReportWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write lost: %v", err)
	}
	if err := report.Format(nil); err == nil {
		t.Fatal("nil writer accepted")
	}
}

type failingReportWriter struct{ err error }

func (w failingReportWriter) Write([]byte) (int, error) { return 0, w.err }

type shortReportWriter struct{}

func (shortReportWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return len(p) - 1, nil
}
