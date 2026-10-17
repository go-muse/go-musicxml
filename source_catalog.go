package musicxml

import (
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileSourceXSDPreview explicitly selects the developmental single-pass
// source XSD assessment scope. It covers the three generated score/opus roots,
// with reached unsupported XSD capabilities reported as incomplete. Unread
// external DOCTYPE subsets prevent acceptance, including for the three committed
// official score samples; no infoset-neutral DTD assumption is made. It excludes
// MusicXML prose, direct models, packages, linked documents and dictionaries;
// it is not a production MusicXML conformance profile or an implicit default.
const ProfileSourceXSDPreview validation.ProfileID = "musicxml-4.0/source-xsd-preview"

var sourceAssessmentEngine = sync.OnceValues(buildSourceAssessmentEngine)

func newSourceAssessmentEngine() (*validation.Engine, error) { return sourceAssessmentEngine() }

func buildSourceAssessmentEngine() (*validation.Engine, error) {
	return buildSourceXSDProfileEngine(false)
}

func buildSourceXSDProfileEngine(internalDTD bool) (*validation.Engine, error) {
	profile := validation.Profile{
		ID: ProfileSourceXSDPreview, MusicXMLVersion: "4.0", Target: validation.TargetSource,
		RegistryCommit:   "3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c",
		SupportedRoots:   []validation.QName{{Local: "score-partwise"}, {Local: "score-timewise"}, {Local: "opus"}},
		NormativeClosure: "Preview source-only execution of the pinned generated MusicXML 4.0 score and opus XSD assemblies. Counts are selected per-node composite XSD operation instances, not research-record coverage. Mandatory XML syntax is separate. Files with an unread external DOCTYPE subset cannot satisfy this preview, including the three committed official score samples. Reached unavailable metadata, opaque/open-content, DTD infoset and unsupported value/identity capabilities remain incomplete. MusicXML prose, current-model traversal, packages, links, external dictionaries and broad production conformance are excluded.",
	}
	if internalDTD {
		profile.ID = ProfileSourceInternalDTD
		profile.NormativeClosure = "Explicit source-only assessment of the pinned MusicXML 4.0 score and opus XSD assemblies after bounded processing of actually read internal DTD declarations, general entities, attribute normalization and defaults. A DOCTYPE external subset is read only from the bundled official MusicXML DTDs, selected by the official catalog's public identifiers or official system identifiers, or from a caller DTD source. Source-origin projections describe admitted element and attribute occurrences, not per-character scalar segmentation. Unrecognized external declarations, unelected parameter-entity forms and unresolved declaration well-formedness remain incomplete. No network retrieval, validating DTD processor, package adoption, model-history reconstruction, musical prose, dictionary or full XML conformance claim is made. Counts are selected operation instances, not original research-record coverage."
	}
	var rules []validation.RuleDefinition
	var plans []validation.PlanBinding
	for _, input := range []struct {
		assembly validation.Assembly
		data     validation.SchemaData
		file     string
	}{
		{validation.AssemblyMusicXML, scoreValidationSchema, "musicxml.xsd"},
		{validation.AssemblyOpus, opusValidationSchema, "opus.xsd"},
	} {
		schema, err := validation.CompileSchema(input.assembly, input.data)
		if err != nil {
			return nil, err
		}
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
			if internalDTD {
				key.Revision = 2
				if operation.check == validation.XSDCheckAttributes {
					operation.explanation = "Effective attributes after selected bounded internal-DTD normalization and defaulting must satisfy XSD permitted, required, prohibited, fixed and datatype constraints. Physical specifiedness is retained separately. Standard schema-instance attributes retain their specific contracts."
				}
			}
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
		if internalDTD {
			plan, err := validation.NewSourceOriginPlan(input.assembly)
			if err != nil {
				return nil, err
			}
			assembly := input.assembly
			key := validation.RuleKey{ID: validation.RuleID("MX40-SOURCE-ORIGIN-" + string(assembly)), Revision: 1}
			// Specified projects original physical occurrence ownership, not the
			// Infoset [specified] property of replacement-markup attributes.
			rule := validation.RuleDefinition{
				Key: key, Selector: validation.Selector{Assembly: &assembly}, Scope: validation.ScopeDocument,
				Targets: []validation.Target{validation.TargetSource}, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative,
				Predicate: string(key.ID), Text: validation.RuleText{Title: "Source occurrence provenance", Explanation: "Each admitted source element and attribute occurrence retains physical specifiedness and independently available physical, declaration and reference sites. Complete enumeration is sealed explicitly; unavailable required provenance remains incomplete.", Message: "Source occurrence provenance is incomplete."},
				Sources: []validation.SourceRef{{IDs: []string{"XML-1.0-Fifth-Edition", "XML-Infoset-Second-Edition"}, URL: "https://www.w3.org/TR/2004/REC-xml-infoset-20040204/", Locator: "2.2; 2.3; specified and origin information; bounded occurrence projection"}},
			}
			rules = append(rules, rule)
			plans = append(plans, validation.PlanBinding{ID: rule.Predicate, Plan: plan})
			profile.Required = append(profile.Required, key)
			profile.Support = append(profile.Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
		}
	}
	catalog, err := validation.NewCatalog(rules, []validation.Profile{profile}, plans)
	if err != nil {
		return nil, err
	}
	return validation.NewEngine(catalog)
}
