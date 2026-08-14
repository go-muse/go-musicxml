package xsdgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanExpandedComplexTypes(t *testing.T) {
	t.Parallel()

	shared := parseSchemaFile(t, "shared.xsd", `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:g="urn:groups"
	targetNamespace="urn:groups"
	elementFormDefault="qualified"
	attributeFormDefault="qualified">
	<xs:group name="leaf">
		<xs:sequence>
			<xs:element name="shared-element" type="xs:string"/>
		</xs:sequence>
	</xs:group>
	<xs:group name="outer">
		<xs:choice>
			<xs:group ref="g:leaf"/>
			<xs:element name="alternative" type="xs:integer"/>
		</xs:choice>
	</xs:group>
	<xs:attributeGroup name="leaf-attributes">
		<xs:attribute name="shared-attribute" type="xs:string"/>
	</xs:attributeGroup>
	<xs:attributeGroup name="outer-attributes">
		<xs:attribute name="direct-attribute" type="xs:integer"/>
		<xs:attributeGroup ref="g:leaf-attributes"/>
	</xs:attributeGroup>
</xs:schema>`)

	score := parseSchemaFile(t, "score.xsd", `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:g="urn:groups"
	targetNamespace="urn:score"
	elementFormDefault="qualified"
	attributeFormDefault="unqualified">
	<xs:complexType name="score">
		<xs:sequence>
			<xs:element name="before" type="xs:string"/>
			<xs:group
				ref="g:outer"
				minOccurs="0"
				maxOccurs="2"/>
			<xs:element name="after" type="xs:string"/>
		</xs:sequence>
		<xs:attribute name="local-attribute" type="xs:string"/>
		<xs:attributeGroup ref="g:outer-attributes"/>
	</xs:complexType>
</xs:schema>`)

	index, err := NewIndex(&Set{
		Files: []*SchemaFile{shared, score},
	})
	require.NoError(t, err)

	plans, err := PlanExpandedComplexTypes(index)
	require.NoError(t, err)
	require.Len(t, plans, 1)

	definition := plans[0].Definition
	require.Empty(t, definition.AttributeGroups)
	require.NotNil(t, definition.Particle)
	require.Len(t, definition.Particle.Children, 3)

	outer := definition.Particle.Children[1]
	assert.Equal(t, ParticleChoice, outer.Kind)
	assert.Equal(
		t,
		OccurrenceRange{Min: 0, Max: 2},
		outer.Occurrence,
	)
	require.NotNil(t, outer.Reference)
	assert.Equal(t, "outer", outer.Reference.Name.Local)
	require.Len(t, outer.Children, 2)

	leaf := outer.Children[0]
	assert.Equal(t, ParticleSequence, leaf.Kind)
	assert.Equal(
		t,
		OccurrenceRange{Min: 1, Max: 1},
		leaf.Occurrence,
	)
	require.NotNil(t, leaf.Reference)
	assert.Equal(t, "leaf", leaf.Reference.Name.Local)
	require.Len(t, leaf.Children, 1)

	element := leaf.Children[0]
	assert.Equal(t, ParticleElement, element.Kind)
	require.NotNil(t, element.Element)
	assert.Equal(
		t,
		QName{
			Namespace: "urn:groups",
			Local:     "shared-element",
		},
		element.Element.Name,
	)
	require.NotNil(t, element.Element.Type)
	assert.Equal(
		t,
		"string",
		element.Element.Type.Declaration.Name.Local,
	)

	require.Len(t, definition.Attributes, 3)
	assert.Equal(
		t,
		QName{Local: "local-attribute"},
		definition.Attributes[0].Name,
	)
	assert.Equal(
		t,
		QName{
			Namespace: "urn:groups",
			Local:     "direct-attribute",
		},
		definition.Attributes[1].Name,
	)
	assert.Equal(
		t,
		QName{
			Namespace: "urn:groups",
			Local:     "shared-attribute",
		},
		definition.Attributes[2].Name,
	)
}

func TestPlanExpandedComplexTypesReferenceCycle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		schema    string
		wantSpace SymbolSpace
		wantPath  []QName
	}{
		{
			name: "groups",
			schema: `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:s="urn:score"
	targetNamespace="urn:score">
	<xs:group name="a">
		<xs:sequence>
			<xs:group ref="s:b"/>
		</xs:sequence>
	</xs:group>
	<xs:group name="b">
		<xs:choice>
			<xs:group ref="s:a"/>
		</xs:choice>
	</xs:group>
	<xs:complexType name="score">
		<xs:group ref="s:a"/>
	</xs:complexType>
</xs:schema>`,
			wantSpace: GroupSymbolSpace,
			wantPath: []QName{
				{Namespace: "urn:score", Local: "a"},
				{Namespace: "urn:score", Local: "b"},
				{Namespace: "urn:score", Local: "a"},
			},
		},
		{
			name: "attribute groups",
			schema: `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:s="urn:score"
	targetNamespace="urn:score">
	<xs:attributeGroup name="a">
		<xs:attributeGroup ref="s:b"/>
	</xs:attributeGroup>
	<xs:attributeGroup name="b">
		<xs:attributeGroup ref="s:a"/>
	</xs:attributeGroup>
	<xs:complexType name="score">
		<xs:attributeGroup ref="s:a"/>
	</xs:complexType>
</xs:schema>`,
			wantSpace: AttributeGroupSymbolSpace,
			wantPath: []QName{
				{Namespace: "urn:score", Local: "a"},
				{Namespace: "urn:score", Local: "b"},
				{Namespace: "urn:score", Local: "a"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			file := parseSchemaFile(t, "cycle.xsd", test.schema)

			index, err := NewIndex(&Set{
				Files: []*SchemaFile{file},
			})
			require.NoError(t, err)

			plans, err := PlanExpandedComplexTypes(index)

			assert.Nil(t, plans)
			assert.ErrorIs(t, err, ErrReferenceCycle)

			var cycleErr *ReferenceCycleError
			if assert.ErrorAs(t, err, &cycleErr) {
				assert.Equal(t, test.wantSpace, cycleErr.Space)
				assert.Equal(t, test.wantPath, cycleErr.Path)
			}
		})
	}
}

func TestPlanExpandedComplexTypesMultipleAnyAttributes(
	t *testing.T,
) {
	t.Parallel()

	file := parseSchemaFile(t, "attributes.xsd", `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:s="urn:score"
	targetNamespace="urn:score">
	<xs:attributeGroup name="wildcard">
		<xs:anyAttribute namespace="##other"/>
	</xs:attributeGroup>
	<xs:complexType name="score">
		<xs:attributeGroup ref="s:wildcard"/>
		<xs:anyAttribute namespace="##local"/>
	</xs:complexType>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	plans, err := PlanExpandedComplexTypes(index)

	assert.Nil(t, plans)
	assert.ErrorIs(t, err, ErrMultipleAnyAttributes)
}
