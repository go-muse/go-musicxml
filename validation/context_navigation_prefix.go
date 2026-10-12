package validation

import "context"

// Before an unresolved ordering/control edge, preserve the maximal initial
// linear segment whose precedence against every remaining event is proved.
// No unique target or paired region is inferred from this truncated domain.
func navigationLinearPrefix(ctx context.Context, s *Session, timeline *timelineService, rows []contextNavigationNode) (int, error) {
	if err := s.step(ctx, 1); err != nil {
		return 0, err
	}
	contained := true
	for _, row := range rows {
		if err := s.step(ctx, 1); err != nil {
			return 0, err
		}
		if row.position.State != EvidenceKnown {
			return 0, nil
		}
		member, err := timeline.State.frameMember(ctx, s, row.position.Value)
		if err != nil {
			return 0, err
		}
		contained = contained && member
	}
	// In proved frames, suffix minima replace all-pairs precedence checks.
	// Unconfined displacement keeps the exact conservative fallback below.
	var suffix []int
	if contained {
		free, err := contextRepeatScratch(ctx, s, len(rows), 8)
		if err != nil {
			return 0, err
		}
		defer free()
		suffix = make([]int, len(rows))
		for i := len(rows) - 1; i >= 0; i-- {
			if err := s.step(ctx, 1); err != nil {
				return 0, err
			}
			suffix[i] = i
			if i+1 == len(rows) {
				continue
			}
			j := suffix[i+1]
			order, err := timeline.State.positionOrder(ctx, s, rows[i].position.Value, rows[j].position.Value)
			if err != nil {
				return 0, err
			}
			if order == stateAfter || order == stateEqual && navigationRowPriority(rows[i]) > navigationRowPriority(rows[j]) {
				suffix[i] = j
			}
		}
	}
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
		candidates := rows[i+1:]
		if contained && i+1 < len(rows) {
			j := suffix[i+1]
			candidates = rows[j : j+1]
		}
		for _, b := range candidates {
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
			if order == stateEqual && (navigationRowPriority(a) < navigationRowPriority(b) || navigationRowPriority(a) == navigationRowPriority(b) && (!navigationControlKind(a.event.Kind) || !navigationControlKind(b.event.Kind))) {
				continue
			}
			return i, nil
		}
	}
	return len(rows), nil
}
func navigationRunRows(ctx context.Context, s *Session, timeline *timelineService, rows []contextNavigationNode, index *navigationInstrumentIndex) (NavigationContextResult, error) {
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(rows)), 1024)
	if err != nil {
		return NavigationContextResult{}, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return NavigationContextResult{}, err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }()
	initial, err := navigationAttachInstrumentUpdates(ctx, s, timeline, rows, index)
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
