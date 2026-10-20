package validation_test

import (
	"context"
	musicxml "github.com/go-muse/go-musicxml"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func repeatRound1Score(t *testing.T, measures []string, other []string, check func(*testing.T, validation.Report)) {
	t.Helper()
	for _, tw := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			header := `<part-list><score-part id="P1"><part-name>Piano</part-name></score-part>`
			if other != nil {
				header += `<score-part id="P2"><part-name>Other</part-name></score-part>`
			}
			header += `</part-list>`
			var source string
			if tw {
				source = `<score-timewise version="4.0">` + header
				for i, m := range measures {
					source += `<measure number="m"><part id="P1">` + m + `</part>`
					if other != nil {
						source += `<part id="P2">` + other[i] + `</part>`
					}
					source += `</measure>`
				}
				source += `</score-timewise>`
			} else {
				source = `<score-partwise version="4.0">` + header + `<part id="P1">`
				for _, m := range measures {
					source += `<measure number="m">` + m + `</measure>`
				}
				source += `</part>`
				if other != nil {
					source += `<part id="P2">`
					for _, m := range other {
						source += `<measure number="m">` + m + `</measure>`
					}
					source += `</part>`
				}
				source += `</score-partwise>`
			}
			t.Run(map[bool]string{true: "timewise", false: "partwise"}[tw]+"/"+map[bool]string{true: "model", false: "source"}[model], func(t *testing.T) {
				var report validation.Report
				var err error
				opts := musicxml.SourceOptions{Profile: musicxml.ProfileSourceContext}
				if model {
					doc, e := musicxml.Decode(strings.NewReader(source))
					if e != nil {
						t.Fatal(e)
					}
					report, err = musicxml.AssessModel(context.Background(), doc, musicxml.ModelOptions{Profile: musicxml.ProfileModelContext})
				} else {
					report, err = musicxml.InspectXML(context.Background(), strings.NewReader(source), opts)
				}
				if err != nil {
					t.Fatal(err)
				}
				check(t, report)
			})
		}
	}
}

const repeatR1Note = `<note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`
const repeatR1Start = `<attributes><measure-style><measure-repeat type="start">1</measure-repeat></measure-style></attributes>`
const repeatR1Stop = `<attributes><measure-style><measure-repeat type="stop"/></measure-style></attributes>`

