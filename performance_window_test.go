package musicxml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

const performanceWindowScore = `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>A</part-name></score-part>SECOND_PART</part-list><part id="P1">MEASURES</part>SECOND_BODY</score-partwise>`

func performanceWindowDocument(measures ...string) []byte {
	return []byte(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(performanceWindowScore, "SECOND_PART", ""), "SECOND_BODY", ""), "MEASURES", strings.Join(measures, "")))
}

func performanceWindowMeasure(number string, body string) string {
	return `<measure number="` + number + `">` + body + `</measure>`
}

var performanceWindowDivisions = `<attributes><divisions>2</divisions><time><beats>4</beats><beat-type>4</beat-type></time></attributes>`

func performanceWindowNote(body string) string {
	return `<note><pitch><step>C</step><octave>4</octave></pitch>` + body + `</note>`
}

func performanceWindowBends(first, second string) string {
	return performanceWindowNote(`<duration>4</duration><notations><technical><bend><bend-alter>` + first + `</bend-alter></bend><bend><bend-alter>` + second + `</bend-alter><release/></bend></technical></notations>`)
}

func TestWindowPerformanceRiskCases(t *testing.T) {
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	ordinary := performanceWindowNote(`<duration>4</duration><type>whole</type>`)
	eighth := performanceWindowNote(`<duration>1</duration><type>eighth</type>`)
	grace := performanceWindowNote(`<grace steal-time-following="50"/><voice>1</voice>`)
	following := performanceWindowNote(`<duration>2</duration><voice>1</voice><type>quarter</type>`)
	swing := `<direction><direction-type><words>swing</words></direction-type><sound><swing><first>2</first><second>1</second></swing></sound></direction>`
	cases := map[string][]byte{
		"grace following in the next measure": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+grace),
			performanceWindowMeasure("2", following),
		),
		"grace at the end of a part": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+grace),
		),
		"tempo and dynamics within and between measures": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+eighth+`<sound tempo="90" dynamics="70"/>`+eighth),
			performanceWindowMeasure("2", `<sound tempo="120" dynamics="90"/>`+eighth),
		),
		"swing after the first note": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+eighth+swing+eighth),
		),
		"swing in the next measure": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+eighth),
			performanceWindowMeasure("2", swing+eighth),
		),
		"late tempo and dynamics at a negative offset": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+`<sound tempo="60" dynamics="50"/>`+eighth),
			performanceWindowMeasure("2", `<sound tempo="120" dynamics="70"><offset>-4</offset></sound>`+eighth),
		),
		"late swing and pizzicato at a negative offset": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+eighth),
			performanceWindowMeasure("2", `<sound pizzicato="yes"><swing><first>2</first><second>1</second></swing><offset>-2</offset></sound>`+eighth),
		),
		"late tempo without divisions": performanceWindowDocument(
			performanceWindowMeasure("1", eighth),
			performanceWindowMeasure("2", `<sound tempo="120"><offset>-1</offset></sound>`+eighth),
		),
		"negative offset within the same measure": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+eighth+`<sound tempo="120" dynamics="70"><offset>-4</offset></sound>`+eighth),
		),
		"second divisions publisher": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+ordinary),
			performanceWindowMeasure("2", `<attributes><divisions>4</divisions></attributes>`+performanceWindowNote(`<duration>8</duration>`)),
		),
		"bend release": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+performanceWindowBends("2", "-2")),
		),
		"bend release fails": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+performanceWindowBends("2", "0")),
		),
		"bend without prior release or pre-bend": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+performanceWindowBends("2", "-2")),
		),
		"bend without release": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+performanceWindowNote(`<duration>4</duration><notations><technical><bend><bend-alter>2</bend-alter></bend><bend><bend-alter>-2</bend-alter></bend></technical></notations>`)),
		),
		"bend with prior release": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+performanceWindowNote(`<duration>4</duration><notations><technical><bend><bend-alter>2</bend-alter><release/></bend><bend><bend-alter>-2</bend-alter><release/></bend></technical></notations>`)),
		),
		"bend with prior pre-bend": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+performanceWindowNote(`<duration>4</duration><notations><technical><bend><bend-alter>2</bend-alter><pre-bend/></bend><bend><bend-alter>-2</bend-alter><release/></bend></technical></notations>`)),
		),
		"attack release time-only and play": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+performanceWindowNote(`<duration>2</duration><attack>1</attack><release>1</release><time-only>1</time-only><play><mute>on</mute></play>`)),
		),
		"direction and figured bass anchors": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+`<direction><direction-type><words>x</words></direction-type></direction><figured-bass><figure><figure-number>6</figure-number></figure></figured-bass>`+following),
		),
		"parts of different lengths": []byte(strings.NewReplacer(
			"SECOND_PART", `<score-part id="P2"><part-name>B</part-name></score-part>`,
			"SECOND_BODY", `<part id="P2">`+performanceWindowMeasure("1", performanceWindowDivisions+ordinary)+performanceWindowMeasure("2", ordinary)+`</part>`,
			"MEASURES", performanceWindowMeasure("1", performanceWindowDivisions+ordinary),
		).Replace(performanceWindowScore)),
		"excursion requiring a staged unit proof": []byte(timelineStagedUnits(2)),
		"content error proves the window unbound": performanceWindowDocument(
			performanceWindowMeasure("1", performanceWindowDivisions+ordinary),
			performanceWindowMeasure("2", ordinary+`<bogus/>`),
		),
		"late duplicate identity": []byte(strings.Replace(string(performanceWindowDocument(performanceWindowMeasure("1", performanceWindowDivisions+ordinary), performanceWindowMeasure("2", ordinary))), `<measure number="2">`, `<measure number="2" id="P1">`, 1)),
		"timewise":                []byte(`<score-timewise version="4.0"><part-list><score-part id="P1"><part-name>A</part-name></score-part></part-list><measure number="1"><part id="P1">` + performanceWindowDivisions + ordinary + `</part></measure></score-timewise>`),
	}
	valid := performanceWindowDocument(performanceWindowMeasure("1", performanceWindowDivisions+ordinary), performanceWindowMeasure("2", ordinary))
	cases["cut inside a measure"] = valid[:bytes.Index(valid, []byte(`<measure number="2">`))+20]
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			previous := validationSetWindowedStrategy(false)
			buffered := windowedReports(t, name, data)
			validationSetWindowedStrategy(true)
			var adopted, dropped int
			var reasons []string
			previousObserver := validationSetWindowPerformanceObserver(func(reason string, results, decisions int) {
				if reason == "" {
					adopted += results + decisions
				} else {
					dropped += results + decisions
					reasons = append(reasons, reason)
				}
			})
			windowed := windowedReports(t, name, data)
			validationSetWindowPerformanceObserver(previousObserver)
			validationSetWindowedStrategy(previous)
			for label, want := range buffered {
				if windowed[label] != want {
					t.Errorf("%s: windowed report differs from buffered", label)
				}
			}
			if name != "timewise" && adopted+dropped+len(reasons) == 0 {
				t.Fatal("performance windows neither adopted nor dropped any work")
			}
			expected := map[string]string{
				"excursion requiring a staged unit proof":       "excursions",
				"late duplicate identity":                       "speculation",
				"cut inside a measure":                          "unsettled",
				"content error proves the window unbound":       "unbound",
				"late tempo and dynamics at a negative offset":  "late-state",
				"late swing and pizzicato at a negative offset": "late-state",
				"late tempo without divisions":                  "late-state",
			}[name]
			if expected != "" && !slices.Contains(reasons, expected) {
				t.Errorf("degradation reasons %v, want %q", reasons, expected)
			}
			if name == "negative offset within the same measure" && slices.Contains(reasons, "late-state") {
				t.Errorf("same-window offset degraded: %v", reasons)
			}
			t.Logf("adopted=%d dropped=%d reasons=%v", adopted, dropped, reasons)
		})
	}
}

