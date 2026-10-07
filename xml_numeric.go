package musicxml

import (
	"strconv"
	"strings"
)

// parseXMLUnsignedInteger accepts the signed lexical forms of the XSD integer
// family when their value fits the unsigned Go representation. Range facets
// such as positiveInteger's exclusion of zero remain the validator's job.
func parseXMLUnsignedInteger(value string, bitSize int) (uint64, error) {
	original := value
	value = strings.Trim(value, " \t\r\n")
	negative := false
	if len(value) != 0 && (value[0] == '+' || value[0] == '-') {
		negative = value[0] == '-'
		value = value[1:]
	}
	if value == "" {
		return 0, &strconv.NumError{Func: "ParseUint", Num: original, Err: strconv.ErrSyntax}
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0, &strconv.NumError{Func: "ParseUint", Num: original, Err: strconv.ErrSyntax}
		}
	}
	parsed, err := strconv.ParseUint(value, 10, bitSize)
	if err != nil {
		return 0, err
	}
	if negative && parsed != 0 {
		return 0, &strconv.NumError{Func: "ParseUint", Num: original, Err: strconv.ErrRange}
	}
	return parsed, nil
}
