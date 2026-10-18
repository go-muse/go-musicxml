package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	scenarioValid          = mandatoryScore("1", `<note><rest/><duration>1</duration></note>`)
	scenarioSourceViolated = mandatoryScore("0", `<note><rest/><duration>1</duration></note>`)
	scenarioIncomplete     = mandatoryScore("1", `<note time-only="1,1"><rest/><duration>1</duration></note>`)
	scenarioModelViolated  = func() string {
		tiny := "0." + strings.Repeat("0", 399) + "1"
		return mandatoryScore(tiny, `<note><rest/><duration>`+tiny+`</duration></note>`)
	}()
)

// scenarioMXL packs an XML score into an MXL archive.
func scenarioMXL(t *testing.T, source string) []byte {
	t.Helper()
	document, err := Decode(strings.NewReader(source))
	require.NoError(t, err)
	var archive bytes.Buffer
	require.NoError(t, EncodeMXL(&archive, document))
	return archive.Bytes()
}

// bundleSummary is a bundle without its run-local identities.
type bundleSummary struct {
	Satisfied bool
	Package   reportSummary
	Resources []reportSummary
}

type reportSummary struct {
	Profile     validation.ProfileID
	Conformance validation.ConformanceState
	Complete    bool
	Counts      validation.Counts
	Findings    []string
}

func summarizeReport(r validation.Report) reportSummary {
	s := reportSummary{Profile: r.Profile.ID, Conformance: r.Conformance, Complete: r.AssessmentComplete, Counts: r.Counts}
	for _, f := range r.Findings {
		s.Findings = append(s.Findings, string(f.Rule.ID)+" "+string(f.Outcome)+" "+string(f.Reason))
	}
	return s
}

func summarizeBundle(b validation.AssessmentBundle) bundleSummary {
	s := bundleSummary{Satisfied: b.Satisfied(), Package: summarizeReport(b.PackageReport())}
	for i := 0; i < b.ResourceCount(); i++ {
		_, r, _ := b.ResourceReport(i)
		s.Resources = append(s.Resources, summarizeReport(r))
	}
	return s
}

func TestEmptyProfileSelectsSemanticProfile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	report, err := InspectXML(ctx, strings.NewReader(scenarioValid), AssessmentOptions{})
	require.NoError(t, err)
	assert.Equal(t, ProfileSourceSemantics, report.Profile.ID)
	assert.True(t, report.Satisfied())

	document, err := Decode(strings.NewReader(scenarioValid))
	require.NoError(t, err)
	report, err = AssessModel(ctx, document, AssessmentOptions{})
	require.NoError(t, err)
	assert.Equal(t, ProfileModelSemantics, report.Profile.ID)
	assert.True(t, report.Satisfied())

	bundle, err := InspectXMLWithResources(ctx, strings.NewReader(scenarioValid), LinkedAssessmentOptions{})
	require.NoError(t, err)
	assert.Equal(t, ProfileResourceGraphSemantics, bundle.PackageReport().Profile.ID)
	bundle, err = AssessModelWithResources(ctx, document, LinkedAssessmentOptions{})
	require.NoError(t, err)
	assert.Equal(t, ProfileResourceGraphSemantics, bundle.PackageReport().Profile.ID)

	archive := scenarioMXL(t, scenarioValid)
	bundle, err = InspectMXL(ctx, bytes.NewReader(archive), MXLAssessmentOptions{})
	require.NoError(t, err)
	assert.Equal(t, ProfileMXLSourceSemantics, bundle.PackageReport().Profile.ID)
	assert.True(t, bundle.Satisfied())
	pkg, err := DecodeMXLPackage(bytes.NewReader(archive))
	require.NoError(t, err)
	bundle, err = AssessMXLPackage(ctx, pkg, MXLAssessmentOptions{})
	require.NoError(t, err)
	assert.Equal(t, ProfileMXLModelSemantics, bundle.PackageReport().Profile.ID)

	// An explicit profile without dictionaries keeps its old behaviour.
	report, err = InspectXML(ctx, strings.NewReader(scenarioValid), AssessmentOptions{Profile: ProfileSourceLocal})
	require.NoError(t, err)
	assert.Equal(t, ProfileSourceLocal, report.Profile.ID)
}

