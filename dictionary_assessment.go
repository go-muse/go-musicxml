package musicxml

import (
	"context"
	"errors"
	"io"

	"github.com/go-muse/go-musicxml/validation"
)

func dictionaryAssessmentConfig(ctx context.Context, options DictionaryAssessmentOptions, target validation.Target) (assessmentRunConfig, error) {
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
	case target == validation.TargetSource && (options.Assessment.Profile == ProfileSourceDictionaries || options.Assessment.Profile == ProfileSourceSoundsDictionaries || options.Assessment.Profile == ProfileSourceInternalDTDDictionaries):
		engine, err = sourceAssessmentEngineForProfile(options.Assessment.Profile)
		cfg.DocumentID = sourceDocumentID
	case target == validation.TargetModel && options.Assessment.Profile == ProfileModelDictionaries:
		engine, err = modelAssessmentEngineForProfile(options.Assessment.Profile)
		cfg.DocumentID = modelDocumentID
	default:
		return cfg, &validation.ConfigurationError{Field: "dictionary profile", Reason: "explicit compatible dictionary profile required"}
	}
	if err != nil {
		return cfg, err
	}
	account, err := validation.NewBudgetAccount(options.Assessment.Budgets)
	if err != nil {
		return cfg, err
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
	return cfg, loadErr
}

// InspectXMLWithDictionaries assesses original bytes with explicitly selected
// pinned dictionary capabilities. It does not load a dictionary implicitly.
// Only the explicit internal-DTD dictionary profile elects DTD composition.
// Source owns its reader; this call
// closes only streams returned by the selected DictionarySource.
func InspectXMLWithDictionaries(ctx context.Context, reader io.Reader, options DictionaryAssessmentOptions) (report validation.Report, err error) {
	if dictionaryNil(reader) {
		return report, ErrNilReader
	}
	cfg, loadErr := dictionaryAssessmentConfig(ctx, options, validation.TargetSource)
	if cfg.Dictionaries == nil {
		return report, loadErr
	}
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

// AssessModelWithDictionaries directly assesses the current generated model and
// selected dictionary evidence. It never reconstructs original lexical values,
// namespaces or discarded xsi:type declarations by encoding the model.
func AssessModelWithDictionaries(ctx context.Context, document Document, options DictionaryAssessmentOptions) (report validation.Report, err error) {
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

// DecodeAssessedWithDictionaries converts and assesses through the same source
// parser. Nil Assessment retains DecodeAssessed's unassessed conversion behavior.
// Any conversion, source, dictionary operation or strict acceptance error
// discards the returned model while preserving the best source report.
func DecodeAssessedWithDictionaries(ctx context.Context, reader io.Reader, options DecodeDictionaryAssessmentOptions) (result DecodeResult, err error) {
	if options.Assessment == nil {
		return DecodeAssessed(ctx, reader, DecodeAssessmentOptions{Decode: options.Decode})
	}
	result.Conversion.State = validation.ConversionIncomplete
	if dictionaryNil(reader) {
		return result, ErrNilReader
	}
	if _, err := options.Decode.xmlDepth(); err != nil {
		return result, err
	}
	cfg, loadErr := dictionaryAssessmentConfig(ctx, *options.Assessment, validation.TargetSource)
	if cfg.Dictionaries == nil {
		return result, loadErr
	}
	defer func() {
		err = errors.Join(err, cfg.Dictionaries.Close())
		if err != nil {
			result.Document = nil
		}
	}()
	run, startErr := newSourceAssessmentRunWithConfig(ctx, reader, options.Decode, &sourceAssessmentConfig{assessmentRunConfig: cfg, ByteOwner: sourceBytesPrimary}, true)
	if startErr != nil {
		if run != nil {
			result.Source = run.startupReport
		}
		return result, errors.Join(loadErr, startErr)
	}
	result, runErr := decodeAssessedRun(ctx, run, true)
	return result, errors.Join(loadErr, runErr)
}
