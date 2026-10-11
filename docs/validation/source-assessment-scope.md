# Single-pass source assessment scope

This page retains the batch-4 developmental source-XSD preview contract and
source obligations. The later [bounded model/local scope](model-assessment-scope.md)
adds explicit `ProfileSourceLocal` using this same source pipeline and adds
`AssessModel`/direct XSD-only `Validate` for current objects. It does not change
this preview's required keys or source/DTD limits; broader readiness is separate.

## Contract and architecture

**Practical input scope:** this preview cannot accept files with an unread
external DOCTYPE subset. All three committed official score examples
(`Chant.musicxml`, `MozartTrio.musicxml`, UTF-16 `MozaChloSample.musicxml`) have
one and therefore remain unsupported/incomplete. Strict assessed decoding
returns no document for them even when conversion succeeds. The two committed
opus examples have no DOCTYPE and are assessable. Thus this batch provides the
single-pass engine/API and a DTD-independent developmental execution scope; it
does not provide source acceptance for common external-DOCTYPE score exports.
The original-byte oracle tests preserve this distinction. Removing a DOCTYPE
is a different input, not evidence that its unread subset was neutral.

`InspectXML` and opt-in `DecodeAssessed` select
`musicxml-4.0/source-xsd-preview` explicitly. This developmental source profile
accepts the pinned no-namespace MusicXML `score-partwise`, `score-timewise` and
`opus` roots. It is not the first bounded production MusicXML profile. No default
profile, arbitrary catalog injection into the root API, package resolver or
implicit network access is introduced.

The input has one owner and one lexical scan:

1. A bounded original-byte cursor identifies the encoding, checks bytes and
   tracks original coordinates before normalization/transcoding.
2. The shared XML/namespace scanner validates original syntax and returns raw
   tokens with lexical namespace-declaration and position metadata.
3. A single source token decoder expands names. Observation precedes every
   model namespace projection, attribute filter and unknown-element skip.
4. The core owns bounded retained source facts. Private pending binding is
   resolved through exact shared-schema witnesses before exact selectors run.
5. Sealed per-node XSD operations assess binding, declarations, attributes,
   scalar domains/facets and content grammar. A document operation finalizes
   identity after actual EOF; earlier duplicate proofs survive later failure.
6. Assessed decoding optionally maps those same tokens into existing Go models.
   Recoverable mapping failure hands draining back to the parser owner. The
   failed reflection decoder never resumes.

There is no input replay, second XML parse, model encoding, or arbitrary
predicate callback. Retained facts can be traversed more than once. Memory is
bounded by the shared account, not constant in document size.

The `internal/xsd` implementation is authoritative for legacy validation and
new source plans. Root compatibility wrappers retain the existing XSD tests.
`validation.CompileSchema` creates a bounded immutable graph snapshot with
assembly-qualified exact component/use identities; `NewXSDPlan` snapshots its
handle. Catalog digests include selected schema graph digests and operation
identity. Compilation preserves lazy reached-metadata assessment: it is not
schema validity/UPA certification and does not erase cycles or alias identity.

## Facts and result semantics

Source facts retain expanded names, physical attribute presence, copied
namespace scopes, direct XML-normalized text, child order, occurrence IDs and
original-byte locations. The explicit optional `CompleteAttributes` event seals
start-tag enumeration before children. A generic unsealed/incomplete attribute
set cannot exclude hidden `xsi:type` or `xsi:nil` controls. Source lexemes are never replaced by XSD whitespace
normalization, selected union members or effective defaults. Kernel results keep
physical/defaulted status and effective values separately. Generic `Atomic`
facts are published only where its exact value vocabulary can represent the
result; lists and unsupported primitive families are not flattened to strings.
Absent physical attributes are not fabricated as present by defaults.

Binding comes from exact declaration/particle-use witnesses. Repeated local
names, anonymous types, references and assembly homonyms do not use paths or
QName-only guesses as semantic identities. Failed grammar leaves unwitnessed
subtrees unavailable or causally blocked. Proven wildcard `skip` excludes local
subtree schema operations; document identity closure remains a separate limit.
Unavailable order, scalar evidence or attribute presence never becomes a known
empty value or a fabricated grammar/required-attribute failure.

