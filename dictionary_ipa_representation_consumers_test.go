package musicxml

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func dictionaryRepresentationTyped(t *testing.T, raw []byte, id validation.DictionaryManifestID) validation.DictionaryImport {
	t.Helper()
	var b struct {
		Records []struct {
			Kind   string    `json:"record_type"`
			Fields [3]string `json:"fields"`
		}
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	p := &validation.IPAPositiveDictionaryData{}
	for _, r := range b.Records {
		f := r.Fields
		switch r.Kind {
		case "symbol":
			p.Symbols = append(p.Symbols, validation.IPASymbolEncoding{Encoding: f[0], Category: f[1], RowID: f[2]})
		case "canonical-equivalence":
			p.Equivalences = append(p.Equivalences, validation.IPACanonicalEquivalent{Encoding: f[0], EquivalentToEncoding: f[1], RowID: f[2]})
		case "example-sequence":
			p.Examples = append(p.Examples, validation.IPAAttestedExample{Encoding: f[0], Category: f[1], RowID: f[2]})
		}
	}
	return validation.DictionaryImport{Manifest: id, State: validation.EvidenceKnown, IPA: p}
}
func dictionaryRepresentationDirect(t *testing.T, text string, model, legacy, selected, known bool) validation.Report {
	t.Helper()
	ctx := context.Background()
	engine, err := newDictionaryAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	a := dictionaryImportAccount(t, validation.Budgets{})
	var selections []validation.DictionarySelection
	var imports []validation.DictionaryImport
	if selected {
		selections = append(selections, dictionaryRepresentationTestSelection())
		if known {
			imports = append(imports, dictionaryRepresentationTyped(t, dictionaryRepresentationFixture(t), validation.DictionaryManifestIPARepresentation2015Unicode13))
		}
	}
	if legacy {
		selections = append(selections, dictionaryImportSelection(validation.DictionaryManifestIPAPositive2015Unicode13))
		imports = append(imports, dictionaryRepresentationTyped(t, dictionaryImportFixture(t, validation.DictionaryManifestIPAPositive2015Unicode13), validation.DictionaryManifestIPAPositive2015Unicode13))
	}
	set, err := validation.NewDictionarySet(ctx, a, selections, imports)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { set.Close(); dictionaryImportNoLive(t, a) }()
	profile := ProfileSourceDictionaries
	if model {
		profile = ProfileModelDictionaries
	}
	s, err := engine.BeginWithDictionaries(ctx, validation.RunOptions{DocumentID: "representation", Profile: profile, Account: a}, set)
	if err != nil {
		t.Fatal(err)
	}
	if model {
		doc, err := Decode(strings.NewReader(text))
		if err != nil {
			t.Fatal(err)
		}
		run := &modelAssessmentRun{ctx: ctx, document: doc, account: a, session: s, documentID: "representation"}
		_, walkErr := run.walk(modelWalkOptions{})
		report, finishErr := run.finish(walkErr)
		if err := errors.Join(walkErr, finishErr); err != nil {
			t.Fatal(err)
		}
		return report
	}
	lexical, err := newSourceXMLTokenReader(strings.NewReader(text), sourceXMLConfig{Context: ctx, Account: a, ByteOwner: sourceBytesPrimary})
	if err != nil {
		t.Fatal(err)
	}
	observer, err := newSourceFactObserverWithIdentity(ctx, s, a, "representation", "")
	if err != nil {
		t.Fatal(err)
	}
	source := &observingXMLSource{ctx: ctx, lexical: lexical, decoder: xml.NewTokenDecoder(lexical), observer: observer}
	run := &sourceAssessmentRun{ctx: ctx, account: a, session: s, source: source, documentID: "representation"}
	readErr := run.drain()
	report, finishErr := run.finish(readErr)
	if err := errors.Join(readErr, finishErr); err != nil {
		t.Fatal(err)
	}
	return report
}
func TestIPARepresentationSelectedConsumerWholeInput(t *testing.T) {
	for _, value := range []string{"", "   ", "ḉ\u0334", "t\u0301\ufe20s\u0300\ufe21", "a\u0327", "aba?", "t\ufe20s\ufe21\ufe20a\ufe21"} {
		good := value != "a\u0327" && value != "aba?" && value != "t\ufe20s\ufe21\ufe20a\ufe21"
		for _, model := range []bool{false, true} {
			for _, legacy := range []bool{false, true} {
				text := contextFixtureXML(`<note><rest/><duration>1</duration><play><ipa>`+value+`</ipa></play></note>`, false)
				report := dictionaryRepresentationDirect(t, text, model, legacy, true, true)
				rows := dictionaryResults(report, validation.ContextProperty("dictionary.ipa-representation"))
				if len(rows) != 1 {
					t.Fatalf("model=%v legacy=%v value=%q representation rows=%d", model, legacy, value, len(rows))
				}
				r := rows[0]
				if (r.Result.State == validation.EvidenceKnown) != good || (r.WholeSequence.State == validation.EvidenceKnown) != good || r.Input.Value.Text != value {
					t.Fatalf("value=%q result=%+v", value, r)
				}
				if good && (!r.Result.Value || !r.WholeSequence.Value) {
					t.Fatal("positive witness not true")
				}
				if !good && (r.Result.State != validation.EvidenceUnknown || r.WholeSequence.State != validation.EvidenceUnknown) {
					t.Fatal("unproved input became negative/absent")
				}
				if len(r.Spans) != 1 || r.Spans[0].Start != 0 || r.Spans[0].End != uint64(len(value)) || r.Spans[0].Text != value || r.Spans[0].RowID != "" || r.Spans[0].Category != "whole-representation" {
					t.Fatalf("whole provenance=%+v", r.Spans)
				}
				if got := dictionaryResults(report, validation.ContextDictionaryIPA); len(got) != 3 {
					t.Fatalf("legacy projections=%d", len(got))
				} else {
					for _, old := range got {
						if old.WholeSequence.State != validation.EvidenceUnsupported {
							t.Fatal("promoted legacy WholeSequence")
						}
					}
				}
				for _, d := range report.Findings {
					if strings.Contains(string(d.Rule.ID), "/ipa-positive") && good {
						t.Fatalf("legacy gate masked witness: %+v", d)
					}
				}
			}
		}
	}
}
func TestIPARepresentationSelectionAndMissingCapability(t *testing.T) {
	text := contextFixtureXML(`<note><rest/><duration>1</duration><play><ipa></ipa></play></note>`, false)
	for _, selected := range []bool{false, true} {
		r := dictionaryRepresentationDirect(t, text, false, true, selected, false)
		rows := dictionaryResults(r, validation.ContextProperty("dictionary.ipa-representation"))
		if !selected {
			if len(rows) != 0 {
				t.Fatal("implicitly activated representation")
			}
			continue
		}
		if len(rows) != 1 || rows[0].Result.State != validation.EvidenceUnknown || len(rows[0].Spans) != 1 || rows[0].Spans[0].Text != "" {
			t.Fatalf("missing capability lost empty raw span: %+v", rows)
		}
	}
}
