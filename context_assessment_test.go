package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestContextProfilesTimeOnlyAcrossTargetsAndHierarchies(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		for _, tc := range []struct {
			value   string
			outcome validation.Outcome
		}{
			{"2,10", validation.OutcomePass}, {"1,10,2", validation.OutcomeFail},
			{"18446744073709551616,18446744073709551617", validation.OutcomePass},
			{"18446744073709551617,18446744073709551616", validation.OutcomeFail},
			{"1,1", validation.OutcomeUnknown}, {"2,2,1", validation.OutcomeFail},
		} {
			body := `<note time-only="` + tc.value + `"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`
			source := contextFixtureXML(body, timewise)
			doc, err := Decode(strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			for _, model := range []bool{false, true} {
				var report validation.Report
				if model {
					report, err = AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelContext})
				} else {
					report, err = InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: ProfileSourceContext})
				}
				if err != nil {
					t.Fatalf("timewise=%t model=%t value=%s: %v", timewise, model, tc.value, err)
				}
				found := false
				for _, f := range report.Findings {
					if strings.HasPrefix(string(f.Rule.ID), "MX40-CONTEXT-time-only-order/") {
						if f.Outcome != tc.outcome {
							t.Errorf("%s got %s want%s", tc.value, f.Outcome, tc.outcome)
						}
						found = true
					}
				}
				if tc.outcome == validation.OutcomePass {
					if found {
						t.Errorf("unexpected ordering finding %s", localFindings(report))
					}
				} else if !found {
					t.Errorf("missing ordering outcome %s: %s", tc.value, localFindings(report))
				}
			}
		}
	}
}

func TestContextProfilesDoNotBroadenEarlierProfiles(t *testing.T) {
	source := (localFixture{body: `<note time-only="10,2"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`}).xml()
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	if err = Validate(doc); err != nil {
		t.Fatalf("legacy changed:%v", err)
	}
	for _, id := range []validation.ProfileID{ProfileModelXSD.ID(), ProfileModelLocal.ID(), ProfileModelTimeline.ID()} {
		report, e := AssessModel(context.Background(), doc, ModelOptions{Profile: ModelProfile(id)})
		if e != nil || !report.Satisfied() {
			t.Errorf("old profile%s:%v %s", id, e, localFindings(report))
		}
	}
	for _, id := range []validation.ProfileID{ProfileSourceXSDPreview.ID(), ProfileSourceLocal.ID(), ProfileSourceTimeline.ID()} {
		report, e := InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: SourceProfile(id)})
		if e != nil || !report.Satisfied() {
			t.Errorf("old profile%s:%v %s", id, e, localFindings(report))
		}
	}
}

func contextFixtureXML(body string, timewise bool) string {
	header := `<part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>`
	if timewise {
		return `<score-timewise version="4.0">` + header + `<measure number="A"><part id="P1">` + body + `</part></measure></score-timewise>`
	}
	return `<score-partwise version="4.0">` + header + `<part id="P1"><measure number="A">` + body + `</measure></part></score-partwise>`
}

func TestContextTimeOnlyInvalidPrerequisiteAndStrictDecode(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		for _, value := range []string{"0,1", "1,,2"} {
			source := contextFixtureXML(`<note time-only="`+value+`"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`, timewise)
			doc, err := Decode(strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			for _, model := range []bool{false, true} {
				var r validation.Report
				if model {
					r, err = AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelContext})
				} else {
					r, err = InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Profile: ProfileSourceContext})
				}
				if err != nil {
					t.Fatal(err)
				}
				blocked, failed := false, false
				for _, f := range r.Findings {
					if strings.HasPrefix(string(f.Rule.ID), "MX40-CONTEXT-time-only-order/") {
						if f.Outcome != validation.OutcomeBlocked || f.Cause == nil {
							t.Errorf("%s ordering prerequisite=%+v", value, f)
						}
						blocked = true
					}
					if strings.HasPrefix(string(f.Rule.ID), "MX40-XSD-") && f.Outcome == validation.OutcomeFail {
						failed = true
					}
				}
				if !blocked || !failed {
					t.Errorf("%s model%t/timewise%t missing causal shared-XSD result: %s", value, model, timewise, localFindings(r))
				}
			}
		}
	}

	source := contextFixtureXML(`<note time-only="10,2"><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration></note>`, false)
	result, err := DecodeAssessed(context.Background(), strings.NewReader(source), DecodeAssessmentOptions{Assessment: &SourceOptions{Profile: ProfileSourceLine}})
	if err == nil || result.Document != nil {
		t.Fatalf("strict context returned rejected model: %+v,%v", result, err)
	}
}

// The historical gate survives only in an explicitly constructed legacy
// catalog. It is no longer selected by the built-in conformance profiles.
func TestContextFigureNumberRetainsUnreviewedGrammarGate(t *testing.T) {
	engine := legacyFigureNumberTestEngine(t)
	for _, timewise := range []bool{false, true} {
		for _, value := range []string{"6", "10", "Ⅵ", "6½", "not-a-number"} {
			xml := contextFixtureXML(`<figured-bass><figure><figure-number>`+value+`</figure-number></figure></figured-bass>`, timewise)
			for _, model := range []bool{false, true} {
				r := localContextAssess(t, engine, xml, model)
				found := false
				for _, f := range r.Findings {
					if f.Rule.ID == "MX40-CONTEXT-figure-number-numeric" {
						found = true
						if f.Outcome != validation.OutcomeUnsupported {
							t.Errorf("%q: %+v", value, f)
						}
					}
				}
				if value == "6" || value == "10" {
					if found {
						t.Errorf("sufficient numeric subset stayed unavailable: %q %s", value, localFindings(r))
					}
				} else if !found || r.Satisfied() {
					t.Errorf("figure grammar silently closed: %q %s", value, localFindings(r))
				}
			}
		}
	}
}

func TestContextFigureNumberAbsentDoesNotCreateGrammarObligation(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		xml := contextFixtureXML(`<figured-bass><figure><prefix>sharp</prefix></figure></figured-bass>`, timewise)
		r, err := InspectXML(context.Background(), strings.NewReader(xml), SourceOptions{Profile: ProfileSourceContext})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range r.Findings {
			if f.Rule.ID == "MX40-CONTEXT-figure-number-numeric" {
				t.Fatalf("absent optional node created grammar obligation: %+v", f)
			}
		}
	}
}

func TestContextProfilesKeepApplicationChoicesExcluded(t *testing.T) {
	engine, err := newContextAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []validation.ProfileID{ProfileSourceContext.ID(), ProfileModelContext.ID()} {
		profile := catalogProfile(t, engine, id)
		for _, key := range profile.Required {
			text := string(key.ID)
			if strings.Contains(text, "binding-workaround") || strings.Contains(text, "sounds-application-mapping") {
				t.Errorf("application implementation advice promoted to document rule: %s", text)
			}
		}
		if !strings.Contains(profile.NormativeClosure, "external dictionaries") || !strings.Contains(profile.NormativeClosure, "physical") && !strings.Contains(profile.NormativeClosure, "playback realization") {
			t.Error("external/application scope boundary disappeared")
		}
	}
}
