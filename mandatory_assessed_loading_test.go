package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// The assessed loaders have a mandatory acceptance boundary. The ordinary
// Decode API remains the separate, permissive model-mapping entry point.
func TestMandatoryAssessedLoadingRejectsInvalidSourceWithDefaults(t *testing.T) {
	const source = `<score-partwise/>`
	document, err := Decode(strings.NewReader(source))
	if err != nil || document == nil {
		t.Fatalf("ordinary Decode changed: document=%T error=%v", document, err)
	}
	for _, explicit := range []bool{false, true} {
		for _, options := range mandatoryAssessmentOptions() {
			t.Run(mandatoryLoaderName(explicit)+"/"+options.name, func(t *testing.T) {
				result, err := mandatoryDecode(explicit, context.Background(), strings.NewReader(source), options.value)
				if result.Document != nil || !errors.Is(err, ErrInvalidDocument) {
					t.Fatalf("invalid source accepted: document=%T error=%v", result.Document, err)
				}
				if !result.SourceLine.Requested || result.SourceLine.Profile.ID != ProfileSourceLine.ID() || result.SourceLine.Conformance != validation.ConformanceViolated {
					t.Fatalf("mandatory source assessment missing: requested=%v profile=%s conformance=%s", result.SourceLine.Requested, result.SourceLine.Profile.ID, result.SourceLine.Conformance)
				}
				if mandatoryModelReport(t, result).Requested {
					t.Fatal("model assessment ran before source acceptance")
				}
			})
		}
	}
}

func TestMandatoryAssessedLoadingReturnsBothSatisfiedReports(t *testing.T) {
	source := mandatoryScore("1", `<note><rest/><duration>1</duration></note>`)
	for _, explicit := range []bool{false, true} {
		for _, options := range mandatoryAssessmentOptions() {
			t.Run(mandatoryLoaderName(explicit)+"/"+options.name, func(t *testing.T) {
				reader := &assessmentCountingReader{source: strings.NewReader(source)}
				result, err := mandatoryDecode(explicit, context.Background(), reader, options.value)
				if err != nil || result.Document == nil || result.Conversion.State != validation.ConversionConverted {
					t.Fatalf("valid load failed: document=%T conversion=%s error=%v", result.Document, result.Conversion.State, err)
				}
				if reader.closed || reader.bytes != len(source) {
					t.Fatalf("reader ownership changed: closed=%v bytes=%d want=%d", reader.closed, reader.bytes, len(source))
				}
				mandatorySatisfiedReport(t, result.SourceLine, ProfileSourceLine.ID(), validation.TargetSource)
				mandatorySatisfiedReport(t, mandatoryModelReport(t, result), ProfileModelSemantics.ID(), validation.TargetModel)
			})
		}
	}
}

func TestMandatoryAssessedLoadingRejectsNarrowProfilesBeforeRead(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, profile := range []validation.ProfileID{
			ProfileSourceXSDPreview.ID(), ProfileSourceLocal.ID(), ProfileSourceContext.ID(),
			ProfileSourceDictionaries.ID(), ProfileSourceInternalDTDDictionaries.ID(),
			ProfileModelSemantics.ID(), ProfileMXLSourceSemantics.ID(), ProfileMXLSourceLine.ID(), "unknown-profile",
			// The full source semantics is a diagnostic profile, not line 2.
			ProfileSourceSemantics.ID(),
		} {
			t.Run(mandatoryLoaderName(explicit)+"/"+string(profile), func(t *testing.T) {
				reader := &assessmentCountingReader{source: strings.NewReader(`<opus/>`)}
				result, err := mandatoryDecode(explicit, context.Background(), reader, &SourceOptions{Profile: SourceProfile(profile)})
				if !errors.Is(err, validation.ErrInvalidConfiguration) || result.Document != nil || reader.reads != 0 || reader.closed {
					t.Fatalf("profile preflight: profile=%s document=%T reads=%d closed=%v error=%v", profile, result.Document, reader.reads, reader.closed, err)
				}
			})
		}
	}
}

