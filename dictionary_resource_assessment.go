package musicxml

import (
	"context"
	"io"

	"github.com/go-muse/go-musicxml/validation"
)

// startDictionaryResourceGraph validates every option before caller I/O, then
// creates the sole root account and imports its immutable capability set once.
// A nonnil graph owns all cleanup even when startup returns an operation error.
func startDictionaryResourceGraph(ctx context.Context, resources LinkedAssessmentOptions, dictionaries DictionaryOptions, profile validation.ProfileID) (*resourceGraphRun, error) {
	return startDictionaryResourceGraphAccount(ctx, resources, dictionaries, profile, nil)
}

// startDictionaryResourceGraphAccount uses account when it is non-nil.
func startDictionaryResourceGraphAccount(ctx context.Context, resources LinkedAssessmentOptions, dictionaries DictionaryOptions, profile validation.ProfileID, account *validation.Account) (*resourceGraphRun, error) {
	selected, composed, err := selectDictionaryPackageProfile(resources.Graph.Profile, profile)
	if err != nil {
		return nil, err
	}
	profile = selected
	if err := validateDictionaryOptions(dictionaries); err != nil {
		return nil, err
	}
	cfg, err := validateLinkedAssessmentOptionsAccount(ctx, resources, profile, account)
	if err != nil {
		return nil, err
	}
	bundle := dictionaryResourceIntrinsicProfiles()
	var catalog *validation.CompiledPackageCatalog
	if isSemanticPackageProfile(profile) {
		catalog, err = semanticPackageAssessmentCatalog()
		bundle = semanticResourceIntrinsicProfiles()
	} else if composed {
		catalog, err = dtdDictionaryPackageAssessmentCatalog()
		bundle = dtdDictionaryResourceIntrinsicProfiles()
	} else {
		catalog, err = newDictionaryPackageAssessmentCatalog()
	}
	if err != nil {
		return nil, err
	}
	g, err := startConfiguredResourceGraph(ctx, cfg, profile, catalog, bundle)
	if err != nil {
		return nil, err
	}
	g.dictionaries, err = loadValidatedDictionarySet(ctx, cfg.account, dictionaries)
	if g.dictionaries == nil {
		if err == nil {
			err = &validation.ConfigurationError{Field: "dictionary set", Reason: "root import did not return a capability set"}
		}
		return g, err
	}
	if dictionaryImportTerminal(err) {
		return g, err
	}
	// Provider failures describe unavailable capabilities. Keep them out of
	// Completion.Cause so all independent intrinsic/package proofs still run.
	g.dictionaryErr = err
	g.dictionaryFingerprint, err = g.dictionaries.Fingerprint()
	if err != nil {
		return g, err
	}
	if err = ctx.Err(); err != nil {
		return g, err
	}
	return g, nil
}

// InspectXMLWithResourcesAndDictionaries assesses original MusicXML/Opus bytes,
// selected resources and explicitly selected dictionaries in one finite account.
// The caller reader stays open. Provider failures accompany a partial result;
// dictionary gaps never suppress independently proved schema/family failures.
// options.Resources.Graph.Dictionaries must be nil; Dictionaries applies.
//
// Deprecated: Use InspectXMLWithResources with Graph.Dictionaries set to
// &options.Dictionaries; it gives the same result for every profile
// this function accepts.
func InspectXMLWithResourcesAndDictionaries(ctx context.Context, reader io.Reader, options LinkedDictionaryAssessmentOptions) (validation.AssessmentBundle, error) {
	if err := singleGraphDictionaries(options.Resources.Graph); err != nil {
		return validation.AssessmentBundle{}, err
	}
	return inspectXMLWithResourcesAndDictionaries(ctx, reader, options)
}

func inspectXMLWithResourcesAndDictionaries(ctx context.Context, reader io.Reader, options LinkedDictionaryAssessmentOptions) (validation.AssessmentBundle, error) {
	if dictionaryNil(reader) {
		return validation.AssessmentBundle{}, ErrNilReader
	}
	g, err := startDictionaryResourceGraph(ctx, options.Resources, options.Dictionaries, ProfileResourceGraphDictionaries)
	if g == nil {
		return validation.AssessmentBundle{}, err
	}
	if err != nil {
		return g.finish(err)
	}
	return g.inspectPrimarySource(reader)
}

