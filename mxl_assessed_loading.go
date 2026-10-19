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

// DecodeMXLAssessmentOptions configures strict loading of an MXL archive
// (scenario 2). Assessment selects resources, opus links, dictionaries,
// budgets, DTDs and the MXL limits. Its Graph.Profile must be empty or
// ProfileMXLSourceLine: the validation lines are fixed. Nil Graph.Dictionaries selects the bundled required
// data; explicit dictionaries must include canonical SMuFL and IPA
// representation relations, as for DecodeAssessed.
type DecodeMXLAssessmentOptions struct {
	Assessment MXLAssessmentOptions
	// Lenient returns the converted package even when an assessment finds
	// violations or is incomplete, with err == nil and DecodeMXLResult.Accepted
	// false. Operational failures still return an error and no package.
	Lenient bool
}

// DecodeMXLResult preserves the bundles of validation line 2 (SourceLine, the
// original archive) and line 3 (ModelLine, the converted package) separately.
// Package is non-nil only after conversion and, in the default strict mode,
// only when Accepted.
type DecodeMXLResult struct {
	Package    *MXLPackage
	SourceLine validation.AssessmentBundle
	// ModelLine is the bundle of the converted package's current state, as
	// AssessMXLPackage reports it with ProfileMXLModelSemantics.
	ModelLine validation.AssessmentBundle
	// Conversion carries the conversion state only; a conversion failure is
	// also the returned *ConversionError.
	Conversion validation.ConversionResult
	// Accepted reports that conversion succeeded, the source bundle was
	// satisfied and the model assessment was accepted.
	Accepted bool
}

// DecodeMXLAssessed is scenario 2, strict loading of an MXL archive. It reads
// the archive once and runs validation line 2 on it, as InspectMXL does with
// ProfileMXLSourceLine: the original archive bytes and schema hints that the
// converted package does not keep and the first rootfile's media type, with
// the first rootfile document, which conversion decodes, assessed by
// ProfileSourceLine. Line 2 follows no resource links, asks the caller's
// resolver for nothing and leaves later rootfiles, which the package keeps as
// bytes, to line 3. It then converts the archive as DecodeMXLPackageContext
// does, decoding the root document and the container from the DTD infoset
// that line 2 assessed, and runs line 3 on the current
// state of the whole converted package, as AssessMXLPackage does with
// ProfileMXLModelSemantics, including the linked files the package retains as
// bytes. AssessMXLPackage on result.Package with the same Assessment options
// and an empty Graph.Profile repeats the model stage.
//
// Every option is checked before the reader is touched. The archive is read
// once into a buffer and indexed for line 2, both metered as in InspectMXL.
// The resources, resolver and dependencies serve line 3; line 2 validates
// them and dispatches no resolver request. The lines
// share one account bounded by Assessment.Graph.Budgets; only line 3 imports
// the dictionaries, and an import that fails but returns a set still lets it
// report the unavailable evidence, while the package is withheld. The archive bytes stay charged to
// that account until conversion finishes, and the line-2 bundle until the
// call returns. Conversion parses the same bytes
// again as DecodeMXLPackageContext does, so the converted package is bounded
// by the MXL limits, as for DecodeMXLPackage. A reader failure or a corrupt
// archive matches ErrInvalidMXL and leaves SourceLine the zero bundle; an
// archive layout the assessment does not process, such as one with a preamble,
// is reported in SourceLine, as by InspectMXL, and its terminated assessment returns
// an error that matches ErrIncompleteAssessment.
//
// In the default strict mode the archive is converted and line 3 runs only
// after line 2 accepts the archive, and Package is returned only when both
// bundles are accepted; a rejection returns an error that matches
// ErrInvalidDocument for violations or ErrIncompleteAssessment otherwise. In
// the lenient mode the archive is converted, line 3 runs and Package is
// returned after any line-2 stage without an operational failure. In both
// modes an operational failure returns its error and no package: invalid
// options, cancellation, an unreadable or unsafe archive, resource limits, a
// terminated assessment, a dictionary import failure or a conversion failure.
// The reader is never closed.
func DecodeMXLAssessed(ctx context.Context, reader io.Reader, options DecodeMXLAssessmentOptions) (result DecodeMXLResult, err error) {
	result.Conversion.State = validation.ConversionIncomplete
	if err := assessmentContext(ctx, "canceled-before-source"); err != nil {
		return result, err
	}
	if nilValue(reader) {
		return result, ErrNilReader
	}
	limits, err := options.Assessment.MXL.limits()
	if err != nil {
		return result, err
	}
	graph := options.Assessment.Graph
	if graph.Profile != "" && graph.Profile != ProfileMXLSourceLine {
		return result, &validation.ConfigurationError{Field: "assessed loading profile", Reason: "requires mxl-source-line; use InspectMXL for individually selected profiles"}
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
	// Every option is checked before the reader is touched. The model stage
	// uses the same options under its own fixed profile.
	if _, err := validateLinkedAssessmentOptionsAccount(ctx, stage(ProfileMXLSourceLine), ProfileMXLSourceLine, account); err != nil {
		return result, err
	}
	// Conversion resolves DTDs as the source stage does: Graph.DTD first.
	limits.dtd = stage(ProfileMXLSourceLine).Graph.DTD
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
	// Line 2 reads no dictionaries; only line 3 imports them.
	source, err := startSourceLineResourceGraph(ctx, stage(ProfileMXLSourceLine), account)
	if source == nil {
		archive.Close()
		return result, err
	}
	// The source bundle stays charged until the loader returns, so the later
	// stages are bounded by the combined live storage.
	source.retainBundle = true
	if err != nil {
		archive.Close()
		result.SourceLine, sourceErr = source.finish(err)
	} else {
		source.mxlOptions = options.Assessment.MXL
		// An archive the assessment cannot process, such as one with a
		// preamble, is reported in the bundle, as by InspectMXL.
		result.SourceLine, sourceErr = source.inspectOpenedRawPackage(archive, openErr)
	}
	defer func() { err = errors.Join(err, source.bundleLease.Close()) }()
	sourceAccepted := result.SourceLine.Satisfied()
	if sourceErr != nil {
		if options.Lenient || sourceAccepted {
			return result, sourceErr
		}
		return result, errors.Join(sourceErr, bundleAcceptanceError("mxl-source", result.SourceLine))
	}
	if !sourceAccepted && !options.Lenient {
		return result, bundleAcceptanceError("mxl-source", result.SourceLine)
	}

	if decodeMXLStageHook != nil {
		decodeMXLStageHook("conversion", account)
	}
	// The model comes from the same DTD infoset the source stage assessed.
	limits.infoset = account
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

	// The model stage assesses the converted package as AssessMXLPackage does.
	model, err := startDictionaryResourceGraphAccount(ctx, stage(ProfileMXLModelSemantics), dictionaries, ProfileMXLModelDictionaries, account)
	if model == nil {
		return result, err
	}
	var modelErr error
	if err != nil {
		result.ModelLine, modelErr = model.finish(err)
	} else {
		model.mxlOptions = options.Assessment.MXL
		result.ModelLine, modelErr = model.assessCurrentPackage(pkg)
	}
	modelAccepted := result.ModelLine.Satisfied()
	if modelErr != nil {
		if options.Lenient || modelAccepted {
			return result, modelErr
		}
		return result, errors.Join(modelErr, bundleAcceptanceError("mxl-model", result.ModelLine))
	}
	if !modelAccepted && !options.Lenient {
		return result, bundleAcceptanceError("mxl-model", result.ModelLine)
	}
	result.Package = pkg
	result.Accepted = sourceAccepted && modelAccepted
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
