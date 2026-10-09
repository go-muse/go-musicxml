package musicxml

import (
	"fmt"
	"strings"
)

// validationInteger is an immutable view into a checked integer lexeme. Digits
// has no leading zeros (except zero itself), and zero is never negative. Parsing
// and comparison take linear time and constant auxiliary space: no machine
// conversion, big-number allocation, exponent expansion or arithmetic is needed.
// The caller still owns the input/normalization and diagnostic allocations.
type validationInteger struct {
	digits   string
	negative bool
}

// Keep lexical acceptance aligned with parseXMLUnsignedInteger; that transport
// helper separately preserves machine-conversion values and strconv error details.
func parseValidationInteger(value string) (validationInteger, bool) {
	value = strings.Trim(value, xmlWhitespace)
	negative := false
	if len(value) != 0 && (value[0] == '+' || value[0] == '-') {
		negative = value[0] == '-'
		value = value[1:]
	}
	if value == "" {
		return validationInteger{}, false
	}
	first := len(value) - 1
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return validationInteger{}, false
		}
		if value[index] != '0' && index < first {
			first = index
		}
	}
	value = value[first:]
	return validationInteger{digits: value, negative: negative && value != "0"}, true
}

func (value validationInteger) compare(other validationInteger) int {
	if value.negative != other.negative {
		if value.negative {
			return -1
		}
		return 1
	}
	result := 0
	switch {
	case len(value.digits) < len(other.digits):
		result = -1
	case len(value.digits) > len(other.digits):
		result = 1
	default:
		result = strings.Compare(value.digits, other.digits)
	}
	if value.negative {
		return -result
	}
	return result
}

// The bounds are XSD value-space restrictions, not Go representation policy.
// Signed spellings of the unsigned types preserve the existing library contract.
func validationIntegerLimits(name string) (minimum, maximum string, integer bool) {
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

func validValidationInteger(name, value string) bool {
	number, ok := parseValidationInteger(value)
	if !ok {
		return false
	}
	minimum, maximum, integer := validationIntegerLimits(name)
	if !integer {
		return false
	}
	if minimum != "" {
		limit, _ := parseValidationInteger(minimum)
		if number.compare(limit) < 0 {
			return false
		}
	}
	if maximum != "" {
		limit, _ := parseValidationInteger(maximum)
		if number.compare(limit) > 0 {
			return false
		}
	}
	return true
}

func validationIntegerValuesEqual(left, right string) bool {
	first, firstOK := parseValidationInteger(left)
	second, secondOK := parseValidationInteger(right)
	return firstOK && secondOK && first.compare(second) == 0
}

// Keep bound rules and diagnostics aligned with validateDecimalBounds; parsing is domain-specific.
func validateIntegerBounds(schema *validationSimpleSchema, value string) *validationSimpleFailure {
	number, ok := parseValidationInteger(value)
	if !ok {
		return &validationSimpleFailure{constraint: "datatype", message: fmt.Sprintf("value %q is not numeric", value)}
	}
	tests := []struct {
		enabled            bool
		constraint, limit  string
		minimum, exclusive bool
	}{
		{schema.HasMinInclusive, "minInclusive", schema.MinInclusive, true, false},
		{schema.HasMaxInclusive, "maxInclusive", schema.MaxInclusive, false, false},
		{schema.HasMinExclusive, "minExclusive", schema.MinExclusive, true, true},
		{schema.HasMaxExclusive, "maxExclusive", schema.MaxExclusive, false, true},
	}
	for _, test := range tests {
		if !test.enabled {
			continue
		}
		limit, ok := parseValidationInteger(test.limit)
		if !ok {
			return &validationSimpleFailure{constraint: "schema", message: fmt.Sprintf("invalid %s value %q", test.constraint, test.limit)}
		}
		comparison := number.compare(limit)
		if (test.minimum && comparison < 0) || (!test.minimum && comparison > 0) || (test.exclusive && comparison == 0) {
			return &validationSimpleFailure{constraint: test.constraint, message: fmt.Sprintf("value %q violates %s=%q", value, test.constraint, test.limit)}
		}
	}
	return nil
}
