package musicxml

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var updateAPISnapshot = flag.Bool(
	"update-api",
	false,
	"rewrite the public API golden files in testdata/api",
)

// TestPublicAPISnapshot pins the exported API of the public packages. Each
// line records one declaration, struct field (with its tag), interface method
// or method-set entry, so an accidental removal, rename, signature change,
// XML tag reassignment, comparability change or marshal method change shows up
// as a golden-file diff. Intentional changes regenerate the files with
// `go test -run TestPublicAPISnapshot -update-api .`.
func TestPublicAPISnapshot(t *testing.T) {
	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "source", nil)

	for _, test := range []struct {
		path   string
		golden string
	}{
		{"github.com/go-muse/go-musicxml", "musicxml.txt"},
		{"github.com/go-muse/go-musicxml/validation", "validation.txt"},
		{"github.com/go-muse/go-musicxml/validation/inventory", "inventory.txt"},
	} {
		t.Run(test.golden, func(t *testing.T) {
			pkg, err := imp.Import(test.path)
			if err != nil {
				t.Fatalf("type-check %s: %v", test.path, err)
			}
			actual := []byte(strings.Join(apiSnapshot(pkg), "\n") + "\n")
			golden := filepath.Join("testdata", "api", test.golden)

			if *updateAPISnapshot {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, actual, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}

			expected, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read %s: %v (run with -update-api to create it)", golden, err)
			}
			expected = bytes.ReplaceAll(expected, []byte("\r\n"), []byte("\n"))
			if bytes.Equal(expected, actual) {
				return
			}
			removed, added := apiSnapshotDiff(expected, actual)
			t.Errorf(
				"public API of %s differs from %s\n"+
					"removed or changed (%d):\n%s\nadded (%d):\n%s\n"+
					"If the change is intentional, run "+
					"go test -run TestPublicAPISnapshot -update-api . "+
					"and explain its compatibility in the pull request.",
				test.path,
				golden,
				len(removed),
				apiSnapshotSample(removed),
				len(added),
				apiSnapshotSample(added),
			)
		})
	}
}

func apiSnapshot(pkg *types.Package) []string {
	qualifier := func(other *types.Package) string {
		if other == pkg {
			return ""
		}
		return other.Name()
	}
	typeString := func(value types.Type) string {
		return types.TypeString(value, qualifier)
	}

	var lines []string
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		object := scope.Lookup(name)
		if !object.Exported() {
			continue
		}

		switch object := object.(type) {
		case *types.Const:
			lines = append(lines, fmt.Sprintf(
				"const %s %s = %s",
				name,
				typeString(object.Type()),
				object.Val().ExactString(),
			))

		case *types.Var:
			lines = append(lines, fmt.Sprintf(
				"var %s %s",
				name,
				typeString(object.Type()),
			))

		case *types.Func:
			signature := object.Type().(*types.Signature)
			params := apiTypeParams(signature.TypeParams(), typeString)
			// A generic signature already prints its type parameters.
			lines = append(lines, fmt.Sprintf(
				"func %s%s%s",
				name,
				params,
				strings.TrimPrefix(strings.TrimPrefix(typeString(signature), "func"), params),
			))

		case *types.TypeName:
			lines = append(lines, apiTypeLines(object, typeString)...)
		}
	}

	sort.Strings(lines)
	return lines
}

func apiTypeLines(
	object *types.TypeName,
	typeString func(types.Type) string,
) []string {
	name := object.Name()
	if object.IsAlias() {
		target := types.Unalias(object.Type())
		rhs := target
		if alias, ok := object.Type().(*types.Alias); ok {
			rhs = alias.Rhs()
		}
		lines := []string{fmt.Sprintf("type %s = %s", name, typeString(rhs))}
		// A named type declared in this package has its own entries. Any other
		// target, including another package's type or an anonymous struct
		// reached through an alias chain, is part of this package's API, so
		// describe its structure and method sets under the alias name.
		if named, ok := target.(*types.Named); ok &&
			named.Obj().Pkg() == object.Pkg() {
			return lines
		}
		return append(lines, apiTypeBody(name, "type "+name, target, typeString)...)
	}

	named := object.Type().(*types.Named)
	header := fmt.Sprintf(
		"type %s%s",
		name,
		apiTypeParams(named.TypeParams(), typeString),
	)
	return apiTypeBody(name, header, named, typeString)
}

