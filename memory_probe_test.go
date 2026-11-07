package musicxml

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// scaledScore repeats every part's measures factor times, renumbering the
// copies after the original measures, so the document grows by measures as a
// longer score would.
func scaledScore(t testing.TB, raw []byte, factor int) []byte {
	t.Helper()
	if factor == 1 {
		return raw
	}
	part := regexp.MustCompile(`(?s)(<part id="[^"]*">)(.*?)(\s*</part>)`)
	measure := regexp.MustCompile(`(?s)<measure\s[^>]*>.*?</measure>`)
	number := regexp.MustCompile(`number="[^"]*"`)
	out := part.ReplaceAllFunc(raw, func(block []byte) []byte {
		m := part.FindSubmatch(block)
		measures := measure.FindAll(m[2], -1)
		var body bytes.Buffer
		body.Write(m[2])
		next := len(measures)
		for copy := 1; copy < factor; copy++ {
			for _, one := range measures {
				// Only the measure's own number changes; an implicit pickup
				// measure keeps its attribute.
				first := number.FindIndex(one)
				body.WriteString("\n    ")
				body.Write(one[:first[0]])
				fmt.Fprintf(&body, `number="%d"`, next)
				body.Write(one[first[1]:])
				next++
			}
		}
		return append(append(append([]byte{}, m[1]...), body.Bytes()...), m[3]...)
	})
	if len(out) <= len(raw) {
		t.Fatal("scaling did not grow the score")
	}
	return out
}

// TestMemoryPerElement measures the live session heap per element at the end
// of Finish for validation lines 2 and 3 under both strategies on MozartTrio
// scaled up. It is a
// measurement, not a check: it runs only with MUSICXML_MEMORY_PROBE=1 and
// prints a table for the wave 3 design notes.
func TestMemoryPerElement(t *testing.T) {
	if os.Getenv("MUSICXML_MEMORY_PROBE") == "" {
		t.Skip("set MUSICXML_MEMORY_PROBE=1 to measure")
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "official", "MozartTrio.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	source := reviewBudgetRemoveDOCTYPE(t, raw)
	unlimited := validation.Budgets{MaxRuleSteps: 1 << 50, MaxBufferedBytes: 1 << 40, MaxInputBytes: 1 << 30, MaxNodes: 1 << 30, MaxEvents: 1 << 32, MaxAttributes: 1 << 32, MaxContextFacts: 1 << 30}
	heap := func() int64 {
		runtime.GC()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return int64(m.HeapAlloc)
	}
	fmt.Printf("| Input | Bytes | Elements | Line | Strategy | Live at end of Finish | Per element | Per input byte | Charged |\n")
	for _, factor := range []int{1, 4, 16, 110} {
		data := scaledScore(t, source, factor)
		document, err := Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		elements := int64(strings.Count(string(data), "<") - strings.Count(string(data), "</") - strings.Count(string(data), "<?") - strings.Count(string(data), "<!"))
		for _, line := range []string{"2 source", "3 model"} {
			for _, windowed := range []bool{false, true} {
				previousStrategy := validationSetWindowedStrategy(windowed)
				var live, charged int64
				for repeat := 0; repeat < 2; repeat++ { // the first run warms process caches
					before := heap()
					previous := validationSetFinishObserver(func(stage string, a *validation.Account) {
						if stage == "finished" {
							live = heap() - before
							charged, _ = a.Usage(validation.BudgetBufferedBytes)
						}
					})
					if line == "2 source" {
						_, err = InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceLine, Budgets: unlimited})
					} else {
						_, err = AssessModel(context.Background(), document, ModelOptions{Profile: ProfileModelSemantics, Budgets: unlimited})
					}
					validationSetFinishObserver(previous)
					if err != nil {
						t.Fatalf("x%d line %s: %v", factor, line, err)
					}
				}
				validationSetWindowedStrategy(previousStrategy)
				strategy := "buffered"
				if windowed {
					strategy = "windowed"
				}
				fmt.Printf("| MozartTrio x%d | %d | %d | %s | %s | %.1f MB | %d B | %d B | %.1f MB |\n", factor, len(data), elements, line, strategy, float64(live)/1e6, live/elements, live/int64(len(data)), float64(charged)/1e6)
			}
		}
	}
}

// TestWindowDegradationOnCorpus counts the documents and parts where a
// per-measure window could not prove its decisions final and would buffer the
// part instead: timewise scores, whose part ends only with the document, and
// parts whose divisions change after their first value, which makes earlier
// excursions depend on later measures. It is a measurement for the wave 3
// design notes and runs only with MUSICXML_MEMORY_PROBE=1.
func TestWindowDegradationOnCorpus(t *testing.T) {
	if os.Getenv("MUSICXML_MEMORY_PROBE") == "" {
		t.Skip("set MUSICXML_MEMORY_PROBE=1 to measure")
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
	timewise := regexp.MustCompile(`<score-timewise\b`)
	part := regexp.MustCompile(`(?s)<part id="[^"]*">(.*?)</part>`)
	divisions := regexp.MustCompile(`<divisions>\s*([^<\s]+)\s*</divisions>`)
	var documents, degradedDocuments, timewiseDocuments, parts, changingParts int
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.HasPrefix(data, []byte{0xfe, 0xff}) || bytes.HasPrefix(data, []byte{0xff, 0xfe}) {
			continue // UTF-16 samples are counted through their UTF-8 siblings
		}
		documents++
		degraded := false
		if timewise.Match(data) {
			timewiseDocuments++
			degraded = true
		}
		for _, p := range part.FindAllSubmatch(data, -1) {
			parts++
			values := map[string]bool{}
			for _, d := range divisions.FindAllSubmatch(p[1], -1) {
				values[string(d[1])] = true
			}
			if len(values) > 1 {
				changingParts++
				degraded = true
			}
		}
		if degraded {
			degradedDocuments++
		}
	}
	fmt.Printf("documents=%d degraded=%d timewise=%d parts=%d parts-with-changing-divisions=%d\n", documents, degradedDocuments, timewiseDocuments, parts, changingParts)
}
