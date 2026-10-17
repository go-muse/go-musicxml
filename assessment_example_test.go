package musicxml_test

import (
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
