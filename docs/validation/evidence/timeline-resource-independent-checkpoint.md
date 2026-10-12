# Independent timeline resource/API acceptance

Latest selected production: **22ae68a731ddc26e15f988fb59232e2dd9d572e6**.
Independent added guards: **38b77a3**. Date: 2026-10-12.

The production-source diff between the independent worktree and this selected
head is empty. The results below apply to this production snapshot; historical
checkpoints and their measurements are retained separately below. Resource/API
acceptance is scoped to the reviewed providers and executed tests. It does not
claim whole-batch closure or external publication approval. **B5-PERF-01 remains
outstanding**, owned by the **shared validation kernel/session workstream** with
**batch 9 reconciliation**.

## Selected-head executed controls

- The complete root `TestIndependent` selection passes. This includes all 32
  practical source/model, local/timeline, default/buffer-only-diagnostic variants
  for 1,000 directions, 1,000 figures and both grace fixtures, plus all 16
  100/1,000-movement uniform-unit excursion variants. Ordinary 100/1,000 notes,
  exact model float64 tenths, repeated divisions, huge exact staff identities,
  parallel immutable-profile sharing and diagnostic failure preservation pass.
- The full validation package passes (10.603 seconds), including the private
  independent and final-review tests. These retain
  checked host/int64 arithmetic, admission-before-allocation, owned causal
  pointers and borrowed vectors, transactional indices, and cleanup controls.
- The 1,000 composite-attribute fixture still stops at the shared 64 MiB limit in
  all four local/timeline source/model profiles. Its incomplete-resource outcome
  remains explicit and does not fabricate a musical failure.
- Independent RED identity guards showed that long canonical DocumentID equality
  could run before work admission. Parent core fixes and the narrowly authorized
  provider fix `4b639a8` now charge comparisons and validity checks before using
  those strings. All eight provider guards and the position/closure guards pass;
  a separate semantic reviewer inspected the production patch for equivalence.

### New raw-divisions certificate controls

`TestIndependentTimelineDivisionsValue*` tests establish a closed map/gap shape,
context-fact admission, byte/work admission before publication, and exact
upfront scratch admission for 32 filtered gap rows. The test certificate retains
1,322 logical bytes; all 9 observed insertion cancellation boundaries leave no
published certificate. All 126 owned-query boundaries preserve the raw gaps,
certificate map and borrowed original index, with scratch released on failure.

Primitive and owned results agree for both same-value consensus and different-
value fallback. Each primitive case passes all 112 observed cancellation
boundaries. Primitive queries retain only the original result's 128-byte owned
causal record, with no unused numeric/provenance copies. Returned witnesses and
fallback causes are independently owned. Same-value certificates never supply a
missing baseline; semantic controls separately cover invalid/hidden publishers,
repair and exclusion.

### Expanded normalization lifecycle controls

`TestIndependentTimelineNormalization*` tests measure the closed map/cause shape
against the named 512-byte gate allowance, reject before map admission at one
byte below that allowance, and verify independently owned actual invalid causes.
A direct stopped-stage control restores a gate exactly once using an independent
known origin and unit publisher, without retaining a duplicate gate/result.
All 47 direct restoration cancellation boundaries preserve the uncommitted gate
and allow a successful retry.

A complete-history fixture explicitly verifies that new normalization gates
exist, then sweeps all 4,460 observed staged-reconstruction cancellation
boundaries. The original service, facts and frame bounds remain unchanged;
selected retries succeed. Exhaustion of the shared context quota cannot
partially swap the original service. All observed scratch gauges reconcile to
session-retained bytes and return to zero on session cleanup. Finite progress is
restricted to admitted per-event gates, strict finite cause-rank upgrades, one
restoration, and per-excursion qualification/removal transitions.

Architecture compile-time guards now include certificate map/gap widths and
normalization map/cause widths, in addition to all prior closed record, frame,
index, grace and excursion shapes. Logical live/retained accounting is not a
formal process-RSS bound. No default quota was increased.

