package xsd

// validateSchemaLocationHints assesses only the built-in attribute datatypes.
// XSD 1.0 Part 1 §3.2.7 declares schemaLocation as a facet-free list(anyURI)
// and noNamespaceSchemaLocation as anyURI. Zero and odd list lengths are legal
// datatype values. Namespace/location pairing belongs to the separate §4.3.2
// discovery contract, not this fixed-schema assessment. Neither hint selects a
// schema, resolves a URI, or performs I/O.
// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#xsi.schemaLocation
// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#schema-loc
func (c *Context) ValidateSchemaLocationHints(node *Node, path string) {
	for _, attribute := range node.Attrs {
		c.step(1)
		if NamespaceDeclaration(attribute) || attribute.Name.Space != XSINamespace {
			continue
		}
		switch attribute.Name.Local {
		case "schemaLocation":
			// The shared list helper collapses exactly XML's four whitespace
			// characters; Unicode Fields would incorrectly split NBSP and peers.
			for _, item := range splitList(attribute.Value, c) {
				c.step(1)
				if !validXSDAnyURI(item, c) {
					c.AddIssue(AttributePath(path, attribute.Name), "datatype", "schemaLocation must be a list of anyURI values")
					break
				}
			}
		case "noNamespaceSchemaLocation":
			if !validXSDAnyURI(collapseWhitespace(attribute.Value, c), c) {
				c.AddIssue(AttributePath(path, attribute.Name), "datatype", "noNamespaceSchemaLocation must be an anyURI value")
			}
		}
	}
}
