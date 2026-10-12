package validation

import "context"

// contextReader borrows admitted schema facts. It owns only its first error;
// callers propagate err at every provider boundary, never substitute absence.
type contextReader struct {
	ctx context.Context
	s   *Session
	b   *contextBindings
	err error
}

func (r *contextReader) step(n int64) bool {
	if r.err == nil {
		r.err = r.s.step(r.ctx, n)
	}
	return r.err == nil
}
func (r *contextReader) sameDeclaration(a, b DeclarationID) bool {
	if !r.step(contextDeclarationBytes(a) + contextDeclarationBytes(b)) {
		return false
	}
	return a == b
}
func (r *contextReader) matches(n *factNode, role contextRole) bool {
	if n == nil || r.b == nil || !r.step(1) || n.header.Declaration.State != EvidenceKnown {
		return false
	}
	for _, u := range r.b.Roles[role] {
		if !u.Attribute && r.sameDeclaration(n.header.Declaration.Value, u.Declaration) {
			return true
		}
	}
	return false
}
func (r *contextReader) enumerated(n *factNode) Evidence[bool] {
	if r.err != nil {
		return stateUnknown[bool]("context-interrupted")
	}
	old := stateReader{ctx: r.ctx, s: r.s}
	v := old.enumerated(n)
	r.err = old.err
	return v
}
func (r *contextReader) complete(n *factNode) Evidence[bool] {
	if r.err != nil {
		return stateUnknown[bool]("context-interrupted")
	}
	old := stateReader{ctx: r.ctx, s: r.s}
	v := old.complete(n)
	r.err = old.err
	return v
}
func (r *contextReader) scalar(n *factNode) Evidence[Atomic] {
	if n == nil {
		return Evidence[Atomic]{State: EvidenceAbsent}
	}
	if r.err != nil {
		return stateUnknown[Atomic]("context-interrupted")
	}
	v, e := r.s.localScalar(r.ctx, n, nil)
	r.err = e
	return v
}
func (r *contextReader) attribute(n *factNode, role contextRole) Evidence[Atomic] {
	if n == nil {
		return Evidence[Atomic]{State: EvidenceAbsent}
	}
	if r.err != nil {
		return stateUnknown[Atomic]("context-interrupted")
	}
	if r.b == nil {
		return Evidence[Atomic]{State: EvidenceUnsupported, Reason: "context-binding-unavailable"}
	}
	a, ok := r.attributeSelector(n, role)
	if r.err != nil {
		return stateUnknown[Atomic]("context-interrupted")
	}
	if !ok {
		if n.header.Type.State != EvidenceKnown {
			return stateUnavailable[ComponentID, Atomic](n.header.Type)
		}
		return Evidence[Atomic]{State: EvidenceUnsupported, Reason: "context-attribute-binding-unavailable"}
	}
	v, e := r.s.localScalar(r.ctx, n, &a)
	r.err = e
	return v
}

// attributeSelector uses compiled owner/type lookup on the common path. The
// uncommon inherited/actual-type path scans its authoritative attribute uses
// once, looking up exact declarations rather than every role candidate.
func (r *contextReader) attributeSelector(n *factNode, role contextRole) (AttributeSelector, bool) {
	if n == nil || r.b == nil || !r.step(1) {
		return AttributeSelector{}, false
	}
	if n.header.Type.State == EvidenceKnown {
		if !r.step(contextComponentBytes(n.header.Type.Value)) {
			return AttributeSelector{}, false
		}
		if a, ok := r.b.Attributes[role][n.header.Type.Value]; ok {
			return a, true
		}
	}
	if r.s.schema != nil && r.s.schema.run != nil {
		bound := r.s.schema.nodes[n.header.ID]
		for attr := range r.s.schema.run.Attributes(bound.binding) {
			d := r.s.schema.active.declaration(attr.Use)
			if !r.step(contextDeclarationBytes(d)) {
				return AttributeSelector{}, false
			}
			if a, ok := r.b.AttributeUses[role][d]; ok {
				return a, true
			}
		}
	}
	return AttributeSelector{}, false
}
func (r *contextReader) attributeOwner(n *factNode, role contextRole) bool {
	_, ok := r.attributeSelector(n, role)
	return ok
}

func (r *contextReader) child(n *factNode, role contextRole) (*factNode, Evidence[bool]) {
	complete := r.complete(n)
	if complete.State != EvidenceKnown || !complete.Value {
		return nil, complete
	}
	var found *factNode
	for _, ch := range n.children {
		if !r.step(1) {
			return nil, stateUnknown[bool]("context-interrupted")
		}
		if ch.header.Declaration.State != EvidenceKnown {
			return nil, stateUnavailable[DeclarationID, bool](ch.header.Declaration)
		}
		if r.matches(ch, role) {
			if found != nil {
				return nil, stateUnknown[bool]("context-multiple-children")
			}
			found = ch
		}
	}
	if found == nil {
		return nil, Evidence[bool]{State: EvidenceAbsent}
	}
	return found, complete
}
func (r *contextReader) childScalar(n *factNode, role contextRole) Evidence[Atomic] {
	ch, e := r.child(n, role)
	if e.State != EvidenceKnown {
		return stateUnavailable[bool, Atomic](e)
	}
	return r.scalar(ch)
}

// presence distinguishes encoded omission from an XSD-effective scalar default.
func (r *contextReader) presence(n *factNode, role contextRole) Evidence[bool] {
	if n == nil {
		return Evidence[bool]{State: EvidenceAbsent}
	}
	a, ok := r.attributeSelector(n, role)
	if r.err != nil {
		return stateUnknown[bool]("context-interrupted")
	}
	if !ok {
		if n.header.Type.State != EvidenceKnown {
			return stateUnavailable[ComponentID, bool](n.header.Type)
		}
		return Evidence[bool]{State: EvidenceUnsupported, Reason: "context-attribute-binding-unavailable"}
	}
	value, _, err := r.s.attributeEvidence(r.ctx, n, a)
	r.err = err
	return value
}
