package validation

import "context"

// TimingStateOrigin identifies the encoded statements used for one effective
// field, including documented defaults and unresolved inheritance ordering.
type TimingStateOrigin struct {
	Property                       Code
	Sources                        []TimingStateSource
	Inherited, InheritanceUnproved bool
}

// TimingStateSource is one established statement or documented default origin.
type TimingStateSource struct {
	Source NodeRef
	Kind   Code
}

// TimingSpatial retains identity-qualified same-sound per-field precedence.
type TimingSpatial struct {
	Instrument                 Evidence[NodeRef]
	Pan, Elevation             Evidence[Decimal]
	PanSource, ElevationSource []TimingStateSource
}

// TimingMeterPair preserves exact encoded meter components, not a meter-fill rule.
type TimingMeterPair struct{ Beats, BeatType Evidence[string] }

func performanceStateSources(ctx context.Context, s *Session, raw []stateProvenance) ([]TimingStateSource, error) {
	if err := stateRetainCount(s, len(raw), 256); err != nil {
		return nil, err
	}
	out := make([]TimingStateSource, len(raw))
	for i, v := range raw {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		kind := Code("explicit")
		switch v.Kind {
		case stateInitialDefault:
			kind = "initial-default"
		case stateDocumentedDefault:
			kind = "documented-default"
		}
		out[i] = TimingStateSource{Source: v.Node, Kind: kind}
	}
	return out, nil
}
func performanceStateOrigin[T any](ctx context.Context, s *Session, out *TimingContextResult, property Code, v stateResult[T]) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if err := s.retain(512); err != nil {
		return err
	}
	sources, err := performanceStateSources(ctx, s, v.Provenance)
	if err != nil {
		return err
	}
	out.StateOrigins = append(out.StateOrigins, TimingStateOrigin{Property: property, Sources: sources, Inherited: v.Inherited, InheritanceUnproved: v.InheritanceUnproved})
	return nil
}
func performanceSoundSpatial(ctx context.Context, s *Session, service *timelineService, index *performanceAdviceIndex, n *factNode, out *TimingContextResult) error {
	sound, ok := service.Playback.Sounds[n.header.ID]
	if !ok {
		return nil
	}
	for _, id := range index.midiByParent[n.header.ID] {
		m := service.State.MIDI[id]
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		result, err := sound.spatial(ctx, s, m.Instrument, Evidence[instrumentRecord[midiState]]{State: EvidenceKnown, Value: m})
		if err != nil {
			return err
		}
		pan, err := performanceStateSources(ctx, s, result.Pan.Provenance)
		if err != nil {
			return err
		}
		elevation, err := performanceStateSources(ctx, s, result.Elevation.Provenance)
		if err != nil {
			return err
		}
		if err = s.retain(1536); err != nil {
			return err
		}
		out.Spatial = append(out.Spatial, TimingSpatial{Instrument: m.Instrument, Pan: result.Pan.Value, Elevation: result.Elevation.Value, PanSource: pan, ElevationSource: elevation})
	}
	return nil
}
func cloneTimingStateOrigins(v []TimingStateOrigin) []TimingStateOrigin {
	out := make([]TimingStateOrigin, len(v))
	for i, p := range v {
		p.Sources = append([]TimingStateSource(nil), p.Sources...)
		out[i] = p
	}
	return out
}
func cloneTimingSpatial(v []TimingSpatial) []TimingSpatial {
	out := make([]TimingSpatial, len(v))
	for i, p := range v {
		p.Instrument = cloneEvidence(p.Instrument)
		p.Pan = cloneEvidence(p.Pan)
		p.Elevation = cloneEvidence(p.Elevation)
		p.PanSource = append([]TimingStateSource(nil), p.PanSource...)
		p.ElevationSource = append([]TimingStateSource(nil), p.ElevationSource...)
		out[i] = p
	}
	return out
}
func cloneTimingMeter(v []TimingMeterPair) []TimingMeterPair {
	out := make([]TimingMeterPair, len(v))
	for i, p := range v {
		p.Beats = cloneEvidence(p.Beats)
		p.BeatType = cloneEvidence(p.BeatType)
		out[i] = p
	}
	return out
}
