package musicxml

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"testing"
)

func TestValidateAssessesExactCurrentValueWithoutEncoding(t *testing.T) {
	doc := NewScorePartwise(PartDefinition{ID: "P1", Name: "Piano"})
	measure := NewScorePartwiseMeasure("1")
	note := NewRestNote(1e100)
	measure.AddNote(note)
	doc.Part[0].AddMeasure(measure)
	if err := Validate(doc); err != nil {
		t.Fatalf("finite current decimal value must not be converted to invalid exponent XML: %v", err)
	}
	// An actual current representation problem still matches the legacy sentinel.
	*note.Duration = PositiveDivisions(math.Inf(1))
	if err := Validate(doc); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("nonfinite current value: %v", err)
	}
}

func TestValidateDirectCallGraphNoTransport(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "validation.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Validate" || fn.Recv != nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			switch name.Name {
			case "Encode", "parseValidationDocument", "checkDocumentNesting":
				t.Errorf("Validate still calls transport or an unmetered prewalk: %s", name.Name)
			}
			return true
		})
	}
}
