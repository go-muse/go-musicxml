package musicxml

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/go-muse/go-musicxml/validation"
)

// assessedLoadingOptions elects the fixed validation lines, never a
// caller-selected weaker profile. The dictionaries serve line 3, so the
// normalized options name ProfileModelSemantics. Explicit dictionary inputs
// remain authoritative: an unavailable or invalid override is not repaired
// with bundled data.
func assessedLoadingOptions(options *dictionaryAssessmentOptions) (dictionaryAssessmentOptions, error) {
	var out dictionaryAssessmentOptions
	if options != nil {
		out = *options
	}
	if out.Assessment.Profile != "" && out.Assessment.Profile != ProfileSourceLine {
		return out, &validation.ConfigurationError{Field: "assessed loading profile", Reason: "requires source-line; use InspectXML for individually selected profiles"}
	}
	out.Assessment.Profile = ProfileModelSemantics
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

// decodeValidated owns one account across both lines. Line 3 alone reads the
// dictionaries, so the set is imported only once line 3 is due: after line 2
// accepts the source, or after any line 2 without an operational failure in
// the lenient mode. Inspect/Assess entry points continue to use their explicit
// profile contracts.
func decodeValidated(ctx context.Context, reader io.Reader, decode DecodeOptions, options *dictionaryAssessmentOptions, lenient bool) (result DecodeResult, err error) {
	result.Conversion.State = validation.ConversionIncomplete
	if err = assessmentContext(ctx, "canceled-before-source"); err != nil {
		return result, err
	}
	if nilValue(reader) {
		return result, ErrNilReader
	}
	if _, err = decode.xmlDepth(); err != nil {
		return result, err
	}
	normalized, err := assessedLoadingOptions(options)
	if err != nil {
		return result, err
	}
	sourceConfig, err := assessedSourceLine(normalized)
	if err != nil {
		return result, err
	}
	run, startErr := newSourceAssessmentRunWithConfig(ctx, reader, decode, &sourceAssessmentConfig{assessmentRunConfig: sourceConfig, ByteOwner: sourceBytesPrimary}, true)
	if startErr != nil {
		if run != nil {
			result.SourceLine = run.startupReport
		}
		return result, startErr
	}
	return decodeAssessedRun(ctx, run, func() (assessmentRunConfig, error) {
		return assessedModelLine(ctx, normalized, sourceConfig.Account)
	}, lenient)
}

// assessedSourceLine configures line 2 of strict loading, ProfileSourceLine,
// which reads the source without dictionaries, on a new account for both
// lines. Every dictionary option is validated here, before the source is read.
func assessedSourceLine(normalized dictionaryAssessmentOptions) (assessmentRunConfig, error) {
	var source assessmentRunConfig
	if err := validateDictionaryOptions(normalized.Dictionaries); err != nil {
		return source, err
	}
	engine, err := sourceAssessmentEngineForProfile(ProfileSourceLine)
	if err != nil {
		return source, err
	}
	account, err := validation.NewBudgetAccount(normalized.Assessment.Budgets)
	if err != nil {
		return source, err
	}
	source.Account, source.Profile, source.Engine, source.DocumentID = account, ProfileSourceLine, engine, sourceDocumentID
	source.DTD, source.OmitContext = normalized.Assessment.DTD, normalized.Assessment.OmitContext
	return source, nil
}

// assessedModelLine configures line 3, ProfileModelSemantics, on the shared
// account and imports its dictionary set. A nil Dictionaries means the import
// failed and err says why; a set with an error is an ordinary import failure.
func assessedModelLine(ctx context.Context, normalized dictionaryAssessmentOptions, account *validation.Account) (assessmentRunConfig, error) {
	return dictionaryAssessmentConfigAccount(ctx, normalized, validation.TargetModel, account)
}

func modelAcceptanceError(report validation.Report, status modelWalkStatus) error {
	if report.Satisfied() && !status.HasProblems {
		return nil
	}
	var invalid error
	if report.Conformance == validation.ConformanceViolated || report.Counts.Fail != 0 || report.ModelFailures != 0 || status.HasProblems {
		issues := make([]ValidationIssue, 0)
		for _, finding := range report.Findings {
			if finding.Outcome == validation.OutcomeFail && (finding.Category == validation.CategoryNormative || finding.Category == validation.CategoryModel) {
				issues = append(issues, assessmentIssue(finding))
			}
		}
		invalid = &ValidationError{Issues: issues}
		if report.DiagnosticsTruncated {
			invalid = fmt.Errorf("%w; model assessment: failed rule evaluations: %d; diagnostic details were truncated", invalid, report.Counts.Fail+report.ModelFailures)
		}
	}
	if !report.Requested || !report.InputComplete || !report.AssessmentComplete || report.Target != validation.TargetModel || report.Profile.ID != ProfileModelSemantics || report.Syntax != validation.SyntaxNotAssessed || report.BlockingProblems > report.ModelFailures || report.Terminal != nil || report.Counts.Unknown != 0 || report.Counts.Unsupported != 0 || report.Counts.Blocked != 0 || invalid == nil {
		return errors.Join(invalid, &validation.IncompleteError{Reason: "model-profile-not-satisfied"})
	}
	return invalid
}

// assessmentIssue maps a failed finding as Validate does: Constraint is the
// reason code, Message the offending value and Rule the rule ID.
func assessmentIssue(finding validation.Finding) ValidationIssue {
	return ValidationIssue{Path: finding.Location.Path, Constraint: string(finding.Reason), Message: finding.Actual, Rule: string(finding.Rule.ID)}
}
