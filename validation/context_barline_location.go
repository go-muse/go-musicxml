package validation

import "context"

const planBarlineLocation planKind = "musicxml-barline-location"

// NewBarlineLocationPlan checks musical position and ordered musical-data
// placement against a structural origin and a bounded measure-end certificate.
// First/last XML-child recommendations do not themselves establish violations,
// and nominal meter capacity does not supply a measure-end certificate.
func NewBarlineLocationPlan(spec ContextPlanSpec) (Plan, error) {
	if !catalogKey(spec.TimelineProvider) {
		return Plan{}, contract("barline timeline", "exact provider revision required")
	}
	return newContextPlan(planBarlineLocation, spec)
}

func evaluateBarlineLocation(ctx context.Context, s *Session, n *factNode, p *contextPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	if err := s.step(ctx, 1+int64(len(p.Timeline.ID))); err != nil {
		return Decision{}, false, err
	}
	service := s.timeline[p.Timeline]
	if service == nil || !service.Final {
		return Decision{Outcome: OutcomeUnknown, Reason: "barline-timeline-unavailable"}, false, nil
	}
	bar, ok := service.Playback.Barlines[n.header.ID]
	if !ok {
		return Decision{Outcome: OutcomeUnknown, Reason: "barline-context-unavailable"}, false, nil
	}
	if bar.Location.State != EvidenceKnown {
		return localGap(bar.Location), false, nil
	}
	fact, ok := service.Facts[n.header.ID]
	if !ok {
		return Decision{Outcome: OutcomeUnknown, Reason: "barline-position-unavailable"}, false, nil
	}
	if fact.Origin.State != EvidenceKnown {
		return localGap(fact.Origin), false, nil
	}
	pos := fact.Origin.Value
	if !pos.Offset.Valid() {
		return Decision{Outcome: OutcomeUnknown, Reason: "barline-position-unavailable"}, false, nil
	}
	location := bar.Location.Value
	if location != "left" && location != "middle" && location != "right" {
		return Decision{Outcome: OutcomeUnknown, Reason: "barline-location-unavailable"}, false, nil
	}
	sign := pos.Offset.value.Sign()
	if location == "left" && sign != 0 {
		return barlineLocationDecision(false), false, nil
	}
	if location == "middle" && sign <= 0 || location == "right" && sign < 0 {
		return barlineLocationDecision(false), false, nil
	}
	provider := s.catalog.plans[s.catalog.rules[p.Timeline].Predicate].timeline
	if provider == nil {
		return Decision{}, false, contract("barline timeline", "selected timeline plan required")
	}
	content := s.nodes[n.header.Parent]
	if location == "left" {
		earlier, err := barlineMovementSide(ctx, s, provider.Bindings, content, n.header.ID, true)
		if err != nil {
			return Decision{}, false, err
		}
		if earlier.State != EvidenceKnown {
			return localGap(earlier), false, nil
		}
		if earlier.Value {
			return Decision{Outcome: OutcomeUnknown, Reason: "barline-left-placement-unestablished"}, false, nil
		}
		return barlineLocationDecision(true), false, nil
	}
	boundary, decisive, err := barlineLocationBoundary(ctx, s, service, provider.Bindings, content, n.header.ID, pos, location)
	if err != nil || decisive.Outcome != "" {
		return decisive, false, err
	}
	if boundary.State != EvidenceKnown {
		return localGap(boundary), false, nil
	}
	cmp, err := s.rationalCmp(ctx, pos.Offset, boundary.Value)
	if err != nil {
		return Decision{}, false, err
	}
	return barlineLocationDecision(location == "right" && cmp == 0 || location == "middle" && cmp < 0), false, nil
}

func barlineLocationDecision(pass bool) Decision {
	if pass {
		return Decision{Outcome: OutcomePass}
	}
	return Decision{Outcome: OutcomeFail, Reason: "barline-location-mismatch"}
}