func TestMandatoryAssessedLoadingCannotReturnDocumentAfterIncompleteSource(t *testing.T) {
	valid := mandatoryScore("1", `<note><rest/><duration>1</duration></note>`)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, explicit := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			ctx     context.Context
			source  string
			budgets validation.Budgets
			want    error
		}{
			{name: "malformed-tail", ctx: context.Background(), source: valid + `<`, want: nil},
			{name: "canceled", ctx: canceled, source: valid, want: context.Canceled},
			{name: "input-quota", ctx: context.Background(), source: valid, budgets: validation.Budgets{MaxInputBytes: 32}, want: validation.ErrResourceLimit},
			{name: "normative-unknown", ctx: context.Background(), source: mandatoryScore("1", `<note time-only="1,1"><rest/><duration>1</duration></note>`), want: ErrIncompleteAssessment},
		} {
			t.Run(mandatoryLoaderName(explicit)+"/"+tc.name, func(t *testing.T) {
				result, err := mandatoryDecode(explicit, tc.ctx, strings.NewReader(tc.source), &SourceOptions{Budgets: tc.budgets})
				if err == nil || result.Document != nil || (tc.want != nil && !errors.Is(err, tc.want)) {
					t.Fatalf("incomplete source accepted: document=%T error=%v want=%v", result.Document, err, tc.want)
				}
				if tc.name == "malformed-tail" {
					var syntax *xml.SyntaxError
					if !errors.As(err, &syntax) {
						t.Fatalf("malformed source error lost: %v", err)
					}
				}
				if tc.name == "normative-unknown" {
					// The time-only order is a line-3 rule: the source line
					// accepts the document and the model line stays incomplete.
					model := mandatoryModelReport(t, result)
					if !result.SourceLine.Satisfied() || model.Counts.Unknown == 0 || model.Conformance == validation.ConformanceViolated {
						t.Fatalf("Unknown must stay incomplete: source=%v counts=%+v conformance=%s", result.SourceLine.Satisfied(), model.Counts, model.Conformance)
					}
					return
				}
				if mandatoryModelReport(t, result).Requested {
					t.Fatal("model assessment ran after an unaccepted source")
				}
			})
		}
	}
}

func TestMandatoryAssessedLoadingRejectsDecimalUnderflowInTheSourceLine(t *testing.T) {
	// Both positive decimal values are exact and equal in the source, but the
	// float64-backed fields would become zero. The source line rejects them as
	// unrepresentable; the source stays conformant.
	tiny := "0." + strings.Repeat("0", 399) + "1"
	source := mandatoryScore(tiny, `<note><rest/><duration>`+tiny+`</duration></note>`)
	sourceReport, err := InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: ProfileSourceLine})
	if err != nil || sourceReport.Satisfied() || sourceReport.Conformance != validation.ConformanceConformant || !hasRepresentabilityFailure(sourceReport) {
		t.Fatalf("source line must reject the unrepresentable values: conformance=%s error=%v findings=%s", sourceReport.Conformance, err, localFindings(sourceReport))
	}
	// Scenario 3 sees the altered structure: zero values violate the model.
	document, err := Decode(strings.NewReader(source))
	if err != nil || document == nil {
		t.Fatalf("fixture must convert successfully: document=%T error=%v", document, err)
	}
	modelReport, err := AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelSemantics})
	if err != nil || modelReport.Conformance != validation.ConformanceViolated {
		t.Fatalf("fixture model must violate semantics: conformance=%s error=%v findings=%s", modelReport.Conformance, err, localFindings(modelReport))
	}
	for _, explicit := range []bool{false, true} {
		t.Run(mandatoryLoaderName(explicit), func(t *testing.T) {
			result, err := mandatoryDecode(explicit, context.Background(), strings.NewReader(source), nil)
			if result.Document != nil || !errors.Is(err, ErrInvalidDocument) || !hasRepresentabilityFailure(result.SourceLine) {
				t.Fatalf("unrepresentable source accepted: document=%T conversion=%s error=%v", result.Document, result.Conversion.State, err)
			}
			if mandatoryModelReport(t, result).Requested {
				t.Fatal("model assessment ran after an unaccepted source")
			}
		})
	}
}

