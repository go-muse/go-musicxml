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
details. Individual diagnostic previews are bounded and are not an exhaustive trace.
Legacy issue locations retain XML-style structural display paths; representation
messages also identify Go fields. These paths do not describe original XML.
`ErrNilDocument`, `ErrUnsupportedDocument`, `ErrDocumentCycle` and
`ErrDocumentTooDeep` remain recognizable where applicable. Depth and cycle errors
still match `ErrInvalidDocument` in this facade. The legacy depth ceiling remains
4096. Other assessment default quotas do not apply to this compatibility facade:
document/scalar/work/diagnostic ceilings use `math.MaxInt64`, while checked
accounting overflow and host-sized allocation guards remain active. This restores
the former size/work contract, not a memory or throughput guarantee. Memory and
work grow with the model. Use `AssessModel` with explicit finite budgets for
untrusted models and independent normative, model and resource outcomes; its
finite defaults are unchanged.

This is an intentional value-contract change from validating serialized text:
finite decimal fields mean their exact current binary rational, including at
bounds, enumeration, fixed and digit facets. For example, current `float64(0.1)`
is not decimal `1/10`; large finite values are no longer assessed through an
encoder's spelling. Unsupported numeric lexical patterns remain incomplete,
without inventing a source lexeme. No serialization fallback is retained.

`ErrIncompleteAssessment` identifies technical incompleteness, including its presence in a joined invalid/incomplete
result. It is the identical sentinel exported by the `validation` subpackage.
`errors.As` can also retrieve its `*validation.IncompleteError`. A legacy
model/XSD failure may be joined with either independent incompleteness or
dependent schema work blocked by that same proven failure. For example, a NaN
measure width definitely fails model representation, while its XSD attribute
assessment cannot establish a decimal value. `Validate` then matches both
`ErrInvalidDocument` and `ErrIncompleteAssessment`, whereas `AssessModel` has a
nil operation error and a report containing the model failure and causally
blocked XSD result. This joined legacy sentinel behavior differs from the former
serialized-value path. Test invalidity first when deciding whether to reject or
retry; incomplete alone does not mean that a definitive invalidity is absent.
The facade does not reinterpret the report as complete because a blocked result
has a known cause: a composite can also contain independent capability limits.
Diagnostic truncation cannot change these report axes.
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
| `ProfileModelTimeline` | `musicxml-4.0/model-timeline` | Model-local scope plus the bounded exact timeline/effective-state provider and selected chord/numeral consumers |
| `ProfileSourceTimeline` | `musicxml-4.0/source-timeline` | Source-local scope plus the same timeline provider and consumers on original input |
| `ProfileModelContext` | `musicxml-4.0/model-context` | Frozen model-timeline scope plus bounded typed context projections and selected contextual predicates |
| `ProfileSourceContext` | `musicxml-4.0/source-context` | The corresponding context extension on original input |

These are bounded implementations under final verification, not a claim of
whole-MusicXML conformance or completed batch acceptance. The complete selected
clause list, exclusions, test evidence and remaining owners are in the
[model assessment scope](docs/validation/model-assessment-scope.md) and
[timeline scope](docs/validation/TIMELINE_ASSESSMENT_SCOPE.md). Each profile
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

For XSD-only budget sizing, the following Go 1.27.2 linux/amd64 measurements
use `ProfileModelXSD` or `ProfileSourceXSDPreview`, with a shared account and
verified success at the exact minimum allowance and resource failure one
byte/step below it. These are fixture-specific costs after compact
optional-attribute absence storage, not universal per-node constants or
local/timeline-profile measurements:

| Input and target | Nodes | Minimum live-buffer allowance (bytes) | Rule steps |
| --- | ---: | ---: | ---: |
| Current `MozartTrio.musicxml` model | 2,516 | 16,321,980 | 2,784,531 |
| Same model with each part's measures repeated twice | 4,913 | 31,644,150 | 5,441,724 |
| Current `MozaChloSample.musicxml` model | 3,185 | 20,338,110 | 3,482,930 |
| Source MozartTrio, DOCTYPE-free measurement control | 2,516 | 18,209,624 | 2,893,122 |

For these current-model fixtures this is about 6.4–6.5 KB and 1,094–1,108 rule
steps per node; the source control is about 7.2 KB and 1,150 steps per node.
Thus the default buffer allowance admits roughly 10,000 model nodes or 9,000
source nodes in this family, before other limits; the step allowance alone
would admit roughly 18,000 or 17,000 respectively. Real scores can exceed these
defaults. These estimates are neither guaranteed capacities nor heap/total
allocation measurements: attributes, text, types, failures and selected local
rules change the cost. Choose informed finite overrides and handle resource
outcomes. The legacy `Validate` facade does not inherit these default ceilings.

