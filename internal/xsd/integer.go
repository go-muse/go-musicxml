package xsd

// validationInteger is an immutable view into a checked integer lexeme. Digits
// has no leading zeros (except zero itself), and zero is never negative. Parsing
// and comparison take linear time and constant auxiliary space: no machine
// conversion, big-number allocation, exponent expansion or arithmetic is needed.
// The caller still owns the input/normalization and diagnostic allocations.
type Integer struct {
	Digits   string
	Negative bool
}

// Keep lexical acceptance aligned with parseXMLUnsignedInteger; that transport
// helper separately preserves machine-conversion values and strconv error details.
func ParseInteger(value string) (Integer, bool) {
	return parseInteger(value, nil)
}

func parseInteger(value string, c *Context) (Integer, bool) {
	c.scalar(int64(len(value)))
	value = trimXMLWhitespace(value, c)
	negative := false
	if len(value) != 0 && (value[0] == '+' || value[0] == '-') {
		negative = value[0] == '-'
		value = value[1:]
	}
	if value == "" {
		return Integer{}, false
	}
	first := len(value) - 1
	for index := range len(value) {
		c.step(1)
		if value[index] < '0' || value[index] > '9' {
			return Integer{}, false
		}
		if value[index] != '0' && index < first {
			first = index
		}
	}
	value = value[first:]
	return Integer{Digits: value, Negative: negative && value != "0"}, true
}

func (value Integer) Compare(other Integer) int {
	return value.compare(other, nil)
}

func (value Integer) compare(other Integer, c *Context) int {
	c.step(1)
	if value.Negative != other.Negative {
		if value.Negative {
			return -1
		}
		return 1
	}
	result := 0
	switch {
	case len(value.Digits) < len(other.Digits):
		result = -1
	case len(value.Digits) > len(other.Digits):
		result = 1
	default:
		result = compareText(value.Digits, other.Digits, c)
	}
	if value.Negative {
		return -result
	}
	return result
}

// The bounds are XSD value-space restrictions, not Go representation policy.
// Signed spellings of the unsigned types preserve the existing library contract.
func IntegerLimits(name string) (minimum, maximum string, integer bool) {
	switch name {
	case "integer":
		return "", "", true
	case "nonPositiveInteger":
		return "", "0", true
	case "negativeInteger":
		return "", "-1", true
	case "nonNegativeInteger":
		return "0", "", true
	case "positiveInteger":
		return "1", "", true
	case "long":
		return "-9223372036854775808", "9223372036854775807", true
	case "int":
		return "-2147483648", "2147483647", true
	case "short":
		return "-32768", "32767", true
	case "byte":
		return "-128", "127", true
	case "unsignedLong":
		return "0", "18446744073709551615", true
	case "unsignedInt":
		return "0", "4294967295", true
	case "unsignedShort":
		return "0", "65535", true
	case "unsignedByte":
		return "0", "255", true
	default:
		return "", "", false
	}
}

func ValidInteger(name, value string) bool {
	return validInteger(name, value, nil)
}

func validInteger(name, value string, c *Context) bool {
	number, ok := parseInteger(value, c)
	if !ok {
		return false
	}
	minimum, maximum, integer := IntegerLimits(name)
	if !integer {
		return false
	}
	if minimum != "" {
		limit, _ := parseInteger(minimum, c)
		if number.compare(limit, c) < 0 {
			return false
		}
	}
	if maximum != "" {
		limit, _ := parseInteger(maximum, c)
		if number.compare(limit, c) > 0 {
			return false
		}
	}
	return true
}

func IntegerValuesEqual(left, right string) bool {
	return integerValuesEqual(left, right, nil)
}

func integerValuesEqual(left, right string, c *Context) bool {
	first, firstOK := parseInteger(left, c)
	second, secondOK := parseInteger(right, c)
	return firstOK && secondOK && first.compare(second, c) == 0
}

// Keep bound rules and diagnostics aligned with validateDecimalBounds; parsing is domain-specific.
func ValidateIntegerBounds(schema *SimpleSchema, value, display string) *SimpleFailure {
	return validateIntegerBounds(schema, value, display, nil)
}

func validateIntegerBounds(schema *SimpleSchema, value, display string, c *Context) *SimpleFailure {
	number, ok := parseInteger(value, c)
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
		limit, ok := parseInteger(test.Limit, c)
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
