package xsdgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanSimpleTypes(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="css-font-size">
		<xs:restriction base="xs:token">
			<xs:enumeration value="xx-small"/>
			<xs:enumeration value="x-large"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:simpleType name="font-size">
		<xs:union memberTypes="xs:decimal css-font-size"/>
	</xs:simpleType>
	<xs:simpleType name="number-or-normal">
		<xs:union memberTypes="xs:decimal">
			<xs:simpleType>
				<xs:restriction base="xs:token">
					<xs:enumeration value="normal"/>
				</xs:restriction>
			</xs:simpleType>
		</xs:union>
	</xs:simpleType>
	<xs:simpleType name="token-list">
		<xs:list itemType="xs:token"/>
	</xs:simpleType>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	plans, err := PlanSimpleTypes(index)
	require.NoError(t, err)
	require.Len(t, plans, 4)

	css := requireSimpleTypePlan(t, plans, "css-font-size")
	assert.Equal(t, "CSSFontSize", css.GoName)
	assert.Equal(
		t,
		SimpleTypeRestriction,
		css.Definition.Form,
	)
	require.NotNil(t, css.Definition.Base)
	assert.Equal(
		t,
		"token",
		css.Definition.Base.Declaration.Name.Local,
	)
	assert.Equal(
		t,
		[]EnumerationPlan{
			{
				Value:  "xx-small",
				GoName: "CSSFontSizeXxSmall",
				Facet: &Facet{
					Value: "xx-small",
				},
			},
			{
				Value:  "x-large",
				GoName: "CSSFontSizeXLarge",
				Facet: &Facet{
					Value: "x-large",
				},
			},
		},
		css.Definition.Enumerations,
	)

	fontSize := requireSimpleTypePlan(t, plans, "font-size")
	assert.Equal(t, SimpleTypeUnion, fontSize.Definition.Form)
	require.Len(t, fontSize.Definition.Members, 2)
	assert.Equal(
		t,
		"decimal",
		fontSize.Definition.Members[0].Declaration.Name.Local,
	)
	assert.Equal(
		t,
		"css-font-size",
		fontSize.Definition.Members[1].Declaration.Name.Local,
	)

	numberOrNormal := requireSimpleTypePlan(
		t,
		plans,
		"number-or-normal",
	)
	require.Len(t, numberOrNormal.Definition.Members, 2)
	require.NotNil(t, numberOrNormal.Definition.Members[1].Inline)
	assert.Equal(
		t,
		"NumberOrNormalNormal",
		numberOrNormal.Definition.Members[1].
			Inline.Enumerations[0].GoName,
	)

	tokenList := requireSimpleTypePlan(t, plans, "token-list")
	assert.Equal(t, SimpleTypeList, tokenList.Definition.Form)
	require.NotNil(t, tokenList.Definition.Item)
	assert.Equal(
		t,
		"token",
		tokenList.Definition.Item.Declaration.Name.Local,
	)
}

func TestPlanSimpleTypesError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		wantErr error
	}{
		{
			name: "missing form",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="value"/>
</xs:schema>`,
			wantErr: ErrInvalidSimpleType,
		},
		{
			name: "multiple forms",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="value">
		<xs:restriction base="xs:string"/>
		<xs:union memberTypes="xs:string"/>
	</xs:simpleType>
</xs:schema>`,
			wantErr: ErrInvalidSimpleType,
		},
		{
			name: "missing restriction base",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="value">
		<xs:restriction/>
	</xs:simpleType>
</xs:schema>`,
			wantErr: ErrInvalidSimpleType,
		},
		{
			name: "empty union",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="value">
		<xs:union/>
	</xs:simpleType>
</xs:schema>`,
			wantErr: ErrInvalidSimpleType,
		},
		{
			name: "complex restriction base",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="complex"/>
	<xs:simpleType name="value">
		<xs:restriction base="complex"/>
	</xs:simpleType>
</xs:schema>`,
			wantErr: ErrInvalidSimpleType,
		},
		{
			name: "enumeration name collision",
			schema: `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="value">
		<xs:restriction base="xs:string">
			<xs:enumeration value="foo-bar"/>
			<xs:enumeration value="foo bar"/>
		</xs:restriction>
	</xs:simpleType>
</xs:schema>`,
			wantErr: ErrGoNameCollision,
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

			plans, err := PlanSimpleTypes(index)

			assert.Nil(t, plans)
			assert.ErrorIs(t, err, test.wantErr)
		})
	}
}

func TestPlanSimpleTypesNilIndex(t *testing.T) {
	t.Parallel()

	plans, err := PlanSimpleTypes(nil)

	assert.Nil(t, plans)
	assert.ErrorIs(t, err, ErrNilIndex)
}

func requireSimpleTypePlan(
	t *testing.T,
	plans []SimpleTypePlan,
	name string,
) SimpleTypePlan {
	t.Helper()

	for _, plan := range plans {
		if plan.Declaration.Name.Local == name {
			return plan
		}
	}

	require.Failf(t, "missing simple type plan", "name: %s", name)

	return SimpleTypePlan{}
}
