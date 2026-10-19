package musicxml

import (
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileSourceContainer assesses original container XML against the pinned
// container XSD assembly and projects exact ordered rootfile metadata.
// Package entries, root targets and ZIP rules are outside
// this source-only profile. It does not add a Document model root.
const ProfileSourceContainer validation.ProfileID = "musicxml-4.0/source-container"

// ProfileSourceSounds assesses original sounds XML against the pinned sounds
// XSD assembly and projects exact directional sound-mapping evidence. Dictionary
// membership, application-library selection and playback are outside its scope.
// It does not add a Document model root.
const ProfileSourceSounds validation.ProfileID = "musicxml-4.0/source-sounds"

var auxiliarySourceAssessmentEngine = sync.OnceValues(buildAuxiliarySourceAssessmentEngine)

func newAuxiliarySourceAssessmentEngine() (*validation.Engine, error) {
	return auxiliarySourceAssessmentEngine()
}

func buildAuxiliarySourceAssessmentEngine() (*validation.Engine, error) {
	d, err := buildAuxiliarySourceAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	catalog, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		return nil, err
	}
	return validation.NewEngine(catalog)
}

func buildAuxiliarySourceAssessmentDefinitions() (*assessmentDefinitions, error) {
	schemas := make(map[validation.Assembly]*validation.CompiledSchema)
	var rules []validation.RuleDefinition
	var plans []validation.PlanBinding
	var profiles []validation.Profile
	for _, input := range []struct {
		assembly validation.Assembly
		data     validationSchemaSet
		file     string
		id       validation.ProfileID
		root     string
	}{
		{validation.AssemblyContainer, containerValidationSchema, "container.xsd", ProfileSourceContainer, "container"},
		{validation.AssemblySounds, soundsValidationSchema, "sounds.xsd", ProfileSourceSounds, "sounds"},
	} {
		profile := validation.Profile{ID: input.id, MusicXMLVersion: "4.0", Target: validation.TargetSource,
			RegistryCommit:   "3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c",
			SupportedRoots:   []validation.QName{{Local: input.root}},
			NormativeClosure: "Source-only execution of the pinned MusicXML 4.0 " + string(input.assembly) + " XSD assembly. Counts are composite operation instances, not research coverage. Mandatory XML syntax, unread external-DTD and reached unsupported-capability limits remain active. No Document model, package, linked-target, dictionary or playback conformance is asserted.",
		}
		schema, err := compileValidationSchema(input.assembly, input.data)
		if err != nil {
			return nil, err
		}
		schemas[input.assembly] = schema
		for _, operation := range []struct {
			check                       validation.XSDCheck
			title, explanation, locator string
		}{
			{validation.XSDCheckBinding, "XSD declaration and actual-type binding", "Each assessed source occurrence must have an exact declaration-use and a supported actual-type binding in the selected schema assembly.", "cvc-elt; type derivation and exact content-particle witness"},
			{validation.XSDCheckDeclaration, "XSD element declaration", "The element must satisfy its declaration, including abstract, nil, actual-type nomination, default and fixed constraints.", "cvc-elt"},
			{validation.XSDCheckAttributes, "XSD attribute constraints", "Physical attributes must satisfy permitted, required, prohibited, fixed and datatype constraints. Standard schema-instance attributes retain their specific contracts.", "cvc-complex-type 3/4; cvc-attribute; schema-instance attributes"},
			{validation.XSDCheckScalar, "XSD scalar value", "An applicable scalar must satisfy its selected lexical/value domain, ordered union/list semantics, whitespace and inherited facets using exact values.", "cvc-datatype-valid; XML Schema Part 2 datatypes and facets"},
			{validation.XSDCheckContent, "XSD content grammar", "The original child sequence and direct character content must satisfy the actual type's complete nested content grammar and occurrence bounds.", "cvc-complex-type 2; element content particle"},
			{validation.XSDCheckIdentity, "XSD document identity", "Declaration-seeded identity entries resolve to their successful actual-ID containing-element bindings. Missing bindings require complete evidence; known duplicates remain violations.", "sic-id; cvc-id"},
		} {
			plan, err := validation.NewXSDPlan(schema, operation.check)
			if err != nil {
				return nil, err
			}
			key := validation.RuleKey{ID: validation.RuleID("MX40-SOURCE-XSD-" + string(input.assembly) + "-" + string(operation.check)), Revision: 1}
			assembly := input.assembly
			rule := validation.RuleDefinition{Key: key, Selector: validation.Selector{Assembly: &assembly}, Scope: validation.ScopeNode,
				Targets: []validation.Target{validation.TargetSource}, Class: validation.ClassMandatory, Role: validation.RolePredicate, Category: validation.CategoryNormative,
				Predicate: string(key.ID), Text: validation.RuleText{Title: operation.title, Explanation: operation.explanation, Message: "The selected XSD composite operation did not complete successfully."},
				Sources: []validation.SourceRef{
					{IDs: []string{"MusicXML-4.0-" + input.file}, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/" + input.file, Locator: "Pinned schema assembly"},
					{IDs: []string{"XML-Schema-1.0-Second-Edition"}, URL: "https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/", Locator: operation.locator},
				},
			}
			if operation.check == validation.XSDCheckScalar {
				rule.Sources[1].URL = "https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/"
			}
			if operation.check == validation.XSDCheckIdentity {
				rule.Scope = validation.ScopeDocument
			}
			rules = append(rules, rule)
			plans = append(plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
			profile.Required = append(profile.Required, key)
			profile.Support = append(profile.Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
		}

		if input.assembly == validation.AssemblyContainer {
			plan, err := validationNewContainerMetadataContextPlan(schema)
			if err != nil {
				return nil, err
			}
			assembly := validation.AssemblyContainer
			key := validation.RuleKey{ID: "MX40-CONTEXT-container-metadata", Revision: 1}
			rule := validation.RuleDefinition{Key: key, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource}, Scope: validation.ScopeDocument, Selector: validation.Selector{Assembly: &assembly}, Predicate: string(key.ID), Text: validation.RuleText{Title: "Ordered container metadata evidence", Explanation: "Retain exact container, rootfiles and rootfile source occurrences with prerequisite-qualified inventory, order and raw/effective token fields. Provider completion makes no package or linked-target conformance assertion.", Message: "Container metadata projection did not finish."}, Sources: []validation.SourceRef{{IDs: []string{"MusicXML-4.0-container.xsd"}, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/container.xsd", Locator: "container, rootfiles and rootfile declarations"}}}
			rules = append(rules, rule)
			plans = append(plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
			profile.Required = append(profile.Required, key)
			profile.Support = append(profile.Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
			profile.NormativeClosure += " Exact ordered container/rootfiles/rootfile metadata is independently owned. Raw lexemes, effective token values, absence and invalid prerequisites remain distinct; incomplete child bindings never certify complete inventory."
		}
		if input.assembly == validation.AssemblySounds {
			plan, err := validationNewSoundsMappingContextPlan(schema)
			if err != nil {
				return nil, err
			}
			assembly := validation.AssemblySounds
			key := validation.RuleKey{ID: "MX40-CONTEXT-sounds-mapping", Revision: 1}
			rule := validation.RuleDefinition{Key: key, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource}, Scope: validation.ScopeDocument, Selector: validation.Selector{Assembly: &assembly}, Predicate: string(key.ID), Text: validation.RuleText{Title: "Directional sound mapping evidence", Explanation: "Retain sound reverse preferences and any, solo and ensemble forward preferences, preserving absence, alternatives and the exact string/NMTOKEN domains. Provider completion does not select a mapping or assert dictionary membership.", Message: "Sound mapping projection did not finish."}, Sources: []validation.SourceRef{{IDs: []string{"MX40-PROSE-sounds-primary-direction", "MX40-PROSE-sounds-any-primary", "MX40-PROSE-sounds-solo-primary", "MX40-PROSE-sounds-ensemble-primary"}, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/sounds.xsd#L54", Locator: "sound, any, solo and ensemble mapping annotations"}}}
			rules = append(rules, rule)
			plans = append(plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
			profile.Required = append(profile.Required, key)
			profile.Support = append(profile.Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
			profile.NormativeClosure += " Exact sound/any/solo/ensemble directional mapping context is independently owned. Omitted primary has no default; competing preferences and empty mappings remain data."
		}
		hints, err := validationNewSchemaHintContextPlan(schema)
		if err != nil {
			return nil, err
		}
		assembly := input.assembly
		hintKey := validation.RuleKey{ID: validation.RuleID("MX40-RESOURCE-schema-hints/" + string(assembly)), Revision: 1}
		hintRule := validation.RuleDefinition{Key: hintKey, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource}, Scope: validation.ScopeDocument, Selector: validation.Selector{Assembly: &assembly}, Predicate: string(hintKey.ID), Text: validation.RuleText{Title: "Schema hint discovery pairs", Explanation: "Retain source discovery pairs independently of the accepted XSD list(anyURI) datatype; never fetch or select hinted schemas.", Message: "Schema hint projection did not finish."}, Sources: []validation.SourceRef{{IDs: []string{"REQ-DEF-XSI-SCHEMALOC", "REQ-DEF-XSI-NONAMESPACE"}, URL: "https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#schema-loc", Locator: "XML Schema 1.0 Second Edition Part 1 section 4.3.2"}}}
		rules = append(rules, hintRule)
		plans = append(plans, validation.PlanBinding{ID: hintRule.Predicate, Plan: hints})
		profile.Required = append(profile.Required, hintKey)
		profile.Support = append(profile.Support, validation.RuleSupport{Rule: hintKey, State: validation.ImplementationAvailable})
		profile.NormativeClosure += " Source schema-hint discovery retains complete pairs and unmatched trailing URI without changing the datatype or performing retrieval."
		profiles = append(profiles, profile)
	}
	return &assessmentDefinitions{rules: rules, profiles: profiles, plans: plans, schemas: schemas}, nil
}
