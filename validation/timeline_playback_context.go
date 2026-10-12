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
	Pan, Elevation  Evidence[Decimal]
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

// Only these freshly assembled record types may transfer their owned slices to
// the map. playbackInstructions and stateReader.play admit and allocate new Play
// and Technique backing on every call. No history/store slice is borrowed here.
type playbackOwnedRecord interface {
	notePlaybackContext | soundPlaybackContext | barlinePlaybackContext
}

// Large Go map values are indirect: growth copies pointers, not these records.
// Each allowance covers the fixed value plus at least 512 bytes for key/control/
// pointer tables and concurrent map growth. Initial header/group allocation is
// separate. Architecture-specific width guards are in the retention tests.
const (
	playbackNoteReserveBytes    int64 = 1536
	playbackSoundReserveBytes   int64 = 2560
	playbackBarlineReserveBytes int64 = 2048
	playbackMapReserveBytes     int64 = 256
)

func playbackStore[T playbackOwnedRecord](ctx context.Context, s *Session, records *map[NodeID]T, id NodeID, v T) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if _, exists := (*records)[id]; exists {
		return contract("playback context", "duplicate publication")
	}
	var bytes int64
	switch any(v).(type) {
	case notePlaybackContext:
		bytes = playbackNoteReserveBytes
	case soundPlaybackContext:
		bytes = playbackSoundReserveBytes
	case barlinePlaybackContext:
		bytes = playbackBarlineReserveBytes
	}
	if err := s.account.Charge(BudgetContextFacts, 1); err != nil {
		return err
	}
	// ownOrigin's bounded reflected temporary can coexist with the indirect map
	// value. Account that logical peak separately and release it on every exit;
	// this is not a claim about compiler allocation scheduling or process RSS.
	if err := s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }()
	if err := s.retain(bytes); err != nil {
		return err
	}
	if *records == nil {
		if err := s.retain(playbackMapReserveBytes); err != nil {
			return err
		}
	}
	// Immutable numerics/strings and the already-admitted uniquely owned vectors
	// need no duplicate retention or clone. Copy every mutable cause independently
	// before publication; cancellation/quota failure never publishes a partial row.
	if err := ownOrigin(ctx, s, &v); err != nil {
		return err
	}
	if *records == nil {
		*records = make(map[NodeID]T)
	}
	(*records)[id] = v
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
	out := barlinePlaybackContext{Origin: playbackOriginAt(s, n, block, fact), RawLocation: stateText(r.playbackRawAttribute(n, timelineRoleBarlineLocation)), Navigation: r.navigation(n, true)}
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
	if origin.Part.State != EvidenceKnown {
		return s.step(ctx, 1)
	}
	if origin.Position.State != EvidenceKnown {
		gap := stateUnavailable[musicalPosition, bool](origin.Position)
		if gap.State == EvidenceAbsent || gap.State == "" {
			gap = stateUnknown[bool]("playback-position-unavailable")
		}
		gap = stateMergeGap(gap, stateUnavailable[T, bool](stateConditionalValue(value, &condition)))
		return addStateGap(ctx, s, h, stateGap{Part: origin.Part.Value, FirstMeasure: origin.MeasureOrdinal, LastMeasure: origin.MeasureOrdinal, Scope: stateScope{AllStaves: true}, Availability: gap})
	}
	// Keep the guard on the publication so conditional changes cannot repair
	// prior unavailable state. Projection belongs to the property query.
	var guard *Evidence[string]
	if !stateUnconditional(&condition) {
		guard = &condition
	}
	return publishState(ctx, s, h, statePublication[T]{TimeOnly: guard, Position: origin.Position.Value, Scope: stateScope{AllStaves: true}, Value: value, Provenance: []stateProvenance{{Node: origin.Source, Kind: stateExplicit}}})
}
func (p *timelinePlaybackContext) readSound(ctx context.Context, s *Session, b *timelineBindings, n *factNode, block timelineMeasureBlock, fact timelineEventFact) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.matches(n, timelineRoleSound) && !r.matches(n, timelineRoleDirectionSound) {
		if r.err != nil {
			return r.err
		}
		return contract("sound playback", "exact sound declaration required")
	}
	out := soundPlaybackContext{Pan: stateDecimalValue(r.attribute(n, timelineRoleSoundPan)), Elevation: stateDecimalValue(r.attribute(n, timelineRoleSoundElevation)), Origin: playbackOriginAt(s, n, block, fact), Tempo: stateDecimalValue(r.attribute(n, timelineRoleSoundTempo)), Dynamics: stateDecimalValue(r.attribute(n, timelineRoleSoundDynamics)), Pizzicato: stateYesNo(r.attribute(n, timelineRoleSoundPizzicato)), TimeOnly: stateText(r.attribute(n, timelineRoleSoundTimeOnly)), Navigation: r.navigation(n, false)}
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
	// The official tutorial's velocity90 corresponds to dynamics100 percent.
	// It is a definite initial baseline, including when a floating statement
	// may or may not yet have applied and agrees with that documented value.
	initial := func() (stateResult[Decimal], error) {
		if err := s.step(ctx, 1); err != nil {
			return stateResult[Decimal]{}, err
		}
		if err := s.retain(256); err != nil {
			return stateResult[Decimal]{}, err
		}
		return stateResult[Decimal]{Value: Evidence[Decimal]{State: EvidenceKnown, Value: noteTimingHundred}, Provenance: []stateProvenance{{Node: pos.Part, Kind: stateInitialDefault}}, Inherited: true}, nil
	}
	return queryStateAtFull(ctx, s, h, &p.Dynamics, pos, Evidence[Integer]{State: EvidenceKnown, Value: stateOne()}, stateWhole[Decimal], true, true, initial)
}

