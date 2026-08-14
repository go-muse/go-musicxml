package xsdgen

import (
	"errors"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanAttributeValueConstraints(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "constraints.xsd", `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:s="urn:score"
	targetNamespace="urn:score">
	<xs:attribute
		name="empty-label"
		type="xs:string"
		default=""/>
	<xs:attribute
		name="shared-state"
		type="xs:token"
		fixed="ready"/>
	<xs:attributeGroup name="shared">
		<xs:attribute ref="s:empty-label"/>
		<xs:attribute
			ref="s:shared-state"
			fixed="ready"/>
	</xs:attributeGroup>
	<xs:complexType name="record">
		<xs:attribute
			name="level"
			type="xs:unsignedInt"
			default="3"/>
		<xs:attribute
			name="enabled"
			type="xs:boolean"
			use="required"
			fixed="true"/>
		<xs:attributeGroup ref="s:shared"/>
	</xs:complexType>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	plans, err := PlanExpandedComplexTypes(index)
	require.NoError(t, err)

	record := requireComplexTypePlan(t, plans, "record")
	require.Len(t, record.Definition.Attributes, 4)

	assert.Equal(
		t,
		&ValueConstraint{Kind: ValueDefault, Value: "3"},
		record.Definition.Attributes[0].Constraint,
	)
	assert.Equal(
		t,
		&ValueConstraint{Kind: ValueFixed, Value: "true"},
		record.Definition.Attributes[1].Constraint,
	)
	assert.Equal(
		t,
		&ValueConstraint{Kind: ValueDefault, Value: ""},
		record.Definition.Attributes[2].Constraint,
	)
	assert.Equal(
		t,
		&ValueConstraint{Kind: ValueFixed, Value: "ready"},
		record.Definition.Attributes[3].Constraint,
	)
}

func TestPlanAttributeValueConstraintError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
	}{
		{
			name: "default and fixed",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="record">
		<xs:attribute
			name="state"
			type="xs:string"
			default="draft"
			fixed="ready"/>
	</xs:complexType>
</xs:schema>`,
		},
		{
			name: "required default",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="record">
		<xs:attribute
			name="state"
			type="xs:string"
			use="required"
			default="ready"/>
	</xs:complexType>
</xs:schema>`,
		},
		{
			name: "prohibited fixed",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="record">
		<xs:attribute
			name="state"
			type="xs:string"
			use="prohibited"
			fixed="ready"/>
	</xs:complexType>
</xs:schema>`,
		},
		{
			name: "conflicting inherited fixed",
			schema: `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:s="urn:score"
	targetNamespace="urn:score">
	<xs:attribute
		name="state"
		type="xs:string"
		fixed="ready"/>
	<xs:complexType name="record">
		<xs:attribute
			ref="s:state"
			fixed="draft"/>
	</xs:complexType>
</xs:schema>`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			file := parseSchemaFile(
				t,
				"constraints.xsd",
				test.schema,
			)
			index, err := NewIndex(
				&Set{Files: []*SchemaFile{file}},
			)
			require.NoError(t, err)

			_, err = PlanComplexTypes(index)

			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidAttribute))
		})
	}
}

func TestGenerateAttributeValueConstraints(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "constraints.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="record">
		<xs:attribute
			name="level"
			type="xs:unsignedInt"
			default="3"/>
		<xs:attribute
			name="enabled"
			type="xs:boolean"
			use="required"
			fixed="1"/>
	</xs:complexType>
</xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	actual, err := GenerateComplexTypes(index, "example")
	require.NoError(t, err)

	_, err = parser.ParseFile(
		token.NewFileSet(),
		"zz_generated_constraints.go",
		actual,
		parser.SkipObjectResolution,
	)
	require.NoError(t, err)

	source := string(actual)
	assert.Contains(
		t,
		source,
		"func (value *Record) EffectiveLevel() uint32",
	)
	assert.Contains(t, source, "return 3")
	assert.Contains(
		t,
		source,
		"func (value *Record) EffectiveEnabled() bool",
	)
	assert.Contains(
		t,
		source,
		"func (value *Record) EnabledMatchesFixed() bool",
	)
	assert.Contains(t, source, "value.Enabled == true")
}
