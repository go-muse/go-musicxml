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
    Assessment AssessmentOptions
    StrictSource bool // zero is the existing permissive behavior
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
  including `container` and `sounds`; those do not become new `Document` types.
- `Assess` traverses the current model directly. It never calls `Encode`, XML
  marshal, or an XML parser, including in an alleged exportability fast path.
- `DecodeAssessed` parses once, optionally assesses source, and maps the same
  stream to a model. Default `StrictSource=false` is permissive MusicXML
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
unfinished broad profile from becoming an accidental default. Non-strict
`DecodeAssessed` does not run assessment, load a catalog, or invoke the resolver;
it rejects nonzero `Assessment` fields rather than silently ignoring a mistaken
strict request. Existing default Decode behavior and its DecodeOptions remain
unchanged. A future named default profile requires a separate compatibility
and coverage decision.

No API closes the caller-owned source reader or rewinds it. One call consumes
one reader; a caller making two independent calls must supply two readers.
`context.Context` is non-nil. Cancellation is checked between reads, tokens,
operator steps and resolver calls; it cannot interrupt arbitrary blocking
`io.Reader.Read`. Hard deadlines require caller-provided cancellable I/O.

## 3. Shared types: identity, evidence, and values

The following core declarations are a compile-checkable contract sketch, not
an implementation. String enums below are closed sets checked by constructors;
empty/unknown values are never interpreted as success.

```go
type ProfileID string
type RuleID string
type RuleKey struct { ID RuleID; Revision uint32 }
type NodeID uint64
type Code string
type Target string // source | model | package

type QName struct { Space, Local string }
type ComponentID struct {
    Assembly string // musicxml | opus | sounds | container; not namespace alone
    Name QName
    Kind string // element | type | group | attribute
}

type EvidenceState string // known | absent | unknown | unsupported | invalid
type Evidence[T any] struct {
    State EvidenceState
    Value T // meaningful only when State == known
    Reason Code
}

type Rational struct { Numerator, Denominator string }
type Atomic struct {
    Kind string // string | boolean | integer | decimal | qname
    Text string // string, canonical boolean, or canonical unbounded integer
    Number Rational // decimal: exact rational, denominator positive
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

Exact integers use canonical signed decimal strings and budgeted arbitrary
precision. Exact decimals use reduced rational values whose denominator factors
permit a finite decimal. No `float64` comparison substitutes for XSD decimal
value-space validation. For current finite Go `float64` fields, the adapter may
supply their exact binary value as a rational (for example, `big.Rat.SetFloat64`),
marked as current-model evidence; it cannot recover the original decimal.
NaN/Inf in a model decimal is a model/value-domain problem, not fabricated source
syntax. Selecting a different decimal/export interpretation requires an explicit
profile decision and tests; calling the encoder to obtain a value is prohibited.

Source `Lexeme` is the parser's XML-normalized attribute/text value, not raw quote,
entity, line-ending, or numeric spelling bytes. XSD whitespace is applied in the
shared scalar operator, separately for each union member in declaration order.
Source QName values need the in-scope namespace bindings below. Model values do
not invent source lexemes. Required non-pointer fields, pointer absence, ordered
content, fixed field order and discarded source information are different
capabilities, as [the existing-code map](existing-code-map.md) documents.

## 4. Fact adapters and session lifetime

```go
type Binding struct { Prefix, URI string }
type NodeHeader struct {
    ID, Parent NodeID
    Name QName
    Component Evidence[ComponentID]
    Location Location
    Bindings []Binding
}
type AttributeFact struct {
    Owner NodeID
    Name QName
    Presence Evidence[bool]
    Scalar Scalar
    Location Location
}
type NodeEnd struct {
    ID NodeID
    Scalar Scalar
    ChildrenComplete Evidence[bool]
    AttributesComplete Evidence[bool]
    Order Evidence[string] // source-order | model-content | model-field-order
}
type Event struct {
    Kind string // open | attribute | close
    Open NodeHeader
    Attribute AttributeFact
    Close NodeEnd
}

