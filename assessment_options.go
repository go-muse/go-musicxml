package musicxml

import "github.com/go-muse/go-musicxml/validation"

// SourceOptions configures InspectXML and the assessment of DecodeAssessed.
// An empty Profile selects the full semantic source profile, with the bundled
// required dictionaries unless Dictionaries is set; a non-empty Profile must
// be known and fit the entry point. DecodeAssessed fixes its validation lines
// and accepts only an empty Profile or ProfileSourceLine. Zero budget fields
// select defaults.
//
// Source facts have fixed identity limits: 1024 UTF-8 bytes per local name or
// prefix, 4096 per namespace URI, and 65536 in-scope namespace bindings.
// Exceeding these is incomplete, never malformed XML. Buffered-byte accounting
// bounds admitted retained representations and logical model-candidate
// storage, not exact Go heap use or garbage-collector overhead.
type SourceOptions struct {
	Profile SourceProfile
	Budgets validation.Budgets
	// DTD supplies caller DTD bytes ahead of the bundled official MusicXML
	// DTDs to profiles that read DTD declarations. Nil uses only the bundled
	// DTDs.
	DTD *DTDOptions
	// Dictionaries selects pinned dictionary capabilities, which the
	// dictionary and semantic profiles consult. With an empty Profile, nil or
	// a value with a nil Source and no selections selects the bundled data
	// those profiles require; with an explicit profile, nil keeps the
	// profile's dictionary-free behaviour.
	Dictionaries *DictionaryOptions
	// Context selects the context families kept in Report.Context of every
	// returned report, including both reports of DecodeAssessed. The zero
	// value keeps none; validation.ContextFamilyAll keeps the complete
	// projection. Every decision, count and finding is the same whatever the
	// selection; an unselected record is computed and checked but not kept,
	// which lowers the buffered storage the call needs.
	Context validation.ContextFamilies
}

func (o SourceOptions) internal() assessmentOptions {
	return assessmentOptions{Profile: o.Profile.ID(), Budgets: o.Budgets, DTD: o.DTD, Dictionaries: o.Dictionaries, Context: o.Context}
}

// ModelOptions configures AssessModel. An empty Profile selects the full
// semantic model profile, with the bundled required dictionaries unless
// Dictionaries is set; a non-empty Profile must be known. Zero budget fields
// select defaults; buffered-byte accounting is logical, as for SourceOptions.
type ModelOptions struct {
	Profile ModelProfile
	Budgets validation.Budgets
	// Dictionaries selects pinned dictionary capabilities, as for
	// SourceOptions.Dictionaries.
	Dictionaries *DictionaryOptions
	// Context selects the context families kept in Report.Context, as for
	// SourceOptions.
	Context validation.ContextFamilies
}

func (o ModelOptions) internal() assessmentOptions {
	return assessmentOptions{Profile: o.Profile.ID(), Budgets: o.Budgets, Dictionaries: o.Dictionaries, Context: o.Context}
}

// GraphOptions configures the graph of InspectXMLWithResources and
// AssessModelWithResources: its profile, the budgets of the whole run, caller
// DTDs and dictionaries. An empty Profile selects ProfileResourceGraphSemantics
// with the bundled required dictionaries unless Dictionaries is set.
type GraphOptions struct {
	Profile      GraphProfile
	Budgets      validation.Budgets
	DTD          *DTDOptions
	Dictionaries *DictionaryOptions
	// Context selects the context families kept in Report.Context of the
	// returned report and of every resource report, as for SourceOptions.
	// The graph's resource snapshots keep the records its package rules read
	// whatever the selection.
	Context validation.ContextFamilies
}

func (o GraphOptions) internal() assessmentOptions {
	return assessmentOptions{Profile: o.Profile.ID(), Budgets: o.Budgets, DTD: o.DTD, Dictionaries: o.Dictionaries, Context: o.Context}
}

// MXLGraphOptions configures the package graph of InspectMXL,
// AssessMXLPackage and DecodeMXLAssessed, as GraphOptions does for a document
// graph. An empty Profile selects the entry point's semantic MXL profile.
type MXLGraphOptions struct {
	Profile      MXLProfile
	Budgets      validation.Budgets
	DTD          *DTDOptions
	Dictionaries *DictionaryOptions
	// Context selects the context families of the returned reports, as for
	// GraphOptions.
	Context validation.ContextFamilies
}

func (o MXLGraphOptions) internal() assessmentOptions {
	return assessmentOptions{Profile: o.Profile.ID(), Budgets: o.Budgets, DTD: o.DTD, Dictionaries: o.Dictionaries, Context: o.Context}
}

// LinkedAssessmentOptions selects an explicit graph profile and one shared set
// of finite limits for primary input, children, metadata and graph operations.
// Empty Dependencies invokes no resolver. Nil (including typed-nil) Resolver
// leaves selected external dependencies unavailable. No schema hint is fetched.
// The caller must not mutate options or input models during assessment.
type LinkedAssessmentOptions struct {
	Graph        GraphOptions
	Identity     ResourceIdentity
	Resolver     validation.ResourceResolver
	Dependencies []ResourceDependency
}

func (o LinkedAssessmentOptions) internal() linkedAssessmentOptions {
	return linkedAssessmentOptions{Graph: o.Graph.internal(), Identity: o.Identity, Resolver: o.Resolver, Dependencies: o.Dependencies}
}

// MXLAssessmentOptions adds the existing independent archive processing caps.
// MXL does not create another account; Graph.Budgets bounds the complete run.
// MXL.MaxXMLDepth independently caps each represented XML document in an MXL
// context (zero uses 256), without restricting graph hops between shallow XML.
// Original raw ZIP evidence is available only to InspectMXL, never reconstructed
// from the current object passed to AssessMXLPackage.
type MXLAssessmentOptions struct {
	Graph        MXLGraphOptions
	Identity     ResourceIdentity
	Resolver     validation.ResourceResolver
	Dependencies []ResourceDependency
	MXL          MXLOptions
}

func (o MXLAssessmentOptions) internal() mxlAssessmentOptions {
	return mxlAssessmentOptions{Graph: o.Graph.internal(), Identity: o.Identity, Resolver: o.Resolver, Dependencies: o.Dependencies, MXL: o.MXL}
}
