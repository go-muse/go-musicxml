package musicxml

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestMandatoryAssessedDictionariesDefaultAndExplicitShareEvidence(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit=%v", explicit), func(t *testing.T) {
			calls := make(map[validation.DictionaryManifestID]int)
			var bodies []*mandatoryDictionaryBody
			var options *dictionaryAssessmentOptions
			if explicit {
				options = &dictionaryAssessmentOptions{Dictionaries: DictionaryOptions{
					Selections: mandatoryDictionarySelections(),
					Source: dictionaryImportSourceFunc(func(ctx context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
						calls[request.Manifest]++
						result, err := BundledDictionarySource().Open(ctx, request)
						if result.Body != nil {
							body := &mandatoryDictionaryBody{ReadCloser: result.Body}
							bodies = append(bodies, body)
							result.Body = body
						}
						return result, err
					}),
				}}
			}
			result, err := decodeAssessedWithDictionaries(context.Background(), strings.NewReader(mandatoryDictionaryScore()), decodeDictionaryTestOptions{Assessment: options})
			if err != nil || result.Document == nil {
				t.Fatalf("dictionary-backed load: document=%T error=%v", result.Document, err)
			}
			mandatorySatisfiedReport(t, result.SourceLine, ProfileSourceLine.ID(), validation.TargetSource)
			mandatorySatisfiedReport(t, result.ModelLine, ProfileModelSemantics.ID(), validation.TargetModel)
			controlOptions := dictionaryAssessmentOptions{Assessment: assessmentOptions{Profile: ProfileModelSemantics.ID()}, Dictionaries: DictionaryOptions{
				Selections: mandatoryDictionarySelections(), Source: BundledDictionarySource(),
			}}
			sourceControl, err := InspectXML(context.Background(), strings.NewReader(mandatoryDictionaryScore()), SourceOptions{Profile: ProfileSourceLine})
			if err != nil || !reflect.DeepEqual(sourceControl, result.SourceLine) {
				t.Fatalf("complete source report differs from standalone assessment: error=%v", err)
			}
			controlOptions.Assessment.Profile = ProfileModelSemantics.ID()
			modelControl, err := assessModelWithDictionaries(context.Background(), result.Document, controlOptions)
			if err != nil || !reflect.DeepEqual(modelControl, result.ModelLine) {
				t.Fatalf("complete model report differs from standalone assessment: error=%v", err)
			}
			// The import set has already been closed. Each returned report must
			// retain independently usable verified evidence from that same set.
			// Only line 3 reads dictionaries.
			for _, property := range []validation.ContextProperty{validation.ContextDictionaryCanonical, validation.ContextDictionaryIPARepresentation} {
				evidence := dictionaryResults(result.ModelLine, property)
				if len(evidence) != 1 || evidence[0].Result.State != validation.EvidenceKnown || !evidence[0].Result.Value || !evidence[0].ComponentVerified {
					t.Fatalf("returned model report lost %s evidence: %+v", property, evidence)
				}
				if evidence := dictionaryResults(result.SourceLine, property); len(evidence) != 0 {
					t.Fatalf("source line read dictionary %s: %+v", property, evidence)
				}
			}
			if explicit {
				if len(calls) != 2 || len(bodies) != 2 {
					t.Fatalf("unexpected component imports: calls=%v bodies=%d", calls, len(bodies))
				}
				for id, count := range calls {
					if count != 1 {
						t.Errorf("component %s imported %d times across the two stages", id, count)
					}
				}
				for _, body := range bodies {
					if body.closed != 1 {
						t.Errorf("dictionary body closed %d times", body.closed)
					}
				}
			}
		})
	}
}

