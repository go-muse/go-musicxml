# Contributing

Contributions are welcome.

## Prerequisites

- Go 1.27 or later
- Git
- Make for the convenience targets, or the equivalent Go commands
- Optional: `xmllint` for running the external XSD conformance test locally

## Local checks

Run the standard checks before opening a pull request:

```bash
make check
```

For the full set of code checks:

```bash
make check-all
```

The latter also checks `go mod tidy` and reproducible generation, uses the
race detector, and runs short fuzzing passes. It works on a worktree with
uncommitted changes. The external XSD test runs locally when `xmllint` is
available; Linux CI installs it and requires that test to execute. The
underlying `scripts/check-all.sh` is also used by CI and the release workflow
on Linux, macOS, and Windows. Tag, changelog, and README version checks run
only in the release workflow.

Single stages can be run directly, for example
`bash scripts/check-all.sh mod generate`; the stages are `mod`, `generate`,
`format`, `test`, `vet`, `race`, and `fuzz`. The Make targets `mod-check`,
`generated`, `format-check`, `test`, `vet`, `race`, and `fuzz` are shortcuts
for the same stages, and the CI jobs run them from the same script. The
source-reader, internal-DTD frame reader, direct-model, bounded context-profile,
auxiliary-source and two validation-core fuzz targets each run 10,000 counted
executions by default (`VALIDATION_FUZZ_TIME` overrides this count). The MXL
round-trip and raw ZIP assessment-observer targets each run 10,000 executions
(`MXL_FUZZ_TIME`). The Linux test job also executes
`GOARCH=386 go test -count=1 ./...` to exercise host-width contracts.
The full race stage uses an explicit finite 15-minute per-package timeout.
Local and release checks retain `go test -race -count=1 -timeout=15m ./...`.
CI runs two exhaustive, disjoint race shards on standard Linux runners: the
module root and every other package discovered by `go list -race ./...`.
Each shard retains a 20-minute job cap and the same detector, test and assessment
budgets. Only each CI execution step sets `RACE_SHARD=root` or
`RACE_SHARD=packages`; an unset value runs the full command, and empty or unknown
values fail. Discovery errors, malformed or duplicate inventories, an absent
root, and an empty selected shard also fail before tests start.
The stable `Race detector` check succeeds only when both shards succeed; failed,
cancelled, skipped, or missing shard results cannot pass its aggregate gate.
This deliberately changes CI from nine jobs to eleven without changing the
seven local stages or the release checks.

Package obligation inventories also have standard-library Go checks,
required in the Linux CI jobs:

```bash
go run ./cmd/package-inventory verify --strict-evidence
go run ./cmd/package-inventory test
go test -count=1 ./internal/packageinventory
```

These verify preserved original records, exact test routes, canonical LF/CRLF
hash equivalence and rejection of real mutations. Existing Linux quality and
release jobs require them without adding a CI job or changing the seven Go
stages. No Python installation is needed for development or CI inventory checks.
The required-xmllint job
explicitly includes the original-byte container/sounds differential test.

The compiled command also works without a Go installation: build it with
`go build -o package-inventory ./cmd/package-inventory`, then run
`./package-inventory verify --strict-evidence`. The `verify` and `refresh`
commands accept `--root`, `--manifest`, `--evidence`, `--archive-dir`, and
`--output`; `test` accepts the same flags except `--archive-dir`. Historical
inventory snapshots retain their original command text and observations;
use the Go commands above for current checkouts.

### Compatibility of maintenance output

The Go tools preserve the checked records, hash conventions and result fields,
but do not promise byte-identical Python output. Result object keys are sorted;
failure lists follow deterministic Go key traversal rather than Python document
order. Compare failure content independent of order, preserving multiplicity.
The first malformed-field diagnostic can also differ. The migration's
parsed-output parity evidence covers its enumerated fixtures and mutations,
not every possible malformed input or ordering of failures.

