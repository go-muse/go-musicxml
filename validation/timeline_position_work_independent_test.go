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

func TestTimelineStateIdentityAdmissionWorkBeforeEarlyMismatch(t *testing.T) {
	for _, mode := range []string{"closure", "publication", "gap-bound", "query", "positioned-gap", "unresolved-instrument"} {
		t.Run(mode, func(t *testing.T) {
			s := rationalSession(t, Budgets{MaxRuleSteps: 32})
			defer s.cleanup()
			ctx := context.Background()
			doc := strings.Repeat("canonical-document-", 1024)
			at := statePosition(t, 0, 0, 1)
			at.Part.DocumentID = strings.Clone(doc) + "a"
			at.Measure.DocumentID = strings.Clone(doc) + "b"
			h := &timelineState{}
			var err error
			switch mode {
			case "closure":
				h.Closures = []stateClosure{{Part: at.Measure, Through: 1, Complete: stateKnown(true)}}
				_, err = h.closure(ctx, s, at)
			case "publication":
				err = publishState(ctx, s, &h.Divisions, statePublication[Decimal]{Position: at, Scope: stateScope{AllStaves: true}, Value: stateKnown(stateDecimal(t, 1, 1))})
			case "gap-bound":
				err = addStateGap(ctx, s, &h.Divisions, stateGap{Part: at.Part, Scope: stateScope{AllStaves: true}, Availability: stateUnknown[bool]("unknown"), Placement: stateGapPlacement{Displaced: true, Upper: &at}})
			case "query":
				_, err = h.effectiveDivisions(ctx, s, at)
			case "positioned-gap":
				_, err = h.gapCouldApply(ctx, s, stateGap{Part: at.Part, Position: &at}, at, true)
			case "unresolved-instrument":
				err = h.addUnresolvedInstrumentChange(ctx, s, instrumentRecord[instrumentState]{Part: at.Part, Position: stateKnown(at), Instrument: stateUnknown[NodeRef]("unknown")}, 0)
			}
			if !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("unmetered variable comparison returned after%d steps: %v", rationalUsage(t, s, BudgetRuleSteps), err)
			}
		})
	}
}

func TestIndependentTimelineClosureAndGraceIdentityWork(t *testing.T) {
	for _, kind := range []string{"closure", "grace"} {
		s := rationalSession(t, Budgets{MaxRuleSteps: 32})
		a, b := statePosition(t, 0, 0, 1), statePosition(t, 1, 0, 1)
		doc := strings.Repeat("canonical-document-", 1024)
		a.Part.DocumentID = strings.Clone(doc)
		a.Measure.DocumentID = a.Part.DocumentID
		b.Part.DocumentID = strings.Clone(doc)
		b.Measure.DocumentID = b.Part.DocumentID
		var err error
		if kind == "closure" {
			h := &timelineState{Closures: []stateClosure{{Part: a.Part, Through: 2, Complete: stateKnown(true)}}}
			_, err = h.closure(context.Background(), s, b)
		} else {
			_, err = s.gracePositionOrder(context.Background(), a, b)
		}
		if !errors.Is(err, ErrResourceLimit) {
			t.Errorf("%s compared%d-byte IDs with%d work: %v", kind, len(doc), rationalUsage(t, s, BudgetRuleSteps), err)
		}
		s.cleanup()
	}
}
