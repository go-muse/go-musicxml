package musicxml

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func dictionaryConsumerSet(t *testing.T, a *validation.Account, ranges bool) *validation.DictionarySet {
	t.Helper()
	raw, err := os.ReadFile("testdata/dictionaries/smufl-1.4/glyphnames.json")
	if err != nil {
		t.Fatal(err)
	}
	var glyphs map[string]struct {
		Codepoint string `json:"codepoint"`
	}
	if err = json.Unmarshal(raw, &glyphs); err != nil {
		t.Fatal(err)
	}
	data := &validation.SMuFLDictionaryData{}
	for name, row := range glyphs {
		v, err := strconv.ParseUint(strings.TrimPrefix(row.Codepoint, "U+"), 16, 32)
		if err != nil {
			t.Fatal(err)
		}
		data.Glyphs = append(data.Glyphs, validation.SMuFLGlyph{Name: name, Codepoint: uint32(v)})
	}
	selections := []validation.DictionarySelection{{Manifest: validation.DictionaryManifestSMuFL14Canonical, Relations: []validation.DictionaryRelation{validation.DictionaryRelationCanonicalName, validation.DictionaryRelationPrimaryCodepoint}}}
	imports := []validation.DictionaryImport{{Manifest: validation.DictionaryManifestSMuFL14Canonical, State: validation.EvidenceKnown, SMuFL: data}}
	if ranges {
		raw, err = os.ReadFile("testdata/dictionaries/smufl-1.4/ranges.json")
		if err != nil {
			t.Fatal(err)
		}
		var rows map[string]struct {
			Start string   `json:"range_start"`
			End   string   `json:"range_end"`
			Names []string `json:"glyphs"`
		}
		if err = json.Unmarshal(raw, &rows); err != nil {
			t.Fatal(err)
		}
		data := &validation.SMuFLDictionaryData{}
		for id, row := range rows {
			start, e := strconv.ParseUint(strings.TrimPrefix(row.Start, "U+"), 16, 32)
			if e != nil {
				t.Fatal(e)
			}
			end, e := strconv.ParseUint(strings.TrimPrefix(row.End, "U+"), 16, 32)
			if e != nil {
				t.Fatal(e)
			}
			data.Ranges = append(data.Ranges, validation.SMuFLRange{ID: id, Start: uint32(start), End: uint32(end), Names: row.Names})
		}
		selections = append(selections, validation.DictionarySelection{Manifest: validation.DictionaryManifestSMuFL14Ranges, Relations: []validation.DictionaryRelation{validation.DictionaryRelationRangeMembership}})
		imports = append(imports, validation.DictionaryImport{Manifest: validation.DictionaryManifestSMuFL14Ranges, State: validation.EvidenceKnown, SMuFL: data})
	}
	raw, err = os.ReadFile("testdata/dictionaries/musicxml-4.0/sounds.xml")
	if err != nil {
		t.Fatal(err)
	}
	var sounds struct {
		Rows []struct {
			ID string `xml:"id,attr"`
		} `xml:"sound"`
	}
	if err = xml.Unmarshal(raw, &sounds); err != nil {
		t.Fatal(err)
	}
	sd := &validation.StandardSoundsDictionaryData{}
	for _, row := range sounds.Rows {
		sd.IDs = append(sd.IDs, row.ID)
	}
	selections = append(selections, validation.DictionarySelection{Manifest: validation.DictionaryManifestStandardSounds40, Relations: []validation.DictionaryRelation{validation.DictionaryRelationStandardSound}})
	imports = append(imports, validation.DictionaryImport{Manifest: validation.DictionaryManifestStandardSounds40, State: validation.EvidenceKnown, Sounds: sd})
	raw, err = os.ReadFile("testdata/dictionaries/ipa-positive/runtime-positive-bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	var ipaRows struct {
		Records []struct {
			Type   string   `json:"record_type"`
			Fields []string `json:"fields"`
		} `json:"records"`
	}
	if err = json.Unmarshal(raw, &ipaRows); err != nil {
		t.Fatal(err)
	}
	ipa := &validation.IPAPositiveDictionaryData{}
	for _, row := range ipaRows.Records {
		f := row.Fields
		switch row.Type {
		case "symbol":
			ipa.Symbols = append(ipa.Symbols, validation.IPASymbolEncoding{Encoding: f[0], Category: f[1], RowID: f[2]})
		case "example-sequence":
			ipa.Examples = append(ipa.Examples, validation.IPAAttestedExample{Encoding: f[0], Category: f[1], RowID: f[2]})
		case "canonical-equivalence":
			ipa.Equivalences = append(ipa.Equivalences, validation.IPACanonicalEquivalent{Encoding: f[0], EquivalentToEncoding: f[1], RowID: f[2]})
		default:
			t.Fatalf("unreviewed IPA record: %s", row.Type)
		}
	}
	selections = append(selections, validation.DictionarySelection{Manifest: validation.DictionaryManifestIPAPositive2015Unicode13, Relations: []validation.DictionaryRelation{validation.DictionaryRelationIPASymbol, validation.DictionaryRelationIPAAttestedExample, validation.DictionaryRelationIPACanonicalEquivalent}})
	imports = append(imports, validation.DictionaryImport{Manifest: validation.DictionaryManifestIPAPositive2015Unicode13, State: validation.EvidenceKnown, IPA: ipa})
	set, err := validation.NewDictionarySet(context.Background(), a, selections, imports)
	if err != nil {
		t.Fatal(err)
	}
	return set
}
func dictionaryInspectTest(t *testing.T, text string, model, available bool, budgets ...validation.Budgets) (validation.Report, error) {
	t.Helper()
	ctx := context.Background()
	engine, err := newDictionaryAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	limits := validation.Budgets{}
	if len(budgets) != 0 {
		limits = budgets[0]
	}
	a, err := validation.NewBudgetAccount(limits)
	if err != nil {
		t.Fatal(err)
	}
	var set *validation.DictionarySet
	if available {
		set = dictionaryConsumerSet(t, a, true)
		defer set.Close()
	}
	profile := ProfileSourceDictionaries.ID()
	if model {
		profile = ProfileModelDictionaries.ID()
	}
	if strings.HasPrefix(text, "<sounds") {
		profile = ProfileSourceSoundsDictionaries.ID()
	}
	s, err := engine.BeginWithDictionaries(ctx, validation.RunOptions{Context: validation.ContextFamilyAll, DocumentID: "dictionary-test", Profile: profile, Account: a}, set)
	if err != nil {
		t.Fatal(err)
	}
	if model {
		doc, err := Decode(strings.NewReader(text))
		if err != nil {
			t.Fatal(err)
		}
		run := &modelAssessmentRun{ctx: ctx, document: doc, account: a, session: s, documentID: "dictionary-test"}
		_, walkErr := run.walk(modelWalkOptions{})
		report, finishErr := run.finish(walkErr)
		return report, errors.Join(walkErr, finishErr)
	}
	lexical, err := newSourceXMLTokenReader(strings.NewReader(text), sourceXMLConfig{Context: ctx, Account: a, ByteOwner: sourceBytesPrimary})
	if err != nil {
		t.Fatal(err)
	}
	observer, err := newSourceFactObserverWithIdentity(ctx, s, a, "dictionary-test", "")
	if err != nil {
		t.Fatal(err)
	}
	source := &observingXMLSource{ctx: ctx, lexical: lexical, decoder: xml.NewTokenDecoder(lexical), observer: observer}
	run := &sourceAssessmentRun{ctx: ctx, account: a, session: s, source: source, documentID: "dictionary-test"}
	readErr := run.drain()
	report, finishErr := run.finish(readErr)
	return report, errors.Join(readErr, finishErr)
}
func dictionaryResults(report validation.Report, property validation.ContextProperty) []validation.DictionaryContextResult {
	var out []validation.DictionaryContextResult
	for _, r := range report.Context {
		if r.Property == property && r.Dictionary != nil {
			out = append(out, *r.Dictionary)
		}
	}
	return out
}
func TestDictionaryCanonicalSourceModelAndIndependentFamily(t *testing.T) {
	cases := []struct {
		name, body       string
		available        bool
		state            validation.EvidenceState
		member, violated bool
	}{
		{"known", `<note><rest/><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental></note>`, true, validation.EvidenceKnown, true, false},
		{"nonmember", `<note><rest/><duration>1</duration><accidental smufl="accidentalPrivate">sharp</accidental></note>`, true, validation.EvidenceKnown, false, true},
		{"missing", `<note><rest/><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental></note>`, false, validation.EvidenceUnknown, false, false},
		{"family-independent", `<note><rest/><duration>1</duration><accidental smufl="restQuarter">sharp</accidental></note>`, false, validation.EvidenceInvalid, false, true},
	}
	for _, tc := range cases {
		for _, model := range []bool{false, true} {
			t.Run(tc.name+strconv.FormatBool(model), func(t *testing.T) {
				report, err := dictionaryInspectTest(t, contextFixtureXML(tc.body, false), model, tc.available)
				if err != nil {
					t.Fatal(err)
				}
				got := dictionaryResults(report, validation.ContextDictionaryCanonical)
				if len(got) != 1 {
					t.Fatalf("canonical count %d: %s", len(got), localFindings(report))
				}
				if got[0].Result.State != tc.state || tc.state == validation.EvidenceKnown && got[0].Result.Value != tc.member {
					t.Fatalf("membership: %+v", got[0])
				}
				if tc.state == validation.EvidenceInvalid && got[0].Result.Cause == nil {
					t.Fatal("lost XSD cause")
				}
				if (report.Conformance == validation.ConformanceViolated) != tc.violated {
					t.Fatalf("conformance: %s %s", report.Conformance, localFindings(report))
				}
			})
		}
	}
}
func TestDictionaryDefaultsWithoutDataAndRangeAdvice(t *testing.T) {
	text := contextFixtureXML(`<note><rest/><duration>1</duration><notehead smufl="noteLaWhole">other</notehead><notations><ornaments><tremolo type="unmeasured">0</tremolo><mordent/></ornaments><technical><stopped/><open/><half-muted/></technical></notations></note>`, false)
	for _, available := range []bool{false, true} {
		report, err := dictionaryInspectTest(t, text, false, available)
		if err != nil {
			t.Fatal(err)
		}
		defaults := map[string]string{}
		for _, r := range dictionaryResults(report, validation.ContextDictionaryRendering) {
			if r.Effective.State == validation.EvidenceKnown {
				defaults[r.Role] = r.Effective.Value.Text
			}
		}
		for role, want := range map[string]string{"stopped-smufl": "brassMuteClosed", "open-smufl": "brassMuteOpen", "half-muted-smufl": "brassMuteHalfClosed", "tremolo-smufl": "buzzRoll", "mordent-long": "no"} {
			if defaults[role] != want {
				t.Errorf("available=%v %s=%q", available, role, defaults[role])
			}
		}
		for _, r := range dictionaryResults(report, validation.ContextDictionaryAdvice) {
			if r.Role == "tremolo-range" || r.Role == "notehead-text-recommended" {
				want := validation.EvidenceUnknown
				if available {
					want = validation.EvidenceKnown
				}
				if r.Result.State != want || available && !r.Result.Value {
					t.Errorf("range: %+v", r)
				}
			}
		}
	}
}
func TestDictionaryActualTypeOneConclusionAndInvalidCause(t *testing.T) {
	for _, tc := range []struct {
		typ, value string
		state      validation.EvidenceState
	}{{"smufl-glyph-name", "restQuarter", validation.EvidenceKnown}, {"formatted-symbol", "restQuarter", validation.EvidenceKnown}, {"smufl-accidental-glyph-name", "restQuarter", validation.EvidenceInvalid}, {"unknownType", "restQuarter", validation.EvidenceInvalid}} {
		text := contextFixtureXML("", false)
		text = strings.Replace(text, "<part-list>", `<movement-title xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="`+tc.typ+`">`+tc.value+`</movement-title><part-list>`, 1)
		report, err := dictionaryInspectTest(t, text, false, true)
		if err != nil {
			t.Fatal(err)
		}
		rows := dictionaryResults(report, validation.ContextDictionaryCanonical)
		if tc.typ == "unknownType" {
			if len(rows) != 0 {
				t.Fatalf("unrelated fallback selected canonical: %+v", rows)
			}
			continue
		}
		if len(rows) != 1 || rows[0].Result.State != tc.state {
			t.Fatalf("%s: %+v", tc.typ, rows)
		}
	}
}
func TestDictionarySoundsCustomAllowedAndIPAGate(t *testing.T) {
	for _, text := range []string{`<sounds><sound id="keyboard.piano"><any/></sound><sound id="custom.allowed"><solo/></sound></sounds>`, strings.Replace(contextFixtureXML("", false), "<part-name>Piano</part-name>", "<part-name>Piano</part-name><score-instrument id=\"I1\"><instrument-name>Piano</instrument-name><instrument-sound>keyboard.piano</instrument-sound></score-instrument>", 1)} {
		report, err := dictionaryInspectTest(t, text, false, true)
		if err != nil {
			t.Fatal(err)
		}
		rows := dictionaryResults(report, validation.ContextDictionarySounds)
		if len(rows) == 0 {
			t.Fatalf("no sound rows: %s", text)
		}
		if report.Conformance == validation.ConformanceViolated {
			t.Fatalf("open sound rejected: %s", localFindings(report))
		}
	}
	text := contextFixtureXML(`<note><rest/><duration>1</duration><play><ipa>a</ipa></play></note>`, false)
	report, err := dictionaryInspectTest(t, text, false, true)
	if err != nil {
		t.Fatal(err)
	}
	rows := dictionaryResults(report, validation.ContextDictionaryIPA)
	if len(rows) != 3 {
		t.Fatalf("IPA relations=%d", len(rows))
	}
	if report.Conformance != validation.ConformanceUndetermined {
		t.Fatalf("partial IPA closed mandatory gate: %s", localFindings(report))
	}
}

func TestDictionaryDefaultOutputScalarBudgetIsTerminal(t *testing.T) {
	text := contextFixtureXML(`<note><rest/><duration>1</duration><notations><technical><stopped/></technical></notations></note>`, false)
	for _, model := range []bool{false, true} {
		report, err := dictionaryInspectTest(t, text, model, false, validation.Budgets{MaxScalarBytes: 14})
		if !errors.Is(err, validation.ErrResourceLimit) || report.AssessmentComplete || report.Terminal == nil {
			t.Fatalf("default output cap bypass: model=%v report=%+v err=%v", model, report, err)
		}
		for _, r := range dictionaryResults(report, validation.ContextDictionaryRendering) {
			if r.Role == "stopped-smufl" {
				t.Fatal("oversized default became visible")
			}
		}
	}
}
