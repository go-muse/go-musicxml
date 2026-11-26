package musicxml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// TestWindowSpansDegradationOnCorpus measures how often the windows' spans
// work is not adopted on the official files and the MusicXML test suite.
func TestWindowSpansDegradationOnCorpus(t *testing.T) {
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	var files []string
	for _, dir := range []string{"testdata/official", "testdata/corpus"} {
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".musicxml")) {
				files = append(files, path)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34}
	defer validationSetWindowedStrategy(validationSetWindowedStrategy(true))
	var current string
	seen, adopted, degraded := map[string]bool{}, map[string]bool{}, map[string]bool{}
	failed := map[string]map[string]bool{}
	endpoints := 0
	defer validationSetWindowSpansObserver(validationSetWindowSpansObserver(func(reason string, count int) {
		seen[current] = true
		if reason == "" {
			adopted[current] = true
			endpoints += count
			return
		}
		degraded[current] = true
		if failed[reason] == nil {
			failed[reason] = map[string]bool{}
		}
		failed[reason][filepath.Base(current)] = true
	}))
	read := 0
	for i := 0; i < len(files); i += windowedStride() {
		current = files[i]
		data, err := os.ReadFile(current)
		if err != nil {
			t.Fatal(err)
		}
		read++
		_, _ = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big, Context: validation.ContextFamilyAll})
	}
	if len(seen) == 0 || endpoints == 0 {
		t.Fatalf("no spans work observed: %d documents, %d endpoints", len(seen), endpoints)
	}
	// A document whose windows never start the work is not adopted either:
	// the bound is on every document read, not on those that reported. The
	// bound of the design is 5 % of the corpus; a sample of it gets a
	// margin of one document in forty.
	bound := 5
	if windowedStride() != 1 {
		bound = 10
	}
	if 100*(read-len(adopted)) > bound*read {
		t.Fatalf("%d of %d documents not adopted; reported: %v", read-len(adopted), read, failed)
	}
	t.Logf("%d documents: %d adopted with %d endpoints, %d reported another end: %v", read, len(adopted), endpoints, len(degraded), failed)
}

func spansWindowNote(pitch, body string) string {
	return spansWindowTiedNote(pitch, "", body)
}

// spansWindowTiedNote is a note with tie elements, which precede its voice.
func spansWindowTiedNote(pitch, ties, body string) string {
	return `<note><pitch><step>` + pitch + `</step><octave>4</octave></pitch><duration>2</duration>` + ties + `<voice>1</voice><type>quarter</type>` + body + `</note>`
}

func spansWindowNotations(body string) string {
	return `<notations>` + body + `</notations>`
}

func spansWindowDirection(body string) string {
	return `<direction><direction-type>` + body + `</direction-type></direction>`
}

// spansWindowTwoParts is a score of two parts in one group.
func spansWindowTwoParts(version, first, second string) []byte {
	return []byte(`<score-partwise` + version + `><part-list><part-group type="start" number="1"><group-symbol>brace</group-symbol></part-group><score-part id="P1"><part-name>A</part-name></score-part><score-part id="P2"><part-name>B</part-name></score-part><part-group type="stop" number="1"/></part-list><part id="P1">` + first + `</part><part id="P2">` + second + `</part></score-partwise>`)
}

// TestWindowSpansRiskCases compares the two strategies on documents where a
// later fact decides what an earlier window read, and on documents whose
// windows cannot be adopted.
func TestWindowSpansRiskCases(t *testing.T) {
	skipInShortTier(t)
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	cases, expected := spansWindowRiskCases()
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			previous := validationSetWindowedStrategy(false)
			buffered := windowedReports(t, name, data)
			validationSetWindowedStrategy(true)
			adopted, endpoints := 0, 0
			var reasons []string
			previousObserver := validationSetWindowSpansObserver(func(reason string, count int) {
				if reason == "" {
					adopted++
					endpoints += count
				} else if !slices.Contains(reasons, reason) {
					reasons = append(reasons, reason)
				}
			})
			windowed := windowedReports(t, name, data)
			validationSetWindowSpansObserver(previousObserver)
			validationSetWindowedStrategy(previous)
			for label, want := range buffered {
				if windowed[label] != want {
					t.Errorf("%s: windowed report differs from buffered", label)
				}
			}
			want := expected[name]
			switch {
			case want != "" && !slices.Contains(reasons, want):
				t.Errorf("windows ended with %v after %d adoptions, want %q", reasons, adopted, want)
			case want == "" && (adopted == 0 || endpoints == 0 || len(reasons) != 0):
				t.Errorf("windows adopted %d times with %d endpoints; other ends: %v", adopted, endpoints, reasons)
			}
			t.Logf("adopted=%d endpoints=%d reasons=%v", adopted, endpoints, reasons)
		})
	}
}

