package validation

import "context"

// Indirect map record plus at least 512 bytes of map growth headroom.
const timelineDirectionReserveBytes int64 = 1536

// This is the optional note association described by the direction annotation,
// not a command to attach every direction or a replacement for its visual or
// playback position. Staff is retained, but is not an attachment filter: the
// clause excludes different voices, and one voice may cross staves.
type directionTimingOrigin struct {
	Direction                     NodeRef
	Part                          Evidence[NodeRef]
	Staff                         Evidence[Integer]
	Voice                         Evidence[string]
	CursorOrigin, DisplayPosition Evidence[musicalPosition]
	Anchor                        Evidence[NodeRef]
	AnchorPosition                Evidence[musicalPosition]
	AnchorStaff                   Evidence[Integer]
	AnchorVoice                   Evidence[string]
}

// A nominal ordering is sufficient for indexing, but never certifies a
// displaced candidate's chronology. Two passes keep constant scratch: select a
// definitely eligible minimum, then check every possible competitor against it.
// The second pass also preserves the strongest actual unavailable cause.
type timelineFollowingSearch struct {
	state            *timelineState
	want             musicalPosition
	best             Evidence[NodeRef]
	fact             timelineEventFact
	gap              Evidence[NodeRef]
	pass             int
	chronology, tied Code
}

func (q *timelineFollowingSearch) add(r *stateReader, value Evidence[NodeRef], position Evidence[musicalPosition], ordinal uint64) {
	q.addPlaced(r, value, position, ordinal, stateGapPlacement{})
}
func (q *timelineFollowingSearch) addPlaced(r *stateReader, value Evidence[NodeRef], position Evidence[musicalPosition], ordinal uint64, placement stateGapPlacement) {
	if q.pass == 0 || !r.step(1) {
		return
	}
	if position.State == EvidenceKnown {
		at := position.Value
		order, err := q.state.positionOrder(r.ctx, r.s, at, q.want)
		if err != nil {
			r.err = err
			return
		}
		if order == stateBefore {
			return
		}
		if q.best.State == EvidenceKnown {
			if at.MeasureOrdinal == q.fact.Position.Value.MeasureOrdinal && !r.sameRef(at.Measure, q.fact.Position.Value.Measure) {
				q.gap = graceStronger(q.gap, value)
				return
			}
			order, err = q.state.positionOrder(r.ctx, r.s, at, q.fact.Position.Value)
			if err != nil {
				r.err = err
				return
			}
			if order == stateAfter {
				return
			}
		}
	} else if placement.Displaced {
		best := Evidence[musicalPosition]{State: EvidenceAbsent}
		if q.best.State == EvidenceKnown {
			best = q.fact.Position
		}
		relevant, err := q.state.associationPlacementRelevant(r, placement, Evidence[musicalPosition]{State: EvidenceKnown, Value: q.want}, best)
		if err != nil {
			r.err = err
			return
		}
		if !relevant {
			return
		}
	} else {
		// Unplaced evidence retains its canonical measure scope; a known displaced
		// position above must instead pass the partial-chronology test.
		member, err := q.state.frameMember(r.ctx, r.s, q.want)
		if err != nil {
			r.err = err
			return
		}
		if member && ordinal < q.want.MeasureOrdinal {
			return
		}
		if q.best.State == EvidenceKnown && ordinal > q.fact.Position.Value.MeasureOrdinal {
			member, err = q.state.frameMember(r.ctx, r.s, q.fact.Position.Value)
			if err != nil {
				r.err = err
				return
			}
			if member {
				return
			}
		}
	}
	q.gap = graceStronger(q.gap, value)
}

// Unavailable coordinates reached through an unproved displacement are not
// confined by their source measure. Independent endpoint bounds can still
// exclude them from this one closed candidate interval.
func (state *timelineState) associationPlacementRelevant(r *stateReader, placement stateGapPlacement, lo, hi Evidence[musicalPosition]) (bool, error) {
	for _, bound := range [2]struct {
		endpoint *musicalPosition
		edge     Evidence[musicalPosition]
		excluded statePositionOrder
	}{
		{placement.Upper, lo, stateBefore}, {placement.Lower, hi, stateAfter},
	} {
		if !r.step(1) {
			return false, r.err
		}
		if bound.endpoint == nil || bound.edge.State != EvidenceKnown {
			continue
		}
		order, err := state.positionOrder(r.ctx, r.s, *bound.endpoint, bound.edge.Value)
		if err != nil {
			return false, err
		}
		if order == bound.excluded {
			return false, nil
		}
	}
	return true, nil
}

