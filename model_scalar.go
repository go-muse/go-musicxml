package musicxml

import (
	"context"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/validation"
)

const modelLexemeUnavailable validation.Code = "model-source-lexeme-unavailable"

type modelScalarResult struct {
	scalar   validation.Scalar
	problem  validation.Code
	retained int64
}

func absentModelScalar() validation.Scalar {
	return validation.Scalar{Lexeme: validation.Evidence[string]{State: validation.EvidenceUnsupported, Reason: modelLexemeUnavailable}, Value: validation.Evidence[validation.Atomic]{State: validation.EvidenceAbsent}}
}

func modelMetadataError(reason string) error {
	return &validation.IncompleteError{Reason: "model representation metadata unavailable", Cause: &validation.ConfigurationError{Field: "model metadata", Reason: reason}}
}

// readModelScalar borrows strings, and owns only bounded exact-value scratch.
// Its caller releases retained after the synchronous sink copies the fact.
func readModelScalar(ctx context.Context, account *validation.Account, value reflect.Value, kind modelScalarKind) (result modelScalarResult, err error) {
	result.scalar = absentModelScalar()
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if err = account.Charge(validation.BudgetEvents, 1); err != nil {
		return result, err
	}
	defer func() {
		if err != nil && result.retained != 0 {
			_ = account.Release(validation.BudgetBufferedBytes, result.retained)
			result.retained = 0
		}
	}()
	fail := func(code validation.Code) (modelScalarResult, error) {
		result.problem = code
		result.scalar.Value = validation.Evidence[validation.Atomic]{State: validation.EvidenceUnsupported, Reason: code}
		return result, nil
	}
	var atomic validation.Atomic
	switch kind {
	case modelString:
		if value.Kind() != reflect.String {
			return result, modelMetadataError("string kind mismatch")
		}
		text := value.String()
		if err = account.Check(validation.BudgetScalarBytes, int64(len(text))); err != nil {
			return result, err
		}
		for offset, nextCheck := 0, 0; offset < len(text); {
			if offset >= nextCheck {
				if err = ctx.Err(); err != nil {
					return result, err
				}
				if err = account.Charge(validation.BudgetEvents, 1); err != nil {
					return result, err
				}
				nextCheck = offset + 1024
			}
			r, n := utf8.DecodeRuneInString(text[offset:])
			if r == utf8.RuneError && n == 1 {
				return fail("model-invalid-utf8")
			}
			if !(r == 9 || r == 10 || r == 13 || r >= 0x20 && r <= 0xd7ff || r >= 0xe000 && r <= 0xfffd || r >= 0x10000 && r <= 0x10ffff) {
				return fail("model-invalid-xml-character")
			}
			offset += n
		}
		atomic = validation.Atomic{Kind: validation.AtomicString, Text: text}
	case modelBoolean:
		if value.Kind() != reflect.Bool {
			return result, modelMetadataError("boolean kind mismatch")
		}
		text := "false"
		if value.Bool() {
			text = "true"
		}
		if err = account.Check(validation.BudgetScalarBytes, int64(len(text))); err != nil {
			return result, err
		}
		atomic = validation.Atomic{Kind: validation.AtomicBoolean, Text: text}
	case modelSigned, modelUnsigned:
		var magnitude uint64
		var signed int64
		if kind == modelSigned {
			if value.Kind() < reflect.Int || value.Kind() > reflect.Int64 {
				return result, modelMetadataError("signed kind mismatch")
			}
			signed = value.Int()
			if signed < 0 {
				magnitude = uint64(-(signed + 1)) + 1
			} else {
				magnitude = uint64(signed)
			}
		} else {
			if value.Kind() < reflect.Uint || value.Kind() > reflect.Uint64 {
				return result, modelMetadataError("unsigned kind mismatch")
			}
			magnitude = value.Uint()
		}
		size := int64(1)
		for n := magnitude; n >= 10; n /= 10 {
			size++
		}
		if kind == modelSigned && signed < 0 {
			size++
		}
		if err = account.Check(validation.BudgetScalarBytes, size); err != nil {
			return result, err
		}
		if err = account.Charge(validation.BudgetBufferedBytes, 512); err != nil {
			return result, err
		}
		result.retained = 512
		exact := new(big.Int)
		if kind == modelSigned {
			exact.SetInt64(signed)
		} else {
			exact.SetUint64(magnitude)
		}
		var integer validation.Integer
		integer, err = validation.NewInteger(exact)
		if err != nil {
			return result, err
		}
		atomic = validation.Atomic{Kind: validation.AtomicInteger, Integer: integer}
	case modelDecimal:
		if value.Kind() != reflect.Float64 {
			return result, modelMetadataError("decimal kind mismatch")
		}
		number := value.Float()
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return fail("model-nonfinite")
		}
		// The model value is the shortest decimal that converts back to the same
		// binary64, the value a decoder read from that text: 0.1 is 0.1, not the
		// nearest binary fraction. Values whose source text has no such shortest
		// form are caught as unrepresentable in the source line. A binary64 has at
		// most 17 significant digits, 309 integral digits and a 324-digit
		// fractional scale, so the rendered text is bounded before it is built.
		if err = account.Check(validation.BudgetScalarBytes, int64(shortestDecimalBound(number))); err != nil {
			return result, err
		}
		scratch := int64(4096 + shortestDecimalBound(number)*8)
		if err = account.Charge(validation.BudgetBufferedBytes, scratch); err != nil {
			return result, err
		}
		result.retained = scratch
		text := strconv.FormatFloat(number, 'f', -1, 64)
		exact, ok := new(big.Rat).SetString(text)
		if !ok {
			return result, modelMetadataError("unparsable shortest decimal")
		}
		var decimal validation.Decimal
		decimal, err = validation.NewDecimal(exact)
		if err != nil {
			return result, err
		}
		if err = account.Check(validation.BudgetScalarBytes, int64(len(decimal.String()))); err != nil {
			return result, err
		}
		atomic = validation.Atomic{Kind: validation.AtomicDecimal, Decimal: decimal}
	default:
		return result, modelMetadataError("unknown scalar kind")
	}
	result.scalar.Value = validation.Evidence[validation.Atomic]{State: validation.EvidenceKnown, Value: atomic}
	return result, nil
}

// shortestDecimalBound bounds the length of strconv.FormatFloat(number, 'f',
// -1, 64): a sign, the integral digits, a point and the fractional digits.
// The shortest form has at most 17 significant digits.
func shortestDecimalBound(number float64) int {
	if number == 0 {
		if math.Signbit(number) {
			return 2
		}
		return 1
	}
	// Log10 may round across a power of ten, so allow one digit either way.
	exponent := int(math.Floor(math.Log10(math.Abs(number))))
	bound := max(exponent+2, 1)
	if fraction := 18 - exponent; fraction > 0 {
		bound += 1 + fraction
	}
	if number < 0 {
		bound++
	}
	return bound
}
