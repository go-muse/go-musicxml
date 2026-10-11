package xsd

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
type Decimal struct {
	Whole, Fraction string
	Negative        bool
}

func ParseDecimal(value string) (Decimal, bool) {
	value = strings.Trim(value, XMLWhitespace)
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
	whole = strings.TrimLeft(whole, "0")
	fraction = strings.TrimRight(fraction, "0")
	return Decimal{Whole: whole, Fraction: fraction, Negative: negative && (whole != "" || fraction != "")}, true
}

func (value Decimal) Compare(other Decimal) int {
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
		result = strings.Compare(value.Whole, other.Whole)
		if result == 0 {
			// If one fraction is a prefix, its implicit zero suffix is smaller than
			// the other's remaining suffix, which ends in a nonzero digit.
			result = strings.Compare(value.Fraction, other.Fraction)
		}
	}
	if value.Negative {
		return -result
	}
	return result
}

func DecimalValuesEqual(left, right string) bool {
	first, firstOK := ParseDecimal(left)
	second, secondOK := ParseDecimal(right)
	return firstOK && secondOK && first.Compare(second) == 0
}

// Keep bound rules and diagnostics aligned with validateIntegerBounds; parsing is domain-specific.
func ValidateDecimalBounds(schema *SimpleSchema, value, display string) *SimpleFailure {
	number, ok := ParseDecimal(value)
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
		limit, ok := ParseDecimal(test.Limit)
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
