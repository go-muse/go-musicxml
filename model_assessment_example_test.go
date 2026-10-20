package musicxml_test

import (
	"context"
	"fmt"

	musicxml "github.com/go-muse/go-musicxml"
)

func ExampleAssessModel() {
	document := musicxml.NewScorePartwise(musicxml.PartDefinition{ID: "P1", Name: "Piano"})
	measure := musicxml.NewScorePartwiseMeasure("1")
	measure.AddNote(musicxml.NewRestNote(1))
	document.Part[0].AddMeasure(measure)
	report, err := musicxml.AssessModel(context.Background(), document, musicxml.ModelOptions{Profile: musicxml.ProfileModelLocal})
	if err != nil {
		panic(err)
	}
	fmt.Println(report.Target, report.Syntax, report.Satisfied())
	// Output: model not-assessed true
}
