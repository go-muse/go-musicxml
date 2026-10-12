package validation

import (
	"context"
	"sort"
)

// These immutable facts identify bases for explicit percentages. They do not
// invent omitted percentages, continuous intervals, or performed redistribution.
type graceDonorTiming struct {
	Note                                    Evidence[NodeRef]
	Raw                                     Evidence[Decimal]
	DonorDuration, Fraction, StolenDuration Evidence[Rational]
}
type graceNeighborTiming struct {
	Note, Grace         NodeRef
	Voice               Evidence[string]
	Previous, Following graceDonorTiming
}

// Closed shapes are guarded in TestGraceNeighborScratchAdmissionCoversClosedShape.
// Each exact note contributes at most one semantic/overlap gap plus one mixed-
// presence gap. Each block contributes sequence, coverage, and one aggregated
// completeness/declaration gap, including blocks without exact note facts.
const (
	timelineGraceScratchPerNote  int64 = 4096
	timelineGraceScratchPerBlock int64 = 4096
	timelineGraceScratchFixed    int64 = 8192
)

type graceStreamKey struct {
	part   NodeRef
	voice  string
	absent bool
}
type graceStream struct {
	key      graceStreamKey
	notes    []NodeID
	floating bool
}
type graceScopedGap struct {
	Note              NodeID
	Value             Evidence[NodeRef]
	Position          Evidence[musicalPosition]
	Voice             Evidence[string]
	From, Through     uint64
	WholePart, Mixing bool
}
type graceDomainNote struct {
	id       NodeID
	ordinal  uint64
	position Evidence[musicalPosition]
}
type gracePartDomain struct {
	seen                                               bool
	last                                               uint64
	notes                                              []graceDomainNote
	gaps                                               []graceScopedGap
	hasAbsent, hasExplicit, voicedGrace, unvoicedGrace bool
}

func graceStronger(a, b Evidence[NodeRef]) Evidence[NodeRef] {
	if stateAvailabilityRank(b.State) > stateAvailabilityRank(a.State) {
		return b
	}
	return a
}
func graceSameMeasure(a, b musicalPosition) bool {
	return a.Part == b.Part && a.Measure == b.Measure && a.MeasureOrdinal == b.MeasureOrdinal
}
func (s *Session) gracePositionOrder(ctx context.Context, a, b musicalPosition) (int, error) {
	if a.Part != b.Part {
		return 0, contract("grace neighbor", "different part positions")
	}
	if err := s.step(ctx, 1); err != nil {
		return 0, err
	}
	if a.MeasureOrdinal < b.MeasureOrdinal {
		return -1, nil
	}
	if a.MeasureOrdinal > b.MeasureOrdinal {
		return 1, nil
	}
	if a.Measure != b.Measure {
		return 0, contract("grace neighbor", "different measures share ordinal")
	}
	return s.rationalCmp(ctx, a.Offset, b.Offset)
}

