package musicxml

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestPublicDeclarationsHaveDocumentation(t *testing.T) {
	files, err := parser.ParseDir(
		token.NewFileSet(),
		".",
		func(info os.FileInfo) bool {
			return !strings.HasSuffix(info.Name(), "_test.go")
		},
		parser.ParseComments,
	)
	if err != nil {
		t.Fatalf("parse package source: %v", err)
	}

	packageValue := files["musicxml"]
	if packageValue == nil {
		t.Fatal("musicxml package source not found")
	}

	var missing []string
	for filename, file := range packageValue.Files {
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				if !ast.IsExported(value.Name.Name) ||
					!publicFunctionOrMethod(value) ||
					hasDocumentation(value.Doc) {
					continue
				}

				missing = append(
					missing,
					filepath.Base(filename)+": "+value.Name.Name,
				)

			case *ast.GenDecl:
				for _, specification := range value.Specs {
					switch item := specification.(type) {
					case *ast.TypeSpec:
						if ast.IsExported(item.Name.Name) &&
							!hasDocumentation(item.Doc) &&
							!hasDocumentation(value.Doc) {
							missing = append(
								missing,
								filepath.Base(filename)+": "+
									item.Name.Name,
							)
						}

					case *ast.ValueSpec:
						if hasDocumentation(item.Doc) ||
							hasDocumentation(value.Doc) {
							continue
						}

						for _, name := range item.Names {
							if ast.IsExported(name.Name) {
								missing = append(
									missing,
									filepath.Base(filename)+": "+
										name.Name,
								)
							}
						}
					}
				}
			}
		}
	}

	if len(missing) == 0 {
		return
	}

	sort.Strings(missing)
	t.Fatalf(
		"exported declarations without documentation:\n%s",
		strings.Join(missing, "\n"),
	)
}

func publicFunctionOrMethod(value *ast.FuncDecl) bool {
	if value.Recv == nil {
		return true
	}
	if len(value.Recv.List) != 1 {
		return false
	}

	return exportedReceiver(value.Recv.List[0].Type)
}

func exportedReceiver(value ast.Expr) bool {
	switch typed := value.(type) {
	case *ast.Ident:
		return ast.IsExported(typed.Name)
	case *ast.StarExpr:
		return exportedReceiver(typed.X)
	case *ast.IndexExpr:
		return exportedReceiver(typed.X)
	case *ast.IndexListExpr:
		return exportedReceiver(typed.X)
	default:
		return false
	}
}

func hasDocumentation(value *ast.CommentGroup) bool {
	return value != nil && strings.TrimSpace(value.Text()) != ""
}
