package xsd

// A policy is resolved from actual scalar ancestry, never from the legacy
// "string" fallback for lists/unions. Zero denotes member-selected union
// normalization, not collapse and not a successfully resolved atomic policy.
type WhitespacePolicy uint8

const (
	WhitespaceMember WhitespacePolicy = iota
	WhitespacePreserve
	WhitespaceReplace
	WhitespaceCollapse
)

type SimpleSemantics struct {
	Whitespace  WhitespacePolicy
	Builtin     string
	StringValue bool
	Union       *SimpleSchema
	List        *SimpleSchema
}

func (s *SimpleSemantics) Normalize(value string) string {
	switch s.Whitespace {
	case WhitespaceReplace:
		return NormalizeWhitespace("normalizedString", value)
	case WhitespaceCollapse:
		return CollapseWhitespace(value)
	default:
		return value
	}
}

func BuiltinSemantics(name string) *SimpleSemantics {
	result := &SimpleSemantics{Builtin: name, Whitespace: WhitespaceCollapse}
	switch name {
	case "string":
		result.Whitespace, result.StringValue = WhitespacePreserve, true
	case "anySimpleType":
		result.Whitespace = WhitespacePreserve
	case "normalizedString":
		result.Whitespace, result.StringValue = WhitespaceReplace, true
	case "token", "language", "Name", "NCName", "NMTOKEN", "ID", "IDREF", "ENTITY":
		result.StringValue = true
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

func (c *Context) SimpleTypeSemantics(reference *TypeRef) (result *SimpleSemantics) {
	defer func() {
		if result == nil {
			c.Incomplete("simple type metadata cannot be resolved")
		}
	}()
	simple, complex, builtin, found := c.ResolveType(reference)
	if !found || complex != nil {
		return nil
	}
	if simple != nil {
		return c.ResolveSimpleSemantics(simple)
	}
	return BuiltinSemantics(builtin)
}

func (c *Context) MemberSemantics(member *SimpleMember) *SimpleSemantics {
	if member == nil {
		return nil
	}
	if member.Inline != nil {
		return c.ResolveSimpleSemantics(member.Inline)
	}
	if member.Name.Space == XSDNamespace {
		return BuiltinSemantics(member.Name.Local)
	}
	resolved := c.Schema.Types[member.Name]
	if resolved == nil || resolved.Simple == nil {
		return nil
	}
	return c.ResolveSimpleSemantics(resolved.Simple)
}

// Resolve the complete dependency graph before assessing values. Nil cache
// entries mark both active and failed resolutions, so missing/cyclic metadata
// cannot become a usable policy on a repeated lookup. The cache is per context;
// neither generated definitions nor the effective complex scalar copies change.
func (c *Context) ResolveSimpleSemantics(schema *SimpleSchema) *SimpleSemantics {
	result := c.ResolveSimpleSemanticsUnchecked(schema)
	if result == nil {
		c.Incomplete("simple type metadata is missing, cyclic or unsupported")
	}
	return result
}

func (c *Context) ResolveSimpleSemanticsUnchecked(schema *SimpleSchema) *SimpleSemantics {
	if schema == nil {
		return nil
	}
	if known, found := c.SimpleSemantics[schema]; found {
		return known
	}
	if c.SimpleSemantics == nil {
		c.SimpleSemantics = make(map[*SimpleSchema]*SimpleSemantics)
	}
	c.SimpleSemantics[schema] = nil
	var result SimpleSemantics
	switch schema.Form {
	case SimpleRestriction:
		base := c.MemberSemantics(schema.Base)
		if base == nil {
			return nil
		}
		result = *base
		if schema.WhiteSpace != "" {
			// A union has no single whiteSpace policy. Member policies may
			// differ; that is ordinary valid union validation, not an error.
			if result.Whitespace == WhitespaceMember {
				return nil
			}
			switch schema.WhiteSpace {
			case "preserve":
				result.Whitespace = WhitespacePreserve
			case "replace":
				result.Whitespace = WhitespaceReplace
			case "collapse":
				result.Whitespace = WhitespaceCollapse
			default:
				return nil
			}
		}
	case SimpleList:
		if c.MemberSemantics(schema.Item) == nil {
			return nil
		}
		result = SimpleSemantics{Whitespace: WhitespaceCollapse, List: schema}
	case SimpleUnion:
		result.Union = schema
		if len(schema.Members) == 0 {
			return nil
		}
		for _, member := range schema.Members {
			if c.MemberSemantics(member) == nil {
				return nil
			}
		}
	default:
		return nil
	}
	c.SimpleSemantics[schema] = &result
	return &result
}

// Keys identify schema literals, never XML instance values. Cache growth is
// bounded by reached declarations/indices, including failed and empty literals.
type EnumerationKey struct {
	Schema *SimpleSchema
	Index  int
}

type EnumerationValue struct {
	Value      string
	Actual     SimpleValue
	Valid      bool
	Incomplete bool
}

func (c *Context) MatchesSimpleEnumeration(schema *SimpleSchema, value SimpleAssessment) bool {
	for index := range schema.Enumerations {
		candidate := c.EnumerationValue(schema, index)
		if candidate.Valid && candidate.Actual.Equal(value.Value) {
			return true
		}
	}
	return false
}

func (c *Context) EnumerationValue(schema *SimpleSchema, index int) EnumerationValue {
	key := EnumerationKey{Schema: schema, Index: index}
	if known, found := c.EnumerationValues[key]; found {
		if known.Incomplete {
			c.Incomplete("cached enumeration literal assessment is incomplete")
		}
		return known
	}
	if c.EnumerationValues == nil {
		c.EnumerationValues = make(map[EnumerationKey]EnumerationValue)
	}
	c.EnumerationValues[key] = EnumerationValue{}
	// The declaring base, not this restriction or a descendant, determines the
	// literal's whitespace and value. A pending entry is also a fail-closed guard.
	before := c.AssessmentVersion
	assessment, valid := c.SimpleLiteralValue(c.MemberSemantics(schema.Base), schema.Enumerations[index])
	result := EnumerationValue{Value: assessment.Lexical, Actual: assessment.Value, Valid: valid, Incomplete: c.AssessmentVersion != before}
	c.EnumerationValues[key] = result
	return result
}
