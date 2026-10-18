package musicxml

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Match the public dictionary entry point's account, transport, and finalization
// path while retaining its private account for directly observed work evidence.
func dictionaryReviewWork(t *testing.T, count int, accidental, dictionaries, available bool, sparse ...bool) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	var cfg assessmentRunConfig
	var err error
	if dictionaries {
		opts := dictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries}}
		if available {
			for _, manifest := range []validation.DictionaryManifestID{validation.DictionaryManifestSMuFL14Canonical, validation.DictionaryManifestSMuFL14Ranges, validation.DictionaryManifestStandardSounds40, validation.DictionaryManifestIPAPositive2015Unicode13} {
				opts.Dictionaries.Selections = append(opts.Dictionaries.Selections, dictionaryImportSelection(manifest))
			}
			var calls []DictionaryComponentRequest
			opts.Dictionaries.Source = dictionaryImportFixtureSource(t, &calls)
		}
		cfg, err = dictionaryAssessmentConfig(ctx, opts, validation.TargetSource)
		if err != nil {
			t.Fatal(err)
		}
		defer cfg.Dictionaries.Close()
	} else {
		cfg.Account, err = validation.NewBudgetAccount(validation.Budgets{})
		if err != nil {
			t.Fatal(err)
		}
		cfg.Engine, err = sourceAssessmentEngineForProfile(ProfileSourceLinks)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Profile, cfg.DocumentID = ProfileSourceLinks, sourceDocumentID
	}
	before, _ := cfg.Account.Usage(validation.BudgetRuleSteps)
	body := `<note><rest/><duration>1</duration><type>quarter</type>`
	if accidental {
		body += `<accidental smufl="accidentalSharp">sharp</accidental>`
	}
	body += `</note>`
	text := strings.Repeat(body, count)
	if len(sparse) != 0 && sparse[0] {
		text = strings.Repeat(`<note><rest/><duration>1</duration><type>quarter</type></note>`, count-1) + body
	}
	run, err := newSourceAssessmentRunWithConfig(ctx, strings.NewReader(resourceScore(text)), DecodeOptions{}, &sourceAssessmentConfig{assessmentRunConfig: cfg, ByteOwner: sourceBytesPrimary}, false)
	if err != nil {
		t.Fatal(err)
	}
	inputErr := run.drain()
	report, finishErr := run.finish(inputErr)
	if inputErr != nil || finishErr != nil || !report.InputComplete || report.Terminal != nil || report.Counts.Fail != 0 {
		t.Fatalf("report=%+v input=%v finish=%v", report, inputErr, finishErr)
	}
	after, _ := cfg.Account.Usage(validation.BudgetRuleSteps)
	return after - before, before
}

func TestDictionaryReviewMatchedConsumerWork(t *testing.T) {
	for _, count := range []int{51, 101} {
		for _, accidental := range []bool{false, true} {
			t.Run(fmt.Sprintf("notes-%d/accidental-%t", count, accidental), func(t *testing.T) {
				links, _ := dictionaryReviewWork(t, count, accidental, false, false)
				empty, emptyImport := dictionaryReviewWork(t, count, accidental, true, false)
				full, fullImport := dictionaryReviewWork(t, count, accidental, true, true)
				t.Logf("links=%d dictionary-empty=%d dictionary-full=%d empty-import=%d full-import=%d", links, empty, full, emptyImport, fullImport)
			})
		}
	}
}

func TestDictionaryReviewSparseSubjectWork(t *testing.T) {
	for _, count := range []int{51, 101} {
		t.Run(fmt.Sprintf("notes-%d/one-accidental", count), func(t *testing.T) {
			links, _ := dictionaryReviewWork(t, count, true, false, false, true)
			empty, _ := dictionaryReviewWork(t, count, true, true, false, true)
			full, imported := dictionaryReviewWork(t, count, true, true, true, true)
			t.Logf("links=%d dictionary-empty=%d dictionary-full=%d full-import=%d", links, empty, full, imported)
		})
	}
}

