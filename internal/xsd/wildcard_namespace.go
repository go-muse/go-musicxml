package xsd

// Namespace admission is separate from processContents and declaration lookup.
// Target-dependent tokens cannot be resolved from the element/type name: schema
// composition may give the defining schema a different target namespace.
// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-wildcard-namespace
// See also section 3.10.2 for the token-to-component correspondence.
type wildcardAdmission struct{ allowed, known, valid bool }

func wildcardNamespace(namespace, name string, c *Context) wildcardAdmission {
	tokens, special, valid := wildcardNamespaceTokens(namespace, c)
	if !valid {
		return wildcardAdmission{}
	}
	if special == "any" {
		return wildcardAdmission{true, true, true}
	}
	if special == "other" {
		if name == "" {
			return wildcardAdmission{false, true, true}
		}
		return wildcardAdmission{true, false, true}
	}
	allowed, dependent := false, false
	for _, token := range tokens {
		c.step(1)
		switch token {
		case "##local":
			allowed = allowed || name == ""
		case "##targetNamespace":
			dependent = true
		default:
			allowed = allowed || compareText(token, name, c) == 0
		}
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

// Lexical list members retain their borrowed spelling; target-dependent list
// members remain unresolved and cannot establish negative namespace admission.
func wildcardNamespaceTokens(namespace string, c *Context) ([]string, string, bool) {
	c.step(1)
	if namespace == "" {
		return nil, "any", true
	}
	var tokens []string
	special := ""
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
		switch token {
		case "##any":
			special = "any"
		case "##other":
			special = "other"
		case "##local", "##targetNamespace":
		default:
			if !validXSDAnyURI(token, c) {
				return nil, "", false
			}
		}
		tokens = appendScratch(c, tokens, token)
	}
	if special != "" {
		if len(tokens) != 1 {
			return nil, "", false
		}
		return nil, special, true
	}
	return tokens, "", true
}