The source control removes DOCTYPE only to isolate cost in an experiment; it is
different input, not a supported workaround for assessing the original file.
The original external-DTD score sources remain incomplete. The shared kernel
keeps ordered per-occurrence facts; unconstrained absent attribute descriptors
are shared only within a run and exact type owner. Present/default/fixed results
remain independently assessed. The session still retains a fact graph, so this
optimization does not claim constant memory or full conformance.

The direct legacy path has a substantial performance cost versus the preceding
Encode/reparse implementation. Five alternating warmed samples of the unchanged
`BenchmarkValidate` on `MozaChloSample.musicxml` (Go 1.27.2, linux/amd64, AMD EPYC
9V74, `GOMAXPROCS=4`, 750 ms per sample, fixture decode/I/O excluded) measured:

| Implementation | Median time (range) | Median allocated bytes/op | Median allocations/op |
| --- | ---: | ---: | ---: |
| Previous Encode/reparse (`511f55b`) | 27.02 ms (25.35–28.00) | 11,705,376 | 123,632 |
| Direct shared assessment after compaction | 100.73 ms (100.13–108.67) | 33,374,378 | 181,857 |

This is **3.73× time, 2.85× allocated bytes and 1.47× allocations** in this
measurement, not performance parity. Compaction reduces allocated bytes by
26.3% from the initial direct implementation's 45,257,229-byte median, while
allocation count rises slightly. The initial direct-path timing samples had different
host contention, so that series is not a matched timing comparison. The direct
path retains schema/session facts and exact current-value/report semantics;
these measurements disclose the remaining cost rather than promising a
throughput or memory improvement. Cold initialization adds further cost.

### Exact timeline and effective-state profiles

Select `ProfileModelTimeline` with `AssessModel`, or `ProfileSourceTimeline`
with `InspectXML` / the explicit assessment in strict `DecodeAssessed`.
These profiles use a separately compiled engine and add exactly three required
bindings to their corresponding local profile: `MX40-TIMELINE-effective-state@1`,
`MX40-TIMELINE-chord-duration-bound@1`, and
`MX40-TIMELINE-numeral-requires-key@1`. Model/source have 39/37 required keys,
respectively, with no additional transitive-only keys. The existing XSD/local
catalogs and their digests remain frozen. Legacy `Validate` remains XSD-only;
ordinary `Decode` and `Encode` do not select a timeline profile.

The session-owned provider retains exact rational musical positions and typed
divisions, key, time, clef, staff, transpose and instrument histories. It includes
independent display/sound/listening offsets, note attack/release/bend origins,
proved grace donors, figured-bass anchors and anchor-based durations, direction
association, barline/navigation statements, and sparse sound/instrument/MIDI
statements. These are bounded context facts, not additional rejection rules.
A provider pass means indexing and finalization completed; it does not assert
that every optional or ambiguous fact is known. This release does not expose a
general public timeline-query or playback API.

The selected consumers check each duration-bearing chord tone against its
established principal note, not the preceding chord tone, and require a valid
local numeral-key or an applicable explicitly encoded key at the harmony's
unshifted score origin and staff. Display offsets cannot borrow a future key.
No initial C major, meter, clef, tempo or divisions value is invented. An own
valid numeral-key remains an independent alternative when fallback timeline
evidence is unavailable.
For the duration-only chord bound, the principal is the established preceding
non-chord `<note>` element, including a duration-bearing rest. The bound does
not decide pitch-based chord eligibility or musical/rendering intent; a rest
principal uses the same shorter/equal/longer duration comparison.

When harmony staff is omitted in a multi-staff part, encoded-key presence can
still be proved if the AllStaves-only baseline and every observed staff-scoped
publication/gap partition agree. This does not select a staff or a concrete key,
and it never enumerates the encoded staff count. The baseline is retained even
when numbered publications appear to cover all staves. If the projection can
vary with staff, it remains Unknown with `timeline-staff-unresolved`; malformed
or unavailable prerequisite evidence retains its actual causal reason. General
concrete-key queries still require resolved staff evidence.
A projection may prove encoded-key presence, or an unchanged divisions value,
without identifying a unique latest publisher. This requires an independently
established baseline and independently valid same-result alternatives; missing,
invalid or hidden candidates remain causal. Actual source witnesses survive,
and uncertain inheritance is retained rather than replaced by fabricated
provenance.


