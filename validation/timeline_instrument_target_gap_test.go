package validation

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestTimelinePositionedTargetGapBoundaries(t *testing.T) {
	ctx := context.Background()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 2, 1)
	id := NodeRef{"d", 71}
	cause := &DecisionRef{Rule: RuleKey{ID: "target", Revision: 1}, Subject: NodeRef{"d", 90}}
	bad := Evidence[NodeRef]{State: EvidenceInvalid, Reason: "target-missing", Cause: cause}
	record := instrumentRecord[midiDeviceState]{Part: at.Part, Source: NodeRef{"d", 90}, Instrument: bad, Position: stateKnown(at), Value: stateKnown(midiDeviceState{Name: stateKnown("bad-target")})}
	if err := h.addMIDIDeviceRecord(ctx, s, record, 0); err != nil {
		t.Fatal(err)
	}
	cause.Rule.ID = "mutated"
	for _, q := range []int64{1, 2, 3} {
		at.Offset = mustRational(t, q, 1)
		got, err := h.effectiveMIDIDeviceStatements(ctx, s, at, id)
		if err != nil {
			t.Fatal(err)
		}
		if q == 1 {
			if got.Name.Value.State != EvidenceAbsent {
				t.Fatalf("future gap affected prefix: %+v", got)
			}
		} else if got.Name.Value.State != EvidenceInvalid || got.Name.Value.Cause == nil || got.Name.Value.Cause.Rule.ID != "target" {
			t.Fatalf("exact gap lost cause: %+v", got)
		}
		if got.Port.Value.State != EvidenceAbsent {
			t.Fatalf("omitted port poisoned: %+v", got)
		}
	}
	at.Offset = mustRational(t, 4, 1)
	record.Position = stateKnown(at)
	record.Instrument = stateKnown(id)
	record.Value = stateKnown(midiDeviceState{Name: stateKnown("repaired")})
	if err := h.addMIDIDeviceRecord(ctx, s, record, 0); err != nil {
		t.Fatal(err)
	}
	got, err := h.effectiveMIDIDeviceStatements(ctx, s, at, id)
	if err != nil || got.Name.Value.State != EvidenceKnown || got.Name.Value.Value != "repaired" {
		t.Fatalf("same-measure repair failed: %+v %v", got, err)
	}
	other, err := h.effectiveMIDIDeviceStatements(ctx, s, at, NodeRef{"d", 72})
	if err != nil || other.Name.Value.State != EvidenceInvalid {
		t.Fatalf("specific repair leaked to another instrument: %+v %v", other, err)
	}
	if len(h.MIDIDevices) != 2 || h.MIDIDevices[0].Value.Value.Name.Value != "bad-target" || h.MIDIDevices[0].Instrument.Cause.Rule.ID != "target" {
		t.Fatal("raw target/value lost")
	}
}

func TestTimelinePointGapOrderingAndOwnedPosition(t *testing.T) {
	ctx := context.Background()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 2, 1)
	g := stateGap{Position: &at, Part: at.Part, FirstMeasure: 0, LastMeasure: 0, Scope: stateScope{AllStaves: true}, Availability: stateUnknown[bool]("exact-gap")}
	if err := addStateGap(ctx, s, &h.Divisions, g); err != nil {
		t.Fatal(err)
	}
	at.Offset = mustRational(t, 100, 1)
	q := statePosition(t, 0, 1, 1)
	got, err := queryState(ctx, s, h, &h.Divisions, q, stateKnown(stateOne()), func(v Decimal) Evidence[Decimal] { return stateKnown(v) })
	if err != nil || got.Value.State != EvidenceAbsent {
		t.Fatalf("future gap poisoned prefix: %+v %v", got, err)
	}
	q.Offset = mustRational(t, 2, 1)
	prior, priorErr := queryStateAt(ctx, s, h, &h.Divisions, q, stateKnown(stateOne()), func(v Decimal) Evidence[Decimal] { return stateKnown(v) }, false)
	if priorErr != nil || prior.Value.State != EvidenceAbsent {
		t.Fatalf("exclusive query included point gap: %+v %v", prior, priorErr)
	}
	if err := publishState(ctx, s, &h.Divisions, statePublication[Decimal]{Position: q, Scope: stateScope{AllStaves: true}, Value: stateKnown(stateDecimal(t, 3, 1))}); err != nil {
		t.Fatal(err)
	}
	got, err = queryState(ctx, s, h, &h.Divisions, q, stateKnown(stateOne()), func(v Decimal) Evidence[Decimal] { return stateKnown(v) })
	if err != nil || got.Value.State != EvidenceUnknown {
		t.Fatalf("equal position silently repaired gap: %+v %v", got, err)
	}
	q.Offset = mustRational(t, 3, 1)
	if err := publishState(ctx, s, &h.Divisions, statePublication[Decimal]{Position: q, Scope: stateScope{AllStaves: true}, Value: stateKnown(stateDecimal(t, 4, 1))}); err != nil {
		t.Fatal(err)
	}
	got, err = queryState(ctx, s, h, &h.Divisions, q, stateKnown(stateOne()), func(v Decimal) Evidence[Decimal] { return stateKnown(v) })
	if err != nil || got.Value.State != EvidenceKnown || got.Value.Value.String() != "4" {
		t.Fatalf("later exact repair failed: %+v %v", got, err)
	}
}

