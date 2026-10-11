// Package validation provides a bounded fact-stream assessment foundation.
// It does not parse XML, traverse musicxml models, fetch external resources, or
// provide a ready MusicXML conformance profile. Callers supply reviewed catalogs
// and truthful facts; reports describe only the explicitly selected obligations.
package validation

// ProfileID identifies an explicit target-specific assessment contract.
type ProfileID string

// RuleID identifies a semantic obligation, separately from research provenance.
type RuleID string

// RuleKey identifies a particular semantic revision of an obligation.
type RuleKey struct {
	ID       RuleID
	Revision uint32
}

// NodeID identifies an occurrence in one document; real nodes start at one.
type NodeID uint64

// NodeRef qualifies a node occurrence by its assessment document identity.
type NodeRef struct {
	DocumentID string
	Node       NodeID
}

// Code is an extensible, stable reason or capability identifier.
type Code string

// Target identifies the representation being assessed.
type Target string

// Supported Target values. Unknown or zero values are rejected at ingress.
const (
	TargetSource  Target = "source"  // TargetSource denotes source facts.
	TargetModel   Target = "model"   // TargetModel denotes current object facts.
	TargetPackage Target = "package" // TargetPackage denotes package facts.
)

// QName is an expanded name; Space is the namespace URI, not a prefix.
type QName struct{ Space, Local string }

// Assembly distinguishes schema assemblies, including no-namespace homonyms.
type Assembly string

// Supported Assembly values. Unknown or zero values are rejected at ingress.
const (
	AssemblyMusicXML  Assembly = "musicxml"
	AssemblyOpus      Assembly = "opus"
	AssemblySounds    Assembly = "sounds"
	AssemblyContainer Assembly = "container"
)

// ComponentKind identifies the category of a schema component.
type ComponentKind string

// Supported ComponentKind values. Unknown or zero values are rejected at ingress.
const (
	ComponentElement   ComponentKind = "element"
	ComponentType      ComponentKind = "type"
	ComponentGroup     ComponentKind = "group"
	ComponentAttribute ComponentKind = "attribute"
)

// ComponentID is an exact assembly-qualified component identity. Anonymous
// components have a nonempty Occurrence instead of a global Name.
type ComponentID struct {
	Assembly   Assembly
	Name       QName
	Kind       ComponentKind
	Occurrence string
}

// DeclarationID identifies an exact declaration or reference-use occurrence.
type DeclarationID struct {
	Assembly                    Assembly
	OwnerOccurrence, Occurrence string
}

// Selector has exactly one branch. Exact type-component and element-declaration
// selection are supported; sealed XSD plans use the exclusive Assembly branch.
// Other component kinds are unavailable.
type Selector struct {
	// Assembly selects the active compiled schema, including unbound nodes.
	// It is an exclusive branch reserved for sealed XSD plans.
	Assembly    *Assembly
	Component   *ComponentID
	Declaration *DeclarationID
}

// EvidenceState separates proven facts from absent, unavailable or invalid facts.
type EvidenceState string

// Supported EvidenceState values. Unknown or zero values are rejected at ingress.
const (
	EvidenceKnown       EvidenceState = "known"
	EvidenceAbsent      EvidenceState = "absent"
	EvidenceUnknown     EvidenceState = "unknown"
	EvidenceUnsupported EvidenceState = "unsupported"
	EvidenceInvalid     EvidenceState = "invalid"
)

// Evidence carries a value only when known. Unknown/unsupported/invalid require
// Reason; invalid also requires the exact causal decision. Absence is an adapter
// assertion and must only be emitted after the relevant scope is complete.
type Evidence[T any] struct {
	State  EvidenceState
	Value  T
	Reason Code
	Cause  *DecisionRef
}

// AtomicKind is the exact value family of an Atomic fact.
type AtomicKind string

// Supported AtomicKind values. Unknown or zero values are rejected at ingress.
const (
	AtomicString  AtomicKind = "string"
	AtomicBoolean AtomicKind = "boolean"
	AtomicInteger AtomicKind = "integer"
	AtomicDecimal AtomicKind = "decimal"
	AtomicQName   AtomicKind = "qname"
)

// Atomic is a tagged exact value. Only its selected field may be populated;
// Boolean uses Text "true" or "false". Numeric wrappers are immutable values.
type Atomic struct {
	Kind    AtomicKind
	Text    string
	Integer Integer
	Decimal Decimal
	Name    QName
}

// Scalar separates original XML-normalized lexeme evidence from current values.
type Scalar struct {
	Lexeme Evidence[string]
	Value  Evidence[Atomic]
}

// SourcePosition names an explicit coordinate system; absent positions are nil.
type SourcePosition struct {
	ByteOffset      int64
	Line, Column    int
	CoordinateSpace string
}

// Location identifies a source/model/package occurrence without inventing lines.
type Location struct {
	Target                  Target
	DocumentID, Entry, Path string
	Position                *SourcePosition
}

// Binding records an in-scope namespace binding supplied by an adapter.
type Binding struct{ Prefix, URI string }

