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

// timelineCompactInputs are synthetic documents for what the corpus has
// little of: grace, chord and cue notes, a bend, note play instructions and
// attributes, a displaced direction with a sound, parts of different length,
// a cross-frame backup after a divisions change (the staged unit proof that
// reads every measure again), and a timewise score.
var timelineCompactInputs = map[string]string{
	"notes": `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>A</part-name></score-part><score-part id="P2"><part-name>B</part-name></score-part></part-list>` +
		`<part id="P1"><measure number="1"><attributes><divisions>2</divisions><time><beats>2</beats><beat-type>4</beat-type></time></attributes>` +
		`<note><grace steal-time-following="50"/><pitch><step>D</step><octave>4</octave></pitch><voice>1</voice></note>` +
		`<note dynamics="80" attack="1" time-only="1"><pitch><step>C</step><octave>4</octave></pitch><duration>2</duration><voice>1</voice></note>` +
		`<note><chord/><pitch><step>E</step><octave>4</octave></pitch><duration>2</duration><voice>1</voice></note>` +
		`<direction placement="above"><direction-type><words>x</words></direction-type><offset>1</offset><sound tempo="90"/></direction>` +
		`<note release="1"><pitch><step>G</step><octave>4</octave></pitch><duration>2</duration><voice>1</voice><notations><technical><bend><bend-alter>1</bend-alter></bend></technical></notations></note>` +
		`<backup><duration>4</duration></backup>` +
		`<note><cue/><pitch><step>C</step><octave>3</octave></pitch><duration>4</duration><voice>2</voice><play><mute>on</mute></play></note>` +
		`</measure><measure number="2"><note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration><voice>1</voice></note></measure></part>` +
		`<part id="P2"><measure number="1"><attributes><divisions>1</divisions></attributes><note><rest/><duration>2</duration></note></measure></part></score-partwise>`,
	"staged units": timelineStagedUnits(120),
	"timewise": `<score-timewise version="4.0"><part-list><score-part id="P1"><part-name>A</part-name></score-part><score-part id="P2"><part-name>B</part-name></score-part></part-list>` +
		`<measure number="1"><part id="P1"><attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note></part>` +
		`<part id="P2"><attributes><divisions>2</divisions></attributes><note><pitch><step>D</step><octave>4</octave></pitch><duration>2</duration></note></part></measure>` +
		`<measure number="2"><part id="P1"><note><pitch><step>E</step><octave>4</octave></pitch><duration>1</duration></note></part>` +
		`<part id="P2"><note><pitch><step>F</step><octave>4</octave></pitch><duration>2</duration></note></part></measure></score-timewise>`,
}

// timelineStagedUnits is a part whose divisions alternate between 1 and 2
// from measure to measure, each measure with a backup past its start: the
// unit proof is staged and reads every measure again. Its timeline is large
// enough for a compact store.
func timelineStagedUnits(measures int) string {
	var b strings.Builder
	b.WriteString(`<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>A</part-name></score-part></part-list><part id="P1">`)
	for m := 1; m <= measures; m++ {
		units := 1 + m%2
		fmt.Fprintf(&b, `<measure number="%d"><attributes><divisions>%d</divisions></attributes>`, m, units)
		fmt.Fprintf(&b, `<note><pitch><step>D</step><octave>4</octave></pitch><duration>%d</duration><voice>1</voice></note>`, units)
		fmt.Fprintf(&b, `<backup><duration>%d</duration></backup>`, 2*units)
		fmt.Fprintf(&b, `<note><pitch><step>E</step><octave>4</octave></pitch><duration>%d</duration><voice>2</voice></note></measure>`, units)
	}
	b.WriteString(`</part></score-partwise>`)
	return b.String()
}

// timelineRecords runs one line on data with the given strategy and returns
// the dumps and counts of every timeline service that finalized.
func timelineRecords(t *testing.T, name, line string, data []byte, document Document, windowed bool) ([]string, [][10]int) {
	t.Helper()
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34}
	var dumps []string
	var counts [][10]int
	previousStrategy := validationSetWindowedStrategy(windowed)
	previous := validationSetTimelineObserver(func(dump func() string, c [10]int) {
		dumps = append(dumps, dump())
		counts = append(counts, c)
	})
	var err error
	if line == "source" {
		_, err = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big})
	} else {
		_, err = AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelSemantics, Budgets: big})
	}
	validationSetTimelineObserver(previous)
	validationSetWindowedStrategy(previousStrategy)
	if err != nil {
		t.Fatalf("%s %s: %v", name, line, err)
	}
	return dumps, counts
}

