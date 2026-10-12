package validation

import "context"

// Figured bass has no staff or voice selector in MusicXML 4.0. Keep absence
// explicit; do not manufacture staff 1 or use XML order to select between
// simultaneous regular notes. Figures retain their stated top-to-bottom order.
type figuredBassTimingOrigin struct {
	FiguredBass                             NodeRef
	Part                                    Evidence[NodeRef]
	Staff                                   Evidence[Integer]
	CursorOrigin, Position                  Evidence[musicalPosition]
	Anchor                                  Evidence[NodeRef]
	AnchorStaff                             Evidence[Integer]
	AnchorVoice                             Evidence[string]
	DurationRaw                             Evidence[Decimal]
	CursorUnits, AnchorUnits, DurationUnits Evidence[Decimal]
	Duration                                Evidence[Rational]
	End                                     Evidence[musicalPosition]
	AnchorPosition, ChainPosition, ChainEnd Evidence[musicalPosition]
	GroupStart                              Evidence[NodeRef]
	ChainInterpretation                     Code
	Figures                                 []NodeRef
	FiguresComplete                         Evidence[bool]
}

type figuredBassDurationKey struct{ raw, units string }

type figuredBassAnchorKey struct {
	part, measure NodeRef
	ordinal       uint64
	offset        string
}
type figuredBassAnchorFact struct {
	anchor Evidence[NodeRef]
	fact   timelineEventFact
}

type figuredBassDomain struct {
	block    timelineMeasureBlock
	complete Evidence[bool]
}

func figuredBassScoreCompare(ctx context.Context, s *Session, a, b musicalPosition) (int, error) {
	if err := s.step(ctx, 1); err != nil {
		return 0, err
	}
	if a.MeasureOrdinal < b.MeasureOrdinal {
		return -1, nil
	}
	if a.MeasureOrdinal > b.MeasureOrdinal {
		return 1, nil
	}
	return s.rationalCmp(ctx, a.Offset, b.Offset)
}