// apiTypeBody describes the public structure of value under name: its kind,
// struct fields in declaration order with tags and comparability, interface
// methods, or underlying type, followed by its value and pointer method sets.
func apiTypeBody(
	name string,
	header string,
	value types.Type,
	typeString func(types.Type) string,
) []string {
	var lines []string
	switch underlying := value.Underlying().(type) {
	case *types.Struct:
		comparability := "not comparable"
		if types.Comparable(value) {
			comparability = "comparable"
		}
		lines = append(lines, fmt.Sprintf(
			"%s struct, %s",
			header,
			comparability,
		))
		lines = append(lines, apiStructLines(name, underlying, typeString)...)

	case *types.Interface:
		lines = append(lines, header+" interface")
		for index := range underlying.NumMethods() {
			method := underlying.Method(index)
			if !method.Exported() {
				lines = append(lines, fmt.Sprintf(
					"type %s interface, unexported method %s",
					name,
					method.Name(),
				))
				continue
			}
			lines = append(lines, fmt.Sprintf(
				"type %s interface, method %s%s",
				name,
				method.Name(),
				strings.TrimPrefix(typeString(method.Type()), "func"),
			))
		}
		return lines

	default:
		lines = append(lines, fmt.Sprintf(
			"%s %s",
			header,
			typeString(underlying),
		))
	}

	// Method sets include methods promoted through embedded fields, which
	// change encoding/xml behavior without any declaration in this type.
	valueMethods := types.NewMethodSet(value)
	pointerMethods := types.NewMethodSet(types.NewPointer(value))
	for index := range pointerMethods.Len() {
		selection := pointerMethods.At(index)
		method := selection.Obj()
		if !method.Exported() {
			continue
		}
		receiver := "*" + name
		if valueMethods.Lookup(method.Pkg(), method.Name()) != nil {
			receiver = name
		}
		promoted := ""
		if len(selection.Index()) > 1 {
			promoted = " (promoted)"
		}
		lines = append(lines, fmt.Sprintf(
			"type %s, method (%s) %s%s%s",
			name,
			receiver,
			method.Name(),
			strings.TrimPrefix(typeString(method.Type()), "func"),
			promoted,
		))
	}

	return lines
}

// apiStructLines records each exported field and, because encoding/xml
// writes elements in field order, one line with the exported field order.
func apiStructLines(
	name string,
	structure *types.Struct,
	typeString func(types.Type) string,
) []string {
	var lines, order []string
	for index := range structure.NumFields() {
		field := structure.Field(index)
		if !field.Exported() {
			continue
		}
		order = append(order, field.Name())
		kind := "field"
		if field.Embedded() {
			kind = "embedded"
		}
		line := fmt.Sprintf(
			"type %s struct, %s %s %s",
			name,
			kind,
			field.Name(),
			typeString(field.Type()),
		)
		if tag := structure.Tag(index); tag != "" {
			line += " `" + tag + "`"
		}
		lines = append(lines, line)
	}
	if len(order) > 1 {
		lines = append(lines, fmt.Sprintf(
			"type %s struct, field order %s",
			name,
			strings.Join(order, " "),
		))
	}

	return lines
}

func apiTypeParams(
	params *types.TypeParamList,
	typeString func(types.Type) string,
) string {
	if params.Len() == 0 {
		return ""
	}

	values := make([]string, params.Len())
	for index := range params.Len() {
		param := params.At(index)
		values[index] = param.Obj().Name() + " " +
			typeString(param.Constraint())
	}

	return "[" + strings.Join(values, ", ") + "]"
}

