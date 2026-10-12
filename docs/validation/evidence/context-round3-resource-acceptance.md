# Round 3 independent resource acceptance

PASS for exercised finite-resource contracts at `115b030ff450d067e51a5ee1ee985afb27967704`.

Production defaults remain 20,000,000 rule steps and 67,108,864 buffered bytes. B5-PERF-01 remains open for batch9. No production edits or GitHub writes were made by this auditor.

Measured production manifest: `b11b9a7c933a611f58bed23120306c0598567498532223f8cd1634c7a7d070d7` (247 entries). All measured production files match that commit; only test overlays were added.

## Matched measurements

Baseline is 0821afd. Eighty prior exact-harness observations are reused only after production-manifest and harness identity verification; 40 timewise baseline observations are fresh. All 120 final observations and 28 isolated-family controls were freshly run. Official source inputs remove only DOCTYPE and retain original encoding and BOM; model inputs decode untouched originals. MozaChlo remains UTF-16BE. Timewise synthetics preserve the existing note fixture, changing only the root and measure/part hierarchy.

| Fixture/path | Steps baseline → final | Minimum bytes baseline → final | Records baseline → final |
|---|---:|---:|---:|
| MozartTrio.musicxml/source | 41,319,658 → 41,381,037 | 92,939,940 → 96,698,435 | 11,105 → 11,105 |
| MozartTrio.musicxml/model | 40,509,894 → 40,569,928 | 90,509,904 → 94,263,641 | 11,105 → 11,105 |
| MozaChloSample.musicxml/source | 47,142,414 → 47,220,377 | 101,145,415 → 106,379,484 | 12,337 → 12,337 |
| MozaChloSample.musicxml/model | 46,226,065 → 46,302,398 | 97,906,233 → 103,134,552 | 12,337 → 12,337 |
| notes100/source | 6,776,892 → 6,776,911 | 21,836,040 → 21,835,544 | 2,174 → 2,174 |
| notes100/model | 6,648,606 → 6,648,625 | 21,410,286 → 21,409,790 | 2,174 → 2,174 |
| notes1000/source | 66,549,023 → 66,549,042 | 211,580,310 → 211,579,814 | 21,074 → 21,074 |
| notes1000/model | 65,293,595 → 65,293,614 | 207,430,356 → 207,429,860 | 21,074 → 21,074 |
| timewise100/source | 6,776,912 → 6,776,931 | 21,835,966 → 21,835,470 | 2,174 → 2,174 |
| timewise100/model | 6,648,602 → 6,648,621 | 21,410,191 → 21,409,695 | 2,174 → 2,174 |
| timewise1000/source | 66,549,943 → 66,549,962 | 211,580,236 → 211,579,740 | 21,074 → 21,074 |
| timewise1000/model | 65,294,491 → 65,294,510 | 207,430,261 → 207,429,765 | 21,074 → 21,074 |

Expanded controls use 512 MiB buffered bytes and 1 << 34 steps; all other limits remain defaults. Minimum bytes are binary-searched logical admission budgets verified exactly and one under, not peak heap. Counters legitimately change when corrected behavior changes traversal; all changes are recorded in JSON.

## Why the official costs increased

Public context row counts are unchanged: MozartTrio 11,105 and MozaChloSample 12,337. The corrected navigation pass now reaches more nested occurrence payloads instead of stopping at ordinary same-position playback updates. An independent baseline/final public-payload probe confirms:

| Official fixture | Nested occurrences | Newly started part instrument states | Context-fact increase |
|---|---:|---:|---:|
| MozartTrio | 43 → 198 (+155) | +4 | 6 × 155 + 4 = 934 |
| MozaChloSample | 60 → 281 (+221) | +1 | 6 × 221 + 1 = 1,327 |

The six existing per-occurrence charges are one runtime occurrence plus five owned state snapshots: note listening, carried play, local play, instruments, and listening. Their existing stateReserve path charges one context fact per snapshot; a newly reached part also creates its initial instrument state. Code: validation/context_navigation_machine.go (runNavigation), context_navigation_note_listen.go (navigationNoteListeningAt), context_navigation_instruments.go (navigationApplyInstruments), and timeline_state.go (stateReserve). These equations exactly reconcile the full-profile fact increases. Larger nested traversal payloads and their owned clones also explain why logical retained byte minima increase despite unchanged top-level row counts. No new duplicate-charge site was introduced for those snapshots.

Isolated model-family controls localize the added work:

| Family | MozartTrio step delta | MozaChloSample step delta |
|---|---:|---:|
| Navigation | +58,293 | +75,564 |
| Layout | +1,718 | +692 |
| Spans | +23 | +77 |
| Timeline, local, graphical repeats, performance | 0 | 0 |

