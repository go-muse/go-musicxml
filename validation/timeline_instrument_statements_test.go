package validation

import (
	"context"
	"testing"
)

func TestTimelineInstrumentSparseStatements(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 3)
	id := NodeRef{"d", 71}
	at := statePosition(t, 0, 0, 1)
	initial := instrumentState{Sound: stateKnown("flute"), Ensemble: stateKnown(ensembleState{Number: stateInt(t, 12)}), Virtual: stateKnown(virtualInstrumentState{Library: stateKnown("A"), Name: stateKnown("Flute")})}
	if err := h.addInstrumentDefinition(context.Background(), s, instrumentDefinition{Part: at.Part, Instrument: id, Value: stateKnown(initial), Provenance: []stateProvenance{{Node: id, Kind: stateExplicit}}}); err != nil {
		t.Fatal(err)
	}
	publish := func(measure uint64, v instrumentState) {
		t.Helper()
		if err := h.addInstrumentChange(context.Background(), s, id, statePublication[instrumentState]{Position: statePosition(t, measure, 0, 1), Scope: stateScope{AllStaves: true}, Value: stateKnown(v), Provenance: []stateProvenance{{Node: NodeRef{"d", NodeID(100 + measure)}, Kind: stateExplicit}}}); err != nil {
			t.Fatal(err)
		}
	}
	publish(1, instrumentState{Sound: stateKnown("saxophone")})
	got, err := h.effectiveInstrument(context.Background(), s, statePosition(t, 1, 1, 1), id)
	if err != nil || got.Value.State != EvidenceKnown || got.Value.Value.Ensemble.State != EvidenceKnown || got.Value.Value.Ensemble.Value.Number.String() != "12" || got.Value.Value.Virtual.State != EvidenceKnown || got.Value.Value.Virtual.Value.Name.Value != "Flute" {
		t.Fatalf("sparse field loss: %+v err=%v", got, err)
	}
	publish(2, instrumentState{Solo: stateKnown(true), Virtual: stateKnown(virtualInstrumentState{Library: stateKnown("B"), Name: stateAbsent[string]()})})
	got, err = h.effectiveInstrument(context.Background(), s, statePosition(t, 2, 1, 1), id)
	if err != nil || got.Value.Value.Sound.Value != "saxophone" || got.Value.Value.Solo.State != EvidenceKnown || got.Value.Value.Ensemble.State != EvidenceAbsent || got.Value.Value.Virtual.Value.Name.State != EvidenceAbsent {
		t.Fatalf("choice or atomic descriptor wrong: %+v err=%v", got, err)
	}
	publish(3, instrumentState{Ensemble: stateKnown(ensembleState{Empty: true})})
	got, err = h.effectiveInstrument(context.Background(), s, statePosition(t, 3, 1, 1), id)
	if err != nil || got.Value.Value.Solo.State != EvidenceAbsent || got.Value.Value.Ensemble.State != EvidenceKnown || !got.Value.Value.Ensemble.Value.Empty || got.Value.Value.Ensemble.Value.Number.Valid() {
		t.Fatalf("empty ensemble inherited old size: %+v err=%v", got, err)
	}
}