func apiSnapshotDiff(expected, actual []byte) (removed, added []string) {
	set := func(data []byte) map[string]bool {
		result := make(map[string]bool)
		for _, line := range strings.Split(string(data), "\n") {
			if line != "" {
				result[line] = true
			}
		}
		return result
	}
	expectedLines, actualLines := set(expected), set(actual)
	for line := range expectedLines {
		if !actualLines[line] {
			removed = append(removed, line)
		}
	}
	for line := range actualLines {
		if !expectedLines[line] {
			added = append(added, line)
		}
	}
	sort.Strings(removed)
	sort.Strings(added)

	return removed, added
}

func apiSnapshotSample(lines []string) string {
	const limit = 40
	if len(lines) > limit {
		return "  " + strings.Join(lines[:limit], "\n  ") +
			fmt.Sprintf("\n  ... and %d more", len(lines)-limit)
	}
	if len(lines) == 0 {
		return "  (none)"
	}
	return "  " + strings.Join(lines, "\n  ")
}

type apiFixtureImporter map[string]*types.Package

func (i apiFixtureImporter) Import(path string) (*types.Package, error) {
	if pkg := i[path]; pkg != nil {
		return pkg, nil
	}
	return nil, fmt.Errorf("unknown fixture import %q", path)
}

// apiFixtureSnapshot type-checks a two-package fixture in which public
// re-exports declarations of kernel, mirroring validation and internal/xsd.
func apiFixtureSnapshot(t *testing.T, kernelSource, publicSource string) []string {
	t.Helper()

	fset := token.NewFileSet()
	check := func(path, source string, imports apiFixtureImporter) *types.Package {
		file, err := parser.ParseFile(fset, path+".go", source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		config := types.Config{Importer: imports}
		pkg, err := config.Check(path, fset, []*ast.File{file}, nil)
		if err != nil {
			t.Fatalf("type-check %s: %v", path, err)
		}
		return pkg
	}

	kernel := check("example.com/m/internal/kernel", kernelSource, nil)
	public := check(
		"example.com/m/public",
		publicSource,
		apiFixtureImporter{kernel.Path(): kernel},
	)
	return apiSnapshot(public)
}

func TestPublicAPISnapshotDetectsReexportedChanges(t *testing.T) {
	t.Parallel()

	const reexport = `package public
import "example.com/m/internal/kernel"
type Name = kernel.QName
type Derivation = kernel.Derivation
type Schema = kernel.Schema
`
	const kernel = `package kernel
type QName = struct{ Space, Local string }
type Derivation uint8
type Schema struct {
	Name QName
	note int
}
`
	baseline := apiFixtureSnapshot(t, kernel, reexport)
	assert.Contains(t, baseline, "type Name struct, field Local string")
	assert.Contains(t, baseline, "type Derivation uint8")
	assert.Contains(t, baseline, "type Schema struct, comparable")

	tests := map[string]string{
		"anonymous struct field renamed": strings.Replace(
			kernel,
			"Space, Local string",
			"Space, Name string",
			1,
		),
		"named target retyped with method": strings.Replace(
			kernel,
			"type Derivation uint8",
			"type Derivation uint16\nfunc (Derivation) Has() bool { return false }",
			1,
		),
		"struct comparability changed": strings.Replace(
			kernel,
			"note int",
			"note []int",
			1,
		),
	}
	for name, mutated := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, baseline, apiFixtureSnapshot(t, mutated, reexport))
		})
	}
}

func TestPublicAPISnapshotDetectsFieldReorder(t *testing.T) {
	t.Parallel()

	const kernel = `package kernel`
	const ordered = `package public
type Work struct {
	WorkNumber *string ` + "`xml:\"work-number,omitempty\"`" + `
	WorkTitle  *string ` + "`xml:\"work-title,omitempty\"`" + `
}
`
	swapped := `package public
type Work struct {
	WorkTitle  *string ` + "`xml:\"work-title,omitempty\"`" + `
	WorkNumber *string ` + "`xml:\"work-number,omitempty\"`" + `
}
`
	baseline := apiFixtureSnapshot(t, kernel, ordered)
	assert.Equal(t, baseline, apiFixtureSnapshot(t, kernel, ordered))
	assert.Contains(t, baseline, "type Work struct, field order WorkNumber WorkTitle")
	assert.NotEqual(t, baseline, apiFixtureSnapshot(t, kernel, swapped))
}
