package musicxml

import (
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ProfileSourceLinks selects bounded nonfetching source XSD/local resource
// context, including exact local attachment and schema-hint discovery. It does
// not select timeline, package, resolver, graph or external-target assessment.
const ProfileSourceLinks validation.ProfileID = "musicxml-4.0/source-links"

// ProfileModelLinks selects the corresponding current-model XSD/local resource
// context. It never reconstructs original order or discarded xsi attributes.
const ProfileModelLinks validation.ProfileID = "musicxml-4.0/model-links"

var linkAssessmentEngine = sync.OnceValues(buildLinkAssessmentEngine)

func newLinkAssessmentEngine() (*validation.Engine, error) { return linkAssessmentEngine() }
func buildLinkAssessmentEngine() (*validation.Engine, error) {
	d, err := buildLinkAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	c, err := validation.NewCatalog(d.rules, d.profiles, d.plans)
	if err != nil {
		return nil, err
	}
	return validation.NewEngine(c)
}
func buildLinkAssessmentDefinitions() (*assessmentDefinitions, error) {
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		return nil, err
	}
	// Keep only the already selected source/model local profiles. Construction
	// uses fresh definitions and never mutates any frozen catalog or profile.
	profiles := d.profiles[:0]
	for _, p := range d.profiles {
		if p.ID == ProfileModelXSD {
			continue
		}
		if p.Target == validation.TargetSource {
			p.ID = ProfileSourceLinks
		} else {
			p.ID = ProfileModelLinks
		}
		p.NormativeClosure = "Bounded nonfetching MusicXML 4.0 source or current-model XSD/local resource context. Reuses typed local references and local Opus version/pagination semantics. Projects exact link/bookmark attachment, XLink effective values and physical/raw evidence, image MIME/dimensions/scaling advice, exact group strings, and independent schema-hint discovery pairs. Missing local attachment is absence, not a mandatory violation. Absent positions, selector/ordinal ambiguities and unavailable enumeration remain explicit. Discarded model xsi attributes are not reconstructed. Optional context gaps do not become normative failures. No URI resolution, I/O, package/graph traversal, timeline, image geometry, external dictionaries or complete MusicXML conformance claim. Existing source syntax, unread-DTD and model representation limits remain active."
		profiles = append(profiles, p)
	}
	d.profiles = profiles
	add := func(assembly validation.Assembly, id, title, explanation string, plan validation.Plan, sources []validation.SourceRef) {
		key := validation.RuleKey{ID: validation.RuleID(id), Revision: 1}
		rule := validation.RuleDefinition{Key: key, Selector: validation.Selector{Assembly: &assembly}, Scope: validation.ScopeDocument, Targets: []validation.Target{validation.TargetSource, validation.TargetModel}, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Predicate: id, Text: validation.RuleText{Title: title, Explanation: explanation, Message: "The selected nonfetching resource projection did not finish."}, Sources: sources}
		d.rules = append(d.rules, rule)
		d.plans = append(d.plans, validation.PlanBinding{ID: id, Plan: plan})
		for i := range d.profiles {
			d.profiles[i].Required = append(d.profiles[i].Required, key)
			d.profiles[i].Support = append(d.profiles[i].Support, validation.RuleSupport{Rule: key, State: validation.ImplementationAvailable})
		}
	}
	for _, assembly := range []validation.Assembly{validation.AssemblyMusicXML, validation.AssemblyOpus} {
		schema := d.schemas[assembly]
		plan, err := validation.NewLocalLinkContextPlan(schema)
		if err != nil {
			return nil, err
		}
		add(assembly, "MX40-RESOURCE-local-links/"+string(assembly), "Local resource and attachment context", "Retain exact assembly-qualified local resource templates and existing typed references without fetching or inventing target requirements.", plan, []validation.SourceRef{{IDs: []string{"MX40-PROSE-link-element-position", "MX40-PROSE-image-scaling", "MX40-PROSE-group-link-match"}, URL: localSchemaSource(2354), Locator: "Exact local MusicXML/Opus link use bindings; external resolution excluded"}})
		hints, err := validation.NewSchemaHintContextPlan(schema)
		if err != nil {
			return nil, err
		}
		add(assembly, "MX40-RESOURCE-schema-hints/"+string(assembly), "Schema hint discovery pairs", "Preserve complete pairs and unmatched trailing URI independently of the accepted list(anyURI) datatype; no hinted schema is loaded or selected.", hints, []validation.SourceRef{{IDs: []string{"REQ-DEF-XSI-SCHEMALOC", "REQ-DEF-XSI-NONAMESPACE"}, URL: "https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#schema-loc", Locator: "XML Schema 1.0 Second Edition Part 1 section 4.3.2"}})
	}
	opus, err := validation.NewOpusLayoutContextPlan(d.schemas[validation.AssemblyOpus])
	if err != nil {
		return nil, err
	}
	add(validation.AssemblyOpus, "MX40-CONTEXT-opus-pagination", "Local Opus metadata and pagination context", "Reuse the accepted local version omission default and application-dependent new-page evidence without resolving linked scores.", opus, []validation.SourceRef{{IDs: []string{"MX40-PROSE-opus-version-default", "MX40-PROSE-opus-new-page-default"}, URL: "https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/opus.xsd#L38", Locator: "Local Opus version and score pagination"}})
	return d, nil
}
