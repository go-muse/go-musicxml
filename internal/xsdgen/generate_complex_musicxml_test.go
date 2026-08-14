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

func TestGenerateMusicXMLComplexTypes(t *testing.T) {
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

	actual, err := GenerateComplexTypesWithOptions(
		index,
		"musicxml",
		ComplexGenerationOptions{
			OrderedContentTypes: []QName{
				{Local: "metronome"},
			},
		},
	)
	require.NoError(t, err)

	generatedPath := filepath.Join(
		"..",
		"..",
		"zz_generated_types.go",
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
	assert.Equal(t, 237, countTypeDeclarations(file))

	source := string(actual)
	assert.Contains(t, source, "type Pitch struct {")
	assert.Contains(t, source, "type Note struct {")
	assert.Contains(t, source, "type MetronomeTuplet struct {")
	assert.Contains(t, source, "type MetronomeContent struct {")
	assert.Contains(t, source, "\tTimeModification\n")
	assert.Contains(
		t,
		source,
		"Lang          *string",
	)
	assert.Contains(
		t,
		source,
		"`xml:\"http://www.w3.org/XML/1998/namespace lang,attr,omitempty\"`",
	)
	assert.Contains(
		t,
		source,
		"CodaAttr  *string",
	)
	assert.Contains(
		t,
		source,
		"Content []ArticulationsContent `xml:\",any\"`",
	)
	assert.Contains(
		t,
		source,
		"KeyAlter      []Semitones     `xml:\"key-alter\"`",
	)
	assert.Contains(
		t,
		source,
		"type PartListContent struct {",
	)
	assert.Contains(
		t,
		source,
		"func (value PartListContent) MarshalXML(",
	)
}
