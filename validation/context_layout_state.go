package validation

import "context"

func layoutStateNode(ctx context.Context, s *Session, key RuleKey, n *factNode, r *contextReader, p *contextPlan) error {
	active := Evidence[bool]{State: EvidenceKnown, Value: true}
	known := func(v string) Evidence[Atomic] {
		return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: v}}
	}
	attr := func(owner, name string) Evidence[Atomic] {
		return localRawAttribute(r, n, layoutStateRole(owner+".@"+name))
	}
	pub := func(property string, raw, value Evidence[Atomic], app Evidence[bool], unit ContextUnit) error {
		return localPublish(ctx, s, key, n, ContextProperty("layout."+property), raw, value, app, NodeRef{}, unit)
	}
	child := func(name string) Evidence[Atomic] { return r.childScalar(n, layoutStateRole(name)) }
	matches := func(name string) bool { return r.matches(n, layoutStateRole(name)) }
	if matches("note") {
		if e := layoutStatePercussionAdvice(ctx, s, key, n, r, p); e != nil {
			return e
		}
	}
	if matches("clef") {
		raw := attr("clef", "number")
		value := raw
		if raw.State == EvidenceAbsent {
			value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: localOne}}
		}
		if e := pub("clef-staff", raw, value, active, ContextUnitNone); e != nil {
			return e
		}
		additional := localDefault(stateYesNo(attr("clef", "additional")), false)
		if e := pub("clef-line", child("line"), child("line"), layoutStateNot(additional), ContextUnitNone); e != nil {
			return e
		}
		if e := pub("clef-size", attr("clef", "size"), attr("clef", "size"), additional, ContextUnitNone); e != nil {
			return e
		}
		eligible := layoutStateAnd(localDefault(stateYesNo(attr("clef", "print-object")), true), layoutStateNot(additional))
		if e := pub("clef-system-print", attr("clef", "print-object"), localBoolAtomic(eligible), active, ContextUnitNone); e != nil {
			return e
		}
		fact := layoutStateFact(r, p, n)
		start, e := atMeasureStart(ctx, s, fact.Position)
		if e != nil {
			return e
		}
		if e := pub("clef-after-barline", attr("clef", "after-barline"), attr("clef", "after-barline"), start, ContextUnitNone); e != nil {
			return e
		}
	}
	if matches("key") {
		raw := attr("key", "print-object")
		if e := pub("key-system-print", raw, localBoolAtomic(localDefault(stateYesNo(raw), true)), active, ContextUnitNone); e != nil {
			return e
		}
	}
	if matches("for-part") {
		raw := attr("for-part", "number")
		value := raw
		if raw.State == EvidenceAbsent {
			value = known("all-staves-of-created-part")
		}
		if e := pub("created-part-staff-scope", raw, value, active, ContextUnitNone); e != nil {
			return e
		}
	}
	if matches("line-detail") {
		raw := attr("line-detail", "print-object")
		visible := localDefault(stateYesNo(raw), true)
		if e := pub("hidden-line-coordinate", raw, known("included-in-clef-and-display-coordinates"), layoutStateNot(visible), ContextUnitNone); e != nil {
			return e
		}
		if e := pub("hidden-line-layout-count", raw, localBoolAtomic(visible), active, ContextUnitNone); e != nil {
			return e
		}
	}
	if matches("staff-details") {
		object := attr("staff-details", "print-object")
		spacing := attr("staff-details", "print-spacing")
		if e := pub("staff-cutaway", object, localBoolAtomic(layoutStateCutaway(localDefault(stateYesNo(object), true), localDefault(stateYesNo(spacing), true))), active, ContextUnitNone); e != nil {
			return e
		}
		typ := localDefault(stateText(child("staff-type")), "regular")
		alt := stateUnavailable[string, bool](typ)
		if typ.State == EvidenceKnown {
			alt = Evidence[bool]{State: EvidenceKnown, Value: typ.Value == "alternate"}
		}
		if e := pub("alternate-staff-playback", child("staff-type"), localBoolAtomic(layoutStateNot(alt)), active, ContextUnitNone); e != nil {
			return e
		}
		source := stateUnavailable[bool, Atomic](alt)
		if alt.State == EvidenceKnown {
			source = Evidence[Atomic]{State: EvidenceAbsent}
			if alt.Value {
				source = stateUnknown[Atomic]("alternate-prior-staff-unavailable")
				number := stateInteger(attr("staff-details", "number"))
				if number.State != EvidenceKnown && number.State != EvidenceAbsent {
					source = stateUnavailable[Integer, Atomic](number)
				}
				if number.State == EvidenceKnown {
					var e error
					source, e = layoutStatePrecedingStaff(ctx, s, number.Value)
					if e != nil {
						return e
					}
				}
			}
		}
		if source.State != EvidenceAbsent {
			if e := pub("alternate-staff-source", attr("staff-details", "number"), source, alt, ContextUnitNone); e != nil {
				return e
			}
		}
		size, has := r.child(n, layoutStateRole("staff-size"))
		_ = size
		if has.State == EvidenceAbsent {
			value := stateUnavailable[string, Atomic](typ)
			if typ.State == EvidenceKnown {
				switch typ.Value {
				case "cue", "ossia", "editorial":
					value = known("less-than-100-implementation-dependent")
				default:
					value = stateUnknown[Atomic]("staff-size-not-encoded")
				}
			}
			if e := pub("staff-size-range", child("staff-type"), value, active, ContextUnitPercent); e != nil {
				return e
			}
		}
	}
	if matches("staff-size") {
		if e := pub("staff-space-scale", r.scalar(n), r.scalar(n), active, ContextUnitPercent); e != nil {
			return e
		}
		raw := attr("staff-size", "scaling")
		if e := pub("staff-notation-scale", raw, layoutUnknownDefault(raw, "notation-scaling-not-encoded"), active, ContextUnitPercent); e != nil {
			return e
		}
		if e := pub("staff-size-affects-margins", r.scalar(n), localSemanticLabel(r.scalar(n), "no"), active, ContextUnitNone); e != nil {
			return e
		}
	}
	if matches("rest") {
		raw := attr("rest", "measure")
		scope := NodeRef{}
		for ancestor := s.nodes[n.header.Parent]; ancestor != nil; ancestor = s.nodes[ancestor.header.Parent] {
			if !r.step(1) {
				return r.err
			}
			if r.attributeOwner(ancestor, layoutStateRole("measure.@implicit")) {
				scope = NodeRef{s.documentID, ancestor.header.ID}
				break
			}
		}
		if e := localPublish(ctx, s, key, n, "layout.measure-rest", raw, raw, active, scope, ContextUnitNone); e != nil {
			return e
		}
	}
	if matches("stem") {
		parent := s.nodes[n.header.Parent]
		rest := stateUnknown[bool]("stem-note-unavailable")
		if parent != nil {
			rest = localPresence(r, parent, layoutStateRole("rest"))
		}
		if e := pub("stemlet", r.scalar(n), localBoolAtomic(rest), active, ContextUnitNone); e != nil {
			return e
		}
		raw := attr("stem", "relative-y")
		app := layoutStateStemApplicability(r.scalar(n), raw)
		if e := pub("stem-relative-y", raw, raw, app, ContextUnitTenths); e != nil {
			return e
		}
	}
	if matches("unpitched") {
		present := localPresence(r, n, layoutStateRole("display-step"))
		value := stateUnavailable[bool, Atomic](present)
		if present.State == EvidenceKnown {
			if present.Value {
				value = Evidence[Atomic]{State: EvidenceAbsent}
			} else {
				value = known("middle-line-of-staff")
			}
		}
		if e := pub("unpitched-default-position", child("display-step"), value, layoutStateNot(present), ContextUnitNone); e != nil {
			return e
		}
	}
	if r.attributeOwner(n, layoutStateRole("accord.@string")) {
		raw := attr("accord", "string")
		if e := pub("string-number-basis", raw, localSemanticLabel(raw, "one-is-highest-pitched-full-length-string"), active, ContextUnitNone); e != nil {
			return e
		}
	}
	if matches("measure-numbering") {
		raw := attr("measure-numbering", "staff")
		value := raw
		if raw.State == EvidenceAbsent {
			value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: localOne}}
		}
		if e := pub("measure-number-staff", raw, value, active, ContextUnitNone); e != nil {
			return e
		}
		policy := stateText(r.scalar(n))
		app := stateUnavailable[string, bool](policy)
		if policy.State == EvidenceKnown {
			app = Evidence[bool]{State: EvidenceKnown, Value: policy.Value != "none"}
		}
		for _, name := range []string{"multiple-rest-always", "multiple-rest-range"} {
			raw := attr("measure-numbering", name)
			if e := pub("measure-number-"+name, raw, raw, app, ContextUnitNone); e != nil {
				return e
			}
		}
	}
	if matches("string") {
		if e := pub("string-number-basis", r.scalar(n), localSemanticLabel(r.scalar(n), "one-is-highest-pitched-full-length-string"), active, ContextUnitNone); e != nil {
			return e
		}
	}
	if matches("note") {
		grace, cue := localPresence(r, n, layoutStateRole("grace")), localPresence(r, n, layoutStateRole("cue"))
		typ, exists := r.child(n, layoutStateRole("type"))
		raw := Evidence[Atomic]{State: EvidenceAbsent}
		if exists.State == EvidenceKnown {
			raw = localRawAttribute(r, typ, layoutStateRole("note-type.@size"))
		} else if exists.State != EvidenceAbsent {
			raw = stateUnavailable[bool, Atomic](exists)
		}
		value := layoutStateNoteCategory(localNoteSize(stateText(raw), grace, cue), grace)
		if e := pub("note-size-category", raw, localStringAtomic(value), active, ContextUnitNone); e != nil {
			return e
		}
	}

	if r.attributeOwner(n, layoutStateRole("measure.@implicit")) {
		implicit := localDefault(stateYesNo(attr("measure", "implicit")), false)
		raw := attr("measure", "text")
		value := raw
		if raw.State == EvidenceAbsent {
			value = attr("measure", "number")
		}
		if e := pub("measure-number-text", raw, value, layoutStateNot(implicit), ContextUnitNone); e != nil {
			return e
		}
	}
	if matches("attributes") || matches("part-symbol") {
		if e := layoutStatePartSymbol(ctx, s, key, n, r, p); e != nil {
			return e
		}
	}
	if matches("dot") || matches("rest") || matches("unpitched") {
		if e := layoutStatePosition(ctx, s, key, n, r, p); e != nil {
			return e
		}
	}
	return r.err
}

