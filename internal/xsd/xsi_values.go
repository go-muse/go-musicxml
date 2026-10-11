package xsd

import "strings"

// Canonical lexical forms here are needed for cvc-elt 5.1, not for the source
// tree or a public PSVI. The erratum E1-56 changes information-set contributions;
// it does not replace the canonical lexical validity rules in 5.1.1/5.1.2.
func CanonicalBuiltin(name, lexical string) (string, bool) {
	if _, _, integer := IntegerLimits(name); integer {
		value, ok := ParseInteger(lexical)
		if !ok {
			return "", false
		}
		if value.Negative {
			return "-" + value.Digits, true
		}
		return value.Digits, true
	}
	switch name {
	case "decimal":
		value, ok := ParseDecimal(lexical)
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
			whole = "-" + whole
		}
		return whole + "." + fraction, true
	case "boolean":
		value, failure := ParseBoolean(lexical)
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
	if semantics == nil {
		c.Incomplete("constraint literal type metadata cannot be resolved")
		return "", false
	}
	literal = semantics.Normalize(literal)
	if semantics.Union != nil {
		for _, member := range semantics.Union.Members {
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
			itemSemantics = BuiltinSemantics(BuiltinListItem(semantics.Builtin))
		}
		var words []string
		for _, word := range SplitList(literal) {
			canonical, ok := c.CanonicalSimpleLiteral(itemSemantics, word)
			if !ok {
				return "", false
			}
			words = append(words, canonical)
		}
		return strings.Join(words, " "), true
	}
	if _, failure := c.AssessBuiltinValue(semantics.Builtin, literal, literal); failure != nil {
		if !TechnicalFailure(failure) {
			c.Incomplete("constraint literal is invalid for its declared type")
		}
		return "", false
	}
	canonical, ok := CanonicalBuiltin(semantics.Builtin, literal)
	if !ok {
		c.Incomplete("canonical constraint literal assessment is unavailable for xs:" + semantics.Builtin)
	}
	return canonical, ok
}

func (c *Context) ScalarElementType(reference *TypeRef) (*TypeRef, bool) {
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
		return !ok || left == canonical
	}
	declarationScalar, ok := c.ScalarElementType(declared)
	if !ok {
		return true
	}
	if declarationScalar == nil {
		// A simple dynamic type replacing mixed anyType compares its scalar value
		// with the declaration's string value.
		first, valid := c.SimpleLiteralValue(c.SimpleTypeSemantics(scalar), left)
		return c.AssessmentVersion != before || valid && first.Value.Equal(SimpleValue{Domain: StringDomain, Text: right})
	}
	first, firstOK := c.SimpleLiteralValue(c.SimpleTypeSemantics(scalar), left)
	second, secondOK := c.SimpleLiteralValue(c.SimpleTypeSemantics(declarationScalar), right)
	return c.AssessmentVersion != before || firstOK && secondOK && first.Value.Equal(second.Value)
}

// cos-valid-default 2 requires simple content, or mixed content with an
// emptiable particle. Even the empty string is not a default for empty or
// element-only complex content. Scalar lexical/facet assessment follows later.
func (c *Context) ElementDefaultImmediate(reference *TypeRef) bool {
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
