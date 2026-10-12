package musicxml

import (
	"context"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

// This measures the actual shared account with individually selected sealed
// family providers. It is an incremental cost diagnostic, not a public profile
// or a normative scope claim; ingress and the accepted timeline remain selected.
func TestIndependentContextFamilyCostAttribution(t *testing.T) {
	doc, err := Decode(strings.NewReader(independentTimelineScore(100, false, "1")))
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range []string{"timeline", "local", "spans", "graphical-repeats", "navigation", "layout", "performance"} {
		t.Run(family, func(t *testing.T) {
			d, err := buildTimelineAssessmentDefinitions()
			if err != nil {
				t.Fatal(err)
			}
			profile := d.profiles[0]
			for _, p := range d.profiles {
				if p.Target == validation.TargetModel {
					profile = p
				}
			}
			if family != "timeline" {
				schema := d.schemas[validation.AssemblyMusicXML]
				provider := validation.RuleKey{ID: "MX40-TIMELINE-effective-state", Revision: 1}
				spec := validation.ContextPlanSpec{Schema: schema, TimelineProvider: provider}
				var plan validation.Plan
				switch family {
				case "local":
					plan, err = validation.NewLocalContextPlan(schema)
				case "spans":
					plan, err = validation.NewSpanContextPlan(spec)
				case "graphical-repeats":
					plan, err = validation.NewRepeatContextPlan(spec)
				case "navigation":
					plan, err = validation.NewNavigationContextPlan(spec)
				case "layout":
					plan, err = validation.NewLayoutContextPlan(spec)
				case "performance":
					plan, err = validation.NewPerformanceContextPlan(spec)
				}
				if err != nil {
					t.Fatal(err)
				}
				assembly := validation.AssemblyMusicXML
				key := validation.RuleKey{ID: validation.RuleID("audit-" + family), Revision: 1}
				rule := validation.RuleDefinition{Key: key, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetModel}, Scope: validation.ScopeDocument, Selector: validation.Selector{Assembly: &assembly}, Predicate: string(key.ID), Text: validation.RuleText{Title: "audit", Explanation: "audit", Message: "audit"}}
				rule.Sources = []validation.SourceRef{{IDs: []string{"MX40-PROSE-printout-conditional-default"}, URL: localSchemaSource(2160), Locator: "Independent family cost attribution of sealed production plan"}}
				if family != "local" {
					rule.Requires = []validation.Dependency{{Kind: validation.DependencyRule, Rule: provider, Relation: validation.RelationDocument}}
				}
				d.rules = append(d.rules, rule)
				d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
				profile.Required = append(profile.Required, key)
				profile.Support = append(profile.Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
			}
			catalog, err := validation.NewCatalog(d.rules, []validation.Profile{profile}, d.plans)
			if err != nil {
				t.Fatal(err)
			}
			engine, err := validation.NewEngine(catalog)
			if err != nil {
				t.Fatal(err)
			}
			account, err := validation.NewBudgetAccount(validation.Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
			if err != nil {
				t.Fatal(err)
			}
			session, err := engine.Begin(context.Background(), validation.RunOptions{Profile: profile.ID, DocumentID: modelDocumentID, Account: account})
			if err != nil {
				t.Fatal(err)
			}
			_, walkErr := walkModel(context.Background(), doc, session, account, modelDocumentID, modelWalkOptions{})
			if walkErr != nil {
				t.Fatal(walkErr)
			}
			ingress, _ := account.Usage(validation.BudgetRuleSteps)
			buffer, _ := account.Usage(validation.BudgetBufferedBytes)
			report, err := session.Finish(context.Background(), validation.Completion{InputComplete: true, Syntax: validation.SyntaxNotAssessed})
			if err != nil || !report.Satisfied() {
				t.Fatalf("%v %s", err, localFindings(report))
			}
			steps, _ := account.Usage(validation.BudgetRuleSteps)
			facts, _ := account.Usage(validation.BudgetContextFacts)
			remaining, _ := account.Usage(validation.BudgetBufferedBytes)
			t.Logf("ingress_steps=%d total_steps=%d finalization_steps=%d context_facts=%d ingress_retention=%d post_finish_retention=%d projected_records=%d", ingress, steps, steps-ingress, facts, buffer, remaining, len(report.Context))
			if remaining != 0 {
				t.Fatal("session retained account after Finish")
			}
		})
	}
}

func TestIndependentContextOrdinaryHundredNotesDefaultWork(t *testing.T) {
	source := independentTimelineScore(100, false, "1")
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []bool{false, true} {
		var r validation.Report
		if model {
			r, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelContext})
		} else {
			r, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceContext})
		}
		if err != nil || !r.Satisfied() {
			t.Errorf("ordinary 100-note context fixture exceeds unchanged finite defaults (model=%t): %v", model, err)
		}
	}
}

func TestIndependentContextFullSharedAccountCost(t *testing.T) {
	engine, err := newContextAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{100, 1000} {
		doc, e := Decode(strings.NewReader(independentTimelineScore(count, false, "1")))
		if e != nil {
			t.Fatal(e)
		}
		for _, retention := range []int64{64 << 20, 512 << 20} {
			account, e := validation.NewBudgetAccount(validation.Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: retention})
			if e != nil {
				t.Fatal(e)
			}
			session, e := engine.Begin(context.Background(), validation.RunOptions{Profile: ProfileModelContext, DocumentID: modelDocumentID, Account: account})
			if e != nil {
				t.Fatal(e)
			}
			_, walkErr := walkModel(context.Background(), doc, session, account, modelDocumentID, modelWalkOptions{})
			if walkErr != nil {
				t.Fatal(walkErr)
			}
			r, finishErr := session.Finish(context.Background(), validation.Completion{InputComplete: true, Syntax: validation.SyntaxNotAssessed})
			steps, _ := account.Usage(validation.BudgetRuleSteps)
			facts, _ := account.Usage(validation.BudgetContextFacts)
			remaining, _ := account.Usage(validation.BudgetBufferedBytes)
			t.Logf("notes=%d retention_limit=%d complete=%t steps=%d facts=%d records=%d post_finish_retention=%d err=%v", count, retention, r.AssessmentComplete, steps, facts, len(r.Context), remaining, finishErr)
			if remaining != 0 {
				t.Fatal("cleanup leaked retained account")
			}
			if retention == 512<<20 && (finishErr != nil || !r.Satisfied()) {
				t.Fatalf("raised-limit reference did not complete: %v", finishErr)
			}
		}
	}
}
