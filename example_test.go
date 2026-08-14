package musicxml_test

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/go-muse/go-musicxml"
)

func ExampleDecodeScorePartwise() {
	score, err := musicxml.DecodeScorePartwise(strings.NewReader(
		`<score-partwise version="4.0">` +
			`<part-list>` +
			`<score-part id="P1"><part-name>Piano</part-name></score-part>` +
			`</part-list>` +
			`<part id="P1"><measure number="1">` +
			`<note><rest/><duration>1</duration></note>` +
			`</measure></part>` +
			`</score-partwise>`,
	))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(score.EffectiveVersion(), score.Part[0].ID)

	// Output:
	// 4.0 P1
}

func ExampleNewScorePartwise() {
	score := musicxml.NewScorePartwise(
		musicxml.PartDefinition{ID: "P1", Name: "Piano"},
	)
	measure := musicxml.NewScorePartwiseMeasure("1")
	measure.AddAttributes(&musicxml.Attributes{
		Divisions: musicxml.Ptr(musicxml.PositiveDivisions(1)),
	})
	measure.AddNote(musicxml.NewPitchedNote(
		musicxml.StepC,
		4,
		1,
	))
	score.Part[0].AddMeasure(measure)

	if err := score.Validate(); err != nil {
		log.Fatal(err)
	}

	var encoded strings.Builder
	if err := musicxml.Encode(&encoded, score); err != nil {
		log.Fatal(err)
	}
	fmt.Println(encoded.String())

	// Output:
	// <score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list><part id="P1"><measure number="1"><attributes><divisions>1</divisions></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note></measure></part></score-partwise>
}

func ExampleEncodeMXL() {
	score := musicxml.NewScorePartwise(
		musicxml.PartDefinition{ID: "P1", Name: "Music"},
	)
	measure := musicxml.NewScorePartwiseMeasure("1")
	measure.AddNote(musicxml.NewRestNote(1))
	score.Part[0].AddMeasure(measure)

	var archive bytes.Buffer
	if err := musicxml.EncodeMXL(&archive, score); err != nil {
		log.Fatal(err)
	}

	decoded, err := musicxml.DecodeMXL(bytes.NewReader(archive.Bytes()))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%T\n", decoded)

	// Output:
	// *musicxml.ScorePartwise
}
