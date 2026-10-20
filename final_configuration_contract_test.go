package musicxml_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	musicxml "github.com/go-muse/go-musicxml"
	"github.com/go-muse/go-musicxml/validation"
)

// These external-package cases fill public-wrapper gaps. Existing profile,
// budget, dictionary-import and package-boundary tests remain authoritative for
// their broader matrices; see docs/validation/final-configuration-contracts.md.
type finalConfigurationIO struct{ reads, opens, resolves int }

func (p *finalConfigurationIO) Read([]byte) (int, error) {
	p.reads++
	return 0, io.EOF
}
func (p *finalConfigurationIO) Open(context.Context, musicxml.DictionaryComponentRequest) (musicxml.DictionaryComponentResult, error) {
	p.opens++
	return musicxml.DictionaryComponentResult{}, errors.New("unexpected dictionary I/O")
}
func (p *finalConfigurationIO) Resolve(context.Context, validation.ResourceRequest) (validation.ResourceResult, error) {
	p.resolves++
	return validation.ResourceResult{}, errors.New("unexpected resolver I/O")
}

func finalConfigurationDictionaries(source musicxml.DictionarySource) musicxml.DictionaryOptions {
	return musicxml.DictionaryOptions{
		Selections: []validation.DictionarySelection{{
			Manifest:  validation.DictionaryManifestSMuFL14Canonical,
			Relations: []validation.DictionaryRelation{validation.DictionaryRelationCanonicalName},
		}},
		Source: source,
	}
}

func assessedDecodeDictionarySelections() []validation.DictionarySelection {
	return []validation.DictionarySelection{
		{Manifest: validation.DictionaryManifestSMuFL14Canonical, Relations: []validation.DictionaryRelation{validation.DictionaryRelationCanonicalName}},
		{Manifest: validation.DictionaryManifestIPARepresentation2015Unicode13, Relations: []validation.DictionaryRelation{validation.DictionaryRelationIPARepresentation}},
	}
}

