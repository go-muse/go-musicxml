package validation

import "context"

// Preserve source-order cause precedence as well as exact exclusion membership.
// firstKnown is where an unavailable note voice first becomes relevant; gapIndex
// is the first source position of the highest-ranked unavailable exclusion.
type contextRepeatVoiceFilter struct {
	known                map[string]bool
	gap                  Evidence[bool]
	firstKnown, gapIndex int
}

func contextRepeatBuildVoiceFilter(ctx context.Context, s *Session, values []Evidence[string]) (*contextRepeatVoiceFilter, func(), error) {
	if err := s.step(ctx, 1); err != nil {
		return nil, nil, err
	}
	if len(values) == 0 {
		return nil, func() {}, nil
	}
	release, err := contextRepeatIndexReserve(ctx, s, len(values))
	if err != nil {
		return nil, nil, err
	}
	out := &contextRepeatVoiceFilter{known: make(map[string]bool), gap: navigationKnownBool(true), firstKnown: -1, gapIndex: -1}
	for i, v := range values {
		if err = s.step(ctx, 1+int64(len(v.Value))); err != nil {
			release()
			return nil, nil, err
		}
		if v.State == EvidenceKnown {
			out.known[v.Value] = true
			if out.firstKnown < 0 {
				out.firstKnown = i
			}
		} else {
			gap := stateUnavailable[string, bool](v)
			if gap.State == EvidenceAbsent || gap.State == "" {
				gap = stateUnknown[bool]("timeline-gap-availability-unresolved")
			}
			if stateAvailabilityRank(gap.State) > stateAvailabilityRank(out.gap.State) {
				out.gap = gap
				out.gapIndex = i
			}
		}
	}
	return out, release, nil
}
func contextRepeatVoiceApplies(ctx context.Context, s *Session, filter *contextRepeatVoiceFilter, voice Evidence[string]) (Evidence[bool], error) {
	if err := s.step(ctx, 1); err != nil {
		return Evidence[bool]{}, err
	}
	if filter == nil {
		return navigationKnownBool(true), nil
	}
	if voice.State == EvidenceKnown {
		if err := s.step(ctx, int64(len(voice.Value))); err != nil {
			return Evidence[bool]{}, err
		}
		if filter.known[voice.Value] {
			return navigationKnownBool(false), nil
		}
		return filter.gap, nil
	}
	if filter.firstKnown < 0 {
		return filter.gap, nil
	}
	gap := stateUnavailable[string, bool](voice)
	if gap.State == EvidenceAbsent {
		gap = stateUnknown[bool]("context-repeat-note-voice-unavailable")
	}
	if filter.gapIndex >= 0 && filter.gapIndex < filter.firstKnown {
		return stateMergeGap(filter.gap, gap), nil
	}
	return stateMergeGap(gap, filter.gap), nil
}
