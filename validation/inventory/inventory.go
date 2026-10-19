// Package inventory exposes the pinned MusicXML 4.0 research and planning
// inventory as inspection data. It is separate from package validation because
// nothing in it is executed: its IDs are provenance, not rule keys.
package inventory

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// Snapshot is an inspection-only snapshot of the pinned MusicXML 4.0 research
// and planning dispositions. Its IDs are provenance, not executable RuleKeys.
// It does not define a profile or establish implementation or test coverage.
// Each call to MusicXML40 returns independently owned nested data.
//
// SHA256 identifies the canonical JSON payload without its sha256 member.
// SourceSHA256 identifies the sorted planning-file manifest, encoded as each
// filename, a NUL, its lowercase SHA-256 hex digest, and a newline. SourceFiles
// records the exact committed inputs, including files not fully reproduced here.
// Status, role, target, dependency and axis labels retain planning vocabulary;
// they are not runtime enums. Narrative predicate/test recipes remain in those
// inputs and the pinned registry, rather than becoming a second rule language.
// Do not infer support from a shared helper or a completed prerequisite repair.
//
// The types of this package mirror the research and planning data format, not a
// runtime contract. Their shape changes whenever that format changes, so the
// module's compatibility guarantees do not cover these types or their JSON
// layout. Treat a snapshot as inspection data.
type Snapshot struct {
	Overlaps               []Overlap               `json:"overlaps"`
	AdvisoryReviews        []AdvisoryReview        `json:"advisory_reviews"`
	Stages                 []Stage                 `json:"stages"`
	Questions              []Question              `json:"questions"`
	Version                string                  `json:"version"`
	SHA256                 string                  `json:"sha256,omitempty"`
	SourceSHA256           string                  `json:"source_sha256"`
	RegistryRepository     string                  `json:"registry_repository"`
	RegistryCommit         string                  `json:"registry_commit"`
	UpstreamMusicXMLCommit string                  `json:"upstream_musicxml_commit"`
	ExternalFiles          []File                  `json:"external_files"`
	SourceFiles            []File                  `json:"source_files"`
	Records                []Record                `json:"records"`
	Instances              []Instance              `json:"instances"`
	Checks                 []Check                 `json:"checks"`
	Capabilities           []Capability            `json:"capabilities"`
	Requirements           []Requirement           `json:"requirements"`
	Tests                  []TestContract          `json:"tests"`
	Contexts               []ContextDisposition    `json:"contexts"`
	TargetPolicies         map[string]TargetPolicy `json:"target_policies"`
	DependencySemantics    map[string]string       `json:"dependency_semantics"`
}

// File identifies exact source bytes. GitBlobSHA1 is provided only for
// the external files whose blob identities are recorded in the planning pin.
type File struct {
	Path        string `json:"path"`
	Bytes       int64  `json:"bytes"`
	SHA256      string `json:"sha256"`
	GitBlobSHA1 string `json:"git_blob_sha1,omitempty"`
}

// Record preserves one research identity and every planning link.
// RegistryURL uses the pinned catalog file with an RFC 6901 JSON Pointer
// fragment; a browser's source viewer may not navigate that fragment itself.
type Record struct {
	ResearchID      string   `json:"research_id"`
	Origin          string   `json:"origin"`
	Role            string   `json:"role"`
	Disposition     string   `json:"disposition"`
	CheckIDs        []string `json:"check_ids"`
	RegistryPointer string   `json:"registry_pointer"`
	RegistryURL     string   `json:"registry_url"`
}

// Instance is a parameter binding to a research record and a generic
// planning check. Composition and DiagnosticPolicy preserve aggregate reuse and
// residual obligations. RuntimeSupport is "unsupported": no executable binding
// is supplied by this artifact, independently of any existing XSD helper.
type Instance struct {
	ID               string           `json:"id"`
	Revision         int              `json:"revision"`
	ResearchID       string           `json:"research_id"`
	TemplateID       string           `json:"template_id"`
	Role             string           `json:"role"`
	RegistryPointer  string           `json:"registry_pointer"`
	RegistryURL      string           `json:"registry_url"`
	Composition      string           `json:"composition"`
	Status           string           `json:"status"`
	Stage            string           `json:"stage"`
	ReuseInstances   []string         `json:"reuse_instances"`
	DiagnosticPolicy string           `json:"diagnostic_policy"`
	PlanDependencies PlanDependencies `json:"plan_dependencies"`
	TargetPolicy     string           `json:"target_policy"`
	RuntimeSupport   string           `json:"runtime_support"`
}

// PlanDependencies records schema-compilation reference edges. Such
// edges may be recursive; they are not a runtime evaluation dependency DAG.
type PlanDependencies struct {
	SchemaComponent   []string `json:"schema_component"`
	LanguageSemantics []string `json:"language_semantics"`
}