Layout's extra boundary/consensus work and span identity preflight are bounded observed operations. The 100- and 1000-note partwise controls each add 19 layout steps and reduce their minimum buffered admission by 496 bytes; their other isolated family meters are unchanged. These family controls characterize each selected plan and must not generally be summed to predict shared-provider public runs.

The completed assessments do not establish complete musical playback: MozartTrio's extended traversals still stop at navigation-repeat-times-unavailable, while MozaChloSample's formerly false-ambiguous part now has a known completed traversal. B5-PERF-01 remains open; this is a correctness fix with higher owned output where more traversal is proved, not a universal performance improvement.

## Actual default outcomes

| Fixture/path | Result | Steps used | Records |
|---|---|---:|---:|
| MozartTrio.musicxml/source | rule-steps | 19,999,949 | 1,774 |
| MozartTrio.musicxml/model | rule-steps | 19,999,948 | 1,919 |
| MozaChloSample.musicxml/source | rule-steps | 19,999,979 | 1,751 |
| MozaChloSample.musicxml/model | rule-steps | 19,999,965 | 1,917 |
| notes100/source | complete | 6,776,911 | 2,174 |
| notes100/model | complete | 6,648,625 | 2,174 |
| notes1000/source | buffered-bytes | 12,994,218 | 444 |
| notes1000/model | buffered-bytes | 13,699,811 | 509 |
| timewise100/source | complete | 6,776,931 | 2,174 |
| timewise100/model | complete | 6,648,621 | 2,174 |
| timewise1000/source | buffered-bytes | 12,994,219 | 444 |
| timewise1000/model | buffered-bytes | 13,699,809 | 509 |

Every resource refusal is incomplete, unsatisfied, and undetermined rather than a document violation. Work-only expansion with the unchanged buffered limit remains separately measured in JSON. Both buffered and depth gauges are zero after every instrumented success and refusal, including binary-search probes. Caller-owned report memory is a separate lifetime.

## Independent boundaries

Focused quota/cancellation/retry checks and focused race checks pass. The independent overlay exhausts cancellation points in the 100-anchor tied preflight, samples 1000-anchor endpoints, and retries on the same Account after repeated one-under refusals. Layout empty-selection scratch exercises exact 512-byte known-part and 1024-byte absent-part empty-selection admissions, cancellation points, repeated refused attempts, and 100/1000-call linear work controls. The isolated cancellation-after-selection regression is verified fixed before measurement; its red evidence is retained. Author coverage also verifies scratch remains live through publication while persistent known-part buckets remain retained. Reader fallback tests distinguish abandoned construction from successfully committed cache ownership. The full accounting review is in [the ownership audit](context-round3-resource-code-audit.md); logs contain exact focused counters.

All 28 isolated-family runs release their live buffer gauges. All six frozen pre-context profile digests pass. Whole-repository seven-stage validation, architecture execution, and exact-head remote CI remain separate lead gates. Allocation and elapsed-time fields are single warmed diagnostics, not stable performance benchmarks.

## Ownership review and evidence limits

Reader fallback caches retain admitted backing only after successful construction. Abandoned tables and role slices release scratch; a cache committed before a later traversal interruption remains legitimately owned. Layout keeps known-part buckets as persistent state and holds unknown-part buckets plus key/order scratch through owned publication. Tied preflight and lyric ambiguity maps release provider-local scratch; escaped lyric identity strings receive independent ownership. Repeat coverage uses one source-order structural pass and one all-part aggregate rather than per-row rescans.

The independent audit exposed cancellation being overwritten by a later buffer refusal in staff selection. Early sticky-error guards were added, including the repeated-print final-candidate path. The fresh isolated red probe and final all-poll green/race evidence are preserved. A separate initial harness assumption that an Unknown-part row needed only scratch bytes was corrected: such a row legitimately requires owned unavailable output. Exact scratch-only 1024-byte checks use an Absent part; Unknown-part live-through-publication tests remain separately covered. No failed preliminary snapshot is used as final acceptance.

Production acceptance transfers to any packaging commit only when its non-test Go/module/schema manifest equals b11b9a7c933a611f58bed23120306c0598567498532223f8cd1634c7a7d070d7. All 247 measured production files were independently matched to the committed tree; the measurement began on its identical approved two-file snapshot. Final raw observations, exact errors and limits, family deltas, navigation payload counts, manifest identities and evidence hashes are in [the JSON companion](context-round3-resource-acceptance.json).
