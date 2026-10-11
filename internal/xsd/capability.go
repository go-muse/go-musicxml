package xsd

import "unsafe"

// Source assessment exposes the prerequisite kernel's documented capability
// boundaries. Compatibility Context calls leave these guards inactive.
func (c *Context) capability(family, reason string) {
	c.step(1)
	if c.local == nil {
		return
	}
	old := c.local.family
	c.local.family = family
	defer func() { c.local.family = old }()
	for _, limitation := range c.local.result().Limitations {
		c.step(1)
		if compareText(limitation.Reason, reason, c) == 0 {
			c.local.family = old
			return
		}
	}
	c.local.family = old
	c.local.limit(family, reason, nil)
}
func (c *Context) opaqueType() {
	if c.local == nil {
		return
	}
	c.capability("content", "opaque-any-type-closure-unverified")
	c.capability("attributes", "opaque-any-type-attributes-unverified")
	c.IdentityIncomplete = true
}
func (c *Context) wildcard(mode, family string) {
	if c.local == nil {
		return
	}
	reason := "wildcard-strict-unavailable"
	switch mode {
	case "skip":
		// The schema expressly excludes local subtree validation; that does
		// not prove document-wide identity closure for the skipped subtree.
		c.IdentityIncomplete = true
		return
	case "lax":
		reason = "wildcard-lax-unavailable"
	}
	c.capability(family, reason)
	c.IdentityIncomplete = true
}
func supportedEquality(value SimpleValue) bool { return controlledSupportedEquality(value, nil) }
func controlledSupportedEquality(value SimpleValue, c *Context) bool {
	c.step(1)
	switch value.Domain {
	case StringDomain, NumberDomain, BooleanDomain:
		return true
	case ListDomain:
		for _, item := range value.Items {
			if !controlledSupportedEquality(item, c) {
				return false
			}
		}
		return true
	default:
		return value.Primitive == "anyURI"
	}
}
func (c *Context) equalityAvailable(value SimpleValue) bool {
	if c.local == nil || controlledSupportedEquality(value, c) {
		return true
	}
	c.Incomplete(format(c, "value-space equality is unavailable for xs:%s", value.Primitive))
	return false
}