// Check retains exact planning identity, evidence dependencies,
// target-specific support gates and normative provenance. Selector is source
// planning text, never a compiled runtime selector. RuntimeSupport is always
// "unsupported" because the inventory does not bind any executable plan.
type Check struct {
	ExpectedOutcomes      *ExpectedOutcomes         `json:"expected_outcomes,omitempty"`
	ResearchClauses       map[string]string         `json:"research_clauses,omitempty"`
	ID                    string                    `json:"id"`
	ResearchIDs           []string                  `json:"research_ids,omitempty"`
	Revision              int                       `json:"revision"`
	Role                  string                    `json:"role"`
	Inputs                []string                  `json:"inputs"`
	Selector              string                    `json:"selector"`
	Targets               []string                  `json:"targets"`
	Capabilities          []string                  `json:"capabilities"`
	Dependencies          []string                  `json:"dependencies"`
	DependencyModes       map[string]DependencyMode `json:"dependency_modes,omitempty"`
	NormativeProvenance   []string                  `json:"normative_provenance"`
	TestContracts         []string                  `json:"test_contracts"`
	Stage                 string                    `json:"stage"`
	Status                string                    `json:"status"`
	Closure               string                    `json:"closure,omitempty"`
	ReadinessGate         string                    `json:"readiness_gate,omitempty"`
	CapabilityIDs         []string                  `json:"capability_ids"`
	SemanticStatus        string                    `json:"semantic_status"`
	ImplementationBinding string                    `json:"implementation_binding"`
	ModelLimit            string                    `json:"model_limit,omitempty"`
	ModelEvidence         *ModelEvidence            `json:"model_evidence,omitempty"`
	RuntimeSupport        string                    `json:"runtime_support"`
}

// DependencyMode preserves the source/current-model prerequisite
// distinction and the restriction against using it as a global assessment gate.
type DependencyMode struct {
	Source         string `json:"source"`
	Model          string `json:"model"`
	NotAGlobalGate string `json:"not_a_global_gate"`
}

// ModelEvidence preserves the independent current-model limitations
// for source spelling, value space, structure and namespace history.
type ModelEvidence struct {
	SourceLexicalClauses string `json:"source_lexical_clauses"`
	ValueSpaceClauses    string `json:"value_space_clauses"`
	StructureClauses     string `json:"structure_clauses"`
	NamespaceClauses     string `json:"namespace_clauses"`
	ReadinessGate        string `json:"readiness_gate"`
}

// TargetPolicy preserves an instance's exact source/model evidence
// policy, without supplying either adapter or implying equal capabilities.
type TargetPolicy struct {
	Source string `json:"source"`
	Model  string `json:"model"`
}

// Capability preserves an unbound evidence-provider gate, including
// reciprocal check links, implementation owners and missing-evidence outcomes.
type Capability struct {
	ID                     string   `json:"id"`
	Label                  string   `json:"label"`
	Kind                   string   `json:"kind"`
	ProviderStatus         string   `json:"provider_status"`
	ProviderRequirementIDs []string `json:"provider_requirement_ids"`
	CheckIDs               []string `json:"check_ids"`
	Targets                []string `json:"targets"`
	Closure                string   `json:"closure"`
	MissingOutcome         string   `json:"missing_outcome"`
}

// Requirement identifies a deferred implementation obligation, not a
// MusicXML normative rule. Its status remains the source planning disposition.
type Requirement struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	Role                string   `json:"role"`
	Inputs              []string `json:"inputs"`
	Applicability       string   `json:"applicability"`
	Targets             []string `json:"targets"`
	Dependencies        []string `json:"dependencies"`
	NormativeProvenance []string `json:"normative_provenance"`
	TestContracts       []string `json:"test_contracts"`
	Stage               string   `json:"stage"`
	Status              string   `json:"status"`
	Acceptance          []string `json:"acceptance"`
	SourceBullets       []string `json:"source_bullets"`
	SourceInventory     string   `json:"source_inventory"`
	ResearchIDs         []string `json:"research_ids,omitempty"`
}

// TestContract records planned test obligations and their five-axis
// dispositions. It contains no executed test results or coverage claim.
type TestContract struct {
	ID                    string            `json:"id"`
	CheckIDs              []string          `json:"check_ids,omitempty"`
	RequirementIDs        []string          `json:"requirement_ids,omitempty"`
	Targets               []string          `json:"targets"`
	Status                string            `json:"status"`
	AxisStatus            map[string]string `json:"axis_status"`
	CaseInstantiationGate string            `json:"case_instantiation_gate,omitempty"`
}

// ContextDisposition preserves role, fact/advisory meaning and
// unfinished interpretation/translation gates for a context research record.
type ContextDisposition struct {
	ResearchID   string   `json:"research_id"`
	Role         string   `json:"role"`
	Disposition  string   `json:"disposition"`
	Rationale    string   `json:"rationale"`
	Produces     *string  `json:"produces"`
	Dependencies []string `json:"dependencies"`
	Closure      string   `json:"closure"`
	Translation  string   `json:"translation"`
}

