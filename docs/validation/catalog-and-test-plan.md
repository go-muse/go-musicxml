# Validation catalog and test-coverage plan

Status: **PLANNING CONTRACTS FOR REVIEW**. This change formalizes traceability and
future test obligations; it does not implement the proposed validator, enable a
profile, freeze a public Go API, or fix the documented decoder defects.

This is the next planning layer after [Go contracts](go-contracts.md) and
[worked traces](go-contract-examples.md). The package baseline is
`ba32b92300a54485f000105615fbe4af10325b8e`. The research remains in the external
[registry at `3e33e4c`][registry]. The 16 MB research corpus is not copied into the
library or downloaded at runtime. Only planning dispositions, parameter bindings,
contract matrices, and pinned references live here.

## Read the artifacts

| Artifact | What it establishes |
| --- | --- |
| [external-snapshot.json](plan/external-snapshot.json) | Immutable source pin, verified byte hashes, exact ID sets, candidate inventory and update policy. |
| [research-map.json](plan/research-map.json) | One explicit disposition for every one of the 2,560 research records. |
| [xsd-contracts.json](plan/xsd-contracts.json) | Parameterized XSD instances, generic check contracts, exact schema/language reference edges and source/current-model evidence policies. |
| [xsd-test-contracts.json](plan/xsd-test-contracts.json) | Five-axis parameterized tests for every XSD instance. |
| [prose-contracts.json](plan/prose-contracts.json) | Detailed clauses, selectors, evidence requirements, outcomes, tests and overlap decisions for all 51 constraint-classified prose records. |
| [context-dispositions.json](plan/context-dispositions.json) | Explicit roles, fact/advisory meanings, readiness gates, tests and overlap handling for the other 291 prose records. Compound records can map to more than one role/check. |
| [advisory-clause-review.json](plan/advisory-clause-review.json) | Review of all 59 advisories, retaining compound defaults, context facts and reused XSD clauses. |
| [capabilities.json](plan/capabilities.json) | Named evidence dependencies with reciprocal links and explicit unbound-provider gates. A name does not imply implementation. |
| [deferred-inventory.md](deferred-inventory.md) | Published English source for all 15 deferred sections and 52 stable DEF keys. |
| [implementation-plan.json](plan/implementation-plan.json) | Every deferred obligation, its test contract, implementation group and acceptance criterion, including technical requirements outside the MusicXML record inventory. |
| [open-questions.json](plan/open-questions.json) | All 28 research questions, proposed interpretations, affected context and explicit decision gates. |

All JSON uses `format: musicxml-validation-plan-1`. Check/instance IDs are stable
planning identities at revision 1. Their names are **not frozen public runtime
error codes** or a new DSL. A later compiler/API decision must specify runtime
identity and migration explicitly, while preserving these provenance links.

