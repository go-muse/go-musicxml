package musicxml

import (
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestLayoutRound5WidthConflictProvenance(t *testing.T) {
	for _, other := range []string{"no", "maybe"} {
		for _, initial := range []bool{false, true} {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("other=%s/initial=%t/reverse=%t", other, initial, reverse), func(t *testing.T) {
					parts := [][]string{{`<print new-system="yes"/>`}, {`<print new-system="` + other + `"/>`}}
					if other == "maybe" {
						parts = append(parts, []string{`<print new-system="no"/>`})
					}
					if reverse {
						parts[0], parts[1] = parts[1], parts[0]
					}
					if !initial {
						for i := range parts {
							parts[i] = append([]string{``}, parts[i]...)
						}
					}
					layoutRound1Reports(t, layoutRound1Supports, parts, nil, func(t *testing.T, r validation.Report) {
						count := 0
						for _, row := range layoutFacts(r, "width-outcome") {
							if row.Layout.System.State == validation.EvidenceKnown {
								continue
							}
							count++
							if row.Layout.Provenance != validation.ContextExplicit {
								t.Errorf("unavailable boundary mislabeled: value=%+v provenance=%s", row.Value, row.Layout.Provenance)
							}
							if row.Layout.Raw.State != validation.EvidenceAbsent {
								t.Errorf("fabricated raw: %+v", row.Layout.Raw)
							}
							if row.Value.State != row.Layout.System.State {
								t.Errorf("boundary evidence changed: value=%+v system=%+v", row.Value, row.Layout.System)
							}
							if other == "maybe" && row.Value.Cause == nil {
								t.Error("malformed boundary lost cause")
							}
						}
						if count != len(parts) {
							t.Fatalf("conflict rows=%d", count)
						}
					})
				})
			}
		}
	}
}
