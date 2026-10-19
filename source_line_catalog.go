package musicxml

import (
	"strings"
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileSourceLine is validation line 2 of strict loading: the MusicXML 4.0
// and XSD requirements that the loaded structure cannot show, because
// decoding and conversion lose or change the data they read. DecodeAssessed
// runs it while it reads the document; line 3, ProfileModelSemantics, checks
// the converted structure.
const ProfileSourceLine validation.ProfileID = "musicxml-4.0/source-line"

var sourceLineEngine = sync.OnceValues(buildSourceLineEngine)

func buildSourceLineEngine() (*validation.Engine, error) {
	d, err := buildSourceLineDefinitions()
	if err != nil {
		return nil, err
	}
	catalog, err := newCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		return nil, err
	}
	return validation.NewEngine(catalog)
}

// sourceLineRule reports whether a source-semantics rule belongs to line 2:
// the six XSD operations on the effective DTD infoset (the one accepted
// overlap with line 3, see ruleLines), the source occurrence provenance that
// the source-infoset plans require, and the schema hints that the MXL
// schema-hint-pairs rule reads from original attributes.
func sourceLineRule(id validation.RuleID) bool {
	for _, prefix := range []string{"MX40-XSD-", "MX40-SOURCE-ORIGIN-", "MX40-RESOURCE-schema-hints/"} {
		if strings.HasPrefix(string(id), prefix) {
			return true
		}
	}
	return false
}

// buildSourceLineDefinitions derives line 2 from the source-semantics
// composition, so that its XSD, provenance and schema-hint rules keep exactly
// the source revisions and plans that composition elects, then adds the
// representability rule, which only strict loading needs.
func buildSourceLineDefinitions() (*assessmentDefinitions, error) {
	d, err := buildSemanticAssessmentDefinitions(validation.TargetSource)
	if err != nil {
		return nil, err
	}
	profile := compositionProfileCopy(d.profiles[0])
	profile.ID = ProfileSourceLine
	profile.NormativeClosure = "Validation line 2 of strict loading: bounded MusicXML 4.0 XSD assessment of the effective internal-DTD source infoset, source occurrence provenance and original schema hints, which the converted model cannot show. A DOCTYPE external subset is read from the bundled official MusicXML DTDs or a caller DTD source, never fetched. Every numeric value the generated model holds must survive conversion unchanged. Local, timeline, contextual, resource-link and dictionary checks belong to line 3 on the converted model; no universal conformance claim."
	profile.Required = profile.Required[:0]
	for _, key := range d.profiles[0].Required {
		if sourceLineRule(key.ID) {
			profile.Required = append(profile.Required, key)
		}
	}
	required := make(map[validation.RuleKey]bool, len(profile.Required))
	for _, key := range profile.Required {
		required[key] = true
	}
	profile.Support = profile.Support[:0]
	for _, support := range d.profiles[0].Support {
		if required[support.Rule] {
			profile.Support = append(profile.Support, support)
		}
	}
	d.profiles = []validation.Profile{profile}
	pruneUnrequiredDefinitions(d)
	if err := appendSourceRepresentabilityDefinitions(d); err != nil {
		return nil, err
	}
	return d, nil
}

// pruneUnrequiredDefinitions keeps only the rules that a profile requires,
// with their rule dependencies, and their plans. It filters d in place: the
// definitions are built fresh for each catalog.
func pruneUnrequiredDefinitions(d *assessmentDefinitions) {
	selected := requiredRuleClosure(d)
	predicates := make(map[string]bool)
	rules := d.rules[:0]
	for _, r := range d.rules {
		if selected[r.Key] {
			rules = append(rules, r)
			predicates[r.Predicate] = true
		}
	}
	plans := d.plans[:0]
	for _, p := range d.plans {
		if predicates[p.ID] {
			plans = append(plans, p)
		}
	}
	d.rules, d.plans = rules, plans
}
