package musicxml

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNameManifestAvoidsHandwrittenNames checks the generator's name manifest
// against the package's hand-written top-level declarations, which the
// generator cannot see: no generated name may take one of them.
func TestNameManifestAvoidsHandwrittenNames(t *testing.T) {
	t.Parallel()

	file, err := os.Open("internal/xsdgen/names/musicxml-4.0.txt")
	require.NoError(t, err)
	defer file.Close()
	generated := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, name, found := strings.Cut(strings.TrimSuffix(scanner.Text(), "\r"), "\t")
		require.True(t, found, scanner.Text())
		generated[name] = key
	}
	require.NoError(t, scanner.Err())
	require.Greater(t, len(generated), 1000, "the manifest lists the generated model")

	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	fset := token.NewFileSet()
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "zz_generated_") {
			continue
		}
		parsed, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		require.NoError(t, err)
		for _, declaration := range parsed.Decls {
			var names []*ast.Ident
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Recv == nil {
					names = append(names, declaration.Name)
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, spec.Name)
					case *ast.ValueSpec:
						names = append(names, spec.Names...)
					}
				}
			}
			for _, id := range names {
				checked++
				if key, found := generated[id.Name]; found {
					t.Errorf("%s declares %s, which the manifest assigns to %s", fset.Position(id.Pos()), id.Name, key)
				}
			}
		}
	}
	require.Greater(t, checked, 100, "the package's hand-written declarations were read")
}
