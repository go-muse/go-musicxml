package musicxml

// A policy is resolved from actual scalar ancestry, never from the legacy
// "string" fallback for lists/unions. Zero denotes member-selected union
// normalization, not collapse and not a successfully resolved atomic policy.
type validationWhitespacePolicy uint8

const (
	validationWhitespaceMember validationWhitespacePolicy = iota
	validationWhitespacePreserve
	validationWhitespaceReplace
	validationWhitespaceCollapse
)

type validationSimpleSemantics struct {
	whitespace  validationWhitespacePolicy
	builtin     string
	stringValue bool
	union       *validationSimpleSchema
}

func (s *validationSimpleSemantics) normalize(value string) string {
	switch s.whitespace {
	case validationWhitespaceReplace:
		return normalizeValidationWhitespace("normalizedString", value)
	case validationWhitespaceCollapse:
		return collapseValidationWhitespace(value)
	default:
		return value
	}
}

func validationBuiltinSemantics(name string) *validationSimpleSemantics {
	result := &validationSimpleSemantics{builtin: name, whitespace: validationWhitespaceCollapse}
	switch name {
	case "string":
		result.whitespace, result.stringValue = validationWhitespacePreserve, true
	case "anySimpleType":
		result.whitespace = validationWhitespacePreserve
	case "normalizedString":
		result.whitespace, result.stringValue = validationWhitespaceReplace, true
	case "token", "language", "Name", "NCName", "NMTOKEN", "ID", "IDREF", "ENTITY":
		result.stringValue = true
	case "NMTOKENS", "IDREFS", "ENTITIES", "boolean", "decimal", "float", "double", "integer", "long", "int", "short", "byte",
		"nonPositiveInteger", "negativeInteger", "nonNegativeInteger", "positiveInteger",
		"unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte", "base64Binary",
		"hexBinary", "anyURI", "QName", "NOTATION", "date", "dateTime", "duration",
		"gDay", "gMonth", "gMonthDay", "gYear", "gYearMonth", "time", "anyType":
	default:
		return nil
	}
	return result
}

func (c *validationContext) simpleTypeSemantics(reference *validationTypeRef) *validationSimpleSemantics {
	simple, complex, builtin, found := c.resolveType(reference)
	if !found || complex != nil {
		return nil
	}
	if simple != nil {
		return c.resolveSimpleSemantics(simple)
	}
	return validationBuiltinSemantics(builtin)
}

func (c *validationContext) memberSemantics(member *validationSimpleMember) *validationSimpleSemantics {
	if member == nil {
		return nil
	}
	if member.Inline != nil {
		return c.resolveSimpleSemantics(member.Inline)
	}
	if member.Name.Space == validationXSDNamespace {
		return validationBuiltinSemantics(member.Name.Local)
	}
	resolved := c.schema.Types[member.Name]
	if resolved == nil || resolved.Simple == nil {
		return nil
	}
	return c.resolveSimpleSemantics(resolved.Simple)
}

// Resolve the complete dependency graph before assessing values. Nil cache
// entries mark both active and failed resolutions, so missing/cyclic metadata
// cannot become a usable policy on a repeated lookup. The cache is per context;
// neither generated definitions nor the effective complex scalar copies change.
func (c *validationContext) resolveSimpleSemantics(schema *validationSimpleSchema) *validationSimpleSemantics {
	if schema == nil {
		return nil
	}
	if known, found := c.simpleSemantics[schema]; found {
		return known
	}
	if c.simpleSemantics == nil {
		c.simpleSemantics = make(map[*validationSimpleSchema]*validationSimpleSemantics)
	}
	c.simpleSemantics[schema] = nil
	var result validationSimpleSemantics
	switch schema.Form {
	case validationSimpleRestriction:
		base := c.memberSemantics(schema.Base)
		if base == nil {
			return nil
		}
		result = *base
		if schema.WhiteSpace != "" {
			// A union has no single whiteSpace policy. Member policies may
			// differ; that is ordinary valid union validation, not an error.
			if result.whitespace == validationWhitespaceMember {
				return nil
			}
			switch schema.WhiteSpace {
			case "preserve":
				result.whitespace = validationWhitespacePreserve
			case "replace":
				result.whitespace = validationWhitespaceReplace
			case "collapse":
				result.whitespace = validationWhitespaceCollapse
			default:
				return nil
			}
		}
	case validationSimpleList:
		if c.memberSemantics(schema.Item) == nil {
			return nil
		}
		result = validationSimpleSemantics{whitespace: validationWhitespaceCollapse}
	case validationSimpleUnion:
		result.union = schema
		if len(schema.Members) == 0 {
			return nil
		}
		for _, member := range schema.Members {
			if c.memberSemantics(member) == nil {
				return nil
			}
		}
	default:
		return nil
	}
	c.simpleSemantics[schema] = &result
	return &result
}

// Keys identify schema literals, never XML instance values. Each context can
// retain at most one entry per reached union-derived enumeration declaration.
// The declaration/index also fixes the original literal and its exact base.
type validationEnumerationKey struct {
	schema *validationSimpleSchema
	index  int
}

type validationEnumerationValue struct {
	value string
	valid bool
}

func (c *validationContext) matchesSimpleEnumeration(schema *validationSimpleSchema, value, builtin string) bool {
	for index, candidate := range schema.Enumerations {
		if _, _, integer := validationIntegerLimits(builtin); integer {
			if validationIntegerValuesEqual(candidate, value) {
				return true
			}
		} else if builtin == "decimal" {
			if validationDecimalValuesEqual(candidate, value) {
				return true
			}
		} else if normalized, valid := c.normalizedEnumeration(schema, index); valid && normalized == value {
			return true
		}
	}
	return false
}

func (c *validationContext) normalizedEnumeration(schema *validationSimpleSchema, index int) (string, bool) {
	base := c.memberSemantics(schema.Base)
	if base == nil {
		return "", false
	}
	literal := schema.Enumerations[index]
	if base.whitespace != validationWhitespaceMember {
		// Facet literals denote values in the declaring restriction's base.
		// Their schema-component validity is a separate concern: reassessing
		// inherited facets here doubles work at every enumeration layer.
		return base.normalize(literal), true
	}
	key := validationEnumerationKey{schema: schema, index: index}
	if known, found := c.enumerationValues[key]; found {
		return known.value, known.valid
	}
	if c.enumerationValues == nil {
		c.enumerationValues = make(map[validationEnumerationKey]validationEnumerationValue)
	}
	c.enumerationValues[key] = validationEnumerationValue{}
	// Restrictions of a union retain its ordered members. Only those members
	// choose the literal's view; outer restriction facets cannot retry member
	// selection. Each member still receives full datatype/facet validation,
	// including any restricted nested union. Cache both success and failure;
	// the empty normalized string is a valid result, not a cache sentinel.
	value, failure := c.validateSimpleValue(base.union, literal, literal)
	result := validationEnumerationValue{value: value, valid: failure == nil}
	c.enumerationValues[key] = result
	return result.value, result.valid
}
