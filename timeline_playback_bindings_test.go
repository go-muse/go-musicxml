package musicxml

import (
	"errors"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func TestTimelinePlaybackRequiresExactNavigationAndNoteRoles(t *testing.T) {
	for _, tc := range []struct{ owner, name string }{{"barline", "location"}, {"barline", "divisions"}, {"barline", "segno"}, {"barline", "coda"}, {"sound", "segno"}, {"sound", "coda"}, {"sound", "dacapo"}, {"sound", "dalsegno"}, {"sound", "tocoda"}, {"sound", "forward-repeat"}, {"sound", "fine"}, {"sound", "time-only"}, {"note", "time-only"}, {"note", "dynamics"}, {"note", "end-dynamics"}, {"note", "pizzicato"}} {
		t.Run(tc.owner+"/"+tc.name, func(t *testing.T) {
			schema := timelineContractVariantSchema(t, func(data *validation.SchemaData) {
				key := validation.SchemaName{Local: tc.owner}
				typ := *data.Types[key]
				c := *typ.Complex
				c.Attributes = nil
				found := false
				for _, a := range typ.Complex.Attributes {
					if a.Name.Local == tc.name {
						found = true
						continue
					}
					c.Attributes = append(c.Attributes, a)
				}
				if !found {
					t.Fatal("fixture attribute missing")
				}
				typ.Complex = &c
				data.Types[key] = &typ
			})
			if _, err := validation.NewTimelinePlan(validation.TimelinePlanSpec{Schema: schema, PartOwners: timelineContractOwners(t, schema)}); !errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatalf("accepted missing navigation/note role: %v", err)
			}
		})
	}
}

func TestTimelinePlaybackNavigationDoesNotReplaceScoreDivisions(t *testing.T) {
	for _, form := range []string{"partwise", "timewise"} {
		for _, marker := range []string{`<sound segno="S" divisions="100"/>`, `<sound coda="C" divisions="0"/>`, `<sound dalsegno="S" divisions="-1"/>`, `<barline location="middle" segno="S" divisions="100"><segno/></barline>`, `<barline location="middle" coda="C" divisions="0"><coda/></barline>`} {
			t.Run(form+"/"+marker, func(t *testing.T) {
				body := `<attributes><divisions>3</divisions></attributes>` + timelineNote("6", "", "") + marker + timelineNote("7", `<chord/>`, "")
				assessTimelineBoth(t, semanticQAScore(form, [][2]string{{body, ""}}), func(t *testing.T, report validation.Report) {
					if !report.AssessmentComplete {
						t.Fatalf("navigation annotation poisoned score units: %s", localFindings(report))
					}
					found := false
					for _, f := range report.Findings {
						if f.Rule.ID == "MX40-TIMELINE-chord-duration-bound" && f.Outcome == validation.OutcomeFail {
							found = true
						}
						if f.Outcome == validation.OutcomeFail && strings.HasPrefix(string(f.Rule.ID), "MX40-XSD-") {
							t.Fatalf("invalid schema fixture: %s", localFindings(report))
						}
					}
					if !found {
						t.Fatalf("navigation divisions replaced attributes/divisions: %s", localFindings(report))
					}
				})
			})
		}
	}
}
