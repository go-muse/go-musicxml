# Bounded validation foundation

The `validation` package implements the batch-3 catalog, typed-plan, fact-session,
report and shared-budget foundation. It is executable through exported APIs,
including the [external-package example](../../validation/example_test.go).
It does not parse XML, walk a `musicxml.Document`, resolve resources, or supply a
production MusicXML conformance profile. Source, model and package target labels
identify caller-supplied facts; they do not create those adapters.

This scope supersedes proposal-only descriptions for the foundation implemented
here. The [proposed contracts](go-contracts.md), [catalog plan](catalog-and-test-plan.md)
and [XSD prerequisite ledger](xsd-prerequisite-scope.md) retain their broader
obligations. No whole `STAGE-DEF-04`, `STAGE-DEF-10`, or profile is declared ready,
and no planning JSON status is changed by the tests cited below.

## Compiled execution and inspection are separate

`NewCatalog` snapshots `RuleDefinition`, `Profile` and `PlanBinding` inputs into
an immutable `CompiledCatalog`. `NewEngine` accepts that catalog; `Engine.Begin`
requires an explicit profile and document-occurrence identity. `Rule`, `Profile`
and `Schedule` return independent copies. The engine/catalog can be shared by
concurrent independent sessions; callers must serialize each session's methods.

A rule has a `RuleKey` (semantic ID plus revision), exact selector, target, scope,
class, role, diagnostic category, provenance, English explanation, typed plan ID
and explicit dependencies. Research IDs are provenance, not runtime rule IDs.
Every profile states its MusicXML version, registry pin, supported roots and
normative closure. Its support rows cover exactly its required rules and all
transitive rule prerequisites, including unsupported obligations. Missing plans
cannot be labeled available. Omitting an unavailable obligation is not support.

Selectors distinguish assembly-qualified type components and exact declaration
or reference-use occurrences. Declaration identity includes assembly, owner and
use occurrence, so equal local names or QNames need not select the same rule.
Only exact type-component and element-declaration selectors execute. Other
component kinds, sibling/measure/part/package scopes, musical-position/part
relations and external services remain unavailable. Node and document scopes,
and same-node, parent and document rule/fact dependencies, are implemented.
Static unsupported support and unavailable evidence are separate dispositions.
A proven selector mismatch is not-applicable; unknown, unsupported or invalid
binding evidence cannot silently select nothing and produce success.

Compilation validates metadata, parameter bounds, full revision references,
target agreement, dependency closure and cycles. It permits at most 65,536 rules
and plans, 4,096 profiles and 64 MiB of configuration accounting, with a separate
64 MiB bound on schedule phase edges (524,288 unique edges at 128 bytes each).
These are logical accounting limits, not Go heap measurements.

The compiled phase graph adds ordering-only edges from selected context writers
to matching readers. All eligible declarations of one identity index register
before any same-index uniqueness/reference verdict; registration itself waits
for the declaration's semantic prerequisites. `Schedule` exposes each rule once
in final-evaluation order, without exposing the private registration phases.
Compilation rejects phase cycles, including a declaration that requires another
same-index uniqueness verdict before registration. Derived ordering does not add
rules to the profile closure, change `Requires`, or make provider non-applicability
a failed prerequisite. Missing/unselected providers remain evidence limitations.

All profiles receive the SHA-256 digest of canonical catalog inputs, including
typed plan parameters and profile claims. Set-like collections are sorted;
supplied digest fields are excluded and any nonempty supplied digest must match.
Changing semantics requires a new rule revision; changes to metadata, provenance
or profile claims also change the digest. Authors own semantic revision policy:
compilation does not compare a catalog with previously published versions.