// spansWindowRiskCases are documents where a later fact decides what an
// earlier window read, or whose windows cannot be adopted, with the end of
// the windows' work each of the latter must report.
func spansWindowRiskCases() (map[string][]byte, map[string]string) {
	m := performanceWindowMeasure
	d := performanceWindowDivisions
	plain := spansWindowNote("C", "")
	slur := func(kind, number string) string {
		return spansWindowNotations(`<slur type="` + kind + `" number="` + number + `"/>`)
	}
	tied := func(kinds ...string) string {
		ties, marks := "", ""
		for _, kind := range kinds {
			ties += `<tie type="` + kind + `"/>`
			marks += `<tied type="` + kind + `"/>`
		}
		return spansWindowTiedNote("C", ties, spansWindowNotations(marks))
	}
	lyric := func(extend string) string {
		return `<lyric number="1"><syllabic>single</syllabic><text>la</text>` + extend + `</lyric>`
	}
	var long []string
	long = append(long, m("1", d+spansWindowNote("C", slur("start", "1"))+plain))
	for i := 2; i < 50; i++ {
		long = append(long, m(fmt.Sprint(i), plain+plain))
	}
	long = append(long, m("50", plain+spansWindowNote("D", slur("stop", "1"))))
	everything := d +
		spansWindowNote("C", `<beam number="1">begin</beam>`+spansWindowNotations(`<tuplet type="start" number="1"/><arpeggiate number="1"/><glissando type="start" number="1"/><slide type="start" number="1"/><technical><hammer-on type="start" number="1">H</hammer-on><pull-off type="start" number="1">P</pull-off></technical><ornaments><tremolo type="start">2</tremolo><wavy-line type="start" number="1"/></ornaments>`)) +
		`<note><chord/><pitch><step>E</step><octave>4</octave></pitch><duration>2</duration><voice>1</voice><type>quarter</type><notations><arpeggiate number="1"/></notations></note>` +
		spansWindowNote("D", `<time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification><beam number="1" fan="accel">continue</beam>`) +
		spansWindowNote("E", `<beam number="1">end</beam>`+spansWindowNotations(`<tuplet type="stop" number="1"/><glissando type="stop" number="1"/><slide type="stop" number="1"/><non-arpeggiate type="bottom" number="2"/><technical><hammer-on type="stop" number="1"/><pull-off type="stop" number="1"/></technical><ornaments><tremolo type="stop">2</tremolo><wavy-line type="stop" number="1"/></ornaments><other-notation type="single" number="1">x</other-notation>`)) +
		spansWindowDirection(`<wedge type="crescendo" number="1"/>`) + spansWindowDirection(`<pedal type="start" line="yes"/>`) + spansWindowDirection(`<octave-shift type="down" size="8" number="1"/>`) + spansWindowDirection(`<bracket type="start" number="1" line-end="up"/>`) + spansWindowDirection(`<dashes type="start" number="1"/>`) + spansWindowDirection(`<principal-voice type="start" symbol="Hauptstimme"/>`) +
		spansWindowDirection(`<metronome><metronome-note><metronome-type>quarter</metronome-type><metronome-tied type="start"/></metronome-note><metronome-relation>equals</metronome-relation><metronome-note><metronome-type>quarter</metronome-type><metronome-tied type="stop"/></metronome-note></metronome>`) +
		`<harmony><root><root-step>C</root-step></root><kind>major</kind><frame><frame-strings>6</frame-strings><frame-frets>4</frame-frets><frame-note><string>6</string><fret>1</fret><barre type="start"/></frame-note><frame-note><string>1</string><fret>1</fret><barre type="stop"/></frame-note></frame></harmony>` +
		`<grouping type="start" number="1"/>` + plain + `<grouping type="stop" number="1"/>` +
		`<barline location="right"><wavy-line type="continue" number="1"/></barline>`
	closing := spansWindowDirection(`<wedge type="stop" number="1"/>`) + spansWindowDirection(`<pedal type="stop" line="yes"/>`) + spansWindowDirection(`<octave-shift type="stop" size="8" number="1"/>`) + spansWindowDirection(`<bracket type="stop" number="1" line-end="down"/>`) + spansWindowDirection(`<dashes type="stop" number="1"/>`) + spansWindowDirection(`<principal-voice type="stop" symbol="none"/>`) + plain
	cases := map[string][]byte{
		"slur across fifty measures": performanceWindowDocument(long...),
		"start without stop and stop without start": performanceWindowDocument(
			m("1", d+spansWindowNote("C", slur("start", "1"))+spansWindowNote("D", slur("stop", "2"))),
			m("2", plain),
		),
		"same number nested and crossing": performanceWindowDocument(
			m("1", d+spansWindowNote("C", slur("start", "1"))+spansWindowNote("D", slur("start", "1"))),
			m("2", spansWindowNote("E", slur("stop", "1"))+spansWindowNote("F", slur("start", "2"))),
			m("3", spansWindowNote("G", slur("stop", "1"))+spansWindowNote("A", slur("stop", "2"))),
		),
		"ties of one pitch in two voices": performanceWindowDocument(
			m("1", d+tied("start")+`<backup><duration>2</duration></backup>`+strings.Replace(tied("start"), `<voice>1</voice>`, `<voice>2</voice>`, 1)),
			m("2", tied("stop")+`<backup><duration>2</duration></backup>`+strings.Replace(tied("stop"), `<voice>1</voice>`, `<voice>2</voice>`, 1)),
		),
		"tied start and stop on one note": performanceWindowDocument(
			m("1", d+spansWindowNote("C", spansWindowNotations(`<tied type="start"/><tied type="stop"/>`))),
			m("2", plain),
		),
		"late same-note tie makes an earlier pair ambiguous": performanceWindowDocument(
			m("1", d+tied("start")),
			m("2", tied("stop")),
			m("3", spansWindowNote("C", spansWindowNotations(`<tied type="start"/><tied type="stop"/>`))),
		),
		"tie chain with attack and release": performanceWindowDocument(
			m("1", d+strings.Replace(tied("start"), `<note>`, `<note release="1">`, 1)),
			m("2", strings.Replace(tied("stop", "start"), `<note>`, `<note attack="1">`, 1)),
			m("3", tied("stop")),
		),
		"lyric extends across measures": performanceWindowDocument(
			m("1", d+spansWindowNote("C", lyric(`<extend type="start"/>`))),
			m("2", spansWindowNote("C", lyric(`<extend type="continue"/>`))),
			m("3", spansWindowNote("C", lyric(`<extend type="stop"/>`))),
		),
		"extend without a type in version 4.0": performanceWindowDocument(
			m("1", d+spansWindowNote("C", lyric(`<extend/>`))),
			m("2", spansWindowNote("C", lyric(``)+`<lyric number="1"><syllabic>single</syllabic><text>b</text><extend/></lyric>`)),
		),
		"extend without a type in version 2.0": []byte(strings.Replace(string(performanceWindowDocument(
			m("1", d+spansWindowNote("C", lyric(`<extend/>`))+`<figured-bass><figure><figure-number>6</figure-number><extend/></figure></figured-bass>`+plain),
			m("2", spansWindowNote("C", lyric(``))),
		)), `version="4.0"`, `version="2.0"`, 1)),
		"extend without a type and no version": []byte(strings.Replace(string(performanceWindowDocument(
			m("1", d+spansWindowNote("C", lyric(`<extend/>`))),
		)), ` version="4.0"`, ``, 1)),
		"extend without a type and an invalid version": []byte(strings.Replace(string(performanceWindowDocument(
			m("1", d+spansWindowNote("C", lyric(`<extend/>`))),
		)), ` version="4.0"`, ` version=""`, 1)),
		"unnamed lyrics twice on one note": performanceWindowDocument(
			m("1", d+spansWindowNote("C", `<lyric><text>a</text><extend type="start"/></lyric><lyric><text>b</text></lyric>`)),
			m("2", spansWindowNote("C", `<lyric><text>c</text><extend type="stop"/></lyric>`)),
		),
		"numbered lyric twice on one note": performanceWindowDocument(
			m("1", d+spansWindowNote("C", lyric(`<extend type="start"/>`)+lyric(``))),
			m("2", spansWindowNote("C", lyric(`<extend type="stop"/>`))),
		),
		"numbered lyric twice on one note, extend without a type": performanceWindowDocument(
			m("1", d+spansWindowNote("C", lyric(`<extend/>`)+lyric(``))),
			m("2", spansWindowNote("C", lyric(`<extend/>`))),
		),
		"numbered lyric twice on one note, extends without a type in version 2.0": []byte(strings.Replace(string(performanceWindowDocument(
			m("1", d+spansWindowNote("C", lyric(`<extend/>`)+lyric(`<extend/>`))),
			m("2", spansWindowNote("C", lyric(`<extend/>`)+lyric(`<extend/>`)+lyric(`<extend type="stop"/>`))),
		)), `version="4.0"`, `version="2.0"`, 1)),
		"tied on unpitched notes and rests": performanceWindowDocument(
			m("1", d+tied("start")+`<note><unpitched><display-step>C</display-step><display-octave>4</display-octave></unpitched><duration>2</duration><voice>1</voice><type>quarter</type><notations><tied type="start"/></notations></note>`),
			m("2", tied("stop")+tied("start")),
			m("3", tied("stop")+`<note><rest/><duration>2</duration><voice>1</voice><type>quarter</type><notations><tied type="stop"/></notations></note>`),
		),
		"tied pair whose stop is earlier in time": performanceWindowDocument(
			m("1", d+tied("start")+`<backup><duration>2</duration></backup>`+plain),
			m("2", plain+`<backup><duration>2</duration></backup>`+tied("stop")),
			m("3", tied("start")+`<backup><duration>2</duration></backup>`+tied("stop")+tied("start")),
			m("4", tied("stop")),
		),
		"numbered tied chains and let-ring": performanceWindowDocument(
			m("1", d+spansWindowNote("C", spansWindowNotations(`<tied type="start" number="1"/><tied type="start" number="2"/><tied type="let-ring"/>`))),
			m("2", spansWindowNote("C", spansWindowNotations(`<tied type="continue" number="1"/><tied type="stop" number="2"/>`))),
			m("3", spansWindowNote("D", spansWindowNotations(`<tied type="stop" number="1"/><tied type="stop" number="2"/><tied type="stop" number="2"/>`))),
		),
		"tremolo on notes of different lengths": performanceWindowDocument(
			m("1", d+strings.Replace(spansWindowNote("C", spansWindowNotations(`<ornaments><tremolo type="start">2</tremolo></ornaments>`)), `<type>quarter</type>`, `<type>half</type>`, 1)+
				spansWindowNote("D", `<time-modification><actual-notes>2</actual-notes><normal-notes>1</normal-notes></time-modification>`+spansWindowNotations(`<ornaments><tremolo type="stop">2</tremolo></ornaments>`))),
		),
		"every span family": performanceWindowDocument(m("1", everything), m("2", closing)),
		"part group over two parts": spansWindowTwoParts(` version="4.0"`,
			m("1", d+spansWindowNote("C", slur("start", "1")))+m("2", spansWindowNote("D", slur("stop", "1"))),
			m("1", d+spansWindowNote("C", slur("start", "1")))+m("2", plain)),
		"stray element after the last part": []byte(strings.Replace(string(spansWindowTwoParts(` version="4.0"`,
			m("1", d+spansWindowNote("C", slur("start", "1")))+m("2", spansWindowNote("D", slur("stop", "1"))),
			m("1", d+spansWindowNote("C", slur("start", "1")))+m("2", spansWindowNote("D", slur("stop", "1"))))), `</score-partwise>`, `<stray/></score-partwise>`, 1)),
		"stray element in the part-list": []byte(strings.Replace(string(spansWindowTwoParts(` version="4.0"`,
			m("1", d+spansWindowNote("C", slur("start", "1")))+m("2", spansWindowNote("D", slur("stop", "1"))),
			m("1", d+plain))), `<part-group type="stop" number="1"/>`, `<stray/><part-group type="stop" number="1"/>`, 1)),
		"stray element between measures": []byte(strings.Replace(string(performanceWindowDocument(
			m("1", d+spansWindowNote("C", slur("start", "1"))),
			m("2", spansWindowNote("D", slur("stop", "1"))),
		)), `<measure number="2">`, `<stray/><measure number="2">`, 1)),
		"content error in a note": performanceWindowDocument(
			m("1", d+spansWindowNote("C", slur("start", "1"))),
			m("2", spansWindowNote("D", slur("stop", "1")+`<bogus/>`)),
		),
		"invalid value in the last measure": performanceWindowDocument(
			m("1", d+spansWindowNote("C", slur("start", "1"))),
			m("2", spansWindowNote("D", spansWindowNotations(`<slur type="stop" number="1" bezier-x="wide"/>`))),
		),
		"invalid value in an earlier measure": performanceWindowDocument(
			m("1", d+spansWindowNote("C", spansWindowNotations(`<slur type="start" number="1" bezier-x="wide"/>`))),
			m("2", spansWindowNote("D", slur("stop", "1"))),
			m("3", plain),
		),
		"grouping in a measure without events": performanceWindowDocument(
			m("1", `<grouping type="start" number="1"/>`),
			m("2", d+plain+`<grouping type="stop" number="1"/>`),
		),
		"grouping beside a note": performanceWindowDocument(
			m("1", d+`<grouping type="start" number="1"/>`+plain),
			m("2", plain+`<grouping type="stop" number="1"/>`),
		),
		"part naming no score-part": []byte(strings.Replace(string(performanceWindowDocument(
			m("1", d+spansWindowNote("C", slur("start", "1"))),
			m("2", spansWindowNote("D", slur("stop", "1"))),
		)), `</part>`, `</part><part id="P9">`+m("1", d+spansWindowNote("C", slur("start", "1")))+`</part>`, 1)),
		"divisions change behind a backup": performanceWindowDocument(
			m("1", d+spansWindowNote("C", slur("start", "1"))+plain),
			m("2", `<attributes><divisions>4</divisions></attributes><backup><duration>8</duration></backup>`+spansWindowNote("D", slur("stop", "1"))),
		),
		"timewise": []byte(`<score-timewise version="4.0"><part-list><score-part id="P1"><part-name>A</part-name></score-part></part-list><measure number="1"><part id="P1">` + d + spansWindowNote("C", slur("start", "1")) + `</part></measure><measure number="2"><part id="P1">` + spansWindowNote("D", slur("stop", "1")) + `</part></measure></score-timewise>`),
	}
	valid := performanceWindowDocument(m("1", d+spansWindowNote("C", slur("start", "1"))), m("2", spansWindowNote("D", slur("stop", "1"))))
	cases["cut inside an open span"] = valid[:bytes.Index(valid, []byte(`<measure number="2">`))+20]
	expected := map[string]string{
		"content error in a note":             "unbound",
		"invalid value in the last measure":   "unbound",
		"cut inside an open span":             "unsettled",
		"stray element after the last part":   "speculation",
		"stray element in the part-list":      "speculation",
		"invalid value in an earlier measure": "unbound",
		"stray element between measures":      "unbound",
		"part naming no score-part":           "speculation",
		"divisions change behind a backup":    "excursions",
		"timewise":                            "speculation",
	}
	return cases, expected
}

