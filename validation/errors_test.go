package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestIncompleteErrorPreservesIdentityAndCause(t *testing.T) {
	for _, cause := range []error{nil, context.Canceled, context.DeadlineExceeded, errors.New("reader failure")} {
		err := &IncompleteError{Reason: "input did not finish", Cause: cause}
		if !errors.Is(err, ErrIncompleteAssessment) {
			t.Errorf("%v lacks incomplete identity", err)
		}
		if cause != nil && !errors.Is(err, cause) {
			t.Errorf("%v lost cause %v", err, cause)
		}
		if !strings.Contains(err.Error(), "input did not finish") {
			t.Errorf("reason missing: %v", err)
		}
		if errors.Is(err, ErrInvalidConfiguration) || errors.Is(err, ErrResourceLimit) {
			t.Errorf("invented category: %v", err)
		}
		var typed *IncompleteError
		if !errors.As(errors.Join(errors.New("other"), err), &typed) || typed != err {
			t.Error("joined typed identity lost")
		}
	}
}

func TestConfigurationErrorAndResourceError(t *testing.T) {
	configuration := &ConfigurationError{Field: "MaxNodes", Reason: "negative"}
	if !errors.Is(configuration, ErrInvalidConfiguration) || errors.Is(configuration, ErrIncompleteAssessment) {
		t.Errorf("configuration identities: %v", configuration)
	}
	for _, word := range []string{"MaxNodes", "negative"} {
		if !strings.Contains(configuration.Error(), word) {
			t.Errorf("missing %q in %q", word, configuration)
		}
	}
	resource := &ResourceError{Kind: BudgetNodes, Limit: 3, Used: 2, Requested: 2}
	for _, identity := range []error{ErrResourceLimit, ErrIncompleteAssessment} {
		if !errors.Is(resource, identity) {
			t.Errorf("missing %v", identity)
		}
	}
	if errors.Is(resource, ErrInvalidConfiguration) {
		t.Error("resource became configuration")
	}
	for _, word := range []string{"nodes", "3", "2"} {
		if !strings.Contains(resource.Error(), word) {
			t.Errorf("missing %q in %q", word, resource)
		}
	}
	wrapped := &IncompleteError{Reason: "stopped", Cause: resource}
	var got *ResourceError
	if !errors.As(wrapped, &got) || got != resource {
		t.Error("lost resource detail")
	}
	if ErrInvalidSession == ErrInvalidConfiguration || ErrInvalidSession == ErrIncompleteAssessment || ErrInvalidSession == ErrResourceLimit {
		t.Error("distinct sentinels aliased")
	}
}

func TestTypedErrorsAreNilSafe(t *testing.T) {
	var incomplete *IncompleteError
	var config *ConfigurationError
	var resource *ResourceError
	for _, err := range []error{incomplete, config, resource} {
		if err.Error() == "" {
			t.Error("nil receiver needs useful text")
		}
		_ = errors.Is(err, errors.New("other"))
	}
}
