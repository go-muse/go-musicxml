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
					profile := ProfileSourceTimeline.ID()
					if model {
						profile = ProfileModelTimeline.ID()
					}
					if extended {
						profile = ProfileSourceContext.ID()
						if model {
							profile = ProfileModelContext.ID()
						}
					}
					budgets := validation.Budgets{MaxBufferedBytes: 512 << 20, MaxRuleSteps: 1 << 34}
					run := func() {
						var r validation.Report
						var err error
						if model {
							r, err = AssessModel(context.Background(), doc, ModelOptions{Profile: ModelProfile(profile), Budgets: budgets})
						} else {
							r, err = InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: SourceProfile(profile), Budgets: budgets})
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
