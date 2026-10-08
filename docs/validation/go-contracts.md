# Go validation foundation: proposed contracts

Status: **DESIGN FOR REVIEW**, not an implemented or frozen API. This proposal
refines [the architecture](architecture.md); it adds no production validator,
generated files, module dependency, or runtime registry download. All identifiers
and signatures below are candidates for a later implementation PR.

Baseline: library `f1f77ae2820a4ce1d97d617c315b4a6d6a08903b` (product code is
unchanged from `e486735cd6e4537e839ff67704b7768a9df3bdb3`). Requirements remain
in the external [registry snapshot][registry] at
`3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c`. Its research IDs remain provenance,
not an automatically executable catalog or a promise of complete conformance.

## 1. Boundaries and dependency direction

Proposed package placement is deliberately small:

| Owner | Responsibility and allowed dependencies |
| --- | --- |
| `validation` subpackage | Public immutable-by-contract report, profile, budget, resolver and fact value types; compiled catalog, operator dispatch and session engine. Imports standard library only; never imports root `musicxml`, XML decoder, generated model types, or network clients. |
| Root `musicxml` package | Public orchestration entry points, source observer, direct model walker and compatibility wrappers. Imports `validation`; owns mapping to generated models and existing decoder/MXL helpers. |
| `internal/xsdgen` | Build-time generation of schema component/adapter metadata for review. Does not fetch or execute the research registry at runtime. |
| Caller or future package adapter | Supplies explicitly authorized, pinned external facts/resources; imports core contracts, never needs engine internals. MXL adapter can reuse root package facilities. |

Within `validation`: catalog data depends only on contract types; operators
consume facts/context; engine depends on catalog and operators. Neither catalog
nor operators depends on an adapter. A rule is implemented once; adapters report
facts and evidence limits, not their own copy of MusicXML predicates. Initial
implementation may keep these responsibilities in fewer files, but must preserve
this import direction. This PR does not move existing code into new packages.

A compiled catalog and engine are immutable and shareable across goroutines.
Each assessment owns a new, single-goroutine session, indexes, context store,
resolver cache and budget counters. No global mutable evaluation state. The
caller must not mutate a model or resolver's backing data during assessment.

## 2. Proposed entry points and return rules

These signatures belong to root `musicxml`; `v` denotes the proposed
`github.com/go-muse/go-musicxml/validation` subpackage. `Document` and
`DecodeOptions` are the existing root-package types.

```go
// Proposed additions, not declarations implemented by this PR.
type AssessmentOptions struct {
    Profile v.ProfileID
    Budgets v.Budgets
    Resolver v.Resolver // nil means no external evidence
}

type DecodeAssessmentOptions struct {
    Decode DecodeOptions
    Assessment *AssessmentOptions // nil is permissive; non-nil requests strict source assessment
}

type DecodeResult struct {
    Document Document
    Source v.Report
    Conversion v.ConversionResult
}

func InspectXML(ctx context.Context, r io.Reader, opts AssessmentOptions) (v.Report, error)
func Assess(ctx context.Context, doc Document, opts AssessmentOptions) (v.Report, error)
func DecodeAssessed(ctx context.Context, r io.Reader, opts DecodeAssessmentOptions) (DecodeResult, error)
```

- `InspectXML` checks source independently of Go representability and does not
  build a model. A profile may select any of the five known schema roots,
  including `container` and `sounds` as a future contract; those do not become
  new `Document` types. The first increment is limited to the existing musicxml
  and opus assemblies (three roots). Container/sounds require new generator
  directives, generated schema data and tests before they are supported; until
  then their profiles/roots explicitly report unsupported.
- `Assess` traverses the current model directly. It never calls `Encode`, XML
  marshal, or an XML parser, including in an alleged exportability fast path.
- `DecodeAssessed` parses once, optionally assesses source, and maps the same
  stream to a model. Default `Assessment=nil` is permissive MusicXML
  decoding, with mandatory XML syntax, namespace and safety checks still active.
  `Source.Requested=false` is an explicit unassessed result, never a pass.
- `InspectXML`/`Assess` return ordinary rule violations in the report with a nil
  operation error. Invalid options, unsupported input root, fatal syntax/I/O,
  cancellation, and resource exhaustion return a non-nil error and a partial
  report. A missing optional resolver yields incomplete findings, not an I/O
  error. Callers must use `Report.Satisfied()`, not `err == nil`, for acceptance.
- Strict decode returns a non-nil assessment error when its source report is
  violated **or incomplete**, and a conversion error if mapping fails.
  `DecodeResult.Document` is non-nil only on successful decode with a nil error;
  no half-built or rejected model escapes. Reports remain available on error.
- Existing `Decode`, typed decode helpers, `DecodeWithOptions`, MXL helpers and
  `Validate` retain their signatures. A later compatibility PR can route them
  through this foundation. This proposal does not add fields to the current
  `DecodeOptions` (which could break unkeyed external literals).

`InspectXML`, `Assess`, and strict decode require a nonempty explicit profile
ID; empty/unknown/incompatible IDs fail before input is read. This prevents an
unfinished broad profile from becoming an accidental default. A non-nil pointer
with an empty profile is invalid. `DecodeAssessed` snapshots non-nil options at
entry; callers must not mutate them concurrently. With nil `Assessment`, it does
not run assessment, load a catalog, or invoke a resolver, and follows the same
permissive transport/conversion contract as `DecodeWithOptions`. The pointer is
the sole strict-mode discriminator; there is no independent conflicting flag.
Existing default Decode behavior and its DecodeOptions remain
unchanged. A future named default profile requires a separate compatibility
and coverage decision.

No API closes the caller-owned source reader or rewinds it. One call consumes
one reader; a caller making two independent calls must supply two readers.
`context.Context` is non-nil. Cancellation is checked between reads, tokens,
operator steps and resolver calls; it cannot interrupt arbitrary blocking
`io.Reader.Read`. Hard deadlines require caller-provided cancellable I/O.

