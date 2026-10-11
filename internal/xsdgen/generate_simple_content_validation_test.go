package xsdgen

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSimpleContentRestrictionFacets(t *testing.T) {
	t.Parallel()
	file := parseSchemaFile(t, "facets.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
 <xs:complexType name="base"><xs:simpleContent><xs:extension base="xs:decimal"/></xs:simpleContent></xs:complexType>
 <xs:complexType name="restricted"><xs:simpleContent><xs:restriction base="base">
 <xs:enumeration value="+001.2000"/><xs:enumeration value="1.2000"/><xs:enumeration value="--"/>
 <xs:pattern value="[0-9]+"/><xs:pattern value="[1-9]+"/>
 <xs:minInclusive value="+001.2000"/><xs:maxInclusive value="99.9"/>
 <xs:minExclusive value="0"/><xs:maxExclusive value="100"/>
 <xs:totalDigits value="3"/><xs:fractionDigits value="2"/>
 <xs:length value="1"/><xs:minLength value="0"/><xs:maxLength value="9"/>
 <xs:whiteSpace value="collapse"/>
 </xs:restriction></xs:simpleContent></xs:complexType>
 </xs:schema>`)
	// This synthetic metadata-shape test intentionally combines inapplicable facets;
	// schema-component validity is separate from faithful extraction.
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)
	actual, err := GenerateValidationSchema(index, "example", "schema")
	require.NoError(t, err)
	_, err = parser.ParseFile(token.NewFileSet(), "validation.go", actual, parser.SkipObjectResolution)
	require.NoError(t, err)
	for _, want := range []string{`SimpleContent:`, `Enumerations: []string{"+001.2000", "1.2000", "--"}`, `Patterns: []string{"[0-9]+", "[1-9]+"}`, `MinInclusive: "+001.2000"`, `MaxInclusive: "99.9"`, `MinExclusive: "0"`, `MaxExclusive: "100"`, `TotalDigits: 3`, `FractionDigits: 2`, `Length: 1`, `MinLength: 0`, `MaxLength: 9`, `WhiteSpace: "collapse"`, `HasMinLength: true`} {
		assert.Contains(t, string(actual), want)
	}
}

func TestGenerateSimpleContentInlineScalar(t *testing.T) {
	t.Parallel()
	file := parseSchemaFile(t, "inline.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:s="urn:source" targetNamespace="urn:source">
 <xs:simpleType name="number"><xs:restriction base="xs:decimal"><xs:minInclusive value="1"/></xs:restriction></xs:simpleType>
 <xs:complexType name="base"><xs:simpleContent><xs:extension base="s:number"/></xs:simpleContent></xs:complexType>
 <xs:element name="value"><xs:complexType><xs:simpleContent><xs:restriction base="s:base">
 <xs:simpleType><xs:restriction base="s:number"><xs:maxInclusive value="4"/></xs:restriction></xs:simpleType><xs:maxExclusive value="3"/>
 </xs:restriction></xs:simpleContent></xs:complexType></xs:element>
 </xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)
	actual, err := GenerateValidationSchema(index, "example", "schema")
	require.NoError(t, err)
	for _, want := range []string{`SimpleContent:`, `Space: "urn:source", Local: "number"`, `MaxInclusive: "4"`, `MaxExclusive: "3"`} {
		assert.Contains(t, string(actual), want)
	}
}

func TestGenerateSimpleContentRestrictionErrors(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, content string }{
		{"unresolved inline", `<xs:simpleType><xs:restriction base="missing"/></xs:simpleType>`},
		{"invalid inline", `<xs:simpleType/>`},
		{"invalid facet", `<xs:maxLength value="not-a-number"/>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := parseSchemaFile(t, "invalid.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:complexType name="base"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType><xs:complexType name="derived"><xs:simpleContent><xs:restriction base="base">`+test.content+`</xs:restriction></xs:simpleContent></xs:complexType></xs:schema>`)
			index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
			require.NoError(t, err)
			actual, err := GenerateValidationSchema(index, "example", "schema")
			assert.Error(t, err)
			assert.Nil(t, actual)
		})
	}
}
