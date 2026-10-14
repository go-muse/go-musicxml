package musicxml

import (
	"fmt"
	"strings"
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileSourceInternalDTDLocal explicitly composes the bounded internal-DTD
// infoset, document ENTITY/ENTITIES evidence and existing local score/Opus rules.
// It does not retrieve external declarations or reconstruct current-model history.
const ProfileSourceInternalDTDLocal validation.ProfileID = "musicxml-4.0/source-internal-dtd-local"

// ProfileSourceInternalDTDLinks adds existing nonfetching local link projections
// with physical occurrence evidence kept separate from effective DTD values.
const ProfileSourceInternalDTDLinks validation.ProfileID = "musicxml-4.0/source-internal-dtd-links"

// ProfileSourceInternalDTDDictionaries adds explicitly supplied dictionary
// capabilities to the standalone DTD/local/link composition. No data is loaded
// automatically, and the existing partial IPA limitation remains applicable.
const ProfileSourceInternalDTDDictionaries validation.ProfileID = "musicxml-4.0/source-internal-dtd-links-dictionaries"

var sourceCompositionEngine = sync.OnceValues(buildSourceCompositionEngine)

// Closed policy, rather than profile-name matching, elects parser capabilities.
func sourceDTDPolicy(id validation.ProfileID) (internal, declarations, lexical bool) {
	switch id {
	case ProfileSourceInternalDTD:
		return true, false, false
	case ProfileSourceInternalDTDLocal:
		return true, true, false
	case ProfileSourceInternalDTDLinks, ProfileSourceInternalDTDDictionaries, ProfileSourceInternalDTDContainer, ProfileSourceInternalDTDSoundsDictionaries, ProfileSourceSemantics:
		return true, true, true
	default:
		return false, false, false
	}
}

func compositionProfileCopy(p validation.Profile) validation.Profile {
	p.Required = append([]validation.RuleKey(nil), p.Required...)
	p.Support = append([]validation.RuleSupport(nil), p.Support...)
	p.SupportedRoots = append([]validation.QName(nil), p.SupportedRoots...)
	return p
}

func compositionSourceProfile(d *assessmentDefinitions, id validation.ProfileID) (validation.Profile, error) {
	for _, p := range d.profiles {
		if p.ID == id && p.Target == validation.TargetSource {
			return compositionProfileCopy(p), nil
		}
	}
	return validation.Profile{}, fmt.Errorf("musicxml: composition source profile %s unavailable", id)
}

func buildSourceCompositionEngine() (*validation.Engine, error) {
	d, err := buildSourceCompositionDefinitions()
	if err != nil {
		return nil, err
	}
	catalog, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		return nil, err
	}
	return validation.NewEngine(catalog)
}

func buildSourceCompositionDefinitions() (*assessmentDefinitions, error) {
	// Each builder owns its definitions. No existing compiled catalog is extended.
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	local, err := compositionSourceProfile(d, ProfileSourceLocal)
	if err != nil {
		return nil, err
	}
	d, err = appendLinkAssessmentDefinitions(d)
	if err != nil {
		return nil, err
	}
	links, err := compositionSourceProfile(d, ProfileSourceLinks)
	if err != nil {
		return nil, err
	}
	dictionaries := compositionProfileCopy(links)
	local.ID, links.ID, dictionaries.ID = ProfileSourceInternalDTDLocal, ProfileSourceInternalDTDLinks, ProfileSourceInternalDTDDictionaries
	baseClosure := "Explicit standalone MusicXML 4.0 score and Opus source assessment after bounded actually-read internal-DTD processing. Effective first general-entity declarations supply document-local ENTITY/ENTITIES membership for the reviewed XSD 1.0 builtins and restriction carriers. Unread declarations, reached unresolved constructed carriers, schema-component validity and unavailable source evidence remain incomplete. Existing local clauses use the effective source infoset; XSD defaults and physical specifiedness remain distinct. Source-origin projections retain independently available occurrence, declaration and use locations. No external DTD retrieval, general validating DTD processor, model-history reconstruction, timeline/context composition, auxiliary Container/Sounds assembly, package adoption or complete MusicXML conformance claim."
	local.NormativeClosure = baseClosure
	links.NormativeClosure = baseClosure + " Adds existing local nonfetching link, attachment, schema-hint and Opus projections; original physical lexical evidence is never reconstructed from effective values. Selected resource snapshots retain effective first-bound DTD ID occurrences separately from XSD IDs, with declaration and occurrence closure, and choose the first document-order joined shorthand owner. XSD identity validity is independent; incomplete identity authorities cannot establish a miss. Invalid ID default declarations retain limited typing evidence without DTD validation. No target is retrieved or assessed."
	dictionaries.NormativeClosure = links.NormativeClosure + " Adds explicitly supplied pinned dictionary capabilities and existing exact consumers. Canonical SMuFL membership remains independent of XSD family checks; sounds remain permitted open strings, and partial IPA evidence does not establish complete repertoire or sequence validity. No dictionary is loaded automatically."
	for _, op := range []validation.DictionaryOperation{validation.DictionaryCanonical, validation.DictionaryStandardSounds, validation.DictionaryIPA, validation.DictionaryRendering} {
		rule, plan, err := dictionaryRule(d.schemas[validation.AssemblyMusicXML], op, []validation.Target{validation.TargetSource})
		if err != nil {
			return nil, err
		}
		d.rules = append(d.rules, rule)
		d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
		dictionaryRequire(&dictionaries, rule.Key)
	}
	d.profiles = []validation.Profile{local, links, dictionaries}
	if err := appendSourceOriginDefinitions(d); err != nil {
		return nil, err
	}
	if err := specializeSourceComposition(d); err != nil {
		return nil, err
	}
	return d, nil
}

