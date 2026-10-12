package validation

import "context"

// Called by the shared scope finalizer with its already established system
// boundary evidence. Numbering policy remains part-local in both hierarchies.
func layoutStateMeasure(ctx context.Context, s *Session, key RuleKey, b timelineMeasureBlock, r *contextReader, p *contextPlan, systemStart Evidence[bool]) error {
	implicitRaw := localRawAttribute(r, b.measure, layoutStateRole("measure.@implicit"))
	implicit := localDefault(stateYesNo(implicitRaw), false)
	value := stateUnavailable[bool, bool](implicit)
	origin := b.measure
	if implicit.State == EvidenceKnown && implicit.Value {
		value = Evidence[bool]{State: EvidenceKnown, Value: false}
	} else if implicit.State == EvidenceKnown {
		publisher, policy := layoutStateNumberingPolicy(r, key, p, b)
		if publisher != nil {
			origin = publisher
		}
		value = stateUnavailable[string, bool](policy)
		if policy.State == EvidenceKnown {
			switch policy.Value {
			case "none":
				value = Evidence[bool]{State: EvidenceKnown, Value: false}
			case "measure":
				value = Evidence[bool]{State: EvidenceKnown, Value: true}
			case "system":
				value = systemStart
			}
		}
	}
	scope := NodeRef{s.documentID, b.content.header.ID}
	if b.part.State == EvidenceKnown {
		scope = b.part.Value
	}
	ref := NodeRef{s.documentID, b.measure.header.ID}
	return s.publishContextResult(ctx, ContextResult{Provider: key, Subject: ref, Property: "layout.measure-number-display", Value: localBoolAtomic(value), Origins: []Location{origin.header.Location, b.measure.header.Location}, Layout: &LayoutContextResult{Scope: scope, Origin: NodeRef{s.documentID, origin.header.ID}, Raw: implicitRaw, Applicability: Evidence[bool]{State: EvidenceKnown, Value: true}, Unit: ContextUnitNone, Provenance: ContextInherited}})
}
func layoutStateNumberingPolicy(r *contextReader, key RuleKey, p *contextPlan, b timelineMeasureBlock) (*factNode, Evidence[string]) {
	if b.part.State != EvidenceKnown {
		return nil, stateUnavailable[NodeRef, string](b.part)
	}
	index, e := layoutGetStateIndex(r, key, p)
	if e != nil {
		r.err = e
		return nil, stateUnknown[string]("context-interrupted")
	}
	if index.Availability.State != EvidenceKnown {
		return nil, stateUnavailable[bool, string](index.Availability)
	}
	if !r.step(1) {
		return nil, stateUnknown[string]("context-interrupted")
	}
	snapshot, ok := index.Numbering[timelineDomain{b.part.Value.Node, b.ordinal}]
	if !ok {
		return nil, stateUnknown[string]("measure-numbering-domain-unavailable")
	}
	gap, e := layoutLastGlobalGap(r.ctx, r.s, index.UnknownNumbering, b.ordinal)
	if e != nil {
		r.err = e
		return nil, stateUnknown[string]("context-interrupted")
	}
	if gap != nil && gap.Ordinal >= snapshot.Ordinal {
		return nil, stateUnavailable[bool, string](gap.Availability)
	}
	return r.s.nodes[snapshot.Origin.Node], snapshot.Value
}
