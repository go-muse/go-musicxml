package validation

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestTimelineStatementIndexCancellationRetryAndByteQuota(t *testing.T) {
	build := func(t *testing.T) (*Session, *stateHistory[Integer]) {
		t.Helper()
		s, raw := stateIndexFixture(t, Budgets{})
		if _, err := stateIndex(context.Background(), s, raw); err != nil {
			t.Fatal(err)
		}
		return s, raw
	}
	project := func(v Integer) Evidence[Integer] { return stateKnown(v) }
	probe, raw := build(t)
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	var cache *stateHistoryIndex
	if _, err := statementIndex(counter, probe, raw, &cache, project); err != nil {
		t.Fatal(err)
	}
	calls := counter.calls
	probe.cleanup()
	for at := 1; at <= calls; at++ {
		s, raw := build(t)
		before := rationalUsage(t, s, BudgetBufferedBytes)
		var cache *stateHistoryIndex
		_, err := statementIndex(&rationalCancelContext{Context: context.Background(), at: at}, s, raw, &cache, project)
		if !errors.Is(err, context.Canceled) || cache != nil || rationalUsage(t, s, BudgetBufferedBytes) != before {
			t.Fatalf("cancel %d left partial cache or bytes: err=%v cache=%p delta=%d", at, err, cache, rationalUsage(t, s, BudgetBufferedBytes)-before)
		}
		if _, err = statementIndex(context.Background(), s, raw, &cache, project); err != nil || cache == nil {
			t.Fatalf("retry=%v cache=%p", err, cache)
		}
		s.cleanup()
	}
	s, raw := build(t)
	defer s.cleanup()
	before := rationalUsage(t, s, BudgetBufferedBytes)
	limits := s.account.Limits()
	remaining := limits.MaxBufferedBytes - before - 1
	if err := s.retain(remaining); err != nil {
		t.Fatal(err)
	}
	cache = nil
	_, err := statementIndex(context.Background(), s, raw, &cache, project)
	if !errors.Is(err, ErrResourceLimit) || cache != nil || rationalUsage(t, s, BudgetBufferedBytes) != before+remaining {
		t.Fatalf("byte failure mutated cache=%p err=%v", cache, err)
	}
}

func TestTimelineStatementIndexInvalidationAndOwnedProvenance(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 1)
	id := NodeRef{"d", 71}
	at := statePosition(t, 0, 0, 1)
	p := statePublication[instrumentState]{Position: at, Scope: stateScope{AllStaves: true}, Value: stateKnown(instrumentState{Sound: stateKnown("first")}), Provenance: []stateProvenance{{Node: NodeRef{"d", 90}, Kind: stateExplicit}}}
	if err := h.addInstrumentChange(context.Background(), s, id, p); err != nil {
		t.Fatal(err)
	}
	got, err := h.effectiveInstrumentStatements(context.Background(), s, at, id)
	if err != nil {
		t.Fatal(err)
	}
	index := h.InstrumentChanges[0].statementIndices[0]
	if index == nil {
		t.Fatal("no cached statement index")
	}
	got.Sound.Provenance[0].Node.Node = 999
	again, err := h.effectiveInstrumentStatements(context.Background(), s, at, id)
	if err != nil || again.Sound.Provenance[0].Node.Node != 90 {
		t.Fatalf("aliased provenance: %+v %v", again, err)
	}
	p.Position = statePosition(t, 1, 0, 1)
	p.Value.Value.Sound = stateKnown("second")
	if err := h.addInstrumentChange(context.Background(), s, id, p); err != nil {
		t.Fatal(err)
	}
	for _, index := range h.InstrumentChanges[0].statementIndices {
		if index != nil {
			t.Fatal("append retained stale property index")
		}
	}
	got, err = h.effectiveInstrumentStatements(context.Background(), s, p.Position, id)
	if err != nil || got.Sound.Value.Value != "second" || h.InstrumentChanges[0].statementIndices[0] == index {
		t.Fatalf("append query stale: %+v err=%v", got, err)
	}
}

