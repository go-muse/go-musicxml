package validation

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestDictionaryComparisonWorkWidths(t *testing.T) {
	for _, tc := range []struct {
		name              string
		left, right, want int64
	}{
		{"near386", 2147483630, 44, 2147483675},
		{"small", 7, 11, 19},
		{"empty", 0, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := dictionaryComparisonWork(tc.left, tc.right)
			if err != nil || got != tc.want {
				t.Fatalf("work=%d err=%v want=%d", got, err, tc.want)
			}
		})
	}
	for _, pair := range [][2]int64{{math.MaxInt64, 0}, {math.MaxInt64 - 1, 1}, {math.MaxInt64, math.MaxInt64}} {
		if _, err := dictionaryComparisonWork(pair[0], pair[1]); !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("%v: %v", pair, err)
		}
	}
}

func TestDictionaryComparisonCancellationPrecedesOverflow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	if err := dictionaryComparisonStep(ctx, a, math.MaxInt64, 1); !errors.Is(err, context.Canceled) || errors.Is(err, ErrResourceLimit) {
		t.Fatal(err)
	}
}
