package musicxml

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// FuzzContextAssessment checks the bounded public context profiles, including
// malformed source and current-model projections, without using one as a
// conformance oracle for the other representation.
func FuzzContextAssessment(f *testing.F) {
	for _, seed := range []string{
		`<opus/>`,
		`<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="never-fetch" new-page="yes"/><opus version="2.0"/></opus>`,
		`<score-partwise version="4.0"><part-list><score-part id="P"><part-name>P</part-name></score-part></part-list><part id="P"><measure number="1"><attributes><divisions>1</divisions></attributes><note><rest/><duration>1</duration><notations><slur type="start"/></notations></note><barline><repeat direction="backward" times="2"/></barline></measure></part></score-partwise>`,
		`<score-timewise><part-list/></score-timewise>`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 2048 {
			t.Skip()
		}
		budgets := validation.Budgets{MaxInputBytes: 4096, MaxBufferedBytes: 8 << 20, MaxDepth: 24, MaxNodes: 128, MaxAttributes: 256, MaxEvents: 2048, MaxRuleSteps: 2000000, MaxScalarBytes: 2048, MaxDiagnostics: 16, MaxDiagnosticBytes: 8192, MaxContextFacts: 2048}
		options := SourceOptions{Profile: ProfileSourceContext, Budgets: budgets}
		a, ea := InspectXML(context.Background(), bytes.NewReader(data), options)
		b, eb := InspectXML(context.Background(), bytes.NewReader(data), options)
		if !reflect.DeepEqual(a, b) || (ea == nil) != (eb == nil) {
			t.Fatal("nondeterministic source context")
		}
		if errors.Is(ea, ErrInvalidDocument) {
			t.Fatal("finding became legacy operation error")
		}
		doc, err := Decode(bytes.NewReader(data))
		if err != nil {
			return
		}
		model := ModelOptions{Profile: ProfileModelContext, Budgets: options.Budgets}
		a, ea = AssessModel(context.Background(), doc, model)
		b, eb = AssessModel(context.Background(), doc, model)
		if !reflect.DeepEqual(a, b) || (ea == nil) != (eb == nil) {
			t.Fatal("nondeterministic current-model context")
		}
		if errors.Is(ea, ErrInvalidDocument) {
			t.Fatal("model finding became legacy operation error")
		}
	})
}
