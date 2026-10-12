package validation

import "context"

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
	if err := s.retain(512); err != nil {
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
	project := func(v keyState) Evidence[bool] { return v.encodingEvidence() }
	original, err := queryState(ctx, s, h, &h.Keys, pos, staff, project)
	if err != nil || len(h.keyPresenceGaps) == 0 || staff.State != EvidenceKnown || original.Value.State == EvidenceKnown && original.Value.Value {
		return original, err
	}
	// Remove only certified value-preserving gaps for an independent baseline
	// proof. Missing baseline and every other causal gap remain in the query.
	count := int64(len(h.Keys.Gaps))
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, count, 512)
	if err != nil {
		return stateResult[bool]{}, err
	}
	bytes, err = rationalCheckedAdd(BudgetBufferedBytes, bytes, 256)
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
	out, err := queryState(ctx, s, h, &view, pos, staff, project)
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