## 3. Shared types: identity, evidence, and values

The following core declarations are a compile-checkable contract sketch, not
an implementation. Named enum types and constants below define the vocabulary;
Go still permits arbitrary conversions and does not enforce exhaustive switches.
Constructors, catalog loading and every public/session ingress must reject unknown
or zero enum values unless explicitly allowed. Exported struct literals cannot
bypass that validation. `Code`, IDs, fingerprints and registry keys are extensible
identifiers, not enums; missing required identifiers are also rejected.

```go
type ProfileID string
type RuleID string
type RuleKey struct { ID RuleID; Revision uint32 }
type NodeID uint64 // real nodes start at 1; zero is scope/no-parent only
type NodeRef struct { DocumentID string; Node NodeID }
type Code string
type Target string
const (
    TargetSource Target = "source"
    TargetModel Target = "model"
    TargetPackage Target = "package"
)

type QName struct { Space, Local string }
type ComponentKind string
const (
    ComponentElement ComponentKind = "element"
    ComponentType ComponentKind = "type"
    ComponentGroup ComponentKind = "group"
    ComponentAttribute ComponentKind = "attribute"
)
type Assembly string
const (
    AssemblyMusicXML Assembly = "musicxml"
    AssemblyOpus Assembly = "opus"
    AssemblySounds Assembly = "sounds"
    AssemblyContainer Assembly = "container"
)
type ComponentID struct {
    Assembly Assembly // schema assembly identity, not namespace alone
    Name QName
    Kind ComponentKind
    Occurrence string // required identity for an anonymous component
}
type DeclarationID struct {
    Assembly Assembly
    OwnerOccurrence string // stable ID of named or anonymous declaring component
    Occurrence string // exact local/global declaration or reference-use occurrence
}
type Selector struct {
    Component *ComponentID // global type/component selection
    Declaration *DeclarationID // exact declaration/use-site selection
}

type EvidenceState string
const (
    EvidenceKnown EvidenceState = "known"
    EvidenceAbsent EvidenceState = "absent"
    EvidenceUnknown EvidenceState = "unknown"
    EvidenceUnsupported EvidenceState = "unsupported"
    EvidenceInvalid EvidenceState = "invalid"
)
type Evidence[T any] struct {
    State EvidenceState
    Value T // meaningful only when State == known
    Reason Code
}

type AtomicKind string
const (
    AtomicString AtomicKind = "string"
    AtomicBoolean AtomicKind = "boolean"
    AtomicInteger AtomicKind = "integer"
    AtomicDecimal AtomicKind = "decimal"
    AtomicQName AtomicKind = "qname"
)
type Integer struct { value *big.Int } // opaque immutable parsed value
type Decimal struct { value *big.Rat } // opaque immutable exact decimal value
// Proposed constructor/accessor signatures; implementation omitted.
func NewInteger(value *big.Int) (Integer, error)
func NewDecimal(value *big.Rat) (Decimal, error)
func (n Integer) Cmp(other Integer) int
func (n Integer) String() string
func (n Decimal) Cmp(other Decimal) int
func (n Decimal) String() string

type Atomic struct {
    Kind AtomicKind
    Text string // string value or canonical boolean; not the numeric store
    Integer Integer
    Decimal Decimal
    Name QName // qname: already expanded in the subject's namespace scope
}
type Scalar struct {
    Lexeme Evidence[string] // XML-normalized character data, before XSD whitespace
    Value Evidence[Atomic]
}

type SourcePosition struct {
    ByteOffset int64
    Line, Column int
    CoordinateSpace string // original-bytes or a named transcoded stream
}
type Location struct {
    Target Target
    DocumentID, Entry, Path string
    Position *SourcePosition // nil when unavailable; never model-invented
}
```

`known` and `absent` are evidence, not truth values. `unknown` means an applicable
fact cannot be established for this instance; `unsupported` means the adapter or
operator lacks that capability. `invalid` means a prerequisite is already
invalid, with a traceable cause. Absence must be proven from a completed scope,
not inferred because an adapter omitted a fact.

Exact integers and decimals use opaque immutable parsed `big.Int`/`big.Rat`
wrappers; canonical strings are generated only for bounded diagnostics. A nil
internal value is uninitialized, not zero. Constructors copy caller data, expose
no mutable pointer/slice aliases, and operators never use a shared value as an
arithmetic receiver. Decimal denominators are positive, reduced, and have factors
permitting a finite decimal. The shared scalar plan parses/normalizes a source
value once per document-qualified scalar subject (element text or exact
attribute declaration/expanded name), domain/union-member and normalization
policy; the session
caches that immutable result for all dependent operators. It never reparses
numeric strings on every comparison. Union members retain distinct whitespace
policies and declaration order. Model adapters wrap their existing exact values;
they do not duplicate source lexical/domain validation. No `float64` comparison
substitutes for XSD decimal
value-space validation. For current finite Go `float64` fields, the adapter may
supply their exact binary value as a rational (for example, `big.Rat.SetFloat64`),
marked as current-model evidence; it cannot recover the original decimal.
NaN/Inf in a model decimal is a model/value-domain problem, not fabricated source
syntax. Selecting a different decimal/export interpretation requires an explicit
profile decision and tests; calling the encoder to obtain a value is prohibited.

Source `Lexeme` is the parser's XML-normalized attribute/text value, not raw quote,
entity, line-ending, or numeric spelling bytes. XSD whitespace is applied in the
shared scalar operator, separately for each union member in declaration order.
Source QName values need the in-scope namespace bindings below.

