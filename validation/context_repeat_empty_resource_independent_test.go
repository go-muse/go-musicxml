package validation

import (
	"context"
	"errors"
	"math"
	"testing"
)

func independentEmptyOnsetFixture(t *testing.T, count int, limits Budgets) (*Session, *contextReader, *timelineService, []contextRepeatNote, []contextRepeatRecord) {
	t.Helper()
	s, r, timeline, all, rows := round1RepeatIndexedFixture(t, count)
	notes := make([]contextRepeatNote, 0, count)
	for i, n := range all {
		if i%2 == 0 {
			n.rest = false
			notes = append(notes, n)
		}
	}
	var err error
	s.account, err = NewBudgetAccount(limits)
	if err != nil {
		t.Fatal(err)
	}
	return s, r, timeline, notes, rows
}

func independentEmptyOnsetRun(t *testing.T, ctx context.Context, s *Session, r *contextReader, timeline *timelineService, notes []contextRepeatNote, rows []contextRepeatRecord, indexed bool) error {
	t.Helper()
	r.ctx = ctx
	var index *contextRepeatProofIndex
	if indexed {
		var free func()
		var err error
		index, free, err = contextRepeatBuildProofIndex(r, timeline, rows)
		if err != nil {
			return err
		}
		defer free()
	}
	roster, free, err := contextRepeatBuildContentRoster(ctx, s, notes)
	if err != nil {
		return err
	}
	defer free()
	for i := range rows {
		var e error
		if indexed {
			e = contextRepeatProveRosterRow(ctx, s, timeline, roster, rows, &rows[i], stateKnown(true), index)
		} else {
			e = contextRepeatProveRosterRow(ctx, s, timeline, roster, rows, &rows[i], stateKnown(true))
		}
		if e != nil {
			return e
		}
		if rows[i].value.Type.Value == "start" && (rows[i].value.ContentEquivalent.State != EvidenceKnown || rows[i].value.ContentEquivalent.Value) {
			t.Fatalf("empty target not disproved: %+v", rows[i].value.ContentEquivalent)
		}
	}
	if indexed {
		if len(index.scopes) != 1 || len(index.parts) != 1 || len(index.releases) != 2 {
			t.Fatalf("duplicate per-start scope retention: scopes=%d parts=%d releases=%d", len(index.scopes), len(index.parts), len(index.releases))
		}
		for _, scope := range index.scopes {
			if len(scope.sourceOnsets) != len(notes) {
				t.Fatalf("wrong sparse witness count %d vs %d", len(scope.sourceOnsets), len(notes))
			}
		}
	}
	return nil
}

func TestIndependentEmptyOnsetFullScopeLinearAndLifetime(t *testing.T) {
	var prior int64
	for _, n := range []int{100, 1000} {
		s, r, timeline, notes, rows := independentEmptyOnsetFixture(t, n, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		err := independentEmptyOnsetRun(t, context.Background(), s, r, timeline, notes, rows, true)
		if err != nil {
			t.Fatal(err)
		}
		work := rationalUsage(t, s, BudgetRuleSteps)
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatalf("scope scratch not released: gauge=%d retained=%d", rationalUsage(t, s, BudgetBufferedBytes), s.retained)
		}
		if prior != 0 && work > 11*prior {
			t.Fatalf("repeated empty starts not linear %d -> %d", prior, work)
		}
		t.Logf("%d distinct empty starts: %d steps, %d retained arithmetic bytes, exactly one scope and onset map", n, work, s.retained)
		prior = work
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup leaked")
		}
	}
}

