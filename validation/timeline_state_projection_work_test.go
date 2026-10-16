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
		// One candidate copies once. Two also measure both projections and
		// admit both payloads before their distinct-candidate comparison.
		passes := int64(1)
		if candidates == 2 {
			passes = 5
		}
		if delta := work[1] - work[0]; delta != passes*8191 {
			t.Fatalf("selected byte passes with %d candidates: delta=%d want=%d", candidates, delta, passes*8191)
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

func TestTimelineProjectionKeyMeterLiteralBoundaries(t *testing.T) {
	v := keyState{Traditional: true, Fifths: stateKnown(stateOne()), Mode: stateAbsent[string](), Cancel: stateAbsent[Integer](), CancelLocation: stateAbsent[string]()}
	for range 3 {
		v.Alterations = append(v.Alterations, keyAlteration{Step: stateKnown("C"), Alter: stateKnown(stateDecimal(t, 0, 1)), Accidental: stateAbsent[string](), SMuFL: stateAbsent[string]()})
		v.Octaves = append(v.Octaves, keyOctave{Number: stateKnown(stateOne()), Octave: stateKnown(stateOne()), Cancel: stateAbsent[bool]()})
	}
	// Independent literal: four fixed components, four per alteration and
	// three per octave. No payload bytes are traversed by this operation.
	const work = int64(25)
	for _, limit := range []int64{work - 1, work} {
		s := rationalSession(t, Budgets{MaxRuleSteps: limit})
		got, err := v.encodingEvidenceMetered(context.Background(), s)
		if limit == work && (err != nil || !reflect.DeepEqual(got, v.encodingEvidence()) || rationalUsage(t, s, BudgetRuleSteps) != work) {
			t.Fatalf("exact: %+v %v", got, err)
		}
		if limit < work && (!errors.Is(err, ErrResourceLimit) || !errors.Is(err, ErrIncompleteAssessment)) {
			t.Fatalf("one-short: %v", err)
		}
		s.cleanup()
	}
	for poll := 1; poll <= int(work); poll++ {
		s := rationalSession(t, Budgets{})
		_, err := v.encodingEvidenceMetered(&rationalCancelContext{Context: context.Background(), at: poll}, s)
		if !errors.Is(err, context.Canceled) || rationalUsage(t, s, BudgetRuleSteps) != int64(poll-1) {
			t.Fatalf("poll%d: work%d err%v", poll, rationalUsage(t, s, BudgetRuleSteps), err)
		}
		s.cleanup()
	}
	// A later actual invalid cause must beat an earlier unknown; source order
	// and ties match the original pure projection exactly.
	v.Mode = stateUnknown[string]("earlier-unknown")
	v.Octaves[2].Cancel = Evidence[bool]{State: EvidenceInvalid, Reason: "later-invalid", Cause: &DecisionRef{Rule: RuleKey{ID: "octave", Revision: 1}, Subject: NodeRef{"d", 102}}}
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	got, err := v.encodingEvidenceMetered(context.Background(), s)
	if err != nil || !reflect.DeepEqual(got, v.encodingEvidence()) || got.State != EvidenceInvalid || got.Cause == nil || got.Cause.Subject.Node != 102 {
		t.Fatalf("causal precedence: %+v %v", got, err)
	}
}

func TestTimelineProjectionKeyQueryEveryCancellationBoundary(t *testing.T) {
	run := func(b Budgets, ctx context.Context) (int64, error) {
		s, h, at := projectionKeyFixture(t, b, 8, 64)
		before := stateClone(h.Keys.Publications[0].Value)
		_, err := h.queryKeyPresenceScope(ctx, s, at, stateKnown(stateOne()), false)
		if !reflect.DeepEqual(before, h.Keys.Publications[0].Value) {
			t.Fatal("query mutated immutable publication")
		}
		work := rationalUsage(t, s, BudgetRuleSteps)
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || s.retained != 0 {
			t.Fatal("cleanup leaked")
		}
		return work, err
	}
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	work, err := run(Budgets{}, counter)
	if err != nil {
		t.Fatal(err)
	}
	for _, delta := range []int64{0, -1} {
		got, err := run(Budgets{MaxRuleSteps: work + delta}, context.Background())
		if delta == 0 && (err != nil || got != work) {
			t.Fatalf("exact: %d %v", got, err)
		}
		if delta < 0 && !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("one-short: %v", err)
		}
	}
	for poll := 1; poll <= counter.calls; poll++ {
		_, err := run(Budgets{}, &rationalCancelContext{Context: context.Background(), at: poll})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll%d/%d: %v", poll, counter.calls, err)
		}
	}
	t.Logf("key query exact=%d; one-short refused; %d cancellation boundaries", work, counter.calls)
}

