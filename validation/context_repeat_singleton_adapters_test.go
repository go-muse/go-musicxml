package validation_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	musicxml "github.com/go-muse/go-musicxml"
	"github.com/go-muse/go-musicxml/validation"
)

const singletonRepeatHeader = `<part-list><score-part id="P1"><part-name>Audit</part-name><score-instrument id="I1"><instrument-name>Audit instrument</instrument-name></score-instrument></score-part></part-list>`
const singletonRepeatAttributes = `<attributes><divisions>1</divisions><staves>1</staves><transpose number="1"><chromatic>0</chromatic></transpose></attributes>`

func singletonRepeatNote(pitch string) string {
	return `<note>` + pitch + `<duration>1</duration><instrument id="I1"/><voice>1</voice><type>quarter</type><staff>1</staff></note>`
}
func singletonRepeatPitch(step, alter, octave string) string {
	return `<pitch><step>` + step + `</step><alter>` + alter + `</alter><octave>` + octave + `</octave></pitch>`
}
func singletonRepeatDocument(timewise bool, measures []string) string {
	root := "score-partwise"
	if timewise {
		root = "score-timewise"
	}
	out := `<` + root + ` version="4.0">` + singletonRepeatHeader
	if !timewise {
		out += `<part id="P1">`
	}
	for i, body := range measures {
		out += fmt.Sprintf(`<measure number="%d">`, i+1)
		if timewise {
			out += `<part id="P1">`
		}
		out += body
		if timewise {
			out += `</part>`
		}
		out += `</measure>`
	}
	if !timewise {
		out += `</part>`
	}
	return out + `</` + root + `>`
}
func singletonRepeatAssess(t *testing.T, measures []string, want validation.EvidenceState) {
	t.Helper()
	for _, timewise := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			t.Run(fmt.Sprintf("timewise=%v/model=%v", timewise, model), func(t *testing.T) {
				source := singletonRepeatDocument(timewise, measures)
				var r validation.Report
				var err error
				if model {
					doc, e := musicxml.Decode(strings.NewReader(source))
					if e != nil {
						t.Fatal(e)
					}
					r, err = musicxml.AssessModel(context.Background(), doc, musicxml.AssessmentOptions{Profile: musicxml.ProfileModelContext})
				} else {
					r, err = musicxml.InspectXML(context.Background(), strings.NewReader(source), musicxml.AssessmentOptions{Profile: musicxml.ProfileSourceContext})
				}
				if err != nil {
					t.Fatal(err)
				}
				if want == validation.EvidenceKnown {
					for _, finding := range r.Findings {
						if finding.Outcome == validation.OutcomeFail && strings.HasPrefix(string(finding.Rule.ID), "xsd.") {
							t.Fatalf("counterexample fixture has unrelated schema failure: %+v", finding)
						}
					}
				}
				found := false
				for _, c := range r.Context {
					if c.Repeat == nil || c.Repeat.Type.Value != "start" {
						continue
					}
					found = true
					v := c.Repeat.ContentEquivalent
					if want == validation.EvidenceInvalid && v.Cause == nil {
						t.Fatalf("invalid prerequisite lost owned cause: %+v", v)
					}
					if v.State != want || want == validation.EvidenceKnown && v.Value {
						t.Fatalf("content=%+v; want independent singleton counterexample state %v", v, want)
					}
					if want == validation.EvidenceKnown {
						failed := false
						for _, f := range r.Findings {
							if f.Subject == c.Subject && (f.Rule.ID == "MX40-CONTEXT-beat-repeat-content" || f.Rule.ID == "MX40-CONTEXT-measure-repeat-content") && f.Outcome == validation.OutcomeFail {
								failed = true
							}
						}
						if !failed {
							t.Fatal("known counterexample did not fail mandatory content consumer")
						}
					}
				}
				if !found {
					t.Fatal("missing start row")
				}
			})
		}
	}
}
func TestSingletonRepeatIndependentCounterexamples(t *testing.T) {
	for _, kind := range []string{"beat-repeat", "measure-repeat"} {
		for _, rest := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/rest=%v", kind, rest), func(t *testing.T) {
				source := singletonRepeatNote(singletonRepeatPitch("C", "0", "4"))
				pitch := singletonRepeatPitch("D", "0", "4")
				if rest {
					pitch = `<rest/>`
				}
				target := singletonRepeatNote(pitch)
				start := `<attributes><measure-style number="1"><` + kind + ` type="start">`
				if kind == "beat-repeat" {
					start += `<slash-type>quarter</slash-type>`
				} else {
					start += `1`
				}
				start += `</` + kind + `></measure-style></attributes>`
				stop := `<attributes><measure-style number="1"><` + kind + ` type="stop"/></measure-style></attributes>`
				measures := []string{singletonRepeatAttributes + source + start + target + stop}
				if kind == "measure-repeat" {
					measures = []string{singletonRepeatAttributes + source + `<attributes/>`, start + target + `<attributes/>`, stop}
				}
				singletonRepeatAssess(t, measures, validation.EvidenceKnown)
			})
		}
	}
}

