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
	value = strings.Trim(value, xmlWhitespace)
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

// xmlDecimal keeps decimal transport in fixed-point form. The public model
// remains float64-based; schema facets are checked separately by Validate.
type xmlDecimal float64

func (value xmlDecimal) MarshalText() ([]byte, error) {
	return []byte(strconv.FormatFloat(float64(value), 'f', -1, 64)), nil
}

func (value *xmlDecimal) UnmarshalText(text []byte) error {
	// Match encoding/xml's permissive float transport, including its empty
	// value handling. Validation, rather than decoding, checks XSD facets.
	lexical := strings.TrimSpace(string(text))
	if lexical == "" {
		*value = 0
		return nil
	}
	parsed, err := strconv.ParseFloat(lexical, 64)
	if err == nil {
		*value = xmlDecimal(parsed)
	}
	return err
}

func parseXMLUnsignedText(text []byte, bitSize int) (uint64, error) {
	lexical := strings.TrimSpace(string(text))
	if lexical == "" {
		return 0, nil
	}
	return parseXMLUnsignedInteger(lexical, bitSize)
}

type xmlUnsigned64 uint64

type xmlUnsigned32 uint32

type xmlUnsigned16 uint16

type xmlUnsigned8 uint8

func (value *xmlUnsigned64) UnmarshalText(text []byte) error {
	parsed, err := parseXMLUnsignedText(text, 64)
	if err == nil {
		*value = xmlUnsigned64(parsed)
	}
	return err
}

func (value *xmlUnsigned32) UnmarshalText(text []byte) error {
	parsed, err := parseXMLUnsignedText(text, 32)
	if err == nil {
		*value = xmlUnsigned32(parsed)
	}
	return err
}

func (value *xmlUnsigned16) UnmarshalText(text []byte) error {
	parsed, err := parseXMLUnsignedText(text, 16)
	if err == nil {
		*value = xmlUnsigned16(parsed)
	}
	return err
}

func (value *xmlUnsigned8) UnmarshalText(text []byte) error {
	parsed, err := parseXMLUnsignedText(text, 8)
	if err == nil {
		*value = xmlUnsigned8(parsed)
	}
	return err
}

func xmlConvertPointer[From, To any](value *From, convert func(From) To) *To {
	if value == nil {
		return nil
	}
	converted := convert(*value)
	return &converted
}

func xmlConvertSlice[From, To any](values []From, convert func(From) To) []To {
	if values == nil {
		return nil
	}
	converted := make([]To, len(values))
	for index, value := range values {
		converted[index] = convert(value)
	}
	return converted
}
