package validation

import (
	"context"
	"errors"
	"math"
	"testing"
)

func repeatIndexRows(t *testing.T, count int) []contextRepeatRecord {
	t.Helper()
	rows := make([]contextRepeatRecord, 0, 2*count)
	for i := 0; i < count; i++ {
		for j, typ := range []string{"start", "stop"} {
			rows = append(rows, contextRepeatRecord{source: NodeRef{"d", NodeID(2*i + j + 1)}, position: stateKnown(statePosition(t, 0, int64(2*i+j), 1)), value: RepeatContextResult{Kind: RepeatStyleSlash, Part: stateKnown(NodeRef{"d", 1}), AllStaves: stateKnown(true), Type: stateKnown(typ)}})
		}
	}
	return rows
}
func TestContextRepeatSinglePartIndexedRangeGrowth(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		rows := repeatIndexRows(t, count)
		if err := contextRepeatPairRanges(context.Background(), s, &timelineService{}, rows); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < count; i++ {
			if rows[2*i].value.Stop.State != EvidenceKnown || rows[2*i].value.Stop.Value != rows[2*i+1].source {
				t.Fatal("lost ordered range")
			}
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("single part ranges %d -> %d", small, large)
	if large > small*15 {
		t.Fatal("single-part ordinary ranges still quadratic")
	}
}
func TestContextRepeatIndexedEqualPositionsRemainUnknown(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	rows := repeatIndexRows(t, 2)
	rows[2].position = rows[1].position
	if err := contextRepeatPairRanges(context.Background(), s, &timelineService{}, rows); err != nil {
		t.Fatal(err)
	}
	if rows[0].value.Complete.State != EvidenceUnknown || rows[2].value.Complete.State != EvidenceUnknown {
		t.Fatalf("equal order inferred %+v", rows)
	}
}
func TestContextRepeatIndexCancellationAndQuota(t *testing.T) {
	for _, kind := range []string{"range", "navigation", "notes"} {
		run := func(ctx context.Context, s *Session) error {
			rows := repeatIndexRows(t, 3)
			switch kind {
			case "range":
				return contextRepeatPairRanges(ctx, s, &timelineService{}, rows)
			case "navigation":
				events := []contextNavigationNode{{event: navigationEvent{Source: NodeRef{"d", 1}, Kind: NavigationSegno}, value: NavigationContextResult{Label: stateKnown("S")}}, {event: navigationEvent{Source: NodeRef{"d", 2}, Kind: NavigationDalSegno}, value: NavigationContextResult{Label: stateKnown("S")}}}
				return navigationBuildEdges(ctx, s, events)
			default:
				timeline := &timelineService{Facts: map[NodeID]timelineEventFact{}, Playback: timelinePlaybackContext{Notes: map[NodeID]notePlaybackContext{}}}
				for i := NodeID(1); i <= 3; i++ {
					s.sequence = append(s.sequence, i)
					timeline.Playback.Notes[i] = notePlaybackContext{Origin: playbackOrigin{Source: NodeRef{"d", i}, Part: stateKnown(NodeRef{"d", 1})}}
				}
				return contextRepeatNoteScopes(ctx, s, timeline, rows)
			}
		}
		probe := rationalSession(t, Budgets{})
		counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
		if err := run(counter, probe); err != nil {
			t.Fatal(err)
		}
		calls := counter.calls
		probe.cleanup()
		for at := 1; at <= calls; at++ {
			s := rationalSession(t, Budgets{})
			ctx := &rationalCancelContext{Context: context.Background(), at: at}
			err := run(ctx, s)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("%s poll%d: %v", kind, at, err)
			}
			work := rationalUsage(t, s, BudgetRuleSteps)
			facts := rationalUsage(t, s, BudgetContextFacts)
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 || rationalUsage(t, s, BudgetRuleSteps) != work || rationalUsage(t, s, BudgetContextFacts) != facts {
				t.Fatalf("%s poll%d leak/refund", kind, at)
			}
		}
		s := rationalSession(t, Budgets{MaxBufferedBytes: 1})
		if err := run(context.Background(), s); !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("%s unadmitted index %v", kind, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("failed index admission retained bytes")
		}
		s.cleanup()
		t.Logf("%s: all %d cancellation polls", kind, calls)
	}
}

func TestContextRepeatIndexedUnknownPartPreservesCause(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	cause := &DecisionRef{Rule: RuleKey{"bad-part", 1}, Subject: NodeRef{"d", 99}}
	rows := repeatIndexRows(t, 1)
	rows = append(rows, contextRepeatRecord{source: NodeRef{"d", 9}, position: Evidence[musicalPosition]{State: EvidenceInvalid, Cause: cause}, value: RepeatContextResult{Kind: RepeatStyleSlash, Part: Evidence[NodeRef]{State: EvidenceInvalid, Cause: cause}, Type: stateKnown("stop")}})
	if err := contextRepeatPairRanges(context.Background(), s, &timelineService{}, rows); err != nil {
		t.Fatal(err)
	}
	if rows[0].value.Complete.State != EvidenceInvalid || rows[0].value.Complete.Cause != cause {
		t.Fatalf("unknown-part range candidate discarded %+v", rows[0].value)
	}
	rows = repeatIndexRows(t, 1)[:1]
	rows[0].value.Complete = stateKnown(true)
	rows[0].value.Stop = stateAbsent[NodeRef]()
	s.sequence = []NodeID{10, 11, 12}
	timeline := &timelineService{Facts: map[NodeID]timelineEventFact{}, Playback: timelinePlaybackContext{Notes: map[NodeID]notePlaybackContext{}}}
	for _, id := range s.sequence {
		part := stateKnown(NodeRef{"d", 1})
		if id == 11 {
			part = Evidence[NodeRef]{State: EvidenceInvalid, Cause: cause}
		}
		if id == 12 {
			part = stateKnown(NodeRef{"d", 2})
		}
		timeline.Playback.Notes[id] = notePlaybackContext{Origin: playbackOrigin{Source: NodeRef{"d", id}, Part: part}}
		timeline.Facts[id] = timelineEventFact{Position: stateKnown(statePosition(t, 0, 0, 1))}
	}
	if err := contextRepeatNoteScopes(context.Background(), s, timeline, rows); err != nil {
		t.Fatal(err)
	}
	notes := rows[0].value.Notes
	if len(notes) != 2 || notes[0].Source.Node != 10 || notes[1].Source.Node != 11 || notes[1].InRange.State != EvidenceInvalid || notes[1].InRange.Cause.Subject != cause.Subject {
		t.Fatalf("unknown roster/order/cause lost %+v", notes)
	}
}

func TestContextNavigationEmptyIndexCanceledBeforeAdmission(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := navigationBuildEdges(ctx, s, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("empty canceled navigation index: %v", err)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("canceled empty index allocated scratch")
	}
}

func TestContextRepeatNoRowsSkipsIndexAllocation(t *testing.T) {
	for _, run := range []func(context.Context, *Session, *timelineService, []contextRepeatRecord) error{contextRepeatPairRanges, contextRepeatNoteScopes, func(ctx context.Context, s *Session, timeline *timelineService, rows []contextRepeatRecord) error {
		return contextRepeatProveContent(ctx, s, &contextPlan{}, timeline, rows)
	}} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: 1})
		s.sequence = make([]NodeID, 1000)
		if err := run(context.Background(), s, &timelineService{}, nil); err != nil {
			t.Fatalf("unused graphical index allocated: %v", err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("unused graphical index retains bytes")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := run(ctx, s, &timelineService{}, nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("empty canceled index: %v", err)
		}
		s.cleanup()
	}
}