func TestTimelineMIDIStatementBudgetsAndFullUnsignedIdentity(t *testing.T) {
	for _, budgets := range []Budgets{{MaxContextFacts: 1}, {MaxBufferedBytes: 1}, {MaxRuleSteps: 1}} {
		s := rationalSession(t, budgets)
		h := &timelineState{}
		at := statePosition(t, 0, 0, 1)
		r := instrumentRecord[midiState]{Part: at.Part, Source: NodeRef{"d", 90}, Instrument: stateKnown(NodeRef{"d", math.MaxUint64}), Position: stateKnown(at), Value: stateKnown(midiState{Program: stateKnown(stateInt(t, 128))})}
		err := h.addMIDIRecord(context.Background(), s, r, 0)
		if !errors.Is(err, ErrResourceLimit) || len(h.MIDI) != 0 {
			t.Fatalf("budget=%+v err=%v raw=%d", budgets, err, len(h.MIDI))
		}
		s.cleanup()
	}
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 0, 1)
	id := NodeRef{"d", math.MaxUint64}
	r := instrumentRecord[midiState]{Part: at.Part, Source: NodeRef{"d", 90}, Instrument: stateKnown(id), Position: stateKnown(at), Value: stateKnown(midiState{Program: stateKnown(stateInt(t, 128))})}
	if err := h.addMIDIRecord(context.Background(), s, r, 0); err != nil {
		t.Fatal(err)
	}
	got, err := h.effectiveMIDIStatements(context.Background(), s, at, id)
	if err != nil || got.Program.Value.Value.String() != "128" {
		t.Fatalf("unsigned identity narrowed: %+v err=%v", got, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.effectiveMIDIStatements(canceled, s, at, id); !errors.Is(err, context.Canceled) {
		t.Fatalf("cached query ignored cancellation: %v", err)
	}
}

func TestTimelineMIDIStatementLargeCauseIsOwned(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 0, 1)
	id := NodeRef{"d", 71}
	cause := &DecisionRef{Rule: RuleKey{ID: RuleID(strings.Repeat("cause", 1000)), Revision: 1}, Subject: NodeRef{"d", 90}}
	r := instrumentRecord[midiState]{Part: at.Part, Source: NodeRef{"d", 90}, Instrument: stateKnown(id), Position: stateKnown(at), Value: stateKnown(midiState{Program: Evidence[Integer]{State: EvidenceInvalid, Reason: Code(strings.Repeat("reason", 1000)), Cause: cause}})}
	if err := h.addMIDIRecord(context.Background(), s, r, 0); err != nil {
		t.Fatal(err)
	}
	cause.Subject.Node = 999
	before := rationalUsage(t, s, BudgetBufferedBytes)
	got, err := h.effectiveMIDIStatements(context.Background(), s, at, id)
	if err != nil || got.Program.Value.Cause == nil || got.Program.Value.Cause.Subject.Node != 90 || rationalUsage(t, s, BudgetBufferedBytes)-before < 11000 {
		t.Fatalf("unowned or unaccounted cause: %+v err=%v delta=%d", got, err, rationalUsage(t, s, BudgetBufferedBytes)-before)
	}
}

func TestTimelineSparseInstrumentAndMIDITrajectoryCosts(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	id := NodeRef{"d", 71}
	at := statePosition(t, 0, 0, 1)
	for i := 0; i < 128; i++ {
		pos := statePosition(t, 0, int64(i), 1)
		if err := h.addInstrumentChange(context.Background(), s, id, statePublication[instrumentState]{Position: pos, Scope: stateScope{AllStaves: true}, Value: stateKnown(instrumentState{Sound: stateKnown("flute")})}); err != nil {
			t.Fatal(err)
		}
		if err := h.addMIDIRecord(context.Background(), s, instrumentRecord[midiState]{Part: at.Part, Source: NodeRef{"d", NodeID(100 + i)}, Instrument: stateKnown(id), Position: stateKnown(pos), Value: stateKnown(midiState{Program: stateKnown(stateInt(t, 74))})}, 0); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 128; i++ {
		pos := statePosition(t, 0, int64(i), 1)
		if got, err := h.effectiveInstrumentStatements(context.Background(), s, pos, id); err != nil || got.Sound.Value.Value != "flute" {
			t.Fatalf("instrument %d: %+v %v", i, got, err)
		}
		if got, err := h.effectiveMIDIStatements(context.Background(), s, pos, id); err != nil || got.Program.Value.Value.String() != "74" {
			t.Fatalf("MIDI %d: %+v %v", i, got, err)
		}
	}
	t.Logf("128 paired publications+queries: work=%d bytes=%d contexts=%d", rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetBufferedBytes), rationalUsage(t, s, BudgetContextFacts))
}
