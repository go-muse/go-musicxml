package validation

import "context"

// atMeasureStart identifies the structural measure's zero coordinate, not its
// printed label or inferred meter. Negative/out-of-domain positions cannot
// establish the mid-measure presentation condition.
func atMeasureStart(ctx context.Context, s *Session, position Evidence[musicalPosition]) (Evidence[bool], error) {
	if position.State != EvidenceKnown {
		return stateOwnEvidence(ctx, s, timelineGapAs[musicalPosition, bool](position, "timeline-position-unavailable"))
	}
	cmp, err := s.rationalCmp(ctx, position.Value.Offset, timelineZero)
	if err != nil {
		return Evidence[bool]{}, err
	}
	if cmp < 0 {
		return stateUnknown[bool]("timeline-measure-boundary-unavailable"), nil
	}
	return Evidence[bool]{State: EvidenceKnown, Value: cmp == 0}, nil
}

// MusicXML4.0 cancel-location: omitted location is left; before-barline is
// treated as left for a mid-measure key. Raw encoding remains in keyState.
func (v keyState) cancelLocation(ctx context.Context, s *Session, position Evidence[musicalPosition]) (Evidence[string], error) {
	if v.Cancel.State == EvidenceAbsent {
		return Evidence[string]{State: EvidenceAbsent}, nil
	}
	if v.CancelLocation.State != EvidenceKnown && v.CancelLocation.State != EvidenceAbsent {
		return stateOwnEvidence(ctx, s, v.CancelLocation)
	}
	if v.Cancel.State != EvidenceKnown {
		return stateOwnEvidence(ctx, s, timelineGapAs[Integer, string](v.Cancel, "timeline-cancel-unavailable"))
	}
	if v.CancelLocation.State == EvidenceAbsent {
		return Evidence[string]{State: EvidenceKnown, Value: "left"}, nil
	}
	if v.CancelLocation.Value != "before-barline" {
		return stateOwnEvidence(ctx, s, v.CancelLocation)
	}
	start, err := atMeasureStart(ctx, s, position)
	if err != nil {
		return Evidence[string]{}, err
	}
	if start.State != EvidenceKnown {
		return stateOwnEvidence(ctx, s, stateUnavailable[bool, string](start))
	}
	if !start.Value {
		return Evidence[string]{State: EvidenceKnown, Value: "left"}, nil
	}
	return stateOwnEvidence(ctx, s, v.CancelLocation)
}

// MusicXML4.0 clef: after-barline is ignored for mid-measure clefs. No absent
// attribute default is invented at measure start, and raw evidence is retained.
func (v clefState) afterBarlineAt(ctx context.Context, s *Session, position Evidence[musicalPosition]) (Evidence[bool], error) {
	start, err := atMeasureStart(ctx, s, position)
	if err != nil {
		return Evidence[bool]{}, err
	}
	if start.State == EvidenceKnown && !start.Value {
		return Evidence[bool]{State: EvidenceAbsent}, nil
	}
	if start.State != EvidenceKnown {
		if stateAvailabilityRank(v.AfterBarline.State) > stateAvailabilityRank(start.State) {
			return stateOwnEvidence(ctx, s, v.AfterBarline)
		}
		return stateOwnEvidence(ctx, s, start)
	}
	return stateOwnEvidence(ctx, s, v.AfterBarline)
}
