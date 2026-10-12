package validation

import (
	"context"
	"errors"
	"testing"
)

func TestIndependentTimelineFactOwnsPlacementEndpoints(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	v := &timelineService{Facts: map[NodeID]timelineEventFact{}}
	lower := statePosition(t, 0, 1, 3)
	upper := statePosition(t, 0, 2, 3)
	fact := timelineEventFact{Placement: stateGapPlacement{Displaced: true, Lower: &lower, Upper: &upper}}
	for id := NodeID(1); id <= 2; id++ {
		if err := v.storeFact(context.Background(), s, id, fact); err != nil {
			t.Fatal(err)
		}
	}
	lower.Offset = mustRational(t, 100, 1)
	upper.Offset = mustRational(t, 200, 1)
	for _, stored := range v.Facts {
		if stored.Placement.Lower == &lower || stored.Placement.Upper == &upper || stored.Placement.Lower.Offset.String() != "1/3" || stored.Placement.Upper.Offset.String() != "2/3" {
			t.Fatal("event fact retained borrowed endpoint record")
		}
	}
	first, second := v.Facts[1], v.Facts[2]
	first.Placement.Lower.Offset = mustRational(t, 999, 1)
	if second.Placement.Lower.Offset.String() != "1/3" {
		t.Fatal("event facts share mutable endpoint record")
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != 2*(timelineFactReserveBytes+1024) {
		t.Fatal("endpoint copies missing fixed admissions or scratch leaked")
	}
	if rationalUsage(t, s, BudgetContextFacts) != 2 {
		t.Fatal("fact context quota bypass")
	}
}

func TestIndependentTimelineFactPlacementFixedPeak(t *testing.T) {
	for _, short := range []bool{false, true} {
		peak := 2*timelineFactReserveBytes + 1024
		if short {
			peak--
		}
		s := rationalSession(t, Budgets{MaxBufferedBytes: peak})
		v := &timelineService{Facts: map[NodeID]timelineEventFact{}}
		at := statePosition(t, 0, 1, 3)
		err := v.storeFact(context.Background(), s, 1, timelineEventFact{Placement: stateGapPlacement{Displaced: true, Lower: &at, Upper: &at}})
		if short {
			if !errors.Is(err, ErrResourceLimit) || len(v.Facts) != 0 {
				t.Fatalf("placement peak-minus1 published: %v", err)
			}
		} else {
			if err != nil || len(v.Facts) != 1 || rationalUsage(t, s, BudgetBufferedBytes) != timelineFactReserveBytes+1024 {
				t.Fatalf("placement exact peak rejected/leaked: %v", err)
			}
			if v.Facts[1].Placement.Lower == v.Facts[1].Placement.Upper {
				t.Fatal("separate owned endpoints preserve borrowed pointer alias")
			}
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("fact temporary scratch leaked")
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup leaked")
		}
	}
}

func TestIndependentTimelineFactPlacementEveryCancellationBoundary(t *testing.T) {
	at := statePosition(t, 0, 1, 3)
	fact := timelineEventFact{Placement: stateGapPlacement{Displaced: true, Lower: &at, Upper: &at}}
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	s := rationalSession(t, Budgets{})
	v := &timelineService{Facts: map[NodeID]timelineEventFact{}}
	if err := v.storeFact(probe, s, 1, fact); err != nil {
		t.Fatal(err)
	}
	s.cleanup()
	for poll := 1; poll <= probe.calls; poll++ {
		s := rationalSession(t, Budgets{})
		v := &timelineService{Facts: map[NodeID]timelineEventFact{}}
		err := v.storeFact(&rationalCancelContext{Context: context.Background(), at: poll}, s, 1, fact)
		if !errors.Is(err, context.Canceled) || len(v.Facts) != 0 || at.Offset.String() != "1/3" {
			t.Fatalf("poll%d published/mutated or lost cancellation: %v", poll, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatalf("poll%d scratch leaked", poll)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup leaked")
		}
	}
	t.Logf("all%d placement publication cancellation polls", probe.calls)
}