type Completion struct {
    InputComplete bool // EOF for source; complete traversal for model
    Syntax string // well-formed | malformed | incomplete | not-assessed
    Cause error // nil on a completed transport/traversal
}
type Session interface {
    Accept(ctx context.Context, event Event) error
    Finish(ctx context.Context, end Completion) (Report, error)
}
type Engine interface {
    NewBudgetAccount(limits Budgets) (BudgetAccount, error)
    Begin(ctx context.Context, target Target, opts RunOptions) (Session, error)
}
type RunOptions struct {
    Profile ProfileID
    Budgets Budgets
    Account BudgetAccount // nil creates root account; children inherit the same one
    External ExternalProvider // supplied by root orchestration, never parses in core
}
```

`Begin` resolves and validates the exact profile and catalog before input is
consumed. `Accept` receives a tagged event with exactly one payload, balanced
parent/child identity and no duplicate node IDs within a document. It consumes
its arguments synchronously; the producer may reuse buffers only after return.
The session must copy anything it retains. `Finish` is called exactly once,
even after cancellation/abort, releases retained buffers and resolver resources,
and returns the best available report without erasing prior findings. After
`Finish`, `Accept` is an error. Cleanup must work with a canceled context.

`open` establishes parent-child occurrence and order. `attribute` includes known
present attributes and model fields with absent/unknown presence when relevant.
`close` supplies accumulated scalar text/value and completeness proofs. A source
adapter can buffer bounded scalar chunks until close; it need not build a second
whole-document tree. Required child absence follows only from a known-complete
child set. If model field representation prevents enumeration, emit incomplete
scope/evidence, not a synthetic missing child. Model zero values that are actual
current values may be checked; they do not prove historical source presence.

The engine derives rule subjects, maintains indexes and releases closed local
scopes when dependencies permit. Parent links, IDs and retained contextual facts
are bounded. Node IDs are assessment-local, not public object pointers or stable
cross-edit identities. Reports own their compact locations and metadata and can
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
type Dependency struct {
    Kind string // rule | fact | external
    Key string
    Relation string // same-node | parent | document | part | musical-position
}
type RuleDefinition struct {
    Key RuleKey
    Class string // mandatory | recommendation | interpretation | default | policy
    Role string // predicate | context-provider | advisory
    Targets []Target
    Scope string // node | siblings | measure | part | document | package
    Selector ComponentID
    Predicate string // reviewed compiled-plan ID, not source code or an expression
    Requires []Dependency
    Sources []SourceRef
    Text RuleText
}
type Profile struct {
    ID ProfileID
    MusicXMLVersion, CatalogDigest, RegistryCommit string
    Target Target
    Required []RuleKey
    NormativeClosure string // declared scope and unresolved external boundaries
}
type Catalog interface {
    Profile(id ProfileID, target Target) (Profile, bool)
    Rule(key RuleKey) (RuleDefinition, bool)
}

type ContextKey struct {
    Kind string // effective-key | divisions | concert-score | identity | intent
    DocumentID, PartID, Staff, Voice string
    At NodeID
}
type ContextFact struct {
    Value Evidence[Atomic]
    Origins []Location
    Providers []RuleKey
}
type ContextView interface {
    Lookup(ctx context.Context, key ContextKey) (ContextFact, error)
}
type DecisionRef struct { Rule RuleKey; Subject NodeID }
type Decision struct {
    Rule RuleKey
    Subject NodeID
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
    Evaluate(ctx context.Context, subject NodeID, facts FactView, context ContextView) (Evaluation, error)
}
type FactView interface {
    Node(id NodeID) (NodeHeader, bool)
    Attributes(id NodeID) ([]AttributeFact, Evidence[bool])
    Children(id NodeID) ([]NodeHeader, Evidence[bool], Evidence[string])
    Scalar(id NodeID) Scalar
}
```

`Catalog.Profile` and `Catalog.Rule` return defensive deep copies, including
nested target/dependency/source-ID slices. Reports also deeply own their compact
metadata. Caller edits to a returned value cannot mutate a shared engine or
another report. Internal compiled plans can use immutable borrowed tables.

Operator instances are immutable, compiled from reviewed typed plans and bound
to one rule key and its parameters. `Predicate` is a stable lookup key in that
compiled artifact; it is not a runtime-loaded DSL. Plan constructors validate
operator-specific arguments (domain/component, grammar, selected child names,
minimum count, reference role/target/scope, or context key) before an engine can
start. The catalog binds schema assembly to eliminate no-namespace homonyms.
Unsupported plans and missing required rules are explicit profile limitations,
not silently omitted catalog entries. A profile must carry its full required
obligation inventory, including known-unimplemented entries; it cannot manufacture
100% coverage by enumerating only implemented rules.

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
the session with origin locations and provider rule keys, making defaults and
invalid prerequisites auditable. Application intent, when required, is an
explicit external dependency, never guessed from model shape.

## 6. External resolution is explicit, bounded, and partial