A composite operation contributes one mandatory predicate instance per subject.
Its zero or many displayed issue details do not multiply counts. Proven failure
and incomplete remaining work are independent axes. Verdicts are retained before
budgeted diagnostic/cache work. Diagnostic truncation cannot erase normative
failure, blockers or the terminal cause. Identity preview can retain duplicate
proofs; only final complete evidence can prove an absent target. Additional EOF
issues append without recounting the same predicate.

Only cause-free `dtd-infoset-unavailable` diagnostic instances are grouped by
full rule key within one document. Each bounded summary identifies its first
representative source occurrence and states how many rule evaluations it
summarizes; semantic counts, blockers and dependency decisions remain per
instance. These summaries are retained optionally and flushed after ordinary
findings, including on interrupted Finish paths. No diagnostics are evicted and
no cumulative quota is refunded. Omitted summaries mark truncation only after
ordinary storage. Other unsupported reasons and causal blockers remain distinct.

`InspectXML` ordinary normative failure is a report outcome with nil operation
error. Callers must inspect `Report.Satisfied()`. `DecodeAssessed` with a requested
profile rejects both violation and incompleteness and returns no document.
Conversion range failures are `ConversionUnrepresentable`; they do not alone
match `ErrInvalidDocument`. Source and conversion failures can coexist and keep
their original `errors.Is`/`errors.As` chains. Converted models retain their
existing numeric precision and field-mapping contract, not a source round-trip
guarantee.

## Precise preview support and exclusions

The six XSD families operate on generated score/opus metadata and reuse the
previous exact integer/decimal, whitespace, restriction/list/union, facet,
attribute, nil/default/fixed, actual-type, particle and identity implementation.
Research inventory counts are not executable predicate counts or test coverage.
`xsi:schemaLocation` and `xsi:noNamespaceSchemaLocation` receive their XSD
**datatype** contracts here. Schema-location pairing/discovery, loading and
resolver policy are not inferred from the former's list datatype.

Reached unsupported metadata or unfinished normative closure remains explicit:

- Opaque `anyType` content/attribute closure
- Wildcard lax/strict assessment and wildcard identity closure
- Unverified temporal, floating, binary, QName and NOTATION equality semantics
- Wider constructed list/union identity participation
- Absent optional default/fixed attribute identity participation
- DTD-dependent defaults, normalization, replacement text and declaration WFCs
- External dictionaries, schema discovery and linked resources

These are capability results, not false normative invalidity. Independently
proved datatype, attribute, content or duplicate-ID failures are preserved even
when another axis is unavailable. The legacy missing-IDREF oracle is not used to
claim complete source identity when omitted ID/IDREF defaults make its evidence
incomplete. Exact scope tests distinguish these cases.

The [parser scope](source-parser-scope.md) specifies syntax, XML editions,
encoding, DTD and compatibility details. Original-byte locations include BOMs;
line/column coordinates are decoded characters and CRLF is one line break.
Digest bookkeeping is private, with separate prefix/EOF completeness. No public
report is represented as a whole-document cryptographic certificate.

## Resource and lifecycle ownership

For requested assessment, one account is created before input. Nil-assessment
transport retains a nil account and its depth-only policy. Negative limits, unknown/empty profiles and
already canceled contexts fail before reading. Zero limits select finite
foundation defaults. Budgets cannot be reset or widened by nested work.

| Resource | Charging owner |
| --- | --- |
| Original input bytes | Cursor refill; an overflow byte is probed/hashed but never charged beyond the cap |
| Raw token/attribute-attempt events | Scanner before returning/growing tokens |
| Admitted node/attribute/depth facts | Session ingress exactly once |
| Retained lexical, observer, fact, index and cache data | Respective owner, before growth |
| Candidate Go model | Separate logical projected-token allowance through source Finish and acceptance |
| Scalar item limits | Scanner accumulation, fact ingress and typed-value bridge |
| XSD operations, graph/grammar/scalar/index work | Shared kernel meter |
| Identity declarations/references | Kernel before retained identity entries |
| Diagnostics | Report count/byte quotas, separate from semantic verdicts |

This is conservative logical allowance accounting, not a heap profiler. Exact
number conversion reserves bounded temporary/storage allowances and work before
big-number construction. Blocking arbitrary `io.Reader.Read` requires
caller-provided cancellable I/O; context checks cannot preempt an arbitrary read.

### Practical default retention limit

The 64 MiB buffered-data default can stop source assessment well before the
one-million-node limit. Input bytes, element counts and logical retained bytes
are different budgets; a small serialized score can retain much more metadata.
They do not define a universal per-element cost or a promised score-size ceiling.

