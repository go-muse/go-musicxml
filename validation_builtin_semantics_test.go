package musicxml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationBuiltinSemanticsCoverage(t *testing.T) {
	t.Parallel()
	// Independently enumerate the 44 named XSD 1.0 datatypes, then the two
	// existing ur-type compatibility entries. This guards dispatch coverage,
	// not complete schema-component validity or aggregate value semantics.
	tests := []struct {
		name, value string
		policy      validationWhitespacePolicy
		stringValue bool
	}{
		{"string", "a", validationWhitespacePreserve, true},
		{"boolean", "true", validationWhitespaceCollapse, false},
		{"decimal", "1.2", validationWhitespaceCollapse, false},
		{"float", "1.2", validationWhitespaceCollapse, false},
		{"double", "1.2", validationWhitespaceCollapse, false},
		{"duration", "P1D", validationWhitespaceCollapse, false},
		{"dateTime", "2000-01-01T00:00:00Z", validationWhitespaceCollapse, false},
		{"time", "00:00:00Z", validationWhitespaceCollapse, false},
		{"date", "2000-01-01", validationWhitespaceCollapse, false},
		{"gYearMonth", "2000-01", validationWhitespaceCollapse, false},
		{"gYear", "2000", validationWhitespaceCollapse, false},
		{"gMonthDay", "--01-01", validationWhitespaceCollapse, false},
		{"gDay", "---01", validationWhitespaceCollapse, false},
		{"gMonth", "--01", validationWhitespaceCollapse, false},
		{"hexBinary", "AB", validationWhitespaceCollapse, false},
		{"base64Binary", "YQ==", validationWhitespaceCollapse, false},
		{"anyURI", "https://example.com/a", validationWhitespaceCollapse, false},
		{"QName", "a", validationWhitespaceCollapse, false},
		{"NOTATION", "a", validationWhitespaceCollapse, false},
		{"normalizedString", "a", validationWhitespaceReplace, true},
		{"token", "a", validationWhitespaceCollapse, true},
		{"language", "en", validationWhitespaceCollapse, true},
		{"NMTOKEN", "a", validationWhitespaceCollapse, true},
		{"NMTOKENS", "a b", validationWhitespaceCollapse, false},
		{"Name", "a", validationWhitespaceCollapse, true},
		{"NCName", "a", validationWhitespaceCollapse, true},
		{"ID", "a", validationWhitespaceCollapse, true},
		{"IDREF", "a", validationWhitespaceCollapse, true},
		{"IDREFS", "a b", validationWhitespaceCollapse, false},
		{"ENTITY", "a", validationWhitespaceCollapse, true},
		{"ENTITIES", "a b", validationWhitespaceCollapse, false},
		{"integer", "1", validationWhitespaceCollapse, false},
		{"nonPositiveInteger", "0", validationWhitespaceCollapse, false},
		{"negativeInteger", "-1", validationWhitespaceCollapse, false},
		{"long", "1", validationWhitespaceCollapse, false},
		{"int", "1", validationWhitespaceCollapse, false},
		{"short", "1", validationWhitespaceCollapse, false},
		{"byte", "1", validationWhitespaceCollapse, false},
		{"nonNegativeInteger", "0", validationWhitespaceCollapse, false},
		{"unsignedLong", "1", validationWhitespaceCollapse, false},
		{"unsignedInt", "1", validationWhitespaceCollapse, false},
		{"unsignedShort", "1", validationWhitespaceCollapse, false},
		{"unsignedByte", "1", validationWhitespaceCollapse, false},
		{"positiveInteger", "1", validationWhitespaceCollapse, false},
		{"anySimpleType", "a", validationWhitespacePreserve, false},
		{"anyType", "a", validationWhitespaceCollapse, false},
	}
	require.Len(t, tests, 46)
	seen := make(map[string]bool)
	for _, test := range tests {
		require.False(t, seen[test.name], "duplicate builtin %s", test.name)
		seen[test.name] = true
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			semantics := validationBuiltinSemantics(test.name)
			require.NotNil(t, semantics)
			assert.Equal(t, test.name, semantics.Builtin)
			assert.Equal(t, test.policy, semantics.Whitespace)
			assert.Equal(t, test.stringValue, semantics.StringValue)
			assert.Nil(t, validateBuiltinValue(test.name, test.value, test.value))
			schema := &validationSimpleSchema{Form: validationSimpleRestriction,
				Base: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: test.name}}}
			context := newTestValidationContext(&validationSchemaSet{})
			failure := context.ValidateSimple(schema, test.value)
			switch test.name {
			case "anyType", "ENTITY", "ENTITIES":
				assert.NotNil(t, failure)
				assert.Error(t, context.AssessmentErr)
			default:
				assert.Nil(t, failure)
				assert.NoError(t, context.AssessmentErr)
			}
		})
	}
	for _, name := range []string{"unknown", "", "String", "xs:string"} {
		assert.Nil(t, validationBuiltinSemantics(name), name)
		failure := validateBuiltinValue(name, "a", "a")
		require.NotNil(t, failure, name)
		assert.Equal(t, "schema", failure.Constraint, name)
	}
}

func TestReplaceWhitespaceConcurrent(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"empty", "", ""},
		{"identity", "a b", "a b"},
		{"XML S", "\t a\r\nb \t", "  a  b  "},
		{"other Unicode", "\u00a0\u0085\u2000\u3000\t", "\u00a0\u0085\u2000\u3000 "},
		{"other controls", "\x00\x01\v\f\x1f\t", "\x00\x01\v\f\x1f "},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			semantics := validationBuiltinSemantics("normalizedString")
			for range 100 {
				assert.Equal(t, test.want, normalizeValidationWhitespace("normalizedString", test.input))
				assert.Equal(t, test.want, semantics.Normalize(test.input))
			}
		})
	}
}

func TestUnresolvedFixedValuesRemainIncomplete(t *testing.T) {
	t.Parallel()
	broken := validationTypeRef{InlineSimple: &validationSimpleSchema{Form: validationSimpleRestriction}}
	set := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{
		{Local: "value"}: {Name: validationQName{Local: "value"}, Type: broken, Fixed: Ptr("same")},
		{Local: "container"}: {Name: validationQName{Local: "container"}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{
			Form:       validationComplexDirect,
			Attributes: []validationAttributeSchema{{Name: validationQName{Local: "code"}, Type: broken, Constraint: &validationValueConstraint{Kind: "fixed", Value: "same"}}},
		}}},
	}}
	for _, test := range []struct {
		name, source, path string
	}{
		{"element", `<value>same</value>`, "/value"},
		{"attribute", `<container code="same"/>`, "/container/@code"},
	} {
		t.Run(test.name, func(t *testing.T) {
			context := assessAttributeSource(t, set, test.source)
			assert.Error(t, context.AssessmentErr)
			assert.Empty(t, context.Issues)
			assert.NotErrorIs(t, context.result(), ErrInvalidDocument)
		})
	}
}
