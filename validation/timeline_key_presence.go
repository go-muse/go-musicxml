package validation

import "context"

const (
	timelineKeyPresenceCertificateBytes  int64 = 512
	timelineKeyPresenceGapScratchBytes   int64 = 512
	timelineKeyPresenceScratchFixedBytes int64 = 256
)

// A certificate describes valid key encoding only. It does not position an
// update, choose a concrete key, or repair an invalid/hidden publisher. The map
// is sparse in actual unplaced keys and indexed by their immutable gap row.
func (h *timelineState) certifyKeyPresenceGap(ctx context.Context, s *Session, index int, node NodeRef) error {
	if index < 0 || index >= len(h.Keys.Gaps) || node.Node == 0 {
		return contract("key presence", "matching key gap and source required")
	}
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if _, ok := h.keyPresenceGaps[index]; ok {
		return contract("key presence", "duplicate certificate")
	}
	provenance := stateProvenance{Node: node, Kind: stateExplicit}
	if err := s.account.Charge(BudgetContextFacts, 1); err != nil {
		return err
	}
	// Map header and old/new bucket coexistence, separately from the owned source.
	if err := s.retain(timelineKeyPresenceCertificateBytes); err != nil {
		return err
	}
	if err := stateRetainCopy(ctx, s, provenance); err != nil {
		return err
	}
	if h.keyPresenceGaps == nil {
		h.keyPresenceGaps = make(map[int]stateProvenance)
	}
	h.keyPresenceGaps[index] = stateClone(provenance)
	return nil
}

func (h *timelineState) queryKeyPresence(ctx context.Context, s *Session, pos musicalPosition, staff Evidence[Integer]) (stateResult[bool], error) {
	if staff.State == EvidenceAbsent || staff.State == EvidenceUnknown && staff.Reason == "timeline-staff-unresolved" && staff.Cause == nil {
		return h.queryStafflessKeyPresence(ctx, s, pos)
	}
	return h.queryKeyPresenceScope(ctx, s, pos, staff, false)
}

func (h *timelineState) queryKeyPresenceScope(ctx context.Context, s *Session, pos musicalPosition, staff Evidence[Integer], allStavesOnly bool) (stateResult[bool], error) {
	project := func(v keyState) (Evidence[bool], error) { return v.encodingEvidenceMetered(ctx, s) }
	query := func(history *stateHistory[keyState]) (stateResult[bool], error) {
		return queryStateAtScopeWithProject(ctx, s, h, history, pos, staff, project, true, true, allStavesOnly, nil)
	}
	original, err := query(&h.Keys)
	if err != nil || len(h.keyPresenceGaps) == 0 || !allStavesOnly && staff.State != EvidenceKnown || original.Value.State == EvidenceKnown && original.Value.Value {
		return original, err
	}
	// Remove only certified value-preserving gaps for an independent baseline
	// proof. Missing baseline and every other causal gap remain in the query.
	count := int64(len(h.Keys.Gaps))
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, count, timelineKeyPresenceGapScratchBytes)
	if err != nil {
		return stateResult[bool]{}, err
	}
	bytes, err = rationalCheckedAdd(BudgetBufferedBytes, bytes, timelineKeyPresenceScratchFixedBytes)
	if err != nil {
		return stateResult[bool]{}, err
	}
	if _, err = AllocationSize(bytes); err != nil {
		return stateResult[bool]{}, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return stateResult[bool]{}, err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }()
	index, err := stateIndex(ctx, s, &h.Keys)
	if err != nil {
		return stateResult[bool]{}, err
	}
	view := h.Keys
	view.index = index
	view.Gaps = make([]stateGap, 0, len(h.Keys.Gaps))
	for i, gap := range h.Keys.Gaps {
		if err = s.step(ctx, 1); err != nil {
			return stateResult[bool]{}, err
		}
		if _, certified := h.keyPresenceGaps[i]; !certified {
			view.Gaps = append(view.Gaps, gap)
		}
	}
	out, err := query(&view)
	if err != nil {
		return stateResult[bool]{}, err
	}
	if out.Value.State != EvidenceKnown || !out.Value.Value {
		return original, nil
	}
	for i, gap := range h.Keys.Gaps {
		if err = s.step(ctx, 1); err != nil {
			return stateResult[bool]{}, err
		}
		witness, certified := h.keyPresenceGaps[i]
		if !certified {
			continue
		}
		relevant, err := h.keyPresenceGapRelevant(ctx, s, gap, pos, staff)
		if err != nil {
			return stateResult[bool]{}, err
		}
		if !relevant {
			continue
		}
		duplicate := false
		for _, old := range out.Provenance {
			if err = s.step(ctx, 1+int64(len(old.Node.DocumentID))+int64(len(witness.Node.DocumentID))); err != nil {
				return stateResult[bool]{}, err
			}
			if old == witness {
				duplicate = true
				break
			}
		}
		if !duplicate {
			if err = stateRetainCopy(ctx, s, witness); err != nil {
				return stateResult[bool]{}, err
			}
			out.Provenance = append(out.Provenance, stateClone(witness))
		}
		before := false
		if gap.Placement.Displaced && gap.Placement.Upper != nil {
			relation, err := h.positionOrder(ctx, s, *gap.Placement.Upper, pos)
			if err != nil {
				return stateResult[bool]{}, err
			}
			before = relation == stateBefore
		} else if !gap.Placement.Displaced {
			member, err := h.frameMember(ctx, s, pos)
			if err != nil {
				return stateResult[bool]{}, err
			}
			before = member && gap.LastMeasure < pos.MeasureOrdinal
		}
		if !before {
			out.Inherited = false
			out.InheritanceUnproved = true
		}
	}
	return out, nil
}

