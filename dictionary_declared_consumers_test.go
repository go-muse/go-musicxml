package musicxml

import (
	"github.com/go-muse/go-musicxml/validation"
	"strconv"
	"strings"
	"testing"
)

func TestDictionaryAllTwentySevenDeclaredUses(t *testing.T) {
	note := func(s string) string { return `<note><rest/><duration>1</duration>` + s + `</note>` }
	technical := func(s string) string { return note(`<notations><technical>` + s + `</technical></notations>`) }
	ornament := func(s string) string { return note(`<notations><ornaments>` + s + `</ornaments></notations>`) }
	direction := func(s string) string { return `<direction><direction-type>` + s + `</direction-type></direction>` }
	percussion := func(s string) string { return direction(`<percussion>` + s + `</percussion>`) }
	type fixture struct {
		name, header, part, body string
		count                    int
	}
	fixtures := []fixture{
		{name: "accidental", body: note(`<accidental smufl="accidentalSharp">sharp</accidental>`)},
		{name: "accidental-mark", body: note(`<notations><accidental-mark smufl="accidentalSharp">sharp</accidental-mark></notations>`)},
		{name: "accidental-text", part: `<part-name-display><accidental-text smufl="accidentalSharp">sharp</accidental-text></part-name-display>`},
		{name: "glyph", header: `<defaults><appearance><glyph type="quarter-rest">restQuarter</glyph></appearance></defaults>`},
		{name: "arrow", body: technical(`<arrow smufl="arrowBlackUp"><arrow-direction>up</arrow-direction></arrow>`)},
		{name: "coda", body: direction(`<coda smufl="coda"/>`)},
		{name: "credit-symbol-both-exact-uses", header: `<credit><credit-symbol>restQuarter</credit-symbol><credit-symbol>restQuarter</credit-symbol></credit>`, count: 2},
		{name: "symbol", body: direction(`<symbol>restQuarter</symbol>`)},
		{name: "effect", body: percussion(`<effect smufl="pictAgogo">anvil</effect>`)},
		{name: "elision", body: note(`<lyric><text>a</text><elision smufl="lyricsElision"> </elision><text>b</text></lyric>`)},
		{name: "empty-placement-smufl", body: technical(`<stopped smufl="brassMuteClosed"/>`)},
		{name: "glass", body: percussion(`<glass smufl="pictAgogo">wind chimes</glass>`)},
		{name: "key-accidental", body: `<attributes><key><key-step>C</key-step><key-alter>1</key-alter><key-accidental smufl="accidentalSharp">sharp</key-accidental></key></attributes>`},
		{name: "membrane", body: percussion(`<membrane smufl="pictAgogo">bass drum</membrane>`)},
		{name: "metal", body: percussion(`<metal smufl="pictAgogo">anvil</metal>`)},
		{name: "notehead", body: note(`<notehead smufl="noteLaWhole">other</notehead>`)},
		{name: "other-direction", body: direction(`<other-direction smufl="restQuarter">custom</other-direction>`)},
		{name: "other-notation", body: note(`<notations><other-notation type="single" smufl="restQuarter">custom</other-notation></notations>`)},
		{name: "other-placement-text", body: technical(`<other-technical smufl="restQuarter">custom</other-technical>`)},
		{name: "other-text", body: direction(`<dynamics><other-dynamics smufl="restQuarter">custom</other-dynamics></dynamics>`)},
		{name: "pitched", body: percussion(`<pitched smufl="pictAgogo">vibraphone</pitched>`)},
		{name: "segno", body: direction(`<segno smufl="segno"/>`)},
		{name: "timpani", body: percussion(`<timpani smufl="pictAgogo"/>`)},
		{name: "tremolo", body: ornament(`<tremolo smufl="buzzRoll">3</tremolo>`)},
		{name: "wavy-line", body: ornament(`<wavy-line type="start" smufl="wiggleArpeggiatoUp"/>`)},
		{name: "wood", body: percussion(`<wood smufl="pictAgogo">wood block</wood>`)},
	}
	total := 0
	for _, f := range fixtures {
		want := f.count
		if want == 0 {
			want = 1
		}
		total += want
		for _, model := range []bool{false, true} {
			t.Run(f.name+strconv.FormatBool(model), func(t *testing.T) {
				text := contextFixtureXML(f.body, false)
				text = strings.Replace(text, "<part-list>", f.header+"<part-list>", 1)
				text = strings.Replace(text, "</score-part>", f.part+"</score-part>", 1)
				report, err := dictionaryInspectTest(t, text, model, true)
				if err != nil {
					t.Fatal(err)
				}
				rows := dictionaryResults(report, validation.ContextDictionaryCanonical)
				if len(rows) != want {
					t.Fatalf("want %d exact uses, got %d; %s", want, len(rows), localFindings(report))
				}
				for _, r := range rows {
					if r.Result.State != validation.EvidenceKnown || !r.Result.Value {
						t.Fatalf("canonical relation: %+v", r)
					}
				}
			})
		}
	}
	if total != 27 {
		t.Fatalf("audited declared-use total=%d", total)
	}
}

func TestDictionaryInactiveRenderingAttributesStillCanonical(t *testing.T) {
	text := contextFixtureXML(`<note><rest/><duration>1</duration><notations><ornaments><tremolo type="single" smufl="notCanonical">3</tremolo></ornaments></notations></note>`, false)
	report, err := dictionaryInspectTest(t, text, false, true)
	if err != nil {
		t.Fatal(err)
	}
	canonical := dictionaryResults(report, validation.ContextDictionaryCanonical)
	if len(canonical) != 1 || canonical[0].Result.State != validation.EvidenceKnown || canonical[0].Result.Value {
		t.Fatalf("inactive present attribute skipped: %+v", canonical)
	}
	if report.Conformance != validation.ConformanceViolated {
		t.Fatal("inactive canonical violation lost")
	}
	rendering := dictionaryResults(report, validation.ContextDictionaryRendering)
	if len(rendering) != 1 || rendering[0].Applicability.State != validation.EvidenceKnown || rendering[0].Applicability.Value || rendering[0].Effective.State != validation.EvidenceAbsent {
		t.Fatalf("inactive rendering: %+v", rendering)
	}
	for _, r := range dictionaryResults(report, validation.ContextDictionaryAdvice) {
		if r.Role == "tremolo-range" {
			t.Fatal("inactive range advice activated")
		}
	}
}