// A scoped gap can affect a donor only if it can fall in that donor-to-grace
// score interval. Complete hierarchy identity bounds even an unplaced note to
// its measure. Unknown ownership/coverage remains conservatively part-wide.
func graceGapRelevant(r *stateReader, service *timelineService, gap graceScopedGap, key graceStreamKey, lo, hi Evidence[musicalPosition]) (bool, error) {
	if !r.step(1) {
		return false, r.err
	}
	if gap.Mixing {
		if key.absent {
			if gap.Voice.State != EvidenceKnown {
				return false, nil
			}
		} else if gap.Voice.State != EvidenceAbsent {
			return false, nil
		}
	} else if gap.Voice.State == EvidenceKnown && !key.absent {
		if !r.step(1 + int64(len(gap.Voice.Value)) + int64(len(key.voice))) {
			return false, r.err
		}
		if gap.Voice.Value != key.voice {
			return false, nil
		}
	}
	if gap.WholePart {
		return true, nil
	}
	if gap.Position.State == EvidenceKnown && stateValidPosition(gap.Position.Value) && gap.Position.Value.Part == key.part {
		at := gap.Position.Value
		for _, bound := range []struct {
			position Evidence[musicalPosition]
			lower    bool
		}{{lo, true}, {hi, false}} {
			if bound.position.State != EvidenceKnown {
				continue
			}
			edge := bound.position.Value
			if at.MeasureOrdinal == edge.MeasureOrdinal && at.Measure != edge.Measure {
				return true, nil
			}
			order, err := service.State.positionOrder(r.ctx, r.s, at, edge)
			if err != nil {
				return false, err
			}
			if (bound.lower && order == stateBefore) || (!bound.lower && order == stateAfter) {
				return false, nil
			}
		}
		return true, nil
	}
	if gap.Note != 0 {
		if !r.step(1) {
			return false, r.err
		}
		n, found := service.NoteOrigins[gap.Note]
		placement := n.Placement
		if !found {
			placement = service.Facts[gap.Note].Placement
		}
		if placement.Displaced {
			return service.State.associationPlacementRelevant(r, placement, lo, hi)
		}
	}
	if lo.State == EvidenceKnown && gap.Through < lo.Value.MeasureOrdinal {
		member, err := service.State.frameMember(r.ctx, r.s, lo.Value)
		if err != nil {
			return false, err
		}
		if member {
			return false, nil
		}
	}
	if hi.State == EvidenceKnown && gap.From > hi.Value.MeasureOrdinal {
		member, err := service.State.frameMember(r.ctx, r.s, hi.Value)
		if err != nil {
			return false, err
		}
		if member {
			return false, nil
		}
	}
	return true, nil
}
func graceRelationGap(r *stateReader, service *timelineService, part *gracePartDomain, key graceStreamKey, global Evidence[NodeRef], lo, hi Evidence[musicalPosition]) (Evidence[NodeRef], error) {
	out := global
	for _, gap := range part.gaps {
		relevant, err := graceGapRelevant(r, service, gap, key, lo, hi)
		if err != nil {
			return out, err
		}
		if relevant {
			out = graceStronger(out, gap.Value)
		}
	}
	return out, nil
}

