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
	case target == validation.TargetSource && (options.Assessment.Profile == ProfileSourceDictionaries || options.Assessment.Profile == ProfileSourceSoundsDictionaries || options.Assessment.Profile == ProfileSourceInternalDTDDictionaries || options.Assessment.Profile == ProfileSourceInternalDTDSoundsDictionaries || options.Assessment.Profile == ProfileSourceSemantics):
		engine, err = sourceAssessmentEngineForProfile(options.Assessment.Profile)
		cfg.DocumentID = sourceDocumentID
	case target == validation.TargetModel && (options.Assessment.Profile == ProfileModelDictionaries || options.Assessment.Profile == ProfileModelSemantics):
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

// DecodeAssessedWithDictionaries uses the same mandatory source/model pipeline
// as DecodeAssessed. Nil/empty options use bundled required dictionary data.
// Explicit dictionary options must include canonical SMuFL and IPA representation
// relations. Their Source is honored without fallback, including unavailable or
// failed inputs; selections cannot disable required checks. One set is imported
// and shared by both stages of each call, without cross-call verified-set reuse.
// Any operation or acceptance failure discards Document.
//
// Referencing this loader embeds all five bundled components regardless of
// options, including custom sources. Distribute the notice/license files listed
// in THIRD_PARTY_NOTICES.md. Inputs with unread external DOCTYPE subsets cannot
// satisfy this loader; ordinary Decode provides conversion only, not validation.
func DecodeAssessedWithDictionaries(ctx context.Context, reader io.Reader, options DecodeDictionaryAssessmentOptions) (DecodeResult, error) {
	return decodeValidated(ctx, reader, options.Decode, options.Assessment)
}