func (h *timelineState) keyPresenceGapRelevant(ctx context.Context, s *Session, gap stateGap, pos musicalPosition, staff Evidence[Integer]) (bool, error) {
	if err := s.step(ctx, 1+int64(len(gap.Part.DocumentID))+int64(len(pos.Part.DocumentID))); err != nil {
		return false, err
	}
	if gap.Part != pos.Part {
		return false, nil
	}
	applies, err := stateApplies(ctx, s, gap.Scope, staff)
	if err != nil || !applies {
		return false, err
	}
	relevant, err := h.gapCouldApply(ctx, s, gap, pos, true)
	if err != nil || !relevant {
		return false, err
	}
	// Only an independently applicable unconditional later publication can make
	// this valid key source irrelevant. Equal or uncertain positions do not win.
	for _, publication := range h.Keys.Publications {
		if err = s.step(ctx, 1+int64(len(publication.Position.Part.DocumentID))+int64(len(pos.Part.DocumentID))); err != nil {
			return false, err
		}
		if publication.Position.Part != pos.Part || !stateUnconditional(publication.TimeOnly) {
			continue
		}
		applies, err = stateApplies(ctx, s, publication.Scope, staff)
		if err != nil {
			return false, err
		}
		if !applies {
			continue
		}
		order, err := h.positionOrder(ctx, s, publication.Position, pos)
		if err != nil {
			return false, err
		}
		if !order.definitelyApplies(true) {
			continue
		}
		repaired, err := h.gapStrictlyBefore(ctx, s, gap, publication.Position)
		if err != nil {
			return false, err
		}
		if repaired {
			return false, nil
		}
	}
	return true, nil
}

// The unnamed partition sees AllStaves statements only. Each observed staff
// partition adds its own statements and gaps. Keeping the unnamed partition
// avoids assuming that observed numbers exhaust the possible staff identities,
// even when a finite staves count is encoded. Work is bounded by actual records,
// never by that count. Each partition keeps the ordinary chronology and repair
// semantics, including certified valid keys whose placement remains unknown.
const timelineKeyPresenceScopeScratchBytes int64 = 512

