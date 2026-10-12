package validation

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestIndependentContextLargeStandaloneProjectionAdmission(t *testing.T) {
	for _, count := range []int{1, 64, 1024} {
		cause := &DecisionRef{Rule: RuleKey{"cause", 1}, Subject: NodeRef{"d", 9}}
		span := SpanContextResult{Members: make([]NodeRef, count), Included: make([]NodeRef, count), Association: Evidence[bool]{State: EvidenceInvalid, Cause: cause}}
		timing := newTimingContext("note")
		timing.Figures = make([]NodeRef, count)
		timing.Bends = make([]TimingBend, count)
		for i := 0; i < count; i++ {
			span.Members[i] = NodeRef{"d", NodeID(i + 1)}
			span.Included[i] = span.Members[i]
			timing.Figures[i] = span.Members[i]
			timing.Bends[i] = TimingBend{Bend: span.Members[i], ReleasePresent: Evidence[bool]{State: EvidenceInvalid, Cause: cause}}
		}
		for _, payload := range []any{span, timing} {
			probe := rationalSession(t, Budgets{MaxBufferedBytes: 512 << 20})
			n, err := stateMeasure(context.Background(), probe, reflect.ValueOf(payload))
			probe.cleanup()
			if err != nil {
				t.Fatal(err)
			}
			for _, allowance := range []int64{n - 1, n} {
				s := rationalSession(t, Budgets{MaxBufferedBytes: allowance})
				var gotSpan *SpanContextResult
				var gotTiming *TimingContextResult
				switch p := payload.(type) {
				case SpanContextResult:
					gotSpan, err = cloneSpanContext(context.Background(), s, &p)
				case TimingContextResult:
					gotTiming, err = cloneTimingContext(context.Background(), s, &p)
				}
				if allowance < n {
					if !errors.Is(err, ErrResourceLimit) || s.retained != 0 || gotSpan != nil || gotTiming != nil {
						t.Fatalf("%T count%d one-under: err=%v retained=%d", payload, count, err, s.retained)
					}
				} else {
					if err != nil || s.retained != n {
						t.Fatalf("%T count%d exact admission: err=%v retained=%d want=%d", payload, count, err, s.retained, n)
					}
					if gotSpan != nil {
						if cap(gotSpan.Members) != count || cap(gotSpan.Included) != count || gotSpan.Association.Cause == cause {
							t.Fatal("span output has excess capacity or causal alias")
						}
					}
					if gotTiming != nil {
						if cap(gotTiming.Figures) != count || cap(gotTiming.Bends) != count || gotTiming.Bends[0].ReleasePresent.Cause == cause {
							t.Fatal("timing output has excess capacity or causal alias")
						}
					}
				}
				s.cleanup()
				if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
					t.Fatal("large standalone clone cleanup leaked")
				}
			}
			t.Logf("%T count=%d exact admitted bytes=%d", payload, count, n)
		}
	}
}
