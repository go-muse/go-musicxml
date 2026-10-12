package validation

type layoutPendingName struct {
	Node    *factNode
	Certain bool
}

func (l *layoutScopes) publishName(n *factNode, frame layoutFrame, start Evidence[bool]) error {
	ref := NodeRef{l.r.s.documentID, n.header.ID}
	onset := frame.system
	if start.State != EvidenceKnown {
		onset = stateUnavailable[bool, NodeRef](start)
	} else if !start.Value {
		onset = Evidence[NodeRef]{State: EvidenceAbsent}
	}
	return l.r.s.publishContextResult(l.r.ctx, ContextResult{Provider: l.key, Subject: ref, Property: "layout.name-display-onset", Value: Evidence[Atomic]{State: EvidenceAbsent}, Origins: []Location{n.header.Location}, Layout: &LayoutContextResult{Scope: ref, Origin: ref, Raw: Evidence[Atomic]{State: EvidenceAbsent}, Applicability: start, System: onset, Page: frame.page, Provenance: ContextExplicit, Basis: "current-or-next-new-system"}})
}
func (l *layoutScopes) nameUpdates(b timelineMeasureBlock, frame layoutFrame, start Evidence[bool]) error {
	r := l.r
	if b.part.State != EvidenceKnown {
		return nil
	}
	part := b.part.Value.Node
	pending := l.names[part]
	if start.State == EvidenceKnown && start.Value {
		for _, p := range pending {
			if !r.step(1) {
				return r.err
			}
			onset := start
			if !p.Certain {
				onset = stateUnknown[bool]("intermediate-system-break-unavailable")
			}
			if err := l.publishName(p.Node, frame, onset); err != nil {
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
	print, _ := l.child(b.content, "print")
	if print != nil {
		for _, n := range print.children {
			if !r.step(1) {
				return r.err
			}
			if !r.matches(n, layoutRole("part-name-display")) && !r.matches(n, layoutRole("part-abbreviation-display")) {
				continue
			}
			if start.State == EvidenceKnown && start.Value {
				if err := l.publishName(n, frame, start); err != nil {
					return err
				}
			} else {
				if err := r.s.retain(512); err != nil {
					return err
				}
				pending = append(pending, layoutPendingName{n, start.State == EvidenceKnown})
			}
		}
	}
	l.names[part] = pending
	return r.err
}
func (l *layoutScopes) closeNames() error {
	for _, id := range l.r.s.sequence {
		if !l.r.step(1) {
			return l.r.err
		}
		for _, p := range l.names[id] {
			active := Evidence[bool]{State: EvidenceKnown, Value: false}
			if !p.Certain {
				active = stateUnknown[bool]("name-display-system-onset-unavailable")
			}
			if err := l.publishName(p.Node, layoutFrame{}, active); err != nil {
				return err
			}
		}
	}
	return l.r.err
}