// NodeHeader opens an element occurrence with independent type/declaration facts.
type NodeHeader struct {
	ID, Parent  NodeID
	Name        QName
	Type        Evidence[ComponentID]
	Declaration Evidence[DeclarationID]
	Location    Location
	Bindings    []Binding
}

// AttributeFact describes one exact attribute slot on the currently open owner.
type AttributeFact struct {
	Owner       NodeID
	Name        QName
	Declaration Evidence[DeclarationID]
	Presence    Evidence[bool]
	Scalar      Scalar
	Location    Location
}

// OrderKind states which order the adapter actually preserves.
type OrderKind string

// Supported OrderKind values. Unknown or zero values are rejected at ingress.
const (
	OrderSource       OrderKind = "source-order"
	OrderModelContent OrderKind = "model-content"
	OrderModelFields  OrderKind = "model-field-order"
)

// NodeEnd closes an occurrence and supplies explicit enumeration completeness.
type NodeEnd struct {
	ID                                   NodeID
	Scalar                               Scalar
	ChildrenComplete, AttributesComplete Evidence[bool]
	Order                                Evidence[OrderKind]
}

// SyntaxState is an adapter assertion; the foundation is not an XML parser.
type SyntaxState string

// Supported SyntaxState values. Unknown or zero values are rejected at ingress.
const (
	SyntaxWellFormed  SyntaxState = "well-formed"
	SyntaxMalformed   SyntaxState = "malformed"
	SyntaxIncomplete  SyntaxState = "incomplete"
	SyntaxNotAssessed SyntaxState = "not-assessed"
)

// Completion records EOF/full traversal or the real reason processing stopped.
type Completion struct {
	InputComplete bool
	Syntax        SyntaxState
	Cause         error
}

// SourceRef retains original research identifiers and offline source locators.
type SourceRef struct {
	IDs          []string
	URL, Locator string
}

// RuleText contains bounded English diagnostic text; IDs are not localized.
type RuleText struct{ Title, Explanation, Message string }

// DependencyKind distinguishes rule prerequisites from unavailable services.
type DependencyKind string

// Supported DependencyKind values. Unknown or zero values are rejected at ingress.
const (
	DependencyRule     DependencyKind = "rule"
	DependencyFact     DependencyKind = "fact"
	DependencyExternal DependencyKind = "external"
)

// RelationKind selects a prerequisite scope. Musical relations remain unavailable.
type RelationKind string

// Supported RelationKind values. Unknown or zero values are rejected at ingress.
const (
	RelationSameNode        RelationKind = "same-node"
	RelationParent          RelationKind = "parent"
	RelationDocument        RelationKind = "document"
	RelationPart            RelationKind = "part"
	RelationMusicalPosition RelationKind = "musical-position"
)

// Dependency identifies a full rule revision or a named fact/external service.
// Rule dependencies require Rule and no Key; other kinds require Key and no Rule.
type Dependency struct {
	Kind     DependencyKind
	Rule     RuleKey
	Key      string
	Relation RelationKind
}

// RuleClass specifies normative force independently from role and dependencies.
type RuleClass string

// Supported RuleClass values. Unknown or zero values are rejected at ingress.
const (
	ClassMandatory      RuleClass = "mandatory"
	ClassRecommendation RuleClass = "recommendation"
	ClassInterpretation RuleClass = "interpretation"
	ClassDefault        RuleClass = "default"
	ClassPolicy         RuleClass = "policy"
)

// RuleRole separates predicates, copied fact providers, and advisories.
type RuleRole string

// Supported RuleRole values. Unknown or zero values are rejected at ingress.
const (
	RolePredicate       RuleRole = "predicate"
	RoleContextProvider RuleRole = "context-provider"
	RoleAdvisory        RuleRole = "advisory"
)

// ScopeKind identifies the lifetime of an obligation. Node and document scopes
// execute here; later musical/package scopes can only be declared unsupported.
type ScopeKind string

// Supported ScopeKind values. Unknown or zero values are rejected at ingress.
const (
	ScopeNode     ScopeKind = "node"
	ScopeSiblings ScopeKind = "siblings"
	ScopeMeasure  ScopeKind = "measure"
	ScopePart     ScopeKind = "part"
	ScopeDocument ScopeKind = "document"
	ScopePackage  ScopeKind = "package"
)

// RuleDefinition is reviewed metadata for one semantic obligation. Category is
// independent from class/role so model and policy failures never become XSD ones.
type RuleDefinition struct {
	Key       RuleKey
	Class     RuleClass
	Role      RuleRole
	Category  DiagnosticCategory
	Targets   []Target
	Scope     ScopeKind
	Selector  Selector
	Predicate string
	Requires  []Dependency
	Sources   []SourceRef
	Text      RuleText
}

// ImplementationState describes static plan support, not instance evidence.
type ImplementationState string

// Supported ImplementationState values. Unknown or zero values are rejected at ingress.
const (
	ImplementationAvailable   ImplementationState = "available"
	ImplementationUnsupported ImplementationState = "unsupported"
)

