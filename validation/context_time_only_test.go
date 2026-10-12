package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestContextTimeOnlyExactOrder(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  Outcome
	}{
		{"1", OutcomePass}, {"1, 2,10", OutcomePass}, {"2,10", OutcomePass},
		{"1,10,2", OutcomeFail}, {"10,2", OutcomeFail},
		{"18446744073709551616,18446744073709551617", OutcomePass},
		{"18446744073709551617,18446744073709551616", OutcomeFail},
		{"1,1", OutcomeUnknown}, {"1,1,2", OutcomeUnknown}, {"2,2,1", OutcomeFail},
		{strings.Repeat("9", 4096) + ",1", OutcomeFail},
	} {
		t.Run(tc.value[:min(50, len(tc.value))], func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			got, err := contextTimeOnlyOrder(context.Background(), s, tc.value)
			if err != nil || got.Outcome != tc.want {
				t.Fatalf("got %+v, %v; want %s", got, err, tc.want)
			}
		})
	}
}

func TestContextTimeOnlyRejectsUnassessedGrammarWithoutNewNormativeFailure(t *testing.T) {
	for _, text := range []string{"", "0", "01", "1,,2", "1,", "1,  2", "1,\t2", "2,1x", "-1", "1.0"} {
		s := rationalSession(t, Budgets{})
		got, err := contextTimeOnlyOrder(context.Background(), s, text)
		s.cleanup()
		if err != nil || got.Outcome != OutcomeUnsupported {
			t.Errorf("%q got %+v,%v", text, got, err)
		}
	}
}

func TestContextTimeOnlySharedWorkAndCancellation(t *testing.T) {
	s := rationalSession(t, Budgets{MaxRuleSteps: 1})
	defer s.cleanup()
	got, err := contextTimeOnlyOrder(context.Background(), s, "10,2")
	if err == nil || got.Outcome == OutcomeFail {
		t.Fatalf("quota manufactured result: %+v,%v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s2 := rationalSession(t, Budgets{})
	defer s2.cleanup()
	got, err = contextTimeOnlyOrder(ctx, s2, "1")
	if !errors.Is(err, context.Canceled) || got.Outcome == OutcomePass {
		t.Fatalf("cancellation=%+v,%v", got, err)
	}
}

func TestContextTimeOnlyCancellationAtEveryScanBoundary(t *testing.T) {
	text := strings.Repeat("8", 4096) + "," + strings.Repeat("9", 4096)
	baseline := rationalSession(t, Budgets{})
	defer baseline.cleanup()
	count := &rationalCancelContext{Context: context.Background(), at: 1000000}
	if got, err := contextTimeOnlyOrder(count, baseline, text); err != nil || got.Outcome != OutcomePass {
		t.Fatalf("baseline=%+v,%v", got, err)
	}
	for at := 1; at <= count.calls; at++ {
		s := rationalSession(t, Budgets{})
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		got, err := contextTimeOnlyOrder(ctx, s, text)
		if !errors.Is(err, context.Canceled) || got.Outcome == OutcomePass {
			t.Fatalf("boundary%d: %+v,%v", at, got, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("boundary%d retained scratch", at)
		}
		s.cleanup()
	}
}

func TestContextTimeOnlyExactWorkBoundary(t *testing.T) {
	text := "2,10,100"
	for _, delta := range []int64{-1, 0} {
		s := rationalSession(t, Budgets{MaxRuleSteps: int64(3*len(text)) + delta})
		defer s.cleanup()
		got, err := contextTimeOnlyOrder(context.Background(), s, text)
		if delta == 0 {
			if err != nil || got.Outcome != OutcomePass {
				t.Fatalf("exact=%+v,%v", got, err)
			}
		} else if err == nil {
			t.Fatal("one-below quota admitted")
		}
	}
}
