package validation

const (
	contextNavPlayer   contextRole = "navigation-player-declaration"
	contextNavPlayerID contextRole = "navigation-player-id"
)

func navigationPlayerBindingSpecs() []contextBindingSpec {
	return []contextBindingSpec{{Role: contextNavPlayer, Owner: "score-part", Name: "player", Type: "player", Count: 1}, {Role: contextNavPlayerID, Owner: "player", Name: "id", Type: "xs:ID", Attribute: true, Count: 1}}
}
func navigationResolvePlayer(r *contextReader, part Evidence[NodeRef], raw Evidence[string]) Evidence[NodeRef] {
	if raw.State == EvidenceAbsent {
		return Evidence[NodeRef]{State: EvidenceAbsent}
	}
	if raw.State != EvidenceKnown {
		return stateUnavailable[string, NodeRef](raw)
	}
	if part.State != EvidenceKnown {
		return stateUnavailable[NodeRef, NodeRef](part)
	}
	owner := r.s.nodes[part.Value.Node]
	complete := r.complete(owner)
	if complete.State != EvidenceKnown {
		return stateUnavailable[bool, NodeRef](complete)
	}
	var found *factNode
	for _, ch := range owner.children {
		if !r.matches(ch, contextNavPlayer) {
			continue
		}
		id := stateText(r.attribute(ch, contextNavPlayerID))
		if id.State != EvidenceKnown {
			return stateUnavailable[string, NodeRef](id)
		}
		if !r.step(int64(len(id.Value) + len(raw.Value))) {
			return stateUnknown[NodeRef]("context-interrupted")
		}
		if id.Value != raw.Value {
			continue
		}
		if found != nil {
			return stateUnknown[NodeRef]("navigation-player-ambiguous")
		}
		found = ch
	}
	if found == nil {
		return stateUnknown[NodeRef]("navigation-player-target-unavailable")
	}
	return Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, found.header.ID}}
}
