package validation

import "context"

// The service is private, session-owned evidence, never a second identity index.
// Final denotes completed publication, not universal availability of each fact.
type timelineService struct {
	State timelineState
	Facts map[NodeID]timelineEventFact
	Final bool
}
type timelineEventFact struct {
	Position                    Evidence[musicalPosition]
	Duration, PrincipalDuration Evidence[Rational]
	Principal                   Evidence[NodeRef]
	Staff                       Evidence[Integer]
	Voice                       Evidence[string]
	Part                        Evidence[NodeRef]
}

func (s *Session) newTimelineService(ctx context.Context, key RuleKey) (*timelineService, error) {
	if err := s.step(ctx, 1); err != nil {
		return nil, err
	}
	if service := s.timeline[key]; service != nil {
		return service, nil
	}
	if err := s.retain(int64(2048 + len(key.ID))); err != nil {
		return nil, err
	}
	if s.timeline == nil {
		s.timeline = make(map[RuleKey]*timelineService)
	}
	service := &timelineService{Facts: make(map[NodeID]timelineEventFact)}
	s.timeline[key] = service
	return service, nil
}
func (service *timelineService) storeFact(ctx context.Context, s *Session, id NodeID, fact timelineEventFact) error {
	// All values are already admitted immutable session facts; charge the fixed
	// record/map backing before publication, without double-charging numerics.
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if _, exists := service.Facts[id]; exists {
		return contract("timeline fact", "duplicate publication")
	}
	if err := s.retain(1024); err != nil {
		return err
	}
	service.Facts[id] = fact
	return nil
}
