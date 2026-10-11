package xsdgen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateValidationDerivationMetadata(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, defaults, body string
		want, absent         []string
	}{
		{name: "complex explicit", body: `<xs:complexType name="value" abstract="true" block="extension" final="restriction"/>`, want: []string{`Block: validationDerivation(1)`, `Final: validationDerivation(2)`, `Abstract: true`}},
		{name: "complex defaults filter substitution and list", defaults: `blockDefault="extension substitution" finalDefault="restriction list union"`, body: `<xs:complexType name="value"/>`, want: []string{`Block: validationDerivation(1)`, `Final: validationDerivation(2)`}},
		{name: "complex all", defaults: `blockDefault="#all" finalDefault="#all"`, body: `<xs:complexType name="value"/>`, want: []string{`Block: validationDerivation(3)`, `Final: validationDerivation(3)`}},
		{name: "complex empty override", defaults: `blockDefault="#all" finalDefault="#all"`, body: `<xs:complexType name="value" block="" final=""/>`, absent: []string{`Block:`, `Final:`}},
		{name: "simple explicit", body: `<xs:simpleType name="value" final="restriction list union"><xs:restriction base="xs:string"/></xs:simpleType>`, want: []string{`Final: validationDerivation(14)`}},
		{name: "simple default filters extension", defaults: `finalDefault="extension list"`, body: `<xs:simpleType name="value"><xs:restriction base="xs:string"/></xs:simpleType>`, want: []string{`Final: validationDerivation(4)`}},
		{name: "simple all", defaults: `finalDefault="#all"`, body: `<xs:simpleType name="value"><xs:restriction base="xs:string"/></xs:simpleType>`, want: []string{`Final: validationDerivation(14)`}},
		{name: "simple empty override", defaults: `finalDefault="#all"`, body: `<xs:simpleType name="value" final=""><xs:restriction base="xs:string"/></xs:simpleType>`, absent: []string{`Final:`}},
		{name: "element explicit", body: `<xs:element name="value" type="xs:string" block="restriction substitution" final="extension"/>`, want: []string{`Block: validationDerivation(18)`, `Final: validationDerivation(1)`}},
		{name: "element defaults filter list union", defaults: `blockDefault="restriction substitution" finalDefault="extension list union"`, body: `<xs:element name="value" type="xs:string"/>`, want: []string{`Block: validationDerivation(18)`, `Final: validationDerivation(1)`}},
		{name: "element all", defaults: `blockDefault="#all" finalDefault="#all"`, body: `<xs:element name="value" type="xs:string"/>`, want: []string{`Block: validationDerivation(19)`, `Final: validationDerivation(3)`}},
		{name: "element empty override", defaults: `blockDefault="#all" finalDefault="#all"`, body: `<xs:element name="value" type="xs:string" block="" final=""/>`, absent: []string{`Block:`, `Final:`}},
		{name: "nested local and inline retain defaults", defaults: `blockDefault="restriction" finalDefault="list"`, body: `<xs:element name="root"><xs:complexType><xs:sequence><xs:element name="child"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:element></xs:sequence></xs:complexType></xs:element>`, want: []string{`Block: validationDerivation(2)`, `Final: validationDerivation(4)`}},
		{name: "XML whitespace", body: `<xs:complexType name="value" block=" extension &#9; restriction &#10; " final=" restriction "/>`, want: []string{`Block: validationDerivation(3)`, `Final: validationDerivation(2)`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			file := parseSchemaFile(t, "derivation.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" `+test.defaults+`>`+test.body+`</xs:schema>`)
			index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
			require.NoError(t, err)
			actual, err := GenerateValidationSchema(index, "example", "schema")
			require.NoError(t, err)
			for _, want := range test.want {
				assert.Contains(t, string(actual), want)
			}
			for _, absent := range test.absent {
				assert.NotContains(t, string(actual), absent)
			}
		})
	}
}

func TestGenerateValidationDerivationSourceOwnership(t *testing.T) {
	t.Parallel()
	owner := parseSchemaFile(t, "owner.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:o="urn:owner" targetNamespace="urn:owner" blockDefault="restriction" finalDefault="list"><xs:group name="fields"><xs:sequence><xs:element name="child"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:element></xs:sequence></xs:group></xs:schema>`)
	consumer := parseSchemaFile(t, "consumer.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:o="urn:owner" blockDefault="extension" finalDefault="union"><xs:complexType name="root"><xs:group ref="o:fields"/></xs:complexType></xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{consumer, owner}})
	require.NoError(t, err)
	actual, err := GenerateValidationSchema(index, "example", "schema")
	require.NoError(t, err)
	source := string(actual)
	assert.Contains(t, source, `Block: validationDerivation(1)`)
	assert.Contains(t, source, `Block: validationDerivation(2)`)
	assert.Contains(t, source, `Final: validationDerivation(4)`)
	assert.NotContains(t, source, `Final: validationDerivation(8)`)
	assert.Equal(t, "", owner.Schema.Groups[0].Sequence.Particles[0].Element.Form)
}

func TestGenerateValidationMalformedDerivation(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, defaults, body string }{
		{name: "unknown schema block", defaults: `blockDefault="typo"`},
		{name: "unknown schema final", defaults: `finalDefault="typo"`},
		{name: "invalid schema block token", defaults: `blockDefault="list"`},
		{name: "invalid schema final token", defaults: `finalDefault="substitution"`},
		{name: "mixed all", defaults: `blockDefault="#all extension"`},
		{name: "non XML whitespace", defaults: `blockDefault="extension&#160;restriction"`},
		{name: "complex block substitution", body: `<xs:complexType name="value" block="substitution"/>`},
		{name: "complex final list", body: `<xs:complexType name="value" final="list"/>`},
		{name: "simple final extension", body: `<xs:simpleType name="value" final="extension"><xs:restriction base="xs:string"/></xs:simpleType>`},
		{name: "element block list", body: `<xs:element name="value" type="xs:string" block="list"/>`},
		{name: "element final union", body: `<xs:element name="value" type="xs:string" final="union"/>`},
		{name: "nested inline unknown", body: `<xs:element name="value"><xs:complexType block="typo"/></xs:element>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			schema, err := Parse(strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" ` + test.defaults + `>` + test.body + `</xs:schema>`))
			if err != nil {
				assert.Contains(t, err.Error(), "derivation")
				return
			}
			index, err := NewIndex(&Set{Files: []*SchemaFile{{Path: "invalid.xsd", Schema: schema}}})
			if err != nil {
				assert.Contains(t, err.Error(), "derivation")
				return
			}
			_, err = GenerateValidationSchema(index, "example", "schema")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "derivation")
		})
	}
}