// spansWindowHeavy is a part of measures full of span endpoints of every
// kind a window keeps something for.
func spansWindowHeavy(measures int) []byte {
	out := make([]string, measures)
	for i := range out {
		body := ""
		if i == 0 {
			body = performanceWindowDivisions
		}
		for j := 0; j < 4; j++ {
			kind, beam := "start", "begin"
			if j%2 == 1 {
				kind, beam = "stop", "end"
			}
			body += spansWindowTiedNote("C", `<tie type="`+kind+`"/>`, `<beam number="1">`+beam+`</beam>`+spansWindowNotations(`<tied type="`+kind+`"/><slur type="`+kind+`" number="1"/><ornaments><tremolo type="`+kind+`">2</tremolo></ornaments>`)+`<lyric number="1"><text>la</text><extend/></lyric>`)
		}
		out[i] = performanceWindowMeasure(fmt.Sprint(i+1), body+spansWindowDirection(`<wedge type="crescendo" number="1"/>`)+spansWindowDirection(`<wedge type="stop" number="1"/>`))
	}
	return performanceWindowDocument(out...)
}

// What windows keep for the spans provider is charged before Finish: with the
// windows' work on, the heap at the start of Finish grows by no more than the
// charge does, with the projection selected and without it.
func TestWindowSpansChargeCoversTheHeap(t *testing.T) {
	skipInShortTier(t)
	if raceEnabled {
		t.Skip("the race detector changes the heap")
	}
	data := spansWindowHeavy(60)
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	for _, families := range []validation.ContextFamilies{0, validation.ContextFamilySpans, validation.ContextFamilyAll} {
		measure := func(off bool) (heap, used int64, endpoints int, report string, account *validation.Account) {
			defer validationSetWindowedStrategy(validationSetWindowedStrategy(true))
			defer validationSetWindowSpansOff(validationSetWindowSpansOff(off))
			run := func() validation.Report {
				r, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big, Context: families})
				if err != nil {
					t.Fatal(err)
				}
				return r
			}
			run()
			defer validationSetWindowSpansObserver(validationSetWindowSpansObserver(func(reason string, count int) {
				if reason == "" {
					endpoints += count
				}
			}))
			before := liveHeap()
			defer validationSetFinishObserver(validationSetFinishObserver(func(stage string, a *validation.Account) {
				account = a
				if stage == "evaluating" {
					heap = liveHeap() - before
					used, _ = a.Usage(validation.BudgetBufferedBytes)
				}
			}))
			encoded, err := json.Marshal(run())
			if err != nil {
				t.Fatal(err)
			}
			return heap, used, endpoints, string(encoded), account
		}
		offHeap, offUsed, _, offReport, _ := measure(true)
		onHeap, onUsed, endpoints, onReport, account := measure(false)
		if onReport != offReport {
			t.Fatalf("context %b: the report differs from the one without the windows' work", families)
		}
		if endpoints < 1500 {
			t.Fatalf("context %b: %d endpoints adopted", families, endpoints)
		}
		if onHeap-offHeap > onUsed-offUsed+finishHeapNoise {
			t.Fatalf("context %b: %d endpoints hold %d live bytes and charge %d", families, endpoints, onHeap-offHeap, onUsed-offUsed)
		}
		if used, _ := account.Usage(validation.BudgetBufferedBytes); used != 0 {
			t.Fatalf("context %b: %d bytes charged after the run", families, used)
		}
		t.Logf("context %b: %d endpoints hold %d live bytes and charge %d", families, endpoints, onHeap-offHeap, onUsed-offUsed)
	}
}

