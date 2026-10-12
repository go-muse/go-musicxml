package validation

import "context"

// Raw sound records distinguish declaration defaults, note-local instructions,
// and positioned continuing changes. They retain assessed values without a
// playback engine, MIDI routing or package lookup. Effective encoded-statement
// selection is separate and preserves these unmerged raw records.
type midiState struct {
	Channel, Bank, Program, Unpitched Evidence[Integer]
	Name                              Evidence[string]
	Volume, Pan, Elevation            Evidence[Decimal]
}
type midiDeviceState struct {
	Name Evidence[string]
	Port Evidence[Integer]
}
type playTechniqueKind uint8

const (
	playIPA playTechniqueKind = iota + 1
	playMute
	playSemiPitched
	playOther
)

type playTechnique struct {
	Kind        playTechniqueKind
	Value, Type Evidence[string]
}
type playState struct{ Techniques []playTechnique }
type instrumentRecord[T any] struct {
	TimeOnly           *Evidence[string]
	Part, Source, Note NodeRef
	Instrument         Evidence[NodeRef]
	Position           Evidence[musicalPosition]
	Value              Evidence[T]
}

func (h *timelineState) addInstrumentDefinition(ctx context.Context, s *Session, d instrumentDefinition) error {
	if d.Part.Node == 0 || d.Instrument.Node == 0 || d.Part.DocumentID != d.Instrument.DocumentID {
		return contract("instrument definition", "canonical part and instrument required")
	}
	if _, e := stateReserve(ctx, s, d); e != nil {
		return e
	}
	h.Definitions = append(h.Definitions, stateClone(d))
	return nil
}

// Fixed append storage covers concurrent old/new backing arrays. References are
// copied at exact length; immutable identity strings may be shared, while every
// mutable causal pointer is independently owned. The temporary fixed record is
// admitted separately and released on every exit.
const timelineNoteInstrumentReserveBytes int64 = 512

func (h *timelineState) addNoteInstruments(ctx context.Context, s *Session, n noteInstrumentSet) error {
	if n.Note.Node == 0 || n.Part.Node == 0 || n.Note.DocumentID != n.Part.DocumentID {
		return contract("note instruments", "exact note and canonical part required")
	}
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if err := s.account.Charge(BudgetContextFacts, 1); err != nil {
		return err
	}
	if err := s.account.Charge(BudgetBufferedBytes, timelineNoteInstrumentReserveBytes); err != nil {
		return err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, timelineNoteInstrumentReserveBytes) }()
	if err := s.retain(timelineNoteInstrumentReserveBytes); err != nil {
		return err
	}
	var err error
	if n.Complete, err = stateOwnEvidence(ctx, s, n.Complete); err != nil {
		return err
	}
	if err := stateRetainCount(s, len(n.References), 128); err != nil {
		return err
	}
	if _, err := AllocationSize(int64(len(n.References))); err != nil {
		return err
	}
	var refs []Evidence[NodeRef]
	if n.References != nil {
		refs = make([]Evidence[NodeRef], len(n.References))
	}
	for i, ref := range n.References {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		if refs[i], err = stateOwnEvidence(ctx, s, ref); err != nil {
			return err
		}
	}
	n.References = refs
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	h.NoteInstruments = append(h.NoteInstruments, n)
	return nil
}

