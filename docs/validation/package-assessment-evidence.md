# Package assessment acceptance evidence

This is the bounded batch-8 implementation record. The [scope and ownership
inventory](package-assessment-scope.md) preserves original obligations and later
owners; this document records engineering evidence separately from normative
closure. Full ZIP/JAR/DEFLATE, dictionaries and unresolved external interpretations
remain explicit. B5-PERF-01 is not closed by these measurements.

## Test-first development and independent corrections

The original unpublished development commits and task logs are preserved in the
engineering archive. Logical publication milestones group each failing contract
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
contain 266 named leaf cases. The target-kind supplement adds 12 cases for Opus
score links to partwise/timewise targets and wrong-kind Opus targets across source,
current-model, raw-MXL and current-MXL entry points. A further 22 independent
XML-depth cases cover exact/refused primary/local/external/nested paths, graph-hop
independence, plain siblings and contextual reuse of an already cached deep source.
Each ordinary CI execution
runs the matrices once; repeated focused independent runs are separate evidence.

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

At independently remeasured production candidate
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

## 2026-10-13 snapshot/local-link review remeasurement

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

| Original source | Published links minimum | Corrected links minimum | Published graph minimum | Corrected graph minimum |
| --- | ---: | ---: | ---: | ---: |
| MozartTrio | 47,656,776 | 36,961,992 | 72,541,715 | 40,939,940 |
| MozaChloSample | 60,222,816 | 46,681,595 | 91,412,475 | 51,394,485 |

All values are logical buffered bytes, not process heap/RSS. Both corrected graph
runs now fit the unchanged 64 MiB default and retain unread-DTD incompleteness.
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

## Final verification gate

Independent semantic acceptance covers the final production candidate above;
resource acceptance rechecks all four entry points and exact capacity boundaries.
The final packaged-tree seven-stage run and exact published-head nine-job CI are
recorded separately at publication. A native 386 cross-build does not
count as actual 386 execution; the remote actual-386 job and required real xmllint
remain separate mandatory gates. Earlier candidate passes are not final-head CI.
