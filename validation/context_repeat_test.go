package validation

import (
	"context"
	"testing"
)

func TestContextRepeatDefaultsAndInactiveDots(t *testing.T) {
	v := RepeatContextResult{Kind: RepeatStyleBeat, Type: stateKnown("start"), Slashes: stateAbsent[Integer](), UseDots: stateAbsent[bool](), Staff: stateAbsent[Integer]()}
	contextRepeatDefaults(&v)
	if v.Slashes.Value.String() != "1" || v.UseDots.Value || !v.AllStaves.Value || !v.Active.Value {
		t.Fatalf("%+v", v)
	}
	v = RepeatContextResult{Kind: RepeatStyleSlash, Type: stateKnown("stop"), UseStems: stateKnown(true), UseDots: stateKnown(true), Staff: stateKnown(stateInt(t, 1000000))}
	contextRepeatDefaults(&v)
	if v.Active.Value || v.AllStaves.Value || v.UseDots.State != EvidenceAbsent {
		t.Fatalf("inactive: %+v", v)
	}
	bad := Evidence[Integer]{State: EvidenceInvalid, Reason: "bad", Cause: &DecisionRef{Rule: RuleKey{"bad", 1}, Subject: NodeRef{"d", 1}}}
	v = RepeatContextResult{Kind: RepeatStyleMeasure, Slashes: bad, Staff: bad}
	contextRepeatDefaults(&v)
	if v.Slashes.State != EvidenceInvalid || v.AllStaves.State != EvidenceInvalid {
		t.Fatalf("default hid cause: %+v", v)
	}
}

func TestContextRepeatHalfOpenRanges(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	timeline := &timelineService{}
	part := NodeRef{"d", 1}
	rows := []contextRepeatRecord{
		{source: NodeRef{"d", 20}, position: stateKnown(statePosition(t, 1, 0, 1)), value: RepeatContextResult{Kind: RepeatStyleMeasure, Part: stateKnown(part), AllStaves: stateKnown(true), Staff: stateAbsent[Integer](), Type: stateKnown("start")}},
		{source: NodeRef{"d", 30}, position: stateKnown(statePosition(t, 4, 0, 1)), value: RepeatContextResult{Kind: RepeatStyleMeasure, Part: stateKnown(part), AllStaves: stateKnown(true), Staff: stateAbsent[Integer](), Type: stateKnown("stop")}},
	}
	if err := contextRepeatPairRanges(context.Background(), s, timeline, rows); err != nil {
		t.Fatal(err)
	}
	if rows[0].value.Stop.State != EvidenceKnown || rows[0].value.Stop.Value.Node != 30 || rows[1].value.Start.Value.Node != 20 {
		t.Fatalf("%+v", rows)
	}
}