Exact established regions remain usable when another region is incomplete.
Unknown same-time conflicts, missing staff/voice/anchor evidence, and unsettled
cursor movement across divisions boundaries retain their specific causes;
they are not silently defaulted or turned into new musical violations.
Signed offsets remain exact local coordinates; continuing-state selection across
measure frames also needs established containment/order evidence. No measure
length is invented from meter or a terminal note endpoint.
Cursor normalization is checked against complete unit history. Staged proofs use
only independently established prior-stage origins/units; derived descendants
cannot certify an unresolved ancestor or its frame bound. Final facts are
replaced only after successful closure.

Grace donor identity does not establish continuous playback time. Figured-bass
consecutive-chain positions are explicitly derived context, not a sum-duration
restriction. Instrument state means latest applicable encoded statements, with
exclusive solo/ensemble and atomic virtual descriptors, not a reconstructed
synthesizer configuration. A sound `time-only` guard is retained on raw updates;
without repeat-pass evidence, only its stated effective properties become
unavailable, and omitted properties make no update. The reviewed
[offset](docs/validation/timeline-offset-interpretation.md),
[note-origin](docs/validation/timeline-note-origin-interpretation.md), and
[instrument](docs/validation/timeline-instrument-statements.md) contracts state
the exact source interpretations and remaining boundaries.

The profiles retain all target-specific XSD, representation and unread-DTD
limits. Full repeat/span, layout, performance realization, author intent,
packages, linked content and external dictionaries are outside their closure.
Opus selection assesses that document only. Predicate counts and the two
consumer milestones are not complete MusicXML coverage or a ceiling on the
provider's typed-state responsibilities. Final acceptance evidence is tracked
in the [full ownership ledger](docs/validation/timeline-assessment-ownership.json).

Timeline arithmetic, indexes and retained context share the session's finite
budgets; a small input can still exceed work or memory limits.
Contained state histories use indexed lookup; exceptional ambiguous cross-frame
candidate comparisons can be quadratic under those same limits. Exceptional
reconstruction can retain both old and staged services until session cleanup;
that conservative cost remains charged, with no automatic budget increase.
The earlier XSD-only costs are not timeline capacity promises. A matched
same-head budget probe at published `632a969` decoded the unchanged
`MozartTrio.musicxml` once, then assessed its **2,516 model nodes** under each
profile. Exact minimum buffer/step limits pass; one byte/step below fails with
the corresponding typed resource error. Both profiles also pass defaults.

| Resource | `ProfileModelLocal` | `ProfileModelTimeline` | Timeline increment | Increment per node |
| --- | ---: | ---: | ---: | ---: |
| Minimum `MaxBufferedBytes` (logical live bytes) | 18,155,049 | 21,077,650 | 2,922,601 (+16.1%) | 1,161.61 bytes |
| Minimum `MaxRuleSteps` | 2,848,163 | 4,193,534 | 1,345,371 (+47.2%) | 534.73 steps |
| Cumulative context facts consumed | 0 | 1,199 | 1,199 | 0.476550 facts |

The respective local/timeline totals are **7,215.84 / 8,377.44 logical bytes**
and **1,132.02 / 1,666.75 steps per node** for this fixture. The timeline requires
`MaxContextFacts=1199`; 1198 fails. Local consumes no context facts here: 1 is
merely its smallest expressible positive limit, because 0 selects the default.
A local session with zero remaining context allowance also succeeds. These
per-node averages are sizing observations, not universal coefficients: scalar
size, types, attributes, diagnostics, state histories and ambiguous cross-frame
reconstruction change the work, retained scratch and publication counts.

`MaxBufferedBytes` accounts for conservatively admitted logical live retained
data and scratch, including overlapping old/staged services. The minimum above
is found by successful-run boundary search; it is not the account's usage after
`Finish`, when this fixture's live gauge is zero. Cumulative allocated bytes
(`B/op` or a `TotalAlloc` difference) also count temporary allocations released
within a run; neither metric is process RSS or a Go heap ceiling. Compiled
metadata, runtime caches, allocator overhead and the caller's model are not a
promise made by the session's live-buffer allowance. Defaults remain **64 MiB
live buffer, 20 million rule steps and one million context facts**. Size finite
overrides for the selected profile and handle resource incompleteness.