```go
type ExternalRequest struct {
    RequiredProfile ProfileID
    Query string // reviewed dependency-specific selector, never executable code
    Kind string // linked-document | dictionary | application-intent
    BaseURI, Href, Revision string
    Location Location
}
type ExternalResource struct {
    State EvidenceState
    Reason Code
    CanonicalID, Revision, Digest, MediaType string
    Body io.ReadCloser // only for a known resource
}
type Resolver interface {
    Resolve(ctx context.Context, request ExternalRequest) (ExternalResource, error)
}

type BudgetAccount interface {
    Charge(kind string, amount int64) error // shared atomic check-and-consume
    Release(kind string, amount int64) // only live gauges, never cumulative quotas
}
type ExternalFacts struct {
    State EvidenceState
    Reason Code
    CanonicalID, Revision, Digest string
    Root NodeID // entry point for Document; zero when no document
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
an owned non-nil `Body`, a canonical identity and revision/digest required by the
profile. Each successful Resolve transfers exclusive ownership of a freshly
readable stream; resolvers cannot reuse an already consumed stream between calls.
The root-owned loader reads it through shared budgets and always closes it,
including on errors. Non-known resources have no body; a body returned with an
error is still closed. No raw resolver error becomes a MusicXML violation.

The root orchestration constructs an `ExternalProvider` around the caller's raw
`Resolver` and its source/dictionary adapters. The core calls only this typed
fact provider: **the core never parses `Body`**. The loader uses the same XML
adapter for linked XML, creates a child session under `RequiredProfile`, and
returns immutable facts plus that child's assessment. Dictionary/intent loaders
return the requested typed context facts. Invalid or incompletely assessed linked
content cannot be laundered into a successful external obligation. The loader
owns child finalization and stream closure; its cache/snapshots are owned by the
root call and released after the parent session finishes. Core and child sessions
share the `BudgetAccount` supplied by the parent through `RunOptions.Account`,
including parsing, retained facts and rule work. A nil account creates one root
account from validated finite limits for a standalone core session. Public root
orchestration instead calls `Engine.NewBudgetAccount` once **before** creating
its reader/adapter/session and passes that same non-nil account to all three;
input accounting is never hidden in an inaccessible session. Children never
reset the shared account or increase its limits. Charging ownership is explicit: byte reader charges input bytes, adapter
charges emitted nodes/attributes/scalars, engine charges rule steps/IDs/context,
and loader charges linked-document count and external bytes. The same resource
is not charged again merely at each wrapper. Live depth/buffer gauges are
released when scopes/buffers close; cumulative byte/node/work quotas are never
refunded. Release cannot make usage negative. The loader snapshots the required
bounded facts before child `Finish`; `ExternalFacts.Document` is an owned
immutable snapshot rooted at `Root`, valid until root-call cleanup, not a
borrowed view into a finished child session. This injection breaks the otherwise circular dependency
between engine and XML/model adapters. There is no network or parse fallback
inside the engine when a loader capability is absent.

Cache by canonical identity **and revision/digest**, scope each cache to one
assessment, detect link cycles, and share aggregate byte/depth/document budgets
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
type Outcome string // pass | fail | not-applicable | unknown | unsupported | blocked
type Finding struct {
    Rule RuleKey
    Subject NodeID
    Outcome Outcome
    Category string // normative | model | capability | resource | syntax | advisory
    Reason Code
    Location Location
    Related []Location
    Expected, Actual string
    Cause *DecisionRef
}
type Counts struct { Pass, Fail, NotApplicable, Unknown, Unsupported, Blocked uint64 }
type Report struct {
    Requested bool
    Profile Profile
    Target Target
    Syntax string
    Conformance string // conformant | violated | undetermined
    AssessmentComplete bool
    Counts Counts // normative mandatory predicate instances only
    BlockingProblems uint64 // untruncated model/policy/terminal technical blockers
    Findings []Finding
    Rules []RuleDefinition // compact metadata for every referenced rule
    DiagnosticsTruncated bool
    InputComplete bool
}
type ConversionResult struct {
    State string // not-requested | converted | unrepresentable | failed | incomplete
    Findings []Finding
}

type Budgets struct {
    MaxInputBytes, MaxExternalBytes, MaxBufferedBytes int64
    MaxDepth, MaxNodes, MaxAttributes, MaxScalarBytes int
    MaxIDs, MaxRuleSteps, MaxContextFacts, MaxLinkedDocuments int
    MaxDiagnostics, MaxDiagnosticBytes int
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
reason; they must not fabricate a MusicXML rule. A `DecisionRef` identifies both
the causal rule and its subject, so two failures of the same rule stay distinct.

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
| `internal/xsdgen/generate_validation.go`, generated validation schemas | Compiled catalog/type/grammar data | Preserve assembly/source identity; version/fingerprint and separate executable obligations from inventory. |
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
