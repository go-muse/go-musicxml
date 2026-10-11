package musicxml

import (
	"errors"
	"fmt"
)

// assessmentErr is private, run-local technical state. It never wraps the
// public invalid-document sentinel unless independently proved issues exist.
func (c *validationContext) incomplete(message string) {
	c.assessmentErr = errors.Join(c.assessmentErr, fmt.Errorf("musicxml: incomplete XSD assessment: %s", message))
	c.assessmentVersion++
}

// A cache hit revisits an existing incomplete fact. Preserve the original cause
// while advancing the event epoch used by value-comparison callers.
func (c *validationContext) incompleteAgain(message string) {
	if c.assessmentErr == nil {
		c.incomplete(message)
		return
	}
	c.assessmentVersion++
}

func (c *validationContext) result() error {
	if len(c.issues) == 0 {
		return c.assessmentErr
	}
	invalid := &ValidationError{Issues: c.issues}
	if c.assessmentErr == nil {
		return invalid
	}
	return errors.Join(invalid, c.assessmentErr)
}

func (c *validationContext) incompleteFailure(message string) *validationSimpleFailure {
	c.incomplete(message)
	return c.assessmentFailure(message)
}

// Preserve the original cause when metadata assessment already recorded it.
// Every assessment failure must still carry a non-nil technical outcome.
func (c *validationContext) assessmentFailure(message string) *validationSimpleFailure {
	if c.assessmentErr == nil {
		c.incomplete(message)
	}
	return &validationSimpleFailure{constraint: "assessment", message: message}
}

func validationTechnicalFailure(failure *validationSimpleFailure) bool {
	return failure != nil && (failure.constraint == "assessment" || failure.constraint == "schema")
}
