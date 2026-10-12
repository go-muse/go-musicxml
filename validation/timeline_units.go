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

// This detector proves a sufficient region-local class without choosing how
// durations cross differing units. A changed declaration may start a region at
// the proven high-water mark; a rewind must remain entirely within that region.
// No provisional coordinates escape this complete preflight.
func timelineUnitPlacementGap(r *stateReader, block timelineMeasureBlock, inherited Evidence[Decimal]) (Evidence[Decimal], error) {
	current := inherited
	cursor := newTimelineCursor()
	high, region := timelineZero, timelineZero
	regionChanged := false
	unavailable := func() Evidence[Decimal] {
		return Evidence[Decimal]{State: EvidenceUnsupported, Reason: "divisions-rewind-placement-unresolved"}
	}
	for _, event := range block.content.children {
		if !r.step(1) {
			return Evidence[Decimal]{}, r.err
		}
		if r.matches(event, timelineRoleAttributes) {
			next := stateDecimalValue(r.childScalar(event, timelineRoleAttributesDivisions))
			if next.State == EvidenceAbsent {
				continue
			}
			same, err := r.s.sameTimelineUnits(r.ctx, current, next)
			if err != nil {
				return Evidence[Decimal]{}, err
			}
			if !same && current.State == EvidenceKnown && next.State == EvidenceKnown {
				if cursor.Position.State != EvidenceKnown {
					return stateUnavailable[Rational, Decimal](cursor.Position), nil
				}
				cmp, err := r.s.rationalCmp(r.ctx, cursor.Position.Value, high)
				if err != nil {
					return Evidence[Decimal]{}, err
				}
				if cmp < 0 {
					return unavailable(), nil
				}
				region = cursor.Position.Value
				atOrigin, err := r.s.rationalCmp(r.ctx, region, timelineZero)
				if err != nil {
					return Evidence[Decimal]{}, err
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
			return Evidence[Decimal]{}, err
		}
		departure := cursor.Position
		if _, err = cursor.advance(r.ctx, r.s, movement); err != nil {
			return Evidence[Decimal]{}, err
		}
		if movement.Kind == timelineMovementBackup && regionChanged {
			if departure.State != EvidenceKnown || cursor.Position.State != EvidenceKnown {
				return timelineGapAs[Rational, Decimal](cursor.Position, "timeline-position-unavailable"), nil
			}
			// A zero displacement is invariant; otherwise both interior departure and
			// landing at or above the region start are necessary for this proof.
			zero, err := r.s.rationalCmp(r.ctx, movement.Duration.Value, timelineZero)
			if err != nil {
				return Evidence[Decimal]{}, err
			}
			if zero != 0 {
				from, err := r.s.rationalCmp(r.ctx, departure.Value, region)
				if err != nil {
					return Evidence[Decimal]{}, err
				}
				to, err := r.s.rationalCmp(r.ctx, cursor.Position.Value, region)
				if err != nil {
					return Evidence[Decimal]{}, err
				}
				if from <= 0 || to < 0 {
					return unavailable(), nil
				}
			}
		}
		if cursor.Position.State == EvidenceKnown {
			cmp, err := r.s.rationalCmp(r.ctx, cursor.Position.Value, high)
			if err != nil {
				return Evidence[Decimal]{}, err
			}
			if cmp > 0 {
				high = cursor.Position.Value
			}
		}
	}
	if r.err != nil {
		return Evidence[Decimal]{}, r.err
	}
	return Evidence[Decimal]{State: EvidenceAbsent}, nil
}
