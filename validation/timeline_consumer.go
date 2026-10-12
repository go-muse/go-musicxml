package validation

import "context"

// timelineConsumerMatch borrows the sealed finite role set. The declaration
// witness is authoritative; names and text are never alternative bindings.
func (s *Session) timelineConsumerMatch(ctx context.Context, n *factNode, p *timelineConsumerPlan, role timelineRole) (bool, Decision, error) {
	if err := s.step(ctx, 1); err != nil {
		return false, Decision{}, err
	}
	if n.header.Declaration.State != EvidenceKnown {
		return false, localGap(n.header.Declaration), nil
	}
	for _, use := range p.Bindings.Roles[role] {
		if err := s.step(ctx, 1); err != nil {
			return false, Decision{}, err
		}
		if use.Declaration == n.header.Declaration.Value {
			return true, Decision{Outcome: OutcomePass}, nil
		}
	}
	return false, Decision{Outcome: OutcomePass}, nil
}

// A complete child set is needed even after finding a candidate: duplicates or
// unavailable exact bindings must not become an arbitrary first-child choice.
func (s *Session) timelineConsumerChild(ctx context.Context, n *factNode, p *timelineConsumerPlan, role timelineRole) (*factNode, Decision, error) {
	gap := Decision{Outcome: OutcomePass}
	if !n.closed || n.end.ChildrenComplete.State != EvidenceKnown || !n.end.ChildrenComplete.Value {
		gap = localIncomplete(n.end.ChildrenComplete, "timeline-children-incomplete")
	}
	var found *factNode
	for _, ch := range n.children {
		match, d, err := s.timelineConsumerMatch(ctx, ch, p, role)
		if err != nil {
			return nil, Decision{}, err
		}
		gap = localWorst(gap, d)
		if !match {
			continue
		}
		if found != nil {
			d, err = s.timelineConsumerStructure(ctx, n)
			if err != nil {
				return nil, Decision{}, err
			}
			if d.Outcome == OutcomePass {
				d = Decision{Outcome: OutcomeUnknown, Reason: "timeline-child-ambiguous"}
			}
			gap = localWorst(gap, d)
		}
		found = ch
	}
	return found, gap, nil
}

// The selected local shape and scalar domains come from the shared kernel.
// Generic fact streams already promise validated values, but still have to
// establish complete child enumeration and the required exact child uses.
func (s *Session) timelineConsumerStructure(ctx context.Context, n *factNode) (Decision, error) {
	if err := s.step(ctx, 1); err != nil {
		return Decision{}, err
	}
	if !n.closed || n.end.ChildrenComplete.State != EvidenceKnown || !n.end.ChildrenComplete.Value {
		return localIncomplete(n.end.ChildrenComplete, "timeline-children-incomplete"), nil
	}
	if s.schema == nil {
		return Decision{Outcome: OutcomePass}, nil
	}
	if s.schema.run == nil {
		return Decision{Outcome: OutcomeUnsupported, Reason: "schema-binding-unavailable"}, nil
	}
	for _, check := range [...]XSDCheck{XSDCheckDeclaration, XSDCheckAttributes, XSDCheckContent} {
		if err := s.step(ctx, 1); err != nil {
			return Decision{}, err
		}
		result := s.schemaResult(n, check)
		if !result.Valid {
			value, err := s.schemaInvalidValue(ctx, n, check)
			return localGap(value), err
		}
		if !result.Applicable || !result.Complete {
			cause, err := s.schemaResultCause(ctx, n, check)
			if err != nil {
				return Decision{}, err
			}
			if cause != nil {
				return Decision{Outcome: OutcomeBlocked, Reason: "timeline-local-shape-unavailable", Cause: cause}, nil
			}
			return Decision{Outcome: OutcomeUnsupported, Reason: "timeline-local-shape-unavailable"}, nil
		}
	}
	return Decision{Outcome: OutcomePass}, nil
}

