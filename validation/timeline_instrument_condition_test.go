package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTimelineConditionalStatementProjectionAndOwnership(t *testing.T) {
	for _, invalidCondition := range []bool{false, true} {
		t.Run(map[bool]string{false: "known-condition", true: "invalid-condition"}[invalidCondition], func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			h := &timelineState{}
			stateClose(t, s, h, 0)
			at := statePosition(t, 0, 0, 1)
			id := NodeRef{"d", 71}
			condition := stateKnown("2")
			if invalidCondition {
				condition = Evidence[string]{State: EvidenceInvalid, Reason: "bad-condition", Cause: &DecisionRef{Rule: RuleKey{ID: "condition", Revision: 1}, Subject: NodeRef{"d", 30}}}
			}
			p := statePublication[instrumentState]{Position: at, Scope: stateScope{AllStaves: true}, TimeOnly: &condition, Value: stateKnown(instrumentState{Sound: stateKnown("conditional")})}
			if err := h.addInstrumentChange(context.Background(), s, id, p); err != nil {
				t.Fatal(err)
			}
			// An equal-position unconditional performance statement and an absent sound
			// field must not clear or conflict with the conditional sound statement.
			p.TimeOnly = nil
			p.Value = stateKnown(instrumentState{Solo: stateKnown(true)})
			if err := h.addInstrumentChange(context.Background(), s, id, p); err != nil {
				t.Fatal(err)
			}
			condition.Value = "mutated"
			if condition.Cause != nil {
				condition.Cause.Rule.ID = "mutated"
			}
			got, err := h.effectiveInstrumentStatements(context.Background(), s, at, id)
			if err != nil {
				t.Fatal(err)
			}
			if got.Performance.Value.State != EvidenceKnown || !got.Performance.Value.Value.Solo || got.Virtual.Value.State != EvidenceAbsent {
				t.Fatalf("condition poisoned absent/unrelated properties: %+v", got)
			}
			if invalidCondition {
				if got.Sound.Value.State != EvidenceInvalid || got.Sound.Value.Cause == nil || got.Sound.Value.Cause.Rule.ID != "condition" {
					t.Fatalf("lost condition cause: %+v", got.Sound)
				}
				got.Sound.Value.Cause.Rule.ID = "query-mutation"
			} else if got.Sound.Value.State != EvidenceUnknown || got.Sound.Value.Reason != "playback-repeat-pass-unavailable" {
				t.Fatalf("conditional statement known: %+v", got.Sound)
			}
			raw := h.InstrumentChanges[0].History.Publications[0]
			if raw.Value.Value.Sound.Value != "conditional" || raw.TimeOnly == nil || (!invalidCondition && raw.TimeOnly.Value != "2") || (invalidCondition && raw.TimeOnly.Cause.Rule.ID != "condition") {
				t.Fatalf("raw conditional evidence mutated: %+v", raw)
			}
		})
	}
}

func TestTimelineConditionalValueStrongestCauseAndTies(t *testing.T) {
	ctx := context.Background()
	for _, reverse := range []bool{false, true} {
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		h := &timelineState{}
		stateClose(t, s, h, 0)
		at := statePosition(t, 0, 0, 1)
		id := NodeRef{"d", 71}
		condition := stateKnown("2")
		invalid := Evidence[Integer]{State: EvidenceInvalid, Reason: "invalid-program", Cause: &DecisionRef{Rule: RuleKey{ID: "program", Revision: 1}, Subject: NodeRef{"d", 31}}}
		records := []instrumentRecord[midiState]{
			{Part: at.Part, Source: NodeRef{"d", 31}, Instrument: stateKnown(id), Position: stateKnown(at), TimeOnly: &condition, Value: stateKnown(midiState{Program: invalid})},
			{Part: at.Part, Source: NodeRef{"d", 32}, Instrument: stateKnown(id), Position: stateKnown(at), Value: stateKnown(midiState{Program: stateKnown(stateInt(t, 74)), Pan: stateKnown(stateDecimal(t, 20, 1))})},
		}
		if reverse {
			records[0], records[1] = records[1], records[0]
		}
		for _, r := range records {
			if err := h.addMIDIRecord(ctx, s, r, 0); err != nil {
				t.Fatal(err)
			}
		}
		got, err := h.effectiveMIDIStatements(ctx, s, at, id)
		if err != nil || got.Program.Value.State != EvidenceInvalid || got.Program.Value.Cause == nil || got.Program.Value.Cause.Rule.ID != "program" || got.Pan.Value.State != EvidenceKnown {
			t.Fatalf("strong actual cause/tie lost: %+v %v", got, err)
		}
	}
}

func TestTimelineConditionalSamePropertyTies(t *testing.T) {
	for _, bothConditional := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			h := &timelineState{}
			stateClose(t, s, h, 0)
			at := statePosition(t, 0, 0, 1)
			id := NodeRef{"d", 71}
			condition := stateKnown("2")
			conditions := []*Evidence[string]{&condition, nil}
			if bothConditional {
				conditions[1] = &condition
			}
			if reverse {
				conditions[0], conditions[1] = conditions[1], conditions[0]
			}
			for _, c := range conditions {
				if err := h.addMIDIDeviceRecord(context.Background(), s, instrumentRecord[midiDeviceState]{Part: at.Part, Source: NodeRef{"d", 31}, Instrument: stateAbsent[NodeRef](), Position: stateKnown(at), TimeOnly: c, Value: stateKnown(midiDeviceState{Name: stateKnown("same")})}, 0); err != nil {
					t.Fatal(err)
				}
			}
			got, err := h.effectiveMIDIDeviceStatements(context.Background(), s, at, id)
			if err != nil || got.Name.Value.State != EvidenceUnknown || got.Name.Value.Reason != "playback-repeat-pass-unavailable" || got.Port.Value.State != EvidenceAbsent {
				t.Fatalf("same-position condition lost: both=%t reverse=%t got=%+v err=%v", bothConditional, reverse, got, err)
			}
		}
	}
}

func TestTimelineConditionalRetentionAdmission(t *testing.T) {
	for _, budget := range []Budgets{{MaxBufferedBytes: 8192}, {MaxRuleSteps: 8192}} {
		s := rationalSession(t, budget)
		defer s.cleanup()
		h := &timelineState{}
		at := statePosition(t, 0, 0, 1)
		condition := stateKnown(strings.Repeat("2,", 8192))
		err := h.addInstrumentChange(context.Background(), s, NodeRef{"d", 71}, statePublication[instrumentState]{Position: at, Scope: stateScope{AllStaves: true}, TimeOnly: &condition, Value: stateKnown(instrumentState{Sound: stateKnown("conditional")})})
		if !errors.Is(err, ErrResourceLimit) || len(h.InstrumentChanges) != 0 {
			t.Fatalf("unadmitted large condition retained: budget=%+v err=%v", budget, err)
		}
	}
}
