package musicxml

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// The MusicXML prose selects the ordinal among all matching descendants of
// the next content anchor. Its beam XPath example does not prescribe a
// per-parent ordinal for unrelated element names.
func TestLinkDescendantOrdinalAcrossValidParents(t *testing.T) {
	for _, model := range []bool{false, true} {
		for _, marker := range []string{"link", "bookmark"} {
			for position := 1; position <= 3; position++ {
				t.Run(fmt.Sprintf("model=%t/%s/%d", model, marker, position), func(t *testing.T) {
					attrs := `xlink:href="other.musicxml"`
					if marker == "bookmark" {
						attrs = `id="mark"`
					}
					source := resourceScore(fmt.Sprintf(`<%s %s element="words" position="%d"/><direction><direction-type><words>A</words></direction-type><direction-type><words>B</words></direction-type></direction>`, marker, attrs, position))
					var report validation.Report
					var err error
					if model {
						var doc Document
						doc, err = Decode(strings.NewReader(source))
						if err == nil {
							report, err = AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelLinks})
						}
					} else {
						report, err = InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: ProfileSourceLinks})
					}
					if err != nil {
						t.Fatal(err)
					}
					if report.Conformance == validation.ConformanceViolated {
						t.Fatalf("invalid fixture: %+v", report.Findings)
					}
					found := false
					for _, row := range report.Context {
						if row.Resource == nil || row.Resource.Link == nil {
							continue
						}
						found = true
						got := row.Resource.Link.Attachment
						want := validation.EvidenceKnown
						if position == 3 {
							want = validation.EvidenceAbsent
						}
						if got.State != want || (position < 3 && got.Value.Node == 0) {
							t.Fatalf("ordinal %d: %+v", position, got)
						}
					}
					if !found {
						t.Fatal("missing marker projection")
					}
				})
			}
		}
	}
}