func TestTimelinePointGapPreservesStrongerSelectedCause(t *testing.T) {
	ctx := context.Background()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 2, 1)
	id := NodeRef{"d", 71}
	bad := instrumentRecord[instrumentState]{Part: at.Part, Source: NodeRef{"d", 90}, Instrument: stateUnknown[NodeRef]("target-unavailable"), Position: stateKnown(at), Value: stateKnown(instrumentState{Sound: stateKnown("possible")})}
	if err := h.addUnresolvedInstrumentChange(ctx, s, bad, 0); err != nil {
		t.Fatal(err)
	}
	actual := Evidence[string]{State: EvidenceInvalid, Reason: "actual-scalar", Cause: &DecisionRef{Rule: RuleKey{ID: "scalar", Revision: 1}, Subject: NodeRef{"d", 91}}}
	if err := h.addInstrumentChange(ctx, s, id, statePublication[instrumentState]{Position: at, Scope: stateScope{AllStaves: true}, Value: stateKnown(instrumentState{Sound: actual})}); err != nil {
		t.Fatal(err)
	}
	got, err := h.effectiveInstrumentStatements(ctx, s, at, id)
	if err != nil || got.Sound.Value.State != EvidenceInvalid || got.Sound.Value.Cause == nil || got.Sound.Value.Cause.Rule.ID != "scalar" {
		t.Fatalf("gap hid stronger selected cause: %+v %v", got.Sound, err)
	}
	for _, reverse := range []bool{false, true} {
		history := stateHistory[Decimal]{}
		gaps := []Evidence[bool]{stateUnknown[bool]("unknown-gap"), {State: EvidenceInvalid, Reason: "invalid-gap", Cause: actual.Cause}}
		if reverse {
			gaps[0], gaps[1] = gaps[1], gaps[0]
		}
		for _, v := range gaps {
			if err := addStateGap(ctx, s, &history, stateGap{Position: &at, Part: at.Part, FirstMeasure: 0, LastMeasure: 0, Scope: stateScope{AllStaves: true}, Availability: v}); err != nil {
				t.Fatal(err)
			}
		}
		result, err := queryState(ctx, s, h, &history, at, stateKnown(stateOne()), func(v Decimal) Evidence[Decimal] { return stateKnown(v) })
		if err != nil || result.Value.State != EvidenceInvalid || result.Value.Cause == nil || result.Value.Cause.Rule.ID != "scalar" {
			t.Fatalf("gap order hid stronger cause: %+v %v", result, err)
		}
	}
}

func TestTimelineConditionalStatementDoesNotRepairInvalidTargetGap(t *testing.T) {
	ctx := context.Background()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 2, 1)
	id := NodeRef{"d", 71}
	cause := &DecisionRef{Rule: RuleKey{ID: "target", Revision: 1}, Subject: NodeRef{"d", 90}}
	r := instrumentRecord[midiState]{Part: at.Part, Source: NodeRef{"d", 90}, Instrument: Evidence[NodeRef]{State: EvidenceInvalid, Reason: "target-invalid", Cause: cause}, Position: stateKnown(at), Value: stateKnown(midiState{Program: stateKnown(stateInt(t, 74))})}
	if err := h.addMIDIRecord(ctx, s, r, 0); err != nil {
		t.Fatal(err)
	}
	at.Offset = mustRational(t, 4, 1)
	condition := stateKnown("2")
	r.Position = stateKnown(at)
	r.Instrument = stateKnown(id)
	r.TimeOnly = &condition
	if err := h.addMIDIRecord(ctx, s, r, 0); err != nil {
		t.Fatal(err)
	}
	got, err := h.effectiveMIDIStatements(ctx, s, at, id)
	if err != nil || got.Program.Value.State != EvidenceInvalid || got.Program.Value.Cause == nil || got.Program.Value.Cause.Rule.ID != "target" {
		t.Fatalf("conditional statement falsely repaired target gap: %+v %v", got.Program, err)
	}
}

