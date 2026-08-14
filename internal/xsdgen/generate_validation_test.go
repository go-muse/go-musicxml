package xsdgen

import (
	"errors"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateValidationSchema(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "validation.xsd", `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:s="urn:score"
	targetNamespace="urn:score"
	elementFormDefault="qualified">
	<xs:simpleType name="level">
		<xs:restriction base="xs:positiveInteger">
			<xs:minInclusive value="1"/>
			<xs:maxInclusive value="8"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:complexType name="note">
		<xs:choice>
			<xs:element name="pitch" type="xs:string"/>
			<xs:element name="rest" type="xs:string"/>
		</xs:choice>
		<xs:attribute
			name="level"
			type="s:level"
			use="required"/>
	</xs:complexType>
	<xs:element name="score" type="s:note"/>
</xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	actual, err := GenerateValidationSchema(
		index,
		"example",
		"scoreValidationSchema",
	)
	require.NoError(t, err)

	_, err = parser.ParseFile(
		token.NewFileSet(),
		"zz_generated_validation.go",
		actual,
		parser.SkipObjectResolution,
	)
	require.NoError(t, err)

	source := string(actual)
	assert.Contains(
		t,
		source,
		"var scoreValidationSchema = validationSchemaSet{",
	)
	assert.Contains(t, source, `Local: "level"`)
	assert.Contains(t, source, `MinInclusive: "1"`)
	assert.Contains(t, source, `MaxInclusive: "8"`)
	assert.Contains(t, source, `Kind: validationParticleKind("choice")`)
	assert.Contains(t, source, `Use: validationAttributeUse("required")`)
	assert.Contains(t, source, `Local: "score"`)
}

func TestGenerateValidationSchemaError(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "validation.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:element name="score" type="xs:string"/>
</xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	tests := []struct {
		name         string
		index        *Index
		packageName  string
		variableName string
		wantErr      error
	}{
		{
			name:         "nil index",
			packageName:  "example",
			variableName: "schema",
			wantErr:      ErrNilIndex,
		},
		{
			name:         "invalid package",
			index:        index,
			packageName:  "invalid-package",
			variableName: "schema",
			wantErr:      ErrInvalidPackageName,
		},
		{
			name:         "invalid variable",
			index:        index,
			packageName:  "example",
			variableName: "invalid-variable",
			wantErr:      ErrInvalidValidationGeneration,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := GenerateValidationSchema(
				test.index,
				test.packageName,
				test.variableName,
			)

			assert.Error(t, err)
			assert.True(t, errors.Is(err, test.wantErr))
		})
	}
}
