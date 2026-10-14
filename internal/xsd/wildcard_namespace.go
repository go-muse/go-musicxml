package xsd

// Namespace admission is separate from processContents and declaration lookup.
// Target-dependent tokens cannot be resolved from the element/type name: schema
// composition may give the defining schema a different target namespace.
// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-wildcard-namespace
// See also section 3.10.2 for the token-to-component correspondence.
type wildcardAdmission struct{ allowed, known, valid bool }

func wildcardNamespace(namespace, name string, c *Context) wildcardAdmission {
	c.step(1)
	if namespace == "" {
		return wildcardAdmission{true, true, true}
	}
	count := 0
	special := ""
	allowed, dependent := false, false
	for start := 0; start < len(namespace); {
		c.step(1)
		if IsWhitespace(rune(namespace[start])) {
			start++
			continue
		}
		end := start
		for end < len(namespace) && !IsWhitespace(rune(namespace[end])) {
			c.step(1)
			end++
		}
		token := namespace[start:end]
		start = end
		count++
		switch token {
		case "##any", "##other":
			special = token
		case "##local":
			allowed = allowed || name == ""
		case "##targetNamespace":
			dependent = true
		default:
			if !validXSDAnyURI(token, c) {
				return wildcardAdmission{}
			}
			allowed = allowed || compareText(token, name, c) == 0
		}
	}
	if special != "" {
		if count != 1 {
			return wildcardAdmission{}
		}
		if special == "##any" {
			return wildcardAdmission{true, true, true}
		}
		if name == "" {
			return wildcardAdmission{false, true, true}
		}
		return wildcardAdmission{true, false, true}
	}
	// A nonempty explicit lexical list can normalize to the empty set.
	return wildcardAdmission{allowed || dependent, allowed || !dependent, true}
}
func wildcardProcessValid(mode string) bool {
	return mode == "" || mode == "skip" || mode == "lax" || mode == "strict"
}
func (c *Context) noteWildcardAdmission(admission wildcardAdmission, mode string) {
	if !admission.valid || !wildcardProcessValid(mode) {
		c.Incomplete("wildcard namespace or processContents metadata is invalid")
	} else if !admission.known {
		c.Incomplete("wildcard defining target namespace is unavailable")
	}
}