func TestTimelineConditionalUpdateDoesNotResurrectRepairedGap(t *testing.T) {
	ctx := context.Background()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 2, 1)
	id := NodeRef{"d", 71}
	r := instrumentRecord[midiState]{Part: at.Part, Source: NodeRef{"d", 90}, Instrument: Evidence[NodeRef]{State: EvidenceInvalid, Reason: "target-invalid"}, Position: stateKnown(at), Value: stateKnown(midiState{Program: stateKnown(stateInt(t, 74))})}
	if err := h.addMIDIRecord(ctx, s, r, 0); err != nil {
		t.Fatal(err)
	}
	at.Offset = mustRational(t, 3, 1)
	r.Position = stateKnown(at)
	r.Instrument = stateKnown(id)
	if err := h.addMIDIRecord(ctx, s, r, 0); err != nil {
		t.Fatal(err)
	}
	at.Offset = mustRational(t, 4, 1)
	condition := stateKnown("2")
	r.Position = stateKnown(at)
	r.TimeOnly = &condition
	if err := h.addMIDIRecord(ctx, s, r, 0); err != nil {
		t.Fatal(err)
	}
	got, err := h.effectiveMIDIStatements(ctx, s, at, id)
	if err != nil || got.Program.Value.State != EvidenceUnknown || got.Program.Value.Reason != "playback-repeat-pass-unavailable" {
		t.Fatalf("conditional update resurrected repaired gap: %+v %v", got.Program, err)
	}
}

func TestTimelineConditionalRepairSearchCancellation(t *testing.T) {
	ctx := context.Background()
	build := func() (*Session, *stateHistory[Integer], stateSelection) {
		s := rationalSession(t, Budgets{})
		history := &stateHistory[Integer]{}
		condition := stateKnown("2")
		for i := int64(0); i < 32; i++ {
			p := statePublication[Integer]{Position: statePosition(t, 0, i, 1), Scope: stateScope{AllStaves: true}, Value: stateKnown(stateInt(t, i+1))}
			if i != 0 {
				p.TimeOnly = &condition
			}
			if err := publishState(ctx, s, history, p); err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				p.TimeOnly = &condition
				if err := publishState(ctx, s, history, p); err != nil {
					t.Fatal(err)
				}
			}
		}
		selection, err := selectState(ctx, s, history, statePosition(t, 0, 32, 1), stateOne(), true)
		if err != nil {
			t.Fatal(err)
		}
		return s, history, selection
	}
	probe, history, selection := build()
	counter := &rationalCancelContext{Context: ctx, at: math.MaxInt}
	repair, err := stateRepairPosition(counter, probe, history, stateOne(), selection)
	if err != nil || repair == nil || repair.Offset.String() != "0" {
		t.Fatalf("repair predecessor lost: %+v %v", repair, err)
	}
	calls := counter.calls
	probe.cleanup()
	// Every boundary gets a fresh ordinary budget; no cumulative quota is relaxed
	// merely to accommodate repeated independent cancellation experiments.
	for at := 1; at <= calls; at++ {
		s, history, selection := build()
		before := rationalUsage(t, s, BudgetBufferedBytes)
		index := history.index
		got, err := stateRepairPosition(&rationalCancelContext{Context: ctx, at: at}, s, history, stateOne(), selection)
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatalf("cancel boundary %d: %+v %v", at, got, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != before || history.index != index || len(history.Publications) != 33 || history.Publications[1].TimeOnly.Value != "2" {
			t.Fatal("repair lookup mutated or retained raw/scratch state")
		}
		repair, err = stateRepairPosition(ctx, s, history, stateOne(), selection)
		if err != nil || repair == nil || repair.Offset.String() != "0" {
			t.Fatalf("retry lost repair: %+v %v", repair, err)
		}
		s.cleanup()
	}
	t.Logf("conditional suffix cancellation boundaries: %d", calls)
}

func TestTimelineKnownPositionHiddenInstrumentPublisherStartsAtPoint(t *testing.T) {
	ctx := context.Background()
	s, svc, b := runtimeFixture(t)
	h := &svc.State
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 2, 1)
	id := NodeRef{"d", 71}
	sound := stateNode(b, timelineRoleSound, 90, stateAbsent[Atomic]())
	sound.end.ChildrenComplete = testEvidence(stateUnknown[bool]("hidden-publisher"))
	if err := h.readInstrumentChanges(ctx, s, b, sound, at); err != nil {
		t.Fatal(err)
	}
	for _, q := range []int64{1, 3} {
		at.Offset = mustRational(t, q, 1)
		got, err := h.effectiveInstrumentStatements(ctx, s, at, id)
		if err != nil {
			t.Fatal(err)
		}
		midi, err := h.effectiveMIDIStatements(ctx, s, at, id)
		if err != nil {
			t.Fatal(err)
		}
		want := EvidenceAbsent
		if q == 3 {
			want = EvidenceUnknown
		}
		if got.Sound.Value.State != want || got.Performance.Value.State != want || got.Virtual.Value.State != want || midi.Program.Value.State != want {
			t.Fatalf("q%d hidden publisher locality: instrument=%+v midi=%+v", q, got, midi.Program)
		}
	}
}
