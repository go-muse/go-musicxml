package musicxml

import (
	"fmt"
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileSourceSemantics composes encoded MusicXML checks with the bounded
// internal-DTD source infoset and explicitly supplied normative dictionaries.
// It is an opt-in interpretation, not a universal conformance certificate.
const ProfileSourceSemantics validation.ProfileID = "musicxml-4.0/source-semantics"

// ProfileModelSemantics assesses current encoded model facts without recreating
// discarded XML, DTD, namespace, lexical or original package history.
const ProfileModelSemantics validation.ProfileID = "musicxml-4.0/model-semantics"

var sourceSemanticEngine = sync.OnceValues(func() (*validation.Engine, error) { return buildSemanticAssessmentEngine(validation.TargetSource) })
var modelSemanticEngine = sync.OnceValues(func() (*validation.Engine, error) { return buildSemanticAssessmentEngine(validation.TargetModel) })

func buildSemanticAssessmentEngine(target validation.Target) (*validation.Engine, error) {
	d, err := buildSemanticAssessmentDefinitions(target)
	if err != nil {
		return nil, err
	}
	catalog, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		return nil, err
	}
	return validation.NewEngine(catalog)
}

func buildSemanticAssessmentDefinitions(target validation.Target) (*assessmentDefinitions, error) {
	if target != validation.TargetSource && target != validation.TargetModel {
		return nil, fmt.Errorf("musicxml: unsupported semantic target %q", target)
	}
	d, err := buildContextAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	wantBase := ProfileModelContext
	if target == validation.TargetSource {
		wantBase = ProfileSourceContext
	}
	var selected validation.Profile
	for _, p := range d.profiles {
		if p.ID == wantBase && p.Target == target {
			if selected.ID != "" {
				return nil, fmt.Errorf("musicxml: duplicate semantic base profile %s", wantBase)
			}
			selected = compositionProfileCopy(p)
		}
	}
	if selected.ID == "" {
		return nil, fmt.Errorf("musicxml: semantic target profile missing")
	}
	selected.ID = ProfileModelSemantics
	if target == validation.TargetSource {
		selected.ID = ProfileSourceSemantics
	}
	selected.NormativeClosure = "Explicit bounded assessment of encoded MusicXML 4.0 XSD, local, exact timeline, contextual and nonfetching resource facts. Selected normative consumers include independently proved barline location and graphical-repeat content. Unavailable timing, unresolved source clauses and unsupported representation capabilities remain explicit; no renderer, external desired-score comparison, inferred exporter intention or universal conformance claim. Canonical SMuFL membership and the actual supported IPA evidence require only caller-selected data at relevant occurrences; no dictionary is loaded automatically. Optional standard-sound recognition and dictionary-backed rendering advice are not added by this composition."
	if target == validation.TargetSource {
		selected.NormativeClosure += " Actually read bounded internal declarations, a DOCTYPE external subset read from the bundled official MusicXML DTDs or a caller DTD source, and owned source origins distinguish physical from effective values. Unrecognized external declarations remain unavailable; no external subset is fetched from the network. Joined DTD/XSD shorthand authority is document-local."
	} else {
		selected.NormativeClosure += " Current-model assessment never reconstructs discarded source, namespaces, DTD declarations or original bytes."
	}
	d.profiles = []validation.Profile{selected}
	// Reuse only the resource rules absent from the context catalog. Its existing
	// Opus pagination rule already owns that behavior and must not be duplicated.
	links, err := buildLinkAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	plans := make(map[string]validation.Plan, len(links.plans))
	for _, p := range links.plans {
		plans[p.ID] = p.Plan
	}
	resourceKeys := map[validation.RuleKey]bool{
		{ID: "MX40-RESOURCE-local-links/musicxml", Revision: 1}:  false,
		{ID: "MX40-RESOURCE-local-links/opus", Revision: 1}:      false,
		{ID: "MX40-RESOURCE-schema-hints/musicxml", Revision: 1}: false,
		{ID: "MX40-RESOURCE-schema-hints/opus", Revision: 1}:     false,
	}
	for _, r := range links.rules {
		seen, owned := resourceKeys[r.Key]
		if !owned {
			continue
		}
		if seen {
			return nil, fmt.Errorf("musicxml: duplicate semantic resource rule %s", r.Key.ID)
		}
		resourceKeys[r.Key] = true
		d.rules = append(d.rules, r)
		d.plans = append(d.plans, validation.PlanBinding{ID: r.Predicate, Plan: plans[r.Predicate]})
		dictionaryRequire(&d.profiles[0], r.Key)
	}
	for key, seen := range resourceKeys {
		if !seen {
			return nil, fmt.Errorf("musicxml: missing semantic resource rule %s", key.ID)
		}
	}
	barline, err := addBarlineLocationAssessment(d)
	if err != nil {
		return nil, err
	}
	dictionaryRequire(&d.profiles[0], barline)
	for _, op := range []validation.DictionaryOperation{validation.DictionaryCanonical, validation.DictionaryIPA} {
		r, p, e := dictionaryRule(d.schemas[validation.AssemblyMusicXML], op, []validation.Target{target})
		if e != nil {
			return nil, e
		}
		d.rules = append(d.rules, r)
		d.plans = append(d.plans, validation.PlanBinding{ID: r.Predicate, Plan: p})
		dictionaryRequire(&d.profiles[0], r.Key)
	}
	if target == validation.TargetSource {
		if err := appendSourceOriginDefinitions(d); err != nil {
			return nil, err
		}
		if err := specializeSourceComposition(d); err != nil {
			return nil, err
		}
	}
	if err := electXMLFragmentBaseline(d, target == validation.TargetSource); err != nil {
		return nil, err
	}
	return d, nil
}
