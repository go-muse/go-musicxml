package xsd

import (
	"fmt"
	"strings"
)

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
	value = strings.Trim(value, XMLWhitespace)
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
		result = strings.Compare(value.Digits, other.Digits)
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
	number, ok := ParseInteger(value)
	if !ok {
		return false
	}
	minimum, maximum, integer := IntegerLimits(name)
	if !integer {
		return false
	}
	if minimum != "" {
		limit, _ := ParseInteger(minimum)
		if number.Compare(limit) < 0 {
			return false
		}
	}
	if maximum != "" {
		limit, _ := ParseInteger(maximum)
		if number.Compare(limit) > 0 {
			return false
		}
	}
	return true
}

func IntegerValuesEqual(left, right string) bool {
	first, firstOK := ParseInteger(left)
	second, secondOK := ParseInteger(right)
	return firstOK && secondOK && first.Compare(second) == 0
}

// Keep bound rules and diagnostics aligned with validateDecimalBounds; parsing is domain-specific.
func ValidateIntegerBounds(schema *SimpleSchema, value, display string) *SimpleFailure {
	number, ok := ParseInteger(value)
	if !ok {
		return &SimpleFailure{Constraint: "datatype", Message: fmt.Sprintf("value %q is not numeric", display)}
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
		if !test.Enabled {
			continue
		}
		limit, ok := ParseInteger(test.Limit)
		if !ok {
			return &SimpleFailure{Constraint: "schema", Message: fmt.Sprintf("invalid %s value %q", test.Constraint, test.Limit)}
		}
		comparison := number.Compare(limit)
		if (test.Minimum && comparison < 0) || (!test.Minimum && comparison > 0) || (test.Exclusive && comparison == 0) {
			return &SimpleFailure{Constraint: test.Constraint, Message: fmt.Sprintf("value %q violates %s=%q", display, test.Constraint, test.Limit)}
		}
	}
	return nil
}
