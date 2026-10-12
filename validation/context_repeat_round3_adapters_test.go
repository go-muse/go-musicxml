package validation_test

import (
	"context"
	musicxml "github.com/go-muse/go-musicxml"
	"github.com/go-muse/go-musicxml/validation"
	"reflect"
	"strings"
	"testing"
)

// An unrepresentable content choice is a model-only input: serializing it
// would discard the exact representation defect this regression exercises.
func TestRound3RepeatEventlessModelContainer(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		t.Run(map[bool]string{false: "partwise", true: "timewise"}[timewise], func(t *testing.T) {
			measures := []string{`<attributes><divisions>1</divisions></attributes>` + repeatR1Note, repeatR1Start + repeatR1Note, repeatR1Note, repeatR1Stop}
			header := `<part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>`
			source := `<score-partwise version="4.0">` + header + `<part id="P1">`
			if timewise {
				source = `<score-timewise version="4.0">` + header
			}
			for _, body := range measures {
				if timewise {
					source += `<measure number="m"><part id="P1">` + body + `</part></measure>`
				} else {
					source += `<measure number="m">` + body + `</measure>`
				}
			}
			if timewise {
				source += `</score-timewise>`
			} else {
				source += `</part></score-partwise>`
			}
			doc, err := musicxml.Decode(strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			switch v := doc.(type) {
			case *musicxml.ScorePartwise:
				v.Part[0].Measure[2].Content[0].Forward = &musicxml.Forward{}
			case *musicxml.ScoreTimewise:
				v.Measure[2].Part[0].Content[0].Forward = &musicxml.Forward{}
			}
			report, err := musicxml.AssessModel(context.Background(), doc, musicxml.AssessmentOptions{Profile: musicxml.ProfileModelContext})
			if err != nil {
				t.Fatal(err)
			}
			var cause *validation.DecisionRef
			for _, f := range report.Findings {
				if f.Reason == "model-choice-cardinality" {
					c := validation.DecisionRef{Rule: f.Rule, Subject: f.Subject}
					cause = &c
				}
			}
			if cause == nil {
				t.Fatal("missing model representation cause")
			}
			starts := 0
			for _, c := range report.Context {
				if c.Repeat == nil || c.Repeat.Type.Value != "start" {
					continue
				}
				starts++
				for name, ev := range map[string]validation.Evidence[bool]{"scope": c.Repeat.NoteScopesComplete, "content": c.Repeat.ContentEquivalent} {
					if ev.State != validation.EvidenceInvalid || ev.Reason != "model-choice-cardinality" || !reflect.DeepEqual(ev.Cause, cause) {
						t.Errorf("%s=%+v want model cause %+v", name, ev, cause)
					}
				}
				if len(c.Repeat.Notes) != 2 {
					t.Errorf("represented notes=%d want 2", len(c.Repeat.Notes))
				}
				foundConsumer := false
				for _, f := range report.Findings {
					if f.Subject == c.Subject && f.Rule.ID == "MX40-CONTEXT-measure-repeat-content" {
						if f.Outcome != validation.OutcomeUnknown || f.Reason != "prerequisite-document-incomplete" || f.Cause != nil {
							t.Errorf("consumer=%+v", f)
						}
						foundConsumer = true
					}
				}
				if !foundConsumer {
					t.Error("missing document-prerequisite consumer")
				}
				// Each caller-owned evidence field retains an independent cause snapshot.
				if c.Repeat.NoteScopesComplete.Cause != nil && c.Repeat.ContentEquivalent.Cause != nil {
					c.Repeat.NoteScopesComplete.Cause.Subject.Node = 999999
					if !reflect.DeepEqual(c.Repeat.ContentEquivalent.Cause, cause) {
						t.Error("scope cause mutation changed content cause")
					}
				}
			}
			if starts != 1 {
				t.Fatalf("starts=%d", starts)
			}
		})
	}
}

func TestRound3RepeatAbsentStopTrailingForwardExtent(t *testing.T) {
	for _, meter := range []string{"", `<time><beats>4</beats><beat-type>4</beat-type></time>`} {
		for _, padding := range []string{"", `<forward><duration>2</duration></forward>`} {
			t.Run(map[bool]string{false: "no-meter", true: "4-4"}[meter != ""]+"/"+map[bool]string{false: "notes-only", true: "forward"}[padding != ""], func(t *testing.T) {
				body := `<attributes><divisions>1</divisions>` + meter + `</attributes>` + repeatR1Note + `<attributes><measure-style><beat-repeat type="start"><slash-type>quarter</slash-type></beat-repeat></measure-style></attributes>` + strings.Repeat(repeatR1Note, 3) + padding
				repeatAssessBoth(t, body, func(t *testing.T, r validation.Report) {
					repeatR1Content(t, r, validation.EvidenceKnown, padding == "")
					fail := false
					for _, f := range r.Findings {
						if f.Reason == "context-repeat-encoded-content-missing" {
							fail = true
							if f.Outcome != validation.OutcomeFail || f.Rule.ID != "MX40-CONTEXT-beat-repeat-content" {
								t.Errorf("wrong owner %+v", f)
							}
						}
					}
					if fail != (padding != "") {
						t.Errorf("consumer failure=%v padding=%q", fail, padding)
					}
				})
			})
		}
	}
}

func TestRound3RepeatTerminalRangeBoundaries(t *testing.T) {
	start := `<attributes><divisions>1</divisions></attributes>` + repeatR1Note + `<attributes><measure-style><beat-repeat type="start"><slash-type>quarter</slash-type></beat-repeat></measure-style></attributes>` + strings.Repeat(repeatR1Note, 3)
	for _, tc := range []struct {
		name, suffix string
		state        validation.EvidenceState
		value        bool
	}{
		{"ordinary-terminal-note", "", validation.EvidenceKnown, true},
		{"forward-beyond-witness", `<forward><duration>2</duration></forward>`, validation.EvidenceKnown, false},
		{"forward-independent-destination-witness", `<forward><duration>2</duration></forward><direction><direction-type><words>cursor witness</words></direction-type></direction>`, validation.EvidenceKnown, false},
		{"forward-explicit-stop", `<forward><duration>2</duration></forward><attributes><measure-style><beat-repeat type="stop"/></measure-style></attributes>`, validation.EvidenceKnown, false},
		{"backup-and-padded-second-voice", `<backup><duration>4</duration></backup><forward><duration>4</duration><voice>2</voice></forward>`, validation.EvidenceKnown, true},
		{"backup-does-not-extend", `<backup><duration>4</duration></backup>`, validation.EvidenceKnown, true},
		{"nonintegral-terminal-range", `<forward><duration>0.5</duration></forward>`, validation.EvidenceUnknown, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repeatAssessBoth(t, start+tc.suffix, func(t *testing.T, r validation.Report) { repeatR1Content(t, r, tc.state, tc.value) })
		})
	}
	t.Run("empty-final-measure", func(t *testing.T) {
		repeatRound1Score(t, []string{start, ""}, nil, func(t *testing.T, r validation.Report) { repeatR1Content(t, r, validation.EvidenceUnknown, false) })
	})
}
