# API policy

This document describes the public contract for the `v0.1` release line.

## Compatibility

`go-musicxml` follows semantic import versioning. The module is still below
`v1`, so later minor releases may contain intentional breaking changes. Patch
releases in the `v0.1.x` line should preserve source compatibility unless a
security or data-integrity defect requires otherwise.

`MusicXMLVersion` is the supported schema version.

## Documents

`Document` is a sealed interface. Its supported implementations are:

- `*ScorePartwise`
- `*ScoreTimewise`
- `*OpusDocument`

`ScoreDocument` narrows that set to the two score roots. Sealing prevents a
value that `Encode`, `Validate`, `AssessModel`, or MXL transport cannot handle
from satisfying the interface accidentally.

`AsScorePartwise`, `AsScoreTimewise`, and `AsOpusDocument` safely narrow a
polymorphic `Document` without a panicking type assertion.

## Generated model

Most exported types mirror MusicXML 4.0 XSD complex and simple types.

- Optional scalar attributes and elements use pointers.
- Repeated elements use slices.
- XSD enumerations use named string types and constants.
- XSD choices use wrapper structs where exactly one field must be set.
- Types whose child order is semantically significant expose `Content` and
  generated `Add...` methods. Their generated `...Contents` slice types append
  recognized elements and discard unknown elements during unmarshalling, so
  every retained entry contains exactly one child variant.
- `Effective...` methods return XSD defaults without mutating the raw field.
- `...MatchesFixed` methods test explicit values against XSD `fixed`
  constraints after applying the attribute type's XML-whitespace policy.

Numeric field types are unchanged by XML lexical normalization. Finite decimals
are written without exponent notation, but the model still uses `float64`
precision. Integers must fit their generated Go type. Exact arbitrary-precision
XSD numbers are outside the model's representational range.

Directly assigning generated fields is supported. Constructors and `Add...`
methods are conveniences, not a separate object model.

## Transport

`Decode` accepts exactly one unqualified `score-partwise`, `score-timewise`, or
`opus` root and rejects non-whitespace content outside it. Boundary whitespace
must be literal XML `S`: space, tab, carriage return or line feed. References
and CDATA sections are forbidden outside the root, even if they decode to
whitespace or no characters; they remain supported inside elements. It does not
call `Validate`. `DecodeWithOptions` changes the XML nesting ceiling; the zero-value
options use the documented safe default.

XML reading rejects duplicate attributes (including collisions after namespace
expansion) and repeated DOCTYPE declarations or declarations inside/after the
root, even in skipped unknown or foreign-namespace subtrees. A single prolog
DOCTYPE is accepted without loading external DTDs; an XML declaration is not
required. If present, the XML declaration must be first, before whitespace,
comments, processing instructions or DOCTYPE; an encoding byte order mark is
allowed before it. Repeated declarations and all other case variants of the
exact processing-instruction target `xml` are rejected. Ordinary targets such
as `xml-stylesheet` remain allowed. These checks also apply to MXL container
metadata and documents parsed while resolving opus links. They do not add DTD
validation or strict MusicXML validation. Empty prefixed namespace declarations
such as `xmlns:p=""` are rejected under Namespaces in XML 1.0; resetting the
default namespace with `xmlns=""` remains allowed.

The shared source scanner checks XML 1.0 declaration and local DTD grammar,
namespace closure and original encoding before model filtering. XML defects wrap
`*xml.SyntaxError`; resource and unavailable-encoding/DTD semantics retain their
separate typed causes. `xml.SyntaxError.Line` identifies a decoded-source
recognition position, not an attribute-start location. Source assessment findings
supply explicit original-byte positions where available.

Literal attribute tabs and line endings normalize to spaces, while numeric
references preserve their referenced whitespace. Contradictory BOM/encoding
declarations, undeclared prefixes, malformed declaration/DTD grammar and illegal
reserved namespace bindings are now rejected. DTD defaults and replacement text
are not implemented or fetched; unsupported declaration WFC/infoset dependencies
are explicit assessment limitations. See the
[source parser scope](docs/validation/source-parser-scope.md) for the precise
syntax, compatibility, encoding and DTD boundary.

When the expected root type is known, `DecodeScorePartwise`,
`DecodeScoreTimewise`, and `DecodeOpusDocument` return the corresponding
concrete pointer type. They return `ErrUnsupportedRoot` for any other root.
Each typed helper has a corresponding `WithOptions` variant.