Dates intentionally use a bounded lexeme-level scalar plan rather than a new
date-arithmetic fact type. The `yyyy-mm-dd` plan first checks the complete pinned
XSD 1.0 `xs:date` calendar/lexical contract, then its MusicXML restriction
`[^:Z]*` and applicable no-timezone prose. `2026-02-30` fails calendar validity;
`2024-02-29` passes that calendar check. The plan handles leap years, XSD 1.0's
no-year-zero rule, budgeted arbitrary-width/signed years, and timezone absence
separately from UTC; a regex or `time.Time` normalization alone is insufficient.
Source input comes from the actual lexeme; a model `YYYYMMDD` string supplies
its current string value directly. `AtomicString` is therefore input to this
resolved date plan, never evidence that date validity already passed. The source
is `schema/musicxml-4.0/musicxml.xsd`'s `yyyy-mm-dd` restriction and registry
`MX40-PROSE-date-no-timezone`. No generic date comparison/arithmetic capability
is claimed by this sketch. Model values do
not invent source lexemes. Required non-pointer fields, pointer absence, ordered
content, fixed field order and discarded source information are different
capabilities, as [the existing-code map](existing-code-map.md) documents.

## 4. Fact adapters and session lifetime

```go
type Binding struct { Prefix, URI string }
type NodeHeader struct {
    ID, Parent NodeID
    Name QName
    Type Evidence[ComponentID] // resolved type, distinct from element declaration
    Declaration Evidence[DeclarationID]
    Location Location
    Bindings []Binding
}
type AttributeFact struct {
    Owner NodeID
    Name QName
    Declaration Evidence[DeclarationID]
    Presence Evidence[bool]
    Scalar Scalar
    Location Location
}
type OrderKind string
const (
    OrderSource OrderKind = "source-order"
    OrderModelContent OrderKind = "model-content"
    OrderModelFields OrderKind = "model-field-order"
)
type NodeEnd struct {
    ID NodeID
    Scalar Scalar
    ChildrenComplete Evidence[bool]
    AttributesComplete Evidence[bool]
    Order Evidence[OrderKind]
}
type SyntaxState string
const (
    SyntaxWellFormed SyntaxState = "well-formed"
    SyntaxMalformed SyntaxState = "malformed"
    SyntaxIncomplete SyntaxState = "incomplete"
    SyntaxNotAssessed SyntaxState = "not-assessed"
)
type Completion struct {
    InputComplete bool // EOF for source; complete traversal for model
    Syntax SyntaxState
    Cause error // nil on a completed transport/traversal
}
type Session interface {
    Open(ctx context.Context, node NodeHeader) error
    Attribute(ctx context.Context, attribute AttributeFact) error
    Close(ctx context.Context, node NodeEnd) error
    Finish(ctx context.Context, end Completion) (Report, error)
}
type Engine interface {
    NewBudgetAccount(limits Budgets) (BudgetAccount, error)
    Begin(ctx context.Context, opts RunOptions) (Session, error)
}
type RunOptions struct {
    DocumentID string // root-run-unique occurrence identity allocated by orchestration
    Profile ProfileID
    Budgets Budgets
    Account BudgetAccount // nil creates root account; children inherit the same one
    External ExternalProvider // supplied by root orchestration, never parses in core
}
```

`Begin` resolves and validates the exact profile and catalog before input is
consumed. Its target comes only from `Catalog.Profile(opts.Profile).Target`;
`RunOptions.DocumentID` must be nonempty and binds the session before input;
all local input events, their locations and local subject references must agree
with it. Cross-document Cause/Related/Origins references keep their own IDs. The
root Open has Parent=0, all real nodes have nonzero IDs, and FactView.DocumentID
returns this bound identity. Each linked occurrence receives its own ID.
Public wrappers require their corresponding target (source for InspectXML/strict
decode, model for Assess) before reading. A mismatch is a configuration error,
never an override. Reports repeat the resolved target as metadata and must agree.
`Open`, `Attribute` and `Close` take only their own payload, eliminating an
all-payload event union. They enforce balanced parent/child identity and unique
local node IDs in a document. Each consumes its arguments synchronously; the
producer may reuse buffers only after return.
The session must copy anything it retains. `Finish` is called exactly once,
even after cancellation/abort, releases retained buffers and resolver resources,
and returns the best available report without erasing prior findings. After
`Finish`, every input method is an error. Cleanup must work with a canceled
context. Every non-nil error from `Open`, `Attribute` or `Close` is terminal:
stop feeding that session and call `Finish` once. Ordinary rule violations remain
findings and do not return input-method errors. The failed session records its
first terminal cause; `Finish` returns the partial report and preserves that
cause through `errors.Is`/`errors.As`, even if Completion.Cause is nil. Additional
transport/cleanup causes may be joined without duplicating the same cause.
A terminal validation/session failure is not the recoverable model-mapping
failure described below; further source observation requires a healthy session.

`Open` establishes parent-child occurrence and order. `Attribute` includes known
present attributes and model fields with absent/unknown presence when relevant.
`Close` supplies accumulated scalar text/value and completeness proofs. A source
adapter can buffer bounded scalar chunks until close; it need not build a second
whole-document tree. Required child absence follows only from a known-complete
child set. If model field representation prevents enumeration, emit incomplete
scope/evidence, not a synthetic missing child. Model zero values that are actual
current values may be checked; they do not prove historical source presence.

The engine derives rule subjects, maintains indexes and releases closed local
scopes when dependencies permit. Parent links, IDs and retained contextual facts
are bounded. Node IDs are local to one document occurrence; `NodeRef` pairs them
with a root-run-unique `DocumentID` allocated by orchestration. This occurrence
identity is distinct from a canonical resource URI/cache key and is propagated
through decisions, causes, context positions and external roots. Re-adopting
cached facts into another occurrence remaps their NodeRefs; IDs are not public
object pointers or stable cross-edit identities. Reports own their compact locations and metadata and can
outlive the session and input/model.

### Single XML parse and conversion failure

The proposed runtime flow is:

1. Bounded byte reader and encoding handling.
2. Raw XML tokenization plus mandatory syntax/namespace checks; namespace
   expansion happens exactly once.
