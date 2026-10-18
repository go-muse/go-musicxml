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
	Model   validation.Report
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
// on it stays incomplete. Both assessments share one account bounded by
// Assessment.Graph.Budgets, and each imports its dictionaries. The archive
// bytes held for the call are charged to that account. The converted package
// itself is bounded by the MXL limits, as for DecodeMXLPackage.
//
// In the default strict mode the model is assessed only after the archive is
// accepted, and Package is returned only when both assessments are accepted;
// a rejection returns an error that matches ErrInvalidDocument for violations
// or ErrIncompleteAssessment otherwise. In the lenient mode the model is
// assessed and Package returned after any successful conversion. In both
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
	data, err := readMXLAssessedArchive(ctx, reader, limits.archiveSize, account)
	if err != nil {
		return result, err
	}
	defer func() { _ = account.Release(validation.BudgetBufferedBytes, int64(len(data))) }()
	if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		return result, fmt.Errorf("%w: open ZIP archive: %w", ErrInvalidMXL, err)
	}

	var sourceErr error
	source, err := startDictionaryResourceGraphAccount(ctx, stage(ProfileMXLSourceSemantics), dictionaries, ProfileMXLSourceDictionaries, account)
	if source == nil {
		return result, err
	}
	if err != nil {
		result.Source, sourceErr = source.finish(err)
	} else {
		source.mxlOptions = options.Assessment.MXL
		result.Source, sourceErr = source.inspectRawPackage(bytes.NewReader(data))
	}
	acceptanceErr := bundleAcceptanceError("mxl-source", result.Source)
	if sourceErr != nil {
		if options.Lenient {
			return result, sourceErr
		}
		return result, errors.Join(sourceErr, acceptanceErr)
	}
	if acceptanceErr != nil && !options.Lenient {
		return result, acceptanceErr
	}

	pkg, convErr := decodeMXLPackageContext(ctx, bytes.NewReader(data), limits)
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
	acceptanceErr = errors.Join(acceptanceErr, modelAcceptanceError(result.Model, status))
	if acceptanceErr != nil && !options.Lenient {
		return result, acceptanceErr
	}
	result.Package = pkg
	result.Accepted = acceptanceErr == nil
	return result, nil
}

// bundleAcceptanceError describes a bundle that is not satisfied: failed rule
// evaluations of its package and resource reports as a *ValidationError, and
// an IncompleteError when anything else kept it from being satisfied.
func bundleAcceptanceError(stage string, bundle validation.AssessmentBundle) error {
	if bundle.Satisfied() {
		return nil
	}
	reports := []validation.Report{bundle.PackageReport()}
	for i := 0; i < bundle.ResourceCount(); i++ {
		if _, report, ok := bundle.ResourceReport(i); ok {
			reports = append(reports, report)
		}
	}
	var issues []ValidationIssue
	violatedOnly := bundle.EnumerationComplete()
	for _, report := range reports {
		for _, finding := range report.Findings {
			if finding.Outcome == validation.OutcomeFail && (finding.Category == validation.CategoryNormative || finding.Category == validation.CategoryModel) {
				issues = append(issues, assessmentIssue(finding))
			}
		}
		if !report.Satisfied() && (report.Conformance != validation.ConformanceViolated || !report.AssessmentComplete || !report.InputComplete || report.Terminal != nil || report.Counts.Unknown != 0 || report.Counts.Unsupported != 0 || report.Counts.Blocked != 0) {
			violatedOnly = false
		}
	}
	var invalid error
	if len(issues) != 0 {
		invalid = &ValidationError{Issues: issues}
	}
	if invalid == nil || !violatedOnly {
		return errors.Join(invalid, &validation.IncompleteError{Reason: stage + "-profile-not-satisfied"})
	}
	return invalid
}

// readMXLAssessedArchive reads the archive within the MXL archive limit and
// the account's remaining input and buffered-byte budgets, and leaves the
// bytes charged as buffered. Exceeding the archive limit is ErrInvalidMXL, as
// in DecodeMXL; exceeding a budget is a *validation.ResourceError.
func readMXLAssessedArchive(ctx context.Context, reader io.Reader, archiveSize int64, account *validation.Account) ([]byte, error) {
	type archiveBudget struct {
		kind        validation.BudgetKind
		limit, used int64
	}
	limits := account.Limits()
	usedInput, _ := account.Usage(validation.BudgetInputBytes)
	usedBuffered, _ := account.Usage(validation.BudgetBufferedBytes)
	budgets := []archiveBudget{
		{validation.BudgetInputBytes, limits.MaxInputBytes, usedInput},
		{validation.BudgetBufferedBytes, limits.MaxBufferedBytes, usedBuffered},
	}
	var binding *archiveBudget
	limit := archiveSize
	for i := range budgets {
		if remaining := budgets[i].limit - budgets[i].used; remaining < limit {
			limit, binding = max(remaining, 0), &budgets[i]
		}
	}
	var data []byte
	var err error
	if binding == nil {
		data, err = readMXLStream(mxlContextReader(ctx, reader), archiveSize, "archive")
	} else {
		data, err = io.ReadAll(io.LimitReader(mxlContextReader(ctx, reader), limit+1))
		if err != nil {
			err = fmt.Errorf("%w: read archive: %w", ErrInvalidMXL, err)
		}
	}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}
	if binding != nil && int64(len(data)) > limit {
		return nil, &validation.ResourceError{Kind: binding.kind, Limit: binding.limit, Used: binding.used, Requested: int64(len(data))}
	}
	if err := account.Charge(validation.BudgetBufferedBytes, int64(len(data))); err != nil {
		return nil, err
	}
	return data, nil
}