`Encode` writes one root element without an XML declaration and does not call
`Validate`. It rejects invalid UTF-8 and characters forbidden by XML 1.0 before
writing output, rather than silently replacing them with U+FFFD. `Validate`
reports these failures as representation issues. MXL encoding adds an XML
declaration to stored MusicXML documents.

Encoding preserves the typed model, not original XML formatting. Unknown XML
extensions are not part of the compatibility guarantee. The package decoders
ignore foreign-namespace children and attributes, including names that match
unqualified MusicXML fields; supported XML and XLink attributes are retained.
`Decode` ignores unknown child elements inside a supported root, including
inside generated ordered `Content`, and `Encode` does not reproduce them.

Direct `encoding/xml` unmarshalling of generated types applies the same
ordered-content filtering. It does not apply the package decoder's character
encoding support, configurable XML-depth limit, duplicate-attribute/DOCTYPE/
XML-declaration checks, or foreign-namespace filtering for ordinary struct
fields, nor the encoder's XML-text preflight. Go's
`encoding/xml` matches unqualified struct tags by local name, so `Decode` and
its typed variants remain the recommended document entry points.

`Encode` and `ResolveOpus` reject cyclic opus models and document nesting deeper
than 4096 elements before recursive traversal. `Validate` preserves that legacy
depth/cycle recognition through the bounded iterative model walker.
`AssessModel` uses its own configurable finite assessment budgets; its default
`MaxDepth` is 1024.

## Validation

`Validate` and the three root `Validate` methods now traverse the current model
directly using the shared XSD kernel. They do not call `Encode`, marshal methods
or an XML parser. Their scope stays XSD-only: the new local musical clauses are
selected only by an explicit local profile. `Decode` and `Encode` still do not
validate automatically.

`Validate` returns:

- `nil` when the selected XSD and model-representation checks accept the current
  document;
- an error containing `*ValidationError` for proven XSD violations or legacy model
  representation failures, possibly joined with a technical error;
- a sentinel argument or document-type error when validation cannot start;
- a technical error when required metadata or contextual facts are unavailable.
  Pure capability/resource incompleteness does not match `ErrInvalidDocument`,
  apart from the preserved legacy depth exception below; incompleteness is not
  evidence that the document is valid or invalid.

Use `errors.Is(err, ErrInvalidDocument)` for the legacy category and
`errors.As` with a `*ValidationError` target to inspect retained `ValidationIssue`
details. Details obey finite diagnostic budgets and are not an exhaustive trace.
Legacy issue locations retain XML-style structural display paths; representation
messages also identify Go fields. These paths do not describe original XML.
`ErrNilDocument`, `ErrUnsupportedDocument`, `ErrDocumentCycle` and
`ErrDocumentTooDeep` remain recognizable where applicable. Depth and cycle errors
still match `ErrInvalidDocument` in this facade. The legacy depth ceiling remains
4096; other finite assessment defaults also apply. Use `AssessModel` for explicit
budgets and independent normative, model and resource outcomes.

This is an intentional value-contract change from validating serialized text:
finite decimal fields mean their exact current binary rational, including at
bounds, enumeration, fixed and digit facets. For example, current `float64(0.1)`
is not decimal `1/10`; large finite values are no longer assessed through an
encoder's spelling. Unsupported numeric lexical patterns remain incomplete,
without inventing a source lexeme. No serialization fallback is retained.

`ErrIncompleteAssessment` identifies technical incompleteness, including its presence in a joined invalid/incomplete
result. It is the identical sentinel exported by the `validation` subpackage.
`errors.As` can also retrieve its `*validation.IncompleteError`. Independent
legacy model/XSD failures may be joined with that incomplete result.
Incomplete-error text and diagnostic wording may change across the direct-walker
migration. Use `errors.Is`/`errors.As` rather than matching an error-string prefix;
the public sentinel and typed cause are the compatibility contract.

Reached `ENTITY`/`ENTITIES` assessment cannot establish unparsed-entity
declarations at this layer. Lexically valid values therefore produce technical
incompleteness; lexical violations remain reportable and can reject a value
before declaration facts are needed. Independent violations elsewhere are still
reported alongside a reached technical error. Validation never fetches a DTD.

URI values are checked using XSD `anyURI` lexical rules, including XLink
escaping of spaces and non-ASCII characters. Validation leaves the stored
string unchanged and does not check resource existence or accessibility.

## Direct current-model assessment

```go
func AssessModel(ctx context.Context, document Document, options AssessmentOptions) (validation.Report, error)
```

