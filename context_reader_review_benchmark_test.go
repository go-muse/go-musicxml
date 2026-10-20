package musicxml

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Real official input complements the two synthetic sizes. These diagnostics
// use explicit finite budgets and do not change defaults or close B5-PERF-01.
func BenchmarkContextReaderReview(b *testing.B) {
	mozart, err := os.ReadFile("testdata/official/MozartTrio.musicxml")
	if err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct{ name, source string }{{"MozartTrio", string(mozart)}, {"notes100", independentTimelineScore(100, false, "1")}, {"notes1000", independentTimelineScore(1000, false, "1")}} {
		b.Run(tc.name, func(b *testing.B) {
			doc, err := Decode(strings.NewReader(tc.source))
			if err != nil {
				b.Fatal(err)
			}
			engine, err := newContextAssessmentEngine()
			if err != nil {
				b.Fatal(err)
			}
			run := func() {
				account, e := validation.NewBudgetAccount(validation.Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
				if e != nil {
					b.Fatal(e)
				}
				session, e := engine.Begin(context.Background(), validation.RunOptions{Profile: ProfileModelContext.ID(), DocumentID: modelDocumentID, Account: account})
				if e != nil {
					b.Fatal(e)
				}
				_, e = walkModel(context.Background(), doc, session, account, modelDocumentID, modelWalkOptions{})
				if e != nil {
					b.Fatal(e)
				}
				r, e := session.Finish(context.Background(), validation.Completion{InputComplete: true, Syntax: validation.SyntaxNotAssessed})
				if e != nil {
					b.Fatal(e)
				}
				steps, _ := account.Usage(validation.BudgetRuleSteps)
				b.ReportMetric(float64(steps), "rule-steps/op")
				b.ReportMetric(float64(len(r.Context)), "records/op")
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
