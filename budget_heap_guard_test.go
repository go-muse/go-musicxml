package musicxml

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// liveHeap returns the live heap after collection.
func liveHeap() int64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.HeapAlloc)
}

// At the end of input a session holds its fact graph, the XSD run's results
// and its rule outcomes, and the buffered-byte charge covers the heap they
// occupy: on corpus documents, for source and model profiles with and
// without XSD, the live heap the run added never exceeds the account's
// buffered usage plus the diagnostic bytes of the findings it stored. A
// synthetic document adds hostile sizes. Completed XSD subjects release their
// work charge, so this guards the release against undercharging.
func TestBufferedChargeCoversTheHeapAtEndOfInput(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector changes the heap")
	}
	names, err := filepath.Glob(filepath.Join("testdata", "corpus", "musicxml-test-suite", "*.xml"))
	if err != nil || len(names) < 100 {
		t.Fatalf("corpus: %d files, %v", len(names), err)
	}
	names = append(names, filepath.Join("testdata", "official", "MozartTrio.musicxml"))
	stride := 6
	if testing.Short() {
		stride = 40
	}
	type input struct {
		name string
		data []byte
	}
	var inputs []input
	for i := 0; i < len(names); i += stride {
		if i+stride >= len(names) {
			i = len(names) - 1
		}
		data, err := os.ReadFile(names[i])
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, input{filepath.Base(names[i]), data})
	}
	// Hostile sizes: a 64 KiB text and a measure of 5000 children, whose node
	// text and child slice a completed subject releases.
	hostile := `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list><part id="P1"><measure number="1"><attributes><divisions>1</divisions></attributes><direction><direction-type><words>` + strings.Repeat("x", 64<<10) + `</words></direction-type></direction>` + strings.Repeat(`<note><rest/><duration>1</duration></note>`, 5000) + `</measure></part></score-partwise>`
	inputs = append(inputs, input{"hostile", []byte(hostile)})
	sources := []validation.ProfileID{profileSourceXSDPreview, profileSourceLine, profileSourceContext}
	models := []validation.ProfileID{profileModelXSD, profileModelContext}
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	measured := 0
	check := func(name string, profile validation.ProfileID, account *validation.Account, heap int64) {
		t.Helper()
		used, err := account.Usage(validation.BudgetBufferedBytes)
		if err != nil {
			t.Fatal(err)
		}
		// Stored diagnostics have their own byte quota.
		diagnostics, err := account.Usage(validation.BudgetDiagnosticBytes)
		if err != nil {
			t.Fatal(err)
		}
		if heap > used+diagnostics {
			t.Errorf("%s %s: live heap %d exceeds buffered charge %d and diagnostic charge %d", name, profile, heap, used, diagnostics)
		}
		measured++
	}
	for _, in := range inputs {
		name, data := in.name, in.data
		document, err := Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		for _, profile := range sources {
			engine, err := sourceAssessmentEngineForProfile(profile)
			if err != nil {
				t.Fatal(err)
			}
			for repeat := 0; repeat < 2; repeat++ { // the first run warms caches
				account, err := validation.NewBudgetAccount(big)
				if err != nil {
					t.Fatal(err)
				}
				reader := bytes.NewReader(data)
				before := liveHeap()
				run, err := newSourceAssessmentRunWithConfig(context.Background(), reader, DecodeOptions{}, &sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: account, DocumentID: sourceDocumentID, Profile: profile, Engine: engine}, ByteOwner: sourceBytesPrimary}, false)
				if err != nil {
					t.Fatal(err)
				}
				inputErr := run.drain()
				heap := liveHeap() - before
				if inputErr != nil {
					t.Fatalf("%s %s: %v", name, profile, inputErr)
				}
				if repeat == 1 {
					check(name, profile, account, heap)
				}
				completion, err := run.finishInput(inputErr)
				if err == nil {
					_, _ = run.session.Finish(context.Background(), completion)
				}
			}
		}
		for _, profile := range models {
			engine, err := modelAssessmentEngineForProfile(profile)
			if err != nil {
				t.Fatal(err)
			}
			for repeat := 0; repeat < 2; repeat++ {
				account, err := validation.NewBudgetAccount(big)
				if err != nil {
					t.Fatal(err)
				}
				before := liveHeap()
				run, err := newModelAssessmentRun(context.Background(), document, assessmentRunConfig{Account: account, DocumentID: modelDocumentID, Profile: profile, Engine: engine})
				if err != nil {
					t.Fatal(err)
				}
				_, walkErr := run.walk(modelWalkOptions{})
				heap := liveHeap() - before
				if walkErr != nil {
					t.Fatalf("%s %s: %v", name, profile, walkErr)
				}
				if repeat == 1 {
					check(name, profile, account, heap)
				}
				completion, err := run.finishInput(walkErr)
				if err == nil {
					_, _ = run.session.Finish(context.Background(), completion)
				}
			}
		}
	}
	if measured < 20 {
		t.Fatalf("measured %d runs", measured)
	}
}

// The official scores load within the default buffer: the default empty
// context selection and the released XSD work charges bring MozartTrio and
// MozaChloSample, which needed about 76 and 84 MiB when the complete
// projection was the default, under 64 MiB. They still need more than the
// default 20 million rule steps.
func TestStrictLoadingOfficialScoresFitTheDefaultBuffer(t *testing.T) {
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	for _, name := range []string{"MozartTrio.musicxml", "MozaChloSample.musicxml"} {
		data, err := os.ReadFile(filepath.Join("testdata", "official", name))
		if err != nil {
			t.Fatal(err)
		}
		result, err := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Assessment: &SourceOptions{Budgets: validation.Budgets{MaxRuleSteps: 40_000_000}}})
		if err != nil || !result.Accepted {
			t.Errorf("%s: accepted=%v error=%v", name, result.Accepted, err)
		}
	}
}

// With the default empty context selection, raising only the rule steps lets
// both context profiles complete on both official samples within the default
// buffer.
func TestContextProfilesOfficialSamplesNeedOnlyMoreSteps(t *testing.T) {
	if raceEnabled || testing.Short() {
		t.Skip("slow under the race detector")
	}
	budgets := validation.Budgets{MaxRuleSteps: 1 << 34}
	for _, name := range []string{"MozartTrio.musicxml", "MozaChloSample.musicxml"} {
		raw, err := os.ReadFile(filepath.Join("testdata", "official", name))
		if err != nil {
			t.Fatal(err)
		}
		document, err := Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		source, err := InspectXML(context.Background(), bytes.NewReader(reviewBudgetRemoveDOCTYPE(t, raw)), SourceOptions{Profile: ProfileSourceContext, Budgets: budgets})
		if err != nil || !source.Satisfied() {
			t.Errorf("%s source: satisfied=%v error=%v", name, source.Satisfied(), err)
		}
		model, err := AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelContext, Budgets: budgets})
		if err != nil || !model.Satisfied() {
			t.Errorf("%s model: satisfied=%v error=%v", name, model.Satisfied(), err)
		}
	}
}