func (h *timelineState) addInstrumentChange(ctx context.Context, s *Session, id NodeRef, p statePublication[instrumentState]) error {
	if id.Node == 0 || id.DocumentID != p.Position.Part.DocumentID {
		return contract("instrument change", "canonical instrument required")
	}
	for i := range h.InstrumentChanges {
		if e := s.step(ctx, 1); e != nil {
			return e
		}
		if h.InstrumentChanges[i].Instrument == id {
			if e := publishState(ctx, s, &h.InstrumentChanges[i].History, p); e != nil {
				return e
			}
			h.InstrumentChanges[i].discardStatementIndices(s)
			return nil
		}
	}
	if e := s.retain(256); e != nil {
		return e
	}
	candidate := instrumentChanges{Instrument: id}
	if e := publishState(ctx, s, &candidate.History, p); e != nil {
		return e
	}
	h.InstrumentChanges = append(h.InstrumentChanges, candidate)
	return nil
}
func (r *stateReader) instrument(n *factNode, change bool) Evidence[instrumentState] {
	complete := r.complete(n)
	if complete.State != EvidenceKnown {
		return stateUnavailable[bool, instrumentState](complete)
	}
	sound, solo, ensemble, virtual := timelineRoleScoreInstrumentInstrumentSound, timelineRoleScoreInstrumentSolo, timelineRoleScoreInstrumentEnsemble, timelineRoleScoreInstrumentVirtualInstrument
	if change {
		sound, solo, ensemble, virtual = timelineRoleInstrumentChangeInstrumentSound, timelineRoleInstrumentChangeSolo, timelineRoleInstrumentChangeEnsemble, timelineRoleInstrumentChangeVirtualInstrument
	}
	v := instrumentState{Sound: stateText(r.childScalar(n, sound))}
	_, v.Solo = r.child(n, solo)
	raw := r.childScalar(n, ensemble)
	if raw.State != EvidenceKnown {
		v.Ensemble = stateUnavailable[Atomic, ensembleState](raw)
	} else if raw.Value.Kind == AtomicInteger {
		v.Ensemble = Evidence[ensembleState]{State: EvidenceKnown, Value: ensembleState{Number: raw.Value.Integer}}
	} else if raw.Value.Kind == AtomicString && raw.Value.Text == "" {
		v.Ensemble = Evidence[ensembleState]{State: EvidenceKnown, Value: ensembleState{Empty: true}}
	} else {
		v.Ensemble = Evidence[ensembleState]{State: EvidenceUnsupported, Reason: "timeline-ensemble-family-unavailable"}
	}
	ch, e := r.child(n, virtual)
	if e.State != EvidenceKnown {
		v.Virtual = stateUnavailable[bool, virtualInstrumentState](e)
	} else {
		c := r.complete(ch)
		if c.State != EvidenceKnown {
			v.Virtual = stateUnavailable[bool, virtualInstrumentState](c)
		} else {
			v.Virtual = Evidence[virtualInstrumentState]{State: EvidenceKnown, Value: virtualInstrumentState{Library: stateText(r.childScalar(ch, timelineRoleVirtualInstrumentVirtualLibrary)), Name: stateText(r.childScalar(ch, timelineRoleVirtualInstrumentVirtualName))}}
		}
	}
	return Evidence[instrumentState]{State: EvidenceKnown, Value: v}
}
func (r *stateReader) reference(n *factNode, idRole timelineRole, part NodeRef) Evidence[NodeRef] {
	id := r.attribute(n, idRole)
	if id.State == EvidenceAbsent && idRole != timelineRoleMidiDeviceID && idRole != timelineRolePlayID {
		return stateUnknown[NodeRef]("timeline-required-instrument-id-absent")
	}
	if id.State != EvidenceKnown {
		return stateUnavailable[Atomic, NodeRef](id)
	}
	a, ok := r.b.attribute(idRole)
	if !ok {
		return Evidence[NodeRef]{State: EvidenceUnsupported, Reason: "timeline-instrument-reference-binding-unavailable"}
	}
	target, d, e := r.s.referenceResolution(r.ctx, n, a)
	if e != nil {
		r.err = e
		return stateUnknown[NodeRef]("timeline-interrupted")
	}
	if d.Outcome != OutcomePass {
		state := EvidenceUnknown
		if d.Outcome == OutcomeBlocked {
			state = EvidenceInvalid
		} else if d.Outcome == OutcomeUnsupported {
			state = EvidenceUnsupported
		}
		return Evidence[NodeRef]{State: state, Reason: d.Reason, Cause: d.Cause}
	}
	if target == nil || !r.matches(target, timelineRoleScoreInstrument) {
		return stateUnknown[NodeRef]("timeline-instrument-target-kind-unavailable")
	}
	if target.header.Parent != part.Node || r.s.documentID != part.DocumentID {
		return stateUnknown[NodeRef]("timeline-instrument-owner-unavailable")
	}
	return Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{DocumentID: r.s.documentID, Node: target.header.ID}}
}
func (r *stateReader) midi(n *factNode) Evidence[midiState] {
	c := r.complete(n)
	if c.State != EvidenceKnown {
		return stateUnavailable[bool, midiState](c)
	}
	return Evidence[midiState]{State: EvidenceKnown, Value: midiState{Channel: stateInteger(r.childScalar(n, timelineRoleMidiInstrumentMidiChannel)), Name: stateText(r.childScalar(n, timelineRoleMidiInstrumentMidiName)), Bank: stateInteger(r.childScalar(n, timelineRoleMidiInstrumentMidiBank)), Program: stateInteger(r.childScalar(n, timelineRoleMidiInstrumentMidiProgram)), Unpitched: stateInteger(r.childScalar(n, timelineRoleMidiInstrumentMidiUnpitched)), Volume: stateDecimalValue(r.childScalar(n, timelineRoleMidiInstrumentVolume)), Pan: stateDecimalValue(r.childScalar(n, timelineRoleMidiInstrumentPan)), Elevation: stateDecimalValue(r.childScalar(n, timelineRoleMidiInstrumentElevation))}}
}
func (r *stateReader) play(n *factNode) Evidence[playState] {
	c := r.complete(n)
	if c.State != EvidenceKnown {
		return stateUnavailable[bool, playState](c)
	}
	if !r.step(int64(len(n.children))) {
		return stateUnknown[playState]("timeline-interrupted")
	}
	if e := stateRetainCount(r.s, len(n.children), 512); e != nil {
		r.err = e
		return stateUnknown[playState]("timeline-interrupted")
	}
	v := playState{}
	for _, ch := range n.children {
		var kind playTechniqueKind
		switch {
		case r.matches(ch, timelineRolePlayIpa):
			kind = playIPA
		case r.matches(ch, timelineRolePlayMute):
			kind = playMute
		case r.matches(ch, timelineRolePlaySemiPitched):
			kind = playSemiPitched
		case r.matches(ch, timelineRolePlayOtherPlay):
			kind = playOther
		default:
			if ch.header.Declaration.State != EvidenceKnown {
				return stateUnavailable[DeclarationID, playState](ch.header.Declaration)
			}
			continue
		}
		technique := playTechnique{Kind: kind, Value: stateText(r.scalar(ch)), Type: Evidence[string]{State: EvidenceAbsent}}
		if kind == playOther {
			technique.Type = stateText(r.attribute(ch, timelineRoleOtherPlayType))
		}
		v.Techniques = append(v.Techniques, technique)
	}
	return Evidence[playState]{State: EvidenceKnown, Value: v}
}
func (h *timelineState) rawInstrumentRecords(r *stateReader, n *factNode, part, note NodeRef, pos Evidence[musicalPosition], midi, device, play timelineRole, ordinal uint64) {
	condition := r.soundCondition(n)
	if r.err != nil {
		return
	}
	for _, ch := range n.children {
		source := NodeRef{DocumentID: r.s.documentID, Node: ch.header.ID}
		switch {
		case midi != timelineRoleInvalid && r.matches(ch, midi):
			record := instrumentRecord[midiState]{Part: part, Source: source, Note: note, Position: pos, TimeOnly: condition, Instrument: r.reference(ch, timelineRoleMidiInstrumentID, part), Value: r.midi(ch)}
			if r.err != nil {
				return
			}
			if r.err = h.addMIDIRecord(r.ctx, r.s, record, ordinal); r.err != nil {
				return
			}
		case device != timelineRoleInvalid && r.matches(ch, device):
			record := instrumentRecord[midiDeviceState]{Part: part, Source: source, Note: note, Position: pos, TimeOnly: condition, Instrument: r.reference(ch, timelineRoleMidiDeviceID, part), Value: Evidence[midiDeviceState]{State: EvidenceKnown, Value: midiDeviceState{Name: stateText(r.scalar(ch)), Port: stateInteger(r.attribute(ch, timelineRoleMidiDevicePort))}}}
			if r.err != nil {
				return
			}
			if r.err = h.addMIDIDeviceRecord(r.ctx, r.s, record, ordinal); r.err != nil {
				return
			}
		case play != timelineRoleInvalid && r.matches(ch, play):
			record := instrumentRecord[playState]{Part: part, Source: source, Note: note, Position: pos, TimeOnly: condition, Instrument: r.reference(ch, timelineRolePlayID, part), Value: r.play(ch)}
			if r.err != nil {
				return
			}
			if _, r.err = stateReserve(r.ctx, r.s, record); r.err != nil {
				return
			}
			h.Play = append(h.Play, stateClone(record))
		}
	}
}
func (h *timelineState) readInstrumentDefinitions(ctx context.Context, s *Session, b *timelineBindings, n *factNode, part NodeRef) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.matches(n, timelineRoleScorePart) || n.header.ID != part.Node {
		if r.err != nil {
			return r.err
		}
		return contract("instrument definitions", "exact canonical score-part required")
	}
	children, complete := r.children(n, timelineRoleScoreInstrument)
	if complete.State != EvidenceKnown {
		g := stateGap{Part: part, Scope: stateScope{AllStaves: true}, Availability: complete}
		if _, e := stateReserve(ctx, s, g); e != nil {
			return e
		}
		h.InstrumentGaps = append(h.InstrumentGaps, g)
		return r.err
	}
	for _, ch := range children {
		d := instrumentDefinition{Name: stateText(r.childScalar(ch, timelineRoleScoreInstrumentInstrumentName)), Abbreviation: stateText(r.childScalar(ch, timelineRoleScoreInstrumentInstrumentAbbreviation)), Part: part, Instrument: NodeRef{DocumentID: s.documentID, Node: ch.header.ID}, Value: r.instrument(ch, false), Provenance: []stateProvenance{{Node: NodeRef{DocumentID: s.documentID, Node: ch.header.ID}, Kind: stateExplicit}}}
		if r.err != nil {
			return r.err
		}
		if e := h.addInstrumentDefinition(ctx, s, d); e != nil {
			return e
		}
	}
	h.rawInstrumentRecords(r, n, part, NodeRef{}, Evidence[musicalPosition]{State: EvidenceAbsent}, timelineRoleScorePartMidiInstrument, timelineRoleScorePartMidiDevice, timelineRoleInvalid, 0)
	return r.err
}
func (h *timelineState) readNoteInstruments(ctx context.Context, s *Session, b *timelineBindings, n *factNode, part NodeRef) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.matches(n, timelineRoleNote) {
		if r.err != nil {
			return r.err
		}
		return contract("note instruments", "exact note declaration required")
	}
	children, complete := r.children(n, timelineRoleNoteInstrument)
	record := noteInstrumentSet{Note: NodeRef{DocumentID: s.documentID, Node: n.header.ID}, Part: part, Complete: complete}
	if e := stateRetainCount(s, len(children), 256); e != nil {
		return e
	}
	for _, ch := range children {
		record.References = append(record.References, r.reference(ch, timelineRoleInstrumentID, part))
		if r.err != nil {
			return r.err
		}
	}
	if e := h.addNoteInstruments(ctx, s, record); e != nil {
		return e
	}
	h.rawInstrumentRecords(r, n, part, record.Note, Evidence[musicalPosition]{State: EvidenceAbsent}, timelineRoleInvalid, timelineRoleInvalid, timelineRoleNotePlay, 0)
	return r.err
}
func (h *timelineState) readInstrumentChanges(ctx context.Context, s *Session, b *timelineBindings, n *factNode, pos musicalPosition) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.matches(n, timelineRoleSound) && !r.matches(n, timelineRoleDirectionSound) {
		if r.err != nil {
			return r.err
		}
		return contract("instrument changes", "exact sound declaration required")
	}
	condition := r.soundCondition(n)
	children, complete := r.children(n, timelineRoleSoundInstrumentChange)
	gap := func(availability Evidence[bool]) error {
		availability = stateConditionalValue(availability, condition)
		g := stateGap{Position: &pos, Part: pos.Part, FirstMeasure: pos.MeasureOrdinal, LastMeasure: pos.MeasureOrdinal, Scope: stateScope{AllStaves: true}, Availability: availability}
		if _, e := stateReserve(ctx, s, g); e != nil {
			return e
		}
		h.InstrumentGaps = append(h.InstrumentGaps, stateClone(g))
		return nil
	}
	if complete.State != EvidenceKnown {
		if r.err != nil {
			return r.err
		}
		if err := h.gapMIDIStatements(ctx, s, stateGap{Position: &pos, Part: pos.Part, FirstMeasure: pos.MeasureOrdinal, LastMeasure: pos.MeasureOrdinal, Scope: stateScope{AllStaves: true}, Availability: stateConditionalValue(complete, condition)}); err != nil {
			return err
		}
		return gap(complete)
	}
	for _, ch := range children {
		id := r.reference(ch, timelineRoleInstrumentChangeID, pos.Part)
		value := r.instrument(ch, true)
		if r.err != nil {
			return r.err
		}
		if id.State != EvidenceKnown {
			record := instrumentRecord[instrumentState]{Part: pos.Part, Source: NodeRef{DocumentID: s.documentID, Node: ch.header.ID}, Instrument: id, Position: Evidence[musicalPosition]{State: EvidenceKnown, Value: pos}, Value: value, TimeOnly: condition}
			if e := h.addUnresolvedInstrumentChange(ctx, s, record, pos.MeasureOrdinal); e != nil {
				return e
			}
			continue
		}
		if e := h.addInstrumentChange(ctx, s, id.Value, statePublication[instrumentState]{Position: pos, TimeOnly: condition, Scope: stateScope{AllStaves: true}, Value: value, Provenance: []stateProvenance{{Node: NodeRef{DocumentID: s.documentID, Node: ch.header.ID}, Kind: stateExplicit}}}); e != nil {
			return e
		}
	}
	h.rawInstrumentRecords(r, n, pos.Part, NodeRef{}, Evidence[musicalPosition]{State: EvidenceKnown, Value: pos}, timelineRoleSoundMidiInstrument, timelineRoleSoundMidiDevice, timelineRoleSoundPlay, pos.MeasureOrdinal)
	return r.err
}

