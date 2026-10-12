package validation

import "context"

// A known publisher affects the query only at/after its exact position. A
// strictly later selected statement repairs its property; an equal-position
// statement cannot silently outrank the unavailable publisher. Genuinely
// unplaced publishers retain the existing conservative interval behavior.
func (h *timelineState) gapCouldApply(ctx context.Context, s *Session, gap stateGap, query musicalPosition, inclusive bool) (bool, error) {
	if gap.Position == nil {
		member, err := h.frameMember(ctx, s, query)
		return !member || gap.FirstMeasure <= query.MeasureOrdinal, err
	}
	if !stateValidPosition(*gap.Position) || gap.Position.Part != gap.Part || gap.FirstMeasure != gap.Position.MeasureOrdinal || gap.LastMeasure != gap.Position.MeasureOrdinal {
		return false, contract("timeline gap", "exact point must match its part and interval")
	}
	order, err := h.positionOrder(ctx, s, *gap.Position, query)
	return order.possiblyApplies(inclusive), err
}
func (h *timelineState) gapStrictlyBefore(ctx context.Context, s *Session, gap stateGap, selected musicalPosition) (bool, error) {
	if gap.Position == nil {
		member, err := h.frameMember(ctx, s, selected)
		return member && selected.MeasureOrdinal > gap.LastMeasure, err
	}
	order, err := h.positionOrder(ctx, s, *gap.Position, selected)
	return order == stateBefore, err
}
func (h *timelineState) stateGapUnrepaired(ctx context.Context, s *Session, gap stateGap, query musicalPosition, selected *musicalPosition, inclusive bool) (bool, error) {
	relevant, err := h.gapCouldApply(ctx, s, gap, query, inclusive)
	if err != nil || !relevant {
		return relevant, err
	}
	if selected == nil {
		return true, nil
	}
	repaired, err := h.gapStrictlyBefore(ctx, s, gap, *selected)
	return !repaired, err
}

func (h *timelineState) addUnresolvedInstrumentChange(ctx context.Context, s *Session, record instrumentRecord[instrumentState], ordinal uint64) error {
	if record.Instrument.State == EvidenceKnown {
		return contract("instrument gap", "unresolved instrument required")
	}
	if record.Position.State == EvidenceKnown && (!stateValidPosition(record.Position.Value) || record.Position.Value.Part != record.Part) {
		return contract("instrument gap", "matching positioned part required")
	}
	gap := stateGap{Part: record.Part, FirstMeasure: ordinal, LastMeasure: ordinal, Scope: stateScope{AllStaves: true}, Availability: stateUnavailable[NodeRef, bool](record.Instrument)}
	if record.Position.State == EvidenceKnown {
		gap.Position = &record.Position.Value
		gap.FirstMeasure = record.Position.Value.MeasureOrdinal
		gap.LastMeasure = gap.FirstMeasure
	} else {
		gap.Availability = stateMergeGap(gap.Availability, stateUnavailable[musicalPosition, bool](record.Position))
	}
	gap.Availability = stateConditionalValue(gap.Availability, record.TimeOnly)
	if _, err := stateReserve(ctx, s, record); err != nil {
		return err
	}
	if err := addInstrumentStatementGaps(ctx, s, &h.InstrumentPropertyGaps, gap, record.Value); err != nil {
		return err
	}
	h.UnresolvedInstrumentChanges = append(h.UnresolvedInstrumentChanges, stateClone(record))
	return nil
}

// Find the latest unconditional statement without rebuilding an index or
// copying payloads. Only an exceptional conditional suffix is walked backwards;
// each tie, scope comparison and predecessor lookup remains charged. A sparse
// property view excludes absent fields before this helper is called. Returning
// a borrowed immutable position is confined to the current query.
func stateRepairPosition[T any](ctx context.Context, s *Session, history *stateHistory[T], staff Integer, selection stateSelection) (*musicalPosition, error) {
	for selection.Best >= 0 {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		selected := &history.Publications[selection.Best]
		if stateUnconditional(selected.TimeOnly) {
			return &selected.Position, nil
		}
		for _, group := range selection.Groups {
			for _, index := range group {
				if err := s.step(ctx, 1); err != nil {
					return nil, err
				}
				candidate := &history.Publications[index]
				if !stateUnconditional(candidate.TimeOnly) {
					continue
				}
				applies, err := stateApplies(ctx, s, candidate.Scope, Evidence[Integer]{State: EvidenceKnown, Value: staff})
				if err != nil {
					return nil, err
				}
				if !applies {
					continue
				}
				cmp, err := stateComparePosition(ctx, s, candidate.Position, selected.Position)
				if err != nil {
					return nil, err
				}
				if cmp == 0 {
					return &candidate.Position, nil
				}
			}
		}
		var err error
		selection, err = selectState(ctx, s, history, selected.Position, staff, false)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}