For the committed UTF-16BE `MozaChloSample.musicxml`, which contains **3,185
elements**, the current Go 1.27.2 implementation was measured by binary-searching
`MaxBufferedBytes`, then verifying one below, exact and one above each boundary:

| Input | Original bytes | Minimum buffered allowance to finish | Result |
| --- | ---: | ---: | --- |
| Unchanged official file | 223,068 | 14,618,930 bytes (13.94 MiB) | EOF, six capability summaries; unsupported/incomplete |
| Controlled input with only its external DOCTYPE removed | 222,824 | 28,513,746 bytes (27.19 MiB) | Complete and satisfied for this changed input |

Both inputs contain the same 3,185 elements. The latter is a measurement control,
not equivalent assessment of the original file or a recommended silent rewrite.
These fixture-specific costs are approximately 4,590 and 8,953 logical bytes per element respectively; they are
neither heap/RSS measurements nor general constants. Active schema execution
retains more state than the unavailable-DTD path. The default can therefore be
exhausted on ordinary larger scores, far below `MaxNodes`; other budgets can also
stop work independently. Callers may select informed, finite larger limits via
`AssessmentOptions.Budgets`. Defaults are not increased or derived from input
bytes because representation cost depends on input shape and assessed operations.

Every successfully begun assessment is finished exactly once, including fatal,
I/O and cancellation paths. Cleanup releases owned live gauges, never cumulative
work, and never closes the caller's reader. Known closed-prefix failures are
assessed after a parse/I/O interruption only when the fact event stream remains
valid and context/resources permit it. Safe complete-match prefix witnesses do
not prove absent suffixes or unavailable original order.

## Verification contract

Permanent regression coverage includes:

- Shared-kernel parity across the existing original-source XSD fixture families
- Required external oracle tests over original bytes, including source order and
  repeated occurrences lost by model decode/re-encoding
- Source integer `2^64` assessment with an independent Go-model range failure
- Same-parser drain after mapping failure, including malformed/I/O/canceled tails
- UTF-8/BOM/UTF-16 positions, literal/reference normalization and namespace lookalikes
- Exact/excess active quotas, independent diagnostic count/bytes and gauge cleanup
- Immutable graph/handle ownership, pending-to-known binding and malformed prefixes
- Count-once/detail-many failure replay, identity EOF semantics and dependency gates
- Independently discovered absent-presence and unavailable-order regressions
- Concurrent run/report ownership, race tests and counted fragmented-input fuzzing

The workflow requires the raw-source oracle root
`TestSourceAssessmentAgainstSchema` alongside the existing 26 external roots,
with `MUSICXML_REQUIRE_XMLLINT=1`. The Linux test job executes `go test -count=1
./...` under `GOARCH=386`; cross-compilation alone is not runtime evidence.
`check-all` runs module, generation, formatting, tests, vet, race and counted fuzz
stages. Execution evidence and exact CI head are recorded in the PR, not implied
by this test inventory.

## Complete remaining ownership

[source-assessment-ownership.json](source-assessment-ownership.json) preserves
all 95 engineering requirements, their original acceptance/five-axis test
contracts, all 28 issue gates and 42 expanded source contracts. Original planning
statuses are unchanged. Separate dependency batches identify prerequisites, not
closure ownership. It records batch-4 scope and remaining ownership rather
than declaring entire cross-cutting requirements closed.

The retained inventories remain 2,560 research records, 2,218 XSD instances, 418
planned checks, 313 capability gates, 513 test contracts and 291 context
dispositions. This batch does not turn inventory entries into runtime passes.

- Batch 5: direct current-model traversal, local typed references/relations,
  all 29 profile-candidate dispositions and the first bounded production profile
- Batch 6: exact musical timeline and effective state
- Batch 7: broader context, repeats, spans and related musical constraints
- Batch 8: MXL/container/sounds and resolver/link/discovery obligations
- Batch 9: external normative corpora and remaining XSD/DTD/value/identity closure

All ambiguity gates retain clause-specific evidence and approval requirements.
No universal measure filling, initial C-major/4-4/clef/divisions defaults,
universal duration requirement, pairwise chord-duration rule or other guessed
musical strengthening is introduced. Nine batches are a packaging target, not
permission to drop requirements or force unresolved work into this preview.
