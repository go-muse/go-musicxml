package musicxml

import (
	"context"
	"errors"

	"github.com/go-muse/go-musicxml/validation"
)

const modelDocumentID = "model"

// AssessModel assesses the current generated Go document directly, without
// encoding it, invoking XML marshal methods, or parsing XML. It does not
// reconstruct original source order, spelling, presence, namespaces, discarded
// content or numeric precision.
//
// An empty Profile selects ProfileModelSemantics with the bundled required
// dictionaries, or options.Dictionaries: exactly the model assessment of
// DecodeAssessed, so a document that DecodeAssessed returned gets the same
// model report while the budgets are not exhausted. (The loader's model stage
// shares its account with the source report, so a tight diagnostic or buffer
// budget can truncate or refuse the two differently.) Otherwise Profile must be a model-target profile, and a
// non-nil options.Dictionaries requires a dictionary or semantic one.
//
// Ordinary normative or model-representation findings have a nil operation
// error; use Report.Satisfied for acceptance. Configuration, cancellation,
// resource and other traversal interruptions return their cause and the best
// partial report. Model reports use SyntaxNotAssessed and field/index locations
// without XML coordinates. The caller must not mutate the document concurrently.
// Assessment never modifies the document or fetches linked resources.
//
// Finite float64 decimal fields mean the shortest decimal that reads back to the
// same float64, so float64(0.1) is decimal 1/10. String-backed fields provide their
// current string to shared schema-domain checks, never an original source lexeme.
func AssessModel(ctx context.Context, document Document, options AssessmentOptions) (validation.Report, error) {
	if assessment, dictionaries, ok := options.scenario(ProfileModelSemantics); ok {
		return assessModelWithDictionaries(ctx, document, dictionaryAssessmentOptions{Assessment: assessment, Dictionaries: dictionaries})
	}
	report, _, err := assessModel(ctx, document, options, modelWalkOptions{})
	return report, err
}

// Root entry points create one account; child orchestration calls the configured
// constructor below and never recurses through AssessModel.
func assessModel(ctx context.Context, document Document, options AssessmentOptions, walkOptions modelWalkOptions) (validation.Report, modelWalkStatus, error) {
	if err := assessmentContext(ctx, "canceled-before-model"); err != nil {
		return validation.Report{}, modelWalkStatus{}, err
	}
	engine, err := modelAssessmentEngineForProfile(options.Profile)
	if err != nil {
		return validation.Report{}, modelWalkStatus{}, err
	}
	account, err := validation.NewBudgetAccount(options.Budgets)
	if err != nil {
		return validation.Report{}, modelWalkStatus{}, err
	}
	config := assessmentRunConfig{Account: account, DocumentID: modelDocumentID, Profile: options.Profile, Engine: engine}
	return assessModelWithConfig(ctx, document, config, walkOptions)
}

func assessModelWithConfig(ctx context.Context, document Document, config assessmentRunConfig, walkOptions modelWalkOptions) (validation.Report, modelWalkStatus, error) {
	run, err := newModelAssessmentRun(ctx, document, config)
	if err != nil {
		return validation.Report{}, modelWalkStatus{}, err
	}
	status, walkErr := run.walk(walkOptions)
	report, finishErr := run.finish(walkErr)
	return report, status, errors.Join(walkErr, finishErr)
}

type modelAssessmentRun struct {
	ctx                   context.Context
	document              Document
	account               *validation.Account
	session               *validation.Session
	documentID, entry     string
	xmlDepth              int
	walked, inputFinished bool
	walkErr               error
}

func newModelAssessmentRun(ctx context.Context, document Document, config assessmentRunConfig) (*modelAssessmentRun, error) {
	if err := assessmentContext(ctx, "canceled-before-model"); err != nil {
		return nil, err
	}
	if err := config.validate(validation.TargetModel); err != nil {
		return nil, err
	}
	// This sealed root check does not walk fields or invoke any model method.
	if _, err := validationSchemaForDocument(document); err != nil {
		return nil, err
	}
	session, err := config.begin(ctx)
	if err != nil {
		return nil, err
	}
	return &modelAssessmentRun{ctx: ctx, document: document, account: config.Account, session: session, documentID: config.DocumentID, entry: config.Entry, xmlDepth: config.XMLDepth}, nil
}

func (r *modelAssessmentRun) walk(options modelWalkOptions) (modelWalkStatus, error) {
	if r.walked || r.inputFinished {
		return modelWalkStatus{}, &validation.SessionError{Operation: "model walk", Reason: "input already traversed or finalized"}
	}
	r.walked = true
	options.Entry = r.entry
	options.XMLDepth = r.xmlDepth
	status, err := walkModel(r.ctx, r.document, r.session, r.account, r.documentID, options)
	r.walkErr = err
	return status, err
}

// The walker releases its own scratch before this seam; the session still owns
// intrinsic facts until exactly one ordinary or resource-snapshot finalizer.
func (r *modelAssessmentRun) finishInput(cause error) (validation.Completion, error) {
	if r.inputFinished {
		return validation.Completion{}, &validation.SessionError{Operation: "model finish", Reason: "input already finalized"}
	}
	r.inputFinished = true
	r.document = nil
	if r.walkErr != nil && !errors.Is(cause, r.walkErr) {
		cause = errors.Join(r.walkErr, cause)
	}
	return validation.Completion{InputComplete: r.walked && cause == nil, Syntax: validation.SyntaxNotAssessed, Cause: cause}, nil
}

func (r *modelAssessmentRun) finish(cause error) (validation.Report, error) {
	completion, err := r.finishInput(cause)
	if err != nil {
		return validation.Report{}, err
	}
	return r.session.Finish(r.ctx, completion)
}