func TestMandatoryAssessedLoadingKeepsSourceReportLeaseDuringModel(t *testing.T) {
	source := mandatoryScore("1", strings.Repeat(`<note><rest/><duration>1</duration></note>`, 4))
	sourceNodes, _ := mandatoryStandaloneNodeCounts(t, source, validation.Budgets{})
	expected := mandatoryIndependentLeaseCheckpoint(t, source, sourceNodes)
	observed := &mandatoryLeaseCheckpointContext{Context: context.Background(), sourceNodes: sourceNodes}
	run, modelConfig := mandatoryLeaseFixture(t, observed, source)
	result, err := decodeAssessedRun(observed, run, preloadedModelLine(modelConfig), false)
	if err != nil || result.Document == nil {
		t.Fatalf("pipeline control failed: document=%T error=%v", result.Document, err)
	}
	mandatorySatisfiedReport(t, result.SourceLine, ProfileSourceLine.ID(), validation.TargetSource)
	mandatorySatisfiedReport(t, result.ModelLine, ProfileModelSemantics.ID(), validation.TargetModel)
	if !observed.modelSeen || observed.modelBuffered != expected.modelBuffered {
		t.Fatalf("source report lease was not retained through model assessment: checkpoint=%d want=%d, independently measured lease=%d", observed.modelBuffered, expected.modelBuffered, expected.leaseBytes)
	}
	t.Logf("model checkpoint=%d; post-source storage=%d; source report lease=%d", observed.modelBuffered, expected.postSourceBuffered, expected.leaseBytes)
}

func TestMandatoryAssessedLoadingModelStartupCancellationLeavesUnrequestedReport(t *testing.T) {
	source := mandatoryScore("1", strings.Repeat(`<note><rest/><duration>1</duration></note>`, 4))
	sourceNodes, _ := mandatoryStandaloneNodeCounts(t, source, validation.Budgets{})
	expected := mandatoryIndependentLeaseCheckpoint(t, source, sourceNodes)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := &mandatoryLeaseCheckpointContext{Context: ctx, sourceNodes: sourceNodes, cancel: cancel, cancelAtBuffered: expected.modelStartupBuffered}
	run, modelConfig := mandatoryLeaseFixture(t, observed, source)
	result, err := decodeAssessedRun(observed, run, preloadedModelLine(modelConfig), false)
	if !observed.startupCanceled || !errors.Is(err, context.Canceled) || !errors.Is(err, ErrIncompleteAssessment) || result.Document != nil || result.Conversion.State != validation.ConversionConverted {
		t.Fatalf("model startup cancellation missed its boundary: canceled=%v document=%T conversion=%s error=%v", observed.startupCanceled, result.Document, result.Conversion.State, err)
	}
	var incomplete *validation.IncompleteError
	if !errors.As(err, &incomplete) || incomplete.Reason != "canceled-before-begin" || !errors.Is(incomplete.Cause, context.Canceled) {
		t.Fatalf("cancellation did not occur at model-session Begin: %v", err)
	}
	mandatorySatisfiedReport(t, result.SourceLine, ProfileSourceLine.ID(), validation.TargetSource)
	if result.ModelLine.Requested || result.ModelLine.Profile.ID != "" || observed.modelSeen {
		t.Fatalf("startup cancellation invented a started model report: requested=%v profile=%s node-observed=%v", result.ModelLine.Requested, result.ModelLine.Profile.ID, observed.modelSeen)
	}
}

type mandatoryLeaseCheckpoint struct {
	modelBuffered, modelStartupBuffered, postSourceBuffered, leaseBytes int64
}

