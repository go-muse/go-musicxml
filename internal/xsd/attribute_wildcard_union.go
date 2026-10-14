package xsd

import "strings"

// MergeAnyAttribute is the legacy metadata-only helper. Representable unions
// are exact. Its historical derived-wildcard fallback for unavailable unions
// cannot express a technical outcome; validation drivers use mergeAnyAttribute
// and retain its separate unavailable state instead.
func MergeAnyAttribute(base, derived *AnyAttributeSchema) *AnyAttributeSchema {
	result, _ := mergeAnyAttribute(base, derived, nil)
	return result
}

// XSD 1.0, 3.4.2 (attribute wildcard, extension) uses the complete derived
// wildcard's processContents, and 3.10.6 unions the namespace constraints.
// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#Complex_Type_Definition_details
// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-aw-union
// The second result is unavailability, not an instance-validity verdict.
func mergeAnyAttribute(base, derived *AnyAttributeSchema, c *Context) (*AnyAttributeSchema, bool) {
	c.step(1)
	if base == nil {
		return derived, false
	}
	if derived == nil {
		return base, false
	}
	left, leftKind := wildcardUnionTokens(base.Namespace, c)
	right, rightKind := wildcardUnionTokens(derived.Namespace, c)
	if leftKind == "invalid" || rightKind == "invalid" || !wildcardProcessValid(base.ProcessContents) || !wildcardProcessValid(derived.ProcessContents) {
		return derived, true
	}
	// No target-namespace provenance is needed to prove union with any.
	if leftKind == "any" || rightKind == "any" {
		c.reserve(32)
		return &AnyAttributeSchema{Namespace: "##any", ProcessContents: derived.ProcessContents}, false
	}
	// ##other is a negation tied to its defining schema. Neither equal token
	// spellings nor an instance/type name establish those defining namespaces.
	// Without them an apparent union can be inexpressible in XSD 1.0 (5.3).
	if leftKind == "other" || rightKind == "other" {
		return derived, true
	}
	c.scratch(c.sum(128, c.size(c.sum(int64(len(left)), int64(len(right))), 96)))
	seen := make(map[string]bool, len(left)+len(right))
	var tokens []string
	length := int64(0)
	for _, list := range [][]string{left, right} {
		for _, token := range list {
			c.step(1)
			c.bytes(len(token)) // Hashing and equality inspect the token bytes.
			if seen[token] {
				continue
			}
			c.bytes(len(token))
			seen[token] = true
			tokens = appendScratch(c, tokens, token)
			length = c.sum(length, int64(len(token)), 1)
		}
	}
	// A lexical empty set must stay a set, not become the default ##any value.
	if len(tokens) == 0 {
		c.reserve(32)
		return &AnyAttributeSchema{Namespace: " ", ProcessContents: derived.ProcessContents}, false
	}
	length--
	c.reserve(c.sum(32, c.builderAllowance(length)))
	var union strings.Builder
	union.Grow(int(length))
	c.step(0)
	for i, token := range tokens {
		c.bytes(len(token))
		if i != 0 {
			union.WriteByte(' ')
		}
		union.WriteString(token)
		c.step(0)
	}
	return &AnyAttributeSchema{Namespace: union.String(), ProcessContents: derived.ProcessContents}, false
}

// Lexical list members retain their borrowed spelling; target-dependent list
// members remain unresolved and cannot establish negative namespace admission.
func wildcardUnionTokens(namespace string, c *Context) ([]string, string) {
	c.step(1)
	if namespace == "" {
		return nil, "any"
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
				return nil, "invalid"
			}
		}
		tokens = appendScratch(c, tokens, token)
	}
	if special != "" {
		if len(tokens) != 1 {
			return nil, "invalid"
		}
		return nil, special
	}
	return tokens, "set"
}