// appendSourceOriginDefinitions extends only caller-owned source definitions.
func appendSourceOriginDefinitions(d *assessmentDefinitions) error {
	for _, assembly := range []validation.Assembly{validation.AssemblyMusicXML, validation.AssemblyOpus} {
		plan, err := validation.NewSourceOriginPlan(assembly)
		if err != nil {
			return err
		}
		key := validation.RuleKey{ID: validation.RuleID("MX40-SOURCE-ORIGIN-" + string(assembly)), Revision: 1}
		rule := validation.RuleDefinition{Key: key, Selector: validation.Selector{Assembly: &assembly}, Scope: validation.ScopeDocument,
			Targets: []validation.Target{validation.TargetSource}, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative,
			Predicate: string(key.ID), Text: validation.RuleText{Title: "Source occurrence provenance", Explanation: "Each admitted source element and attribute occurrence retains physical specifiedness and independently available physical, declaration and reference sites. Complete enumeration is sealed explicitly; unavailable required provenance remains incomplete.", Message: "Source occurrence provenance is incomplete."},
			Sources: []validation.SourceRef{{IDs: []string{"XML-1.0-Fifth-Edition", "XML-Infoset-Second-Edition"}, URL: "https://www.w3.org/TR/2004/REC-xml-infoset-20040204/", Locator: "2.2; 2.3; specified and origin information; bounded occurrence projection"}},
		}
		d.rules = append(d.rules, rule)
		d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
		for i := range d.profiles {
			dictionaryRequire(&d.profiles[i], key)
		}
	}
	return nil
}

