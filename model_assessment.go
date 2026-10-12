package musicxml

import (
	"context"
	"errors"

	"github.com/go-muse/go-musicxml/validation"
)

const modelDocumentID = "model"

// AssessModel assesses the current generated Go document directly, without
// encoding it, invoking XML marshal methods, or parsing XML. Options must name
// an explicit model-target profile. It does not reconstruct original source
// order, spelling, presence, namespaces, discarded content or numeric precision.
//
// Ordinary normative or model-representation findings have a nil operation
// error; use Report.Satisfied for acceptance. Configuration, cancellation,
// resource and other traversal interruptions return their cause and the best
// partial report. Model reports use SyntaxNotAssessed and field/index locations
// without XML coordinates. The caller must not mutate the document concurrently.
// Assessment never modifies the document or fetches linked resources.
//
// Finite float64 decimal fields mean their exact current binary rational value,
// not an encoder's shortest decimal spelling. String-backed fields provide their
// current string to shared schema-domain checks, never an original source lexeme.
func AssessModel(ctx context.Context, document Document, options AssessmentOptions) (validation.Report, error) {
	report, _, err := assessModel(ctx, document, options, modelWalkOptions{})
	return report, err
}

func assessModel(ctx context.Context, document Document, options AssessmentOptions, walkOptions modelWalkOptions) (validation.Report, modelWalkStatus, error) {
	if ctx == nil {
		return validation.Report{}, modelWalkStatus{}, &validation.ConfigurationError{Field: "context", Reason: "must not be nil"}
	}
	if err := ctx.Err(); err != nil {
		return validation.Report{}, modelWalkStatus{}, &validation.IncompleteError{Reason: "canceled-before-model", Cause: err}
	}
	if options.Profile != ProfileModelXSD && options.Profile != ProfileModelLocal && options.Profile != ProfileModelTimeline {
		return validation.Report{}, modelWalkStatus{}, &validation.ConfigurationError{Field: "profile", Reason: "unknown, empty or incompatible explicit model profile"}
	}
	account, err := validation.NewBudgetAccount(options.Budgets)
	if err != nil {
		return validation.Report{}, modelWalkStatus{}, err
	}
	// This sealed root check does not walk fields or invoke a model method.
	if _, err := validationSchemaForDocument(document); err != nil {
		return validation.Report{}, modelWalkStatus{}, err
	}
	var engine *validation.Engine
	if options.Profile == ProfileModelTimeline {
		engine, err = newTimelineAssessmentEngine()
	} else {
		engine, err = newModelAssessmentEngine()
	}
	if err != nil {
		return validation.Report{}, modelWalkStatus{}, err
	}
	session, err := engine.Begin(ctx, validation.RunOptions{DocumentID: modelDocumentID, Profile: options.Profile, Account: account})
	if err != nil {
		return validation.Report{}, modelWalkStatus{}, err
	}
	status, walkErr := walkModel(ctx, document, session, account, modelDocumentID, walkOptions)
	// Every admitted session is finished exactly once, including interruption.
	report, finishErr := session.Finish(ctx, validation.Completion{InputComplete: walkErr == nil, Syntax: validation.SyntaxNotAssessed, Cause: walkErr})
	return report, status, errors.Join(walkErr, finishErr)
}
