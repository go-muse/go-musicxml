package xsdgen

import (
	"os"
	"strings"
	"testing"
)

func TestGeneratedModelMetadataUsesRenderPlans(t *testing.T) {
	set, err := Load(os.DirFS("../../schema/musicxml-4.0"), "musicxml.xsd", "catalog.xml")
	if err != nil {
		t.Fatal(err)
	}
	index, err := NewIndex(set)
	if err != nil {
		t.Fatal(err)
	}
	source, err := GenerateComplexTypesWithOptions(index, "musicxml", ComplexGenerationOptions{
		OrderedContentTypes: []QName{{Local: "credit"}, {Local: "lyric"}, {Local: "metronome"}}, ModelMetadata: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`modelDescriptors["Pitch"]`, `SchemaType: modelName{Local: "pitch"}`, `Name: "Octave"`, `Kind: modelSigned`,
		`Name: "Content"`, `Variants: []modelField`, `Name: "CodaAttr"`, `Space: "http://www.w3.org/1999/xlink"`,
		`Name: "TimeModification.ActualNotes"`,
	} {
		if !strings.Contains(string(source), fragment) {
			t.Errorf("generated metadata lacks %q", fragment)
		}
	}
}