func (service *timelineService) figuredBassAnchor(r *stateReader, domains []figuredBassDomain, origin timelineEventFact, complete Evidence[bool]) (Evidence[NodeRef], timelineEventFact) {
	empty := timelineEventFact{}
	if complete.State != EvidenceKnown || !complete.Value {
		return timelineGapAs[bool, NodeRef](complete, "figured-bass-domain-incomplete"), empty
	}
	if origin.Position.State != EvidenceKnown {
		return timelineGapAs[musicalPosition, NodeRef](origin.Position, "figured-bass-position-unavailable"), empty
	}
	var best *factNode
	bestFact := empty
	gap := Evidence[NodeRef]{State: EvidenceAbsent}
	gapPosition := Evidence[musicalPosition]{State: EvidenceAbsent}
	addGap := func(value Evidence[NodeRef], position Evidence[musicalPosition]) {
		if gap.State != EvidenceAbsent {
			if gapPosition.State != EvidenceKnown {
				return
			}
			if position.State == EvidenceKnown {
				cmp, err := figuredBassScoreCompare(r.ctx, r.s, position.Value, gapPosition.Value)
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
	for _, domain := range domains {
		if !r.step(1) {
			return stateUnknown[NodeRef]("timeline-interrupted"), empty
		}
		block := domain.block
		if block.part.State != EvidenceKnown {
			addGap(timelineGapAs[NodeRef, NodeRef](block.part, "figured-bass-part-unavailable"), Evidence[musicalPosition]{State: EvidenceUnknown})
			continue
		}
		if block.part.Value != origin.Position.Value.Part || block.ordinal < origin.Position.Value.MeasureOrdinal {
			continue
		}
		lower := timelineAt(block, Evidence[Rational]{State: EvidenceKnown, Value: timelineZero})
		if block.ordinal == origin.Position.Value.MeasureOrdinal {
			lower = origin.Position
			if block.measure.header.ID != origin.Position.Value.Measure.Node {
				addGap(stateUnknown[NodeRef]("figured-bass-measure-identity-unresolved"), lower)
				continue
			}
		}
		if domain.complete.State != EvidenceKnown || !domain.complete.Value {
			addGap(timelineGapAs[bool, NodeRef](domain.complete, "figured-bass-domain-incomplete"), lower)
		}
		for _, candidate := range block.content.children {
			if !r.step(1) {
				return stateUnknown[NodeRef]("timeline-interrupted"), empty
			}
			if !r.matches(candidate, timelineRoleNote) {
				if candidate.header.Declaration.State != EvidenceKnown && r.potential(candidate, timelineRoleNote) {
					addGap(stateUnavailable[DeclarationID, NodeRef](candidate.header.Declaration), lower)
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
				addGap(stateUnknown[NodeRef]("figured-bass-note-fact-unavailable"), lower)
				continue
			}
			if fact.Position.State != EvidenceKnown {
				if fact.Part.State == EvidenceKnown && fact.Part.Value != origin.Position.Value.Part {
					continue
				}
				addGap(timelineGapAs[musicalPosition, NodeRef](fact.Position, "figured-bass-note-position-unavailable"), lower)
				continue
			}
			at, want := fact.Position.Value, origin.Position.Value
			if at.Part != want.Part {
				continue
			}
			if at.Measure.Node != block.measure.header.ID || at.MeasureOrdinal != block.ordinal {
				continue
			}
			cmp, err := figuredBassScoreCompare(r.ctx, r.s, at, want)
			if err != nil {
				r.err = err
				return stateUnknown[NodeRef]("timeline-interrupted"), empty
			}
			if cmp < 0 {
				continue
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
				best, bestFact, tied = candidate, fact, false
				continue
			}
			cmp, err = figuredBassScoreCompare(r.ctx, r.s, at, bestFact.Position.Value)
			if err != nil {
				r.err = err
				return stateUnknown[NodeRef]("timeline-interrupted"), empty
			}
			if cmp < 0 {
				best, bestFact, tied = candidate, fact, false
			} else if cmp == 0 {
				tied = true
			}
		}
	}
	if gap.State != EvidenceAbsent {
		if best == nil || gapPosition.State != EvidenceKnown {
			return gap, empty
		}
		cmp, err := figuredBassScoreCompare(r.ctx, r.s, gapPosition.Value, bestFact.Position.Value)
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

// The block wrapper is retained for isolated provider fixtures. Production calls
// the document-wide finalizer after all divisions declarations and event facts.
func (service *timelineService) resolveFiguredBassOrigins(ctx context.Context, s *Session, b *timelineBindings, block timelineMeasureBlock) error {
	return service.resolveFiguredBassOriginsAll(ctx, s, b, []timelineMeasureBlock{block})
}

func (service *timelineService) resolveFiguredBassOriginsAll(ctx context.Context, s *Session, b *timelineBindings, blocks []timelineMeasureBlock) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	var figureCount int64
	for _, block := range blocks {
		if !r.step(1) {
			return r.err
		}
		for _, event := range block.content.children {
			if r.matches(event, timelineRoleFiguredBass) {
				var err error
				figureCount, err = rationalCheckedAdd(BudgetBufferedBytes, figureCount, 1)
				if err != nil {
					return err
				}
			}
		}
	}
	if r.err != nil {
		return r.err
	}
	if figureCount == 0 {
		return s.step(ctx, 1)
	}
	scratch, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(blocks)), 2048)
	if err != nil {
		return err
	}
	itemBytes, err := rationalCheckedMul(BudgetBufferedBytes, figureCount, 2048)
	if err != nil {
		return err
	}
	scratch, err = rationalCheckedAdd(BudgetBufferedBytes, scratch, itemBytes)
	if err != nil {
		return err
	}
	scratch, err = rationalCheckedAdd(BudgetBufferedBytes, scratch, 8192)
	if err != nil {
		return err
	}
	if _, err := AllocationSize(scratch); err != nil {
		return err
	}
	if err := s.account.Charge(BudgetBufferedBytes, scratch); err != nil {
		return err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, scratch) }()
	pending := make(map[NodeID]figuredBassTimingOrigin)
	anchorCache := make(map[figuredBassAnchorKey]figuredBassAnchorFact)
	unitCache := make(map[figuredBassAnchorKey]Evidence[Decimal])
	durationCache := make(map[figuredBassDurationKey]Evidence[Rational])
	ids := make([]NodeID, 0, int(figureCount))
	domains := make([]figuredBassDomain, 0)
	counts := make(map[timelineDomain]int)
	for _, block := range blocks {
		if !r.step(1) {
			return r.err
		}
		complete := block.complete
		if complete.State == EvidenceKnown && complete.Value {
			complete = r.complete(block.content)
		}
		if block.coverage.State != "" && (block.coverage.State != EvidenceKnown || !block.coverage.Value) {
			if stateAvailabilityRank(block.coverage.State) > stateAvailabilityRank(complete.State) {
				complete = block.coverage
			}
		}
		domains = append(domains, figuredBassDomain{block: block, complete: complete})
		if block.part.State == EvidenceKnown {
			counts[timelineDomain{block.part.Value.Node, block.ordinal}]++
		}
	}
	for i := range domains {
		if !r.step(1) {
			return r.err
		}
		domain := &domains[i]
		if domain.block.part.State == EvidenceKnown && counts[timelineDomain{domain.block.part.Value.Node, domain.block.ordinal}] > 1 {
			domain.complete = stateUnknown[bool]("timeline-duplicate-part-measure-domain")
		}
	}
	if r.err != nil {
		return r.err
	}
	noteIndex, releaseIndex, err := service.buildFiguredBassNoteIndex(r, domains)
	if err != nil {
		return err
	}
	defer releaseIndex()
	for _, domain := range domains {
		block, complete := domain.block, domain.complete
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
			if err := s.retain(4096); err != nil {
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
			if _, err = AllocationSize(bytes); err != nil {
				return err
			}
			out.Figures = make([]NodeRef, len(figures))
			for i, figure := range figures {
				if !r.step(1) {
					return r.err
				}
				out.Figures[i] = NodeRef{s.documentID, figure.header.ID}
			}
			var anchor Evidence[NodeRef]
			var anchorFact timelineEventFact
			if fact.Position.State == EvidenceKnown {
				at := fact.Position.Value
				key := figuredBassAnchorKey{part: at.Part, measure: at.Measure, ordinal: at.MeasureOrdinal, offset: at.Offset.text}
				if !r.step(1 + int64(len(key.offset)) + int64(len(key.part.DocumentID)) + int64(len(key.measure.DocumentID))) {
					return r.err
				}
				cached, found := anchorCache[key]
				if !found {
					var used bool
					cached.anchor, cached.fact, used, err = noteIndex.anchor(ctx, s, service, fact)
					if err != nil {
						return err
					}
					if !used {
						cached.anchor, cached.fact = service.figuredBassAnchor(r, domains, fact, complete)
					}
					if r.err != nil {
						return r.err
					}
					anchorCache[key] = cached
				}
				anchor, anchorFact = cached.anchor, cached.fact
			} else {
				anchor, anchorFact = service.figuredBassAnchor(r, domains, fact, complete)
			}
			out.Anchor = anchor
			if anchor.State == EvidenceKnown {
				out.Position = anchorFact.Position
				out.AnchorStaff = anchorFact.Staff
				out.AnchorVoice = anchorFact.Voice

			} else {
				out.Position = stateUnavailable[NodeRef, musicalPosition](anchor)
				out.AnchorStaff = stateUnavailable[NodeRef, Integer](anchor)
				out.AnchorVoice = stateUnavailable[NodeRef, string](anchor)
				out.AnchorUnits = stateUnavailable[NodeRef, Decimal](anchor)
			}
			if r.err != nil {
				return r.err
			}
			if out.CursorUnits, err = service.figuredBassUnits(ctx, s, out.CursorOrigin, unitCache); err != nil {
				return err
			}
			if out.AnchorUnits, err = service.figuredBassUnits(ctx, s, out.Position, unitCache); err != nil {
				return err
			}
			if err = s.figuredBassDuration(ctx, &out, durationCache); err != nil {
				return err
			}
			if err = s.step(ctx, 1); err != nil {
				return err
			}
			out.AnchorPosition = out.Position
			if _, exists := pending[event.header.ID]; exists {
				return contract("figured bass origins", "duplicate staged publication")
			}
			pending[event.header.ID] = out
			ids = append(ids, event.header.ID)
		}
	}
	if r.err != nil {
		return r.err
	}
	if err := resolveFiguredBassChains(r, domains, pending, ids); err != nil {
		return err
	}
	for _, id := range ids {
		if !r.step(1) {
			return r.err
		}
		out := pending[id]
		if err := ownOrigin(ctx, s, &out); err != nil {
			return err
		}
		pending[id] = out
	}
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if len(service.FiguredBassOrigins) != 0 {
		return contract("figured bass origins", "duplicate finalization")
	}
	service.FiguredBassOrigins = pending

	return r.err
}

func (service *timelineService) figuredBassUnits(ctx context.Context, s *Session, at Evidence[musicalPosition], cache map[figuredBassAnchorKey]Evidence[Decimal]) (Evidence[Decimal], error) {
	if at.State != EvidenceKnown {
		return timelineGapAs[musicalPosition, Decimal](at, "figured-bass-unit-position-unavailable"), nil
	}
	value := at.Value
	key := figuredBassAnchorKey{part: value.Part, measure: value.Measure, ordinal: value.MeasureOrdinal, offset: value.Offset.text}
	if err := s.step(ctx, 1+int64(len(key.offset))+int64(len(key.part.DocumentID))+int64(len(key.measure.DocumentID))); err != nil {
		return Evidence[Decimal]{}, err
	}
	if units, found := cache[key]; found {
		return units, nil
	}
	result, err := service.State.effectiveDivisions(ctx, s, at.Value)
	if err != nil {
		return Evidence[Decimal]{}, err
	}
	cache[key] = result.Value
	return result.Value, nil
}

// Duration is expressed in divisions. Where cursor-time and anchor-time unit
// interpretations agree, normalization is exact without choosing an unstated
// precedence. Disputed or missing unit evidence remains explicit.
func (s *Session) figuredBassDuration(ctx context.Context, out *figuredBassTimingOrigin, cache map[figuredBassDurationKey]Evidence[Rational]) error {
	out.DurationUnits = out.CursorUnits
	if out.CursorUnits.State != EvidenceKnown || out.AnchorUnits.State != EvidenceKnown {
		out.DurationUnits = Evidence[Decimal]{State: EvidenceKnown}
		for _, value := range []Evidence[Decimal]{out.CursorUnits, out.AnchorUnits} {
			if value.State != EvidenceKnown {
				gap := timelineGapAs[Decimal, Decimal](value, "figured-bass-units-unavailable")
				if stateAvailabilityRank(gap.State) > stateAvailabilityRank(out.DurationUnits.State) {
					out.DurationUnits = gap
				}
			}
		}
	} else {
		same, err := s.sameTimelineUnits(ctx, out.CursorUnits, out.AnchorUnits)
		if err != nil {
			return err
		}
		if !same {
			out.DurationUnits = stateUnknown[Decimal]("figured-bass-duration-unit-origin-unresolved")
		}
	}
	out.Duration = Evidence[Rational]{State: EvidenceAbsent}
	out.End = Evidence[musicalPosition]{State: EvidenceAbsent}
	if out.DurationRaw.State == EvidenceAbsent {
		return nil
	}
	var err error
	if out.DurationRaw.State == EvidenceKnown && out.DurationUnits.State == EvidenceKnown && out.DurationRaw.Value.Valid() && out.DurationUnits.Value.Valid() {
		key := figuredBassDurationKey{raw: out.DurationRaw.Value.text, units: out.DurationUnits.Value.text}
		if err := s.step(ctx, 1+int64(len(key.raw))+int64(len(key.units))); err != nil {
			return err
		}
		var found bool
		out.Duration, found = cache[key]
		if !found {
			out.Duration, err = s.normalizeTimelineDuration(ctx, out.DurationRaw, out.DurationUnits)
			if err != nil {
				return err
			}
			cache[key] = out.Duration
		}
	} else {
		out.Duration, err = s.normalizeTimelineDuration(ctx, out.DurationRaw, out.DurationUnits)
		if err != nil {
			return err
		}
	}
	out.End, err = s.noteTimingShift(ctx, out.Position, out.Duration)
	return err
}

// This explicit derived interpretation is supported by the duration clause and
// the maintainers' clarification in MusicXML issue 645. Only the common case of
// a complete consecutive run with one unique anchor is established here. The
// anchor and independent duration survive when chain membership is unproved.
func resolveFiguredBassChains(r *stateReader, domains []figuredBassDomain, pending map[NodeID]figuredBassTimingOrigin, ids []NodeID) error {
	counts := map[NodeID]int{}
	partGaps := map[NodeID]Evidence[NodeRef]{}
	globalGap := Evidence[NodeRef]{State: EvidenceKnown}
	for _, id := range ids {
		if !r.step(1) {
			return r.err
		}
		out := pending[id]
		if out.Anchor.State == EvidenceKnown {
			counts[out.Anchor.Value.Node]++
		} else if out.Part.State == EvidenceKnown {
			gap := partGaps[out.Part.Value.Node]
			if gap.State == "" {
				gap.State = EvidenceKnown
			}
			partGaps[out.Part.Value.Node] = graceStronger(gap, timelineGapAs[NodeRef, NodeRef](out.Anchor, "figured-bass-chain-anchor-unavailable"))
		} else {
			globalGap = graceStronger(globalGap, timelineGapAs[NodeRef, NodeRef](out.Part, "figured-bass-chain-part-unavailable"))
		}
	}
	for _, domain := range domains {
		events := domain.block.content.children
		for at := 0; at < len(events); {
			if !r.step(1) {
				return r.err
			}
			first, found := pending[events[at].header.ID]
			if !found {
				at++
				continue
			}
			end := at + 1
			if first.Anchor.State == EvidenceKnown {
				for end < len(events) {
					if !r.step(1) {
						return r.err
					}
					next, found := pending[events[end].header.ID]
					if !found || next.Anchor.State != EvidenceKnown || next.Anchor.Value != first.Anchor.Value {
						break
					}
					end++
				}
			}
			group := Evidence[NodeRef]{State: EvidenceKnown, Value: first.FiguredBass}
			if first.Anchor.State != EvidenceKnown {
				group = timelineGapAs[NodeRef, NodeRef](first.Anchor, "figured-bass-chain-anchor-unavailable")
			} else if counts[first.Anchor.Value.Node] != end-at {
				group = stateUnknown[NodeRef]("figured-bass-chain-membership-unresolved")
			}
			group = graceStronger(group, globalGap)
			if first.Part.State == EvidenceKnown {
				if gap, ok := partGaps[first.Part.Value.Node]; ok {
					group = graceStronger(group, gap)
				}
			}
			position := first.AnchorPosition
			if group.State != EvidenceKnown {
				position = timelineGapAs[NodeRef, musicalPosition](group, "figured-bass-chain-unavailable")
			}
			start := at
			for ; at < end; at++ {
				if !r.step(1) {
					return r.err
				}
				out := pending[events[at].header.ID]
				out.GroupStart = group
				out.ChainPosition = position
				if group.State == EvidenceKnown {
					out.ChainInterpretation = "consecutive-figured-bass-duration-chain"
				}
				out.ChainEnd = Evidence[musicalPosition]{State: EvidenceAbsent}
				if at == start && group.State == EvidenceKnown {
					out.ChainEnd = out.End
				} else if out.DurationRaw.State != EvidenceAbsent {
					var err error
					out.ChainEnd, err = r.s.noteTimingShift(r.ctx, out.ChainPosition, out.Duration)
					if err != nil {
						return err
					}
				}
				pending[out.FiguredBass.Node] = out
				position = out.ChainEnd
				if position.State == EvidenceAbsent {
					position = stateUnknown[musicalPosition]("figured-bass-chain-duration-unavailable")
				}
			}
		}
	}
	return r.err
}
