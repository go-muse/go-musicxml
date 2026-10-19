package musicxml

import (
	"github.com/go-muse/go-musicxml/validation"
	"sync"
)

// ProfileSourceDictionaries selects source XSD/local links plus exact pinned
// dictionary consumers and narrow rendering advice. It performs no I/O itself.
const ProfileSourceDictionaries validation.ProfileID = "musicxml-4.0/source-links-dictionaries"

// ProfileModelDictionaries assesses current-model XSD/local links and exact
// dictionary consumers, without reconstructing discarded source xsi:type.
const ProfileModelDictionaries validation.ProfileID = "musicxml-4.0/model-links-dictionaries"

// ProfileSourceSoundsDictionaries adds non-normative standard-ID recognition
// to source-only sounds XSD, mapping and discovery context. No model is added.
const ProfileSourceSoundsDictionaries validation.ProfileID = "musicxml-4.0/source-sounds-dictionaries"

var dictionaryAssessmentEngine = sync.OnceValues(buildDictionaryAssessmentEngine)

func newDictionaryAssessmentEngine() (*validation.Engine, error) { return dictionaryAssessmentEngine() }
func buildDictionaryAssessmentEngine() (*validation.Engine, error) {
	d, err := buildDictionaryAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	c, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		return nil, err
	}
	return validation.NewEngine(c)
}
func buildDictionaryAssessmentDefinitions() (*assessmentDefinitions, error) {
	d, err := buildLinkAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	for i := range d.profiles {
		p := &d.profiles[i]
		if p.Target == validation.TargetSource {
			p.ID = ProfileSourceDictionaries
		} else {
			p.ID = ProfileModelDictionaries
		}
		p.NormativeClosure = "Bounded MusicXML 4.0 source or current-model XSD/local links plus exact independently verified SMuFL 1.4 canonical-name membership. The six XSD name-family facets remain independent. Standard sounds are open-string recognition evidence; legacy IPA 2015/Unicode 13 positive tables remain partial. A separately selected representation manifest may establish an affirmative whole-input repertoire witness, without phonotactic, rendering or pronunciation claims. Missing selected capabilities remain unavailable per occurrence. Narrow mute/symbol/mordent defaults, active unmeasured-tremolo range facts and note-name/glyph/arrow advice do not assert general rendering intent. Local source/current-model/link limitations remain active. No fetching, implicit DTD composition, timeline, graph/package traversal or complete MusicXML conformance claim."
	}
	for _, op := range []validation.DictionaryOperation{validation.DictionaryCanonical, validation.DictionaryStandardSounds, validation.DictionaryIPA, validation.DictionaryRendering} {
		rule, plan, err := dictionaryRule(d.schemas[validation.AssemblyMusicXML], op, []validation.Target{validation.TargetSource, validation.TargetModel})
		if err != nil {
			return nil, err
		}
		d.rules = append(d.rules, rule)
		d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
		for i := range d.profiles {
			dictionaryRequire(&d.profiles[i], rule.Key)
		}
	}
	if err := appendDictionarySoundsDefinitions(d); err != nil {
		return nil, err
	}
	return d, nil
}
func dictionaryRequire(p *validation.Profile, key validation.RuleKey) {
	p.Required = append(p.Required, key)
	p.Support = append(p.Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
}
func dictionaryRule(schema *validation.CompiledSchema, op validation.DictionaryOperation, targets []validation.Target) (validation.RuleDefinition, validation.Plan, error) {
	plan, err := validationNewDictionaryPlan(schema, op)
	if err != nil {
		return validation.RuleDefinition{}, validation.Plan{}, err
	}
	assembly := schema.Assembly()
	key := validation.RuleKey{ID: validation.RuleID("MX40-DICTIONARY-" + string(assembly) + "/" + string(op)), Revision: 1}
	role := validation.RoleContextProvider
	title, explanation, ids := "Pinned dictionary evidence", "Retain exact occurrence-qualified dictionary evidence without upgrading advisory observations to normative failures.", []string{"CHECK-context-sounds-standard-identifiers"}
	switch op {
	case validation.DictionaryCanonical:
		role = validation.RolePredicate
		title = "Canonical SMuFL 1.4 names"
		explanation = "Every exact assessed SMuFL scalar domain and declared attribute use requires an exact name in the complete verified canonical set. Independent XSD lexical/family defects retain their own causes."
		ids = []string{"REQ-DEF-EXTERNAL-SMUFL", "CHECK-smufl-canonical-name", "MX40-ISSUE-smufl-version-dictionary"}
	case validation.DictionaryIPA:
		role = validation.RolePredicate
		key.Revision = 2
		title = "IPA positive facts and selected whole-input representation witness"
		explanation = "Legacy symbols, literal examples and canonical equivalents retain partial positive evidence. Only the separately selected fixed representation manifest can establish an affirmative whole-input repertoire witness. Missing or unproved witnesses remain unavailable; phonotactics, rendering and pronunciation intent are not assessed."
		ids = []string{"REQ-DEF-EXTERNAL-IPA", "CHECK-ipa-2015-unicode13", "MX40-ISSUE-ipa-character-repertoire"}
	case validation.DictionaryRendering:
		title = "Narrow dictionary-backed rendering context"
		ids = []string{"CHECK-context-stopped-smufl-default", "CHECK-context-open-smufl-default", "CHECK-context-half-muted-smufl-default", "CHECK-context-tremolo-smufl-condition", "CHECK-context-named-notehead-text", "CHECK-context-symbol-defaults-and-name", "CHECK-context-symbol-semantic-markup", "CHECK-context-glyph-match-type", "CHECK-context-arrow-glyph-consistency", "CHECK-context-mordent-long-default"}
	}
	rule := validation.RuleDefinition{Key: key, Selector: validation.Selector{Assembly: &assembly}, Scope: validation.ScopeNode, Targets: targets, Class: validation.ClassMandatory, Role: role, Category: validation.CategoryNormative, Predicate: string(key.ID), Text: validation.RuleText{Title: title, Explanation: explanation, Message: "The selected exact dictionary operation did not establish its complete obligation."}, Sources: []validation.SourceRef{{IDs: ids, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd", Locator: "Exact reviewed dictionary consumer declarations and independent compiled data/policy pins"}}}
	return rule, plan, nil
}
func appendDictionarySoundsDefinitions(d *assessmentDefinitions) error {
	schema, err := compileValidationSchema(validation.AssemblySounds, soundsValidationSchema)
	if err != nil {
		return err
	}
	assembly := validation.AssemblySounds
	d.schemas[assembly] = schema
	p := validation.Profile{ID: ProfileSourceSoundsDictionaries, MusicXMLVersion: "4.0", Target: validation.TargetSource, RegistryCommit: "3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c", SupportedRoots: []validation.QName{{Local: "sounds"}}, NormativeClosure: "Source-only pinned sounds XSD, exact directional mapping and nonfetching schema-hint discovery, with verified MusicXML 4.0 standard-ID recognition. Known nonstandard identifiers remain allowed open-string evidence. Missing dictionary evidence does not become a normative violation. Original source syntax and unread external-DTD limitations remain active. No SoundsDocument model, application mapping choice, package traversal or playback claim."}
	for _, check := range []validation.XSDCheck{validation.XSDCheckBinding, validation.XSDCheckDeclaration, validation.XSDCheckAttributes, validation.XSDCheckScalar, validation.XSDCheckContent, validation.XSDCheckIdentity} {
		plan, err := validation.NewXSDPlan(schema, check)
		if err != nil {
			return err
		}
		key := validation.RuleKey{ID: validation.RuleID("MX40-SOURCE-XSD-sounds-" + string(check)), Revision: 1}
		rule := validation.RuleDefinition{Key: key, Selector: validation.Selector{Assembly: &assembly}, Scope: validation.ScopeNode, Targets: []validation.Target{validation.TargetSource}, Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryNormative, Predicate: string(key.ID), Text: validation.RuleText{Title: "Sounds XSD " + string(check), Explanation: "Assess the exact pinned sounds schema operation independently of dictionary evidence.", Message: "The selected sounds XSD obligation did not complete."}, Sources: []validation.SourceRef{{IDs: []string{"MusicXML-4.0-sounds.xsd"}, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/sounds.xsd", Locator: "Pinned source-only sounds schema"}}}
		if check == validation.XSDCheckIdentity {
			rule.Scope = validation.ScopeDocument
		}
		d.rules = append(d.rules, rule)
		d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
		dictionaryRequire(&p, key)
	}
	mapping, err := validationNewSoundsMappingContextPlan(schema)
	if err != nil {
		return err
	}
	hints, err := validationNewSchemaHintContextPlan(schema)
	if err != nil {
		return err
	}
	for _, v := range []struct {
		id   string
		plan validation.Plan
	}{{"MX40-CONTEXT-sounds-mapping", mapping}, {"MX40-RESOURCE-schema-hints/sounds", hints}} {
		key := validation.RuleKey{ID: validation.RuleID(v.id), Revision: 1}
		rule := validation.RuleDefinition{Key: key, Selector: validation.Selector{Assembly: &assembly}, Scope: validation.ScopeDocument, Targets: []validation.Target{validation.TargetSource}, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Predicate: v.id, Text: validation.RuleText{Title: "Exact sounds source context", Explanation: "Retain exact mapping and discovery evidence without fetching or application choice.", Message: "Sounds source context did not finish."}, Sources: []validation.SourceRef{{IDs: []string{"MusicXML-4.0-sounds.xsd"}, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/sounds.xsd", Locator: "Pinned source sounds context"}}}
		d.rules = append(d.rules, rule)
		d.plans = append(d.plans, validation.PlanBinding{ID: v.id, Plan: v.plan})
		dictionaryRequire(&p, key)
	}
	rule, plan, err := dictionaryRule(schema, validation.DictionaryStandardSounds, []validation.Target{validation.TargetSource})
	if err != nil {
		return err
	}
	d.rules = append(d.rules, rule)
	d.plans = append(d.plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
	dictionaryRequire(&p, rule.Key)
	d.profiles = append(d.profiles, p)
	return nil
}