// Resolve exact chronological neighbors in independently bounded voice regions.
// Explicit voice labels separate streams across staves. Absent labels are not
// replaced by voice 1; conflicting voice presence remains a local dependency.
func (service *timelineService) resolveGraceNeighbors(ctx context.Context, s *Session, b *timelineBindings, blocks []timelineMeasureBlock) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	var noteCount int64
	hasGrace := false
	for _, block := range blocks {
		if !r.step(1) {
			return r.err
		}
		for _, event := range block.content.children {
			if !r.matches(event, timelineRoleNote) {
				continue
			}
			var err error
			noteCount, err = rationalCheckedAdd(BudgetBufferedBytes, noteCount, 1)
			if err != nil {
				return err
			}
			n := service.NoteOrigins[event.header.ID]
			if n.Grace.State == EvidenceKnown && n.Grace.Value {
				hasGrace = true
			}
		}
	}
	if r.err != nil {
		return r.err
	}
	if !hasGrace {
		return s.step(ctx, 1)
	}
	// This bound covers live old/new slice backing, scoped gap records, stream
	// and part maps, and orphan IDs. None is retained with the published facts.
	scratch, err := rationalCheckedMul(BudgetBufferedBytes, noteCount, timelineGraceScratchPerNote)
	if err != nil {
		return err
	}
	blockBytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(blocks)), timelineGraceScratchPerBlock)
	if err != nil {
		return err
	}
	scratch, err = rationalCheckedAdd(BudgetBufferedBytes, scratch, blockBytes)
	if err != nil {
		return err
	}
	scratch, err = rationalCheckedAdd(BudgetBufferedBytes, scratch, timelineGraceScratchFixed)
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
	streams := []graceStream{}
	indices := map[graceStreamKey]int{}
	sequence := map[NodeID]int{}
	parts := map[NodeRef]*gracePartDomain{}
	orphanIDs := []NodeID{}
	orphanGaps := map[NodeID]Evidence[NodeRef]{}
	global := Evidence[NodeRef]{State: EvidenceKnown}
	for _, block := range blocks {
		if !r.step(1) {
			return r.err
		}
		if block.part.State != EvidenceKnown {
			global = graceStronger(global, timelineGapAs[NodeRef, NodeRef](block.part, "grace-part-unavailable"))
			continue
		}
		owner := block.part.Value
		if !r.step(int64(len(owner.DocumentID))) {
			return r.err
		}
		part := parts[owner]
		if part == nil {
			part = &gracePartDomain{}
			parts[owner] = part
		}
		if part.seen {
			if block.ordinal <= part.last {
				part.gaps = append(part.gaps, graceScopedGap{Value: stateUnknown[NodeRef]("grace-measure-sequence-unresolved"), From: block.ordinal, Through: part.last})
			} else if block.ordinal-part.last > 1 {
				part.gaps = append(part.gaps, graceScopedGap{Value: stateUnknown[NodeRef]("grace-measure-sequence-unresolved"), From: part.last + 1, Through: block.ordinal - 1})
			}
		}
		part.seen, part.last = true, block.ordinal
		if block.coverage.State != "" && (block.coverage.State != EvidenceKnown || !block.coverage.Value) {
			part.gaps = append(part.gaps, graceScopedGap{Value: timelineGapAs[bool, NodeRef](block.coverage, "grace-part-coverage-unavailable"), WholePart: true})
		}
		complete := block.complete
		if complete.State == EvidenceKnown && complete.Value {
			complete = r.complete(block.content)
		}
		blockGap := Evidence[NodeRef]{State: EvidenceKnown}
		if complete.State != EvidenceKnown || !complete.Value {
			blockGap = timelineGapAs[bool, NodeRef](complete, "grace-domain-incomplete")
		}
		for _, event := range block.content.children {
			if !r.matches(event, timelineRoleNote) {
				if event.header.Declaration.State != EvidenceKnown && r.potential(event, timelineRoleNote) {
					// Potential notes were not counted as exact note allocations.
					// Their common canonical measure is one scoped dependency;
					// retain its strongest cause without growing per-event scratch.
					blockGap = graceStronger(blockGap, timelineGapAs[DeclarationID, NodeRef](event.header.Declaration, "grace-note-declaration-unavailable"))
				}
				continue
			}
			id := event.header.ID
			n, found := service.NoteOrigins[id]
			if !found {
				gap := graceScopedGap{Note: id, Value: stateUnknown[NodeRef]("grace-note-origin-unavailable"), From: block.ordinal, Through: block.ordinal}
				if fact, ok := service.Facts[id]; ok {
					gap.Position, gap.Voice = fact.Position, fact.Voice
				}
				part.gaps = append(part.gaps, gap)
				continue
			}
			if n.Voice.State == EvidenceKnown {
				part.hasExplicit = true
			} else if n.Voice.State == EvidenceAbsent {
				part.hasAbsent = true
			}
			isGrace := n.Grace.State == EvidenceKnown && n.Grace.Value
			if isGrace {
				if n.Voice.State == EvidenceKnown {
					part.voicedGrace = true
				} else if n.Voice.State == EvidenceAbsent {
					part.unvoicedGrace = true
				}
			}
			cause := Evidence[NodeRef]{State: EvidenceKnown}
			positioned := n.ScoreOnset.State == EvidenceKnown && stateValidPosition(n.ScoreOnset.Value) && n.ScoreOnset.Value.Part == owner && n.ScoreOnset.Value.Measure == (NodeRef{s.documentID, block.measure.header.ID}) && n.ScoreOnset.Value.MeasureOrdinal == block.ordinal
			at := n.ScoreOnset
			if !positioned {
				cause = timelineGapAs[musicalPosition, NodeRef](n.ScoreOnset, "grace-note-position-unavailable")
				if n.ScoreOnset.State == EvidenceKnown {
					cause = stateUnknown[NodeRef]("grace-note-domain-unresolved")
				}
				at = stateUnavailable[NodeRef, musicalPosition](cause)
			}
			part.notes = append(part.notes, graceDomainNote{id: id, ordinal: block.ordinal, position: at})
			voiceKnown := n.Voice.State == EvidenceKnown || n.Voice.State == EvidenceAbsent
			if !voiceKnown {
				cause = graceStronger(cause, timelineGapAs[string, NodeRef](n.Voice, "grace-voice-unavailable"))
			}
			if n.Grace.State != EvidenceKnown {
				cause = graceStronger(cause, timelineGapAs[bool, NodeRef](n.Grace, "grace-note-kind-unavailable"))
			}
			if !isGrace && n.Chord.State != EvidenceKnown {
				cause = graceStronger(cause, timelineGapAs[bool, NodeRef](n.Chord, "grace-note-kind-unavailable"))
			}
			if cause.State != EvidenceKnown {
				part.gaps = append(part.gaps, graceScopedGap{Note: id, Value: cause, Position: at, Voice: n.Voice, From: block.ordinal, Through: block.ordinal})
			}
			if !positioned || !voiceKnown || n.Grace.State != EvidenceKnown {
				if isGrace {
					orphanIDs = append(orphanIDs, id)
					orphanGaps[id] = cause
				}
				continue
			}
			key := graceStreamKey{part: owner, voice: n.Voice.Value, absent: n.Voice.State == EvidenceAbsent}
			if !r.step(1 + int64(len(key.voice)) + int64(len(owner.DocumentID))) {
				return r.err
			}
			i, exists := indices[key]
			if !exists {
				i = len(streams)
				indices[key] = i
				streams = append(streams, graceStream{key: key})
			}
			member, err := service.State.frameMember(ctx, s, at.Value)
			if err != nil {
				return err
			}
			if !member {
				streams[i].floating = true
			}
			sequence[id] = len(sequence)
			streams[i].notes = append(streams[i].notes, id)
		}
		if blockGap.State != EvidenceKnown {
			part.gaps = append(part.gaps, graceScopedGap{Value: blockGap, From: block.ordinal, Through: block.ordinal})
		}
	}
	if r.err != nil {
		return r.err
	}
	// A voice-presence ambiguity matters only where it could supply an alternate
	// member of the target interval, not everywhere else in the same part.
	for _, part := range parts {
		if !r.step(1) {
			return r.err
		}
		if !part.hasAbsent || !part.hasExplicit {
			continue
		}
		for _, member := range part.notes {
			if !r.step(1) {
				return r.err
			}
			n := service.NoteOrigins[member.id]
			if (n.Voice.State == EvidenceAbsent && part.voicedGrace) || (n.Voice.State == EvidenceKnown && part.unvoicedGrace) {
				gap := graceScopedGap{Note: member.id, Value: stateUnknown[NodeRef]("grace-mixed-voice-identity-unresolved"), Position: member.position, Voice: n.Voice, Mixing: true, From: member.ordinal, Through: member.ordinal}
				part.gaps = append(part.gaps, gap)
			}
		}
	}
	// Score order, not backup/forward serialization, defines the local stream.
	// Explicit original-sequence ties preserve encoded grace runs while ordinary
	// sorting keeps both comparisons and the post-cancellation drain O(n log n).
	for i := range streams {
		stream := &streams[i]
		levels := int64(1)
		for n := len(stream.notes); n > 1; n /= 2 {
			levels++
		}
		work, err := rationalCheckedMul(BudgetRuleSteps, int64(len(stream.notes)), levels)
		if err != nil {
			return err
		}
		work, err = rationalCheckedMul(BudgetRuleSteps, work, 32)
		if err != nil {
			return err
		}
		if !r.step(work) {
			return r.err
		}
		var interrupted error
		sort.Slice(stream.notes, func(i, j int) bool {
			if interrupted != nil {
				return false
			}
			a, z := service.NoteOrigins[stream.notes[i]].ScoreOnset.Value, service.NoteOrigins[stream.notes[j]].ScoreOnset.Value
			// Duplicate canonical ordinals already have a scoped gap. Give those
			// unavailable domains a deterministic internal order without treating
			// their different measure identities as chronological evidence.
			if a.MeasureOrdinal == z.MeasureOrdinal && a.Measure != z.Measure {
				interrupted = s.step(ctx, 1)
				return interrupted == nil && a.Measure.Node < z.Measure.Node
			}
			var cmp int
			cmp, interrupted = s.gracePositionOrder(ctx, a, z)
			if interrupted != nil {
				return false
			}
			if cmp == 0 {
				return sequence[stream.notes[i]] < sequence[stream.notes[j]]
			}
			return cmp < 0
		})
		if interrupted != nil {
			return interrupted
		}
		var lastOrdinary NodeID
		for _, id := range stream.notes {
			if !r.step(1) {
				return r.err
			}
			n := service.NoteOrigins[id]
			if n.Grace.Value || n.Chord.State != EvidenceKnown || n.Chord.Value {
				continue
			}
			if lastOrdinary != 0 {
				prior := service.NoteOrigins[lastOrdinary]
				if prior.ScoreEnd.State == EvidenceKnown && graceSameMeasure(prior.ScoreEnd.Value, n.ScoreOnset.Value) {
					cmp, err := s.rationalCmp(ctx, prior.ScoreEnd.Value.Offset, n.ScoreOnset.Value.Offset)
					if err != nil {
						return err
					}
					if cmp > 0 {
						parts[stream.key.part].gaps = append(parts[stream.key.part].gaps, graceScopedGap{Value: stateUnknown[NodeRef]("grace-overlapping-voice-events"), Position: n.ScoreOnset, Voice: n.Voice, From: n.ScoreOnset.Value.MeasureOrdinal, Through: n.ScoreOnset.Value.MeasureOrdinal})
					}
				}
			}
			lastOrdinary = id
		}
	}
	pending := map[NodeID]graceNeighborTiming{}
	publish := func(n noteTimingOrigin, stream *graceStream, previous, following int, previousGap, followingGap Evidence[NodeRef]) error {
		if _, exists := service.GraceNeighbors[n.Note.Node]; exists {
			return contract("grace neighbors", "duplicate publication")
		}
		if err := s.account.Charge(BudgetContextFacts, 1); err != nil {
			return err
		}
		if err := s.retain(4096); err != nil {
			return err
		}
		local := Evidence[NodeRef]{State: EvidenceKnown}
		if n.GraceTiming.State != EvidenceKnown {
			local = graceStronger(local, timelineGapAs[graceTimingOrigin, NodeRef](n.GraceTiming, "grace-timing-unavailable"))
		}
		if n.Chord.State != EvidenceKnown {
			local = graceStronger(local, timelineGapAs[bool, NodeRef](n.Chord, "grace-note-kind-unavailable"))
		} else if n.Chord.Value {
			local = graceStronger(local, stateUnknown[NodeRef]("grace-chord-donor-unresolved"))
		}
		out := graceNeighborTiming{Note: n.Note, Grace: n.GraceTiming.Value.Grace, Voice: n.Voice}
		var err error
		out.Previous, err = service.graceDonor(ctx, s, stream, n, previous, true, n.GraceTiming.Value.StealPrevious, graceStronger(previousGap, local))
		if err != nil {
			return err
		}
		out.Following, err = service.graceDonor(ctx, s, stream, n, following, false, n.GraceTiming.Value.StealFollowing, graceStronger(followingGap, local))
		if err != nil {
			return err
		}
		if err := ownOrigin(ctx, s, &out); err != nil {
			return err
		}
		pending[n.Note.Node] = out
		return nil
	}
	for i := range streams {
		stream := &streams[i]
		part := parts[stream.key.part]
		previous := -1
		for at := 0; at < len(stream.notes); {
			if !r.step(1) {
				return r.err
			}
			n := service.NoteOrigins[stream.notes[at]]
			if !n.Grace.Value {
				previous = at
				at++
				continue
			}
			end := at + 1
			for end < len(stream.notes) {
				if !r.step(1) {
					return r.err
				}
				if !service.NoteOrigins[stream.notes[end]].Grace.Value {
					break
				}
				end++
			}
			following := end
			if following == len(stream.notes) {
				following = -1
			}
			cached := false
			var cachedAt musicalPosition
			var previousGap, followingGap Evidence[NodeRef]
			chosenPrevious, chosenFollowing := previous, following
			for ; at < end; at++ {
				if !r.step(1) {
					return r.err
				}
				n = service.NoteOrigins[stream.notes[at]]
				position := n.ScoreOnset.Value
				if !r.step(int64(len(position.Offset.text))) {
					return r.err
				}
				same := cached && graceSameMeasure(position, cachedAt) && position.Offset.text == cachedAt.Offset.text && position.UnprovedFrame == cachedAt.UnprovedFrame
				if !same {
					chosenPrevious, chosenFollowing = previous, following
					previousProof, followingProof := Evidence[NodeRef]{State: EvidenceKnown}, Evidence[NodeRef]{State: EvidenceKnown}
					if stream.floating {
						chosenPrevious, previousProof, err = service.graceChronologicalDonor(r, stream, at, true)
						if err != nil {
							return err
						}
						chosenFollowing, followingProof, err = service.graceChronologicalDonor(r, stream, at, false)
						if err != nil {
							return err
						}
					}
					lo, hi := Evidence[musicalPosition]{State: EvidenceAbsent}, Evidence[musicalPosition]{State: EvidenceAbsent}
					if chosenPrevious >= 0 {
						lo = service.NoteOrigins[stream.notes[chosenPrevious]].ScoreOnset
					}
					if chosenFollowing >= 0 {
						hi = service.NoteOrigins[stream.notes[chosenFollowing]].ScoreOnset
					}
					previousGap, err = graceRelationGap(r, service, part, stream.key, global, lo, n.ScoreOnset)
					if err != nil {
						return err
					}
					followingGap, err = graceRelationGap(r, service, part, stream.key, global, n.ScoreOnset, hi)
					if err != nil {
						return err
					}
					previousGap = graceStronger(previousGap, previousProof)
					followingGap = graceStronger(followingGap, followingProof)
					cached, cachedAt = true, position
				}
				if err := publish(n, stream, chosenPrevious, chosenFollowing, previousGap, followingGap); err != nil {
					return err
				}
			}
		}
	}
	for _, id := range orphanIDs {
		if !r.step(1) {
			return r.err
		}
		gap := graceStronger(global, orphanGaps[id])
		if err := publish(service.NoteOrigins[id], &graceStream{}, -1, -1, gap, gap); err != nil {
			return err
		}
	}
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if len(service.GraceNeighbors) != 0 {
		return contract("grace neighbors", "duplicate finalization")
	}
	service.GraceNeighbors = pending
	return nil
}

