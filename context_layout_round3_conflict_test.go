package musicxml

import (
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"testing"
)

func TestLayoutRound3ConflictingBreakPreservesCauseInEveryOrder(t *testing.T) {
	t.Parallel()
	permutations := [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
	for _, invalid := range []bool{false, true} {
		for _, initial := range []bool{false, true} {
			for _, order := range permutations {
				t.Run(fmt.Sprintf("invalid=%t/initial=%t/order=%v", invalid, initial, order), func(t *testing.T) {
					third := `<print new-page="yes" blank-page="1"/>`
					want := validation.EvidenceUnknown
					if invalid {
						third = `<print new-page="bad"/>`
						want = validation.EvidenceInvalid
					}
					statements := []string{`<print new-page="yes" blank-page="1"/>`, `<print new-page="no"/>`, third}
					parts := [][]string{{statements[order[0]]}, {statements[order[1]]}, {statements[order[2]]}}
					if !initial {
						for i := range parts {
							parts[i] = append([]string{``}, parts[i]...)
						}
					}
					header := layoutRound1Supports + `<defaults>` + layoutRound3Margins + `<system-layout><system-distance>20</system-distance></system-layout></defaults>`
					layoutRound1Reports(t, header, parts, nil, func(t *testing.T, r validation.Report) {
						rows := layoutFacts(r, "page-left-margin")
						if !initial {
							rows = rows[3:]
						}
						if len(rows) != 3 {
							t.Fatalf("rows: %+v", rows)
						}
						for _, row := range rows {
							if row.Value.State != want || invalid && row.Value.Cause == nil {
								t.Errorf("margin conflict lost cause: %+v", row.Value)
							}
							for _, scope := range []validation.Evidence[validation.NodeRef]{row.Layout.Page, row.Layout.System} {
								if scope.State != want || invalid && scope.Cause == nil {
									t.Errorf("scope conflict depends on fold order: %+v", scope)
								}
							}
							if !invalid && row.Value.Reason != "parallel-print-break-disagreement" {
								t.Errorf("plain disagreement changed: %+v", row.Value)
							}
						}
						systems := layoutFacts(r, "system-distance")
						if !initial {
							systems = systems[3:]
						}
						for _, row := range systems {
							if got := row.Layout.Applicability; got.State != want || invalid && got.Cause == nil {
								t.Errorf("system-first conflict lost cause: %+v", got)
							}
						}
					})
				})
			}
		}
	}
}

func TestLayoutRound3UnavailableSelectionKeepsStrongerBreakCause(t *testing.T) {
	t.Parallel()
	for _, invalid := range []bool{false, true} {
		for _, initial := range []bool{false, true} {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("invalid=%t/initial=%t/reverse=%t", invalid, initial, reverse), func(t *testing.T) {
					peer := `<print new-page="no"/>`
					want := validation.EvidenceUnknown
					if invalid {
						peer = `<print new-page="bad"/>`
						want = validation.EvidenceInvalid
					}
					parts := [][]string{{`<print new-page="yes"/><print new-page="no"/>`}, {peer}}
					if reverse {
						parts[0], parts[1] = parts[1], parts[0]
					}
					if !initial {
						for i := range parts {
							parts[i] = append([]string{``}, parts[i]...)
						}
					} else {
						// A proved following page clears the publisher-selection gap,
						// exposing the initial parity cause without requiring an
						// unknown override to depend on default margin parity.
						for i := range parts {
							parts[i] = append(parts[i], `<print new-page="yes"/>`)
						}
					}
					layoutRound1Reports(t, layoutRound1Supports+`<defaults>`+layoutRound3Margins+`</defaults>`, parts, nil, func(t *testing.T, r validation.Report) {
						rows := layoutFacts(r, "page-left-margin")
						if initial {
							if len(rows) != 4 {
								t.Fatalf("initial and repaired rows: %+v", rows)
							}
							for _, row := range rows[:2] {
								if row.Value.State != validation.EvidenceUnknown || row.Value.Reason != "context-multiple-children" {
									t.Errorf("independent unknown publisher selection was overridden: %+v", row.Value)
								}
							}
						}
						rows = rows[2:]
						if len(rows) != 2 {
							t.Fatalf("rows: %+v", rows)
						}
						for _, row := range rows {
							if row.Value.State != want || invalid && row.Value.Cause == nil {
								t.Errorf("selection replaced stronger explicit cause: %+v", row.Value)
							}
							if !initial {
								for _, scope := range []validation.Evidence[validation.NodeRef]{row.Layout.Page, row.Layout.System} {
									if scope.State != want || invalid && scope.Cause == nil {
										t.Errorf("selection fabricated later membership: %+v", scope)
									}
								}
							}
						}
					})
				})
			}
		}
	}
}
