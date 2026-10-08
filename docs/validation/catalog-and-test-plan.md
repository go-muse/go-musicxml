# Validation catalog and test-coverage plan

Status: **PLANNING CONTRACTS FOR REVIEW**. This change formalizes traceability and
future test obligations; it does not implement the proposed validator, enable a
profile, freeze a public Go API, or fix the documented decoder defects.

This is the next planning layer after [Go contracts](go-contracts.md) and
[worked traces](go-contract-examples.md). The package baseline is
`ba32b92300a54485f000105615fbe4af10325b8e`. The research remains in the external
[registry at `3e33e4c`][registry]. The 16 MB research corpus is not copied into the
library or downloaded at runtime. Only compact dispositions, parameter bindings,
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
| [implementation-plan.json](plan/implementation-plan.json) | Every deferred obligation, its test contract, implementation group and acceptance criterion, including technical requirements outside the MusicXML record inventory. |
| [open-questions.json](plan/open-questions.json) | All 28 research questions, proposed interpretations, affected context and explicit decision gates. |

All JSON uses `format: musicxml-validation-plan-1`. Check/instance IDs are stable
planning identities at revision 1. Their names are **not frozen public runtime
error codes** or a new DSL. A later compiler/API decision must specify runtime
identity and migration explicitly, while preserving these provenance links.

### Resolve one research record

1. Find its exact `research_id` in `research-map.json`.
2. Follow every `check_ids` link. An XSD instance names one `template_id`; its
   `registry_pointer` selects the exact record in the pinned external catalog.
3. Compose the template with that record's **entire** `conditions`, `parameters`,
   source locations, and semantic references. The pointer is a parameter binding,
   not a summary that drops inconvenient facets, branches, defaults or limits.
4. Follow `plan_dependencies` and `reuse_instances`, or the prose check's typed
   prerequisite description and `capability_ids`.
5. Follow `test_contracts` to the five case axes, then `stage` to acceptance.

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

Every prose record retains its overlap relation. A linked XSD ID is provenance,
not proof that the whole record is equivalent. Reviewed constraint records name
the common clause and any additional condition separately. Other relations remain
`review_required` where that proof is not available. An unreviewed relation is
neither dropped nor compiled into two independent defects.

Shared fermata defaults and additive transposition facts use actual shared
contracts. The latter retains the additional all-staves scope condition rather
than treating the entire source records as interchangeable.

## Test-contract interpretation

Every check or engineering requirement has a reciprocal test-contract link with:

- **positive**: the condition/fact holds for each advertised target
- **negative**: an isolated violation or incorrect provider result is detected
- **boundary**: exact endpoints, empty/absent/zero distinctions, count and scope limits
- **missing_fact**: unknown, unsupported or invalid evidence is preserved honestly
- **interaction**: meaningful inheritance, ordering, scope, dependency or compatibility cases

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
to those groups; they are not additional promised PRs.

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
All budgets in the deferred inventory have separate requirements. The open API,
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
go run ./internal/validationplan -registry /path/to/go-musicxml-registry
```

Offline integrity checks ID uniqueness, exact inventory identity, both directions
of contract/test/capability links, dependency/stage links and DAGs where required,
all five test axes, all deferred bullet/paragraph keys, aggregate policies,
translation/question sets and honest status fields. The optional external check
verifies file byte hashes, exact record/issue pointers, dependency references and
pinned overlap records. It does not crawl normative websites or prove semantic
correctness. The Go tests participate in existing `go test ./...` CI without a
new Python requirement, module dependency or public API.

Pin updates are deliberate review work: verify new bytes, reconcile every
added/removed/changed record, review role/overlap/capability/test/stage changes,
then update the integrity expectations together. There is no automatic research
synchronization or runtime registry loading.

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
