package musicxml

import (
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func TestDictionaryIPAPositiveNeverClosesSequence(t *testing.T) {
	for _, tc := range []struct {
		name, value         string
		symbols             validation.EvidenceState
		example, equivalent bool
	}{{"all-atoms", "aba", validation.EvidenceKnown, false, false}, {"example", "è", validation.EvidenceKnown, true, false}, {"equivalence", "é", validation.EvidenceUnknown, false, true}, {"unknown", "a?b", validation.EvidenceUnknown, false, false}, {"empty", "", validation.EvidenceUnknown, false, false}, {"space", " ", validation.EvidenceUnknown, false, false}, {"compatibility-not-normalized", "ａ", validation.EvidenceUnknown, false, false}, {"leading-mark", "́", validation.EvidenceKnown, false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			text := contextFixtureXML(`<note><rest/><duration>1</duration><play><ipa>`+tc.value+`</ipa></play></note>`, false)
			for _, model := range []bool{false, true} {
				report, err := dictionaryInspectTest(t, text, model, true)
				if err != nil {
					t.Fatal(err)
				}
				if report.Conformance != validation.ConformanceUndetermined {
					t.Fatalf("closed IPA: %s", localFindings(report))
				}
				rows := dictionaryResults(report, validation.ContextDictionaryIPA)
				if len(rows) != 3 {
					t.Fatalf("relations=%d", len(rows))
				}
				for _, r := range rows {
					if r.Input.Value.Text != tc.value {
						t.Fatalf("input normalized: %+v", r.Input)
					}
					if r.WholeSequence.State != validation.EvidenceUnsupported {
						t.Fatalf("full sequence: %+v", r.WholeSequence)
					}
					if r.Result.State == validation.EvidenceKnown && !r.Result.Value {
						t.Fatal("partial table asserted negative membership")
					}
					switch r.Relation {
					case validation.DictionaryRelationIPASymbol:
						if r.Result.State != tc.symbols {
							t.Errorf("symbols: %+v", r)
						}
					case validation.DictionaryRelationIPAAttestedExample:
						if (r.Result.State == validation.EvidenceKnown) != tc.example {
							t.Errorf("example: %+v", r)
						}
					case validation.DictionaryRelationIPACanonicalEquivalent:
						if (r.Result.State == validation.EvidenceKnown) != tc.equivalent {
							t.Errorf("equivalence: %+v", r)
						}
					}
					if len(r.Spans) == 0 {
						t.Fatal("lost empty/unknown span")
					}
					pos := uint64(0)
					for _, span := range r.Spans {
						if span.Start != pos || span.End < span.Start || span.End > uint64(len(tc.value)) || tc.value[span.Start:span.End] != span.Text {
							t.Fatalf("span coverage: %+v", r.Spans)
						}
						pos = span.End
					}
					if pos != uint64(len(tc.value)) {
						t.Fatalf("span suffix lost: %+v", r.Spans)
					}
				}
			}
		})
	}
}

func TestDictionaryENTITYAndSemanticUseRemainIndependent(t *testing.T) {
	for _, element := range []string{"ipa", "instrument-sound"} {
		body := `<note><rest/><duration>1</duration><play><ipa xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" xsi:type="xs:ENTITY">entityName</ipa></play></note>`
		text := contextFixtureXML(body, false)
		property := validation.ContextDictionaryIPA
		if element == "instrument-sound" {
			text = replaceDictionaryInstrumentSound(`<instrument-sound xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" xsi:type="xs:ENTITY">entityName</instrument-sound>`)
			property = validation.ContextDictionarySounds
		}
		report, err := dictionaryInspectTest(t, text, false, true)
		if err != nil {
			t.Fatal(err)
		}
		rows := dictionaryResults(report, property)
		if len(rows) == 0 {
			t.Fatal("semantic role erased by actual type")
		}
		for _, r := range rows {
			if r.Result.State == validation.EvidenceKnown || r.Result.State == validation.EvidenceAbsent {
				t.Fatalf("ENTITY fabricated available evidence: %+v", r)
			}
		}
	}
}

func replaceDictionaryInstrumentSound(value string) string {
	return strings.Replace(contextFixtureXML("", false), "</score-part>", `<score-instrument id="I1"><instrument-name>Piano</instrument-name>`+value+`</score-instrument></score-part>`, 1)
}