The strict JSON reader rejects the non-standard `NaN`, `Infinity` and
`-Infinity` literals accepted by Python's permissive JSON decoder. Floating
values used in canonical hashes must also have a finite binary64 representation;
an overflowing token such as `1e400` cannot be canonicalized as infinity. An
unused unknown-metadata token is not canonicalized merely by parsing: `1e400`
remains accepted and byte-preserved through a narrow current-hash refresh.
Integer tokens retain arbitrary precision. These are explicit compatibility
differences, not a claim of parity with Python's non-finite extensions.

The line-anchored function-route matcher uses Go's RE2 ASCII whitespace class:
tab, newline, form feed, carriage return and space. Python's additional
vertical-tab and Unicode whitespace matches are not retained. This deliberately
narrowed text check is not a Go parser and does not establish source validity;
the actual Go test run remains a separate requirement.

Parallel test scheduling aims to reduce wall time, not total CPU or memory.
Independent cases retain their own mutable data and assessment accounts, while
only immutable engines are reused. More simultaneous cases and xmllint processes
can increase system CPU and peak memory. Both the whitespace and lexical XML
oracle matrices use the shared parallel helper; Go's `-parallel` limit bounds
their active cases. Compare wall time, CPU and memory separately on the actual
runner rather than extrapolating a many-core local speedup.
Each Linux race shard records its available CPU count and memory, then wraps
its race stage with `/usr/bin/time -v`. User and system CPU can exceed wall time
on multiple cores. Its maximum resident-set measurement is a maximum
child/process observation, not the aggregate peak of simultaneous processes or
a proof of whole-job memory headroom. The wrapper preserves the race command's
exit status; no additional measurement-only run is required. Sharding uses two
runners and can increase total CPU and runner-minutes even when wall time falls.
Compare the elapsed time from workflow creation through completion of the
`Race detector` aggregate, including queueing and setup, rather than the short
gate job's own duration. Local shard timings alone do not establish hosted CI
latency or available concurrency.

