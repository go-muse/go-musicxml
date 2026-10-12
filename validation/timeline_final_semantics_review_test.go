package validation

import (
	"context"
	"testing"
)

// The pinned MusicXML 4.0 sound annotation applies time-only to the sound
// element, including its instrument-change/MIDI/device children. A statement
// query without a repeat-pass argument cannot establish an unconditional value.
func TestTimelineFinalReviewConditionalMIDIDevice(t *testing.T) {
	for _, conditional := range []bool{false, true} {
		t.Run(map[bool]string{false: "unconditional", true: "time-only"}[conditional], func(t *testing.T) {
			s, svc, b := runtimeFixture(t)
			device := stateNode(b, timelineRoleSoundMidiDevice, 31, stateAtomicText("second-pass-device"))
			sound := stateNode(b, timelineRoleSound, 30, stateAbsent[Atomic](), device)
			if conditional {
				stateAttribute(b, sound, timelineRoleSoundTimeOnly, Atomic{Kind: AtomicString, Text: "2"})
			}
			attr := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
			if err := svc.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, attr, sound)); err != nil {
				t.Fatal(err)
			}
			if len(svc.State.MIDIDevices) != 1 {
				t.Fatalf("missing raw record: %+v", svc.State.MIDIDevices)
			}
			raw := svc.State.MIDIDevices[0]
			if conditional && (raw.TimeOnly == nil || raw.TimeOnly.State != EvidenceKnown || raw.TimeOnly.Value != "2") {
				t.Fatalf("lost raw device repeat condition: %+v", raw.TimeOnly)
			}
			if !conditional && raw.TimeOnly != nil {
				t.Fatalf("invented device repeat condition: %+v", raw.TimeOnly)
			}
			if raw.Value.State != EvidenceKnown || raw.Value.Value.Name.Value != "second-pass-device" {
				t.Fatalf("lost raw value: %+v", raw)
			}
			got, err := svc.State.effectiveMIDIDeviceStatements(context.Background(), s, svc.Facts[30].Position.Value, NodeRef{"d", 71})
			if err != nil {
				t.Fatal(err)
			}
			if conditional && got.Name.Value.State == EvidenceKnown {
				t.Fatalf("conditional sound became unconditional device: %+v", got)
			}
			if !conditional && (got.Name.Value.State != EvidenceKnown || got.Name.Value.Value != "second-pass-device") {
				t.Fatalf("unconditional device unavailable: %+v", got)
			}
		})
	}
}

// This small schema supplies real shared ID/IDREF resolution and assessed scalar
// causes. It is an extraction/query fixture, not an original-source adapter or
// a substitute MusicXML schema conformance oracle.
func finalReviewInstrumentFixture(t *testing.T, target Target, sounds ...*localTree) (*Session, *timelineState, *timelineBindings, []*factNode, musicalPosition, NodeRef) {
	t.Helper()
	types := map[SchemaName]*SchemaType{}
	add := func(name string, c *SchemaComplex) { types[SchemaName{Local: name}] = &SchemaType{Complex: c} }
	for _, name := range []string{"string", "integer", "decimal"} {
		types[SchemaName{Local: name}] = &SchemaType{Simple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: &SchemaSimpleMember{Name: localBuiltin(name).Name}}}
	}
	types[SchemaName{Local: "condition"}] = &SchemaType{Simple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: &SchemaSimpleMember{Name: localBuiltin("string").Name}, Enumerations: []string{"2"}}}
	id := func(typ string) SchemaAttribute {
		return SchemaAttribute{Name: SchemaName{Local: "id"}, Type: localBuiltin(typ), Use: SchemaAttributeRequired}
	}
	add("empty", &SchemaComplex{Form: SchemaComplexDirect})
	add("score-instrument", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{id("ID")}})
	add("score-part", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{id("ID")}, Particle: localChoice(localElement("score-instrument", "score-instrument"))})
	add("instrument-change", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{id("IDREF")}, Particle: localChoice(localElement("instrument-sound", "string"), localElement("solo", "empty"))})
	add("midi-instrument", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{id("IDREF")}, Particle: localChoice(localElement("midi-program", "integer"), localElement("pan", "decimal"))})
	add("sound", &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{{Name: SchemaName{Local: "time-only"}, Type: localType("condition")}}, Particle: localChoice(localElement("instrument-change", "instrument-change"), localElement("midi-instrument", "midi-instrument"))})
	add("root", &SchemaComplex{Form: SchemaComplexDirect, Particle: localChoice(localElement("score-part", "score-part"), localElement("sound", "sound"))})
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Types: types, Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: localType("root")}}})
	if err != nil {
		t.Fatal(err)
	}
	root := lt("root", la("score-part", "P1", la("score-instrument", "I1")))
	root.children = append(root.children, sounds...)
	s := localAdmit(t, localEngine(t, schema, target), target, root, nil)
	t.Cleanup(s.cleanup)
	if err = s.bindSourceSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.inputComplete = true
	if err = s.finalizeSourceSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	b := stateBindings()
	for role, slot := range map[timelineRole][2]string{
		timelineRoleScoreInstrument: {"score-part", "score-instrument"}, timelineRoleSound: {"root", "sound"}, timelineRoleSoundInstrumentChange: {"sound", "instrument-change"}, timelineRoleSoundMidiInstrument: {"sound", "midi-instrument"}, timelineRoleInstrumentChangeInstrumentSound: {"instrument-change", "instrument-sound"}, timelineRoleInstrumentChangeSolo: {"instrument-change", "solo"}, timelineRoleMidiInstrumentMidiProgram: {"midi-instrument", "midi-program"}, timelineRoleMidiInstrumentPan: {"midi-instrument", "pan"},
	} {
		b.Roles[role] = []SchemaUse{localUse(t, schema, slot[0], slot[1], false)}
	}
	for role, slot := range map[timelineRole][2]string{timelineRoleInstrumentChangeID: {"instrument-change", "id"}, timelineRoleMidiInstrumentID: {"midi-instrument", "id"}, timelineRoleSoundTimeOnly: {"sound", "time-only"}} {
		b.Roles[role] = []SchemaUse{localUse(t, schema, slot[0], slot[1], true)}
	}
	part := NodeRef{"doc", 2}
	instrument := NodeRef{"doc", 3}
	pos := musicalPosition{Part: part, Measure: NodeRef{"doc", 1}, Offset: mustRational(t, 0, 1)}
	h := &timelineState{}
	if err = h.closePart(context.Background(), s, part, 0, stateKnown(true)); err != nil {
		t.Fatal(err)
	}
	return s, h, b, s.root.children[1:], pos, instrument
}