The existing-reader XML repairs and their executable tests are recorded in the
[code-map update](existing-code-map.md#xml-reading-repair-update), including the
[namespace-declaration provenance repair](existing-code-map.md#namespace-declaration-provenance-repair-update).
The bounded [schema-location hint name repair](existing-code-map.md#schema-location-hint-name-repair-update)
also records its remaining lexical/list/pair reconciliation explicitly.
A checked,
machine-readable implementation-evidence field is an explicit
[follow-up](existing-code-map.md#xml-reading-follow-ups); the planning JSON and
its `status: planned` contract remain unchanged by those repairs.

### Resolve one research record

1. Find its exact `research_id` in `research-map.json`.
2. Follow every `check_ids` link. An XSD instance names one `template_id`; its
   `registry_pointer` selects the exact record in the pinned external catalog.
3. Compose the template with that record's **entire** `conditions`, `parameters`,
   source locations, and semantic references. The pointer is a parameter binding,
   not a summary that drops inconvenient facets, branches, defaults or limits.
4. Follow `plan_dependencies` and `reuse_instances`, or the prose check's typed
   prerequisite description and `capability_ids`.
5. Follow `test_contracts` to each axis and its explicit `axis_status`, then `stage`
   to acceptance. A populated generic axis is still an uninstantiated test obligation.

For XSD instances, source `source_url`, `source_xpath`, assembly, exact
`scope_occurrence_id`, and reference-use identity are inherited from that exact
record. Imports, anonymous declarations and reference sites must retain those
identities. The six schema modules participate in four root assemblies:
MusicXML, opus, container and sounds. `xml.xsd` and `xlink.xsd` are imported
modules, not extra document-root assemblies.

## Roles, dependencies and readiness are different dimensions

The four planning roles are `predicate`, `context_fact`, `advisory`, and
`out_of_scope`. The record-level role is its dominant disposition, not a claim
that a compound record contains only one clause. Follow all check links to see
additional context/default clauses. In particular, advice about repeat endpoints
must not discard the same record's explicit defaults or stop semantics.

`inputs` names evidence payloads, while `capabilities` names the abilities
needed to supply or assess them. Their labels often coincide in this planning
snapshot; both fields are retained so later typed contracts can specialize them
without conflating data with availability. The checker separately keeps the
capability label list equal to its linked gate labels.

External dictionaries, authorized linked files, author intent, musical time and
current-model evidence are dependencies. They are not extra roles. Renderer or
exporter applicability in research does not automatically exclude a semantic
fact or authorize a hard error.

The 291 English role/fact summaries are not a completed translation of their
research fields. The original Russian descriptions, IDs and provenance remain
at the external pin; their complete English-field translation is still tracked
by `REQ-DEF-LANG-291`. Context fact shapes, per-field adapter bindings and runtime
selectors that are not finalized have explicit readiness gates. The eight
additional predicate candidates found among interpretation records have separate
clause/test contracts; their classification does not itself approve rejection.

Every capability resolves to an evidence gate. `provider_status: unbound` means
that the typed provider, scope, ownership and target availability still require
implementation/design acceptance. Provider binding is an implementation-readiness axis, independent of semantic
interpretation. `semantic_status: interpretation_pending` names a real unresolved
clause decision; a specified planning contract can have an unimplemented provider
without becoming semantically undecided. These gates prevent a string such as
`musical_time_index` from masquerading as an implemented context service.

The reviewed snapshot has 418 check contracts: 281 specified/planned, 76 with
explicit clause decisions still open, 59 retained advisories and 2 exclusions.
There are 513 test contracts and 95 deferred implementation requirements. The
76 gates are clause-level units (including external normative closure and
conditional semantics), not a replacement count for the 28 research questions.
All 313 named evidence dependencies have deferred implementation bindings.
The distinction is deliberate: a specified contract can be ready for bounded
implementation while its adapter/provider code has not been written.

A test contract is a test obligation. `status: planned` does not mean the test
exists, ran, or passed. Source fixtures and current-model fixtures remain separate.
No product validation coverage percentage can be inferred from these counts.

## Composition and deduplication

### XSD

The 2,218 XSD records are not 2,218 independent rejection rules. The 155
`simple_value_domain` aggregate views reuse the exact linked restriction/union
origin. The 242 `complex_content_contract` views reuse child, attribute, scalar,
base and particle clauses **and keep residual closedness and text-category
conditions**. Removing those residual conditions would lose constraints.

`plan_dependencies.schema_component` lists references from structured parameter
IDs resolved through the pinned occurrence-to-requirement map.
`plan_dependencies.language_semantics` lists the exact semantic rule references.
These are schema compilation edges. Recursive element/type graphs are valid;
they are not mistaken for a runtime evaluation DAG. Generic operation
`dependencies` and their target-specific modes describe evidence prerequisites.

Group definitions and their use sites can share compiled operators while retaining
different occurrence bounds and locations. Operator-family reuse is not semantic
equivalence. A future canonical diagnostic key needs the obligation/clause,
revision, document occurrence and subject; sharing code cannot collapse different
subjects or erase extra source conditions.

### Prose/XSD and prose/prose

Every prose record retains its overlap relation. Context added-condition text is
bound to that record's own disposition summary or an explicitly associated
`research_clauses` entry on its check. Reviewed shared-text groups are narrowly
allowlisted in `overlap-policy.json`; duplicate-text checks supplement source
association and do not prove semantic equivalence.

A linked XSD ID is provenance,
not proof that the whole record is equivalent. Reviewed constraint records name
the common clause and any additional condition separately. Other relations remain
`review_required` where that proof is not available. An unreviewed relation is
neither dropped nor compiled into two independent defects.

Shared fermata defaults and additive transposition facts use actual shared
contracts. The latter retains the additional all-staves scope condition rather
than treating the entire source records as interchangeable.

## Test-contract interpretation

Every check or engineering requirement has a reciprocal test-contract link
listing obligations on five axes. A listed axis is not necessarily a
record-specific case:

- **positive**: the condition/fact holds for each advertised target
- **negative**: an isolated violation or incorrect provider result is detected
- **boundary**: exact endpoints, empty/absent/zero distinctions, count and scope limits
- **missing_fact**: unknown, unsupported or invalid evidence is preserved honestly
- **interaction**: meaningful inheritance, ordering, scope, dependency or compatibility cases

Each `axis_status` distinguishes `record_specific`, `parameterized`, and
`generic`. There are 724 record-specific axis descriptions, 300 parameterized
XSD axes, and 1,541 generic axes in this snapshot (2,565 axes across 513
contracts). All 279 synopsis/template-derived context contracts label every
axis generic, even where appending the record's summary makes the text unique.
Repeated individual case recipes elsewhere are likewise generic, even when
other cases make their enclosing axis array unique. The checker detects shared
case strings across contracts and prevents a mixed shared axis from being
labeled record-specific. Neither text uniqueness
nor a nonempty array establishes concrete case coverage. These are future test
contracts, and even record-specific descriptions are not executed fixtures.

Every generic or parameterized axis has a `case_instantiation_gate`. Its owner
must supply exact subject inputs, source/model evidence and expected outcomes
before implementation acceptance. The generic slots remain visible work rather
than being counted as completed record-level coverage.

Parameterized XSD matrices must be instantiated for each actual enumeration,
facet, union member, branch and cardinality. A representative family test cannot
close every instance. If an axis is genuinely irrelevant to a particular
instance, its implementation review must record the reason rather than silently
omit it. For pending context contracts, record-specific fixture selection is
itself an acceptance gate. Advisory and exclusion tests verify reporting/scope
behavior, not additional MusicXML rejection predicates.

Normative source claims, current-model invariants and technical implementation
requirements have different oracles. `normative_provenance` on an engineering
requirement can point to the agreed architecture or existing-code map; that does
not turn a project API/resource decision into a MusicXML normative clause.

### Required outcome distinctions

The structured outcome vocabulary follows the proposed Go contracts: `fail` and
`not-applicable`. Natural-language “violation” describes a failed normative
condition; it is not an additional outcome enum. `catalog` is a planning-only
target for metadata/design requirements and has no runtime `Target` equivalent;
the runtime targets remain `source`, `model`, and `package`.

- A known false applicability antecedent can yield `not-applicable`.
- Missing instance data yields `unknown`; an absent capability yields
  `unsupported`; an invalid prerequisite yields `blocked` with a cause.
- No unavailable fact is treated as a successful predicate or a proven absence.
- A known violation can coexist with incomplete assessment. Independent checks
  may still report known failures while another clause lacks evidence.
- Current model values can satisfy value-space checks without historical source
  lexemes. Do not make all model bounds depend on recovering original spelling.
- A duplicate ID may fail on the second known declaration. Complete document scope
  is needed to prove uniqueness or absence of a dangling reference, not to erase
  a violation already observed.

The normative/XSD status of `<staves>18446744073709551616</staves>` is independent
of its inability to fit the current `uint64` field. Budget exhaustion and model
conversion failure are not invented XSD range facets. Written pitch, repeated
music equivalence and external part-file association likewise require their real
evidence; they cannot be inferred from convenient XML spelling alone.

## Bounded implementation sequence

The thirteen `STAGE-DEF-*` groups provide the sequencing plan. Semantic
`STAGE-xsd/local/context/external/package/review/advisory` labels attach contracts
to those groups; they are not additional promised PRs. `STAGE-review` is now
reserved for decision-gated clauses (50 checks); the 177 specified context
contracts previously parked there have moved to `STAGE-context`. Semantic domain
stages can still contain a decision-gated check, whose explicit closure must be
resolved before implementing it. Stage means ownership/sequence, while `status`
and `implementation_binding` separately state semantic and implementation
readiness. A nonempty closure or pending translation alone does not place a
specified contract in the review stage.

`STAGE-context` deliberately remains a coarse **batch-acceptance** group.
Its dependency on `STAGE-DEF-08` closes the complete context-provider batch;
it does not mean that every member requires musical time. Per-element defaults
such as notehead fill or justify alignment can be implemented and tested earlier
in a bounded local slice. Their actual evaluation prerequisites remain their
own evidence capabilities and clause dependencies. The stage record states this
distinction explicitly; no musical-time input is added to a local default.

1. Prepare narrow regression tests for mandatory XML duplicate-attribute/DOCTYPE
   checks, simple/builtin attributes plus all four xsi contracts, and exact numeric
   domains versus conversion. These three groups can progress independently.
2. Accept shared operator, catalog and diagnostic/security contracts. Review
   reusable matchers, facets, value spaces, defaults and scoped identity indexes.
3. Add a single-pass opt-in source adapter and a direct current-model adapter,
   with field-level capability maps and no Encode/reparse path.
4. Review all 29 first-profile candidates, including the existing
   `bend-release-negative`, then implement bounded local relations and references.
5. Add only the selected musical-time/effective-state and package/external
   services whose clauses, evidence and tests are ready. Never invent initial
   key, time, clef or divisions defaults.
6. Establish profile readiness through its mandatory obligations and incomplete
   outcomes, raw-source differential checks and the standard cross-platform CI.

Diagnostics, cancellation, no-automatic-fetch behavior and resource limits are
entry gates for each implementation slice, not optional cleanup at the end.
All budgets in the [published deferred inventory](deferred-inventory.md) have
separate requirements. Every `source_bullets` key resolves to the correspondingly
named heading there (for example, `DEF01-01` → `#def01-01`); each `source_section`
is a real relative section link. The snapshot pins the inventory's byte hash,
and offline integrity verifies the document, all 52 headings, all 15 sections,
and requirement/source links. These keys are editorial traceability labels for
the published task specification, not new MusicXML normative IDs. The open API,
compiled-catalog delivery, error compatibility and profile decisions form a
parallel decision lane. Issues block only affected disputed clauses. Optional
opus byte replacement, quotations and edit-invalidated evidence snapshots remain
separate decisions and are not silently promoted to required work.

## Automated integrity and reproducibility

Run the offline checker and its corruption tests with the standard Go toolchain:

```sh
go run ./internal/validationplan
go test ./internal/validationplan
```

For pin verification, materialize the external repository separately and supply
its root. The command does not clone or fetch it:

```sh
go run ./internal/validationplan -registry /absolute/path/to/go-musicxml-registry
MUSICXML_PLAN_REGISTRY=/absolute/path/to/go-musicxml-registry go test ./internal/validationplan
```

`MUSICXML_PLAN_REGISTRY` enables the test's external snapshot verification; it
must be an **absolute path** because Go executes this package's tests from
`internal/validationplan`. The CLI `-registry` flag instead resolves a relative
path against its invocation directory. Without the environment variable, the
external test is explicitly skipped and the offline integrity tests still run.

Offline integrity checks ID uniqueness, exact inventory identity, both directions
of contract/test/capability links, dependency/stage links and DAGs where required,
axis specificity and instantiation gates, all published deferred source links,
unique overlap rows and own-record clauses, contract/test target subsets,
capability label/ID consistency, aggregate policies,
translation/question sets and honest status fields. The optional external check
verifies file byte hashes, exact record/issue pointers, dependency references and
pinned overlap records. It does not crawl normative websites or prove semantic
correctness. The Go tests participate in existing `go test ./...` CI without a
new Python requirement, module dependency or public API.

Pin updates are deliberate review work: verify new bytes, reconcile every
added/removed/changed record, review role/overlap/capability/test/stage changes,
then update the integrity expectations together. There is no automatic research
synchronization or runtime registry loading.

## Storage and reviewability decision

This PR deliberately keeps the planning snapshot with its offline checker so
one repository contains the reviewable contract graph, published task inventory,
and CI inputs. It therefore increases the library distribution footprint. The
16 MB research corpus remains external; this decision does not make the derived
planning data free to download.

Measured Git ZIP archives: `ba32b923` = 625,015 bytes; initial PR head `a92c653` =
1,053,407 bytes (+68.5%). The revised-tree measurement is recorded in the PR's
review-response section. These are reproducible `git archive --format=zip`
measurements, a distribution-size proxy rather than an exact Go proxy ZIP byte
count. The additional reviewed JSON is the explicit cost of self-contained
offline verification. A future externalization would need its own immutable pin,
availability and update workflow; it is not performed silently in this PR.

GitHub's `linguist-generated` attribute collapses large planning JSON diffs by
default for review navigation. It does not mean the semantic judgments were
machine-proven or that the data is production-generated validation code.

## Task record and limits

**Goal:** make the next implementation work bounded and reviewable by connecting
all research dispositions and deferred obligations to evidence, test contracts
and acceptance, while preserving the architecture's honest outcomes.

**Decisions:** keep research external and pinned; compose parameterized schema
contracts rather than copy their parameters; separate roles from dependencies;
retain clause-level dedup and unresolved evidence gates; add executable checks
only for the planning artifacts.

**Results:** the catalog, matrices, dependency/role traceability and integrity
checker are reviewable together. The PR description records final artifact counts
and verification results for its exact commit.

**Limits:** zero new executable product predicates and zero new product tests;
no production engine, decoder bug fixes, public API, external resolver, dictionary,
full English research translation, or full-standard conformance proof. The
registry ledger proves scoped bookkeeping, not complete transitive normative
closure. Passing this PR's CI validates current code plus planning integrity;
it does not implement the future test contracts.

[registry]: https://github.com/go-muse/go-musicxml-registry/tree/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c
