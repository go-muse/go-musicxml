package musicxml

import (
	"context"
	"errors"
	"io"

	"github.com/go-muse/go-musicxml/validation"
)

func dictionaryAssessmentConfig(ctx context.Context, options dictionaryAssessmentOptions, target validation.Target) (assessmentRunConfig, error) {
	return dictionaryAssessmentConfigAccount(ctx, options, target, nil)
}

// dictionaryAssessmentConfigAccount imports on account, or on a new account
// bounded by the options' budgets when account is nil.
func dictionaryAssessmentConfigAccount(ctx context.Context, options dictionaryAssessmentOptions, target validation.Target, account *validation.Account) (assessmentRunConfig, error) {
	var cfg assessmentRunConfig
	if err := assessmentContext(ctx, "canceled-before-dictionaries"); err != nil {
		return cfg, err
	}
	if err := validateDictionaryOptions(options.Dictionaries); err != nil {
		return cfg, err
	}
	var engine *validation.Engine
	var err error
	switch {
	case target == validation.TargetSource && (options.Assessment.Profile == profileSourceDictionaries || options.Assessment.Profile == profileSourceSoundsDictionaries || options.Assessment.Profile == profileSourceInternalDTDDictionaries || options.Assessment.Profile == profileSourceInternalDTDSoundsDictionaries || options.Assessment.Profile == profileSourceSemantics):
		engine, err = sourceAssessmentEngineForProfile(options.Assessment.Profile)
		cfg.DocumentID = sourceDocumentID
	case target == validation.TargetModel && (options.Assessment.Profile == profileModelDictionaries || options.Assessment.Profile == profileModelSemantics):
		engine, err = modelAssessmentEngineForProfile(options.Assessment.Profile)
		cfg.DocumentID = modelDocumentID
	default:
		return cfg, &validation.ConfigurationError{Field: "dictionary profile", Reason: "explicit compatible dictionary profile required"}
	}
	if err != nil {
		return cfg, err
	}
	if account == nil {
		if account, err = validation.NewBudgetAccount(options.Assessment.Budgets); err != nil {
			return cfg, err
		}
	}
	set, loadErr := loadValidatedDictionarySet(ctx, account, options.Dictionaries)
	if dictionaryImportTerminal(loadErr) {
		if set != nil {
			loadErr = errors.Join(loadErr, set.Close())
		}
		return cfg, loadErr
	}
	if set == nil && loadErr == nil {
		return cfg, &validation.ConfigurationError{Field: "dictionary capability", Reason: "loader returned neither a set nor a failure"}
	}
	cfg.Account, cfg.Profile, cfg.Engine, cfg.Dictionaries = account, options.Assessment.Profile, engine, set
	cfg.OmitContext = options.Assessment.OmitContext
	return cfg, loadErr
}

// inspectXMLWithDictionaries assesses original bytes with explicitly selected
// pinned dictionary capabilities, for InspectXML with Dictionaries or an empty
// profile. It does not load a dictionary implicitly. Only the explicit
// internal-DTD dictionary profile elects DTD composition. The caller owns its
// reader; this call closes only streams returned by the selected
// DictionarySource.
func inspectXMLWithDictionaries(ctx context.Context, reader io.Reader, options dictionaryAssessmentOptions) (report validation.Report, err error) {
	if nilValue(reader) {
		return report, ErrNilReader
	}
	cfg, loadErr := dictionaryAssessmentConfig(ctx, options, validation.TargetSource)
	if cfg.Dictionaries == nil {
		return report, loadErr
	}
	cfg.DTD = options.Assessment.DTD
	defer func() { err = errors.Join(err, cfg.Dictionaries.Close()) }()
	run, startErr := newSourceAssessmentRunWithConfig(ctx, reader, DecodeOptions{}, &sourceAssessmentConfig{assessmentRunConfig: cfg, ByteOwner: sourceBytesPrimary}, false)
	if startErr != nil {
		if run != nil {
			report = run.startupReport
		}
		return report, errors.Join(loadErr, startErr)
	}
	inputErr := run.drain()
	report, finishErr := run.finish(inputErr)
	return report, errors.Join(loadErr, inputErr, finishErr)
}

// assessModelWithDictionaries directly assesses the current generated model
// and selected dictionary evidence, for AssessModel with Dictionaries or an
// empty profile. It never reconstructs original lexical values, namespaces or
// discarded xsi:type declarations by encoding the model.
func assessModelWithDictionaries(ctx context.Context, document Document, options dictionaryAssessmentOptions) (report validation.Report, err error) {
	if err := assessmentContext(ctx, "canceled-before-model"); err != nil {
		return report, err
	}
	if _, err := validationSchemaForDocument(document); err != nil {
		return report, err
	}
	cfg, loadErr := dictionaryAssessmentConfig(ctx, options, validation.TargetModel)
	if cfg.Dictionaries == nil {
		return report, loadErr
	}
	defer func() { err = errors.Join(err, cfg.Dictionaries.Close()) }()
	report, _, runErr := assessModelWithConfig(ctx, document, cfg, modelWalkOptions{})
	return report, errors.Join(loadErr, runErr)
}
