package musicxml

import "github.com/go-muse/go-musicxml/validation"

// Added only by new explicit completion compositions. Frozen context catalogs
// and their binding/profile digests deliberately do not select this predicate.
func addBarlineLocationAssessment(d *assessmentDefinitions) (validation.RuleKey, error) {
	timeline := validation.RuleKey{ID: "MX40-TIMELINE-effective-state", Revision: 1}
	key := validation.RuleKey{ID: "MX40-CONTEXT-barline-location", Revision: 1}
	plan, err := validation.NewBarlineLocationPlan(validation.ContextPlanSpec{Schema: d.schemas[validation.AssemblyMusicXML], TimelineProvider: timeline})
	if err != nil {
		return validation.RuleKey{}, err
	}
	rule := validation.RuleDefinition{Key: key, Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Scope: validation.ScopeNode, Selector: localType("barline"), Predicate: string(key.ID), Requires: []validation.Dependency{{Kind: validation.DependencyRule, Rule: timeline, Relation: validation.RelationDocument}}, Text: validation.RuleText{Title: "Barline musical location", Explanation: "Compare the unshifted cursor with the structural measure origin and independently certified terminal-note or restoring-forward end. Ordered note/backup/forward placement guards structural-left passes and right lower-bound violations; trailing unequal-voice cursors remain unavailable. XML first/last placement remains a recommendation; meter, implicit numbering and arbitrary forward maxima do not supply an endpoint.", Message: "The barline location does not agree with its musical position."}, Sources: []validation.SourceRef{{IDs: []string{"MX40-PROSE-barline-location"}, URL: localSchemaSource(3227), Locator: "Mandatory location agreement; first/last placement recommendation"}, {IDs: []string{"MX40-PROSE-barline-location"}, URL: "https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/barline/", Locator: "Location agreement with ordered musical data; recommended first/last placement"}, {IDs: []string{"MX40-PROSE-barline-location"}, URL: "https://www.w3.org/2021/06/musicxml40/tutorial/midi-compatible-part/", Locator: "Backup and forward restoration between voices"}}}
	d.rules = append(d.rules, rule)
	d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
	return key, nil
}