func TestFinalConfigurationUncoveredEntrypointsRejectProfilesBeforeIO(t *testing.T) {
	ctx := context.Background()
	document := musicxml.NewOpusDocument()
	linked := func(profile validation.ProfileID, probe *finalConfigurationIO) musicxml.LinkedAssessmentOptions {
		return musicxml.LinkedAssessmentOptions{
			Graph: musicxml.GraphOptions{Profile: musicxml.GraphProfile(profile)}, Resolver: probe,
			Dependencies: []musicxml.ResourceDependency{musicxml.ResourceDocuments},
		}
	}
	mxl := func(profile validation.ProfileID, probe *finalConfigurationIO) musicxml.MXLAssessmentOptions {
		return musicxml.MXLAssessmentOptions{
			Graph: musicxml.MXLGraphOptions{Profile: musicxml.MXLProfile(profile)}, Resolver: probe,
			Dependencies: []musicxml.ResourceDependency{musicxml.ResourceDocuments},
		}
	}
	for _, entry := range []struct {
		name  string
		wrong validation.ProfileID
		run   func(validation.ProfileID, *finalConfigurationIO) error
	}{
		{"InspectXMLWithResources", musicxml.ProfileMXLSource.ID(), func(profile validation.ProfileID, p *finalConfigurationIO) error {
			_, err := musicxml.InspectXMLWithResources(ctx, p, linked(profile, p))
			return err
		}},
		{"AssessModelWithResources", musicxml.ProfileModelLinks.ID(), func(profile validation.ProfileID, p *finalConfigurationIO) error {
			_, err := musicxml.AssessModelWithResources(ctx, document, linked(profile, p))
			return err
		}},
		{"InspectMXL", musicxml.ProfileMXLModel.ID(), func(profile validation.ProfileID, p *finalConfigurationIO) error {
			_, err := musicxml.InspectMXL(ctx, p, mxl(profile, p))
			return err
		}},
		{"AssessMXLPackage", musicxml.ProfileMXLSource.ID(), func(profile validation.ProfileID, p *finalConfigurationIO) error {
			_, err := musicxml.AssessMXLPackage(ctx, &musicxml.MXLPackage{Document: document}, mxl(profile, p))
			return err
		}},
		{"DecodeAssessed with dictionaries", musicxml.ProfileModelSemantics.ID(), func(profile validation.ProfileID, p *finalConfigurationIO) error {
			dictionaries := finalConfigurationDictionaries(p)
			dictionaries.Selections = assessedDecodeDictionarySelections()
			result, err := musicxml.DecodeAssessed(ctx, p, musicxml.DecodeAssessmentOptions{
				Assessment: &musicxml.SourceOptions{Profile: musicxml.SourceProfile(profile), Dictionaries: &dictionaries},
			})
			if result.Document != nil || result.SourceLine.Requested || result.ModelLine.Requested || result.Conversion.State != validation.ConversionIncomplete {
				t.Fatalf("invalid configuration exposed a converted or assessed document: %+v", result)
			}
			return err
		}},
		{"AssessModelWithResources with dictionaries", musicxml.ProfileModelSemantics.ID(), func(profile validation.ProfileID, p *finalConfigurationIO) error {
			options, dictionaries := linked(profile, p), finalConfigurationDictionaries(p)
			options.Graph.Dictionaries = &dictionaries
			_, err := musicxml.AssessModelWithResources(ctx, document, options)
			return err
		}},
		{"InspectMXL with dictionaries", musicxml.ProfileMXLModelSemantics.ID(), func(profile validation.ProfileID, p *finalConfigurationIO) error {
			options, dictionaries := mxl(profile, p), finalConfigurationDictionaries(p)
			options.Graph.Dictionaries = &dictionaries
			_, err := musicxml.InspectMXL(ctx, p, options)
			return err
		}},
		{"AssessMXLPackage with dictionaries", musicxml.ProfileMXLSourceSemantics.ID(), func(profile validation.ProfileID, p *finalConfigurationIO) error {
			options, dictionaries := mxl(profile, p), finalConfigurationDictionaries(p)
			options.Graph.Dictionaries = &dictionaries
			_, err := musicxml.AssessMXLPackage(ctx, &musicxml.MXLPackage{Document: document}, options)
			return err
		}},
	} {
		t.Run(entry.name, func(t *testing.T) {
			for _, selection := range []struct {
				name string
				id   validation.ProfileID
			}{{"empty", ""}, {"unknown", "test/unknown-profile"}, {"wrong-target", entry.wrong}} {
				if entry.name == "DecodeAssessed with dictionaries" && selection.name == "empty" {
					selection.name, selection.id = "narrow-source", musicxml.ProfileSourceDictionaries.ID()
				}
				// An empty profile selects the full semantic profile of the
				// non-deprecated entry points (TestEmptyProfileSelectsSemanticProfile).
				if selection.name == "empty" && finalConfigurationEmptySelectsSemantic[entry.name] {
					continue
				}
				t.Run(selection.name, func(t *testing.T) {
					probe := &finalConfigurationIO{}
					err := entry.run(selection.id, probe)
					var configuration *validation.ConfigurationError
					if !errors.Is(err, validation.ErrInvalidConfiguration) || !errors.As(err, &configuration) {
						t.Fatalf("profile %q: want typed configuration error, got %v", selection.id, err)
					}
					if *probe != (finalConfigurationIO{}) {
						t.Fatalf("invalid configuration performed I/O: %+v", probe)
					}
				})
			}
		})
	}
}

type finalConfigurationNilSource struct{}

func (*finalConfigurationNilSource) Open(context.Context, musicxml.DictionaryComponentRequest) (musicxml.DictionaryComponentResult, error) {
	panic("typed-nil dictionary source was invoked")
}

const finalConfigurationSMuFLScore = `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list><part id="P1"><measure number="1"><note><rest/><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental></note></measure></part></score-partwise>`

func finalConfigurationRequireUnavailableDictionary(t *testing.T, report validation.Report) {
	t.Helper()
	if !report.Requested || !report.InputComplete || report.AssessmentComplete || report.Satisfied() || report.Terminal != nil || report.Counts.Fail != 0 || report.Counts.Unknown == 0 || report.Conformance == validation.ConformanceViolated {
		t.Fatalf("missing dictionary must leave an incomplete assessment, not a normative failure: %+v", report)
	}
	for _, row := range report.Context {
		if row.Property == validation.ContextDictionaryCanonical && row.Dictionary != nil && row.Dictionary.Input.Value.Text == "accidentalSharp" {
			if row.Dictionary.Result.State != validation.EvidenceUnknown || row.Dictionary.ComponentVerified {
				t.Fatalf("missing provider became verified dictionary evidence: %+v", row.Dictionary)
			}
			return
		}
	}
	t.Fatal("missing canonical-name occurrence evidence")
}

