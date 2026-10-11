package xsd

// Source assessment exposes the prerequisite kernel's documented capability
// boundaries. Compatibility Context calls leave these guards inactive.
func (c *Context) capability(family, reason string) {
	if c.local == nil {
		return
	}
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
func supportedEquality(value SimpleValue) bool {
	switch value.Domain {
	case StringDomain, NumberDomain, BooleanDomain:
		return true
	case ListDomain:
		for _, item := range value.Items {
			if !supportedEquality(item) {
				return false
			}
		}
		return true
	default:
		return value.Primitive == "anyURI"
	}
}
func (c *Context) equalityAvailable(value SimpleValue) bool {
	if c.local == nil || supportedEquality(value) {
		return true
	}
	c.Incomplete("value-space equality is unavailable for xs:" + value.Primitive)
	return false
}

// Constructed selected members/items are not derivation ancestry. Their wider
// identity participation is unverified, so they cannot establish absent targets.
func (c *Context) constructedIdentity(reference *TypeRef) bool {
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
	seen := map[*SimpleSchema]bool{}
	var member func(*SimpleMember) bool
	var visit func(*SimpleSchema) bool
	member = func(m *SimpleMember) bool {
		if m == nil {
			return false
		}
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
		seen[s] = true
		switch s.Form {
		case SimpleRestriction:
			return member(s.Base)
		case SimpleList:
			return member(s.Item)
		case SimpleUnion:
			for _, m := range s.Members {
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
	seen := map[TypeID]bool{}
	id, ok := r.typeID(actual)
	for ok && !seen[id] {
		r.step(1)
		seen[id] = true
		if id.complex == nil {
			return Use{}
		}
		for i, attribute := range id.complex.Attributes {
			r.step(1)
			if attribute.Name == name {
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
		failure := c.ValidateSimpleType(&schema.Type, value)
		if failure != nil {
			c.Incomplete("effective attribute value constraint cannot be assessed: " + failure.Message)
		}
		c.attributeResult(schema, value, false, failure == nil, failure == nil)
		last := &s.attributes[len(s.attributes)-1]
		last.Scalar.Defaulted = true
		last.Scalar.Lexeme = ""
		if c.IdentityBuiltin(&schema.Type) != "" || c.constructedIdentity(&schema.Type) {
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
		active[p] = true
		defer delete(active, p)
		switch p.Kind {
		case ParticleElement:
			element, ok := c.ResolveElement(p.Element)
			if !ok || element == nil {
				wildcard = true
				return
			}
			counts[element.Name]++
		case ParticleAny:
			wildcard = true
		default:
			for _, child := range p.Children {
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
		if match.Schema != nil && counts[match.Schema.Name] == 1 {
			c.reserve(32)
			result = append(result, match)
		}
	}
	return result
}