The name is deliberately `AssessModel`: `Assess` is already an exported generated
MusicXML model type and remains unchanged. `AssessmentOptions` retains its
`Profile` and `Budgets` fields. `AssessModel` accepts `*ScorePartwise`,
`*ScoreTimewise` and `*OpusDocument`; it rejects nil/typed-nil documents and
unsupported concrete implementations. A nonnil context and an explicit model
profile are required. Empty, unknown or source-target profiles, invalid budgets
and already-canceled contexts fail before model traversal.

| Constant | Profile ID | Declared scope |
| --- | --- | --- |
| `ProfileModelXSD` | `musicxml-4.0/model-xsd` | Current generated structure/value XSD plus required model representation checks |
| `ProfileModelLocal` | `musicxml-4.0/model-local` | The same model checks plus the explicitly selected local musical and typed-reference clauses |
| `ProfileSourceLocal` | `musicxml-4.0/source-local` | Original single-pass source XSD plus those same local clauses; used with `InspectXML` or strict `DecodeAssessed` |

These are bounded implementations under final verification, not a claim of
whole-MusicXML conformance or completed batch acceptance. The complete selected
clause list, exclusions, test evidence and remaining owners are in the
[model assessment scope](docs/validation/model-assessment-scope.md). Each profile
supports the three roots above; score-only local predicates are inapplicable to
opus and never assess linked opus/score content. Version selection is the pinned
MusicXML 4.0 schema, not an invented requirement that the `version` attribute
equal `4.0`. No profile is an implicit default. The separately compiled
`ProfileSourceXSDPreview` remains developmental with its existing scope and IDs.

The direct adapter uses generated field/choice metadata and the same authoritative
XSD grammar, scalar, attribute/default and identity machinery as source. It never
serializes, parses XML, invokes user marshal methods or mutates the model.
The caller must not mutate a document during assessment. Independent assessments
may read the same unchanged document concurrently.

Current evidence has precise limits:

- Nil optional pointers are absent; nonnil pointers are present even at zero or
  empty. Required nonpointer fields are present current values, including zero.
- Valid `Content` entries preserve current interleaving; ordinary fields use their
  generated order. Empty/multiple-branch entries and active-path cycles are model
  problems. A hole never becomes a complete compacted child sequence.
- Root identity comes from the concrete generated type. `XMLName` is ignored
  transport metadata, including inconsistent or invalid text in that field.
- Strings supply their current text to shared schema-domain/whitespace/union
  checks. Integers supply exact current signed/unsigned values. Every finite
  `float64` decimal supplies the exact `big.Rat.SetFloat64` value. In particular,
  `0.1` means `3602879701896397/36028797018963968`, not decimal `1/10`. NaN,
  infinity and invalid XML text are model representation problems.
- Scalar source lexemes remain unavailable. Numeric lexical patterns and other
  reached unsupported value/default/identity capabilities remain incomplete.
  Numeric zero does not trigger an empty-element default; absent attribute
  defaults remain separate effective facts and never overwrite fields.
- The current model cannot recover discarded attributes/elements, repeated
  singleton occurrences, original required-scalar presence, namespaces, xsi
  controls, numeric precision or original order/spelling. A model pass cannot
  retroactively validate a permissively decoded source.

For a started model assessment, `Report.Target` is `TargetModel` and
`Report.Syntax` is `SyntaxNotAssessed`; a preflight failure can return a zero report.
Locations use Go field/index paths, such as
`ScorePartwise.Part[0].Measure[0].Content[0].Note.Pitch.Step`, with occurrence IDs
and no XML byte/line coordinates. Long paths use bounded parent-occurrence anchors;
paths and node IDs are run-local display/occurrence evidence, not stable edit IDs.
Shared pointers outside the active path create independent occurrences.

Ordinary normative and model-representation findings return a report with a nil
operation error. Always use `Report.Satisfied()`. A model blocker can prevent
acceptance without proving an XSD violation. Configuration, cancellation,
resource or traversal interruptions retain their typed cause and best partial
report. Diagnostic truncation cannot erase counts or blockers; known violations
can coexist with incomplete assessment. Reports own retained metadata after the
call returns.

One finite budget account covers the iterative walker, schema/session work,
indexes, exact-value scratch and diagnostics. `MaxEvents` includes examined
fields/choice entries even when they emit no node; session ingress owns node,
attribute and depth charges. Scalar and live-buffer limits apply before expensive
work/retention. No input-byte charge is invented for the existing object graph,
and the 64 MiB default live-buffer limit is not a model-size or heap guarantee.
No schema, DTD or linked resource is fetched.

