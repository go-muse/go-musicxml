package musicxml

import "fmt"

// XSD 1.0 sic-id has two independent predicates: declaration-valid eligible
// items seed entry strings; successful actual ID types bind containing elements.
// This bounded 1.0 interpretation does not register selected list/union trials
// or absent default attributes; see docs/validation/xsd-prerequisite-scope.md.
// An owner is the original source node, not its display path.
// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#sic-id
// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-id
// These XSD owners are unrelated to future MusicXML musical reference scopes.
type validationIdentityEntry struct {
	value, path, constraint string
}

type validationIdentityBindings struct {
	owners                map[*validationNode]struct{}
	firstPath, secondPath string
}

// Follow real derivation ancestry. A union/list's selected item or member is
// not an ancestor; genuine complex simple-content extension/restriction is.
func (c *validationContext) identityBuiltin(reference *validationTypeRef) string {
	typeID, found := c.typeIdentity(reference)
	if !found || !c.validTypeMetadata(typeID) {
		return ""
	}
	for {
		switch typeID.builtin {
		case "ID", "IDREF", "IDREFS":
			return typeID.builtin
		}
		base, method, ok := c.typeBase(typeID)
		if !ok || method == 0 {
			return ""
		}
		typeID = base
	}
}

func (c *validationContext) recordElementIdentity(declared, actual *validationTypeRef, value, path string, owner *validationNode, declarationValid bool) {
	if declarationValid {
		c.recordIdentity(declared, value, path)
	}
	c.recordIdentityBinding(actual, value, path, owner)
}

// Called only after successful local declaration validation; this never checks
// multiplicity. Dynamic IDs can precede or follow a separately seeded entry.
func (c *validationContext) recordIdentity(reference *validationTypeRef, value, path string) {
	builtin := c.identityBuiltin(reference)
	normalized := normalizeValidationWhitespace(builtin, value)
	switch builtin {
	case "ID":
		if c.identifiers == nil {
			c.identifiers = make(map[string]string)
		}
		if _, found := c.identifiers[normalized]; !found {
			c.identifiers[normalized] = path
		}
		c.seedIdentity(normalized, path, "ID")
	case "IDREF":
		c.references = append(c.references, validationIdentityReference{value: normalized, path: path})
		c.seedIdentity(normalized, path, "IDREF")
	case "IDREFS":
		for _, item := range splitValidationList(normalized) {
			c.references = append(c.references, validationIdentityReference{value: item, path: path})
			c.seedIdentity(item, path, "IDREF")
		}
	}
}

func (c *validationContext) seedIdentity(value, path, constraint string) {
	if c.identityEntrySeen == nil {
		c.identityEntrySeen = make(map[string]bool)
	}
	if !c.identityEntrySeen[value] {
		c.identityEntrySeen[value] = true
		c.identityEntries = append(c.identityEntries, validationIdentityEntry{value, path, constraint})
	}
}

// Called only after successful actual type validation and usable scalar content.
// A root's scalar has no owner inside the validation scope and cannot self-bind.
func (c *validationContext) recordIdentityBinding(reference *validationTypeRef, value, path string, owner *validationNode) {
	if owner == nil || c.identityBuiltin(reference) != "ID" {
		return
	}
	value = collapseValidationWhitespace(value)
	if c.identityBindings == nil {
		c.identityBindings = make(map[string]*validationIdentityBindings)
	}
	bindings := c.identityBindings[value]
	if bindings == nil {
		bindings = &validationIdentityBindings{owners: make(map[*validationNode]struct{}), firstPath: path}
		c.identityBindings[value] = bindings
	}
	if _, seen := bindings.owners[owner]; seen {
		return
	}
	if len(bindings.owners) == 1 {
		bindings.secondPath = path
	}
	bindings.owners[owner] = struct{}{}
}

// An ordinary invalid content path may skip existing descendants without a
// technical failure. Do not manufacture an absence fact from that partial walk.
// Otherwise accepted anyType/wildcard paths retain their existing limitations;
// expanding their closure requires separate open-content assessment, not this flag.
func (c *validationContext) skipIdentityChildren(node *validationNode) {
	if len(node.Children) != 0 {
		c.identityIncomplete = true
	}
}

func (c *validationContext) validateIdentityReferences() {
	for _, entry := range c.identityEntries {
		bindings := c.identityBindings[entry.value]
		if bindings != nil && len(bindings.owners) > 1 {
			c.addIssue(bindings.secondPath, "ID", fmt.Sprintf("duplicate ID %q; first bound at %s", entry.value, bindings.firstPath))
		} else if bindings == nil && c.assessmentErr == nil && !c.identityIncomplete && entry.constraint == "ID" {
			// XSD 1.0 Second Edition, sic-id (3.15.5), [binding] clause 2
			// binds the containing element, not the ID-valued item itself.
			// Thus root ID content has no binding; cvc-id (3.3.4), clause 1
			// rejects that empty entry even though both tested oracles accept it.
			// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#sic-id
			// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-id
			c.addIssue(entry.path, "ID", fmt.Sprintf("ID %q has no containing element binding in the document", entry.value))
		}
	}
	// Incomplete traversal cannot prove absence. Retain per-reference diagnostic
	// compatibility, including repeated references, but never duplicate owner errors.
	if c.assessmentErr != nil || c.identityIncomplete {
		return
	}
	for _, reference := range c.references {
		if c.identityBindings[reference.value] != nil {
			continue
		}
		c.addIssue(reference.path, "IDREF", fmt.Sprintf("IDREF %q does not identify an element in the document", reference.value))
	}
}
