package musicxml

import (
	"context"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func TestResourceXLinkBehaviorIntentMatrix(t *testing.T) {
	for _, show := range []string{"", "new", "replace", "embed", "other", "none", "bad"} {
		for _, actuate := range []string{"", "onRequest", "onLoad", "other", "none", "bad"} {
			attrs := ""
			if show != "" {
				attrs += ` xlink:show="` + show + `"`
			}
			if actuate != "" {
				attrs += ` xlink:actuate="` + actuate + `"`
			}
			source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="score.musicxml"` + attrs + `/></opus>`
			for _, model := range []bool{false, true} {
				var r validation.Report
				var err error
				if model {
					doc, e := Decode(strings.NewReader(source))
					if e != nil {
						t.Fatal(e)
					}
					r, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelLinks})
				} else {
					r, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceLinks})
				}
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, v := range r.Context {
					if v.Resource == nil || v.Resource.Link == nil {
						continue
					}
					found = true
					l := v.Resource.Link
					for _, axis := range []struct {
						input, def string
						e          validation.Evidence[string]
					}{{show, "replace", l.PresentationIntent}, {actuate, "onRequest", l.ActuationIntent}} {
						expected := axis.input
						if expected == "" {
							expected = axis.def
						}
						if expected == "other" || expected == "none" {
							if axis.e.State != validation.EvidenceUnknown || axis.e.Reason != "application-dependent-xlink-behavior" {
								t.Fatalf("%q %+v", expected, axis.e)
							}
						} else if expected == "bad" {
							if axis.e.State != validation.EvidenceInvalid || axis.e.Cause == nil {
								t.Fatalf("invalid lost %+v", axis.e)
							}
						} else if axis.e.State != validation.EvidenceKnown || axis.e.Value != expected {
							t.Fatalf("expected%q got%+v", expected, axis.e)
						}
					}
				}
				if !found {
					t.Fatal("no link")
				}
			}
		}
	}
}
