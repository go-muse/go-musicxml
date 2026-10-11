package xsd

import "strings"

// Canonical lexical forms here are needed for cvc-elt 5.1, not for the source
// tree or a public PSVI. The erratum E1-56 changes information-set contributions;
// it does not replace the canonical lexical validity rules in 5.1.1/5.1.2.
func CanonicalBuiltin(name, lexical string) (string, bool) {
	return canonicalBuiltin(name, lexical, nil)
}

func canonicalBuiltin(name, lexical string, c *Context) (string, bool) {
	c.scalar(int64(len(lexical)))
	c.step(1)
	if _, _, integer := IntegerLimits(name); integer {
		value, ok := parseInteger(lexical, c)
		if !ok {
			return "", false
		}
		if value.Negative {
			c.bytes(len(value.Digits))
			size := c.sum(1, int64(len(value.Digits)))
			c.scalar(size)
			c.reserve(size)
			result := "-" + value.Digits
			c.step(0)
			return result, true
		}
		return value.Digits, true
	}
	switch name {
	case "decimal":
		value, ok := parseDecimal(lexical, c)
		if !ok {
			return "", false
		}
		whole, fraction := value.Whole, value.Fraction
		if whole == "" {
			whole = "0"
		}
		if fraction == "" {
			fraction = "0"
		}
		if value.Negative {
			c.bytes(len(whole))
			c.scratch(c.sum(1, int64(len(whole))))
			whole = "-" + whole
			c.step(0)
		}
		c.bytes(len(whole))
		c.bytes(len(fraction))
		size := c.sum(int64(len(whole)), 1, int64(len(fraction)))
		c.scalar(size)
		c.reserve(size)
		result := whole + "." + fraction
		c.step(0)
		return result, true
	case "boolean":
		value, failure := parseBoolean(lexical, c)
		if failure != nil {
			return "", false
		}
		if value {
			return "true", true
		}
		return "false", true
	case "string", "normalizedString", "token", "language", "Name", "NCName", "NMTOKEN", "ID", "IDREF", "ENTITY", "anyURI":
		return lexical, true
	default:
		return "", false
	}
}

func (c *Context) CanonicalSimpleLiteral(semantics *SimpleSemantics, literal string) (string, bool) {
	c.step(1)
	if semantics == nil {
		c.Incomplete("constraint literal type metadata cannot be resolved")
		return "", false
	}
	literal = semantics.normalize(literal, c)
	if semantics.Union != nil {
		for _, member := range semantics.Union.Members {
			c.step(1)
			memberSemantics := c.MemberSemantics(member)
			_, failure := c.AssessUnionMemberValue(member, literal, literal)
			if TechnicalFailure(failure) {
				return "", false
			}
			if failure == nil {
				return c.CanonicalSimpleLiteral(memberSemantics, literal)
			}
		}
		c.Incomplete("constraint literal is not a member of its declared union")
		return "", false
	}
	if semantics.List != nil || BuiltinListItem(semantics.Builtin) != "" {
		var itemSemantics *SimpleSemantics
		if semantics.List != nil {
			itemSemantics = c.MemberSemantics(semantics.List.Item)
		} else {
			itemSemantics = builtinSemantics(BuiltinListItem(semantics.Builtin), c)
		}
		var words []string
		for _, word := range splitList(literal, c) {
			c.step(1)
			canonical, ok := c.CanonicalSimpleLiteral(itemSemantics, word)
			if !ok {
				return "", false
			}
			words = appendScalarWord(words, canonical, c)
		}
		return joinScalarWords(words, c), true
	}
	if _, failure := c.AssessBuiltinValue(semantics.Builtin, literal, literal); failure != nil {
		if !TechnicalFailure(failure) {
			c.Incomplete("constraint literal is invalid for its declared type")
		}
		return "", false
	}
	canonical, ok := canonicalBuiltin(semantics.Builtin, literal, c)
	if !ok {
		c.Incomplete(format(c, "canonical constraint literal assessment is unavailable for xs:%s", semantics.Builtin))
	}
	return canonical, ok
}