Logs in the batch-6 evidence bundle:
`resource-independent-22ae68a-public.log`,
`resource-independent-22ae68a-private.log`,
`resource-independent-22ae68a-validation-full.log`, and
`resource-independent-22ae68a-new-guards.log`.

## Selected-head race, architecture and matched costs

Focused race checks pass for certificate/normalization, staged reconstruction,
Placement ownership, floating associations, key presence and reference-work
controls (validation 89.778 seconds), plus the parallel-profile/owned-report and
diagnostic-cap public controls (1.857 seconds). `go vet ./validation ./internal/xsd`
and both root and validation `GOARCH=386 go test -c` compilations succeed. The
compiled binaries are identified as 32-bit Intel386 ELF. **No 386 execution was
performed**: the earlier host `exec format error` remains an external CI gate.
Log: `resource-independent-22ae68a-race-vet-386.log`.

The refreshed cancellation fixtures additionally cover 3,384 stopped-stage and
7,205 restored-stage reconstruction polls, 34 inventory polls with idempotent
release, 936 all-family Placement polls, and 1,532 direction / 1,528 figure
floating-association polls. These counts describe the observed fixture at this
revision, not a universal cancellation latency guarantee.

Five separate warmed matched runs used Go1.27.2 linux/amd64, GOMAXPROCS4,
`-benchtime=200ms -count=1 -benchmem`. Each local/timeline pair uses the same
selected production and fixture. Source includes XML parsing; model reuses a
decoded model. Timing-only budgets are 512 MiB and 2^34 steps, with successful
fixture assertions. Default-budget acceptance is established by the separate
public tests above. No other coordinated heavy test/build ran during this
interval; shared-host timing noise remains visible and must not be read as a
throughput guarantee.

| Fixture / target | Local median ms (range) | Timeline median ms (range) | Median bytes local → timeline | Median allocs local → timeline |
| --- | ---: | ---: | ---: | ---: |
| 100 notes / source | 21.40 (18.94–35.69) | 26.11 (23.34–41.67) | 5,895,383 → 8,521,459 | 32,278 → 64,655 |
| 100 notes / model | 20.12 (19.78–28.77) | 25.24 (23.46–29.59) | 5,449,500 → 8,075,763 | 19,877 → 52,254 |
| 1,000 notes / source | 211.05 (202.49–308.64) | 292.55 (254.86–334.04) | 52,677,464 → 78,759,784 | 300,862 → 621,638 |
| 1,000 notes / model | 211.73 (186.00–248.60) | 263.81 (249.80–291.17) | 48,341,472 → 74,423,784 | 181,418 → 502,194 |
| 100 attribute sets / source | 59.49 (53.73–68.22) | 71.07 (63.96–77.28) | 15,954,008 → 20,765,028 | 92,150 → 149,562 |
| 100 attribute sets / model | 67.16 (52.99–69.15) | 70.52 (67.41–81.99) | 14,748,365 → 19,547,405 | 60,155 → 117,566 |

For 1,000 ordinary notes, the measured median timeline/local time ratio is
1.39× source and 1.25× model; cumulative allocation ratios are 1.50× / 1.54×
bytes and 2.07× / 2.77× allocation counts. B/op is cumulative allocation, not
simultaneous retained bytes or process RSS. These are incremental profile costs
at the selected implementation. They do not remeasure the earlier B5 legacy
baseline or close B5-PERF-01.
Log: `resource-independent-22ae68a-profile-bench.log` (all 60 samples passed).

## Acceptance boundary and remaining integration gates

No concrete resource/API blocker remains in the reviewed production snapshot.
The exact final documentation/test integration still requires the parent's
aggregate seven-stage check and scope reconciliation. The parent separately
reports its initial production `22ae68a` seven-stage run green; that is not
substituted for a final integrated-head result here. Actual 386 execution remains
an external CI requirement. Whole-batch completion and external publication are
outside this resource checkpoint. **B5-PERF-01 stays outstanding under shared
kernel/session ownership and batch 9 reconciliation.**


