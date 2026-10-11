package musicxml

import "strings"

// Canonical lexical forms here are needed for cvc-elt 5.1, not for the source
// tree or a public PSVI. The erratum E1-56 changes information-set contributions;
// it does not replace the canonical lexical validity rules in 5.1.1/5.1.2.
func validationCanonicalBuiltin(name, lexical string) (string, bool) {
	if _, _, integer := validationIntegerLimits(name); integer {
		value, ok := parseValidationInteger(lexical)
		if !ok {
			return "", false
		}
		if value.negative {
			return "-" + value.digits, true
		}
		return value.digits, true
	}
	switch name {
	case "decimal":
		value, ok := parseValidationDecimal(lexical)
		if !ok {
			return "", false
		}
		whole, fraction := value.whole, value.fraction
		if whole == "" {
			whole = "0"
		}
		if fraction == "" {
			fraction = "0"
		}
		if value.negative {
			whole = "-" + whole
		}
		return whole + "." + fraction, true
	case "boolean":
		value, failure := parseValidationBoolean(lexical)
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

func (c *validationContext) canonicalSimpleLiteral(semantics *validationSimpleSemantics, literal string) (string, bool) {
	if semantics == nil {
		c.incomplete("constraint literal type metadata cannot be resolved")
		return "", false
	}
	literal = semantics.normalize(literal)
	if semantics.union != nil {
		for _, member := range semantics.union.Members {
			memberSemantics := c.memberSemantics(member)
			_, failure := c.assessUnionMemberValue(member, literal, literal)
			if validationTechnicalFailure(failure) {
				return "", false
			}
			if failure == nil {
				return c.canonicalSimpleLiteral(memberSemantics, literal)
			}
		}
		c.incomplete("constraint literal is not a member of its declared union")
		return "", false
	}
	if semantics.list != nil || validationBuiltinListItem(semantics.builtin) != "" {
		var itemSemantics *validationSimpleSemantics
		if semantics.list != nil {
			itemSemantics = c.memberSemantics(semantics.list.Item)
		} else {
			itemSemantics = validationBuiltinSemantics(validationBuiltinListItem(semantics.builtin))
		}
		var words []string
		for _, word := range splitValidationList(literal) {
			canonical, ok := c.canonicalSimpleLiteral(itemSemantics, word)
			if !ok {
				return "", false
			}
			words = append(words, canonical)
		}
		return strings.Join(words, " "), true
	}
	if _, failure := c.assessBuiltinValue(semantics.builtin, literal, literal); failure != nil {
		if !validationTechnicalFailure(failure) {
			c.incomplete("constraint literal is invalid for its declared type")
		}
		return "", false
	}
	canonical, ok := validationCanonicalBuiltin(semantics.builtin, literal)
	if !ok {
		c.incomplete("canonical constraint literal assessment is unavailable for xs:" + semantics.builtin)
	}
	return canonical, ok
}

func (c *validationContext) scalarElementType(reference *validationTypeRef) (*validationTypeRef, bool) {
	_, complex, builtin, found := c.resolveType(reference)
	if !found {
		c.incomplete("element scalar type metadata cannot be resolved")
		return nil, false
	}
	if complex != nil {
		effective, ok := c.effectiveComplex(complex)
		if !ok {
			c.incomplete("element scalar content metadata cannot be resolved")
			return nil, false
		}
		if effective.simple == nil {
			return nil, true
		}
		return effective.simple, true
	}
	if builtin == "anyType" {
		return nil, true
	}
	return reference, true
}

func (c *validationContext) canonicalElementConstraint(reference *validationTypeRef, literal string) (string, bool) {
	scalar, ok := c.scalarElementType(reference)
	if !ok {
		return "", false
	}
	if scalar == nil {
		return literal, true
	}
	return c.canonicalSimpleLiteral(c.simpleTypeSemantics(scalar), literal)
}

func (c *validationContext) elementConstraintEqual(actual, declared *validationTypeRef, left, right string) bool {
	before := c.assessmentVersion
	if actual == declared {
		equal := c.elementFixedValuesEqual(actual, left, right)
		return equal || c.assessmentVersion != before
	}
	scalar, ok := c.scalarElementType(actual)
	if !ok {
		return true
	}
	if scalar == nil {
		canonical, ok := c.canonicalElementConstraint(declared, right)
		return !ok || left == canonical
	}
	declarationScalar, ok := c.scalarElementType(declared)
	if !ok {
		return true
	}
	if declarationScalar == nil {
		// A simple dynamic type replacing mixed anyType compares its scalar value
		// with the declaration's string value.
		first, valid := c.simpleLiteralValue(c.simpleTypeSemantics(scalar), left)
		return c.assessmentVersion != before || valid && first.value.equal(validationSimpleValue{domain: validationStringDomain, text: right})
	}
	first, firstOK := c.simpleLiteralValue(c.simpleTypeSemantics(scalar), left)
	second, secondOK := c.simpleLiteralValue(c.simpleTypeSemantics(declarationScalar), right)
	return c.assessmentVersion != before || firstOK && secondOK && first.value.equal(second.value)
}

// cos-valid-default 2 requires simple content, or mixed content with an
// emptiable particle. Even the empty string is not a default for empty or
// element-only complex content. Scalar lexical/facet assessment follows later.
func (c *validationContext) elementDefaultImmediate(reference *validationTypeRef) bool {
	_, complex, _, found := c.resolveType(reference)
	if !found {
		c.incomplete("actual default type metadata cannot be resolved")
		return false
	}
	if complex == nil {
		return true
	}
	effective, ok := c.effectiveComplex(complex)
	if !ok {
		c.incomplete("actual default content metadata cannot be resolved")
		return false
	}
	if effective.simple != nil {
		return true
	}
	if !effective.mixed {
		return false
	}
	if effective.particle == nil {
		return true
	}
	_, failure, matched := c.matchParticle(effective.particle, nil, 0)
	if failure.schema != "" {
		c.incomplete(failure.schema)
	}
	return matched
}