func TestScenarioThreeMatchesStrictLoadingModel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for name, source := range map[string]string{"valid": scenarioValid, "model violated": scenarioModelViolated} {
		result, err := DecodeAssessed(ctx, strings.NewReader(source), DecodeAssessmentOptions{Lenient: true})
		require.NoError(t, err, name)
		require.NotNil(t, result.Document, name)
		report, err := AssessModel(ctx, result.Document, AssessmentOptions{})
		require.NoError(t, err, name)
		assert.True(t, reflect.DeepEqual(result.Model, report), "%s: AssessModel equals the model stage of DecodeAssessed", name)

		mxl, err := DecodeMXLAssessed(ctx, bytes.NewReader(scenarioMXL(t, source)), DecodeMXLAssessmentOptions{Lenient: true})
		require.NoError(t, err, name)
		require.NotNil(t, mxl.Package, name)
		report, err = AssessModel(ctx, mxl.Package.Document, AssessmentOptions{})
		require.NoError(t, err, name)
		assert.True(t, reflect.DeepEqual(mxl.Model, report), "%s: AssessModel equals the model stage of DecodeMXLAssessed", name)
	}
}

func TestDeprecatedAssessmentWrappersAreEquivalent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dictionaries := DictionaryOptions{Selections: mandatoryDictionarySelections(), Source: BundledDictionarySource()}
	document, err := Decode(strings.NewReader(mandatoryDictionaryScore()))
	require.NoError(t, err)
	archive := scenarioMXL(t, mandatoryDictionaryScore())
	pkg, err := DecodeMXLPackage(bytes.NewReader(archive))
	require.NoError(t, err)

	for _, profile := range []validation.ProfileID{ProfileSourceDictionaries, ProfileSourceSemantics} {
		old, oldErr := InspectXMLWithDictionaries(ctx, strings.NewReader(mandatoryDictionaryScore()), DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: profile}, Dictionaries: dictionaries})
		new, newErr := InspectXML(ctx, strings.NewReader(mandatoryDictionaryScore()), AssessmentOptions{Profile: profile, Dictionaries: &dictionaries})
		assert.Equal(t, oldErr, newErr, profile)
		assert.True(t, reflect.DeepEqual(old, new), "InspectXMLWithDictionaries %s", profile)
	}
	for _, profile := range []validation.ProfileID{ProfileModelDictionaries, ProfileModelSemantics} {
		old, oldErr := AssessModelWithDictionaries(ctx, document, DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: profile}, Dictionaries: dictionaries})
		new, newErr := AssessModel(ctx, document, AssessmentOptions{Profile: profile, Dictionaries: &dictionaries})
		assert.Equal(t, oldErr, newErr, profile)
		assert.True(t, reflect.DeepEqual(old, new), "AssessModelWithDictionaries %s", profile)
	}
	{
		old, oldErr := DecodeAssessedWithDictionaries(ctx, strings.NewReader(mandatoryDictionaryScore()), DecodeDictionaryAssessmentOptions{Assessment: &DictionaryAssessmentOptions{Dictionaries: dictionaries}})
		new, newErr := DecodeAssessed(ctx, strings.NewReader(mandatoryDictionaryScore()), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Dictionaries: &dictionaries}})
		assert.Equal(t, oldErr, newErr)
		assert.True(t, reflect.DeepEqual(old, new), "DecodeAssessedWithDictionaries")
	}
	for _, profile := range []validation.ProfileID{ProfileResourceGraphDictionaries, ProfileResourceGraphSemantics} {
		old, oldErr := InspectXMLWithResourcesAndDictionaries(ctx, strings.NewReader(mandatoryDictionaryScore()), LinkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: profile}}, Dictionaries: dictionaries})
		new, newErr := InspectXMLWithResources(ctx, strings.NewReader(mandatoryDictionaryScore()), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: profile, Dictionaries: &dictionaries}})
		assert.Equal(t, oldErr, newErr, profile)
		assert.Equal(t, summarizeBundle(old), summarizeBundle(new), "InspectXMLWithResourcesAndDictionaries %s", profile)

		old, oldErr = AssessModelWithResourcesAndDictionaries(ctx, document, LinkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: profile}}, Dictionaries: dictionaries})
		new, newErr = AssessModelWithResources(ctx, document, LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: profile, Dictionaries: &dictionaries}})
		assert.Equal(t, oldErr, newErr, profile)
		assert.Equal(t, summarizeBundle(old), summarizeBundle(new), "AssessModelWithResourcesAndDictionaries %s", profile)
	}
	for _, profile := range []validation.ProfileID{ProfileMXLSourceDictionaries, ProfileMXLSourceSemantics} {
		old, oldErr := InspectMXLWithDictionaries(ctx, bytes.NewReader(archive), MXLDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: profile}}, Dictionaries: dictionaries})
		new, newErr := InspectMXL(ctx, bytes.NewReader(archive), MXLAssessmentOptions{Graph: AssessmentOptions{Profile: profile, Dictionaries: &dictionaries}})
		assert.Equal(t, oldErr, newErr, profile)
		assert.Equal(t, summarizeBundle(old), summarizeBundle(new), "InspectMXLWithDictionaries %s", profile)
	}
	for _, profile := range []validation.ProfileID{ProfileMXLModelDictionaries, ProfileMXLModelSemantics} {
		old, oldErr := AssessMXLPackageWithDictionaries(ctx, pkg, MXLDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: profile}}, Dictionaries: dictionaries})
		new, newErr := AssessMXLPackage(ctx, pkg, MXLAssessmentOptions{Graph: AssessmentOptions{Profile: profile, Dictionaries: &dictionaries}})
		assert.Equal(t, oldErr, newErr, profile)
		assert.Equal(t, summarizeBundle(old), summarizeBundle(new), "AssessMXLPackageWithDictionaries %s", profile)
	}

	// A deprecated option type sets dictionaries once; each call uses a
	// profile it accepts, so only the duplicate is rejected.
	twice := func(profile validation.ProfileID) AssessmentOptions {
		return AssessmentOptions{Profile: profile, Dictionaries: &dictionaries}
	}
	for name, call := range map[string]func() error{
		"InspectXMLWithDictionaries": func() error {
			_, err := InspectXMLWithDictionaries(ctx, strings.NewReader(scenarioValid), DictionaryAssessmentOptions{Assessment: twice(ProfileSourceDictionaries), Dictionaries: dictionaries})
			return err
		},
		"AssessModelWithDictionaries": func() error {
			_, err := AssessModelWithDictionaries(ctx, document, DictionaryAssessmentOptions{Assessment: twice(ProfileModelDictionaries), Dictionaries: dictionaries})
			return err
		},
		"DecodeAssessedWithDictionaries": func() error {
			_, err := DecodeAssessedWithDictionaries(ctx, strings.NewReader(scenarioValid), DecodeDictionaryAssessmentOptions{Assessment: &DictionaryAssessmentOptions{Assessment: twice(ProfileSourceSemantics), Dictionaries: dictionaries}})
			return err
		},
		"InspectXMLWithResourcesAndDictionaries": func() error {
			_, err := InspectXMLWithResourcesAndDictionaries(ctx, strings.NewReader(scenarioValid), LinkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: twice(ProfileResourceGraphDictionaries)}, Dictionaries: dictionaries})
			return err
		},
		"AssessModelWithResourcesAndDictionaries": func() error {
			_, err := AssessModelWithResourcesAndDictionaries(ctx, document, LinkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: twice(ProfileResourceGraphDictionaries)}, Dictionaries: dictionaries})
			return err
		},
		"InspectMXLWithDictionaries": func() error {
			_, err := InspectMXLWithDictionaries(ctx, bytes.NewReader(archive), MXLDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: twice(ProfileMXLSourceDictionaries)}, Dictionaries: dictionaries})
			return err
		},
		"AssessMXLPackageWithDictionaries": func() error {
			_, err := AssessMXLPackageWithDictionaries(ctx, pkg, MXLDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: twice(ProfileMXLModelDictionaries)}, Dictionaries: dictionaries})
			return err
		},
	} {
		var configuration *validation.ConfigurationError
		require.ErrorAs(t, call(), &configuration, name)
		assert.Contains(t, configuration.Field, "Dictionaries", name)
	}
}