## Fact-stream validation foundation

The `validation` subpackage exports `NewCatalog`, eight generic sealed typed-plan
constructors, the sealed shared-schema `NewXSDPlan`, bounded local relation and
typed-reference constructors, `NewModelRepresentationPlan`, `NewEngine`, and
explicit-profile fact sessions using `Open`, `Attribute`, optional
`CompleteAttributes`/`ModelRepresentation`, `Close` and `Finish`. The optional
attribute-closure event seals explicit start-tag enumeration before children;
it enables truthful prefix assessment when an ancestor never closes. Catalogs are immutable; sessions copy retained
facts and return independently owned reports. Exact selectors distinguish schema
assemblies and declaration/use occurrences. There is no default MusicXML profile,
resolver or package-assessment entry point.
The root package provides the explicit source and direct-model adapters described
here. The [exported example](validation/example_test.go) uses synthetic facts.

`CompiledSchema.Uses()` returns copied `SchemaUse` descriptors with exact
declaration, owner and declared-type identities; the `Known` flags distinguish
unavailable metadata from anonymous types. It does not establish runtime support.
`NewXSDPlan` accepts source/current-model facts without inventing source history.

The additive local constructors are `NewDistinctChildAttributePlan`,
`NewScalarOneOfPlan`, `NewPresentAttributeValuePlan`,
`NewAttributeNumberOrTokensPlan`, `NewPresenceImpliesAttributePlan`,
`NewAttributeImpliesRelatedChildPlan`, `NewDocumentChildPresencePlan` and
`NewDocumentMarkerValuesPlan` (with `ChildValueRequirement`).
`NewTypedReferencePlan(ReferencePlanSpec)` uses authoritative XSD ID resolution;
`ReferenceOwner` binds the exact provider rule/revision defining matching scope.
These sealed constructors require bounded exact selectors/parameters and copy
retained inputs. They are not a callback ABI or a linked-resource resolver.

`Session.ModelRepresentation` accepts one bounded `ModelRepresentationFact` for
the currently open model occurrence before `Close`. `ModelProblem` identifies a
scalar, exact attribute, children or node axis through the `ModelAxis` constants.
The session copies the examination and establishes a real required model decision
before exposing causal invalid evidence. The corresponding
`NewModelRepresentationPlan` counts model blockers separately from normative
failures; missing examination evidence cannot pass.

A profile explicitly names all required and transitive rules, static support,
supported roots and normative closure. `CompiledCatalog.Schedule` exposes each
rule once in final-evaluation order, after derived context-provider/identity
phase ordering; these edges do not add rules or semantic dependencies. Catalog
digests cover canonical metadata and typed plan parameters.
`MusicXML40Inventory` is a separate pinned inspection snapshot of 2,560 research records, not executable rule coverage.

`Report.Satisfied()` evaluates independent syntax, input/assessment completion,
conformance and blocker axes. A nil operation error alone is not acceptance.
Evidence-only limitations and normative findings are report outcomes; terminal
failures preserve typed incomplete/resource/cancellation causes. Call `Finish`
exactly once even after an event error to release owned live resources. Required
model/policy/context blockers do not become normative violations, and diagnostic
truncation does not erase counts or blockers. Formatting is bounded and offline.

`Report.Findings` stores only fail, unknown, unsupported and blocked diagnostics;
`Report.Rules` contains metadata only for those stored diagnostics. Pass and
not-applicable outcomes still contribute to applicable `Counts` and are retained
compactly for dependency execution, but never consume diagnostic quotas or set
`DiagnosticsTruncated`. There is no all-decisions trace option. The compact
per-rule success/not-applicable tables and document-prerequisite aggregates have
separate live-buffer charges. A `RelationDocument` rule-prerequisite aggregate is
computed lazily once after its prerequisite's final evaluation phase and reused
by consumers. This does not remove rules-times-nodes dispatch or make the retained
fact session a constant-memory stream.

The identity plans accept upstream-domain-validated string keys. A well-formed
known atomic value in another family produces a per-rule unsupported outcome,
not a terminal event error. An unsupported declaration makes its index incomplete
so an unresolved reference cannot fabricate a missing-target violation; malformed
atomic payloads remain terminal fact-contract failures.

