package xsd

import (
	"strings"
	"unsafe"
)

// An assessment keeps the lexical subject for patterns separate from its value.
// In particular, a union carries the first successful member's value; comparing
// that member's normalized spelling cannot safely select a member a second time.
type SimpleAssessment struct {
	Builtin string
	Lexical string
	Value   SimpleValue
}

type ValueDomain uint8

const (
	LexicalDomain ValueDomain = iota
	StringDomain
	NumberDomain
	BooleanDomain
	ListDomain
)

// Values are immutable and private to an assessment/context. Numeric values use
// the exact decimal representation shared by XSD integer-derived types. Other
// primitive families retain their distinct primitive identity. Boolean values
// compare by truth value; URI values retain collapsed spelling without URL
// equivalence. Temporal, QName, binary and floating-point same-family equality
// remains lexical, not a claim of complete value-space support. Ur-types remain
// opaque lexical compatibility carriers rather than additional primitives.
type SimpleValue struct {
	Domain ValueDomain
	// primitive distinguishes fallback domains; derived string/integer types
	// instead share the verified string/decimal domains above.
	Primitive string
	Boolean   bool
	Text      string
	Number    Decimal
	Items     []SimpleValue
}

func (value SimpleValue) Equal(other SimpleValue) bool {
	return value.equal(other, nil)
}

func (value SimpleValue) equal(other SimpleValue, c *Context) bool {
	c.step(1)
	if value.Domain != other.Domain {
		return false
	}
	switch value.Domain {
	case BooleanDomain:
		return value.Boolean == other.Boolean
	case NumberDomain:
		return value.Number.compare(other.Number, c) == 0
	case ListDomain:
		if len(value.Items) != len(other.Items) {
			return false
		}
		for index, item := range value.Items {
			if !item.equal(other.Items[index], c) {
				return false
			}
		}
		return true
	default:
		return compareText(value.Primitive, other.Primitive, c) == 0 && compareText(value.Text, other.Text, c) == 0
	}
}

func BuiltinListItem(name string) string {
	switch name {
	case "NMTOKENS":
		return "NMTOKEN"
	case "IDREFS":
		return "IDREF"
	case "ENTITIES":
		return "ENTITY"
	default:
		return ""
	}
}

func AtomicValue(semantics *SimpleSemantics, lexical string) SimpleValue {
	return atomicValue(semantics, lexical, nil)
}

func atomicValue(semantics *SimpleSemantics, lexical string, c *Context) SimpleValue {
	c.step(1)
	value := SimpleValue{Text: lexical, Primitive: semantics.Builtin}
	if semantics.StringValue {
		value.Domain, value.Primitive = StringDomain, ""
		return value
	}
	if semantics.Builtin == "boolean" {
		value.Domain, value.Primitive = BooleanDomain, ""
		value.Boolean, _ = parseBoolean(lexical, c)
		return value
	}
	_, _, integer := IntegerLimits(semantics.Builtin)
	if integer || semantics.Builtin == "decimal" {
		value.Domain, value.Primitive = NumberDomain, ""
		// The caller has already checked this lexical form in its builtin domain.
		value.Number, _ = parseDecimal(lexical, c)
	}
	return value
}

func (c *Context) AssessBuiltinValue(name, value, display string) (SimpleAssessment, *SimpleFailure) {
	if failure := validateBuiltinValue(name, value, display, c); failure != nil {
		if failure.Constraint == "schema" {
			c.Incomplete(failure.Message)
		}
		return SimpleAssessment{}, failure
	}
	if name == "ENTITY" || name == "ENTITIES" {
		return SimpleAssessment{}, c.IncompleteFailure(format(c, "unparsed entity declarations are unavailable for xs:%s", name))
	}
	semantics := builtinSemantics(name, c)
	lexical := semantics.normalize(value, c)
	result := SimpleAssessment{Lexical: lexical, Builtin: name}
	if item := BuiltinListItem(name); item != "" {
		result.Value.Domain = ListDomain
		itemSemantics := builtinSemantics(item, c)
		for _, word := range splitList(lexical, c) {
			c.step(1)
			result.Value.Items = appendSimpleValue(result.Value.Items, atomicValue(itemSemantics, word, c), c)
		}
	} else {
		result.Value = atomicValue(semantics, lexical, c)
	}
	return result, nil
}

