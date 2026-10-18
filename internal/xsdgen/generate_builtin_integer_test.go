package xsdgen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const builtinIntegerSchema = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="level">
		<xs:restriction base="xs:positiveInteger">
			<xs:maxInclusive value="8"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:complexType name="integer">
		<xs:sequence>
			<xs:element name="value" type="xs:integer"/>
		</xs:sequence>
	</xs:complexType>
	<xs:complexType name="count">
		<xs:simpleContent>
			<xs:extension base="xs:nonNegativeInteger">
				<xs:attribute name="unit" type="xs:string"/>
			</xs:extension>
		</xs:simpleContent>
	</xs:complexType>
	<xs:complexType name="staff">
		<xs:sequence>
			<xs:element name="lines" type="xs:positiveInteger" minOccurs="0"/>
			<xs:element name="level" type="level"/>
			<xs:element name="shift" type="xs:integer"/>
		</xs:sequence>
		<xs:attribute name="number" type="xs:positiveInteger"/>
	</xs:complexType>
</xs:schema>`

// Fields whose type is a built-in integer get named types, named through the
// manifest; derived simple types keep their raw base; structs with only such
// fields need no XML methods; and the output type-checks.
func TestGenerateNamedBuiltinIntegers(t *testing.T) {
	t.Parallel()

	index, err := NewIndex(&Set{Files: []*SchemaFile{parseSchemaFile(t, "types.xsd", builtinIntegerSchema)}})
	require.NoError(t, err)
	manifest := NewNameManifest()
	index.SetNameManifest(manifest)
	simple, err := GenerateSimpleTypes(index, "example")
	require.NoError(t, err)
	complex, err := GenerateComplexTypes(index, "example")
	require.NoError(t, err)
	source := string(complex)

	assert.Contains(t, string(simple), "type Level uint64\n", "a derived simple type keeps its raw base")
	// The complex type "integer" holds the name Integer, so the built-in
	// type takes a suffix and every field follows it.
	assert.Contains(t, source, "type Integer struct {\n\tValue Integer2 `xml:\"value\"`\n}")
	assert.Contains(t, source, "type Integer2 int64\n")
	assert.Contains(t, source, "\tValue NonNegativeInteger `xml:\",chardata\"`")
	assert.Contains(t, source, "\tLines  *PositiveInteger `xml:\"lines,omitempty\"`")
	assert.Contains(t, source, "\tNumber *PositiveInteger `xml:\"number,attr,omitempty\"`")
	assert.Contains(t, source, "\tShift  Integer2")
	assert.Contains(t, source, "func (value *PositiveInteger) UnmarshalText(text []byte) error {\n\treturn (*xmlUnsigned64)(value).UnmarshalText(text)\n}")
	assert.Contains(t, source, "func (value *NonNegativeInteger) UnmarshalText(")
	assert.NotContains(t, source, "func (value *Integer2) UnmarshalText", "encoding/xml decodes every signed form")
	// Only Staff's derived level still needs a wire struct.
	assert.NotContains(t, source, "type xmlCount struct")
	assert.NotContains(t, source, "func (value Count) MarshalXML")
	assert.Equal(t, map[string]string{
		"XMLSchema builtin integer":            "Integer2",
		"XMLSchema builtin nonNegativeInteger": "NonNegativeInteger",
		"XMLSchema builtin positiveInteger":    "PositiveInteger",
	}, builtinEntries(manifest))
	typeCheckGenerated(t, simple, complex)
}

func builtinEntries(manifest *NameManifest) map[string]string {
	entries := make(map[string]string)
	for key, name := range manifest.byKey {
		if strings.HasPrefix(key, "XMLSchema builtin ") {
			entries[key] = name
		}
	}
	return entries
}
