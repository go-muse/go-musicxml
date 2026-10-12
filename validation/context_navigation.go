package validation

import (
	"context"
	"sort"
)

type contextNavigationService struct{ Final bool }
type contextNavigationNode struct {
	event    navigationEvent
	position Evidence[musicalPosition]
	part     Evidence[NodeRef]
	value    NavigationContextResult
}

func navigationKnownBool(v bool) Evidence[bool] {
	return Evidence[bool]{State: EvidenceKnown, Value: v}
}

func navigationBackwardApplicability(direction Evidence[string], ending Evidence[bool]) Evidence[bool] {
	if direction.State != EvidenceKnown {
		return stateUnavailable[string, bool](direction)
	}
	if direction.Value != "backward" {
		return navigationKnownBool(false)
	}
	if ending.State != EvidenceKnown {
		return ending
	}
	return navigationKnownBool(!ending.Value)
}

func navigationDefaultGuard(kind NavigationKind, raw Evidence[string]) (Evidence[string], bool) {
	if raw.State != EvidenceAbsent && raw.State != "" {
		return raw, false
	}
	switch kind {
	case NavigationDaCapo, NavigationDalSegno:
		return Evidence[string]{State: EvidenceKnown, Value: "1"}, true
	case NavigationToCoda:
		return Evidence[string]{State: EvidenceKnown, Value: "2"}, true
	}
	return Evidence[string]{State: EvidenceAbsent}, false
}

func navigationSoundKind(raw navigationContext) (NavigationKind, Evidence[string], Evidence[bool]) {
	kind, label, count := NavigationSound, Evidence[string]{State: EvidenceAbsent}, 0
	status := navigationKnownBool(true)
	for _, v := range []struct {
		kind NavigationKind
		raw  Evidence[string]
	}{{NavigationSegno, raw.Segno}, {NavigationCoda, raw.Coda}, {NavigationDalSegno, raw.Dalsegno}, {NavigationToCoda, raw.Tocoda}, {NavigationFine, raw.Fine}} {
		if v.raw.State == EvidenceKnown {
			kind, label = v.kind, v.raw
			count++
		} else if v.raw.State != EvidenceAbsent && v.raw.State != "" {
			status = stateMergeGap(status, stateUnavailable[string, bool](v.raw))
		}
	}
	for _, v := range []struct {
		kind NavigationKind
		raw  Evidence[bool]
	}{{NavigationDaCapo, raw.Dacapo}, {NavigationForward, raw.ForwardRepeat}} {
		if v.raw.State == EvidenceKnown && v.raw.Value {
			kind = v.kind
			count++
		} else if v.raw.State != EvidenceAbsent && v.raw.State != "" && v.raw.State != EvidenceKnown {
			status = stateMergeGap(status, v.raw)
		}
	}
	if count > 1 {
		status = stateMergeGap(status, stateUnknown[bool]("navigation-multiple-directives-unresolved"))
	}
	return kind, label, status
}

func navigationAppend(ctx context.Context, s *Session, rows *[]contextNavigationNode, row contextNavigationNode) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if err := s.retain(6144); err != nil {
		return err
	}
	*rows = append(*rows, row)
	return nil
}

