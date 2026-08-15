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

func TestGenerateMusicXMLElements(t *testing.T) {
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

	actual, err := GenerateElements(
		index,
		"musicxml",
		QName{Local: "score-partwise"},
		QName{Local: "score-timewise"},
	)
	require.NoError(t, err)

	generatedPath := filepath.Join(
		"..",
		"..",
		"zz_generated_documents.go",
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
	assert.Equal(t, 10, countTypeDeclarations(file))

	source := string(actual)
	assert.Contains(t, source, "type ScorePartwise struct {")
	assert.Contains(t, source, "type ScoreTimewise struct {")
	assert.Contains(t, source, "type ScorePartwisePart struct {")
	assert.Contains(
		t,
		source,
		"type ScorePartwisePartMeasure struct {",
	)
	assert.Contains(
		t,
		source,
		"ScorePartwisePartMeasureContents `xml:\",any\"`",
	)
	assert.Contains(
		t,
		source,
		"ScoreTimewiseMeasurePartContents `xml:\",any\"`",
	)
}
