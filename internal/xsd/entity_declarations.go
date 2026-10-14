package xsd

// EntityDeclaration describes an effective general binding. Sequence is a
// document-local declaration identity, never an element SubjectID.
type EntityDeclaration struct {
	Unparsed bool
	Sequence uint64
}

// EntityDeclarations is an immutable borrowed document domain. Its owner admits
// and charges names, indexes and provenance before construction and keeps them
// unchanged until Run.Close. A nil domain is unavailable, never complete-empty.
type EntityDeclarations struct {
	Bindings map[string]EntityDeclaration
	Complete bool
	Reason   string
}

// NewRunWithSourceDeclarations explicitly elects the bounded XSD 1.0 source
// prerequisite, independently of whether the domain itself is available. The
// legacy NewRun behavior remains unchanged. Domain storage belongs to the caller.
func NewRunWithSourceDeclarations(schema *Compiled, meter Meter, domain *EntityDeclarations) (*Run, error) {
	r, err := NewRun(schema, meter)
	if err != nil {
		return nil, err
	}
	r.context.sourceDeclarations = true
	r.context.entityDeclarations = domain
	return r, nil
}

func entityBuiltin(name string) bool { return name == "ENTITY" || name == "ENTITIES" }

// XSD 1.0 Second Edition, Structures 3.14.4 covers these direct types and
// restriction chains. R-200 remains Errata Drafting; bug 10662 explicitly fixes
// XSD 1.1. Do not import that later constructed-list/union rule implicitly.
func (c *Context) entityConstructionUnavailable() *SimpleFailure {
	return c.IncompleteFailure("XSD 1.0 constructed ENTITY-bearing list/union semantics are unavailable")
}
func (c *Context) entityBearingMember(member *SimpleMember) bool {
	c.step(1)
	semantics := c.MemberSemantics(member)
	if semantics == nil {
		return false
	}
	if entityBuiltin(semantics.Builtin) {
		return true
	}
	if semantics.List != nil {
		return c.entityBearingMember(semantics.List.Item)
	}
	if semantics.Union != nil {
		for _, m := range semantics.Union.Members {
			if c.entityBearingMember(m) {
				return true
			}
		}
	}
	return false
}
func (c *Context) entityDomainUnavailable(reason string) *SimpleFailure {
	if reason == "" {
		reason = "source-entity-declarations-unavailable"
	}
	failure := c.IncompleteFailure(reason)
	if c.local != nil {
		limitations := c.local.result().Limitations
		limitations[len(limitations)-1].Reason = reason
		limitations[len(limitations)-1].Unknown = true
	}
	return failure
}
func (c *Context) assessEntityMembership(builtin, lexical string) *SimpleFailure {
	c.step(1)
	c.entityIssueSequence = 0
	if c.entityConstructedDepth != 0 {
		return c.entityConstructionUnavailable()
	}
	domain := c.entityDeclarations
	if domain == nil {
		return c.entityDomainUnavailable("source-entity-declarations-unavailable")
	}
	words := []string{lexical}
	if builtin == "ENTITIES" {
		words = splitList(lexical, c)
	}
	missing := false
	for _, name := range words {
		c.bytes(len(name))
		c.step(1)
		declaration, found := domain.Bindings[name]
		if found && declaration.Unparsed {
			continue
		}
		if !found && !domain.Complete {
			missing = true
			continue
		}
		// Preserve an established negative before any diagnostic formatting/storage
		// or later cancellation can interrupt publication.
		if c.local != nil {
			c.local.result().Valid = false
			if c.local.attribute != (QName{}) {
				c.local.attributeInvalid = true
			}
		}
		if found {
			c.entityIssueSequence = declaration.Sequence
		}
		return &SimpleFailure{Constraint: "entity-declaration", Message: format(c, "value %q does not name an unparsed general entity in the source document", name)}
	}
	if missing {
		return c.entityDomainUnavailable(domain.Reason)
	}
	return nil
}
