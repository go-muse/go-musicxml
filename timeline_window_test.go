package musicxml

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// timelineTriggerNames name the triggers of validation.timelineTrigger, in
// order.
var timelineTriggerNames = []string{"none", "timewise", "unbound", "hierarchy", "partless-gap", "duplicate-domain", "blocks", "incomplete-input", "identity", "late-failure"}

// timelineWindowRuns runs one line on data with windowed evaluation and
// returns the trigger and the measures read at window close of every
// timeline it read.
func timelineWindowRuns(t *testing.T, line string, data []byte, document Document) [][2]int {
	t.Helper()
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34}
	var runs [][2]int
	previousStrategy := validationSetWindowedStrategy(true)
	previous := validationSetTimelineDegradationObserver(func(trigger, measures int) { runs = append(runs, [2]int{trigger, measures}) })
	var err error
	if line == "source" {
		_, err = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big})
	} else {
		_, err = AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelSemantics, Budgets: big})
	}
	validationSetTimelineDegradationObserver(previous)
	validationSetWindowedStrategy(previousStrategy)
	if err != nil {
		t.Fatalf("%s: %v", line, err)
	}
	return runs
}

// Windowed evaluation reads the measures of a partwise score as their windows
// close. On the corpus the speculation holds for at least 95 % of the
// documents; each degradation is counted by its trigger.
func TestTimelineWindowCloseOnCorpus(t *testing.T) {
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
	counts := make([]int, len(timelineTriggerNames))
	documents, degraded, measures := 0, 0, 0
	for i := 0; i < len(files); i += windowedStride() {
		data, err := os.ReadFile(files[i])
		if err != nil {
			t.Fatal(err)
		}
		document, decodeErr := Decode(bytes.NewReader(data))
		worst := 0
		seen := false
		for _, line := range []string{"source", "model"} {
			if line == "model" && decodeErr != nil {
				continue
			}
			for _, run := range timelineWindowRuns(t, line, data, document) {
				seen = true
				counts[run[0]]++
				measures += run[1]
				if run[0] != 0 {
					worst = run[0]
					t.Logf("%s %s: %s", filepath.Base(files[i]), line, timelineTriggerNames[run[0]])
				}
			}
		}
		if seen {
			documents++
			if worst != 0 {
				degraded++
			}
		}
	}
	for k, name := range timelineTriggerNames {
		if counts[k] != 0 {
			t.Logf("%s: %d timelines", name, counts[k])
		}
	}
	t.Logf("%d documents, %d degraded; %d measures read as their windows closed", documents, degraded, measures)
	if documents == 0 || degraded*20 > documents {
		t.Fatalf("%d of %d documents degraded", degraded, documents)
	}
}

// timelineWindowScore is a partwise score: header, a part-list of the given
// score-parts and the given parts.
func timelineWindowScore(scoreParts, parts string) string {
	return `<?xml version="1.0" encoding="UTF-8"?><score-partwise version="4.0"><part-list>` + scoreParts + `</part-list>` + parts + `</score-partwise>`
}

func timelineWindowPart(id string, measures ...string) string {
	out := `<part id="` + id + `">`
	for i, m := range measures {
		out += fmt.Sprintf(`<measure number="%d">%s</measure>`, i+1, m)
	}
	return out + `</part>`
}

const (
	timelineWindowNote = `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`
	timelineWindowAttr = `<attributes><divisions>1</divisions></attributes>`
	timelineWindowSP   = `<score-part id="P1"><part-name>A</part-name><score-instrument id="I1"><instrument-name>Flute</instrument-name></score-instrument><midi-instrument id="I1"><midi-channel>1</midi-channel></midi-instrument></score-part>`
	timelineWindowSP2  = `<score-part id="P2"><part-name>B</part-name></score-part>`
	// timelineWindowBare has no instrument references to resolve.
	timelineWindowBare = `<score-part id="P1"><part-name>A</part-name></score-part>`
)

// Each assumption of a timeline read as its windows close has a document
// that breaks it. The timeline is then read whole in Finish, the
// degradation is counted by its trigger, and every report, with and without
// the complete projection, is byte-identical to buffered evaluation's.
func TestTimelineWindowCloseTriggers(t *testing.T) {
	instrumentNote := `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><instrument id="I1"/></note>`
	for _, tc := range []struct {
		name, trigger, data string
	}{
		{"read at window close", "none", timelineWindowScore(timelineWindowSP+timelineWindowSP2, timelineWindowPart("P1", timelineWindowAttr+instrumentNote, timelineWindowNote)+timelineWindowPart("P2", timelineWindowAttr+timelineWindowNote))},
		{"timewise", "timewise", windowedTimewiseScore},
		{"part naming no score-part", "hierarchy", timelineWindowScore(timelineWindowSP, timelineWindowPart("P1", timelineWindowAttr+timelineWindowNote)+timelineWindowPart("P9", timelineWindowAttr+timelineWindowNote))},
		{"part child other than a measure", "unbound", timelineWindowScore(timelineWindowSP, strings.Replace(timelineWindowPart("P1", timelineWindowAttr+timelineWindowNote, timelineWindowNote), `<measure number="2">`, `<stray/><measure number="2">`, 1))},
		{"two parts naming one score-part", "duplicate-domain", timelineWindowScore(timelineWindowSP, timelineWindowPart("P1", timelineWindowAttr+timelineWindowNote)+timelineWindowPart("P1", timelineWindowAttr+timelineWindowNote))},
		{"score-part without a part", "blocks", timelineWindowScore(timelineWindowSP+timelineWindowSP2, timelineWindowPart("P1", timelineWindowAttr+timelineWindowNote))},
		{"stray root child", "partless-gap", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", timelineWindowAttr+timelineWindowNote)+`<stray/>`)},
		{"content error after the last measure", "partless-gap", timelineWindowScore(timelineWindowBare, strings.Replace(timelineWindowPart("P1", timelineWindowAttr+timelineWindowNote), `</part>`, `<stray/></part>`, 1))},
		{"identity repeated later", "identity", timelineWindowScore(timelineWindowSP, timelineWindowPart("P1", timelineWindowAttr+instrumentNote, `<note id="I1"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`))},
		{"invalid attribute value", "none", timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", timelineWindowAttr+`<note dynamics="abc"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`+`<barline location="bogus"/>`))},
		{"invalid scalar", "none", timelineWindowScore(timelineWindowSP, timelineWindowPart("P1", timelineWindowAttr+`<note><pitch><step>C</step><octave>4</octave></pitch><duration>x</duration></note>`))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.data)
			document, decodeErr := Decode(bytes.NewReader(data))
			seen := map[string]bool{}
			for _, line := range []string{"source", "model"} {
				if line == "model" && decodeErr != nil {
					continue
				}
				for _, run := range timelineWindowRuns(t, line, data, document) {
					seen[timelineTriggerNames[run[0]]] = true
					t.Logf("%s: %s after %d measures", line, timelineTriggerNames[run[0]], run[1])
				}
			}
			if !seen[tc.trigger] {
				t.Fatalf("triggers %v, want %s", seen, tc.trigger)
			}
			buffered := windowedReports(t, tc.name, data)
			previous := validationSetWindowedStrategy(true)
			windowed := windowedReports(t, tc.name, data)
			validationSetWindowedStrategy(previous)
			for label, want := range buffered {
				if windowed[label] != want {
					t.Errorf("%s: windowed report differs from buffered", label)
				}
			}
		})
	}
}