func navigationReadRows(ctx context.Context, s *Session, p *contextPlan, timeline *timelineService) ([]contextNavigationNode, error) {
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	var rows []contextNavigationNode
	for _, id := range s.sequence {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		node := s.nodes[id]
		if r.matches(node, contextNavListening) {
			fact, e := contextRepeatOrigin(ctx, s, timeline, node)
			if e != nil {
				return nil, e
			}
			updates, e := navigationReadListening(r, node, fact.Part)
			if e != nil {
				return nil, e
			}
			row := contextNavigationNode{event: navigationEvent{Source: NodeRef{s.documentID, id}, Kind: NavigationListening, Listening: updates}, part: fact.Part, position: fact.Position, value: NavigationContextResult{Kind: NavigationListening, Part: fact.Part, Listening: updates, TargetsDistinct: navigationListeningDistinct(r, node, updates)}}
			if fact.Position.State == EvidenceKnown {
				v := fact.Position.Value
				row.value.Position = Evidence[ContextPosition]{State: EvidenceKnown, Value: ContextPosition{Part: v.Part, Measure: v.Measure, MeasureOrdinal: v.MeasureOrdinal, Offset: v.Offset, UnprovedFrame: v.UnprovedFrame}}
			} else {
				row.value.Position = stateUnavailable[musicalPosition, ContextPosition](fact.Position)
			}
			if e = navigationAppend(ctx, s, &rows, row); e != nil {
				return nil, e
			}
		}
		if note, ok := timeline.Playback.Notes[id]; ok {
			noteListening, recommended, distinct, e := navigationReadNoteListening(r, node, note.Origin.Part)
			if e != nil {
				return nil, e
			}
			play, e := performancePlayInstructions(ctx, s, note.Play)
			if e != nil {
				return nil, e
			}
			row := contextNavigationNode{event: navigationEvent{Source: note.Origin.Source, Kind: NavigationNote, NoteListening: noteListening, Play: play, Guard: note.TimeOnly, NoteDynamics: note.Dynamics, EndDynamics: note.EndDynamics, NotePizzicato: note.Pizzicato}, part: note.Origin.Part, position: note.Origin.Position, value: NavigationContextResult{Kind: NavigationNote, NoteListening: noteListening, RecommendedAssess: recommended, TargetsDistinct: distinct, Play: play, Part: note.Origin.Part, TimeOnly: note.TimeOnly}}
			if err := navigationAppend(ctx, s, &rows, row); err != nil {
				return nil, err
			}
		}
		if sound, ok := timeline.Playback.Sounds[id]; ok {
			play, e := performancePlayInstructions(ctx, s, sound.Play)
			if e != nil {
				return nil, e
			}
			kind, label, available := navigationSoundKind(sound.Navigation)
			guard, defaulted := navigationDefaultGuard(kind, sound.TimeOnly)
			row := contextNavigationNode{event: navigationEvent{Source: sound.Origin.Source, Kind: kind, Play: play, Guard: guard, Tempo: sound.Tempo, Dynamics: sound.Dynamics, Pizzicato: sound.Pizzicato, Control: available}, part: sound.Origin.Part, position: sound.Origin.Position, value: NavigationContextResult{Kind: kind, Play: play, Part: sound.Origin.Part, Label: label, TimeOnly: guard, Defaulted: defaulted, Guard: stateUnknown[bool]("navigation-pass-domain-unavailable")}}
			if kind == NavigationFine {
				if label.State == EvidenceKnown && label.Value == "yes" {
					row.value.DurationUnchanged = navigationKnownBool(true)
				} else if label.State == EvidenceKnown {
					row.value.DurationUnchanged = stateUnknown[bool]("navigation-fine-duration-units-unresolved")
					row.value.FineDuration = stateUnknown[Decimal]("navigation-fine-duration-units-unresolved")
				} else {
					row.value.DurationUnchanged = stateUnavailable[string, bool](label)
					row.value.FineDuration = stateUnavailable[string, Decimal](label)
				}
			}
			if err := navigationAppend(ctx, s, &rows, row); err != nil {
				return nil, err
			}
		}
		if barline, ok := timeline.Playback.Barlines[id]; ok {
			for _, marker := range []struct {
				kind    NavigationKind
				label   Evidence[string]
				present Evidence[bool]
			}{{NavigationSegno, barline.Navigation.Segno, barline.Navigation.SegnoElement}, {NavigationCoda, barline.Navigation.Coda, barline.Navigation.CodaElement}} {
				if marker.label.State == EvidenceAbsent {
					continue
				}
				active := playbackMarker(marker.label, marker.present, true)
				row := contextNavigationNode{event: navigationEvent{Source: barline.Origin.Source, Kind: marker.kind, Control: active}, part: barline.Origin.Part, position: barline.Origin.Position, value: NavigationContextResult{Kind: marker.kind, Part: barline.Origin.Part, Label: marker.label, Guard: active}}
				// Raw inactive marker facts are retained, but do not become target nodes.
				if active.State == EvidenceKnown && !active.Value {
					row.event.Kind = NavigationSound
				}
				if err := navigationAppend(ctx, s, &rows, row); err != nil {
					return nil, err
				}
			}
		}
		if r.matches(node, contextBarlineRepeat) || r.matches(node, contextBarlineEnding) {
			fact, err := contextRepeatOrigin(ctx, s, timeline, node)
			if err != nil {
				return nil, err
			}
			row := contextNavigationNode{part: fact.Part, position: fact.Position, event: navigationEvent{Source: NodeRef{s.documentID, id}}, value: NavigationContextResult{Part: fact.Part}}
			if r.matches(node, contextBarlineRepeat) {
				direction := stateText(r.attribute(node, contextRepeatDirection))
				row.value.Direction = direction
				row.value.Times = stateInteger(r.attribute(node, contextRepeatTimes))
				row.value.AfterJump = stateYesNo(r.attribute(node, contextRepeatAfterJump))
				row.event.Kind = NavigationBackward
				if direction.State == EvidenceKnown && direction.Value == "forward" {
					row.event.Kind = NavigationForward
				}
				row.value.Kind = row.event.Kind
				row.event.Times = row.value.Times
				row.event.AfterJump = row.value.AfterJump
				row.value.AttributesActive = navigationBackwardApplicability(direction, stateUnknown[bool]("navigation-ending-membership-unavailable"))
				if direction.State != EvidenceKnown {
					row.event.Control = stateUnavailable[string, bool](direction)
				}
			} else {
				row.event.Kind = NavigationEnding
				row.value.Kind = NavigationEnding
				row.value.Direction = stateText(r.attribute(node, contextEndingType))
				row.value.EndingNumber = stateText(r.attribute(node, contextEndingNumber))
				if row.value.EndingNumber.State == EvidenceKnown && row.value.EndingNumber.Value == "" {
					row.value.Guard = stateUnknown[bool]("navigation-ending-number-unknown")
				}
			}
			if err = navigationAppend(ctx, s, &rows, row); err != nil {
				return nil, err
			}
		}
	}
	return rows, r.err
}

