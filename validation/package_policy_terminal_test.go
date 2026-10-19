package validation

import (
	"context"
	"errors"
	"testing"
)

func TestProcessingPolicyTerminalClassification(t *testing.T) {
	ordinary := errors.New("unavailable transport")
	resource := &ResourceError{Kind: BudgetBufferedBytes, Limit: 1, Requested: 2}
	for _, tc := range []struct {
		name      string
		cause     error
		malformed bool
		category  DiagnosticCategory
		reason    Code
	}{
		{"policy", ErrProcessingPolicy, false, CategoryPolicy, "processing-policy"},
		{"joined ordinary", errors.Join(ordinary, ErrProcessingPolicy), false, CategoryPolicy, "processing-policy"},
		{"wrapped policy", &IncompleteError{Reason: "transport-refused", Cause: ErrProcessingPolicy}, false, CategoryPolicy, "processing-policy"},
		{"canceled", errors.Join(ErrProcessingPolicy, context.Canceled), false, CategoryResource, "canceled"},
		{"deadline", errors.Join(ErrProcessingPolicy, context.DeadlineExceeded), false, CategoryResource, "deadline-exceeded"},
		{"resource", errors.Join(ErrProcessingPolicy, resource), false, CategoryResource, "resource-limit"},
		{"existing resource before cancellation", errors.Join(ErrProcessingPolicy, context.Canceled, resource), false, CategoryResource, "resource-limit"},
		{"actual malformed source", ErrProcessingPolicy, true, CategorySyntax, "malformed-source"},
		{"ordinary unmarked", ordinary, false, CategoryCapability, "assessment-interrupted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			category, reason := terminalClassification(tc.cause, tc.malformed)
			if category != tc.category || reason != tc.reason {
				t.Errorf("got %s/%s want %s/%s", category, reason, tc.category, tc.reason)
			}
		})
	}
}
func TestPackagePolicyRefusalIsIncompleteWithoutNormativeViolation(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	legacy := errors.New("legacy path refusal")
	original := errors.Join(legacy, ErrProcessingPolicy)
	b, err := s.finish(context.Background(), PackageCompletion{Cause: original})
	report := b.PackageReport()
	if !errors.Is(err, legacy) || !errors.Is(err, ErrProcessingPolicy) {
		t.Errorf("lost error identities: %v", err)
	}
	if b.Satisfied() || report.AssessmentComplete || report.Conformance == ConformanceViolated || report.Counts.Fail != 0 || report.Terminal == nil || report.Terminal.Category != CategoryPolicy || report.Terminal.Reason != "processing-policy" {
		t.Errorf("policy refusal misreported: %+v", report)
	}
}
