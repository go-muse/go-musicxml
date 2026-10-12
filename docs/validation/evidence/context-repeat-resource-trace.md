# Repeat content and meter resource trace

Production checkpoint: `59dcdff9dde34cb25e1ee20730b8e3a77f207fb4`, including ordered comparison, measure buckets and beat-window integration at `0de79a1`. Independent QA checked this production tree plus test-only `0044570` as QA commit `dfb445b`; the combined root/validation regression run passed in [the exact-checkpoint log](independent-resource-59dcdff-regressions.log) (27.474s / 2.142s). This records bounded fixture evidence, not full normative closure, global linearity or B5-PERF-01 acceptance.

## Authentic RED to GREEN

Counts are charged rule steps for 100 then 1,000 inputs. GREEN includes index construction on the measured path. The 15× guards were retained; collector fixtures were adapted only to build/use the production index. RED logs were captured before the corresponding fixes.

| Probe | Authentic RED | Integrated GREEN | Fix / test provenance |
|---|---:|---:|---|
| Navigation edge regions | 31,200 → 3,012,000 | 1,501 → 15,001 | `2029047`; [original repeat RED](independent-resource-repeats-red.log) |
| Independent part ranges | 100,100 → 8,201,000 | 87,302 → 873,002 | `2029047`; [original repeat RED](independent-resource-repeats-red.log) |
| Per-part note membership | 33,000 → 1,230,000 | 24,002 → 240,002 | `2029047` plus metered voice filtering; [RED](independent-resource-repeat-note-scopes-red.log) |
| Transpose-presence lookup | 46,800 → 2,268,000 | 27,401 → 274,001 | `bac0df8`; test `7e8553b`; [RED](independent-resource-transpose-red.log) |
| Content selection across unrelated parts | 30,500 → 3,005,000 | 1,401 → 14,001 | `a7263d6`; test `b958aa2`; [RED](independent-resource-repeat-content-scope-red.log) |
| Voice exclusions | 153,407 → 13,234,007 | 24,810 → 248,010 | `a3f8d0f`; test `8a426e5`; [RED](independent-resource-repeat-voice-red.log) |
| Same-order two-measure content | 72,966 → 2,139,660 | 57,919 → 639,163 | `a024f90` + `0de79a1`; test `f2aa58f`; [RED](independent-resource-repeat-ordered-measure-red.log) |
| One-note sequential measures | 55,155 → 1,905,105 | 40,705 → 410,605 | `0de79a1`; test `6f3f3e0`; [RED](independent-resource-repeat-sequential-measures-red.log) |
| One-note sequential beats | 9,920,777 → 1,152,727,921 | 556,813 → 6,308,341 | `4c01801` + `0de79a1`; test `a3684e3`; [RED](independent-resource-repeat-sequential-beats-red.log) |

The last three unchanged guards also passed independently at `0de79a1` in [the integration log](independent-resource-measure-window-integration-green.log), and the parent retained [its growth run](repeat-integrated-growth-green.log). Final GREEN ratios range from 9.99× to 11.33×. Earlier 23,902/239,002 membership, 1,301/13,001 part-scope and 24,809/248,009 voice results describe earlier checkpoints; final counts include subsequently charged admission/voice-pass work, rather than an unexplained measurement substitution.

## SEM10: separate new collection work from existing query cost

The original `f865107` / independent test `db19d39` [RED](independent-resource-repeat-meter-unrelated-red.log) measured full/base/added work:

- 100 parts: 208,904 / 178,504 / 30,400
- 1,000 parts: 7,489,004 / 4,485,004 / 3,004,000

The shared provider roster, owned `03cf59b` and integrated `59dcdff`, produces:

- 100 parts: 179,706 / 178,504 / 1,202
- 1,000 parts: 4,497,006 / 4,485,004 / 12,002

Added-work growth falls from 98.82× to 9.99×. Both sides subtract identical authoritative effective-staves/time queries. Their 178,504 → 4,485,004 baseline is unchanged and is not claimed linear. The owner's separate [intermediate RED replay](repeats-sem10-roster-authentic-red.log) measured added 21,100 → 2,011,000; it is distinct from the original independent RED. [Owned GREEN](repeats-sem10-roster-authentic-green.log) agrees with the exact integrated run. No loop expands the 2^256 staff domain.

## Proof and ownership limits

Ordered matching proves only true; reordered inputs retain conservative multiset comparison and original unavailable-cause order. Exact staff/voice filters retain unclassified candidates. Measure grouping preserves coverage before selecting content. Beat indexing retains fallback for unknown placement, overlap, chords and unordered content. Generic mismatches remain Unknown; only the separately witnessed empty-target theorem can establish the supported negative.

Exact/one-under allocations, cumulative work and cleanup are checked. Final logs include 31/32 ordered-path and 70 fallback cancellation polls; 4 measure-roster polls; 125 beat-index polls; 8 meter-roster build and 106 cached-query polls. These are exhaustive observed boundaries for the named fixtures, not a universal cancellation theorem. The borrowed note value measures 664 bytes on the reviewed host versus the conservative 4,096-byte per-input measure-roster reserve.

Reordered multiset fallback and unproved within-part scopes can still be quadratic under finite quotas; emitting membership for every range/note pair may itself be output-sized. Existing shared budget defaults are unchanged. Local 386 binaries cross-compile but cannot execute on this kernel; actual 386 execution remains a CI gate. The [final independent summary](independent-resource-final-summary.json) confirms selected exact-checkpoint [race PASS](independent-resource-59dcdff-race.log) (198.163s / 18.813s), both 386 cross-compiles and five-sample [benchmarks](independent-resource-59dcdff-bench.log) complete. Both live gauges are zero in four independently checked success/refusal cases. No ordinary resource finding remains at this checkpoint; full-repository/CI gates remain separate.