func TestTimelineFinalReviewConditionalInstrumentProperties(t *testing.T) {
	ctx := context.Background()
	for _, target := range []Target{TargetSource, TargetModel} {
		for _, condition := range []string{"", "2", "invalid"} {
			t.Run(string(target)+"/"+condition, func(t *testing.T) {
				scalar := func(name, text string) *localTree { return &localTree{name: name, text: text} }
				changed := lt("sound", la("instrument-change", "I1", scalar("instrument-sound", "conditional-patch")), la("midi-instrument", "I1", scalar("midi-program", "74")))
				if condition != "" {
					changed.attrs = map[string]string{"time-only": condition}
				}
				other := lt("sound", la("instrument-change", "I1", lt("solo")), la("midi-instrument", "I1", scalar("pan", "20")))
				repair := lt("sound", la("instrument-change", "I1", scalar("instrument-sound", "unconditional-patch")), la("midi-instrument", "I1", scalar("midi-program", "41")))
				s, h, b, sounds, pos, id := finalReviewInstrumentFixture(t, target, changed, other, repair)
				for i, n := range sounds {
					at := pos
					at.Offset = mustRational(t, int64(i), 1)
					if err := h.readInstrumentChanges(ctx, s, b, n, at); err != nil {
						t.Fatal(err)
					}
				}
				at := pos
				at.Offset = mustRational(t, 1, 1)
				instrument, err := h.effectiveInstrumentStatements(ctx, s, at, id)
				if err != nil {
					t.Fatal(err)
				}
				midi, err := h.effectiveMIDIStatements(ctx, s, at, id)
				if err != nil {
					t.Fatal(err)
				}
				if condition == "" {
					if instrument.Sound.Value.State != EvidenceKnown || instrument.Sound.Value.Value != "conditional-patch" || midi.Program.Value.State != EvidenceKnown || midi.Program.Value.Value.String() != "74" {
						t.Fatalf("unconditional control: instrument=%+v midi=%+v", instrument, midi)
					}
				} else if condition == "2" {
					if instrument.Sound.Value.State == EvidenceKnown || midi.Program.Value.State == EvidenceKnown {
						t.Errorf("repeat-dependent statements became unconditional: instrument=%+v midi=%+v", instrument.Sound, midi.Program)
					}
				} else {
					for label, v := range map[string]Evidence[bool]{"instrument": stateUnavailable[string, bool](instrument.Sound.Value), "midi": stateUnavailable[Integer, bool](midi.Program.Value)} {
						if v.State != EvidenceInvalid || v.Cause == nil || v.Cause.Rule.ID != "xsd.attributes" || v.Cause.Subject.Node != sounds[0].header.ID {
							t.Errorf("%s lost condition cause: %+v", label, v)
						}
					}
				}
				if instrument.Performance.Value.State != EvidenceKnown || !instrument.Performance.Value.Value.Solo || midi.Pan.Value.State != EvidenceKnown || midi.Pan.Value.Value.String() != "20" {
					t.Errorf("unrelated unconditional fields lost: %+v %+v", instrument, midi)
				}
				if len(h.MIDI) != 3 || h.MIDI[0].Value.Value.Program.State != EvidenceKnown || h.MIDI[0].Value.Value.Program.Value.String() != "74" {
					t.Errorf("raw MIDI lost: %+v", h.MIDI)
				}
				if len(h.InstrumentChanges) != 1 || h.InstrumentChanges[0].History.Publications[0].Value.Value.Sound.State != EvidenceKnown || h.InstrumentChanges[0].History.Publications[0].Value.Value.Sound.Value != "conditional-patch" {
					t.Fatalf("lost raw instrument sound: %+v", h.InstrumentChanges)
				}
				for label, rawCondition := range map[string]*Evidence[string]{"instrument": h.InstrumentChanges[0].History.Publications[0].TimeOnly, "midi": h.MIDI[0].TimeOnly} {
					switch condition {
					case "":
						if rawCondition != nil {
							t.Errorf("%s invented raw condition: %+v", label, rawCondition)
						}
					case "2":
						if rawCondition == nil || rawCondition.State != EvidenceKnown || rawCondition.Value != "2" {
							t.Errorf("%s lost raw condition: %+v", label, rawCondition)
						}
					default:
						if rawCondition == nil || rawCondition.State != EvidenceInvalid || rawCondition.Cause == nil || rawCondition.Cause.Rule.ID != "xsd.attributes" {
							t.Errorf("%s lost raw invalid condition: %+v", label, rawCondition)
						}
					}
				}
				at.Offset = mustRational(t, 3, 1)
				instrument, err = h.effectiveInstrumentStatements(ctx, s, at, id)
				if err != nil {
					t.Fatal(err)
				}
				midi, err = h.effectiveMIDIStatements(ctx, s, at, id)
				if err != nil {
					t.Fatal(err)
				}
				if instrument.Sound.Value.State != EvidenceKnown || instrument.Sound.Value.Value != "unconditional-patch" || midi.Program.Value.State != EvidenceKnown || midi.Program.Value.Value.String() != "41" {
					t.Errorf("later unconditional same-field repair failed: instrument=%+v midi=%+v", instrument.Sound, midi.Program)
				}
			})
		}
	}
}

