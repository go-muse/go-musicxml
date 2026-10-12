package validation

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"
)

func TestRound1NavigationKnownTargetsLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		rows := make([]NavigationListeningStatement, count)
		for i := range rows {
			rows[i] = NavigationListeningStatement{Kind: NavigationListeningSync, Player: stateAbsent[string](), TimeOnly: stateKnown(strconv.Itoa(i + 1))}
			if i%2 == 0 {
				rows[i].Player = stateKnown("player")
			}
		}
		node := &factNode{closed: true, end: NodeEnd{ChildrenComplete: stateKnown(true), Order: stateKnown(OrderSource)}}
		r := &contextReader{ctx: context.Background(), s: s}
		got := navigationListeningDistinct(r, node, rows)
		if r.err != nil || got.State != EvidenceKnown || !got.Value {
			t.Fatalf("distinct=%+v %v", got, r.err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("distinctness scratch leaked")
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("known listening targets: %d -> %d steps", small, large)
	if large > 15*small {
		t.Fatalf("known targets remain quadratic: %d -> %d", small, large)
	}
}
func TestRound1NavigationPresenceComparisonIsMetered(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	_, err := navigationSameTarget(context.Background(), s, stateAbsent[string](), stateAbsent[string](), stateKnown("P"), stateAbsent[string]())
	if err != nil {
		t.Fatal(err)
	}
	if rationalUsage(t, s, BudgetRuleSteps) == 0 {
		t.Fatal("presence mismatch did uncharged work")
	}
}
func TestRound1NavigationDistinctCancellationAndQuota(t *testing.T) {
	rows := []NavigationListeningStatement{{Kind: NavigationListeningSync, Player: stateAbsent[string](), TimeOnly: stateKnown("1")}, {Kind: NavigationListeningSync, Player: stateKnown("P"), TimeOnly: stateKnown("2")}, {Kind: NavigationListeningSync, Player: stateUnknown[string]("unplaced-player"), TimeOnly: stateKnown("3")}}
	run := func(ctx context.Context, s *Session) error {
		_, err := navigationDistinctTargets(ctx, s, len(rows), stateKnown(true), func(i int) (string, Evidence[string], Evidence[string]) {
			v := rows[i]
			return string(v.Kind), v.Player, v.TimeOnly
		})
		return err
	}
	probe := rationalSession(t, Budgets{})
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	if err := run(counter, probe); err != nil {
		t.Fatal(err)
	}
	work := rationalUsage(t, probe, BudgetRuleSteps)
	probe.cleanup()
	for at := 1; at <= counter.calls; at++ {
		s := rationalSession(t, Budgets{})
		err := run(&rationalCancelContext{Context: context.Background(), at: at}, s)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll %d: %v", at, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cancellation leaked scratch")
		}
		s.cleanup()
	}
	for _, delta := range []int64{-1, 0} {
		s := rationalSession(t, Budgets{MaxRuleSteps: work + delta})
		err := run(context.Background(), s)
		if (err == nil) != (delta == 0) {
			t.Fatalf("quota %d: %v", delta, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("quota leaked scratch")
		}
		s.cleanup()
	}
}

func TestRound1NavigationInstrumentIndexLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		timeline := &timelineService{}
		for i := 0; i < count; i++ {
			timeline.State.Definitions = append(timeline.State.Definitions, instrumentDefinition{Part: NodeRef{"d", NodeID(i + 1)}, Instrument: NodeRef{"d", NodeID(count + i + 1)}, Value: stateKnown(instrumentState{Sound: stateKnown("piano"), Solo: stateAbsent[bool](), Ensemble: stateAbsent[ensembleState](), Virtual: stateAbsent[virtualInstrumentState]()})})
		}
		index, err := navigationBuildInstrumentIndex(context.Background(), s, timeline)
		if err != nil {
			t.Fatal(err)
		}
		defer index.release()
		for _, d := range timeline.State.Definitions {
			rows := []contextNavigationNode{{part: stateKnown(d.Part)}}
			initial, err := navigationAttachInstrumentUpdates(context.Background(), s, timeline, rows, index)
			if err != nil || len(initial) != 1 || initial[0].Instrument.Value != d.Instrument {
				t.Fatalf("part-index mismatch: %+v %v", initial, err)
			}
		}
		if got := rationalUsage(t, s, BudgetContextFacts); got != 0 {
			t.Fatalf("temporary projections charged %d permanent facts", got)
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("instrument part-index: %d -> %d steps", small, large)
	if large > 15*small {
		t.Fatalf("instrument extraction remains quadratic: %d -> %d", small, large)
	}
}

func TestRound1NavigationInstrumentIndexCancellationAndGauge(t *testing.T) {
	timeline := &timelineService{State: timelineState{Definitions: []instrumentDefinition{{Part: NodeRef{"d", 1}, Instrument: NodeRef{"d", 2}, Value: stateKnown(instrumentState{})}}}}
	run := func(ctx context.Context, s *Session) error {
		index, err := navigationBuildInstrumentIndex(ctx, s, timeline)
		if err != nil {
			return err
		}
		defer index.release()
		rows := []contextNavigationNode{{part: stateKnown(NodeRef{"d", 1})}}
		_, err = navigationAttachInstrumentUpdates(ctx, s, timeline, rows, index)
		return err
	}
	s := rationalSession(t, Budgets{})
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	if err := run(counter, s); err != nil {
		t.Fatal(err)
	}
	work := rationalUsage(t, s, BudgetRuleSteps)
	s.cleanup()
	for at := 1; at <= counter.calls; at++ {
		s := rationalSession(t, Budgets{})
		err := run(&rationalCancelContext{Context: context.Background(), at: at}, s)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll %d: %v", at, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("instrument scratch leak")
		}
		s.cleanup()
	}
	for _, delta := range []int64{-1, 0} {
		s := rationalSession(t, Budgets{MaxRuleSteps: work + delta})
		err := run(context.Background(), s)
		if (err == nil) != (delta == 0) {
			t.Fatalf("quota %d: %v", delta, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("quota scratch leak")
		}
		s.cleanup()
	}
}