Terminal summaries use deterministic precedence across joined causes:
resource-limit, cancellation and deadline exhaustion are resource failures;
otherwise a valid source completion asserting malformed syntax with an explicit
cause is syntax; unsupported root, invalid session, invalid configuration and
unknown infrastructure failures are capability
failures. Each has its own reason code, listed in the
[terminal disposition table](docs/validation/foundation-scope.md#terminal-disposition).
Invalid or contradictory completion assertions remain capability contract or
lifecycle failures rather than proving malformed XML. Policy remains an explicit
rule category. Selecting one terminal summary does
not discard the underlying causes available through `errors.Is`/`errors.As`.

`Budgets` has finite zero defaults; a shared `Account` cannot be reset or widened
by child sessions. All limits, charging owners, exact-value constructor caps,
plan boundaries and remaining obligations are documented in the
[foundation scope](docs/validation/foundation-scope.md). Numeric constructor
resource limits are not XSD facets. `DecodeOptions` and generated public field
representations remain unchanged. The additive model-representation event is
required only when its corresponding sealed plan is selected; older generic
fact sessions and the source preview retain their contracts.

## Opt-in original-source assessment

**Current source input limit:** an unread external DOCTYPE subset prevents
acceptance by either source profile. The committed official score samples
`Chant.musicxml`,
`MozartTrio.musicxml` and UTF-16 `MozaChloSample.musicxml` therefore produce
unsupported/incomplete source reports. Strict `DecodeAssessed` returns no
`Document` for them even though model conversion succeeds. The DOCTYPE-free
`OpusLink.musicxml` and `OpusScore.musicxml` fixtures are within the currently
assessable source scope. This is a developmental DTD-independent preview, not
validation support for the usual external-DOCTYPE MusicXML score exports.
A familiar public identifier is not treated as proof that an unread DTD is
infoset-neutral. Removing a DOCTYPE changes the input and is not equivalent
assessment of the original file. DTD closure remains later work.

`InspectXML(ctx, reader, AssessmentOptions{Profile: ProfileSourceXSDPreview})`
assesses original XML without creating a Go document. Use `ProfileSourceLocal`
instead to add the frozen local clauses. An empty/unknown profile,
invalid budgets, or an already canceled context fail before the first input read.
A nil operation error does not imply acceptance: inspect `Report.Satisfied()`.
Ordinary normative failures return a report with no operational error. Syntax,
I/O, cancellation, resource and fatal capability failures preserve their causes
with the best partial report.

`DecodeAssessed(ctx, reader, DecodeAssessmentOptions{...})` returns a
`DecodeResult` with independent `Source`, `Conversion` and `Document` fields.
A nonnil `Assessment` requires an explicit profile and strict acceptance; source
failure or incompleteness discards the returned document. A nil `Assessment`
keeps permissive model conversion and mandatory XML checks, with no catalog load
and `Source.Requested == false`. Existing `DecodeOptions` is unchanged.

The observer sees every original element, physical attribute, namespace binding,
child order and direct character segment before Go-model filtering or skipping.
The XML parser has one owner. After a recoverable model mapping error, that same
healthy parser is drained; the failed reflection decoder is never resumed.
There is no reader replay, second XML parse or model Encode→parse in this path.
The implementation retains a budgeted fact graph; single-pass input does not
mean constant-memory processing. The caller retains ownership of the reader.

The default 64 MiB `MaxBufferedBytes` is a safety cap, not a promise that a
64 MiB file or one million nodes can be assessed. For example, the 223,068-byte,
3,185-element official UTF-16 score needs about 13.94 MiB merely to finish its
unsupported external-DTD report; the otherwise identical DOCTYPE-removed
measurement control needs about 27.19 MiB for complete assessment. These are
fixture-specific logical allowances, not heap/RSS or a universal element limit.
Ordinary larger scores can exceed the default. Set informed finite overrides in
`AssessmentOptions.Budgets` where appropriate; all other limits still apply.
See the source scope's exact measurements and interpretation.

`ConversionConverted` means mapping succeeded under the existing Go model
contract, including its `float64` precision and field representation. It is not
an exact lexical or numeric round-trip guarantee. `ConversionUnrepresentable`
and `ConversionError` preserve machine-range errors without inventing XSD
invalidity. Strict error chains may include both an actual normative
`ValidationError` and `validation.IncompleteError` when both axes apply.

`ProfileSourceXSDPreview` is a developmental, source-only profile for
`score-partwise`, `score-timewise` and `opus`. Six sealed XSD operations cover
binding, element declarations, attributes, scalars, content grammar and document
identity, reusing the authoritative shared XSD implementation. Each selected
operation counts once per applicable subject, regardless of how many diagnostic
details it emits. A proved failure and incomplete remaining work can coexist.
A small diagnostic quota cannot turn failure/incompleteness into acceptance.
Repeated cause-free `dtd-infoset-unavailable` outcomes are displayed as one
bounded summary per full rule key and document, with the number of summarized
rule evaluations and the first representative occurrence. All instance counts
and blockers remain exact. Those summaries are admitted after ordinary details,
so they cannot consume a fitting later failure's only diagnostic slot. Other
unsupported reasons and causal blockers are not coalesced. `ValidationError.Issues`
contains only quota-admitted normative details and may be empty; a truncated
source error additionally explains the failed-evaluation count and truncation.

The preview remains a separate developmental profile. It excludes
MusicXML prose, direct current-model evidence, local typed musical relations,
timeline/repeats/context, packages, external documents and dictionaries.
Reached unverified XSD/DTD capabilities remain unavailable; catalog compilation
is an immutable bounded snapshot, not general XSD schema validation or UPA
closure. The [source assessment scope](docs/validation/source-assessment-scope.md)
defines its exact support, ownership, verification and remaining obligations.
`ProfileSourceLocal` adds only the [selected local clauses](docs/validation/model-assessment-scope.md#selected-musical-obligations)
and retains all source-parser/XSD/DTD limits. Empty accordion registration, for
example, passes the XSD-only preview and legacy `Validate`, but fails either local
profile. Strict source-local decode withholds that otherwise converted document.
No DTD stripping, fetching, trust or ignore-DTD option is introduced.

Positions use zero-based original-byte offsets, including BOMs, and one-based
decoded-character lines/columns. CRLF is one line break. If either coordinate
exceeds the host integer range, both are omitted as `0/0` for the rest of the
source; original-byte offsets remain available. Physical attribute
positions survive skipped content and foreign namespace lookalikes. Overlong
display paths are omitted while occurrence identity/positions remain available.
Fact identity ceilings (1024 UTF-8 bytes per local name/prefix, 4096 per namespace
URI and 65536 in-scope bindings) cause incompleteness, not malformed XML.
Clean input EOF can coexist with incomplete XML well-formedness assessment when
DTD declaration semantics remain unavailable. Context checks cannot interrupt a
blocking arbitrary `io.Reader.Read`; use cancellable I/O where needed.

## MXL packages

`MXLPackage.RootFiles[0]` identifies `Document`. `Resources` contains every
other regular file except `mimetype` and `META-INF/container.xml`.

Resource order and bytes are preserved; ZIP compression metadata is not.
Container elements and attributes are matched by their expanded XML names;
namespace declarations and foreign-namespace lookalikes cannot select root
files. Root-file paths and media types are interpreted using XSD token
whitespace normalization. Opus hrefs use XSD anyURI whitespace normalization
before URI parsing; literal ZIP resource names and percent-encoded spaces are
unchanged. Encoding validates archive paths and rejects collisions with
reserved or primary paths. Root-file paths and media types must also be
representable as XML 1.0 text, and are checked before whitespace
normalization. Opus links normalize dot segments in both relative and
archive-root-relative paths; references to directories cannot bind to a
same-named regular file.

`DecodeMXLWithOptions` and `DecodeMXLPackageWithOptions` expose the archive,
metadata, primary-document, per-resource, aggregate-resource, and XML-depth
limits. Zero-value fields select the package defaults. XML depth defaults to
256 and may be configured up to 4096; larger values are rejected because they
cannot be decoded safely on every supported platform.

`DecodeMXLScorePartwise`, `DecodeMXLScoreTimewise`, and
`DecodeMXLOpusDocument`, together with their `WithOptions` variants, return a
concrete root type when the expected MXL document kind is known. The `As...`
accessors cover `MXLPackage.Document` and other polymorphic document values.

`ResolveOpus` decodes linked XML resources with the default XML depth limit of
256, independently of the `MXLOptions.MaxXMLDepth` used to read the primary
document. There is currently no separate linked-document depth option.

`ResolveOpus` builds a memoized graph. Repeated links share targets, and
opus-link cycles between archive documents are supported. `SyncResolvedOpus`
accepts only the graph created for the same, unchanged package. It commits
linked-resource updates atomically.

## Errors

Public sentinel errors are intended for `errors.Is`, including
`ErrDocumentTooDeep` and `ErrDocumentCycle` for unsafe programmatically built
models. `UnsupportedRootError`, `MXLLinkError`, and `ValidationError` provide
structured context and are intended for `errors.As`.
