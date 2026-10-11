package xsd

// validationDecimal is an immutable view of a checked decimal lexeme. Whole
// digits have no leading zeros, fractional digits have no trailing zeros, and
// zero has two empty digit views and is never negative. The views retain the
// input's storage without copying or expanding a scale. Parsing and comparison
// take linear time and constant auxiliary space; caller normalization, document
// storage and diagnostic allocations are separate.
type Decimal struct {
	Whole, Fraction string
	Negative        bool
}

func ParseDecimal(value string) (Decimal, bool) {
	return parseDecimal(value, nil)
}

func parseDecimal(value string, c *Context) (Decimal, bool) {
	c.scalar(int64(len(value)))
	value = trimXMLWhitespace(value, c)
	negative := false
	if len(value) != 0 && (value[0] == '+' || value[0] == '-') {
		negative = value[0] == '-'
		value = value[1:]
	}
	point := -1
	digits := 0
	for index := range len(value) {
		c.step(1)
		if value[index] == '.' && point == -1 {
			point = index
			continue
		}
		if value[index] < '0' || value[index] > '9' {
			return Decimal{}, false
		}
		digits++
	}
	if digits == 0 {
		return Decimal{}, false
	}
	whole, fraction := value, ""
	if point != -1 {
		whole, fraction = value[:point], value[point+1:]
	}
	for len(whole) != 0 && whole[0] == '0' {
		c.step(1)
		whole = whole[1:]
	}
	for len(fraction) != 0 && fraction[len(fraction)-1] == '0' {
		c.step(1)
		fraction = fraction[:len(fraction)-1]
	}
	return Decimal{Whole: whole, Fraction: fraction, Negative: negative && (whole != "" || fraction != "")}, true
}

func (value Decimal) Compare(other Decimal) int {
	return value.compare(other, nil)
}

func (value Decimal) compare(other Decimal, c *Context) int {
	c.step(1)
	if value.Negative != other.Negative {
		if value.Negative {
			return -1
		}
		return 1
	}
	result := 0
	switch {
	case len(value.Whole) < len(other.Whole):
		result = -1
	case len(value.Whole) > len(other.Whole):
		result = 1
	default:
		result = compareText(value.Whole, other.Whole, c)
		if result == 0 {
			// If one fraction is a prefix, its implicit zero suffix is smaller than
			// the other's remaining suffix, which ends in a nonzero digit.
			result = compareText(value.Fraction, other.Fraction, c)
		}
	}
	if value.Negative {
		return -result
	}
	return result
}

func DecimalValuesEqual(left, right string) bool {
	return decimalValuesEqual(left, right, nil)
}

func decimalValuesEqual(left, right string, c *Context) bool {
	first, firstOK := parseDecimal(left, c)
	second, secondOK := parseDecimal(right, c)
	return firstOK && secondOK && first.compare(second, c) == 0
}

// Keep bound rules and diagnostics aligned with validateIntegerBounds; parsing is domain-specific.
func ValidateDecimalBounds(schema *SimpleSchema, value, display string) *SimpleFailure {
	return validateDecimalBounds(schema, value, display, nil)
}

func validateDecimalBounds(schema *SimpleSchema, value, display string, c *Context) *SimpleFailure {
	number, ok := parseDecimal(value, c)
	if !ok {
		return &SimpleFailure{Constraint: "datatype", Message: format(c, "value %q is not numeric", display)}
	}
	tests := []struct {
		Enabled            bool
		Constraint, Limit  string
		Minimum, Exclusive bool
	}{
		{schema.HasMinInclusive, "minInclusive", schema.MinInclusive, true, false},
		{schema.HasMaxInclusive, "maxInclusive", schema.MaxInclusive, false, false},
		{schema.HasMinExclusive, "minExclusive", schema.MinExclusive, true, true},
		{schema.HasMaxExclusive, "maxExclusive", schema.MaxExclusive, false, true},
	}
	for _, test := range tests {
		c.step(1)
		if !test.Enabled {
			continue
		}
		limit, ok := parseDecimal(test.Limit, c)
		if !ok {
			return &SimpleFailure{Constraint: "schema", Message: format(c, "invalid %s value %q", test.Constraint, test.Limit)}
		}
		comparison := number.compare(limit, c)
		if (test.Minimum && comparison < 0) || (!test.Minimum && comparison > 0) || (test.Exclusive && comparison == 0) {
			return &SimpleFailure{Constraint: test.Constraint, Message: format(c, "value %q violates %s=%q", display, test.Constraint, test.Limit)}
		}
	}
	return nil
}