---

# Historical checkpoint: ab8da72

Reviewed production: **ab8da72c1c538946945d4c06cb8139360714b1e7**. Date: 2026-10-12.

This is an accepted resource checkpoint, not whole-batch or semantic closure. The
reviewer used an independent worktree and changed tests/documentation only.
Subsequent instrument point-gap, figure-locality and cross-frame refinements
require a selected-head refresh. B5-PERF-01 remains **outstanding**, owned by the
**shared validation kernel/session workstream**, with **batch 9 reconciliation**.

## Executed results

- All root `TestIndependentTimeline*` tests pass. This includes all eight public
  source/model, local/timeline, default/buffer-only-diagnostic variants for each
  of 1,000 notes plus 1,000 directions and 1,000 notes plus 1,000 figures.
- Ordinary 100/1,000-note source/model cases, integer and exact float64-tenth
  duration cases, and 1,000 repeated divisions declarations pass default limits.
- 8,192-bit staff identities remain sparse; exact 1/3 positions and source/model
  numeric distinctions survive. Parallel profiles sharing compiled immutables,
  owned returned reports, and diagnostic-cap failure preservation pass.
- Independent validation/rational tests pass: quota boundaries, nested causal
  ownership, checked arithmetic, transactionality, cleanup and scratch release.
- Exhaustive observed cancellation polls: direction index 1,110; figure index
  762; cursor/deferred finalization 869; nested note origins 443; note-instrument
  copy 17; cache eviction 3; direct Decimal quotient 4. Poll counts describe this
  fixture/revision rather than a universal cancellation interval.
- The 1,000 composite-attribute fixture remains resource-incomplete in all four
  prior local/new timeline source/model profiles, with no fabricated musical
  failure. This existing boundary is not closed by the new-provider improvements.

Commands: `go test . -run TestIndependentTimeline -count=1 -v` and
`go test ./validation -run 'TestIndependentTimeline|TestTimelineFigureCachesRemainBounded|TestTimelineRational|TestRational' -count=1 -v`.
Logs: `resource-independent-ab8da72-public.log` and
`resource-independent-ab8da72-focused.log` in the batch-6 evidence bundle.

## Independent RED → GREEN evidence

| Defect or guard | Independent test commit | Accepted correction/checkpoint |
| --- | --- | --- |
| Event-fact context quota and borrowed query causes | `50f001c` | `f584e62` |
| Exact model-tenth/repeated-divisions practical work | `e87485b`, root resource fixtures | `b16dc03`, rechecked here |
| Note/figure quota and origin cause ownership | `9aeb515`, `379917b`, `5330ace` | owned-cause walk and precedence corrections |
| Bend append old/new backing peak | `a0f7dca` | named 8,192-byte per-record bound |
| Late same-position units for offsets/grace origins | `e981eeb`, `0ce39f7` | deferred finalization, `e902e22` |
| Cancellation incorrectly became a contract failure | `3d010c9` | `6bcf8d4` and exhaustive refreshed polling |
| Quadratic ordinary direction/figure candidate searches | `730f349`, `fcc81b5` | sufficient-proof indices; causal fallback retained |
| Unused divisions query copies/work | `363af7d` | closed immutable Decimal projection, owned causes |
| Fixed note/direction/fact live peaks | `8f2c54c`, `5376544` | named allowances plus equal released per-call scratch |
| Pending scratch released twice refunded unrelated bytes | `17f9e58` | idempotent early release |
| Document-wide figure quota atomicity | `dd1c097` | second admission failure leaves zero published records |
| Direct Decimal quotient ownership and literal limits | `7fb33b0` | `c0ac72f`; 56 independent oracle pairs through 4,096 bits |
| Closed note-instrument copy/append/cause/peak guards | `2b59222` | `ab8da72` |
| Bounded figure cache widths and eviction work | `94b33e8` | `46cd579`, independently executed here |

