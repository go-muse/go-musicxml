package validation

import "context"

// Figured bass has no staff or voice selector in MusicXML 4.0. Keep absence
// explicit; do not manufacture staff 1 or use XML order to select between
// simultaneous regular notes. Figures retain their stated top-to-bottom order.
type figuredBassTimingOrigin struct {
	FiguredBass            NodeRef
	Part                   Evidence[NodeRef]
	Staff                  Evidence[Integer]
	CursorOrigin, Position Evidence[musicalPosition]
	Anchor                 Evidence[NodeRef]
	AnchorStaff            Evidence[Integer]
	AnchorVoice            Evidence[string]
	DurationRaw            Evidence[Decimal]
	Figures                []NodeRef
	FiguresComplete        Evidence[bool]
}

func (service *timelineService) figuredBassAnchor(r *stateReader, block timelineMeasureBlock, origin timelineEventFact, complete Evidence[bool]) (Evidence[NodeRef], timelineEventFact) {
	empty := timelineEventFact{}
	if complete.State != EvidenceKnown || !complete.Value {
		return timelineGapAs[bool, NodeRef](complete, "figured-bass-domain-incomplete"), empty
	}
	if origin.Position.State != EvidenceKnown {
		return timelineGapAs[musicalPosition, NodeRef](origin.Position, "figured-bass-position-unavailable"), empty
	}
	if origin.Staff.State != EvidenceKnown && origin.Staff.State != EvidenceAbsent {
		return stateUnavailable[Integer, NodeRef](origin.Staff), empty
	}
	var best *factNode
	bestFact := empty
	gap := Evidence[NodeRef]{State: EvidenceAbsent}
	gapPosition := Evidence[musicalPosition]{State: EvidenceAbsent}
	// A later unavailable note cannot displace an already earlier exact anchor.
	// Unplaced candidate gaps remain global because their score order is unknown.
	addGap := func(value Evidence[NodeRef], position Evidence[musicalPosition]) {
		if gap.State != EvidenceAbsent {
			if gapPosition.State != EvidenceKnown {
				return
			}
			if position.State == EvidenceKnown {
				cmp, err := r.s.rationalCmp(r.ctx, position.Value.Offset, gapPosition.Value.Offset)
				if err != nil {
					r.err = err
					return
				}
				if cmp >= 0 {
					return
				}
			}
		}
		gap, gapPosition = value, position
	}
	tied := false
	for _, candidate := range block.content.children {
		if !r.step(1) {
			return stateUnknown[NodeRef]("timeline-interrupted"), empty
		}
		if !r.matches(candidate, timelineRoleNote) {
			if candidate.header.Declaration.State != EvidenceKnown && r.potential(candidate, timelineRoleNote) {
				addGap(stateUnavailable[DeclarationID, NodeRef](candidate.header.Declaration), Evidence[musicalPosition]{State: EvidenceUnknown})
			}
			continue
		}
		fact, found := service.Facts[candidate.header.ID]
		_, grace := r.child(candidate, timelineRoleNoteGrace)
		_, chord := r.child(candidate, timelineRoleNoteChord)
		if (grace.State == EvidenceKnown && grace.Value) || (chord.State == EvidenceKnown && chord.Value) {
			continue
		}
		if !found {
			addGap(stateUnknown[NodeRef]("figured-bass-note-fact-unavailable"), Evidence[musicalPosition]{State: EvidenceUnknown})
			continue
		}
		if fact.Position.State != EvidenceKnown {
			if fact.Part.State == EvidenceKnown && fact.Part.Value != origin.Position.Value.Part {
				continue
			}
			addGap(timelineGapAs[musicalPosition, NodeRef](fact.Position, "figured-bass-note-position-unavailable"), fact.Position)
			continue
		}
		// Candidate domains come from normalized hierarchy identity, never printed
		// measure numbers or neighboring lexical containers.
		at, want := fact.Position.Value, origin.Position.Value
		if at.Part != want.Part || at.Measure != want.Measure || at.MeasureOrdinal != want.MeasureOrdinal {
			continue
		}
		cmp, err := r.s.rationalCmp(r.ctx, at.Offset, want.Offset)
		if err != nil {
			r.err = err
			return stateUnknown[NodeRef]("timeline-interrupted"), empty
		}
		if cmp < 0 {
			continue
		}
		if origin.Staff.State == EvidenceKnown {
			if fact.Staff.State != EvidenceKnown {
				addGap(timelineGapAs[Integer, NodeRef](fact.Staff, "figured-bass-note-staff-unavailable"), fact.Position)
				continue
			}
			applies, err := stateApplies(r.ctx, r.s, stateScope{Staff: origin.Staff.Value}, fact.Staff)
			if err != nil {
				r.err = err
				return stateUnknown[NodeRef]("timeline-interrupted"), empty
			}
			if !applies {
				continue
			}
		}
		if grace.State != EvidenceAbsent {
			addGap(stateUnavailable[bool, NodeRef](grace), fact.Position)
			continue
		}
		if chord.State != EvidenceAbsent {
			addGap(stateUnavailable[bool, NodeRef](chord), fact.Position)
			continue
		}
		if best == nil {
			best = candidate
			bestFact = fact
			tied = false
			continue
		}
		cmp, err = r.s.rationalCmp(r.ctx, at.Offset, bestFact.Position.Value.Offset)
		if err != nil {
			r.err = err
			return stateUnknown[NodeRef]("timeline-interrupted"), empty
		}
		if cmp < 0 {
			best = candidate
			bestFact = fact
			tied = false
		} else if cmp == 0 {
			tied = true
		}
	}
	if gap.State != EvidenceAbsent {
		if best == nil || gapPosition.State != EvidenceKnown {
			return gap, empty
		}
		cmp, err := r.s.rationalCmp(r.ctx, gapPosition.Value.Offset, bestFact.Position.Value.Offset)
		if err != nil {
			r.err = err
			return stateUnknown[NodeRef]("timeline-interrupted"), empty
		}
		if cmp <= 0 {
			return gap, empty
		}
	}
	if best == nil {
		return stateUnknown[NodeRef]("figured-bass-following-note-unresolved"), empty
	}
	if tied {
		return stateUnknown[NodeRef]("figured-bass-simultaneous-notes-unresolved"), empty
	}
	return Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, best.header.ID}}, bestFact
}

