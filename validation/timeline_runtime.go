package validation

import "context"

type timelineMeasureBlock struct {
	content, measure *factNode
	part             Evidence[NodeRef]
	ordinal          uint64
	complete         Evidence[bool]
}

// timelineEnumeration checks enumeration evidence, without turning an unrelated
// invalid sibling into a fabricated loss of every exactly bound child.
func timelineEnumeration(n *factNode) Evidence[bool] {
	if n == nil || !n.closed {
		return stateUnknown[bool]("timeline-enumeration-open")
	}
	v := n.end.ChildrenComplete
	if v.State != EvidenceKnown || !v.Value {
		if v.State == EvidenceKnown || v.State == EvidenceAbsent {
			return stateUnknown[bool]("timeline-enumeration-incomplete")
		}
		return v
	}
	return Evidence[bool]{State: EvidenceKnown, Value: true}
}
func timelineDecisionEvidence[T any](d Decision) Evidence[T] {
	state := EvidenceUnknown
	if d.Outcome == OutcomeUnsupported {
		state = EvidenceUnsupported
	}
	if d.Outcome == OutcomeBlocked {
		state = EvidenceInvalid
	}
	return Evidence[T]{State: state, Reason: d.Reason, Cause: d.Cause}
}
func (s *Session) timelinePart(ctx context.Context, n *factNode, p *timelinePlan) (Evidence[NodeRef], error) {
	for _, owner := range p.PartOwners {
		if err := s.step(ctx, 1); err != nil {
			return Evidence[NodeRef]{}, err
		}
		if n.header.Declaration.State != EvidenceKnown {
			return stateUnavailable[DeclarationID, NodeRef](n.header.Declaration), nil
		}
		if n.header.Declaration.Value != owner.Declaration {
			continue
		}
		if err := s.evaluate(ctx, n, owner.Rule, true); err != nil {
			return Evidence[NodeRef]{}, err
		}
		d, ok := s.lookupDecision(decisionID{owner.Rule, n.header.ID})
		if !ok {
			return stateUnknown[NodeRef]("timeline-part-owner-unavailable"), nil
		}
		if d.Outcome == OutcomeFail {
			return Evidence[NodeRef]{State: EvidenceInvalid, Reason: "timeline-part-owner-invalid", Cause: &DecisionRef{Rule: owner.Rule, Subject: NodeRef{s.documentID, n.header.ID}}}, nil
		}
		if d.Outcome != OutcomePass {
			return timelineDecisionEvidence[NodeRef](d), nil
		}
		target, d, err := s.referenceResolution(ctx, n, owner.Attribute)
		if err != nil {
			return Evidence[NodeRef]{}, err
		}
		if d.Outcome != OutcomePass {
			return timelineDecisionEvidence[NodeRef](d), nil
		}
		return Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{s.documentID, target.header.ID}}, nil
	}
	return Evidence[NodeRef]{State: EvidenceUnsupported, Reason: "timeline-part-binding-unavailable"}, nil
}
func (s *Session) timelineBlocks(ctx context.Context, p *timelinePlan) ([]timelineMeasureBlock, error) {
	r := &stateReader{ctx: ctx, s: s, b: p.Bindings}
	hierarchy := -1
	for i, h := range p.Bindings.Hierarchies {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		if s.root.header.Declaration.State == EvidenceKnown && s.root.header.Declaration.Value == h.Root.Declaration {
			hierarchy = i
			break
		}
	}
	if hierarchy < 0 {
		return nil, nil
	} // Opus has no local score timeline.
	var blocks []timelineMeasureBlock
	add := func(content, measure *factNode, part Evidence[NodeRef], ordinal uint64) error {
		if err := s.retain(512); err != nil {
			return err
		}
		complete := timelineEnumeration(content)
		if content.end.Order.State != EvidenceKnown {
			complete = stateUnavailable[OrderKind, bool](content.end.Order)
		} else if content.end.Order.Value != OrderSource && content.end.Order.Value != OrderModelContent {
			complete = stateUnknown[bool]("timeline-event-order-unavailable")
		}
		blocks = append(blocks, timelineMeasureBlock{content: content, measure: measure, part: part, ordinal: ordinal, complete: complete})
		return nil
	}
	h := p.Bindings.Hierarchies[hierarchy]
	ordinal := uint64(0)
	for _, outer := range s.root.children {
		if !r.step(1) {
			return nil, r.err
		}
		if outer.header.Declaration.State != EvidenceKnown {
			continue
		}
		if hierarchy == 0 {
			if outer.header.Declaration.Value != h.Part.Declaration {
				continue
			}
			part, err := s.timelinePart(ctx, outer, p)
			if err != nil {
				return nil, err
			}
			ordinal = 0
			for _, measure := range outer.children {
				if !r.step(1) {
					return nil, r.err
				}
				if measure.header.Declaration.State != EvidenceKnown || measure.header.Declaration.Value != h.Measure.Declaration {
					continue
				}
				if err = add(measure, measure, part, ordinal); err != nil {
					return nil, err
				}
				ordinal++
			}
		} else {
			if outer.header.Declaration.Value != h.Measure.Declaration {
				continue
			}
			for _, partNode := range outer.children {
				if !r.step(1) {
					return nil, r.err
				}
				if partNode.header.Declaration.State != EvidenceKnown || partNode.header.Declaration.Value != h.Part.Declaration {
					continue
				}
				part, err := s.timelinePart(ctx, partNode, p)
				if err != nil {
					return nil, err
				}
				if err = add(partNode, outer, part, ordinal); err != nil {
					return nil, err
				}
			}
			ordinal++
		}
	}
	return blocks, r.err
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
	// Canonical identities come only from selected shared typed-reference proofs.
	// Repeated timewise part references are expected; duplicate occurrence domains
	// do not acquire an arbitrary concatenation order.
	type partInfo struct {
		through  uint64
		count    map[uint64]int
		complete Evidence[bool]
	}
	parts := map[NodeID]*partInfo{}
	for _, block := range blocks {
		if err = s.step(ctx, 1); err != nil {
			return Decision{}, false, err
		}
		if block.part.State != EvidenceKnown {
			continue
		}
		id := block.part.Value.Node
		info := parts[id]
		if info == nil {
			if err = s.retain(512); err != nil {
				return Decision{}, false, err
			}
			info = &partInfo{count: map[uint64]int{}, complete: Evidence[bool]{State: EvidenceKnown, Value: true}}
			parts[id] = info
		}
		if _, exists := info.count[block.ordinal]; !exists {
			if err = s.retain(64); err != nil {
				return Decision{}, false, err
			}
		}
		info.count[block.ordinal]++
		if block.ordinal > info.through {
			info.through = block.ordinal
		}
	}
	// Iterate blocks, not a map, so interruption and evidence are deterministic.
	initialized := map[NodeID]bool{}
	for _, block := range blocks {
		if err = s.step(ctx, 1); err != nil {
			return Decision{}, false, err
		}
		if block.part.State != EvidenceKnown {
			continue
		}
		id := block.part.Value.Node
		if initialized[id] {
			continue
		}
		if err = s.retain(32); err != nil {
			return Decision{}, false, err
		}
		initialized[id] = true
		if err = service.State.closePart(ctx, s, block.part.Value, parts[id].through, parts[id].complete); err != nil {
			return Decision{}, false, err
		}
		if err = service.State.readInstrumentDefinitions(ctx, s, p.Bindings, s.nodes[id], block.part.Value); err != nil {
			return Decision{}, false, err
		}
	}
	for _, block := range blocks {
		if block.part.State == EvidenceKnown && parts[block.part.Value.Node].count[block.ordinal] > 1 {
			block.complete = stateUnknown[bool]("timeline-duplicate-part-measure-domain")
		}
		if err = service.readMeasure(ctx, s, p.Bindings, block); err != nil {
			return Decision{}, false, err
		}
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
	cursor := newTimelineCursor()
	units := Evidence[Decimal]{State: EvidenceAbsent}
	origin := timelineAt(block, cursor.Position)
	if origin.State == EvidenceKnown {
		inherited, err := service.State.effectiveDivisions(ctx, s, origin.Value)
		if err != nil {
			return err
		}
		units = inherited.Value
	}
	unitGap, err := timelineUnitPlacementGap(r, block, units)
	if err != nil {
		return err
	}
	principalUnits := Evidence[Decimal]{State: EvidenceAbsent}
	if block.complete.State != EvidenceKnown || !block.complete.Value {
		cursor.interrupt(block.complete)
		units = stateUnavailable[bool, Decimal](block.complete)
		if block.part.State == EvidenceKnown {
			if err := service.State.gapAll(ctx, s, block.part.Value, block.ordinal, stateUnavailable[bool, musicalPosition](block.complete)); err != nil {
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
		}
		fact := timelineEventFact{Part: block.part, Staff: Evidence[Integer]{State: EvidenceAbsent}, Voice: Evidence[string]{State: EvidenceAbsent}, Duration: Evidence[Rational]{State: EvidenceAbsent}, Principal: stateUnknown[NodeRef]("timeline-principal-unavailable"), PrincipalDuration: stateUnknown[Rational]("timeline-principal-unavailable")}
		if event.header.Declaration.State != EvidenceKnown {
			gap := stateUnavailable[DeclarationID, bool](event.header.Declaration)
			if _, err := cursor.advance(ctx, s, timelineMovement{Kind: timelineMovementGap, Unavailability: gap}); err != nil {
				return err
			}
			fact.Position = stateUnavailable[DeclarationID, musicalPosition](event.header.Declaration)
			if block.part.State == EvidenceKnown && r.potential(event, timelineRoleAttributes) {
				if err := service.State.gapAll(ctx, s, block.part.Value, block.ordinal, fact.Position); err != nil {
					return err
				}
				units = stateUnavailable[musicalPosition, Decimal](fact.Position)
			}
			if err := service.storeFact(ctx, s, event.header.ID, fact); err != nil {
				return err
			}
			continue
		}
		movement := timelineMovement{Subject: NodeRef{s.documentID, event.header.ID}, Kind: timelineMovementOther, Chord: Evidence[bool]{State: EvidenceKnown, Value: false}, Grace: Evidence[bool]{State: EvidenceKnown, Value: false}, Duration: Evidence[Rational]{State: EvidenceAbsent}}
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
				selectedUnits := units
				if unitGap.State != EvidenceAbsent {
					selectedUnits = unitGap
				}
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
			movement.Kind = timelineMovementBackup
			raw := stateDecimalValue(r.childScalar(event, timelineRoleBackupDuration))
			var err error
			movement.Duration, err = s.normalizeTimelineDuration(ctx, raw, units)
			if err != nil {
				return err
			}
		case r.matches(event, timelineRoleForward):
			if unitGap.State != EvidenceAbsent {
				units = unitGap
			}
			movement.Kind = timelineMovementForward
			raw := stateDecimalValue(r.childScalar(event, timelineRoleForwardDuration))
			var err error
			movement.Duration, err = s.normalizeTimelineDuration(ctx, raw, units)
			if err != nil {
				return err
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
		fact.Duration = cursorFact.Duration
		fact.Principal = cursorFact.Principal
		fact.PrincipalDuration = cursorFact.PrincipalDuration
		if r.matches(event, timelineRoleHarmony) || r.matches(event, timelineRoleDirection) {
			role := timelineRoleHarmonyOffset
			if r.matches(event, timelineRoleDirection) {
				role = timelineRoleDirectionOffset
			}
			offset := stateDecimalValue(r.childScalar(event, role))
			if offset.State != EvidenceAbsent {
				delta, err := s.normalizeTimelineDuration(ctx, offset, units)
				if err != nil {
					return err
				}
				if fact.Position.State == EvidenceKnown && delta.State == EvidenceKnown {
					sum, err := s.rationalAdd(ctx, fact.Position.Value.Offset, delta.Value)
					if err != nil {
						return err
					}
					fact.Position.Value.Offset = sum
				} else if delta.State != EvidenceKnown {
					fact.Position = stateUnavailable[Rational, musicalPosition](delta)
				}
			}
		}
		if r.matches(event, timelineRoleAttributes) {
			current := stateDecimalValue(r.childScalar(event, timelineRoleAttributesDivisions))
			if fact.Position.State == EvidenceKnown {
				if err := service.State.readAttributes(ctx, s, b, event, fact.Position.Value); err != nil {
					return err
				}
			} else if block.part.State == EvidenceKnown {
				if err := service.State.gapAttributes(ctx, s, b, event, block.part.Value, block.ordinal, fact.Position); err != nil {
					return err
				}
			}
			if current.State != EvidenceAbsent {
				if fact.Position.State == EvidenceKnown {
					result, err := service.State.effectiveDivisions(ctx, s, fact.Position.Value)
					if err != nil {
						return err
					}
					units = result.Value
				} else {
					units = stateUnavailable[musicalPosition, Decimal](fact.Position)
				}
			}
		}
		if r.matches(event, timelineRoleSound) && fact.Position.State == EvidenceKnown {
			if err := service.State.readInstrumentChanges(ctx, s, b, event, fact.Position.Value); err != nil {
				return err
			}
		}
		if err := service.storeFact(ctx, s, event.header.ID, fact); err != nil {
			return err
		}
	}
	return r.err
}