// Constructed selected members/items are not derivation ancestry. Their wider
// identity participation is unverified, so they cannot establish absent targets.
func (c *Context) constructedIdentity(reference *TypeRef) bool {
	c.step(1)
	if c.local == nil {
		return false
	}
	simple, complex, _, ok := c.ResolveType(reference)
	if !ok {
		return false
	}
	if complex != nil {
		effective, found := c.EffectiveComplex(complex)
		if !found || effective.Simple == nil {
			return false
		}
		return c.constructedIdentity(effective.Simple)
	}
	if simple == nil {
		return false
	}
	sem := c.ResolveSimpleSemantics(simple)
	if sem == nil || !sem.Aggregate() {
		return false
	}
	c.scratch(64)
	seen := map[*SimpleSchema]bool{}
	var member func(*SimpleMember) bool
	var visit func(*SimpleSchema) bool
	member = func(m *SimpleMember) bool {
		c.step(1)
		if m == nil {
			return false
		}
		c.scratch(int64(unsafe.Sizeof(TypeRef{})))
		ref := &TypeRef{Name: m.Name, InlineSimple: m.Inline}
		if c.IdentityBuiltin(ref) != "" {
			return true
		}
		s, _, _, ok := c.ResolveType(ref)
		return ok && s != nil && visit(s)
	}
	visit = func(s *SimpleSchema) bool {
		c.step(1)
		if s == nil || seen[s] {
			return false
		}
		c.scratch(32)
		seen[s] = true
		switch s.Form {
		case SimpleRestriction:
			return member(s.Base)
		case SimpleList:
			return member(s.Item)
		case SimpleUnion:
			for _, m := range s.Members {
				c.step(1)
				if member(m) {
					return true
				}
			}
		}
		return false
	}
	return visit(simple)
}
func (c *Context) noteConstructedIdentity(reference *TypeRef) {
	if c.constructedIdentity(reference) {
		c.capability(c.local.family, "constructed-identity-participation-unverified")
		c.IdentityIncomplete = true
	}
}
func (r *Run) attributeUse(actual *TypeRef, name QName) Use {
	r.context.scratch(64)
	seen := map[TypeID]bool{}
	id, ok := r.typeID(actual)
	for ok {
		r.step(1)
		r.context.bytes(len(id.name.Space))
		r.context.bytes(len(id.name.Local))
		r.context.bytes(len(id.builtin))
		r.context.bytes(len(id.path))
		if seen[id] {
			break
		}
		r.context.scratch(int64(unsafe.Sizeof(TypeID{})) + 32)
		seen[id] = true
		if id.complex == nil {
			return Use{}
		}
		for i, attribute := range id.complex.Attributes {
			r.step(1)
			if compareText(attribute.Name.Space, name.Space, &r.context) == 0 && compareText(attribute.Name.Local, name.Local, &r.context) == 0 {
				return r.schema.attributeUses[attributeUseKey{id, i}]
			}
		}
		if id.complex.Base == nil {
			return Use{}
		}
		id, ok = r.typeID(id.complex.Base)
	}
	return Use{}
}
func (c *Context) absentAttributeFacts(schemas []AttributeSchema, present map[QName]Attr) {
	if c.local == nil || !c.local.subject.AttributesComplete {
		return
	}
	s := c.local
	for _, schema := range schemas {
		c.step(1)
		c.bytes(len(schema.Name.Space))
		c.bytes(len(schema.Name.Local))
		if _, found := present[schema.Name]; found || schema.Use == AttributeProhibited || schema.Use == AttributeRequired {
			continue
		}
		s.attribute = schema.Name
		s.attributeEpoch = c.AssessmentVersion
		s.attributeKnown = true
		s.scalarAssessed = false
		s.scalarAssessment = SimpleAssessment{}
		if schema.Constraint == nil {
			c.attributeResult(schema, "", false, false, true)
			continue
		}
		value := schema.Constraint.Value
		c.scalar(int64(len(value)))
		failure := c.ValidateSimpleType(&schema.Type, value)
		if failure != nil {
			c.Incomplete(format(c, "effective attribute value constraint cannot be assessed: %s", failure.Message))
		}
		identityFact := c.IdentityBuiltin(&schema.Type) != "" || c.constructedIdentity(&schema.Type)
		if identityFact {
			c.identity(1)
		}
		c.attributeResult(schema, value, false, failure == nil, failure == nil)
		last := &s.attributes[len(s.attributes)-1]
		last.Scalar.Defaulted = true
		last.Scalar.Lexeme = ""
		if identityFact {
			c.capability("attributes", "absent-default-identity-participation-unverified")
			c.IdentityIncomplete = true
		}
	}
	s.attribute = QName{}
}

// A full match of an incomplete enumeration supplies safe prefix witnesses only
// when a QName can reach exactly one declaration leaf in the effective grammar.
// Ambiguous alternatives and wildcards cannot establish that exact-use fact.
func (c *Context) stablePrefixMatches(particle *ParticleSchema, matches []ElementMatch) []ElementMatch {
	c.scratch(128)
	counts := make(map[QName]int)
	active := make(map[*ParticleSchema]bool)
	wildcard := false
	var walk func(*ParticleSchema)
	walk = func(p *ParticleSchema) {
		c.step(1)
		if p == nil || active[p] {
			wildcard = true
			return
		}
		c.scratch(32)
		active[p] = true
		defer delete(active, p)
		switch p.Kind {
		case ParticleElement:
			element, ok := c.ResolveElement(p.Element)
			if !ok || element == nil {
				wildcard = true
				return
			}
			c.bytes(len(element.Name.Space))
			c.bytes(len(element.Name.Local))
			if _, found := counts[element.Name]; !found {
				c.scratch(80)
			}
			counts[element.Name]++
		case ParticleAny:
			wildcard = true
		default:
			for _, child := range p.Children {
				c.step(1)
				walk(child)
			}
		}
	}
	walk(particle)
	if wildcard {
		return nil
	}
	var result []ElementMatch
	for _, match := range matches {
		c.step(1)
		if match.Schema != nil {
			c.bytes(len(match.Schema.Name.Space))
			c.bytes(len(match.Schema.Name.Local))
		}
		if match.Schema != nil && counts[match.Schema.Name] == 1 {
			result = appendRetained(c, result, match)
		}
	}
	return result
}
