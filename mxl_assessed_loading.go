package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/go-muse/go-musicxml/validation"
)

// DecodeMXLAssessmentOptions configures strict loading of an MXL archive.
// Assessment selects resources, opus links, dictionaries, budgets, DTDs and
// the MXL limits. Its Graph.Profile must be empty or ProfileMXLSourceSemantics:
// the stages are fixed. Nil Graph.Dictionaries selects the bundled required
// data; explicit dictionaries must include canonical SMuFL and IPA
// representation relations, as for DecodeAssessed.
type DecodeMXLAssessmentOptions struct {
	Assessment MXLAssessmentOptions
	// Lenient returns the converted package even when an assessment finds
	// violations or is incomplete, with err == nil and DecodeMXLResult.Accepted
	// false. Operational failures still return an error and no package.
	Lenient bool
}

// DecodeMXLResult preserves the original-archive bundle and the primary
// document's model report separately. Package is non-nil only after
// conversion and, in the default strict mode, only when Accepted.
type DecodeMXLResult struct {
	Package *MXLPackage
	Source  validation.AssessmentBundle
	// Model is the report of the primary document only. The other resources
	// of the package are assessed in Source, from the original archive.
	Model validation.Report
	// Conversion carries the conversion state only; a conversion failure is
	// also the returned *ConversionError.
	Conversion validation.ConversionResult
	// Accepted reports that conversion succeeded, the source bundle was
	// satisfied and the model assessment was accepted.
	Accepted bool
}