// Figured-bass anchors follow regular notes in score order. An unresolved
// trailing figure at q=2 cannot belong to a proved earlier group anchored at
// q=0, so it cannot erase that group's independent chain origin.
func TestTimelineFinalReviewLaterFigureGapKeepsEarlierChain(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	first := stateNode(b, timelineRoleFiguredBass, 25, stateAbsent[Atomic](), stateNode(b, timelineRoleFiguredBassFigure, 27, stateAbsent[Atomic]()), stateNode(b, timelineRoleFiguredBassDuration, 26, stateAtomicDecimal(t, 1, 1)))
	trailing := stateNode(b, timelineRoleFiguredBass, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleFiguredBassFigure, 42, stateAbsent[Atomic]()), stateNode(b, timelineRoleFiguredBassDuration, 41, stateAtomicDecimal(t, 1, 1)))
	block := runtimeBlock(b, 0, attrs, first, runtimeNote(t, b, 30, 2, false), trailing)
	if err := svc.readMeasure(context.Background(), s, b, block); err != nil {
		t.Fatal(err)
	}
	if err := svc.finalizeTimingOrigins(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
		t.Fatal(err)
	}
	early, late := svc.FiguredBassOrigins[25], svc.FiguredBassOrigins[40]
	if early.Anchor.State != EvidenceKnown || early.Anchor.Value.Node != 30 || early.AnchorPosition.Value.Offset.String() != "0" || late.CursorOrigin.Value.Offset.String() != "2" {
		t.Fatalf("fixture facts wrong: early=%+v late=%+v", early, late)
	}
	if early.ChainPosition.State != EvidenceKnown || early.ChainPosition.Value.Offset.String() != "0" || early.ChainEnd.State != EvidenceKnown || early.ChainEnd.Value.Offset.String() != "1" {
		t.Fatalf("later unrelated anchor gap erased proved prefix chain: %+v", early)
	}
}

func TestTimelineFinalReviewLaterVoiceGapKeepsEarlierGraceDonors(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	voiced := func(id NodeID, duration int64) *factNode {
		n := runtimeNote(t, b, id, duration, false)
		n.children = append(n.children, stateNode(b, timelineRoleNoteVoice, id+3, stateAtomicText("one")))
		return n
	}
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	grace := stateNode(b, timelineRoleNoteGrace, 41, stateAbsent[Atomic]())
	stateAttribute(b, grace, timelineRoleGraceStealPrevious, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 25, 1)})
	stateAttribute(b, grace, timelineRoleGraceStealFollowing, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 25, 1)})
	note := stateNode(b, timelineRoleNote, 40, stateAbsent[Atomic](), grace, stateNode(b, timelineRoleNoteVoice, 43, stateAtomicText("one")))
	first := runtimeBlock(b, 0, attrs, voiced(30, 1), note, voiced(50, 1))
	bad := voiced(60, 1)
	cause := &DecisionRef{Rule: RuleKey{"later-voice-scalar", 1}, Subject: NodeRef{"d", 63}}
	bad.children[1].end.Scalar.Value = Evidence[Atomic]{State: EvidenceInvalid, Reason: "later-invalid-voice", Cause: cause}
	second := runtimeBlock(b, 1, bad)
	for _, block := range []timelineMeasureBlock{first, second} {
		if err := svc.readMeasure(context.Background(), s, b, block); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.finalizeTimingOrigins(context.Background(), s, b, []timelineMeasureBlock{first, second}); err != nil {
		t.Fatal(err)
	}
	got := svc.GraceNeighbors[40]
	if got.Previous.Note.State != EvidenceKnown || got.Previous.Note.Value.Node != 30 || got.Following.Note.State != EvidenceKnown || got.Following.Note.Value.Node != 50 {
		t.Fatalf("later-measure voice gap erased earlier closed donors: %+v", got)
	}
	if got.Previous.StolenDuration.State != EvidenceKnown || got.Previous.StolenDuration.Value.String() != "1/4" || got.Following.StolenDuration.State != EvidenceKnown || got.Following.StolenDuration.Value.String() != "1/4" {
		t.Fatalf("earlier exact stolen amounts lost: %+v", got)
	}
}

