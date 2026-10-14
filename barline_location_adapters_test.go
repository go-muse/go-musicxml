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

var barlineBaselineEngine = sync.OnceValues(func() (spanEngineFixture, error) {
	d, err := buildTimelineAssessmentDefinitions()
	if err != nil {
		return spanEngineFixture{}, err
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
		{"left-before-invalid-duration", barlineDivisions + `<barline location="left"/><note><rest/><duration>-1</duration></note>`, "", []validation.Outcome{validation.OutcomePass}},
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
					baseline, err := barlineBaselineEngine()
					if err != nil {
						t.Fatal(err)
					}
					base := map[validation.Target]validation.Report{}
					spanBothWithEngine(t, baseline.engine, baseline.profiles, barlineLocationScore(timewise, tc.body, tc.attrs), true, "", func(t *testing.T, r validation.Report) { base[r.Target] = r })
					spanBothWithEngine(t, fixture.engine, fixture.profiles, barlineLocationScore(timewise, tc.body, tc.attrs), true, "", func(t *testing.T, r validation.Report) {
						expectedPass := uint64(0)
						for _, w := range tc.wants {
							if w == validation.OutcomePass {
								expectedPass++
							}
						}
						if got := r.Counts.Pass - base[r.Target].Counts.Pass; got != expectedPass {
							t.Errorf("positive barline decisions=%d want %d", got, expectedPass)
						}
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

func TestBarlineLocationMetadataAndChordMatrix(t *testing.T) {
	fixture, err := barlineLocationEngine()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, suffix string
		outcome      validation.Outcome
	}{
		{"print", `<print/>`, validation.OutcomePass},
		{"bookmark", `<bookmark id="B"/>`, validation.OutcomePass},
		{"link", `<link xmlns:xlink="http://www.w3.org/1999/xlink" xlink:href="https://example.com/score"/>`, validation.OutcomePass},
		{"direction-offset", `<direction><direction-type><words>after</words></direction-type><offset>100</offset></direction>`, validation.OutcomePass},
		{"sound-offset", `<sound tempo="80"><offset>100</offset></sound>`, validation.OutcomePass},
		{"listening-offset", `<listening><sync type="none"/><offset>100</offset></listening>`, validation.OutcomePass},
		{"harmony-offset", `<harmony><root><root-step>C</root-step></root><kind>major</kind><offset>100</offset></harmony>`, validation.OutcomePass},
		{"figured-bass-duration", `<figured-bass><figure><figure-number>6</figure-number></figure><duration>100</duration></figured-bass>`, validation.OutcomePass},
		{"grace", `<note><grace/><rest/></note>`, validation.OutcomePass},
		{"shorter-chord", `<note><chord/><pitch><step>E</step><octave>4</octave></pitch><duration>1</duration></note>`, validation.OutcomePass},
		{"equal-chord", `<note><chord/><pitch><step>E</step><octave>4</octave></pitch><duration>4</duration></note>`, validation.OutcomePass},
		{"overlong-chord", `<note><chord/><pitch><step>E</step><octave>4</octave></pitch><duration>5</duration></note>`, validation.OutcomeBlocked},
		{"invalid-chord-duration", `<note><chord/><pitch><step>E</step><octave>4</octave></pitch><duration>-1</duration></note>`, validation.OutcomeBlocked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, tw := range []bool{false, true} {
				t.Run(map[bool]string{false: "partwise", true: "timewise"}[tw], func(t *testing.T) {
					body := barlineDivisions + `<note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration></note><barline/>` + tc.suffix
					spanBothWithEngine(t, fixture.engine, fixture.profiles, barlineLocationScore(tw, body, ""), true, "", func(t *testing.T, r validation.Report) {
						var got *validation.Finding
						for _, f := range r.Findings {
							if f.Rule.ID == "MX40-CONTEXT-barline-location" {
								v := f
								got = &v
							}
						}
						if tc.outcome == validation.OutcomePass {
							if got != nil {
								t.Fatalf("metadata/chord changed endpoint: %+v", *got)
							}
							return
						}
						if got == nil || got.Outcome != tc.outcome || got.Cause == nil {
							t.Fatalf("missing blocked causal decision: %+v findings=%+v", got, r.Findings)
						}
						if tc.name == "overlong-chord" && got.Cause.Rule.ID != "MX40-TIMELINE-chord-duration-bound" {
							t.Fatalf("wrong prerequisite: %+v", got)
						}
					})
				})
			}
		})
	}
}

func TestBarlineLocationUnrelatedInvalidFieldsAndRelevantGaps(t *testing.T) {
	fixture, err := barlineLocationEngine()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body, gap string
		outcome         validation.Outcome
		cause           bool
	}{
		{"bad-pitch", barlineDivisions + `<note><pitch><step>H</step><octave>4</octave></pitch><duration>1</duration></note><barline/>`, "", validation.OutcomePass, false},
		{"bad-color", barlineDivisions + `<note color="oops"><rest/><duration>1</duration></note><barline/>`, "", validation.OutcomePass, false},
		{"bad-duration", barlineDivisions + `<note><rest/><duration>-1</duration></note><barline/>`, "", validation.OutcomeBlocked, true},
		{"orphan-chord", barlineDivisions + `<note><chord/><rest/><duration>1</duration></note><barline/>`, "", validation.OutcomeUnknown, false},
		{"unknown-note-enumeration", barlineDivisions + barlineNote("1") + `<barline/>`, "note", validation.OutcomeUnsupported, false},
		{"unknown-location", barlineDivisions + barlineNote("1") + `<barline/>`, "@barline", validation.OutcomeUnsupported, false},
		{"negative-excursion", barlineDivisions + barlineBackup("1") + barlineNote("2") + `<barline/>`, "", validation.OutcomeUnknown, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, tw := range []bool{false, true} {
				t.Run(map[bool]string{false: "partwise", true: "timewise"}[tw], func(t *testing.T) {
					spanBothWithEngine(t, fixture.engine, fixture.profiles, barlineLocationScore(tw, tc.body, ""), true, tc.gap, func(t *testing.T, r validation.Report) {
						var got *validation.Finding
						for _, f := range r.Findings {
							if f.Rule.ID == "MX40-CONTEXT-barline-location" {
								v := f
								got = &v
							}
						}
						if tc.outcome == validation.OutcomePass {
							if got != nil {
								t.Fatalf("unrelated field poisoned boundary: %+v", got)
							}
							return
						}
						if got == nil || got.Outcome != tc.outcome || (tc.cause && got.Cause == nil) {
							t.Fatalf("boundary=%+v findings=%+v", got, r.Findings)
						}
					})
				})
			}
		})
	}
}