// DecodeMXLAssessed reads an MXL archive once, assesses the original archive
// as InspectMXL does with ProfileMXLSourceSemantics, converts it as
// DecodeMXLPackageContext does and assesses the primary document's current
// model as DecodeAssessed and AssessModel do with ProfileModelSemantics.
// Package rules are proved from the original archive: a decoded package
// cannot carry evidence such as discarded schema hints, so AssessMXLPackage
// on it stays incomplete. The model stage covers the primary document only;
// AssessModel(result.Package.Document) repeats it, but neither reassesses the
// whole package.
//
// Every option is checked before the reader is touched. The archive is read
// once into a buffer and indexed for the source stage, both metered as in
// InspectMXL. Both assessments share one account bounded by
// Assessment.Graph.Budgets, and each imports its dictionaries. The archive
// bytes stay charged to that account until conversion finishes, and the
// source bundle until the call returns. Conversion parses the same bytes again
// as DecodeMXLPackageContext does, so the converted package is bounded by the
// MXL limits, as for DecodeMXLPackage. A reader failure or a corrupt archive
// matches ErrInvalidMXL and leaves Source the zero bundle; an archive layout
// the assessment does not process, such as one with a preamble, is reported in
// Source, as by InspectMXL, and its terminated assessment returns an error that
// matches ErrIncompleteAssessment.
//
// In the default strict mode the archive is converted and its model assessed
// only after the source bundle is accepted, and Package is returned only when
// both assessments are accepted; a rejection returns an error that matches
// ErrInvalidDocument for violations or ErrIncompleteAssessment otherwise. In
// the lenient mode the archive is converted, the model assessed and Package
// returned after any source stage without an operational failure. In both
// modes an operational failure returns its error and no package: invalid
// options, cancellation, an unreadable or unsafe archive, resource limits, a
// terminated assessment, a dictionary import failure or a conversion failure.
// The reader is never closed.
func DecodeMXLAssessed(ctx context.Context, reader io.Reader, options DecodeMXLAssessmentOptions) (result DecodeMXLResult, err error) {
	result.Conversion.State = validation.ConversionIncomplete
	if err := assessmentContext(ctx, "canceled-before-source"); err != nil {
		return result, err
	}
	if dictionaryNil(reader) {
		return result, ErrNilReader
	}
	limits, err := options.Assessment.MXL.limits()
	if err != nil {
		return result, err
	}
	graph := options.Assessment.Graph
	if graph.Profile != "" && graph.Profile != ProfileMXLSourceSemantics {
		return result, &validation.ConfigurationError{Field: "assessed loading profile", Reason: "requires mxl-source-semantics; use InspectMXL for individually selected profiles"}
	}
	dictionaries := requiredDictionaryOptions()
	// As for DecodeAssessed, a nil Source with no selections asks for the
	// defaults and a typed-nil Source is an explicit override.
	if graph.Dictionaries != nil && (graph.Dictionaries.Source != nil || len(graph.Dictionaries.Selections) != 0) {
		dictionaries = *graph.Dictionaries
		if err := requireDictionaryRelations(dictionaries, requiredDictionarySelections()); err != nil {
			return result, err
		}
	}
	account, err := validation.NewBudgetAccount(graph.Budgets)
	if err != nil {
		return result, err
	}
	stage := func(profile validation.ProfileID) LinkedAssessmentOptions {
		linked := options.Assessment.linked()
		linked.Graph.Profile, linked.Graph.Dictionaries = profile, nil
		return linked
	}
	// Every option is checked before the reader is touched.
	if _, err := validateLinkedAssessmentOptionsAccount(ctx, stage(ProfileMXLSourceSemantics), ProfileMXLSourceSemantics, account); err != nil {
		return result, err
	}
	// Conversion resolves DTDs as the source stage does: Graph.DTD first.
	limits.dtd = stage(ProfileMXLSourceSemantics).Graph.DTD
	// The archive is read and indexed on the shared account, metered as in
	// InspectMXL: reads, buffer growth and the ZIP inventory are admitted
	// before they happen.
	recorder := &mxlReadErrorRecorder{reader: reader}
	archive, openErr := openMXLAssessmentArchive(ctx, recorder, options.Assessment.MXL, account)
	if fatal := decodeMXLOpenError(ctx, archive, openErr, recorder.err); fatal != nil {
		archive.Close()
		return result, fatal
	}
	// The loader keeps the archive bytes and their reservation for conversion;
	// the source stage reads them until it closes the archive.
	data, reserved := archive.detachData()
	releaseData := func() {
		if reserved != 0 {
			_ = account.Release(validation.BudgetBufferedBytes, reserved)
			data, reserved = nil, 0
		}
	}
	defer releaseData()

	var sourceErr error
	source, err := startDictionaryResourceGraphAccount(ctx, stage(ProfileMXLSourceSemantics), dictionaries, ProfileMXLSourceDictionaries, account)
	if source == nil {
		archive.Close()
		return result, err
	}
	// The source bundle stays charged until the loader returns, so the later
	// stages are bounded by the combined live storage.
	source.retainBundle = true
	if err != nil {
		archive.Close()
		result.Source, sourceErr = source.finish(err)
	} else {
		source.mxlOptions = options.Assessment.MXL
		// An archive the assessment cannot process, such as one with a
		// preamble, is reported in the bundle, as by InspectMXL.
		result.Source, sourceErr = source.inspectOpenedRawPackage(archive, openErr)
	}
	defer func() { err = errors.Join(err, source.bundleLease.Close()) }()
	sourceAccepted := result.Source.Satisfied()
	if sourceErr != nil {
		if options.Lenient || sourceAccepted {
			return result, sourceErr
		}
		return result, errors.Join(sourceErr, bundleAcceptanceError("mxl-source", result.Source))
	}
	if !sourceAccepted && !options.Lenient {
		return result, bundleAcceptanceError("mxl-source", result.Source)
	}

	if decodeMXLStageHook != nil {
		decodeMXLStageHook("conversion", account)
	}
	pkg, convErr := decodeMXLPackageContext(ctx, bytes.NewReader(data), limits)
	releaseData()
	if convErr != nil {
		state := validation.ConversionFailed
		switch {
		case ctx.Err() != nil:
			state = validation.ConversionIncomplete
		case errors.Is(convErr, strconv.ErrRange):
			state = validation.ConversionUnrepresentable
		}
		result.Conversion.State = state
		return result, &ConversionError{State: state, Cause: convErr}
	}
	result.Conversion.State = validation.ConversionConverted
	if decodeMXLStageHook != nil {
		decodeMXLStageHook("model", account)
	}

	modelEngine, err := modelAssessmentEngineForProfile(ProfileModelSemantics)
	if err != nil {
		return result, err
	}
	set, loadErr := loadValidatedDictionarySet(ctx, account, dictionaries)
	if set == nil || loadErr != nil {
		if set != nil {
			loadErr = errors.Join(loadErr, set.Close())
		}
		if loadErr == nil {
			loadErr = &validation.ConfigurationError{Field: "dictionary set", Reason: "loader returned neither a set nor a failure"}
		}
		return result, loadErr
	}
	config := assessmentRunConfig{Account: account, DocumentID: modelDocumentID, Profile: ProfileModelSemantics, Engine: modelEngine, Dictionaries: set}
	var status modelWalkStatus
	var modelErr error
	result.Model, status, modelErr = assessModelWithConfig(ctx, pkg.Document, config, modelWalkOptions{})
	if modelErr = errors.Join(modelErr, set.Close()); modelErr != nil {
		return result, modelErr
	}
	acceptanceErr := modelAcceptanceError(result.Model, status)
	if acceptanceErr != nil && !options.Lenient {
		return result, acceptanceErr
	}
	result.Package = pkg
	result.Accepted = sourceAccepted && acceptanceErr == nil
	return result, nil
}