// barlineMovementSide checks note/backup/forward placement without treating
// metadata as movement or inspecting unrelated scalar values. The exact subject
// must occur in the ordered stream; unknown declarations are never zero-time
// assumptions. A left-side proof stops at the subject, so an unrelated suffix
// cannot erase the independently known beginning. No retained storage is needed.
func barlineMovementSide(ctx context.Context, s *Session, b *timelineBindings, content *factNode, subject NodeID, before bool) (Evidence[bool], error) {
	if err := s.step(ctx, 1); err != nil {
		return Evidence[bool]{}, err
	}
	gap := timelineHierarchyAvailability(content)
	if gap.State != EvidenceKnown || !gap.Value {
		return gap, nil
	}
	if content.end.Order.Value != OrderSource && content.end.Order.Value != OrderModelContent {
		return stateUnknown[bool]("barline-measure-order-unavailable"), nil
	}
	r := &stateReader{ctx: ctx, s: s, b: b}
	found, movement := false, false
	result := func() Evidence[bool] {
		if movement {
			return Evidence[bool]{State: EvidenceKnown, Value: true}
		}
		if gap.State != EvidenceKnown || !gap.Value {
			return gap
		}
		return Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	for _, event := range content.children {
		if err := s.step(ctx, 1); err != nil {
			return Evidence[bool]{}, err
		}
		if event.header.ID == subject {
			found = true
			if before {
				return result(), nil
			}
			continue
		}
		if before == found {
			continue
		}
		if event.header.Declaration.State != EvidenceKnown {
			gap = timelineMergeAvailability(gap, timelineGapAs[DeclarationID, bool](event.header.Declaration, "barline-event-declaration-unavailable"))
			continue
		}
		moving := r.matches(event, timelineRoleNote) || r.matches(event, timelineRoleBackup) || r.matches(event, timelineRoleForward)
		if r.err != nil {
			return Evidence[bool]{}, r.err
		}
		movement = movement || moving
		if !before && movement {
			return Evidence[bool]{State: EvidenceKnown, Value: true}, nil
		}
	}
	if !found {
		return stateUnknown[bool]("barline-placement-unavailable"), nil
	}
	return result(), nil
}

// Only this predicate consumes these certificates. Nothing is written into
// frameBounds, UnprovedFrame, cross-measure ordering or repeat partEnd.
// lower is rebuilt without barlines, so the queried boundary cannot witness
// itself, directly or through a later excursion's containment.
func barlineLocationBoundary(ctx context.Context, s *Session, service *timelineService, b *timelineBindings, content *factNode, subject NodeID, at musicalPosition, location string) (Evidence[Rational], Decision, error) {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !at.Offset.Valid() {
		return stateUnknown[Rational]("barline-position-unavailable"), Decision{}, nil
	}
	gap := timelineHierarchyAvailability(content)
	if gap.State != EvidenceKnown || !gap.Value {
		return timelineGapAs[bool, Rational](gap, "barline-measure-enumeration-incomplete"), Decision{}, nil
	}
	if content.end.Order.Value != OrderSource && content.end.Order.Value != OrderModelContent {
		return stateUnknown[Rational]("barline-measure-order-unavailable"), Decision{}, nil
	}
	// Earlier voices can extend beyond a final voice's cursor. A later lower
	// bound only disproves right when a known movement follows this barline.
	// Otherwise finish the bounded end proof, retaining unequal-voice Unknown.
	rightInterior := false
	if location == "right" {
		later, err := barlineMovementSide(ctx, s, b, content, subject, false)
		if err != nil {
			return Evidence[Rational]{}, Decision{}, err
		}
		if later.State != EvidenceKnown {
			return timelineGapAs[bool, Rational](later, "barline-placement-unavailable"), Decision{}, nil
		}
		rightInterior = later.Value
	}
	lower, maximum, noteMaximum, excursionMaximum := timelineZero, timelineZero, timelineZero, timelineZero
	candidate := stateUnknown[Rational]("barline-measure-end-unestablished")
	last := timelineMovementOther
	prefix, haveNote, haveExcursion := true, false, false
	merge := func(e Evidence[bool]) {
		if e.State != EvidenceKnown || !e.Value {
			gap = timelineMergeAvailability(gap, e)
		}
	}
	max := func(dst *Rational, value Rational) error {
		c, e := s.rationalCmp(ctx, value, *dst)
		if e == nil && c > 0 {
			*dst = value
		}
		return e
	}
	contained := func(value Rational) (bool, error) {
		if value.value.Sign() < 0 {
			return false, nil
		}
		c, e := s.rationalCmp(ctx, value, lower)
		return c <= 0, e
	}
	for _, event := range content.children {
		if err := s.step(ctx, 1); err != nil {
			return Evidence[Rational]{}, Decision{}, err
		}
		if event.header.Declaration.State != EvidenceKnown {
			merge(timelineGapAs[DeclarationID, bool](event.header.Declaration, "barline-event-declaration-unavailable"))
			prefix = false
			continue
		}
		f, ok := service.Facts[event.header.ID]
		if !ok {
			merge(stateUnknown[bool]("barline-event-timing-unavailable"))
			prefix = false
			continue
		}
		note, backup, forward, barline := r.matches(event, timelineRoleNote), r.matches(event, timelineRoleBackup), r.matches(event, timelineRoleForward), r.matches(event, timelineRoleBarline)
		if r.err != nil {
			return Evidence[Rational]{}, Decision{}, r.err
		}
		chord, grace := Evidence[bool]{State: EvidenceKnown}, Evidence[bool]{State: EvidenceKnown}
		if note {
			_, chord = r.child(event, timelineRoleNoteChord)
			chord = noteTimingPresence(chord)
			_, grace = r.child(event, timelineRoleNoteGrace)
			grace = noteTimingPresence(grace)
			if r.err != nil {
				return Evidence[Rational]{}, Decision{}, r.err
			}
			if chord.State != EvidenceKnown || grace.State != EvidenceKnown {
				if chord.State != EvidenceKnown {
					merge(timelineGapAs[bool, bool](chord, "barline-note-branch-unavailable"))
				}
				if grace.State != EvidenceKnown {
					merge(timelineGapAs[bool, bool](grace, "barline-note-branch-unavailable"))
				}
				prefix = false
				continue
			}
		}
		if f.Origin.State != EvidenceKnown {
			merge(timelineGapAs[musicalPosition, bool](f.Origin, "barline-event-position-unavailable"))
			prefix = false
			continue
		}
		origin := f.Origin.Value
		if !origin.Offset.Valid() {
			merge(stateUnknown[bool]("barline-event-position-unavailable"))
			prefix = false
			continue
		}
		if err := s.step(ctx, int64(len(origin.Part.DocumentID)+len(origin.Measure.DocumentID)+len(at.Part.DocumentID)+len(at.Measure.DocumentID))); err != nil {
			return Evidence[Rational]{}, Decision{}, err
		}
		if frameOf(origin) != frameOf(at) {
			merge(stateUnknown[bool]("barline-event-frame-unavailable"))
			prefix = false
			continue
		}
		if origin.Offset.value.Sign() < 0 {
			merge(stateUnknown[bool]("barline-negative-cursor-excursion"))
			prefix = false
		}
		if err := max(&maximum, origin.Offset); err != nil {
			return Evidence[Rational]{}, Decision{}, err
		}
		// Chords reuse the principal onset and cannot witness the current cursor.
		if !chord.Value {
			if !prefix {
				var err error
				prefix, err = contained(origin.Offset)
				if err != nil {
					return Evidence[Rational]{}, Decision{}, err
				}
			}
			if prefix && !barline {
				if err := max(&lower, origin.Offset); err != nil {
					return Evidence[Rational]{}, Decision{}, err
				}
				cmp, err := s.rationalCmp(ctx, at.Offset, lower)
				if err != nil {
					return Evidence[Rational]{}, Decision{}, err
				}
				if cmp < 0 && (location == "middle" || rightInterior) {
					return Evidence[Rational]{}, barlineLocationDecision(location == "middle"), nil
				}
			}
		}
		if !note && !backup && !forward {
			continue
		}
		if note && chord.Value && f.Principal.State != EvidenceKnown {
			merge(timelineGapAs[NodeRef, bool](f.Principal, "chord-principal-unavailable"))
			continue
		}
		if note && grace.Value {
			continue
		}
		if f.Duration.State != EvidenceKnown {
			merge(timelineGapAs[Rational, bool](f.Duration, "barline-duration-unavailable"))
			if !chord.Value {
				prefix = false
			}
			continue
		}
		if f.Units.State != EvidenceKnown {
			merge(timelineGapAs[Decimal, bool](f.Units, "barline-divisions-unavailable"))
			if !chord.Value {
				prefix = false
			}
			continue
		}
		if !f.Units.Value.Valid() || f.Units.Value.value.Sign() <= 0 {
			merge(stateUnknown[bool]("barline-positive-divisions-unavailable"))
			if !chord.Value {
				prefix = false
			}
			continue
		}
		if !f.Duration.Value.Valid() || f.Duration.Value.value.Sign() <= 0 {
			merge(stateUnknown[bool]("barline-positive-duration-unavailable"))
			if !chord.Value {
				prefix = false
			}
			continue
		}
		if note && chord.Value {
			if f.PrincipalDuration.State != EvidenceKnown {
				merge(timelineGapAs[Rational, bool](f.PrincipalDuration, "chord-principal-duration-unavailable"))
				continue
			}
			if !f.PrincipalDuration.Value.Valid() {
				merge(stateUnknown[bool]("chord-principal-duration-unavailable"))
				continue
			}
			cmp, err := s.rationalCmp(ctx, f.Duration.Value, f.PrincipalDuration.Value)
			if err != nil {
				return Evidence[Rational]{}, Decision{}, err
			}
			if cmp > 0 {
				e, err := barlineChordFailure(ctx, s, event)
				if err != nil {
					return Evidence[Rational]{}, Decision{}, err
				}
				merge(e)
			}
		}
		var end Rational
		var err error
		if note && !chord.Value {
			if f.End.State != EvidenceKnown {
				merge(timelineGapAs[musicalPosition, bool](f.End, "barline-note-end-unavailable"))
				prefix = false
				continue
			}
			end = f.End.Value.Offset
			if !end.Valid() {
				merge(stateUnknown[bool]("barline-note-end-unavailable"))
				prefix = false
				continue
			}
		} else if backup {
			end, err = s.rationalSub(ctx, origin.Offset, f.Duration.Value)
		} else {
			end, err = s.rationalAdd(ctx, origin.Offset, f.Duration.Value)
		}
		if err != nil {
			return Evidence[Rational]{}, Decision{}, err
		}
		if end.value.Sign() < 0 {
			merge(stateUnknown[bool]("barline-negative-cursor-excursion"))
		}
		if err = max(&maximum, end); err != nil {
			return Evidence[Rational]{}, Decision{}, err
		}
		if note {
			haveNote = true
			if err = max(&noteMaximum, end); err != nil {
				return Evidence[Rational]{}, Decision{}, err
			}
			if !chord.Value {
				candidate = Evidence[Rational]{State: EvidenceKnown, Value: end}
				last = timelineMovementNote
			}
		} else {
			haveExcursion = true
			if err = max(&excursionMaximum, origin.Offset); err != nil {
				return Evidence[Rational]{}, Decision{}, err
			}
			if err = max(&excursionMaximum, end); err != nil {
				return Evidence[Rational]{}, Decision{}, err
			}
			prefix, err = contained(end)
			if err != nil {
				return Evidence[Rational]{}, Decision{}, err
			}
			candidate = Evidence[Rational]{State: EvidenceKnown, Value: end}
			last = timelineMovementBackup
			if forward {
				last = timelineMovementForward
			}
		}
	}
	if gap.State != EvidenceKnown || !gap.Value {
		return timelineGapAs[bool, Rational](gap, "barline-extent-coverage-unavailable"), Decision{}, nil
	}
	if !haveNote || candidate.State != EvidenceKnown {
		return stateUnknown[Rational]("barline-measure-end-unestablished"), Decision{}, nil
	}
	if haveExcursion {
		cmp, err := s.rationalCmp(ctx, excursionMaximum, lower)
		if err != nil {
			return Evidence[Rational]{}, Decision{}, err
		}
		if cmp > 0 {
			return stateUnknown[Rational]("barline-measure-end-unproved-excursion"), Decision{}, nil
		}
	}
	cmp, err := s.rationalCmp(ctx, maximum, candidate.Value)
	if err != nil {
		return Evidence[Rational]{}, Decision{}, err
	}
	if cmp > 0 {
		return stateUnknown[Rational]("barline-measure-end-unequal-voices"), Decision{}, nil
	}
	if location == "right" && !rightInterior {
		cmp, err = s.rationalCmp(ctx, at.Offset, candidate.Value)
		if err != nil {
			return Evidence[Rational]{}, Decision{}, err
		}
		if cmp < 0 {
			return stateUnknown[Rational]("barline-measure-end-unequal-voices"), Decision{}, nil
		}
	}
	if last == timelineMovementNote {
		return candidate, Decision{}, nil
	}
	if last == timelineMovementForward {
		cmp, err = s.rationalCmp(ctx, noteMaximum, candidate.Value)
		if err != nil {
			return Evidence[Rational]{}, Decision{}, err
		}
		if cmp == 0 {
			return candidate, Decision{}, nil
		}
	}
	return stateUnknown[Rational]("barline-measure-end-unestablished"), Decision{}, nil
}

// When the selected chord predicate already owns the violation, preserve its
// decision reference instead of manufacturing a second barline violation.
func barlineChordFailure(ctx context.Context, s *Session, n *factNode) (Evidence[bool], error) {
	if s.catalog != nil {
		for _, key := range s.catalog.orders[s.profile.ID] {
			if err := s.step(ctx, 1); err != nil {
				return Evidence[bool]{}, err
			}
			rule := s.catalog.rules[key]
			if s.catalog.plans[rule.Predicate].kind != planChordDuration {
				continue
			}
			if err := s.evaluate(ctx, n, key, true); err != nil {
				return Evidence[bool]{}, err
			}
			if d, ok := s.lookupDecision(decisionID{key, n.header.ID}); ok && d.Outcome == OutcomeFail {
				return Evidence[bool]{State: EvidenceInvalid, Reason: d.Reason, Cause: &DecisionRef{Rule: key, Subject: NodeRef{s.documentID, n.header.ID}}}, nil
			}
		}
	}
	return stateUnknown[bool]("barline-chord-duration-unproved"), nil
}