func navigationPriority(kind NavigationKind) int {
	if kind == NavigationForward {
		return -2
	}
	if kind == NavigationSegno || kind == NavigationCoda {
		return -1
	}
	if kind == NavigationNote {
		return 1
	}
	return 0
}

// Sorting is a candidate arrangement only. Every adjacent relation is checked
// afterward using the exact partial-order service; an unresolved relation never
// becomes a deterministic playback order from the sorting fallback.
func navigationOrder(ctx context.Context, s *Session, timeline *timelineService, rows []contextNavigationNode) (Evidence[bool], error) {
	available := navigationKnownBool(true)
	for _, row := range rows {
		if err := s.step(ctx, 1); err != nil {
			return Evidence[bool]{}, err
		}
		if row.position.State != EvidenceKnown {
			available = stateMergeGap(available, stateUnavailable[musicalPosition, bool](row.position))
		}
	}
	if available.State != EvidenceKnown {
		return available, nil
	}
	var compareErr error
	sort.SliceStable(rows, func(i, j int) bool {
		if compareErr != nil {
			return false
		}
		a, b := rows[i], rows[j]
		order, err := timeline.State.positionOrder(ctx, s, a.position.Value, b.position.Value)
		if err != nil {
			compareErr = err
			return false
		}
		if order == stateBefore {
			return true
		}
		if order == stateAfter {
			return false
		}
		if order == stateEqual {
			return navigationPriority(a.event.Kind) < navigationPriority(b.event.Kind)
		}
		available = stateMergeGap(available, stateUnknown[bool]("navigation-position-order-unresolved"))
		return false
	})
	if compareErr != nil {
		return Evidence[bool]{}, compareErr
	}
	for i := 1; i < len(rows); i++ {
		order, err := timeline.State.positionOrder(ctx, s, rows[i-1].position.Value, rows[i].position.Value)
		if err != nil {
			return Evidence[bool]{}, err
		}
		if order != stateBefore && order != stateEqual {
			available = stateMergeGap(available, stateUnknown[bool]("navigation-position-order-unresolved"))
		}
		if order == stateEqual && navigationPriority(rows[i-1].event.Kind) == navigationPriority(rows[i].event.Kind) && rows[i].event.Kind != NavigationNote {
			available = stateMergeGap(available, stateUnknown[bool]("navigation-simultaneous-control-unresolved"))
		}
	}
	return available, nil
}

