package musicxml

// An assessment keeps the lexical subject for patterns separate from its value.
// In particular, a union carries the first successful member's value; comparing
// that member's normalized spelling cannot safely select a member a second time.
type validationSimpleAssessment struct {
	lexical string
	value   validationSimpleValue
}

type validationValueDomain uint8

const (
	validationLexicalDomain validationValueDomain = iota
	validationStringDomain
	validationNumberDomain
	validationListDomain
)

// Values are immutable and private to an assessment/context. Numeric values use
// the exact decimal representation shared by XSD integer-derived types. Other
// primitive families deliberately retain lexical comparison, not a claim of
// complete boolean, temporal, QName, binary, URI or floating-point value spaces.
type validationSimpleValue struct {
	domain validationValueDomain
	text   string
	number validationDecimal
	items  []validationSimpleValue
}

func (value validationSimpleValue) equal(other validationSimpleValue) bool {
	if value.domain != other.domain {
		return false
	}
	switch value.domain {
	case validationNumberDomain:
		return value.number.compare(other.number) == 0
	case validationListDomain:
		if len(value.items) != len(other.items) {
			return false
		}
		for index, item := range value.items {
			if !item.equal(other.items[index]) {
				return false
			}
		}
		return true
	default:
		return value.text == other.text
	}
}

func validationBuiltinListItem(name string) string {
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

func validationAtomicValue(semantics *validationSimpleSemantics, lexical string) validationSimpleValue {
	value := validationSimpleValue{text: lexical}
	if semantics.stringValue {
		value.domain = validationStringDomain
		return value
	}
	_, _, integer := validationIntegerLimits(semantics.builtin)
	if integer || semantics.builtin == "decimal" {
		value.domain = validationNumberDomain
		// The caller has already checked this lexical form in its builtin domain.
		value.number, _ = parseValidationDecimal(lexical)
	}
	return value
}

func (c *validationContext) assessBuiltinValue(name, value, display string) (validationSimpleAssessment, *validationSimpleFailure) {
	if failure := validateBuiltinValue(name, value, display); failure != nil {
		return validationSimpleAssessment{}, failure
	}
	semantics := validationBuiltinSemantics(name)
	lexical := semantics.normalize(value)
	result := validationSimpleAssessment{lexical: lexical}
	if item := validationBuiltinListItem(name); item != "" {
		result.value.domain = validationListDomain
		itemSemantics := validationBuiltinSemantics(item)
		for _, word := range splitValidationList(lexical) {
			result.value.items = append(result.value.items, validationAtomicValue(itemSemantics, word))
		}
	} else {
		result.value = validationAtomicValue(semantics, lexical)
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
func (c *validationContext) simpleLiteralValue(semantics *validationSimpleSemantics, literal string) (validationSimpleAssessment, bool) {
	if semantics == nil {
		return validationSimpleAssessment{}, false
	}
	literal = semantics.normalize(literal)
	if semantics.union != nil {
		value, failure := c.assessSimpleValue(semantics.union, literal, literal)
		return value, failure == nil
	}
	if semantics.list != nil {
		result := validationSimpleAssessment{lexical: literal, value: validationSimpleValue{domain: validationListDomain}}
		for _, item := range splitValidationList(literal) {
			value, valid := c.simpleLiteralValue(c.memberSemantics(semantics.list.Item), item)
			if !valid {
				return validationSimpleAssessment{}, false
			}
			result.value.items = append(result.value.items, value.value)
		}
		return result, true
	}
	result, failure := c.assessBuiltinValue(semantics.builtin, literal, literal)
	return result, failure == nil
}

// Fixed comparisons convert both operands per occurrence. Only enumeration
// literals use the schema-keyed cache; instance spellings are never cache keys.
func (c *validationContext) aggregateValuesEqual(semantics *validationSimpleSemantics, left, right string) bool {
	first, firstOK := c.simpleLiteralValue(semantics, left)
	second, secondOK := c.simpleLiteralValue(semantics, right)
	return firstOK && secondOK && first.value.equal(second.value)
}

func (s *validationSimpleSemantics) aggregate() bool {
	return s.list != nil || s.union != nil || validationBuiltinListItem(s.builtin) != ""
}