// The nominal stream remains a bounded index. A displaced member switches
// only that stream's donor proof to exact partial chronology. Candidate and
// competitor passes allocate no per-query scratch and retain separate sides.
func (service *timelineService) graceChronologicalDonor(r *stateReader, stream *graceStream, target int, previous bool) (int, Evidence[NodeRef], error) {
	n := service.NoteOrigins[stream.notes[target]]
	best := -1
	gap := Evidence[NodeRef]{State: EvidenceKnown}
	for pass := 0; pass < 2; pass++ {
		for i, id := range stream.notes {
			if !r.step(1) {
				return -1, gap, r.err
			}
			candidate := service.NoteOrigins[id]
			if candidate.Grace.State != EvidenceKnown || candidate.Grace.Value || candidate.Chord.State != EvidenceKnown {
				continue
			}
			order, err := service.State.positionOrder(r.ctx, r.s, candidate.ScoreOnset.Value, n.ScoreOnset.Value)
			if err != nil {
				return -1, gap, err
			}
			// Same-frame equality retains the encoded ordering of grace runs. No
			// such source-order tie break is permitted for different measure frames.
			if order == stateEqual && graceSameMeasure(candidate.ScoreOnset.Value, n.ScoreOnset.Value) {
				if i < target {
					order = stateBefore
				} else {
					order = stateAfter
				}
			}
			eligible, impossible := stateAfter, stateBefore
			if previous {
				eligible, impossible = stateBefore, stateAfter
			}
			if order == impossible {
				continue
			}
			if pass == 0 {
				if order != eligible {
					continue
				}
				if best >= 0 {
					relation, err := service.State.positionOrder(r.ctx, r.s, candidate.ScoreOnset.Value, service.NoteOrigins[stream.notes[best]].ScoreOnset.Value)
					if err != nil {
						return -1, gap, err
					}
					if relation != impossible {
						continue
					}
				}
				best = i
				continue
			}
			if i == best {
				continue
			}
			if best >= 0 {
				relation, err := service.State.positionOrder(r.ctx, r.s, candidate.ScoreOnset.Value, service.NoteOrigins[stream.notes[best]].ScoreOnset.Value)
				if err != nil {
					return -1, gap, err
				}
				if relation == eligible {
					continue
				}
			}
			gap = stateUnknown[NodeRef]("grace-cross-frame-chronology-unresolved")
		}
	}
	return best, gap, nil
}

