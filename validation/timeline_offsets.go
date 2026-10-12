package validation

import "context"

// timelinePositionDependency keeps the strongest required causal evidence.
// Equal-rank causes retain their deterministic dependency order.
func timelinePositionDependency(position, dependency Evidence[musicalPosition]) Evidence[musicalPosition] {
	if stateAvailabilityRank(dependency.State) > stateAvailabilityRank(position.State) {
		return dependency
	}
	return position
}

// offsetPosition applies an optional signed displacement in already-established
// divisions. It does not choose units across a divisions boundary, move the
// serialization cursor, or infer a measure length. An absent offset leaves the
// base unchanged and therefore has no divisions dependency.
func (s *Session) offsetPosition(ctx context.Context, base Evidence[musicalPosition], offset, units Evidence[Decimal]) (Evidence[musicalPosition], error) {
	if err := s.step(ctx, 1); err != nil {
		return Evidence[musicalPosition]{}, err
	}
	if base.State != EvidenceKnown {
		base = timelineGapAs[musicalPosition, musicalPosition](base, "timeline-position-unavailable")
	}
	if offset.State == EvidenceAbsent {
		return base, nil
	}
	position := base
	if offset.State != EvidenceKnown {
		position = timelinePositionDependency(position, timelineGapAs[Decimal, musicalPosition](offset, "timeline-offset-unavailable"))
	}
	if units.State != EvidenceKnown {
		position = timelinePositionDependency(position, timelineGapAs[Decimal, musicalPosition](units, "initial-divisions-unavailable"))
	}
	if position.State != EvidenceKnown {
		return position, nil
	}
	delta, err := s.normalizeTimelineDuration(ctx, offset, units)
	if err != nil {
		return Evidence[musicalPosition]{}, err
	}
	if delta.State != EvidenceKnown {
		return timelineGapAs[Rational, musicalPosition](delta, "timeline-offset-unavailable"), nil
	}
	sum, err := s.rationalAdd(ctx, base.Value.Offset, delta.Value)
	if err != nil {
		return Evidence[musicalPosition]{}, err
	}
	position.Value.Offset = sum
	if delta.Value.value.Sign() != 0 {
		position.Value.UnprovedFrame = true
		if !base.Value.UnprovedFrame && sum.value.Sign() >= 0 && delta.Value.value.Sign() < 0 {
			position.Value.UnprovedFrame = false
		}
	}
	return position, nil
}

// readSoundAt uses the original containing cursor as its base. MusicXML 4.0's
// sound documentation makes its own offset override a direction's offset, not
// add to it. The offset documentation's sound attribute defaults to no: only a
// known yes applies the parent direction displacement to playback. Defaults are
// used only for proven absence, never unknown or invalid evidence. See pinned
// musicxml.xsd complex types offset (line 3937) and sound (line 4123).
func (service *timelineService) readSoundAt(ctx context.Context, s *Session, b *timelineBindings, sound *factNode, block timelineMeasureBlock, base Evidence[musicalPosition], units Evidence[Decimal], directionOffset Evidence[Decimal], directionSound Evidence[bool]) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.matches(sound, timelineRoleSound) && !r.matches(sound, timelineRoleDirectionSound) {
		if r.err != nil {
			return r.err
		}
		return contract("sound placement", "exact sound declaration required")
	}
	own := stateDecimalValue(r.childScalar(sound, timelineRoleSoundOffset))
	if r.err != nil {
		return r.err
	}
	if block.part.State != EvidenceKnown {
		base = timelinePositionDependency(base, timelineGapAs[NodeRef, musicalPosition](block.part, "timeline-part-unavailable"))
	}
	position, err := s.playbackOffsetPosition(ctx, base, own, units, directionOffset, directionSound)
	if err != nil {
		return err
	}
	if err = service.State.proveFramePosition(ctx, s, &position); err != nil {
		return err
	}
	if block.part.State == EvidenceKnown {
		if position.State == EvidenceKnown {
			err = service.State.readInstrumentChanges(ctx, s, b, sound, position.Value)
		} else {
			err = service.State.gapInstrumentChanges(ctx, s, b, sound, block.part.Value, block.ordinal, position)
		}
		if err != nil {
			return err
		}
	}
	fact := timelineEventFact{
		Part: block.part, Origin: base, Position: position, Units: units,
		Staff: Evidence[Integer]{State: EvidenceAbsent}, Voice: Evidence[string]{State: EvidenceAbsent},
		Duration: Evidence[Rational]{State: EvidenceAbsent}, Principal: Evidence[NodeRef]{State: EvidenceAbsent}, PrincipalDuration: Evidence[Rational]{State: EvidenceAbsent},
	}
	if err := service.Playback.readSound(ctx, s, b, sound, block, fact); err != nil {
		return err
	}
	return service.storeFact(ctx, s, sound.header.ID, fact)
}