func TestTimelineFinalReviewKnownPositionInvalidInstrumentTargetIsLocal(t *testing.T) {
	ctx := context.Background()
	for _, target := range []Target{TargetSource, TargetModel} {
		t.Run(string(target), func(t *testing.T) {
			scalar := func(name, text string) *localTree { return &localTree{name: name, text: text} }
			first := lt("sound", la("instrument-change", "I1", scalar("instrument-sound", "first")), la("midi-instrument", "I1", scalar("midi-program", "74")))
			invalid := lt("sound", la("instrument-change", "MISSING", scalar("instrument-sound", "bad-target")), la("midi-instrument", "MISSING", scalar("midi-program", "2")))
			repair := lt("sound", la("instrument-change", "I1", scalar("instrument-sound", "repaired")), la("midi-instrument", "I1", scalar("midi-program", "41")))
			s, h, b, sounds, pos, id := finalReviewInstrumentFixture(t, target, first, invalid, repair)
			for i, n := range sounds {
				at := pos
				at.Offset = mustRational(t, int64(i*2), 1)
				if err := h.readInstrumentChanges(ctx, s, b, n, at); err != nil {
					t.Fatal(err)
				}
			}
			for _, q := range []int64{1, 3, 5} {
				at := pos
				at.Offset = mustRational(t, q, 1)
				instrument, err := h.effectiveInstrumentStatements(ctx, s, at, id)
				if err != nil {
					t.Fatal(err)
				}
				midi, err := h.effectiveMIDIStatements(ctx, s, at, id)
				if err != nil {
					t.Fatal(err)
				}
				if q == 3 {
					for label, v := range map[string]Evidence[bool]{"instrument": stateUnavailable[string, bool](instrument.Sound.Value), "midi": stateUnavailable[Integer, bool](midi.Program.Value)} {
						if v.State != EvidenceInvalid || v.Cause == nil || v.Cause.Rule.ID != "xsd.identity" {
							t.Errorf("%s lost unresolved-target cause: %+v", label, v)
						}
					}
				} else {
					sound, program := "first", "74"
					if q == 5 {
						sound, program = "repaired", "41"
					}
					if instrument.Sound.Value.State != EvidenceKnown || instrument.Sound.Value.Value != sound || midi.Program.Value.State != EvidenceKnown || midi.Program.Value.Value.String() != program {
						t.Errorf("q%d known-position bad target erased independent prefix/repair: instrument=%+v midi=%+v", q, instrument.Sound, midi.Program)
					}
				}
				if instrument.Performance.Value.State != EvidenceAbsent || instrument.Virtual.Value.State != EvidenceAbsent || midi.Pan.Value.State != EvidenceAbsent {
					t.Errorf("q%d unrelated omitted fields poisoned: instrument=%+v midi pan=%+v", q, instrument, midi.Pan)
				}
			}
		})
	}
}

// A signed position remains an exact measure-relative fact. With no established
// inter-measure duration, a negative next-measure sound can occur on either side
// of an earlier measure's q=1 query. Ordinal order alone is not chronological
// evidence for continuing-state selection across those frames.
func TestTimelineFinalReviewCrossMeasureOffsetCannotClaimKnownChronology(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	initial := stateNode(b, timelineRoleSound, 30, stateAbsent[Atomic]())
	stateAttribute(b, initial, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 60, 1)})
	shifted := stateNode(b, timelineRoleSound, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleSoundOffset, 41, stateAtomicDecimal(t, -1, 1)))
	stateAttribute(b, shifted, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 120, 1)})
	first, second := runtimeBlock(b, 0, attrs, initial, runtimeNote(t, b, 50, 1, false)), runtimeBlock(b, 1, shifted)
	for _, block := range []timelineMeasureBlock{first, second} {
		if err := svc.readMeasure(context.Background(), s, b, block); err != nil {
			t.Fatal(err)
		}
	}
	if got := svc.Facts[40].Position; got.State != EvidenceKnown || got.Value.Offset.String() != "-1" || got.Value.MeasureOrdinal != 1 {
		t.Fatalf("lost exact local coordinate: %+v", got)
	}
	query := svc.Facts[50].End.Value
	got, err := svc.Playback.effectiveTempo(context.Background(), s, &svc.State, query)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value.State == EvidenceKnown {
		t.Fatalf("ordinal order fabricated cross-frame chronology without measure span: %+v", got)
	}
}