// Dropping the windows' work releases all it charged: a run whose windows
// are dropped in Finish ends its phases with the charge of a run that never
// read windows for spans.
func TestWindowSpansDropReleasesItsCharge(t *testing.T) {
	skipInShortTier(t)
	// Two unnamed lyrics on one note leave a part-wide gap the windows keep
	// to the end.
	data := bytes.Replace(spansWindowHeavy(40), []byte(`<lyric number="1"><text>la</text><extend/></lyric>`), []byte(`<lyric><text>la</text><extend/></lyric><lyric><text>lo</text></lyric>`), 2)
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	for _, families := range []validation.ContextFamilies{0, validation.ContextFamilySpans, validation.ContextFamilyAll} {
		// Without the performance windows the spans windows are the first
		// to build the shared attribute tables, which a drop must leave.
		for _, performanceOff := range []bool{false, true} {
			run := func(off bool, tamper string) (used int64, reason string, report string) {
				defer validationSetWindowedStrategy(validationSetWindowedStrategy(true))
				defer validationSetWindowPerformanceOff(validationSetWindowPerformanceOff(performanceOff))
				defer validationSetWindowSpansOff(validationSetWindowSpansOff(off))
				defer validationSetWindowSpansTamper(validationSetWindowSpansTamper(tamper))
				defer validationSetWindowSpansObserver(validationSetWindowSpansObserver(func(r string, _ int) { reason = r }))
				defer validationSetFinishObserver(validationSetFinishObserver(func(stage string, a *validation.Account) {
					if stage == "finished" {
						used, _ = a.Usage(validation.BudgetBufferedBytes)
					}
				}))
				r, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big, Context: families})
				if err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				return used, reason, string(encoded)
			}
			offUsed, _, offReport := run(true, "")
			dropUsed, reason, dropReport := run(false, "covered")
			if reason != "unbound" {
				t.Fatalf("context %b: the windows' work ended with %q", families, reason)
			}
			if dropReport != offReport {
				t.Fatalf("context %b: the report after a drop differs", families)
			}
			// With a projection selected the windowed strategy's charge varies
			// by a few kilobytes between identical runs; the provisional state
			// dropped here is megabytes.
			slack := int64(0)
			if families != 0 {
				slack = finishHeapNoise
			}
			if dropUsed-offUsed > slack || offUsed-dropUsed > slack {
				t.Fatalf("context %b, performance windows off %v: %d bytes charged after a drop, %d without the windows' work", families, performanceOff, dropUsed, offUsed)
			}
		}
	}
}

