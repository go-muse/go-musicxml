package validation

import "math/big"

var layoutTwo = func() Rational { v := big.NewRat(2, 1); return rationalOwned(v, v.RatString()) }()

func layoutRationalScalar(value Evidence[Rational]) Evidence[Atomic] {
	if value.State == EvidenceKnown {
		return Evidence[Atomic]{State: EvidenceAbsent}
	}
	return stateUnavailable[Rational, Atomic](value)
}
func (l *layoutScopes) publishDividers(b timelineMeasureBlock, frame layoutFrame, defaults *factNode) error {
	r := l.r
	explicit, e := l.child(frame.systemLayout, "system-dividers")
	fallback, fe := l.child(defaults, "system-dividers")
	if e.State == EvidenceAbsent && fe.State == EvidenceAbsent {
		return nil
	}
	_, distance, _ := l.effective(frame.systemLayout, defaults, "system-distance", frame.systemCertain)
	half, err := l.rational(distance)
	if err != nil {
		return err
	}
	if half.State == EvidenceKnown {
		half.Value, err = r.s.rationalQuo(r.ctx, half.Value, layoutTwo)
		if err != nil {
			return err
		}
	}
	for _, side := range []string{"left", "right"} {
		divider, de := l.child(explicit, side+"-divider")
		if de.State == EvidenceAbsent {
			divider, de = l.child(fallback, side+"-divider")
		}
		ref := NodeRef{r.s.documentID, b.measure.header.ID}
		from := ref
		if divider != nil {
			from.Node = divider.header.ID
		}
		active := frame.first
		if active.State == EvidenceKnown {
			active.Value = !active.Value
		}
		raw := layoutAttribute(r, divider, "default-y")
		value := half
		if raw.State != EvidenceAbsent {
			value, err = l.rational(raw)
			if err != nil {
				return err
			}
		}
		if de.State != EvidenceKnown && de.State != EvidenceAbsent {
			value = stateUnavailable[bool, Rational](de)
		}
		if err := r.s.publishContextResult(r.ctx, ContextResult{Provider: l.key, Subject: ref, Property: ContextProperty("layout." + side + "-divider-y"), Value: layoutRationalScalar(value), Layout: &LayoutContextResult{Scope: ref, Origin: from, Page: frame.page, System: frame.system, Raw: raw, Applicability: active, Conversion: value, Unit: ContextUnitTenths, Basis: "midway-between-current-and-previous-system-above-current-top"}}); err != nil {
			return err
		}
		raw = layoutAttribute(r, divider, "default-x")
		value, err = l.rational(raw)
		if err != nil {
			return err
		}
		if raw.State == EvidenceAbsent {
			sys, _ := l.child(frame.systemLayout, "system-margins")
			def, _ := l.child(defaults, "system-margins")
			_, v, _ := l.effective(sys, def, side+"-margin", frame.systemCertain)
			value, err = l.rational(v)
			if err != nil {
				return err
			}
		}
		if err := r.s.publishContextResult(r.ctx, ContextResult{Provider: l.key, Subject: ref, Property: ContextProperty("layout." + side + "-divider-x"), Value: layoutRationalScalar(value), Layout: &LayoutContextResult{Scope: ref, Origin: from, Page: frame.page, System: frame.system, Raw: raw, Applicability: active, Conversion: value, Unit: ContextUnitTenths, Basis: "corresponding-system-margin"}}); err != nil {
			return err
		}
	}
	return r.err
}

type layoutSystemWidth struct {
	Node       *factNode
	Frame      layoutFrame
	Width, Sum Evidence[Rational]
	Complete   bool
}

func (l *layoutScopes) systemWidth(frame layoutFrame, pageDefaults, systemDefaults *factNode) (Evidence[Rational], error) {
	_, width, _ := l.effectivePage(frame, pageDefaults, "page-width")
	out, err := l.rational(width)
	if err != nil {
		return out, err
	}
	sys, _ := l.child(frame.systemLayout, "system-margins")
	def, _ := l.child(systemDefaults, "system-margins")
	for _, side := range []string{"left-margin", "right-margin"} {
		page := l.frameMargin(frame, side)
		if !frame.pageCertain && page.State != EvidenceAbsent {
			page = stateUnknown[Atomic]("page-margin-lifetime-unavailable")
		}
		if page.State == EvidenceAbsent {
			page = l.pageMargin(pageDefaults, side, frame.parity, false)
		}
		_, system, _ := l.effective(sys, def, side, frame.systemCertain)
		for _, v := range []Evidence[Atomic]{page, system} {
			margin, e := l.rational(v)
			if e != nil {
				return out, e
			}
			if margin.State != EvidenceKnown {
				out = stateUnavailable[Rational, Rational](margin)
			} else if out.State == EvidenceKnown {
				out.Value, e = l.r.s.rationalSub(l.r.ctx, out.Value, margin.Value)
				if e != nil {
					return out, e
				}
			}
		}
	}
	return out, nil
}
func (l *layoutScopes) addWidth(b timelineMeasureBlock, frame layoutFrame, first Evidence[NodeRef], pageDefaults, systemDefaults *factNode) error {
	if first.State != EvidenceKnown || b.part.State != EvidenceKnown || first.Value != b.part.Value || frame.system.State != EvidenceKnown {
		return nil
	}
	r := l.r
	k := frame.system.Value.Node
	v, ok := l.widths[k]
	if !ok {
		if err := r.s.retain(2048); err != nil {
			return err
		}
		width, err := l.systemWidth(frame, pageDefaults, systemDefaults)
		if err != nil {
			return err
		}
		v = layoutSystemWidth{Node: b.measure, Frame: frame, Width: width, Sum: Evidence[Rational]{State: EvidenceKnown, Value: timelineZero}, Complete: l.systemComplete.State == EvidenceKnown && l.systemComplete.Value}
	}
	raw := layoutAttribute(r, b.measure, "width")
	width, err := l.rational(raw)
	if err != nil {
		return err
	}
	if width.State != EvidenceKnown {
		v.Sum = stateUnavailable[Rational, Rational](width)
	} else if v.Sum.State == EvidenceKnown {
		v.Sum.Value, err = r.s.rationalAdd(r.ctx, v.Sum.Value, width.Value)
		if err != nil {
			return err
		}
	}
	l.widths[k] = v
	return r.err
}
func (l *layoutScopes) publishWidths() error {
	r := l.r
	// Deterministic structural order, using already admitted source node order.
	for _, id := range r.s.sequence {
		if !r.step(1) {
			return r.err
		}
		v, ok := l.widths[id]
		if !ok {
			continue
		}
		value := stateUnknown[Atomic]("system-width-prerequisite-unavailable")
		if v.Width.State == EvidenceKnown && v.Sum.State == EvidenceKnown && v.Complete {
			cmp, err := r.s.rationalCmp(r.ctx, v.Width.Value, v.Sum.Value)
			if err != nil {
				return err
			}
			if cmp == 0 {
				value = layoutKnownText("consistent")
			} else {
				value = stateUnknown[Atomic]("system-width-disagreement-rendering-unspecified")
			}
		}
		if err := l.publish(v.Node, "width-outcome", Evidence[Atomic]{State: EvidenceAbsent}, value, Evidence[bool]{State: EvidenceKnown, Value: true}, NodeRef{r.s.documentID, v.Node.header.ID}, v.Frame, nil); err != nil {
			return err
		}
	}
	return r.err
}
