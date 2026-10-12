package validation

import (
	"context"
	"errors"
	"math"
	"testing"
	"unsafe"
)

func TestIndependentContextMeasureRosterAdmissionAndCauseOrder(t *testing.T) {
	// Account for up to three simultaneous growth-array copies plus map overhead.
	if width := int64(unsafe.Sizeof(contextRepeatNote{})); 3*width+256 > 4096 {
		t.Fatalf("measure roster bound too small for copied note width %d", width)
	} else {
		t.Logf("note width=%d, three copies+map allowance=%d within4096 per input", width, 3*width+256)
	}
	part := NodeRef{"d", 1}
	pos := statePosition(t, math.MaxUint64, 0, 1)
	pos.Part = part
	causeA := &DecisionRef{Rule: RuleKey{ID: "unknown-part", Revision: 1}, Subject: NodeRef{"d", 1}}
	causeB := &DecisionRef{Rule: RuleKey{ID: "unknown-position", Revision: 1}, Subject: NodeRef{"d", 2}}
	good := contextRepeatNote{source: NodeRef{"d", 3}, part: stateKnown(part), pos: pos, available: stateKnown(true)}
	unknown := contextRepeatNote{part: Evidence[NodeRef]{State: EvidenceInvalid, Reason: "unknown-part", Cause: causeA}}
	unplaced := contextRepeatNote{part: stateKnown(part), available: Evidence[bool]{State: EvidenceInvalid, Reason: "unknown-position", Cause: causeB}}
	unrelated := contextRepeatNote{part: stateKnown(NodeRef{"other", 1}), available: Evidence[bool]{State: EvidenceInvalid, Reason: "unrelated", Cause: causeB}}
	notes := []contextRepeatNote{unknown, unrelated, good, unplaced, good}
	const bytes = 512 + 5*4096
	for _, limit := range []int64{bytes, bytes - 1} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		roster, complete, release, err := contextRepeatBuildMeasureRoster(context.Background(), s, notes, part)
		if limit == bytes {
			if err != nil || len(roster) != 1 || len(roster[math.MaxUint64]) != 2 || complete.State != EvidenceInvalid || complete.Cause != causeA {
				t.Fatalf("exact sparse/order admission: %v %+v %+v", err, complete, roster)
			}
			roster[math.MaxUint64][0].source.Node = 99
			if notes[2].source.Node != 3 {
				t.Fatal("bucket mutates input value slice")
			}
			release()
		} else if err == nil || roster != nil || release != nil {
			t.Fatalf("one-under published partial roster: %v", err)
		}
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Fatalf("retained %d", got)
		}
		s.cleanup()
	}
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	notes[0], notes[3] = notes[3], notes[0]
	_, complete, release, err := contextRepeatBuildMeasureRoster(context.Background(), s, notes, part)
	if err != nil || complete.Cause != causeB {
		t.Fatalf("reversed cause order lost: %v %+v", err, complete)
	}
	release()
}

func TestIndependentContextMeasureRosterCancellation(t *testing.T) {
	part := NodeRef{"d", 1}
	pos := statePosition(t, 0, 0, 1)
	pos.Part = part
	notes := []contextRepeatNote{{part: stateKnown(part), pos: pos}, {part: stateUnknown[NodeRef]("unknown")}, {part: stateKnown(NodeRef{"other", 1})}}
	probe := &rationalCancelContext{Context: context.Background(), at: 1000000}
	s := rationalSession(t, Budgets{})
	_, _, release, err := contextRepeatBuildMeasureRoster(probe, s, notes, part)
	if err != nil {
		t.Fatal(err)
	}
	release()
	s.cleanup()
	for at := 1; at <= probe.calls; at++ {
		s := rationalSession(t, Budgets{})
		out, _, release, err := contextRepeatBuildMeasureRoster(&rationalCancelContext{Context: context.Background(), at: at}, s, notes, part)
		if !errors.Is(err, context.Canceled) || out != nil || release != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("poll%d: %v", at, err)
		}
		s.cleanup()
	}
	t.Logf("all %d roster cancellation polls", probe.calls)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s = rationalSession(t, Budgets{})
	defer s.cleanup()
	out, _, release, err := contextRepeatBuildMeasureRoster(ctx, s, nil, part)
	if !errors.Is(err, context.Canceled) || out != nil || release != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatalf("empty pre-cancel accepted: %v", err)
	}
}
