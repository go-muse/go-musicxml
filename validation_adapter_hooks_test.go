package musicxml

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"
)

// Every hook that package validation registers is looked up here, and every
// lookup has a registration, so no adapter entry point is lost or left behind.
func TestValidationAdapterHooksMatchRegistrations(t *testing.T) {
	registered := hookNames(t, "validation/adapter_hooks.go", "Register")
	looked := hookNames(t, "validation_adapter_hooks.go", "Hook")
	if len(registered) == 0 || !slices.Equal(registered, looked) {
		t.Fatalf("registered %v\nlooked up %v", registered, looked)
	}
}

func hookNames(t *testing.T, path, function string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun := call.Fun
		if index, ok := fun.(*ast.IndexExpr); ok {
			fun = index.X
		}
		if sel, ok := fun.(*ast.SelectorExpr); ok && sel.Sel.Name == function && len(call.Args) != 0 {
			if literal, ok := call.Args[0].(*ast.BasicLit); ok {
				name, _ := strconv.Unquote(literal.Value)
				names = append(names, name)
			}
		}
		return true
	})
	slices.Sort(names)
	return names
}
