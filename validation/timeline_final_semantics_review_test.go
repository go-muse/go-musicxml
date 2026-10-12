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