3. Unfiltered source observer, before namespace/attribute/model filtering.
4. Permissive model mapping from the same events.
5. Root-tail/EOF checks and document-scope finalization.

The observer must sit below all skipping paths: `Skip()` of a foreign subtree
must still feed syntax checks, limits and strict source facts. Retained attributes,
text and bindings are copied before transient token buffers are reused or
`namespaceXMLTokenReader.attributes` mutates its slice. Do not pass expanded
names through another namespace-expanding `xml.Decoder` as though they were raw
prefixes. The current raw-token and decode-once regression tests are the starting
point, not proof that an observer can simply be inserted anywhere.

The orchestration owns the parser, not the generated model decoder. On a
recoverable model conversion failure it disables model mapping, discards the
candidate model and drains the **same parser** through EOF for source assessment.
The observer's stack and counters continue independently of the model decoder's
position, including mid-element errors. If existing `DecodeElement` cannot safely
support that contract, use an event-driven mapping boundary in the implementation;
do not replay bytes or feed a re-encoded model. Fatal XML/I/O, cancellation and
safety limits terminate parsing; remaining obligations are incomplete. Source-only
`InspectXML` has no Go conversion phase at all.

### Direct model mapping and honest applicability

A generated or explicitly reviewed walker emits the same fact vocabulary with
model field/index locations. It handles typed nils, invalid choice members,
unsupported implementations, depth/volume and cycles before unsafe traversal.
`Content` slices preserve current interleaving; fixed field order describes the
model's defined structure, never past source order. Shared pointers are not
cycles unless repeated on the active recursion path; each occurrence gets its
own path and ID. Historical source snapshots are not silently consulted after
edits.

A target restriction is different from missing evidence. A source lexical rule
can be outside a current-model profile by definition; an applicable current-model
rule cannot be relabeled source-only just because a field lost necessary facts.
Required but unavailable facts produce unknown/unsupported outcomes and incomplete
assessment. The profile records its target and evidence scope in the report.

## 5. Catalog, operators, and dependencies

```go
type SourceRef struct { IDs []string; URL, Locator string }
type RuleText struct { Title, Explanation, Message string }
type DependencyKind string
const (
    DependencyRule DependencyKind = "rule"
    DependencyFact DependencyKind = "fact"
    DependencyExternal DependencyKind = "external"
)
type RelationKind string
const (
    RelationSameNode RelationKind = "same-node"
    RelationParent RelationKind = "parent"
    RelationDocument RelationKind = "document"
    RelationPart RelationKind = "part"
    RelationMusicalPosition RelationKind = "musical-position"
)
type RuleClass string
const (
    ClassMandatory RuleClass = "mandatory"
    ClassRecommendation RuleClass = "recommendation"
    ClassInterpretation RuleClass = "interpretation"
    ClassDefault RuleClass = "default"
    ClassPolicy RuleClass = "policy"
)
type RuleRole string
const (
    RolePredicate RuleRole = "predicate"
    RoleContextProvider RuleRole = "context-provider"
    RoleAdvisory RuleRole = "advisory"
)
type ScopeKind string
const (
    ScopeNode ScopeKind = "node"
    ScopeSiblings ScopeKind = "siblings"
    ScopeMeasure ScopeKind = "measure"
    ScopePart ScopeKind = "part"
    ScopeDocument ScopeKind = "document"
    ScopePackage ScopeKind = "package"
)
type Dependency struct { Kind DependencyKind; Key string; Relation RelationKind }
type RuleDefinition struct {
    Key RuleKey
    Class RuleClass
    Role RuleRole
    Targets []Target
    Scope ScopeKind
    Selector Selector
    Predicate string // reviewed compiled-plan ID, not source code or an expression
    Requires []Dependency
    Sources []SourceRef
    Text RuleText
}
type ImplementationState string
const (
    ImplementationAvailable ImplementationState = "available"
    ImplementationUnsupported ImplementationState = "unsupported"
)
type RuleSupport struct { Rule RuleKey; State ImplementationState; Reason Code }
type Profile struct {
    ID ProfileID
    MusicXMLVersion, CatalogDigest, RegistryCommit string
    Target Target
    Required []RuleKey
    Support []RuleSupport // one entry per required/transitive dependency rule
    SupportedRoots []QName // no implied support from merely knowing a schema root
    NormativeClosure string // declared scope and unresolved external boundaries
}
type Catalog interface {
    Profile(id ProfileID) (Profile, bool)
    Rule(key RuleKey) (RuleDefinition, bool)
}

type ContextKind string
const (
    ContextEffectiveKey ContextKind = "effective-key"
    ContextDivisions ContextKind = "divisions"
    ContextConcertScore ContextKind = "concert-score"
    ContextIdentity ContextKind = "identity"
    ContextIntent ContextKind = "intent"
)
type ContextKey struct {
    Kind ContextKind
    PartID, Staff, Voice string
    At NodeRef // Node == 0 denotes the document scope, with DocumentID still set
}
type ContextFact struct {
    Value Evidence[Atomic]
    Origins []Location
    Providers []RuleKey
}
type ContextView interface {
    Lookup(ctx context.Context, key ContextKey) (ContextFact, error)
}
type DecisionRef struct { Rule RuleKey; Subject NodeRef }
type Decision struct {
    Rule RuleKey
    Subject NodeRef
    Outcome Outcome
    Cause *DecisionRef
    Reason Code
    Expected, Actual string
    Related []Location
}
type ContextUpdate struct { Key ContextKey; Fact ContextFact }
type Evaluation struct {
    Decision Decision
    Produced []ContextUpdate // context-provider output, copied by the session
}
type Operator interface {
    Evaluate(ctx context.Context, subject NodeRef, facts FactView, context ContextView) (Evaluation, error)
}
type FactView interface {
    DocumentID() string
    Node(id NodeID) (NodeHeader, bool)
    Attributes(id NodeID) ([]AttributeFact, Evidence[bool])
    Children(id NodeID) ([]NodeHeader, Evidence[bool], Evidence[OrderKind])
    Scalar(id NodeID) Scalar
}
```

