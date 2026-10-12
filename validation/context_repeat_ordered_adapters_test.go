package validation_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	musicxml "github.com/go-muse/go-musicxml"
	"github.com/go-muse/go-musicxml/validation"
)

// Different voices encoded with backups may have identical musical content in
// different XML order. An ordered fast proof must retain the existing multiset
// proof on all four public adapter/hierarchy paths, while refusing new negatives.
func TestRepeatOrderedAdaptersPreserveFallback(t *testing.T) {
	note := func(pitch, voice, extra string) string {
		return `<note><pitch><step>` + pitch + `</step><octave>4</octave></pitch><duration>1</duration><voice>` + voice + `</voice>` + extra + `</note>`
	}
	backup := `<backup><duration>1</duration></backup>`
	pattern := note("C", "1", "") + backup + note("E", "2", "")
	for _, kind := range []validation.RepeatStyleKind{validation.RepeatStyleMeasure, validation.RepeatStyleBeat} {
		for _, mode := range []string{"ordered", "reordered", "different", "unavailable"} {
			target := pattern
			switch mode {
			case "reordered":
				target = note("E", "2", "") + backup + note("C", "1", "")
			case "different":
				target = note("F", "2", "") + backup + note("C", "1", "")
			case "unavailable":
				target = note("C", "1", `<notations><ornaments><trill-mark/></ornaments></notations>`) + backup + note("E", "2", "")
			}
			measures := []string{
				`<attributes><divisions>1</divisions></attributes>` + pattern,
				`<attributes><measure-style><measure-repeat type="start">1</measure-repeat></measure-style></attributes>` + target,
				`<attributes><measure-style><measure-repeat type="stop"/></measure-style></attributes>`,
			}
			if kind == validation.RepeatStyleBeat {
				measures = []string{`<attributes><divisions>1</divisions></attributes>` + pattern + `<attributes><measure-style><beat-repeat type="start"><slash-type>quarter</slash-type></beat-repeat></measure-style></attributes>` + target + `<attributes><measure-style><beat-repeat type="stop"/></measure-style></attributes>`}
			}
			for _, timewise := range []bool{false, true} {
				for _, model := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/timewise=%t/model=%t", kind, mode, timewise, model), func(t *testing.T) {
						hierarchy := "partwise"
						if timewise {
							hierarchy = "timewise"
						}
						source := `<score-` + hierarchy + ` version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>`
						if !timewise {
							source += `<part id="P1">`
						}
						for i, body := range measures {
							source += fmt.Sprintf(`<measure number="%d">`, i+1)
							if timewise {
								source += `<part id="P1">` + body + `</part>`
							} else {
								source += body
							}
							source += `</measure>`
						}
						if !timewise {
							source += `</part>`
						}
						source += `</score-` + hierarchy + `>`
						var report validation.Report
						var err error
						if model {
							doc, e := musicxml.Decode(strings.NewReader(source))
							if e != nil {
								t.Fatal(e)
							}
							report, err = musicxml.AssessModel(context.Background(), doc, musicxml.AssessmentOptions{Profile: musicxml.ProfileModelContext})
						} else {
							report, err = musicxml.InspectXML(context.Background(), strings.NewReader(source), musicxml.AssessmentOptions{Profile: musicxml.ProfileSourceContext})
						}
						if err != nil {
							t.Fatal(err)
						}
						found := false
						for _, c := range report.Context {
							if c.Repeat == nil || c.Repeat.Kind != kind || c.Repeat.Type.Value != "start" {
								continue
							}
							found = true
							proof := c.Repeat.ContentEquivalent
							if mode == "ordered" || mode == "reordered" {
								if proof.State != validation.EvidenceKnown || !proof.Value {
									t.Fatalf("equivalent voices lost: %+v", proof)
								}
							} else if proof.State != validation.EvidenceUnknown {
								t.Fatalf("unreviewed negative or unsupported equivalence: %+v", proof)
							}
						}
						if !found {
							t.Fatalf("missing start repeat: %+v", report.Findings)
						}
					})
				}
			}
		}
	}
}
