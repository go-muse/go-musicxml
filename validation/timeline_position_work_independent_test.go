package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Published positions own separately cloned strings. Equal canonical IDs can
// therefore require a variable-length comparison even before exact arithmetic.
func TestIndependentTimelinePositionIdentityWorkBeforeComparison(t *testing.T) {
	for _, partial := range []bool{false, true} {
		s := rationalSession(t, Budgets{MaxRuleSteps: 32})
		a, b := statePosition(t, 0, 0, 1), statePosition(t, 1, 0, 1)
		doc := strings.Repeat("canonical-document-", 1024)
		a.Part.DocumentID = strings.Clone(doc)
		a.Measure.DocumentID = a.Part.DocumentID
		b.Part.DocumentID = strings.Clone(doc)
		b.Measure.DocumentID = b.Part.DocumentID
		var err error
		if partial {
			_, err = (&timelineState{}).positionOrder(context.Background(), s, a, b)
		} else {
			_, err = stateComparePosition(context.Background(), s, a, b)
		}
		if !errors.Is(err, ErrResourceLimit) {
			t.Errorf("partial%t compared two%d-byte independently owned IDs with%d work: %v", partial, len(doc), rationalUsage(t, s, BudgetRuleSteps), err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("identity comparison retained scratch")
		}
		s.cleanup()
	}
}