// localNumeralKeyProof is intentionally called before generic prerequisites.
// Its bool means conclusive local proof or blocker. Only proven local absence
// returns false, allowing the exact provider dependency and positioned fallback.
func (s *Session) localNumeralKeyProof(ctx context.Context, n *factNode, p *timelineConsumerPlan) (Decision, bool, error) {
	match, d, err := s.timelineConsumerMatch(ctx, n, p, timelineRoleHarmonyNumeral)
	if err != nil {
		return Decision{}, false, err
	}
	if d.Outcome != OutcomePass {
		return d, true, nil
	}
	if !match {
		return Decision{Outcome: OutcomeNotApplicable, Reason: "timeline-numeral-role-mismatch"}, true, nil
	}
	key, d, err := s.timelineConsumerChild(ctx, n, p, timelineRoleNumeralKey)
	if err != nil {
		return Decision{}, false, err
	}
	if d.Outcome != OutcomePass {
		return d, true, nil
	}
	if key == nil {
		return Decision{}, false, nil
	}
	d, err = s.timelineConsumerStructure(ctx, key)
	if err != nil || d.Outcome != OutcomePass {
		return d, true, err
	}
	unresolved := Decision{Outcome: OutcomePass}
	for _, field := range [...]struct {
		role timelineRole
		kind AtomicKind
	}{{timelineRoleNumeralKeyFifths, AtomicInteger}, {timelineRoleNumeralKeyMode, AtomicString}} {
		child, gap, err := s.timelineConsumerChild(ctx, key, p, field.role)
		if err != nil {
			return Decision{}, false, err
		}
		unresolved = localWorst(unresolved, gap)
		if child == nil {
			unresolved = localWorst(unresolved, Decision{Outcome: OutcomeUnknown, Reason: "numeral-key-field-unavailable"})
			continue
		}
		shape, err := s.timelineConsumerStructure(ctx, child)
		if err != nil {
			return Decision{}, false, err
		}
		unresolved = localWorst(unresolved, shape)
		value, err := s.localScalar(ctx, child, nil)
		if err != nil {
			return Decision{}, false, err
		}
		if value.State != EvidenceKnown {
			unresolved = localWorst(unresolved, localGap(value))
		} else if value.Value.Kind != field.kind {
			unresolved = localWorst(unresolved, Decision{Outcome: OutcomeUnsupported, Reason: "numeral-key-field-domain-unavailable"})
		}
	}
	return unresolved, true, nil
}

func (s *Session) evaluateTimelineConsumer(ctx context.Context, n *factNode, p *timelineConsumerPlan, kind planKind, final bool) (Decision, bool, error) {
	if err := s.step(ctx, 1); err != nil {
		return Decision{}, false, err
	}
	if !n.closed {
		return Decision{}, true, nil
	}
	if kind == planNumeralKey {
		d, conclusive, err := s.localNumeralKeyProof(ctx, n, p)
		if err != nil || conclusive {
			return d, false, err
		}
	} else if kind == planChordDuration {
		match, d, err := s.timelineConsumerMatch(ctx, n, p, timelineRoleNote)
		if err != nil {
			return Decision{}, false, err
		}
		if d.Outcome != OutcomePass {
			return d, false, nil
		}
		if !match {
			return Decision{Outcome: OutcomeNotApplicable, Reason: "timeline-note-role-mismatch"}, false, nil
		}
		chord, d, err := s.timelineConsumerChild(ctx, n, p, timelineRoleNoteChord)
		if err != nil {
			return Decision{}, false, err
		}
		if d.Outcome != OutcomePass {
			return d, false, nil
		}
		if chord == nil {
			return Decision{Outcome: OutcomeNotApplicable, Reason: "chord-marker-absent"}, false, nil
		}
		duration, d, err := s.timelineConsumerChild(ctx, n, p, timelineRoleNoteDuration)
		if err != nil {
			return Decision{}, false, err
		}
		if d.Outcome != OutcomePass {
			return d, false, nil
		}
		if duration == nil {
			grace, d, err := s.timelineConsumerChild(ctx, n, p, timelineRoleNoteGrace)
			if err != nil {
				return Decision{}, false, err
			}
			if d.Outcome != OutcomePass {
				return d, false, nil
			}
			if grace != nil {
				return Decision{Outcome: OutcomeNotApplicable, Reason: "grace-chord-duration-absent"}, false, nil
			}
			return Decision{Outcome: OutcomeUnknown, Reason: "chord-duration-unavailable"}, false, nil
		}
	} else {
		return Decision{}, false, contract("timeline consumer", "unsupported sealed consumer family")
	}
	service := s.timeline[p.Provider]
	if service == nil || !service.Final {
		if !final {
			return Decision{}, true, nil
		}
		return Decision{Outcome: OutcomeUnsupported, Reason: "timeline-service-unavailable"}, false, nil
	}
	if kind == planChordDuration {
		d, err := s.timelineChordDuration(ctx, n, p, service)
		return d, false, err
	}
	d, err := s.timelineNumeralKey(ctx, n, p, service)
	return d, false, err
}

