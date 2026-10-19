package musicxml

import (
	"strings"
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileModelXSD selects direct current-model XSD assessment of the generated
// score-partwise, score-timewise and opus roots, including model representation
// checks. It assesses current values and model-defined structure, not original
// XML or an encoder's output. Reached unsupported capabilities remain incomplete.
const ProfileModelXSD validation.ProfileID = "musicxml-4.0/model-xsd"

// ProfileModelLocal selects current-model XSD and the explicitly bounded local
// musical relations and typed references described in the profile closure. It
// excludes timeline, intent, repeats, packages, links and external dictionaries;
// a satisfied report is not a claim of complete MusicXML conformance.
const ProfileModelLocal validation.ProfileID = "musicxml-4.0/model-local"

// ProfileSourceLocal selects original-source XSD plus the same bounded local
// musical clauses as ProfileModelLocal. It retains the source preview's DTD and
// XSD capability limits; unread external subsets prevent acceptance. It is not
// an implicit default or complete MusicXML conformance profile.
const ProfileSourceLocal validation.ProfileID = "musicxml-4.0/source-local"

const assessmentRegistryCommit = "3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c"

var modelAssessmentEngine = sync.OnceValues(buildModelAssessmentEngine)

func newModelAssessmentEngine() (*validation.Engine, error) { return modelAssessmentEngine() }

// The preview catalog remains separate so adding target-specific local profiles
// cannot silently change its obligation list or catalog digest.
type assessmentDefinitions struct {
	rules    []validation.RuleDefinition
	profiles []validation.Profile
	plans    []validation.PlanBinding
	schemas  map[validation.Assembly]*validation.CompiledSchema
}

func buildModelAssessmentEngine() (*validation.Engine, error) {
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	catalog, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		return nil, err
	}
	return validation.NewEngine(catalog)
}