// Windowed evaluation keeps each evaluated measure's timeline facts, note
// timing origins and note playback contexts in compact records. Every reader
// of the timeline service sees exactly the records buffered evaluation holds:
// on the corpus and on synthetic documents, for both lines, the canonical
// dumps of the two strategies are equal, and most records are compact.
func TestTimelineCompactRecordsMatchBuffered(t *testing.T) {
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	type input struct {
		name string
		data []byte
	}
	var inputs []input
	for name, text := range timelineCompactInputs {
		inputs = append(inputs, input{name, []byte(text)})
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
		inputs = append(inputs, input{files[i], data})
	}
	var total [10]int
	services, packedServices := 0, 0
	for _, in := range inputs {
		document, err := Decode(bytes.NewReader(in.data))
		for _, line := range []string{"source", "model"} {
			if line == "model" && err != nil {
				continue
			}
			buffered, bufferedCounts := timelineRecords(t, in.name, line, in.data, document, false)
			windowed, windowedCounts := timelineRecords(t, in.name, line, in.data, document, true)
			if len(buffered) != len(windowed) {
				t.Fatalf("%s %s: %d buffered timeline services, %d windowed", in.name, line, len(buffered), len(windowed))
			}
			for k := range buffered {
				if buffered[k] != windowed[k] {
					t.Fatalf("%s %s: windowed timeline records differ:\nbuffered %s\nwindowed %s", in.name, line, firstDifferentLine(buffered[k], windowed[k]), firstDifferentLine(windowed[k], buffered[k]))
				}
			}
			stored := 0
			for k := range buffered {
				b, w := bufferedCounts[k], windowedCounts[k]
				if b[3]+b[4]+b[5] != 0 || [3]int(b[:3]) != [3]int(b[6:]) {
					t.Fatalf("%s %s: buffered evaluation holds records outside its working maps: %v", in.name, line, b)
				}
				if [3]int(b[:3]) != [3]int(w[:3]) {
					t.Fatalf("%s %s: record counts %v buffered, %v windowed", in.name, line, b, w)
				}
				services++
				if w[3]+w[4]+w[5] == 0 {
					// A timeline too small to pay for a store keeps its working
					// maps, as buffered evaluation does.
					if [3]int(w[:3]) != [3]int(w[6:]) {
						t.Fatalf("%s %s: windowed records outside the working maps without compact ones: %v", in.name, line, w)
					}
					continue
				}
				// Once a store exists, every record is packed, also after a
				// staged unit proof that read every measure again.
				stored++
				if w[6]+w[7]+w[8] != 0 {
					t.Fatalf("%s %s: %v records left in the working maps", in.name, line, w[6:])
				}
				if _, synthetic := timelineCompactInputs[in.name]; !synthetic {
					for i := range total {
						total[i] += w[i]
					}
					packedServices++
				}
			}
			switch in.name {
			case "notes", "timewise":
				if stored != 0 {
					t.Fatalf("%s %s: a small timeline started a compact store", in.name, line)
				}
			case "staged units":
				if stored != len(windowed) {
					t.Fatalf("%s %s: %d of %d timeline services packed", in.name, line, stored, len(windowed))
				}
				// Once the store exists, each measure (an attributes element,
				// two notes and a backup) is packed as soon as it is read.
				for _, w := range windowedCounts {
					if w[9] > 4 {
						t.Fatalf("%s %s: %d facts packed at once after the store started", in.name, line, w[9])
					}
				}
			}
		}
	}
	t.Logf("%d services, %d corpus services with a compact store; there facts %d (compact %d), note origins %d (compact %d), note playback %d (compact %d)", services, packedServices, total[0], total[3], total[1], total[4], total[2], total[5])
	// Overflow is for exceptional records: where a corpus timeline has a
	// store, the encoding must hold at least nine in ten records of each kind.
	// (The synthetic staged units are mostly displaced and overflow.)
	for kind := range 3 {
		if total[kind] == 0 || 10*total[kind+3] < 9*total[kind] {
			t.Fatalf("record kind %d: %d of %d compact", kind, total[kind+3], total[kind])
		}
	}
}

func firstDifferentLine(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := range al {
		if i >= len(bl) || al[i] != bl[i] {
			return al[i]
		}
	}
	return "<no further line>"
}

// Windowed evaluation packs each measure's timeline records as it is read.
// On MozartTrio four times over, from the start of Finish to where the
// timeline finalizes, the heap a windowed run adds stays within what it
// charges, and is at most a third of what buffered evaluation adds there.
func TestTimelineCompactChargeCoversWindowedTimeline(t *testing.T) {
	if raceEnabled || testing.Short() {
		t.Skip("measures the heap")
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "official", "MozartTrio.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	data := scaledScore(t, raw, 4)
	document, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	for _, line := range []string{"source", "model"} {
		run := func() {
			if line == "source" {
				_, _ = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big})
			} else {
				_, _ = AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelSemantics, Budgets: big})
			}
		}
		measure := func(windowed bool) (heap, used int64) {
			previousStrategy := validationSetWindowedStrategy(windowed)
			defer validationSetWindowedStrategy(previousStrategy)
			run()
			var heap0, used0 int64
			var account *validation.Account
			observed := false
			previousFinish := validationSetFinishObserver(func(stage string, a *validation.Account) {
				if stage == "evaluating" {
					account = a
					heap0 = liveHeap()
					used0, _ = a.Usage(validation.BudgetBufferedBytes)
				}
			})
			previousTimeline := validationSetTimelineObserver(func(_ func() string, _ [10]int) {
				if !observed && account != nil {
					observed = true
					heap = liveHeap() - heap0
					now, _ := account.Usage(validation.BudgetBufferedBytes)
					used = now - used0
				}
			})
			run()
			validationSetTimelineObserver(previousTimeline)
			validationSetFinishObserver(previousFinish)
			if !observed {
				t.Fatalf("%s: no timeline finalized", line)
			}
			return heap, used
		}
		bufferedHeap, bufferedUsed := measure(false)
		windowedHeap, windowedUsed := measure(true)
		t.Logf("%s: buffered adds %d live bytes, charged %d; windowed %d, charged %d", line, bufferedHeap, bufferedUsed, windowedHeap, windowedUsed)
		if windowedHeap > windowedUsed+finishHeapNoise {
			t.Errorf("%s: windowed evaluation holds %d live bytes but charged %d", line, windowedHeap, windowedUsed)
		}
		if 3*windowedHeap > bufferedHeap {
			t.Errorf("%s: windowed evaluation holds %d live bytes, buffered %d", line, windowedHeap, bufferedHeap)
		}
	}
}
