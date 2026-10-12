package validation

import "context"

func (s *Session) sameTimelineUnits(ctx context.Context, a, b Evidence[Decimal]) (bool, error) {
	if a.State != EvidenceKnown || b.State != EvidenceKnown {
		return false, nil
	}
	if err := s.step(ctx, 1+int64(len(a.Value.text))/4+int64(len(b.Value.text))/4); err != nil {
		return false, err
	}
	return a.Value.text == b.Value.text, nil
}

type timelineUnitGate struct {
	Gap  Evidence[Decimal]
	From NodeID // Zero means a late publisher can affect earlier non-origin facts.
}

// This detector proves a sufficient region-local class without choosing how
// durations cross differing units. A changed declaration may start a region at
// the proven high-water mark; a rewind must remain entirely within that region.
// No provisional coordinates escape this complete preflight.
func timelineUnitPlacementGap(r *stateReader, block timelineMeasureBlock, inherited Evidence[Decimal]) (timelineUnitGate, error) {
	needed, err := timelineNeedsUnitPreflight(r, block, inherited)
	if err != nil || !needed {
		return timelineUnitGate{}, err
	}
	current := inherited
	cursor := newTimelineCursor()
	high, region := timelineZero, timelineZero
	regionChanged := false
	unavailable := func(from NodeID) timelineUnitGate {
		return timelineUnitGate{Gap: Evidence[Decimal]{State: EvidenceUnsupported, Reason: "divisions-rewind-placement-unresolved"}, From: from}
	}
	for _, event := range block.content.children {
		if !r.step(1) {
			return timelineUnitGate{}, r.err
		}
		if r.matches(event, timelineRoleAttributes) {
			next := timelineAttributeUnits(r, event)
			if next.State == EvidenceAbsent {
				continue
			}
			same, err := r.s.sameTimelineUnits(r.ctx, current, next)
			if err != nil {
				return timelineUnitGate{}, err
			}
			if !same && current.State == EvidenceKnown && next.State == EvidenceKnown {
				if cursor.Position.State != EvidenceKnown {
					return timelineUnitGate{}, nil
				}
				cmp, err := r.s.rationalCmp(r.ctx, cursor.Position.Value, high)
				if err != nil {
					return timelineUnitGate{}, err
				}
				if cmp < 0 {
					return unavailable(0), nil
				}
				region = cursor.Position.Value
				atOrigin, err := r.s.rationalCmp(r.ctx, region, timelineZero)
				if err != nil {
					return timelineUnitGate{}, err
				}
				regionChanged = atOrigin != 0
			}
			current = next
			continue
		}
		movement := timelineMovement{Subject: NodeRef{r.s.documentID, event.header.ID}, Kind: timelineMovementOther, Chord: Evidence[bool]{State: EvidenceKnown}, Grace: Evidence[bool]{State: EvidenceKnown}}
		var role timelineRole
		switch {
		case r.matches(event, timelineRoleBackup):
			movement.Kind = timelineMovementBackup
			role = timelineRoleBackupDuration
		case r.matches(event, timelineRoleForward):
			movement.Kind = timelineMovementForward
			role = timelineRoleForwardDuration
		case r.matches(event, timelineRoleNote):
			_, chord := r.child(event, timelineRoleNoteChord)
			_, grace := r.child(event, timelineRoleNoteGrace)
			if chord.State == EvidenceKnown || grace.State == EvidenceKnown {
				continue
			}
			if chord.State != EvidenceAbsent || grace.State != EvidenceAbsent {
				cursor.interrupt(stateUnavailable[bool, bool](chord))
				continue
			}
			movement.Kind = timelineMovementNote
			role = timelineRoleNoteDuration
		default:
			continue
		}
		raw := stateDecimalValue(r.childScalar(event, role))
		var err error
		movement.Duration, err = r.s.normalizeTimelineDuration(r.ctx, raw, current)
		if err != nil {
			return timelineUnitGate{}, err
		}
		departure := cursor.Position
		if _, err = cursor.advance(r.ctx, r.s, movement); err != nil {
			return timelineUnitGate{}, err
		}
		if movement.Kind == timelineMovementBackup && regionChanged {
			if departure.State != EvidenceKnown || cursor.Position.State != EvidenceKnown {
				return timelineUnitGate{}, nil
			}
			// A zero displacement is invariant; otherwise both interior departure and
			// landing at or above the region start are necessary for this proof.
			zero, err := r.s.rationalCmp(r.ctx, movement.Duration.Value, timelineZero)
			if err != nil {
				return timelineUnitGate{}, err
			}
			if zero != 0 {
				from, err := r.s.rationalCmp(r.ctx, departure.Value, region)
				if err != nil {
					return timelineUnitGate{}, err
				}
				to, err := r.s.rationalCmp(r.ctx, cursor.Position.Value, region)
				if err != nil {
					return timelineUnitGate{}, err
				}
				if from <= 0 || to < 0 {
					return unavailable(event.header.ID), nil
				}
			}
		}
		if cursor.Position.State == EvidenceKnown {
			cmp, err := r.s.rationalCmp(r.ctx, cursor.Position.Value, high)
			if err != nil {
				return timelineUnitGate{}, err
			}
			if cmp > 0 {
				high = cursor.Position.Value
			}
		}
	}
	if r.err != nil {
		return timelineUnitGate{}, r.err
	}
	return timelineUnitGate{}, nil
}

