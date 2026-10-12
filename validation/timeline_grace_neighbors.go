package validation

import "context"

// These immutable provider facts identify the base of each explicit percentage.
// They neither invent an omitted percentage nor redistribute performed time.
// A group shares its proven ordinary neighbors; each grace keeps its own amount.
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
type graceStreamKey struct {
	part   NodeID
	voice  string
	absent bool
}
type graceStream struct {
	key   graceStreamKey
	notes []NodeID
	gap   Evidence[NodeRef]
}
type gracePartDomain struct {
	gap                    Evidence[NodeRef]
	absent, explicit, seen bool
	last                   uint64
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

// resolveGraceNeighbors runs after all note facts are complete. Explicit voices
// define separate linear sequences across staves. An absent voice is accepted
// only when all notes of the complete part have absent voice and the resulting
// sequence is proven linear; no synthetic voice identity is installed.
//
// Score coordinates corroborate sequence order. Consecutive grace notes are
// skipped when finding ordinary neighbors, rather than treated as zero-duration
// donors. These are chronological identities, even across a forward gap or a
// complete measure boundary; no continuous performed-time interval is asserted.
func (service *timelineService) resolveGraceNeighbors(ctx context.Context, s *Session, b *timelineBindings, blocks []timelineMeasureBlock) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	// First count without allocating: ordinary scores need no neighbor index.
	hasGrace := false
	var noteCount int64
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
	// Conservative live-peak bound includes old+new append backing (at least
	// 32 bytes per NodeID), stream/map buckets, part domains and fixed scratch.
	// Scratch is not retained with the resulting immutable provider records.
	scratch, err := rationalCheckedMul(BudgetBufferedBytes, noteCount, 1024)
	if err != nil {
		return err
	}
	blockBytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(blocks)), 1024)
	if err != nil {
		return err
	}
	scratch, err = rationalCheckedAdd(BudgetBufferedBytes, scratch, blockBytes)
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
	streams := []graceStream{}
	indices := map[graceStreamKey]int{}
	parts := map[NodeID]*gracePartDomain{}
	global := Evidence[NodeRef]{State: EvidenceKnown}
	for _, block := range blocks {
		if !r.step(1) {
			return r.err
		}
		if block.part.State != EvidenceKnown {
			global = graceStronger(global, timelineGapAs[NodeRef, NodeRef](block.part, "grace-part-unavailable"))
			continue
		}
		part := parts[block.part.Value.Node]
		if part == nil {
			part = &gracePartDomain{gap: Evidence[NodeRef]{State: EvidenceKnown}}
			parts[block.part.Value.Node] = part
		}
		if part.seen && (block.ordinal <= part.last || block.ordinal-part.last != 1) {
			part.gap = graceStronger(part.gap, stateUnknown[NodeRef]("grace-measure-sequence-unresolved"))
		}
		part.seen, part.last = true, block.ordinal
		complete := block.complete
		if complete.State == EvidenceKnown && complete.Value {
			complete = r.complete(block.content)
		}
		if block.coverage.State != "" && (block.coverage.State != EvidenceKnown || !block.coverage.Value) {
			part.gap = graceStronger(part.gap, timelineGapAs[bool, NodeRef](block.coverage, "grace-part-coverage-unavailable"))
		}
		if complete.State != EvidenceKnown || !complete.Value {
			part.gap = graceStronger(part.gap, timelineGapAs[bool, NodeRef](complete, "grace-domain-incomplete"))
		}
		for _, event := range block.content.children {
			if !r.matches(event, timelineRoleNote) {
				if event.header.Declaration.State != EvidenceKnown && r.potential(event, timelineRoleNote) {
					part.gap = graceStronger(part.gap, timelineGapAs[DeclarationID, NodeRef](event.header.Declaration, "grace-note-declaration-unavailable"))
				}
				continue
			}
			n, found := service.NoteOrigins[event.header.ID]
			if !found {
				part.gap = graceStronger(part.gap, stateUnknown[NodeRef]("grace-note-origin-unavailable"))
				continue
			}
			if n.Voice.State != EvidenceKnown && n.Voice.State != EvidenceAbsent {
				part.gap = graceStronger(part.gap, timelineGapAs[string, NodeRef](n.Voice, "grace-voice-unavailable"))
			}
			key := graceStreamKey{part: block.part.Value.Node, voice: n.Voice.Value, absent: n.Voice.State != EvidenceKnown}
			if n.Voice.State == EvidenceKnown {
				part.explicit = true
			} else {
				part.absent = true
			}
			if !r.step(1 + int64(len(key.voice))) {
				return r.err
			}
			i, found := indices[key]
			if !found {
				i = len(streams)
				indices[key] = i
				streams = append(streams, graceStream{key: key, gap: Evidence[NodeRef]{State: EvidenceKnown}})
			}
			stream := &streams[i]
			if n.ScoreOnset.State != EvidenceKnown {
				stream.gap = graceStronger(stream.gap, timelineGapAs[musicalPosition, NodeRef](n.ScoreOnset, "grace-note-position-unavailable"))
			} else if n.ScoreOnset.Value.Part != block.part.Value || n.ScoreOnset.Value.Measure.Node != block.measure.header.ID || n.ScoreOnset.Value.MeasureOrdinal != block.ordinal {
				stream.gap = graceStronger(stream.gap, stateUnknown[NodeRef]("grace-note-domain-unresolved"))
			}
			for _, marker := range []Evidence[bool]{n.Grace, n.Chord} {
				if marker.State != EvidenceKnown {
					stream.gap = graceStronger(stream.gap, timelineGapAs[bool, NodeRef](marker, "grace-note-kind-unavailable"))
				}
			}
			stream.notes = append(stream.notes, event.header.ID)
		}
	}
	if r.err != nil {
		return r.err
	}
	pending := map[NodeID]graceNeighborTiming{}
	for i := range streams {
		stream := &streams[i]
		part := parts[stream.key.part]
		gap := graceStronger(graceStronger(global, part.gap), stream.gap)
		if part.absent && part.explicit {
			gap = graceStronger(gap, stateUnknown[NodeRef]("grace-mixed-voice-identity-unresolved"))
		}
		// Establish that the serialized voice sequence agrees with score order,
		// and that ordinary events do not compete at the same position.
		var last, lastOrdinary *noteTimingOrigin
		for _, id := range stream.notes {
			if !r.step(1) {
				return r.err
			}
			n := service.NoteOrigins[id]
			if n.ScoreOnset.State == EvidenceKnown && last != nil && last.ScoreOnset.State == EvidenceKnown {
				if n.ScoreOnset.Value.MeasureOrdinal == last.ScoreOnset.Value.MeasureOrdinal && n.ScoreOnset.Value.Measure != last.ScoreOnset.Value.Measure {
					gap = graceStronger(gap, stateUnknown[NodeRef]("grace-measure-identity-unresolved"))
				} else {
					cmp, err := s.gracePositionOrder(ctx, last.ScoreOnset.Value, n.ScoreOnset.Value)
					if err != nil {
						return err
					}
					if cmp > 0 {
						gap = graceStronger(gap, stateUnknown[NodeRef]("grace-voice-score-order-unresolved"))
					}
				}
			}
			if n.Grace.State == EvidenceKnown && !n.Grace.Value && n.Chord.State == EvidenceKnown && !n.Chord.Value {
				if lastOrdinary != nil && lastOrdinary.ScoreEnd.State == EvidenceKnown && n.ScoreOnset.State == EvidenceKnown && graceSameMeasure(lastOrdinary.ScoreEnd.Value, n.ScoreOnset.Value) {
					cmp, err := s.rationalCmp(ctx, lastOrdinary.ScoreEnd.Value.Offset, n.ScoreOnset.Value.Offset)
					if err != nil {
						return err
					}
					if cmp > 0 {
						gap = graceStronger(gap, stateUnknown[NodeRef]("grace-overlapping-voice-events"))
					}
				}
				copy := n
				lastOrdinary = &copy
			}
			copy := n
			last = &copy
		}
		previous := -1
		for at := 0; at < len(stream.notes); {
			if !r.step(1) {
				return r.err
			}
			n := service.NoteOrigins[stream.notes[at]]
			if n.Grace.State != EvidenceKnown || !n.Grace.Value {
				previous = at
				at++
				continue
			}
			end := at + 1
			for end < len(stream.notes) {
				if !r.step(1) {
					return r.err
				}
				next := service.NoteOrigins[stream.notes[end]]
				if next.Grace.State != EvidenceKnown || !next.Grace.Value {
					break
				}
				end++
			}
			following := end
			if following == len(stream.notes) {
				following = -1
			}
			for ; at < end; at++ {
				n = service.NoteOrigins[stream.notes[at]]
				if _, exists := service.GraceNeighbors[n.Note.Node]; exists {
					return contract("grace neighbors", "duplicate publication")
				}
				if err := s.account.Charge(BudgetContextFacts, 1); err != nil {
					return err
				}
				if err := s.retain(4096); err != nil {
					return err
				}
				out := graceNeighborTiming{Note: n.Note, Voice: n.Voice}
				localGap := gap
				if n.GraceTiming.State != EvidenceKnown {
					localGap = graceStronger(localGap, timelineGapAs[graceTimingOrigin, NodeRef](n.GraceTiming, "grace-timing-unavailable"))
				}
				if n.Chord.State == EvidenceKnown && n.Chord.Value {
					localGap = graceStronger(localGap, stateUnknown[NodeRef]("grace-chord-donor-unresolved"))
				}
				out.Grace = n.GraceTiming.Value.Grace
				var err error
				out.Previous, err = service.graceDonor(ctx, s, stream, n, previous, true, n.GraceTiming.Value.StealPrevious, localGap)
				if err != nil {
					return err
				}
				out.Following, err = service.graceDonor(ctx, s, stream, n, following, false, n.GraceTiming.Value.StealFollowing, localGap)
				if err != nil {
					return err
				}
				if err := ownOrigin(ctx, s, &out); err != nil {
					return err
				}
				pending[n.Note.Node] = out
			}
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
