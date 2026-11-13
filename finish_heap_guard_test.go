package musicxml

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// While Finish evaluates the document's phases, the buffered charge covers
// the heap: on corpus documents, the official scores and hostile documents,
// for the semantic and context profiles with and without the complete
// projection (on MozartTrio also with windowed evaluation, which packs the
// timeline's records per measure) and for the source line, which releases
// typed XSD results, the
// live heap never exceeds what has been charged to the buffer
// and the diagnostic quota, plus 64 KiB for the runtime's own allocations.
// Two measurements: the whole run's heap against its whole charge, and,
// from the start of Finish, the heap the evaluation adds against what it
// charges, so that input-phase slack cannot hide a Finish undercharge. Both
// are checked at sampled returns of individual evaluations, after their
// transient storage is released, and after the last phase. The Finish delta
// is skipped for the rejected hostile document: its tens of thousands of
// failed decisions make the failed-decision map grow during Finish and move
// entries charged on input into its new table.
func TestFinishChargeCoversTheHeapItAdds(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector changes the heap")
	}
	names, err := filepath.Glob(filepath.Join("testdata", "corpus", "musicxml-test-suite", "*.xml"))
	if err != nil || len(names) < 100 {
		t.Fatalf("corpus: %d files, %v", len(names), err)
	}
	stride, sample := 12, 2000
	if testing.Short() {
		stride, sample = 80, 8000
	}
	type input struct {
		name string
		data []byte
	}
	var inputs []input
	for i := 0; i < len(names); i += stride {
		data, err := os.ReadFile(names[i])
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, input{filepath.Base(names[i]), data})
	}
	official, err := os.ReadFile(filepath.Join("testdata", "official", "MozartTrio.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	inputs = append(inputs, input{"MozartTrio.musicxml", official})
	score := func(content string) []byte {
		return []byte(`<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list><part id="P1"><measure number="1">` + content + `</measure></part></score-partwise>`)
	}
	note := `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`
	// Hostile sizes: 64 KiB of formatted words with 1000 notes, the same with
	// content the schema rejects, a backward repeat played 200 times over 50
	// notes, and 100 notes whose voices are 8 KiB strings.
	words := `<attributes><divisions>1</divisions></attributes><direction><direction-type><words font-family="serif" justify="left">` + strings.Repeat("x", 64<<10) + `</words></direction-type></direction>` + strings.Repeat(note, 1000)
	repeat := `<attributes><divisions>1</divisions></attributes><barline location="left"><repeat direction="forward"/></barline>` + strings.Repeat(note, 50) + `<barline location="right"><repeat direction="backward" times="200"/></barline>`
	voice := `<attributes><divisions>1</divisions></attributes>` + strings.Repeat(`<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><voice>`+strings.Repeat("v", 8<<10)+`</voice></note>`, 100)
	inputs = append(inputs, input{"hostile words", score(words)}, input{"hostile rejected", score(words + strings.Repeat(`<unexpected/>`, 500))}, input{"hostile repeat", score(repeat)}, input{"hostile voices", score(voice)})
	// Each hostile input runs only where its storage is built: formatted
	// words in every profile, repeats in the context profiles' navigation,
	// voices in the source profiles' typed schema values.
	only := map[string]func(label string) bool{
		"hostile repeat": func(label string) bool { return strings.Contains(label, "context") },
		"hostile voices": func(label string) bool { return strings.HasPrefix(label, "source") },
	}
	warmed := map[string]bool{}
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	usage := func(account *validation.Account) int64 {
		buffered, _ := account.Usage(validation.BudgetBufferedBytes)
		diagnostics, _ := account.Usage(validation.BudgetDiagnosticBytes)
		return buffered + diagnostics
	}
	measured := 0
	for _, in := range inputs {
		name, data := in.name, in.data
		// A document the model cannot hold is assessed from its source only.
		document, _ := Decode(bytes.NewReader(data))
		model := func(options ModelOptions) func() {
			return func() {
				if document != nil {
					options.Budgets = big
					_, _ = AssessModel(context.Background(), document, options)
				}
			}
		}
		source := func(options SourceOptions) func() {
			return func() {
				options.Budgets = big
				_, _ = InspectXML(context.Background(), bytes.NewReader(data), options)
			}
		}
		runs := map[string]func(){
			"model semantics":                      model(ModelOptions{}),
			"model semantics, complete projection": model(ModelOptions{Context: validation.ContextFamilyAll}),
			"model context":                        model(ModelOptions{Profile: ProfileModelContext}),
			"model context, complete projection":   model(ModelOptions{Profile: ProfileModelContext, Context: validation.ContextFamilyAll}),
			"source semantics":                     source(SourceOptions{}),
			"source line":                          source(SourceOptions{Profile: ProfileSourceLine}),
			"source context, complete projection":  source(SourceOptions{Profile: ProfileSourceContext, Context: validation.ContextFamilyAll}),
		}
		// Windowed evaluation packs the timeline's records per measure;
		// MozartTrio, where they dominate Finish, runs in it too.
		if name == "MozartTrio.musicxml" {
			for _, label := range []string{"model semantics", "model context, complete projection", "source semantics", "source context, complete projection"} {
				buffered := runs[label]
				runs[label+", windowed"] = func() {
					previous := validationSetWindowedStrategy(true)
					defer validationSetWindowedStrategy(previous)
					buffered()
				}
			}
		}
		for label, run := range runs {
			if keep := only[name]; keep != nil && !keep(label) {
				continue
			}
			if !warmed[label] {
				run() // warm the process-wide caches once per profile
				warmed[label] = true
			}
			var runHeap, finishHeap, finishUsed int64
			failures := name == "hostile rejected"
			calls := 0
			check := func(stage string, account *validation.Account) {
				heap, used := liveHeap(), usage(account)
				if heap-runHeap > used+finishHeapNoise {
					t.Errorf("%s %s at %s: the run holds %d live bytes but charged %d", name, label, stage, heap-runHeap, used)
				}
				if !failures && heap-finishHeap > used-finishUsed+finishHeapNoise {
					t.Errorf("%s %s at %s: Finish added %d live bytes but charged %d", name, label, stage, heap-finishHeap, used-finishUsed)
				}
				measured++
			}
			runHeap = liveHeap()
			previous := validationSetFinishObserver(func(stage string, account *validation.Account) {
				switch stage {
				case "evaluating":
					finishHeap, finishUsed = liveHeap(), usage(account)
				case "evaluated":
					if calls++; calls%sample == 0 {
						check(stage, account)
					}
				case "finished":
					check(stage, account)
				}
			})
			run()
			validationSetFinishObserver(previous)
		}
	}
	if measured < 40 {
		t.Fatalf("measured %d points", measured)
	}
}

// finishHeapNoise bounds the runtime's own allocations between two
// measurements of the live heap.
const finishHeapNoise = 64 << 10

// Evaluation scratch is charged only while its evaluation runs, so what model
// semantics' Finish charges on MozartTrio stays within five and a half times
// the heap it adds (it was more than eight times when layout formatting,
// navigation runs and performance records stayed charged until cleanup).
func TestFinishChargeStaysNearTheHeapItAdds(t *testing.T) {
	if raceEnabled || testing.Short() {
		t.Skip("measures the heap")
	}
	data, err := os.ReadFile(filepath.Join("testdata", "official", "MozartTrio.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	run := func() {
		_, _ = AssessModel(context.Background(), document, ModelOptions{Budgets: validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}})
	}
	run()
	var heap0, used0, heap, used int64
	previous := validationSetFinishObserver(func(stage string, account *validation.Account) {
		switch stage {
		case "evaluating":
			heap0 = liveHeap()
			used0, _ = account.Usage(validation.BudgetBufferedBytes)
		case "finished":
			heap = liveHeap()
			used, _ = account.Usage(validation.BudgetBufferedBytes)
		}
	})
	run()
	validationSetFinishObserver(previous)
	if heap <= heap0 || 2*(used-used0) > 11*(heap-heap0) {
		t.Fatalf("Finish charged %d for %d live bytes", used-used0, heap-heap0)
	}
	t.Logf("Finish charged %d for %d live bytes", used-used0, heap-heap0)
}