func TestLenientAssessedLoading(t *testing.T) {
	t.Parallel()

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name        string
		ctx         context.Context
		source      string
		budgets     validation.Budgets
		strictErr   error
		operational bool
	}{
		{name: "valid", ctx: context.Background(), source: scenarioValid},
		{name: "source violated", ctx: context.Background(), source: scenarioSourceViolated, strictErr: ErrInvalidDocument},
		{name: "source incomplete", ctx: context.Background(), source: scenarioIncomplete, strictErr: ErrIncompleteAssessment},
		{name: "model violated", ctx: context.Background(), source: scenarioModelViolated, strictErr: ErrInvalidDocument},
		{name: "malformed XML", ctx: context.Background(), source: scenarioValid + "<", operational: true},
		{name: "canceled", ctx: canceled, source: scenarioValid, strictErr: context.Canceled, operational: true},
		{name: "resource limit", ctx: context.Background(), source: scenarioValid, budgets: validation.Budgets{MaxInputBytes: 32}, strictErr: validation.ErrResourceLimit, operational: true},
		{name: "conversion failure", ctx: context.Background(), source: `<unknown-root/>`, operational: true},
	} {
		strict, strictErr := DecodeAssessed(tc.ctx, strings.NewReader(tc.source), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Budgets: tc.budgets}})
		lenient, lenientErr := DecodeAssessed(tc.ctx, strings.NewReader(tc.source), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Budgets: tc.budgets}, Lenient: true})

		// Strict: the document is returned exactly when the result is accepted.
		assert.Equal(t, strict.Accepted, strict.Document != nil, tc.name)
		assert.Equal(t, strict.Accepted, strictErr == nil, tc.name)
		if tc.strictErr != nil {
			assert.ErrorIs(t, strictErr, tc.strictErr, tc.name)
		}
		switch {
		case tc.name == "valid":
			assert.True(t, strict.Accepted, tc.name)
			assert.True(t, lenient.Accepted, tc.name)
			assert.NoError(t, lenientErr, tc.name)
		case tc.operational:
			assert.Error(t, lenientErr, tc.name)
			assert.Nil(t, lenient.Document, tc.name)
			assert.False(t, lenient.Accepted, tc.name)
			if tc.strictErr != nil {
				assert.ErrorIs(t, lenientErr, tc.strictErr, tc.name)
			}
		default:
			assert.NoError(t, lenientErr, tc.name)
			assert.NotNil(t, lenient.Document, tc.name)
			assert.False(t, lenient.Accepted, tc.name)
			assert.True(t, lenient.Source.Requested, tc.name)
			assert.True(t, lenient.Model.Requested, "%s: the model is assessed after source findings", tc.name)
			assert.False(t, lenient.Source.Satisfied() && lenient.Model.Satisfied(), tc.name)
		}
	}
	var syntax *xml.SyntaxError
	_, err := DecodeAssessed(context.Background(), strings.NewReader(scenarioValid+"<"), DecodeAssessmentOptions{Lenient: true})
	assert.ErrorAs(t, err, &syntax)
	_, err = DecodeAssessed(context.Background(), strings.NewReader(`<unknown-root/>`), DecodeAssessmentOptions{Lenient: true})
	var conversion *ConversionError
	assert.ErrorAs(t, err, &conversion)
}

