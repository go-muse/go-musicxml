package musicxml

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/go-muse/go-musicxml/validation"
)

// assessedLoadingOptions elects fixed validation stages, never a caller-selected
// weaker profile. Explicit dictionary inputs remain authoritative: an unavailable
// or invalid override is not repaired with bundled data.
func assessedLoadingOptions(options *dictionaryAssessmentOptions) (dictionaryAssessmentOptions, error) {
	var out dictionaryAssessmentOptions
	if options != nil {
		out = *options
	}
	if out.Assessment.Profile != "" && out.Assessment.Profile != ProfileSourceSemantics {
		return out, &validation.ConfigurationError{Field: "assessed loading profile", Reason: "requires source-semantics; use InspectXML for individually selected profiles"}
	}
	out.Assessment.Profile = ProfileSourceSemantics
	required := requiredDictionarySelections()
	// A typed-nil Source is an explicit override, not a request for defaults.
	if out.Dictionaries.Source == nil && len(out.Dictionaries.Selections) == 0 {
		out.Dictionaries = requiredDictionaryOptions()
		return out, nil
	}
	return out, requireDictionaryRelations(out.Dictionaries, required)
}

// requireDictionaryRelations accepts explicit dictionary options only when
// they select every required relation.
func requireDictionaryRelations(dictionaries DictionaryOptions, required []validation.DictionarySelection) error {
	if err := validateDictionaryOptions(dictionaries); err != nil {
		return err
	}
	for _, want := range required {
		found := false
		for _, selected := range dictionaries.Selections {
			if selected.Manifest != want.Manifest {
				continue
			}
			for _, relation := range selected.Relations {
				if relation == want.Relations[0] {
					found = true
				}
			}
		}
		if !found {
			return &validation.ConfigurationError{Field: "assessed loading dictionaries", Reason: "explicit selections must include canonical SMuFL and IPA representation relations"}
		}
	}
	return nil
}

// decodeValidated owns one account and one imported set across both stages.
// Inspect/Assess entry points continue to use their explicit profile contracts.
func decodeValidated(ctx context.Context, reader io.Reader, decode DecodeOptions, options *dictionaryAssessmentOptions, lenient bool) (result DecodeResult, err error) {
	result.Conversion.State = validation.ConversionIncomplete
	if err = assessmentContext(ctx, "canceled-before-source"); err != nil {
		return result, err
	}
	if dictionaryNil(reader) {
		return result, ErrNilReader
	}
	if _, err = decode.xmlDepth(); err != nil {
		return result, err
	}
	normalized, err := assessedLoadingOptions(options)
	if err != nil {
		return result, err
	}
	modelEngine, err := modelAssessmentEngineForProfile(ProfileModelSemantics)
	if err != nil {
		return result, err
	}
	cfg, loadErr := dictionaryAssessmentConfig(ctx, normalized, validation.TargetSource)
	if cfg.Dictionaries == nil {
		return result, loadErr
	}
	cfg.DTD = normalized.Assessment.DTD
	defer func() {
		err = errors.Join(err, cfg.Dictionaries.Close())
		if err != nil {
			result.Document, result.Accepted = nil, false
		}
	}()
	run, startErr := newSourceAssessmentRunWithConfig(ctx, reader, decode, &sourceAssessmentConfig{assessmentRunConfig: cfg, ByteOwner: sourceBytesPrimary}, true)
	if startErr != nil {
		if run != nil {
			result.Source = run.startupReport
		}
		return result, errors.Join(loadErr, startErr)
	}
	modelConfig := cfg
	modelConfig.Profile, modelConfig.Engine, modelConfig.DocumentID = ProfileModelSemantics, modelEngine, modelDocumentID
	return decodeAssessedRun(ctx, run, modelConfig, loadErr, lenient)
}

func modelAcceptanceError(report validation.Report, status modelWalkStatus) error {
	if report.Satisfied() && !status.HasProblems {
		return nil
	}
	var invalid error
	if report.Conformance == validation.ConformanceViolated || report.Counts.Fail != 0 || status.HasProblems {
		issues := make([]ValidationIssue, 0)
		for _, finding := range report.Findings {
			if finding.Outcome == validation.OutcomeFail && (finding.Category == validation.CategoryNormative || finding.Category == validation.CategoryModel) {
				issues = append(issues, assessmentIssue(finding))
			}
		}
		invalid = &ValidationError{Issues: issues}
		if report.DiagnosticsTruncated {
			invalid = fmt.Errorf("%w; model assessment: failed rule evaluations: %d; diagnostic details were truncated", invalid, report.Counts.Fail)
		}
	}
	if !report.Requested || !report.InputComplete || !report.AssessmentComplete || report.Target != validation.TargetModel || report.Profile.ID != ProfileModelSemantics || report.Syntax != validation.SyntaxNotAssessed || report.BlockingProblems != 0 || report.Terminal != nil || report.Counts.Unknown != 0 || report.Counts.Unsupported != 0 || report.Counts.Blocked != 0 || invalid == nil {
		return errors.Join(invalid, &validation.IncompleteError{Reason: "model-profile-not-satisfied"})
	}
	return invalid
}

// assessmentIssue maps a failed finding as Validate does: Constraint is the
// reason code, Message the offending value and Rule the rule ID.
func assessmentIssue(finding validation.Finding) ValidationIssue {
	return ValidationIssue{Path: finding.Location.Path, Constraint: string(finding.Reason), Message: finding.Actual, Rule: string(finding.Rule.ID)}
}