func TestGenerateValidationProhibitedDerivation(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, body string }{
		{name: "simple restriction", body: `<xs:simpleType name="base" final="restriction"><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType name="child"><xs:restriction base="base"/></xs:simpleType>`},
		{name: "list item", body: `<xs:simpleType name="base" final="list"><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType name="child"><xs:list itemType="base"/></xs:simpleType>`},
		{name: "union member", body: `<xs:simpleType name="base" final="union"><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType name="child"><xs:union memberTypes="base xs:integer"/></xs:simpleType>`},
		{name: "complex extension", body: `<xs:complexType name="base" final="extension"/><xs:complexType name="child"><xs:complexContent><xs:extension base="base"/></xs:complexContent></xs:complexType>`},
		{name: "complex restriction", body: `<xs:complexType name="base" final="restriction"/><xs:complexType name="child"><xs:complexContent><xs:restriction base="base"/></xs:complexContent></xs:complexType>`},
		{name: "simple content extension", body: `<xs:complexType name="base" final="extension"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType><xs:complexType name="child"><xs:simpleContent><xs:extension base="base"/></xs:simpleContent></xs:complexType>`},
		{name: "simple content restriction", body: `<xs:complexType name="base" final="restriction"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType><xs:complexType name="child"><xs:simpleContent><xs:restriction base="base"/></xs:simpleContent></xs:complexType>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			file := parseSchemaFile(t, "prohibited.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`+test.body+`</xs:schema>`)
			index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
			require.NoError(t, err)
			_, err = GenerateValidationSchema(index, "example", "schema")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "final")
		})
	}
}