func TestDecodeMXLAssessed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valid := scenarioMXL(t, scenarioValid)
	owned := &assessmentCountingReader{source: bytes.NewReader(valid)}
	result, err := DecodeMXLAssessed(ctx, owned, DecodeMXLAssessmentOptions{})
	require.NoError(t, err)
	assert.False(t, owned.closed, "the reader is never closed")
	require.NotNil(t, result.Package)
	assert.True(t, result.Accepted)
	assert.Equal(t, validation.ConversionConverted, result.Conversion.State)
	assert.True(t, result.Source.Satisfied())
	assert.True(t, result.Model.Satisfied())
	assert.Equal(t, ProfileMXLSourceSemantics, result.Source.PackageReport().Profile.ID)
	assert.Equal(t, ProfileModelSemantics, result.Model.Profile.ID)

	explicit, err := DecodeMXLAssessed(ctx, bytes.NewReader(valid), DecodeMXLAssessmentOptions{Assessment: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLSourceSemantics}}})
	require.NoError(t, err)
	assert.True(t, explicit.Accepted)

	violated := scenarioMXL(t, scenarioSourceViolated)
	strict, err := DecodeMXLAssessed(ctx, bytes.NewReader(violated), DecodeMXLAssessmentOptions{})
	assert.ErrorIs(t, err, ErrInvalidDocument)
	assert.Nil(t, strict.Package)
	assert.False(t, strict.Accepted)
	assert.False(t, strict.Model.Requested, "strict mode assesses the model only after an accepted archive")
	lenient, err := DecodeMXLAssessed(ctx, bytes.NewReader(violated), DecodeMXLAssessmentOptions{Lenient: true})
	require.NoError(t, err)
	require.NotNil(t, lenient.Package)
	assert.False(t, lenient.Accepted)
	assert.True(t, lenient.Model.Requested)

	modelViolated := scenarioMXL(t, scenarioModelViolated)
	strict, err = DecodeMXLAssessed(ctx, bytes.NewReader(modelViolated), DecodeMXLAssessmentOptions{})
	assert.ErrorIs(t, err, ErrInvalidDocument)
	assert.Nil(t, strict.Package)

	// Configuration is checked before any read.
	reader := &assessmentCountingReader{source: bytes.NewReader(valid)}
	for name, options := range map[string]DecodeMXLAssessmentOptions{
		"narrow profile":       {Assessment: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLSource}}},
		"missing dictionaries": {Assessment: MXLAssessmentOptions{Graph: AssessmentOptions{Dictionaries: &DictionaryOptions{Source: BundledDictionarySource(), Selections: mandatoryDictionarySelections()[:1]}}}},
		"invalid MXL limits":   {Assessment: MXLAssessmentOptions{MXL: MXLOptions{MaxArchiveBytes: -1}}},
		"typed-nil source":     {Assessment: MXLAssessmentOptions{Graph: AssessmentOptions{Dictionaries: &DictionaryOptions{Source: DictionarySourceFunc(nil)}}}},
	} {
		_, err := DecodeMXLAssessed(ctx, reader, options)
		assert.ErrorIs(t, err, ErrInvalidConfiguration, name)
	}
	assert.Zero(t, reader.reads)
	_, err = DecodeAssessed(ctx, reader, DecodeAssessmentOptions{Assessment: &AssessmentOptions{Dictionaries: &DictionaryOptions{Source: DictionarySourceFunc(nil)}}})
	assert.ErrorIs(t, err, ErrInvalidConfiguration, "a typed-nil source is an explicit override, not a request for defaults")
	assert.Zero(t, reader.reads)
	_, err = DecodeMXLAssessed(ctx, nil, DecodeMXLAssessmentOptions{})
	assert.ErrorIs(t, err, ErrNilReader)

	// The archive bytes held for the call are charged to the shared account
	// on top of what the source stage needs on its own.
	minimum := func(run func(validation.Budgets) error) int64 {
		low, high := int64(1), int64(1<<30)
		for low < high {
			middle := (low + high) / 2
			var resource *validation.ResourceError
			if err := run(validation.Budgets{MaxBufferedBytes: middle}); errors.As(err, &resource) && resource.Kind == validation.BudgetBufferedBytes {
				low = middle + 1
			} else {
				high = middle
			}
		}
		return low
	}
	inspectMinimum := minimum(func(b validation.Budgets) error {
		_, err := InspectMXL(ctx, bytes.NewReader(valid), MXLAssessmentOptions{Graph: AssessmentOptions{Budgets: b}})
		return err
	})
	loadMinimum := minimum(func(b validation.Budgets) error {
		_, err := DecodeMXLAssessed(ctx, bytes.NewReader(valid), DecodeMXLAssessmentOptions{Assessment: MXLAssessmentOptions{Graph: AssessmentOptions{Budgets: b}}, Lenient: true})
		return err
	})
	assert.GreaterOrEqual(t, loadMinimum, inspectMinimum+int64(len(valid)))

	// Operational failures.
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = DecodeMXLAssessed(canceled, bytes.NewReader(valid), DecodeMXLAssessmentOptions{Lenient: true})
	assert.ErrorIs(t, err, context.Canceled)
	_, err = DecodeMXLAssessed(ctx, bytes.NewReader(valid), DecodeMXLAssessmentOptions{Assessment: MXLAssessmentOptions{MXL: MXLOptions{MaxDocumentBytes: 16}}, Lenient: true})
	assert.ErrorIs(t, err, ErrInvalidMXL, "the MXL limits bound the archive and its conversion")
	corrupt, err := DecodeMXLAssessed(ctx, strings.NewReader("not a zip"), DecodeMXLAssessmentOptions{Lenient: true})
	assert.Error(t, err)
	assert.Nil(t, corrupt.Package)

	// Resources and their resolver reach the source stage as in InspectMXL.
	var calls atomic.Int32
	resolver := validation.ResourceResolverFunc(func(context.Context, validation.ResourceRequest) (validation.ResourceResult, error) {
		calls.Add(1)
		return validation.ResourceResult{}, errors.New("unavailable")
	})
	resources := MXLAssessmentOptions{Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}, Identity: ResourceIdentity{CanonicalID: "urn:test:score"}}
	inspected, inspectErr := InspectMXL(ctx, bytes.NewReader(valid), resources)
	loaded, loadErr := DecodeMXLAssessed(ctx, bytes.NewReader(valid), DecodeMXLAssessmentOptions{Assessment: resources, Lenient: true})
	assert.Equal(t, inspectErr, loadErr)
	assert.Equal(t, summarizeBundle(inspected), summarizeBundle(loaded.Source))
}

