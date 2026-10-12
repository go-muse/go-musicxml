package validation

import (
	"context"
	"strconv"
)

func layoutPropertyValid(p ContextProperty) bool {
	for _, r := range layoutConstantRules {
		if p == ContextProperty("layout."+r.Property) {
			return true
		}
	}
	for _, e := range layoutElements {
		if e.typ == "tenths" && p == ContextProperty("layout.declared-"+e.name) {
			return true
		}
	}
	for _, a := range layoutAttributes {
		if p == ContextProperty("layout."+a.name) {
			return true
		}
	}
	switch p {
	case "layout.name-display-onset", "layout.kind-text-placement", "layout.opus-new-page", "layout.language", "layout.lyric-placement", "layout.scope", "layout.page-height", "layout.page-width", "layout.system-distance", "layout.top-system-distance", "layout.staff-distance", "layout.measure-distance", "layout.left-margin", "layout.right-margin", "layout.top-margin", "layout.bottom-margin", "layout.staff-number", "layout.margin-type", "layout.millimeters", "layout.tenths", "layout.millimeters-per-tenth", "layout.first-system-on-page", "layout.first-visible-staff", "layout.physical-page", "layout.system", "layout.breaks-complete", "layout.display", "layout.horizontal-anchor", "layout.geometry", "layout.width-outcome", "layout.display-active", "layout.page-left-margin", "layout.page-right-margin", "layout.page-top-margin", "layout.page-bottom-margin", "layout.system-left-margin", "layout.system-right-margin", "layout.effective-left-margin", "layout.effective-right-margin", "layout.left-divider-x", "layout.left-divider-y", "layout.right-divider-x", "layout.right-divider-y":
		return true
	}
	return false
}
func evaluateContextLayout(ctx context.Context, s *Session, n *factNode, key RuleKey, p *contextPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	formatting, err := newLayoutFormatting(r, key)
	if err != nil {
		return Decision{}, false, err
	}
	for _, id := range s.sequence {
		if !r.step(1) {
			return Decision{}, false, r.err
		}
		node := s.nodes[id]
		if node == nil {
			continue
		}
		if err := layoutStatic(r, key, node, p); err != nil {
			return Decision{}, false, err
		}
		if err := formatting.resolve(node); err != nil {
			return Decision{}, false, err
		}
		for i, rule := range layoutConstantRules {
			base := "constant." + strconv.Itoa(i)
			if rule.Element != "" {
				if !r.matches(node, layoutRole(base+".element")) {
					continue
				}
			} else if !r.attributeOwner(node, layoutRole(base+".attribute")) {
				continue
			}
			raw := r.scalar(node)
			if rule.Attribute != "" {
				raw = r.attribute(node, layoutRole(base+".attribute"))
			}
			if r.err != nil {
				return Decision{}, false, r.err
			}
			if err := localPublish(ctx, s, key, node, ContextProperty("layout."+rule.Property), raw, localConstantEffective(rule, raw), Evidence[bool]{State: EvidenceKnown, Value: true}, NodeRef{}, ContextUnitNone); err != nil {
				return Decision{}, false, err
			}
		}
	}
	if r.err == nil {
		r.err = evaluateLayoutScopes(ctx, s, key, p, formatting.defaults)
	}
	return Decision{Outcome: OutcomePass}, false, r.err
}
