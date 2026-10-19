package musicxml

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourceDeclarationCompositionDictionaryEffectiveValues(t *testing.T) {
	for _, tc := range []struct {
		name, value       string
		available, member bool
	}{
		{"canonical", "accidentalSharp", true, true},
		{"nonmember", "accidentalPrivate", true, false},
		{"unavailable", "accidentalSharp", false, false},
	} {
		for _, physical := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/default", true: "/entity"}[physical], func(t *testing.T) {
				declaration := `<!ATTLIST accidental smufl CDATA "` + tc.value + `">`
				attribute := ""
				if physical {
					declaration = `<!ENTITY glyph "` + tc.value + `">`
					attribute = ` smufl="&glyph;"`
				}
				input := `<!DOCTYPE score-partwise [` + declaration + `]>` + contextFixtureXML(`<note><rest/><duration>1</duration><accidental`+attribute+`>sharp</accidental></note>`, false)
				options := dictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceInternalDTDDictionaries}}
				calls := 0
				if tc.available {
					options.Dictionaries.Selections = []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical)}
					options.Dictionaries.Source = dictionaryImportSourceFunc(func(_ context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
						calls++
						return DictionaryComponentResult{Request: request, State: validation.EvidenceKnown, Body: &resourceTestBody{Reader: bytes.NewReader(dictionaryImportFixture(t, request.Manifest))}}, nil
					})
				}
				report, err := inspectXMLWithDictionaries(context.Background(), strings.NewReader(input), options)
				if err != nil {
					t.Fatal(err)
				}
				rows := dictionaryResults(report, validation.ContextDictionaryCanonical)
				if len(rows) != 1 {
					t.Fatalf("rows=%d findings=%s", len(rows), localFindings(report))
				}
				want := validation.EvidenceUnknown
				if tc.available {
					want = validation.EvidenceKnown
				}
				if rows[0].Result.State != want || tc.available && rows[0].Result.Value != tc.member {
					t.Fatalf("membership=%+v", rows[0])
				}
				if (report.Conformance == validation.ConformanceViolated) != (tc.available && !tc.member) {
					t.Fatalf("conformance=%s findings=%s", report.Conformance, localFindings(report))
				}
				if calls != map[bool]int{false: 0, true: 1}[tc.available] {
					t.Fatal(calls)
				}
			})
		}
	}
}

func TestSourceDeclarationCompositionDictionaryPreconditions(t *testing.T) {
	calls := 0
	options := dictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceInternalDTDDictionaries}, Dictionaries: DictionaryOptions{
		Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical)},
		Source: dictionaryImportSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
			calls++
			return DictionaryComponentResult{}, errors.New("unexpected import")
		}),
	}}
	var reader *strings.Reader
	if _, err := inspectXMLWithDictionaries(context.Background(), reader, options); !errors.Is(err, ErrNilReader) {
		t.Fatal(err)
	}
	if _, err := assessModelWithDictionaries(context.Background(), &OpusDocument{}, options); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("invalid invocation imported %d components", calls)
	}
}

func TestSourceDeclarationCompositionStrictDecode(t *testing.T) {
	input := `<!DOCTYPE opus [<!ATTLIST opus version CDATA "4.0">]><opus/>`
	result, err := decodeAssessedWithDictionaries(context.Background(), strings.NewReader(input), decodeDictionaryTestOptions{Assessment: &dictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceLine}}})
	if err != nil || result.Document == nil || !result.SourceLine.Satisfied() || !result.ModelLine.Satisfied() {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}