func navigationBuildEdges(ctx context.Context, s *Session, rows []contextNavigationNode) error {
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(rows)), 16)
	if err != nil {
		return err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }()
	stack := make([]int, 0, len(rows))
	hasRepeat, hasEnding, hasDS := false, false, false
	for _, row := range rows {
		if err = s.step(ctx, 1); err != nil {
			return err
		}
		hasRepeat = hasRepeat || row.event.Kind == NavigationForward || row.event.Kind == NavigationBackward
		hasEnding = hasEnding || row.event.Kind == NavigationEnding
		hasDS = hasDS || row.event.Kind == NavigationDalSegno
	}
	endingDepth := 0
	endingAvailable := navigationKnownBool(true)
	for i := range rows {
		if err = s.step(ctx, 1); err != nil {
			return err
		}
		row := &rows[i]
		// Movement pass is proved for the simple D.C. movement with no local
		// repeat/ending or D.S. submovement. More complex bindings stay unavailable.
		if !hasRepeat && !hasEnding && !hasDS {
			row.event.PassDomain = navigationMovementPass
		}
		switch row.event.Kind {
		case NavigationForward:
			stack = append(stack, i)
		case NavigationEnding:
			if row.value.Direction.State != EvidenceKnown {
				endingAvailable = stateUnavailable[string, bool](row.value.Direction)
			} else if row.value.Direction.Value == "start" {
				endingDepth++
			} else if endingDepth > 0 {
				endingDepth--
			} else {
				endingAvailable = stateUnknown[bool]("navigation-ending-boundary-unresolved")
			}
		case NavigationBackward:
			ending := endingAvailable
			if ending.State == EvidenceKnown {
				ending.Value = endingDepth > 0
				// A stop/discontinue on this same barline owns its trailing repeat even
				// though its ending interval is otherwise half-open.
				for j := range rows {
					if err = s.step(ctx, 1); err != nil {
						return err
					}
					if rows[j].event.Kind == NavigationEnding && s.nodes[rows[j].event.Source.Node] != nil && s.nodes[row.event.Source.Node] != nil && s.nodes[rows[j].event.Source.Node].header.Parent == s.nodes[row.event.Source.Node].header.Parent {
						ending.Value = true
					}
				}
			}
			row.value.AttributesActive = navigationBackwardApplicability(row.value.Direction, ending)
			if row.value.AttributesActive.State != EvidenceKnown || !row.value.AttributesActive.Value {
				row.event.Control = stateUnknown[bool]("navigation-ending-repeat-realization-unresolved")
			}
			if len(stack) == 0 {
				row.event.Target = stateUnknown[int]("navigation-repeat-start-unavailable")
			} else {
				row.event.Target = Evidence[int]{State: EvidenceKnown, Value: stack[len(stack)-1]}
				row.value.Target = Evidence[NodeRef]{State: EvidenceKnown, Value: rows[stack[len(stack)-1]].event.Source}
				stack = stack[:len(stack)-1]
			}
		case NavigationDaCapo:
			if len(rows) > 0 {
				row.event.Target = Evidence[int]{State: EvidenceKnown, Value: 0}
				row.value.Target = Evidence[NodeRef]{State: EvidenceKnown, Value: rows[0].event.Source}
			}
		case NavigationDalSegno, NavigationToCoda:
			kind := NavigationSegno
			if row.event.Kind == NavigationToCoda {
				kind = NavigationCoda
			}
			found := -1
			ambiguous := false
			if row.value.Label.State != EvidenceKnown {
				row.event.Target = stateUnavailable[string, int](row.value.Label)
				break
			}
			for j := range rows {
				if err = s.step(ctx, 1); err != nil {
					return err
				}
				candidate := rows[j]
				if candidate.event.Kind != kind {
					continue
				}
				if candidate.value.Label.State != EvidenceKnown {
					ambiguous = true
					continue
				}
				if err = s.step(ctx, int64(len(candidate.value.Label.Value)+len(row.value.Label.Value))); err != nil {
					return err
				}
				if candidate.value.Label.Value != row.value.Label.Value {
					continue
				}
				if found >= 0 {
					ambiguous = true
				}
				found = j
			}
			if ambiguous || found < 0 || row.event.Kind == NavigationDalSegno && found >= i || row.event.Kind == NavigationToCoda && found <= i {
				row.event.Target = stateUnknown[int]("navigation-target-unresolved")
				row.value.Target = stateUnknown[NodeRef]("navigation-target-unresolved")
			} else {
				row.event.Target = Evidence[int]{State: EvidenceKnown, Value: found}
				row.value.Target = Evidence[NodeRef]{State: EvidenceKnown, Value: rows[found].event.Source}
			}
		}
	}
	// Establish the domain of a single D.C. movement or single D.S. section.
	// Events beyond its departure (including a coda reached only on re-entry)
	// do not inherit the movement epoch as their own section pass.
	jumpIndex, jumpCount := -1, 0
	for i := range rows {
		if err = s.step(ctx, 1); err != nil {
			return err
		}
		if rows[i].event.Kind == NavigationDaCapo || rows[i].event.Kind == NavigationDalSegno {
			jumpIndex = i
			jumpCount++
		}
	}
	if !hasRepeat && !hasEnding && jumpCount == 1 && rows[jumpIndex].event.Target.State == EvidenceKnown {
		start := rows[jumpIndex].event.Target.Value
		for i := range rows {
			if err = s.step(ctx, 1); err != nil {
				return err
			}
			rows[i].event.PassDomain = navigationUnknownPass
			if i >= start && i <= jumpIndex {
				rows[i].event.PassDomain = navigationMovementPass
			}
		}
	} else if jumpCount > 1 {
		for i := range rows {
			rows[i].event.PassDomain = navigationUnknownPass
		}
	}
	// A single ordinary region supplies a section pass; nested selector domains
	// remain distinct and therefore unknown unless the selector is unrestricted.
	for i := range rows {
		if err = s.step(ctx, 1); err != nil {
			return err
		}
		if hasRepeat && !hasEnding && !hasDS && rows[i].event.PassDomain == navigationUnknownPass {
			rows[i].event.PassDomain = navigationSingleRepeatPass
		}
	}
	return nil
}

