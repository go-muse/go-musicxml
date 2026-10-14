package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestTimelinePracticalExactTrajectories(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, duration string
		repeated       bool
	}{{"exact-model-tenth", "0.1", false}, {"repeated-equal-divisions", "1", true}} {
		t.Run(tc.name, func(t *testing.T) {
			var body strings.Builder
			body.WriteString(`<attributes><divisions>3</divisions></attributes>`)
			for i := 0; i < 1000; i++ {
				if tc.repeated {
					body.WriteString(`<attributes><divisions>3</divisions></attributes>`)
				}
				body.WriteString(timelineNote(tc.duration, "", ""))
			}
			source := (localFixture{body: body.String()}).xml()
			doc, err := Decode(strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			for _, target := range []validation.Target{validation.TargetSource, validation.TargetModel} {
				t.Run(string(target), func(t *testing.T) {
					var report validation.Report
					if target == validation.TargetSource {
						report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceTimeline})
					} else {
						report, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelTimeline})
					}
					if err != nil || !report.Satisfied() {
						t.Fatalf("ordinary exact trajectory exhausted default budget: err=%v %s", err, localFindings(report))
					}
				})
			}
		})
	}
}
