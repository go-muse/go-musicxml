package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestIndependentContextFinishReleasesBothLiveGauges(t *testing.T) {
	engine, err := newContextAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{100, 1000} {
		doc, err := Decode(strings.NewReader(independentTimelineScore(count, false, "1")))
		if err != nil {
			t.Fatal(err)
		}
		for _, retention := range []int64{64 << 20, 512 << 20} {
			account, err := validation.NewBudgetAccount(validation.Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: retention})
			if err != nil {
				t.Fatal(err)
			}
			session, err := engine.Begin(context.Background(), validation.RunOptions{Profile: ProfileModelContext, DocumentID: modelDocumentID, Account: account})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := walkModel(context.Background(), doc, session, account, modelDocumentID, modelWalkOptions{}); err != nil {
				t.Fatal(err)
			}
			report, finishErr := session.Finish(context.Background(), validation.Completion{InputComplete: true, Syntax: validation.SyntaxNotAssessed})
			buffered, e1 := account.Usage(validation.BudgetBufferedBytes)
			depth, e2 := account.Usage(validation.BudgetDepth)
			if e1 != nil || e2 != nil || buffered != 0 || depth != 0 {
				t.Fatalf("notes%d retention%d: buffered%d depth%d errors%v/%v", count, retention, buffered, depth, e1, e2)
			}
			if retention == 512<<20 && (finishErr != nil || !report.Satisfied()) {
				t.Fatalf("expanded reference failed: %v", finishErr)
			}
			t.Logf("notes=%d retention_limit=%d complete=%t post_finish_buffered_bytes=%d post_finish_depth=%d err=%v", count, retention, report.AssessmentComplete, buffered, depth, finishErr)
		}
	}
}
