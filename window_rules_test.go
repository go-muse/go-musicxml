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

// Under the windowed strategy the node rules of a bound measure window whose
// plans read only the window are evaluated as the window closes; their
// outcomes are recorded when the root closes. On the corpus most windows are
// evaluated then and they decide a share of every run's node rules, and the
// windows that wait for the root are those whose binding proved a failure
// or that were not bound.
func TestWindowRulesOnCorpus(t *testing.T) {
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
	evaluated, waited, decisions := 0, 0, 0
	previousStrategy := validationSetWindowedStrategy(true)
	defer validationSetWindowedStrategy(previousStrategy)
	for i := 0; i < len(files); i += windowedStride() {
		data, err := os.ReadFile(files[i])
		if err != nil {
			t.Fatal(err)
		}
		last := 0
		previous := validationSetWindowRulesObserver(func(done bool, kept int) {
			if done {
				evaluated++
			} else {
				waited++
			}
			last = kept
		})
		_, err = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceSemantics, Budgets: big})
		validationSetWindowRulesObserver(previous)
		if err != nil {
			t.Fatalf("%s: %v", files[i], err)
		}
		decisions += last
	}
	t.Logf("%d windows evaluated as they closed, %d waited for the root; %d decisions kept aside", evaluated, waited, decisions)
	if evaluated == 0 || decisions == 0 || 10*waited > evaluated {
		t.Fatalf("%d windows evaluated, %d waited, %d decisions", evaluated, waited, decisions)
	}
}