// Values a measure read as unavailable or invalid, and schema checks it read
// with no failure recorded, are read again in Finish; a different answer
// drops the speculation. No document of the corpus or of the triggers above
// changes them, since a content error first leaves part identities
// unresolved, so the test breaks a recorded read instead.
func TestTimelineWindowCloseRechecksReads(t *testing.T) {
	invalidDynamics := []byte(timelineWindowScore(timelineWindowBare, timelineWindowPart("P1", timelineWindowAttr+`<note dynamics="abc"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`)))
	repeatedID := []byte(timelineWindowScore(timelineWindowSP, timelineWindowPart("P1", timelineWindowAttr+`<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><instrument id="I1"/></note>`, `<note id="I1"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`)))
	for _, tc := range []struct {
		tamper, trigger string
		data            []byte
	}{{"scalar", "late-failure", invalidDynamics}, {"invalid", "late-failure", repeatedID}, {"closure", "incomplete-input", invalidDynamics}} {
		previous := validationSetTimelineSpeculationTamper(tc.tamper)
		document, _ := Decode(bytes.NewReader(tc.data))
		runs := timelineWindowRuns(t, "source", tc.data, document)
		validationSetTimelineSpeculationTamper(previous)
		if len(runs) == 0 || timelineTriggerNames[runs[0][0]] != tc.trigger {
			t.Fatalf("%s: runs %v, want %s", tc.tamper, runs, tc.trigger)
		}
	}
}

// A speculation dropped in Finish releases what reading its measures
// retained, and its timeline is unreachable then. On MozartTrio four times
// over with a stray root child, which drops it after every measure was read,
// the heap a dropped windowed run holds at the end of Finish beyond a
// windowed run that reads no timeline as its windows close is covered by
// what it charges beyond it, and the release leaves it charging within 2 % of
// that run.
func TestTimelineWindowCloseDroppedChargeCoversTheHeap(t *testing.T) {
	if raceEnabled || testing.Short() {
		t.Skip("measures the heap")
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "official", "MozartTrio.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Replace(scaledScore(t, raw, 4), []byte("</score-partwise>"), []byte("<stray/></score-partwise>"), 1)
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	measure := func(speculate bool) (heap, used int64, triggers []int) {
		previousStrategy := validationSetWindowedStrategy(true)
		defer validationSetWindowedStrategy(previousStrategy)
		if !speculate {
			previous := validationSetTimelineSpeculationTamper("off")
			defer validationSetTimelineSpeculationTamper(previous)
		}
		run := func() {
			_, _ = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big})
		}
		run()
		previousObserver := validationSetTimelineDegradationObserver(func(trigger, measures int) {
			if measures == 0 {
				t.Errorf("trigger %d before any measure was read", trigger)
			}
			triggers = append(triggers, trigger)
		})
		heap0 := liveHeap()
		previous := validationSetFinishObserver(func(stage string, account *validation.Account) {
			if stage == "finished" {
				heap = liveHeap() - heap0
				used, _ = account.Usage(validation.BudgetBufferedBytes)
			}
		})
		run()
		validationSetFinishObserver(previous)
		validationSetTimelineDegradationObserver(previousObserver)
		return heap, used, triggers
	}
	bufferedHeap, bufferedUsed, _ := measure(false)
	heap, used, triggers := measure(true)
	if len(triggers) == 0 || triggers[0] == 0 {
		t.Fatalf("triggers %v: the speculation was not dropped", triggers)
	}
	t.Logf("dropped (%s): %d live bytes, %d charged; not read at window close %d live, %d charged", timelineTriggerNames[triggers[0]], heap, used, bufferedHeap, bufferedUsed)
	if heap-bufferedHeap > used-bufferedUsed+finishHeapNoise {
		t.Errorf("the dropped run holds %d live bytes more but charges %d more", heap-bufferedHeap, used-bufferedUsed)
	}
	if diff := used - bufferedUsed; 50*diff > bufferedUsed || -50*diff > bufferedUsed {
		t.Errorf("the dropped run charges %d, the other %d", used, bufferedUsed)
	}
}
