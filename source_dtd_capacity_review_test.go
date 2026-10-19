package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourceOriginReviewOfficialCapacity(t *testing.T) {
	data, err := os.ReadFile("testdata/official/MozartTrio.musicxml")
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int64{0, 16000000} {
		for _, profile := range []validation.ProfileID{ProfileSourceXSDPreview, ProfileSourceInternalDTD} {
			limits := validation.Budgets{MaxBufferedBytes: limit}
			run, err := newSourceAssessmentRun(context.Background(), bytes.NewReader(data), DecodeOptions{}, &AssessmentOptions{Profile: profile, Budgets: limits}, false)
			if err != nil {
				t.Fatal(err)
			}
			cause := run.drain()
			beforeFinish, _ := run.account.Usage(validation.BudgetBufferedBytes)
			nodes, _ := run.account.Usage(validation.BudgetNodes)
			attrs, _ := run.account.Usage(validation.BudgetAttributes)
			steps, _ := run.account.Usage(validation.BudgetRuleSteps)
			report, err := run.finish(cause)
			afterFinish, _ := run.account.Usage(validation.BudgetBufferedBytes)
			t.Logf("profile=%s max_buffered=%d input_bytes=%d sha256=%x before_finish=%d after_finish=%d nodes=%d attrs=%d origins=%d steps_before_finish=%d input_complete=%v assessment_complete=%v syntax=%s satisfied=%v resource_refusal=%v err=%v", profile, run.account.Limits().MaxBufferedBytes, len(data), sha256.Sum256(data), beforeFinish, afterFinish, nodes, attrs, len(report.Context), steps, report.InputComplete, report.AssessmentComplete, report.Syntax, report.Satisfied(), errors.Is(err, validation.ErrResourceLimit), err)
			switch {
			case profile == ProfileSourceXSDPreview:
				// The preview does not read DTDs; its external subset stays unread.
				if err != nil || !report.InputComplete || report.AssessmentComplete || report.Satisfied() {
					t.Fatalf("bounded official input must complete with unread-DTD assessment evidence: err=%v report=%+v", err, report)
				}
			case limit == 0:
				// The internal-DTD profile reads the bundled external subset and
				// completes within the default budgets.
				if err != nil || !report.InputComplete || !report.AssessmentComplete || !report.Satisfied() {
					t.Fatalf("bundled external DTD assessment: err=%v report=%+v", err, report)
				}
			default:
				// Its larger live storage exceeds the reduced buffer and is
				// refused as a resource limit, never accepted.
				if !errors.Is(err, validation.ErrResourceLimit) || report.AssessmentComplete || report.Satisfied() {
					t.Fatalf("reduced-capacity refusal: err=%v report=%+v", err, report)
				}
			}
			if afterFinish != 0 {
				t.Errorf("live bytes leaked: %d", afterFinish)
			}
		}
	}
}