func TestTimelineFinalReviewOffsetContainmentPositiveControls(t *testing.T) {
	for _, tc := range []struct {
		name               string
		offset, lead, tail int64
	}{
		{"negative-contained-by-own-origin", -1, 2, 0},
		{"positive-contained-by-later-origin", 1, 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, svc, b := runtimeFixture(t)
			attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
			sound := stateNode(b, timelineRoleSound, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleSoundOffset, 41, stateAtomicDecimal(t, tc.offset, 1)))
			stateAttribute(b, sound, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 120, 1)})
			events := []*factNode{attrs, runtimeNote(t, b, 30, tc.lead, false), sound}
			if tc.tail > 0 {
				events = append(events, runtimeNote(t, b, 50, tc.tail, false), runtimeNote(t, b, 60, 1, false))
			}
			first, second := runtimeBlock(b, 0, events...), runtimeBlock(b, 1, runtimeNote(t, b, 70, 1, false))
			for _, block := range []timelineMeasureBlock{first, second} {
				if err := svc.readMeasure(context.Background(), s, b, block); err != nil {
					t.Fatal(err)
				}
			}
			if err := svc.finalizeTimingOrigins(context.Background(), s, b, []timelineMeasureBlock{first, second}); err != nil {
				t.Fatal(err)
			}
			got, err := svc.Playback.effectiveTempo(context.Background(), s, &svc.State, svc.Facts[70].Position.Value)
			if err != nil || got.Value.State != EvidenceKnown || got.Value.Value.String() != "120" {
				t.Fatalf("proved-contained offset lost ordinary carry: %+v err=%v", got, err)
			}
		})
	}
}

func TestTimelineFinalReviewPositiveOverflowMayCompeteInNextFrame(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	delayed := stateNode(b, timelineRoleSound, 30, stateAbsent[Atomic](), stateNode(b, timelineRoleSoundOffset, 31, stateAtomicDecimal(t, 10, 1)))
	stateAttribute(b, delayed, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 60, 1)})
	next := stateNode(b, timelineRoleSound, 40, stateAbsent[Atomic]())
	stateAttribute(b, next, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 120, 1)})
	first, second := runtimeBlock(b, 0, attrs, delayed, runtimeNote(t, b, 35, 1, false)), runtimeBlock(b, 1, next, runtimeNote(t, b, 50, 1, false), runtimeNote(t, b, 60, 1, false))
	for _, block := range []timelineMeasureBlock{first, second} {
		if err := svc.readMeasure(context.Background(), s, b, block); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.finalizeTimingOrigins(context.Background(), s, b, []timelineMeasureBlock{first, second}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Playback.effectiveTempo(context.Background(), s, &svc.State, svc.Facts[60].Position.Value)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value.State == EvidenceKnown {
		t.Fatalf("earlier-frame positive overflow was silently superseded: %+v", got)
	}
}

func TestTimelineFinalReviewFloatingSoundSameFrameRepairAndPropertyIsolation(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	initial := stateNode(b, timelineRoleSound, 25, stateAbsent[Atomic]())
	stateAttribute(b, initial, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 60, 1)})
	floating := stateNode(b, timelineRoleSound, 30, stateAbsent[Atomic](), stateNode(b, timelineRoleSoundOffset, 31, stateAtomicDecimal(t, -2, 1)))
	stateAttribute(b, floating, timelineRoleSoundDynamics, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 50, 1)})
	repair := stateNode(b, timelineRoleSound, 40, stateAbsent[Atomic]())
	stateAttribute(b, repair, timelineRoleSoundDynamics, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 90, 1)})
	first, second := runtimeBlock(b, 0, attrs, initial, runtimeNote(t, b, 50, 1, false)), runtimeBlock(b, 1, floating, repair, runtimeNote(t, b, 60, 1, false))
	for _, block := range []timelineMeasureBlock{first, second} {
		if err := svc.readMeasure(context.Background(), s, b, block); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.finalizeTimingOrigins(context.Background(), s, b, []timelineMeasureBlock{first, second}); err != nil {
		t.Fatal(err)
	}
	tempo, err := svc.Playback.effectiveTempo(context.Background(), s, &svc.State, svc.Facts[50].Position.Value)
	if err != nil || tempo.Value.State != EvidenceKnown || tempo.Value.Value.String() != "60" {
		t.Fatalf("unrelated floating dynamics erased tempo: %+v err=%v", tempo, err)
	}
	dynamics, err := svc.Playback.effectiveDynamics(context.Background(), s, &svc.State, svc.Facts[60].Position.Value)
	if err != nil || dynamics.Value.State != EvidenceKnown || dynamics.Value.Value.String() != "90" {
		t.Fatalf("same-frame unconditional repair lost: %+v err=%v", dynamics, err)
	}
}