func evaluateContextNavigation(ctx context.Context, s *Session, n *factNode, key RuleKey, p *contextPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	service, err := s.newContextService(ctx, key)
	if err != nil {
		return Decision{}, false, err
	}
	if service.Navigation != nil && service.Navigation.Final {
		return Decision{Outcome: OutcomePass}, false, nil
	}
	timeline := s.timeline[p.Timeline]
	if timeline == nil {
		timeline = &timelineService{}
	}
	if err = navigationPublishGraphics(ctx, s, p, key, timeline); err != nil {
		return Decision{}, false, err
	}
	rows, err := navigationReadRows(ctx, s, p, timeline)
	if err != nil {
		return Decision{}, false, err
	}
	// Part grouping remains symbolic and sparse; printed measure labels never
	// participate in identity. Source-qualified NodeRef keys prevent aliasing.
	if err = s.retain(256); err != nil {
		return Decision{}, false, err
	}
	parts := make(map[NodeRef][]contextNavigationNode)
	for _, row := range rows {
		if err = s.step(ctx, 1); err != nil {
			return Decision{}, false, err
		}
		if row.part.State != EvidenceKnown {
			row.value.Complete = stateUnavailable[NodeRef, bool](row.part)
			if err = s.publishContextResult(ctx, ContextResult{Provider: key, Subject: row.event.Source, Property: ContextPropertyNavigation, Navigation: &row.value}); err != nil {
				return Decision{}, false, err
			}
			continue
		}
		if err = s.retain(8192); err != nil {
			return Decision{}, false, err
		}
		parts[row.part.Value] = append(parts[row.part.Value], row)
	}
	// Iterate first-encounter order rather than Go map order for deterministic
	// bounded output and cancellation boundaries.
	for _, row := range rows {
		if row.part.State != EvidenceKnown {
			continue
		}
		partRows, ok := parts[row.part.Value]
		if !ok {
			continue
		}
		delete(parts, row.part.Value)
		available, e := navigationOrder(ctx, s, timeline, partRows)
		if e != nil {
			return Decision{}, false, e
		}
		traversal := NavigationContextResult{Kind: NavigationTraversal, Part: row.part, Complete: available}
		if available.State == EvidenceKnown {
			if e = navigationBuildEdges(ctx, s, partRows); e != nil {
				return Decision{}, false, e
			}
			bytes, e := rationalCheckedMul(BudgetBufferedBytes, int64(len(partRows)), 1024)
			if e != nil {
				return Decision{}, false, e
			}
			if e = s.account.Charge(BudgetBufferedBytes, bytes); e != nil {
				return Decision{}, false, e
			}
			events := make([]navigationEvent, len(partRows))
			for i := range partRows {
				events[i] = partRows[i].event
			}
			initial, e := navigationAttachInstrumentUpdates(ctx, s, timeline, partRows)
			if e != nil {
				_ = s.account.Release(BudgetBufferedBytes, bytes)
				return Decision{}, false, e
			}
			for i := range partRows {
				events[i] = partRows[i].event
			}
			traversal, e = runNavigation(ctx, s, events, initial)
			_ = s.account.Release(BudgetBufferedBytes, bytes)
			if e != nil {
				return Decision{}, false, e
			}
			traversal.Part = row.part
		}
		for i := range partRows {
			v := &partRows[i]
			if err = s.publishContextResult(ctx, ContextResult{Provider: key, Subject: v.event.Source, Property: ContextPropertyNavigation, Navigation: &v.value}); err != nil {
				return Decision{}, false, err
			}
		}
		if err = s.publishContextResult(ctx, ContextResult{Provider: key, Subject: row.part.Value, Property: ContextPropertyNavigationTraversal, Navigation: &traversal}); err != nil {
			return Decision{}, false, err
		}
	}
	if err = s.retain(128); err != nil {
		return Decision{}, false, err
	}
	service.Navigation = &contextNavigationService{Final: true}
	return Decision{Outcome: OutcomePass}, false, nil
}
