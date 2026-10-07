package xsdgen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateFixedAttributeWhitespaceComparison(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		types      string
		attribute  string
		wantPolicy string
	}{
		{
			name:      "string preserves whitespace",
			attribute: `type="xs:string"`,
		},
		{
			name:       "normalized string replaces whitespace",
			attribute:  `type="xs:normalizedString"`,
			wantPolicy: "normalizedString",
		},
		{
			name:       "NMTOKEN collapses whitespace",
			attribute:  `type="xs:NMTOKEN"`,
			wantPolicy: "token",
		},
		{
			name: "named restrictions inherit whitespace",
			types: `<xs:simpleType name="base"><xs:restriction base="xs:NMTOKEN"/></xs:simpleType>
<xs:simpleType name="derived"><xs:restriction base="base"/></xs:simpleType>`,
			attribute:  `type="derived"`,
			wantPolicy: "token",
		},
		{
			name: "named facet overrides builtin whitespace",
			types: `<xs:simpleType name="base"><xs:restriction base="xs:string"><xs:whiteSpace value="collapse"/></xs:restriction></xs:simpleType>
<xs:simpleType name="derived"><xs:restriction base="base"/></xs:simpleType>`,
			attribute:  `type="derived"`,
			wantPolicy: "token",
		},
		{
			name:       "inline replace facet",
			attribute:  `><xs:simpleType><xs:restriction base="xs:string"><xs:whiteSpace value="replace"/></xs:restriction></xs:simpleType></xs:attribute`,
			wantPolicy: "normalizedString",
		},
		{
			name:      "explicit preserve facet",
			attribute: `><xs:simpleType><xs:restriction base="xs:string"><xs:whiteSpace value="preserve"/></xs:restriction></xs:simpleType></xs:attribute`,
		},
		{
			name: "union members preserve whitespace",
			types: `<xs:simpleType name="base"><xs:restriction base="xs:string"/></xs:simpleType>
<xs:simpleType name="choice"><xs:union memberTypes="xs:string base"/></xs:simpleType>`,
			attribute: `type="choice"`,
		},
		{
			name: "union members replace whitespace",
			types: `<xs:simpleType name="base"><xs:restriction base="xs:normalizedString"/></xs:simpleType>
<xs:simpleType name="choice"><xs:union memberTypes="xs:normalizedString base"/></xs:simpleType>`,
			attribute:  `type="choice"`,
			wantPolicy: "normalizedString",
		},
		{
			name:       "union members collapse whitespace",
			types:      `<xs:simpleType name="choice"><xs:union memberTypes="xs:NMTOKEN xs:token"/></xs:simpleType>`,
			attribute:  `type="choice"`,
			wantPolicy: "token",
		},
		{
			name: "nested string union",
			types: `<xs:simpleType name="choice"><xs:union memberTypes="xs:NMTOKEN xs:token"/></xs:simpleType>
<xs:simpleType name="nested"><xs:union memberTypes="choice xs:NCName"/></xs:simpleType>`,
			attribute:  `type="nested"`,
			wantPolicy: "token",
		},
		{
			name: "restriction inherits union whitespace",
			types: `<xs:simpleType name="choice"><xs:union memberTypes="xs:NMTOKEN xs:token"/></xs:simpleType>
<xs:simpleType name="derived"><xs:restriction base="choice"/></xs:simpleType>`,
			attribute:  `type="derived"`,
			wantPolicy: "token",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			attribute := `<xs:attribute name="state" fixed="ready" ` + test.attribute
			if strings.Contains(test.attribute, "<xs:simpleType>") {
				attribute += ">"
			} else {
				attribute += "/>"
			}
			file := parseSchemaFile(t, "constraints.xsd", fmt.Sprintf(
				`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">%s<xs:complexType name="record">%s</xs:complexType></xs:schema>`,
				test.types, attribute,
			))
			index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
			require.NoError(t, err)
			actual, err := GenerateComplexTypes(index, "example")
			require.NoError(t, err)
			source := string(actual)
			if test.wantPolicy == "" {
				assert.Contains(t, source, `value.State == nil || *value.State == "ready"`)
				assert.NotContains(t, source, "normalizeValidationWhitespace")
			} else {
				assert.Contains(t, source, fmt.Sprintf(
					`normalizeValidationWhitespace(%q, string(*value.State)) == normalizeValidationWhitespace(%q, "ready")`,
					test.wantPolicy, test.wantPolicy,
				))
			}
			assert.Contains(t, source, "return *value.State", "EffectiveState must preserve explicit values")
		})
	}
}