// A count the windows' work keeps wrong is reported, not hidden: releasing
// more than was retained, or reading a closed window's node in Finish, ends
// the run with a contract error.
func TestWindowSpansReportsItsOwnDefects(t *testing.T) {
	data := spansWindowHeavy(4)
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	for _, tamper := range []string{"retained", "captured"} {
		func() {
			defer validationSetWindowedStrategy(validationSetWindowedStrategy(true))
			defer validationSetWindowSpansTamper(validationSetWindowSpansTamper(tamper))
			r, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big, Context: validation.ContextFamilyAll})
			if err == nil || r.AssessmentComplete || !strings.Contains(err.Error(), "window spans") {
				t.Fatalf("%s: complete=%v err=%v", tamper, r.AssessmentComplete, err)
			}
		}()
	}
}

// Provisional facts are charged once, when Finish adopts them: under every
// context-fact quota the two strategies complete or stop alike, and a
// completed run reports the same.
func TestWindowSpansUnderContextFactQuotas(t *testing.T) {
	skipInShortTier(t)
	data := spansWindowHeavy(2)
	type outcome struct {
		report   string
		complete bool
		terminal *validation.TerminalSummary
		err      error
		facts    int64
	}
	for _, families := range []validation.ContextFamilies{0, validation.ContextFamilyAll} {
		run := func(windowed bool, quota int64) outcome {
			defer validationSetWindowedStrategy(validationSetWindowedStrategy(windowed))
			var out outcome
			defer validationSetFinishObserver(validationSetFinishObserver(func(stage string, account *validation.Account) {
				if stage == "finished" {
					out.facts, _ = account.Usage(validation.BudgetContextFacts)
				}
			}))
			r, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34, MaxContextFacts: quota}, Context: families})
			encoded, marshalErr := json.Marshal(r)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			out.report, out.complete, out.terminal, out.err = string(encoded), r.AssessmentComplete, r.Terminal, err
			return out
		}
		full, windowed := run(false, 1<<30), run(true, 1<<30)
		if full.facts == 0 || full.facts != windowed.facts {
			t.Fatalf("context %b: %d context facts buffered, %d windowed", families, full.facts, windowed.facts)
		}
		checked := 0
		for quota := full.facts - 40; quota <= full.facts+1; quota++ {
			buffered, windowed := run(false, quota), run(true, quota)
			if buffered.complete != windowed.complete {
				t.Fatalf("context %b quota %d: completeness %v/%v", families, quota, buffered.complete, windowed.complete)
			}
			if buffered.complete {
				if buffered.report != windowed.report {
					t.Fatalf("context %b quota %d: reports differ", families, quota)
				}
				checked++
				continue
			}
			if buffered.terminal == nil || windowed.terminal == nil || *buffered.terminal != *windowed.terminal {
				t.Fatalf("context %b quota %d: terminals %+v/%+v", families, quota, buffered.terminal, windowed.terminal)
			}
			if (buffered.err == nil) != (windowed.err == nil) {
				t.Fatalf("context %b quota %d: errors %v/%v", families, quota, buffered.err, windowed.err)
			}
			checked++
		}
		if checked < 40 {
			t.Fatalf("context %b: checked %d quotas", families, checked)
		}
	}
}