// Establish the expected checkpoint through the validation seam itself, without
// calling either root finalization helper or the pipeline under test. This
// remains a live oracle if the root closes its lease early or uses plain Finish.
func mandatoryIndependentLeaseCheckpoint(t *testing.T, source string, sourceNodes int64) mandatoryLeaseCheckpoint {
	t.Helper()
	observed := &mandatoryLeaseCheckpointContext{Context: context.Background(), sourceNodes: sourceNodes}
	run, modelConfig := mandatoryLeaseFixture(t, observed, source)
	dictionaryBytes := observed.dictionaryBuffered
	projected := &namespaceXMLTokenReader{source: run.source, allowQualifiedAttribute: musicXMLQualifiedAttribute}
	modelTokens := &assessedModelTokenReader{source: projected, account: run.account, ctx: observed, observer: run.source.observer}
	defer modelTokens.close()
	decoder := xml.NewTokenDecoder(modelTokens)
	start, err := readRoot(decoder)
	if err != nil {
		t.Fatal(err)
	}
	document := &ScorePartwise{}
	if err := decoder.DecodeElement(document, &start); err != nil {
		t.Fatal(err)
	}
	completion, err := run.finishInput(run.drain())
	if err != nil {
		t.Fatal(err)
	}
	report, lease, err := run.session.FinishWithRetainedReport(observed, completion)
	if err != nil || lease == nil || !report.Satisfied() || len(report.Context) == 0 {
		t.Fatalf("independent source lease missing: lease=%v context=%d satisfied=%v error=%v", lease != nil, len(report.Context), report.Satisfied(), err)
	}
	defer lease.Close()
	postSource := mandatoryLiveBuffered(t, run.account)
	observed.postSourceBuffered = postSource
	modelReport, _, err := assessModelWithConfig(observed, document, modelConfig, modelWalkOptions{})
	if err != nil || !modelReport.Satisfied() || !observed.modelSeen || observed.modelStartupBuffered <= postSource {
		t.Fatalf("independent model control failed: satisfied=%v checkpoint-seen=%v error=%v", modelReport.Satisfied(), observed.modelSeen, err)
	}
	beforeClose := mandatoryLiveBuffered(t, run.account)
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	afterClose := mandatoryLiveBuffered(t, run.account)
	leaseBytes := beforeClose - afterClose
	if leaseBytes <= 0 || beforeClose != postSource || afterClose != dictionaryBytes+modelTokens.retained {
		t.Fatalf("independent lease-close delta is not isolated: before=%d after=%d dictionaries=%d candidate=%d post-source=%d", beforeClose, afterClose, dictionaryBytes, modelTokens.retained, postSource)
	}
	if err := modelTokens.close(); err != nil {
		t.Fatal(err)
	}
	return mandatoryLeaseCheckpoint{modelBuffered: observed.modelBuffered, modelStartupBuffered: observed.modelStartupBuffered, postSourceBuffered: postSource, leaseBytes: leaseBytes}
}

func mandatoryLeaseFixture(t *testing.T, observed *mandatoryLeaseCheckpointContext, source string) (*sourceAssessmentRun, assessmentRunConfig) {
	t.Helper()
	options := dictionaryAssessmentOptions{Assessment: assessmentOptions{Profile: ProfileModelSemantics.ID()}, Dictionaries: DictionaryOptions{
		Selections: mandatoryDictionarySelections(), Source: BundledDictionarySource(),
	}}
	config, modelConfig, err := assessedLoadingLines(observed, options)
	if err != nil || modelConfig.Dictionaries == nil {
		t.Fatalf("fixture dictionary configuration: %v", err)
	}
	t.Cleanup(func() {
		if err := modelConfig.Dictionaries.Close(); err != nil {
			t.Error(err)
		}
		for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
			if used, err := config.Account.Usage(kind); err != nil || used != 0 {
				t.Errorf("finished root retained %s=%d: %v", kind, used, err)
			}
		}
	})
	observed.account = config.Account
	observed.dictionaryBuffered = mandatoryLiveBuffered(t, config.Account)
	run, err := newSourceAssessmentRunWithConfig(observed, strings.NewReader(source), DecodeOptions{}, &sourceAssessmentConfig{assessmentRunConfig: config, ByteOwner: sourceBytesPrimary}, true)
	if err != nil {
		t.Fatal(err)
	}
	observed.source = run
	return run, modelConfig
}

// assessedLoadingLines configures both lines up front, for tests that drive
// decodeAssessedRun with an already imported line-3 configuration.
func assessedLoadingLines(ctx context.Context, normalized dictionaryAssessmentOptions) (source, model assessmentRunConfig, err error) {
	if source, err = assessedSourceLine(normalized); err != nil {
		return source, model, err
	}
	model, err = assessedModelLine(ctx, normalized, source.Account)
	return source, model, err
}

// preloadedModelLine hands decodeAssessedRun an imported configuration.
func preloadedModelLine(model assessmentRunConfig) func() (assessmentRunConfig, error) {
	return func() (assessmentRunConfig, error) { return model, nil }
}

func mandatoryLiveBuffered(t *testing.T, account *validation.Account) int64 {
	t.Helper()
	used, err := account.Usage(validation.BudgetBufferedBytes)
	if err != nil {
		t.Fatal(err)
	}
	return used
}

// Observe real account transitions rather than relying on a poll count, a timer,
// stack inspection, or a production-only test hook. The startup cancel point is
// independently measured after source finalization has released its scratch.
type mandatoryLeaseCheckpointContext struct {
	context.Context
	account              *validation.Account
	dictionaryBuffered   int64
	source               *sourceAssessmentRun
	sourceNodes          int64
	modelSeen            bool
	modelBuffered        int64
	postSourceBuffered   int64
	modelStartupBuffered int64
	cancel               context.CancelFunc
	cancelAtBuffered     int64
	startupCanceled      bool
}

