package xsd

import (
	"strings"
	"unsafe"
)

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
	return s.normalize(value, nil)
}

func (s *SimpleSemantics) normalize(value string, c *Context) string {
	c.scalar(int64(len(value)))
	c.step(1)
	switch s.Whitespace {
	case WhitespaceReplace:
		return normalizeWhitespace("normalizedString", value, c)
	case WhitespaceCollapse:
		return collapseWhitespace(value, c)
	default:
		return value
	}
}

func BuiltinSemantics(name string) *SimpleSemantics {
	return builtinSemantics(name, nil)
}

func builtinSemantics(name string, c *Context) *SimpleSemantics {
	c.step(1)
	c.scratch(int64(unsafe.Sizeof(SimpleSemantics{})))
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
	c.step(1)
	defer func() {
		if stop := recover(); stop != nil {
			panic(stop)
		}
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
	return builtinSemantics(builtin, c)
}

func (c *Context) MemberSemantics(member *SimpleMember) *SimpleSemantics {
	c.step(1)
	if member == nil {
		return nil
	}
	if member.Inline != nil {
		return c.ResolveSimpleSemantics(member.Inline)
	}
	if member.Name.Space == XSDNamespace {
		return builtinSemantics(member.Name.Local, c)
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
	c.step(1)
	if schema == nil {
		return nil
	}
	if known, found := c.SimpleSemantics[schema]; found {
		return known
	}
	if c.SimpleSemantics == nil {
		c.reserve(64)
		c.SimpleSemantics = make(map[*SimpleSchema]*SimpleSemantics)
	}
	c.reserve(64 + int64(unsafe.Sizeof(SimpleSemantics{})))
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
			c.step(1)
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
		c.step(1)
		candidate := c.EnumerationValue(schema, index)
		if candidate.Valid && candidate.Actual.equal(value.Value, c) {
			return true
		}
	}
	return false
}

func (c *Context) EnumerationValue(schema *SimpleSchema, index int) EnumerationValue {
	c.step(1)
	key := EnumerationKey{Schema: schema, Index: index}
	if known, found := c.EnumerationValues[key]; found {
		if known.Incomplete {
			c.Incomplete("cached enumeration literal assessment is incomplete")
		}
		return known
	}
	if c.EnumerationValues == nil {
		c.reserve(64)
		c.EnumerationValues = make(map[EnumerationKey]EnumerationValue)
	}
	c.reserve(64 + int64(unsafe.Sizeof(EnumerationValue{})))
	c.EnumerationValues[key] = EnumerationValue{}
	// The declaring base, not this restriction or a descendant, determines the
	// literal's whitespace and value. A pending entry is also a fail-closed guard.
	before := c.AssessmentVersion
	assessment, valid := c.SimpleLiteralValue(c.MemberSemantics(schema.Base), schema.Enumerations[index])
	result := EnumerationValue{Value: assessment.Lexical, Actual: assessment.Value, Valid: valid, Incomplete: c.AssessmentVersion != before}
	c.EnumerationValues[key] = result
	return result
}

// Trim only XML whitespace, preserving immutable views of the original string.
func trimXMLWhitespace(value string, c *Context) string {
	start, end := 0, len(value)
	for start < end {
		c.step(1)
		if strings.IndexByte(XMLWhitespace, value[start]) < 0 {
			break
		}
		start++
	}
	for end > start {
		c.step(1)
		if strings.IndexByte(XMLWhitespace, value[end-1]) < 0 {
			break
		}
		end--
	}
	return value[start:end]
}