func (h *timelineState) queryStafflessKeyPresence(ctx context.Context, s *Session, pos musicalPosition) (stateResult[bool], error) {
	count, err := rationalCheckedAdd(BudgetBufferedBytes, int64(len(h.Keys.Publications)), int64(len(h.Keys.Gaps)))
	if err != nil {
		return stateResult[bool]{}, err
	}
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, count, timelineKeyPresenceScopeScratchBytes)
	if err != nil {
		return stateResult[bool]{}, err
	}
	bytes, err = rationalCheckedAdd(BudgetBufferedBytes, bytes, timelineKeyPresenceScratchFixedBytes)
	if err != nil {
		return stateResult[bool]{}, err
	}
	if _, err = AllocationSize(bytes); err != nil {
		return stateResult[bool]{}, err
	}
	capacity, err := AllocationSize(count)
	if err != nil {
		return stateResult[bool]{}, err
	}
	if err = s.step(ctx, count+1); err != nil {
		return stateResult[bool]{}, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return stateResult[bool]{}, err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }()
	seen := make(map[string]struct{})
	staves := make([]Integer, 0, capacity)
	addScope := func(part NodeRef, scope stateScope) error {
		same, err := stateSameRef(ctx, s, part, pos.Part)
		if err != nil || !same || scope.AllStaves {
			return err
		}
		if err = s.step(ctx, scope.Staff.ByteSize()+1); err != nil {
			return err
		}
		key := scope.Staff.String()
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			staves = append(staves, scope.Staff)
		}
		return nil
	}
	for _, publication := range h.Keys.Publications {
		if err = addScope(publication.Position.Part, publication.Scope); err != nil {
			return stateResult[bool]{}, err
		}
	}
	for _, gap := range h.Keys.Gaps {
		if err = addScope(gap.Part, gap.Scope); err != nil {
			return stateResult[bool]{}, err
		}
	}
	out, err := h.queryKeyPresenceScope(ctx, s, pos, Evidence[Integer]{State: EvidenceAbsent}, true)
	if err != nil {
		return stateResult[bool]{}, err
	}
	first := out.Value
	conflict := false
	allBefore := out.Inherited && !out.InheritanceUnproved
	allEqual := !out.Inherited && !out.InheritanceUnproved
	for _, staff := range staves {
		candidate, err := h.queryKeyPresenceScope(ctx, s, pos, Evidence[Integer]{State: EvidenceKnown, Value: staff}, false)
		if err != nil {
			return stateResult[bool]{}, err
		}
		if err = s.step(ctx, 1); err != nil {
			return stateResult[bool]{}, err
		}
		conflict = conflict || first.State != candidate.Value.State || first.Value != candidate.Value.Value
		if stateAvailabilityRank(candidate.Value.State) > stateAvailabilityRank(out.Value.State) {
			out.Value = candidate.Value
		}
		allBefore = allBefore && candidate.Inherited && !candidate.InheritanceUnproved
		allEqual = allEqual && !candidate.Inherited && !candidate.InheritanceUnproved
		for _, witness := range candidate.Provenance {
			duplicate := false
			for _, old := range out.Provenance {
				if err = s.step(ctx, 1+int64(len(old.Node.DocumentID))+int64(len(witness.Node.DocumentID))); err != nil {
					return stateResult[bool]{}, err
				}
				if old == witness {
					duplicate = true
					break
				}
			}
			if !duplicate {
				if err = stateRetainCopy(ctx, s, witness); err != nil {
					return stateResult[bool]{}, err
				}
				out.Provenance = append(out.Provenance, stateClone(witness))
			}
		}
	}
	if conflict && (out.Value.State == EvidenceKnown || out.Value.State == EvidenceAbsent) {
		out.Value = stateUnknown[bool]("timeline-staff-unresolved")
	}
	out.Inherited = allBefore
	out.InheritanceUnproved = !allBefore && !allEqual
	return out, nil
}