func TestMandatoryAssessedDictionariesNeverRepairExplicitUnavailableOverride(t *testing.T) {
	providerFailure := errors.New("explicit dictionary provider failed")
	for _, kind := range []string{"nil", "typed-nil", "unknown", "unsupported", "provider-error", "corrupt"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			var bodies []*mandatoryDictionaryBody
			var source DictionarySource
			if kind == "typed-nil" {
				source = (*mandatoryNilDictionarySource)(nil)
			} else if kind != "nil" {
				source = dictionaryImportSourceFunc(func(ctx context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
					calls++
					switch kind {
					case "unknown":
						return DictionaryComponentResult{Request: request, State: validation.EvidenceUnknown, Reason: "explicit-unavailable"}, nil
					case "unsupported":
						return DictionaryComponentResult{Request: request, State: validation.EvidenceUnsupported, Reason: "explicit-unsupported"}, nil
					case "provider-error":
						return DictionaryComponentResult{}, providerFailure
					default:
						result, err := BundledDictionarySource().Open(ctx, request)
						if result.Body != nil {
							body := &mandatoryDictionaryBody{ReadCloser: result.Body}
							if request.Manifest == validation.DictionaryManifestSMuFL14Canonical {
								body.ReadCloser = &bundledCorruptReader{ReadCloser: result.Body}
							}
							bodies = append(bodies, body)
							result.Body = body
						}
						return result, err
					}
				})
			}
			options := dictionaryAssessmentOptions{Dictionaries: DictionaryOptions{Selections: mandatoryDictionarySelections(), Source: source}}
			result, err := decodeAssessedWithDictionaries(context.Background(), strings.NewReader(mandatoryDictionaryScore()), decodeDictionaryTestOptions{Assessment: &options})
			// Line 2 reads no dictionaries, so it accepts the source; line 3
			// reports the unavailable evidence and rejects the document.
			if result.Document != nil || err == nil || !result.SourceLine.Satisfied() || !result.ModelLine.Requested || result.ModelLine.Satisfied() {
				t.Fatalf("explicit override silently repaired: document=%T source-satisfied=%v model-requested=%v model-satisfied=%v error=%v", result.Document, result.SourceLine.Satisfied(), result.ModelLine.Requested, result.ModelLine.Satisfied(), err)
			}
			if !result.ModelLine.InputComplete || !errors.Is(err, ErrIncompleteAssessment) {
				t.Fatalf("missing independent incomplete model report: complete=%v error=%v", result.ModelLine.InputComplete, err)
			}
			if kind == "provider-error" && !errors.Is(err, providerFailure) {
				t.Fatalf("provider error lost: %v", err)
			}
			if kind != "nil" && kind != "typed-nil" && calls != 2 {
				t.Errorf("explicit imports=%d, want one per selected component", calls)
			}
			for _, property := range []validation.ContextProperty{validation.ContextDictionaryCanonical, validation.ContextDictionaryIPARepresentation} {
				evidence := dictionaryResults(result.ModelLine, property)
				if kind == "corrupt" && property == validation.ContextDictionaryIPARepresentation {
					// The caller supplied this separate component intact.
					if len(evidence) != 1 || !evidence[0].ComponentVerified || evidence[0].Result.State != validation.EvidenceKnown {
						t.Fatalf("intact caller-selected IPA component lost: %+v", evidence)
					}
					continue
				}
				if len(evidence) != 1 || evidence[0].ComponentVerified || evidence[0].Result.State == validation.EvidenceKnown {
					t.Fatalf("unavailable override acquired bundled evidence for %s: %+v", property, evidence)
				}
			}
			for _, body := range bodies {
				if body.closed != 1 {
					t.Errorf("corrupt dictionary body closed %d times", body.closed)
				}
			}
		})
	}
}

