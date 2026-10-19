package musicxml

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileModelTimeline selects direct current-model XSD/local assessment and
// the explicitly bounded exact timeline/effective-state clauses. It is not an
// implicit default or a claim of complete MusicXML conformance.
const ProfileModelTimeline validation.ProfileID = "musicxml-4.0/model-timeline"

// ProfileSourceTimeline selects the corresponding original-source timeline
// assessment. Existing source/XSD and unread external-DTD limits still apply.
const ProfileSourceTimeline validation.ProfileID = "musicxml-4.0/source-timeline"

var timelineAssessmentEngine = sync.OnceValues(buildTimelineAssessmentEngine)

func newTimelineAssessmentEngine() (*validation.Engine, error) { return timelineAssessmentEngine() }

func buildTimelineAssessmentEngine() (*validation.Engine, error) {
	d, err := buildTimelineAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	catalog, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		return nil, err
	}
	return validation.NewEngine(catalog)
}

// Definition reuse composes a new explicit profile without changing this frozen
// timeline catalog, its semantic revision, or its normative closure.
func buildTimelineAssessmentDefinitions() (*assessmentDefinitions, error) {
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	var profiles []validation.Profile
	for _, previous := range d.profiles {
		if previous.ID != ProfileModelLocal && previous.ID != ProfileSourceLocal {
			continue
		}
		p := previous
		p.Required = append([]validation.RuleKey(nil), previous.Required...)
		p.Support = append([]validation.RuleSupport(nil), previous.Support...)
		if p.Target == validation.TargetModel {
			p.ID = ProfileModelTimeline
		} else {
			p.ID = ProfileSourceTimeline
		}
		p.NormativeClosure = "Explicit bounded MusicXML 4.0 source or current-model XSD, local clauses and shared exact musical timeline/effective-state assessment. The timeline provider finalizes typed part/measure/event and state evidence; its pass does not assert that every optional or ambiguous musical fact is known. The selected mandatory consumers are principal-note chord duration bounds and explicit numeral-key or applicable encoded effective-key presence at the harmony's unshifted score origin. Ordinary exact rational positions are not rounded. Ambiguous, missing, invalid or unsupported dependent evidence remains incomplete. Existing target-specific XSD, current-model representation, source syntax and unread external-DTD limits remain active. Broader repeat/span/intent/layout/playback consumers, packages, linked scores, external dictionaries and remaining normative closure are excluded. Opus selection assesses only that document, never linked scores. Counts are selected predicate instances, not complete MusicXML coverage."
		profiles = append(profiles, p)
	}
	schema := d.schemas[validation.AssemblyMusicXML]
	b := newLocalCatalogBuilder(schema)
	parts := b.namedElementUses("part")
	if len(parts) != 2 {
		return nil, fmt.Errorf("musicxml: timeline catalog: expected two exact part uses, got %d", len(parts))
	}
	var owners []validation.ReferenceOwner
	for i, part := range parts {
		if !part.DeclaredTypeKnown {
			return nil, fmt.Errorf("musicxml: timeline catalog: part type unavailable")
		}
		attribute, err := b.typeAttribute(part.DeclaredType, "id")
		if err != nil {
			return nil, err
		}
		owners = append(owners, validation.ReferenceOwner{Declaration: part.Declaration, Attribute: attribute, Rule: validation.RuleKey{ID: validation.RuleID("MX40-LOCAL-part-id-target/use-" + strconv.Itoa(i+1)), Revision: 1}})
	}
	provider := validation.RuleKey{ID: "MX40-TIMELINE-effective-state", Revision: 1}
	providerPlan, err := validation.NewTimelinePlan(validation.TimelinePlanSpec{Schema: schema, PartOwners: owners})
	if err != nil {
		return nil, err
	}
	assembly := validation.AssemblyMusicXML
	providerRule := validation.RuleDefinition{
		Key: provider, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative,
		Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Scope: validation.ScopeDocument,
		Selector: validation.Selector{Assembly: &assembly}, Predicate: string(provider.ID),
		Text: validation.RuleText{Title: "Exact musical timeline and effective state", Explanation: "Finalize bounded exact event positions and typed part/staff state histories from authoritative schema and identity facts. A completed service retains per-query unavailable evidence rather than inventing initial state or precedence.", Message: "The selected musical timeline service did not finish."},
		Sources: []validation.SourceRef{
			{IDs: []string{"MX40-PROSE-attribute-score-order", "MX40-PROSE-divisions-tuplet-units", "MX40-PROSE-key-staff-scope", "MX40-PROSE-staves-default", "MX40-PROSE-instruments-default"}, URL: localSchemaSource(2815), Locator: "Attributes, divisions and part/staff state"},
			{IDs: []string{"MX40-PROSE-duration-cursor", "MX40-PROSE-chord-anchor"}, URL: localSchemaSource(6472), Locator: "Duration and principal-note cursor movement"},
		},
	}
	d.rules = append(d.rules, providerRule)
	d.plans = append(d.plans, validation.PlanBinding{ID: providerRule.Predicate, Plan: providerPlan})
	for _, spec := range []struct {
		id, subject, title, explanation string
		line                            int
		plan                            func(validation.ConsumerSpec) (validation.Plan, error)
	}{
		{"chord-duration-bound", "note", "Principal-note chord duration bound", "Each duration-bearing chord tone must be no longer than its established principal non-chord note in exact common units. This does not impose pairwise decreasing chord-tone durations.", 6497, validation.NewChordDurationPlan},
		{"numeral-requires-key", "numeral", "Explicit key for a numeral", "A numeral requires a valid local numeral-key or an applicable explicitly encoded key at its enclosing harmony's unshifted score origin and staff. No initial C-major key is invented.", 3889, validation.NewNumeralKeyPlan},
	} {
		plan, err := spec.plan(validation.ConsumerSpec{Schema: schema, Provider: provider})
		if err != nil {
			return nil, err
		}
		key := validation.RuleKey{ID: validation.RuleID("MX40-TIMELINE-" + spec.id), Revision: 1}
		rule := validation.RuleDefinition{
			Key: key, Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryNormative,
			Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Scope: validation.ScopeNode,
			Selector: localType(spec.subject), Predicate: string(key.ID), Requires: []validation.Dependency{{Kind: validation.DependencyRule, Rule: provider, Relation: validation.RelationDocument}},
			Text:    validation.RuleText{Title: spec.title, Explanation: spec.explanation, Message: "The selected timeline-dependent MusicXML requirement was not satisfied."},
			Sources: []validation.SourceRef{{IDs: []string{"MX40-PROSE-" + spec.id}, URL: localSchemaSource(spec.line), Locator: "Exact timeline-dependent clause"}},
		}
		d.rules = append(d.rules, rule)
		d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
	}
	for i := range profiles {
		for _, rule := range d.rules[len(d.rules)-3:] {
			profiles[i].Required = append(profiles[i].Required, rule.Key)
			profiles[i].Support = append(profiles[i].Support, validation.RuleSupport{Rule: rule.Key, State: validation.ImplementationAvailable})
		}
	}
	d.profiles = profiles
	return d, nil
}