// resolveFiguredBassOrigins runs after every score-position fact in the measure
// has been published. It finds a unique nearest regular note in exact score
// order, without treating a lexically following note as musical evidence.
// Association beyond this established measure domain remains explicit Unknown.
func (service *timelineService) resolveFiguredBassOrigins(ctx context.Context, s *Session, b *timelineBindings, block timelineMeasureBlock) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.step(1) {
		return r.err
	}
	complete := block.complete
	if complete.State == EvidenceKnown && complete.Value {
		complete = r.complete(block.content)
	}
	for _, event := range block.content.children {
		if !r.matches(event, timelineRoleFiguredBass) {
			if r.err != nil {
				return r.err
			}
			continue
		}
		if _, exists := service.FiguredBassOrigins[event.header.ID]; exists {
			return contract("figured bass origins", "duplicate publication")
		}
		if err := s.account.Charge(BudgetContextFacts, 1); err != nil {
			return err
		}
		if err := s.retain(2048); err != nil {
			return err
		}
		fact, found := service.Facts[event.header.ID]
		if !found {
			fact = timelineEventFact{Position: stateUnknown[musicalPosition]("figured-bass-position-unavailable"), Part: block.part, Staff: Evidence[Integer]{State: EvidenceAbsent}}
		}
		out := figuredBassTimingOrigin{FiguredBass: NodeRef{s.documentID, event.header.ID}, Part: fact.Part, Staff: fact.Staff, CursorOrigin: fact.Position, DurationRaw: stateDecimalValue(r.childScalar(event, timelineRoleFiguredBassDuration))}
		figures, enumerated := r.children(event, timelineRoleFiguredBassFigure)
		out.FiguresComplete = enumerated
		bytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(figures)), 32)
		if err != nil {
			return err
		}
		if err = s.retain(bytes); err != nil {
			return err
		}
		out.Figures = make([]NodeRef, len(figures))
		for i, figure := range figures {
			if !r.step(1) {
				return r.err
			}
			out.Figures[i] = NodeRef{s.documentID, figure.header.ID}
		}
		anchor, anchorFact := service.figuredBassAnchor(r, block, fact, complete)
		out.Anchor = anchor
		if anchor.State == EvidenceKnown {
			out.Position = anchorFact.Position
			out.AnchorStaff = anchorFact.Staff
			out.AnchorVoice = anchorFact.Voice
		} else {
			out.Position = stateUnavailable[NodeRef, musicalPosition](anchor)
			out.AnchorStaff = stateUnavailable[NodeRef, Integer](anchor)
			out.AnchorVoice = stateUnavailable[NodeRef, string](anchor)
		}
		if r.err != nil {
			return r.err
		}
		if err = s.step(ctx, 1); err != nil {
			return err
		}
		if err := ownOrigin(ctx, s, &out); err != nil {
			return err
		}
		if service.FiguredBassOrigins == nil {
			service.FiguredBassOrigins = make(map[NodeID]figuredBassTimingOrigin)
		}
		service.FiguredBassOrigins[event.header.ID] = out
	}
	return r.err
}