func (h *timelineState) gapInstrumentChanges(ctx context.Context, s *Session, b *timelineBindings, n *factNode, part NodeRef, ordinal uint64, availability Evidence[musicalPosition]) error {
	if availability.State == EvidenceKnown || availability.State == EvidenceAbsent {
		return contract("instrument gap", "unavailable position required")
	}
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.matches(n, timelineRoleSound) && !r.matches(n, timelineRoleDirectionSound) {
		if r.err != nil {
			return r.err
		}
		return contract("instrument gap", "exact sound declaration required")
	}
	condition := r.soundCondition(n)
	g := stateGap{Part: part, FirstMeasure: ordinal, LastMeasure: ordinal, Scope: stateScope{AllStaves: true}, Availability: stateUnavailable[musicalPosition, bool](availability)}
	g.Availability = stateConditionalValue(g.Availability, condition)
	global := func(g stateGap) error {
		if _, e := stateReserve(ctx, s, g); e != nil {
			return e
		}
		h.InstrumentGaps = append(h.InstrumentGaps, stateClone(g))
		return nil
	}
	children, complete := r.children(n, timelineRoleSoundInstrumentChange)
	if r.err != nil {
		return r.err
	}
	if complete.State != EvidenceKnown {
		g.Availability = stateMergeGap(g.Availability, complete)
		if err := h.gapMIDIStatements(ctx, s, g); err != nil {
			return err
		}
		return global(g)
	}
	for _, ch := range children {
		id := r.reference(ch, timelineRoleInstrumentChangeID, part)
		value := r.instrument(ch, true)
		if r.err != nil {
			return r.err
		}
		record := instrumentRecord[instrumentState]{Part: part, Source: NodeRef{DocumentID: s.documentID, Node: ch.header.ID}, Instrument: id, Position: availability, Value: value, TimeOnly: condition}
		if id.State != EvidenceKnown {
			if e := h.addUnresolvedInstrumentChange(ctx, s, record, ordinal); e != nil {
				return e
			}
			continue
		}
		if _, e := stateReserve(ctx, s, record); e != nil {
			return e
		}
		h.UnresolvedInstrumentChanges = append(h.UnresolvedInstrumentChanges, stateClone(record))
		at := -1
		for i := range h.InstrumentChanges {
			if e := s.step(ctx, 1); e != nil {
				return e
			}
			if h.InstrumentChanges[i].Instrument == id.Value {
				at = i
				break
			}
		}
		if at < 0 {
			if e := s.retain(256); e != nil {
				return e
			}
			item := instrumentChanges{Instrument: id.Value}
			if e := item.addStatementGap(ctx, s, g, value); e != nil {
				return e
			}
			h.InstrumentChanges = append(h.InstrumentChanges, item)
		} else {
			if e := h.InstrumentChanges[at].addStatementGap(ctx, s, g, value); e != nil {
				return e
			}
		}
	}
	h.rawInstrumentRecords(r, n, part, NodeRef{}, availability, timelineRoleSoundMidiInstrument, timelineRoleSoundMidiDevice, timelineRoleSoundPlay, ordinal)
	return r.err
}