func specializeSourceComposition(d *assessmentDefinitions) error {
	selected := make(map[validation.RuleKey]bool)
	for _, p := range d.profiles {
		for _, key := range p.Required {
			selected[key] = true
		}
	}
	// Include explicit dependencies before pruning model-only definitions.
	for changed := true; changed; {
		changed = false
		for _, r := range d.rules {
			if !selected[r.Key] {
				continue
			}
			for _, dep := range r.Requires {
				if dep.Kind == validation.DependencyRule && !selected[dep.Rule] {
					selected[dep.Rule], changed = true, true
				}
			}
		}
	}
	plans := make(map[string]validation.Plan, len(d.plans))
	for _, p := range d.plans {
		plans[p.ID] = p.Plan
	}
	replacements := make(map[validation.RuleKey]validation.RuleKey)
	local := newLocalCatalogBuilder(d.schemas[validation.AssemblyMusicXML])
	var rules []validation.RuleDefinition
	var bindings []validation.PlanBinding
	for _, original := range d.rules {
		if !selected[original.Key] {
			continue
		}
		r := original
		plan, ok := plans[r.Predicate]
		if !ok {
			return fmt.Errorf("musicxml: composition plan %s unavailable", r.Predicate)
		}
		replacement, changed, err := compositionPlan(d, local, r)
		if err != nil {
			return err
		}
		if changed {
			plan = replacement
			r.Key.Revision++
			r.Predicate += "/source-infoset-v1"
			r.Targets = []validation.Target{validation.TargetSource}
			if strings.HasPrefix(string(r.Key.ID), "MX40-XSD-") {
				r.Text.Explanation += " This source operation assesses the selected effective DTD infoset. Document-dependent ENTITY/ENTITIES checks use only the owned declaration domain; unresolved constructed carriers remain incomplete."
			} else if strings.HasPrefix(string(r.Key.ID), "MX40-RESOURCE-") {
				r.Text.Explanation += " Effective source values drive the projection, while physical occurrence and retained original lexical evidence remain independently qualified."
			} else {
				r.Text.Explanation += " Supplied source attributes include admitted DTD defaults and entity markup; XSD-only defaults do not establish supplied presence."
			}
			replacements[original.Key] = r.Key
		}
		rules = append(rules, r)
		bindings = append(bindings, validation.PlanBinding{ID: r.Predicate, Plan: plan})
	}
	for i := range rules {
		for j := range rules[i].Requires {
			if key, ok := replacements[rules[i].Requires[j].Rule]; ok {
				rules[i].Requires[j].Rule = key
			}
		}
	}
	for i := range d.profiles {
		p := &d.profiles[i]
		for j, key := range p.Required {
			if replacement, ok := replacements[key]; ok {
				p.Required[j] = replacement
			}
		}
		for j := range p.Support {
			if replacement, ok := replacements[p.Support[j].Rule]; ok {
				p.Support[j].Rule = replacement
			}
		}
	}
	d.rules, d.plans = rules, bindings
	return nil
}

func compositionPlan(d *assessmentDefinitions, b *localCatalogBuilder, r validation.RuleDefinition) (validation.Plan, bool, error) {
	for _, assembly := range []validation.Assembly{validation.AssemblyMusicXML, validation.AssemblyOpus} {
		for _, check := range []validation.XSDCheck{validation.XSDCheckBinding, validation.XSDCheckDeclaration, validation.XSDCheckAttributes, validation.XSDCheckScalar, validation.XSDCheckContent, validation.XSDCheckIdentity} {
			if r.Key.ID == validation.RuleID("MX40-XSD-"+string(assembly)+"-"+string(check)) {
				p, err := validation.NewXSDPlanWithSourceDeclarations(d.schemas[assembly], check)
				return p, true, err
			}
		}
		if r.Key.ID == validation.RuleID("MX40-RESOURCE-local-links/"+string(assembly)) {
			p, err := validation.NewSourceLocalLinkContextPlan(d.schemas[assembly])
			return p, true, err
		}
		if r.Key.ID == validation.RuleID("MX40-RESOURCE-schema-hints/"+string(assembly)) {
			p, err := validation.NewSourceSchemaHintContextPlan(d.schemas[assembly])
			return p, true, err
		}
	}
	for _, spec := range []struct{ id, attribute string }{{"sound-dacapo-yes", "dacapo"}, {"sound-forward-repeat-yes", "forward-repeat"}} {
		if r.Key.ID == validation.RuleID("MX40-LOCAL-"+spec.id) {
			a, err := b.attribute("sound", spec.attribute)
			if err != nil {
				return validation.Plan{}, false, err
			}
			p, err := validation.NewSourceAttributeValuePlan(a, []validation.Atomic{localString("yes")})
			return p, true, err
		}
	}
	for _, owner := range []string{"slur", "tied"} {
		if r.Key.ID != validation.RuleID("MX40-LOCAL-bezier-second-control/"+owner) {
			continue
		}
		var triggers []validation.AttributeSelector
		for _, name := range []string{"bezier-x2", "bezier-y2", "bezier-offset2"} {
			a, err := b.attribute(owner, name)
			if err != nil {
				return validation.Plan{}, false, err
			}
			triggers = append(triggers, a)
		}
		a, err := b.attribute(owner, "type")
		if err != nil {
			return validation.Plan{}, false, err
		}
		p, err := validation.NewSourcePresenceImpliesAttributePlan(triggers, localType("slur"), a, localString("continue"))
		return p, true, err
	}
	return validation.Plan{}, false, nil
}
