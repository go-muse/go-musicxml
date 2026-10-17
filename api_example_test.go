package musicxml_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-muse/go-musicxml"
)

func ExampleDecode() {
	document, err := musicxml.Decode(strings.NewReader(
		`<score-timewise version="4.0"><part-list>` +
			`<score-part id="P1"><part-name>Flute</part-name></score-part>` +
			`</part-list></score-timewise>`,
	))
	if err != nil {
		log.Fatal(err)
	}

	if score, ok := musicxml.AsScorePartwise(document); ok {
		fmt.Println("partwise parts:", len(score.Part))
	} else if score, ok := musicxml.AsScoreTimewise(document); ok {
		fmt.Println("timewise measures:", len(score.Measure))
	} else if opus, ok := musicxml.AsOpusDocument(document); ok {
		fmt.Println("opus entries:", len(opus.Content))
	}

	// Output:
	// timewise measures: 0
}

func ExampleDecodeContext() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	document, err := musicxml.DecodeContext(
		ctx,
		strings.NewReader(`<score-partwise><part-list/></score-partwise>`),
		musicxml.DecodeOptions{MaxXMLDepth: 64},
	)
	if err != nil {
		log.Fatal(err)
	}
	_, partwise := musicxml.AsScorePartwise(document)
	fmt.Println(partwise)

	canceled, stop := context.WithCancel(context.Background())
	stop()
	_, err = musicxml.DecodeContext(
		canceled,
		strings.NewReader(`<score-partwise><part-list/></score-partwise>`),
		musicxml.DecodeOptions{},
	)
	fmt.Println(errors.Is(err, context.Canceled))

	// Output:
	// true
	// true
}

func ExampleEncodeWithOptions() {
	score := musicxml.NewScorePartwise(
		musicxml.PartDefinition{ID: "P1", Name: "Music"},
	)

	err := musicxml.EncodeWithOptions(
		os.Stdout,
		score,
		musicxml.EncodeOptions{XMLDeclaration: true, Indent: "  "},
	)
	if err != nil {
		log.Fatal(err)
	}

	// Output:
	// <?xml version="1.0" encoding="UTF-8"?>
	// <score-partwise version="4.0">
	//   <part-list>
	//     <score-part id="P1">
	//       <part-name>Music</part-name>
	//     </score-part>
	//   </part-list>
	//   <part id="P1"></part>
	// </score-partwise>
}

func ExampleValidate() {
	score := musicxml.NewScorePartwise(
		musicxml.PartDefinition{ID: "P1", Name: "Music"},
	)
	score.Part[0].ID = "P2"

	err := musicxml.Validate(score)
	fmt.Println(errors.Is(err, musicxml.ErrInvalidDocument))

	var invalid *musicxml.ValidationError
	if errors.As(err, &invalid) {
		fmt.Println(len(invalid.Issues) > 0)
	}

	// Output:
	// true
	// true
}

func ExampleMXLPackage_ResolveOpus() {
	value := &musicxml.MXLPackage{
		Document: &musicxml.OpusDocument{
			Content: []musicxml.OpusDocumentContent{
				{Score: &musicxml.OpusScore{Href: "scores/first.musicxml"}},
			},
		},
		Resources: []musicxml.MXLResource{{
			Path: "scores/first.musicxml",
			Data: []byte(`<score-partwise><part-list/></score-partwise>`),
		}},
	}

	var archive bytes.Buffer
	if err := musicxml.EncodeMXLPackage(&archive, value); err != nil {
		log.Fatal(err)
	}
	decoded, err := musicxml.DecodeMXLPackage(&archive)
	if err != nil {
		log.Fatal(err)
	}

	resolved, err := decoded.ResolveOpusWithOptions(
		musicxml.DecodeOptions{MaxXMLDepth: 64},
	)
	if err != nil {
		log.Fatal(err)
	}
	score := resolved.Content[0].Score
	_, partwise := musicxml.AsScorePartwise(score.Target)
	fmt.Println(score.Path, partwise)

	// Output:
	// scores/first.musicxml true
}