`Catalog.Profile` and `Catalog.Rule` return defensive deep copies, including
nested target/dependency/source-ID slices and the non-nil Selector.Component
and Selector.Declaration pointees. A value copy of RuleDefinition alone is not
sufficient: mutating either returned selector branch must not alter the catalog
or another result. Reports apply the same deep-copy guarantee to their compact
metadata, including selector branches. Caller edits to a returned value cannot
mutate a shared engine or another report. Internal compiled plans can use immutable borrowed tables.

Operator instances are immutable, compiled from reviewed typed plans and bound
to one rule key and its parameters. `Predicate` is a stable lookup key in that
compiled artifact; it is not a runtime-loaded DSL. Plan constructors validate
operator-specific arguments (domain/component, grammar, selected child names,
minimum count, reference role/target/scope, or context key) before an engine can
start. The catalog binds schema assembly to eliminate no-namespace homonyms.
`Selector` has exactly one non-nil branch. Component selection is for globally
identified types/components; declaration selection uses stable owner and exact
occurrence IDs, including anonymous owners and group reference use-sites. QName
alone is never a local-declaration key. The same `DeclarationID` is emitted by
node/attribute adapters; a plan can select part/@id without selecting every id.
For reused groups, preserve both the shared declaration and each use occurrence
in compiled metadata, and bind the plan explicitly to the intended level. IDs
can derive from the pinned registry's MX40-XSD occurrences or an equivalently
stable generated declaration map, with source XPath retained as provenance.
Missing declaration evidence stays unknown/unsupported, never a wildcard match.
NodeHeader.Type is resolved type evidence and Declaration is element declaration
identity; AttributeFact.Declaration identifies the attribute declaration/use.
The source observer may initially leave binding unknown. A shared catalog binder
resolves it from parent/type/grammar context; adapters must not guess by local
name or duplicate content-grammar predicates. The model walker uses the reviewed
generated declaration map. Global type/group selectors compile to their precise
bound declaration/use set; no component-kind ambiguity is hidden in one field.
Unsupported plans and missing required rules are explicit profile limitations,
not silently omitted catalog entries. A profile must carry its full required
obligation inventory, including known-unimplemented entries; it cannot manufacture
100% coverage by enumerating only implemented rules.
`Profile.Support` makes implementation availability machine-readable before input:
every required and transitive dependency rule has exactly one entry; unsupported
entries carry a reason, while available entries must resolve to a compiled plan.
`Begin` rejects missing/duplicate entries, unresolved available plans, invalid
keys and target mismatches. Known unsupported entries are retained in the run;
they produce unsupported/incomplete obligations whenever applicability cannot be
proven false. Dynamic missing evidence (for example, a nil resolver) is separate
from static plan availability. Profile IDs identify target-specific profiles;
there is no second target argument competing with the resolved profile.

The contract intentionally does not design a general expression language or
plugin operator ABI. Initial families are exact scalar domain, child grammar,
presence/count, effective default, uniqueness/reference and contextual relations.
Source and model adapters select the same rule/plan, with target-specific evidence.

`Role`, normative `Class`, and `Requires` are independent: a mandatory predicate,
a fact provider or an advisory can each need external data. Defaults and
interpretations produce context without becoming errors merely because they are
present. Full XSD/prose overlap contributes provenance to one predicate; partial
overlap is split into distinct obligations, not duplicated validation.

The engine topologically orders rule/fact providers; catalog dependency cycles
are configuration errors. Document identity indexes finalize only at EOF; missing
ID before EOF is pending, not failure. A duplicate ID gives one uniqueness failure
with both locations; dependent resolution is ambiguous/blocked, not arbitrary
first-match success. Missing ID produces one common IDREF failure; a dependent
kind/scope predicate is `blocked`, retaining that cause rather than reporting a
second missing-ID violation. Scope matters: timewise references may repeat;
identity declarations remain document-wide.

Context providers own effective defaults and musical ordering. `At` names a
subject whose position is computed from preserved event order, divisions,
backup/forward and scope. It is not “the last XML element seen.” Staff/voice/part
keys prevent accidental bleed. Missing initial key/time/clef/divisions is unknown
unless a reviewed rule proves absence; no invented C major, 4/4 or divisions=1.
Unsupported timeline calculations remain unsupported. A selected contextual rule
cannot drop dependencies simply because full timeline implementation is deferred.

`FactView` and `ContextView` are read-only, session-scoped borrowed views. Operators
must not retain or modify returned slices. Provider output is stored/copied by
the session with document-qualified origin/subject locations and provider rule keys, making defaults and
invalid prerequisites auditable. Application intent, when required, is an
explicit external dependency, never guessed from model shape.

## 6. External resolution is explicit, bounded, and partial