func TestTimelineFinalReviewGraceDonorsUseKnownScoreOrder(t *testing.T) {
	s, svc, b := runtimeFixture(t)
	voiced := func(id NodeID, duration int64) *factNode {
		n := runtimeNote(t, b, id, duration, false)
		n.children = append(n.children, stateNode(b, timelineRoleNoteVoice, id+3, stateAtomicText("one")))
		return n
	}
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	forward := stateNode(b, timelineRoleForward, 25, stateAbsent[Atomic](), stateNode(b, timelineRoleForwardDuration, 26, stateAtomicDecimal(t, 2, 1)))
	backup := stateNode(b, timelineRoleBackup, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleBackupDuration, 41, stateAtomicDecimal(t, 3, 1)))
	grace := stateNode(b, timelineRoleNoteGrace, 61, stateAbsent[Atomic]())
	stateAttribute(b, grace, timelineRoleGraceStealPrevious, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 25, 1)})
	stateAttribute(b, grace, timelineRoleGraceStealFollowing, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 25, 1)})
	note := stateNode(b, timelineRoleNote, 60, stateAbsent[Atomic](), grace, stateNode(b, timelineRoleNoteVoice, 63, stateAtomicText("one")))
	block := runtimeBlock(b, 0, attrs, forward, voiced(30, 1), backup, voiced(50, 1), note, voiced(70, 1))
	if err := svc.readMeasure(context.Background(), s, b, block); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[NodeID]string{30: "2", 50: "0", 60: "1", 70: "1"} {
		if f := svc.Facts[id]; f.Position.State != EvidenceKnown || f.Position.Value.Offset.String() != want {
			t.Fatalf("fixture origin%d=%+v want%s", id, f.Position, want)
		}
	}
	if err := svc.finalizeTimingOrigins(context.Background(), s, b, []timelineMeasureBlock{block}); err != nil {
		t.Fatal(err)
	}
	got := svc.GraceNeighbors[60]
	if got.Previous.Note.State != EvidenceKnown || got.Previous.Note.Value.Node != 50 || got.Following.Note.State != EvidenceKnown || got.Following.Note.Value.Node != 70 {
		t.Fatalf("lexical disorder hid unique score-order donors: %+v", got)
	}
}

// Initial score-part statements are definite baselines, even when a positioned
// change might or might not yet apply. Equal values can prove an effective value
// by consensus, but the proof must not invent a definite floating source.
func TestTimelineFinalReviewFloatingDeviceIncludesInitialBaseline(t *testing.T) {
	for _, changedName := range []string{"initial-device", "different-device"} {
		t.Run(changedName, func(t *testing.T) {
			ctx := context.Background()
			s, svc, b := runtimeFixture(t)
			initial := instrumentRecord[midiDeviceState]{Part: NodeRef{"d", 1}, Source: NodeRef{"d", 10}, Instrument: stateAbsent[NodeRef](), Position: stateAbsent[musicalPosition](), Value: stateKnown(midiDeviceState{Name: stateKnown("initial-device"), Port: stateKnown(stateInt(t, 3))})}
			if err := svc.State.addMIDIDeviceRecord(ctx, s, initial, 0); err != nil {
				t.Fatal(err)
			}
			attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
			device := stateNode(b, timelineRoleSoundMidiDevice, 41, stateAtomicText(changedName))
			sound := stateNode(b, timelineRoleSound, 40, stateAbsent[Atomic](), device, stateNode(b, timelineRoleSoundOffset, 42, stateAtomicDecimal(t, 10, 1)))
			first, second := runtimeBlock(b, 0, attrs, sound, runtimeNote(t, b, 50, 1, false)), runtimeBlock(b, 1, runtimeNote(t, b, 60, 1, false), runtimeNote(t, b, 70, 1, false))
			for _, block := range []timelineMeasureBlock{first, second} {
				if err := svc.readMeasure(ctx, s, b, block); err != nil {
					t.Fatal(err)
				}
			}
			if err := svc.finalizeTimingOrigins(ctx, s, b, []timelineMeasureBlock{first, second}); err != nil {
				t.Fatal(err)
			}
			got, err := svc.State.effectiveMIDIDeviceStatements(ctx, s, svc.Facts[70].Position.Value, NodeRef{"d", 71})
			if err != nil {
				t.Fatal(err)
			}
			if got.Port.Value.State != EvidenceKnown || got.Port.Value.Value.String() != "3" {
				t.Fatalf("omitted floating property erased initial port: %+v", got.Port)
			}
			if changedName != "initial-device" {
				if got.Name.Value.State == EvidenceKnown {
					t.Fatalf("floating change fabricated definite supersession of initial value: %+v", got.Name)
				}
				return
			}
			if got.Name.Value.State != EvidenceKnown || got.Name.Value.Value != "initial-device" {
				t.Fatalf("same-value initial baseline consensus lost: %+v", got.Name)
			}
			initialSource := false
			for _, source := range got.Name.Provenance {
				initialSource = initialSource || source.Node == initial.Source
			}
			if !initialSource {
				t.Fatalf("consensus hid the required initial-source proof: %+v", got.Name)
			}
		})
	}
}

