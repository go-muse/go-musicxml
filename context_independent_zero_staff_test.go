package musicxml

import (
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestIndependentContextZeroStaffCannotPublishSystem(t *testing.T) {
	bodies := [][]string{{`<print><system-layout><top-system-distance>100</top-system-distance></system-layout></print><attributes><staves>0</staves></attributes>`}, {`<print><system-layout><top-system-distance>200</top-system-distance></system-layout></print><attributes><staves>1</staves></attributes>`}}
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			t.Run(fmt.Sprintf("timewise%t/model%t", tw, model), func(t *testing.T) {
				report := independentContextReport(t, independentContextScore("", bodies, tw), model)
				facts := layoutFacts(report, "top-system-distance")
				if len(facts) != 2 {
					t.Fatalf("expected 2 facts, got %d", len(facts))
				}
				for _, fact := range facts {
					if fact.Value.State != validation.EvidenceKnown || fact.Value.Value.Decimal.String() != "200" {
						t.Errorf("zero-staff part supplied system layout: %+v", fact.Value)
					}
				}
			})
		}
	}
}
