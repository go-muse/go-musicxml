package musicxml

import (
	"math/big"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationDecimalExactOperations(t *testing.T) {
	t.Parallel()
	values := []string{"0", "-000.00", "+001.2500", "1.", ".1", "-.1", "1.2", "1.2001", "-1.2001", "9007199254740993.1"}
	for _, width := range []int{25, 310, 4096} {
		for _, sign := range []string{"", "-"} {
			values = append(values, sign+strings.Repeat("9", width)+".001", sign+"0."+strings.Repeat("0", width)+"1", sign+"1"+strings.Repeat("0", width))
		}
	}
	check := func(left, right string) {
		t.Helper()
		first, ok := parseValidationDecimal(left)
		require.True(t, ok)
		second, ok := parseValidationDecimal(right)
		require.True(t, ok)
		wantFirst, ok := new(big.Rat).SetString(left)
		require.True(t, ok)
		wantSecond, ok := new(big.Rat).SetString(right)
		require.True(t, ok)
		assert.Equal(t, wantFirst.Cmp(wantSecond), first.compare(second), "left=%q right=%q", left, right)
		assert.Equal(t, wantFirst.Cmp(wantSecond) == 0, validationDecimalValuesEqual(left, right))
	}
	for _, left := range values {
		for _, right := range values {
			check(left, right)
		}
	}
	random := rand.New(rand.NewPCG(7, 19))
	lexical := func() string {
		count := 1 + random.IntN(200)
		var value strings.Builder
		if random.IntN(2) == 0 {
			value.WriteByte('-')
		} else {
			value.WriteByte('+')
		}
		point := random.IntN(count + 1)
		for i := 0; i < count; i++ {
			if i == point {
				value.WriteByte('.')
			}
			value.WriteByte(byte('0' + random.IntN(10)))
		}
		if point == count {
			value.WriteByte('.')
		}
		return value.String()
	}
	for range 2500 {
		check(lexical(), lexical())
	}
}

func TestValidationDecimalLexicalCompatibility(t *testing.T) {
	t.Parallel()
	check := func(value string) {
		t.Helper()
		_, ok := parseValidationDecimal(value)
		assert.Equal(t, validateBuiltin("decimal", value) == nil, ok, "%q", value)
	}
	// Exhaustively compare short candidate lexemes against the existing builtin
	// validator, including repeated signs/points and internal XML whitespace.
	var visit func(string, int)
	visit = func(value string, remaining int) {
		check(value)
		if remaining == 0 {
			return
		}
		for _, character := range []byte("01+-.e \t") {
			visit(value+string(character), remaining-1)
		}
	}
	visit("", 5)
	for _, value := range []string{"\r\n+00.100\t ", "١", "１", "\u00a01.0", "1.0\u0085", "1\x00", "1\xff", "NaN", "INF", "-INF", "0x1", "1_0"} {
		check(value)
	}
	for _, value := range []string{"0", "-0", "+000.000", "-.0"} {
		number, ok := parseValidationDecimal(value)
		require.True(t, ok)
		assert.Equal(t, validationDecimal{}, number)
	}
	assert.False(t, validationDecimalValuesEqual("invalid", "invalid"))
	assert.False(t, validationDecimalValuesEqual("1e0", "1"))
}

func TestValidationDecimalConstantSpace(t *testing.T) {
	value := "-" + strings.Repeat("0", 1<<18) + strings.Repeat("9", 1<<18) + "." + strings.Repeat("0", 1<<18) + "1" + strings.Repeat("0", 1<<18)
	assert.Zero(t, testing.AllocsPerRun(5, func() {
		first, ok := parseValidationDecimal(value)
		if !ok || first.compare(first) != 0 {
			panic("unexpected decimal comparison")
		}
	}))
}

func TestDecimalGeneratedLexemesRemainExact(t *testing.T) {
	t.Parallel()
	fixed := validationDecimalGenerated.Elements[validationQName{Local: "fixed"}]
	require.NotNil(t, fixed)
	require.NotNil(t, fixed.Fixed)
	enum := validationDecimalGenerated.Types[validationQName{Local: "enum-type"}]
	require.NotNil(t, enum)
	require.NotNil(t, enum.Simple)
	root, err := parseValidationDocument([]byte("<fixed> \t+0001.25000 </fixed>"))
	require.NoError(t, err)
	before := root.Text.String()
	context := &validationContext{schema: &validationDecimalGenerated, effective: make(map[*validationComplexSchema]*validationEffectiveComplex), identifiers: make(map[string]string)}
	context.validateElement(root, fixed, "/fixed")
	assert.Empty(t, context.issues)
	assert.Equal(t, before, root.Text.String())
	assert.Equal(t, "+0001.2500", *fixed.Fixed)
	assert.Equal(t, []string{"+0001.2500", "-000.00"}, enum.Simple.Enumerations)
}

func TestValidateDecimalInvalidBoundLiterals(t *testing.T) {
	t.Parallel()
	for _, literal := range []string{"", "1e0", "INF", "-INF", "NaN", "0x1", "١", "1.2.3"} {
		for _, schema := range []*validationSimpleSchema{
			{HasMinInclusive: true, MinInclusive: literal}, {HasMaxInclusive: true, MaxInclusive: literal},
			{HasMinExclusive: true, MinExclusive: literal}, {HasMaxExclusive: true, MaxExclusive: literal},
		} {
			failure := validateBounds(schema, "1", "decimal")
			if assert.NotNil(t, failure, "%q", literal) {
				assert.Equal(t, "schema", failure.constraint)
			}
		}
	}
	failure := validateBounds(&validationSimpleSchema{HasMinInclusive: true, MinInclusive: "0"}, "1e0", "decimal")
	if assert.NotNil(t, failure) {
		assert.Equal(t, "datatype", failure.constraint)
	}
}

func TestDecimalComparisonDispatchScope(t *testing.T) {
	t.Parallel()
	context := &validationContext{schema: &validationDecimalGenerated}
	for _, builtin := range []string{"string", "token", "float", "double"} {
		reference := &validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: builtin}}
		assert.False(t, context.simpleValuesEqual(reference, "1.0", "1.00"), builtin)
		assert.False(t, context.elementFixedValuesEqual(reference, "1.0", "1.00"), builtin)
	}
	assert.NotNil(t, validateBuiltin("integer", "1.0"))
	assert.Nil(t, validateBounds(&validationSimpleSchema{HasMinInclusive: true, MinInclusive: "1e0"}, "1e0", "double"))
	for _, form := range []validationSimpleForm{validationSimpleUnion, validationSimpleList} {
		reference := &validationTypeRef{InlineSimple: &validationSimpleSchema{Form: form, Members: []*validationSimpleMember{{Name: validationQName{Space: validationXSDNamespace, Local: "decimal"}}}, Item: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "decimal"}}}}
		assert.False(t, context.simpleValuesEqual(reference, "1.0", "1.00"), form)
		assert.False(t, context.elementFixedValuesEqual(reference, "1.0", "1.00"), form)
	}
}