func TestValidationIssueSemantics(t *testing.T) {
	t.Parallel()

	result, err := DecodeAssessed(context.Background(), strings.NewReader(scenarioSourceViolated), DecodeAssessmentOptions{})
	var invalid *ValidationError
	require.ErrorAs(t, err, &invalid)
	require.NotEmpty(t, invalid.Issues)
	failed := map[string]validation.Finding{}
	for _, f := range result.Source.Findings {
		if f.Outcome == validation.OutcomeFail {
			failed[string(f.Rule.ID)] = f
		}
	}
	for _, issue := range invalid.Issues {
		finding, ok := failed[issue.Rule]
		require.True(t, ok, "Rule is the violated rule ID: %+v", issue)
		assert.Equal(t, string(finding.Reason), issue.Constraint)
		assert.Equal(t, finding.Actual, issue.Message)
	}

	document, err := Decode(strings.NewReader(scenarioSourceViolated))
	require.NoError(t, err)
	err = Validate(document)
	require.ErrorAs(t, err, &invalid)
	for _, issue := range invalid.Issues {
		assert.NotEmpty(t, issue.Rule, "Validate reports the rule too: %+v", issue)
	}

	assert.Equal(t, "/a: 0", ValidationIssue{Path: "/a", Constraint: "datatype", Message: "0"}.String())
	assert.Equal(t, "/a: datatype", ValidationIssue{Path: "/a", Constraint: "datatype"}.String(), "an empty message shows the constraint")
	assert.Equal(t, "datatype", ValidationIssue{Constraint: "datatype"}.String())
}

