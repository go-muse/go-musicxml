package musicxml

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// All movement units are identical. These ordinary forward-only documents
// exercise excursion inventory without semantic gate reconstruction.
func TestIndependentTimelineUniformExcursionPracticalDefaults(t *testing.T) {
	t.Parallel()
	for _, count := range []int{100, 1000} {
		for _, repeated := range []bool{false, true} {
			var body strings.Builder
			if !repeated {
				body.WriteString(`<attributes><divisions>1</divisions></attributes>`)
			}
			for i := 0; i < count; i++ {
				if repeated {
					body.WriteString(`<attributes><divisions>1</divisions></attributes>`)
				}
				body.WriteString(`<forward><duration>1</duration></forward>`)
			}
			source := (localFixture{body: body.String()}).xml()
			document, err := Decode(strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			for _, model := range []bool{false, true} {
				for _, timeline := range []bool{false, true} {
					t.Run(fmt.Sprintf("moves%d/repeated%t/model%t/timeline%t", count, repeated, model, timeline), func(t *testing.T) {
						p := ProfileSourceLocal
						if model {
							p = ProfileModelLocal
						}
						if timeline {
							p = ProfileSourceTimeline
							if model {
								p = ProfileModelTimeline
							}
						}
						var r validation.Report
						var err error
						if model {
							r, err = AssessModel(context.Background(), document, AssessmentOptions{Profile: p})
						} else {
							r, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: p})
						}
						t.Logf("complete%t satisfied%t: %v", r.AssessmentComplete, r.Satisfied(), err)
						if err != nil || !r.Satisfied() {
							t.Fatalf("uniform movements exceeded practical defaults: %v; %s", err, localFindings(r))
						}
					})
				}
			}
		}
	}
}
