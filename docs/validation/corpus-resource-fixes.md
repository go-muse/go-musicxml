# Corpus-derived assessment resource corrections

## Dated acceptance receipt

The initial PR 43 tree `cf17ec116c264d0b2c19d1c040d8ade0a8f3b756`, published
at `c0d73972ffb3ba729f7628e85e7ce3dc80dab7ce`, passed all seven local stages
on 2026-10-16, 19:23:49–19:50:48 UTC: module consistency, generation,
formatting, uncached tests, vet, full uncached race tests, and all 11 fuzz
targets at the repository's default execution-count budgets. The toolchain was
Go 1.27.2 linux/amd64 with required real xmllint 2.9.14. The exact-head
[hosted CI run](https://github.com/go-muse/go-musicxml/actions/runs/38958016526)
completed at 19:57:51 UTC with all 11 checks successful, including actual
386 source/validation test execution. These are dated results for that tree;
subsequent review corrections require their own exact-head acceptance receipt.

The corrected full-corpus rerun and additional memory campaign were not run.
Neither contributes acceptance evidence to this PR.

## Scope and baseline

The baseline is published main `d336f08975e586ceba3eb9baffa7542298ba6100`.
The completed baseline campaign covered 3,291 distinct MusicXML 4.0 XML inputs
and 1,782 physical MXL packages. Every ordinary two-cycle decode/encode roundtrip
completed. The 247 normalized-tree differences were classified as formatting or
invalid-input effects; no valid-input serialization loss was established.

The corpus counts and large-score observations are unpublished external
observations, not a portable corpus benchmark supplied by this repository.
No corpus-licensed file is added by this PR; retained regression fixtures are
generated examples. Exact population-wide reproducibility is not claimed.

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
   session-canonical immutable declaration evidence. Different tables and
   sessions cannot alias. Repeated nodes of the same exact declaration reuse
   one entry; unknown or causal declarations are not canonicalized. The initial
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

Container growth introduces a caller-visible accounting change: map/vector
copying now consumes one step per copied entry plus one growth step. Previously
this real storage-management work was not charged to `MaxRuleSteps`. Thus a
previously just-sufficient explicit limit can fail on the same input, even when
no additional downstream assessment is reached. The public regression pins two
such baseline limits (953,719 model-XSD steps and 556,569 source-XSD steps);
these controls complete under the unchanged default work allowance. The
reviewer's XSD/local controls observed about 0.6–1.1% added steps. Tuned limits
need revalidation and headroom. Separately, each compiled type/use identity adds
one compile record and 64 logical bytes; unchanged fixed compile/catalog
ceilings can consequently reject near-limit custom schemas. The temporary
attribute-name map now leases a modeled 512 + 128 × entry-count bytes,
including zero-entry calls, and releases the lease at function return. This
replaces published `d336f089`'s cumulative 96-byte-per-attribute retention
until session cleanup. The initial PR 43 checkpoint `c0d73972` had already
changed that charge to a transient 96 × entry-count lease. Only relative to
that intermediate checkpoint, the corrected lease adds 512 + 32 × entry-count
logical bytes and can refuse a tuned transient limit sooner. Against the
published baseline, the tradeoff instead depends on its accumulated retention;
the current lease does not add permanent storage. Escaping-map controls compare sampled mean cumulative allocation with the
envelope and log individual maxima separately. Process-wide `TotalAlloc` can
include unrelated allocation and dead replaced hash tables, so neither mean
nor maximum is an exact per-map live peak. Fixed/group and slot-size guards
complement those regressions; this is not a universal physical-memory proof.

The changed retained-storage and container-growth paths admit their modeled
logical storage before allocation; the envelopes are not exact Go allocator or
RSS measurements. Vector/map growth accounts
for simultaneous old and new storage, copying work, cancellation and cleanup.
Unknown, invalid, causal and noncanonical evidence is not promoted or collapsed.

## Established focused evidence

The following focused observations supported that initial acceptance:

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
  same later checkpoint relative to the initial candidate. That initial checkpoint cache kept
  6,370 exact binding/table pairs for two distinct tables, reusing repeated
  readers without merging the tables. Full report parity was preserved. A later
  review correction changes the key from per-node run bindings to canonical
  declaration evidence: the generated 33-node/two-table regression retains
  four entries (1,280 logical bytes), instead of 66 entries (17,152 bytes).
  The earlier 6,370-pair population is historical, not the current key shape.
- Exact/one-short storage, cancellation, prior-failure preservation, independent
  sessions, snapshot lifetime and focused race tests cover the changed ownership
  paths. Local 386 cross-compilation succeeds; this executor cannot execute 386
  binaries. Actual 386 execution subsequently passed in the dated hosted receipt above.

The ordinary report comparison concerns successful completed scope. Interruption
checkpoints can change as real work changes. In particular, the witness count
pass may stop before producing an incremental witness prefix that the old
builder produced. The API does not guarantee a minimum interrupted prefix;
already established issues and explicit incompleteness must remain preserved.

## Current graph capacity witnesses

On Go 1.27.2 linux/amd64, the unchanged single-note-per-measure source-origin
fixture now completes at 576 measures (77,227 source bytes) and refuses snapshot
admission at 577 measures (77,361 bytes): 42,053,858 live bytes plus a
25,064,439-byte request. Historical 531/532 and initial PR 43 568/569 controls
remain and now all complete. The initial PR 43 checkpoint `c0d73972` refused
569 at 42,495,274 live bytes plus 24,717,911 requested bytes; declaration-key
role reuse reduces that fixture's live charge to 41,477,418 bytes.

Explicit exact/one-short controls pin both the 569-measure snapshot at
66,195,329 bytes and the 577-measure snapshot at 67,118,297 bytes. These are
fixture-specific overlap-admission tests, not changed defaults or general limit
recommendations. Successful controls check every expected source-origin row.

The separate 86,052-byte fragment fixture still refuses its 34,741,160-byte
snapshot request; live storage is now 54,486,614 bytes rather than the initial
PR 43 54,998,870 or historical PR 41 57,607,574 bytes. These logical charges
are not heap or RSS measurements. The changed boundaries reflect less retained
role-cache duplication, not relaxed checks or a raised default budget.

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
The current witness pins buffered-byte refusal and its incomplete outcome,
so a return to the earlier repeated-work limit is detected.

A compact-attribute follow-on was evaluated but is not implemented here. Its
measured estimate would remove about 4.48 MB of native attribute structures for
Litaniet and 0.70 MB for Bizet, at the cost of another representation refactor.
Even the proposed Litaniet full-input fact accounting was at least 65,725,054
bytes before success maps and XSD state; the schema-node map alone would add
11,224,832 bytes. This is not a reason to claim universal capacity or to change
limits silently. The current PR stops at the demonstrated corrections above.

Cause-free unavailable evidence outside the explicitly canonicalized states
still has independent records (including `dtd-infoset-unavailable` and
`typed-scalar-unavailable`). General state/reason interning is not implemented;
this remaining duplication is not claimed eliminated.

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

## Residual limitations and open decisions

Interpret future capacity and equivalence evidence by distinguishing:

- default-budget completed assessments from incomplete resource-limited cases;
- fully compared reports from improved prefixes and truncated baseline evidence;
- invalid corpus inputs from library defects;
- real memory measurements from accounting limits and cumulative allocations.

The current 64 MiB policy is preserved, not justified here as a universal
ordinary-score capacity requirement. Its product rationale and configuration
policy remain a separate decision. Remaining retained output/fact duplication
and the evaluated attribute design are explicit follow-on evidence, not hidden
completion claims.

An automatic mandatory loading chain remains an unresolved public-contract gap:
XML well-formedness, checks of source facts lost during conversion, then complete
current-model MusicXML validation before successful return. Current default
loaders do not provide that chain. Implementing it requires a specific
loss-sensitive source contract and whole-entrypoint acceptance across XML,
MXL/package and opus paths; calling legacy XSD-only `Validate` or selecting a
broad source profile wholesale would not implement it. No such public-contract
migration is included in this resource PR.
