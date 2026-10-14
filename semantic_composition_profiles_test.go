package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Literal IDs reproduce missing closed dispatch before composition is added.
func TestSemanticCompositionPublicProfiles(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		input := barlineLocationScore(timewise, barlineDivisions+barlineNote("1")+`<barline/>`, "")
		for _, target := range []validation.Target{validation.TargetSource, validation.TargetModel} {
			name := string(target) + map[bool]string{false: "/partwise", true: "/timewise"}[timewise]
			t.Run(name, func(t *testing.T) {
				profile := validation.ProfileID("musicxml-4.0/" + string(target) + "-semantics")
				options := DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: profile}}
				var report validation.Report
				var err error
				if target == validation.TargetSource {
					report, err = InspectXMLWithDictionaries(context.Background(), strings.NewReader(input), options)
				} else {
					doc, e := Decode(strings.NewReader(input))
					if e != nil {
						t.Fatal(e)
					}
					report, err = AssessModelWithDictionaries(context.Background(), doc, options)
				}
				if err != nil || !report.Satisfied() || report.Profile.ID != profile {
					t.Fatalf("profile=%s err=%v satisfied=%v findings=%+v", profile, err, report.Satisfied(), report.Findings)
				}
				selected := map[validation.RuleID]bool{}
				for _, key := range report.Profile.Required {
					selected[key.ID] = true
				}
				for _, id := range []validation.RuleID{"MX40-TIMELINE-effective-state", "MX40-CONTEXT-barline-location", "MX40-CONTEXT-graphical-repeats", "MX40-RESOURCE-local-links/musicxml"} {
					if !selected[id] {
						t.Errorf("missing required composition %s", id)
					}
				}
				if selected["MX40-CONTEXT-figure-number-numeric"] {
					t.Fatal("invented figure grammar selected")
				}
			})
		}
	}
}