func TestTimelineProjectionWorkReferenceEquivalence(t *testing.T) {
	count := 0
	for _, inclusive := range []bool{false, true} {
		for _, staff := range []Evidence[Integer]{stateKnown(stateOne()), stateKnown(stateInt(t, 2)), stateAbsent[Integer](), stateUnknown[Integer]("unresolved")} {
			for _, candidateState := range []EvidenceState{EvidenceKnown, EvidenceAbsent, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
				for _, partial := range []bool{false, true} {
					for _, withGap := range []bool{false, true} {
						s, h, history, at := projectionWorkFixture(t, Budgets{}, 256, 1, "selected")
						later := at
						later.Offset = mustRational(t, 1, 1)
						later.UnprovedFrame = partial
						cause := &DecisionRef{Rule: RuleKey{ID: RuleID(strings.Repeat("cause", 32)), Revision: 1}, Subject: NodeRef{"d", 103}}
						selected := Evidence[string]{State: candidateState, Value: "different", Reason: "candidate", Cause: cause}
						v := projectionWorkRecord{Selected: selected, Other: []string{strings.Repeat("unread", 256)}}
						statePublish(t, s, history, later, stateScope{Staff: stateOne()}, stateKnown(v), 101)
						// An all-staves statement at the same position exercises overlap;
						// a new publication also forces index generation invalidation.
						statePublish(t, s, history, later, stateScope{AllStaves: true}, stateKnown(v), 102)
						if withGap {
							gap := stateGap{Part: at.Part, Scope: stateScope{AllStaves: true}, Availability: Evidence[bool]{State: EvidenceUnsupported, Reason: "gap", Cause: cause}}
							if err := addStateGap(context.Background(), s, history, gap); err != nil {
								t.Fatal(err)
							}
						}
						project := func(v projectionWorkRecord) Evidence[string] { return v.Selected }
						want, err := crossReviewStateBefore(context.Background(), s, h, history, later, staff, project, inclusive, true, false, nil)
						if err != nil {
							t.Fatal(err)
						}
						got, err := queryStateAtScope(context.Background(), s, h, history, later, staff, project, inclusive, true, false, nil)
						if err != nil || !reflect.DeepEqual(got, want) {
							t.Fatalf("inclusive%v staff%+v state%s partial%v gap%v: want%+v got%+v err%v", inclusive, staff, candidateState, partial, withGap, want, got, err)
						}
						if got.Value.Cause != nil {
							got.Value.Cause.Subject.Node = 999
						}
						if len(got.Provenance) != 0 {
							got.Provenance[0].Node.Node = 999
						}
						again, err := queryStateAtScope(context.Background(), s, h, history, later, staff, project, inclusive, true, false, nil)
						if err != nil || !reflect.DeepEqual(again, want) {
							t.Fatalf("result mutation leaked: want%+v got%+v err%v", want, again, err)
						}
						s.cleanup()
						if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
							t.Fatal("cleanup leaked")
						}
						count++
					}
				}
			}
		}
	}
	t.Logf("%d reference comparisons preserve value, cause, provenance and inheritance", count)
}

func TestTimelineProjectionRefusalStopsQueryImmediately(t *testing.T) {
	for _, partial := range []bool{false, true} {
		for _, failCall := range []int{1, 2} {
			t.Run(fmt.Sprintf("partial=%v/failCall=%d", partial, failCall), func(t *testing.T) {
				const candidates = 3 // A later candidate must remain unprojected after either refusal.
				s, h, history, at := projectionWorkFixture(t, Budgets{}, 64, 0, "selected")
				publicationAt := at
				publicationAt.UnprovedFrame = partial
				for i := range candidates {
					v := projectionWorkRecord{Selected: stateKnown("selected"), Other: []string{strings.Repeat("unread", 64)}}
					statePublish(t, s, history, publicationAt, stateScope{AllStaves: true}, stateKnown(v), NodeID(100+i))
				}
				// Partiality comes from the publications, not a flag on the query.
				isPartial, err := statePartialQuery(context.Background(), s, h, history, at, stateOne())
				if err != nil || isPartial != partial {
					t.Fatalf("partial route=%v, want %v: %v", isPartial, partial, err)
				}
				var scratch int64
				if partial {
					// Three 64-byte candidate slots plus the 256-byte scratch header.
					scratch = 448
				}
				calls := 0
				var stoppedWork, stoppedRetained, stoppedBuffered int64
				// Deliberately leave ample account capacity. A rejected operation
				// does not poison Account; immediate error propagation is required.
				refusal := &ResourceError{Kind: BudgetRuleSteps, Limit: 1, Used: 0, Requested: 2}
				project := func(v projectionWorkRecord) (Evidence[string], error) {
					calls++
					if got := rationalUsage(t, s, BudgetBufferedBytes) - s.retained; got != scratch {
						t.Fatalf("projection scratch=%d, want %d for partial=%v", got, scratch, partial)
					}
					if calls == failCall {
						stoppedWork = rationalUsage(t, s, BudgetRuleSteps)
						stoppedRetained = s.retained
						stoppedBuffered = rationalUsage(t, s, BudgetBufferedBytes)
						return Evidence[string]{}, refusal
					}
					return v.Selected, nil
				}
				_, err = queryStateAtScopeWithProject(context.Background(), s, h, history, at, stateKnown(stateOne()), project, true, true, false, nil)
				if err != refusal || calls != failCall || rationalUsage(t, s, BudgetRuleSteps) != stoppedWork || s.retained != stoppedRetained {
					t.Fatalf("calls=%d/%d work=%d/%d retained=%d/%d err=%v; want original refusal", calls, failCall, rationalUsage(t, s, BudgetRuleSteps), stoppedWork, s.retained, stoppedRetained, err)
				}
				if got := rationalUsage(t, s, BudgetBufferedBytes); got != stoppedBuffered-scratch || got != s.retained {
					t.Fatalf("projection refusal leaked scratch: buffered=%d, stopped=%d, scratch=%d, retained=%d", got, stoppedBuffered, scratch, s.retained)
				}
				s.cleanup()
				if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
					t.Fatal("cleanup leaked")
				}
			})
		}
	}
}