func repeatR1Content(t *testing.T, r validation.Report, want validation.EvidenceState, value bool) {
	t.Helper()
	found := false
	for _, c := range r.Context {
		if c.Repeat != nil && c.Repeat.Type.Value == "start" && c.Repeat.Kind != validation.RepeatStyleSlash {
			found = true
			got := c.Repeat.ContentEquivalent
			if got.State != want || want == validation.EvidenceKnown && got.Value != value {
				t.Errorf("content=%+v want %s %v", got, want, value)
			}
		}
	}
	if !found {
		t.Error("missing repeat start")
	}
}
func TestRound1RepeatEmptyTargetConsumers(t *testing.T) {
	for _, target := range []string{"", `<forward><duration>1</duration></forward>`} {
		repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes>` + repeatR1Note + `<direction><direction-type><words>endpoint witness</words></direction-type></direction>`, repeatR1Start + target, repeatR1Stop}, nil, func(t *testing.T, r validation.Report) {
			repeatR1Content(t, r, validation.EvidenceKnown, false)
			found := false
			for _, f := range r.Findings {
				if f.Reason == "context-repeat-encoded-content-missing" {
					found = true
				}
			}
			if !found {
				t.Error("missing public consumer failure")
			}
		})
	}
	repeatAssessBoth(t, `<attributes><divisions>1</divisions></attributes>`+repeatR1Note+`<attributes><measure-style><beat-repeat type="start"><slash-type>quarter</slash-type></beat-repeat></measure-style></attributes><forward><duration>1</duration></forward><attributes><measure-style><beat-repeat type="stop"/></measure-style></attributes>`, func(t *testing.T, r validation.Report) {
		repeatR1Content(t, r, validation.EvidenceKnown, false)
		found := false
		for _, f := range r.Findings {
			if f.Reason == "context-repeat-encoded-content-missing" {
				found = true
			}
		}
		if !found {
			t.Error("missing public beat consumer failure")
		}
	})
}
func TestRound1RepeatClosureAndEvidence(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>1</divisions></attributes>`+repeatR1Note+`<attributes><measure-style><beat-repeat type="start"><slash-type>quarter</slash-type></beat-repeat></measure-style></attributes>`+strings.Repeat(repeatR1Note, 3), func(t *testing.T, r validation.Report) { repeatR1Content(t, r, validation.EvidenceKnown, true) })
	repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes>` + repeatR1Note, repeatR1Start + repeatR1Note, repeatR1Stop}, nil, func(t *testing.T, r validation.Report) {
		for _, c := range r.Context {
			if c.Repeat == nil {
				continue
			}
			if c.Repeat.Beat.State == "" || c.Repeat.PatternMeasures.State == "" {
				t.Error("zero repeat evidence")
			}
			if c.Repeat.Type.Value == "stop" && c.Repeat.ContentEquivalent.State != validation.EvidenceAbsent {
				t.Error("stop content must be absent")
			}
		}
	})
}
func TestRound1RepeatUnrelatedPartCoverage(t *testing.T) {
	repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes>` + repeatR1Note, repeatR1Start + repeatR1Note, repeatR1Stop}, []string{`<attributes><divisions>1</divisions></attributes><note><rest/><duration>0</duration></note>`, "", ""}, func(t *testing.T, r validation.Report) { repeatR1Content(t, r, validation.EvidenceKnown, true) })
}
func TestRound1RepeatEngravingContent(t *testing.T) {
	for _, extra := range []string{`<type>quarter</type><dot/>`, `<accidental>natural</accidental>`, `<beam number="1">begin</beam>`, `<notations><slur type="start"/></notations>`, `<time-modification><actual-notes>3</actual-notes><normal-notes>2</normal-notes></time-modification>`} {
		note := strings.Replace(repeatR1Note, `</note>`, extra+`</note>`, 1)
		repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes>` + note, repeatR1Start + note, repeatR1Stop}, nil, func(t *testing.T, r validation.Report) { repeatR1Content(t, r, validation.EvidenceKnown, true) })
	}
}
func TestRound1RepeatInterchangeableBeat(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>1</divisions><time><beats>3</beats><beat-type>4</beat-type><interchangeable><beats>6</beats><beat-type>8</beat-type></interchangeable></time><measure-style><beat-repeat type="start"/></measure-style></attributes>`+repeatR1Note, func(t *testing.T, r validation.Report) {
		for _, c := range r.Context {
			if c.Repeat != nil && c.Repeat.Beat.State == validation.EvidenceKnown {
				t.Error("interchangeable beat guessed")
			}
		}
	})
}
func TestRound1RepeatUnwitnessedEndAndPlaybackBoundaries(t *testing.T) {
	// Missing encoded scored onsets need no terminal-End containment proof.
	repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes>` + repeatR1Note, repeatR1Start, repeatR1Stop}, nil, func(t *testing.T, r validation.Report) { repeatR1Content(t, r, validation.EvidenceKnown, false) })
	for _, extra := range []string{`<tie type="start"/>`, `<notations><ornaments><trill-mark/></ornaments></notations>`} {
		note := strings.Replace(repeatR1Note, `<duration>1</duration>`, `<duration>1</duration>`+extra, 1)
		repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes>` + note, repeatR1Start + note, repeatR1Stop}, nil, func(t *testing.T, r validation.Report) { repeatR1Content(t, r, validation.EvidenceUnknown, false) })
	}
}
func TestRound1RepeatUnprovedIdentityIsNotInvented(t *testing.T) {
	repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes>` + repeatR1Note, repeatR1Start + repeatR1Note, repeatR1Stop}, []string{`<attributes><divisions>1</divisions></attributes><note><rest/></note>`, "", ""}, func(t *testing.T, r validation.Report) {
		for _, c := range r.Context {
			if c.Repeat != nil && c.Repeat.Type.Value == "start" && c.Repeat.ContentEquivalent.State == validation.EvidenceKnown {
				t.Fatal("unavailable canonical part identity invented")
			}
		}
	})
}

func TestRound1RepeatClosureIgnoresDisplayDisplacement(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>1</divisions></attributes>`+repeatR1Note+`<attributes><measure-style><beat-repeat type="start"><slash-type>quarter</slash-type></beat-repeat></measure-style></attributes>`+strings.Repeat(repeatR1Note, 3)+`<direction><direction-type><words>displaced</words></direction-type><offset>1000</offset></direction>`, func(t *testing.T, r validation.Report) { repeatR1Content(t, r, validation.EvidenceKnown, true) })
}

func TestRound1RepeatClosureDoesNotAddBackupDuration(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>1</divisions></attributes>`+repeatR1Note+`<attributes><measure-style><beat-repeat type="start"><slash-type>quarter</slash-type></beat-repeat></measure-style></attributes>`+strings.Repeat(repeatR1Note, 3)+`<backup><duration>4</duration></backup>`, func(t *testing.T, r validation.Report) { repeatR1Content(t, r, validation.EvidenceKnown, true) })
}
