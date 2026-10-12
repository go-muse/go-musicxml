package validation

import "context"

// A normalization gate is first unavailable and can be restored once, using an
// independently retained origin and the preceding closed history. The fixed
// allowance covers map entry growth and its independently owned cause.
const timelineNormalizationGateBytes int64 = 512

type timelineNormalizationGate struct {
	Units    Evidence[Decimal]
	Restored bool
}

// Close sequential reconstruction against its COMPLETE history before any
// coordinate or bound is used as a certificate. New gates only remove dependent
// coordinates. Every invalidating pass adds a gate or strengthens its finite
// availability rank; raw publishers remain present in rebuilt histories.
// Per source event there is at most one gate addition, at most three strict
// unavailable-rank upgrades, and one independent restoration. Restored gates
// cannot be invalidated by refinement (the defensive check rejects a weakened
// proof). Together with one direction qualifier and one removal per excursion,
// each successful outer pass consumes a member of this finite transition set.
func (root *timelineService) closeUnitStage(ctx context.Context, s *Session, b *timelineBindings, blocks []timelineMeasureBlock, gates map[NodeID]timelineExcursionGate, positive map[NodeRef]bool, normalizations map[NodeID]timelineNormalizationGate, stage *timelineService) (*timelineService, error) {
	for {
		changed := false
		for _, block := range blocks {
			for _, event := range block.content.children {
				if err := s.step(ctx, 1); err != nil {
					return nil, err
				}
				fact, ok := stage.Facts[event.header.ID]
				if !ok || fact.Origin.State != EvidenceKnown {
					continue
				}
				previous, blocked := normalizations[event.header.ID]
				if !blocked && (fact.Duration.State != EvidenceKnown || fact.Units.State != EvidenceKnown) {
					continue
				}
				units, err := stage.State.effectiveDivisionsEvidence(ctx, s, fact.Origin.Value)
				if err != nil {
					return nil, err
				}
				if units.State == EvidenceKnown {
					same, e := s.sameTimelineUnits(ctx, fact.Units, units)
					if e != nil {
						return nil, e
					}
					if same || blocked && !previous.Restored {
						continue
					}
				}
				gap := units
				if gap.State == EvidenceKnown || gap.State == EvidenceAbsent || gap.State == "" {
					gap = Evidence[Decimal]{State: EvidenceUnsupported, Reason: "divisions-complete-history-placement-unresolved"}
				}
				if blocked {
					if previous.Restored {
						return nil, contract("timeline units", "independent normalization proof weakened")
					}
					if stateAvailabilityRank(gap.State) <= stateAvailabilityRank(previous.Units.State) {
						continue
					}
				} else {
					if err := s.retain(timelineNormalizationGateBytes); err != nil {
						return nil, err
					}
				}
				gap, err = stateOwnEvidence(ctx, s, gap)
				if err != nil {
					return nil, err
				}
				if normalizations == nil {
					normalizations = make(map[NodeID]timelineNormalizationGate)
				}
				normalizations[event.header.ID] = timelineNormalizationGate{Units: gap}
				changed = true
			}
		}
		stage.unitNormalizations = normalizations
		if !changed {
			return stage, nil
		}
		var err error
		stage, err = root.rebuildUnitStage(ctx, s, b, blocks, gates, positive, normalizations)
		if err != nil {
			return nil, err
		}
	}
}

// Restorations use ONLY the preceding closed stage. A gated note's unavailable
// end/descendants cannot establish the origin or effective units which restore
// that same note. Rebuilt coordinates refine previously retained uncertainty.
func (stage *timelineService) restoreNormalizations(ctx context.Context, s *Session, blocks []timelineMeasureBlock) (int, error) {
	restored := 0
	if len(stage.unitNormalizations) == 0 {
		return 0, nil
	}
	for _, block := range blocks {
		for _, event := range block.content.children {
			if err := s.step(ctx, 1); err != nil {
				return 0, err
			}
			gate, ok := stage.unitNormalizations[event.header.ID]
			if !ok || gate.Restored {
				continue
			}
			fact := stage.Facts[event.header.ID]
			if fact.Origin.State != EvidenceKnown {
				continue
			}
			units, err := stage.State.effectiveDivisionsEvidence(ctx, s, fact.Origin.Value)
			if err != nil {
				return 0, err
			}
			if units.State != EvidenceKnown || units.Value.value.Sign() <= 0 {
				continue
			}
			gate.Units = units
			gate.Restored = true
			stage.unitNormalizations[event.header.ID] = gate
			restored++
		}
	}
	return restored, nil
}

// Unavailable units caused by an unresolved displaced publisher also make the
// next unavailable cursor displaced. A repaired or unrelated gap cannot widen
// an ordinary unknown cursor's canonical scope.
func (h *timelineState) displacedUnitDependency(ctx context.Context, s *Session, at musicalPosition) (bool, error) {
	unrepaired := func(g stateGap) (bool, error) {
		applies, err := h.gapCouldApply(ctx, s, g, at, true)
		if err != nil || !applies {
			return false, err
		}
		for _, pub := range h.Divisions.Publications {
			if err = s.step(ctx, 1+int64(len(pub.Position.Part.DocumentID))+int64(len(at.Part.DocumentID))); err != nil {
				return false, err
			}
			if pub.Position.Part != at.Part || pub.TimeOnly != nil || pub.Value.State == EvidenceAbsent {
				continue
			}
			order, e := h.positionOrder(ctx, s, pub.Position, at)
			if e != nil {
				return false, e
			}
			if !order.definitelyApplies(true) {
				continue
			}
			before, e := h.gapStrictlyBefore(ctx, s, g, pub.Position)
			if e != nil {
				return false, e
			}
			if before {
				return false, nil
			}
		}
		return true, nil
	}
	for _, gap := range h.Divisions.Gaps {
		if err := s.step(ctx, 1+int64(len(gap.Part.DocumentID))+int64(len(at.Part.DocumentID))); err != nil {
			return false, err
		}
		if gap.Part != at.Part || !gap.Placement.Displaced && (gap.Position == nil || !gap.Position.UnprovedFrame) {
			continue
		}
		yes, err := unrepaired(gap)
		if err != nil || yes {
			return yes, err
		}
	}
	for _, pub := range h.Divisions.Publications {
		if err := s.step(ctx, 1+int64(len(pub.Position.Part.DocumentID))+int64(len(at.Part.DocumentID))); err != nil {
			return false, err
		}
		if pub.Position.Part != at.Part || !pub.Position.UnprovedFrame {
			continue
		}
		pos := pub.Position
		gap := stateGap{Part: at.Part, FirstMeasure: pos.MeasureOrdinal, LastMeasure: pos.MeasureOrdinal, Position: &pos}
		yes, err := unrepaired(gap)
		if err != nil || yes {
			return yes, err
		}
	}
	return false, nil
}