func TestWindowPerformanceDegradationOnCorpus(t *testing.T) {
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
	previousStrategy := validationSetWindowedStrategy(true)
	defer validationSetWindowedStrategy(previousStrategy)
	var current string
	seen := map[string]bool{}
	adopted := map[string]bool{}
	degraded := map[string]bool{}
	failed := map[string]map[string]bool{}
	previousObserver := validationSetWindowPerformanceObserver(func(reason string, results, decisions int) {
		seen[current] = true
		if reason == "" {
			adopted[current] = true
			return
		}
		degraded[current] = true
		if failed[reason] == nil {
			failed[reason] = map[string]bool{}
		}
		failed[reason][current] = true
	})
	defer validationSetWindowPerformanceObserver(previousObserver)
	for i := 0; i < len(files); i += windowedStride() {
		current = files[i]
		data, err := os.ReadFile(current)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big, Context: validation.ContextFamilyAll})
	}
	if len(seen) == 0 {
		t.Fatal("no performance work observed")
	}
	if 100*len(degraded) > 5*len(seen) {
		t.Fatalf("%d of %d documents degraded (%v)", len(degraded), len(seen), failed)
	}
	triggers := map[string]int{}
	for reason, documents := range failed {
		triggers[reason] = len(documents)
	}
	t.Logf("%d documents: %d adopted, %d degraded; triggers=%v", len(seen), len(adopted), len(degraded), triggers)
}

