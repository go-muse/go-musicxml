package validation

import (
	"context"
	"errors"
	"math"
	"testing"
)

func repeatMeterRosterFixture(t *testing.T) *timelineService {
	t.Helper()
	a, b := NodeRef{"first", 1}, NodeRef{"second", 1}
	service := &timelineService{}
	for _, part := range []NodeRef{a, b, a} {
		pos := statePosition(t, 0, 0, 1)
		pos.Part = part
		service.State.Times.Publications = append(service.State.Times.Publications, statePublication[timeState]{Position: pos})
	}
	for _, part := range []NodeRef{b, a, b} {
		service.State.Times.Gaps = append(service.State.Times.Gaps, stateGap{Part: part})
	}
	return service
}
func TestContextRepeatMeterRosterOrderIdentityAndReuse(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	service := repeatMeterRosterFixture(t)
	roster := &contextRepeatMeterRoster{}
	if err := roster.prepare(context.Background(), s, service); err != nil {
		t.Fatal(err)
	}
	a, b := NodeRef{"first", 1}, NodeRef{"second", 1}
	if len(roster.publications[a]) != 2 || roster.publications[a][0] != 0 || roster.publications[a][1] != 2 || len(roster.publications[b]) != 1 || roster.publications[b][0] != 1 || len(roster.gaps[b]) != 2 || roster.gaps[b][0] != 0 || roster.gaps[b][1] != 2 {
		t.Fatalf("part/document identity or source order lost %+v %+v", roster.publications, roster.gaps)
	}
	bytes := rationalUsage(t, s, BudgetBufferedBytes)
	for i := 0; i < 100; i++ {
		if err := roster.prepare(context.Background(), s, service); err != nil {
			t.Fatal(err)
		}
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != bytes {
		t.Fatal("reused roster allocated again")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := roster.prepare(ctx, s, service); !errors.Is(err, context.Canceled) {
		t.Fatalf("cached pre-cancel: %v", err)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != bytes {
		t.Fatal("cached cancellation altered scratch ownership")
	}
	roster.close()
	if err := roster.prepare(context.Background(), s, service); err != nil {
		t.Fatal(err)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != bytes {
		t.Fatal("close/reprepare admission changed")
	}
	roster.close()
	roster.close()
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("roster scratch leaked")
	}
}
func TestContextRepeatMeterRosterEveryCancellationAndAdmission(t *testing.T) {
	service := repeatMeterRosterFixture(t)
	s := rationalSession(t, Budgets{})
	roster := &contextRepeatMeterRoster{}
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	if err := roster.prepare(counter, s, service); err != nil {
		t.Fatal(err)
	}
	calls := counter.calls
	roster.close()
	s.cleanup()
	for at := 1; at <= calls; at++ {
		s := rationalSession(t, Budgets{})
		roster := &contextRepeatMeterRoster{}
		err := roster.prepare(&rationalCancelContext{Context: context.Background(), at: at}, s, service)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll%d %v", at, err)
		}
		if roster.release != nil || roster.publications != nil || roster.gaps != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("poll%d partial index or scratch leak", at)
		}
		s.cleanup()
	}
	s = rationalSession(t, Budgets{MaxBufferedBytes: 1})
	defer s.cleanup()
	roster = &contextRepeatMeterRoster{}
	if err := roster.prepare(context.Background(), s, service); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("unadmitted roster %v", err)
	}
	if roster.release != nil || roster.publications != nil || roster.gaps != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("failed roster admission allocated")
	}
	t.Logf("interrupted all %d roster construction polls", calls)
}
func TestContextRepeatMeterRosterRejectsChangedTimeline(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	service := repeatMeterRosterFixture(t)
	roster := &contextRepeatMeterRoster{}
	defer roster.close()
	if err := roster.prepare(context.Background(), s, service); err != nil {
		t.Fatal(err)
	}
	service.State.Times.Gaps = append(service.State.Times.Gaps, stateGap{})
	if err := roster.prepare(context.Background(), s, service); err == nil {
		t.Fatal("changed timeline reused stale roster")
	}
}

func TestContextRepeatMeterRosterOneUnderAdmission(t *testing.T) {
	service := repeatMeterRosterFixture(t)
	required := int64(512 + 1024*(len(service.State.Times.Publications)+len(service.State.Times.Gaps)))
	for _, limit := range []int64{required - 1, required} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		roster := &contextRepeatMeterRoster{}
		err := roster.prepare(context.Background(), s, service)
		if limit < required {
			if !errors.Is(err, ErrResourceLimit) || rationalUsage(t, s, BudgetBufferedBytes) != 0 || roster.release != nil {
				t.Fatalf("one-under admitted/leaked: %v", err)
			}
		} else if err != nil || rationalUsage(t, s, BudgetBufferedBytes) != required {
			t.Fatalf("exact admission %v", err)
		}
		roster.close()
		s.cleanup()
	}
}
func TestContextRepeatMeterCachedQueryCancellation(t *testing.T) {
	build := func() (*Session, *timelineService, musicalPosition, *contextRepeatMeterRoster) {
		s, v, pos := repeatUniformFixture(t, stateInt(t, 2))
		statePublish(t, s, &v.State.Times, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(repeatMeterValue("4", "4")), 91)
		roster := &contextRepeatMeterRoster{}
		if err := roster.prepare(context.Background(), s, v); err != nil {
			t.Fatal(err)
		}
		return s, v, pos, roster
	}
	s, v, pos, roster := build()
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	r := &contextReader{ctx: counter, s: s}
	got := contextRepeatUniformBeat(r, v, pos, roster)
	if r.err != nil || got.State != EvidenceKnown {
		t.Fatalf("cached probe %+v %v", got, r.err)
	}
	calls := counter.calls
	roster.close()
	s.cleanup()
	for at := 1; at <= calls; at++ {
		s, v, pos, roster := build()
		r := &contextReader{ctx: &rationalCancelContext{Context: context.Background(), at: at}, s: s}
		_ = contextRepeatUniformBeat(r, v, pos, roster)
		if !errors.Is(r.err, context.Canceled) {
			t.Fatalf("cached poll%d %v", at, r.err)
		}
		if roster.release == nil {
			t.Fatal("query released provider-owned roster")
		}
		work, facts := rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetContextFacts)
		roster.close()
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || rationalUsage(t, s, BudgetRuleSteps) != work || rationalUsage(t, s, BudgetContextFacts) != facts {
			t.Fatalf("cached poll%d leak/refund", at)
		}
	}
	t.Logf("interrupted all %d cached query polls", calls)
}
