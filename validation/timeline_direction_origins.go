package validation

import "context"

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

type directionCandidateGap struct {
	Value    Evidence[NodeRef]
	Position Evidence[musicalPosition]
	Ordinal  uint64
}

// An unavailable position can affect its own measure, not arbitrarily earlier
// measures. Keep the earliest potentially relevant gap; a later bad selector
// cannot displace an earlier proved candidate.
func (g *directionCandidateGap) add(r *stateReader, value Evidence[NodeRef], position Evidence[musicalPosition], ordinal uint64) {
	if !r.step(1) {
		return
	}
	if g.Value.State != "" {
		if ordinal > g.Ordinal || ordinal == g.Ordinal && g.Position.State != EvidenceKnown {
			return
		}
		if ordinal == g.Ordinal && position.State == EvidenceKnown {
			cmp, err := r.s.rationalCmp(r.ctx, position.Value.Offset, g.Position.Value.Offset)
			if err != nil {
				r.err = err
				return
			}
			if cmp >= 0 {
				return
			}
		}
	}
	g.Value, g.Position, g.Ordinal = value, position, ordinal
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
	if want.Part != origin.Part.Value {
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
	best := Evidence[NodeRef]{State: EvidenceAbsent}
	bestFact := empty
	gap := directionCandidateGap{}
	tied := false
	for _, block := range blocks {
		if !r.step(1) {
			return stateUnknown[NodeRef]("timeline-interrupted"), empty
		}
		if block.ordinal < want.MeasureOrdinal || block.part.State == EvidenceKnown && block.part.Value != want.Part {
			continue
		}
		if best.State == EvidenceKnown && block.ordinal > bestFact.Position.Value.MeasureOrdinal {
			continue
		}
		if block.part.State != EvidenceKnown {
			gap.add(r, timelineGapAs[NodeRef, NodeRef](block.part, "direction-candidate-part-unavailable"), Evidence[musicalPosition]{State: EvidenceUnknown}, block.ordinal)
			continue
		}
		if block.ordinal == want.MeasureOrdinal && (NodeRef{r.s.documentID, block.measure.header.ID}) != want.Measure {
			gap.add(r, stateUnknown[NodeRef]("direction-measure-domain-conflict"), Evidence[musicalPosition]{State: EvidenceUnknown}, block.ordinal)
			continue
		}
		if block.complete.State != EvidenceKnown || !block.complete.Value {
			gap.add(r, timelineGapAs[bool, NodeRef](block.complete, "direction-candidate-domain-incomplete"), Evidence[musicalPosition]{State: EvidenceUnknown}, block.ordinal)
		}
		for _, candidate := range block.content.children {
			if !r.step(1) {
				return stateUnknown[NodeRef]("timeline-interrupted"), empty
			}
			if !r.matches(candidate, timelineRoleNote) {
				if candidate.header.Declaration.State != EvidenceKnown && r.potential(candidate, timelineRoleNote) {
					gap.add(r, timelineGapAs[DeclarationID, NodeRef](candidate.header.Declaration, "direction-candidate-binding-unavailable"), Evidence[musicalPosition]{State: EvidenceUnknown}, block.ordinal)
				}
				continue
			}
			fact, found := service.Facts[candidate.header.ID]
			if !found {
				gap.add(r, stateUnknown[NodeRef]("direction-note-fact-unavailable"), Evidence[musicalPosition]{State: EvidenceUnknown}, block.ordinal)
				continue
			}
			if fact.Part.State == EvidenceKnown && fact.Part.Value != want.Part {
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
				gap.add(r, timelineGapAs[musicalPosition, NodeRef](fact.Position, "direction-note-position-unavailable"), fact.Position, block.ordinal)
				continue
			}
			at := fact.Position.Value
			if at.Part != want.Part || at.MeasureOrdinal != block.ordinal || at.Measure != (NodeRef{r.s.documentID, block.measure.header.ID}) {
				continue
			}
			cmp, err := stateComparePosition(r.ctx, r.s, at, want)
			if err != nil {
				r.err = err
				return stateUnknown[NodeRef]("timeline-interrupted"), empty
			}
			if cmp < 0 {
				continue
			}
			if origin.Voice.State == EvidenceKnown && fact.Voice.State != EvidenceKnown {
				gap.add(r, timelineGapAs[string, NodeRef](fact.Voice, "direction-note-voice-unavailable"), fact.Position, block.ordinal)
				continue
			}
			if best.State == EvidenceKnown {
				if at.MeasureOrdinal == bestFact.Position.Value.MeasureOrdinal && at.Measure != bestFact.Position.Value.Measure {
					gap.add(r, stateUnknown[NodeRef]("direction-measure-domain-conflict"), fact.Position, block.ordinal)
					continue
				}
				cmp, err = stateComparePosition(r.ctx, r.s, at, bestFact.Position.Value)
				if err != nil {
					r.err = err
					return stateUnknown[NodeRef]("timeline-interrupted"), empty
				}
				if cmp > 0 {
					continue
				}
				if cmp == 0 {
					tied = true
					continue
				}
			}
			best = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, candidate.header.ID}}
			bestFact, tied = fact, false
		}
	}
	if gap.Value.State != "" {
		if best.State != EvidenceKnown || gap.Ordinal < bestFact.Position.Value.MeasureOrdinal || gap.Ordinal == bestFact.Position.Value.MeasureOrdinal && gap.Position.State != EvidenceKnown {
			return gap.Value, empty
		}
		if gap.Ordinal == bestFact.Position.Value.MeasureOrdinal {
			cmp, err := r.s.rationalCmp(r.ctx, gap.Position.Value.Offset, bestFact.Position.Value.Offset)
			if err != nil {
				r.err = err
				return stateUnknown[NodeRef]("timeline-interrupted"), empty
			}
			if cmp <= 0 {
				return gap.Value, empty
			}
		}
	}
	if tied {
		return stateUnknown[NodeRef]("direction-simultaneous-notes-unresolved"), empty
	}
	return best, bestFact
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
			if _, exists := service.DirectionOrigins[event.header.ID]; exists {
				return contract("direction origins", "duplicate publication")
			}
			if err := s.account.Charge(BudgetContextFacts, 1); err != nil {
				return err
			}
			if err := s.retain(2048); err != nil {
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
		}
	}
	return r.err
}