// Only this attribute family is relevant to unit interpretation. An unrelated
// malformed key/time child cannot masquerade as a divisions publication.
func timelineAttributeUnits(r *stateReader, n *factNode) Evidence[Decimal] {
	complete := r.enumerated(n)
	if complete.State != EvidenceKnown || !complete.Value {
		return timelineGapAs[bool, Decimal](complete, "timeline-divisions-enumeration-unavailable")
	}
	value := Evidence[Decimal]{State: EvidenceAbsent}
	seen := false
	for _, ch := range n.children {
		if !r.step(1) {
			return stateUnknown[Decimal]("timeline-interrupted")
		}
		if ch.header.Declaration.State != EvidenceKnown {
			if r.potential(ch, timelineRoleAttributesDivisions) {
				gap := timelineGapAs[DeclarationID, Decimal](ch.header.Declaration, "timeline-divisions-declaration-unavailable")
				value = timelineUnitWorst(value, gap)
			}
			continue
		}
		if !r.matches(ch, timelineRoleAttributesDivisions) {
			continue
		}
		next := stateDecimalValue(r.scalar(ch))
		if seen {
			next = timelineUnitWorst(next, stateUnknown[Decimal]("timeline-divisions-child-ambiguous"))
		}
		seen = true
		if value.State == EvidenceAbsent {
			value = next
		} else {
			value = timelineUnitWorst(value, next)
		}
	}
	return value
}
func timelineUnitWorst(a, b Evidence[Decimal]) Evidence[Decimal] {
	rank := func(v Evidence[Decimal]) int {
		switch v.State {
		case EvidenceInvalid:
			return 3
		case EvidenceUnsupported:
			return 2
		case EvidenceUnknown:
			return 1
		}
		return 0
	}
	if rank(b) > rank(a) {
		return b
	}
	return a
}

// Skip exact reconstruction preflight where no varying units are rewound. This
// is a logical proof shortcut, not reduced arithmetic precision or wider limits.
func timelineNeedsUnitPreflight(r *stateReader, block timelineMeasureBlock, inherited Evidence[Decimal]) (bool, error) {
	current := inherited
	moved, changed, backup := false, false, false
	for _, event := range block.content.children {
		if !r.step(1) {
			return false, r.err
		}
		if r.matches(event, timelineRoleBackup) {
			backup = true
			moved = true
		}
		if r.matches(event, timelineRoleNote) || r.matches(event, timelineRoleForward) {
			moved = true
		}
		if r.matches(event, timelineRoleAttributes) {
			next := timelineAttributeUnits(r, event)
			if next.State == EvidenceAbsent {
				continue
			}
			same, err := r.s.sameTimelineUnits(r.ctx, current, next)
			if err != nil {
				return false, err
			}
			if moved && !same {
				changed = true
			}
			current = next
		}
	}
	return backup && changed, r.err
}