func layoutStateFact(r *contextReader, p *contextPlan, n *factNode) timelineEventFact {
	missing := timelineEventFact{Position: stateUnknown[musicalPosition]("layout-timeline-position-unavailable"), Staff: stateUnknown[Integer]("layout-staff-unavailable")}
	service := r.s.timeline[p.Timeline]
	if service == nil {
		return missing
	}
	for current := n; current != nil; current = r.s.nodes[current.header.Parent] {
		if !r.step(1) {
			return missing
		}
		if fact, ok := service.Facts[current.header.ID]; ok {
			return fact
		}
	}
	return missing
}
func layoutStateStemApplicability(stem, relative Evidence[Atomic]) Evidence[bool] {
	if stem.State != EvidenceKnown {
		return stateUnavailable[Atomic, bool](stem)
	}
	if stem.Value.Text != "up" && stem.Value.Text != "down" {
		return Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	if relative.State == EvidenceAbsent {
		return Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	if relative.State != EvidenceKnown {
		return stateUnavailable[Atomic, bool](relative)
	}
	if !relative.Value.Decimal.Valid() {
		return stateUnknown[bool]("stem-relative-coordinate-unavailable")
	}
	if relative.Value.Decimal.value.Sign() < 0 {
		return stateUnknown[bool]("stem-default-length-required-for-flip-test")
	}
	return Evidence[bool]{State: EvidenceKnown, Value: true}
}

func layoutStateScope(s *Session, n *factNode, fact timelineEventFact) NodeRef {
	if fact.Part.State == EvidenceKnown {
		return fact.Part.Value
	}
	return NodeRef{s.documentID, n.header.ID}
}