func (c *mandatoryLeaseCheckpointContext) Err() error {
	if c.account != nil && c.source != nil && c.source.inputFinished {
		nodes, nodeErr := c.account.Usage(validation.BudgetNodes)
		buffered, bufferErr := c.account.Usage(validation.BudgetBufferedBytes)
		if nodeErr == nil && bufferErr == nil {
			if c.postSourceBuffered != 0 && c.modelStartupBuffered == 0 && nodes == c.sourceNodes && buffered > c.postSourceBuffered {
				c.modelStartupBuffered = buffered
			}
			if nodes > c.sourceNodes && !c.modelSeen {
				c.modelSeen, c.modelBuffered = true, buffered
			}
			if c.cancel != nil && !c.startupCanceled && nodes == c.sourceNodes && buffered == c.cancelAtBuffered {
				c.startupCanceled = true
				c.cancel()
			}
		}
	}
	return c.Context.Err()
}

// mandatoryDecode runs DecodeAssessed with the default dictionaries or, when
// explicit is set, with the bundled required dictionaries given explicitly in
// Assessment.Dictionaries; both must behave alike.
func mandatoryDecode(explicit bool, ctx context.Context, reader io.Reader, options *SourceOptions) (DecodeResult, error) {
	if explicit {
		var copied SourceOptions
		if options != nil {
			copied = *options
		}
		dictionaries := requiredDictionaryOptions()
		copied.Dictionaries = &dictionaries
		options = &copied
	}
	return DecodeAssessed(ctx, reader, DecodeAssessmentOptions{Assessment: options})
}

func mandatoryAssessmentOptions() []struct {
	name  string
	value *SourceOptions
} {
	return []struct {
		name  string
		value *SourceOptions
	}{
		{name: "nil-assessment"},
		{name: "empty-assessment", value: &SourceOptions{}},
		{name: "explicit-source-line", value: &SourceOptions{Profile: ProfileSourceLine}},
	}
}

func mandatoryLoaderName(explicit bool) string {
	if explicit {
		return "DecodeAssessed with explicit dictionaries"
	}
	return "DecodeAssessed"
}

func mandatoryScore(divisions, notes string) string {
	return `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>` +
		`<part id="P1"><measure number="1"><attributes><divisions>` + divisions + `</divisions></attributes>` + notes + `</measure></part></score-partwise>`
}

func mandatoryModelReport(t *testing.T, result DecodeResult) validation.Report {
	t.Helper()
	return result.ModelLine
}

func mandatorySatisfiedReport(t *testing.T, report validation.Report, profile validation.ProfileID, target validation.Target) {
	t.Helper()
	if !report.Requested || report.Profile.ID != profile || report.Target != target || !report.Satisfied() {
		t.Fatalf("mandatory assessment missing: requested=%v profile=%s target=%s conformance=%s counts=%+v", report.Requested, report.Profile.ID, report.Target, report.Conformance, report.Counts)
	}
	// These concrete dependencies protect against accidentally substituting a
	// semantic-only profile that omits the encoded MusicXML XSD obligations.
	for _, id := range []validation.RuleID{
		"MX40-XSD-musicxml-binding", "MX40-XSD-musicxml-declaration",
		"MX40-XSD-musicxml-attributes", "MX40-XSD-musicxml-scalar",
		"MX40-XSD-musicxml-content", "MX40-XSD-musicxml-identity",
	} {
		found := false
		for _, key := range report.Profile.Required {
			if key.ID == id && key.Revision != 0 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("mandatory profile %s is missing XSD dependency %s", profile, id)
		}
	}
}

// decodeDictionaryTestOptions and decodeAssessedWithDictionaries run strict
// loading with explicit dictionary options through the shared implementation.
type decodeDictionaryTestOptions struct {
	Decode     DecodeOptions
	Assessment *dictionaryAssessmentOptions
}

func decodeAssessedWithDictionaries(ctx context.Context, reader io.Reader, options decodeDictionaryTestOptions) (DecodeResult, error) {
	return decodeValidated(ctx, reader, options.Decode, options.Assessment, false)
}