The initial source-figure 64 MiB regression remained after the work fix. It was
resolved by shorter scratch lifetimes, compact/bounded caches and avoiding
redundant ownership copies. Default budgets were not increased. Buffer-only
512 MiB diagnostic cases isolate work limits; they are not default-limit passes.

## Accounting and API boundaries

- Named fixed allowances include explicit growth headroom. Mutable causal
  pointers and borrowed reference vectors remain copied; immutable numeric
  wrappers and strings may be shared. Empty note-instrument records retain512
  rather than846 logical bytes; no ownership-transfer shortcut is applied to
  their borrowed input references.
- At amd64 widths, the three capped64-entry cache tables have an independent
  conservative four-times-entry-width bound99,840 bytes including header slack,
  below131,072 admitted. Variable chain scratch bound296 is below384/figure;
  domain shape/growth bound792 is below2,048/domain. Cache clear work is charged
  before traversal. Candidate indices retain only immutable-record IDs.
- Fixed record peaks and user-sized vector admission are checked before
  publication/allocation. Every cancellation/error cleanup preserves cumulative
  counters and releases the session's own live scratch; another session's live
  bytes cannot be refunded.
- Sorting has cooperative cancellation: after comparison failure, admitted
  sorting may drain with constant-work failed comparisons before discarding its
  candidate. Total post-cancellation drain is not claimed constant-time.
- Logical retained/scratch budgets are not a formal process-RSS bound. Go runtime
  caches, allocator behavior and concurrent process overhead are separate.
- Sparse index selection, equal-position ties and unavailable causes match the
  original causal scans on independent position/voice/owner/domain controls.
  Exact typed-reference gaps remain explicitly unavailable, never inferred Pass.

## Historical matched incremental profile costs (ab8da72)

Five separate warmed runs at the pinned head, Go1.27.2 linux/amd64, GOMAXPROCS4,
`-benchtime=200ms -count=1 -benchmem`. Both profiles use the same head and fixture.
Source includes XML parsing; model reuses a decoded model. Timing budgets are
512 MiB and 2^34 steps, with successful fixture assertions; default limits are
separately verified above. These are incremental timeline costs against the
existing local profile, not the earlier B5 legacy baseline.

| Fixture / target | Local median ms (range) | Timeline median ms (range) | Median bytes local → timeline | Median allocs local → timeline |
| --- | ---: | ---: | ---: | ---: |
| 100 notes / source | 19.67 (19.08–22.52) | 25.09 (23.07–26.65) | 5,895,366 → 8,259,168 | 32,278 → 63,451 |
| 100 notes / model | 18.91 (18.56–20.24) | 23.06 (22.40–26.35) | 5,449,492 → 7,813,537 | 19,877 → 51,052 |
| 1,000 notes / source | 202.88 (195.70–257.36) | 254.79 (235.33–279.27) | 52,677,312 → 76,157,104 | 300,860 → 609,634 |
| 1,000 notes / model | 200.20 (191.37–209.02) | 252.80 (246.12–262.68) | 48,341,496 → 71,821,656 | 181,418 → 490,192 |
| 100 attribute sets / source | 55.13 (52.48–57.56) | 66.35 (64.89–70.08) | 15,953,882 → 20,386,508 | 92,150 → 148,260 |
| 100 attribute sets / model | 53.44 (53.02–54.36) | 68.03 (61.82–73.31) | 14,777,084 → 19,235,748 | 60,157 → 116,266 |

For ordinary1,000-note fixtures, timeline median time is approximately1.26×,
bytes1.45× source/1.49× model, allocations2.03× source/2.70× model. Shared-host
wall-clock noise is visible; this is not a general throughput guarantee. B/op
measures cumulative allocations, not simultaneous live budget usage or RSS.
Log: `resource-independent-ab8da72-profile-bench.log`.

