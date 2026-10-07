package musicxml

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateUnsignedIntegerLexicalForms(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		builtin, value string
		valid          bool
	}{
		{"positiveInteger", "+1", true},
		{"nonNegativeInteger", "-0", true},
		{"nonNegativeInteger", "-000", true},
		{"unsignedByte", "+255", true},
		{"unsignedByte", "+256", false},
		{"positiveInteger", "-0", false},
		{"nonNegativeInteger", "-1", false},
		{"nonNegativeInteger", "+-0", false},
		{"nonNegativeInteger", "++1", false},
		{"nonNegativeInteger", "1.0", false},
		{"nonNegativeInteger", "1e0", false},
		{"nonNegativeInteger", "1 0", false},
		{"nonNegativeInteger", "\u00a01", false},
	} {
		failure := validateBuiltin(test.builtin, test.value)
		if test.valid {
			assert.Nil(t, failure, "%s %q", test.builtin, test.value)
		} else {
			assert.NotNil(t, failure, "%s %q", test.builtin, test.value)
		}
	}
}

func TestValidatePositiveIntegerUnionLeadingPlus(t *testing.T) {
	t.Parallel()
	doc, err := Decode(strings.NewReader(`<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Test</part-name><score-instrument id="I1"><instrument-name>Piano</instrument-name><ensemble>+1</ensemble></score-instrument></score-part></part-list><part id="P1"><measure number="1"/></part></score-partwise>`))
	require.NoError(t, err)
	assert.NoError(t, Validate(doc))
}

func TestValidationPatternUnicodeClasses(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		pattern, value string
		valid          bool
	}{
		{`acc\c+`, "accidentalЖ", true},
		{`acc\c+`, "accidental樂", true},
		{`acc\c+`, "accidental\u0301", true},
		{`acc\c+`, "accidental\U00010000", false},
		{`\i\c*`, "Ж1", true},
		{`\i\c*`, "\u200cname", false},
		{`\i\c*`, "\u0301name", false},
		{`acc\c+`, "accidental name", false},
		{`acc\c+`, "accidental\u2000", false},
		{`acc\c+`, "accidental/", false},
		{`#[\dA-F]{6}`, "#١٢٣٤٥٦", true},
		{`#[\dA-F]{6}`, "#１２３４５６", true},
		{`#[\dA-F]{6}`, "#abcdef", false},
	} {
		actual, err := matchValidationPattern(test.pattern, test.value)
		require.NoError(t, err)
		assert.Equal(t, test.valid, actual, "%s %q", test.pattern, test.value)
	}
}

func TestValidateUnicodeSMuFLPattern(t *testing.T) {
	t.Parallel()
	score := decodeValidationScore(t)
	note := validationFirstNote(t, score)
	note.Accidental = &Accidental{Value: AccidentalValueOther, SMuFL: Ptr(SMuFLAccidentalGlyphName("accidentalЖ"))}
	assert.NoError(t, Validate(score))
}