func TestIndependentEmptyOnsetProviderAdmissionAndAllPolls(t *testing.T) {
	for _, indexed := range []bool{false, true} {
		s, r, timeline, notes, rows := independentEmptyOnsetFixture(t, 2, Budgets{})
		counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
		if err := independentEmptyOnsetRun(t, counter, s, r, timeline, notes, rows, indexed); err != nil {
			t.Fatal(err)
		}
		work := rationalUsage(t, s, BudgetRuleSteps)
		s.cleanup()
		check := func(ctx context.Context, limits Budgets) error {
			s, r, timeline, notes, rows := independentEmptyOnsetFixture(t, 2, limits)
			err := independentEmptyOnsetRun(t, ctx, s, r, timeline, notes, rows, indexed)
			if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
				t.Fatalf("indexed=%t leaked scratch: gauge=%d retained=%d err=%v", indexed, rationalUsage(t, s, BudgetBufferedBytes), s.retained, err)
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("cleanup leaked")
			}
			return err
		}
		for at := 1; at <= counter.calls; at++ {
			if err := check(&rationalCancelContext{Context: context.Background(), at: at}, Budgets{}); !errors.Is(err, context.Canceled) {
				t.Fatalf("indexed=%t poll=%d error=%v", indexed, at, err)
			}
		}
		for _, delta := range []int64{0, -1} {
			err := check(context.Background(), Budgets{MaxRuleSteps: work + delta})
			if delta == 0 && err != nil || delta < 0 && !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("indexed=%t step delta=%d err=%v", indexed, delta, err)
			}
		}
		low, high := int64(1), int64(1<<20)
		for low < high {
			mid := low + (high-low)/2
			err := check(context.Background(), Budgets{MaxBufferedBytes: mid})
			if err == nil {
				high = mid
			} else if errors.Is(err, ErrResourceLimit) {
				low = mid + 1
			} else {
				t.Fatal(err)
			}
		}
		if err := check(context.Background(), Budgets{MaxBufferedBytes: low}); err != nil {
			t.Fatal(err)
		}
		if err := check(context.Background(), Budgets{MaxBufferedBytes: low - 1}); !errors.Is(err, ErrResourceLimit) {
			t.Fatal(err)
		}
		t.Logf("indexed=%t: all %d cancellation polls; exact steps=%d, exact bytes=%d; one-under refused and all gauges clean", indexed, counter.calls, work, low)
	}
}

func TestIndependentEmptyOnsetScopeKeysAndUncachedRelease(t *testing.T) {
	ctx := context.Background()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	x := &contextRepeatProofIndex{scopes: make(map[contextRepeatProofScopeKey]*contextRepeatProofScope)}
	defer func() {
		for _, f := range x.releases {
			f()
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("scopes leaked %d", rationalUsage(t, s, BudgetBufferedBytes))
		}
	}()
	var previous []*contextRepeatProofScope
	for _, spec := range []struct {
		part  NodeRef
		all   bool
		staff int64
	}{{NodeRef{"d", 1}, true, 0}, {NodeRef{"other", 1}, true, 0}, {NodeRef{"d", 2}, true, 0}, {NodeRef{"d", 1}, false, 1}, {NodeRef{"d", 1}, false, 2}} {
		notes, _ := orderedRepeatNotes(t, 1)
		notes[0].part = stateKnown(spec.part)
		notes[0].pos.Part = spec.part
		notes[0].staff = stateKnown(stateInt(t, spec.staff))
		row := RepeatContextResult{Part: stateKnown(spec.part), AllStaves: stateKnown(spec.all), Staff: stateKnown(stateInt(t, spec.staff))}
		scope, free, err := x.scope(ctx, s, notes, row)
		if err != nil {
			t.Fatal(err)
		}
		free()
		for _, old := range previous {
			if old == scope {
				t.Fatalf("scope keys collided: %+v", spec)
			}
		}
		previous = append(previous, scope)
		got, err := contextRepeatEmptyMeasure(ctx, s, &timelineService{}, scope, spec.part, 0, 1)
		if err != nil || got.State != EvidenceKnown || got.Value {
			t.Fatalf("key proof %+v err=%v", got, err)
		}
		bytes := rationalUsage(t, s, BudgetBufferedBytes)
		again, free, err := x.scope(ctx, s, notes, row)
		if err != nil {
			t.Fatal(err)
		}
		free()
		if again != scope || rationalUsage(t, s, BudgetBufferedBytes) != bytes {
			t.Fatal("identical key did not borrow cached scope")
		}
	}
	baseline := rationalUsage(t, s, BudgetBufferedBytes)
	notes, _ := orderedRepeatNotes(t, 1)
	row := RepeatContextResult{Part: stateKnown(NodeRef{"d", 1}), AllStaves: stateKnown(true), ExceptVoices: []Evidence[string]{stateKnown("2")}}
	for i := 0; i < 3; i++ {
		scope, free, err := x.scope(ctx, s, notes, row)
		if err != nil {
			t.Fatal(err)
		}
		got, err := contextRepeatEmptyMeasure(ctx, s, &timelineService{}, scope, row.Part.Value, 0, 1)
		if err != nil || got.State != EvidenceKnown || got.Value {
			t.Fatalf("uncached proof %+v err=%v", got, err)
		}
		free()
		if rationalUsage(t, s, BudgetBufferedBytes) != baseline || len(x.scopes) != 5 {
			t.Fatal("voice-qualified scope leaked or was cached")
		}
	}
	t.Log("canonical document, part, all-staff, exact-staff keys remain isolated; voice-qualified scope released after each use")
}

