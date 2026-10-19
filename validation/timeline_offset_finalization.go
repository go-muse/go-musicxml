package validation

import "context"

type timelinePendingOffset struct {
	Event *factNode
	Fact  timelineEventFact
}

const timelinePendingOffsetBytes int64 = 2048

func timelineOffsetEvent(r *stateReader, n *factNode) bool {
	return r.matches(n, timelineRoleHarmony) || r.matches(n, timelineRoleDirection) || r.matches(n, timelineRoleSound) || r.matches(n, timelineRoleListening)
}

// Allocate only actual deferred events, with a checked scratch admission. Notes
// do not get a duplicate measure-sized fact array. Every exit releases scratch.
func timelineNoScratch() {}

func newTimelinePendingOffsets(r *stateReader, block timelineMeasureBlock) ([]timelinePendingOffset, func(), error) {
	count := 0
	for _, event := range block.content.children {
		if !r.step(1) {
			return nil, func() {}, r.err
		}
		if timelineOffsetEvent(r, event) {
			count++
		}
	}
	if r.err != nil {
		return nil, func() {}, r.err
	}
	if count == 0 {
		return nil, timelineNoScratch, nil
	}
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(count), timelinePendingOffsetBytes)
	if err != nil {
		return nil, func() {}, err
	}
	if _, err = allocationSize(bytes); err != nil {
		return nil, func() {}, err
	}
	if err = r.s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return nil, func() {}, err
	}
	release := func() {
		if bytes != 0 {
			_ = r.s.account.Release(BudgetBufferedBytes, bytes)
			bytes = 0
		}
	}
	return make([]timelinePendingOffset, 0, count), release, nil
}

// Non-cursor offsets use units effective at their original musical coordinate,
// after all same-measure attribute publications are known. A later serialized
// divisions at that same score coordinate must not leave a stale lexical unit.
func (service *timelineService) finalizeOffsetEvent(ctx context.Context, s *Session, b *timelineBindings, block timelineMeasureBlock, event *factNode, fact timelineEventFact) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if err := service.State.proveFramePosition(ctx, s, &fact.Origin); err != nil {
		return err
	}
	units := fact.Units
	if fact.Origin.State == EvidenceKnown {
		selected, err := service.State.effectiveDivisionsEvidence(ctx, s, fact.Origin.Value)
		if err != nil {
			return err
		}
		units = selected
	} else {
		gap := timelineGapAs[musicalPosition, Decimal](fact.Origin, "timeline-position-unavailable")
		if stateAvailabilityRank(gap.State) > stateAvailabilityRank(units.State) {
			units = gap
		}
	}
	fact.Units = units
	var err error
	base := fact.Origin
	if r.matches(event, timelineRoleSound) {
		if err := service.readSoundAt(ctx, s, b, event, block, base, units, Evidence[Decimal]{State: EvidenceAbsent}, Evidence[bool]{State: EvidenceAbsent}, fact.Placement); err != nil {
			return err
		}
		return nil // readSoundAt retains the independently addressable fact.
	}
	if r.matches(event, timelineRoleListening) {
		if err := service.readListeningAt(ctx, s, b, event, block, base, units, Evidence[Decimal]{State: EvidenceAbsent}, Evidence[bool]{State: EvidenceAbsent}, fact.Placement); err != nil {
			return err
		}
		return nil
	}
	if r.matches(event, timelineRoleHarmony) || r.matches(event, timelineRoleDirection) {
		role := timelineRoleHarmonyOffset
		if r.matches(event, timelineRoleDirection) {
			role = timelineRoleDirectionOffset
		}
		offsetNode, offsetPresence := r.child(event, role)
		offset := stateUnavailable[bool, Decimal](offsetPresence)
		sound := stateUnavailable[bool, bool](offsetPresence)
		if offsetPresence.State == EvidenceKnown {
			offset = stateDecimalValue(r.scalar(offsetNode))
			sound = stateYesNo(r.attribute(offsetNode, timelineRoleOffsetSound))
		}
		fact.Position, err = s.offsetPosition(ctx, base, offset, units)
		if err != nil {
			return err
		}
		if err = service.State.proveFramePosition(ctx, s, &fact.Position); err != nil {
			return err
		}
		if r.matches(event, timelineRoleDirection) {
			if complete := r.enumerated(event); complete.State != EvidenceKnown && block.part.State == EvidenceKnown {
				if err := service.gapSoundPublishers(ctx, s, block.part.Value, block.ordinal, stateUnavailable[bool, musicalPosition](complete)); err != nil {
					return err
				}
			}
			// Read exact children independently: a malformed parent offset
			// cannot erase a valid local sound/listening override.
			for _, child := range event.children {
				if !r.step(1) {
					return r.err
				}
				if child.header.Declaration.State != EvidenceKnown && r.potential(child, timelineRoleDirectionSound) && block.part.State == EvidenceKnown {
					if err := service.gapSoundPublishers(ctx, s, block.part.Value, block.ordinal, stateUnavailable[DeclarationID, musicalPosition](*child.header.Declaration)); err != nil {
						return err
					}
				}
				switch {
				case r.matches(child, timelineRoleDirectionSound):
					err = service.readSoundAt(ctx, s, b, child, block, base, units, offset, sound, fact.Placement)
				case r.matches(child, timelineRoleDirectionListening):
					err = service.readListeningAt(ctx, s, b, child, block, base, units, offset, sound, fact.Placement)
				}
				if err != nil {
					return err
				}
			}
		}
	}

	if r.err != nil {
		return r.err
	}
	return service.storeFact(ctx, s, event.header.ID, fact)
}
