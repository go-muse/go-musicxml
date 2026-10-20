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
				options := dictionaryAssessmentOptions{Assessment: assessmentOptions{Profile: profile}}
				var report validation.Report
				var err error
				if target == validation.TargetSource {
					report, err = inspectXMLWithDictionaries(context.Background(), strings.NewReader(input), options)
				} else {
					doc, e := Decode(strings.NewReader(input))
					if e != nil {
						t.Fatal(e)
					}
					report, err = assessModelWithDictionaries(context.Background(), doc, options)
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
				for _, id := range []validation.RuleID{"MX40-DICTIONARY-musicxml/standard-sounds", "MX40-DICTIONARY-musicxml/rendering"} {
					if selected[id] {
						t.Errorf("optional dictionary observation selected: %s", id)
					}
				}
			})
		}
	}
}

func TestSemanticCompositionEffectiveDTDBarline(t *testing.T) {
	const profile validation.ProfileID = "musicxml-4.0/source-semantics"
	input := `<!DOCTYPE score-partwise [<!ATTLIST barline location CDATA "left">]>` + barlineLocationScore(false, barlineDivisions+barlineNote("1")+`<barline/>`, "")
	report, err := InspectXML(context.Background(), strings.NewReader(input), SourceOptions{Profile: SourceProfile(profile)})
	if err != nil || report.Conformance != validation.ConformanceViolated {
		t.Fatalf("defaulted wrong location not assessed: %v %+v", err, report.Findings)
	}
	foundFailure, foundOrigin := false, false
	for _, finding := range report.Findings {
		if finding.Rule.ID == "MX40-CONTEXT-barline-location" && finding.Outcome == validation.OutcomeFail {
			foundFailure = true
		}
	}
	for _, row := range report.Context {
		if o := row.SourceOrigin; o != nil && o.Attribute != nil && o.Attribute.Local == "location" && o.Specified.State == validation.EvidenceKnown && !o.Specified.Value && o.Physical.State == validation.EvidenceAbsent {
			foundOrigin = true
		}
	}
	if !foundFailure || !foundOrigin {
		t.Fatalf("effective value and physical absence not both retained: failure=%v origin=%v", foundFailure, foundOrigin)
	}
}

func TestSemanticCompositionRejectsWrongTargetBeforeDictionaryIO(t *testing.T) {
	calls := 0
	provider := dictionaryImportSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
		calls++
		return DictionaryComponentResult{}, nil
	})
	options := dictionaryAssessmentOptions{Assessment: assessmentOptions{Profile: "musicxml-4.0/model-semantics"}, Dictionaries: DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical)}, Source: provider}}
	reader := &assessmentCountingReader{source: strings.NewReader(assessedScore("1", ""))}
	if _, err := inspectXMLWithDictionaries(context.Background(), reader, options); err == nil {
		t.Fatal("wrong source profile accepted")
	}
	doc, err := Decode(strings.NewReader(assessedScore("1", "")))
	if err != nil {
		t.Fatal(err)
	}
	options.Assessment.Profile = "musicxml-4.0/source-semantics"
	if _, err = assessModelWithDictionaries(context.Background(), doc, options); err == nil {
		t.Fatal("wrong model profile accepted")
	}
	if calls != 0 || reader.bytes != 0 {
		t.Fatalf("invalid target performed I/O: provider=%d bytes=%d", calls, reader.bytes)
	}
}
