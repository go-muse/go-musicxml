package musicxml

import (
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestIndependentContextPartSymbolCrossFrameUncertainty(t *testing.T) {
	for _, excursion := range []bool{false, true} {
		second := ""
		if excursion {
			second = `<backup><duration>2</duration></backup>`
		}
		second += `<attributes><part-symbol>bracket</part-symbol></attributes>`
		first := `<attributes><divisions>1</divisions><staves>2</staves></attributes><note><rest/><duration>1</duration></note>`
		for _, tw := range []bool{false, true} {
			for _, model := range []bool{false, true} {
				t.Run(fmt.Sprintf("excursion%t/timewise%t/model%t", excursion, tw, model), func(t *testing.T) {
					report := independentContextReport(t, independentContextScore("", [][]string{{first, second}}, tw), model)
					facts := layoutFacts(report, "part-symbol")
					if len(facts) != 2 {
						t.Fatalf("expected 2 symbol facts, got %d", len(facts))
					}
					if excursion {
						if facts[0].Value.State == validation.EvidenceKnown {
							t.Errorf("later-label unconfined publisher ignored: %+v", facts[0].Value)
						}
					} else if facts[0].Value.State != validation.EvidenceKnown || facts[0].Value.Value.Text != "brace" {
						t.Errorf("contained later publisher polluted initial default: %+v", facts[0].Value)
					}
					if facts[1].Value.State != validation.EvidenceKnown || facts[1].Value.Value.Text != "bracket" {
						t.Errorf("explicit local symbol lost: %+v", facts[1].Value)
					}
				})
			}
		}
	}
}