// playbackOffsetPosition is shared by sound and listening. Both local offset
// declarations override their parent direction offset relative to the original
// cursor. This is only score placement, not interpretation of a performance.
func (s *Session) playbackOffsetPosition(ctx context.Context, base Evidence[musicalPosition], own, units, directionOffset Evidence[Decimal], directionSound Evidence[bool]) (Evidence[musicalPosition], error) {
	offset := own
	flag := Evidence[musicalPosition]{State: EvidenceKnown}
	if own.State == EvidenceAbsent {
		switch {
		case directionOffset.State == EvidenceAbsent:
			// No parent displacement exists; its playback flag is irrelevant.
		case directionSound.State == EvidenceAbsent:
			// The documented default is no, so the parent offset is visual only.
		case directionSound.State == EvidenceKnown && !directionSound.Value:
			// An explicit no likewise has no offset or divisions dependency.
		default:
			offset = directionOffset
			if directionSound.State != EvidenceKnown {
				flag = timelineGapAs[bool, musicalPosition](directionSound, "timeline-offset-sound-unavailable")
			}
		}
	}
	position, err := s.offsetPosition(ctx, base, offset, units)
	if err != nil {
		return Evidence[musicalPosition]{}, err
	}
	return timelinePositionDependency(position, flag), nil
}

// readListeningAt retains an independently addressable listening origin without
// running a machine-listening engine or interpreting player/repeat selectors.
// The override rule is explicit in musicxml.xsd listening (line 3739); the parent
// direction's sound=no default applies to listening as well (offset, line 3937).
func (service *timelineService) readListeningAt(ctx context.Context, s *Session, b *timelineBindings, listening *factNode, block timelineMeasureBlock, base Evidence[musicalPosition], units Evidence[Decimal], directionOffset Evidence[Decimal], directionSound Evidence[bool]) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.matches(listening, timelineRoleListening) && !r.matches(listening, timelineRoleDirectionListening) {
		if r.err != nil {
			return r.err
		}
		return contract("listening placement", "exact listening declaration required")
	}
	own := stateDecimalValue(r.childScalar(listening, timelineRoleListeningOffset))
	if r.err != nil {
		return r.err
	}
	if block.part.State != EvidenceKnown {
		base = timelinePositionDependency(base, timelineGapAs[NodeRef, musicalPosition](block.part, "timeline-part-unavailable"))
	}
	position, err := s.playbackOffsetPosition(ctx, base, own, units, directionOffset, directionSound)
	if err != nil {
		return err
	}
	if err = service.State.proveFramePosition(ctx, s, &position); err != nil {
		return err
	}
	return service.storeFact(ctx, s, listening.header.ID, timelineEventFact{
		Part: block.part, Origin: base, Position: position, Units: units,
		Staff: Evidence[Integer]{State: EvidenceAbsent}, Voice: Evidence[string]{State: EvidenceAbsent},
		Duration: Evidence[Rational]{State: EvidenceAbsent}, Principal: Evidence[NodeRef]{State: EvidenceAbsent}, PrincipalDuration: Evidence[Rational]{State: EvidenceAbsent},
	})
}
