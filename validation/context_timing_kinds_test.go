package validation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestTimingKindConstantsAreTheFullSet pins the exported TimingContextResult.Kind
// values and checks the package's own code: every exported kind is used, and
// no Kind is assigned, compared or started from a string literal.
func TestTimingKindConstantsAreTheFullSet(t *testing.T) {
	want := map[string]string{
		"TimingKindEvent": "event", "TimingKindNote": "note", "TimingKindFiguredBass": "figured-bass",
		"TimingKindDirection": "direction", "TimingKindSound": "sound", "TimingKindForward": "forward",
		"TimingKindBackup": "backup", "TimingKindFret": "fret", "TimingKindEditorialLevel": "editorial-level",
		"TimingKindRootAlter": "root-alter", "TimingKindTrill": "trill", "TimingKindOrnamentAccidental": "ornament-accidental",
		"TimingKindMeasureLabel": "measure-label", "TimingKindPartOrder": "part-order", "TimingKindKey": "key",
		"TimingKindTime": "time", "TimingKindTranspose": "transpose", "TimingKindStaffDetails": "staff-details",
		"TimingKindMIDIUnpitched": "midi-unpitched", "TimingKindPan": "pan", "TimingKindElevation": "elevation",
		"TimingKindSwing": "swing", "TimingKindBend": "bend",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	declared := map[string]string{}
	used := map[string]int{}
	var literals []string
	// An empty literal is never a timing kind; other Kind fields compare to it.
	isString := func(e ast.Expr) bool {
		lit, ok := e.(*ast.BasicLit)
		return ok && lit.Kind == token.STRING && lit.Value != `""`
	}
	isKind := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "Kind"
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ValueSpec:
				for i, id := range n.Names {
					if strings.HasPrefix(id.Name, "TimingKind") && i < len(n.Values) {
						value, err := strconv.Unquote(n.Values[i].(*ast.BasicLit).Value)
						if err != nil {
							t.Fatal(err)
						}
						declared[id.Name] = value
					}
				}
				for _, v := range n.Values {
					ast.Inspect(v, func(m ast.Node) bool {
						if id, ok := m.(*ast.Ident); ok {
							used[id.Name]++
						}
						return true
					})
				}
				return false
			case *ast.Ident:
				used[n.Name]++
			case *ast.AssignStmt:
				for i, lhs := range n.Lhs {
					if i < len(n.Rhs) && isKind(lhs) && isString(n.Rhs[i]) {
						literals = append(literals, fset.Position(n.Pos()).String())
					}
				}
			case *ast.BinaryExpr:
				if (n.Op == token.EQL || n.Op == token.NEQ) && (isKind(n.X) && isString(n.Y) || isKind(n.Y) && isString(n.X)) {
					literals = append(literals, fset.Position(n.Pos()).String())
				}
			case *ast.CallExpr:
				if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "newTimingContext" {
					for _, arg := range n.Args {
						if isString(arg) {
							literals = append(literals, fset.Position(n.Pos()).String())
						}
					}
				}
			}
			return true
		})
	}
	if !reflect.DeepEqual(declared, want) {
		t.Fatalf("exported timing kinds changed:\n got %v\nwant %v", declared, want)
	}
	for name := range want {
		if used[name] == 0 {
			t.Errorf("%s is never produced", name)
		}
	}
	if len(literals) != 0 {
		t.Errorf("timing kinds must use the constants: %v", literals)
	}
}
