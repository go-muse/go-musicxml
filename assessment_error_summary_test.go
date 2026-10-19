package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestDecodeAssessedExplainsTruncatedSourceIssues(t *testing.T) {
	options := strictSourceOptions()
	options.Assessment.Budgets.MaxDiagnosticBytes = 1
	result, err := DecodeAssessed(context.Background(), strings.NewReader(`<score-partwise/>`), options)
	var invalid *ValidationError
	if !errors.As(err, &invalid) || !errors.Is(err, ErrInvalidDocument) || result.SourceLine.Counts.Fail == 0 || !result.SourceLine.DiagnosticsTruncated {
		t.Fatalf("fixture did not prove a truncated violation: %+v %v", result, err)
	}
	if len(invalid.Issues) != 0 || len(result.SourceLine.Findings) != 0 {
		t.Errorf("diagnostic byte quota bypassed: %+v / %+v", invalid.Issues, result.SourceLine.Findings)
	}
	if !strings.Contains(err.Error(), "diagnostic details were truncated") || !strings.Contains(err.Error(), "failed rule evaluations") {
		t.Errorf("truncated source failure lost explanation: %v", err)
	}
}

func TestSourceAcceptanceSummaryKeepsCompositeCountsDistinctFromDetails(t *testing.T) {
	detail := func(path string) validation.Finding {
		return validation.Finding{Decision: validation.Decision{Rule: validation.RuleKey{ID: "composite", Revision: 1}, Outcome: validation.OutcomeFail, Reason: "detail"}, Category: validation.CategoryNormative, Location: validation.Location{Path: path}}
	}
	for _, truncated := range []bool{false, true} {
		report := validation.Report{Requested: true, InputComplete: true, AssessmentComplete: true, Syntax: validation.SyntaxWellFormed, Conformance: validation.ConformanceViolated, Counts: validation.Counts{Fail: 1}, Findings: []validation.Finding{detail("/first"), detail("/second")}, DiagnosticsTruncated: truncated}
		err := sourceAcceptanceError(report)
		var invalid *ValidationError
		if !errors.As(err, &invalid) || len(invalid.Issues) != 2 {
			t.Fatalf("stored real details changed: %+v %v", invalid, err)
		}
		if got := strings.Contains(err.Error(), "diagnostic details were truncated"); got != truncated {
			t.Errorf("truncation summary=%t want %t: %v", got, truncated, err)
		}
		if truncated && !strings.Contains(err.Error(), "failed rule evaluations: 1") {
			t.Errorf("composite count confused with detail count: %v", err)
		}
		if invalid.Issues[0].Path != "/first" || invalid.Issues[1].Path != "/second" {
			t.Errorf("invented or replaced locations: %+v", invalid.Issues)
		}
	}
}

func TestDecodeAssessedTruncatedSummaryPreservesJoinedCauses(t *testing.T) {
	options := strictSourceOptions()
	options.Assessment.Budgets.MaxDiagnosticBytes = 1
	result, err := DecodeAssessed(context.Background(), strings.NewReader(assessedScore("18446744073709551616", `<instruments bad="x">1</instruments>`)+`<tail>`), options)
	var invalid *ValidationError
	var syntax *xml.SyntaxError
	var conversion *ConversionError
	if !errors.As(err, &invalid) || !errors.As(err, &syntax) || !errors.As(err, &conversion) || !errors.Is(err, strconv.ErrRange) || !errors.Is(err, validation.ErrIncompleteAssessment) {
		t.Fatalf("truncation summary lost joined causes: %v", err)
	}
	if !strings.Contains(err.Error(), "diagnostic details were truncated") || !result.SourceLine.DiagnosticsTruncated || result.SourceLine.Counts.Fail == 0 || len(invalid.Issues) != 0 || result.Document != nil {
		t.Errorf("truncation altered independent result: %+v %v", result, err)
	}
}
