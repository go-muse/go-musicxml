package musicxml

import (
	"fmt"
	"strings"
)

// validationDecimal is an immutable view of a checked decimal lexeme. Whole
// digits have no leading zeros, fractional digits have no trailing zeros, and
// zero has two empty digit views and is never negative. The views retain the
// input's storage without copying or expanding a scale. Parsing and comparison
// take linear time and constant auxiliary space; caller normalization, document
// storage and diagnostic allocations are separate.
type validationDecimal struct {
	whole, fraction string
	negative        bool
}

func parseValidationDecimal(value string) (validationDecimal, bool) {
	value = strings.Trim(value, xmlWhitespace)
	negative := false
	if len(value) != 0 && (value[0] == '+' || value[0] == '-') {
		negative = value[0] == '-'
		value = value[1:]
	}
	point := -1
	digits := 0
	for index := range len(value) {
		if value[index] == '.' && point == -1 {
			point = index
			continue
		}
		if value[index] < '0' || value[index] > '9' {
			return validationDecimal{}, false
		}
		digits++
	}
	if digits == 0 {
		return validationDecimal{}, false
	}
	whole, fraction := value, ""
	if point != -1 {
		whole, fraction = value[:point], value[point+1:]
	}
	whole = strings.TrimLeft(whole, "0")
	fraction = strings.TrimRight(fraction, "0")
	return validationDecimal{whole: whole, fraction: fraction, negative: negative && (whole != "" || fraction != "")}, true
}

func (value validationDecimal) compare(other validationDecimal) int {
	if value.negative != other.negative {
		if value.negative {
			return -1
		}
		return 1
	}
	result := 0
	switch {
	case len(value.whole) < len(other.whole):
		result = -1
	case len(value.whole) > len(other.whole):
		result = 1
	default:
		result = strings.Compare(value.whole, other.whole)
		if result == 0 {
			// If one fraction is a prefix, its implicit zero suffix is smaller than
			// the other's remaining suffix, which ends in a nonzero digit.
			result = strings.Compare(value.fraction, other.fraction)
		}
	}
	if value.negative {
		return -result
	}
	return result
}

func validationDecimalValuesEqual(left, right string) bool {
	first, firstOK := parseValidationDecimal(left)
	second, secondOK := parseValidationDecimal(right)
	return firstOK && secondOK && first.compare(second) == 0
}

// Keep bound rules and diagnostics aligned with validateIntegerBounds; parsing is domain-specific.
func validateDecimalBounds(schema *validationSimpleSchema, value, display string) *validationSimpleFailure {
	number, ok := parseValidationDecimal(value)
	if !ok {
		return &validationSimpleFailure{constraint: "datatype", message: fmt.Sprintf("value %q is not numeric", display)}
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
		limit, ok := parseValidationDecimal(test.limit)
		if !ok {
			return &validationSimpleFailure{constraint: "schema", message: fmt.Sprintf("invalid %s value %q", test.constraint, test.limit)}
		}
		comparison := number.compare(limit)
		if (test.minimum && comparison < 0) || (!test.minimum && comparison > 0) || (test.exclusive && comparison == 0) {
			return &validationSimpleFailure{constraint: test.constraint, message: fmt.Sprintf("value %q violates %s=%q", display, test.constraint, test.limit)}
		}
	}
	return nil
}
