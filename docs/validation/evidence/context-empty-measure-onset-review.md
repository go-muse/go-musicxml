# Empty measure-repeat: scored-onset counterexample

Follow-up to [the remaining Round 1 review thread](https://github.com/go-muse/go-musicxml/pull/36#discussion_r4250385312), on canonical baseline `41eeaf482e2008d83bc00a476a7cd0c28f5a3bad`.

## Reproduction and accepted timeline boundary

The exact fixture has divisions 4 and 4/4, a whole C note of duration 16 in measure 1, measure-repeat start 1 and no notes in measure 2, and stop plus a note in measure 3. The same failure occurs with a duration-16 forward in measure 2 or no stop. All source/current-model and partwise/timewise combinations had complete range and note-scope evidence but returned Unknown content equivalence. The public reason was `context-repeat-content-equivalence-unresolved`; `context-repeat-empty-coverage-unresolved` was the private helper's reason. Twelve exact reviewer expectations were recorded RED before changing production.

Ordinary note End is intentionally marked `UnprovedFrame`. Existing frame containment derives its bounds from independently contained event origins, not terminal endpoints or nominal meter. The isolated one-note source therefore has a contained onset at zero but no independent terminal-end containment witness. This is the precise reason the previous end-before-target-origin proof could not order its quarter-note offset 4 against the next measure's origin. No timeline code, frame bound, position ordering or measure-capacity policy is changed here.

## Source-supported sufficient proof

The pinned MusicXML schema at musicxml.xsd:3048–3052 and the [official measure-repeat reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/measure-repeat/) require actual repeated music in each affected measure, with the stop marking the first unaffected measure. The new inference concerns encoded scored onsets: an uncertain earlier sustain endpoint cannot supply a newly encoded onset in the target measure. It does not assert acoustic silence. The [note](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/note/) and [duration](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/duration/) references distinguish intended score timing from performance attack/release adjustments.

The sufficient counterexample requires:

- A complete repeat range, semantic note universe and selected part/staff scope under existing gates.
- Zero selected note records in the target measure. Unsupported or malformed notes cannot be filtered away to manufacture an empty target.
- Available, valid, independently contained positions for every selected note, including later measures. No ordinal-only exclusion of displaced or uncontained onsets is introduced.
- A positive-duration ordinary non-rest source onset in the corresponding source measure. Rest-only sources and unavailable, grace/cue, sound-tie or other unsupported content keep their existing limitations.

An ordinary principal note with a supported chord remains a sufficient source witness even when the chord has no independently established End. A target containing rests is not covered by this new sufficient theorem. General mismatch, sounding carry, transposed/enharmonic equivalence and performance realization remain outside this proof.

## Resource and verification evidence

The onset proof is lazy and cached per existing selected scope. It reuses the scoped note roster and measure buckets, with an admitted sparse witness map and one availability/containment aggregate. Its charge remains live until that scope closes; cancellation and failed construction release temporary charges without publishing a partial cache.

The preserved author commit is `8a7a097c5ea011268aaeea14108513b11c361bd9`; its canonical integration is `c1512ff48fe149d8e2966a4422355858b4f5a167`. The test-only causal-strengthening commit is `9fe070528deff86bd75c332b879ba374f3e546c5` (canonical integration `d15a090`). Final packaging preserves the combined tree as one coherent follow-up commit; every original commit remains retained.

- `TestContextRepeatEmptyTargetOnsetProof`, `TestContextRepeatEmptyTargetOnsetProofBoundaries` and `TestContextRepeatEmptyTargetOnsetProofPatternAndStop` exercise 108 public adapter/hierarchy cases. They include the exact empty/forward/absent-stop fixtures; principal-plus-chord, overlong source and source backup; malformed or unavailable source/target content; negative and uncontained later onsets; scope gaps; multi-measure source mapping; and stop exclusion.
- The public positive fixture separately asserts that the ordinary source ScoreEnd remains `UnprovedFrame=true` at exact quarter-note offset 4 even while content equivalence is Known(false).
- `TestContextRepeatMissingOnsetResources` establishes exact admission at 15 work units and 3584 buffered bytes for its three-note internal fixture, one-under failures, and atomic cleanup at all nine cancellation polls. These numbers describe the isolated helper fixture, not full assessment.
- `TestContextRepeatMissingOnsetCacheIsLazyAndLinear` checks no onset cache allocation for a nonempty target; 1000 cached checks cost exactly 1000 further work units for both 100-note and 1000-note scopes, without additional retained bytes.
- `TestContextRepeatMissingOnsetPreservesGaps` checks Invalid causal evidence, malformed false/empty/absent availability, unavailable positions, future displacement, rest-only/no-principal and nonpositive-duration controls. Malformed availability cannot become either a false content counterexample or an unbuilt cache sentinel that leaks on retry.
- Existing shared-scope cleanup/cancellation and B6 cross-frame/displacement regressions remain part of independent verification. No B6 production file changes.

Authentic exact-fixture probe log SHA-256 values: RED `3df2299c77b0372f0c3ed97d335cf389680cd9301409a01c6ef55e4b8abe1b9b`; GREEN `1af67f2a6bc25924821e111891824b3983ae14904e386b27ec486060251c1fec`. The initial observational baseline log is `1ac663bf111b1b69865f620201638735804baa667239bde17b32a6096e0897a5`. These are provenance pins for preserved development evidence, not production inputs.

Independent semantic review accepted the exact three-file production manifest `926bf3344925c0f17dd090f828b2998618fbde9f88d847c94cef5b35af97cfb7`. Focused race passed in 1.139 seconds. The whole 247-entry production manifest is `818be3c075f68b96629d163b9ec010b3d1d708df8c3124daf148fc1e911251bb`.

Independent resource remeasurement used the identical retained Round 1 harness. Its [full machine-readable comparison](context-empty-measure-onset-resource.json) records 80 new observations against 80 already-validated baseline observations whose 246-entry production identity was independently reconciled to canonical `41eeaf4`. All deterministic values, thresholds, result summaries and source hashes match; diagnostic elapsed/allocation samples are not equality claims. All 32 public DeepEqual/error comparisons and 48 exact/one-under controls passed, with zero final buffered/depth gauges. Family controls and all six frozen profile digests also passed.

The additional committed `context_repeat_empty_resource_independent_test.go` tests the whole provider rather than only the helper: 100 versus 1000 distinct empty-repeat starts take 44,410 versus 444,010 work units with a single cached scope/map; indexed/unindexed paths check every cancellation poll (87/49), exact and one-under admission (898 steps/25,636 bytes and 71 steps/17,920 bytes), exact document/part/staff key separation and uncached voice-qualified release. An unproved-frame arithmetic fixture checks all 21 cancellation polls and exact 789-step/8384-byte admission. These finite fixture measurements do not claim unrestricted score scaling.

The practical limits are unchanged: 100-note default fixtures complete; the 1000-note default fixture still exhausts buffered bytes; official MozartTrio and MozaChloSample still need both explicit work and buffer budgets. `MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20` completes the exercised source/model cases. Original source DOCTYPE remains unsupported; source measurements remove only that declaration and preserve remaining bytes, including the Moza UTF-16BE BOM. B5-PERF-01 remains open.

The seven local stages and all exact-head CI jobs, including actual 386 execution and required real xmllint, remain separate final publication gates. Their exact head/run results are recorded on PR36 after publication. Local 386 compilation is not execution; the native executor cannot run that binary. Earlier green heads are not acceptance for this follow-up.
