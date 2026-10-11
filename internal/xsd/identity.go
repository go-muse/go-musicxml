package xsd

import "fmt"

// XSD 1.0 sic-id has two independent predicates: declaration-valid eligible
// items seed entry strings; successful actual ID types bind containing elements.
// This bounded 1.0 interpretation does not register selected list/union trials
// or absent default attributes; see docs/validation/xsd-prerequisite-scope.md.
// An owner is the original source node, not its display path.
// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#sic-id
// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-id
// These XSD owners are unrelated to future MusicXML musical reference scopes.
type identityDiagnostic struct {
	kind  uint8
	index int
}

type identityOrigin struct {
	subject   SubjectID
	attribute QName
}

func (c *Context) origin() identityOrigin {
	if c.local == nil {
		return identityOrigin{}
	}
	return identityOrigin{c.local.subject.ID, c.local.attribute}
}

type IdentityEntry struct {
	origin                  identityOrigin
	Value, Path, Constraint string
}

type IdentityBindings struct {
	firstOrigin, secondOrigin identityOrigin
	Owners                    map[*Node]struct{}
	FirstPath, SecondPath     string
}

// Follow real derivation ancestry. A union/list's selected item or member is
// not an ancestor; genuine complex simple-content extension/restriction is.
func (c *Context) IdentityBuiltin(reference *TypeRef) string {
	typeID, found := c.TypeIdentity(reference)
	if !found || !c.ValidTypeMetadata(typeID) {
		return ""
	}
	for {
		switch typeID.Builtin {
		case "ID", "IDREF", "IDREFS":
			return typeID.Builtin
		}
		base, method, ok := c.TypeBase(typeID)
		if !ok || method == 0 {
			return ""
		}
		typeID = base
	}
}

func (c *Context) RecordElementIdentity(declared, actual *TypeRef, value, path string, owner *Node, declarationValid bool) {
	if declarationValid {
		c.RecordIdentity(declared, value, path)
	}
	c.RecordIdentityBinding(actual, value, path, owner)
}

// Called only after successful local declaration validation; this never checks
// multiplicity. Dynamic IDs can precede or follow a separately seeded entry.
func (c *Context) RecordIdentity(reference *TypeRef, value, path string) {
	c.noteConstructedIdentity(reference)
	builtin := c.IdentityBuiltin(reference)
	normalized := NormalizeWhitespace(builtin, value)
	switch builtin {
	case "ID":
		if c.Identifiers == nil {
			c.Identifiers = make(map[string]string)
		}
		if _, found := c.Identifiers[normalized]; !found {
			c.Identifiers[normalized] = path
		}
		c.SeedIdentity(normalized, path, "ID")
	case "IDREF":
		c.References = append(c.References, IdentityReference{Value: normalized, Path: path, origin: c.origin()})
		c.SeedIdentity(normalized, path, "IDREF")
	case "IDREFS":
		for _, item := range SplitList(normalized) {
			c.References = append(c.References, IdentityReference{Value: item, Path: path, origin: c.origin()})
			c.SeedIdentity(item, path, "IDREF")
		}
	}
}

func (c *Context) SeedIdentity(value, path, constraint string) {
	if c.IdentityEntrySeen == nil {
		c.IdentityEntrySeen = make(map[string]bool)
	}
	if !c.IdentityEntrySeen[value] {
		c.IdentityEntrySeen[value] = true
		c.IdentityEntries = append(c.IdentityEntries, IdentityEntry{Value: value, Path: path, Constraint: constraint, origin: c.origin()})
	}
}

// Called only after successful actual type validation and usable scalar content.
// A root's scalar has no owner inside the validation scope and cannot self-bind.
func (c *Context) RecordIdentityBinding(reference *TypeRef, value, path string, owner *Node) {
	c.noteConstructedIdentity(reference)
	if owner == nil || c.IdentityBuiltin(reference) != "ID" {
		return
	}
	value = CollapseWhitespace(value)
	if c.IdentityBindings == nil {
		c.IdentityBindings = make(map[string]*IdentityBindings)
	}
	bindings := c.IdentityBindings[value]
	if bindings == nil {
		bindings = &IdentityBindings{Owners: make(map[*Node]struct{}), FirstPath: path, firstOrigin: c.origin()}
		c.IdentityBindings[value] = bindings
	}
	if _, seen := bindings.Owners[owner]; seen {
		return
	}
	if len(bindings.Owners) == 1 {
		bindings.SecondPath = path
		bindings.secondOrigin = c.origin()
	}
	bindings.Owners[owner] = struct{}{}
}

// An ordinary invalid content path may skip existing descendants without a
// technical failure. Do not manufacture an absence fact from that partial walk.
// Otherwise accepted anyType/wildcard paths retain their existing limitations;
// expanding their closure requires separate open-content assessment, not this flag.
func (c *Context) SkipIdentityChildren(node *Node) {
	if len(node.Children) != 0 {
		c.IdentityIncomplete = true
	}
}

func (c *Context) ValidateIdentityReferences() {
	for index, entry := range c.IdentityEntries {
		c.identityDiagnostic = identityDiagnostic{kind: 1, index: index}
		bindings := c.IdentityBindings[entry.Value]
		if bindings != nil && len(bindings.Owners) > 1 {
			c.identitySource = bindings.secondOrigin
			c.proveIdentityFailure()
			c.AddIssue(bindings.SecondPath, "ID", fmt.Sprintf("duplicate ID %q; first bound at %s", entry.Value, bindings.FirstPath))
		} else if bindings == nil && c.AssessmentErr == nil && !c.IdentityIncomplete && entry.Constraint == "ID" {
			// XSD 1.0 Second Edition, sic-id (3.15.5), [binding] clause 2
			// binds the containing element, not the ID-valued item itself.
			// Thus root ID content has no binding; cvc-id (3.3.4), clause 1
			// rejects that empty entry even though both tested oracles accept it.
			// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#sic-id
			// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-id
			c.identitySource = entry.origin
			c.proveIdentityFailure()
			c.AddIssue(entry.Path, "ID", fmt.Sprintf("ID %q has no containing element binding in the document", entry.Value))
		}
	}
	// Incomplete traversal cannot prove absence. Retain per-reference diagnostic
	// compatibility, including repeated references, but never duplicate owner errors.
	if c.AssessmentErr != nil || c.IdentityIncomplete {
		return
	}
	for index, reference := range c.References {
		c.identityDiagnostic = identityDiagnostic{kind: 2, index: index}
		if c.IdentityBindings[reference.Value] != nil {
			continue
		}
		c.identitySource = reference.origin
		c.proveIdentityFailure()
		c.AddIssue(reference.Path, "IDREF", fmt.Sprintf("IDREF %q does not identify an element in the document", reference.Value))
	}
}