func TestIndependentEmptyOnsetUnprovedFrameArithmeticCleanup(t *testing.T) {
	run := func(ctx context.Context, limits Budgets) (int64, error) {
		s := rationalSession(t, limits)
		notes, _ := orderedRepeatNotes(t, 3)
		timeline := &timelineService{}
		timeline.State.frameBounds = make(map[timelineFrame]Rational)
		for i := range notes {
			notes[i].pos.UnprovedFrame = true
			notes[i].pos.Offset = mustRational(t, int64(i+1), 3)
			timeline.State.frameBounds[frameOf(notes[i].pos)] = mustRational(t, 1, 1)
		}
		scope := &contextRepeatProofScope{notes: notes, complete: stateKnown(true), buckets: map[uint64][]contextRepeatNote{0: notes}}
		got, err := contextRepeatEmptyMeasure(ctx, s, timeline, scope, NodeRef{"d", 1}, 0, 1)
		if err == nil && (got.State != EvidenceKnown || got.Value) {
			t.Fatalf("contained unproved-frame proof %+v", got)
		}
		if err != nil && (scope.sourceOnsets != nil || scope.onsetComplete.State != "" || scope.releaseOnsets != nil) {
			t.Fatal("failed proof published partial index")
		}
		steps := rationalUsage(t, s, BudgetRuleSteps)
		scope.closeOnsets()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || s.retained != 0 {
			t.Fatalf("rational scratch survived proof: %d / %d", rationalUsage(t, s, BudgetBufferedBytes), s.retained)
		}
		s.cleanup()
		return steps, err
	}
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	steps, err := run(counter, Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	for at := 1; at <= counter.calls; at++ {
		if _, err := run(&rationalCancelContext{Context: context.Background(), at: at}, Budgets{}); !errors.Is(err, context.Canceled) {
			t.Fatalf("poll %d: %v", at, err)
		}
	}
	for _, delta := range []int64{0, -1} {
		_, err := run(context.Background(), Budgets{MaxRuleSteps: steps + delta})
		if delta == 0 && err != nil || delta < 0 && !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("step delta %d: %v", delta, err)
		}
	}
	low, high := int64(1), int64(1<<20)
	for low < high {
		mid := low + (high-low)/2
		_, err := run(context.Background(), Budgets{MaxBufferedBytes: mid})
		if err == nil {
			high = mid
		} else if errors.Is(err, ErrResourceLimit) {
			low = mid + 1
		} else {
			t.Fatal(err)
		}
	}
	if _, err := run(context.Background(), Budgets{MaxBufferedBytes: low}); err != nil {
		t.Fatal(err)
	}
	if _, err := run(context.Background(), Budgets{MaxBufferedBytes: low - 1}); !errors.Is(err, ErrResourceLimit) {
		t.Fatal(err)
	}
	t.Logf("unproved frame arithmetic: all %d cancellation polls; %d exact steps and %d exact bytes; all transient rational storage released", counter.calls, steps, low)
}