func TestErrInvalidConfigurationMatchesOptionSentinels(t *testing.T) {
	t.Parallel()

	assert.Equal(t, validation.ErrInvalidConfiguration, ErrInvalidConfiguration)
	for sentinel, message := range map[error]string{
		ErrInvalidDecodeOptions: "musicxml: invalid decode options",
		ErrInvalidEncodeOptions: "musicxml: invalid encode options",
		ErrInvalidMXLOptions:    "musicxml: invalid MXL options",
	} {
		assert.ErrorIs(t, sentinel, ErrInvalidConfiguration, message)
		assert.Equal(t, message, sentinel.Error())
	}
	assert.NotErrorIs(t, ErrInvalidDecodeOptions, ErrInvalidMXLOptions)

	_, err := DecodeWithOptions(strings.NewReader(scenarioValid), DecodeOptions{MaxXMLDepth: -1})
	assert.ErrorIs(t, err, ErrInvalidDecodeOptions)
	assert.ErrorIs(t, err, ErrInvalidConfiguration)
	_, err = DecodeMXLWithOptions(bytes.NewReader(nil), MXLOptions{MaxArchiveBytes: -1})
	assert.ErrorIs(t, err, ErrInvalidMXLOptions)
	assert.ErrorIs(t, err, ErrInvalidConfiguration)
	_, err = InspectXML(context.Background(), strings.NewReader(scenarioValid), AssessmentOptions{Profile: "unknown"})
	assert.ErrorIs(t, err, ErrInvalidConfiguration)
}