func singletonRepeatBeatBody(attributes, source, target string) string {
	return attributes + source + `<attributes><measure-style number="1"><beat-repeat type="start"><slash-type>quarter</slash-type></beat-repeat></measure-style></attributes>` + target + `<attributes><measure-style number="1"><beat-repeat type="stop"/></measure-style></attributes>`
}
func TestSingletonRepeatCanonicalPitchControls(t *testing.T) {
	for _, tc := range []struct {
		name, a, b string
		want       validation.EvidenceState
	}{
		{"enharmonic", singletonRepeatPitch("C", "1", "4"), singletonRepeatPitch("D", "-1", "4"), validation.EvidenceUnknown},
		{"octave-edge", singletonRepeatPitch("B", "1", "3"), singletonRepeatPitch("C", "0", "4"), validation.EvidenceUnknown},
		{"fractional-enharmony", singletonRepeatPitch("C", "1.5", "4"), singletonRepeatPitch("D", "-0.5", "4"), validation.EvidenceUnknown},
		{"fractional-mismatch", singletonRepeatPitch("C", "0.5", "4"), singletonRepeatPitch("C", "0.25", "4"), validation.EvidenceKnown},
		{"same-written", singletonRepeatPitch("C", "0", "4"), singletonRepeatPitch("C", "0", "4"), validation.EvidenceUnknown},
		{"natural-absence", `<pitch><step>C</step><octave>4</octave></pitch>`, singletonRepeatPitch("D", "0", "4"), validation.EvidenceKnown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			singletonRepeatAssess(t, []string{singletonRepeatBeatBody(singletonRepeatAttributes, singletonRepeatNote(tc.a), singletonRepeatNote(tc.b))}, tc.want)
		})
	}
}
func TestSingletonRepeatTransformationControls(t *testing.T) {
	source := singletonRepeatNote(singletonRepeatPitch("C", "0", "4"))
	target := singletonRepeatNote(singletonRepeatPitch("D", "0", "4"))
	for _, tc := range []struct {
		name, attributes, target string
		want                     validation.EvidenceState
	}{
		{"common-nonzero", strings.Replace(singletonRepeatAttributes, `<chromatic>0</chromatic>`, `<chromatic>-2</chromatic>`, 1), target, validation.EvidenceKnown},
		{"common-octave", strings.Replace(singletonRepeatAttributes, `</chromatic>`, `</chromatic><octave-change>-1</octave-change>`, 1), target, validation.EvidenceKnown},
		{"unspecified", `<attributes><divisions>1</divisions><staves>1</staves></attributes>`, target, validation.EvidenceUnknown},
		{"unqualified-transform", strings.Replace(singletonRepeatAttributes, `transpose number="1"`, `transpose`, 1), target, validation.EvidenceUnknown},
		{"changed-transpose-equal-pitch", singletonRepeatAttributes, `<attributes><transpose number="1"><chromatic>-2</chromatic></transpose></attributes>` + target, validation.EvidenceUnknown},
		{"doubling", strings.Replace(singletonRepeatAttributes, `</transpose>`, `<double/></transpose>`, 1), target, validation.EvidenceUnknown},
		{"capo", strings.Replace(singletonRepeatAttributes, `<transpose`, `<staff-details><capo>2</capo></staff-details><transpose`, 1), target, validation.EvidenceUnknown},
		{"instrument-change", singletonRepeatAttributes, `<sound><instrument-change id="I1"><instrument-sound>keyboard.organ</instrument-sound></instrument-change></sound>` + target, validation.EvidenceUnknown},
		{"scordatura", singletonRepeatAttributes, `<direction><direction-type><scordatura><accord string="1"><tuning-step>D</tuning-step><tuning-octave>4</tuning-octave></accord></scordatura></direction-type></direction>` + target, validation.EvidenceUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			singletonRepeatAssess(t, []string{singletonRepeatBeatBody(tc.attributes, source, tc.target)}, tc.want)
		})
	}
}
func TestSingletonRepeatOrdinaryDomainControls(t *testing.T) {
	source := singletonRepeatNote(singletonRepeatPitch("C", "0", "4"))
	target := singletonRepeatNote(singletonRepeatPitch("D", "0", "4"))
	for _, tc := range []struct{ name, target string }{
		{"attack", strings.Replace(target, `<note>`, `<note attack="1">`, 1)},
		{"release", strings.Replace(target, `<note>`, `<note release="1">`, 1)},
		{"pass", strings.Replace(target, `<note>`, `<note time-only="2">`, 1)},
		{"hidden", strings.Replace(target, `<note>`, `<note print-object="no">`, 1)},
		{"cue", strings.Replace(target, `<note>`, `<note><cue/>`, 1)},
		{"grace", strings.Replace(strings.Replace(target, `<note>`, `<note><grace/>`, 1), `<duration>1</duration>`, ``, 1)},
		{"chord", strings.Replace(target, `<note>`, `<note><chord/>`, 1)},
		{"sound-tie", strings.Replace(target, `</duration>`, `</duration><tie type="start"/>`, 1)},
		{"ornament", strings.Replace(target, `</note>`, `<notations><ornaments><trill-mark/></ornaments></notations></note>`, 1)},
		{"technical", strings.Replace(target, `</note>`, `<notations><technical><bend><bend-alter>1</bend-alter></bend></technical></notations></note>`, 1)},
		{"play", strings.Replace(target, `</note>`, `<play><mute>on</mute></play></note>`, 1)},
		{"listen", strings.Replace(target, `</note>`, `<listen><wait/></listen></note>`, 1)},
		{"voice-absent", strings.Replace(target, `<voice>1</voice>`, ``, 1)},
		{"voice-changed", strings.Replace(target, `<voice>1</voice>`, `<voice>2</voice>`, 1)},
		{"staff-absent", strings.Replace(target, `<staff>1</staff>`, ``, 1)},
		{"instrument-absent", strings.Replace(target, `<instrument id="I1"/>`, ``, 1)},
		{"different-duration", strings.Replace(target, `<duration>1</duration>`, `<duration>2</duration>`, 1)},
		{"subdivision", strings.ReplaceAll(target, `<duration>1</duration>`, `<duration>0.5</duration>`) + strings.ReplaceAll(target, `<duration>1</duration>`, `<duration>0.5</duration>`)},
		{"backup-simultaneous", target + `<backup><duration>1</duration></backup>` + source},
	} {
		t.Run(tc.name, func(t *testing.T) {
			singletonRepeatAssess(t, []string{singletonRepeatBeatBody(singletonRepeatAttributes, source, tc.target)}, validation.EvidenceUnknown)
		})
	}
}
func TestSingletonRepeatEncodedDurationUnits(t *testing.T) {
	source := strings.Replace(singletonRepeatNote(singletonRepeatPitch("C", "0", "4")), `<duration>1</duration>`, `<duration>2</duration>`, 1)
	target := `<attributes><divisions>4</divisions></attributes>` + strings.Replace(singletonRepeatNote(singletonRepeatPitch("D", "0", "4")), `<duration>1</duration>`, `<duration>4</duration>`, 1)
	singletonRepeatAssess(t, []string{singletonRepeatBeatBody(strings.Replace(singletonRepeatAttributes, `<divisions>1</divisions>`, `<divisions>2</divisions>`, 1), source, target)}, validation.EvidenceKnown)
}