func TestMandatoryAssessedDictionariesRejectPartialSelectionBeforeIO(t *testing.T) {
	selections := mandatoryDictionarySelections()
	for _, selected := range [][]validation.DictionarySelection{nil, selections[:1], selections[1:]} {
		calls := 0
		reader := &assessmentCountingReader{source: strings.NewReader(mandatoryDictionaryScore())}
		options := dictionaryAssessmentOptions{Dictionaries: DictionaryOptions{
			Selections: selected,
			Source: dictionaryImportSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
				calls++
				return DictionaryComponentResult{}, errors.New("unexpected provider call")
			}),
		}}
		result, err := decodeAssessedWithDictionaries(context.Background(), reader, decodeDictionaryTestOptions{Assessment: &options})
		if !errors.Is(err, validation.ErrInvalidConfiguration) || result.Document != nil || reader.reads != 0 || calls != 0 {
			t.Fatalf("partial selection touched I/O: selections=%v document=%T reads=%d provider-calls=%d error=%v", selected, result.Document, reader.reads, calls, err)
		}
	}
}

// A corrupt override is imported only once line 3 is due, after line 2
// accepted the source, and fails as a configuration error without a fallback
// to the bundled data and without starting line 3.
func TestMandatoryAssessedDictionariesRejectCorruptIPABeforeLineThree(t *testing.T) {
	for _, corruptCanonical := range []bool{false, true} {
		t.Run(fmt.Sprintf("corrupt-canonical=%v", corruptCanonical), func(t *testing.T) {
			calls := 0
			var bodies []*mandatoryDictionaryBody
			reader := &assessmentCountingReader{source: strings.NewReader(mandatoryDictionaryScore())}
			options := dictionaryAssessmentOptions{Dictionaries: DictionaryOptions{
				Selections: mandatoryDictionarySelections(),
				Source: dictionaryImportSourceFunc(func(ctx context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
					calls++
					result, err := BundledDictionarySource().Open(ctx, request)
					if result.Body != nil {
						body := &mandatoryDictionaryBody{ReadCloser: result.Body}
						if corruptCanonical || request.Manifest == validation.DictionaryManifestIPARepresentation2015Unicode13 {
							body.ReadCloser = &bundledCorruptReader{ReadCloser: result.Body}
						}
						bodies = append(bodies, body)
						result.Body = body
					}
					return result, err
				}),
			}}
			result, err := decodeAssessedWithDictionaries(context.Background(), reader, decodeDictionaryTestOptions{Assessment: &options})
			var configuration *validation.ConfigurationError
			if !errors.Is(err, validation.ErrInvalidConfiguration) || !errors.As(err, &configuration) || configuration.Field != "dictionary IPA representation" || configuration.Reason != "source SHA-256 mismatch" {
				t.Fatalf("corrupt IPA override did not preserve its configuration failure: %v", err)
			}
			if result.Document != nil || !result.SourceLine.Satisfied() || result.ModelLine.Requested || result.Conversion.State != validation.ConversionConverted || reader.reads == 0 || calls != 2 || len(bodies) != 2 {
				t.Fatalf("corrupt IPA override fell back or ran out of order: document=%T source-satisfied=%v model-requested=%v conversion=%s reads=%d provider-calls=%d bodies=%d", result.Document, result.SourceLine.Satisfied(), result.ModelLine.Requested, result.Conversion.State, reader.reads, calls, len(bodies))
			}
			for _, body := range bodies {
				if body.closed != 1 {
					t.Errorf("rejected import body closed %d times", body.closed)
				}
			}
		})
	}
}

