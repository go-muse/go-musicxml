package xsdgen

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateMusicXMLOpusDocument(t *testing.T) {
	t.Parallel()

	schemaFS := os.DirFS(filepath.Join(
		"..",
		"..",
		"schema",
		"musicxml-4.0",
	))

	set, err := Load(schemaFS, "opus.xsd", "catalog.xml")
	require.NoError(t, err)

	index, err := NewIndex(set)
	require.NoError(t, err)

	actual, err := GenerateDocument(
		index,
		"musicxml",
		DocumentGenerationOptions{
			Element: QName{Local: "opus"},
			GoName:  "OpusDocument",
			TypeNames: []TypeNameOverride{{
				Name:   QName{Local: "score"},
				GoName: "OpusScore",
			}},
			ExternalTypes: []QName{{Local: "yes-no"}},
		},
	)
	require.NoError(t, err)

	generatedPath := filepath.Join(
		"..",
		"..",
		"zz_generated_opus.go",
	)
	committed, err := os.ReadFile(generatedPath)
	require.NoError(t, err)
	assert.Equal(t, string(committed), string(actual))

	file, err := parser.ParseFile(
		token.NewFileSet(),
		generatedPath,
		actual,
		parser.SkipObjectResolution,
	)
	require.NoError(t, err)
	assert.Equal(t, 5, countTypeDeclarations(file))

	source := string(actual)
	assert.Contains(t, source, "type OpusDocument struct {")
	assert.Contains(t, source, "type OpusLink struct {")
	assert.Contains(t, source, "type OpusScore struct {")
	assert.Contains(t, source, "type OpusDocumentContents []OpusDocumentContent")
	assert.Contains(t, source, "type OpusDocumentContent struct {")
	assert.NotContains(t, source, "type YesNo ")
}

func TestMusicXMLOpusExternalYesNoMatchesScoreSchema(t *testing.T) {
	t.Parallel()

	schemaFS := os.DirFS(filepath.Join(
		"..",
		"..",
		"schema",
		"musicxml-4.0",
	))

	opusSet, err := Load(schemaFS, "opus.xsd", "catalog.xml")
	require.NoError(t, err)
	opusIndex, err := NewIndex(opusSet)
	require.NoError(t, err)

	scoreSet, err := Load(schemaFS, "musicxml.xsd", "catalog.xml")
	require.NoError(t, err)
	scoreIndex, err := NewIndex(scoreSet)
	require.NoError(t, err)

	opusYesNo, found := opusIndex.LookupType(QName{Local: "yes-no"})
	require.True(t, found)
	scoreYesNo, found := scoreIndex.LookupType(QName{Local: "yes-no"})
	require.True(t, found)

	assert.Equal(
		t,
		simpleTypeEnumerationValues(scoreYesNo),
		simpleTypeEnumerationValues(opusYesNo),
	)
}

func simpleTypeEnumerationValues(
	declaration *Declaration,
) []string {
	if declaration == nil ||
		declaration.SimpleType == nil ||
		declaration.SimpleType.Restriction == nil {
		return nil
	}

	facets := declaration.SimpleType.Restriction.Enumerations
	result := make([]string, len(facets))
	for index := range facets {
		result[index] = facets[index].Value
	}

	return result
}
