package validation

import "context"

const planBarlineLocation planKind = "musicxml-barline-location"

// NewBarlineLocationPlan checks musical position against a structural origin
// and a bounded, independently supported measure-end certificate. It does not
// enforce the first/last XML-child recommendation or nominal meter capacity.
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
	if err := s.step(ctx, 1); err != nil {
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
	location := bar.Location.Value
	if location != "left" && location != "middle" && location != "right" {
		return Decision{Outcome: OutcomeUnknown, Reason: "barline-location-unavailable"}, false, nil
	}
	sign := pos.Offset.value.Sign()
	if location == "left" {
		return barlineLocationDecision(sign == 0), false, nil
	}
	if location == "middle" && sign <= 0 || location == "right" && sign < 0 {
		return barlineLocationDecision(false), false, nil
	}
	provider := s.catalog.plans[s.catalog.rules[p.Timeline].Predicate].timeline
	if provider == nil {
		return Decision{}, false, contract("barline timeline", "selected timeline plan required")
	}
	boundary, decisive, err := barlineLocationBoundary(ctx, s, service, provider.Bindings, s.nodes[n.header.Parent], pos, location)
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

// Only this predicate consumes these certificates. Nothing is written into
// frameBounds, UnprovedFrame, cross-measure ordering or repeat partEnd.
// lower is rebuilt without barlines, so the queried boundary cannot witness
// itself, directly or through a later excursion's containment.
func barlineLocationBoundary(ctx context.Context, s *Session, service *timelineService, b *timelineBindings, content *factNode, at musicalPosition, location string) (Evidence[Rational], Decision, error) {
	r := &stateReader{ctx: ctx, s: s, b: b}
	gap := timelineHierarchyAvailability(content)
	if gap.State != EvidenceKnown || !gap.Value {
		return timelineGapAs[bool, Rational](gap, "barline-measure-enumeration-incomplete"), Decision{}, nil
	}
	if content.end.Order.Value != OrderSource && content.end.Order.Value != OrderModelContent {
		return stateUnknown[Rational]("barline-measure-order-unavailable"), Decision{}, nil
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
				if cmp < 0 {
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
		if f.Units.Value.value.Sign() <= 0 {
			merge(stateUnknown[bool]("barline-positive-divisions-unavailable"))
			if !chord.Value {
				prefix = false
			}
			continue
		}
		if f.Duration.Value.value.Sign() <= 0 {
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