func TestWindowPerformanceChargeCoversTheHeap(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector changes the heap")
	}
	measures := make([]string, 120)
	for i := range measures {
		body := performanceWindowDivisions + performanceWindowNote(`<duration>2</duration><type>quarter</type>`) + `<direction><direction-type><words>x</words></direction-type></direction>` + performanceWindowBends("2", "-2")
		measures[i] = performanceWindowMeasure(fmt.Sprint(i+1), body)
	}
	data := performanceWindowDocument(measures...)
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	measure := func(off bool) (heap, used int64, work int, report string, account *validation.Account) {
		defer validationSetWindowedStrategy(validationSetWindowedStrategy(true))
		defer validationSetWindowPerformanceOff(validationSetWindowPerformanceOff(off))
		run := func() validation.Report {
			r, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big, Context: validation.ContextFamilyAll})
			if err != nil {
				t.Fatal(err)
			}
			return r
		}
		run()
		previousObserver := validationSetWindowPerformanceObserver(func(_ string, results, decisions int) { work += results + decisions })
		before := liveHeap()
		previousFinish := validationSetFinishObserver(func(stage string, a *validation.Account) {
			account = a
			if stage == "evaluating" {
				heap = liveHeap() - before
				used, _ = a.Usage(validation.BudgetBufferedBytes)
			}
		})
		r := run()
		validationSetFinishObserver(previousFinish)
		validationSetWindowPerformanceObserver(previousObserver)
		encoded, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		return heap, used, work, string(encoded), account
	}
	offHeap, offUsed, _, offReport, _ := measure(true)
	onHeap, onUsed, work, onReport, account := measure(false)
	if onReport != offReport {
		t.Fatal("windowed performance report differs from its fallback")
	}
	if work < 1000 {
		t.Fatalf("%d provisional results", work)
	}
	if onHeap-offHeap > onUsed-offUsed+finishHeapNoise {
		t.Fatalf("provisional records hold %d live bytes and charge %d", onHeap-offHeap, onUsed-offUsed)
	}
	if account == nil {
		t.Fatal("no account observed")
	}
	if used, _ := account.Usage(validation.BudgetBufferedBytes); used != 0 {
		t.Fatalf("%d bytes charged after the run", used)
	}
	t.Logf("%d provisional results hold %d live bytes and charge %d", work, onHeap-offHeap, onUsed-offUsed)
}

func TestWindowPerformanceUnderContextFactQuotas(t *testing.T) {
	data := performanceWindowDocument(
		performanceWindowMeasure("1", `<attributes><divisions>2</divisions><key><fifths>1</fifths></key></attributes>`+performanceWindowNote(`<duration>2</duration>`)+performanceWindowBends("2", "-2")),
		performanceWindowMeasure("2", performanceWindowNote(`<duration>2</duration>`)),
	)
	type outcome struct {
		report   string
		complete bool
		terminal *validation.TerminalSummary
		err      error
		facts    int64
	}
	run := func(windowed bool, quota int64) outcome {
		previous := validationSetWindowedStrategy(windowed)
		defer validationSetWindowedStrategy(previous)
		var out outcome
		previousFinish := validationSetFinishObserver(func(stage string, account *validation.Account) {
			if stage == "finished" {
				out.facts, _ = account.Usage(validation.BudgetContextFacts)
			}
		})
		defer validationSetFinishObserver(previousFinish)
		r, err := InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34, MaxContextFacts: quota}, Context: validation.ContextFamilyAll})
		encoded, marshalErr := json.Marshal(r)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		out.report, out.complete, out.terminal, out.err = string(encoded), r.AssessmentComplete, r.Terminal, err
		return out
	}
	full := run(false, 1<<30)
	if full.facts == 0 {
		t.Fatal("no context facts measured")
	}
	checked := 0
	for quota := int64(0); quota <= full.facts+1; quota++ {
		buffered, windowed := run(false, quota), run(true, quota)
		if buffered.complete && windowed.complete {
			if buffered.report != windowed.report {
				t.Fatalf("quota %d: reports differ", quota)
			}
			checked++
			continue
		}
		if buffered.complete != windowed.complete {
			t.Fatalf("quota %d: completeness %v/%v", quota, buffered.complete, windowed.complete)
		}
		if buffered.complete {
			t.Fatalf("quota %d: buffered completed, windowed did not", quota)
		}
		if buffered.terminal == nil || windowed.terminal == nil || *buffered.terminal != *windowed.terminal {
			t.Fatalf("quota %d: terminals %+v/%+v", quota, buffered.terminal, windowed.terminal)
		}
		if (buffered.err == nil) != (windowed.err == nil) {
			t.Fatalf("quota %d: errors %v/%v", quota, buffered.err, windowed.err)
		}
		checked++
	}
	if checked < 10 {
		t.Fatalf("checked %d quotas", checked)
	}
}