// Convert a literal in its declared value space without rechecking the outer
// restriction chain. Rechecking ancestor enumerations for every literal causes
// exponential work. Union traversal expands union-variety members according to
// XSD 1.0; atomic/list member restrictions still receive full assessment and
// decide which member supplies the actual value.
// The same operation obtains fixed operands independently of outer facet
// failure, so an equal value rejected by a pattern does not invent a fixed issue.
func (c *Context) SimpleLiteralValue(semantics *SimpleSemantics, literal string) (SimpleAssessment, bool) {
	c.step(1)
	if semantics == nil {
		c.Incomplete("literal type metadata cannot be resolved")
		return SimpleAssessment{}, false
	}
	literal = semantics.normalize(literal, c)
	if semantics.Union != nil {
		value, failure := c.AssessSimpleValue(semantics.Union, literal, literal)
		return value, failure == nil && c.equalityAvailable(value.Value)
	}
	if semantics.List != nil {
		result := SimpleAssessment{Lexical: literal, Value: SimpleValue{Domain: ListDomain}}
		for _, item := range splitList(literal, c) {
			c.step(1)
			value, valid := c.SimpleLiteralValue(c.MemberSemantics(semantics.List.Item), item)
			if !valid {
				return SimpleAssessment{}, false
			}
			result.Value.Items = appendSimpleValue(result.Value.Items, value.Value, c)
		}
		return result, true
	}
	result, failure := c.AssessBuiltinValue(semantics.Builtin, literal, literal)
	return result, failure == nil && c.equalityAvailable(result.Value)
}

// Fixed comparisons convert both operands per occurrence. Only enumeration
// literals use the schema-keyed cache; instance spellings are never cache keys.
func (c *Context) AggregateValuesEqual(semantics *SimpleSemantics, left, right string) bool {
	first, firstOK := c.SimpleLiteralValue(semantics, left)
	second, secondOK := c.SimpleLiteralValue(semantics, right)
	return firstOK && secondOK && first.Value.equal(second.Value, c)
}

func (s *SimpleSemantics) Aggregate() bool {
	return s.List != nil || s.Union != nil || BuiltinListItem(s.Builtin) != ""
}

// compareText retains the standard lexical ordering while making long shared
// prefixes cancellable. The legacy path remains the allocation-free library call.
func compareText(left, right string, c *Context) int {
	c.step(0)
	if c == nil || c.run == nil {
		return strings.Compare(left, right)
	}
	for index, end := 0, min(len(left), len(right)); index < end; {
		next := index + min(256, end-index)
		c.step(int64(next - index))
		result := strings.Compare(left[index:next], right[index:next])
		c.step(0)
		if result != 0 {
			return result
		}
		index = next
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return 0
}

// Explicit capacity growth keeps the charged backing size ahead of allocation.
// Nil-run callers preserve the historical append allocation behavior.
func appendSimpleValue(values []SimpleValue, value SimpleValue, c *Context) []SimpleValue {
	if c != nil && c.run != nil && len(values) == cap(values) {
		capacity := c.sum(int64(cap(values)), int64(max(1, cap(values))))
		c.reserve(c.size(capacity, int64(unsafe.Sizeof(SimpleValue{}))))
		next := make([]SimpleValue, len(values), int(capacity))
		c.bytes(int(c.size(int64(len(values)), int64(unsafe.Sizeof(SimpleValue{})))))
		copy(next, values)
		c.step(0)
		values = next
	}
	return append(values, value)
}
