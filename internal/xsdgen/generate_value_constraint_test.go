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
