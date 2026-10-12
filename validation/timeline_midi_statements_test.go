package validation

import (
	"context"
	"testing"
)

func TestTimelineMIDISparseStatementsAndDeviceScope(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 3)
	at := statePosition(t, 0, 0, 1)
	id := NodeRef{"d", 71}
	other := NodeRef{"d", 72}
	initial := instrumentRecord[midiState]{Part: at.Part, Source: NodeRef{"d", 80}, Instrument: stateKnown(id), Position: stateAbsent[musicalPosition](), Value: stateKnown(midiState{Channel: stateKnown(stateInt(t, 1)), Program: stateKnown(stateInt(t, 74)), Pan: stateKnown(stateDecimal(t, 0, 1))})}
	if err := h.addMIDIRecord(context.Background(), s, initial, 0); err != nil {
		t.Fatal(err)
	}
	change := initial
	change.Source.Node = 81
	change.Position = stateKnown(statePosition(t, 1, 0, 1))
	change.Value = stateKnown(midiState{Pan: stateKnown(stateDecimal(t, -80, 1)), Elevation: stateKnown(stateDecimal(t, 45, 1))})
	if err := h.addMIDIRecord(context.Background(), s, change, 1); err != nil {
		t.Fatal(err)
	}
	got, err := h.effectiveMIDIStatements(context.Background(), s, statePosition(t, 2, 0, 1), id)
	if err != nil || got.Program.Value.Value.String() != "74" || got.Channel.Value.Value.String() != "1" || got.Pan.Value.Value.String() != "-80" || got.Elevation.Value.Value.String() != "45" || got.Bank.Value.State != EvidenceAbsent || got.Program.Provenance[0].Node.Node != 80 || got.Pan.Provenance[0].Node.Node != 81 {
		t.Fatalf("MIDI sparse statements: %+v err=%v", got, err)
	}
	device := instrumentRecord[midiDeviceState]{Part: at.Part, Source: NodeRef{"d", 90}, Instrument: stateKnown(id), Position: stateKnown(statePosition(t, 0, 0, 1)), Value: stateKnown(midiDeviceState{Name: stateKnown("old")})}
	if err := h.addMIDIDeviceRecord(context.Background(), s, device, 0); err != nil {
		t.Fatal(err)
	}
	device.Source.Node = 91
	device.Instrument = stateAbsent[NodeRef]()
	device.Position = stateKnown(statePosition(t, 1, 0, 1))
	device.Value = stateKnown(midiDeviceState{Name: stateKnown("all"), Port: stateKnown(stateInt(t, 2))})
	scope, err := midiDeviceScope(context.Background(), s, device)
	if err != nil || scope.State != EvidenceKnown || !scope.Value.AllInstruments || scope.Value.Part != at.Part {
		t.Fatalf("omitted ID scope: %+v err=%v", scope, err)
	}
	if err := h.addMIDIDeviceRecord(context.Background(), s, device, 1); err != nil {
		t.Fatal(err)
	}
	for _, target := range []NodeRef{id, other} {
		g, err := h.effectiveMIDIDeviceStatements(context.Background(), s, statePosition(t, 2, 0, 1), target)
		if err != nil || g.Name.Value.Value != "all" || g.Port.Value.Value.String() != "2" {
			t.Fatalf("all-target precedence: %+v err=%v", g, err)
		}
	}
	device.Source.Node = 92
	device.Instrument = stateKnown(id)
	device.Value = stateKnown(midiDeviceState{Name: stateKnown("specific")})
	if err := h.addMIDIDeviceRecord(context.Background(), s, device, 1); err != nil {
		t.Fatal(err)
	}
	g, err := h.effectiveMIDIDeviceStatements(context.Background(), s, statePosition(t, 2, 0, 1), id)
	if err != nil || g.Name.Value.State != EvidenceUnknown || g.Port.Value.Value.String() != "2" {
		t.Fatalf("invented specificity winner: %+v err=%v", g, err)
	}
}

