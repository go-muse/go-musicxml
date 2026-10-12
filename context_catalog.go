package musicxml

import (
	"cmp"
	"fmt"
	"slices"
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileModelContext selects the bounded current-model context extension of
// the frozen timeline profile. Context facts are caller-owned report snapshots;
// unavailable optional facts do not become invented conformance failures.
const ProfileModelContext validation.ProfileID = "musicxml-4.0/model-context"

// ProfileSourceContext selects the corresponding original-source context
// assessment, preserving existing source syntax and unread external-DTD limits.
const ProfileSourceContext validation.ProfileID = "musicxml-4.0/source-context"

var contextAssessmentEngine = sync.OnceValues(buildContextAssessmentEngine)

func newContextAssessmentEngine() (*validation.Engine, error) { return contextAssessmentEngine() }

func buildContextAssessmentEngine() (*validation.Engine, error) {
	d, err := buildTimelineAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	for i := range d.profiles {
		p := &d.profiles[i]
		if p.Target == validation.TargetModel {
			p.ID = ProfileModelContext
		} else {
			p.ID = ProfileSourceContext
		}
		p.Required = append([]validation.RuleKey(nil), p.Required...)
		p.Support = append([]validation.RuleSupport(nil), p.Support...)
		p.NormativeClosure = "Explicit bounded MusicXML 4.0 source or current-model XSD/local/timeline and contextual assessment. New sealed context families produce independently owned typed scalar/default, span, graphical-repeat, navigation, layout and timing evidence. A completed provider means extraction/projection finished; it does not assert every optional fact or ambiguous correspondence is known. Mandatory consumers compare numerical time-only order, release sign for established bend gestures, and proved graphical-repeat content. Equal-adjacent passes, unresolved musical-content equivalence, and the unreviewed figure-number numeric grammar remain explicitly incomplete. Type-specific identity, presence, applicability, causal unavailability and exact values are preserved without a universal span-balance, meter-fill or rendering requirement. Source/current-model representation and unread external-DTD limits remain active. Unproved author intent, playback realization, application geometry, ambiguous navigation policies and precise unresolved source clauses retain bounded unavailable evidence. Packages, linked resources, external dictionaries and transitive normative closure remain excluded. Opus assessment projects only local score pagination hints, preserves application-dependent absence, and never follows linked scores. Rule and fact counts are not MusicXML coverage denominators."
	}
	schema := d.schemas[validation.AssemblyMusicXML]
	timeline := validation.RuleKey{ID: "MX40-TIMELINE-effective-state", Revision: 1}
	spec := validation.ContextPlanSpec{Schema: schema, TimelineProvider: timeline}
	assembly := validation.AssemblyMusicXML
	first := len(d.rules)
	families := []struct {
		id, title, explanation string
		sourceIDs              []string
		line                   int
		timeline               bool
		plan                   func() (validation.Plan, error)
	}{
		{"local", "Local contextual defaults and applicability", "Project reviewed use-specific defaults, inheritance and applicability from exact source or current-model evidence; unknown presence never activates a default.", []string{"MX40-PROSE-printout-conditional-default", "MX40-PROSE-direction-format-inheritance", "MX40-PROSE-lyric-defaults"}, 2160, false, func() (validation.Plan, error) { return validation.NewLocalContextPlan(schema) }},
		{"spans", "Type-specific span correspondence", "Retain document-overlap identity separately from musical endpoint roles, including implied visual ties, beams, tuplets, lyrics and part-list intervals; no universal balance or staff/voice partition.", []string{"MX40-PROSE-number-level-overlap", "MX40-PROSE-tied-number-implied", "MX40-PROSE-part-group-boundaries"}, 287, true, func() (validation.Plan, error) { return validation.NewSpanContextPlan(spec) }},
		{"graphical-repeats", "Graphical repeat ranges", "Project beat/measure/slash display scopes and proven content relations without synthesizing missing music or using XML strings as musical equality.", []string{"MX40-PROSE-beat-repeat-content", "MX40-PROSE-measure-repeat-content", "MX40-PROSE-slash-start-stop"}, 2907, true, func() (validation.Plan, error) { return validation.NewRepeatContextPlan(spec) }},
		{"navigation", "Bounded repeat navigation and pass context", "Keep typed navigation targets, exact repeat/pass evidence and proven traversal separate from graphical repetition; unresolved edges and limits are not MusicXML violations.", []string{"MX40-PROSE-repeat-backward-attributes", "MX40-PROSE-sound-jump-passes", "MX40-PROSE-unknown-ending-number"}, 4125, true, func() (validation.Plan, error) { return validation.NewNavigationContextPlan(spec) }},
		{"layout", "Scoped layout applicability", "Project score defaults and page/system/measure-local overrides, preserving first-visible-staff and geometry availability without constructing a full engraver.", []string{"MX40-PROSE-staff-layout-scope", "MX40-PROSE-system-layout-scope", "MX40-PROSE-page-layout-fallback"}, 4479, true, func() (validation.Plan, error) { return validation.NewLayoutContextPlan(spec) }},
		{"performance", "Encoded performance and timing context", "Project documented performance defaults and exact accepted timing facts, keeping cue silence, swing eligibility, signed adjustments and intent-dependent realization distinct.", []string{"MX40-PROSE-cue-silent", "MX40-PROSE-swing-applicability", "MX40-PROSE-grace-time-semantics", "MX40-PROSE-transpose-additive"}, 4214, true, func() (validation.Plan, error) { return validation.NewPerformanceContextPlan(spec) }},
	}
	for _, f := range families {
		plan, err := f.plan()
		if err != nil {
			return nil, err
		}
		key := validation.RuleKey{ID: validation.RuleID("MX40-CONTEXT-" + f.id), Revision: 1}
		r := validation.RuleDefinition{Key: key, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Scope: validation.ScopeDocument, Selector: validation.Selector{Assembly: &assembly}, Predicate: string(key.ID), Text: validation.RuleText{Title: f.title, Explanation: f.explanation, Message: "The selected contextual fact service did not finish."}, Sources: []validation.SourceRef{{IDs: f.sourceIDs, URL: localSchemaSource(f.line), Locator: "Reviewed type-specific context clauses"}}}
		if f.timeline {
			r.Requires = []validation.Dependency{{Kind: validation.DependencyRule, Rule: timeline, Relation: validation.RelationDocument}}
		}
		d.rules = append(d.rules, r)
		d.plans = append(d.plans, validation.PlanBinding{ID: r.Predicate, Plan: plan})
	}

	opusAssembly := validation.AssemblyOpus
	opusPlan, err := validation.NewOpusLayoutContextPlan(d.schemas[opusAssembly])
	if err != nil {
		return nil, err
	}
	opusKey := validation.RuleKey{ID: "MX40-CONTEXT-opus-pagination", Revision: 1}
	opusRule := validation.RuleDefinition{Key: opusKey, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Scope: validation.ScopeDocument, Selector: validation.Selector{Assembly: &opusAssembly}, Predicate: string(opusKey.ID), Text: validation.RuleText{Title: "Local Opus pagination context", Explanation: "Retain explicit score new-page hints and application-dependent absence within this Opus only; never follow linked scores.", Message: "Local Opus pagination projection did not finish."}, Sources: []validation.SourceRef{{IDs: []string{"MX40-PROSE-opus-new-page-default"}, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/opus.xsd#L92", Locator: "Opus score new-page application default"}}}
	d.rules = append(d.rules, opusRule)
	d.plans = append(d.plans, validation.PlanBinding{ID: opusRule.Predicate, Plan: opusPlan})
	for _, consumer := range []struct {
		id, subject, title, explanation, provider string
		line                                      int
		makePlan                                  func(validation.ContextConsumerSpec) (validation.Plan, error)
	}{
		{"bend-release-negative", "bend", "Negative release distance after an established bend", "For an established release-after-bend gesture the exact bend-alter must be negative. Lone, pre-bend or ambiguous gestures do not establish this antecedent.", "performance", 4744, validation.NewBendReleaseSignPlan},
		{"beat-repeat-content", "beat-repeat", "Encoded music for a graphical beat repeat", "A graphical repeat does not supply missing musical events. Consume only proved scoped content correspondence; unresolved timing or equivalence remains incomplete.", "graphical-repeats", 2907, validation.NewGraphicalRepeatContentPlan},
		{"measure-repeat-content", "measure-repeat", "Encoded music for a graphical measure repeat", "Repeated musical content must be encoded in the affected measures. Exact supported content proofs are separate from unspecified broader event-equivalence policies.", "graphical-repeats", 3048, validation.NewGraphicalRepeatContentPlan},
	} {
		provider := validation.RuleKey{ID: validation.RuleID("MX40-CONTEXT-" + consumer.provider), Revision: 1}
		plan, err := consumer.makePlan(validation.ContextConsumerSpec{Schema: schema, Provider: provider})
		if err != nil {
			return nil, err
		}
		key := validation.RuleKey{ID: validation.RuleID("MX40-CONTEXT-" + consumer.id), Revision: 1}
		rule := validation.RuleDefinition{Key: key, Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Scope: validation.ScopeNode, Selector: localType(consumer.subject), Predicate: string(key.ID), Requires: []validation.Dependency{{Kind: validation.DependencyRule, Rule: provider, Relation: validation.RelationDocument}}, Text: validation.RuleText{Title: consumer.title, Explanation: consumer.explanation, Message: "The selected contextual condition was not established."}, Sources: []validation.SourceRef{{IDs: []string{"MX40-PROSE-" + consumer.id}, URL: localSchemaSource(consumer.line), Locator: "Reviewed conditional contextual predicate"}}}
		d.rules = append(d.rules, rule)
		d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
	}

	figurePlan, err := validation.NewFigureNumberNumericPlan(schema)
	if err != nil {
		return nil, err
	}
	figureCount := 0
	for _, use := range schema.Uses() {
		if use.Attribute || !use.OwnerTypeKnown || use.OwnerType != *localType("figure").Component || use.Name.Local != "figure-number" || use.Name.Space != "" {
			continue
		}
		figureCount++
		declaration := use.Declaration
		key := validation.RuleKey{ID: "MX40-CONTEXT-figure-number-numeric", Revision: 1}
		rule := validation.RuleDefinition{Key: key, Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Scope: validation.ScopeNode, Selector: validation.Selector{Declaration: &declaration}, Predicate: string(key.ID), Text: validation.RuleText{Title: "Figure-number numeric content", Explanation: "The source requires numeric content but a reviewed lexical grammar is unavailable. Present valid style-text remains explicitly unsupported rather than silently passing or applying an invented number syntax.", Message: "The figure-number numeric grammar is not established."}, Sources: []validation.SourceRef{{IDs: []string{"MX40-PROSE-figure-number-numeric"}, URL: localSchemaSource(4836), Locator: "figure-number numeric content clause"}}}
		d.rules = append(d.rules, rule)
		d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: figurePlan})
	}
	if figureCount != 1 {
		return nil, fmt.Errorf("context figure-number: expected one exact declaration, got %d", figureCount)
	}
	// Type ownership is derived once from the authoritative compiled schema;
	// each selected exact attribute use is still checked by the sealed plan.
	owners := map[validation.ComponentID]bool{}
	for _, u := range schema.Uses() {
		if u.Attribute && u.DeclaredTypeKnown && u.DeclaredType == validation.ComponentID(*localType("time-only").Component) && u.OwnerTypeKnown {
			owners[u.OwnerType] = true
		}
	}
	order := make([]validation.ComponentID, 0, len(owners))
	for owner := range owners {
		order = append(order, owner)
	}
	slices.SortFunc(order, func(a, b validation.ComponentID) int { return cmp.Compare(a.Name.Local, b.Name.Local) })
	plan, err := validation.NewTimeOnlyOrderPlan(schema)
	if err != nil {
		return nil, err
	}
	for _, owner := range order {
		key := validation.RuleKey{ID: validation.RuleID("MX40-CONTEXT-time-only-order/" + owner.Name.Local), Revision: 1}
		r := validation.RuleDefinition{Key: key, Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Scope: validation.ScopeNode, Selector: validation.Selector{Component: &owner}, Predicate: string(key.ID), Text: validation.RuleText{Title: "Numerical time-only pass order", Explanation: "Present time-only values are compared as exact positive integers independently of repeat reachability. A descending adjacent pair fails; equal adjacent passes retain the unresolved duplicate policy.", Message: "The time-only pass sequence is not proven ascending."}, Sources: []validation.SourceRef{{IDs: []string{"MX40-PROSE-time-only-ascending"}, URL: localSchemaSource(583), Locator: "time-only numerical ascending clause"}}}
		d.rules = append(d.rules, r)
		d.plans = append(d.plans, validation.PlanBinding{ID: r.Predicate, Plan: plan})
	}
	for i := range d.profiles {
		for _, r := range d.rules[first:] {
			d.profiles[i].Required = append(d.profiles[i].Required, r.Key)
			d.profiles[i].Support = append(d.profiles[i].Support, validation.RuleSupport{Rule: r.Key, State: validation.ImplementationAvailable})
		}
	}
	c, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		return nil, err
	}
	return validation.NewEngine(c)
}