func (c *Context) ScalarElementType(reference *TypeRef) (*TypeRef, bool) {
	c.step(1)
	_, complex, builtin, found := c.ResolveType(reference)
	if !found {
		c.Incomplete("element scalar type metadata cannot be resolved")
		return nil, false
	}
	if complex != nil {
		effective, ok := c.EffectiveComplex(complex)
		if !ok {
			c.Incomplete("element scalar content metadata cannot be resolved")
			return nil, false
		}
		if effective.Simple == nil {
			return nil, true
		}
		return effective.Simple, true
	}
	if builtin == "anyType" {
		return nil, true
	}
	return reference, true
}

func (c *Context) CanonicalElementConstraint(reference *TypeRef, literal string) (string, bool) {
	scalar, ok := c.ScalarElementType(reference)
	if !ok {
		return "", false
	}
	if scalar == nil {
		return literal, true
	}
	return c.CanonicalSimpleLiteral(c.SimpleTypeSemantics(scalar), literal)
}

func (c *Context) ElementConstraintEqual(actual, declared *TypeRef, left, right string) bool {
	c.step(1)
	before := c.AssessmentVersion
	if actual == declared {
		equal := c.ElementFixedValuesEqual(actual, left, right)
		return equal || c.AssessmentVersion != before
	}
	scalar, ok := c.ScalarElementType(actual)
	if !ok {
		return true
	}
	if scalar == nil {
		canonical, ok := c.CanonicalElementConstraint(declared, right)
		return !ok || compareText(left, canonical, c) == 0
	}
	declarationScalar, ok := c.ScalarElementType(declared)
	if !ok {
		return true
	}
	if declarationScalar == nil {
		// A simple dynamic type replacing mixed anyType compares its scalar value
		// with the declaration's string value.
		first, valid := c.SimpleLiteralValue(c.SimpleTypeSemantics(scalar), left)
		return c.AssessmentVersion != before || valid && first.Value.equal(SimpleValue{Domain: StringDomain, Text: right}, c)
	}
	first, firstOK := c.SimpleLiteralValue(c.SimpleTypeSemantics(scalar), left)
	second, secondOK := c.SimpleLiteralValue(c.SimpleTypeSemantics(declarationScalar), right)
	return c.AssessmentVersion != before || firstOK && secondOK && first.Value.equal(second.Value, c)
}

// cos-valid-default 2 requires simple content, or mixed content with an
// emptiable particle. Even the empty string is not a default for empty or
// element-only complex content. Scalar lexical/facet assessment follows later.
func (c *Context) ElementDefaultImmediate(reference *TypeRef) bool {
	c.step(1)
	_, complex, _, found := c.ResolveType(reference)
	if !found {
		c.Incomplete("actual default type metadata cannot be resolved")
		return false
	}
	if complex == nil {
		return true
	}
	effective, ok := c.EffectiveComplex(complex)
	if !ok {
		c.Incomplete("actual default content metadata cannot be resolved")
		return false
	}
	if effective.Simple != nil {
		return true
	}
	if !effective.Mixed {
		return false
	}
	if effective.Particle == nil {
		return true
	}
	_, failure, matched := c.MatchParticle(effective.Particle, nil, 0)
	if failure.Schema != "" {
		c.Incomplete(failure.Schema)
	}
	return matched
}

func appendScalarWord(words []string, word string, c *Context) []string {
	if c != nil && c.run != nil && len(words) == cap(words) {
		capacity := c.sum(int64(cap(words)), int64(max(1, cap(words))))
		c.scratch(c.size(capacity, 16))
		next := make([]string, len(words), int(capacity))
		c.bytes(int(c.size(int64(len(words)), 16)))
		copy(next, words)
		c.step(0)
		words = next
	}
	return append(words, word)
}
func joinScalarWords(words []string, c *Context) string {
	if len(words) > 1 {
		size := int64(len(words) - 1)
		for _, word := range words {
			c.step(1)
			size = c.sum(size, int64(len(word)))
			c.bytes(len(word))
		}
		c.scalar(size)
		c.reserve(size)
	}
	result := strings.Join(words, " ")
	c.step(0)
	return result
}
