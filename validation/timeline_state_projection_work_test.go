package validation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

// A projection reads one fixed header. The unselected slice is deliberately
// much larger than the returned component; its publication cost is measured
// separately, before measuring the already-indexed query.
type projectionWorkRecord struct {
	Selected Evidence[string]
	Other    []string
}

func projectionWorkFixture(t *testing.T, b Budgets, padding, candidates int, selected string) (*Session, *timelineState, *stateHistory[projectionWorkRecord], musicalPosition) {
	t.Helper()
	s := rationalSession(t, b)
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 0, 1)
	history := &stateHistory[projectionWorkRecord]{}
	for i := range candidates {
		value := projectionWorkRecord{Selected: stateKnown(selected), Other: []string{strings.Repeat(string(rune('a'+i)), padding)}}
		statePublish(t, s, history, at, stateScope{AllStaves: true}, stateKnown(value), NodeID(100+i))
	}
	if _, err := stateIndex(context.Background(), s, history); err != nil {
		t.Fatal(err)
	}
	return s, h, history, at
}

func TestTimelineProjectionWorkExcludesUnselectedPayload(t *testing.T) {
	for _, candidates := range []int{1, 2, 4} {
		t.Run(fmt.Sprint(candidates), func(t *testing.T) {
			var work [2]int64
			for i, padding := range []int{1, 8192} {
				s, h, history, at := projectionWorkFixture(t, Budgets{}, padding, candidates, "selected")
				calls := 0
				before := rationalUsage(t, s, BudgetRuleSteps)
				out, err := queryState(context.Background(), s, h, history, at, stateKnown(stateOne()), func(v projectionWorkRecord) Evidence[string] {
					calls++
					return v.Selected
				})
				work[i] = rationalUsage(t, s, BudgetRuleSteps) - before
				if err != nil || out.Value.State != EvidenceKnown || out.Value.Value != "selected" || len(out.Provenance) != candidates || calls != candidates {
					t.Fatalf("out=%+v calls=%d err=%v", out, calls, err)
				}
				s.cleanup()
				if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
					t.Fatal("cleanup leaked")
				}
			}
			if work[0] != work[1] {
				t.Fatalf("unread payload inflated projected query: small=%d large=%d", work[0], work[1])
			}
			t.Logf("%d projections, work=%d for 1-byte and 8192-byte unrelated payloads", candidates, work[0])
		})
	}
}

func projectionKeyFixture(t *testing.T, b Budgets, count, width int) (*Session, *timelineState, musicalPosition) {
	t.Helper()
	s := rationalSession(t, b)
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 0, 1)
	v := keyState{Traditional: true, Fifths: stateKnown(stateOne()), Mode: stateKnown(strings.Repeat("x", width)), Cancel: stateAbsent[Integer](), CancelLocation: stateAbsent[string]()}
	for range count {
		v.Alterations = append(v.Alterations, keyAlteration{Step: stateKnown("C"), Alter: stateKnown(stateDecimal(t, 0, 1)), Accidental: stateKnown(strings.Repeat("a", width)), SMuFL: stateAbsent[string]()})
		v.Octaves = append(v.Octaves, keyOctave{Number: stateKnown(stateOne()), Octave: stateKnown(stateOne()), Cancel: stateAbsent[bool]()})
	}
	statePublish(t, s, &h.Keys, at, stateScope{AllStaves: true}, stateKnown(v), 100)
	if _, err := stateIndex(context.Background(), s, &h.Keys); err != nil {
		t.Fatal(err)
	}
	return s, h, at
}

func TestTimelineProjectionKeyPresenceVisitsComponentsNotPayloadBytes(t *testing.T) {
	var baseline int64
	for _, count := range []int{0, 1, 8, 32} {
		for _, width := range []int{1, 4096} {
			s, h, at := projectionKeyFixture(t, Budgets{}, count, width)
			before := rationalUsage(t, s, BudgetRuleSteps)
			out, err := h.queryKeyPresenceScope(context.Background(), s, at, stateKnown(stateOne()), false)
			work := rationalUsage(t, s, BudgetRuleSteps) - before
			if err != nil || out.Value.State != EvidenceKnown || !out.Value.Value {
				t.Fatalf("count%d width%d: %+v %v", count, width, out, err)
			}
			if count == 0 && width == 1 {
				baseline = work
			}
			// Each added alteration inspects four component states, each octave
			// three. Their string/numeric payloads are never inspected.
			if work != baseline+int64(7*count) {
				t.Errorf("count%d width%d: work=%d, want component-bound %d", count, width, work, baseline+int64(7*count))
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("cleanup leaked")
			}
		}
	}
}

func TestTimelineProjectionWorkStillChargesSelectedBytes(t *testing.T) {
	for _, candidates := range []int{1, 2} {
		var work [2]int64
		for i, width := range []int{1, 8192} {
			s, h, history, at := projectionWorkFixture(t, Budgets{}, 1, candidates, strings.Repeat("s", width))
			before := rationalUsage(t, s, BudgetRuleSteps)
			out, err := queryState(context.Background(), s, h, history, at, stateKnown(stateOne()), func(v projectionWorkRecord) Evidence[string] { return v.Selected })
			work[i] = rationalUsage(t, s, BudgetRuleSteps) - before
			if err != nil || len(out.Value.Value) != width {
				t.Fatalf("%+v %v", out, err)
			}
			s.cleanup()
		}
		if work[1]-work[0] < 8191 {
			t.Fatalf("selected byte work lost with %d candidates: %v", candidates, work)
		}
	}
}

func TestTimelineProjectionWorkBudgetAndCancellation(t *testing.T) {
	run := func(b Budgets, ctx context.Context) (int64, stateResult[string], error) {
		s, h, history, at := projectionWorkFixture(t, b, 256, 3, strings.Repeat("s", 512))
		out, err := queryState(ctx, s, h, history, at, stateKnown(stateOne()), func(v projectionWorkRecord) Evidence[string] { return v.Selected })
		work := rationalUsage(t, s, BudgetRuleSteps)
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || s.retained != 0 {
			t.Fatal("cleanup leaked")
		}
		return work, out, err
	}
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	work, want, err := run(Budgets{}, counter)
	if err != nil {
		t.Fatal(err)
	}
	for _, delta := range []int64{0, -1} {
		gotWork, got, err := run(Budgets{MaxRuleSteps: work + delta}, context.Background())
		if delta == 0 && (err != nil || gotWork != work || !reflect.DeepEqual(got, want)) {
			t.Fatalf("exact: work%d out%+v err%v", gotWork, got, err)
		}
		if delta < 0 && (!errors.Is(err, ErrResourceLimit) || !errors.Is(err, ErrIncompleteAssessment)) {
			t.Fatalf("one-short: %v", err)
		}
	}
	for poll := 1; poll <= counter.calls; poll++ {
		_, _, err := run(Budgets{}, &rationalCancelContext{Context: context.Background(), at: poll})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll%d/%d: %v", poll, counter.calls, err)
		}
	}
	t.Logf("exact=%d; one-short refused; %d cancellation boundaries", work, counter.calls)
}
