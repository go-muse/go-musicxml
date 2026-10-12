package validation

import "context"

type playbackSpatialState struct {
	Pan, Elevation stateResult[Decimal]
}

// spatial projects only a caller-selected, identity-qualified MIDI record from
// this same sound event. It never selects an instrument or imports a continuing
// instrument-property omission policy. Sound's MusicXML 4.0 annotation specifies
// MIDI pan/elevation precedence independently, so a missing elevation does not
// undo an explicit MIDI pan. Unknown/invalid is never treated as absent.
func (sound soundPlaybackContext) spatial(ctx context.Context, s *Session, instrument Evidence[NodeRef], midi Evidence[instrumentRecord[midiState]]) (playbackSpatialState, error) {
	if err := s.step(ctx, 1); err != nil {
		return playbackSpatialState{}, err
	}
	// Admit the two temporary provenance entries before constructing their slices.
	if err := s.retain(1024); err != nil {
		return playbackSpatialState{}, err
	}
	field := func(value Evidence[Decimal], source NodeRef) stateResult[Decimal] {
		result := stateResult[Decimal]{Value: value}
		if value.State != EvidenceAbsent && source.Node != 0 {
			result.Provenance = []stateProvenance{{Node: source, Kind: stateExplicit}}
		}
		return result
	}
	out := playbackSpatialState{Pan: field(sound.Pan, sound.Origin.Source), Elevation: field(sound.Elevation, sound.Origin.Source)}
	unavailable := func(v Evidence[bool]) {
		out.Pan = stateResult[Decimal]{Value: stateUnavailable[bool, Decimal](v)}
		out.Elevation = stateResult[Decimal]{Value: stateUnavailable[bool, Decimal](v)}
	}
	if midi.State != EvidenceAbsent {
		if midi.State != EvidenceKnown {
			unavailable(stateUnavailable[instrumentRecord[midiState], bool](midi))
		} else {
			record := midi.Value
			gap := Evidence[bool]{State: EvidenceKnown, Value: true}
			if instrument.State != EvidenceKnown {
				gap = stateMergeGap(gap, stateUnavailable[NodeRef, bool](instrument))
			}
			if record.Instrument.State != EvidenceKnown {
				gap = stateMergeGap(gap, stateUnavailable[NodeRef, bool](record.Instrument))
			}
			if sound.Origin.Part.State != EvidenceKnown {
				gap = stateMergeGap(gap, stateUnavailable[NodeRef, bool](sound.Origin.Part))
			}
			if gap.State != EvidenceKnown {
				unavailable(gap)
			} else {
				if record.Instrument.Value != instrument.Value || record.Part != sound.Origin.Part.Value || record.Source.DocumentID != sound.Origin.Source.DocumentID || record.Source.DocumentID != s.documentID || record.Source.Node == 0 || instrument.Value.Node == 0 {
					return playbackSpatialState{}, contract("sound spatial precedence", "matching canonical part and instrument required")
				}
				node := s.nodes[record.Source.Node]
				if node == nil {
					unavailable(stateUnknown[bool]("playback-midi-owner-unavailable"))
				} else if node.header.Parent != sound.Origin.Source.Node {
					return playbackSpatialState{}, contract("sound spatial precedence", "MIDI record must belong to the same sound event")
				} else if record.Value.State != EvidenceKnown {
					v := stateUnavailable[midiState, bool](record.Value)
					if v.State == EvidenceAbsent || v.State == "" {
						v = stateUnknown[bool]("playback-midi-value-unavailable")
					}
					unavailable(v)
				} else {
					if record.Value.Value.Pan.State != EvidenceAbsent {
						out.Pan = field(record.Value.Value.Pan, record.Source)
					}
					if record.Value.Value.Elevation.State != EvidenceAbsent {
						out.Elevation = field(record.Value.Value.Elevation, record.Source)
					}
				}
			}
		}
	}
	if err := stateRetainCopy(ctx, s, out); err != nil {
		return playbackSpatialState{}, err
	}
	return stateClone(out), nil
}