func TestAssessmentFuncAdapters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	bundled := BundledDictionarySource()
	var opened atomic.Int32
	source := DictionarySourceFunc(func(ctx context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
		opened.Add(1)
		return bundled.Open(ctx, request)
	})
	result, err := DecodeAssessed(ctx, strings.NewReader(mandatoryDictionaryScore()), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Dictionaries: &DictionaryOptions{Source: source, Selections: mandatoryDictionarySelections()}}})
	require.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Positive(t, opened.Load())

	// A nil DictionarySourceFunc is an absent source.
	var none DictionarySourceFunc
	withNil, errNil := InspectXML(ctx, strings.NewReader(mandatoryDictionaryScore()), AssessmentOptions{Profile: ProfileSourceDictionaries, Dictionaries: &DictionaryOptions{Source: none, Selections: mandatoryDictionarySelections()}})
	without, errWithout := InspectXML(ctx, strings.NewReader(mandatoryDictionaryScore()), AssessmentOptions{Profile: ProfileSourceDictionaries, Dictionaries: &DictionaryOptions{Selections: mandatoryDictionarySelections()}})
	assert.Equal(t, errWithout, errNil)
	assert.True(t, reflect.DeepEqual(without, withNil))

	assert.Equal(t, validation.Evidence[int]{State: validation.EvidenceKnown, Value: 3}, validation.Known(3))
	assert.Equal(t, validation.Evidence[string]{State: validation.EvidenceUnknown, Reason: "why"}, validation.Unknown[string]("why"))

	var formatted strings.Builder
	require.NoError(t, result.Source.Format(&formatted))
	assert.Equal(t, formatted.String(), result.Source.String())
	assert.Contains(t, result.Source.String(), string(ProfileSourceSemantics))
}

// rawMXL packs raw score XML into an MXL archive without decoding it.
func rawMXL(t *testing.T, score string) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, entry := range []struct{ name, content string }{
		{"META-INF/container.xml", `<container><rootfiles><rootfile full-path="score.musicxml" media-type="application/vnd.recordare.musicxml+xml"/></rootfiles></container>`},
		{"score.musicxml", score},
	} {
		w, err := writer.Create(entry.name)
		require.NoError(t, err)
		_, err = io.WriteString(w, entry.content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return archive.Bytes()
}

type failingAfterReader struct {
	source io.Reader
	after  int
	read   int
	cancel context.CancelFunc
}

func (r *failingAfterReader) Read(p []byte) (int, error) {
	if r.read >= r.after {
		if r.cancel != nil {
			r.cancel()
			return 0, context.Canceled
		}
		return 0, errors.New("disk failure")
	}
	p = p[:min(len(p), r.after-r.read)]
	n, err := r.source.Read(p)
	r.read += n
	return n, err
}

func TestAssessedLoadingOperationalFailures(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	failing := DictionarySourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
		return DictionaryComponentResult{}, errors.New("dictionary store offline")
	})
	for name, run := range map[string]func(lenient bool) (DecodeResult, error){
		"I/O failure": func(lenient bool) (DecodeResult, error) {
			return DecodeAssessed(ctx, &failingAfterReader{source: strings.NewReader(scenarioValid), after: 64}, DecodeAssessmentOptions{Lenient: lenient})
		},
		"cancellation during the read": func(lenient bool) (DecodeResult, error) {
			canceled, cancel := context.WithCancel(ctx)
			defer cancel()
			return DecodeAssessed(canceled, &failingAfterReader{source: strings.NewReader(scenarioValid), after: 64, cancel: cancel}, DecodeAssessmentOptions{Lenient: lenient})
		},
		"dictionary import failure": func(lenient bool) (DecodeResult, error) {
			return DecodeAssessed(ctx, strings.NewReader(scenarioValid), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Dictionaries: &DictionaryOptions{Source: failing, Selections: mandatoryDictionarySelections()}}, Lenient: lenient})
		},
		"invalid configuration": func(lenient bool) (DecodeResult, error) {
			return DecodeAssessed(ctx, strings.NewReader(scenarioValid), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: "unknown"}, Lenient: lenient})
		},
	} {
		for _, lenient := range []bool{false, true} {
			result, err := run(lenient)
			assert.Error(t, err, "%s lenient=%v", name, lenient)
			assert.Nil(t, result.Document, "%s lenient=%v", name, lenient)
			assert.False(t, result.Accepted, "%s lenient=%v", name, lenient)
		}
	}

	// An MXL conversion failure after an accepted archive.
	unrepresentable := rawMXL(t, `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>`+
		`<part id="P1"><measure number="1"><attributes><divisions>1</divisions><staves>99999999999999999999</staves></attributes><note><rest/><duration>1</duration></note></measure></part></score-partwise>`)
	for _, lenient := range []bool{false, true} {
		result, err := DecodeMXLAssessed(ctx, bytes.NewReader(unrepresentable), DecodeMXLAssessmentOptions{Lenient: lenient})
		var conversion *ConversionError
		assert.ErrorAs(t, err, &conversion)
		assert.True(t, result.Source.Satisfied(), "the archive itself is accepted")
		assert.Equal(t, validation.ConversionUnrepresentable, result.Conversion.State)
		assert.Nil(t, result.Package)
	}
}