func TestSingletonRepeatContainmentControls(t *testing.T) {
	source := singletonRepeatNote(singletonRepeatPitch("C", "0", "4"))
	target := singletonRepeatNote(singletonRepeatPitch("D", "0", "4"))
	t.Run("source-crosses-beat", func(t *testing.T) {
		singletonRepeatAssess(t, []string{singletonRepeatBeatBody(singletonRepeatAttributes, strings.Replace(source, `<duration>1</duration>`, `<duration>2</duration>`, 1), target)}, validation.EvidenceUnknown)
	})
	t.Run("terminal-measure-end-unproved", func(t *testing.T) {
		singletonRepeatAssess(t, []string{singletonRepeatAttributes + source, `<attributes><measure-style number="1"><measure-repeat type="start">1</measure-repeat></measure-style></attributes>` + target + `<attributes/>`, `<attributes><measure-style number="1"><measure-repeat type="stop"/></measure-style></attributes>`}, validation.EvidenceUnknown)
	})
	t.Run("stop-absent", func(t *testing.T) {
		body := singletonRepeatBeatBody(singletonRepeatAttributes, source, target)
		body = strings.TrimSuffix(body, `<attributes><measure-style number="1"><beat-repeat type="stop"/></measure-style></attributes>`)
		singletonRepeatAssess(t, []string{body}, validation.EvidenceUnknown)
	})
}

