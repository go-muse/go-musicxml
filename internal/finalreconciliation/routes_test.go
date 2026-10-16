package finalreconciliation

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

func (r *ledgerReferences) validateCase(root, id string, c map[string]any) error {
	file, name := stringField(c, "file"), stringField(c, "function")
	data, err := r.readLocal(root, file)
	if err != nil {
		return err
	}
	source := normalize(data)
	fs := token.NewFileSet()
	parsed, err := parser.ParseFile(fs, file, source, 0)
	if err != nil {
		return err
	}
	var function *ast.FuncDecl
	for _, d := range parsed.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Name.Name == name && f.Recv == nil && strings.HasPrefix(name, "Test") {
			function = f
			break
		}
	}
	if function == nil || function.Body == nil {
		return fmt.Errorf("%s missing test function %s in %s", id, name, file)
	}
	if stringField(c, "input") == "" || stringField(c, "expected") == "" || stringField(c, "scope") == "" {
		return fmt.Errorf("%s lacks substantive input/outcome/scope", id)
	}
	anchors, ok := c["anchors"].([]any)
	if !ok || len(anchors) < 2 {
		return fmt.Errorf("%s needs fixture and assertion anchors", id)
	}
	referenced := map[string]bool{}
	ast.Inspect(function.Body, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			referenced[ident.Name] = true
		}
		return true
	})
	startOf := func(n ast.Node) int { return fs.Position(n.Pos()).Offset }
	endOf := func(n ast.Node) int { return fs.Position(n.End()).Offset }
	inFunction := false
	seen := map[string]bool{}
	for _, raw := range anchors {
		a, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("%s malformed anchor", id)
		}
		if a["file"] != file {
			return fmt.Errorf("%s anchor must name its test file %s", id, file)
		}
		text := strings.TrimSpace(stringField(a, "text"))
		if text == "" || strings.Count(source, text) != 1 {
			return fmt.Errorf("%s anchor must occur exactly once in %s: %q", id, file, text)
		}
		if seen[text] {
			return fmt.Errorf("%s duplicate fixture/assertion anchor", id)
		}
		seen[text] = true
		start := strings.Index(source, text)
		end := start + len(text)
		if start >= startOf(function) && end <= endOf(function) && end > startOf(function.Body)+1 && start < endOf(function.Body)-1 {
			inFunction = true
			continue
		}
		bound := false
		for _, d := range parsed.Decls {
			if start < startOf(d) || end > endOf(d) {
				continue
			}
			switch declaration := d.(type) {
			case *ast.FuncDecl:
				// A fixture-building helper must be directly referenced by this
				// test; mere presence elsewhere in the file is insufficient.
				bound = declaration.Recv == nil && !strings.HasPrefix(declaration.Name.Name, "Test") && referenced[declaration.Name.Name]
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					if start < startOf(spec) || end > endOf(spec) {
						continue
					}
					if value, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range value.Names {
							bound = bound || referenced[name.Name]
						}
					}
				}
			}
		}
		if !bound {
			return fmt.Errorf("%s anchor is outside its test and directly referenced fixtures: %q", id, text)
		}
	}
	if !inFunction {
		return fmt.Errorf("%s needs an anchor in the named test body", id)
	}
	return nil
}

// Matrix integrity is a property of the source graph itself, independent of
// whether the additive ledger faithfully copies an already broken graph.
func validateMatrixEdges(requirements, tests map[string]map[string]any) error {
	for id, r := range requirements {
		ids, err := stringList(r["test_contracts"])
		if err != nil || (r["status"] != "optional" && len(ids) == 0) {
			return fmt.Errorf("%s missing mandatory matrix edge to a test contract", id)
		}
		for _, test := range ids {
			if tests[test] == nil || !listContains(tests[test]["requirement_ids"], id) {
				return fmt.Errorf("%s has unknown or nonreciprocal matrix edge %s", id, test)
			}
		}
	}
	for id, r := range tests {
		ids, err := stringList(r["requirement_ids"])
		if err != nil || len(ids) == 0 {
			return fmt.Errorf("%s missing matrix edge to a requirement", id)
		}
		for _, requirement := range ids {
			if requirements[requirement] == nil || !listContains(requirements[requirement]["test_contracts"], id) {
				return fmt.Errorf("%s has unknown or nonreciprocal matrix edge %s", id, requirement)
			}
		}
	}
	return nil
}
func listContains(value any, id string) bool {
	list, err := stringList(value)
	if err != nil {
		return false
	}
	for _, item := range list {
		if item == id {
			return true
		}
	}
	return false
}