// RuleSupport is required exactly once for every selected/transitive rule.
type RuleSupport struct {
	Rule   RuleKey
	State  ImplementationState
	Reason Code
}

// Profile explicitly identifies its complete claimed obligation inventory.
// CatalogDigest is filled by NewCatalog; a nonempty supplied digest must match.
type Profile struct {
	ID                                             ProfileID
	MusicXMLVersion, CatalogDigest, RegistryCommit string
	Target                                         Target
	Required                                       []RuleKey
	Support                                        []RuleSupport
	SupportedRoots                                 []QName
	NormativeClosure                               string
}

// Outcome distinguishes proven findings from every unresolved disposition.
type Outcome string

// Supported Outcome values. Unknown or zero values are rejected at ingress.
const (
	OutcomePass          Outcome = "pass"
	OutcomeFail          Outcome = "fail"
	OutcomeNotApplicable Outcome = "not-applicable"
	OutcomeUnknown       Outcome = "unknown"
	OutcomeUnsupported   Outcome = "unsupported"
	OutcomeBlocked       Outcome = "blocked"
)

// DiagnosticCategory separates normative conclusions from technical constraints.
type DiagnosticCategory string

// Supported DiagnosticCategory values. Unknown or zero values are rejected at ingress.
const (
	CategoryNormative  DiagnosticCategory = "normative"
	CategoryModel      DiagnosticCategory = "model"
	CategoryCapability DiagnosticCategory = "capability"
	CategoryResource   DiagnosticCategory = "resource"
	CategorySyntax     DiagnosticCategory = "syntax"
	CategoryAdvisory   DiagnosticCategory = "advisory"
	CategoryPolicy     DiagnosticCategory = "policy"
)

// DecisionRef identifies a causal rule instance without cross-document collisions.
type DecisionRef struct {
	Rule    RuleKey
	Subject NodeRef
}

// Decision records one semantic outcome; Cause never names a display path.
type Decision struct {
	Rule             RuleKey
	Subject          NodeRef
	Outcome          Outcome
	Cause            *DecisionRef
	Reason           Code
	Expected, Actual string
	Related          []Location
}

// Finding adds the category and location needed for independent diagnostics.
// Session reports retain only fail, unknown, unsupported and blocked findings;
// eligible pass and not-applicable outcomes are counted without diagnostic storage.
type Finding struct {
	Decision
	Category DiagnosticCategory
	Location Location
}

// Counts includes only normative mandatory predicate instances.
type Counts struct{ Pass, Fail, NotApplicable, Unknown, Unsupported, Blocked uint64 }

// ConformanceState is independent from assessment completion and conversion.
type ConformanceState string

// Supported ConformanceState values. Unknown or zero values are rejected at ingress.
const (
	ConformanceConformant   ConformanceState = "conformant"
	ConformanceViolated     ConformanceState = "violated"
	ConformanceUndetermined ConformanceState = "undetermined"
)

// TerminalSummary is a fixed-size-category/code infrastructure reserve, independent
// from ordinary stored findings. It cannot carry attacker-controlled text.
type TerminalSummary struct {
	Category DiagnosticCategory
	Reason   Code
}

// Report is an independently owned result. Public values are editable metadata,
// not tamper-proof certificates; Satisfied evaluates their explicit result axes.
// Findings contains only retained fail/unknown/unsupported/blocked diagnostics;
// Rules contains metadata for those findings. Pass and not-applicable outcomes
// contribute to eligible Counts without consuming diagnostic quotas or setting
// DiagnosticsTruncated. Reports do not provide an all-decisions execution trace.
type Report struct {
	Requested            bool
	Profile              Profile
	Target               Target
	Syntax               SyntaxState
	Conformance          ConformanceState
	AssessmentComplete   bool
	Counts               Counts
	BlockingProblems     uint64
	Findings             []Finding
	Rules                []RuleDefinition
	DiagnosticsTruncated bool
	InputComplete        bool
	Terminal             *TerminalSummary
}

// ConversionState reserves the independent model-conversion vocabulary. No
// conversion or source/model orchestration is implemented by this package.
type ConversionState string

// Supported ConversionState values. Unknown or zero values are rejected at ingress.
const (
	ConversionNotRequested    ConversionState = "not-requested"
	ConversionConverted       ConversionState = "converted"
	ConversionUnrepresentable ConversionState = "unrepresentable"
	ConversionFailed          ConversionState = "failed"
	ConversionIncomplete      ConversionState = "incomplete"
)

// ConversionResult carries conversion findings independently of source assessment.
type ConversionResult struct {
	State    ConversionState
	Findings []Finding
}

// ContextKey identifies a generic fact at an exact node or document scope (Node0).
// It does not infer musical time, staff, voice or effective-state precedence.
type ContextKey struct {
	Name string
	At   NodeRef
}

// ContextFact preserves copied exact values, origins and provider identities.
type ContextFact struct {
	Value     Evidence[Atomic]
	Origins   []Location
	Providers []RuleKey
}

// ContextUpdate binds a copied provider result to its document-qualified key.
type ContextUpdate struct {
	Key  ContextKey
	Fact ContextFact
}