func TestDecodeMXLAssessedReview(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valid := scenarioMXL(t, scenarioValid)

	// Graph.DTD reaches conversion as it reaches the source stage.
	house := &DTDOptions{Source: mapDTDSource{"house.dtd": `<!ENTITY house "H">`}}
	result, err := DecodeMXLAssessed(ctx, bytes.NewReader(houseDTDArchive(t)), DecodeMXLAssessmentOptions{Assessment: MXLAssessmentOptions{Graph: AssessmentOptions{DTD: house}}})
	require.NoError(t, err)
	require.NotNil(t, result.Package)
	score, ok := AsScorePartwise(result.Package.Document)
	require.True(t, ok)
	assert.Equal(t, "H", *score.Work.WorkTitle)

	// Resource options are checked before the reader is touched.
	reader := &assessmentCountingReader{source: bytes.NewReader(valid)}
	for name, options := range map[string]MXLAssessmentOptions{
		"invalid identity":   {Identity: ResourceIdentity{CanonicalID: "urn:a\nb"}},
		"unknown dependency": {Dependencies: []ResourceDependency{"unknown"}},
	} {
		_, err := DecodeMXLAssessed(ctx, reader, DecodeMXLAssessmentOptions{Assessment: options})
		assert.ErrorIs(t, err, ErrInvalidConfiguration, name)
	}
	assert.Zero(t, reader.reads)

	// The archive is read within the input and buffered-byte budgets.
	for kind, budgets := range map[validation.BudgetKind]validation.Budgets{
		validation.BudgetInputBytes:    {MaxInputBytes: 100},
		validation.BudgetBufferedBytes: {MaxBufferedBytes: 100},
	} {
		reader := &assessmentCountingReader{source: bytes.NewReader(valid)}
		_, err := DecodeMXLAssessed(ctx, reader, DecodeMXLAssessmentOptions{Assessment: MXLAssessmentOptions{Graph: AssessmentOptions{Budgets: budgets}}})
		var resource *validation.ResourceError
		require.ErrorAs(t, err, &resource, kind)
		assert.Equal(t, kind, resource.Kind)
		assert.LessOrEqual(t, reader.bytes, 101, "%s: no more than the budget plus one byte is read", kind)
	}

	// A corrupt archive is an invalid MXL, not an assessment rejection.
	for _, lenient := range []bool{false, true} {
		_, err := DecodeMXLAssessed(ctx, strings.NewReader("not a zip"), DecodeMXLAssessmentOptions{Lenient: lenient})
		assert.ErrorIs(t, err, ErrInvalidMXL)
		assert.NotErrorIs(t, err, ErrIncompleteAssessment)
	}

	// A zero DictionaryOptions selects the defaults with an empty profile,
	// as in DecodeAssessed. The score uses both dictionaries.
	document, err := Decode(strings.NewReader(mandatoryDictionaryScore()))
	require.NoError(t, err)
	zero, err := AssessModel(ctx, document, AssessmentOptions{Dictionaries: &DictionaryOptions{}})
	require.NoError(t, err)
	plain, err := AssessModel(ctx, document, AssessmentOptions{})
	require.NoError(t, err)
	assert.True(t, reflect.DeepEqual(plain, zero))
	assert.True(t, zero.Satisfied())
	inspected, err := InspectXML(ctx, strings.NewReader(mandatoryDictionaryScore()), AssessmentOptions{Dictionaries: &DictionaryOptions{}})
	require.NoError(t, err)
	assert.True(t, inspected.Satisfied())
}
