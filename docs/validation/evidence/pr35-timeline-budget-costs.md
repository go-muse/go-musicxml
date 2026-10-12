# PR 35 matched timeline budget costs

Published baseline: **632a9690be60586daf68996914b1fdcb353c4a38**.
Date: 2026-10-12. Toolchain: Go 1.27.2 linux/amd64, `GOMAXPROCS=4`.
Review: [API budget thread 4246488604](https://github.com/go-muse/go-musicxml/pull/35#discussion_r4246488604).

This documentation/evidence change does not change production quotas or code.
The count/boundary controls below were independently executed. The existing
matched 1,000-note measurements and the reviewer's additional timings have
separate provenance and are not promoted to fresh measurements of a later head.
**B5-PERF-01 remains open**, with shared validation kernel/session ownership and
batch-9 reconciliation. This note makes no whole-batch acceptance claim.

## Reproduce the account/boundary controls

The opt-in root test `timeline_budget_evidence_test.go` runs only with the
`timeline_budget_evidence` build tag. It measures current implementation costs;
it intentionally does not freeze budget coefficients as an API contract.
Run from the repository root, using the pinned toolchain:

```sh
go version
GOMAXPROCS=4 go test -tags timeline_budget_evidence \
  -run '^TestPR35TimelineBudgetEvidence$' -count=1 -v .
```

For an exact baseline reproduction after production changes, use a separate
worktree at `632a9690be60586daf68996914b1fdcb353c4a38` and copy this opt-in test
into that worktree before running it. The baseline fixture SHA-256 is:

```text
8d491c8db4d19c886ec157e2c26fdda91ee8faa801e24b31aaf8cf092d73c63f  testdata/official/MozartTrio.musicxml
```

The probe performs these checks without timing or allocation benchmarks:

1. Decode the unchanged official MozartTrio fixture once. Model assessment does
   not reparse or measure source DTD support. Reuse that immutable model for both
   explicit model profiles. Each sees exactly 2,516 input nodes.
2. Check `AssessModel` with unchanged default budgets and require a satisfied
   report with no operation error for each profile.
3. Open the same engine/profile/walker path with a public `validation.Account`,
   use `Usage` to read cumulative steps and context facts, and check that live
   buffered usage is zero after `Finish`.
4. For boundary experiments, use `validation.DefaultBudgets()` with only
   `MaxBufferedBytes=256<<20` (268,435,456 bytes) and
   `MaxRuleSteps=100_000_000` lifted. All other finite defaults stay unchanged:
   64 MiB input/external, depth 1,024, 1,000,000 nodes/IDs/context facts,
   4,000,000 attributes, 8,000,000 events, 1 MiB scalar,
   128 linked documents, 1,000 diagnostics and 1 MiB diagnostic bytes.
5. Bisect the buffer cap in `[1, 256<<20]` using **public `AssessModel` calls**.
   Require every unsuccessful candidate to be incomplete with the expected
   typed `buffered-bytes` resource error, not an unrelated normative failure.
6. Verify success at each exact buffer/step/context minimum and a typed resource
   failure one below where a positive boundary exists. Then verify success with
   all three minima set simultaneously. Only the selected cap varies during
   each individual boundary control.

### Context-fact zero is not a zero cap

This fixture's local profile consumes **zero** context facts. Its smallest
positive configurable cap is 1; setting a `Budgets` field to zero selects the
finite default. Therefore no one-below failure exists for local context usage.
The probe checks public `AssessModel` with `MaxContextFacts=1`, then starts an
engine session with that one-unit shared account already consumed. The latter
still succeeds, with cumulative usage remaining 1, proving that the local
session needs no additional facts. This reservation control is not presented as
an `AssessModel` option to request a literal zero cap.

Timeline uses 1,199 facts. It succeeds at 1,199 and fails at 1,198 with
`ResourceError.Kind == BudgetContextFacts`. The review's local figure of 1 is
thus a configurable-cap observation, not one consumed fact. The actual
cumulative fact increment is **1,199**, or **0.476550 facts per node**; the
positive configurable cap difference is 1,198.

## Independently executed baseline result

[Complete raw probe output](pr35-timeline-budget-costs.log) records all exact
success and one-below controls. The final run passed in 12.217 seconds; that
whole-test duration includes bisection and is **not a per-assessment benchmark**.

| Quantity | Model local | Model timeline | Timeline minus local |
| --- | ---: | ---: | ---: |
| Nodes | 2,516 | 2,516 | 0 |
| Minimum logical live-buffer allowance | 18,155,049 bytes | 21,077,650 bytes | 2,922,601 bytes (+16.098%) |
| Minimum rule-step allowance | 2,848,163 | 4,193,534 | 1,345,371 (+47.236%) |
| Cumulative context facts | 0 | 1,199 | 1,199 |
| Smallest successful positive context cap | 1 | 1,199 | 1,198 |
| Logical buffer bytes per node | 7,215.8382 | 8,377.4444 | 1,161.6061 |
| Rule steps per node | 1,132.0203 | 1,666.7464 | 534.7262 |
| Context facts per node | 0 | 0.476550 | 0.476550 |

`MaxBufferedBytes` bounds the conservative live reservations in the shared
account, including retained facts and admitted scratch; it is not total bytes
ever allocated. The exact minimum is a successful-run admission threshold,
not a sampled process-memory peak. `Account.Usage(BudgetBufferedBytes)` returns
current usage, not a historical maximum, and is zero after this fixture's
cleanup. `B/op` and `runtime.MemStats.TotalAlloc` differences count cumulative
Go allocation, including temporary objects released during a call. Neither
is RSS or a formal process heap bound. Compilation/runtime caches, allocator
behavior, the caller's model and concurrent activity prevent such an inference.

These are fixture averages. Long exact values, different schema types,
attributes, failures, state-history structure and ambiguous cross-frame
reconstruction can change all three costs; some bounded exceptional searches
are quadratic. Do not multiply the per-node averages into a universal node
capacity. No default limit is raised by these measurements.

## Existing independently measured matched timing

The [resource checkpoint](timeline-resource-independent-checkpoint.md#selected-head-race-architecture-and-matched-costs)
is the source of the five-run 1,000-note comparison. It is pinned to production
`22ae68a731ddc26e15f988fb59232e2dd9d572e6`, whose production Go files are identical
to published `632a969`; the intervening diff contains documentation and tests.
This documentation pass has not rerun those benchmarks.

| 1,000-note target | Local median (range) | Timeline median (range) | Local → timeline B/op |
| --- | ---: | ---: | ---: |
| Source | 211.05 ms (202.49–308.64) | 292.55 ms (254.86–334.04) | 52,677,464 → 78,759,784 |
| Model | 211.73 ms (186.00–248.60) | 263.81 ms (249.80–291.17) | 48,341,472 → 74,423,784 |

Source/model timeline-to-local time ratios are **1.39× / 1.25×**; cumulative
allocated-byte ratios are **1.50× / 1.54×**. The source path includes parsing;
the model path reuses a decoded model. The checkpoint states Go 1.27.2
linux/amd64, `GOMAXPROCS=4`, five separate warmed 200 ms samples, finite timing
limits of 512 MiB buffer and 2^34 steps, successful fixture assertions and no
other coordinated heavy test/build during the window. Shared-host noise is
visible in the ranges. Its separate default-budget tests, not the lifted-limit
benchmark, establish default-budget acceptance.

## Reviewer-reported additional timing/allocation

The [review comment](https://github.com/go-muse/go-musicxml/pull/35#discussion_r4246488604)
reports these additional matched current-model figures at `632a969`:

| Fixture | Local | Timeline | Local → timeline cumulative allocation |
| --- | ---: | ---: | ---: |
| MozaChloSample | 68 ms | 76 ms | Not supplied |
| MozartTrio, each part's measures repeated 10× | 0.61 s | 0.71 s | 269 MB → 344 MB |
| Same, 50× | 3.88 s | 4.50 s | 1.44 GB → 1.83 GB |

The two replicated timing ratios are approximately 1.16×; allocation ratios
are approximately 1.28× / 1.27×. Values and units above are retained as reported.
The review says lifted budgets, but does not supply their numeric values,
sample protocol, timing ranges or whether MB/GB are decimal or binary. These
are **not independently reproduced controlled benchmark results** in this
note. The baseline 1× MozartTrio budget minima were independently reproduced
above with explicitly finite limits; the larger fixtures were not remeasured. No throughput guarantee, later-head timing claim or closure of
B5-PERF-01 follows from either source.
