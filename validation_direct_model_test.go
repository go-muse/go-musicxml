package musicxml

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestValidateDependentRepresentationIncompleteness(t *testing.T) {
	doc := modelMinimalScore()
	doc.Part[0].Measure[0].Width = Ptr(Tenths(math.NaN()))
	report, err := AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelXSD})
	if err != nil || !report.InputComplete || report.AssessmentComplete || report.Counts.Blocked == 0 || report.Terminal != nil {
		t.Fatalf("representation report: complete=%t counts=%+v terminal=%+v err=%v", report.AssessmentComplete, report.Counts, report.Terminal, err)
	}
	failed := make(map[validation.DecisionRef]bool)
	for _, finding := range report.Findings {
		if finding.Category == validation.CategoryModel && finding.Outcome == validation.OutcomeFail {
			failed[validation.DecisionRef{Rule: finding.Rule, Subject: finding.Subject}] = true
		}
	}
	causal := false
	for _, finding := range report.Findings {
		if finding.Outcome == validation.OutcomeBlocked && finding.Cause != nil && failed[*finding.Cause] {
			causal = true
		}
	}
	if !causal {
		t.Fatal("missing same-report model failure causing blocked XSD assessment")
	}
	// The facade retains both truthful axes: definitely invalid representation,
	// and dependent XSD work that could not assess a non-existent decimal value.
	err = Validate(doc)
	var invalid *ValidationError
	if !errors.As(err, &invalid) || !errors.Is(err, ErrInvalidDocument) || !errors.Is(err, ErrIncompleteAssessment) {
		t.Fatalf("joined representation/incompleteness contract: %v", err)
	}
	// Diagnostic omission does not turn unresolved schema work into complete work.
	truncated, err := AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelXSD, Budgets: validation.Budgets{MaxDiagnosticBytes: 1}})
	if err != nil || !truncated.DiagnosticsTruncated || len(truncated.Findings) != 0 || truncated.Counts != report.Counts || truncated.AssessmentComplete {
		t.Fatalf("diagnostic-independent report contract: %+v err=%v", truncated, err)
	}
	// A complete, ordinary schema violation does not acquire incompleteness.
	doc = modelMinimalScore()
	doc.Part[0].Measure[0].Content[0].Note.Duration = Ptr(PositiveDivisions(-1))
	err = Validate(doc)
	if !errors.Is(err, ErrInvalidDocument) || errors.Is(err, ErrIncompleteAssessment) {
		t.Fatalf("complete schema failure changed category: %v", err)
	}
}

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
