package xsd

import (
	"errors"
)

// assessmentErr is private, run-local technical state. It never wraps the
// public invalid-document sentinel unless independently proved issues exist.
func (c *Context) Incomplete(message string) {
	c.AssessmentErr = errors.Join(c.AssessmentErr, &TechnicalError{Message: message})
	c.AssessmentVersion++
	if c.local != nil {
		c.local.limit(c.local.family, "xsd-assessment-incomplete", c.AssessmentErr)
	}
}

// A cache hit revisits an existing incomplete fact. Preserve the original cause
// while advancing the event epoch used by value-comparison callers.
func (c *Context) IncompleteAgain(message string) {
	if c.AssessmentErr == nil {
		c.Incomplete(message)
		return
	}
	c.AssessmentVersion++
	if c.local != nil {
		c.local.limit(c.local.family, "xsd-assessment-incomplete", c.AssessmentErr)
	}
}

func (c *Context) IncompleteFailure(message string) *SimpleFailure {
	c.Incomplete(message)
	return c.AssessmentFailure(message)
}

// Preserve the original cause when metadata assessment already recorded it.
// Every assessment failure must still carry a non-nil technical outcome.
func (c *Context) AssessmentFailure(message string) *SimpleFailure {
	if c.AssessmentErr == nil {
		c.Incomplete(message)
	}
	return &SimpleFailure{Constraint: "assessment", Message: message}
}

func TechnicalFailure(failure *SimpleFailure) bool {
	return failure != nil && (failure.Constraint == "assessment" || failure.Constraint == "schema")
}
