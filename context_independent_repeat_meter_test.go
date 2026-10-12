package musicxml

import (
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Uniform simple meter is independent of whether its authoritative publishers
// used one all-staff selector or exhaustive numbered-staff selectors. Different
// simple beats must retain unavailable common-beat evidence.
func TestIndependentContextRepeatUniformNumberedMeter(t *testing.T) {
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			for _, base := range []bool{false, true} {
				for _, mixed := range []bool{false, true} {
					for _, kind := range []string{"slash", "beat-repeat"} {
						t.Run(fmt.Sprintf("timewise%t/model%t/base%t/mixed%t/%s", tw, model, base, mixed, kind), func(t *testing.T) {
							times := ""
							if base {
								times = `<time><beats>4</beats><beat-type>4</beat-type></time>`
							}
							times += `<time number="1"><beats>4</beats><beat-type>4</beat-type></time>`
							if mixed {
								times += `<time number="2"><beats>2</beats><beat-type>2</beat-type></time>`
							} else {
								times += `<time number="2"><beats>4</beats><beat-type>4</beat-type></time>`
							}
							body := `<attributes><divisions>1</divisions>` + times + `<staves>2</staves><measure-style><` + kind + ` type="start"/></measure-style></attributes><note><rest/><duration>1</duration><staff>1</staff></note>`
							report := independentContextReport(t, independentContextScore("", [][]string{{body}}, tw), model)
							found := false
							for _, fact := range report.Context {
								if fact.Repeat == nil || fact.Repeat.Type.Value != "start" {
									continue
								}
								found = true
								got := fact.Repeat.Beat
								if mixed {
									if got.State == validation.EvidenceKnown {
										t.Errorf("different staff beats got common beat %+v", got)
									}
								} else if got.State != validation.EvidenceKnown || got.Value.String() != "1" {
									t.Errorf("identical numbered meters lack quarter beat: %+v", got)
								}
							}
							if !found {
								t.Fatal("missing graphical repeat")
							}
						})
					}
				}
			}
		}
	}
}
