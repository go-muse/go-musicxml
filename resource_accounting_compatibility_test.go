package musicxml

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// These explicit limits completed on published d336f089. Retained-container
// growth now meters real copying work, so they are historical tuned limits,
// not promises that the same allowance always completes after an update.
func TestReviewRetainedGrowthTunedBudgetCompatibility(t *testing.T) {
	raw, err := os.ReadFile("testdata/corpus/musicxml-test-suite/01a-Pitches-Pitches.xml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	var before bytes.Buffer
	if err := Encode(&before, doc); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		steps  int64
		assess func(validation.Budgets) (validation.Report, error)
	}{
		{"model", 953719, func(b validation.Budgets) (validation.Report, error) {
			return AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelXSD, Budgets: b})
		}},
		{"source", 556569, func(b validation.Budgets) (validation.Report, error) {
			return InspectXML(context.Background(), strings.NewReader(reviewBudgetScore(100)), AssessmentOptions{Profile: ProfileSourceXSDPreview, Budgets: b})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := tc.assess(validation.Budgets{MaxRuleSteps: tc.steps, MaxBufferedBytes: 1 << 30})
			var limit *validation.ResourceError
			if !errors.As(err, &limit) || limit.Kind != validation.BudgetRuleSteps || limit.Limit != tc.steps || limit.Used < 0 || limit.Used > tc.steps || limit.Requested <= tc.steps-limit.Used || r.AssessmentComplete || r.Satisfied() {
				t.Fatalf("historical tuned budget: report=%+v error=%v", r, err)
			}
			var after bytes.Buffer
			if err := Encode(&after, doc); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Bytes(), after.Bytes()) {
				t.Fatal("assessment refusal mutated the caller model")
			}
			t.Logf("historical limit refusal: %v", limit)
			r, err = tc.assess(validation.Budgets{MaxBufferedBytes: 1 << 30})
			if err != nil || !r.InputComplete || !r.AssessmentComplete || !r.Satisfied() {
				t.Fatalf("default work control: complete=%v input=%v satisfied=%v err=%v", r.AssessmentComplete, r.InputComplete, r.Satisfied(), err)
			}
		})
	}
}
