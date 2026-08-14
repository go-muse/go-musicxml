package xsdgen

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Parallel()

	const input = `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	elementFormDefault="qualified"
	attributeFormDefault="unqualified">
	<xs:import
		namespace="http://www.w3.org/1999/xlink"
		schemaLocation="xlink.xsd"/>
	<xs:simpleType name="above-below">
		<xs:restriction base="xs:token">
			<xs:enumeration value="above"/>
			<xs:enumeration value="below"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:complexType name="pitch">
		<xs:sequence>
			<xs:element name="step" type="step"/>
			<xs:group ref="editorial"/>
			<xs:choice minOccurs="0">
				<xs:element name="alter" type="xs:decimal"/>
				<xs:element name="octave" type="xs:integer"/>
			</xs:choice>
		</xs:sequence>
		<xs:attribute name="id" type="xs:ID"/>
	</xs:complexType>
	<xs:element name="score-partwise" type="score-partwise"/>
</xs:schema>`

	schema, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	assert.Equal(
		t,
		NamespaceBindings{
			"xml": XMLNamespace,
			"xs":  Namespace,
		},
		schema.Namespaces,
	)

	require.Len(t, schema.Imports, 1)
	assert.Equal(t, "xlink.xsd", schema.Imports[0].SchemaLocation)

	require.Len(t, schema.SimpleTypes, 1)
	require.NotNil(t, schema.SimpleTypes[0].Restriction)
	require.Len(t, schema.SimpleTypes[0].Restriction.Enumerations, 2)
	assert.Equal(
		t,
		"below",
		schema.SimpleTypes[0].Restriction.Enumerations[1].Value,
	)

	require.Len(t, schema.ComplexTypes, 1)
	require.NotNil(t, schema.ComplexTypes[0].Sequence)
	particles := schema.ComplexTypes[0].Sequence.Particles
	require.Len(t, particles, 3)
	require.NotNil(t, particles[0].Element)
	require.NotNil(t, particles[1].Group)
	require.NotNil(t, particles[2].Choice)

	assert.Equal(t, "step", particles[0].Element.Name)
	assert.Equal(t, "editorial", particles[1].Group.Ref)
	assert.Equal(t, "0", particles[2].Choice.MinOccurs)

	base, err := schema.ResolveQName(
		schema.SimpleTypes[0].Restriction.Base,
	)
	assert.NoError(t, err)
	assert.Equal(
		t,
		QName{
			Namespace: Namespace,
			Local:     "token",
			Prefix:    "xs",
		},
		base,
	)

	memberTypes, err := schema.ResolveQNames("xs:decimal above-below")
	assert.NoError(t, err)
	assert.Equal(
		t,
		[]QName{
			{
				Namespace: Namespace,
				Local:     "decimal",
				Prefix:    "xs",
			},
			{
				Local: "above-below",
			},
		},
		memberTypes,
	)
}

func TestParseDefaultNamespaceBinding(t *testing.T) {
	t.Parallel()

	const input = `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns="urn:default"
	xmlns:types="urn:types">
</xs:schema>`

	schema, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	assert.Equal(
		t,
		NamespaceBindings{
			"":      "urn:default",
			"types": "urn:types",
			"xml":   XMLNamespace,
			"xs":    Namespace,
		},
		schema.Namespaces,
	)

	resolved, err := schema.ResolveQName("value")
	assert.NoError(t, err)
	assert.Equal(
		t,
		QName{
			Namespace: "urn:default",
			Local:     "value",
		},
		resolved,
	)
}

func TestParseError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		reader  io.Reader
		wantErr error
	}{
		{
			name:    "nil reader",
			wantErr: ErrNilReader,
		},
		{
			name: "wrong root",
			reader: strings.NewReader(
				`<root xmlns="http://www.w3.org/2001/XMLSchema"/>`,
			),
		},
		{
			name: "trailing content",
			reader: strings.NewReader(
				`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>` +
					`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`,
			),
			wantErr: ErrTrailingContent,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			schema, err := Parse(test.reader)

			assert.Nil(t, schema)
			if test.wantErr == nil {
				assert.Error(t, err)
				return
			}
			assert.ErrorIs(t, err, test.wantErr)
		})
	}
}