// MusicXML40 returns an independently owned inspection snapshot of all
// 2,560 pinned research records and their exact planned support dispositions.
// It only reads compiled data: no filesystem, network, registry synchronization,
// adapter selection, semantic execution or broad ready profile is involved.
func MusicXML40() Snapshot {
	compressed, err := base64.StdEncoding.DecodeString(musicXML40InventoryBase64)
	if err != nil {
		panic("inventory: corrupt compiled inventory encoding: " + err.Error())
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		panic("inventory: corrupt compiled inventory compression: " + err.Error())
	}
	data, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil {
		panic("inventory: corrupt compiled inventory payload: " + err.Error())
	}
	if closeErr != nil {
		panic("inventory: corrupt compiled inventory trailer: " + closeErr.Error())
	}
	var inv Snapshot
	if err := json.Unmarshal(data, &inv); err != nil {
		panic("inventory: corrupt compiled inventory JSON: " + err.Error())
	}
	inv.SHA256 = musicXML40InventorySHA256
	return inv
}

// ExpectedOutcomes preserves either the original outcome descriptions
// or the original context/advisory outcome text. Exactly one representation is
// populated in the compiled inventory; its JSON form matches the source plan.
type ExpectedOutcomes struct {
	Description string
	Outcomes    map[string]string
}

// MarshalJSON retains the original string-or-object planning representation.
func (p ExpectedOutcomes) MarshalJSON() ([]byte, error) {
	if p.Description != "" {
		if p.Outcomes != nil {
			return nil, fmt.Errorf("validation: ambiguous inventory outcome representation")
		}
		return json.Marshal(p.Description)
	}
	return json.Marshal(p.Outcomes)
}

// UnmarshalJSON reads a planning outcome description or outcome-description map.
func (p *ExpectedOutcomes) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return fmt.Errorf("validation: empty inventory outcomes")
	}
	var next ExpectedOutcomes
	switch data[0] {
	case '"':
		if err := json.Unmarshal(data, &next.Description); err != nil {
			return err
		}
	case '{':
		if err := json.Unmarshal(data, &next.Outcomes); err != nil {
			return err
		}
	default:
		return fmt.Errorf("validation: inventory outcomes must be text or an object")
	}
	*p = next
	return nil
}

// Overlap preserves the exact limited shared/additional clause scope.
// Provenance overlap is never automatically treated as predicate equivalence.
type Overlap struct {
	ID                          string   `json:"id"`
	ResearchID                  string   `json:"research_id"`
	Kind                        string   `json:"kind"`
	SharedCondition             string   `json:"shared_condition"`
	AdditionalCondition         string   `json:"additional_condition"`
	XSDResearchIDs              []string `json:"xsd_research_ids"`
	Disposition                 string   `json:"disposition"`
	Justification               string   `json:"justification"`
	AdditionalConditionCheckIDs []string `json:"additional_condition_check_ids"`
	AdditionalConditionSource   string   `json:"additional_condition_source"`
}

// AdvisoryReview retains compound advisory/context reuse decisions.
// ReuseCheckIDs may identify checks or XSD instances, following the source plan.
// Reused clauses do not erase primary checks or add normative rejection rules.
type AdvisoryReview struct {
	ResearchID            string   `json:"research_id"`
	Decision              string   `json:"decision"`
	RelatedResearchIDs    []string `json:"related_research_ids"`
	RelatedXSDResearchIDs []string `json:"related_xsd_research_ids"`
	Rationale             string   `json:"rationale"`
	NormativeProvenance   []string `json:"normative_provenance"`
	ReuseCheckIDs         []string `json:"reuse_check_ids"`
	PrimaryCheckIDs       []string `json:"primary_check_ids"`
	ReuseScope            string   `json:"reuse_scope"`
}

// Stage identifies implementation sequencing and batch acceptance,
// separately from individual runtime evidence or interpretation dependencies.
type Stage struct {
	ID                       string   `json:"id"`
	Title                    string   `json:"title"`
	DependsOn                []string `json:"depends_on"`
	Acceptance               []string `json:"acceptance"`
	DependencyInterpretation string   `json:"dependency_interpretation,omitempty"`
	ScopeNote                string   `json:"scope_note,omitempty"`
}

// Question records an unresolved research decision and its exact
// closure gate. A proposed interpretation is not authorization for rejection.
// RegistryURL selects the pinned issues file using a JSON Pointer fragment.
type Question struct {
	ID                     string   `json:"id"`
	RegistryPointer        string   `json:"registry_pointer"`
	RegistryURL            string   `json:"registry_url"`
	Description            string   `json:"description"`
	Disposition            string   `json:"disposition"`
	ProposedInterpretation string   `json:"proposed_interpretation"`
	HardErrorBasis         bool     `json:"hard_error_basis"`
	Requirements           []string `json:"requirements"`
	ContextualResearchIDs  []string `json:"contextual_research_ids"`
	Closure                string   `json:"closure"`
}
