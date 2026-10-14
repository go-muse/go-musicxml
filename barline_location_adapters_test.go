package musicxml

import (
	"sync"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

var barlineLocationEngine = sync.OnceValues(func() (spanEngineFixture, error) {
	d, err := buildTimelineAssessmentDefinitions()
	if err != nil {
		return spanEngineFixture{}, err
	}
	key, err := addBarlineLocationAssessment(d)
	if err != nil {
		return spanEngineFixture{}, err
	}
	for i := range d.profiles {
		d.profiles[i].ID = validation.ProfileID("barline-location-test/" + string(d.profiles[i].Target))
		d.profiles[i].Required = append(d.profiles[i].Required, key)
		d.profiles[i].Support = append(d.profiles[i].Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
	}
	c, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		return spanEngineFixture{}, err
	}
	e, err := validation.NewEngine(c)
	return spanEngineFixture{engine: e, profiles: d.profiles}, err
})

func barlineLocationScore(timewise bool, body, attrs string) string {
	header := `<part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>`
	if timewise {
		return `<score-timewise version="4.0">` + header + `<measure number="A" ` + attrs + `><part id="P1">` + body + `</part></measure></score-timewise>`
	}
	return `<score-partwise version="4.0">` + header + `<part id="P1"><measure number="A" ` + attrs + `>` + body + `</measure></part></score-partwise>`
}

const barlineDivisions = `<attributes><divisions>1</divisions></attributes>`

func barlineNote(d string) string    { return `<note><rest/><duration>` + d + `</duration></note>` }
func barlineBackup(d string) string  { return `<backup><duration>` + d + `</duration></backup>` }
func barlineForward(d string) string { return `<forward><duration>` + d + `</duration></forward>` }

func TestBarlineLocationAdapterMatrix(t *testing.T) {
	fixture, err := barlineLocationEngine()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body, attrs string
		wants             []validation.Outcome
	}{
		{"left-after-attributes", barlineDivisions + `<barline location="left"/>`, "", []validation.Outcome{validation.OutcomePass}},
		{"left-after-note", barlineDivisions + barlineNote("1") + `<barline location="left"/>`, "", []validation.Outcome{validation.OutcomeFail}},
		{"left-after-backup", barlineDivisions + barlineNote("1") + barlineBackup("1") + `<barline location="left"/>`, "", []validation.Outcome{validation.OutcomePass}},
		{"default-terminal-note", barlineDivisions + barlineNote("1") + `<barline/>`, "", []validation.Outcome{validation.OutcomePass}},
		{"right-before-second-voice", barlineDivisions + barlineNote("4") + `<barline/>` + barlineBackup("4") + barlineNote("4") + `<barline/>`, "", []validation.Outcome{validation.OutcomePass, validation.OutcomePass}},
		{"short-final-voice", barlineDivisions + barlineNote("4") + `<barline/>` + barlineBackup("4") + barlineNote("2") + `<barline/>`, "", []validation.Outcome{validation.OutcomeUnknown, validation.OutcomeFail}},
		{"restoring-forward", barlineDivisions + barlineNote("4") + barlineBackup("4") + barlineNote("1") + barlineForward("3") + `<barline/>`, "", []validation.Outcome{validation.OutcomePass}},
		{"new-maximum-forward", barlineDivisions + barlineNote("1") + barlineForward("3") + `<barline/>`, "", []validation.Outcome{validation.OutcomeUnknown}},
		{"forward-only", barlineDivisions + barlineForward("4") + `<barline/>`, "", []validation.Outcome{validation.OutcomeUnknown}},
		{"empty-right", `<barline/>`, "", []validation.Outcome{validation.OutcomeUnknown}},
		{"empty-middle", `<barline location="middle"/>`, "", []validation.Outcome{validation.OutcomeFail}},
		{"grace-only", `<note><grace/><rest/></note><barline/>`, "", []validation.Outcome{validation.OutcomeUnknown}},
		{"attributes-only", barlineDivisions + `<barline/>`, "", []validation.Outcome{validation.OutcomeUnknown}},
		{"interior-positions", barlineDivisions + barlineNote("3") + `<barline location="middle"/>` + barlineNote("2") + `<barline location="middle"/>` + barlineNote("4") + `<barline/>`, "", []validation.Outcome{validation.OutcomePass, validation.OutcomePass, validation.OutcomePass}},
		{"middle-at-end", barlineDivisions + barlineNote("1") + `<barline location="middle"/>`, "", []validation.Outcome{validation.OutcomeFail}},
		{"middle-after-backup", barlineDivisions + barlineNote("4") + barlineBackup("2") + `<barline location="middle"/>`, "", []validation.Outcome{validation.OutcomePass}},
		{"pickup-yes", `<attributes><divisions>1</divisions><time><beats>4</beats><beat-type>4</beat-type></time></attributes>` + barlineNote("1") + `<barline/>`, `implicit="yes"`, []validation.Outcome{validation.OutcomePass}},
		{"pickup-no", `<attributes><divisions>1</divisions><time><beats>4</beats><beat-type>4</beat-type></time></attributes>` + barlineNote("1") + `<barline/>`, `implicit="no"`, []validation.Outcome{validation.OutcomePass}},
		{"unknown-divisions", barlineNote("1") + `<barline/>`, "", []validation.Outcome{validation.OutcomeUnknown}},
		{"no-special-measure-rest", barlineDivisions + `<note><rest measure="yes"/><duration>4</duration></note>` + barlineBackup("4") + barlineNote("2") + `<barline/>`, "", []validation.Outcome{validation.OutcomeFail}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, timewise := range []bool{false, true} {
				t.Run(map[bool]string{false: "partwise", true: "timewise"}[timewise], func(t *testing.T) {
					spanBothWithEngine(t, fixture.engine, fixture.profiles, barlineLocationScore(timewise, tc.body, tc.attrs), true, "", func(t *testing.T, r validation.Report) {
						var got []validation.Outcome
						for _, d := range r.Findings {
							if d.Rule.ID == "MX40-CONTEXT-barline-location" {
								got = append(got, d.Outcome)
							}
						}
						var wants []validation.Outcome
						for _, w := range tc.wants {
							if w != validation.OutcomePass {
								wants = append(wants, w)
							}
						}
						if len(got) != len(wants) {
							t.Fatalf("decisions=%v want=%v findings=%+v", got, wants, r.Findings)
						}
						for i := range got {
							if got[i] != wants[i] {
								t.Errorf("barline %d = %s want %s; findings=%+v", i, got[i], wants[i], r.Findings)
							}
						}
					})
				})
			}
		})
	}
}