func TestMandatoryAssessedLoadingSharesNodeBudgetAcrossStages(t *testing.T) {
	source := mandatoryScore("1", `<note><rest/><duration>1</duration></note>`)
	sourceNodes, modelNodes := mandatoryStandaloneNodeCounts(t, source, validation.Budgets{})
	standaloneLimit := max(sourceNodes, modelNodes)
	combined := sourceNodes + modelNodes
	if standaloneLimit >= combined || sourceNodes == 0 || modelNodes == 0 {
		t.Fatalf("fixture does not distinguish cumulative nodes: source=%d model=%d", sourceNodes, modelNodes)
	}
	limitedSourceNodes, limitedModelNodes := mandatoryStandaloneNodeCounts(t, source, validation.Budgets{MaxNodes: standaloneLimit})
	if limitedSourceNodes != sourceNodes || limitedModelNodes != modelNodes {
		t.Fatalf("standalone controls changed under their sufficient shared cap: source=%d model=%d", limitedSourceNodes, limitedModelNodes)
	}
	t.Logf("source nodes=%d model nodes=%d; each standalone stage fits %d, combined needs %d", sourceNodes, modelNodes, standaloneLimit, combined)
	for _, explicit := range []bool{false, true} {
		for _, limit := range []int64{standaloneLimit, combined - 1, combined} {
			t.Run(fmt.Sprintf("%s/nodes=%d", mandatoryLoaderName(explicit), limit), func(t *testing.T) {
				result, err := mandatoryDecode(explicit, context.Background(), strings.NewReader(source), &SourceOptions{Budgets: validation.Budgets{MaxNodes: limit}})
				mandatorySatisfiedReport(t, result.SourceLine, ProfileSourceLine.ID(), validation.TargetSource)
				if limit == combined {
					if err != nil || result.Document == nil {
						t.Fatalf("exact combined node budget failed: document=%T error=%v", result.Document, err)
					}
					mandatorySatisfiedReport(t, result.ModelLine, ProfileModelSemantics.ID(), validation.TargetModel)
					return
				}
				var resource *validation.ResourceError
				if !errors.Is(err, validation.ErrResourceLimit) || !errors.As(err, &resource) || resource.Kind != validation.BudgetNodes || resource.Limit != limit || result.Document != nil || !result.ModelLine.Requested || result.ModelLine.Satisfied() || result.ModelLine.InputComplete {
					t.Fatalf("model stage did not exhaust shared node budget: source=%v model-requested=%v model-complete=%v document=%T error=%v", result.SourceLine.Satisfied(), result.ModelLine.Requested, result.ModelLine.InputComplete, result.Document, err)
				}
			})
		}
	}
}

func TestMandatoryAssessedLoadingModelCancellationPreservesSourceAndReleasesOwnership(t *testing.T) {
	source := mandatoryScore("1", `<note><rest/><duration>1</duration></note>`)
	sourceNodes, modelNodes := mandatoryStandaloneNodeCounts(t, source, validation.Budgets{})
	for _, cancelModel := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel-model=%v", cancelModel), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			observed := &mandatoryModelCancelContext{Context: ctx, cancel: cancel, sourceNodes: sourceNodes, enabled: cancelModel}
			options := dictionaryAssessmentOptions{Assessment: assessmentOptions{Profile: ProfileModelSemantics.ID()}, Dictionaries: DictionaryOptions{
				Selections: mandatoryDictionarySelections(), Source: BundledDictionarySource(),
			}}
			cfg, modelCfg, err := assessedLoadingLines(observed, options)
			if err != nil || modelCfg.Dictionaries == nil {
				t.Fatalf("import configuration failed: %v", err)
			}
			defer modelCfg.Dictionaries.Close()
			observed.account = cfg.Account
			run, err := newSourceAssessmentRunWithConfig(observed, strings.NewReader(source), DecodeOptions{}, &sourceAssessmentConfig{assessmentRunConfig: cfg, ByteOwner: sourceBytesPrimary}, true)
			if err != nil {
				t.Fatal(err)
			}
			result, err := decodeAssessedRun(observed, run, preloadedModelLine(modelCfg), false)
			err = errors.Join(err, modelCfg.Dictionaries.Close())
			mandatorySatisfiedReport(t, result.SourceLine, ProfileSourceLine.ID(), validation.TargetSource)
			if result.Conversion.State != validation.ConversionConverted || !result.ModelLine.Requested {
				t.Fatalf("source/model stage boundary lost: conversion=%s model-requested=%v error=%v", result.Conversion.State, result.ModelLine.Requested, err)
			}
			if cancelModel {
				if !errors.Is(err, context.Canceled) || result.Document != nil || result.ModelLine.Satisfied() || result.ModelLine.InputComplete {
					t.Fatalf("model cancellation accepted: document=%T model-complete=%v error=%v", result.Document, result.ModelLine.InputComplete, err)
				}
			} else {
				if err != nil || result.Document == nil {
					t.Fatalf("control load failed: document=%T error=%v", result.Document, err)
				}
				mandatorySatisfiedReport(t, result.ModelLine, ProfileModelSemantics.ID(), validation.TargetModel)
				used, err := cfg.Account.Usage(validation.BudgetNodes)
				if err != nil || used != sourceNodes+modelNodes {
					t.Fatalf("stages did not share their root account: nodes=%d want=%d error=%v", used, sourceNodes+modelNodes, err)
				}
			}
			for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
				used, err := cfg.Account.Usage(kind)
				if err != nil || used != 0 {
					t.Errorf("source report, candidate or dictionaries leaked %s=%d after completion: %v", kind, used, err)
				}
			}
		})
	}
}

