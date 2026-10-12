# Independent timeline resource/API checkpoint

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

## Matched incremental profile costs

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

## Remaining gates

- Refresh the exact final semantic integration head, including matched costs,
  focused race/vet and 386 compilation. Earlier-head checks are not promoted.
- Actual386 execution remains an external CI gate: this executor previously
  returned `exec format error`. Cross-compilation is not an execution result.
- Final repository seven-stage checks, required external xmllint execution,
  semantic prefix/cross-frame review, and scope-ledger reconciliation remain
  assigned workstream gates. No whole-batch or publication approval is asserted.
