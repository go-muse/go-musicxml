package validation

import (
	"context"
	"errors"
	"testing"
	"unsafe"
)

func TestIndependentTimelinePendingExactScratch(t *testing.T) {
	if timelinePendingOffsetBytes < int64(unsafe.Sizeof(timelinePendingOffset{})) {
		t.Fatal("pending allowance is smaller than its fixed element")
	}
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	b := stateBindings()
	block := runtimeBlock(b, 0, stateNode(b, timelineRoleNote, 10, stateAbsent[Atomic]()), stateNode(b, timelineRoleHarmony, 20, stateAbsent[Atomic]()), stateNode(b, timelineRoleDirection, 30, stateAbsent[Atomic]()))
	before, retained := rationalUsage(t, s, BudgetBufferedBytes), s.retained
	r := &stateReader{ctx: context.Background(), s: s, b: b}
	pending, release, err := newTimelinePendingOffsets(r, block)
	if err != nil || len(pending) != 0 || cap(pending) != 2 {
		t.Fatalf("pending=%d/%d err=%v", len(pending), cap(pending), err)
	}
	if got := rationalUsage(t, s, BudgetBufferedBytes) - before; got != 2*timelinePendingOffsetBytes || s.retained != retained {
		t.Fatalf("scratch charge=%d session retained delta=%d", got, s.retained-retained)
	}
	release()
	if rationalUsage(t, s, BudgetBufferedBytes) != before {
		t.Fatal("pending scratch was not released")
	}
}

func TestIndependentTimelinePendingNoOffsetsAllocatesNothing(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	b := stateBindings()
	block := runtimeBlock(b, 0, stateNode(b, timelineRoleNote, 10, stateAbsent[Atomic]()))
	r := &stateReader{ctx: context.Background(), s: s, b: b}
	allocs := testing.AllocsPerRun(100, func() {
		pending, release, err := newTimelinePendingOffsets(r, block)
		if err != nil || len(pending) != 0 || cap(pending) != 0 {
			t.Fatalf("zero pending: %d/%d %v", len(pending), cap(pending), err)
		}
		release()
	})
	if allocs != 0 {
		t.Fatalf("note-only pending setup allocated %g objects; expected no scratch or release closure", allocs)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("zero pending has live scratch")
	}
}

func independentDeferredFixture(t *testing.T) (*Session, *timelineService, *timelineBindings, timelineMeasureBlock) {
	t.Helper()
	s, v, b := runtimeFixture(t)
	d2 := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 2, 1)))
	off := stateNode(b, timelineRoleDirectionOffset, 41, stateAtomicDecimal(t, 2, 1))
	stateAttribute(b, off, timelineRoleOffsetSound, Atomic{Kind: AtomicString, Text: "yes"})
	sound := stateNode(b, timelineRoleDirectionSound, 50, stateAbsent[Atomic]())
	listening := stateNode(b, timelineRoleDirectionListening, 60, stateAbsent[Atomic]())
	dir := stateNode(b, timelineRoleDirection, 40, stateAbsent[Atomic](), off, sound, listening)
	d4 := stateNode(b, timelineRoleAttributes, 70, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 71, stateAtomicDecimal(t, 4, 1)))
	return s, v, b, runtimeBlock(b, 0, d2, runtimeNote(t, b, 30, 2, false), dir, d4)
}

func TestIndependentTimelineDeferredEveryCancellationBoundary(t *testing.T) {
	s, v, b, block := independentDeferredFixture(t)
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if err := v.readMeasure(probe, s, b, block); err != nil {
		t.Fatal(err)
	}
	calls := probe.calls
	for _, id := range []NodeID{40, 50, 60} {
		if got := v.Facts[id].Position; got.State != EvidenceKnown || got.Value.Offset.String() != "3/2" {
			t.Fatalf("finalized node%d=%+v", id, got)
		}
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
		t.Fatal("successful finalization leaked scratch")
	}
	for at := 1; at <= calls; at++ {
		s, v, b, block := independentDeferredFixture(t)
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		if err := v.readMeasure(ctx, s, b, block); !errors.Is(err, context.Canceled) {
			t.Fatalf("poll%d err=%v", at, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatalf("poll%d pending scratch leak: gauge%d retained%d", at, rationalUsage(t, s, BudgetBufferedBytes), s.retained)
		}
		work := rationalUsage(t, s, BudgetRuleSteps)
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || rationalUsage(t, s, BudgetRuleSteps) != work {
			t.Fatalf("poll%d cleanup leaked or refunded cumulative work", at)
		}
	}
	t.Logf("interrupted every one of %d cursor/deferred finalization polls", calls)
}
