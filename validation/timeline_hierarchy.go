package validation

import "context"

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
	add := func(content, measure *factNode, part Evidence[NodeRef], ordinal uint64, coverage Evidence[bool]) error {
		if err := s.retain(512); err != nil {
			return err
		}
		complete := timelineEnumeration(content)
		if content.end.Order.State != EvidenceKnown {
			complete = stateUnavailable[OrderKind, bool](content.end.Order)
		} else if content.end.Order.Value != OrderSource && content.end.Order.Value != OrderModelContent {
			complete = stateUnknown[bool]("timeline-event-order-unavailable")
		}
		blocks = append(blocks, timelineMeasureBlock{content: content, measure: measure, part: part, ordinal: ordinal, complete: complete, coverage: coverage})
		return nil
	}
	h := p.Bindings.Hierarchies[hierarchy]
	rootCoverage := timelineHierarchyAvailability(s.root)
	ordinal := uint64(0)
	for _, outer := range s.root.children {
		if !r.step(1) {
			return nil, r.err
		}
		if outer.header.Declaration.State != EvidenceKnown {
			role := timelineRolePart
			if hierarchy == 1 {
				role = timelineRoleMeasure
			}
			if r.potential(outer, role) {
				if err := add(outer, outer, stateUnavailable[DeclarationID, NodeRef](outer.header.Declaration), ordinal, rootCoverage); err != nil {
					return nil, err
				}
				if hierarchy == 1 {
					ordinal++
				}
			}
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
			coverage := timelineMergeAvailability(rootCoverage, timelineHierarchyAvailability(outer))
			for _, measure := range outer.children {
				if !r.step(1) {
					return nil, r.err
				}
				if measure.header.Declaration.State != EvidenceKnown {
					if r.potential(measure, timelineRoleMeasure) {
						if err = add(measure, measure, part, ordinal, coverage); err != nil {
							return nil, err
						}
						blocks[len(blocks)-1].complete = stateUnavailable[DeclarationID, bool](measure.header.Declaration)
						ordinal++
					}
					continue
				}
				if measure.header.Declaration.Value != h.Measure.Declaration {
					continue
				}
				if err = add(measure, measure, part, ordinal, coverage); err != nil {
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
				if partNode.header.Declaration.State != EvidenceKnown {
					if r.potential(partNode, timelineRolePart) {
						if err := add(partNode, outer, stateUnavailable[DeclarationID, NodeRef](partNode.header.Declaration), ordinal, rootCoverage); err != nil {
							return nil, err
						}
					}
					continue
				}
				if partNode.header.Declaration.Value != h.Part.Declaration {
					continue
				}
				part, err := s.timelinePart(ctx, partNode, p)
				if err != nil {
					return nil, err
				}
				if err = add(partNode, outer, part, ordinal, rootCoverage); err != nil {
					return nil, err
				}
			}
			if availability := timelineHierarchyAvailability(outer); availability.State != EvidenceKnown || !availability.Value {
				if err := add(outer, outer, timelineGapAs[bool, NodeRef](availability, "timeline-part-list-incomplete"), ordinal, rootCoverage); err != nil {
					return nil, err
				}
			}
			ordinal++
		}
	}
	return blocks, r.err
}

func timelineMergeAvailability(a, b Evidence[bool]) Evidence[bool] {
	rank := func(v Evidence[bool]) int {
		switch v.State {
		case EvidenceInvalid:
			return 3
		case EvidenceUnsupported:
			return 2
		case EvidenceKnown:
			if v.Value {
				return 0
			}
		}
		return 1
	}
	if rank(b) > rank(a) {
		return b
	}
	return a
}
func timelineHierarchyAvailability(n *factNode) Evidence[bool] {
	v := timelineEnumeration(n)
	if v.State != EvidenceKnown || !v.Value {
		return v
	}
	if n.end.Order.State != EvidenceKnown {
		return timelineGapAs[OrderKind, bool](n.end.Order, "timeline-hierarchy-order-unavailable")
	}
	if n.end.Order.Value != OrderSource && n.end.Order.Value != OrderModelFields && n.end.Order.Value != OrderModelContent {
		return stateUnknown[bool]("timeline-hierarchy-order-unavailable")
	}
	return v
}

type timelineDomain struct {
	Part    NodeID
	Ordinal uint64
}
type timelineScopeIndex struct {
	Parts  []NodeRef
	Counts map[timelineDomain]uint64
}

func (service *timelineService) prepareTimelineParts(ctx context.Context, s *Session, blocks []timelineMeasureBlock) (timelineScopeIndex, error) {
	type partInfo struct {
		through  uint64
		complete Evidence[bool]
	}
	if err := s.retain(512); err != nil {
		return timelineScopeIndex{}, err
	}
	index := timelineScopeIndex{Counts: map[timelineDomain]uint64{}}
	parts := map[NodeID]partInfo{}
	for _, block := range blocks {
		if err := s.step(ctx, 1); err != nil {
			return timelineScopeIndex{}, err
		}
		if block.part.State != EvidenceKnown {
			continue
		}
		id := block.part.Value.Node
		info, exists := parts[id]
		if !exists {
			if err := s.retain(384); err != nil {
				return timelineScopeIndex{}, err
			}
			index.Parts = append(index.Parts, block.part.Value)
			info.complete = Evidence[bool]{State: EvidenceKnown, Value: true}
		}
		domain := timelineDomain{id, block.ordinal}
		if index.Counts[domain] == 0 {
			if err := s.retain(64); err != nil {
				return timelineScopeIndex{}, err
			}
		}
		index.Counts[domain]++
		if block.ordinal > info.through {
			info.through = block.ordinal
		}
		coverage := block.coverage
		if coverage.State == "" {
			coverage = stateUnknown[bool]("timeline-hierarchy-unavailable")
		}
		info.complete = timelineMergeAvailability(info.complete, coverage)
		parts[id] = info
	}
	for _, part := range index.Parts {
		if err := s.step(ctx, 1); err != nil {
			return timelineScopeIndex{}, err
		}
		info := parts[part.Node]
		if err := service.State.closePart(ctx, s, part, info.through, info.complete); err != nil {
			return timelineScopeIndex{}, err
		}
	}
	return index, nil
}
