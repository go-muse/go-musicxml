# State measurement boxing

## Scope

Relative to merged main `ca3165e6b087d17e9e5a0c0e1aaf52a0b6365fc5`, the private
state-measurement walker avoids converting unrelated reflected fields to interface
values merely to recognize Integer, Decimal and Rational wrappers. After the
unchanged validity and interface-access checks, only an interface value or a
struct with one of those exact types enters the unchanged dynamic type switch.

Interfaces retain their original dynamic-type handling. True aliases match;
distinct defined types, private fields, pointers and unsupported payloads follow
the original traversal/error path. Every work charge, context poll, traversal,
error order, measured size and ownership rule remains unchanged. No public API,
default, cache or logical accounting change is introduced.

## Test-first and independent evidence

The bounded allocation regression uses 24 addressable nonnumeric records. It
fails on unchanged main at 216 allocations and passes with zero after the guard.
The frozen legacy walker compares 59 fixtures at every measured work limit and
cancellation poll. Additional controls cover direct and interface-held typed nil,
private numeric fields, embedded wrappers, combined quota/cancellation frontiers
and saturated work accounting. Existing ownership/admission controls also pass.

Independent public comparison covers 31 source/model/assessed-decode cases and
four concurrent checks: small, 100/1,000-note and official Mozart inputs; invalid
input; four resource kinds; and cancellation. Unnormalized complete report,
context, typed-error and conversion JSON is byte-identical: 157,119,734 bytes,
SHA256 `39d0a4a299ee4c41db5979c31b0e07b4dc0641abfb1045edc2bff769d76c4087`.
Official Mozart model assessment completes; its source/decode retain their
existing unavailable-DTD partial report. No partial result is promoted to success.
Auxiliary total public context-poll observations vary slightly on unchanged
baseline; deterministic walker-level polls and first-input boundaries are checked
separately. No report fields were normalized away.

## Bounded measurements

On 2026-10-17, Go 1.27.2 linux/amd64 and GOMAXPROCS=1, five alternating baseline/
candidate pairs used the existing ContextReaderReview benchmarks, three iterations
per sample. These benchmarks use explicit finite diagnostic budgets, not changed
defaults. Setup is outside the measured loop. Records and rule steps matched.

- Mozart: cumulative allocated bytes decreased about 3.0%, allocations 11.8%.
- 100 notes: cumulative allocated bytes decreased about 5.1%, allocations 19.5%.
- Timing is inconclusive: paired results have mixed signs and host noise. No
  runtime speedup is claimed, and no reproducible CPU regression was established.

Allocation counts/bytes are cumulative, not peak reachable heap or RSS. An initial
type-only prototype was slower and was rejected. The final Kind prefilter is the
measured patch. An earlier prototype also missed interface-held numeric wrappers;
legacy-reference tests exposed that discrepancy before production integration.

Report backing-array growth, owned context payload copies, snapshot sharing,
lookup memoization and budget synchronization were audited but not changed.
They require different ownership/accounting or cancellation tradeoffs, or lack a
measured material gain. Whole-PR ordinary/race/fuzz acceptance and exact-head CI
are recorded separately in the PR, not inferred from these component checks.