// Unions select their normalization through the matched member. Until fixed
// helpers support that selection, mixed policies must fail explicitly rather
// than produce false matches such as xs:integer|xs:string " ready " == "ready".
func TestGenerateFixedAttributeRejectsMixedUnionWhitespace(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, types, attribute string
	}{
		{
			name:      "named mixed union",
			types:     `<xs:simpleType name="choice"><xs:union memberTypes="xs:integer xs:string"/></xs:simpleType>`,
			attribute: `type="choice"/>`,
		},
		{
			name:      "inline mixed union",
			attribute: `><xs:simpleType><xs:union memberTypes="xs:integer xs:string"/></xs:simpleType></xs:attribute>`,
		},
		{
			name: "restriction of mixed union",
			types: `<xs:simpleType name="choice"><xs:union memberTypes="xs:integer xs:string"/></xs:simpleType>
<xs:simpleType name="derived"><xs:restriction base="choice"/></xs:simpleType>`,
			attribute: `type="derived"/>`,
		},
		{
			name: "nested mixed union",
			types: `<xs:simpleType name="choice"><xs:union memberTypes="xs:integer xs:string"/></xs:simpleType>
<xs:simpleType name="nested"><xs:union memberTypes="xs:token choice"/></xs:simpleType>`,
			attribute: `type="nested"/>`,
		},
		{
			name:      "replace and collapse",
			types:     `<xs:simpleType name="choice"><xs:union memberTypes="xs:normalizedString xs:token"/></xs:simpleType>`,
			attribute: `type="choice"/>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			xsd := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">%s<xs:complexType name="record"><xs:attribute name="state" fixed="ready" %s</xs:complexType></xs:schema>`, test.types, test.attribute)
			file := parseSchemaFile(t, "constraints.xsd", xsd)
			index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
			require.NoError(t, err)
			actual, err := GenerateComplexTypes(index, "example")
			require.ErrorIs(t, err, ErrUnsupportedComplexGeneration)
			assert.Contains(t, err.Error(), `fixed attribute "state"`)
			assert.Contains(t, err.Error(), "mixed whitespace policies")
			assert.Nil(t, actual)

			// Defaults preserve explicit lexical values and need no fixed
			// comparison, so these unions remain supported there.
			file = parseSchemaFile(t, "defaults.xsd", strings.Replace(xsd, `fixed="ready"`, `default="ready"`, 1))
			index, err = NewIndex(&Set{Files: []*SchemaFile{file}})
			require.NoError(t, err)
			actual, err = GenerateComplexTypes(index, "example")
			require.NoError(t, err)
			assert.Contains(t, string(actual), "EffectiveState")
			assert.NotContains(t, string(actual), "StateMatchesFixed")
		})
	}
}

func TestGenerateFixedAttributeRejectsCyclicUnionWhitespace(t *testing.T) {
	file := parseSchemaFile(t, "constraints.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
<xs:simpleType name="choice"><xs:union memberTypes="choice xs:string"/></xs:simpleType>
<xs:complexType name="record"><xs:attribute name="state" type="choice" fixed="ready"/></xs:complexType>
</xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)
	actual, err := GenerateComplexTypes(index, "example")
	require.ErrorIs(t, err, ErrUnsupportedComplexGeneration)
	assert.Contains(t, err.Error(), "cyclic fixed-attribute whitespace dependency")
	assert.Nil(t, actual)
}

func TestGenerateFixedAttributeRejectsNonStringUnionValues(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, types, members, fixed string
	}{
		{name: "integer and token", members: "xs:integer xs:token", fixed: "1"},
		{name: "numeric members", members: "xs:integer xs:decimal", fixed: "1"},
		{name: "boolean and token", members: "xs:boolean xs:token", fixed: "true"},
		{name: "dateTime stored as string", members: "xs:dateTime xs:token", fixed: "2000-01-01T00:00:00Z"},
		{name: "QName stored as string", members: "xs:QName xs:token", fixed: "ready"},
		{
			name:    "named numeric restriction",
			types:   `<xs:simpleType name="number"><xs:restriction base="xs:integer"/></xs:simpleType>`,
			members: "number xs:token", fixed: "1",
		},
		{
			name:    "list stored as string",
			types:   `<xs:simpleType name="numbers"><xs:list itemType="xs:integer"/></xs:simpleType>`,
			members: "numbers xs:token", fixed: "1",
		},
		{
			name:    "nested numeric union",
			types:   `<xs:simpleType name="numbers"><xs:union memberTypes="xs:integer xs:decimal"/></xs:simpleType>`,
			members: "numbers xs:token", fixed: "1",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			xsd := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">%s
<xs:simpleType name="choice"><xs:union memberTypes="%s"/></xs:simpleType>
<xs:complexType name="record"><xs:attribute name="state" type="choice" fixed="%s"/></xs:complexType>
</xs:schema>`, test.types, test.members, test.fixed)
			file := parseSchemaFile(t, "constraints.xsd", xsd)
			index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
			require.NoError(t, err)
			actual, err := GenerateComplexTypes(index, "example")
			require.ErrorIs(t, err, ErrUnsupportedComplexGeneration)
			assert.Contains(t, err.Error(), `fixed attribute "state"`)
			assert.Contains(t, err.Error(), "non-string value space")
			assert.Nil(t, actual)

			file = parseSchemaFile(t, "defaults.xsd", strings.Replace(xsd, `fixed="`, `default="`, 1))
			index, err = NewIndex(&Set{Files: []*SchemaFile{file}})
			require.NoError(t, err)
			actual, err = GenerateComplexTypes(index, "example")
			require.NoError(t, err)
			assert.Contains(t, string(actual), "EffectiveState")
			assert.NotContains(t, string(actual), "StateMatchesFixed")
		})
	}
}
