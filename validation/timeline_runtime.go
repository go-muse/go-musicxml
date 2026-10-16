package validation

import "context"

type timelineMeasureBlock struct {
	content, measure *factNode
	part             Evidence[NodeRef]
	ordinal          uint64
	complete         Evidence[bool]
	coverage         Evidence[bool]
}

func (s *Session) evaluateTimeline(ctx context.Context, n *factNode, key RuleKey, p *timelinePlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	service, err := s.newTimelineService(ctx, key)
	if err != nil {
		return Decision{}, false, err
	}
	if service.Final {
		return Decision{Outcome: OutcomePass}, false, nil
	}
	blocks, err := s.timelineBlocks(ctx, p)
	if err != nil {
		return Decision{}, false, err
	}
	index, err := service.prepareTimelineParts(ctx, s, blocks)
	if err != nil {
		return Decision{}, false, err
	}
	for _, part := range index.Parts {
		if err = service.State.readInstrumentDefinitions(ctx, s, p.Bindings, s.nodes[part.Node], part); err != nil {
			return Decision{}, false, err
		}
	}
	for _, block := range blocks {
		if block.part.State != EvidenceKnown {
			for _, part := range index.Parts {
				if err = s.step(ctx, 1); err != nil {
					return Decision{}, false, err
				}
				if err = service.gapAllPublishers(ctx, s, part, block.ordinal, timelineGapAs[NodeRef, musicalPosition](block.part, "timeline-part-owner-unavailable")); err != nil {
					return Decision{}, false, err
				}
			}
		}
	}
	for _, block := range blocks {
		if block.part.State == EvidenceKnown && index.Counts[timelineDomain{block.part.Value.Node, block.ordinal}] > 1 {
			block.complete = stateUnknown[bool]("timeline-duplicate-part-measure-domain")
		}

		if err = service.readMeasure(ctx, s, p.Bindings, block); err != nil {
			return Decision{}, false, err
		}
	}
	if err := service.finalizeTimingOrigins(ctx, s, p.Bindings, blocks); err != nil {
		return Decision{}, false, err
	}
	service.Final = true
	return Decision{Outcome: OutcomePass}, false, nil
}
func timelineAt(block timelineMeasureBlock, offset Evidence[Rational]) Evidence[musicalPosition] {
	if block.part.State != EvidenceKnown {
		return stateUnavailable[NodeRef, musicalPosition](block.part)
	}
	if offset.State != EvidenceKnown {
		return stateUnavailable[Rational, musicalPosition](offset)
	}
	return Evidence[musicalPosition]{State: EvidenceKnown, Value: musicalPosition{Part: block.part.Value, Measure: NodeRef{DocumentID: block.part.Value.DocumentID, Node: block.measure.header.ID}, MeasureOrdinal: block.ordinal, Offset: offset.Value}}
}
func (service *timelineService) readMeasure(ctx context.Context, s *Session, b *timelineBindings, block timelineMeasureBlock) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	pending, releasePending, err := newTimelinePendingOffsets(r, block)
	if err != nil {
		return err
	}
	defer releasePending()
	cursor := newTimelineCursor()
	// An exact ordinary note prefix proves observed current locations, not its
	// unobserved terminal End. Cursor excursions cannot create new witnesses.
	prefix, principalPrefix, monotone := true, false, true
	cursorPlacement := stateGapPlacement{}
	positiveUnits := false
	if len(service.unitPositive) != 0 && block.part.State == EvidenceKnown {
		if err := s.step(ctx, 1+int64(len(block.part.Value.DocumentID))); err != nil {
			return err
		}
		positiveUnits = service.unitPositive[block.part.Value]
	}
	units := Evidence[Decimal]{State: EvidenceAbsent}
	unitPlacementUnproved := false
	origin := timelineAt(block, cursor.Position)
	if origin.State == EvidenceKnown {
		inherited, err := service.State.effectiveDivisions(ctx, s, origin.Value)
		if err != nil {
			return err
		}
		units = inherited.Value
		if service.unitGates != nil && units.State != EvidenceKnown {
			unitPlacementUnproved, err = service.State.displacedUnitDependency(ctx, s, origin.Value)
			if err != nil {
				return err
			}
		}
	}
	unitGate, err := timelineUnitPlacementGap(r, block, units)
	if err != nil {
		return err
	}
	unitGap := Evidence[Decimal]{State: EvidenceAbsent}
	if unitGate.From == 0 && unitGate.Gap.State != "" {
		unitGap = unitGate.Gap
	}
	principalUnits := Evidence[Decimal]{State: EvidenceAbsent}
	lastDivisionPosition := Evidence[musicalPosition]{State: EvidenceAbsent}
	if block.complete.State != EvidenceKnown || !block.complete.Value {
		cursor.interrupt(block.complete)
		units = stateUnavailable[bool, Decimal](block.complete)
		if block.part.State == EvidenceKnown {
			if err := service.gapAllPublishers(ctx, s, block.part.Value, block.ordinal, stateUnavailable[bool, musicalPosition](block.complete)); err != nil {
				return err
			}
		}
	}
	for _, event := range block.content.children {
		if !r.step(1) {
			return r.err
		}
		if block.complete.State != EvidenceKnown || !block.complete.Value {
			cursor.interrupt(block.complete)
			prefix = false
		}
		if unitGate.From == event.header.ID && unitGate.Gap.State != "" {
			unitGap = unitGate.Gap
		}
		fact := timelineEventFact{Placement: cursorPlacement, Part: block.part, Staff: Evidence[Integer]{State: EvidenceAbsent}, Voice: Evidence[string]{State: EvidenceAbsent}, Duration: Evidence[Rational]{State: EvidenceAbsent}, Principal: stateUnknown[NodeRef]("timeline-principal-unavailable"), PrincipalDuration: stateUnknown[Rational]("timeline-principal-unavailable")}
		if event.header.Declaration.State != EvidenceKnown {
			prefix = false
			gap := stateUnavailable[DeclarationID, bool](*event.header.Declaration)
			if _, err := cursor.advance(ctx, s, timelineMovement{Kind: timelineMovementGap, Unavailability: gap}); err != nil {
				return err
			}
			fact.Position = stateUnavailable[DeclarationID, musicalPosition](*event.header.Declaration)
			fact.Origin = fact.Position
			if block.part.State == EvidenceKnown && r.potential(event, timelineRoleAttributes) {
				if err := service.State.gapAttributeFamilies(ctx, s, block.part.Value, block.ordinal, fact.Position, fact.Placement); err != nil {
					return err
				}
				units = stateUnavailable[musicalPosition, Decimal](fact.Position)
			}
			if block.part.State == EvidenceKnown && (r.potential(event, timelineRoleSound) || r.potential(event, timelineRoleDirection)) {
				if err := service.gapSoundPublishers(ctx, s, block.part.Value, block.ordinal, fact.Position, fact.Placement); err != nil {
					return err
				}
			}
			if err := service.storeFact(ctx, s, event.header.ID, fact); err != nil {
				return err
			}
			continue
		}
		movement := timelineMovement{Subject: NodeRef{s.documentID, event.header.ID}, Kind: timelineMovementOther, Chord: Evidence[bool]{State: EvidenceKnown, Value: false}, Grace: Evidence[bool]{State: EvidenceKnown, Value: false}, Duration: Evidence[Rational]{State: EvidenceAbsent}}
		rawMovement := Evidence[Decimal]{State: EvidenceAbsent}
		selectedUnits := units
		if unitGap.State != EvidenceAbsent {
			selectedUnits = unitGap
		}
		normalization, normalizationOverride := service.unitNormalizations[event.header.ID]
		if normalizationOverride {
			selectedUnits = normalization.Units
		}
		switch {
		case r.matches(event, timelineRoleNote):
			movement.Kind = timelineMovementNote
			_, movement.Chord = r.child(event, timelineRoleNoteChord)
			if movement.Chord.State == EvidenceAbsent {
				movement.Chord = Evidence[bool]{State: EvidenceKnown, Value: false}
			}
			_, movement.Grace = r.child(event, timelineRoleNoteGrace)
			if movement.Grace.State == EvidenceAbsent {
				movement.Grace = Evidence[bool]{State: EvidenceKnown, Value: false}
			}
			raw := stateDecimalValue(r.childScalar(event, timelineRoleNoteDuration))
			if movement.Grace.State == EvidenceKnown && movement.Grace.Value {
				movement.Duration = Evidence[Rational]{State: EvidenceAbsent}
			} else {
				var err error
				if movement.Chord.State == EvidenceKnown && movement.Chord.Value && principalUnits.State == EvidenceKnown {
					same, e := s.sameTimelineUnits(ctx, principalUnits, units)
					if e != nil {
						return e
					}
					if !same {
						selectedUnits = Evidence[Decimal]{State: EvidenceUnsupported, Reason: "chord-divisions-placement-unresolved"}
					}
				}
				movement.Duration, err = s.normalizeTimelineDuration(ctx, raw, selectedUnits)
				if err != nil {
					return err
				}
			}
			fact.Staff = stateInteger(r.childScalar(event, timelineRoleNoteStaff))
			fact.Voice = stateText(r.childScalar(event, timelineRoleNoteVoice))
			if block.part.State == EvidenceKnown {
				if err := service.State.readNoteInstruments(ctx, s, b, event, block.part.Value); err != nil {
					return err
				}
			}
		case r.matches(event, timelineRoleBackup):
			if unitGap.State != EvidenceAbsent {
				units = unitGap
			}
			if normalizationOverride {
				units = normalization.Units
			}
			movement.Kind = timelineMovementBackup
			raw := stateDecimalValue(r.childScalar(event, timelineRoleBackupDuration))
			rawMovement = raw
			if gate, found := service.unitGates[event.header.ID]; found {
				movement.Duration = stateUnavailable[Decimal, Rational](gate.Availability)
				units = gate.Availability
			} else {
				var err error
				movement.Duration, err = s.normalizeTimelineDuration(ctx, raw, units)
				if err != nil {
					return err
				}
			}
		case r.matches(event, timelineRoleForward):
			if unitGap.State != EvidenceAbsent {
				units = unitGap
			}
			if normalizationOverride {
				units = normalization.Units
			}
			movement.Kind = timelineMovementForward
			raw := stateDecimalValue(r.childScalar(event, timelineRoleForwardDuration))
			rawMovement = raw
			if gate, found := service.unitGates[event.header.ID]; found {
				movement.Duration = stateUnavailable[Decimal, Rational](gate.Availability)
				units = gate.Availability
			} else {
				var err error
				movement.Duration, err = s.normalizeTimelineDuration(ctx, raw, units)
				if err != nil {
					return err
				}
			}
			fact.Staff = stateInteger(r.childScalar(event, timelineRoleForwardStaff))
			fact.Voice = stateText(r.childScalar(event, timelineRoleForwardVoice))
		case r.matches(event, timelineRoleHarmony):
			fact.Staff = stateInteger(r.childScalar(event, timelineRoleHarmonyStaff))
		case r.matches(event, timelineRoleDirection):
			fact.Staff = stateInteger(r.childScalar(event, timelineRoleDirectionStaff))
			fact.Voice = stateText(r.childScalar(event, timelineRoleDirectionVoice))
		}
		if movement.Kind == timelineMovementNote && movement.Chord.State == EvidenceKnown && !movement.Chord.Value {
			if normalizationOverride {
				units = normalization.Units
			}
			principalUnits = units
		}
		if r.err != nil {
			return r.err
		}
		cursorFact, err := cursor.advance(ctx, s, movement)
		if err != nil {
			return err
		}
		fact.Position = timelineAt(block, cursorFact.Position)
		factPrefix := prefix
		if movement.Kind == timelineMovementNote && movement.Chord.State == EvidenceKnown && movement.Chord.Value {
			factPrefix = principalPrefix
		}
		if fact.Position.State == EvidenceKnown {
			fact.Position.Value.UnprovedFrame = !factPrefix || fact.Position.Value.Offset.value.Sign() < 0
			// A chord reuses its already observed principal onset; it is not
			// a new current-cursor witness and may precede the current maximum.
			if !(movement.Kind == timelineMovementNote && movement.Chord.State == EvidenceKnown && movement.Chord.Value) {
				if err := service.State.observeFrameOrigin(ctx, s, fact.Position.Value, monotone); err != nil {
					return err
				}
			}
		}
		if movement.Kind == timelineMovementNote && movement.Chord.State == EvidenceKnown && !movement.Chord.Value {
			principalPrefix = factPrefix
		}
		if movement.Kind == timelineMovementBackup || movement.Kind == timelineMovementForward {
			principalPrefix = false
			departureMember := false
			if fact.Position.State == EvidenceKnown {
				var e error
				departureMember, e = service.State.frameMember(ctx, s, fact.Position.Value)
				if e != nil {
					return e
				}
			}
			destination := timelineAt(block, cursor.Position)
			if destination.State == EvidenceKnown {
				destination.Value.UnprovedFrame = true
				member, e := service.State.frameMember(ctx, s, destination.Value)
				if e != nil {
					return e
				}
				prefix = member
				if !member || !departureMember {
					if err := service.recordUnitExcursion(ctx, s, event.header.ID, block, rawMovement, movement.Kind == timelineMovementBackup, movement.Duration); err != nil {
						return err
					}
				}
			} else {
				prefix = false
				if err := service.recordUnitExcursion(ctx, s, event.header.ID, block, rawMovement, movement.Kind == timelineMovementBackup, movement.Duration); err != nil {
					return err
				}
			}
			if cursor.Position.State != EvidenceKnown {
				cursorPlacement = service.movementPlacement(fact.Position, cursorPlacement, rawMovement, movement.Kind == timelineMovementBackup, positiveUnits || service.unitGates[event.header.ID].PositiveDirection)
			}
		} else if movement.Kind == timelineMovementNote && movement.Chord.State == EvidenceKnown && !movement.Chord.Value && movement.Grace.State == EvidenceKnown && !movement.Grace.Value && cursor.Position.State != EvidenceKnown && (cursorPlacement.Displaced || normalizationOverride && !normalization.Restored || unitPlacementUnproved) {
			// An ordinary note advances the unknown cursor. A prior upper
			// bound does not bound its new end; an established lower survives
			// only an assessed positive duration and positive unit-domain proof.
			cursorPlacement = service.movementPlacement(fact.Position, cursorPlacement, stateDecimalValue(r.childScalar(event, timelineRoleNoteDuration)), false, positiveUnits)
		}
		if movement.Kind == timelineMovementBackup || movement.Duration.State == EvidenceKnown && movement.Duration.Value.value.Sign() < 0 {
			monotone = false
		}
		if cursor.Position.State != EvidenceKnown {
			prefix = false
		} else if cursor.Position.Value.value.Sign() == 0 {
			prefix = true
		}
		if cursor.Position.State == EvidenceKnown {
			// Keep displacement scope independently of the exact coordinate, so
			// a later unavailable movement cannot silently regain confinement.
			cursorPlacement = stateGapPlacement{Displaced: !prefix}
		}
		fact.Origin = fact.Position
		fact.Units = selectedUnits
		fact.Duration = cursorFact.Duration
		fact.Principal = cursorFact.Principal
		fact.PrincipalDuration = cursorFact.PrincipalDuration
		if movement.Kind == timelineMovementNote {
			switch {
			case movement.Grace.State == EvidenceKnown && movement.Grace.Value:
				fact.End = Evidence[musicalPosition]{State: EvidenceAbsent}
			case movement.Chord.State == EvidenceKnown && !movement.Chord.Value:
				fact.End = timelineAt(block, cursor.Position)
				if fact.End.State == EvidenceKnown {
					fact.End.Value.UnprovedFrame = true
				}
			}
			if err := service.Playback.readNote(ctx, s, b, event, block, fact); err != nil {
				return err
			}
		}
		if r.matches(event, timelineRoleBarline) {
			if err := service.Playback.readBarline(ctx, s, b, event, block, fact); err != nil {
				return err
			}
		}
		if timelineOffsetEvent(r, event) {
			pending = append(pending, timelinePendingOffset{Event: event, Fact: fact})
			continue
		}
		if r.matches(event, timelineRoleAttributes) {
			current := timelineAttributeUnits(r, event)
			if fact.Position.State == EvidenceKnown {
				if err := service.State.readAttributes(ctx, s, b, event, fact.Position.Value); err != nil {
					return err
				}
			} else if block.part.State == EvidenceKnown {
				if err := service.State.gapAttributes(ctx, s, b, event, block.part.Value, block.ordinal, fact.Position, fact.Placement); err != nil {
					return err
				}
			}
			if current.State != EvidenceAbsent {
				if fact.Position.State == EvidenceKnown {
					samePosition := false
					if lastDivisionPosition.State == EvidenceKnown {
						cmp, err := stateComparePosition(ctx, s, lastDivisionPosition.Value, fact.Position.Value)
						if err != nil {
							return err
						}
						samePosition = cmp == 0
					}
					if samePosition {
						if units.State == EvidenceKnown && current.State == EvidenceKnown {
							same, err := s.sameTimelineUnits(ctx, units, current)
							if err != nil {
								return err
							}
							if !same {
								units = stateUnknown[Decimal]("timeline-state-conflict")
							}
						} else {
							units = timelineUnitWorst(units, current)
						}
					} else {
						units = current
					}
				} else {
					units = stateUnavailable[musicalPosition, Decimal](fact.Position)
				}
				lastDivisionPosition = fact.Position
				if service.unitGates != nil && fact.Position.State == EvidenceKnown {
					// An explicit declaration cannot repair an older unconfined
					// publisher solely by its XML serialization or measure ordinal.
					units, err = service.State.effectiveDivisionsEvidence(ctx, s, fact.Position.Value)
					if err != nil {
						return err
					}
					unitPlacementUnproved = false
					if units.State != EvidenceKnown {
						unitPlacementUnproved, err = service.State.displacedUnitDependency(ctx, s, fact.Position.Value)
						if err != nil {
							return err
						}
					}
				}
			}
		}
		if err := service.storeFact(ctx, s, event.header.ID, fact); err != nil {
			return err
		}
	}
	if r.err != nil {
		return r.err
	}
	for _, item := range pending {
		if err := service.finalizeOffsetEvent(ctx, s, b, block, item.Event, item.Fact); err != nil {
			return err
		}
	}
	// No deferred offset record is live beyond this point. Release the logical
	// scratch before constructing the independently owned note-origin records.
	pending = nil
	releasePending()
	if err := service.finalizeNoteOrigins(ctx, s, b, block); err != nil {
		return err
	}
	return nil
}

// Position-dependent associations run after every canonical measure has
// published its facts and effective-state history, including later voices.
func (service *timelineService) finalizeTimingOrigins(ctx context.Context, s *Session, b *timelineBindings, blocks []timelineMeasureBlock) error {
	if err := service.finalizeExcursionUnits(ctx, s, b, blocks); err != nil {
		return err
	}
	if err := service.resolveFiguredBassOriginsAll(ctx, s, b, blocks); err != nil {
		return err
	}
	if err := service.resolveGraceNeighbors(ctx, s, b, blocks); err != nil {
		return err
	}
	return service.resolveDirectionOrigins(ctx, s, b, blocks)
}