func (p *timelinePlaybackContext) effectivePizzicato(ctx context.Context, s *Session, h *timelineState, pos musicalPosition) (stateResult[bool], error) {
	return queryState(ctx, s, h, &p.Pizzicato, pos, Evidence[Integer]{State: EvidenceKnown, Value: stateOne()}, stateWhole[bool])
}

// gapAll is used only when hierarchy/enumeration loss can hide a sound publisher.
// A single unplaced known sound instead gaps its actually encoded families in
// playbackPublish. Earlier established measures retain their independent facts.
func (p *timelinePlaybackContext) gapAll(ctx context.Context, s *Session, part NodeRef, ordinal uint64, availability Evidence[musicalPosition]) error {
	if availability.State == EvidenceKnown || availability.State == EvidenceAbsent || availability.State == "" {
		return contract("playback gap", "causal unavailable evidence required")
	}
	gap := stateGap{Part: part, FirstMeasure: ordinal, LastMeasure: ordinal, Scope: stateScope{AllStaves: true}, Availability: stateUnavailable[musicalPosition, bool](availability)}
	if err := addStateGap(ctx, s, &p.Tempo, gap); err != nil {
		return err
	}
	if err := addStateGap(ctx, s, &p.Dynamics, gap); err != nil {
		return err
	}
	return addStateGap(ctx, s, &p.Pizzicato, gap)
}

// Preserve physical presence separately from an XSD defaulted effective scalar.
// Exact declaration/name matching and validated values remain shared with XSD.
func (r *stateReader) playbackRawAttribute(n *factNode, role timelineRole) Evidence[Atomic] {
	selector, ok := r.b.attribute(role)
	if !ok {
		return Evidence[Atomic]{State: EvidenceUnsupported, Reason: "timeline-attribute-binding-unavailable"}
	}
	if r.err != nil {
		return stateUnknown[Atomic]("timeline-interrupted")
	}
	presence, _, err := r.s.attributeEvidence(r.ctx, n, selector)
	if err != nil {
		r.err = err
		return stateUnknown[Atomic]("timeline-interrupted")
	}
	if presence.State != EvidenceKnown {
		return stateUnavailable[bool, Atomic](presence)
	}
	return r.attribute(n, role)
}