func TestFinalConfigurationSelectedNilDictionaryPublicReports(t *testing.T) {
	ctx := context.Background()
	document, err := musicxml.Decode(strings.NewReader(finalConfigurationSMuFLScore))
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range []struct {
		name   string
		source musicxml.DictionarySource
	}{{"nil", nil}, {"typed-nil", (*finalConfigurationNilSource)(nil)}} {
		t.Run(provider.name, func(t *testing.T) {
			dictionaries := finalConfigurationDictionaries(provider.source)
			options := musicxml.SourceOptions{Profile: musicxml.ProfileSourceDictionaries, Dictionaries: &dictionaries}
			report, err := musicxml.InspectXML(ctx, strings.NewReader(finalConfigurationSMuFLScore), options)
			if err != nil {
				t.Fatal(err)
			}
			finalConfigurationRequireUnavailableDictionary(t, report)
			decodeDictionaries := dictionaries
			decodeDictionaries.Selections = assessedDecodeDictionarySelections()
			decodeOptions := musicxml.SourceOptions{Profile: musicxml.ProfileSourceLine, Dictionaries: &decodeDictionaries}
			result, err := musicxml.DecodeAssessed(ctx, strings.NewReader(finalConfigurationSMuFLScore), musicxml.DecodeAssessmentOptions{Assessment: &decodeOptions})
			if !errors.Is(err, musicxml.ErrIncompleteAssessment) || errors.Is(err, musicxml.ErrInvalidDocument) || result.Document != nil || result.Conversion.State != validation.ConversionConverted || !result.SourceLine.Satisfied() || !result.ModelLine.Requested {
				t.Fatalf("strict incomplete result=%+v error=%v", result, err)
			}
			finalConfigurationRequireUnavailableDictionary(t, result.ModelLine)
			report, err = musicxml.AssessModel(ctx, document, musicxml.ModelOptions{Profile: musicxml.ProfileModelDictionaries, Budgets: options.Budgets, Dictionaries: options.Dictionaries, OmitContext: options.OmitContext})
			if err != nil || report.Target != validation.TargetModel || report.Syntax != validation.SyntaxNotAssessed {
				t.Fatalf("current model report=%+v error=%v", report, err)
			}
			finalConfigurationRequireUnavailableDictionary(t, report)
		})
	}
}

type finalConfigurationNilResolver struct{}

func (*finalConfigurationNilResolver) Resolve(context.Context, validation.ResourceRequest) (validation.ResourceResult, error) {
	panic("typed-nil resource resolver was invoked")
}

func TestFinalConfigurationSelectedNilResolverDictionaryGraph(t *testing.T) {
	const input = `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://example.test/child.opus"/></opus>`
	for _, provider := range []struct {
		name     string
		resolver validation.ResourceResolver
	}{{"nil", nil}, {"typed-nil", (*finalConfigurationNilResolver)(nil)}} {
		t.Run(provider.name, func(t *testing.T) {
			bundle, err := musicxml.InspectXMLWithResources(context.Background(), strings.NewReader(input), musicxml.LinkedAssessmentOptions{
				Graph:        musicxml.GraphOptions{Profile: musicxml.ProfileResourceGraphDictionaries, Dictionaries: &musicxml.DictionaryOptions{}},
				Dependencies: []musicxml.ResourceDependency{musicxml.ResourceDocuments}, Resolver: provider.resolver,
			})
			if err != nil || bundle.Satisfied() || bundle.ResourceCount() != 1 {
				t.Fatalf("missing resolver: error=%v resources=%d satisfied=%t", err, bundle.ResourceCount(), bundle.Satisfied())
			}
			report := bundle.PackageReport()
			if report.AssessmentComplete || report.Conformance == validation.ConformanceViolated || report.Counts.Fail != 0 || report.Terminal != nil {
				t.Fatalf("missing resolver became a violation or terminal operation failure: %+v", report)
			}
			for _, finding := range report.Findings {
				if finding.Reason == "resolver-unavailable" && finding.Outcome == validation.OutcomeUnknown {
					return
				}
			}
			t.Fatalf("selected missing resolver reason lost: %+v", report.Findings)
		})
	}
}

var finalConfigurationEmptySelectsSemantic = map[string]bool{
	"InspectXMLWithResources":                    true,
	"AssessModelWithResources":                   true,
	"InspectMXL":                                 true,
	"AssessMXLPackage":                           true,
	"AssessModelWithResources with dictionaries": true,
	"InspectMXL with dictionaries":               true,
	"AssessMXLPackage with dictionaries":         true,
}
