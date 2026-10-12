# Round 1 independent resource acceptance

**PASS for the exercised finite-resource contracts and measured reductions at `3e778bf36e60fca4644b65f1c448a32e9073a128`.** The official context profiles still need explicit budgets. B5-PERF-01 remains open.

Canonical baseline: `b54786a95a1203db98397bfb4ee046d9e1835355`. The auditor made no production changes. Later documentation/test-only integration is equivalent only if the reported production manifest remains unchanged.

## Inputs and scope

- Two unreplicated official MusicXML 4.0 files: MozartTrio (262 notes) and MozaChloSample (278 notes).
- Current-model assessment decodes the untouched original file. Bounded source assessment removes only the external DOCTYPE, retaining original encoding, BOM, declaration, and every other byte.
- MozartTrio: 94,916 original bytes, 94,794 bounded-source bytes. MozaChloSample: UTF-16BE, 223,068 original bytes, 222,824 bounded-source bytes.
- Synthetic inputs are the existing independentTimelineScore(n, false, "1") fixture: 100 notes / 8,853 bytes and 1000 notes / 82,653 bytes.
- No replicated-official x2/x4/x10 scaling was rerun. The two synthetic sizes do not prove global linearity.

## Full-budget matched results

All eight final source/model cases complete, are conformant and satisfied, and return no operation error with MaxRuleSteps = 1 << 34 and MaxBufferedBytes = 512 << 20; all other fields use unchanged defaults. Buffered values below are binary-searched minimum admission budgets, verified at exact and one-under limits. They are not peak Go heap.

| Fixture/path | Rule steps, baseline → final | Min buffered bytes, baseline → final | Facts, baseline → final | Records, baseline → final |
|---|---:|---:|---:|---:|
| MozartTrio/source | 93,685,368 → 41,319,658 | 118,296,584 → 92,939,940 | 12,887 → 12,874 | 11,105 → 11,105 |
| MozartTrio/model | 92,850,985 → 40,509,894 | 115,826,858 → 90,509,904 | 12,887 → 12,874 | 11,105 → 11,105 |
| MozaChloSample/source | 108,981,246 → 47,142,414 | 129,729,396 → 101,145,415 | 14,177 → 14,161 | 12,349 → 12,337 |
| MozaChloSample/model | 108,041,999 → 46,226,065 | 126,490,428 → 97,906,233 | 14,177 → 14,161 | 12,349 → 12,337 |
| notes100/source | 15,073,979 → 6,776,892 | 26,554,661 → 21,836,040 | 3,284 → 3,283 | 2,174 → 2,174 |
| notes100/model | 14,944,682 → 6,648,606 | 26,129,300 → 21,410,286 | 3,284 → 3,283 | 2,174 → 2,174 |
| notes1000/source | 146,555,410 → 66,549,023 | 257,486,715 → 211,580,310 | 32,084 → 32,083 | 21,074 → 21,074 |
| notes1000/model | 145,289,971 → 65,293,595 | 253,340,754 → 207,430,356 | 32,084 → 32,083 | 21,074 → 21,074 |

## Actual default and work-only outcomes

Default limits remain 20,000,000 rule steps and 67,108,864 buffered bytes. “Work-only” sets MaxRuleSteps = 1 << 34 while retaining the default buffered limit. Every resource refusal below is incomplete, unsatisfied, and undetermined; it is not an invalid-document verdict. All have complete input traversal. The JSON companion contains exact error text, requested/used/limit values, normalized budgets, and every usage gauge/counter.

| Fixture/path | Default result; records; used steps | Work-only result; records; used steps |
|---|---|---|
| MozartTrio/source | rule-steps limit; 1,774; 19,999,941 | buffered-bytes limit; 5,266; 28,366,928 |
| MozartTrio/model | rule-steps limit; 1,919; 19,999,940 | buffered-bytes limit; 5,813; 29,113,180 |
| MozaChloSample/source | rule-steps limit; 1,751; 19,999,971 | buffered-bytes limit; 4,591; 26,662,008 |
| MozaChloSample/model | rule-steps limit; 1,917; 19,999,957 | buffered-bytes limit; 5,288; 27,954,709 |
| notes100/source | complete/conformant; 2,174; 6,776,892 | complete/conformant; 2,174; 6,776,892 |
| notes100/model | complete/conformant; 2,174; 6,648,606 | complete/conformant; 2,174; 6,648,606 |
| notes1000/source | buffered-bytes limit; 444; 12,994,218 | buffered-bytes limit; 444; 12,994,218 |
| notes1000/model | buffered-bytes limit; 509; 13,699,811 | buffered-bytes limit; 509; 13,699,811 |