func mandatoryStandaloneNodeCounts(t *testing.T, source string, budgets validation.Budgets) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	run, err := newSourceAssessmentRun(ctx, strings.NewReader(source), DecodeOptions{}, &assessmentOptions{Profile: ProfileSourceLine.ID(), Budgets: budgets}, false)
	if err != nil {
		t.Fatal(err)
	}
	report, err := run.finish(run.drain())
	if err != nil || !report.Satisfied() {
		t.Fatalf("standalone source fixture failed: error=%v", err)
	}
	sourceNodes, err := run.account.Usage(validation.BudgetNodes)
	if err != nil {
		t.Fatal(err)
	}
	document, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	account, err := validation.NewBudgetAccount(budgets)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := modelAssessmentEngineForProfile(ProfileModelSemantics.ID())
	if err != nil {
		t.Fatal(err)
	}
	report, _, err = assessModelWithConfig(ctx, document, assessmentRunConfig{Account: account, DocumentID: modelDocumentID, Profile: ProfileModelSemantics.ID(), Engine: engine}, modelWalkOptions{})
	if err != nil || !report.Satisfied() {
		t.Fatalf("standalone model fixture failed: error=%v", err)
	}
	modelNodes, err := account.Usage(validation.BudgetNodes)
	if err != nil {
		t.Fatal(err)
	}
	return sourceNodes, modelNodes
}

func mandatoryDictionarySelections() []validation.DictionarySelection {
	return []validation.DictionarySelection{
		{Manifest: validation.DictionaryManifestSMuFL14Canonical, Relations: []validation.DictionaryRelation{validation.DictionaryRelationCanonicalName}},
		{Manifest: validation.DictionaryManifestIPARepresentation2015Unicode13, Relations: []validation.DictionaryRelation{validation.DictionaryRelationIPARepresentation}},
	}
}

func mandatoryDictionaryScore() string {
	return mandatoryScore("1", `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental><play><ipa>a</ipa></play></note>`)
}

type mandatoryDictionaryBody struct {
	io.ReadCloser
	closed int
}

func (b *mandatoryDictionaryBody) Close() error {
	b.closed++
	return b.ReadCloser.Close()
}

type mandatoryNilDictionarySource struct{}

func (*mandatoryNilDictionarySource) Open(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
	panic("typed-nil dictionary source must remain unavailable")
}

// This context cancels deterministically after the complete source has been
// accepted and the first current-model node has entered the same root account.
// It uses a real cancelable context, with no scheduling or timing assumptions.
type mandatoryModelCancelContext struct {
	context.Context
	cancel      context.CancelFunc
	account     *validation.Account
	sourceNodes int64
	enabled     bool
}

func (c *mandatoryModelCancelContext) Err() error {
	if c.enabled && c.account != nil {
		used, err := c.account.Usage(validation.BudgetNodes)
		if err == nil && used > c.sourceNodes {
			c.cancel()
		}
	}
	return c.Context.Err()
}
