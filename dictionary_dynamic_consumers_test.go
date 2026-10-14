package musicxml

import (
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func TestDictionaryAllTenAssessedSMuFLTypes(t *testing.T) {
	for typ, value := range map[string]string{"smufl-glyph-name": "restQuarter", "smufl-accidental-glyph-name": "accidentalSharp", "smufl-coda-glyph-name": "coda", "smufl-lyrics-glyph-name": "lyricsElision", "smufl-pictogram-glyph-name": "pictAgogo", "smufl-segno-glyph-name": "segno", "smufl-wavy-line-glyph-name": "wiggleArpeggiatoUp", "glyph": "restQuarter", "formatted-symbol": "restQuarter", "formatted-symbol-id": "restQuarter"} {
		t.Run(typ, func(t *testing.T) {
			text := strings.Replace(contextFixtureXML("", false), "<part-list>", `<movement-title xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="`+typ+`">`+value+`</movement-title><part-list>`, 1)
			report, err := dictionaryInspectTest(t, text, false, true)
			if err != nil {
				t.Fatal(err)
			}
			rows := dictionaryResults(report, validation.ContextDictionaryCanonical)
			if len(rows) != 1 || rows[0].Result.State != validation.EvidenceKnown || !rows[0].Result.Value {
				t.Fatalf("actual domain lost: %+v; %s", rows, localFindings(report))
			}
		})
	}
}

func TestDictionaryAllThirtyFiveDynamicStringCarriers(t *testing.T) {
	// These are all 35 audited exact xs:string declarations, including both
	// anonymous score roots. Cases assert runtime dispatch, not just name counts.
	type fixture struct {
		name, header, part, body string
		timewise                 bool
	}
	note := func(s string) string { return `<note><rest/><duration>1</duration>` + s + `</note>` }
	tech := func(s string) string { return note(`<notations><technical>` + s + `</technical></notations>`) }
	fixtures := []fixture{
		{name: "beat-repeat/except-voice", body: `<attributes><measure-style><beat-repeat type="start"><except-voice>VALUE</except-voice></beat-repeat></measure-style></attributes>`},
		{name: "credit/credit-type", header: `<credit><credit-type>VALUE</credit-type><credit-words>x</credit-words></credit>`},
		{name: "direction/voice", body: `<direction><direction-type><words>x</words></direction-type><voice>VALUE</voice></direction>`},
		{name: "encoding/software", header: `<identification><encoding><software>VALUE</software></encoding></identification>`},
		{name: "encoding/encoding-description", header: `<identification><encoding><encoding-description>VALUE</encoding-description></encoding></identification>`},
		{name: "forward/voice", body: `<forward><duration>1</duration><voice>VALUE</voice></forward>`},
		{name: "hole/hole-type", body: tech(`<hole><hole-type>VALUE</hole-type><hole-closed>yes</hole-closed></hole>`)},
		{name: "hole/hole-shape", body: tech(`<hole><hole-closed>yes</hole-closed><hole-shape>VALUE</hole-shape></hole>`)},
		{name: "identification/source", header: `<identification><source>VALUE</source></identification>`},
		{name: "instrument-change/instrument-sound", body: `<sound><instrument-change id="I1"><instrument-sound>VALUE</instrument-sound></instrument-change></sound>`},
		{name: "interchangeable/beats", body: `<attributes><time><beats>4</beats><beat-type>4</beat-type><interchangeable><beats>VALUE</beats><beat-type>4</beat-type></interchangeable></time></attributes>`},
		{name: "interchangeable/beat-type", body: `<attributes><time><beats>4</beats><beat-type>4</beat-type><interchangeable><beats>4</beats><beat-type>VALUE</beat-type></interchangeable></time></attributes>`},
		{name: "metronome/metronome-relation", body: `<direction><direction-type><metronome><metronome-note><metronome-type>quarter</metronome-type></metronome-note><metronome-relation>VALUE</metronome-relation><metronome-note><metronome-type>half</metronome-type></metronome-note></metronome></direction-type></direction>`},
		{name: "midi-instrument/midi-name", part: `<midi-instrument id="I1"><midi-name>VALUE</midi-name></midi-instrument>`},
		{name: "note/voice", body: note(`<voice>VALUE</voice>`)},
		{name: "part-link/group-link", part: `<part-link xmlns:xlink="http://www.w3.org/1999/xlink" xlink:href="part.xml"><group-link>VALUE</group-link></part-link>`},
		{name: "play/ipa", body: note(`<play><ipa>VALUE</ipa></play>`)},
		{name: "player/player-name", part: `<player id="player1"><player-name>VALUE</player-name></player>`},
		{name: "score-instrument/instrument-name", part: `<score-instrument id="I2"><instrument-name>VALUE</instrument-name></score-instrument>`},
		{name: "score-instrument/instrument-abbreviation", part: `<score-instrument id="I2"><instrument-name>piano</instrument-name><instrument-abbreviation>VALUE</instrument-abbreviation></score-instrument>`},
		{name: "score-instrument/instrument-sound", part: `<score-instrument id="I2"><instrument-name>piano</instrument-name><instrument-sound>VALUE</instrument-sound></score-instrument>`},
		{name: "score-part/group", part: `<group>VALUE</group>`},
		{name: "slash/except-voice", body: `<attributes><measure-style><slash type="start"><except-voice>VALUE</except-voice></slash></measure-style></attributes>`},
		{name: "swing/swing-style", body: `<sound><swing><first>1</first><second>1</second><swing-style>VALUE</swing-style></swing></sound>`},
		{name: "time/beats", body: `<attributes><time><beats>VALUE</beats><beat-type>4</beat-type></time></attributes>`},
		{name: "time/beat-type", body: `<attributes><time><beats>4</beats><beat-type>VALUE</beat-type></time></attributes>`},
		{name: "time/senza-misura", body: `<attributes><time><senza-misura>VALUE</senza-misura></time></attributes>`},
		{name: "virtual-instrument/virtual-library", part: `<score-instrument id="I2"><instrument-name>piano</instrument-name><virtual-instrument><virtual-library>VALUE</virtual-library></virtual-instrument></score-instrument>`},
		{name: "virtual-instrument/virtual-name", part: `<score-instrument id="I2"><instrument-name>piano</instrument-name><virtual-instrument><virtual-name>VALUE</virtual-name></virtual-instrument></score-instrument>`},
		{name: "work/work-number", header: `<work><work-number>VALUE</work-number></work>`},
		{name: "work/work-title", header: `<work><work-title>VALUE</work-title></work>`},
		{name: "partwise/movement-number", header: `<movement-number>VALUE</movement-number>`},
		{name: "partwise/movement-title", header: `<movement-title>VALUE</movement-title>`},
		{name: "timewise/movement-number", header: `<movement-number>VALUE</movement-number>`, timewise: true},
		{name: "timewise/movement-title", header: `<movement-title>VALUE</movement-title>`, timewise: true},
	}
	if len(fixtures) != 35 {
		t.Fatal("audited carrier count changed")
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			text := contextFixtureXML(f.body, f.timewise)
			text = strings.Replace(text, "<part-list>", f.header+"<part-list>", 1)
			if f.name == "part-link/group-link" {
				text = strings.Replace(text, "<part-name>", f.part+"<part-name>", 1)
			} else {
				text = strings.Replace(text, "</score-part>", f.part+"</score-part>", 1)
			}
			at := strings.Index(text, ">VALUE</")
			if at < 0 {
				t.Fatal("missing scalar placeholder")
			}
			text = text[:at] + ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="smufl-glyph-name">restQuarter</` + text[at+len(">VALUE</"):]
			report, err := dictionaryInspectTest(t, text, false, true)
			if err != nil {
				t.Fatal(err)
			}
			rows := dictionaryResults(report, validation.ContextDictionaryCanonical)
			if len(rows) != 1 || rows[0].Result.State != validation.EvidenceKnown || !rows[0].Result.Value {
				t.Fatalf("carrier lost: %+v; %s", rows, localFindings(report))
			}
		})
	}
}
func TestDictionaryDynamicAttributeOwnerAndDiscardedNomination(t *testing.T) {
	text := strings.Replace(contextFixtureXML("", false), "<part-list>", `<movement-title xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="accidental" smufl="accidentalSharp">sharp</movement-title><part-list>`, 1)
	source, err := dictionaryInspectTest(t, text, false, true)
	if err != nil {
		t.Fatal(err)
	}
	rows := dictionaryResults(source, validation.ContextDictionaryCanonical)
	if len(rows) != 1 || rows[0].Attribute == nil || !rows[0].Result.Value {
		t.Fatalf("dynamic attribute use lost: %+v", rows)
	}
	model, err := dictionaryInspectTest(t, text, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := dictionaryResults(model, validation.ContextDictionaryCanonical); len(got) != 0 {
		t.Fatalf("model invented discarded xsi:type/attribute: %+v", got)
	}
}

func TestDictionarySixFamilyFailuresAndBadNominationStayCausal(t *testing.T) {
	for _, family := range []string{"accidental", "coda", "lyrics", "pictogram", "segno", "wavy-line"} {
		for _, available := range []bool{false, true} {
			text := strings.Replace(contextFixtureXML("", false), "<part-list>", `<movement-title xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="smufl-`+family+`-glyph-name">restQuarter</movement-title><part-list>`, 1)
			report, err := dictionaryInspectTest(t, text, false, available)
			if err != nil {
				t.Fatal(err)
			}
			rows := dictionaryResults(report, validation.ContextDictionaryCanonical)
			if len(rows) != 1 || rows[0].Result.State != validation.EvidenceInvalid || rows[0].Result.Cause == nil {
				t.Fatalf("family=%s available=%v: %+v", family, available, rows)
			}
			for _, finding := range report.Findings {
				if finding.Reason == "smufl-canonical-name-not-listed" {
					t.Fatal("dependent lookup duplicated independent family defect")
				}
			}
		}
	}
	for _, nomination := range []string{`xsi:type="unknownType"`, `xsi:type="unbound:type"`, `xmlns:xs="http://www.w3.org/2001/XMLSchema" xsi:type="xs:string"`} {
		text := contextFixtureXML(`<direction><direction-type><symbol xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" `+nomination+`>restQuarter</symbol></direction-type></direction>`, false)
		report, err := dictionaryInspectTest(t, text, false, true)
		if err != nil {
			t.Fatal(err)
		}
		rows := dictionaryResults(report, validation.ContextDictionaryCanonical)
		if len(rows) != 1 || rows[0].Result.State != validation.EvidenceInvalid || rows[0].Result.Cause == nil {
			t.Fatalf("nomination %s: %+v", nomination, rows)
		}
	}
}
