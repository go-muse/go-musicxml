package validation

import "context"

// The service is private, session-owned evidence, never a second identity index.
// Final denotes completed publication, not universal availability of each fact.
type timelineService struct {
	State              timelineState
	Playback           timelinePlaybackContext
	NoteOrigins        map[NodeID]noteTimingOrigin
	GraceNeighbors     map[NodeID]graceNeighborTiming
	FiguredBassOrigins map[NodeID]figuredBassTimingOrigin
	DirectionOrigins   map[NodeID]directionTimingOrigin
	Facts              map[NodeID]timelineEventFact
	Final              bool
}

// Indirect fixed record plus at least 512 bytes of map/growth headroom.
// A matching per-call scratch allowance covers the unpublished temporary.
const timelineFactReserveBytes int64 = 1536

type timelineEventFact struct {
	// Origin is the unshifted cursor/onset coordinate. Position may describe
	// a direction/harmony display displacement or a sound/listening effect.
	Origin                      Evidence[musicalPosition]
	Position, End               Evidence[musicalPosition]
	Duration, PrincipalDuration Evidence[Rational]
	Principal                   Evidence[NodeRef]
	Units                       Evidence[Decimal]
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
	if err := s.account.Charge(BudgetContextFacts, 1); err != nil {
		return err
	}
	if err := s.account.Charge(BudgetBufferedBytes, timelineFactReserveBytes); err != nil {
		return err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, timelineFactReserveBytes) }()
	if err := s.retain(timelineFactReserveBytes); err != nil {
		return err
	}
	var err error
	if fact.Origin, err = stateOwnEvidence(ctx, s, fact.Origin); err != nil {
		return err
	}
	if fact.Position, err = stateOwnEvidence(ctx, s, fact.Position); err != nil {
		return err
	}
	if fact.End, err = stateOwnEvidence(ctx, s, fact.End); err != nil {
		return err
	}
	if fact.Units, err = stateOwnEvidence(ctx, s, fact.Units); err != nil {
		return err
	}
	if fact.Duration, err = stateOwnEvidence(ctx, s, fact.Duration); err != nil {
		return err
	}
	if fact.PrincipalDuration, err = stateOwnEvidence(ctx, s, fact.PrincipalDuration); err != nil {
		return err
	}
	if fact.Principal, err = stateOwnEvidence(ctx, s, fact.Principal); err != nil {
		return err
	}
	if fact.Staff, err = stateOwnEvidence(ctx, s, fact.Staff); err != nil {
		return err
	}
	if fact.Voice, err = stateOwnEvidence(ctx, s, fact.Voice); err != nil {
		return err
	}
	if fact.Part, err = stateOwnEvidence(ctx, s, fact.Part); err != nil {
		return err
	}
	service.Facts[id] = fact
	return nil
}