```go
type ExternalKind string
const (
    ExternalLinkedDocument ExternalKind = "linked-document"
    ExternalDictionary ExternalKind = "dictionary"
    ExternalApplicationIntent ExternalKind = "application-intent"
)
type ExternalRequest struct {
    RequiredProfile ProfileID
    Query string // reviewed dependency-specific selector, never executable code
    Kind ExternalKind
    BaseURI, Href, Revision string
    Location Location
}
type ExternalResource struct {
    State EvidenceState
    Reason Code
    CanonicalID, Revision, Digest, MediaType string
    Body io.ReadCloser // known linked document or encoded dictionary/intent
    Facts []ContextUpdate // non-nil alternative for already-typed dictionary/intent
}
type Resolver interface {
    Resolve(ctx context.Context, request ExternalRequest) (ExternalResource, error)
}

type BudgetKind string
const (
    BudgetInputBytes BudgetKind = "input-bytes"
    BudgetExternalBytes BudgetKind = "external-bytes"
    BudgetBufferedBytes BudgetKind = "buffered-bytes"
    BudgetDepth BudgetKind = "depth"
    BudgetNodes BudgetKind = "nodes"
    BudgetAttributes BudgetKind = "attributes"
    BudgetScalarBytes BudgetKind = "scalar-bytes"
    BudgetIDs BudgetKind = "ids"
    BudgetRuleSteps BudgetKind = "rule-steps"
    BudgetContextFacts BudgetKind = "context-facts"
    BudgetLinkedDocuments BudgetKind = "linked-documents"
    BudgetDiagnostics BudgetKind = "diagnostics"
    BudgetDiagnosticBytes BudgetKind = "diagnostic-bytes"
)
type BudgetAccount interface {
    Charge(kind BudgetKind, amount int64) error // cumulative quota or live gauge
    Release(kind BudgetKind, amount int64) error // live gauges only
    Check(kind BudgetKind, amount int64) error // per-item limit only
}
type ExternalFacts struct {
    State EvidenceState
    Reason Code
    CanonicalID, Revision, Digest string
    Root NodeRef // document-qualified entry point; zero only with no Document
    Document FactView // optional owned immutable snapshot, not a finished child view
    Context []ContextUpdate // dictionary/intent or derived evidence
    Assessment *Report // required linked-document assessment, if any
}
type ExternalProvider interface {
    ResolveFacts(ctx context.Context, request ExternalRequest, budget BudgetAccount) (ExternalFacts, error)
}
```

A nil resolver means no external capability. The engine has no HTTP client and
never fetches DTDs, entities, schemas, XInclude or location hints. A caller may
provide a local immutable dictionary or package resolver. Explicit caller code
can choose a network-backed resolver, but the library does not invoke one merely
because XML contains a URL. Resolution is allowed only for dependencies selected
by the caller's profile/resolver configuration, after local preconditions pass.

The resolver receives identity, base/href and requested revision, never the whole
model. It must identify authoritative absence versus unavailable/not-supported;
a fetch error is not proof a referenced object does not exist. `known` requires
a canonical identity and revision/digest required by the
profile, and exactly one payload: an owned `Body` or non-nil `Facts`. Linked XML
requires Body. Dictionary/intent resolvers may supply already-typed Facts,
including explicit absent/unknown evidence for the requested query; an empty
slice alone is not proof of absence or success. The loader validates query,
profile, value types, identity/revision/digest and scope before accepting facts,
and copies/rebinds document-qualified context into the current assessment.
Each successful Body-returning Resolve transfers exclusive ownership of a freshly
readable stream; resolvers cannot reuse an already consumed stream between calls.
The root-owned loader reads it through shared budgets and always closes it,
including on errors. Non-known resources have no payload; a body returned with an
error or invalid mixed payload is still closed. No raw resolver error becomes a
MusicXML violation.

The root orchestration constructs an `ExternalProvider` around the caller's raw
`Resolver` and its source/dictionary adapters. The core calls only this typed
fact provider: **the core never parses `Body`**. The loader uses the same XML
adapter for linked XML, creates a child session under `RequiredProfile`, and
returns immutable facts plus that child's assessment. Dictionary/intent loaders
return the requested typed context facts, parsing Body only if typed Facts were
not supplied. Invalid or incompletely assessed linked
content cannot be laundered into a successful external obligation. The loader
owns child finalization and stream closure; its cache/snapshots are owned by the
root call and released after the parent session finishes. Core and child sessions
share the `BudgetAccount` supplied by the parent through `RunOptions.Account`,
including parsing, retained facts and rule work. A nil account creates one root
account from validated finite limits for a standalone core session. Public root
orchestration instead calls `Engine.NewBudgetAccount` once **before** creating
its reader/adapter/session and passes that same non-nil account to all three;
input accounting is never hidden in an inaccessible session. Children never
reset the shared account or increase its limits. Charging ownership is explicit:
byte reader charges input bytes, adapter
charges emitted nodes/attributes/scalars, engine charges rule steps/IDs/context,
and loader charges linked-document count and external bytes. The same resource
is not charged again merely at each wrapper. Live depth/buffer gauges are
released when scopes/buffers close; cumulative byte/node/work quotas are never
refunded. Release cannot make usage negative. Unknown kinds, negative amounts,
wrong-mode calls and overflow fail closed without changing counters; typed string
constants alone are not enough. Each kind maps to the correspondingly named
Budgets field (BudgetInputBytes → MaxInputBytes, etc.). Depth and BufferedBytes
are live gauges; ScalarBytes is a per-scalar cap checked with Check before
allocation/parsing, while aggregate scalar retention also charges BufferedBytes.
All other kinds are cumulative quotas. Check does not consume shared quota;
Release is never valid for a per-item cap or cumulative quota. Counter arithmetic
and host-int allocation conversion are checked before overflow or allocation.
The loader snapshots the required
bounded facts before child `Finish`; `ExternalFacts.Document` is an owned
immutable snapshot rooted at `Root`, valid until root-call cleanup, not a
borrowed view into a finished child session. This injection breaks the otherwise
circular dependency
between engine and XML/model adapters. There is no network or parse fallback
inside the engine when a loader capability is absent.

Keep linked-document/session caches within one assessment and key them by
canonical identity **and revision/digest**. An optional caller-owned bounded,
thread-safe cache may share immutable parsed dictionary data across assessments,
keyed additionally by dictionary schema/parser/interpretation version. It shares
no NodeRefs, ContextUpdates, reports, application intent or mutable evaluation
state. Each query binds fresh context to its current DocumentID/At; cache hits
still charge the current run for lookup work, emitted facts and retained bytes.
No network is implied and content identity is verified before cache adoption.
Detect link cycles and share aggregate byte/depth/document budgets
across all descendants. Parse each resolved XML resource at most once per pinned
identity in that assessment; reusing its facts is allowed. URI fragments do not
cause repeated parsing. Package-local path and traversal rules stay in the
package adapter. Complete ZIP/JAR semantics are outside this increment.

