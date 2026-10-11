package xsd

// An assessment keeps the lexical subject for patterns separate from its value.
// In particular, a union carries the first successful member's value; comparing
// that member's normalized spelling cannot safely select a member a second time.
type SimpleAssessment struct {
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
	if value.Domain != other.Domain {
		return false
	}
	switch value.Domain {
	case BooleanDomain:
		return value.Boolean == other.Boolean
	case NumberDomain:
		return value.Number.Compare(other.Number) == 0
	case ListDomain:
		if len(value.Items) != len(other.Items) {
			return false
		}
		for index, item := range value.Items {
			if !item.Equal(other.Items[index]) {
				return false
			}
		}
		return true
	default:
		return value.Primitive == other.Primitive && value.Text == other.Text
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
	value := SimpleValue{Text: lexical, Primitive: semantics.Builtin}
	if semantics.StringValue {
		value.Domain, value.Primitive = StringDomain, ""
		return value
	}
	if semantics.Builtin == "boolean" {
		value.Domain, value.Primitive = BooleanDomain, ""
		value.Boolean, _ = ParseBoolean(lexical)
		return value
	}
	_, _, integer := IntegerLimits(semantics.Builtin)
	if integer || semantics.Builtin == "decimal" {
		value.Domain, value.Primitive = NumberDomain, ""
		// The caller has already checked this lexical form in its builtin domain.
		value.Number, _ = ParseDecimal(lexical)
	}
	return value
}

func (c *Context) AssessBuiltinValue(name, value, display string) (SimpleAssessment, *SimpleFailure) {
	if failure := ValidateBuiltinValue(name, value, display); failure != nil {
		if failure.Constraint == "schema" {
			c.Incomplete(failure.Message)
		}
		return SimpleAssessment{}, failure
	}
	if name == "ENTITY" || name == "ENTITIES" {
		return SimpleAssessment{}, c.IncompleteFailure("unparsed entity declarations are unavailable for xs:" + name)
	}
	semantics := BuiltinSemantics(name)
	lexical := semantics.Normalize(value)
	result := SimpleAssessment{Lexical: lexical}
	if item := BuiltinListItem(name); item != "" {
		result.Value.Domain = ListDomain
		itemSemantics := BuiltinSemantics(item)
		for _, word := range SplitList(lexical) {
			result.Value.Items = append(result.Value.Items, AtomicValue(itemSemantics, word))
		}
	} else {
		result.Value = AtomicValue(semantics, lexical)
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
	if semantics == nil {
		c.Incomplete("literal type metadata cannot be resolved")
		return SimpleAssessment{}, false
	}
	literal = semantics.Normalize(literal)
	if semantics.Union != nil {
		value, failure := c.AssessSimpleValue(semantics.Union, literal, literal)
		return value, failure == nil
	}
	if semantics.List != nil {
		result := SimpleAssessment{Lexical: literal, Value: SimpleValue{Domain: ListDomain}}
		for _, item := range SplitList(literal) {
			value, valid := c.SimpleLiteralValue(c.MemberSemantics(semantics.List.Item), item)
			if !valid {
				return SimpleAssessment{}, false
			}
			result.Value.Items = append(result.Value.Items, value.Value)
		}
		return result, true
	}
	result, failure := c.AssessBuiltinValue(semantics.Builtin, literal, literal)
	return result, failure == nil
}

// Fixed comparisons convert both operands per occurrence. Only enumeration
// literals use the schema-keyed cache; instance spellings are never cache keys.
func (c *Context) AggregateValuesEqual(semantics *SimpleSemantics, left, right string) bool {
	first, firstOK := c.SimpleLiteralValue(semantics, left)
	second, secondOK := c.SimpleLiteralValue(semantics, right)
	return firstOK && secondOK && first.Value.Equal(second.Value)
}

func (s *SimpleSemantics) Aggregate() bool {
	return s.List != nil || s.Union != nil || BuiltinListItem(s.Builtin) != ""
}