`MusicXML40Inventory()` instead returns independently owned inspection data
compiled from committed [planning files](plan). Its registry commit is
`3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c`. It preserves all 2,560 research records
(2,218 XSD and 342 prose), 2,218 parameterized instances, 418 planned checks,
313 capabilities, 95 implementation requirements, 513 test contracts and 291
context dispositions. These are provenance counts, not runtime coverage.
All inspection instances/checks retain unsupported runtime bindings; capability
providers remain unbound. Original planned, decision-required, advisory, excluded
and optional dispositions remain intact. Recursive schema-component references
in this inventory are not executable dependency cycles.

The inventory carries a canonical payload digest, a sorted source-manifest
digest, exact source-file hashes and pinned registry pointers. Its generator
validates local planning integrity without fetching or synchronizing the registry.
It supplies no profile and executes no predicate. See [generation](../../CONTRIBUTING.md#generated-code).

## Eight bounded plan families

Plans are sealed immutable values produced by constructors, with no callbacks,
XML parser, expression interpreter or runtime DSL:

| Constructor | Implemented contract |
| --- | --- |
| `NewChildCountPlan` | Count exact child declarations within minimum/maximum bounds; `-1` means no normative maximum, not unlimited resources. |
| `NewAttributePresencePlan` | Require or forbid one exact declaration/name slot on its owner. |
| `NewAtomicComparisonPlan` | Compare an already-typed element value; equality for supported atomic kinds, ordering only for exact integers/decimals. |
| `NewContextValuePlan` | Publish a copied constant for an exact selected node and same-node, parent or document scope. |
| `NewContextDefaultPlan` | Publish the exact attribute value or a reviewed fallback only after proven absence. |
| `NewContextComparisonPlan` | Compare an exact generic context fact without inventing a missing provider or default. |
| `NewIdentityDeclarationPlan` | Index one exact attribute slot containing an upstream-domain-validated string key. |
| `NewIdentityReferencePlan` | Resolve that kind of slot against a named document index after complete enumeration. |

Attribute plans assess one exact slot per owner/plan. There is no general
attribute-scalar subject ABI or wildcard selector. Count is not child grammar;
atomic comparison is not XSD lexical, facet, list or union assessment. Generic
identity is not XSD ID/IDREF lexical or actual-type binding, nor a musical
reference-kind/scope check. Existing XSD helpers remain the authoritative bounded
implementation pending real fact binding, rather than being copied here.

`Evidence` distinguishes known, absent, unknown, unsupported and invalid facts.
Only known evidence carries a value; invalid evidence identifies its causal
rule revision and document-qualified node. `Scalar` keeps lexeme and value
evidence separate. Current model values need no invented source spelling.
Attribute/child absence needs complete relevant enumeration. An incomplete count
may prove an exceeded maximum, but cannot prove an unmet minimum.

Context publications preserve copied values, origins and provider identities.
Conflicting publications remain unknown; later writers cannot silently replace
unavailable/conflicting evidence. Generic document contexts have no musical-time,
staff, voice or effective-state interpretation. Default plans never treat
unknown or unsupported presence as absence. Identity indexes preserve forward
references, duplicate evidence and causal ambiguity. A missing target fails only
with complete provider enumeration; incomplete input cannot prove absence.
Known duplicates can be reported before EOF and survive later interruption.
Completeness is checked in the relevant provider domain: unrelated incomplete
subtrees do not erase a proven local failure or duplicate, while possible unseen
parent/document publishers prevent invented context values.

A known, well-formed integer, decimal, boolean or QName atomic value is outside
the identity plans' string-key family. It produces a per-rule
`OutcomeUnsupported` with reason `identity-family-unavailable`, without coercion
or terminating independent assessment. An unsupported declaration makes its
index incomplete, so an unresolved reference is unknown rather than a fabricated
missing-target violation. An unsupported reference does not make a complete
declaration index incomplete. Malformed atomic payloads, including invalid tags
or uninitialized numeric wrappers, remain terminal fact-contract failures.

`NewInteger(*big.Int)` and `NewDecimal(*big.Rat)` copy exact values; accessors
return copies, comparisons reject uninitialized operands, and zero wrappers are
not numeric zero. Decimals must have finite base-ten expansions. Neither
constructor parses XML or source text. `MaxExactValueBytes` is 1 MiB, independently
limiting significant magnitude bytes (numerator plus denominator for decimals)
and canonical output bytes. This constructor resource cap is not an XSD facet,
and is separate from session scalar/buffer budgets. `ByteSize` includes retained
magnitude and canonical text, excluding fixed Go object overhead.

## Session lifecycle and acceptance

Use `Open`, `Attribute`, `Close`, then exactly one `Finish`. Attributes precede
children on their open owner. Node IDs are nonzero and unique per document;
parents, target/document locations, evidence and balanced closure are validated.
Retained facts are copied. The current implementation retains a bounded fact set
until finalization; it does not promise streaming constant memory.

Independent local rules execute at `Close`; pending document, context and
reference obligations finalize in schedule order. Every event error is terminal.
Later events are rejected without further assessment. Call `Finish` even after
failure or cancellation: it releases the session's live depth/buffer charges,
preserves prior findings, and retains the original cause plus distinct later
completion/cancellation causes. Repeated `Finish` returns a session error and no
second report. No reader or resolver exists here for the core to interrupt.

Pass and not-applicable outcomes use compact per-rule, per-node state for
dependency execution rather than full cached `Decision` payloads. This state is
charged separately from diagnostics to `MaxBufferedBytes`: 32 logical bytes per
successful/excluded entry plus 128 bytes and the rule ID length for each rule's
table. Problem-dependency entries retain outcome, reason and cause, without value
previews or related-location lists. These are bounded accounting allowances, not
measurements of Go map overhead.

Each `RelationDocument` rule-prerequisite aggregate is computed lazily on its
first actual use after the prerequisite's final evaluation phase, then reused
by all consumers. Excluded consumers do not trigger an aggregate scan. The
aggregate reserves 512 logical bytes plus the prerequisite rule ID length in
`MaxBufferedBytes`, separately from the per-node result table. Aggregation keeps
the same subject-order causal priority and incomplete-enumeration semantics as
an uncached scan. This avoids repeated document scans for the same prerequisite;
it does not remove rules-times-nodes dispatch, general context/identity work, or
the bounded fact set retained until finalization.

`Report` independently records requested profile/target, syntax, conformance,
assessment completion, input completion, mandatory normative counts, required
non-normative blockers, diagnostic truncation and terminal infrastructure status.
`ConversionResult` only supplies separate conversion vocabulary; conversion and
source/model orchestration are not implemented.

A nil operation error from `Close` or `Finish` does not mean acceptance.
Violations and evidence-only unknown/unsupported outcomes live in the report.
Use `Report.Satisfied()` for its explicit acceptance axes: requested matching
profile/target, complete input and assessment, appropriate syntax, conformant
normative result, and no blockers or terminal failure. Source facts require a
well-formed syntax assertion; model/package facts use not-assessed syntax.
A zero, unrequested or contradictory report is never satisfied. Reports are
editable result values, not tamper-proof certificates.

Only `CategoryNormative` + `ClassMandatory` + `RolePredicate` contribute to
`Counts`. Required model, policy or context-provider failures block acceptance
without becoming normative failures. Their unknown/unsupported/blocked outcomes
also prevent complete assessment. Advisory outcomes do not block acceptance.
A normative failure remains violated even when another obligation or input is
incomplete; unknown-only normative assessment is undetermined.

`Report.Findings` contains only fail, unknown, unsupported and blocked diagnostics.
`Report.Rules` contains metadata only for those retained findings. Pass and
not-applicable outcomes still contribute to eligible counts and dependency
execution, but do not consume `MaxDiagnostics` or `MaxDiagnosticBytes` and cannot
set `DiagnosticsTruncated`, even when their metadata would exceed those limits.
There is no all-decisions trace option. Diagnostic count/byte exhaustion truncates
problem storage without stopping assessment or changing counts/blockers.
Terminal infrastructure status has a separate bounded reserve and does not invent a MusicXML rule. Reports own their retained metadata
and remain usable after cleanup. `Format` and `FormatLocalized` work offline,
escape untrusted text, identify source coordinate spaces, and never invent model
line numbers. Formatting caps fields at 4,096 encoded bytes, total output at
1 MiB, findings/rules at 1,000 and nested records at 64; omissions are marked.
Localization changes text only, with bounded English fallback. Writer errors,
including short writes, propagate.

### Terminal disposition

`Report.Terminal` selects the first matching row below from all retained causes
and the validated completion assertion. This precedence is independent of the
order of joined causes. The returned incomplete error preserves the original and
distinct later completion/cancellation causes through `errors.Is`/`errors.As`; selecting
one summary neither discards them nor changes proven normative failures.

| Priority | Matching terminal condition | Category | Reason code |
| ---: | --- | --- | --- |
| 1 | `errors.Is(err, ErrResourceLimit)` | `CategoryResource` | `resource-limit` |
| 2 | `errors.Is(err, context.Canceled)` | `CategoryResource` | `canceled` |
| 3 | `errors.Is(err, context.DeadlineExceeded)` | `CategoryResource` | `deadline-exceeded` |
| 4 | Valid source completion asserts `SyntaxMalformed` with an explicit `Completion.Cause` | `CategorySyntax` | `malformed-source` |
| 5 | `errors.As` finds `*UnsupportedRootError` | `CategoryCapability` | `unsupported-root` |
| 6 | `errors.Is(err, ErrInvalidSession)` | `CategoryCapability` | `invalid-session` |
| 7 | `errors.Is(err, ErrInvalidConfiguration)` | `CategoryCapability` | `invalid-configuration` |
| 8 | Other infrastructure interruption | `CategoryCapability` | `assessment-interrupted` |

For example, a joined resource-limit/cancellation error remains a resource-limit
summary even with a valid malformed-source assertion, while a session error
wrapping configuration failure is summarized as invalid-session. Invalid
model/package syntax assertions, a malformed-source assertion without an explicit
cause, or contradictory `InputComplete` plus `SyntaxMalformed` are capability
contract/lifecycle failures, not proof of malformed XML. `CategoryPolicy` is
reserved for explicitly categorized rule outcomes; it is not a terminal fallback.

## Shared finite resource accounting

All 14 `Budgets` fields are `int64`. Zero selects the finite defaults below;
negative limits fail configuration. The defaults are safety ceilings, not
throughput or full-assessment guarantees. Larger byte limits do not guarantee
that every other quota can accommodate an input of that size.

| Field | Default | Unit, mode and charging owner |
| --- | ---: | --- |
| `MaxInputBytes` | 64 MiB | Cumulative primary raw bytes; future source reader. No input reader in this core. |
| `MaxExternalBytes` | 64 MiB | Cumulative resolved/decompressed bytes; future package/loader owner. |
| `MaxBufferedBytes` | 64 MiB | Live logical retained bytes; core facts, compact success/not-applicable tables, problem-dependency state, document aggregates, context/identity and report-profile metadata; future adapters own their private buffers. |
| `MaxDepth` | 1,024 | Live open fact scopes, charged/released by the session; future private traversal/link scopes must share the account without charging the same scope twice. |
| `MaxNodes` | 1,000,000 | Cumulative `Open` node attempts, charged by the core after event admission. |
| `MaxAttributes` | 4,000,000 | Cumulative admitted owner-slot attribute attempts, charged by the core. |
| `MaxEvents` | 8,000,000 | Cumulative admitted `Open`/`Attribute`/`Close` attempts; future raw-token work is a distinct additional unit. |
| `MaxScalarBytes` | 1 MiB | Per-item lexeme/value payload cap via `Check`; no cumulative scalar quota. |
| `MaxIDs` | 1,000,000 | Cumulative retained declaration and reference slots, charged by generic identity plans. |
| `MaxRuleSteps` | 20,000,000 | Cumulative dispatch, dependency, search/index and comparison work; byte-proportional value work where implemented. |
| `MaxContextFacts` | 1,000,000 | Cumulative context publication attempts, including conflict/provenance work; core providers. |
| `MaxLinkedDocuments` | 128 | Cumulative linked document occurrences; future resolver/loader owner. |
| `MaxDiagnostics` | 1,000 | Cumulative stored fail/unknown/unsupported/blocked findings, charged by report accumulation; pass/not-applicable outcomes are excluded. |
| `MaxDiagnosticBytes` | 1 MiB | Cumulative stored problem-finding and distinct associated rule metadata payload plus fixed allowances; report accumulation excludes pass/not-applicable outcomes. |

`NewBudgetAccount` creates the only valid account; nil/zero accounts reject
accounting operations. A nil `RunOptions.Account` creates an independent account.
A supplied account requires zero `RunOptions.Budgets`; children cannot reset or
widen it. Share its pointer across related sessions. `Charge` handles cumulative
quotas/live gauges; only depth/buffer gauges support `Release`; scalar caps use
`Check`. Exact-limit operations succeed and refusals/overflow leave usage
unchanged. Cleanup refunds only the session's own live acquisitions, never
cumulative work. Diagnostic count/byte reservations are atomic.

The buffer and diagnostic byte models account logical payload and fixed record
allowances; neither is a precise process-heap or garbage-collector cap. Immutable
catalog compilation and exact-value construction have separate finite bounds.
`AllocationSize` checks host-int conversion but does not charge or allocate.
Future adapters must account before their own reads, parsing and allocations,
including skipped/foreign XML work, and must not re-charge core-emitted counters.
Direct-model visited fields/choices map to node/work accounting when its walker
exists; this batch supplies no measured direct-traversal guarantee. Package
children must retain all existing MXL limits as well as the shared account.

## Public error compatibility

`musicxml.ErrIncompleteAssessment` aliases the identical
`validation.ErrIncompleteAssessment` sentinel. `validation.IncompleteError`
preserves causes through `errors.Is`/`errors.As`; `ResourceError` matches both
resource-limit and incomplete-assessment sentinels. Configuration and session
errors remain separately inspectable. Terminal session errors do not fabricate
`ErrInvalidDocument` or a `ValidationError`.

The legacy private XSD carrier now exposes the public incomplete identity and
typed cause. Pure incompleteness does not match `ErrInvalidDocument`; a genuine
XSD `ValidationError` joined with incompleteness matches both families and retains
its original issues. Its error text intentionally migrates from
`musicxml: incomplete XSD assessment: …` to
`validation: incomplete assessment: incomplete XSD assessment: …`.
Callers should use `errors.Is`/`errors.As` instead of matching string prefixes;
recognition is supported through the public sentinel and typed carrier.
Existing XSD-only `Validate` still encodes and reparses the
model. It is not redirected to this engine and does not become prose validation.
Existing root signatures, `DecodeOptions` fields, permissive decoding, generated
models and transport behavior are unchanged.

## Requirement and executable-test bindings

The authoritative [implementation requirements and reciprocal test contracts](plan/implementation-plan.json)
retain all 95 IDs, stages, statuses and acceptance axes. The prefix `REQ-DEF-`
applies to the shortened requirement suffixes below; their planned counterparts
are `TEST-DEF-` with the same suffix. These rows bind implemented slices, not
whole-record completion or executed results for future adapters.

| Requirements | Foundation evidence |
| --- | --- |
| `CAT-METADATA`, `CAT-ROLES`, `CAT-MATRIX`, `DEC-CATALOG` | [Catalog compilation](../../validation/catalog_compilation_test.go): full-revision closure, exhaustive support, metadata/provenance, canonical digest and copied accessors. [Inventory tests](../../validation/inventory_test.go): all-record projection, pins, relationships and reproducibility. |
| `CAT-OPERATORS`, `CAT-OVERLAP`, `OPS-GRAMMAR`, `OPS-SCALARS` | [Typed-plan tests](../../validation/plans_test.go), [session tests](../../validation/session_test.go), [schedule tests](../../validation/catalog_schedule_test.go): only the eight families above, exact selectors and derived ordering; full XSD grammar/scalar binding remains open. |
| `OPS-IDENTITY`, `CONTEXT-OUTCOMES` | [Context/identity tests](../../validation/context_identity_test.go), [adversarial tests](../../validation/session_adversarial_test.go), [phase regressions](../../validation/session_phase_test.go), [provider-scope regressions](../../validation/session_scope_test.go) and [identity review regressions](../../validation/review_identity_test.go): forward/missing/duplicate targets, causes, copied defaults, conflicts, late document providers, unsupported key families without false absence, and malformed atomic contract failures. |
| `NUM-DOMAIN`, `NUM-EXACT` | [Exact-value tests](../../validation/values_test.go): copies, independent exact comparisons, finite decimals, magnitude/output boundaries; no source lexical or model conversion claim. |
| `RESULT-AXES`, `DIAG-METADATA`, `DIAG-CATEGORIES`, `DIAG-BOUNDS`, shared `NUM-CONVERSION` vocabulary | [Accumulator tests](../../validation/report_accumulator_test.go), [report/session regressions](../../validation/report_session_test.go), [terminal review regressions](../../validation/review_terminal_test.go), [formatting bounds](../../validation/report_bounds_test.go): diagnostic-only storage, independent counts/blockers and dependency causes, terminal categories/precedence, offline metadata and output limits. |
| `DIAG-COMPAT`, `DEC-API` | [Root bridge](../../validation_foundation_bridge_test.go), [error tests](../../validation/errors_test.go), [external example](../../validation/example_test.go): public alias, pure/joined outcomes and usable core API. |
| `BUDGET-ACCOUNT`, `BUDGET-INPUT`, `BUDGET-UNPACKED`, `BUDGET-BUFFER`, `BUDGET-DEPTH`, `BUDGET-EVENTS`, `BUDGET-ATTRS`, `BUDGET-SCALAR`, `BUDGET-IDS`, `BUDGET-MATCH`, `BUDGET-CONTEXT`, `BUDGET-LINKS`, `BUDGET-DIAGS`, `BUDGET-MODEL`, `SEC-CANCEL` | [All-resource arithmetic tests](../../validation/budgets_test.go), [runtime review regressions](../../validation/review_runtime_test.go), session/context/identity/adversarial tests: defaults, modes, shared quotas, host-width guards, compact success retention, linear repeated-document-prerequisite work, terminal cleanup and cause retention. Reader/loader/walker integrations are future obligations. |
| `SEC-NOFETCH`, `VERIFY-CI`, `VERIFY-DIFFERENTIAL`, `VERIFY-PROFILE` | [Import/verification boundary](../../validation/boundary_test.go), [independent contracts](../../validation/independent_contracts_test.go), [independent review oracles](../../validation/independent_review_test.go), [independent terminal edges](../../validation/independent_terminal_test.go), [report fuzzing](../../validation/report_bounds_test.go), [session fuzzing](../../validation/session_fuzz_test.go), existing prerequisite/oracle suites: bounded core proof and regression gates, not profile readiness. |

## All-requirement ownership summary and remaining work

The following grouping accounts for all 95 requirements without duplicating their
full acceptance text. Batch numbers are implementation ownership, not replacements
for the original thirteen deferred stages or seven semantic stages.

| Original stage | Records | Remaining ownership |
| --- | ---: | --- |
| `STAGE-DEF-01` | 3 | Batch 4: source XML/namespace closure and original-byte integration; retain earlier mandatory XML repair evidence. |
| `STAGE-DEF-02` | 6 | Batches 4/5: shared facts for attributes, actual types/nil/defaults and hints; batch 8 discovery pairing; batch 9 unresolved identity/default participation. |
| `STAGE-DEF-03` | 3 | Batch 4 source conversion/draining, batch 5 current-model value evidence, batch 6 contextual arithmetic; exact-value/account foundation is bounded evidence only. |
| `STAGE-DEF-04` | 8 | Catalog foundation above; full shared XSD grammar/scalar/binder/operator migration and per-instance capability/test bindings remain with source/model integration and external closure. |
| `STAGE-DEF-05` | 2 | Batch 4: actual opt-in single-parser source assessment and strict conversion. |
| `STAGE-DEF-06` | 3 | Batch 5: direct model walker, field/declaration capabilities and representation invariants. |
| `STAGE-DEF-07` | 4 | Batch 5: bounded local relations, typed references and first explicit profile scope/candidate decisions. |
| `STAGE-DEF-08` | 2 | Generic outcomes above; batch 6 musical timeline and batch 7 real context/default/repeat/span providers. |
| `STAGE-DEF-09` | 5 | Batch 8 package/container/sounds adapters and resolver; batch 9 SMuFL, IPA and external specifications. |
| `STAGE-DEF-10` | 21 | Shared reports/errors/accounts above; batches 4–8 must prove actual adapter, provider, loader, cancellation and resource ownership. |
| `STAGE-DEF-11` | 3 | Every batch verifies its scope and prior regressions; batch 9 reconciles final whole-profile/external coverage. |
| `STAGE-DEF-12` | 32 | Core API/catalog decisions here; batch 5 profile/local issues, batches 6/7 timeline/context issues, batch 8 package issues, batch 9 external issues and remaining original-field translations. |
| `STAGE-DEF-13` | 3 | Optional opus replacement, quotations and edit-invalidated snapshots require separate election; retained as optional. |

In particular, the prior PR 30/31 XSD grammar, scalar, dynamic binding and identity
repairs remain in the root validator. No full XSD runtime operator family has
been migrated into this fact engine. Batch 4 source binding and batch 5 model
binding must reuse/refactor those helpers rather than copy a second validator.
Catalog inspection keeps their runtime plans unbound. General schema validity/UPA,
opaque anyType and wildcard closure remain unavailable; deliberate
`processContents=skip` must stay distinct from unimplemented lax/strict behavior.

Batch 9 retains absent optional default/fixed attribute identity participation,
arbitrary constructed list/union identity closure, DTD ENTITY declaration facts,
unverified primitive equality and external normative closure. Batch 8 retains
package/resolver behavior and schema-hint namespace/location pairing; fixed-schema
hint datatype assessment never fetches or selects schemas. Preserve documented
oracle divergences and their independent normative expectations.

The [28 issue dispositions](open-questions.md) and [deferred inventory](deferred-inventory.md)
remain gates, not hard-error predicates. No guessed initial context, musical
position, universal span balance or broad `ProductionMusicXML` profile is added.
`InspectXML`, `DecodeAssessed`, direct `Assess` and package-assessment entry points
are intentionally absent until their adapters can satisfy their contracts.

Required verification remains `make check-all`, the existing required Linux
external-XSD roots and planning/generation reproducibility. The Linux CI test job
executes `GOARCH=386 go test -count=1 ./validation` for actual host-width coverage;
a cross-build alone does not establish 32-bit runtime coverage. `check-all` runs
`FuzzReportFieldIsBoundedEscapedUTF8` and `FuzzSessionLifecycle` for 10,000 counted
executions each by default, configurable with `VALIDATION_FUZZ_TIME`, as documented
in [local checks](../../CONTRIBUTING.md#local-checks). Test links describe checked
contracts; they do not assert that remote CI or all whole-change checks have passed.