A local typed IDREF can pass while a linked-file obligation remains unknown.
The whole requirement/report cannot be reported satisfied merely because its
local sub-contract passed. A resolver result is evidence, not permission to run
instructions or load executable predicates from the resource.

## 7. Results, diagnostics, and error compatibility

```go
type Outcome string
const (
    OutcomePass Outcome = "pass"
    OutcomeFail Outcome = "fail"
    OutcomeNotApplicable Outcome = "not-applicable"
    OutcomeUnknown Outcome = "unknown"
    OutcomeUnsupported Outcome = "unsupported"
    OutcomeBlocked Outcome = "blocked"
)
type DiagnosticCategory string
const (
    CategoryNormative DiagnosticCategory = "normative"
    CategoryModel DiagnosticCategory = "model"
    CategoryCapability DiagnosticCategory = "capability"
    CategoryResource DiagnosticCategory = "resource"
    CategorySyntax DiagnosticCategory = "syntax"
    CategoryAdvisory DiagnosticCategory = "advisory"
    CategoryPolicy DiagnosticCategory = "policy"
)
type ConformanceState string
const (
    ConformanceConformant ConformanceState = "conformant"
    ConformanceViolated ConformanceState = "violated"
    ConformanceUndetermined ConformanceState = "undetermined"
)
type ConversionState string
const (
    ConversionNotRequested ConversionState = "not-requested"
    ConversionConverted ConversionState = "converted"
    ConversionUnrepresentable ConversionState = "unrepresentable"
    ConversionFailed ConversionState = "failed"
    ConversionIncomplete ConversionState = "incomplete"
)
type Finding struct {
    Decision
    Category DiagnosticCategory
    Location Location
}
type Counts struct { Pass, Fail, NotApplicable, Unknown, Unsupported, Blocked uint64 }
type Report struct {
    Requested bool
    Profile Profile
    Target Target
    Syntax SyntaxState
    Conformance ConformanceState
    AssessmentComplete bool
    Counts Counts // normative mandatory predicate instances only
    BlockingProblems uint64 // untruncated model/policy/terminal technical blockers
    Findings []Finding
    Rules []RuleDefinition // compact metadata for every referenced rule
    DiagnosticsTruncated bool
    InputComplete bool
}
type ConversionResult struct {
    State ConversionState
    Findings []Finding
}

type Budgets struct {
    MaxInputBytes, MaxExternalBytes, MaxBufferedBytes int64
    MaxDepth, MaxNodes, MaxAttributes, MaxScalarBytes int64
    MaxIDs, MaxRuleSteps, MaxContextFacts, MaxLinkedDocuments int64
    MaxDiagnostics, MaxDiagnosticBytes int64
}
```

Proposed report methods are `Satisfied() bool` and
`Format(io.Writer) error`. `Satisfied` is true only when requested, input is
complete, syntax is well-formed for source (not assessed for model), conformance
is conformant, assessment is complete, and `BlockingProblems == 0`. No zero report is satisfied.

Aggregation is independent of operation errors:

| Evidence at end | Conformance | AssessmentComplete |
| --- | --- | --- |
| All selected mandatory applicable obligations pass, all exclusions proven | conformant | true |
| At least one proven normative mandatory failure, everything else assessed | violated | true |
| Any unknown/unsupported/blocked normative required obligation, no proven failure | undetermined | false |
| Proven failure plus unfinished/blocked obligations or interrupted input | violated | false |
| Required non-normative policy/model check is unresolved, normative checks all pass | conformant | false |
| Required non-normative policy/model check fails, all checks are otherwise assessed | conformant | true |
| Advisory fails, mandatory obligations all assessed successfully | conformant | true |

`not-applicable` requires a proven false applicability condition; lacking context
is not that proof. Counts are outcome counts for selected normative mandatory predicate instances,
not the registry's research-record denominator. Complete selection/enumeration
is itself required; aborted traversal cannot report complete because it has not
yet encountered the remaining nodes. Required model/policy obligations participate in `AssessmentComplete` even though
they do not change normative conformance or its counts. Context/advisory findings
do not inflate mandatory failure counts. Recommendation failures do not invalidate MusicXML.

A model-invariant, representability, resource or capability finding is not an XSD
failure. Such a finding can block assessment or decode success independently of
normative conformance. Reports of syntax failure never claim conformant source;
prior proven MusicXML failures may still be retained. Unsupported valid XML
features are capability/security limitations, not invented syntax violations.

Diagnostics carry stable candidate semantic IDs plus revision, target/path,
category/reason, actual/expected, related locations and causal dependency. The
report owns short English rule text and source URLs offline; `Format` prints
explanation and URL automatically, not only an opaque ID. Duplicate ID/beam
findings include the first conflicting location. Original bytes/lines are emitted
only when correctly mapped through transcoding; otherwise omit or label the
coordinate space. Trim/escape untrusted values and cap both count and byte size.

All budget limits and charge quantities are int64, including byte and count limits;
host-size conversion is checked and cannot weaken limits on 32-bit targets.
Budget zero fields select documented finite defaults; negative or impossible
values fail before reading. Existing XML depth defaults/maxima and five separate
MXL byte limits remain intact. Strict decode enforces both Decode.MaxXMLDepth
and the assessment depth budget after each zero/default is resolved; the tighter
limit wins, and neither option weakens the other. This proposal leaves numeric defaults for new
budgets to measurement in implementation, but zero must never mean unlimited.
If evaluation stops at a budget, retain findings, return its resource error,
and mark incomplete. If only diagnostic storage is truncated while evaluation
continues, preserve counts and completeness and set `DiagnosticsTruncated`;
if evaluation also stops, completeness is false. Reserve a bounded terminal
summary so reaching the diagnostic cap cannot hide interruption.

