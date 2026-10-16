package validation

import (
	"context"
	"errors"
	"testing"
)

func dtdIDSnapshotFixture(t *testing.T) *Session {
	t.Helper()
	s, k := dtdIDSession(t)
	f := dtdIDAdmittedAttribute(t, s, k, "a", "value")
	if e := s.SourceDTDIDAttribute(context.Background(), f); e != nil {
		t.Fatal(e)
	}
	if e := s.Close(context.Background(), schemaLexemeEnd("1")); e != nil {
		t.Fatal(e)
	}
	if e := s.CompleteSourceDTDIDs(context.Background(), "source"); e != nil {
		t.Fatal(e)
	}
	return s
}
func TestSourceDTDIDSnapshotCancellation(t *testing.T) {
	s := dtdIDSnapshotFixture(t)
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	snap, e := s.makeResourceSnapshot(probe, Report{})
	if e != nil {
		t.Fatal(e)
	}
	snap.Close()
	s.cleanup()
	for at := 1; at <= probe.calls; at++ {
		s := dtdIDSnapshotFixture(t)
		snap, e := s.makeResourceSnapshot(&rationalCancelContext{Context: context.Background(), at: at}, Report{})
		if !errors.Is(e, context.Canceled) || snap != nil {
			t.Fatalf("poll %d: %v %v", at, snap, e)
		}
		s.cleanup()
		if n, _ := s.account.Usage(BudgetBufferedBytes); n != 0 {
			t.Fatalf("poll %d leaked %d", at, n)
		}
	}
	t.Logf("checked %d snapshot cancellation points", probe.calls)
}
func TestSourceDTDIDSnapshotExactBoundary(t *testing.T) {
	run := func(allowance int64) (bool, error) {
		s := dtdIDSnapshotFixture(t)
		used, _ := s.account.Usage(BudgetBufferedBytes)
		held := s.account.Limits().MaxBufferedBytes - used - allowance
		if e := s.account.Charge(BudgetBufferedBytes, held); e != nil {
			t.Fatal(e)
		}
		_, snap, e := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
		ok := snap != nil
		if snap != nil {
			snap.Close()
		}
		s.account.Release(BudgetBufferedBytes, held)
		if n, _ := s.account.Usage(BudgetBufferedBytes); n != 0 {
			t.Fatalf("leak %d", n)
		}
		return ok, e
	}
	low, high := int64(1), int64(1<<20)
	for low < high {
		mid := low + (high-low)/2
		_, e := run(mid)
		if errors.Is(e, ErrResourceLimit) {
			low = mid + 1
		} else if e != nil {
			t.Fatal(e)
		} else {
			high = mid
		}
	}
	if ok, e := run(low); !ok || e != nil {
		t.Fatalf("exact %d: %v %v", low, ok, e)
	}
	if ok, e := run(low - 1); ok || !errors.Is(e, ErrResourceLimit) {
		t.Fatalf("below %d: %v %v", low-1, ok, e)
	}
	t.Logf("exact snapshot boundary %d", low)
}