// Reusing definition construction does not broaden the frozen compiled catalog.
// Each target family receives an independent catalog and therefore its own digest.
func buildModelAssessmentDefinitions() (*assessmentDefinitions, error) {
	modelXSD := validation.Profile{
		ID: ProfileModelXSD, MusicXMLVersion: "4.0", Target: validation.TargetModel,
		RegistryCommit:   assessmentRegistryCommit,
		SupportedRoots:   []validation.QName{{Local: "score-partwise"}, {Local: "score-timewise"}, {Local: "opus"}},
		NormativeClosure: "Current generated MusicXML 4.0 score and opus model structure and value XSD assessment, with required model representation checks. Current field/Content order is assessed without reconstructing original XML. Finite float64 decimal fields mean the shortest decimal that reads back to the same float64. Source lexical history, encoder spellings, MusicXML prose, timeline, intent, packages, linked resources and external dictionaries are outside this profile. Reached unavailable lexical/value/default/identity capabilities remain incomplete. Composite counts are predicate instances, not research-record coverage.",
	}
	modelLocal := validation.Profile{
		ID: ProfileModelLocal, MusicXMLVersion: "4.0", Target: validation.TargetModel,
		RegistryCommit: assessmentRegistryCommit, SupportedRoots: append([]validation.QName(nil), modelXSD.SupportedRoots...),
		NormativeClosure: strings.Replace(modelXSD.NormativeClosure, "MusicXML prose,", "unselected MusicXML prose,", 1) + " Additionally includes only the exact local musical and typed same-part reference clauses bound in this catalog. Instrument-link is local-only. Date restrictions and MIDI instrument existence reuse shared predicates. Excluded clauses include timeline/effective state, broader context/intent, repeats/spans, time-only ordering, linked resources, packages, external dictionaries and all remaining normative translations. Score-only clauses are inapplicable to opus, not claims about linked opus content.",
	}
	sourceLocal := validation.Profile{
		ID: ProfileSourceLocal, MusicXMLVersion: "4.0", Target: validation.TargetSource,
		RegistryCommit: assessmentRegistryCommit, SupportedRoots: append([]validation.QName(nil), modelXSD.SupportedRoots...),
		NormativeClosure: "Original single-pass MusicXML 4.0 score and opus source XSD plus only the exact local musical and typed same-part reference clauses bound in this catalog. Instrument-link is local-only; date restrictions and MIDI instrument existence reuse shared predicates. Original syntax and format conformance are independent. Reached unsupported XSD, value, identity or DTD capabilities remain incomplete; unread external subsets, including the official score examples, cannot be accepted. No DTD trust, stripping, fetching or ignore policy is introduced. Excludes timeline/effective state, broader context/intent, repeats/spans, time-only ordering, linked resources, packages, external dictionaries and remaining normative translations. Score-only clauses are inapplicable to opus, not claims about linked opus content. Predicate-instance counts are not research-record coverage or complete MusicXML conformance.",
	}
	addRequired := func(profile *validation.Profile, key validation.RuleKey) {
		profile.Required = append(profile.Required, key)
		profile.Support = append(profile.Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
	}
	var rules []validation.RuleDefinition
	var plans []validation.PlanBinding
	schemas := make(map[validation.Assembly]*validation.CompiledSchema)
	for _, input := range []struct {
		assembly validation.Assembly
		data     validationSchemaSet
		file     string
	}{
		{validation.AssemblyMusicXML, scoreValidationSchema, "musicxml.xsd"},
		{validation.AssemblyOpus, opusValidationSchema, "opus.xsd"},
	} {
		schema, err := compileValidationSchema(input.assembly, input.data)
		if err != nil {
			return nil, err
		}
		schemas[input.assembly] = schema
		for _, operation := range []struct {
			check                       validation.XSDCheck
			title, explanation, locator string
		}{
			{validation.XSDCheckBinding, "XSD declaration and type binding", "Each selected occurrence must have an exact declaration-use and supported type binding in the selected pinned assembly.", "cvc-elt; exact content-particle witness"},
			{validation.XSDCheckDeclaration, "XSD element declaration", "Each selected element must satisfy its exact declaration and applicable value constraints.", "cvc-elt"},
			{validation.XSDCheckAttributes, "XSD attribute constraints", "Original-source or current-model attribute slots must satisfy permitted, required, prohibited, fixed and datatype constraints; absent defaults remain separate effective facts.", "cvc-complex-type 3/4; cvc-attribute"},
			{validation.XSDCheckScalar, "XSD scalar value", "Original-source strings and current strings use the shared declared domain; current numeric values satisfy value-space facets exactly without invented source spellings.", "XML Schema Part 2 datatypes and facets"},
			{validation.XSDCheckContent, "XSD content grammar", "The original-source or current model-defined field/Content sequence satisfies complete nested grammar and occurrence bounds; unavailable sequence evidence cannot prove absence.", "cvc-complex-type 2; element content particle"},
			{validation.XSDCheckIdentity, "XSD document identity", "Declaration-seeded identity entries bind successful actual-ID containing elements; missing or unique targets require complete identity evidence.", "sic-id; cvc-id"},
		} {
			plan, err := validation.NewXSDPlan(schema, operation.check)
			if err != nil {
				return nil, err
			}
			key := validation.RuleKey{ID: validation.RuleID("MX40-XSD-" + string(input.assembly) + "-" + string(operation.check)), Revision: 1}
			assembly := input.assembly
			rule := validation.RuleDefinition{Key: key, Selector: validation.Selector{Assembly: &assembly}, Scope: validation.ScopeNode,
				Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryNormative,
				Predicate: string(key.ID), Text: validation.RuleText{Title: operation.title, Explanation: operation.explanation, Message: "The selected target-specific XSD operation did not complete successfully."},
				Sources: []validation.SourceRef{
					{IDs: []string{"MusicXML-4.0-" + input.file}, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/" + input.file, Locator: "Pinned schema assembly"},
					{IDs: []string{"XML-Schema-1.0-Second-Edition"}, URL: "https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/", Locator: operation.locator},
				},
			}
			if operation.check == validation.XSDCheckScalar {
				rule.Sources[1].URL = "https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/"
				if input.assembly == validation.AssemblyMusicXML {
					rule.Sources = append(rule.Sources, validation.SourceRef{IDs: []string{"MX40-PROSE-date-no-timezone"}, URL: localSchemaSource(705), Locator: "yyyy-mm-dd restriction; one shared scalar consequence"})
				}
			}
			if operation.check == validation.XSDCheckIdentity {
				rule.Scope = validation.ScopeDocument
			}
			rules = append(rules, rule)
			plans = append(plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
			addRequired(&modelXSD, key)
			addRequired(&modelLocal, key)
			addRequired(&sourceLocal, key)
		}
		plan, err := validation.NewModelRepresentationPlan(input.assembly)
		if err != nil {
			return nil, err
		}
		key := validation.RuleKey{ID: validation.RuleID("MX40-MODEL-REPRESENTATION-" + string(input.assembly)), Revision: 1}
		assembly := input.assembly
		rule := validation.RuleDefinition{Key: key, Selector: validation.Selector{Assembly: &assembly}, Scope: validation.ScopeNode,
			Targets: []validation.Target{validation.TargetModel}, Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryModel,
			Predicate: string(key.ID), Text: validation.RuleText{Title: "Current model representation", Explanation: "Every visited model occurrence must have a completely examined supported representation, valid Content selections and representable current values. These are model findings, not original XML syntax findings.", Message: "The current model representation is invalid or unavailable."},
			Sources: []validation.SourceRef{{IDs: []string{"REQ-DEF-ADAPT-INVARIANTS"}, URL: "https://github.com/go-muse/go-musicxml/blob/511f55b4fa01dee1dd7542414ccd1eb5ad7ec6bf/docs/validation/go-contracts.md", Locator: "Direct model mapping and honest applicability"}},
		}
		rules = append(rules, rule)
		plans = append(plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
		addRequired(&modelXSD, key)
		addRequired(&modelLocal, key)
		if input.assembly == validation.AssemblyMusicXML {
			local := newLocalCatalogBuilder(schema)
			if err := local.bindLocalRelations(); err != nil {
				return nil, err
			}
			if err := local.bindTypedReferences(); err != nil {
				return nil, err
			}
			for _, rule := range local.rules {
				addRequired(&modelLocal, rule.Key)
				addRequired(&sourceLocal, rule.Key)
			}
			rules = append(rules, local.rules...)
			plans = append(plans, local.plans...)
		}
	}
	return &assessmentDefinitions{rules: rules, profiles: []validation.Profile{modelXSD, modelLocal, sourceLocal}, plans: plans, schemas: schemas}, nil
}