func TestTimelineInstrumentDisjointEqualPositionStatements(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	id := NodeRef{"d", 71}
	at := statePosition(t, 0, 0, 1)
	for _, v := range []instrumentState{{Sound: stateKnown("flute")}, {Solo: stateKnown(true)}} {
		if err := h.addInstrumentChange(context.Background(), s, id, statePublication[instrumentState]{Position: at, Scope: stateScope{AllStaves: true}, Value: stateKnown(v)}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := h.effectiveInstrument(context.Background(), s, at, id)
	if err != nil || got.Value.State != EvidenceKnown || got.Value.Value.Sound.Value != "flute" || got.Value.Value.Solo.State != EvidenceKnown || got.Value.Value.Virtual.State != EvidenceAbsent {
		t.Fatalf("disjoint updates conflict or invent value: %+v err=%v", got, err)
	}
}

func TestTimelineInstrumentSparseGapIsNotClearedByUnrelatedField(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 3)
	id := NodeRef{"d", 71}
	at := statePosition(t, 0, 0, 1)
	cause := &DecisionRef{Rule: RuleKey{ID: "gap", Revision: 1}, Subject: NodeRef{"d", 88}}
	h.InstrumentGaps = []stateGap{{Part: at.Part, FirstMeasure: 1, LastMeasure: 1, Scope: stateScope{AllStaves: true}, Availability: Evidence[bool]{State: EvidenceInvalid, Reason: "missing-change", Cause: cause}}}
	if err := h.addInstrumentChange(context.Background(), s, id, statePublication[instrumentState]{Position: statePosition(t, 2, 0, 1), Scope: stateScope{AllStaves: true}, Value: stateKnown(instrumentState{Sound: stateKnown("repaired")})}); err != nil {
		t.Fatal(err)
	}
	got, err := h.effectiveInstrument(context.Background(), s, statePosition(t, 3, 0, 1), id)
	if err != nil || got.Value.State != EvidenceKnown || got.Value.Value.Sound.Value != "repaired" || got.Value.Value.Virtual.State != EvidenceInvalid || got.Value.Value.Virtual.Cause == nil || got.Value.Value.Virtual.Cause.Subject.Node != 88 {
		t.Fatalf("unrelated publication cleared gap: %+v err=%v", got, err)
	}
	got.Value.Value.Virtual.Cause.Subject.Node = 999
	again, err := h.effectiveInstrument(context.Background(), s, statePosition(t, 3, 0, 1), id)
	if err != nil || again.Value.Value.Virtual.Cause.Subject.Node != 88 {
		t.Fatalf("aliased cause: %+v err=%v", again, err)
	}
}

func TestTimelineInstrumentPropertySourcesConflictAndGap(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 3)
	id := NodeRef{"d", 71}
	at := statePosition(t, 0, 0, 1)
	if err := h.addInstrumentDefinition(context.Background(), s, instrumentDefinition{Part: at.Part, Instrument: id, Value: stateKnown(instrumentState{Virtual: stateKnown(virtualInstrumentState{Library: stateKnown("A")})}), Provenance: []stateProvenance{{Node: id, Kind: stateExplicit}}}); err != nil {
		t.Fatal(err)
	}
	for i, sound := range []string{"flute", "saxophone"} {
		if err := h.addInstrumentChange(context.Background(), s, id, statePublication[instrumentState]{Position: statePosition(t, 1, 0, 1), Scope: stateScope{AllStaves: true}, Value: stateKnown(instrumentState{Sound: stateKnown(sound)}), Provenance: []stateProvenance{{Node: NodeRef{"d", NodeID(100 + i)}, Kind: stateExplicit}}}); err != nil {
			t.Fatal(err)
		}
	}
	fields, err := h.effectiveInstrumentStatements(context.Background(), s, statePosition(t, 2, 0, 1), id)
	if err != nil || fields.Sound.Value.State != EvidenceUnknown || fields.Sound.Value.Reason != "timeline-state-conflict" || fields.Virtual.Value.State != EvidenceKnown || len(fields.Virtual.Provenance) != 1 || fields.Virtual.Provenance[0].Node != id || !fields.Virtual.Inherited {
		t.Fatalf("field provenance/conflict: %+v err=%v", fields, err)
	}
	g := stateGap{Part: at.Part, FirstMeasure: 2, LastMeasure: 2, Scope: stateScope{AllStaves: true}, Availability: stateUnknown[bool]("unknown-position")}
	if err := h.InstrumentChanges[0].addStatementGap(context.Background(), s, g, stateKnown(instrumentState{Solo: stateKnown(true)})); err != nil {
		t.Fatal(err)
	}
	fields, err = h.effectiveInstrumentStatements(context.Background(), s, statePosition(t, 3, 0, 1), id)
	if err != nil || fields.Performance.Value.State != EvidenceUnknown || fields.Performance.Value.Reason != "unknown-position" || fields.Virtual.Value.State != EvidenceKnown {
		t.Fatalf("property-local gap escaped: %+v err=%v", fields, err)
	}
}

func TestTimelineInstrumentChoicePreservesStrongestCause(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{ID: "choice-shape", Revision: 1}, Subject: NodeRef{"d", 123}}
	for _, reverse := range []bool{false, true} {
		value := instrumentState{Solo: stateUnknown[bool]("unavailable"), Ensemble: Evidence[ensembleState]{State: EvidenceInvalid, Reason: "bad-choice", Cause: cause}}
		if reverse {
			value.Solo = Evidence[bool]{State: EvidenceInvalid, Reason: "bad-choice", Cause: cause}
			value.Ensemble = stateUnknown[ensembleState]("unavailable")
		}
		got := instrumentPerformanceStatement(value)
		if got.State != EvidenceInvalid || got.Cause != cause {
			t.Fatalf("reverse=%v lost stronger cause: %+v", reverse, got)
		}
	}
}
