# Package assessment acceptance evidence

This is the bounded batch-8 implementation record, with historical candidate
evidence distinguished from later review measurements. The [scope and ownership
inventory](package-assessment-scope.md) preserves original obligations and later
owners; this document records engineering evidence separately from normative
closure. Full ZIP/JAR/DEFLATE, dictionaries and unresolved external interpretations
remain explicit. B5-PERF-01 is not closed by these measurements.

A later incremental checkpoint was published on 13 October 2026 at 10:41 UTC:
`e8db43747d46f4af42c0796e4fc71159b95e729e` (tree
`ce61d65026931569aef1141398c8f4e1e671c638`). Publication was confirmed by the
10:42:04 UTC PR readback. [CI run 38385055748](https://github.com/go-muse/go-musicxml/actions/runs/38385055748)
was in progress at the separate 10:44 UTC observation.
This dated checkpoint is not final acceptance and does not transfer the archived
candidate's execution results to the published tree.

A subsequent readback confirmed that run 38385055748 completed with failure. The
stale all-assembly discovery test expected Unknown for an observed odd hint list;
[commit 6b2bc2953d40ddcbc79f97f8b919548b4b95e1a6](https://github.com/go-muse/go-musicxml/commit/6b2bc2953d40ddcbc79f97f8b919548b4b95e1a6)
corrected that expectation to the reviewed Known(false) pairing contract and
updated only its current test-file hashes. The 10:44 UTC in-progress observation
above remains historical; this terminal outcome neither rewrites it nor certifies
any later tree. The failed run is not acceptance evidence.

## Test-first development and independent corrections

The original unpublished development commits and task logs are preserved in the
engineering archive. The private `388dbe06aaedda02b7d1f07a9ebe35ee6a5fd75f`
commit/tree and the earlier publication report's `13d635ce` candidate are not
checkout-verifiable published objects. The repository-resolvable PR baseline is
`7b5460af2e2028dc27079353e5f44f2901054d19` (tree
`232598ae2bc28651f8de267770595f5c264470aa`); naming it does not transfer any
historical execution result to that tree. Logical publication milestones group each failing contract
with its working implementation; a missing-API compilation failure is not
presented as an executed semantic failure.

| Task | Original failing observation | Retained regression / result |
| --- | --- | --- |
| Auxiliary assemblies and local link contexts | Source container/sounds profiles and scoped discovery consumers were unavailable | Auxiliary original-byte fixtures, real-xmllint differential controls and source/model link tests |
| Snapshot/package contracts | No owned terminal snapshot, independent resource report, or sealed package coordinator | Snapshot ownership, cross-account rejection, invalid transfer, immutable getters and reserved failure summary tests |
| XML graphs and loaders | No selected graph entry points; cycle, read-cap, joined-EOF and returned-pin regressions exposed incorrect boundaries during development | Exact body ownership, canonical parse reuse, distinct context expansion, URI and no-fetch tests |
| Raw/current/nested packages | New assessment entry points absent; independent probes found cross-package byte quota and incoming-depth resets | Raw/current representation controls, shared five-cap and nested-depth tests |
| Early proof preservation | Later cancellation/refusal erased independently established edge, rootfile, role or advice failures | Stable-ingress proof and interruption-order tests |
| Member roles | A complete selected pin table could overlook another physical member | Exact member contexts, explicit discovery seal, partial/forged/legacy ambiguity and four-entry-point dispatch tests |
| File observations | Failed parsing prevented independent verified filename/MIME advice; observed pins bypassed canonical conflict checks | Metadata-only capabilities, no parsed promotion, same-pin aliases and both-order canonical conflict tests |
| Rejected targets and raw names | Known wrong QName was lost after intrinsic failure; legacy non-UTF8 entry bytes lost advisory evidence | Four-surface partial-target matrix and observation-only raw-name tests |
| Independent XML depth | Explicit MXL XML limits were validated but ignored; cached plain XML could otherwise bypass a receiving package cap | Scalar-inclusive exact/one-under source/current/local/external/nested controls, default 256, shallow long graphs and cached-depth witness checks |
| Late snapshot admission | Successful export followed by OpenPackage/AddResource refusal returned no known invalid child report | Public step-limit reproducer plus controlled live-buffer refusal at primary/metadata/nested/current/external seams and post-export cancellation |

The late-admission tests explicitly distinguish a public API reproducer from
accounted-pressure injection at private transfer seams. Successful transfer is a
negative control: the canonical report stays with the package session and is not
recorded twice. Source and current-model omission are not equated where the model
has no presence bit.

## Independent semantic coverage

The committed `resource_graph_independent_{partial,row,metadata}_test.go` matrices
contain 267 named leaf cases. The target-kind supplement adds 12 cases for Opus
score links to partwise/timewise targets and wrong-kind Opus targets across source,
current-model, raw-MXL and current-MXL entry points. A further 22 independent
XML-depth cases cover exact/refused primary/local/external/nested paths, graph-hop
independence, plain siblings and contextual reuse of an already cached deep source.
These total 301 cases on the linked-target correction tree `321be6e` (published
`10b7a49`), verified from the completed uncached race event stream. The earlier
300-case acceptance record remains historical: a later explicit `<opus/>`
control added the extra partial-target case. Each ordinary CI execution runs
the matrices once; repeated focused independent runs are separate evidence.

The matrices cover optional image ratios and unavailable dimensions, exact URI
bases, attachment positions/causes, prefix semantics, MIME defaults and parameters,
root kinds/namespaces, authoritative role scope, file provenance, associations,
partial child reports and forbidden resolver continuation after intrinsic errors.
They do not substitute source-only lexical evidence for current-model history.

Eight existing profile identities and complete minimal-Opus reports were captured
at canonical main `3d3d936fa4ba250083da03651cbfb4bbbe936914` and compared byte-for-byte
after integration. All eight captures are identical. This is a compatibility
control, not evidence of unrestricted score readiness.

## Finite-default usability and residual limits

Independent measurements use the existing clean 100-note timeline fixture and
the repository's official MozartTrio and MozaChlo samples. Original files are
read-only. Adjusted source controls remove only the single external DOCTYPE;
MozaChlo retains its UTF-16 big-endian BOM and declaration. Current models are
walked directly after decoding, never encoded to reconstruct source history.

At the historically measured unpublished production candidate
`388dbe06aaedda02b7d1f07a9ebe35ee6a5fd75f` (after XML-depth/witness corrections):

| Workload | Default graph outcome | Explicit finite override |
| --- | --- | --- |
| Minimal Opus and clean 100-note source | Complete and satisfied | None |
| Official samples, DOCTYPE-only removed | Complete and satisfied | None |
| Original MozartTrio source | Buffered-byte refusal; known best report retained | 128 MiB buffered bytes removes operation error; unread-DTD incompleteness remains |
| Original MozaChlo source | Buffered-byte refusal; known best report retained | Same 128 MiB buffer-only override; unread-DTD incompleteness remains |
| Current models | Within finite defaults; incomplete for discarded original schema hints | Raising limits cannot reconstruct missing evidence |

Exact logical minimum graph buffer budgets were 72,541,715 bytes for original
MozartTrio and 91,412,475 bytes for original MozaChlo. These exceed the unchanged
64 MiB default. They are admitted logical accounting thresholds, not RSS or heap
peaks. Work was 6,805,850 and 8,547,703 rule steps respectively, below the unchanged
20,000,000-step limit. The explicit successful override changes only
`MaxBufferedBytes` to 134,217,728; all other graph budgets keep their defaults.
Exact/one-byte-under probes and terminal zero-live-gauge checks are retained.

Synthetic stored MXL wrappers preserve the exact same musical bytes and add a
known container/mimetype. Clean and DOCTYPE-removed raw wrappers fit defaults;
original DTD-bearing raw wrappers have the same buffer limitation. Current package
objects fit defaults but retain current-model discovery incompleteness. These
wrappers are not claimed to be an original official MXL corpus.

## Historical 2026-10-13 snapshot/local-link review remeasurement

The earlier candidate figures above remain historical evidence. Fresh measurements
from the published `7b5460af2e2028dc27079353e5f44f2901054d19` baseline reproduced
both graph thresholds exactly. The local-link correction avoids allocating full
attachment indexes for link-free documents, retains a single document-wide DTD
hint gap plus physically observed descendant hints, and removes reflected boxing
from snapshot measurement/copying. The private canonical-report ownership and
independent caller-returned report contract are unchanged.

`TestResourceReviewOfficialExactBufferWitness`, enabled with
`MUSICXML_SNAPSHOT_BUFFER_WITNESS=1`, checks the exact successful threshold and
one byte below for each original source. No source bytes or DOCTYPEs are removed.
These source-only results do not assess nested compressed-rootfile interpretation.

| Original source | Published links minimum | Earlier corrected links minimum | Published graph minimum | Earlier corrected graph minimum |
| --- | ---: | ---: | ---: | ---: |
| MozartTrio | 47,656,776 | 36,961,992 | 72,541,715 | 40,939,940 |
| MozaChloSample | 60,222,816 | 46,681,595 | 91,412,475 | 51,394,485 |

All values are logical buffered bytes, not process heap/RSS. Both earlier corrected
graph runs fit the unchanged 64 MiB default and retain unread-DTD incompleteness.
The ordinary default-budget regression checks both samples without claiming
conformance. A 128 MiB buffer-only override remains a concrete finite option for
other workloads, not a requirement for these corrected source cases.

One-iteration warmed benchmarks with Go 1.27.2, linux/amd64, GOMAXPROCS=2 and
`-p=1` measured graph allocations as follows; timing is not a stable speed claim.

| Original source | Published allocated bytes / objects | Corrected allocated bytes / objects |
| --- | ---: | ---: |
| MozartTrio | 181,427,488 / 3,058,618 | 46,919,440 / 510,442 |
| MozaChloSample | 225,670,904 / 3,836,178 | 55,848,760 / 630,374 |

The small owned-report measurement probe dropped from 168 allocations to zero.
A 4096-byte slice now measures 4128 logical bytes rather than 131104. String copy
work uses bounded 4 KiB blocks; byte slices retain per-byte work admission and
contiguous storage/copying. Cancellation tests cover both block-copy paths and
verify zero temporary retained bytes. This is scoped review evidence, not a new
whole-tree, race, 386 or remote CI claim.

## 2026-10-13 packaged-tree exact-buffer remeasurement

An independent exact/one-byte-under run started at 14:29:34 UTC on the clean local
packaged commit `861aec472c61ae284c25908684ad680dd32d4060`, tree
`2faaa347078bc1be8ee27ec526e75d265bc2ed4a`. That tree is identical to published
`a9a53a135fd9214cc5a60818add27dc6108bf2e4`. The command was:

```bash
MUSICXML_SNAPSHOT_BUFFER_WITNESS=1 go test -count=1 -run '^TestResourceReviewOfficialExactBufferWitness$' -v .
```

It passed with Go 1.27.2 on linux/amd64, `GOMAXPROCS=2` and `GOFLAGS=-p=1`.
The committed [witness record](evidence/package-buffer-witness-20261013.json)
retains the exact revision, tree, command, source hashes, thresholds and log hash.
No original source bytes, DOCTYPEs or encoding were changed.

| Original source | Source-links minimum | Graph minimum |
| --- | ---: | ---: |
| MozartTrio | 36,961,992 | 40,946,084 |
| MozaChloSample | 46,681,595 | 51,400,629 |

Each graph minimum is 6,144 bytes above the earlier corrected value, accounting
for the fixed prepaid package diagnostic summary reserve. Source-links minima
are unchanged. These exact logical admission thresholds remain below the
unchanged 64 MiB default; they are not heap/RSS measurements. Unread-DTD
incompleteness remains and no normative closure follows from fitting the budget.

The earlier correction log and its allocation results above remain historical.
Its review record identifies final scoped commit
`d52560b911852eeb37fb4b8390c1f5ebdc39aa9b`, tree
`371fde64e76b0ff63f7175986ae444b6bba2aaed`; that provenance is not an assertion
that the earlier log was executed on the later packaged tree. The new witness
supersedes the earlier graph thresholds only for the explicitly measured tree.
It does not rerun benchmarks or certify whole-tree stages, race, actual 386,
remote CI or the subsequent archive-to-primary linked-target correction.

## 2026-10-13 linked-target indirection regression

Review thread `4255994212` exposed a false direct-target judgment after the
loader substituted a linked archive's primary XML. The unchanged packaged base
`be3e0f8436f9f53f9650fd86fb5533839ee82608` (tree
`a4afcea40e31e361521d539bcd6a38b437ed34ca`, identical to published `21c22f6`)
failed the new public regression in all 16 raw/current/external cases. Matching
inner roots falsely passed and different inner roots falsely failed the outer
predicate. Six additional context regressions reproduced fragment, association,
primary-path, depth and ancestor-alias cases while direct XML controls passed.

The correction adds required per-edge `Indirection` evidence, preserves inner
reports and keeps outer target-dependent predicates Unknown with
`nested-link-indirection-unresolved`. Core tests cover unavailable/invalid
evidence and owned causes, local prerequisite precedence, immediate and final
association decisions, direct controls and resource admission. Public tests also
retain inner failures, independent inner associations/images, cancellation and
post-import edge-admission cleanup. Independent integration review additionally
identified the missing-provider association reason; a separate public/core RED
regression precedes that correction.

Focused RED log SHA-256 values are
`c8550b6b30c810086c13e3362cb344a9211d9bcf30bc14b61632dbc29e53711e`
(16 public cases),
`c53838adbb7d5b19b9695dcd67074284b0258e9c756562899ae631acf913fee0`
(six context regressions), and
`0b7624cf7f57a1262d1fe458a20661dc476feaf7a0ff85ea217288560b5222c9`
(initial core evidence matrix). These are historical regression artifacts, not
whole-tree acceptance. Final exact-tree local stages and published-head CI remain
separate gates below. Original requirement/status/owner records are unchanged;
the inventory refresher updates only current hashes of changed routed test files.

## Round 3 exact-buffer checkpoint

An independent repeat on the linked-target baseline `7e271f4` (tree `321be6e`)
and corrected production candidate `a99d5ab` (tree `37469d6`, identical to remote
commit `9ea6c67`) found identical thresholds. The additive
[Round 3 witness](evidence/package-buffer-witness-round3-20261013.json) records
the exact commits/trees, original input hashes, command, environment and log
hashes. Every minimum succeeds and one byte below returns `ErrResourceLimit`.

| Original source | Source-links minimum | Graph minimum |
| --- | ---: | ---: |
| MozartTrio | 36,961,992 | 40,946,102 |
| MozaChloSample | 46,681,595 | 51,400,647 |

The 18-byte increase over the historical graph witness above is retained profile
metadata: `NormativeClosure` gained ` and linked-target`, and
`profileLogicalBytes` charges that string. Source-links minima and default
budgets are unchanged. Original DOCTYPEs and MozaChlo's UTF-16BE BOM are retained.
Both graphs fit the unchanged 64 MiB limit while unread-DTD incompleteness stays
explicit. These logical admission thresholds are not heap/RSS or benchmark
results. The measured production tree precedes this documentation/evidence
commit; this record does not claim the later commit was itself measured.

## Final verification gate

The earlier semantic acceptance and seven-stage/nine-job pending fields describe
the historical private candidate. They are preserved without turning source
routes or later focused passes into final acceptance. Current final packaged-tree
seven-stage results and exact published-head nine-job CI must be recorded
separately with their own commit/tree and execution evidence. A native 386
cross-build does not count as actual 386 execution; the remote actual-386 job and required real xmllint
remain separate mandatory gates. Earlier candidate passes are not final-head CI.
