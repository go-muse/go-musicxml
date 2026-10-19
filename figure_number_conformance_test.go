package musicxml

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// MusicXML deliberately retained style-text rather than the proposed integer
// restriction: https://github.com/w3c-cg/musicxml/issues/94#issuecomment-290221733.
// These checks establish conformance to that open text declaration, not numeric
// meaning for arbitrary notation.
func TestFigureNumberUnrestrictedTextConformance(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		for _, value := range []string{"6", "10", "0006", "0", "+6", "-6", "6.0", "6/5", "6½", "VI", "Ⅵ", "٦", "６", "not-a-number", "", " ", "\t\n", " 6 "} {
			source := contextFixtureXML(`<figured-bass><figure><prefix>sharp</prefix><figure-number>`+value+`</figure-number><suffix>slash</suffix></figure></figured-bass>`, timewise)
			doc, err := Decode(strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			for _, model := range []bool{false, true} {
				t.Run(fmt.Sprintf("timewise=%t/model=%t/text=%q", timewise, model, value), func(t *testing.T) {
					var report validation.Report
					if model {
						report, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
					} else {
						report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceContext})
					}
					if err != nil || !report.Satisfied() || !report.AssessmentComplete || report.Counts.Unsupported != 0 {
						t.Fatalf("unrestricted style-text incurred a conformance penalty: %v; %s", err, localFindings(report))
					}
					assertNoFigureNumberNumericRequirement(t, report)
					textPreserved := false
					for _, row := range report.Context {
						if row.Property == validation.ContextFigureNumber {
							textPreserved = row.Value.State == validation.EvidenceKnown && row.Value.Value.Kind == validation.AtomicString && row.Value.Value.Text == value
						}
					}
					if !textPreserved {
						t.Error("unrestricted figure text was lost or normalized as a number")
					}
				})
			}
		}
	}
}

func TestFigureNumberUnrestrictedTextPreservesXSDConstraints(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		source := contextFixtureXML(`<figured-bass><figure><figure-number font-style="invalid">Ⅵ</figure-number></figure></figured-bass>`, timewise)
		doc, err := Decode(strings.NewReader(source))
		if err != nil {
			t.Fatal(err)
		}
		for _, model := range []bool{false, true} {
			var report validation.Report
			if model {
				report, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
			} else {
				report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceContext})
			}
			if err != nil || report.Satisfied() || report.Conformance != validation.ConformanceViolated {
				t.Fatalf("invalid style-text attribute accepted: model=%t timewise=%t: %v; %s", model, timewise, err, localFindings(report))
			}
			failed := false
			for _, finding := range report.Findings {
				failed = failed || strings.HasPrefix(string(finding.Rule.ID), "MX40-XSD-") && finding.Outcome == validation.OutcomeFail
			}
			if !failed {
				t.Fatalf("independent XSD cause lost: %s", localFindings(report))
			}
			assertNoFigureNumberNumericRequirement(t, report)
		}
	}
}

func TestFigureNumberUnrestrictedTextPreservesSourceDeclaration(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		source := contextFixtureXML(`<figured-bass><figure><figure-number xmlns="urn:other">Ⅵ</figure-number></figure></figured-bass>`, timewise)
		report, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceContext})
		if err != nil || report.Satisfied() || report.Conformance != validation.ConformanceViolated || report.Counts.Fail == 0 {
			t.Fatalf("invalid figure-number declaration accepted: %v; %s", err, localFindings(report))
		}
		assertNoFigureNumberNumericRequirement(t, report)
	}
}

func TestFigureNumberUnrestrictedTextStrictDecode(t *testing.T) {
	for _, timewise := range []bool{false, true} {
		source := contextFixtureXML(`<figured-bass><figure><figure-number>Ⅵ</figure-number></figure></figured-bass>`, timewise)
		result, err := DecodeAssessed(context.Background(), strings.NewReader(source), DecodeAssessmentOptions{Assessment: &AssessmentOptions{Profile: ProfileSourceSemantics}})
		if err != nil || result.Document == nil || !result.Source.Satisfied() || !result.Model.Satisfied() {
			t.Fatalf("valid open notation rejected by strict decode: %v", err)
		}
	}
}

func assertNoFigureNumberNumericRequirement(t *testing.T, report validation.Report) {
	t.Helper()
	const retired = "MX40-CONTEXT-figure-number-numeric"
	for _, rule := range report.Profile.Required {
		if rule.ID == retired {
			t.Error("built-in profile still selects the withdrawn numeric overconstraint")
		}
	}
	for _, support := range report.Profile.Support {
		if support.Rule.ID == retired {
			t.Error("built-in support still advertises the withdrawn numeric overconstraint")
		}
	}
	for _, finding := range report.Findings {
		if finding.Rule.ID == retired {
			t.Errorf("withdrawn numeric overconstraint produced a finding: %+v", finding)
		}
	}
}

// Retain the public legacy constructor's binding and outcomes without selecting
// it as a built-in MusicXML obligation. These local fixture profiles deliberately
// add the old caller-chosen overconstraint.
func legacyFigureNumberTestEngine(t *testing.T) *validation.Engine {
	t.Helper()
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	schema := d.schemas[validation.AssemblyMusicXML]
	plan, err := validationNewFigureNumberNumericPlan(schema)
	if err != nil {
		t.Fatal(err)
	}
	var declaration validation.DeclarationID
	found := 0
	for _, use := range schema.Uses() {
		if !use.Attribute && use.OwnerTypeKnown && use.OwnerType == *localType("figure").Component && use.Name == (validation.QName{Local: "figure-number"}) {
			declaration = use.Declaration
			found++
		}
	}
	if found != 1 {
		t.Fatalf("exact figure-number declarations: %d", found)
	}
	key := validation.RuleKey{ID: "MX40-CONTEXT-figure-number-numeric", Revision: 1}
	rule := validation.RuleDefinition{Key: key, Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Scope: validation.ScopeNode, Selector: validation.Selector{Declaration: &declaration}, Predicate: string(key.ID), Text: validation.RuleText{Title: "Legacy figure-number compatibility", Explanation: "Explicit caller-selected historical restriction beyond MusicXML conformance.", Message: "Text is outside the legacy ASCII-digit subset."}, Sources: []validation.SourceRef{{IDs: []string{"CHECK-figure-number-numeric"}, URL: "https://github.com/w3c-cg/musicxml/issues/94#issuecomment-290221733", Locator: "Compatibility test only; MusicXML deliberately retained unrestricted text"}}}
	d.rules = append(d.rules, rule)
	d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
	for i := range d.profiles {
		if d.profiles[i].ID == ProfileSourceLocal || d.profiles[i].ID == ProfileModelLocal {
			d.profiles[i].Required = append(d.profiles[i].Required, key)
			d.profiles[i].Support = append(d.profiles[i].Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
		}
	}
	catalog, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := validation.NewEngine(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}
