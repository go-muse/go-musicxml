package validation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestFoundationImportBoundary(t *testing.T) {
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(i os.FileInfo) bool { return !strings.HasSuffix(i.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	for filename, file := range packages["validation"].Files {
		for _, imp := range file.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if path == "encoding/xml" || strings.HasPrefix(path, "net/") || path == "net" || strings.HasPrefix(path, "github.com/") {
				t.Errorf("foundation forbidden import %s in %s", path, filename)
			}
		}
	}
	command := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", ".")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v: %s", err, output)
	}
	for _, line := range strings.Fields(string(output)) {
		if line != "github.com/go-muse/go-musicxml/validation" {
			t.Errorf("nonstandard dependency %s", line)
		}
	}
}
func TestFoundationExportedDeclarationsDocumented(t *testing.T) {
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(i os.FileInfo) bool { return !strings.HasSuffix(i.Name(), "_test.go") }, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for filename, file := range packages["validation"].Files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if ast.IsExported(d.Name.Name) && d.Doc == nil {
					if d.Recv != nil {
						continue
					}
					t.Errorf("undocumented function %s:%s", filename, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch x := spec.(type) {
					case *ast.TypeSpec:
						if ast.IsExported(x.Name.Name) && d.Doc == nil && x.Doc == nil {
							t.Errorf("undocumented type %s:%s", filename, x.Name.Name)
						}
					case *ast.ValueSpec:
						if d.Doc == nil && x.Doc == nil {
							for _, n := range x.Names {
								if ast.IsExported(n.Name) {
									t.Errorf("undocumented value %s:%s", filename, n.Name)
								}
							}
						}
					}
				}
			}
		}
	}
}
