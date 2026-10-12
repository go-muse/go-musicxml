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
	unprovedFrame bool
	part, measure NodeRef
	ordinal       uint64
	offset        string
}

// Indirect retained record plus at least 512 bytes of map growth headroom.
// The finalizer's separately admitted fixed scratch covers temporary records.
const timelineFigureReserveBytes int64 = 2560
const timelineFigureCacheEntries = 64
const timelineFigureCacheReserveBytes int64 = 131072
const timelineFigureVariableScratchBytes int64 = 384

// Cache storage is bounded independently of document size. Clear reuses the
// admitted map tables, and its traversal and cancellation are still metered.
func timelineFigureCachePut[K comparable, V any](ctx context.Context, s *Session, cache map[K]V, key K, value V) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if _, present := cache[key]; !present && len(cache) >= timelineFigureCacheEntries {
		if err := s.step(ctx, int64(len(cache))); err != nil {
			return err
		}
		clear(cache)
		if err := s.step(ctx, 1); err != nil {
			return err
		}
	}
	cache[key] = value
	return nil
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
	q := timelineFollowingSearch{state: &service.State, want: origin.Position.Value, best: Evidence[NodeRef]{State: EvidenceAbsent}, gap: Evidence[NodeRef]{State: EvidenceKnown}, chronology: "figured-bass-cross-frame-chronology-unresolved", tied: "figured-bass-simultaneous-notes-unresolved"}
	for q.pass = 0; q.pass < 2; q.pass++ {
		for _, domain := range domains {
			if !r.step(1) {
				return stateUnknown[NodeRef]("timeline-interrupted"), empty
			}
			block := domain.block
			addGap := func(value Evidence[NodeRef], position Evidence[musicalPosition]) {
				q.add(r, value, position, block.ordinal)
			}
			if block.part.State != EvidenceKnown {
				addGap(timelineGapAs[NodeRef, NodeRef](block.part, "figured-bass-part-unavailable"), Evidence[musicalPosition]{State: EvidenceUnknown})
				continue
			}
			if !r.sameRef(block.part.Value, origin.Position.Value.Part) {
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
					if fact.Part.State == EvidenceKnown && !r.sameRef(fact.Part.Value, origin.Position.Value.Part) {
						continue
					}
					q.addPlaced(r, timelineGapAs[musicalPosition, NodeRef](fact.Position, "figured-bass-note-position-unavailable"), fact.Position, block.ordinal, fact.Placement)
					continue
				}
				at, want := fact.Position.Value, origin.Position.Value
				if !r.sameRef(at.Part, want.Part) {
					continue
				}
				if at.Measure.Node != block.measure.header.ID || at.MeasureOrdinal != block.ordinal {
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
				q.candidate(r, Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, candidate.header.ID}}, fact)
			}
		}
	}
	if q.gap.State != EvidenceKnown {
		return q.gap, empty
	}
	if q.best.State != EvidenceKnown {
		return stateUnknown[NodeRef]("figured-bass-following-note-unresolved"), empty
	}
	return q.best, q.fact

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
	itemBytes, err := rationalCheckedMul(BudgetBufferedBytes, figureCount, timelineFigureVariableScratchBytes)
	if err != nil {
		return err
	}
	scratch, err = rationalCheckedAdd(BudgetBufferedBytes, scratch, itemBytes)
	if err != nil {
		return err
	}
	scratch, err = rationalCheckedAdd(BudgetBufferedBytes, scratch, 8192+timelineFigureCacheReserveBytes)
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
	anchorCache := make(map[figuredBassAnchorKey]Evidence[NodeRef])
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
			if err := s.retain(timelineFigureReserveBytes); err != nil {
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
				key := figuredBassAnchorKey{part: at.Part, measure: at.Measure, ordinal: at.MeasureOrdinal, offset: at.Offset.text, unprovedFrame: at.UnprovedFrame}
				if !r.step(1 + int64(len(key.offset)) + int64(len(key.part.DocumentID)) + int64(len(key.measure.DocumentID))) {
					return r.err
				}
				cached, found := anchorCache[key]
				if !found {
					var used bool
					cached, _, used, err = noteIndex.anchor(ctx, s, service, fact)
					if err != nil {
						return err
					}
					if !used {
						cached, _ = service.figuredBassAnchor(r, domains, fact, complete)
					}
					if r.err != nil {
						return r.err
					}
					if err := timelineFigureCachePut(ctx, s, anchorCache, key, cached); err != nil {
						return err
					}
				}
				anchor = cached
				if anchor.State == EvidenceKnown {
					anchorFact = service.Facts[anchor.Value.Node]
				}
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
	if err := service.State.resolveFiguredBassChains(r, domains, pending, ids); err != nil {
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
	key := figuredBassAnchorKey{part: value.Part, measure: value.Measure, ordinal: value.MeasureOrdinal, offset: value.Offset.text, unprovedFrame: value.UnprovedFrame}
	if err := s.step(ctx, 1+int64(len(key.offset))+int64(len(key.part.DocumentID))+int64(len(key.measure.DocumentID))); err != nil {
		return Evidence[Decimal]{}, err
	}
	if units, found := cache[key]; found {
		return units, nil
	}
	result, err := service.State.effectiveDivisionsEvidence(ctx, s, at.Value)
	if err != nil {
		return Evidence[Decimal]{}, err
	}
	if err := timelineFigureCachePut(ctx, s, cache, key, result); err != nil {
		return Evidence[Decimal]{}, err
	}
	return result, nil
}

// Figured bass takes its score position from its regular-note anchor. Duration
// uses the divisions effective at that documented position; the serialization
// cursor's units remain provenance and are not a competing unit origin.
func (s *Session) figuredBassDuration(ctx context.Context, out *figuredBassTimingOrigin, cache map[figuredBassDurationKey]Evidence[Rational]) error {
	out.DurationUnits = out.AnchorUnits
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
			if err := timelineFigureCachePut(ctx, s, cache, key, out.Duration); err != nil {
				return err
			}
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
	return (&timelineState{}).resolveFiguredBassChains(r, domains, pending, ids)
}
func (state *timelineState) resolveFiguredBassChains(r *stateReader, domains []figuredBassDomain, pending map[NodeID]figuredBassTimingOrigin, ids []NodeID) error {
	counts := map[NodeID]int{}
	// Unknown anchors retain IDs only: on 64-bit hosts each map entry is
	// NodeRef24 + slice24, conservatively 4*48 bytes, plus 32 bytes per ID
	// for append backing overlap and 8 for the finalizer's exact ID list.
	// The resulting 232 bytes/figure fits the chain scratch allowance; the
	// known-anchor count entry is disjoint and narrower. Positions stay shared.
	partGaps := map[NodeRef][]NodeID{}
	globalGap := Evidence[NodeRef]{State: EvidenceKnown}
	for _, id := range ids {
		if !r.step(1) {
			return r.err
		}
		out := pending[id]
		if out.Anchor.State == EvidenceKnown {
			counts[out.Anchor.Value.Node]++
		} else if out.Part.State == EvidenceKnown {
			if !r.step(int64(len(out.Part.Value.DocumentID))) {
				return r.err
			}
			// Only IDs are indexed here; positions and causes remain in the
			// already admitted private staged record. Append peak is covered
			// by the finalizer's per-figure scratch reservation.
			partGaps[out.Part.Value] = append(partGaps[out.Part.Value], id)
		} else {
			globalGap = graceStronger(globalGap, timelineGapAs[NodeRef, NodeRef](out.Part, "figured-bass-chain-part-unavailable"))
			globalGap = graceStronger(globalGap, timelineGapAs[NodeRef, NodeRef](out.Anchor, "figured-bass-chain-anchor-unavailable"))
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
				if !r.step(int64(len(first.Part.Value.DocumentID))) {
					return r.err
				}
				for _, id := range partGaps[first.Part.Value] {
					if !r.step(1) {
						return r.err
					}
					other := pending[id]
					relevant, err := state.figuredBassChainGapRelevant(r, other, first.AnchorPosition)
					if err != nil {
						return err
					}
					if relevant {
						group = graceStronger(group, timelineGapAs[NodeRef, NodeRef](other.Anchor, "figured-bass-chain-anchor-unavailable"))
					}
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

// The first following regular note cannot precede a figure's proved cursor
// position. Thus a strictly later gap cannot share an earlier known anchor,
// regardless of XML serialization order. An unplaced or incoherently owned gap
// still may be relevant and is never discarded by a lexical guess.
func (state *timelineState) figuredBassChainGapRelevant(r *stateReader, gap figuredBassTimingOrigin, anchor Evidence[musicalPosition]) (bool, error) {
	if !r.step(1) {
		return false, r.err
	}
	if gap.Part.State != EvidenceKnown || gap.CursorOrigin.State != EvidenceKnown || anchor.State != EvidenceKnown {
		return true, r.err
	}
	at, target := gap.CursorOrigin.Value, anchor.Value
	if !r.validPosition(at) || !r.validPosition(target) || !r.sameRef(at.Part, gap.Part.Value) {
		return true, r.err
	}
	if !r.sameRef(at.Part, target.Part) {
		return false, r.err
	}
	if at.MeasureOrdinal == target.MeasureOrdinal && !r.sameRef(at.Measure, target.Measure) {
		return true, r.err
	}
	order, err := state.positionOrder(r.ctx, r.s, at, target)
	return order != stateAfter, err
}
