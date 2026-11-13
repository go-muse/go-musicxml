package musicxml

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Windowed evaluation keeps a source origin whole only while its node is
// open, and releases its copy when the node closes. On MozartTrio four times
// over, at the end of input of the semantic source line, the windowed run
// holds at most three quarters of the buffered run's heap, so the released
// copies are really gone (kept alive, they leave it above the buffered heap),
// and its charge still covers what it holds. The timeline is not read as
// windows close here, so that only what input keeps differs.
func TestWindowedSourceOriginsFreeTheirCopies(t *testing.T) {
	if raceEnabled || testing.Short() {
		t.Skip("measures the heap")
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "official", "MozartTrio.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	data := scaledScore(t, raw, 4)
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	previousTamper := validationSetTimelineSpeculationTamper("off")
	defer validationSetTimelineSpeculationTamper(previousTamper)
	measure := func(windowed bool) (heap, used int64) {
		previous := validationSetWindowedStrategy(windowed)
		defer validationSetWindowedStrategy(previous)
		run := func() {
			_, _ = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big})
		}
		run()
		heap0 := liveHeap()
		previousObserver := validationSetFinishObserver(func(stage string, account *validation.Account) {
			if stage == "evaluating" {
				heap = liveHeap() - heap0
				used, _ = account.Usage(validation.BudgetBufferedBytes)
			}
		})
		run()
		validationSetFinishObserver(previousObserver)
		return heap, used
	}
	bufferedHeap, bufferedUsed := measure(false)
	windowedHeap, windowedUsed := measure(true)
	t.Logf("buffered %d live, %d charged; windowed %d live, %d charged", bufferedHeap, bufferedUsed, windowedHeap, windowedUsed)
	if 4*windowedHeap > 3*bufferedHeap {
		t.Fatalf("windowed evaluation holds %d live bytes, buffered %d", windowedHeap, bufferedHeap)
	}
	if windowedHeap > windowedUsed {
		t.Fatalf("windowed evaluation holds %d live bytes but charges %d", windowedHeap, windowedUsed)
	}
}