func TestSingletonRepeatPreservesSufficientPositiveProof(t *testing.T) {
	source := strings.Replace(singletonRepeatNote(singletonRepeatPitch("C", "0", "4")), `<duration>1</duration>`, `<duration>2</duration>`, 1)
	target := `<attributes><divisions>4</divisions></attributes>` + strings.Replace(singletonRepeatNote(singletonRepeatPitch("C", "0", "4")), `<duration>1</duration>`, `<duration>4</duration>`, 1)
	body := singletonRepeatBeatBody(`<attributes><divisions>2</divisions><staves>1</staves></attributes>`, source, target)
	repeatAssessHeader(t, body, singletonRepeatHeader, func(t *testing.T, r validation.Report) {
		found := false
		for _, row := range r.Context {
			if row.Repeat != nil && row.Repeat.Type.Value == "start" {
				found = true
				if v := row.Repeat.ContentEquivalent; v.State != validation.EvidenceKnown || !v.Value {
					t.Fatalf("established sufficient positive changed %+v", v)
				}
			}
		}
		if !found {
			t.Fatal("missing repeat")
		}
	})
}
func TestSingletonRepeatInvalidInputsKeepCauses(t *testing.T) {
	source := singletonRepeatNote(singletonRepeatPitch("C", "0", "4"))
	target := singletonRepeatNote(singletonRepeatPitch("D", "0", "4"))
	for _, tc := range []struct{ name, attributes, target string }{
		{"transpose", strings.Replace(singletonRepeatAttributes, `<chromatic>0</chromatic>`, `<chromatic>NaN</chromatic>`, 1), target},
		{"pitch", singletonRepeatAttributes, strings.Replace(target, `<step>D</step>`, `<step>Q</step>`, 1)},
		{"instrument", singletonRepeatAttributes, strings.Replace(target, `id="I1"`, `id="missing"`, 1)},
		{"staff", singletonRepeatAttributes, strings.Replace(target, `<staff>1</staff>`, `<staff>0</staff>`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			singletonRepeatAssess(t, []string{singletonRepeatBeatBody(tc.attributes, source, tc.target)}, validation.EvidenceInvalid)
		})
	}
}
