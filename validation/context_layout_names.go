package validation

type layoutPendingName struct {
	Part    Evidence[NodeRef]
	Node    *factNode
	Certain bool
}

func (l *layoutScopes) publishName(n *factNode, frame layoutFrame, start Evidence[bool], part Evidence[NodeRef]) error {
	if l.r.err != nil {
		return l.r.err
	}
	ref := NodeRef{l.r.s.documentID, n.header.ID}
	onset := frame.system
	if start.State != EvidenceKnown {
		onset = stateUnavailable[bool, NodeRef](start)
	} else if !start.Value {
		onset = Evidence[NodeRef]{State: EvidenceAbsent}
	}
	return l.r.s.publishContextResult(l.r.ctx, ContextResult{Provider: l.key, Subject: ref, Property: "layout.name-display-onset", Value: Evidence[Atomic]{State: EvidenceAbsent}, Origins: []Location{n.header.Location}, Layout: &LayoutContextResult{Staff: Evidence[Integer]{State: EvidenceAbsent}, Conversion: Evidence[Rational]{State: EvidenceAbsent}, Part: part, Scope: ref, Origin: ref, Raw: Evidence[Atomic]{State: EvidenceAbsent}, Applicability: start, System: onset, Page: frame.page, Provenance: ContextExplicit, Basis: "current-or-next-new-system"}})
}
func (l *layoutScopes) nameUpdates(b timelineMeasureBlock, frame layoutFrame, start Evidence[bool]) error {
	r := l.r
	part := b.part.Value.Node
	var pending []layoutPendingName
	if b.part.State == EvidenceKnown {
		pending = l.names[part]
	}
	if start.State == EvidenceKnown && start.Value {
		for _, p := range pending {
			if !r.step(1) {
				return r.err
			}
			onset := start
			if !p.Certain {
				onset = stateUnknown[bool]("intermediate-system-break-unavailable")
			}
			if err := l.publishName(p.Node, frame, onset, p.Part); err != nil {
				return err
			}
		}
		pending = nil
	} else if start.State != EvidenceKnown {
		for i := range pending {
			if !r.step(1) {
				return r.err
			}
			pending[i].Certain = false
		}
	}
	_, selected := l.child(b.content, "print")
	onset := start
	if selected.State != EvidenceKnown && selected.State != EvidenceAbsent {
		onset = selected
	}
	if b.part.State != EvidenceKnown {
		onset = stateUnavailable[NodeRef, bool](b.part)
	}
	for _, print := range b.content.children {
		if !r.step(1) {
			return r.err
		}
		if !r.matches(print, layoutRole("print")) {
			continue
		}
		complete := r.enumerated(print)
		localOnset := onset
		if complete.State != EvidenceKnown || !complete.Value {
			localOnset = timelineGapAs[bool, bool](complete, "name-display-publisher-unavailable")
		}
		for _, n := range print.children {
			if !r.step(1) {
				return r.err
			}
			if !r.matches(n, layoutRole("part-name-display")) && !r.matches(n, layoutRole("part-abbreviation-display")) {
				continue
			}
			if b.part.State != EvidenceKnown || selected.State != EvidenceKnown || localOnset.State != EvidenceKnown || localOnset.Value {
				if err := l.publishName(n, frame, localOnset, b.part); err != nil {
					return err
				}
			} else {
				if err := r.s.retain(512); err != nil {
					return err
				}
				pending = append(pending, layoutPendingName{Node: n, Certain: start.State == EvidenceKnown, Part: b.part})
			}
		}
	}
	if b.part.State == EvidenceKnown {
		l.names[part] = pending
	}
	return r.err
}
func (l *layoutScopes) closeNames() error {
	for _, id := range l.r.s.sequence {
		if !l.r.step(1) {
			return l.r.err
		}
		for _, p := range l.names[id] {
			if !l.r.step(1) {
				return l.r.err
			}
			active := Evidence[bool]{State: EvidenceKnown, Value: false}
			if !p.Certain {
				active = stateUnknown[bool]("name-display-system-onset-unavailable")
			}
			if err := l.publishName(p.Node, layoutFrame{page: Evidence[NodeRef]{State: EvidenceAbsent}, system: Evidence[NodeRef]{State: EvidenceAbsent}}, active, p.Part); err != nil {
				return err
			}
		}
	}
	return l.r.err
}
