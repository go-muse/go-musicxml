package validation

import (
	"github.com/go-muse/go-musicxml/internal/xsd"
)

type resourceParentName struct {
	parent NodeID
	name   string
}
type resourceNameOrdinal struct {
	name    string
	ordinal uint64
}
type resourcePartReference struct {
	key  RuleKey
	plan *referencePlan
}
type resourceLinkIndex struct {
	bytes             int64
	position, end     map[NodeID]int
	complete          map[NodeID]Evidence[bool]
	parts             map[NodeID]Evidence[NodeRef]
	next, nextContent map[NodeID]Evidence[NodeRef]
	names             map[string][]NodeID
	ordinals          map[resourceNameOrdinal][]NodeID
}

// Two document-order inverted indexes make every attachment query logarithmic:
// name indexes implement the global ordinal reading; (name,sibling ordinal)
// indexes implement the per-parent XPath reading. They must select the same
// unique node (or both no node). Neither numeric position nor node depth drives
// an allocation, and repeated links do not rescan sibling or descendant trees.
func newResourceLinkIndex(r *contextReader) *resourceLinkIndex {
	i := &resourceLinkIndex{}
	// Other resource families still need local part ownership, but attachment
	// indexes are useful only when an exact link/bookmark occurrence is selected.
	attachments := false
	for _, id := range r.s.sequence {
		for _, role := range r.roles(r.s.nodes[id]) {
			if role == resourceRole("link") || role == resourceRole("bookmark") {
				attachments = true
				break
			}
		}
		if r.err != nil || attachments {
			break
		}
	}
	if r.err != nil {
		return i
	}
	count := int64(len(r.s.sequence))
	perNode := int64(256)
	if attachments {
		perNode = 1408
	}
	size, err := rationalCheckedMul(BudgetBufferedBytes, count+1, perNode)
	if err != nil {
		r.err = err
		return i
	}
	if r.err = r.s.account.Charge(BudgetBufferedBytes, size); r.err != nil {
		return i
	}
	i.bytes += size
	i.parts = make(map[NodeID]Evidence[NodeRef])
	var counts map[resourceParentName]uint64
	if attachments {
		i.position = make(map[NodeID]int)
		i.end = make(map[NodeID]int)
		i.complete = make(map[NodeID]Evidence[bool])
		i.next = make(map[NodeID]Evidence[NodeRef])
		i.nextContent = make(map[NodeID]Evidence[NodeRef])
		i.names = make(map[string][]NodeID)
		i.ordinals = make(map[resourceNameOrdinal][]NodeID)
		counts = make(map[resourceParentName]uint64)
	}
	partReferences := make(map[DeclarationID]resourcePartReference)
	for _, key := range r.s.catalog.orders[r.s.profile.ID] {
		if !r.step(1 + int64(len(key.ID))) {
			return i
		}
		if key.ID != "MX40-LOCAL-part-id-target/use-1" && key.ID != "MX40-LOCAL-part-id-target/use-2" {
			continue
		}
		rule := r.s.catalog.rules[key]
		plan := r.s.catalog.plans[rule.Predicate]
		if rule.Selector.Declaration != nil && plan.kind == planTypedReference {
			partReferences[*rule.Selector.Declaration] = resourcePartReference{key, plan.reference}
		}
	}
	for pos, id := range r.s.sequence {
		if !r.step(1) {
			return i
		}
		n := r.s.nodes[id]
		if n == nil {
			continue
		}
		part := Evidence[NodeRef]{State: EvidenceAbsent}
		if n.header.Parent != 0 {
			part = i.parts[n.header.Parent]
			if part.State == "" {
				part = stateUnknown[NodeRef]("resource-part-owner-unavailable")
			}
		}
		if r.matches(n, resourceRole("score-part")) {
			part = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, id}}
		} else if ref, ok := partReferences[n.header.Declaration.Value]; ok && n.header.Declaration.State == EvidenceKnown {
			d, known := r.s.lookupDecision(decisionID{ref.key, id})
			if !known {
				part = stateUnknown[NodeRef]("local-part-target-provider-unavailable")
			} else if d.Outcome != OutcomePass {
				if d.Cause == nil && d.Outcome == OutcomeFail {
					d.Cause = &DecisionRef{Rule: ref.key, Subject: NodeRef{r.s.documentID, id}}
				}
				part = resourceDecisionNode(r.s, nil, d)
			} else {
				target, decision, err := r.s.referenceResolution(r.ctx, n, ref.plan.Attribute)
				if err != nil {
					r.err = err
					return i
				}
				part = resourceDecisionNode(r.s, target, decision)
			}
		}
		i.parts[id] = part
		if !attachments {
			continue
		}
		i.position[id] = pos
		i.end[id] = pos + 1
		if n.header.Name.Space != "" || n.header.Declaration.State != EvidenceKnown || n.header.Declaration.Value.Assembly != r.b.assembly {
			continue
		}
		name := n.header.Name.Local
		if !r.step(int64(len(name))*4 + 1) {
			return i
		}
		if r.err = r.s.account.Charge(BudgetBufferedBytes, int64(len(name))*4); r.err != nil {
			return i
		}
		i.bytes += int64(len(name)) * 4
		parent := resourceParentName{n.header.Parent, name}
		counts[parent]++
		i.names[name] = append(i.names[name], id)
		ordinal := resourceNameOrdinal{name, counts[parent]}
		i.ordinals[ordinal] = append(i.ordinals[ordinal], id)
	}
	if !attachments {
		return i
	}
	for pos := len(r.s.sequence) - 1; pos >= 0; pos-- {
		if !r.step(1) {
			return i
		}
		id := r.s.sequence[pos]
		n := r.s.nodes[id]
		if n == nil {
			continue
		}
		enumeration := r.enumerated(n)
		complete := enumeration
		next := Evidence[NodeRef]{State: EvidenceAbsent}
		nextContent := next
		for childIndex := len(n.children) - 1; childIndex >= 0; childIndex-- {
			if !r.step(1) {
				return i
			}
			child := n.children[childIndex]
			i.end[id] = max(i.end[id], i.end[child.header.ID])
			if gap := i.complete[child.header.ID]; gap.State != EvidenceKnown || !gap.Value {
				if complete.State == EvidenceKnown {
					complete = gap
				}
			}
			marker := r.matches(child, resourceRole("link")) || r.matches(child, resourceRole("bookmark"))
			if marker {
				i.next[child.header.ID] = next
				i.nextContent[child.header.ID] = nextContent
				if enumeration.State != EvidenceKnown || !enumeration.Value {
					gap := stateUnavailable[bool, NodeRef](enumeration)
					i.next[child.header.ID] = gap
					i.nextContent[child.header.ID] = gap
				}
			}
			next = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, child.header.ID}}
			if !marker {
				nextContent = next
				if child.header.Declaration.State != EvidenceKnown && child.header.Name.Space == "" && (child.header.Name.Local == "link" || child.header.Name.Local == "bookmark") {
					nextContent = stateUnavailable[DeclarationID, NodeRef](child.header.Declaration)
				}
			}
		}
		if n.header.Declaration.State != EvidenceKnown && complete.State == EvidenceKnown {
			complete = stateUnavailable[DeclarationID, bool](n.header.Declaration)
			if complete.State == EvidenceAbsent {
				complete = stateUnknown[bool]("resource-descendant-declaration-unavailable")
			}
		}
		i.complete[id] = complete
	}
	return i
}
func (i *resourceLinkIndex) lower(r *contextReader, nodes []NodeID, position int) int {
	low, high := 0, len(nodes)
	for low < high {
		if !r.step(1) {
			return 0
		}
		middle := low + (high-low)/2
		if i.position[nodes[middle]] < position {
			low = middle + 1
		} else {
			high = middle
		}
	}
	return low
}
func (i *resourceLinkIndex) attachment(r *contextReader, n *factNode, element, position Evidence[Atomic]) Evidence[NodeRef] {
	if element.State == EvidenceAbsent {
		if result, ok := i.next[n.header.ID]; ok {
			return result
		}
		return stateUnknown[NodeRef]("resource-sibling-enumeration-unavailable")
	}
	if element.State != EvidenceKnown {
		return stateUnavailable[Atomic, NodeRef](element)
	}
	if element.Value.Kind != AtomicString {
		return Evidence[NodeRef]{State: EvidenceUnsupported, Reason: "resource-element-name-domain"}
	}
	anchor, ok := i.nextContent[n.header.ID]
	if !ok {
		return stateUnknown[NodeRef]("resource-sibling-enumeration-unavailable")
	}
	if anchor.State != EvidenceKnown {
		return anchor
	}
	if position.State == EvidenceAbsent {
		return stateUnknown[NodeRef]("link-position-default-unspecified")
	}
	if position.State != EvidenceKnown {
		return stateUnavailable[Atomic, NodeRef](position)
	}
	if position.Value.Kind != AtomicInteger || !position.Value.Integer.Valid() {
		return Evidence[NodeRef]{State: EvidenceUnsupported, Reason: "resource-position-domain"}
	}
	name := element.Value.Text
	if !r.step(int64(len(name)) + position.Value.Integer.ByteSize() + 1) {
		return stateUnknown[NodeRef]("context-interrupted")
	}
	if !xsd.ValidXSDNCName(name) {
		return Evidence[NodeRef]{State: EvidenceUnsupported, Reason: "link-element-name-interpretation"}
	}
	if complete := i.complete[anchor.Value.Node]; complete.State != EvidenceKnown || !complete.Value {
		return stateUnavailable[bool, NodeRef](complete)
	}
	start, end := i.position[anchor.Value.Node]+1, i.end[anchor.Value.Node]
	candidates := i.names[name]
	low, high := i.lower(r, candidates, start), i.lower(r, candidates, end)
	ordinal := position.Value.Integer.value
	if ordinal.Sign() <= 0 {
		return Evidence[NodeRef]{State: EvidenceUnsupported, Reason: "resource-position-domain-unavailable"}
	}
	// An arbitrarily large positive integer has no target in either reading;
	// compare magnitude before any host-width conversion or ordinal lookup.
	if !ordinal.IsUint64() || ordinal.Uint64() > uint64(high-low) {
		return Evidence[NodeRef]{State: EvidenceAbsent}
	}
	k := ordinal.Uint64()
	global := candidates[low+int(k-1)]
	localCandidates := i.ordinals[resourceNameOrdinal{name, k}]
	localLow, localHigh := i.lower(r, localCandidates, start), i.lower(r, localCandidates, end)
	if localHigh-localLow != 1 || localCandidates[localLow] != global {
		return stateUnknown[NodeRef]("link-descendant-ordinal-interpretation")
	}
	return Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, global}}
}

func (i *resourceLinkIndex) release(s *Session) {
	if i != nil && i.bytes != 0 {
		_ = s.account.Release(BudgetBufferedBytes, i.bytes)
		i.bytes = 0
	}
}
