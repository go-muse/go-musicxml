package musicxml_test

import (
	"context"
	"fmt"
	"strings"

	musicxml "github.com/go-muse/go-musicxml"
	"github.com/go-muse/go-musicxml/validation"
)

func ExampleBundledDictionarySource() {
	const score = `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list><part id="P1"><measure number="1"><note><rest/><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental></note></measure></part></score-partwise>`
	report, err := musicxml.InspectXML(context.Background(), strings.NewReader(score), musicxml.SourceOptions{
		Profile: musicxml.ProfileSourceDictionaries,
		Dictionaries: &musicxml.DictionaryOptions{
			Source: musicxml.BundledDictionarySource(),
			Selections: []validation.DictionarySelection{{
				Manifest:  validation.DictionaryManifestSMuFL14Canonical,
				Relations: []validation.DictionaryRelation{validation.DictionaryRelationCanonicalName},
			}},
		},
	})
	fmt.Println("assessment error:", err)
	for _, result := range report.Context {
		if result.Property == validation.ContextDictionaryCanonical && result.Dictionary != nil {
			fmt.Println("pinned component verified:", result.Dictionary.ComponentVerified)
			fmt.Println("canonical glyph name:", result.Dictionary.Result.Value)
		}
	}
	// Output:
	// assessment error: <nil>
	// pinned component verified: true
	// canonical glyph name: true
}
