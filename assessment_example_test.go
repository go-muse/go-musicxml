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
	result, err := musicxml.DecodeAssessed(context.Background(), strings.NewReader(`<score-partwise/>`), musicxml.DecodeAssessmentOptions{})
	fmt.Println(result.Document != nil, result.Source.Requested, result.Conversion.State, err)
	// Output: true false converted <nil>
}
