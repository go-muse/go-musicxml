package musicxml

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// FuzzAssessCurrentModel exercises caller-created representation and value
// states, including states that no successful XML decoder could produce. It
// never serializes the graph and uses explicit finite per-run limits.
func FuzzAssessCurrentModel(f *testing.F) {
	for _, seed := range []struct {
		text  string
		bits  uint64
		shape uint8
	}{
		{"Piano", math.Float64bits(1), 0}, {"a\x00b", math.Float64bits(math.NaN()), 1}, {"\xff", math.Float64bits(math.Inf(1)), 2}, {"é", math.Float64bits(0.1), 3}, {"", math.Float64bits(-1), 4}, {"C", 1, 5},
	} {
		f.Add(seed.text, seed.bits, seed.shape)
	}
	f.Fuzz(func(t *testing.T, text string, bits uint64, shape uint8) {
		if len(text) > 1024 {
			t.Skip()
		}
		doc := modelMinimalScore()
		doc.MovementTitle = Ptr(text)
		note := doc.Part[0].Measure[0].Content[0].Note
		note.Duration = Ptr(PositiveDivisions(math.Float64frombits(bits)))
		var input Document = doc
		switch shape % 6 {
		case 0:
			note.Pitch = &Pitch{Step: Step(text), Octave: 4}
		case 1:
			doc.Part[0].Measure[0].Content = append(doc.Part[0].Measure[0].Content, ScorePartwisePartMeasureContent{})
		case 2:
			doc.Part[0].Measure[0].Content = append(doc.Part[0].Measure[0].Content, ScorePartwisePartMeasureContent{Note: note, Sound: &Sound{}})
		case 3:
			root := NewOpusDocument()
			root.AddOpus(root)
			root.Title = Ptr(text)
			input = root
		case 4:
			shared := NewOpusDocument()
			shared.Title = Ptr(text)
			root := NewOpusDocument()
			root.AddOpus(shared)
			root.AddOpus(shared)
			input = root
		case 5:
			note.Rest = nil
			note.Pitch = &Pitch{Step: Step(text), Octave: Octave(shape)}
		}
		options := ModelOptions{Profile: ProfileModelLocal, Budgets: validation.Budgets{MaxBufferedBytes: 2 << 20, MaxDepth: 32, MaxNodes: 256, MaxAttributes: 512, MaxEvents: 4096, MaxRuleSteps: 50000, MaxScalarBytes: 2048, MaxDiagnostics: 16, MaxDiagnosticBytes: 8192}}
		a, errA := AssessModel(context.Background(), input, options)
		b, errB := AssessModel(context.Background(), input, options)
		if !reflect.DeepEqual(a, b) || ((errA == nil) != (errB == nil)) {
			t.Fatalf("nondeterministic current assessment: %v / %v", errA, errB)
		}
		if errors.Is(errA, ErrInvalidDocument) {
			t.Fatalf("ordinary finding became legacy operation error: %v", errA)
		}
		if !a.Requested || a.Target != validation.TargetModel || a.Syntax != validation.SyntaxNotAssessed {
			t.Fatalf("lost current target axes: %+v", a)
		}
		if *doc.MovementTitle != text || math.Float64bits(float64(*note.Duration)) != bits {
			t.Fatal("assessment changed current fields")
		}
	})
}
