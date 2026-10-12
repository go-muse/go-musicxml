package validation

func layoutKnownText(text string) Evidence[Atomic] {
	return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: text}}
}
func layoutAncestor(r *contextReader, n *factNode, roles ...string) *factNode {
	for a := n; a != nil; a = r.s.nodes[a.header.Parent] {
		if !r.step(1) {
			return nil
		}
		for _, role := range roles {
			if r.matches(a, layoutRole(role)) {
				return a
			}
		}
	}
	return nil
}
func layoutStatic(r *contextReader, key RuleKey, n *factNode) error {
	s := r.s
	ref := NodeRef{s.documentID, n.header.ID}
	yes := Evidence[bool]{State: EvidenceKnown, Value: true}
	publish := func(property string, raw, value Evidence[Atomic], active Evidence[bool], unit ContextUnit, basis string) error {
		return s.publishContextResult(r.ctx, ContextResult{Provider: key, Subject: ref, Property: ContextProperty("layout." + property), Value: value, Origins: []Location{n.header.Location}, Layout: &LayoutContextResult{Scope: ref, Origin: ref, Raw: raw, Applicability: active, Unit: unit, Provenance: ContextExplicit, Basis: basis}})
	}
	for _, name := range []string{"default-x", "default-y", "relative-x", "relative-y", "bezier-x", "bezier-y", "bezier-x2", "bezier-y2", "bezier-offset", "bezier-offset2"} {
		if !r.attributeOwner(n, layoutRole("@"+name)) {
			continue
		}
		raw := layoutAttribute(r, n, name)
		active := yes
		unit := ContextUnitTenths
		basis := "default-position-plus-placement-offset-directive"
		switch name {
		case "default-x", "default-y":
			basis = "note-or-musical-position-x-staff-top-y"
			if layoutAncestor(r, n, "credit") != nil {
				basis = "physical-page-lower-left"
			} else if layoutAncestor(r, n, "part-name-display", "part-abbreviation-display", "group-name-display", "group-abbreviation-display") != nil || r.matches(n, layoutRole("group-symbol")) {
				basis = "first-measure-of-system-x-staff-top-y"
			} else if layoutAncestor(r, n, "part-list") != nil || r.matches(n, layoutRole("note")) || r.matches(n, layoutRole("figured-bass")) || r.matches(n, layoutRole("harmony")) || r.matches(n, layoutRole("link")) || r.matches(n, layoutRole("directive")) || r.matches(n, layoutRole("measure-numbering")) || r.matches(s.nodes[n.header.Parent], layoutRole("direction-type")) {
				basis = "current-measure-start-x-staff-top-y"
			}
		case "bezier-x", "bezier-x2", "bezier-y", "bezier-y2":
			basis = "slur-or-tied-position"
		case "bezier-offset", "bezier-offset2":
			unit = ContextUnitDivisions
			basis = "musical-position-nonadditive-to-bezier-x"
			other := "bezier-x"
			if name == "bezier-offset2" {
				other = "bezier-x2"
			}
			x := layoutAttribute(r, n, other)
			if x.State == EvidenceKnown {
				active.Value = false
			} else if x.State != EvidenceAbsent {
				active = stateUnavailable[Atomic, bool](x)
			}
		}
		if err := publish(name, raw, raw, active, unit, basis); err != nil {
			return err
		}
	}
	if r.matches(n, layoutRole("print")) {
		page := stateYesNo(layoutAttribute(r, n, "new-page"))
		if page.State == EvidenceAbsent {
			page = Evidence[bool]{State: EvidenceKnown, Value: false}
		}
		for _, name := range []string{"page-number", "blank-page"} {
			raw := layoutAttribute(r, n, name)
			if err := publish(name, raw, raw, page, ContextUnitNone, "new-page-only"); err != nil {
				return err
			}
		}
		raw := layoutAttribute(r, n, "staff-spacing")
		active := yes
		complete := r.enumerated(n)
		if complete.State != EvidenceKnown {
			active = complete
		} else {
			for _, ch := range n.children {
				if !r.step(1) {
					return r.err
				}
				if r.matches(ch, layoutRole("staff-layout")) {
					active.Value = false
					break
				}
			}
		}
		if err := publish("staff-spacing", raw, raw, active, ContextUnitTenths, "deprecated-staff-layout-fallback"); err != nil {
			return err
		}
	}
	parent := s.nodes[n.header.Parent]
	if r.matches(parent, layoutRole("direction-type")) {
		direction := s.nodes[parent.header.Parent]
		raw := layoutAttribute(r, direction, "directive")
		active := yes
		for _, name := range []string{"default-x", "justify", "halign"} {
			v := layoutAttribute(r, n, name)
			if v.State == EvidenceKnown {
				active = Evidence[bool]{State: EvidenceKnown, Value: false}
				break
			} else if v.State != EvidenceAbsent {
				active = stateUnavailable[Atomic, bool](v)
			}
		}
		if err := publish("directive", raw, raw, active, ContextUnitNone, "time-signature-or-first-notational-element"); err != nil {
			return err
		}
	}
	for _, names := range [][2]string{{"group-name", "group-name-display"}, {"group-abbreviation", "group-abbreviation-display"}} {
		if !r.matches(n, layoutRole(names[0])) {
			continue
		}
		_, e := r.child(parent, layoutRole(names[1]))
		active := yes
		if e.State == EvidenceKnown {
			active.Value = false
		} else if e.State != EvidenceAbsent {
			active = e
		}
		if err := publish("display-active", r.scalar(n), localBoolAtomic(active), active, ContextUnitNone, "corresponding-name-display-overrides-deprecated-format"); err != nil {
			return err
		}
	}

	if r.matches(n, layoutRole("dynamics")) {
		raw := layoutAttribute(r, n, "placement")
		value := raw
		if direction := layoutAncestor(r, s.nodes[n.header.Parent], "direction"); direction != nil {
			parent := layoutAttribute(r, direction, "placement")
			if parent.State != EvidenceAbsent {
				value = parent
			}
		}
		if err := publish("placement", raw, value, yes, ContextUnitNone, "containing-direction-placement-overrides"); err != nil {
			return err
		}
	}
	if r.matches(n, layoutRole("lyric")) {
		raw := layoutAttribute(r, n, "placement")
		if err := publish("lyric-placement", raw, layoutDefault(raw, "below"), yes, ContextUnitNone, "lyric-below-default"); err != nil {
			return err
		}
	}
	for _, spec := range []struct{ element, alternative string }{{"elision", "smufl"}, {"tap", "hand"}} {
		if !r.matches(n, layoutRole(spec.element)) {
			continue
		}
		raw := r.scalar(n)
		value := raw
		active := Evidence[bool]{State: EvidenceKnown, Value: false}
		if raw.State == EvidenceKnown && raw.Value.Kind == AtomicString && raw.Value.Text == "" || raw.State == EvidenceAbsent {
			value = layoutAttribute(r, n, spec.alternative)
			active.Value = true
			if value.State == EvidenceAbsent {
				value = stateUnknown[Atomic]("application-symbol-choice")
			}
		}
		if raw.State != EvidenceKnown && raw.State != EvidenceAbsent {
			active = stateUnavailable[Atomic, bool](raw)
		}
		if err := publish("display", raw, value, yes, ContextUnitNone, "nonempty-text-before-symbol-selector"); err != nil {
			return err
		}
		alternate := layoutAttribute(r, n, spec.alternative)
		if err := publish(spec.alternative, alternate, alternate, active, ContextUnitNone, "text-precedence"); err != nil {
			return err
		}
	}
	if r.matches(n, layoutRole("arrowhead")) {
		if err := publish("geometry", Evidence[Atomic]{State: EvidenceAbsent}, layoutKnownText("head-only"), yes, ContextUnitNone, "no-shaft-implied"); err != nil {
			return err
		}
	}
	for _, spec := range []struct{ owner, property string }{{"direction", "system"}, {"measure-numbering", "system"}} {
		role := layoutRole(spec.owner + ".@system")
		if !r.attributeOwner(n, role) {
			continue
		}
		raw := localRawAttribute(r, n, role)
		if err := publish("system", raw, raw, yes, ContextUnitNone, "current-system-top-bottom-and-containing-part-selectors"); err != nil {
			return err
		}
	}
	if r.matches(n, layoutRole("page-margins")) {
		raw := r.attribute(n, layoutRole("page-margins.@type"))
		value := raw
		active := yes
		if layoutAncestor(r, n, "print") != nil {
			active.Value = false
		} else {
			value = layoutDefault(raw, "both")
		}
		if err := publish("margin-type", raw, value, active, ContextUnitNone, "physical-page-parity"); err != nil {
			return err
		}
	}
	if r.matches(n, layoutRole("scaling")) {
		mm := r.childScalar(n, layoutRole("millimeters"))
		tenths := r.childScalar(n, layoutRole("tenths"))
		conversion := stateUnknown[Rational]("scaling-relation-unavailable")
		if mm.State == EvidenceKnown && tenths.State == EvidenceKnown {
			var a, b Rational
			var err error
			a, err = s.rationalFromDecimal(r.ctx, mm.Value.Decimal)
			if err != nil {
				return err
			}
			b, err = s.rationalFromDecimal(r.ctx, tenths.Value.Decimal)
			if err != nil {
				return err
			}
			zero, err := s.rationalCmp(r.ctx, b, timelineZero)
			if err != nil {
				return err
			}
			if zero == 0 {
				conversion = stateUnknown[Rational]("zero-scaling-denominator")
			} else {
				v, err := s.rationalQuo(r.ctx, a, b)
				if err != nil {
					return err
				}
				conversion = Evidence[Rational]{State: EvidenceKnown, Value: v}
			}
		} else if mm.State != EvidenceKnown {
			conversion = stateUnavailable[Atomic, Rational](mm)
		} else {
			conversion = stateUnavailable[Atomic, Rational](tenths)
		}
		if err := s.publishContextResult(r.ctx, ContextResult{Provider: key, Subject: ref, Property: "layout.millimeters-per-tenth", Value: stateUnknown[Atomic]("exact-rational-in-layout-conversion"), Layout: &LayoutContextResult{Scope: ref, Origin: ref, Raw: tenths, Applicability: yes, Conversion: conversion, Unit: ContextUnitMillimeters, Basis: "millimeters-per-global-tenth"}}); err != nil {
			return err
		}
	}
	return r.err
}
