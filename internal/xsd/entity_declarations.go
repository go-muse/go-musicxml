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
