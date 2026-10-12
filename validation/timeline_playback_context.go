package validation

import "context"

// Playback providers retain score-time facts, not a rendered performance or a
// repeat traversal. Pinned MusicXML 4.0: play (2749), barline (3227), sound (4125),
// note (5160); the MIDI-compatible tutorial supplies the initial forte default.
// The later repeat consumer owns pass selection and navigation re-entry.
type playbackScope uint8

const (
	playbackCurrentNote playbackScope = iota + 1
	playbackFollowingNotes
)

type playbackOrigin struct {
	Source, Measure NodeRef
	Part            Evidence[NodeRef]
	MeasureOrdinal  uint64
	Position        Evidence[musicalPosition]
	Staff           Evidence[Integer]
	Voice           Evidence[string]
	Order           Evidence[OrderKind]
	MeasureComplete Evidence[bool]
}

type playbackInstruction struct {
	Source     NodeRef
	Instrument Evidence[NodeRef]
	Scope      playbackScope
	Value      Evidence[playState]
}

// Navigation divisions are a separate attribute use from attributes/divisions.
// They are never an ordinary cursor update. Barline marker attributes are active
// for playback only with their corresponding segno/coda child. A known false
// applicability does not erase a raw divisions value (even zero or negative).
type navigationContext struct {
	Divisions                        Evidence[Decimal]
	DivisionsApplicable              Evidence[bool]
	Segno, Coda                      Evidence[string]
	SegnoElement, CodaElement        Evidence[bool]
	Dacapo, ForwardRepeat            Evidence[bool]
	Dalsegno, Tocoda, Fine, TimeOnly Evidence[string]
}

type barlinePlaybackContext struct {
	Origin                playbackOrigin
	RawLocation, Location Evidence[string]
	LocationDefaulted     bool
	Navigation            navigationContext
}

type soundPlaybackContext struct {
	Origin          playbackOrigin
	Tempo, Dynamics Evidence[Decimal]
	Pizzicato       Evidence[bool]
	TimeOnly        Evidence[string]
	Navigation      navigationContext
	Play            []playbackInstruction
	PlayComplete    Evidence[bool]
}

type notePlaybackContext struct {
	Origin                playbackOrigin
	Principal             Evidence[NodeRef]
	Chord                 Evidence[bool]
	Dynamics, EndDynamics Evidence[Decimal]
	Pizzicato             Evidence[bool]
	TimeOnly              Evidence[string]
	Play                  []playbackInstruction
	PlayComplete          Evidence[bool]
}

type timelinePlaybackContext struct {
	Barlines        map[NodeID]barlinePlaybackContext
	Sounds          map[NodeID]soundPlaybackContext
	Notes           map[NodeID]notePlaybackContext
	Tempo, Dynamics stateHistory[Decimal]
	Pizzicato       stateHistory[bool]
}

func playbackOriginAt(s *Session, n *factNode, block timelineMeasureBlock, fact timelineEventFact) playbackOrigin {
	return playbackOrigin{Source: NodeRef{s.documentID, n.header.ID}, Measure: NodeRef{s.documentID, block.measure.header.ID}, Part: fact.Part, MeasureOrdinal: block.ordinal, Position: fact.Position, Staff: fact.Staff, Voice: fact.Voice, Order: n.end.Order, MeasureComplete: block.complete}
}

func playbackStore[T any](ctx context.Context, s *Session, records *map[NodeID]T, id NodeID, v T) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if _, exists := (*records)[id]; exists {
		return contract("playback context", "duplicate publication")
	}
	// Charge the deep owned record and map capacity before either allocation.
	if _, err := stateReserve(ctx, s, v); err != nil {
		return err
	}
	if err := s.retain(256); err != nil {
		return err
	}
	if *records == nil {
		if err := s.retain(256); err != nil {
			return err
		}
		*records = make(map[NodeID]T)
	}
	(*records)[id] = stateClone(v)
	return nil
}

