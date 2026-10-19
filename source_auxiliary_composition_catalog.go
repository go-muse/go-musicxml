package musicxml

import (
	"strings"
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileSourceInternalDTDContainer explicitly assesses original container XML
// after bounded internal-DTD processing. Effective path and media type values
// are separate from retained physical Raw values and source-origin evidence.
const ProfileSourceInternalDTDContainer validation.ProfileID = "musicxml-4.0/source-internal-dtd-container"

// ProfileSourceInternalDTDSoundsDictionaries explicitly composes bounded
// internal-DTD processing with effective sounds mappings and supplied standard
// sound dictionaries. It does not discover auxiliary files or select mappings.
const ProfileSourceInternalDTDSoundsDictionaries validation.ProfileID = "musicxml-4.0/source-internal-dtd-sounds-dictionaries"

var sourceAuxiliaryCompositionEngine = sync.OnceValues(buildSourceAuxiliaryCompositionEngine)

func buildSourceAuxiliaryCompositionEngine() (*validation.Engine, error) {
	d, err := buildSourceAuxiliaryCompositionDefinitions()
	if err != nil {
		return nil, err
	}
	catalog, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		return nil, err
	}
	return validation.NewEngine(catalog)
}

func buildSourceAuxiliaryCompositionDefinitions() (*assessmentDefinitions, error) {
	// Fresh definitions preserve the old auxiliary and dictionary catalogs exactly.
	d, err := buildAuxiliarySourceAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	replacements := make(map[validation.RuleKey]validation.RuleKey)
	for i := range d.rules {
		r := &d.rules[i]
		assembly := *r.Selector.Assembly
		schema := d.schemas[assembly]
		var plan validation.Plan
		changed := true
		switch {
		case strings.HasPrefix(string(r.Key.ID), "MX40-SOURCE-XSD-"):
			check := validation.XSDCheck(strings.TrimPrefix(string(r.Key.ID), "MX40-SOURCE-XSD-"+string(assembly)+"-"))
			plan, err = validationNewXSDPlanWithSourceDeclarations(schema, check)
			r.Text.Explanation += " Assess the effective source infoset and only actually-read bounded internal declarations; unavailable declaration evidence remains incomplete."
		case r.Key.ID == "MX40-CONTEXT-container-metadata":
			plan, err = validationNewSourceContainerMetadataContextPlan(schema)
			r.Text.Explanation += " Effective DTD values may select rootfiles while physical Raw is independently absent or unavailable."
		case strings.HasPrefix(string(r.Key.ID), "MX40-RESOURCE-schema-hints/"):
			plan, err = validationNewSourceSchemaHintContextPlan(schema)
			r.Text.Explanation += " Effective DTD hints remain separate from independently retained physical occurrence and lexical evidence."
		default:
			changed = false // Sounds mapping fields already describe effective supplied data.
		}
		if err != nil {
			return nil, err
		}
		if !changed {
			continue
		}
		old := r.Key
		r.Key.Revision++
		r.Predicate += "/source-infoset-v1"
		replacements[old] = r.Key
		d.plans[i] = validation.PlanBinding{ID: r.Predicate, Plan: plan}
	}
	for i := range d.profiles {
		p := &d.profiles[i]
		assembly := validation.AssemblyContainer
		p.ID = ProfileSourceInternalDTDContainer
		if p.SupportedRoots[0].Local == "sounds" {
			assembly = validation.AssemblySounds
			p.ID = ProfileSourceInternalDTDSoundsDictionaries
		}
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
		p.NormativeClosure = "Explicit source-only pinned MusicXML 4.0 " + string(assembly) + " XSD assessment after bounded actually-read internal-DTD processing, including a DOCTYPE external subset read from the bundled official MusicXML DTDs or a caller DTD source. Effective values, physical occurrence evidence and retained lexical spellings remain independently qualified. Source origins retain separately available physical, declaration and reference sites. Unrecognized external declarations, syntax, unsupported capabilities and incomplete provenance retain their gates. Schema hints never fetch or select a schema. No Document model, automatic auxiliary discovery, application mapping choice, playback or standalone package-conformance claim."
		if assembly == validation.AssemblyContainer {
			p.NormativeClosure += " Ordered rootfile inventory and effective path/media-type values preserve content prerequisites; physical Raw never reconstructs defaults or entity replacement spellings."
		} else {
			p.NormativeClosure += " Directional mapping fields are effective supplied data, including explicitly selected DTD defaults; omission alone has no XSD or prose default. Standard-sound recognition uses only explicitly supplied pinned capabilities and keeps nonstandard identifiers allowed."
			r, plan, e := dictionaryRule(d.schemas[assembly], validation.DictionaryStandardSounds, []validation.Target{validation.TargetSource})
			if e != nil {
				return nil, e
			}
			d.rules = append(d.rules, r)
			d.plans = append(d.plans, validation.PlanBinding{ID: r.Predicate, Plan: plan})
			dictionaryRequire(p, r.Key)
		}
		plan, e := validationNewSourceOriginPlan(assembly)
		if e != nil {
			return nil, e
		}
		key := validation.RuleKey{ID: validation.RuleID("MX40-SOURCE-ORIGIN-" + string(assembly)), Revision: 1}
		r := validation.RuleDefinition{Key: key, Selector: validation.Selector{Assembly: &assembly}, Scope: validation.ScopeDocument, Targets: []validation.Target{validation.TargetSource}, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Predicate: string(key.ID), Text: validation.RuleText{Title: "Source occurrence provenance", Explanation: "Retain physical specifiedness and independently available physical, declaration and reference sites for each admitted auxiliary source occurrence. Unavailable required provenance remains incomplete.", Message: "Source occurrence provenance is incomplete."}, Sources: []validation.SourceRef{{IDs: []string{"XML-1.0-Fifth-Edition", "XML-Infoset-Second-Edition"}, URL: "https://www.w3.org/TR/2004/REC-xml-infoset-20040204/", Locator: "2.2; 2.3; specified and origin information; bounded occurrence projection"}}}
		d.rules = append(d.rules, r)
		d.plans = append(d.plans, validation.PlanBinding{ID: r.Predicate, Plan: plan})
		dictionaryRequire(p, key)
	}
	return d, nil
}
