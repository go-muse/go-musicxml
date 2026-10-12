package musicxml

import (
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestIndependentContextUnplacedSymbolRetainsPlacement(t *testing.T) {
	first := `<attributes><divisions>1</divisions><staves>2</staves></attributes><note><rest/><duration>1</duration></note>`
	// Changing divisions and rewinding across their unproved boundary leaves the
	// symbol's origin unavailable and unconfined. Its later measure label is not
	// proof that it cannot precede the first attributes declaration.
	second := `<note><rest/><duration>2</duration></note><attributes><divisions>2</divisions></attributes><backup><duration>8</duration></backup><attributes><part-symbol>bracket</part-symbol></attributes>`
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			t.Run(fmt.Sprintf("timewise%t/model%t", tw, model), func(t *testing.T) {
				report := independentContextReport(t, independentContextScore("", [][]string{{first, second}}, tw), model)
				facts := layoutFacts(report, "part-symbol")
				if len(facts) != 3 {
					t.Fatalf("expected 3 symbol facts, got %d", len(facts))
				}
				if facts[0].Value.State == validation.EvidenceKnown {
					t.Errorf("unconfined symbol gap treated as future-only: %+v", facts[0].Value)
				}
				if facts[2].Value.State != validation.EvidenceKnown || facts[2].Value.Value.Text != "bracket" {
					t.Errorf("explicit local symbol lost: %+v", facts[2].Value)
				}
			})
		}
	}
}