func playbackMarker(attribute Evidence[string], child Evidence[bool], barline bool) Evidence[bool] {
	if attribute.State == EvidenceAbsent {
		return Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	if attribute.State != EvidenceKnown {
		return stateUnavailable[string, bool](attribute)
	}
	if !barline {
		return Evidence[bool]{State: EvidenceKnown, Value: true}
	}
	if child.State == EvidenceAbsent {
		return Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	return child
}
func playbackEither(a, b Evidence[bool]) Evidence[bool] {
	if a.State == EvidenceKnown && a.Value || b.State == EvidenceKnown && b.Value {
		return Evidence[bool]{State: EvidenceKnown, Value: true}
	}
	if a.State != EvidenceKnown || b.State != EvidenceKnown {
		return stateMergeGap(a, b)
	}
	return Evidence[bool]{State: EvidenceKnown, Value: false}
}
func (r *stateReader) navigation(n *factNode, barline bool) navigationContext {
	out := navigationContext{SegnoElement: Evidence[bool]{State: EvidenceAbsent}, CodaElement: Evidence[bool]{State: EvidenceAbsent}, Dacapo: Evidence[bool]{State: EvidenceAbsent}, ForwardRepeat: Evidence[bool]{State: EvidenceAbsent}, Dalsegno: Evidence[string]{State: EvidenceAbsent}, Tocoda: Evidence[string]{State: EvidenceAbsent}, Fine: Evidence[string]{State: EvidenceAbsent}, TimeOnly: Evidence[string]{State: EvidenceAbsent}}
	divisions, segno, coda := timelineRoleSoundDivisions, timelineRoleSoundSegno, timelineRoleSoundCoda
	if barline {
		divisions, segno, coda = timelineRoleBarlineDivisions, timelineRoleBarlineSegno, timelineRoleBarlineCoda
		_, out.SegnoElement = r.child(n, timelineRoleBarlineSegnoElement)
		_, out.CodaElement = r.child(n, timelineRoleBarlineCodaElement)
	} else {
		out.Dacapo = stateYesNo(r.attribute(n, timelineRoleSoundDacapo))
		out.ForwardRepeat = stateYesNo(r.attribute(n, timelineRoleSoundForwardRepeat))
		out.Dalsegno = stateText(r.attribute(n, timelineRoleSoundDalsegno))
		out.Tocoda = stateText(r.attribute(n, timelineRoleSoundTocoda))
		out.Fine = stateText(r.attribute(n, timelineRoleSoundFine))
		out.TimeOnly = stateText(r.attribute(n, timelineRoleSoundTimeOnly))
	}
	out.Divisions = stateDecimalValue(r.attribute(n, divisions))
	out.Segno = stateText(r.attribute(n, segno))
	out.Coda = stateText(r.attribute(n, coda))
	out.DivisionsApplicable = playbackEither(playbackMarker(out.Segno, out.SegnoElement, barline), playbackMarker(out.Coda, out.CodaElement, barline))
	return out
}
func (r *stateReader) playbackInstructions(n *factNode, part Evidence[NodeRef], role timelineRole, scope playbackScope) ([]playbackInstruction, Evidence[bool]) {
	children, complete := r.children(n, role)
	if r.err != nil {
		return nil, complete
	}
	if err := stateRetainCount(r.s, len(children), 512); err != nil {
		r.err = err
		return nil, stateUnknown[bool]("timeline-interrupted")
	}
	out := make([]playbackInstruction, 0, len(children))
	for _, ch := range children {
		if !r.step(1) {
			return nil, stateUnknown[bool]("timeline-interrupted")
		}
		id := Evidence[NodeRef]{State: EvidenceAbsent}
		if part.State == EvidenceKnown {
			id = r.reference(ch, timelineRolePlayID, part.Value)
		} else {
			raw := r.attribute(ch, timelineRolePlayID)
			if raw.State != EvidenceAbsent {
				id = stateUnavailable[NodeRef, NodeRef](part)
			}
		}
		value := r.play(ch)
		if r.err != nil {
			return nil, stateUnknown[bool]("timeline-interrupted")
		}
		out = append(out, playbackInstruction{Source: NodeRef{r.s.documentID, ch.header.ID}, Instrument: id, Scope: scope, Value: value})
	}
	return out, complete
}

// A location describes the barline's boundary role; it does not license moving
// an observed cursor or inventing a right-boundary coordinate from nominal meter
// or final XML child position. B7 compares this role to independently proved
// boundary facts. Middle barlines keep their exact interior score coordinates.
func (p *timelinePlaybackContext) readBarline(ctx context.Context, s *Session, b *timelineBindings, n *factNode, block timelineMeasureBlock, fact timelineEventFact) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.matches(n, timelineRoleBarline) {
		if r.err != nil {
			return r.err
		}
		return contract("barline context", "exact barline declaration required")
	}
	out := barlinePlaybackContext{Origin: playbackOriginAt(s, n, block, fact), RawLocation: stateText(r.attribute(n, timelineRoleBarlineLocation)), Navigation: r.navigation(n, true)}
	out.Location = out.RawLocation
	if out.RawLocation.State == EvidenceAbsent {
		out.Location = Evidence[string]{State: EvidenceKnown, Value: "right"}
		out.LocationDefaulted = true
	}
	if r.err != nil {
		return r.err
	}
	return playbackStore(ctx, s, &p.Barlines, n.header.ID, out)
}

// A note-local instruction remains attached to this exact note even when it is
// a chord tone. The principal is context only, never an expanded recipient set.
func (p *timelinePlaybackContext) readNote(ctx context.Context, s *Session, b *timelineBindings, n *factNode, block timelineMeasureBlock, fact timelineEventFact) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.matches(n, timelineRoleNote) {
		if r.err != nil {
			return r.err
		}
		return contract("note playback", "exact note declaration required")
	}
	out := notePlaybackContext{Origin: playbackOriginAt(s, n, block, fact), Principal: fact.Principal, Dynamics: stateDecimalValue(r.attribute(n, timelineRoleNoteDynamics)), EndDynamics: stateDecimalValue(r.attribute(n, timelineRoleNoteEndDynamics)), Pizzicato: stateYesNo(r.attribute(n, timelineRoleNotePizzicato)), TimeOnly: stateText(r.attribute(n, timelineRoleNoteTimeOnly))}
	_, out.Chord = r.child(n, timelineRoleNoteChord)
	if out.Chord.State == EvidenceAbsent {
		out.Chord = Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	out.Play, out.PlayComplete = r.playbackInstructions(n, fact.Part, timelineRoleNotePlay, playbackCurrentNote)
	if r.err != nil {
		return r.err
	}
	return playbackStore(ctx, s, &p.Notes, n.header.ID, out)
}

func playbackPublish[T any](ctx context.Context, s *Session, h *stateHistory[T], origin playbackOrigin, condition Evidence[string], value Evidence[T]) error {
	// Omission is no update, never an implicit reset to a default.
	if value.State == EvidenceAbsent {
		return s.step(ctx, 1)
	}
	if condition.State == EvidenceKnown {
		value = stateUnavailable[bool, T](stateMergeGap(stateUnavailable[T, bool](value), stateUnknown[bool]("playback-repeat-pass-unavailable")))
	} else if condition.State != EvidenceAbsent {
		value = stateUnavailable[bool, T](stateMergeGap(stateUnavailable[T, bool](value), stateUnavailable[string, bool](condition)))
	}
	if origin.Part.State != EvidenceKnown {
		return s.step(ctx, 1)
	}
	if origin.Position.State != EvidenceKnown {
		gap := stateUnavailable[musicalPosition, bool](origin.Position)
		if gap.State == EvidenceAbsent || gap.State == "" {
			gap = stateUnknown[bool]("playback-position-unavailable")
		}
		gap = stateMergeGap(gap, stateUnavailable[T, bool](value))
		return addStateGap(ctx, s, h, stateGap{Part: origin.Part.Value, FirstMeasure: origin.MeasureOrdinal, LastMeasure: origin.MeasureOrdinal, Scope: stateScope{AllStaves: true}, Availability: gap})
	}
	return publishState(ctx, s, h, statePublication[T]{Position: origin.Position.Value, Scope: stateScope{AllStaves: true}, Value: value, Provenance: []stateProvenance{{Node: origin.Source, Kind: stateExplicit}}})
}
func (p *timelinePlaybackContext) readSound(ctx context.Context, s *Session, b *timelineBindings, n *factNode, block timelineMeasureBlock, fact timelineEventFact) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.matches(n, timelineRoleSound) && !r.matches(n, timelineRoleDirectionSound) {
		if r.err != nil {
			return r.err
		}
		return contract("sound playback", "exact sound declaration required")
	}
	out := soundPlaybackContext{Origin: playbackOriginAt(s, n, block, fact), Tempo: stateDecimalValue(r.attribute(n, timelineRoleSoundTempo)), Dynamics: stateDecimalValue(r.attribute(n, timelineRoleSoundDynamics)), Pizzicato: stateYesNo(r.attribute(n, timelineRoleSoundPizzicato)), TimeOnly: stateText(r.attribute(n, timelineRoleSoundTimeOnly)), Navigation: r.navigation(n, false)}
	out.Play, out.PlayComplete = r.playbackInstructions(n, fact.Part, timelineRoleSoundPlay, playbackFollowingNotes)
	if r.err != nil {
		return r.err
	}
	if err := playbackStore(ctx, s, &p.Sounds, n.header.ID, out); err != nil {
		return err
	}
	if err := playbackPublish(ctx, s, &p.Tempo, out.Origin, out.TimeOnly, out.Tempo); err != nil {
		return err
	}
	if err := playbackPublish(ctx, s, &p.Dynamics, out.Origin, out.TimeOnly, out.Dynamics); err != nil {
		return err
	}
	return playbackPublish(ctx, s, &p.Pizzicato, out.Origin, out.TimeOnly, out.Pizzicato)
}
func (p *timelinePlaybackContext) effectiveTempo(ctx context.Context, s *Session, h *timelineState, pos musicalPosition) (stateResult[Decimal], error) {
	return queryState(ctx, s, h, &p.Tempo, pos, Evidence[Integer]{State: EvidenceKnown, Value: stateOne()}, stateWhole[Decimal])
}
func (p *timelinePlaybackContext) effectiveDynamics(ctx context.Context, s *Session, h *timelineState, pos musicalPosition) (stateResult[Decimal], error) {
	out, err := queryState(ctx, s, h, &p.Dynamics, pos, Evidence[Integer]{State: EvidenceKnown, Value: stateOne()}, stateWhole[Decimal])
	if err != nil {
		return out, err
	}
	// The official tutorial specifies MIDI velocity 90 (forte); sound dynamics
	// are percentages of that value, hence 100 percent on established absence.
	if out.Value.State == EvidenceAbsent {
		if err := s.step(ctx, 1); err != nil {
			return stateResult[Decimal]{}, err
		}
		if err := s.retain(256); err != nil {
			return stateResult[Decimal]{}, err
		}
		out.Value = Evidence[Decimal]{State: EvidenceKnown, Value: noteTimingHundred}
		out.Provenance = []stateProvenance{{Node: pos.Part, Kind: stateInitialDefault}}
	}
	return out, nil
}
func (p *timelinePlaybackContext) effectivePizzicato(ctx context.Context, s *Session, h *timelineState, pos musicalPosition) (stateResult[bool], error) {
	return queryState(ctx, s, h, &p.Pizzicato, pos, Evidence[Integer]{State: EvidenceKnown, Value: stateOne()}, stateWhole[bool])
}