// Between windows the spans provider keeps endpoints and nothing else: a
// long part of notes with lyrics and time modifications, which have facts
// but no endpoints, holds no more live memory at the start of Finish with
// the windows' work than without it.
func TestWindowSpansKeepNothingForNodesWithoutEndpoints(t *testing.T) {
	skipInShortTier(t)
	if raceEnabled {
		t.Skip("the race detector changes the heap")
	}
	measures := make([]string, 400)
	for i := range measures {
		body := ""
		if i == 0 {
			body = performanceWindowDivisions
		}
		for j := 0; j < 4; j++ {
			body += spansWindowNote("C", `<time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification><lyric number="1" name="a"><text>la</text></lyric><lyric number="2"><text>lo</text></lyric>`)
		}
		measures[i] = performanceWindowMeasure(fmt.Sprint(i+1), body)
	}
	data := performanceWindowDocument(measures...)
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	measure := func(off bool) (heap, used int64) {
		defer validationSetWindowedStrategy(validationSetWindowedStrategy(true))
		defer validationSetWindowSpansOff(validationSetWindowSpansOff(off))
		run := func() {
			if _, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big}); err != nil {
				t.Fatal(err)
			}
		}
		run()
		before := liveHeap()
		defer validationSetFinishObserver(validationSetFinishObserver(func(stage string, a *validation.Account) {
			if stage == "evaluating" {
				heap = liveHeap() - before
				used, _ = a.Usage(validation.BudgetBufferedBytes)
			}
		}))
		run()
		return heap, used
	}
	offHeap, offUsed := measure(true)
	onHeap, onUsed := measure(false)
	if onHeap-offHeap > finishHeapNoise || onUsed-offUsed > finishHeapNoise {
		t.Fatalf("3200 lyrics and 1600 time modifications: the windows' work holds %d live bytes and charges %d", onHeap-offHeap, onUsed-offUsed)
	}
	t.Logf("the windows' work holds %d live bytes and charges %d", onHeap-offHeap, onUsed-offUsed)
}

