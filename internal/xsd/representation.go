package xsd

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// ModelRepresentation describes the Go type in which the generated model holds
// values of one numeric built-in type: a binary64 for the decimal family, or a
// fixed-width integer. A value is representable when the model holds it
// without change: an integer within the range, or a decimal whose shortest
// binary64 form is the same number.
type ModelRepresentation struct {
	Binary64 bool
	Min, Max *big.Int
}

var modelRepresentations = map[string]ModelRepresentation{
	"decimal":            {Binary64: true},
	"double":             {Binary64: true},
	"integer":            signedRepresentation(64),
	"long":               signedRepresentation(64),
	"negativeInteger":    signedRepresentation(64),
	"nonPositiveInteger": signedRepresentation(64),
	"int":                signedRepresentation(32),
	"short":              signedRepresentation(16),
	"byte":               signedRepresentation(8),
	"nonNegativeInteger": unsignedRepresentation(64),
	"positiveInteger":    unsignedRepresentation(64),
	"unsignedLong":       unsignedRepresentation(64),
	"unsignedInt":        unsignedRepresentation(32),
	"unsignedShort":      unsignedRepresentation(16),
	"unsignedByte":       unsignedRepresentation(8),
}

func signedRepresentation(bits uint) ModelRepresentation {
	limit := new(big.Int).Lsh(big.NewInt(1), bits-1)
	return ModelRepresentation{Min: new(big.Int).Neg(limit), Max: limit.Sub(limit, big.NewInt(1))}
}

func unsignedRepresentation(bits uint) ModelRepresentation {
	limit := new(big.Int).Lsh(big.NewInt(1), bits)
	return ModelRepresentation{Min: new(big.Int), Max: limit.Sub(limit, big.NewInt(1))}
}

// ModelRepresentationOf returns the model representation of a numeric
// built-in type. xs:float is absent: the model holds it as a binary32, which
// no MusicXML 4.0 schema uses.
func ModelRepresentationOf(builtin string) (ModelRepresentation, bool) {
	representation, ok := modelRepresentations[builtin]
	return representation, ok
}

// Represents reports whether the model holds value without change. Its work
// is bounded by 17 significant digits and the binary64 exponent range, not by
// the length of value.
func (r ModelRepresentation) Represents(value Decimal) bool {
	if !r.Binary64 {
		if value.Fraction != "" || len(value.Whole) > 20 {
			return false
		}
		number, ok := new(big.Int).SetString("0"+value.Whole, 10)
		if !ok {
			return false
		}
		if value.Negative {
			number.Neg(number)
		}
		return number.Cmp(r.Min) >= 0 && number.Cmp(r.Max) <= 0
	}
	if value.Whole == "" && value.Fraction == "" {
		return true
	}
	// A binary64 covers 309 integral digits and 324 leading fractional zeros,
	// and its shortest form has at most 17 significant digits. Check these
	// bounds on the parsed parts before building any text: the significant
	// digits run from the first to the last nonzero digit, and Decimal keeps
	// neither leading integral nor trailing fractional zeros.
	if len(value.Whole) > 309 {
		return false
	}
	if value.Whole != "" {
		last := len(strings.TrimRight(value.Whole, "0"))
		if value.Fraction != "" {
			last = len(value.Whole) + len(value.Fraction)
		}
		if last > 17 {
			return false
		}
	} else {
		leading := len(value.Fraction) - len(strings.TrimLeft(value.Fraction, "0"))
		if leading > 324 || len(value.Fraction)-leading > 17 {
			return false
		}
	}
	text := "0" + value.Whole
	if value.Fraction != "" {
		text += "." + value.Fraction
	}
	number, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(number, 0) {
		return false
	}
	shortest, ok := ParseDecimal(strconv.FormatFloat(number, 'f', -1, 64))
	return ok && shortest.Whole == value.Whole && shortest.Fraction == value.Fraction
}
