package validation

import "context"

// Part-symbol is a part-wide presentation state. A later attributes element
// that omits it does not reset a previously encoded symbol to the brace default.
func layoutStatePartSymbol(ctx context.Context, s *Session, key RuleKey, n *factNode, r *contextReader, p *contextPlan) error {
	fact := layoutStateFact(r, p, n)
	service := s.timeline[p.Timeline]
	count := stateUnknown[Integer]("part-staves-unavailable")
	if service != nil && fact.Position.State == EvidenceKnown {
		v, e := service.State.effectiveStaves(ctx, s, fact.Position.Value)
		if e != nil {
			return e
		}
		count = v.Value
	}
	active := stateUnavailable[Integer, bool](count)
	if count.State == EvidenceKnown {
		cmp, e := count.Value.Cmp(localOne)
		if e != nil {
			return e
		}
		active = Evidence[bool]{State: EvidenceKnown, Value: cmp > 0}
	}
	var origin *factNode
	availability := Evidence[bool]{State: EvidenceKnown, Value: true}
	if r.matches(n, layoutStateRole("part-symbol")) {
		origin = n
		active = Evidence[bool]{State: EvidenceKnown, Value: true}
	} else {
		child, exists := r.child(n, layoutStateRole("part-symbol"))
		if exists.State == EvidenceKnown {
			_ = child
			return nil
		}
		if exists.State != EvidenceAbsent {
			availability = exists
		} else {
			origin, availability = layoutStatePriorSymbol(r, key, p, n, fact)
		}
	}
	raw := Evidence[Atomic]{State: EvidenceAbsent}
	value := stateUnavailable[bool, Atomic](availability)
	if origin != nil {
		raw = r.scalar(origin)
		value = raw
	} else if availability.State == EvidenceKnown {
		value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "brace"}}
	}
	ref := NodeRef{s.documentID, n.header.ID}
	from := ref
	provenance := ContextDocumentedDefault
	if origin != nil {
		from.Node = origin.header.ID
		provenance = ContextExplicit
		if origin != n {
			provenance = ContextInherited
		}
	}
	publish := func(property string, raw, value Evidence[Atomic], app Evidence[bool]) error {
		basis := ""
		if property == "part-symbol-barline-scope-change" {
			basis = "partial-symbol-coverage-implies-matching-common-barline-scope-change; false-does-not-mean-no-common-barlines"
		}
		return s.publishContextResult(ctx, ContextResult{Provider: key, Subject: ref, Property: ContextProperty("layout." + property), Value: value, Origins: []Location{n.header.Location}, Layout: &LayoutContextResult{Part: fact.Part, Scope: layoutStateScope(s, n, fact), Origin: from, Raw: raw, Applicability: app, Unit: ContextUnitNone, Provenance: provenance, Basis: basis}})
	}
	if e := publish("part-symbol", raw, value, active); e != nil {
		return e
	}
	topRaw, bottomRaw := Evidence[Atomic]{State: EvidenceAbsent}, Evidence[Atomic]{State: EvidenceAbsent}
	if origin != nil {
		topRaw = localRawAttribute(r, origin, layoutStateRole("part-symbol.@top-staff"))
		bottomRaw = localRawAttribute(r, origin, layoutStateRole("part-symbol.@bottom-staff"))
	}
	top, bottom := topRaw, bottomRaw
	if top.State == EvidenceAbsent {
		top = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: localOne}}
	}
	if bottom.State == EvidenceAbsent {
		bottom = stateUnavailable[Integer, Atomic](count)
		if count.State == EvidenceKnown {
			bottom = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: count.Value}}
		}
	}
	if availability.State != EvidenceKnown {
		top = stateUnavailable[bool, Atomic](availability)
		bottom = top
	}
	if e := publish("part-symbol-top", topRaw, top, active); e != nil {
		return e
	}
	if e := publish("part-symbol-bottom", bottomRaw, bottom, active); e != nil {
		return e
	}
	common := stateUnavailable[Integer, bool](count)
	if top.State != EvidenceKnown {
		common = stateUnavailable[Atomic, bool](top)
	} else if bottom.State != EvidenceKnown {
		common = stateUnavailable[Atomic, bool](bottom)
	} else if count.State == EvidenceKnown {
		a, e := top.Value.Integer.Cmp(localOne)
		if e != nil {
			return e
		}
		b, e := bottom.Value.Integer.Cmp(count.Value)
		if e != nil {
			return e
		}
		common = Evidence[bool]{State: EvidenceKnown, Value: a != 0 || b != 0}
	}
	return publish("part-symbol-barline-scope-change", raw, localBoolAtomic(common), active)
}
func layoutStatePriorSymbol(r *contextReader, key RuleKey, p *contextPlan, n *factNode, fact timelineEventFact) (*factNode, Evidence[bool]) {
	if fact.Position.State != EvidenceKnown {
		return nil, stateUnavailable[musicalPosition, bool](fact.Position)
	}
	index, e := layoutGetStateIndex(r, key, p)
	if e != nil {
		r.err = e
		return nil, stateUnknown[bool]("context-interrupted")
	}
	if index.Availability.State != EvidenceKnown {
		return nil, index.Availability
	}
	service := r.s.timeline[p.Timeline]
	if service == nil {
		return nil, stateUnknown[bool]("part-symbol-timeline-unavailable")
	}
	result, e := queryState(r.ctx, r.s, &service.State, &index.Symbols, fact.Position.Value, Evidence[Integer]{State: EvidenceKnown, Value: localOne}, stateWhole[NodeRef])
	if e != nil {
		r.err = e
		return nil, stateUnknown[bool]("context-interrupted")
	}
	// An unassigned part can name any canonical part. Without a canonical
	// frame the ordinal alone cannot prove a cross-part temporal bound.
	if len(index.UnknownSymbols) > 0 {
		return nil, index.UnknownSymbols[0].Availability
	}

	if result.Value.State == EvidenceAbsent {
		return nil, Evidence[bool]{State: EvidenceKnown, Value: true}
	}
	if result.Value.State != EvidenceKnown {
		return nil, stateUnavailable[NodeRef, bool](result.Value)
	}
	return r.s.nodes[result.Value.Value.Node], Evidence[bool]{State: EvidenceKnown, Value: true}
}