func TestDictionaryTremoloRangeRetainsRawAndEffectiveEvidence(t *testing.T) {
	for _, model := range []bool{false, true} {
		for _, tc := range []struct {
			name, attrs, content string
			raw                  validation.EvidenceState
			effective            string
			member               bool
		}{
			{"default", "", "", validation.EvidenceAbsent, "buzzRoll", true},
			{"default-valid-content", "", "0", validation.EvidenceAbsent, "buzzRoll", true},
			{"explicit-member", ` smufl="buzzRoll"`, "0", validation.EvidenceKnown, "buzzRoll", true},
			{"explicit-nonmember", ` smufl="restQuarter"`, "0", validation.EvidenceKnown, "restQuarter", false},
		} {
			t.Run(fmt.Sprintf("model-%t/%s", model, tc.name), func(t *testing.T) {
				report, err := dictionaryInspectTest(t, contextFixtureXML(`<note><rest/><duration>1</duration><notations><ornaments><tremolo type="unmeasured"`+tc.attrs+`>`+tc.content+`</tremolo></ornaments></notations></note>`, false), model, true)
				if err != nil {
					t.Fatal(err)
				}
				found := 0
				for _, row := range dictionaryResults(report, validation.ContextDictionaryAdvice) {
					if row.Role != "tremolo-range" {
						continue
					}
					found++
					if row.Input.State != tc.raw || row.Effective.State != validation.EvidenceKnown || row.Effective.Value.Text != tc.effective || row.Attribute == nil || row.Attribute.Name.Local != "smufl" || row.Result.State != validation.EvidenceKnown || row.Result.Value != tc.member {
						t.Errorf("raw/effective range provenance mismatch: %+v", row)
					}
				}
				if found != 1 {
					t.Fatalf("range count=%d", found)
				}
			})
		}
	}
}

func TestDictionaryTremoloRangeInvalidAndInactiveControls(t *testing.T) {
	for _, model := range []bool{false, true} {
		for _, available := range []bool{false, true} {
			for _, tc := range []struct {
				name, attrs string
				rangeCount  int
				invalid     bool
			}{
				{"invalid-smufl", ` type="unmeasured" smufl="bad glyph"`, 1, true},
				{"invalid-type", ` type="bad-type" smufl="buzzRoll"`, 0, false},
				{"default-single", ` smufl="buzzRoll"`, 0, false},
				{"explicit-single", ` type="single" smufl="buzzRoll"`, 0, false},
			} {
				t.Run(fmt.Sprintf("model-%t/available-%t/%s", model, available, tc.name), func(t *testing.T) {
					text := contextFixtureXML(`<note><rest/><duration>1</duration><notations><ornaments><tremolo`+tc.attrs+`>0</tremolo></ornaments></notations></note>`, false)
					report, err := dictionaryInspectTest(t, text, model, available)
					if err != nil {
						t.Fatal(err)
					}
					count := 0
					for _, row := range dictionaryResults(report, validation.ContextDictionaryAdvice) {
						if row.Role != "tremolo-range" {
							continue
						}
						count++
						if tc.invalid && (row.Input.State != validation.EvidenceInvalid || row.Input.Cause == nil || !reflect.DeepEqual(row.Effective, row.Input) || row.Result.State != validation.EvidenceInvalid || row.Result.Cause == nil || *row.Result.Cause != *row.Input.Cause) {
							t.Errorf("invalid raw input lost provenance: %+v", row)
						}
					}
					if count != tc.rangeCount {
						t.Errorf("range count=%d want=%d", count, tc.rangeCount)
					}
				})
			}
		}
	}
}

func TestDictionarySemanticRolesCoexistWithDynamicCanonicalType(t *testing.T) {
	for _, semantic := range []string{"ipa", "sound"} {
		for _, model := range []bool{false, true} {
			for _, available := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/model-%t/available-%t", semantic, model, available), func(t *testing.T) {
					text := contextFixtureXML(`<note><rest/><duration>1</duration><play><ipa xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="smufl-glyph-name">restQuarter</ipa></play></note>`, false)
					property := validation.ContextDictionaryIPA
					count := 3
					if semantic == "sound" {
						text = replaceDictionaryInstrumentSound(`<instrument-sound xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="smufl-glyph-name">restQuarter</instrument-sound>`)
						property = validation.ContextDictionarySounds
						count = 1
					}
					report, err := dictionaryInspectTest(t, text, model, available)
					if err != nil {
						t.Fatal(err)
					}
					canonical := dictionaryResults(report, validation.ContextDictionaryCanonical)
					want := 1
					if model {
						want = 0
					}
					if len(canonical) != want {
						t.Fatalf("actual type canonical rows=%d want=%d", len(canonical), want)
					}
					if !model && available && (canonical[0].Result.State != validation.EvidenceKnown || !canonical[0].Result.Value) {
						t.Fatalf("canonical lookup lost: %+v", canonical[0])
					}
					rows := dictionaryResults(report, property)
					if len(rows) != count {
						t.Fatalf("semantic rows=%d want=%d", len(rows), count)
					}
					for _, row := range rows {
						if row.Input.State != validation.EvidenceKnown || row.Input.Value.Text != "restQuarter" {
							t.Errorf("semantic input lost: %+v", row)
						}
						if semantic == "ipa" && row.WholeSequence.State != validation.EvidenceUnsupported {
							t.Errorf("canonical proof promoted complete IPA repertoire: %+v", row)
						}
						if semantic == "sound" && available && (row.Result.State != validation.EvidenceKnown || row.Result.Value) {
							t.Errorf("canonical proof promoted sound membership: %+v", row)
						}
					}
				})
			}
		}
	}
}