// AssessModelWithResourcesAndDictionaries assesses the current MusicXML/Opus
// model directly with one root-owned dictionary set and selected resources.
// It never encodes the model or reconstructs discarded source information.
// options.Resources.Graph.Dictionaries must be nil; Dictionaries applies.
//
// Deprecated: Use AssessModelWithResources with Graph.Dictionaries set to
// &options.Dictionaries; it gives the same result for every profile
// this function accepts.
func AssessModelWithResourcesAndDictionaries(ctx context.Context, document Document, options LinkedDictionaryAssessmentOptions) (validation.AssessmentBundle, error) {
	if err := singleGraphDictionaries(options.Resources.Graph); err != nil {
		return validation.AssessmentBundle{}, err
	}
	return assessModelWithResourcesAndDictionaries(ctx, document, options)
}

func assessModelWithResourcesAndDictionaries(ctx context.Context, document Document, options LinkedDictionaryAssessmentOptions) (validation.AssessmentBundle, error) {
	if _, err := validationSchemaForDocument(document); err != nil {
		return validation.AssessmentBundle{}, err
	}
	g, err := startDictionaryResourceGraph(ctx, options.Resources, options.Dictionaries, ProfileResourceGraphDictionaries)
	if g == nil {
		return validation.AssessmentBundle{}, err
	}
	if err != nil {
		return g.finish(err)
	}
	return g.assessPrimaryModel(document)
}

// InspectMXLWithDictionaries assesses original MXL bytes and selected resources
// with one root account and dictionary import. Container/ZIP evidence, nested
// boundaries and indirection limits retain the ordinary package contracts.
// options.Resources.Graph.Dictionaries must be nil; Dictionaries applies.
//
// Deprecated: Use InspectMXL with Graph.Dictionaries set to
// &options.Dictionaries; it gives the same result for every profile
// this function accepts.
func InspectMXLWithDictionaries(ctx context.Context, reader io.Reader, options MXLDictionaryAssessmentOptions) (validation.AssessmentBundle, error) {
	if err := singleGraphDictionaries(options.Resources.Graph); err != nil {
		return validation.AssessmentBundle{}, err
	}
	return inspectMXLWithDictionaries(ctx, reader, options)
}

func inspectMXLWithDictionaries(ctx context.Context, reader io.Reader, options MXLDictionaryAssessmentOptions) (validation.AssessmentBundle, error) {
	if dictionaryNil(reader) {
		return validation.AssessmentBundle{}, ErrNilReader
	}
	if _, err := options.Resources.MXL.limits(); err != nil {
		return validation.AssessmentBundle{}, err
	}
	g, err := startDictionaryResourceGraph(ctx, options.Resources.linked(), options.Dictionaries, ProfileMXLSourceDictionaries)
	if g == nil {
		return validation.AssessmentBundle{}, err
	}
	if err != nil {
		return g.finish(err)
	}
	g.mxlOptions = options.Resources.MXL
	return g.inspectRawPackage(reader)
}

// AssessMXLPackageWithDictionaries assesses a current MXLPackage without encoding
// its primary model. Selected retained XML and nested packages share the same
// verified dictionaries and finite account; source-only evidence stays separate.
// options.Resources.Graph.Dictionaries must be nil; Dictionaries applies.
//
// Deprecated: Use AssessMXLPackage with Graph.Dictionaries set to
// &options.Dictionaries; it gives the same result for every profile
// this function accepts.
func AssessMXLPackageWithDictionaries(ctx context.Context, value *MXLPackage, options MXLDictionaryAssessmentOptions) (validation.AssessmentBundle, error) {
	if err := singleGraphDictionaries(options.Resources.Graph); err != nil {
		return validation.AssessmentBundle{}, err
	}
	return assessMXLPackageWithDictionaries(ctx, value, options)
}

func assessMXLPackageWithDictionaries(ctx context.Context, value *MXLPackage, options MXLDictionaryAssessmentOptions) (validation.AssessmentBundle, error) {
	if value == nil {
		return validation.AssessmentBundle{}, ErrNilMXLPackage
	}
	if _, err := options.Resources.MXL.limits(); err != nil {
		return validation.AssessmentBundle{}, err
	}
	if _, err := validationSchemaForDocument(value.Document); err != nil {
		return validation.AssessmentBundle{}, err
	}
	g, err := startDictionaryResourceGraph(ctx, options.Resources.linked(), options.Dictionaries, ProfileMXLModelDictionaries)
	if g == nil {
		return validation.AssessmentBundle{}, err
	}
	if err != nil {
		return g.finish(err)
	}
	g.mxlOptions = options.Resources.MXL
	return g.assessCurrentPackage(value)
}

// singleGraphDictionaries rejects dictionary options given twice in a
// deprecated wrapper.
func singleGraphDictionaries(graph AssessmentOptions) error {
	if graph.Dictionaries != nil {
		return &validation.ConfigurationError{Field: "Graph.Dictionaries", Reason: "dictionaries are set by the Dictionaries field of this deprecated option type"}
	}
	return nil
}