// The report's lease holds the projection the windows' work published: under
// the windowed strategy both strict-loading lines lease at least what their
// context records occupy.
func TestWindowSpansLeaseCoversTheProjection(t *testing.T) {
	if raceEnabled {
		t.Skip("measures the heap")
	}
	defer validationSetWindowedStrategy(validationSetWindowedStrategy(true))
	data := spansWindowHeavy(20)
	document, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	adopted := 0
	defer validationSetWindowSpansObserver(validationSetWindowSpansObserver(func(reason string, _ int) {
		if reason == "" {
			adopted++
		}
	}))
	source, model := projectionLeases(t, data, document)
	if adopted == 0 {
		t.Fatal("the windows' work was not adopted")
	}
	// The records are the same under both strategies, and so is their lease.
	validationSetWindowedStrategy(false)
	_, buffered := projectionLeases(t, data, document)
	validationSetWindowedStrategy(true)
	if model.leased-buffered.leased > finishHeapNoise || buffered.leased-model.leased > finishHeapNoise {
		t.Fatalf("the model line leases %d bytes windowed and %d buffered", model.leased, buffered.leased)
	}
	for name, got := range map[string]projectionLease{"source": source, "model": model} {
		spans := 0
		for _, record := range got.report.Context {
			if record.Span != nil {
				spans++
			}
		}
		real := projectionHeap(got.report.Context)
		// Only the model line of strict loading has a spans provider.
		if name == "model" && spans < 500 || got.leased < real {
			t.Fatalf("%s: %d records, %d of spans, leased %d bytes, occupy %d", name, len(got.report.Context), spans, got.leased, real)
		}
		t.Logf("%s: %d records leased %d bytes, occupy %d", name, len(got.report.Context), got.leased, real)
	}
}

// spansContextFacts runs one line with no projection and returns the context
// facts it charged.
func spansContextFacts(t *testing.T, data []byte, windowed, model bool) (facts int64, complete bool) {
	t.Helper()
	defer validationSetWindowedStrategy(validationSetWindowedStrategy(windowed))
	defer validationSetFinishObserver(validationSetFinishObserver(func(stage string, account *validation.Account) {
		if stage == "finished" {
			facts, _ = account.Usage(validation.BudgetContextFacts)
		}
	}))
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34}
	if model {
		document, err := Decode(bytes.NewReader(data))
		if err != nil {
			return 0, false
		}
		report, err := AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelSemantics, Budgets: big})
		return facts, err == nil && report.AssessmentComplete
	}
	report, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big})
	return facts, err == nil && report.AssessmentComplete
}

// spansWindowRandom is a random part of notes with tied marks of every
// shape, lyrics with extends, slurs and beams, in a random version.
func spansWindowRandom(seed int64) []byte {
	rng := rand.New(rand.NewSource(seed))
	pick := func(options ...string) string { return options[rng.Intn(len(options))] }
	var measures []string
	for i := 0; i < 2+rng.Intn(5); i++ {
		body := ""
		if i == 0 {
			body = performanceWindowDivisions
		}
		for j := 0; j < 1+rng.Intn(4); j++ {
			if rng.Intn(8) == 0 {
				body += `<backup><duration>2</duration></backup>`
			}
			ties, marks, extra := "", "", ""
			for n := rng.Intn(3); n > 0; n-- {
				kind := pick("start", "stop", "start", "stop", "continue", "let-ring")
				number := pick("", "", "", ` number="1"`, ` number="2"`)
				marks += `<tied type="` + kind + `"` + number + `/>`
				if kind == "start" || kind == "stop" {
					if rng.Intn(2) == 0 && !strings.Contains(ties, kind) {
						ties += `<tie type="` + kind + `"/>`
					}
				}
			}
			if rng.Intn(4) == 0 {
				marks += `<slur type="` + pick("start", "stop", "continue") + `" number="` + pick("1", "2") + `"/>`
			}
			if rng.Intn(6) == 0 {
				extra += `<beam number="1">` + pick("begin", "continue", "end") + `</beam>`
			}
			if marks != "" {
				extra += spansWindowNotations(marks)
			}
			for n := rng.Intn(3); n > 0 && rng.Intn(3) == 0; n-- {
				extra += `<lyric` + pick(``, ` number="1"`, ` number="1"`, ` number="2"`) + `><text>la</text>` + pick(``, `<extend/>`, `<extend/>`, `<extend type="start"/>`, `<extend type="stop"/>`) + `</lyric>`
			}
			voice := pick("1", "1", "2")
			note := spansWindowTiedNote(pick("C", "C", "D"), ties, extra)
			switch rng.Intn(12) {
			case 0:
				note = strings.Replace(note, `<pitch><step>C</step><octave>4</octave></pitch>`, `<unpitched/>`, 1)
			case 1:
				if ties == "" {
					note = strings.Replace(strings.Replace(note, `<pitch><step>C</step><octave>4</octave></pitch>`, `<rest/>`, 1), `<pitch><step>D</step><octave>4</octave></pitch>`, `<rest/>`, 1)
				}
			}
			body += strings.Replace(note, `<voice>1</voice>`, `<voice>`+voice+`</voice>`, 1)
		}
		measures = append(measures, performanceWindowMeasure(fmt.Sprint(i+1), body))
	}
	document := string(performanceWindowDocument(measures...))
	return []byte(strings.Replace(document, ` version="4.0"`, pick(` version="4.0"`, ` version="4.0"`, ` version="2.0"`, ` version="1.1"`, ``), 1))
}

