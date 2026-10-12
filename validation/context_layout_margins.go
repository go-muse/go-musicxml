package validation

func (l *layoutScopes) pageMargin(page *factNode, name string, parity Evidence[bool], isPrint bool) Evidence[Atomic] {
	if page == nil {
		return Evidence[Atomic]{State: EvidenceAbsent}
	}
	r := l.r
	complete := r.enumerated(page)
	if complete.State != EvidenceKnown || !complete.Value {
		return stateUnavailable[bool, Atomic](complete)
	}
	result := Evidence[Atomic]{State: EvidenceAbsent}
	for _, m := range page.children {
		if !r.step(1) {
			return stateUnknown[Atomic]("context-interrupted")
		}
		if !r.matches(m, layoutRole("page-margins")) {
			continue
		}
		eligible := Evidence[bool]{State: EvidenceKnown, Value: true}
		if !isPrint {
			typ := layoutDefault(localRawAttribute(r, m, layoutRole("page-margins.@type")), "both")
			if typ.State != EvidenceKnown {
				eligible = stateUnavailable[Atomic, bool](typ)
			} else if typ.Value.Text != "both" {
				if parity.State != EvidenceKnown {
					eligible = parity
				} else {
					eligible.Value = (typ.Value.Text == "odd") == parity.Value
				}
			}
		}
		if eligible.State == EvidenceKnown && !eligible.Value {
			continue
		}
		v := l.scalar(m, name)
		if eligible.State != EvidenceKnown && v.State != EvidenceAbsent {
			v = stateUnavailable[bool, Atomic](eligible)
		}
		if !r.step(1 + atomicLogicalBytes(result.Value) + atomicLogicalBytes(v.Value)) {
			return stateUnknown[Atomic]("context-interrupted")
		}
		result = layoutConsensusPair(result, v)
	}
	return result
}
func (l *layoutScopes) rational(value Evidence[Atomic]) (Evidence[Rational], error) {
	if value.State != EvidenceKnown {
		return stateUnavailable[Atomic, Rational](value), nil
	}
	if value.Value.Kind != AtomicDecimal {
		return Evidence[Rational]{State: EvidenceUnsupported, Reason: "layout-numeric-value-unavailable"}, nil
	}
	v, err := l.r.s.rationalFromDecimal(l.r.ctx, value.Value.Decimal)
	return Evidence[Rational]{State: EvidenceKnown, Value: v}, err
}
func (l *layoutScopes) publishMargins(b timelineMeasureBlock, frame layoutFrame, pageDefaults, systemDefaults *factNode) error {
	r := l.r
	ref := NodeRef{r.s.documentID, b.measure.header.ID}
	yes := Evidence[bool]{State: EvidenceKnown, Value: true}
	sysOverride, _ := l.child(frame.systemLayout, "system-margins")
	sysDefaults, _ := l.child(systemDefaults, "system-margins")
	for _, name := range []string{"left-margin", "right-margin", "top-margin", "bottom-margin"} {
		raw := l.frameMargin(frame, name)
		value := raw
		origin := frame.pageLayout
		if !frame.pageCertain && raw.State != EvidenceAbsent {
			value = stateUnknown[Atomic]("page-margin-lifetime-unavailable")
		}
		if value.State == EvidenceAbsent {
			value = l.pageMargin(pageDefaults, name, frame.parity, false)
			origin = pageDefaults
		}
		value = layoutUnknownDefault(value, "application-page-margin")
		if err := l.publish(b.measure, "page-"+name, raw, value, yes, ref, frame, origin, b.part); err != nil {
			return err
		}
		if name != "left-margin" && name != "right-margin" {
			continue
		}
		sysRaw, sysValue, origin := l.effective(sysOverride, sysDefaults, name, frame.systemCertain)
		if err := l.publish(b.measure, "system-"+name, sysRaw, sysValue, yes, ref, frame, origin, b.part); err != nil {
			return err
		}
		page, err := l.rational(value)
		if err != nil {
			return err
		}
		system, err := l.rational(sysValue)
		if err != nil {
			return err
		}
		sum := page
		if page.State == EvidenceKnown && system.State == EvidenceKnown {
			v, err := r.s.rationalAdd(r.ctx, page.Value, system.Value)
			if err != nil {
				return err
			}
			sum = Evidence[Rational]{State: EvidenceKnown, Value: v}
		} else if system.State != EvidenceKnown {
			sum = system
		}
		if err := r.s.publishContextResult(r.ctx, ContextResult{Provider: l.key, Subject: ref, Property: ContextProperty("layout.effective-" + name), Value: layoutRationalScalar(sum), Layout: &LayoutContextResult{Part: b.part, Scope: ref, Origin: ref, Raw: sysRaw, Applicability: yes, Page: frame.page, System: frame.system, Conversion: sum, Unit: ContextUnitTenths, Basis: "page-margin-plus-signed-system-inset"}}); err != nil {
			return err
		}
	}
	return r.err
}

func (l *layoutScopes) effectivePage(frame layoutFrame, defaults *factNode, name string) (Evidence[Atomic], Evidence[Atomic], *factNode) {
	raw := Evidence[Atomic]{State: EvidenceAbsent}
	origin := frame.pageLayout
	for _, page := range frame.pagePeers {
		v := l.scalar(page, name)
		if !l.r.step(1 + atomicLogicalBytes(raw.Value) + atomicLogicalBytes(v.Value)) {
			return raw, stateUnknown[Atomic]("context-interrupted"), origin
		}
		raw = layoutConsensusPair(raw, v)
	}
	value := raw
	if !frame.pageCertain && raw.State != EvidenceAbsent {
		value = stateUnknown[Atomic]("page-override-lifetime-unavailable")
	}
	if value.State == EvidenceAbsent {
		value = l.scalar(defaults, name)
		origin = defaults
	}
	return raw, layoutUnknownDefault(value, "application-page-layout"), origin
}
func (l *layoutScopes) frameMargin(frame layoutFrame, name string) Evidence[Atomic] {
	raw := Evidence[Atomic]{State: EvidenceAbsent}
	for _, page := range frame.pagePeers {
		value := l.pageMargin(page, name, frame.parity, true)
		if !l.r.step(1 + atomicLogicalBytes(raw.Value) + atomicLogicalBytes(value.Value)) {
			return stateUnknown[Atomic]("context-interrupted")
		}
		raw = layoutConsensusPair(raw, value)
	}
	return raw
}
