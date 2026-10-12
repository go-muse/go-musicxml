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
			if path == "github.com/go-muse/go-musicxml/internal/xsd" {
				continue
			}
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
		if line != "github.com/go-muse/go-musicxml/validation" && line != "github.com/go-muse/go-musicxml/internal/xsd" {
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

func TestFoundationFuzzTargetsAreExecutedByCheckAll(t *testing.T) {
	data, err := os.ReadFile("../scripts/check-all.sh")
	if err != nil {
		t.Fatal(err)
	}
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(i os.FileInfo) bool { return strings.HasSuffix(i.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, file := range packages["validation"].Files {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && strings.HasPrefix(fn.Name.Name, "Fuzz") {
				count++
				if !strings.Contains(string(data), "-fuzz='^"+fn.Name.Name+"$'") {
					t.Errorf("fuzz target %s is not invoked by check-all", fn.Name.Name)
				}
			}
		}
	}
	if count < 2 {
		t.Fatalf("foundation fuzz targets=%d, want at least 2", count)
	}
}

func TestRaceStageRetainsFullCoverageWithFiniteHeadroom(t *testing.T) {
	data, err := os.ReadFile("../scripts/check-all.sh")
	if err != nil {
		t.Fatal(err)
	}
	_, stage, ok := strings.Cut(string(data), "stage_race() {")
	if !ok {
		t.Fatal("missing shared race stage")
	}
	stage, _, ok = strings.Cut(stage, "\n}")
	if !ok {
		t.Fatal("unterminated shared race stage")
	}
	// The full bounded suite approaches Go's implicit ten-minute package limit.
	// Retain all race checks and one uncached execution, with an explicit finite
	// deadline below the unchanged twenty-minute CI job cap.
	if !strings.Contains(stage, `"$check_go" test -race -count=1 -timeout=15m ./...`) {
		t.Fatal("race stage must retain full coverage and a finite fifteen-minute package deadline")
	}
	workflow, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	lf := strings.ReplaceAll(string(workflow), "\r\n", "\n")
	for _, checkout := range []string{lf, strings.ReplaceAll(lf, "\n", "\r\n")} {
		_, job, ok := strings.Cut(strings.ReplaceAll(checkout, "\r\n", "\n"), "\n  race:\n")
		if !ok {
			t.Fatal("missing CI race job")
		}
		job, _, ok = strings.Cut(job, "\n  xsd-conformance:")
		if !ok || !strings.Contains(job, "timeout-minutes: 20") {
			t.Fatal("race CI job must retain its finite twenty-minute cap")
		}
	}
}