func (q *timelineFollowingSearch) candidate(r *stateReader, value Evidence[NodeRef], fact timelineEventFact) {
	if !r.step(1) {
		return
	}
	at := fact.Position.Value
	order, err := q.state.positionOrder(r.ctx, r.s, at, q.want)
	if err != nil {
		r.err = err
		return
	}
	if order == stateBefore {
		return
	}
	if q.pass == 1 {
		if q.best.State == EvidenceKnown && r.sameRef(value.Value, q.best.Value) {
			return
		}
		reason := q.chronology
		if q.best.State == EvidenceKnown {
			relation, err := q.state.positionOrder(r.ctx, r.s, at, q.fact.Position.Value)
			if err != nil {
				r.err = err
				return
			}
			if relation == stateEqual {
				reason = q.tied
			}
		}
		q.add(r, stateUnknown[NodeRef](reason), fact.Position, at.MeasureOrdinal)
		return
	}
	if order&stateBefore != 0 {
		return
	}
	if q.best.State == EvidenceKnown {
		relation, err := q.state.positionOrder(r.ctx, r.s, at, q.fact.Position.Value)
		if err != nil {
			r.err = err
			return
		}
		if relation != stateBefore {
			return
		}
	}
	q.best, q.fact = value, fact
}

func (service *timelineService) directionAnchor(r *stateReader, origin timelineEventFact, blocks []timelineMeasureBlock) (Evidence[NodeRef], timelineEventFact) {
	empty := timelineEventFact{}
	if origin.Part.State != EvidenceKnown {
		return timelineGapAs[NodeRef, NodeRef](origin.Part, "direction-part-unavailable"), empty
	}
	if origin.Origin.State != EvidenceKnown {
		return timelineGapAs[musicalPosition, NodeRef](origin.Origin, "direction-origin-unavailable"), empty
	}
	if origin.Voice.State != EvidenceKnown && origin.Voice.State != EvidenceAbsent {
		return timelineGapAs[string, NodeRef](origin.Voice, "direction-voice-unavailable"), empty
	}
	want := origin.Origin.Value
	if !r.sameRef(want.Part, origin.Part.Value) {
		return stateUnknown[NodeRef]("direction-part-position-conflict"), empty
	}
	complete, err := service.State.closure(r.ctx, r.s, want)
	if err != nil {
		r.err = err
		return stateUnknown[NodeRef]("timeline-interrupted"), empty
	}
	if complete.State != EvidenceKnown || !complete.Value {
		return timelineGapAs[bool, NodeRef](complete, "direction-part-incomplete"), empty
	}
	q := timelineFollowingSearch{state: &service.State, want: want, best: Evidence[NodeRef]{State: EvidenceAbsent}, gap: Evidence[NodeRef]{State: EvidenceKnown}, chronology: "direction-cross-frame-chronology-unresolved", tied: "direction-simultaneous-notes-unresolved"}
	for q.pass = 0; q.pass < 2; q.pass++ {
		for _, block := range blocks {
			if !r.step(1) {
				return stateUnknown[NodeRef]("timeline-interrupted"), empty
			}
			if block.part.State == EvidenceKnown && !r.sameRef(block.part.Value, want.Part) {
				continue
			}
			if block.part.State != EvidenceKnown {
				q.add(r, timelineGapAs[NodeRef, NodeRef](block.part, "direction-candidate-part-unavailable"), Evidence[musicalPosition]{State: EvidenceUnknown}, block.ordinal)
				continue
			}
			if block.ordinal == want.MeasureOrdinal && !r.sameRef(NodeRef{r.s.documentID, block.measure.header.ID}, want.Measure) {
				q.add(r, stateUnknown[NodeRef]("direction-measure-domain-conflict"), Evidence[musicalPosition]{State: EvidenceUnknown}, block.ordinal)
				continue
			}
			if block.complete.State != EvidenceKnown || !block.complete.Value {
				q.add(r, timelineGapAs[bool, NodeRef](block.complete, "direction-candidate-domain-incomplete"), Evidence[musicalPosition]{State: EvidenceUnknown}, block.ordinal)
			}
			for _, candidate := range block.content.children {
				if !r.step(1) {
					return stateUnknown[NodeRef]("timeline-interrupted"), empty
				}
				if !r.matches(candidate, timelineRoleNote) {
					if candidate.header.Declaration.State != EvidenceKnown && r.potential(candidate, timelineRoleNote) {
						q.add(r, timelineGapAs[DeclarationID, NodeRef](*candidate.header.Declaration, "direction-candidate-binding-unavailable"), Evidence[musicalPosition]{State: EvidenceUnknown}, block.ordinal)
					}
					continue
				}
				fact, found := service.Facts[candidate.header.ID]
				if !found {
					q.add(r, stateUnknown[NodeRef]("direction-note-fact-unavailable"), Evidence[musicalPosition]{State: EvidenceUnknown}, block.ordinal)
					continue
				}
				if fact.Part.State == EvidenceKnown && !r.sameRef(fact.Part.Value, want.Part) {
					continue
				}
				if origin.Voice.State == EvidenceKnown && fact.Voice.State == EvidenceKnown {
					if !r.step(int64(len(origin.Voice.Value)) + int64(len(fact.Voice.Value))) {
						return stateUnknown[NodeRef]("timeline-interrupted"), empty
					}
					if origin.Voice.Value != fact.Voice.Value {
						continue
					}
				}
				if fact.Position.State != EvidenceKnown {
					q.addPlaced(r, timelineGapAs[musicalPosition, NodeRef](fact.Position, "direction-note-position-unavailable"), fact.Position, block.ordinal, fact.Placement)
					continue
				}
				at := fact.Position.Value
				if !r.sameRef(at.Part, want.Part) || at.MeasureOrdinal != block.ordinal || !r.sameRef(at.Measure, NodeRef{r.s.documentID, block.measure.header.ID}) {
					continue
				}

				if origin.Voice.State == EvidenceKnown && fact.Voice.State != EvidenceKnown {
					q.add(r, timelineGapAs[string, NodeRef](fact.Voice, "direction-note-voice-unavailable"), fact.Position, block.ordinal)
					continue
				}
				q.candidate(r, Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, candidate.header.ID}}, fact)
			}
		}
	}
	if q.gap.State != EvidenceKnown {
		return q.gap, empty
	}
	return q.best, q.fact

}

