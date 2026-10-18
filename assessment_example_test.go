package musicxml_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/go-muse/go-musicxml"
)

func ExampleInspectXML() {
	report, err := musicxml.InspectXML(context.Background(), strings.NewReader(`<score-partwise/>`), musicxml.AssessmentOptions{Profile: musicxml.ProfileSourceXSDPreview})
	fmt.Println(report.InputComplete, report.Conformance, err)
	// Output: true violated <nil>
}

func ExampleDecodeAssessed() {
	result, err := musicxml.DecodeAssessed(context.Background(), strings.NewReader(`<opus/>`), musicxml.DecodeAssessmentOptions{})
	fmt.Println(result.Document != nil, result.Source.Satisfied(), result.Model.Satisfied(), result.Conversion.State, err)
	// Output: true true true converted <nil>
}

// The lenient mode returns the converted document with both reports even
// when an assessment rejects it. Here divisions must be positive.
func ExampleDecodeAssessed_lenient() {
	source := `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>` +
		`<part id="P1"><measure number="1"><attributes><divisions>0</divisions></attributes><note><rest/><duration>1</duration></note></measure></part></score-partwise>`
	strict, err := musicxml.DecodeAssessed(context.Background(), strings.NewReader(source), musicxml.DecodeAssessmentOptions{})
	fmt.Println(strict.Document != nil, strict.Accepted, err != nil)
	lenient, err := musicxml.DecodeAssessed(context.Background(), strings.NewReader(source), musicxml.DecodeAssessmentOptions{Lenient: true})
	fmt.Println(lenient.Document != nil, lenient.Accepted, err, lenient.Source.Conformance, lenient.Model.Conformance)
	// Output:
	// false false true
	// true false <nil> violated violated
}

// Scenario 3 checks a model built in Go with the same semantic profile that
// strict loading applies to the converted model.
func ExampleAssessModel_scenario() {
	score := musicxml.NewScorePartwise(musicxml.PartDefinition{ID: "P1", Name: "Piano"})
	measure := musicxml.NewScorePartwiseMeasure("1")
	measure.AddNote(musicxml.NewRestNote(1))
	score.Part[0].Measure = append(score.Part[0].Measure, measure)
	report, err := musicxml.AssessModel(context.Background(), score, musicxml.AssessmentOptions{})
	fmt.Println(report.Profile.ID, report.Satisfied(), err)
	// Output: musicxml-4.0/model-semantics true <nil>
}

func ExampleDecodeMXLAssessed() {
	var archive bytes.Buffer
	if err := musicxml.EncodeMXL(&archive, musicxml.NewOpusDocument()); err != nil {
		panic(err)
	}
	result, err := musicxml.DecodeMXLAssessed(context.Background(), bytes.NewReader(archive.Bytes()), musicxml.DecodeMXLAssessmentOptions{})
	fmt.Println(result.Package != nil, result.Accepted, result.Source.Satisfied(), result.Model.Satisfied(), err)
	// Output: true true true true <nil>
}