Matched warmed **1,000-note** results at production `22ae68a` (the same production
code as `632a969`) disclose the incremental timeline cost:

| Target | Local median time | Timeline median time | Time ratio | Allocated bytes/op, local → timeline |
| --- | ---: | ---: | ---: | ---: |
| Source | 211.05 ms | 292.55 ms | 1.39× | 52,677,464 → 78,759,784 (1.50×) |
| Model | 211.73 ms | 263.81 ms | 1.25× | 48,341,472 → 74,423,784 (1.54×) |

These are five separate warmed runs per case, Go 1.27.2 linux/amd64,
`GOMAXPROCS=4`, 200 ms per sample, with finite timing-only limits of 512 MiB
buffer and 2^34 steps. Source includes parsing; model reuses a decoded model.
The [independent resource checkpoint](docs/validation/evidence/timeline-resource-independent-checkpoint.md)
records ranges, allocation counts and controls. Shared-host timing noise remains;
the medians do not establish universal throughput or default-budget capacity.

The [PR 35 review](https://github.com/go-muse/go-musicxml/pull/35#discussion_r4246488604)
additionally reported these matched local/timeline figures at `632a969`:

| Model fixture | Local time | Timeline time | Cumulative allocation, local → timeline |
| --- | ---: | ---: | ---: |
| `MozaChloSample.musicxml` | 68 ms | 76 ms | Not reported |
| `MozartTrio.musicxml`, measures repeated 10× | 0.61 s | 0.71 s | 269 MB → 344 MB |
| Same, 50× | 3.88 s | 4.50 s | 1.44 GB → 1.83 GB |

The replicated cases used lifted budgets. These timing/allocation values are
reviewer-reported, not independently remeasured here; their exact budget values,
sampling method and MB/GB convention were not supplied. The independently
reproduced budget boundaries, finite probe limits, raw output and procedure are
in the [PR 35 budget evidence](docs/validation/evidence/pr35-timeline-budget-costs.md).
**B5-PERF-01 remains outstanding** with the shared validation kernel/session
workstream and final batch-9 reconciliation. Incremental timeline measurements
and initialization improvements do not close the older matched-performance
obligation.

### Bounded context profiles

`ProfileModelContext` (`musicxml-4.0/model-context`) and `ProfileSourceContext`
(`musicxml-4.0/source-context`) compose the frozen timeline profiles with six
sealed families: local defaults/applicability, type-specific spans, graphical
repeats, bounded navigation, layout and encoded performance/timing. Earlier
profile IDs, selected rules, digests and ordinary validation remain unchanged.

`validation.Report.Context` exposes independently owned `ContextResult` values.
A closed `ContextProperty` identifies a scalar evidence value or a typed
`SpanContextResult`, `RepeatContextResult`, `NavigationContextResult`,
`LayoutContextResult` or `TimingContextResult`. Local scalar defaults also use
layout-shaped scope/applicability metadata. A provider's completion does not
claim that each optional field is known. Mandatory consumers separately assess
exact `time-only` ordering, supported bend-release signs and proved graphical
repeat content. Nonempty ASCII-digit figure-number content satisfies a sufficient
numeric subset; other spellings retain an unsupported unreviewed grammar gate.
This does not define or reject the remaining numeric lexical domain.

`NavigationContextResult.MarkerActive` records activation of an encoded barline
marker; `Guard` records pass-selection evidence. Neither field is a realized
playback count. Traversal extent remains separately described by `Complete`;
ambiguous mixed jump/repeat control preserves an unavailable bounded prefix.

Catalog users can bind the same sealed contracts with `NewLocalContextPlan`,
`NewSpanContextPlan`, `NewRepeatContextPlan`, `NewNavigationContextPlan`,
`NewLayoutContextPlan`, `NewPerformanceContextPlan`, `NewTimeOnlyOrderPlan`,
`NewBendReleaseSignPlan`, `NewGraphicalRepeatContentPlan` and
`NewFigureNumberNumericPlan`. `NewOpusLayoutContextPlan` is separately
bound to the Opus assembly and projects only local score pagination hints;
missing new-page remains application-dependent and no linked score is opened. `ContextPlanSpec` binds the exact selected timeline
provider where needed; `ContextConsumerSpec` binds an exact type-specific
provider. There are no caller expressions, evaluator callbacks or unmetered
post-Finish queries. Configuration is schema-bound and revisioned separately
from the accepted timeline contract.

Projection is admitted under the same context-fact, work and buffered-byte
budgets before cleanup. Reports are editable caller snapshots, not immutable
certificates. Unavailable optional geometry, author intent and physical playback
realization remain per-field evidence or explicit exclusions; they are not new
conformance failures. Repeated/ambiguous ranges remain bounded by the selected
finite budgets. The matched `BenchmarkContextProfiles` compares warmed timeline
and context profiles with documented larger finite budgets; it does not close
`B5-PERF-01` or promise default-budget throughput.

The repository's official MusicXML 4.0 samples `MozartTrio.musicxml` (262 notes)
and `MozaChloSample.musicxml` (278 notes) do **not** complete either context
profile under the default budgets. Both stop at the 20,000,000-step limit;
raising work alone still reaches the default 64 MiB buffered-byte limit.
Explicit finite `MaxRuleSteps: 1 << 34` and `MaxBufferedBytes: 512 << 20`
complete the measured source/current-model cases. Source measurements remove
only the unsupported DOCTYPE; MozaChlo's UTF-16BE BOM and encoding declaration
are preserved. Unmodified DOCTYPE-bearing source remains unsupported by the
source assessment contract.

The corrected Mozart model run uses 40,509,894 shared steps, 12,874 facts and
11,105 records; its measured minimum buffered limit is 90,509,904 bytes.
The MozaChlo model run uses 46,226,065 steps, 14,161 facts and 12,337 records,
with a 97,906,233-byte minimum. These are fixture-specific results, not a
score-size guarantee. See [matched resource evidence](docs/validation/evidence/context-round1-resource-acceptance.md)
for source minima, exact/one-under controls and cleanup.

See [scope and limits](docs/validation/context-assessment-scope.md) for the
source/model contract and deliberately retained normative boundaries.


## Fact-stream validation foundation

The `validation` subpackage exports `NewCatalog`, eight generic sealed typed-plan
constructors, the sealed shared-schema `NewXSDPlan`, bounded local relation and
typed-reference constructors, `NewModelRepresentationPlan`, `NewTimelinePlan`,
`NewChordDurationPlan`, `NewNumeralKeyPlan`, `NewEngine`, and
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

`NewTimelinePlan(TimelinePlanSpec)` requires the exact compiled MusicXML schema
and both part-reference owners. `NewChordDurationPlan(TimelineConsumerSpec)` and
`NewNumeralKeyPlan(TimelineConsumerSpec)` require the matching schema and exact
provider rule/revision; their rule definitions must also declare that document
dependency. Sealed role/type validation rejects similarly named but incompatible
schema declarations. These plans do not add generic interpretation to unrelated
context keys or expose mutable session state.

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
resource limits are not XSD facets. `validation.Rational` / `NewRational` add
an immutable reduced fraction for musical positions, including nonterminating
values such as `1/3`; they do not widen `Decimal` or XSD `Atomic`. The zero wrapper
is uninitialized. `Value` returns a copy, `String` returns a canonical fraction
(or an integer), and `ByteSize` counts significant magnitude plus text. Constructor
inputs, intermediate magnitude and canonical output are statically bounded by
`MaxExactValueBytes`; overly large unreduced inputs may be refused even if they
would cancel. Internal session arithmetic additionally precharges conservative
quadratic work and live scratch before normalization, cross-products or rendering.
Work counts architecture-independent logical bulk units (32-bit limbs and
four-byte text chunks), not individual bits or literal machine instructions;
large operands retain quadratic costs. Its output is retained until `Finish`;
resource rejection means incomplete
assessment, not musical invalidity. Cancellation is checked before and after
synchronous `math/big` calls, which cannot be preempted in flight.

`DecodeOptions` and generated public field
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
to add the frozen local clauses, or `ProfileSourceTimeline` to add the bounded
timeline provider and consumers described above. An empty/unknown profile,
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
64 MiB file or one million nodes can be assessed. The current budget-sizing
table above measures the DOCTYPE-free MozartTrio source control at 18,209,624
bytes and 2,893,122 rule steps for 2,516 nodes: about 7.2 KB and 1,150 steps per
node, or roughly 9,000 such nodes under the default buffer allowance. Those are
logical accounting estimates, not heap/RSS measurements or a universal element
limit. Ordinary larger scores can exceed the default. Set informed finite
overrides in `AssessmentOptions.Budgets`; all other limits still apply. Earlier
source-scope measurements remain historical evidence for their stated revision,
before the shared absent-attribute optimization.

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
