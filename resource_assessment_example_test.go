package musicxml_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	musicxml "github.com/go-muse/go-musicxml"
)

func ExampleInspectXMLWithResources() {
	bundle, err := musicxml.InspectXMLWithResources(context.Background(), strings.NewReader(`<opus/>`), musicxml.LinkedAssessmentOptions{
		Graph: musicxml.AssessmentOptions{Profile: musicxml.ProfileResourceGraph},
		// No selected dependencies means no external resolver invocation.
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(bundle.Satisfied(), bundle.ResourceCount())
	// Output: true 1
}

func ExampleInspectMXL() {
	var original bytes.Buffer
	if err := musicxml.EncodeMXL(&original, musicxml.NewOpusDocument()); err != nil {
		panic(err)
	}
	bundle, err := musicxml.InspectMXL(context.Background(), bytes.NewReader(original.Bytes()), musicxml.MXLAssessmentOptions{
		Graph: musicxml.AssessmentOptions{Profile: musicxml.ProfileMXLSource},
	})
	if err != nil {
		panic(err)
	}
	// The original container and musical XML retain separate intrinsic reports.
	fmt.Println(bundle.Satisfied(), bundle.ResourceCount())
	// Output: true 2
}
