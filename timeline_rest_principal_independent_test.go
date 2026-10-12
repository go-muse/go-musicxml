package musicxml

import (
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// This characterizes the bounded duration clause's note-element interpretation,
// including the rest branch of note. It does not certify musical chord intent.
func TestTimelineRestPrincipalDurationCharacterization(t *testing.T) {
	const principal = `<note><rest/><duration>4</duration></note>`
	for _, form := range []string{"partwise", "timewise"} {
		for _, tc := range []struct {
			name, tones string
			failures    int
		}{
			{"shorter-one", timelineNote("1", `<chord/>`, ""), 0},
			{"shorter-three", timelineNote("3", `<chord/>`, ""), 0},
			{"increasing-one-three", timelineNote("1", `<chord/>`, "") + timelineNote("3", `<chord/>`, ""), 0},
			{"equal-four", timelineNote("4", `<chord/>`, ""), 0},
			{"longer-five", timelineNote("5", `<chord/>`, ""), 1},
			{"rest-member-equal-four", `<note><chord/><rest/><duration>4</duration></note>`, 0},
			{"rest-member-longer-five", `<note><chord/><rest/><duration>5</duration></note>`, 1},
		} {
			t.Run(form+"/"+tc.name, func(t *testing.T) {
				source := semanticQAScore(form, [][2]string{{semanticQADivisions + principal + tc.tones, `<note><rest/><duration>4</duration></note>`}})
				assessTimelineBoth(t, source, func(t *testing.T, r validation.Report) {
					semanticQAExpect(t, r, "MX40-TIMELINE-chord-duration-bound", validation.OutcomeFail, tc.failures)
					if !r.AssessmentComplete || len(r.Findings) != tc.failures {
						t.Fatalf("unexpected extra or incomplete assessment: %s", localFindings(r))
					}
					if r.Satisfied() != (tc.failures == 0) {
						t.Fatalf("duration-only acceptance changed: %+v", r)
					}
					for _, f := range r.Findings {
						principalPath := "/score-" + form + "/part/measure/note"
						if form == "timewise" {
							principalPath = "/score-timewise/measure/part/note"
						}
						if f.Location.Target == validation.TargetModel {
							principalPath = "ScorePartwise.Part[0].Measure[0].Content[1].Note"
							if form == "timewise" {
								principalPath = "ScoreTimewise.Measure[0].Part[0].Content[1].Note"
							}
						}
						if f.Reason != "chord-duration-exceeds-principal" || len(f.Related) != 1 || f.Related[0].Path != principalPath {
							t.Errorf("duration failure lost its rest principal: %+v", f)
						}
					}
				})
			})
		}
	}
}