func (service *timelineService) graceDonor(ctx context.Context, s *Session, stream *graceStream, n noteTimingOrigin, index int, previous bool, raw Evidence[Decimal], gap Evidence[NodeRef]) (graceDonorTiming, error) {
	out := graceDonorTiming{Note: gap, Raw: raw, DonorDuration: stateUnknown[Rational]("grace-donor-duration-unavailable"), Fraction: Evidence[Rational]{State: EvidenceAbsent}, StolenDuration: Evidence[Rational]{State: EvidenceAbsent}}
	if err := s.step(ctx, 1); err != nil {
		return out, err
	}
	if index < 0 {
		out.Note = graceStronger(out.Note, stateUnknown[NodeRef]("grace-neighbor-outside-proven-domain"))
	} else {
		donor := service.NoteOrigins[stream.notes[index]]
		out.DonorDuration = donor.Duration
		if donor.Grace.State != EvidenceKnown || donor.Grace.Value || donor.Chord.State != EvidenceKnown || donor.Chord.Value {
			out.Note = graceStronger(out.Note, stateUnknown[NodeRef]("grace-ordinary-donor-unresolved"))
		}
		if index+1 < len(stream.notes) {
			next := service.NoteOrigins[stream.notes[index+1]]
			if next.Grace.State == EvidenceKnown && !next.Grace.Value && next.Chord.State == EvidenceKnown && next.Chord.Value {
				out.Note = graceStronger(out.Note, stateUnknown[NodeRef]("grace-chord-donor-unresolved"))
			}
		}
		if out.Note.State == EvidenceKnown {
			out.Note.Value = donor.Note
		}
	}
	if raw.State == EvidenceAbsent {
		return out, nil
	}
	fraction, err := s.normalizeTimelineDuration(ctx, raw, Evidence[Decimal]{State: EvidenceKnown, Value: noteTimingHundred})
	if err != nil {
		return out, err
	}
	out.Fraction = fraction
	amountGap := Evidence[Rational]{State: EvidenceKnown}

	if out.Note.State != EvidenceKnown {
		amountGap = timelineGapAs[NodeRef, Rational](out.Note, "grace-donor-unavailable")
	}
	for _, dependency := range []Evidence[Rational]{out.DonorDuration, fraction} {
		if dependency.State != EvidenceKnown {
			candidate := timelineGapAs[Rational, Rational](dependency, "grace-timing-input-unavailable")
			if stateAvailabilityRank(candidate.State) > stateAvailabilityRank(amountGap.State) {
				amountGap = candidate
			}
		}
	}
	if amountGap.State != EvidenceKnown {
		out.StolenDuration = amountGap
		return out, nil
	}
	value, err := s.rationalMul(ctx, out.DonorDuration.Value, fraction.Value)
	if err != nil {
		return out, err
	}
	out.StolenDuration = Evidence[Rational]{State: EvidenceKnown, Value: value}
	return out, nil
}