func TestTimelineFinalReviewConditionalPlaybackCannotRepairPriorGap(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		conditional, priorRepair bool
	}{
		{"conditional-retains-causal-gap", true, false},
		{"unconditional-repairs-gap", false, false},
		{"prior-unconditional-repair-is-not-resurrected", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s, svc, b := runtimeFixture(t)
			attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
			events := []*factNode{attrs, runtimeNote(t, b, 30, 3, false)}
			if tc.priorRepair {
				repair := stateNode(b, timelineRoleSound, 35, stateAbsent[Atomic]())
				stateAttribute(b, repair, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 80, 1)})
				events = append(events, repair)
			}
			sound := stateNode(b, timelineRoleSound, 40, stateAbsent[Atomic]())
			stateAttribute(b, sound, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 120, 1)})
			if tc.conditional {
				stateAttribute(b, sound, timelineRoleSoundTimeOnly, Atomic{Kind: AtomicString, Text: "2"})
			}
			events = append(events, runtimeNote(t, b, 36, 1, false), sound, runtimeNote(t, b, 50, 1, false), runtimeNote(t, b, 60, 1, false))
			block := runtimeBlock(b, 0, events...)
			if err := svc.readMeasure(ctx, s, b, block); err != nil {
				t.Fatal(err)
			}
			if err := svc.finalizeTimingOrigins(ctx, s, b, []timelineMeasureBlock{block}); err != nil {
				t.Fatal(err)
			}
			gapAt := svc.Facts[40].Position.Value
			gapAt.Offset = mustRational(t, 2, 1)
			cause := &DecisionRef{Rule: RuleKey{"review-invalid-tempo", 1}, Subject: NodeRef{"d", 90}}
			if err := addStateGap(ctx, s, &svc.Playback.Tempo, stateGap{Position: &gapAt, Part: gapAt.Part, Scope: stateScope{AllStaves: true}, Availability: Evidence[bool]{State: EvidenceInvalid, Reason: "invalid-tempo-publisher", Cause: cause}}); err != nil {
				t.Fatal(err)
			}
			got, err := svc.Playback.effectiveTempo(ctx, s, &svc.State, svc.Facts[60].Position.Value)
			if err != nil {
				t.Fatal(err)
			}
			if tc.conditional && !tc.priorRepair {
				if got.Value.State != EvidenceInvalid || got.Value.Cause == nil || *got.Value.Cause != *cause {
					t.Fatalf("conditional playback publication erased unrepaired actual cause: %+v", got)
				}
			} else if tc.conditional {
				if got.Value.State != EvidenceUnknown || got.Value.Cause != nil {
					t.Fatalf("conditional update resurrected previously repaired invalid gap: %+v", got)
				}
			} else if got.Value.State != EvidenceKnown || got.Value.Value.String() != "120" {
				t.Fatalf("unconditional control failed to repair: %+v", got)
			}
		})
	}
}

func TestTimelineFinalReviewFloatingDynamicsIncludesDocumentedDefault(t *testing.T) {
	for _, value := range []int64{100, 50} {
		t.Run(map[int64]string{100: "same-default", 50: "different-value"}[value], func(t *testing.T) {
			ctx := context.Background()
			s, svc, b := runtimeFixture(t)
			attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
			sound := stateNode(b, timelineRoleSound, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleSoundOffset, 41, stateAtomicDecimal(t, 10, 1)))
			stateAttribute(b, sound, timelineRoleSoundDynamics, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, value, 1)})
			blocks := []timelineMeasureBlock{runtimeBlock(b, 0, attrs, sound, runtimeNote(t, b, 50, 1, false)), runtimeBlock(b, 1, runtimeNote(t, b, 60, 1, false), runtimeNote(t, b, 70, 1, false))}
			for _, block := range blocks {
				if err := svc.readMeasure(ctx, s, b, block); err != nil {
					t.Fatal(err)
				}
			}
			if err := svc.finalizeTimingOrigins(ctx, s, b, blocks); err != nil {
				t.Fatal(err)
			}
			got, err := svc.Playback.effectiveDynamics(ctx, s, &svc.State, svc.Facts[70].Position.Value)
			if err != nil {
				t.Fatal(err)
			}
			if value != 100 {
				if got.Value.State == EvidenceKnown {
					t.Fatalf("floating dynamics fabricated definite replacement of default: %+v", got)
				}
				return
			}
			if got.Value.State != EvidenceKnown || got.Value.Value.String() != "100" {
				t.Fatalf("same-value documented-default consensus lost: %+v", got)
			}
			defaultSource := false
			for _, source := range got.Provenance {
				defaultSource = defaultSource || source.Kind == stateInitialDefault && source.Node == (NodeRef{"d", 1})
			}
			if !defaultSource {
				t.Fatalf("consensus hid necessary documented-default proof: %+v", got)
			}
		})
	}
}

func TestTimelineFinalReviewFloatingInvalidPublicationPreservesActualCause(t *testing.T) {
	ctx := context.Background()
	s, svc, b := runtimeFixture(t)
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	initial := stateNode(b, timelineRoleSound, 30, stateAbsent[Atomic]())
	stateAttribute(b, initial, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 60, 1)})
	floating := stateNode(b, timelineRoleSound, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleSoundOffset, 41, stateAtomicDecimal(t, -1, 1)))
	stateAttribute(b, floating, timelineRoleSoundTempo, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 120, 1)})
	cause := &DecisionRef{Rule: RuleKey{"review-invalid-tempo-scalar", 1}, Subject: NodeRef{"d", 40}}
	floating.attributes[0].Scalar.Value = Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-tempo-scalar", Cause: cause}
	blocks := []timelineMeasureBlock{runtimeBlock(b, 0, attrs, initial, runtimeNote(t, b, 50, 1, false)), runtimeBlock(b, 1, floating)}
	for _, block := range blocks {
		if err := svc.readMeasure(ctx, s, b, block); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.finalizeTimingOrigins(ctx, s, b, blocks); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Playback.effectiveTempo(ctx, s, &svc.State, svc.Facts[50].End.Value)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value.State != EvidenceInvalid || got.Value.Cause == nil || *got.Value.Cause != *cause {
		t.Fatalf("floating actual invalid evidence was ignored or replaced by synthetic order uncertainty: %+v", got)
	}
	got.Value.Cause.Subject.Node = 999
	again, err := svc.Playback.effectiveTempo(ctx, s, &svc.State, svc.Facts[50].End.Value)
	if err != nil || again.Value.Cause == nil || again.Value.Cause.Subject.Node != 40 {
		t.Fatalf("floating result aliased causal ownership: %+v err=%v", again, err)
	}
}