## Gates remaining at the ab8da72 checkpoint

- Refresh the exact final semantic integration head, including matched costs,
  focused race/vet and 386 compilation. Earlier-head checks are not promoted.
- Actual386 execution remains an external CI gate: this executor previously
  returned `exec format error`. Cross-compilation is not an execution result.
- Final repository seven-stage checks, required external xmllint execution,
  semantic prefix/cross-frame review, and scope-ledger reconciliation remain
  assigned workstream gates. No whole-batch or publication approval is asserted.

## Cross-frame and displacement refresh (2026-10-12)

The original benchmark above remains pinned to `ab8da72`; it has not been
promoted to a later implementation. The following independently executed
checkpoints extend the resource evidence without closing the remaining gates.

At production **904c7cf**, all root `TestIndependentTimeline*` cases pass,
including all **32** practical direction/figure/grace variants. The additional
16 grace variants cover 1,000 total notes with 100 spaced grace notes and a
1,000-grace group between two regular donors, across source/model,
local/timeline, and default/buffer-only-diagnostic profiles. The selected private
resource, grace, point-gap, conditional-repair and cross-frame suites also pass.
The composite-attribute fixture continues to hit the shared local/timeline
64 MiB limit. Logs: `resource-independent-904c7cf-public.log` and
`resource-independent-904c7cf-private.log`.

At production **1f7797bb3f4e5070c769ff16eef66756f46675d0**, independently added
publisher-Placement tests (`e7cf607`) establish:

- All 16 attribute/instrument/MIDI/playback gap publications own separate
  mutable endpoint records and causes, including separation between families.
- Context quotas 1, 7 and 15 stop after exactly that many admitted publications.
  The first closed gap's 3,262-byte admission minus one rejects before any
  publication; cumulative context accounting is not refunded.
- Every one of 880 observed all-family cancellation polls preserves borrowed
  inputs, leaves no unreleased scratch, and returns live bytes to zero on
  session cleanup. A partially completed multi-family operation may retain its
  already admitted independent publications until cleanup.
- Floating two-pass association searches pass all 1,113 direction and 1,240
  figure cancellation polls without retained query scratch or raw mutation.
  These are measured fixture counts, not fixed cancellation latency promises.
- Grace's closed scratch shape is 4,072 bytes/note and 3,656 bytes/block against
  separately admitted 4,096-byte coefficients. The independent actual slice
  growth test covers up to 4,096 empty blocks. Compile-time width constraints
  also enforce these margins in architecture-only builds.

The selected private independent/grace/Placement suite, focused race run,
`go vet ./validation ./internal/xsd`, and validation-package GOARCH=386 test
binary compilation pass at that production snapshot with the independent tests.
Logs: `resource-independent-1f7797b-placement.log`,
`resource-independent-1f7797b-private.log`, and
`resource-independent-1f7797b-race-vet-386.log`. The 386 binary was compiled,
not executed.

Other independently tested cross-frame safeguards include 216 exact sum/order
oracle triples through 4,096 bits; literal nested arithmetic scratch, scalar and
work boundaries; transactional partial-query scratch admission and cancellation;
separate owned unavailable causes; and index/frame closed-shape bounds. An
unrelated floating gap initially forced 128 ordinary publications from 4,211
work units to about 2.734 million. Independent RED `593383a` is green after
scoping the exceptional-path trigger to the queried part/staff (`a55aee9`). The
ordinary indexed path is therefore tested independently of exceptional history.

Still pending: global unit-region reconstruction's staged-service admission,
progress and cancellation review; the selected final-head full practical rerun,
matched costs, race/vet and 386 compilation; and the parent workstream's full
repository checks and semantic reconciliation. Actual 386 execution remains a
CI gate. **B5-PERF-01 remains outstanding with shared kernel/session ownership
and batch 9 reconciliation.**