func TestTimelineMIDIUnplacedFieldGapAndRepair(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 4)
	at := statePosition(t, 0, 0, 1)
	id := NodeRef{"d", 71}
	record := instrumentRecord[midiState]{Part: at.Part, Source: NodeRef{"d", 80}, Instrument: stateKnown(id), Position: stateAbsent[musicalPosition](), Value: stateKnown(midiState{Channel: stateKnown(stateInt(t, 1)), Program: stateKnown(stateInt(t, 74))})}
	if err := h.addMIDIRecord(context.Background(), s, record, 0); err != nil {
		t.Fatal(err)
	}
	cause := &DecisionRef{Rule: RuleKey{ID: "offset", Revision: 1}, Subject: NodeRef{"d", 81}}
	record.Source.Node = 81
	record.Position = Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "bad-offset", Cause: cause}
	record.Value = stateKnown(midiState{Program: stateKnown(stateInt(t, 72))})
	if err := h.addMIDIRecord(context.Background(), s, record, 1); err != nil {
		t.Fatal(err)
	}
	record.Source.Node = 82
	record.Position = stateKnown(statePosition(t, 2, 0, 1))
	record.Value = stateKnown(midiState{Pan: stateKnown(stateDecimal(t, 20, 1))})
	if err := h.addMIDIRecord(context.Background(), s, record, 2); err != nil {
		t.Fatal(err)
	}
	got, err := h.effectiveMIDIStatements(context.Background(), s, statePosition(t, 2, 1, 1), id)
	if err != nil || got.Program.Value.State != EvidenceInvalid || got.Program.Value.Cause == nil || got.Program.Value.Cause.Subject.Node != 81 || got.Channel.Value.Value.String() != "1" || got.Pan.Value.Value.String() != "20" {
		t.Fatalf("gap locality: %+v err=%v", got, err)
	}
	got.Program.Value.Cause.Subject.Node = 999
	record.Source.Node = 83
	record.Position = stateKnown(statePosition(t, 3, 0, 1))
	record.Value = stateKnown(midiState{Program: stateKnown(stateInt(t, 80))})
	if err := h.addMIDIRecord(context.Background(), s, record, 3); err != nil {
		t.Fatal(err)
	}
	got, err = h.effectiveMIDIStatements(context.Background(), s, statePosition(t, 4, 0, 1), id)
	if err != nil || got.Program.Value.Value.String() != "80" || got.Channel.Value.Value.String() != "1" || cause.Subject.Node != 81 {
		t.Fatalf("repair or ownership: %+v err=%v", got, err)
	}
}

func TestTimelineMIDIBroadPublisherGapAndPartIsolation(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 3)
	at := statePosition(t, 0, 0, 1)
	id := NodeRef{"d", 71}
	record := instrumentRecord[midiDeviceState]{Part: at.Part, Source: NodeRef{"d", 80}, Instrument: stateAbsent[NodeRef](), Position: stateAbsent[musicalPosition](), Value: stateKnown(midiDeviceState{Name: stateKnown("initial"), Port: stateKnown(stateInt(t, 1))})}
	if err := h.addMIDIDeviceRecord(context.Background(), s, record, 0); err != nil {
		t.Fatal(err)
	}
	if err := h.gapInstrumentPublisher(context.Background(), s, at.Part, 1, stateUnknown[musicalPosition]("hidden-sound")); err != nil {
		t.Fatal(err)
	}
	record.Source.Node = 81
	record.Position = stateKnown(statePosition(t, 2, 0, 1))
	record.Value = stateKnown(midiDeviceState{Port: stateKnown(stateInt(t, 2))})
	if err := h.addMIDIDeviceRecord(context.Background(), s, record, 2); err != nil {
		t.Fatal(err)
	}
	got, err := h.effectiveMIDIDeviceStatements(context.Background(), s, statePosition(t, 3, 0, 1), id)
	if err != nil || got.Name.Value.State != EvidenceUnknown || got.Name.Value.Reason != "hidden-sound" || got.Port.Value.Value.String() != "2" {
		t.Fatalf("global hole erased by unrelated field: %+v err=%v", got, err)
	}
	other := statePosition(t, 3, 0, 1)
	other.Part.Node = 200
	other.Measure.Node = 201
	if err := h.closePart(context.Background(), s, other.Part, 3, stateKnown(true)); err != nil {
		t.Fatal(err)
	}
	got, err = h.effectiveMIDIDeviceStatements(context.Background(), s, other, NodeRef{"d", 201})
	if err != nil || got.Name.Value.State != EvidenceAbsent || got.Port.Value.State != EvidenceAbsent {
		t.Fatalf("part scope escaped: %+v err=%v", got, err)
	}
}

func TestTimelineMIDIInitialPreservesStrongestCause(t *testing.T) {
	for _, fieldInvalid := range []bool{false, true} {
		s := rationalSession(t, Budgets{})
		h := &timelineState{}
		stateClose(t, s, h, 0)
		at := statePosition(t, 0, 0, 1)
		id := NodeRef{"d", 71}
		cause := &DecisionRef{Rule: RuleKey{ID: "actual-invalid", Revision: 1}, Subject: NodeRef{"d", 90}}
		target := Evidence[NodeRef]{State: EvidenceInvalid, Reason: "actual-invalid", Cause: cause}
		program := stateUnknown[Integer]("unavailable")
		if fieldInvalid {
			target = stateUnknown[NodeRef]("unavailable")
			program = Evidence[Integer]{State: EvidenceInvalid, Reason: "actual-invalid", Cause: cause}
		}
		r := instrumentRecord[midiState]{Part: at.Part, Source: NodeRef{"d", 90}, Instrument: target, Position: stateAbsent[musicalPosition](), Value: stateKnown(midiState{Program: program})}
		if err := h.addMIDIRecord(context.Background(), s, r, 0); err != nil {
			t.Fatal(err)
		}
		got, err := h.effectiveMIDIStatements(context.Background(), s, at, id)
		if err != nil || got.Program.Value.State != EvidenceInvalid || got.Program.Value.Cause == nil || got.Program.Value.Cause.Subject.Node != 90 {
			t.Fatalf("fieldInvalid=%v strongest cause lost: %+v err=%v", fieldInvalid, got, err)
		}
		s.cleanup()
	}
}
