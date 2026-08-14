package xsdgen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateMusicXMLSimpleTypes(t *testing.T) {
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

	actual, err := GenerateSimpleTypes(index, "musicxml")
	require.NoError(t, err)

	generatedPath := filepath.Join(
		"..",
		"..",
		"zz_generated_simple.go",
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
	assert.Equal(t, 145, countTypeDeclarations(file))
}

func countTypeDeclarations(file *ast.File) int {
	result := 0

	ast.Inspect(file, func(node ast.Node) bool {
		if _, ok := node.(*ast.TypeSpec); ok {
			result++
		}

		return true
	})

	return result
}