func (s *Session) timelineChordDuration(ctx context.Context, n *factNode, p *timelineConsumerPlan, service *timelineService) (Decision, error) {
	fact, ok := service.Facts[n.header.ID]
	if !ok {
		return Decision{Outcome: OutcomeUnknown, Reason: "timeline-note-unavailable"}, nil
	}
	if fact.Duration.State != EvidenceKnown {
		return localGap(timelineGapAs[Rational, Rational](fact.Duration, "chord-duration-unavailable")), nil
	}
	if fact.Principal.State != EvidenceKnown {
		return localGap(timelineGapAs[NodeRef, NodeRef](fact.Principal, "chord-principal-unavailable")), nil
	}
	if fact.PrincipalDuration.State != EvidenceKnown {
		return localGap(timelineGapAs[Rational, Rational](fact.PrincipalDuration, "chord-principal-duration-unavailable")), nil
	}
	principal := s.nodes[fact.Principal.Value.Node]
	if fact.Principal.Value.DocumentID != s.documentID || principal == nil {
		return Decision{}, contract("timeline chord principal", "same-document established note required")
	}
	comparison, err := s.rationalCmp(ctx, fact.Duration.Value, fact.PrincipalDuration.Value)
	if err != nil {
		return Decision{}, err
	}
	if comparison <= 0 {
		return Decision{Outcome: OutcomePass}, nil
	}
	d := Decision{Outcome: OutcomeFail, Reason: "chord-duration-exceeds-principal", Expected: "duration no longer than the principal non-chord note", Actual: fact.Duration.Value.String()}
	return s.withTimelineProvedRelated(n, p, planChordDuration, d, principal.header.Location)
}

func (s *Session) timelineNumeralKey(ctx context.Context, n *factNode, p *timelineConsumerPlan, service *timelineService) (Decision, error) {
	harmony := s.nodes[n.header.Parent]
	if harmony == nil {
		return Decision{Outcome: OutcomeUnknown, Reason: "numeral-harmony-unavailable"}, nil
	}
	match, d, err := s.timelineConsumerMatch(ctx, harmony, p, timelineRoleHarmony)
	if err != nil {
		return Decision{}, err
	}
	if d.Outcome != OutcomePass {
		return d, nil
	}
	if !match {
		return Decision{Outcome: OutcomeUnknown, Reason: "numeral-harmony-unavailable"}, nil
	}
	fact, ok := service.Facts[harmony.header.ID]
	if !ok {
		return Decision{Outcome: OutcomeUnknown, Reason: "timeline-harmony-unavailable"}, nil
	}
	if fact.Position.State != EvidenceKnown {
		return localGap(timelineGapAs[musicalPosition, musicalPosition](fact.Position, "harmony-position-unavailable")), nil
	}
	// This clause needs valid encoded-key existence, not one uniquely resolved
	// tonal value. The component-aware projection preserves malformed fields.
	result, err := service.State.effectiveKeyPresence(ctx, s, fact.Position.Value, fact.Staff)
	if err != nil {
		return Decision{}, err
	}
	if result.Value.State == EvidenceKnown {
		return Decision{Outcome: OutcomePass}, nil
	}
	if result.Value.State != EvidenceAbsent {
		return localGap(result.Value), nil
	}
	d = Decision{Outcome: OutcomeFail, Reason: "numeral-key-required", Expected: "a valid numeral-key or applicable encoded key", Actual: "neither key alternative is present"}
	return s.withTimelineProvedRelated(n, p, planNumeralKey, d, harmony.header.Location)
}

func (s *Session) withTimelineProvedRelated(n *factNode, p *timelineConsumerPlan, kind planKind, d Decision, location Location) (Decision, error) {
	if err := s.retain(128); err != nil {
		return d, &provedPlanInterruption{node: n.header.ID, timeline: p, kind: kind, decision: d, cause: err}
	}
	d.Related = []Location{location}
	return d, nil
}
