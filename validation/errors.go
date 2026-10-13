package validation

import (
	"errors"
	"fmt"
)

// ErrIncompleteAssessment identifies an assessment that could not be completed.
// It does not identify a normative document violation.
var ErrIncompleteAssessment = errors.New("validation: incomplete assessment")

// ErrInvalidConfiguration identifies invalid options, catalog data, or API arguments.
var ErrInvalidConfiguration = errors.New("validation: invalid configuration")

// ErrInvalidSession identifies an invalid session event or lifecycle transition.
var ErrInvalidSession = errors.New("validation: invalid session")

// ErrResourceLimit identifies exhaustion of an assessment resource limit.
var ErrResourceLimit = errors.New("validation: resource limit exceeded")

// ErrProcessingPolicy identifies a technical processing refusal by an adapter's
// explicit policy. It does not identify a normative document violation, malformed
// syntax, unsupported capability, or an ordinary unavailable transport. Adapters
// may preserve this marker alongside their existing public error identities.
var ErrProcessingPolicy = errors.New("validation: processing policy refusal")

// IncompleteError describes interrupted or unsupported assessment and preserves
// its underlying cause, including cancellation and independent resource errors.
type IncompleteError struct {
	Reason string
	Cause  error
}

// Error describes the unfinished assessment and its cause.
func (e *IncompleteError) Error() string {
	if e == nil {
		return "validation: nil incomplete assessment error"
	}
	message := ErrIncompleteAssessment.Error()
	if e.Reason != "" {
		message += ": " + e.Reason
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

// Unwrap preserves both ErrIncompleteAssessment and the underlying cause.
func (e *IncompleteError) Unwrap() []error {
	if e == nil {
		return nil
	}
	if e.Cause == nil {
		return []error{ErrIncompleteAssessment}
	}
	return []error{ErrIncompleteAssessment, e.Cause}
}

// ConfigurationError identifies a rejected configuration field or argument.
type ConfigurationError struct {
	Field  string
	Reason string
}

// Error describes the rejected field and reason.
func (e *ConfigurationError) Error() string {
	if e == nil {
		return "validation: nil configuration error"
	}
	return fmt.Sprintf("%s: %s: %s", ErrInvalidConfiguration, e.Field, e.Reason)
}

// Unwrap preserves ErrInvalidConfiguration.
func (e *ConfigurationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return ErrInvalidConfiguration
}

// ResourceError records a refused resource operation. Used excludes Requested;
// a refused operation never increases the account's usage.
type ResourceError struct {
	Kind      BudgetKind
	Limit     int64
	Used      int64
	Requested int64
}

// Error describes the resource, limit, existing usage and refused amount.
func (e *ResourceError) Error() string {
	if e == nil {
		return "validation: nil resource error"
	}
	return fmt.Sprintf("%s: %s (limit %d, used %d, requested %d)", ErrResourceLimit, e.Kind, e.Limit, e.Used, e.Requested)
}

// Unwrap preserves resource exhaustion and incomplete-assessment identities.
func (e *ResourceError) Unwrap() []error {
	if e == nil {
		return nil
	}
	return []error{ErrResourceLimit, ErrIncompleteAssessment}
}