## Isolated family controls

Each row independently selects one sealed provider plus the same-head accepted timeline and ingress. The delta subtracts that same-head timeline control. These rows must not be summed to predict the combined public profile. The selected-state optimization also changes timeline query work, so an unchanged timeline meter is not assumed. All 28 baseline and 28 final family controls complete and clean up.

| Official fixture/family | Total steps, baseline → final | Steps above same-head timeline, baseline → final | Records, baseline → final |
|---|---:|---:|---:|
| MozartTrio/timeline | 4,193,534 → 3,753,938 | 0 → 0 | 0 → 0 |
| MozartTrio/local | 22,298,207 → 9,825,890 | 18,104,673 → 6,071,952 | 2,575 → 2,575 |
| MozartTrio/layout | 33,434,735 → 19,587,791 | 29,241,201 → 15,833,853 | 7,204 → 7,204 |
| MozartTrio/performance | 36,666,843 → 15,135,089 | 32,473,309 → 11,381,151 | 433 → 433 |
| MozaChloSample/timeline | 4,852,752 → 4,607,109 | 0 → 0 | 0 → 0 |
| MozaChloSample/local | 27,594,275 → 12,078,638 | 22,741,523 → 7,471,529 | 2,847 → 2,847 |
| MozaChloSample/layout | 41,094,962 → 22,860,816 | 36,242,210 → 18,253,707 | 7,620 → 7,620 |
| MozaChloSample/performance | 37,452,024 → 15,516,850 | 32,599,272 → 10,909,741 | 387 → 387 |

## Synthetic scaling and retained limits

- source: 100→1000 notes gives 9.820× work and 9.690× minimum buffered budget for this exact fixture; records 2,174→21,074.
- model: 100→1000 notes gives 9.821× work and 9.688× minimum buffered budget for this exact fixture; records 2,174→21,074.

Unknown/overlapping/reordered conservative fallback and output-size-dependent costs remain explicitly limited. No result closes B5-PERF-01 or establishes unrestricted readiness.

## Admission, lifetime and cleanup verification

- 80 recorded checks at each head; 64 instrumented-versus-public report/error comparisons, 32 at each head; 48 final exact/one-under byte/work/fact checks, plus all intermediate binary-search runs.
- Both live buffered-byte and depth gauges are zero after every instrumented success and refusal. Cumulative work/fact counters are never refunded. A zero live session gauge does not mean a returned caller-owned report occupies no heap.
- Actual code inspection confirms attribute-table scratch remains charged through persistent table construction; merge-sort scratch through final copying; font-category builder through owned publication; performance and repeat indexes through their last provider use; and lyric identity strings receive independent retained ownership before scratch release. Navigation borrowed update indexes remain caller-owned through traversal.
- Exact role dispatch keeps element and attribute axes separate, maintains constant output order, preserves the parent-only direction-type path, propagates no-match refusal, and adds no per-node scratch.
- Focused shared-reader, layout, local, performance, span, repeat, navigation, and independent resource regressions pass. Six frozen pre-context profile identities remain unchanged.
- Go 1.27.2, linux/amd64, GOMAXPROCS=4, package parallelism 1. Real xmllint/libxml2 2.9.14 is installed; this resource harness is not a separate schema-oracle run.
- Root and validation test binaries cross-compile for 386. Actual 386 execution is not passed here (the environment has an established Exec format error). Broad race, whole-repository CI and remote exact-head gates remain separate lead checks.
- Allocation/time values in JSON are single warmed diagnostics, not stable throughput benchmarks or peak heap measurements.

## Reproduction and evidence

The JSON companion contains source hashes, exact heads and production fingerprints, all normalized budgets, all 160 recorded matrix observations, 56 isolated-family observations, and harness/log SHA256 hashes. Raw log and runner hashes identify retained external audit artifacts; those files are not bundled in the repository. The complete normalized observations are included in JSON, and the two exact test harnesses are supplied below for reproduction.

The initial ASCII-only DOCTYPE matcher missed the UTF-16BE MozaChlo declaration. That run is excluded and preserved as a harness diagnostic; the corrected entire baseline matrix was rerun and passed.

The checked-in [resource harness](context-round1-resource-harness_test.go.txt) and
[family harness](context-round1-family-harness_test.go.txt) preserve the audited
source bytes. Copy both to root-package `_test.go` files in a clean worktree at
each stated head before running the matrix command:

    RESOURCE_MINIMUMS=all go test -p 1 -count=1 -timeout 30m . -run '^TestRound1IndependentResourceMatrix$' -v

The original external-DTD source scope is not claimed complete. External resources and physical performance/engraving interpretations remain separate boundaries.
