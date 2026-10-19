package validation

import (
	"context"
	"errors"
	"testing"
)

func TestReviewTerminalClassification(t *testing.T) {
	resource := &ResourceError{Kind: BudgetRuleSteps, Limit: 1, Used: 1, Requested: 1}
	configuration := contract("test", "bad configuration")
	lifecycle := &SessionError{Operation: "test", Reason: "bad lifecycle", Cause: configuration}
	unsupported := &ExcludedRootError{Name: QName{Local: "other"}, Profile: "test.source"}
	transport := errors.New("transport interrupted")
	cases := []struct {
		name     string
		cause    error
		syntax   SyntaxState
		category DiagnosticCategory
		reason   Code
	}{
		{"resource", resource, SyntaxIncomplete, CategoryResource, "resource-limit"},
		{"canceled", context.Canceled, SyntaxIncomplete, CategoryResource, "canceled"},
		{"deadline", context.DeadlineExceeded, SyntaxIncomplete, CategoryResource, "deadline-exceeded"},
		{"configuration", configuration, SyntaxIncomplete, CategoryCapability, "invalid-configuration"},
		{"lifecycle", lifecycle, SyntaxIncomplete, CategoryCapability, "invalid-session"},
		{"unsupported", unsupported, SyntaxIncomplete, CategoryCapability, "unsupported-root"},
		{"transport", transport, SyntaxIncomplete, CategoryCapability, "assessment-interrupted"},
		{"malformed", transport, SyntaxMalformed, CategorySyntax, "malformed-source"},
		{"malformed-canceled", context.Canceled, SyntaxMalformed, CategoryResource, "canceled"},
		{"joined-resource", errors.Join(transport, context.Canceled, resource), SyntaxMalformed, CategoryResource, "resource-limit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, s := sessionFixture(t, Budgets{})
			qaOpenRoot(t, s)
			qaClose(t, s, factEnd(1)) // Preserve this known failure through terminal interruption.
			report, err := s.Finish(context.Background(), Completion{Syntax: tc.syntax, Cause: tc.cause})
			if !errors.Is(err, tc.cause) {
				t.Fatalf("lost actual cause: %v", err)
			}
			if report.Terminal == nil || report.Terminal.Category != tc.category || report.Terminal.Reason != tc.reason {
				t.Errorf("terminal=%+v want=%s/%s", report.Terminal, tc.category, tc.reason)
			}
			if report.Counts.Fail != 1 || report.Conformance != ConformanceViolated || report.AssessmentComplete {
				t.Fatalf("known failure lost: %+v", report)
			}
		})
	}
}

func TestReviewMalformedClassificationRequiresValidSourceCompletion(t *testing.T) {
	cause := errors.New("malformed XML")
	cases := []struct {
		name       string
		target     Target
		completion Completion
		reason     Code
		sentinel   error
	}{
		{"model-malformed", TargetModel, Completion{Syntax: SyntaxMalformed, Cause: cause}, "invalid-configuration", ErrInvalidConfiguration},
		{"package-malformed", TargetPackage, Completion{Syntax: SyntaxMalformed, Cause: cause}, "invalid-configuration", ErrInvalidConfiguration},
		{"source-no-cause", TargetSource, Completion{Syntax: SyntaxMalformed}, "invalid-session", ErrInvalidSession},
		{"source-contradictory", TargetSource, Completion{InputComplete: true, Syntax: SyntaxMalformed, Cause: cause}, "invalid-configuration", ErrInvalidConfiguration},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rules, profiles, plans := catalogFixture(t)
			rules[0].Targets = []Target{tc.target}
			profiles[0].Target = tc.target
			c, err := NewCatalog(rules, profiles, plans)
			if err != nil {
				t.Fatal(err)
			}
			e, err := NewEngine(c)
			if err != nil {
				t.Fatal(err)
			}
			s, err := e.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: profiles[0].ID})
			if err != nil {
				t.Fatal(err)
			}
			r, err := s.Finish(context.Background(), tc.completion)
			if !errors.Is(err, tc.sentinel) || r.Terminal == nil || r.Terminal.Category != CategoryCapability || r.Terminal.Reason != tc.reason {
				t.Fatalf("invalid completion misclassified: terminal=%+v err=%v", r.Terminal, err)
			}
			if tc.completion.Cause != nil && !errors.Is(err, cause) {
				t.Fatalf("explicit cause lost: %v", err)
			}
		})
	}
}
