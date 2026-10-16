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

func TestFinalConfigurationUncoveredEntrypointsRejectProfilesBeforeIO(t *testing.T) {
	ctx := context.Background()
	document := musicxml.NewOpusDocument()
	linked := func(profile validation.ProfileID, probe *finalConfigurationIO) musicxml.LinkedAssessmentOptions {
		return musicxml.LinkedAssessmentOptions{
			Graph: musicxml.AssessmentOptions{Profile: profile}, Resolver: probe,
			Dependencies: []musicxml.ResourceDependency{musicxml.ResourceDocuments},
		}
	}
	mxl := func(profile validation.ProfileID, probe *finalConfigurationIO) musicxml.MXLAssessmentOptions {
		return musicxml.MXLAssessmentOptions{
			Graph: musicxml.AssessmentOptions{Profile: profile}, Resolver: probe,
			Dependencies: []musicxml.ResourceDependency{musicxml.ResourceDocuments},
		}
	}
	for _, entry := range []struct {
		name  string
		wrong validation.ProfileID
		run   func(validation.ProfileID, *finalConfigurationIO) error
	}{
		{"InspectXMLWithResources", musicxml.ProfileMXLSource, func(profile validation.ProfileID, p *finalConfigurationIO) error {
			_, err := musicxml.InspectXMLWithResources(ctx, p, linked(profile, p))
			return err
		}},
		{"AssessModelWithResources", musicxml.ProfileModelLinks, func(profile validation.ProfileID, p *finalConfigurationIO) error {
			_, err := musicxml.AssessModelWithResources(ctx, document, linked(profile, p))
			return err
		}},
		{"InspectMXL", musicxml.ProfileMXLModel, func(profile validation.ProfileID, p *finalConfigurationIO) error {
			_, err := musicxml.InspectMXL(ctx, p, mxl(profile, p))
			return err
		}},
		{"AssessMXLPackage", musicxml.ProfileMXLSource, func(profile validation.ProfileID, p *finalConfigurationIO) error {
			_, err := musicxml.AssessMXLPackage(ctx, &musicxml.MXLPackage{Document: document}, mxl(profile, p))
			return err
		}},
		{"DecodeAssessedWithDictionaries", musicxml.ProfileModelSemantics, func(profile validation.ProfileID, p *finalConfigurationIO) error {
			result, err := musicxml.DecodeAssessedWithDictionaries(ctx, p, musicxml.DecodeDictionaryAssessmentOptions{
				Assessment: &musicxml.DictionaryAssessmentOptions{Assessment: musicxml.AssessmentOptions{Profile: profile}, Dictionaries: finalConfigurationDictionaries(p)},
			})
			if result.Document != nil || result.Source.Requested || result.Conversion.State != validation.ConversionIncomplete {
				t.Fatalf("invalid configuration exposed a converted or assessed document: %+v", result)
			}
			return err
		}},
		{"AssessModelWithResourcesAndDictionaries", musicxml.ProfileModelSemantics, func(profile validation.ProfileID, p *finalConfigurationIO) error {
			_, err := musicxml.AssessModelWithResourcesAndDictionaries(ctx, document, musicxml.LinkedDictionaryAssessmentOptions{Resources: linked(profile, p), Dictionaries: finalConfigurationDictionaries(p)})
			return err
		}},
		{"InspectMXLWithDictionaries", musicxml.ProfileMXLModelSemantics, func(profile validation.ProfileID, p *finalConfigurationIO) error {
			_, err := musicxml.InspectMXLWithDictionaries(ctx, p, musicxml.MXLDictionaryAssessmentOptions{Resources: mxl(profile, p), Dictionaries: finalConfigurationDictionaries(p)})
			return err
		}},
		{"AssessMXLPackageWithDictionaries", musicxml.ProfileMXLSourceSemantics, func(profile validation.ProfileID, p *finalConfigurationIO) error {
			_, err := musicxml.AssessMXLPackageWithDictionaries(ctx, &musicxml.MXLPackage{Document: document}, musicxml.MXLDictionaryAssessmentOptions{Resources: mxl(profile, p), Dictionaries: finalConfigurationDictionaries(p)})
			return err
		}},
	} {
		t.Run(entry.name, func(t *testing.T) {
			for _, selection := range []struct {
				name string
				id   validation.ProfileID
			}{{"empty", ""}, {"unknown", "test/unknown-profile"}, {"wrong-target", entry.wrong}} {
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
			options := musicxml.DictionaryAssessmentOptions{Assessment: musicxml.AssessmentOptions{Profile: musicxml.ProfileSourceDictionaries}, Dictionaries: finalConfigurationDictionaries(provider.source)}
			report, err := musicxml.InspectXMLWithDictionaries(ctx, strings.NewReader(finalConfigurationSMuFLScore), options)
			if err != nil {
				t.Fatal(err)
			}
			finalConfigurationRequireUnavailableDictionary(t, report)
			result, err := musicxml.DecodeAssessedWithDictionaries(ctx, strings.NewReader(finalConfigurationSMuFLScore), musicxml.DecodeDictionaryAssessmentOptions{Assessment: &options})
			if !errors.Is(err, musicxml.ErrIncompleteAssessment) || errors.Is(err, musicxml.ErrInvalidDocument) || result.Document != nil || result.Conversion.State != validation.ConversionConverted {
				t.Fatalf("strict incomplete result=%+v error=%v", result, err)
			}
			finalConfigurationRequireUnavailableDictionary(t, result.Source)
			options.Assessment.Profile = musicxml.ProfileModelDictionaries
			report, err = musicxml.AssessModelWithDictionaries(ctx, document, options)
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
			bundle, err := musicxml.InspectXMLWithResourcesAndDictionaries(context.Background(), strings.NewReader(input), musicxml.LinkedDictionaryAssessmentOptions{
				Resources: musicxml.LinkedAssessmentOptions{
					Graph:        musicxml.AssessmentOptions{Profile: musicxml.ProfileResourceGraphDictionaries},
					Dependencies: []musicxml.ResourceDependency{musicxml.ResourceDocuments}, Resolver: provider.resolver,
				},
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