`BlockingProblems` counts failed **or unresolved** selected required
model-invariant/policy obligations and terminal technical failures independently
of stored diagnostics. Unknown/unsupported/blocked required non-normative checks
block acceptance even when every normative check passes. Optional advisories do
not. `Satisfied()` uses this
untruncated counter, never a scan of potentially truncated findings. These
acceptance blockers do not change normative `Conformance` to violated.
Strict `DecodeAssessed` separately combines source report acceptance with
`Conversion.State == converted`; `Report.Satisfied()` cannot inspect another
object's conversion state. Non-strict decode gates only XML/transport and
conversion success, not its intentionally unrequested source report.
`InspectXML` has no conversion requirement.
Infrastructure findings may use an empty `RuleKey` with a nonempty category and
reason; they must not fabricate a MusicXML rule. A `DecisionRef` identifies the causal rule and document-qualified subject, so
same-numbered nodes in different linked documents cannot collide. Finding embeds
Decision to keep these fields synchronized, but session finalization still deep
copies Cause/Related and locations; embedding does not transfer borrowed memory.
Operator subjects must match FactView.DocumentID; mismatches are contract errors.

### Error families and legacy wrappers

Proposed new typed operation errors distinguish syntax, configuration,
representability, resource, cancellation and incomplete strict assessment. They
wrap the real cause so `errors.Is(err, context.Canceled)` and `DeadlineExceeded`
work. A strict aggregate error can use `Unwrap() []error` for simultaneous source
assessment and model conversion failures; the structured reports are authoritative.

Existing sentinel identities are not recreated. Preserve `errors.As` for
`*ValidationError`, `*UnsupportedRootError`, and `*MXLLinkError` where applicable.
The future `Validate` compatibility wrapper maps genuine XSD failures to existing
`ValidationIssue`/`ValidationError`; it must not report success on an incomplete
assessment. New model/prose assessment is an explicit profile, not a silent
expansion of legacy `Validate`'s advertised XSD-only scope.

Compatibility cases that must be tested during implementation:

| Existing observable contract | Proposed bridge |
| --- | --- |
| `ValidationError` unwraps `ErrInvalidDocument` | Retain for genuine legacy XSD failures; richer report retains rule/category data. |
| `Validate` depth/cycle failures match both `ErrInvalidDocument` and the specific sentinel | Preserve wrapper recognition for legacy callers; new reports classify resource/model accurately. |
| `ErrMXLTooLarge` already wraps `ErrInvalidMXL` | Keep that identity/wrapping; resource category does not relabel it as normative failure. |
| `UnsupportedRootError` matches `ErrUnsupportedRoot` | Preserve type/name/cause; unsupported profile is not necessarily malformed XML. |
| Previously serialized representation/lexical paths | Audit old behavior with focused tests before redirecting `Validate`; document unavoidable changes, do not fabricate new XSD failures to imitate bugs. |

A representability-only error and unknown/unsupported-only strict result must
not newly wrap `ErrInvalidDocument`. Genuine source normative failure may do so
through the explicit assessment error contract; exact exported new error names
and prose-versus-legacy sentinel policy need API review before implementation.
No new error behavior is shipped by this documentation PR.

## 8. Existing code reuse and first implementation seam

| Existing code | Use in the proposed contracts | Required change or proof |
| --- | --- | --- |
| `decode.go`, `xml_decoder.go`, `xml_namespace.go` | Source orchestrator and observer | Observe before destructive filtering/skips, establish mandatory missing syntax checks, prove once-only namespace expansion, drain after conversion failure. |
| `internal/xsdgen/generate_validation.go`, `generate.go`, generated validation schemas | Compiled catalog/type/grammar data | Preserve assembly/declaration/use-site identity and support inventory. Currently only score and opus validation schemas are generated; add container/sounds generation directives, data and tests before advertising those roots. |
| `validation.go`: `matchParticle` family | Shared child-grammar operator | Verify nested choice/sequence, nullable/repeated particles and work budgets, not independent child counters. |
| `validateSimple`, `validateBuiltin`, facet helpers; scalar helper files | Shared scalar plans | Exact integers/decimals, union whitespace/member order, proper XSD facets/pattern semantics; do not keep machine-int validity caps. |
| `validateAttributes` | Attribute operator | Cover simple-type elements and specific xsi contracts; QName values need bindings. |
| `recordIdentity`, `validateIdentityReferences` | Document identity provider and reference plans | Typed/scope constraints, ambiguity and causal suppression; no premature absence. |
| `Validate`, `ValidationError`, `errors.go` | Compatibility facade | Replace Encode→parse with direct facts only after behavior audit and category migration tests. |
| Generated `Content` interfaces and fields, `Effective...` methods | Direct model adapter metadata | Presence/multiplicity/order capability map per field; defaults stay facts, never mutate model. |
| `document_depth.go`, `mxl*.go`, `options.go` | Guards, package evidence and local resolver | Active-path cycles, volume/cumulative budgets and errors compatibility; do not imply full external-spec closure. |

See [existing-code-map.md](existing-code-map.md) for pinned source links and
confirmed deficiencies. None of those defects is fixed here. Existing product
code and generated output remain untouched.

A bounded implementation can begin with report/evidence types, one shared local
operator, two adapters for a narrow profile, and rule-level tests. The six
[worked examples](go-contract-examples.md) are acceptance traces, not a promise
that six rules close the full MusicXML profile. Then add grammar/identity and
context in independently reviewable slices. Every slice must name unimplemented
obligations; a partial profile must advertise its restricted claim.

Deferred: full executable catalog/DSL, all 2,560 records, full TDD suite, complete
timeline, dictionary/ZIP closure, source-snapshot editing, new public model
representations, performance tuning, production validator code and known bug
fixes. The unresolved API/default-number details above are deliberate review
points; none authorizes treating unknown as valid.

[registry]: https://github.com/go-muse/go-musicxml-registry/tree/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c