func TestGenerateValidationPermittedDerivation(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, defaults, body string }{
		{name: "simple different final", body: `<xs:simpleType name="base" final="list union"><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType name="child"><xs:restriction base="base"/></xs:simpleType>`},
		{name: "complex different final", body: `<xs:complexType name="base" final="restriction"/><xs:complexType name="child"><xs:complexContent><xs:extension base="base"/></xs:complexContent></xs:complexType>`},
		{name: "simple empty final clears restriction default", defaults: `finalDefault="restriction"`, body: `<xs:simpleType name="base" final=""><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType name="child"><xs:restriction base="base"/></xs:simpleType>`},
		{name: "complex empty final clears extension default", defaults: `finalDefault="extension"`, body: `<xs:complexType name="base" final=""/><xs:complexType name="child"><xs:complexContent><xs:extension base="base"/></xs:complexContent></xs:complexType>`},
		{name: "element final does not prohibit type derivation", body: `<xs:complexType name="base"/><xs:complexType name="child"><xs:complexContent><xs:extension base="base"/></xs:complexContent></xs:complexType><xs:element name="value" type="base" final="#all"/>`},
		{name: "type block does not prohibit schema derivation", body: `<xs:complexType name="base" block="#all"/><xs:complexType name="child"><xs:complexContent><xs:extension base="base"/></xs:complexContent></xs:complexType>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			file := parseSchemaFile(t, "permitted.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" `+test.defaults+`>`+test.body+`</xs:schema>`)
			index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
			require.NoError(t, err)
			first, err := GenerateValidationSchema(index, "example", "schema")
			require.NoError(t, err)
			second, err := GenerateValidationSchema(index, "example", "schema")
			require.NoError(t, err)
			assert.Equal(t, first, second)
		})
	}
}

func TestParseDerivationPresence(t *testing.T) {
	t.Parallel()
	file := parseSchemaFile(t, "presence.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" blockDefault="#all" finalDefault="#all"><xs:simpleType name="absent"><xs:restriction base="xs:string"/></xs:simpleType><xs:simpleType name="empty" final=""><xs:restriction base="xs:string"/></xs:simpleType><xs:complexType name="absentComplex"/><xs:complexType name="emptyComplex" block="" final=""/><xs:element name="absentElement" type="xs:string"/><xs:element name="emptyElement" type="xs:string" block="" final=""/></xs:schema>`)
	schema := file.Schema
	assert.Nil(t, schema.SimpleTypes[0].Final)
	require.NotNil(t, schema.SimpleTypes[1].Final)
	assert.Empty(t, *schema.SimpleTypes[1].Final)
	assert.Nil(t, schema.ComplexTypes[0].Block)
	assert.Nil(t, schema.ComplexTypes[0].Final)
	require.NotNil(t, schema.ComplexTypes[1].Block)
	require.NotNil(t, schema.ComplexTypes[1].Final)
	assert.Empty(t, *schema.ComplexTypes[1].Block)
	assert.Empty(t, *schema.ComplexTypes[1].Final)
	assert.Nil(t, schema.Elements[0].Block)
	assert.Nil(t, schema.Elements[0].Final)
	require.NotNil(t, schema.Elements[1].Block)
	require.NotNil(t, schema.Elements[1].Final)
	assert.Empty(t, *schema.Elements[1].Block)
	assert.Empty(t, *schema.Elements[1].Final)
}

func TestGenerateValidationDerivationReferencedSource(t *testing.T) {
	t.Parallel()
	owner := parseSchemaFile(t, "owner.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:o="urn:owner" targetNamespace="urn:owner" blockDefault="restriction" finalDefault="extension"><xs:complexType name="base"/><xs:element name="value" type="o:base"/></xs:schema>`)
	consumer := parseSchemaFile(t, "consumer.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:o="urn:owner" blockDefault="extension" finalDefault="restriction"><xs:complexType name="root"><xs:sequence><xs:element ref="o:value"/></xs:sequence></xs:complexType><xs:complexType name="child"><xs:complexContent><xs:restriction base="o:base"/></xs:complexContent></xs:complexType></xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{consumer, owner}})
	require.NoError(t, err)
	actual, err := GenerateValidationSchema(index, "example", "schema")
	require.NoError(t, err)
	assert.Contains(t, string(actual), `&validationElementSchema{Name: validationQName{Space: "urn:owner", Local: "value"}, Reference:`)
	assert.Contains(t, string(actual), `&validationElementSchema{Block: validationDerivation(2), Final: validationDerivation(1), Name: validationQName{Space: "urn:owner", Local: "value"}, Type:`)
	require.NotNil(t, owner.Schema)
	assert.Nil(t, owner.Schema.ComplexTypes[0].Final)
	assert.Nil(t, owner.Schema.Elements[0].Block)
	assert.Nil(t, owner.Schema.Elements[0].Final)
}
