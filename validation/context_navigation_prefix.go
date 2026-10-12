package validation

import "context"

// Before an unresolved ordering/control edge, preserve the maximal initial
// linear segment whose precedence against every remaining event is proved.
// No unique target or paired region is inferred from this truncated domain.
func navigationLinearPrefix(ctx context.Context, s *Session, timeline *timelineService, rows []contextNavigationNode) (int, error) {
	for i, a := range rows {
		if err := s.step(ctx, 1); err != nil {
			return 0, err
		}
		switch a.event.Kind {
		case NavigationNote, NavigationSound, NavigationListening, NavigationSegno, NavigationCoda:
		default:
			return i, nil
		}
		if a.position.State != EvidenceKnown {
			return i, nil
		}
		if a.event.Control.State != "" && a.event.Control.State != EvidenceAbsent && a.event.Control.State != EvidenceKnown {
			return i, nil
		}
		for _, b := range rows[i+1:] {
			if b.position.State != EvidenceKnown {
				return i, nil
			}
			order, err := timeline.State.positionOrder(ctx, s, a.position.Value, b.position.Value)
			if err != nil {
				return 0, err
			}
			if order == stateBefore {
				continue
			}
			if order == stateEqual && (navigationPriority(a.event.Kind) < navigationPriority(b.event.Kind) || a.event.Kind == NavigationNote && b.event.Kind == NavigationNote) {
				continue
			}
			return i, nil
		}
	}
	return len(rows), nil
}
func navigationRunRows(ctx context.Context, s *Session, timeline *timelineService, rows []contextNavigationNode) (NavigationContextResult, error) {
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(rows)), 1024)
	if err != nil {
		return NavigationContextResult{}, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return NavigationContextResult{}, err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }()
	initial, err := navigationAttachInstrumentUpdates(ctx, s, timeline, rows)
	if err != nil {
		return NavigationContextResult{}, err
	}
	events := make([]navigationEvent, len(rows))
	for i := range rows {
		if err = s.step(ctx, 1); err != nil {
			return NavigationContextResult{}, err
		}
		events[i] = rows[i].event
	}
	return runNavigation(ctx, s, events, initial)
}
