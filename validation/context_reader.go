package validation

import (
	"context"
	"github.com/go-muse/go-musicxml/internal/xsd"
)

// contextReader borrows admitted schema facts. It owns only its first error;
// callers propagate err at every provider boundary, never substitute absence.
type contextReader struct {
	ctx            context.Context
	s              *Session
	b              *contextBindings
	err            error
	attributeCache map[xsd.Binding]map[contextRole]AttributeSelector
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
	if r.b.Elements != nil {
		if !r.step(int64(len(role)) + contextDeclarationBytes(n.header.Declaration.Value)) {
			return false
		}
		_, ok := r.b.Elements[role][n.header.Declaration.Value]
		return ok
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
	if e == nil && v.State == EvidenceUnsupported && v.Reason == "exact-attribute-use-unavailable" {
		if invalid, ok := r.missingRequiredAttribute(n, a); ok {
			return invalid
		}
	}
	return v
}

// missingRequiredAttribute bridges the deliberate absence of a kernel
// AttributeResult for an omitted required slot. It proves only a selected use
// directly declared by the authoritative actual type, in the same assembly and
// schema graph. Equivalent compiled copies remain portable through catalog
// interning. Inherited omissions remain conservative rather than guessing
// whether a derived type retains or replaces its base attribute requirement.
func (r *contextReader) missingRequiredAttribute(n *factNode, a AttributeSelector) (Evidence[Atomic], bool) {
	if !r.step(contextDeclarationBytes(a.Declaration) + int64(len(a.Name.Space)+len(a.Name.Local)+len(r.b.requiredSchemaDigest))) {
		return Evidence[Atomic]{}, false
	}
	owner, required := r.b.RequiredAttributes[a]
	if !required || !n.closed || n.end.AttributesComplete.State != EvidenceKnown || !n.end.AttributesComplete.Value || r.s.schema == nil || r.s.schema.run == nil || r.s.schema.active == nil || r.s.schema.active.assembly != a.Declaration.Assembly || r.b.requiredSchemaDigest == "" || r.s.schema.active.Digest() != r.b.requiredSchemaDigest {
		return Evidence[Atomic]{}, false
	}
	bound, ok := r.s.schema.nodes[n.header.ID]
	if !ok || !bound.available {
		return Evidence[Atomic]{}, false
	}
	actual, known := r.s.schema.run.ActualType(bound.binding)
	if !known {
		return Evidence[Atomic]{}, false
	}
	actualOwner := r.s.schema.active.component(actual)
	if !r.step(contextComponentBytes(actualOwner)+contextComponentBytes(owner)) || actualOwner != owner {
		return Evidence[Atomic]{}, false
	}
	// Exact expanded-name absence is independent of declaration matching. A
	// present or uncertain same-named fact with the wrong selector cannot be
	// turned into a missing required attribute by attributeEvidence's fallback.
	for _, input := range n.attributes {
		if !r.step(1 + int64(len(input.Name.Space)+len(input.Name.Local))) {
			return Evidence[Atomic]{}, false
		}
		if input.Name == a.Name && input.Presence.State != EvidenceAbsent && (input.Presence.State != EvidenceKnown || input.Presence.Value) {
			return Evidence[Atomic]{}, false
		}
	}
	invalid, err := r.s.schemaInvalidValue(r.ctx, n, XSDCheckAttributes)
	r.err = err
	return invalid, err == nil && invalid.State == EvidenceInvalid && invalid.Cause != nil
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
		if r.b.AttributeRoles != nil {
			if !r.step(1 + int64(len(role))) {
				return AttributeSelector{}, false
			}
			selectors, ready := r.attributeCache[bound.binding]
			if !ready {
				if !r.step(1) {
					return AttributeSelector{}, false
				}
				if r.err = r.s.retain(512); r.err != nil {
					return AttributeSelector{}, false
				}
				selectors = make(map[contextRole]AttributeSelector)
				for attr := range r.s.schema.run.Attributes(bound.binding) {
					d := r.s.schema.active.declaration(attr.Use)
					if !r.step(contextDeclarationBytes(d)) {
						return AttributeSelector{}, false
					}
					for _, selected := range r.b.AttributeRoles[d] {
						if !r.step(1 + int64(len(selected.role))) {
							return AttributeSelector{}, false
						}
						if _, exists := selectors[selected.role]; exists {
							continue
						}
						if r.err = r.s.retain(256 + int64(len(selected.role))); r.err != nil {
							return AttributeSelector{}, false
						}
						selectors[selected.role] = selected.selector
					}
				}
				if !r.step(1) {
					return AttributeSelector{}, false
				}
				if r.attributeCache == nil {
					if r.err = r.s.retain(128); r.err != nil {
						return AttributeSelector{}, false
					}
					r.attributeCache = make(map[xsd.Binding]map[contextRole]AttributeSelector)
				}
				if r.err = r.s.retain(128); r.err != nil {
					return AttributeSelector{}, false
				}
				r.attributeCache[bound.binding] = selectors
			}
			a, ok := selectors[role]
			return a, ok
		}
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
