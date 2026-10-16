# Corpus-derived assessment resource corrections

Status: implementation checkpoint, 2026-10-16. Ordinary whole-PR acceptance is pending
at this checkpoint. The user explicitly canceled the corrected full-corpus rerun
and additional memory campaigns; neither is an acceptance claim for this PR.
The PR's final receipt must identify the tested tree and distinguish local from
hosted checks.

## Scope and baseline

The baseline is published main `d336f08975e586ceba3eb9baffa7542298ba6100`.
The completed baseline campaign covered 3,291 distinct MusicXML 4.0 XML inputs
and 1,782 physical MXL packages. Every ordinary two-cycle decode/encode roundtrip
completed. The 247 normalized-tree differences were classified as formatting or
invalid-input effects; no valid-input serialization loss was established.

The confirmed engineering problem was resource amplification during bounded
assessment. This change removes repeated work and unnecessary retained storage;
it does not change selected requirements, default budgets or public loading
behavior. Ordinary `Decode` remains conversion with mandatory XML parsing, not
a new automatic source-plus-model assessment gate.

## Ordered corrections

1. Share exact canonical model/attribute examination seals within one session.
   Exceptional states, reasons, causes and caller-owned evidence remain distinct.
2. Admit schema DFS scratch for actual live stack/completion capacity rather than
   reserving a frame for every document node. Old/new capacity overlap is charged.
3. Reuse immutable element-role classification during final provider evaluation.
   The final cache is session-owned and keyed by the exact compiled table and
   run-local binding. Different tables and sessions cannot alias. The initial
   reader-local implementation was corrected after matched-progress diagnostics
   exposed repeated maps whose accounting lasted until session cleanup.
4. Charge projected state for the values actually compared, explicitly meter
   key-component traversal, and stop immediately at the first runtime error.
5. Build XSD witness vectors with exact capacity in two metered passes, avoiding
   retained geometric-growth arrays and copies.
6. Release the function-local schema attribute index after publication; published
   scalar/declaration evidence retains its independent ownership.
7. Compact private node/header/end storage using immutable, session-owned
   evidence. Public values and independently owned exported snapshots remain
   unchanged. Exact compiled owner/ordinal tokens avoid repeated path hashing;
   they are assembly-local identities, not cross-schema semantic equality.

Every allocation remains admitted before it occurs. Vector/map growth accounts
for simultaneous old and new storage, copying work, cancellation and cleanup.
Unknown, invalid, causal and noncanonical evidence is not promoted or collapsed.

## Established focused evidence

These results precede final whole-tree acceptance:

- Twenty-two full source/model report controls, including dictionary contexts,
  invalid inputs, partial input and source positions, were byte-identical before
  and after node compaction: 13,649,339 bytes per matrix, SHA256
  `2d5567116f3d45313ff613bca992aface29797aebcbe955646e33e970cc3231c`.
- A separate official-source attribution comparison used the same explicit
  diagnostic limits on both revisions: 200M rule steps and 128 MiB buffered
  bytes. Both complete reports were byte-identical, 51,363,499 bytes, SHA256
  `c64631fb7178cf32d25f7f14206b348ed9323078c34e7990e4238efafe84e8a9`.
  These raised-limit observations establish attribution/equivalence only; they
  are not default-budget acceptance and do not change production defaults.
- At the same layout-entry checkpoint, both revisions had complete input,
  3,185 nodes, 1,341 attributes and 4,913 published results with matching
  provider/property counts. The compact candidate retained 2,899,128 fewer
  logical bytes. The old default run exhausted 20M steps before layout
  publication; the candidate advanced farther and then exhausted buffered bytes.
- Correcting role-cache lifetime removed another 5,601,536 logical bytes at the
  same later checkpoint relative to the initial candidate. The final cache kept
  6,370 exact binding/table pairs for two distinct tables, reusing repeated
  readers without merging the tables. Full report parity was preserved.
- Exact/one-short storage, cancellation, prior-failure preservation, independent
  sessions, snapshot lifetime and focused race tests cover the changed ownership
  paths. Local 386 cross-compilation succeeds; this executor cannot execute 386
  binaries. Actual 386 execution remains a hosted-CI requirement.

The ordinary report comparison concerns successful completed scope. Interruption
checkpoints can change as real work changes. In particular, the witness count
pass may stop before producing an incremental witness prefix that the old
builder produced. The API does not guarantee a minimum interrupted prefix;
already established issues and explicit incompleteness must remain preserved.

## Residual limits and interpretation

The unchanged-default large-model controls do not establish a usable validated
model. Litaniet progressed from 46,109 to 73,570 charged node events and from
15,896 to 23,027 attributes after node compaction, but remained incomplete under
the 64 MiB buffered limit. Bizet reached all 36,286 node events and 4,266
attributes on both compared revisions, then still failed later assessment
buffer admission. Cleanup returned the live buffered gauge to zero.

These are improved prefixes, not completed validation. Cumulative allocation
counts, sampled heap, live logical accounting and RSS are different quantities;
none may be substituted for another or compared across different processed
prefixes as a complete-assessment saving.

The official source witness also remains explicitly incomplete at unchanged
defaults: complete input does not imply complete assessment. Its later buffered
refusal replaces an earlier work refusal because more required work is reached.
The witness must assert the typed refusal and incomplete outcome, rather than
require one historical refusal mechanism forever.

A compact-attribute follow-on was evaluated but is not implemented here. Its
measured estimate would remove about 4.48 MB of native attribute structures for
Litaniet and 0.70 MB for Bizet, at the cost of another representation refactor.
Even the proposed Litaniet full-input fact accounting was at least 65,725,054
bytes before success maps and XSD state; the schema-node map alone would add
11,224,832 bytes. This is not a reason to claim universal capacity or to change
limits silently. The current PR stops at the demonstrated corrections above.

Dictionary import decoding, hashing, sorting and indexing remain account-owned
work. No process-wide cache or cross-account certificate is introduced. Full
report/context payloads still consume real storage; required diagnostics are
not dropped to manufacture success. Unread external DTD evidence remains
unavailable where required, with no automatic fetch or invented completeness.

The original corpus receipts store context counts, not complete context bodies.
Consequently full context equivalence is supported by the targeted report
comparisons above, not inferred from the mass baseline. Truncated findings also
cannot prove the absence of a later finding. A newly reached finding after an
old resource stop is newly observed evidence, not automatically a regression.

## Handoff boundaries

The next developer must distinguish:

- default-budget completed assessments from incomplete resource-limited cases;
- fully compared reports from improved prefixes and truncated baseline evidence;
- invalid corpus inputs from library defects;
- real memory measurements from accounting limits and cumulative allocations.

The current 64 MiB policy is preserved, not justified here as a universal
ordinary-score capacity requirement. Its product rationale and configuration
policy remain a separate decision. Remaining retained output/fact duplication
and the evaluated attribute design are explicit follow-on evidence, not hidden
completion claims.

The user's expected mandatory loading chain also remains a separate handoff:
XML well-formedness, checks of source facts lost during conversion, then complete
current-model MusicXML validation before successful return. Current default
loaders do not provide that chain. Implementing it requires a specific
loss-sensitive source contract and whole-entrypoint acceptance across XML,
MXL/package and opus paths; calling legacy XSD-only `Validate` or selecting a
broad source profile wholesale would not implement it. No such public-contract
migration is included in this resource PR.