// Run once after every measure has published its exact positions. This enables
// an end-of-measure direction to find a following note in the same part without
// interpreting a printed measure number or selecting a lexical neighbor.
func (service *timelineService) resolveDirectionOrigins(ctx context.Context, s *Session, b *timelineBindings, blocks []timelineMeasureBlock) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.step(1) {
		return r.err
	}
	index, release, err := service.buildDirectionNoteIndex(r, blocks)
	if err != nil {
		return err
	}
	defer release()
	for _, block := range blocks {
		if !r.step(1) {
			return r.err
		}
		for _, event := range block.content.children {
			if !r.matches(event, timelineRoleDirection) {
				if r.err != nil {
					return r.err
				}
				continue
			}
			if err := service.resolveDirectionOrigin(r, event, block, blocks, index); err != nil {
				return err
			}
		}
	}
	return r.err
}

func (service *timelineService) resolveDirectionOrigin(r *stateReader, event *factNode, block timelineMeasureBlock, blocks []timelineMeasureBlock, index directionNoteIndex) error {
	ctx, s := r.ctx, r.s
	if _, exists := service.DirectionOrigins[event.header.ID]; exists {
		return contract("direction origins", "duplicate publication")
	}
	if err := s.account.Charge(BudgetContextFacts, 1); err != nil {
		return err
	}
	if err := s.account.Charge(BudgetBufferedBytes, timelineDirectionReserveBytes); err != nil {
		return err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, timelineDirectionReserveBytes) }()
	if err := s.retain(timelineDirectionReserveBytes); err != nil {
		return err
	}
	fact, found := service.Facts[event.header.ID]
	if !found {
		fact = timelineEventFact{Part: block.part, Origin: stateUnknown[musicalPosition]("direction-origin-unavailable"), Position: stateUnknown[musicalPosition]("direction-display-position-unavailable"), Staff: stateUnknown[Integer]("direction-staff-unavailable"), Voice: stateUnknown[string]("direction-voice-unavailable")}
	}
	out := directionTimingOrigin{Direction: NodeRef{s.documentID, event.header.ID}, Part: fact.Part, Staff: fact.Staff, Voice: fact.Voice, CursorOrigin: fact.Origin, DisplayPosition: fact.Position}
	anchor, anchorFact, proved, err := index.anchor(ctx, s, service, fact)
	if err != nil {
		return err
	}
	if !proved {
		anchor, anchorFact = service.directionAnchor(r, fact, blocks)
	}
	out.Anchor = anchor
	if anchor.State == EvidenceKnown {
		out.AnchorPosition, out.AnchorStaff, out.AnchorVoice = anchorFact.Position, anchorFact.Staff, anchorFact.Voice
	} else {
		out.AnchorPosition = stateUnavailable[NodeRef, musicalPosition](anchor)
		out.AnchorStaff = stateUnavailable[NodeRef, Integer](anchor)
		out.AnchorVoice = stateUnavailable[NodeRef, string](anchor)
	}
	if r.err != nil {
		return r.err
	}
	if err := ownOrigin(ctx, s, &out); err != nil {
		return err
	}
	if service.DirectionOrigins == nil {
		service.DirectionOrigins = make(map[NodeID]directionTimingOrigin)
	}
	service.DirectionOrigins[event.header.ID] = out
	return r.err
}
