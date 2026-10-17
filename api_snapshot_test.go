package musicxml

import (
	"bytes"
	"flag"
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
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
			lines = append(lines, fmt.Sprintf(
				"func %s%s%s",
				name,
				apiTypeParams(signature.TypeParams(), typeString),
				strings.TrimPrefix(typeString(signature), "func"),
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
		// An alias of another package's struct exposes that struct's fields.
		if named, ok := target.(*types.Named); ok &&
			named.Obj().Pkg() != object.Pkg() {
			lines = append(lines, apiStructLines(name, named, typeString)...)
		}
		return lines
	}

	named := object.Type().(*types.Named)
	header := fmt.Sprintf(
		"type %s%s",
		name,
		apiTypeParams(named.TypeParams(), typeString),
	)

	var lines []string
	switch underlying := named.Underlying().(type) {
	case *types.Struct:
		comparability := "not comparable"
		if types.Comparable(named) {
			comparability = "comparable"
		}
		lines = append(lines, fmt.Sprintf(
			"%s struct, %s",
			header,
			comparability,
		))
		lines = append(lines, apiStructLines(name, named, typeString)...)

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
	valueMethods := types.NewMethodSet(named)
	pointerMethods := types.NewMethodSet(types.NewPointer(named))
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

func apiStructLines(
	name string,
	named *types.Named,
	typeString func(types.Type) string,
) []string {
	structure, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil
	}

	var lines []string
	for index := range structure.NumFields() {
		field := structure.Field(index)
		if !field.Exported() {
			continue
		}
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