// mxlReadErrorRecorder keeps the last failure of the caller's reader, so that
// it can be told apart from archive and budget failures.
type mxlReadErrorRecorder struct {
	reader io.Reader
	err    error
}

func (r *mxlReadErrorRecorder) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}

// decodeMXLOpenError returns the error that ends the loader when the archive
// cannot be read or indexed, classified as DecodeMXL does: a reader failure,
// including a reader that makes no progress, and a corrupt archive match
// ErrInvalidMXL; cancellation, limits, budgets and configuration keep their
// own errors. Any other failure of a completely read archive, such as an
// archive layout the assessment does not process, returns nil: the source
// stage reports it in the bundle.
func decodeMXLOpenError(ctx context.Context, archive *mxlAssessmentArchive, err, readErr error) error {
	if err == nil {
		if archive == nil {
			return &validation.ConfigurationError{Field: "archive", Reason: "opener returned neither an archive nor a failure"}
		}
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
		return err
	}
	var resource *validation.ResourceError
	var configuration *validation.ConfigurationError
	switch {
	case readErr != nil, errors.Is(err, io.ErrNoProgress):
		return fmt.Errorf("%w: read archive: %w", ErrInvalidMXL, err)
	case errors.Is(err, zip.ErrFormat):
		return fmt.Errorf("%w: open ZIP archive: %w", ErrInvalidMXL, err)
	case errors.As(err, &resource), errors.As(err, &configuration), errors.Is(err, ErrMXLTooLarge), archive == nil, !archive.inputComplete:
		return err
	}
	return nil
}

// bundleAcceptanceError describes a bundle that is not satisfied, from the
// authoritative conformance and counts of its reports: a *ValidationError for
// failed rule evaluations, even when their diagnostics were truncated, and an
// IncompleteError when anything else kept it from being satisfied. Only the
// stored failed findings are copied.
func bundleAcceptanceError(stage string, bundle validation.AssessmentBundle) error {
	if bundle.Satisfied() {
		return nil
	}
	failures := bundle.Failures()
	var invalid error
	if failures.Violated {
		issues := make([]ValidationIssue, 0, len(failures.Findings))
		for _, finding := range failures.Findings {
			issues = append(issues, assessmentIssue(finding))
		}
		invalid = &ValidationError{Issues: issues}
		if failures.Truncated {
			invalid = fmt.Errorf("%w; %s assessment: failed rule evaluations: %d; diagnostic details were truncated", invalid, stage, failures.Failed)
		}
	}
	if invalid == nil || failures.Incomplete {
		return errors.Join(invalid, &validation.IncompleteError{Reason: stage + "-profile-not-satisfied"})
	}
	return invalid
}

// decodeMXLStageHook observes the shared account just before the conversion
// and model stages of DecodeMXLAssessed. Tests set it; it is nil otherwise.
var decodeMXLStageHook func(stage string, account *validation.Account)
