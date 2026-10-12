package musicxml

import (
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// An omitted assess element, a present assess without its required type, and a
// present valid override are distinct. Neither default scope means default type.
func TestIndependentContextAssessRequiredTypeCausal(t *testing.T) {
	for _, attr := range []string{"", ` type="bad"`, ` type="yes"`, ` type="no"`} {
		for _, tw := range []bool{false, true} {
			for _, model := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/timewise%t/model%t", attr, tw, model), func(t *testing.T) {
					body := `<note><rest/><duration>1</duration><listen><assess` + attr + `/></listen></note>`
					report := independentContextReport(t, independentContextScore("", [][]string{{body}}, tw), model)
					var rows []validation.NavigationNoteListening
					for _, f := range report.Context {
						if f.Navigation != nil && f.Navigation.Kind == validation.NavigationNote {
							rows = append(rows, f.Navigation.NoteListening...)
						}
					}
					if len(rows) != 1 {
						t.Fatalf("got %d assessment rows", len(rows))
					}
					got := rows[0]
					if attr == "" || attr == ` type="bad"` {
						if got.Override.State != validation.EvidenceInvalid || got.Override.Cause == nil {
							t.Errorf("required type lost causal invalidity: %+v", got.Override)
						}
					} else if got.Override.State != validation.EvidenceKnown || got.Override.Value != (attr == ` type="yes"`) {
						t.Errorf("valid override lost: %+v", got.Override)
					}
					if got.AllPlayers.State != validation.EvidenceKnown || !got.AllPlayers.Value || got.AllPasses.State != validation.EvidenceKnown || !got.AllPasses.Value {
						t.Errorf("independent absent scope defaults lost: %+v", got)
					}
				})
			}
		}
	}
}
