package validation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestResourceReviewMeasurementAvoidsPerFieldBoxing(t *testing.T) {
	s := snapshotTestSession(t, "bad")
	report, err := s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	value := reflect.ValueOf(&report).Elem()
	a, err := NewBudgetAccount(Budgets{MaxRuleSteps: 1 << 40})
	if err != nil {
		t.Fatal(err)
	}
	allocations := testing.AllocsPerRun(10, func() {
		if _, err := resourceMeasure(context.Background(), a, value); err != nil {
			panic(err)
		}
	})
	t.Logf("measurement allocations: %.0f", allocations)
	if allocations != 0 {
		t.Errorf("measurement boxed closed report fields: %.0f allocations", allocations)
	}
}

func TestResourceReviewContiguousByteMeasurementAndOwnership(t *testing.T) {
	a, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{0, 1, 64, 4096, 65536} {
		raw := []byte(strings.Repeat("x", n))
		got, err := resourceMeasure(context.Background(), a, reflect.ValueOf(raw))
		if err != nil {
			t.Fatal(err)
		}
		if got != int64(32+n) {
			t.Errorf("len=%d charged=%d, want=%d", n, got, 32+n)
		}
		copy, err := resourceLiveCopy(context.Background(), a, raw, "", "")
		if err != nil {
			t.Fatal(err)
		}
		clone := stateClone(raw)
		if n > 0 {
			copy[0], clone[n-1] = 'y', 'z'
			if raw[0] != 'x' || raw[n-1] != 'x' {
				t.Fatal("byte copy aliases caller storage")
			}
		}
	}
	if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("scratch leaked %d", used)
	}
}

func TestResourceReviewStringWorkIsChunked(t *testing.T) {
	a, err := NewBudgetAccount(Budgets{MaxRuleSteps: 4096})
	if err != nil {
		t.Fatal(err)
	}
	value := strings.Repeat("x", 1<<20)
	got, err := resourceMeasure(context.Background(), a, reflect.ValueOf(value))
	if err != nil {
		t.Fatalf("metadata-length measurement charged a step per retained string byte: %v", err)
	}
	if got != int64(32+len(value)) {
		t.Fatalf("size=%d", got)
	}
}

func TestResourceReviewTransferredSnapshotReason(t *testing.T) {
	s := snapshotTestSession(t, "7")
	_, snap, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	defer snap.release()
	snap.owner = &resourceLeaseOwner{}
	for _, read := range []func() error{
		func() error { _, err := snap.Report(context.Background()); return err },
		func() error { _, _, err := snap.ContextAt(context.Background(), 0); return err },
		func() error { _, _, err := snap.IDAt(context.Background(), 0); return err },
	} {
		var got *SessionError
		if err := read(); !errors.As(err, &got) || got.Reason != "ownership transferred to package" {
			t.Errorf("transferred access=%v", err)
		}
	}
}

func TestResourceReviewByteAndStringCopyCancellationBoundaries(t *testing.T) {
	for _, value := range []any{[]byte(strings.Repeat("x", 3*resourceCopyChunk)), strings.Repeat("x", 3*resourceCopyChunk)} {
		probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
		_, err := resourceCopyAs(probe, reflect.ValueOf(value), "", "")
		if err != nil {
			t.Fatal(err)
		}
		if probe.calls < 4 {
			t.Fatalf("large contiguous payload has no bounded cancellation checks: %d", probe.calls)
		}
		for at := 1; at <= probe.calls; at++ {
			a, err := NewBudgetAccount(Budgets{})
			if err != nil {
				t.Fatal(err)
			}
			ctx := &rationalCancelContext{Context: context.Background(), at: at}
			// Include measurement, admission, copying and the post-copy check.
			_, err = resourceLiveCopy(ctx, a, value, "", "")
			if !errors.Is(err, context.Canceled) {
				t.Errorf("boundary %d: %v", at, err)
			}
			if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
				t.Errorf("boundary %d leaked %d", at, used)
			}
		}
	}
}

func TestResourceReviewNumericMeasurementIsAllocationFree(t *testing.T) {
	value := struct {
		Integer  Integer
		Decimal  Decimal
		Rational Rational
	}{}
	v := reflect.ValueOf(&value).Elem()
	a, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	allocations := testing.AllocsPerRun(10, func() {
		got, err := resourceMeasure(context.Background(), a, v)
		if err != nil || got != 32+3*64 {
			panic("incorrect immutable numeric measurement")
		}
	})
	if allocations != 0 {
		t.Errorf("numeric wrappers boxed: %f", allocations)
	}
}

func TestResourceReviewSnapshotRefusalIsExplicitTerminalResult(t *testing.T) {
	ordinary := snapshotTestSession(t, "7")
	expected, err := ordinary.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil || !expected.Satisfied() {
		t.Fatalf("ordinary report=%+v err=%v", expected, err)
	}
	s := snapshotTestSession(t, "7")
	used, _ := s.account.Usage(BudgetBufferedBytes)
	hold := s.account.Limits().MaxBufferedBytes - used
	if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
		t.Fatal(err)
	}
	report, snap, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	_ = s.account.Release(BudgetBufferedBytes, hold)
	if !errors.Is(err, ErrResourceLimit) || snap != nil || report.Counts != expected.Counts || report.AssessmentComplete || report.Conformance != ConformanceUndetermined || report.Terminal == nil || report.BlockingProblems != expected.BlockingProblems+1 {
		t.Fatalf("refusal report=%+v snapshot=%v err=%v", report, snap, err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Errorf("refusal leaked %d", used)
	}
}