A matched local shared-host observation of the pre-sharding tree `5d2264f5`
used Go 1.27.2 on Linux/amd64, `GOMAXPROCS=4`, `-p=4`, and warmed, identical
race binaries. The full race run completed in 390.254 seconds on CPUs 0–3.
Two simultaneous shards, pinned to CPUs 0–3 and 4–7, completed in 309.096 and
218.091 seconds: about 20.8% less
critical-path wall time, with roughly 4.9% more summed CPU and 35.1% more summed
runner occupancy. This used two CPU allocations rather than one; shared-host
load, queueing, setup, and hosted runner availability limit extrapolation.
This external pre-sharding experiment did not run the new shard script. Its
[recorded conditions](docs/validation/evidence/context-race-reliability.md#exhaustive-ci-sharding-observation)
include the unchanged full-suite flags. Hosted end-to-end acceptance for this
sharding change remains pending.

### Updating inspected test hashes

The inventory deliberately rejects changed original sources and stale inspected
test bytes. After a reviewed edit to one of the routed Go test files, preview the
current test-file hash changes with:

```bash
go run ./cmd/package-inventory refresh
```

The default is a read-only dry run. Inspect `changed_test_files` and
`changed_test_routes`, then explicitly apply the same narrow update and verify:

```bash
go run ./cmd/package-inventory refresh --write
go run ./cmd/package-inventory verify --strict-evidence
go run ./cmd/package-inventory test
go test -count=1 ./internal/packageinventory
git diff -- docs/validation/evidence/package-row-evidence.json
```

Commit the reviewed test edit and corresponding current `tests[].file_sha256`
changes together. The refresher first runs the verifier's complete preflight:
original source-file hashes and sizes, canonical record values, statuses,
original/selected denominators, source pointers, five-axis routes and exact named
test functions must remain valid. A missing function, record, source or route
blocks the update. LF and CRLF checkouts have the same hashes; every other byte
change remains significant. The tool only replaces current test hash fields,
including all duplicated routes for the same file. Repeating `--write` with no
changed hashes is a byte-for-byte no-op.

No original record, source-file hash, status, owner, line hint, snapshot,
`historical_tests`, execution result or acceptance claim is refreshed. This does
not execute the Go tests or certify their assertions. Rerun affected tests and
retain new exact-head execution evidence separately before final acceptance.
Result JSON is printed on stdout. Optional `--output PATH` saves an atomic JSON
report; it cannot alias any input, including symlinks or hard links. If malformed
inputs prevent a complete alias check, the failure report stays on stdout.
`--write` also uses a same-directory temporary file and atomic replacement; a
write, close or replacement failure leaves the prior evidence file intact and
reports any temporary-file cleanup failure.

### Reviewing original source-record changes

There is deliberately no blanket source-hash refresh switch. Editing a pinned
planning or ownership source, including whitespace or any status/owner field,
requires a separate reviewed source reconciliation before test hashes can be
refreshed. The review must name each changed original record ID and source
pointer, its old/new values and the reason and review reference. Do not approve
an entire source file or all IDs merely to make CI pass.

After that review, reconcile only those explicit IDs in the additive manifest's
`preserved_collections` and applicable `selected` routes. Retain the old source
and execution evidence; do not relabel old observations as new execution.
Canonical record hashes use compact UTF-8 JSON with sorted keys, no ASCII or
HTML escaping, and lossless integer values. Source-file SHA-256 and `bytes` cover
the full file
after replacing CRLF with LF only. Review the affected whole-file hash/size
changes alongside the per-ID diffs. Original denominators remain fixed; a new
inventory scope needs a separately reviewed format/denominator change. Run the
strict verifier and both Go control suites after the reviewed reconciliation.

## Generated code

Most files named `zz_generated_*.go` are generated from XSD inputs. The exception
is `validation/zz_generated_inventory.go`, an inspection-only provenance artifact
generated by `internal/validationcatalog` from committed `docs/validation/plan`
JSON. Its generator validates local planning integrity and pinned metadata, does
not fetch the research registry, and does not create executable rule bindings.
Change its reviewed inputs/generator and regenerate; never edit the compressed
output or promote planning statuses merely because a helper test passes.

XSD-generated production files use `schema/musicxml-4.0`;
`zz_generated_container_validation.go` and `zz_generated_sounds_validation.go`
provide source-only auxiliary assemblies, without new generated Document roots.
The test-only `zz_generated_nil_validation_test.go`
uses `testdata/validation/nil-contract.xsd` for nil, element value-constraint and
content-category contracts. `zz_generated_integer_validation_test.go` uses
`testdata/validation/integer-contract.xsd` for exact integer value-space contracts.
`zz_generated_decimal_validation_test.go` uses
`testdata/validation/decimal-contract.xsd` for atomic decimal ordering, equality and digit facets.
`zz_generated_simple_content_validation_test.go` uses
`testdata/validation/simple-content-contract.xsd` for complex simple-content
restriction metadata and supported scalar facets.
`zz_generated_whitespace_validation_test.go` uses
`testdata/validation/whitespace-contract.xsd` for effective whitespace, ordered
union lexical views and atomic string-derived fixed comparisons. Its source
matrix retains explicit element-fixed value-space expectations internally;
libxml2's observed lexical comparison excludes only those cases from the oracle.
`zz_generated_pattern_validation_test.go` uses
`testdata/validation/pattern-contract.xsd` for local pattern alternatives,
intersection across restrictions and normalized lexical subjects. Its exact-source
matrix compares internal assessment and xmllint on the same bytes; the single
numeric element-fixed lexical discrepancy remains an internal normative case.
Malformed and translator-unsupported patterns use separate metadata tests rather
than weakening the valid-schema oracle. `zz_generated_builtin_list_validation_test.go` uses
`testdata/validation/builtin-list-contract.xsd` for built-in list item-count facets.
Its declared unparsed entities expose a contextual assessment limitation: the
current source representation does not retain those declarations. Lexically valid
`ENTITY`/`ENTITIES` cases are now explicitly incomplete, rather than valid or
invalid; datatype-only helpers remain lexical. These cases are excluded from
complete-assessment parity claims. `zz_generated_aggregate_validation_test.go`
uses `testdata/validation/aggregate-contract.xsd` for list/union enumeration and
fixed comparisons in exact numeric and string-derived domains. Boolean equality and primitive domain identity are covered by the later
primitive fixture. Other same-primitive equality limitations remain documented. Concrete libxml2 fixed and singleton-list discrepancies are documented in the
code map;
empty-list enumeration is metadata-only because libxml2 cannot compile it.
`zz_generated_union_member_validation_test.go` uses
`testdata/validation/union-member-contract.xsd` for XSD 1.0 union-member
flattening and its direct-restriction, list-item and atomic/list-member controls.
`zz_generated_primitive_validation_test.go` uses
`testdata/validation/primitive-value-contract.xsd` for boolean/list/union value
identity, fixed-value equality and URI whitespace. Six explicit element-fixed
alternate-spelling cases remain internal because libxml2 compares them lexically;
the same-byte external subset retains attribute-fixed and negative controls.
`zz_generated_datatype_name_validation_test.go` uses
`testdata/validation/datatype-name-contract.xsd` for XSD 1.0 Second Edition
name-character lexical contracts, including restricted, list, union and attribute
paths. Contextually reached, lexically valid ENTITY/ENTITIES cases explicitly
report incomplete assessment and are excluded from complete-outcome parity;
standalone lexical checks remain covered. General QName/NOTATION context is
separate from the later xsi:type-specific binding tests.
`zz_generated_grammar_validation_test.go` uses
`testdata/validation/grammar-contract.xsd` for nullable and repeated particle
endpoints, deterministic declaration witnesses and XSD 1.0 all-group controls.
Its same-byte external test is required in Linux CI. Huge occurrence bounds and
malformed/cyclic metadata are separate internal tests, not external-schema
parity claims.
`zz_generated_xsi_validation_test.go` and
`zz_generated_xsi_defaults_validation_test.go` use `xsi-type-contract.xsd` and
`xsi-defaults-contract.xsd` for namespace-aware actual type selection, derivation,
block/final/abstract metadata, canonical dynamic defaults, and independent
attribute/content checks. `TestXSITypeAgainstSchema` executes internal assertions
for every case and external checks for the comparable cases; named libxml2
QName-whitespace, canonical-default/fixed and IDREF divergences retain exact
source evidence plus supplemental JAXP results.
`zz_generated_schema_hint_validation_test.go` uses
`testdata/validation/schema-hint-contract.xsd` for ordinary attribute carriers
of the standard hint datatypes: facet-free list(anyURI) and anyURI. Same-byte
carrier comparisons supply lexical evidence because libxml2 fixed-schema
validation does not assess malformed standard hint values. Exact URI-parser
exclusions and the separately retained discovery-pairing obligation are in the
code map. `zz_generated_identity_validation_test.go` uses
`testdata/validation/identity-contract.xsd` for XSD 1.0 declaration-seeded ID
entries, actual-type binding and distinct containing-element ownership. The
same-byte oracle root runs in Linux CI; documented identity-table divergences
retain the pinned-version expectations. The selected-member interpretation stays
explicitly bounded in the scope ledger rather than proving generic closure. All
test schemas use the local empty catalog.

Do not edit generated files directly. Change the schema generator under
`internal/xsdgen`, its configuration in `generate.go`, or the schema inputs,
then run:

```bash
go generate ./...
go fmt ./...
```

Commit the generator change, regenerated output, and focused tests together.
CI rejects non-reproducible generated files.

## Tests

- Add table-driven unit tests for focused behavior.
- Add round-trip tests for model or transport changes.
- Add malformed input tests for parser and archive safety changes.
- Add a fuzz seed only when it represents a useful structure or a regression.
- Preserve upstream notices for any added fixture.

Use `require` only when continuing the test would be impossible or
meaningless; otherwise prefer `assert` so one run can report more failures.

## Public API changes

Document every exported declaration. Explain compatibility consequences in the
pull request and update `API.md` and `CHANGELOG.md` when the public contract
changes.
