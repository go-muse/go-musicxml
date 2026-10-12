package musicxml

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Same-head incremental measurements include all selected context projections.
// Deliberately larger finite budgets isolate throughput from safety defaults.
// This does not remeasure or close the batch-9 B5-PERF-01 kernel obligation.
func BenchmarkContextProfiles(b *testing.B) {
	for _, count := range []int{100, 1000} {
		source := independentTimelineScore(count, false, "1")
		doc, err := Decode(strings.NewReader(source))
		if err != nil {
			b.Fatal(err)
		}
		for _, model := range []bool{false, true} {
			for _, extended := range []bool{false, true} {
				b.Run(fmt.Sprintf("notes%d/model%t/context%t", count, model, extended), func(b *testing.B) {
					profile := ProfileSourceTimeline
					if model {
						profile = ProfileModelTimeline
					}
					if extended {
						profile = ProfileSourceContext
						if model {
							profile = ProfileModelContext
						}
					}
					opts := AssessmentOptions{Profile: profile, Budgets: validation.Budgets{MaxBufferedBytes: 512 << 20, MaxRuleSteps: 1 << 34}}
					run := func() {
						var r validation.Report
						var err error
						if model {
							r, err = AssessModel(context.Background(), doc, opts)
						} else {
							r, err = InspectXML(context.Background(), strings.NewReader(source), opts)
						}
						if err != nil || !r.Satisfied() {
							b.Fatalf("%v; %s", err, localFindings(r))
						}
						if extended && len(r.Context) == 0 {
							b.Fatal("missing projection")
						}
					}
					run()
					b.ReportAllocs()
					b.ResetTimer()
					for b.Loop() {
						run()
					}
				})
			}
		}
	}
}
