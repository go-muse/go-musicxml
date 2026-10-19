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

// Only the complex generation declares the named types, so an element
// generation keeps raw Go integers and compiles on its own.
func TestElementGenerationKeepsRawBuiltinIntegers(t *testing.T) {
	t.Parallel()

	index, err := NewIndex(&Set{Files: []*SchemaFile{parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:element name="count" type="xs:nonNegativeInteger"/>
	<xs:element name="pair">
		<xs:complexType>
			<xs:sequence>
				<xs:element name="n" type="xs:positiveInteger"/>
				<xs:element name="i" type="xs:integer" minOccurs="0"/>
			</xs:sequence>
		</xs:complexType>
	</xs:element>
</xs:schema>`)}})
	require.NoError(t, err)
	index.SetNameManifest(NewNameManifest())
	elements, err := GenerateElements(index, "example", QName{Local: "count"}, QName{Local: "pair"})
	require.NoError(t, err)
	source := string(elements)
	for _, name := range []string{"PositiveInteger", "NonNegativeInteger", "Integer"} {
		assert.NotRegexp(t, `\b`+name+`\b`, source, "the element generation does not declare %s", name)
	}
	assert.Contains(t, source, "uint64")
	assert.Empty(t, builtinEntries(index.names), "no named type was assigned")
	typeCheckGenerated(t, elements)
}

// Without a manifest the named type avoids the schema's own type names, and
// model metadata keeps the unsigned kind.
func TestNamedBuiltinAvoidsSchemaNamesWithoutManifest(t *testing.T) {
	t.Parallel()

	index, err := NewIndex(&Set{Files: []*SchemaFile{parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="positive-integer">
		<xs:restriction base="xs:string"/>
	</xs:simpleType>
	<xs:complexType name="holder">
		<xs:sequence>
			<xs:element name="label" type="positive-integer"/>
			<xs:element name="count" type="xs:positiveInteger"/>
		</xs:sequence>
	</xs:complexType>
</xs:schema>`)}})
	require.NoError(t, err)
	simple, err := GenerateSimpleTypes(index, "example")
	require.NoError(t, err)
	complex, err := GenerateComplexTypesWithOptions(index, "example", ComplexGenerationOptions{ModelMetadata: true})
	require.NoError(t, err)
	source := string(complex)
	assert.Contains(t, string(simple), "type PositiveInteger string")
	assert.Contains(t, source, "type PositiveInteger2 uint64")
	assert.Contains(t, source, "\tLabel PositiveInteger  `xml:\"label\"`")
	assert.Contains(t, source, "\tCount PositiveInteger2 `xml:\"count\"`")
	assert.Contains(t, source, `{Name: "Count", Index: []int{1}, XML: modelName{Local: "count"}, Optional: false, Repeated: false, Kind: modelUnsigned}`)
	plain, err := GenerateComplexTypes(index, "example")
	require.NoError(t, err)
	typeCheckGenerated(t, simple, plain)
}

// Without a manifest the named type also avoids enumeration constants, such
// as PositiveInteger for the value "integer" of the type "positive".
func TestNamedBuiltinAvoidsConstantsWithoutManifest(t *testing.T) {
	t.Parallel()

	index, err := NewIndex(&Set{Files: []*SchemaFile{parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="positive">
		<xs:restriction base="xs:string">
			<xs:enumeration value="integer"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:simpleType name="non-negative">
		<xs:union>
			<xs:simpleType>
				<xs:restriction base="xs:token">
					<xs:enumeration value="integer"/>
				</xs:restriction>
			</xs:simpleType>
			<xs:simpleType>
				<xs:restriction base="xs:integer"/>
			</xs:simpleType>
		</xs:union>
	</xs:simpleType>
	<xs:complexType name="holder">
		<xs:sequence>
			<xs:element name="count" type="xs:positiveInteger"/>
			<xs:element name="total" type="xs:nonNegativeInteger"/>
			<xs:element name="tag" type="positive"/>
			<xs:element name="kind" type="non-negative"/>
		</xs:sequence>
	</xs:complexType>
</xs:schema>`)}})
	require.NoError(t, err)
	simple, err := GenerateSimpleTypes(index, "example")
	require.NoError(t, err)
	complex, err := GenerateComplexTypes(index, "example")
	require.NoError(t, err)
	assert.Contains(t, string(simple), "PositiveInteger Positive = \"integer\"")
	assert.Contains(t, string(simple), "NonNegativeInteger NonNegative = \"integer\"")
	assert.Contains(t, string(complex), "type PositiveInteger2 uint64")
	assert.Contains(t, string(complex), "type NonNegativeInteger2 uint64")
	typeCheckGenerated(t, simple, complex)
}
