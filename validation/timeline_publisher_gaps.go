package validation

import "context"

// Only publishers capable of changing sound context poison instrument and
// playback histories. An unbound attributes declaration alone cannot do so.
func (service *timelineService) gapSoundPublishers(ctx context.Context, s *Session, part NodeRef, ordinal uint64, gap Evidence[musicalPosition], placement ...stateGapPlacement) error {
	if err := service.State.gapInstrumentPublisher(ctx, s, part, ordinal, gap, placement...); err != nil {
		return err
	}
	return service.Playback.gapAll(ctx, s, part, ordinal, gap, placement...)
}
func (service *timelineService) gapAllPublishers(ctx context.Context, s *Session, part NodeRef, ordinal uint64, gap Evidence[musicalPosition], placement ...stateGapPlacement) error {
	if err := service.State.gapAttributeFamilies(ctx, s, part, ordinal, gap, placement...); err != nil {
		return err
	}
	return service.gapSoundPublishers(ctx, s, part, ordinal, gap, placement...)
}
