package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"reflect"
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
	for _, wrapper := range []bool{false, true} {
		for _, options := range mandatoryAssessmentOptions() {
			t.Run(mandatoryLoaderName(wrapper)+"/"+options.name, func(t *testing.T) {
				result, err := mandatoryDecode(wrapper, context.Background(), strings.NewReader(source), options.value)
				if result.Document != nil || !errors.Is(err, ErrInvalidDocument) {
					t.Fatalf("invalid source accepted: document=%T error=%v", result.Document, err)
				}
				if !result.Source.Requested || result.Source.Profile.ID != ProfileSourceSemantics || result.Source.Conformance != validation.ConformanceViolated {
					t.Fatalf("mandatory source assessment missing: requested=%v profile=%s conformance=%s", result.Source.Requested, result.Source.Profile.ID, result.Source.Conformance)
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
	for _, wrapper := range []bool{false, true} {
		for _, options := range mandatoryAssessmentOptions() {
			t.Run(mandatoryLoaderName(wrapper)+"/"+options.name, func(t *testing.T) {
				reader := &assessmentCountingReader{source: strings.NewReader(source)}
				result, err := mandatoryDecode(wrapper, context.Background(), reader, options.value)
				if err != nil || result.Document == nil || result.Conversion.State != validation.ConversionConverted {
					t.Fatalf("valid load failed: document=%T conversion=%s error=%v", result.Document, result.Conversion.State, err)
				}
				if reader.closed || reader.bytes != len(source) {
					t.Fatalf("reader ownership changed: closed=%v bytes=%d want=%d", reader.closed, reader.bytes, len(source))
				}
				mandatorySatisfiedReport(t, result.Source, ProfileSourceSemantics, validation.TargetSource)
				mandatorySatisfiedReport(t, mandatoryModelReport(t, result), ProfileModelSemantics, validation.TargetModel)
			})
		}
	}
}

func TestMandatoryAssessedLoadingRejectsNarrowProfilesBeforeRead(t *testing.T) {
	for _, wrapper := range []bool{false, true} {
		for _, profile := range []validation.ProfileID{
			ProfileSourceXSDPreview, ProfileSourceLocal, ProfileSourceContext,
			ProfileSourceDictionaries, ProfileSourceInternalDTDDictionaries,
			ProfileModelSemantics, ProfileMXLSourceSemantics, "unknown-profile",
		} {
			t.Run(mandatoryLoaderName(wrapper)+"/"+string(profile), func(t *testing.T) {
				reader := &assessmentCountingReader{source: strings.NewReader(`<opus/>`)}
				result, err := mandatoryDecode(wrapper, context.Background(), reader, &AssessmentOptions{Profile: profile})
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
	for _, wrapper := range []bool{false, true} {
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
			t.Run(mandatoryLoaderName(wrapper)+"/"+tc.name, func(t *testing.T) {
				result, err := mandatoryDecode(wrapper, tc.ctx, strings.NewReader(tc.source), &AssessmentOptions{Budgets: tc.budgets})
				if err == nil || result.Document != nil || (tc.want != nil && !errors.Is(err, tc.want)) {
					t.Fatalf("incomplete source accepted: document=%T error=%v want=%v", result.Document, err, tc.want)
				}
				if tc.name == "malformed-tail" {
					var syntax *xml.SyntaxError
					if !errors.As(err, &syntax) {
						t.Fatalf("malformed source error lost: %v", err)
					}
				}
				if tc.name == "normative-unknown" && (result.Source.Counts.Unknown == 0 || result.Source.Conformance == validation.ConformanceViolated) {
					t.Fatalf("Unknown must stay incomplete: counts=%+v conformance=%s", result.Source.Counts, result.Source.Conformance)
				}
				if mandatoryModelReport(t, result).Requested {
					t.Fatal("model assessment ran after an unaccepted source")
				}
			})
		}
	}
}

func TestMandatoryAssessedLoadingRejectsModelInvalidAfterDecimalUnderflow(t *testing.T) {
	// Both positive decimal values are exact and equal in the source. The
	// float64-backed fields become zero during ordinary, successful conversion.
	tiny := "0." + strings.Repeat("0", 399) + "1"
	source := mandatoryScore(tiny, `<note><rest/><duration>`+tiny+`</duration></note>`)
	sourceReport, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceSemantics})
	if err != nil || !sourceReport.Satisfied() {
		t.Fatalf("fixture source must satisfy semantics: conformance=%s counts=%+v error=%v findings=%s", sourceReport.Conformance, sourceReport.Counts, err, localFindings(sourceReport))
	}
	document, err := Decode(strings.NewReader(source))
	if err != nil || document == nil {
		t.Fatalf("fixture must convert successfully: document=%T error=%v", document, err)
	}
	modelReport, err := AssessModel(context.Background(), document, AssessmentOptions{Profile: ProfileModelSemantics})
	if err != nil || modelReport.Conformance != validation.ConformanceViolated {
		t.Fatalf("fixture model must violate semantics: conformance=%s error=%v findings=%s", modelReport.Conformance, err, localFindings(modelReport))
	}
	for _, wrapper := range []bool{false, true} {
		t.Run(mandatoryLoaderName(wrapper), func(t *testing.T) {
			result, err := mandatoryDecode(wrapper, context.Background(), strings.NewReader(source), nil)
			if result.Document != nil || !errors.Is(err, ErrInvalidDocument) || result.Conversion.State != validation.ConversionConverted {
				t.Fatalf("converted invalid model accepted: document=%T conversion=%s error=%v", result.Document, result.Conversion.State, err)
			}
			mandatorySatisfiedReport(t, result.Source, ProfileSourceSemantics, validation.TargetSource)
			model := mandatoryModelReport(t, result)
			if !model.Requested || model.Profile.ID != ProfileModelSemantics || model.Conformance != validation.ConformanceViolated || model.Satisfied() {
				t.Fatalf("independent model failure missing: requested=%v profile=%s conformance=%s", model.Requested, model.Profile.ID, model.Conformance)
			}
		})
	}
}

func mandatoryDecode(wrapper bool, ctx context.Context, reader io.Reader, options *AssessmentOptions) (DecodeResult, error) {
	if !wrapper {
		return DecodeAssessed(ctx, reader, DecodeAssessmentOptions{Assessment: options})
	}
	var dictionaryOptions *DictionaryAssessmentOptions
	if options != nil {
		dictionaryOptions = &DictionaryAssessmentOptions{Assessment: *options}
	}
	return DecodeAssessedWithDictionaries(ctx, reader, DecodeDictionaryAssessmentOptions{Assessment: dictionaryOptions})
}

func mandatoryAssessmentOptions() []struct {
	name  string
	value *AssessmentOptions
} {
	return []struct {
		name  string
		value *AssessmentOptions
	}{
		{name: "nil-assessment"},
		{name: "empty-assessment", value: &AssessmentOptions{}},
		{name: "explicit-semantics", value: &AssessmentOptions{Profile: ProfileSourceSemantics}},
	}
}

func mandatoryLoaderName(wrapper bool) string {
	if wrapper {
		return "DecodeAssessedWithDictionaries"
	}
	return "DecodeAssessed"
}

func mandatoryScore(divisions, notes string) string {
	return `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>` +
		`<part id="P1"><measure number="1"><attributes><divisions>` + divisions + `</divisions></attributes>` + notes + `</measure></part></score-partwise>`
}

func mandatoryModelReport(t *testing.T, result DecodeResult) validation.Report {
	t.Helper()
	field := reflect.ValueOf(result).FieldByName("Model")
	if !field.IsValid() {
		t.Fatal("DecodeResult must expose the independent Model assessment report")
	}
	report, ok := field.Interface().(validation.Report)
	if !ok {
		t.Fatal("DecodeResult.Model must be a validation.Report")
	}
	return report
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
