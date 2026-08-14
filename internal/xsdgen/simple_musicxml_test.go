package xsdgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanMusicXMLSimpleTypes(t *testing.T) {
	t.Parallel()

	schemaFS := os.DirFS(filepath.Join(
		"..",
		"..",
		"schema",
		"musicxml-4.0",
	))

	set, err := Load(schemaFS, "musicxml.xsd", "catalog.xml")
	require.NoError(t, err)

	index, err := NewIndex(set)
	require.NoError(t, err)

	plans, err := PlanSimpleTypes(index)
	require.NoError(t, err)
	require.Len(t, plans, 145)

	forms := map[SimpleTypeForm]int{}
	for _, plan := range plans {
		forms[plan.Definition.Form]++
	}

	assert.Equal(t, 141, forms[SimpleTypeRestriction])
	assert.Equal(t, 4, forms[SimpleTypeUnion])
	assert.Zero(t, forms[SimpleTypeList])

	assert.Equal(
		t,
		[]string{
			"FontSize",
			"NumberOrNormal",
			"PositiveIntegerOrEmpty",
			"YesNoNumber",
		},
		simpleTypeNamesByForm(plans, SimpleTypeUnion),
	)

	aboveBelow := requireSimpleTypePlan(t, plans, "above-below")
	assert.Equal(
		t,
		[]string{
			"AboveBelowAbove",
			"AboveBelowBelow",
		},
		enumerationGoNames(aboveBelow.Definition),
	)

	noteType := requireSimpleTypePlan(t, plans, "note-type-value")
	require.NotEmpty(t, noteType.Definition.Enumerations)
	assert.Equal(
		t,
		"NoteTypeValue1024th",
		noteType.Definition.Enumerations[0].GoName,
	)
}

func simpleTypeNamesByForm(
	plans []SimpleTypePlan,
	form SimpleTypeForm,
) []string {
	result := make([]string, 0)

	for _, plan := range plans {
		if plan.Definition.Form == form {
			result = append(result, plan.GoName)
		}
	}

	return result
}

func enumerationGoNames(
	definition *SimpleTypeDefinition,
) []string {
	result := make([]string, len(definition.Enumerations))

	for index, enumeration := range definition.Enumerations {
		result[index] = enumeration.GoName
	}

	return result
}
