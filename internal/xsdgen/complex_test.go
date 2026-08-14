package xsdgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanComplexTypes(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "types.xsd", `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:s="urn:score"
	targetNamespace="urn:score"
	elementFormDefault="qualified"
	attributeFormDefault="unqualified">
	<xs:simpleType name="label">
		<xs:restriction base="xs:string"/>
	</xs:simpleType>
	<xs:attribute name="shared-attribute" type="xs:integer"/>
	<xs:attributeGroup name="common-attributes">
		<xs:attribute name="common" type="xs:string"/>
	</xs:attributeGroup>
	<xs:group name="common-elements">
		<xs:sequence>
			<xs:element name="from-group" type="xs:string"/>
		</xs:sequence>
	</xs:group>
	<xs:element name="shared-element" type="xs:string"/>
	<xs:complexType name="base">
		<xs:sequence>
			<xs:element name="required" type="xs:string"/>
			<xs:element
				name="optional"
				type="xs:decimal"
				minOccurs="0"/>
			<xs:group
				ref="s:common-elements"
				minOccurs="0"
				maxOccurs="2"/>
			<xs:choice minOccurs="0" maxOccurs="unbounded">
				<xs:element name="inline">
					<xs:complexType>
						<xs:simpleContent>
							<xs:extension base="s:label">
								<xs:attribute
									name="code"
									type="xs:string"
									use="required"/>
							</xs:extension>
						</xs:simpleContent>
					</xs:complexType>
				</xs:element>
				<xs:element ref="s:shared-element"/>
			</xs:choice>
		</xs:sequence>
		<xs:attribute
			name="id"
			type="xs:ID"
			use="required"/>
		<xs:attribute ref="s:shared-attribute"/>
		<xs:attributeGroup ref="s:common-attributes"/>
	</xs:complexType>
	<xs:complexType name="derived">
		<xs:complexContent>
			<xs:extension base="s:base">
				<xs:attribute name="extra" type="xs:boolean"/>
			</xs:extension>
		</xs:complexContent>
	</xs:complexType>
	<xs:complexType name="formatted">
		<xs:simpleContent>
			<xs:extension base="s:label">
				<xs:attribute name="lang" type="xs:language"/>
			</xs:extension>
		</xs:simpleContent>
	</xs:complexType>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	plans, err := PlanComplexTypes(index)
	require.NoError(t, err)
	require.Len(t, plans, 3)

	base := requireComplexTypePlan(t, plans, "base")
	assert.Equal(t, "Base", base.GoName)
	assert.Equal(t, ComplexTypeDirect, base.Definition.Form)
	assert.Nil(t, base.Definition.Base)
	require.NotNil(t, base.Definition.Particle)
	assert.Equal(t, ParticleSequence, base.Definition.Particle.Kind)
	assert.Equal(
		t,
		OccurrenceRange{Min: 1, Max: 1},
		base.Definition.Particle.Occurrence,
	)
	require.Len(t, base.Definition.Particle.Children, 4)

	required := base.Definition.Particle.Children[0]
	assert.Equal(t, ParticleElement, required.Kind)
	assert.Equal(t, "required", required.Element.Name.Local)
	assert.Equal(t, "urn:score", required.Element.Name.Namespace)
	assert.True(t, required.Occurrence.Required())
	assert.False(t, required.Occurrence.Repeated())
	require.NotNil(t, required.Element.Type)
	assert.Equal(
		t,
		"string",
		required.Element.Type.Declaration.Name.Local,
	)

	optional := base.Definition.Particle.Children[1]
	assert.Equal(
		t,
		OccurrenceRange{Min: 0, Max: 1},
		optional.Occurrence,
	)
	assert.False(t, optional.Occurrence.Required())

	group := base.Definition.Particle.Children[2]
	assert.Equal(t, ParticleGroup, group.Kind)
	assert.Equal(
		t,
		OccurrenceRange{Min: 0, Max: 2},
		group.Occurrence,
	)
	assert.True(t, group.Occurrence.Repeated())
	assert.Equal(t, "common-elements", group.Reference.Name.Local)

	choice := base.Definition.Particle.Children[3]
	assert.Equal(t, ParticleChoice, choice.Kind)
	assert.Equal(
		t,
		OccurrenceRange{Min: 0, Unbounded: true},
		choice.Occurrence,
	)
	require.Len(t, choice.Children, 2)

	inline := choice.Children[0].Element
	require.NotNil(t, inline.Type)
	require.NotNil(t, inline.Type.InlineComplex)
	assert.Equal(
		t,
		ComplexTypeSimpleContentExtension,
		inline.Type.InlineComplex.Form,
	)
	assert.Equal(
		t,
		"label",
		inline.Type.InlineComplex.Base.Name.Local,
	)
	require.Len(t, inline.Type.InlineComplex.Attributes, 1)
	assert.True(t, inline.Type.InlineComplex.Attributes[0].Required())

	reference := choice.Children[1].Element
	assert.Equal(t, "shared-element", reference.Name.Local)
	assert.Equal(
		t,
		"shared-element",
		reference.Reference.Name.Local,
	)
	assert.Nil(t, reference.Type)

	require.Len(t, base.Definition.Attributes, 2)
	assert.Equal(t, "id", base.Definition.Attributes[0].Name.Local)
	assert.Empty(t, base.Definition.Attributes[0].Name.Namespace)
	assert.True(t, base.Definition.Attributes[0].Required())
	assert.Equal(
		t,
		"ID",
		base.Definition.Attributes[0].Type.Declaration.Name.Local,
	)

	shared := base.Definition.Attributes[1]
	assert.Equal(t, "urn:score", shared.Name.Namespace)
	assert.Equal(t, "shared-attribute", shared.Name.Local)
	assert.Equal(
		t,
		"integer",
		shared.Type.Declaration.Name.Local,
	)
	assert.Equal(t, AttributeOptional, shared.Use)

	require.Len(t, base.Definition.AttributeGroups, 1)
	assert.Equal(
		t,
		"common-attributes",
		base.Definition.AttributeGroups[0].Reference.Name.Local,
	)

	derived := requireComplexTypePlan(t, plans, "derived")
	assert.Equal(
		t,
		ComplexTypeComplexContentExtension,
		derived.Definition.Form,
	)
	assert.Equal(t, "base", derived.Definition.Base.Name.Local)
	assert.Nil(t, derived.Definition.Particle)
	require.Len(t, derived.Definition.Attributes, 1)

	formatted := requireComplexTypePlan(t, plans, "formatted")
	assert.Equal(
		t,
		ComplexTypeSimpleContentExtension,
		formatted.Definition.Form,
	)
	assert.Equal(t, "label", formatted.Definition.Base.Name.Local)
	require.Len(t, formatted.Definition.Attributes, 1)
}

func TestParseOccurrence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		source  Occurs
		want    OccurrenceRange
		wantErr error
	}{
		{
			name: "defaults",
			want: OccurrenceRange{Min: 1, Max: 1},
		},
		{
			name: "bounded",
			source: Occurs{
				MinOccurs: "0",
				MaxOccurs: "8",
			},
			want: OccurrenceRange{Min: 0, Max: 8},
		},
		{
			name: "unbounded",
			source: Occurs{
				MinOccurs: "2",
				MaxOccurs: "unbounded",
			},
			want: OccurrenceRange{
				Min:       2,
				Unbounded: true,
			},
		},
		{
			name: "invalid minimum",
			source: Occurs{
				MinOccurs: "-1",
			},
			wantErr: ErrInvalidOccurrence,
		},
		{
			name: "invalid maximum",
			source: Occurs{
				MaxOccurs: "many",
			},
			wantErr: ErrInvalidOccurrence,
		},
		{
			name: "maximum below minimum",
			source: Occurs{
				MinOccurs: "2",
				MaxOccurs: "1",
			},
			wantErr: ErrInvalidOccurrence,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := parseOccurrence(test.source)

			if test.wantErr != nil {
				assert.Zero(t, actual)
				assert.ErrorIs(t, err, test.wantErr)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, actual)
		})
	}
}

func TestPlanComplexTypeAdditionalForms(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="text">
		<xs:restriction base="xs:string"/>
	</xs:simpleType>
	<xs:complexType name="base">
		<xs:sequence>
			<xs:element name="value" type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
	<xs:complexType name="simple-restricted">
		<xs:simpleContent>
			<xs:restriction base="text">
				<xs:attribute
					name="legacy"
					type="xs:string"
					use="prohibited"/>
			</xs:restriction>
		</xs:simpleContent>
	</xs:complexType>
	<xs:complexType name="complex-restricted">
		<xs:complexContent>
			<xs:restriction base="base">
				<xs:all>
					<xs:element name="value" type="xs:string"/>
				</xs:all>
			</xs:restriction>
		</xs:complexContent>
	</xs:complexType>
	<xs:complexType name="wildcard">
		<xs:sequence>
			<xs:element name="status">
				<xs:simpleType>
					<xs:restriction base="xs:token">
						<xs:enumeration value="ready"/>
					</xs:restriction>
				</xs:simpleType>
			</xs:element>
			<xs:any
				namespace="##other"
				processContents="lax"
				minOccurs="0"
				maxOccurs="unbounded"/>
		</xs:sequence>
		<xs:attribute name="flag">
			<xs:simpleType>
				<xs:restriction base="xs:token">
					<xs:enumeration value="on"/>
				</xs:restriction>
			</xs:simpleType>
		</xs:attribute>
	</xs:complexType>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	plans, err := PlanComplexTypes(index)
	require.NoError(t, err)
	require.Len(t, plans, 4)

	simpleRestricted := requireComplexTypePlan(
		t,
		plans,
		"simple-restricted",
	)
	assert.Equal(
		t,
		ComplexTypeSimpleContentRestriction,
		simpleRestricted.Definition.Form,
	)
	assert.Equal(
		t,
		AttributeProhibited,
		simpleRestricted.Definition.Attributes[0].Use,
	)

	complexRestricted := requireComplexTypePlan(
		t,
		plans,
		"complex-restricted",
	)
	assert.Equal(
		t,
		ComplexTypeComplexContentRestriction,
		complexRestricted.Definition.Form,
	)
	assert.Equal(
		t,
		ParticleAll,
		complexRestricted.Definition.Particle.Kind,
	)

	wildcard := requireComplexTypePlan(t, plans, "wildcard")
	require.Len(t, wildcard.Definition.Particle.Children, 2)

	status := wildcard.Definition.Particle.Children[0].Element
	require.NotNil(t, status.Type.InlineSimple)
	require.Len(t, status.Type.InlineSimple.Enumerations, 1)
	assert.Equal(
		t,
		"WildcardStatusReady",
		status.Type.InlineSimple.Enumerations[0].GoName,
	)

	any := wildcard.Definition.Particle.Children[1]
	assert.Equal(t, ParticleAny, any.Kind)
	assert.True(t, any.Occurrence.Unbounded)
	assert.Equal(t, "##other", any.Source.Any.Namespace)

	require.Len(t, wildcard.Definition.Attributes, 1)
	flag := wildcard.Definition.Attributes[0]
	require.NotNil(t, flag.Type.InlineSimple)
	assert.Equal(
		t,
		"WildcardFlagOn",
		flag.Type.InlineSimple.Enumerations[0].GoName,
	)
}

func TestPlanComplexTypesError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		wantErr error
	}{
		{
			name: "both content wrappers",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="value">
		<xs:simpleContent>
			<xs:extension base="xs:string"/>
		</xs:simpleContent>
		<xs:complexContent>
			<xs:extension base="xs:anyType"/>
		</xs:complexContent>
	</xs:complexType>
</xs:schema>`,
			wantErr: ErrInvalidComplexType,
		},
		{
			name: "missing simple content derivation",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="value">
		<xs:simpleContent/>
	</xs:complexType>
</xs:schema>`,
			wantErr: ErrInvalidComplexType,
		},
		{
			name: "simple complex content base",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="base">
		<xs:restriction base="xs:string"/>
	</xs:simpleType>
	<xs:complexType name="value">
		<xs:complexContent>
			<xs:extension base="base"/>
		</xs:complexContent>
	</xs:complexType>
</xs:schema>`,
			wantErr: ErrInvalidComplexType,
		},
		{
			name: "multiple top-level particles",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="value">
		<xs:sequence/>
		<xs:choice/>
	</xs:complexType>
</xs:schema>`,
			wantErr: ErrInvalidParticle,
		},
		{
			name: "invalid occurrence",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="value">
		<xs:sequence maxOccurs="wrong"/>
	</xs:complexType>
</xs:schema>`,
			wantErr: ErrInvalidOccurrence,
		},
		{
			name: "missing element name",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="value">
		<xs:sequence>
			<xs:element type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
</xs:schema>`,
			wantErr: ErrInvalidElement,
		},
		{
			name: "invalid attribute use",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="value">
		<xs:attribute
			name="id"
			type="xs:string"
			use="sometimes"/>
	</xs:complexType>
</xs:schema>`,
			wantErr: ErrInvalidAttribute,
		},
		{
			name: "complex attribute type",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="base"/>
	<xs:complexType name="value">
		<xs:attribute name="id" type="base"/>
	</xs:complexType>
</xs:schema>`,
			wantErr: ErrInvalidAttribute,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			file := parseSchemaFile(t, "types.xsd", test.schema)
			index, err := NewIndex(&Set{
				Files: []*SchemaFile{file},
			})
			require.NoError(t, err)

			plans, err := PlanComplexTypes(index)

			assert.Nil(t, plans)
			assert.ErrorIs(t, err, ErrInvalidComplexType)
			assert.ErrorIs(t, err, test.wantErr)
		})
	}
}

func TestPlanComplexTypesNilIndex(t *testing.T) {
	t.Parallel()

	plans, err := PlanComplexTypes(nil)

	assert.Nil(t, plans)
	assert.ErrorIs(t, err, ErrNilIndex)
}

func requireComplexTypePlan(
	t *testing.T,
	plans []ComplexTypePlan,
	name string,
) ComplexTypePlan {
	t.Helper()

	for _, plan := range plans {
		if plan.Declaration.Name.Local == name {
			return plan
		}
	}

	require.Failf(t, "missing complex type plan", "name: %s", name)

	return ComplexTypePlan{}
}