// Structural measure order is sufficient for contained note candidates. A
// later-labeled note reached by an unproved negative cursor excursion cannot
// borrow that proof for direction/figure/grace following-note association.
func TestTimelineFinalReviewUnprovedCursorCannotCertifyCrossFrameFollowingNote(t *testing.T) {
	for _, family := range []string{"direction", "figure", "grace"} {
		for _, excursion := range []bool{false, true} {
			t.Run(family+"/"+map[bool]string{false: "contained-control", true: "negative-cursor"}[excursion], func(t *testing.T) {
				ctx := context.Background()
				s, svc, b := runtimeFixture(t)
				attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
				var origin *factNode
				switch family {
				case "direction":
					origin = stateNode(b, timelineRoleDirection, 40, stateAbsent[Atomic]())
				case "figure":
					origin = stateNode(b, timelineRoleFiguredBass, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleFiguredBassFigure, 41, stateAbsent[Atomic]()), stateNode(b, timelineRoleFiguredBassDuration, 42, stateAtomicDecimal(t, 1, 1)))
				case "grace":
					grace := stateNode(b, timelineRoleNoteGrace, 41, stateAbsent[Atomic]())
					stateAttribute(b, grace, timelineRoleGraceStealFollowing, Atomic{Kind: AtomicDecimal, Decimal: stateDecimal(t, 25, 1)})
					origin = stateNode(b, timelineRoleNote, 40, stateAbsent[Atomic](), grace)
				}
				first := runtimeBlock(b, 0, attrs, runtimeNote(t, b, 30, 1, false), origin)
				var secondEvents []*factNode
				if excursion {
					secondEvents = append(secondEvents, stateNode(b, timelineRoleBackup, 60, stateAbsent[Atomic](), stateNode(b, timelineRoleBackupDuration, 61, stateAtomicDecimal(t, 2, 1))))
				}
				secondEvents = append(secondEvents, runtimeNote(t, b, 70, 1, false))
				blocks := []timelineMeasureBlock{first, runtimeBlock(b, 1, secondEvents...)}
				for _, block := range blocks {
					if err := svc.readMeasure(ctx, s, b, block); err != nil {
						t.Fatal(err)
					}
				}
				if err := svc.finalizeTimingOrigins(ctx, s, b, blocks); err != nil {
					t.Fatal(err)
				}
				wantCoordinate := "0"
				if excursion {
					wantCoordinate = "-2"
				}
				if pos := svc.Facts[70].Position; pos.State != EvidenceKnown || pos.Value.Offset.String() != wantCoordinate {
					t.Fatalf("uniform-unit local coordinate lost: %+v", pos)
				}
				var got Evidence[NodeRef]
				switch family {
				case "direction":
					got = svc.DirectionOrigins[40].Anchor
				case "figure":
					got = svc.FiguredBassOrigins[40].Anchor
				case "grace":
					got = svc.GraceNeighbors[40].Following.Note
				}
				if excursion {
					if got.State == EvidenceKnown {
						t.Fatalf("measure label certified an unproved cross-frame candidate: %+v", got)
					}
				} else if got.State != EvidenceKnown || got.Value.Node != 70 {
					t.Fatalf("contained structural-order control lost: %+v", got)
				}
			})
		}
	}
}

func TestTimelineFinalReviewBoundaryExcursionCannotChooseChangedDivisionUnits(t *testing.T) {
	ctx := context.Background()
	s, svc, b := runtimeFixture(t)
	firstUnits := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	nextUnits := stateNode(b, timelineRoleAttributes, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 41, stateAtomicDecimal(t, 2, 1)))
	backup := stateNode(b, timelineRoleBackup, 50, stateAbsent[Atomic](), stateNode(b, timelineRoleBackupDuration, 51, stateAtomicDecimal(t, 2, 1)))
	blocks := []timelineMeasureBlock{runtimeBlock(b, 0, firstUnits, runtimeNote(t, b, 30, 1, false)), runtimeBlock(b, 1, nextUnits, backup, runtimeNote(t, b, 60, 1, false))}
	for _, block := range blocks {
		if err := svc.readMeasure(ctx, s, b, block); err != nil {
			t.Fatal(err)
		}
	}
	if earlier := svc.Facts[30]; earlier.Position.State != EvidenceKnown || earlier.End.State != EvidenceKnown || earlier.End.Value.Offset.String() != "1" {
		t.Fatalf("independent earlier prefix lost: %+v", earlier)
	}
	if origin := svc.Facts[40].Position; origin.State != EvidenceKnown || origin.Value.Offset.String() != "0" {
		t.Fatalf("new frame origin lost: %+v", origin)
	}
	if got := svc.Facts[60].Position; got.State == EvidenceKnown {
		t.Fatalf("cross-boundary rewind chose changed departure units: %+v", got)
	}
}