// With no projection the windows count the provider's facts and keep no
// endpoint. The count is the buffered provider's on every document: the
// risk cases, random documents full of tied marks and typeless extends, and
// the corpus; on both lines.
func TestWindowSpansCountedFactsMatchBuffered(t *testing.T) {
	skipInShortTier(t)
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	documents, _ := spansWindowRiskCases()
	// A hundred random documents by default, three hundred with the corpus.
	random := int64(100)
	if windowedStride() == 1 {
		random = 300
	}
	for seed := int64(0); seed < random; seed++ {
		documents[fmt.Sprintf("random %d", seed)] = spansWindowRandom(seed)
	}
	var files []string
	for _, dir := range []string{"testdata/official", "testdata/corpus"} {
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".musicxml")) {
				files = append(files, path)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < len(files); i += windowedStride() {
		data, err := os.ReadFile(files[i])
		if err != nil {
			t.Fatal(err)
		}
		documents[files[i]] = data
	}
	adopted, counted := 0, 0
	defer validationSetWindowSpansObserver(validationSetWindowSpansObserver(func(reason string, endpoints int) {
		if reason == "" {
			adopted++
			counted += endpoints
		}
	}))
	compared := 0
	for name, data := range documents {
		for _, model := range []bool{false, true} {
			want, complete := spansContextFacts(t, data, false, model)
			got, windowed := spansContextFacts(t, data, true, model)
			if complete != windowed {
				t.Errorf("%s model=%v: complete %v buffered, %v windowed", name, model, complete, windowed)
				continue
			}
			if complete && got != want {
				t.Errorf("%s model=%v: %d context facts windowed, %d buffered\n%s", name, model, got, want, data)
			}
			if complete {
				compared++
			}
		}
	}
	if compared < 250 || adopted < 200 || counted < 2000 {
		t.Fatalf("compared %d runs; %d adopted with %d endpoints counted", compared, adopted, counted)
	}
	t.Logf("compared %d runs; %d adopted with %d endpoints counted", compared, adopted, counted)
}

// Counting windows keep no endpoint: with no projection a part full of span
// marks holds, between windows, the tied runs and nothing that grows with
// the part; and Finish ends with less than the provider that keeps its
// endpoints.
func TestWindowSpansCountingKeepsNoEndpoints(t *testing.T) {
	skipInShortTier(t)
	if raceEnabled {
		t.Skip("the race detector changes the heap")
	}
	data := spansWindowHeavy(100)
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	measure := func(off bool) (heap, used, finishedHeap, finishedUsed int64, endpoints int) {
		defer validationSetWindowedStrategy(validationSetWindowedStrategy(true))
		defer validationSetWindowSpansOff(validationSetWindowSpansOff(off))
		run := func() {
			if _, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big}); err != nil {
				t.Fatal(err)
			}
		}
		run()
		defer validationSetWindowSpansObserver(validationSetWindowSpansObserver(func(reason string, count int) {
			if reason == "" {
				endpoints += count
			}
		}))
		before := liveHeap()
		defer validationSetFinishObserver(validationSetFinishObserver(func(stage string, a *validation.Account) {
			switch stage {
			case "evaluating":
				heap = liveHeap() - before
				used, _ = a.Usage(validation.BudgetBufferedBytes)
			case "finished":
				finishedHeap = liveHeap() - before
				finishedUsed, _ = a.Usage(validation.BudgetBufferedBytes)
			}
		}))
		run()
		return heap, used, finishedHeap, finishedUsed, endpoints
	}
	offHeap, offUsed, offFinishedHeap, offFinishedUsed, _ := measure(true)
	onHeap, onUsed, onFinishedHeap, onFinishedUsed, endpoints := measure(false)
	if endpoints < 2600 {
		t.Fatalf("%d endpoints counted", endpoints)
	}
	if onHeap-offHeap > finishHeapNoise || onUsed-offUsed > finishHeapNoise {
		t.Fatalf("%d endpoints: counting windows hold %d live bytes and charge %d", endpoints, onHeap-offHeap, onUsed-offUsed)
	}
	// The buffered path keeps every endpoint to the end of the session.
	if kept := int64(endpoints) * 1000; offFinishedHeap-onFinishedHeap < kept || offFinishedUsed-onFinishedUsed < kept {
		t.Fatalf("%d endpoints: Finish ends with %d live bytes and %d charged less than with kept endpoints", endpoints, offFinishedHeap-onFinishedHeap, offFinishedUsed-onFinishedUsed)
	}
	t.Logf("%d endpoints: counting windows hold %d live bytes and charge %d; Finish ends %d live bytes and %d charged below kept endpoints", endpoints, onHeap-offHeap, onUsed-offUsed, offFinishedHeap-onFinishedHeap, offFinishedUsed-onFinishedUsed)

}
