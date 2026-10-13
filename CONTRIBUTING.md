# Contributing

Contributions are welcome.

## Prerequisites

- Go 1.27 or later
- Git
- Python 3.9 or later for the standard-library-only package inventory checks
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
source-reader, direct-model, bounded context-profile, auxiliary-source and two
validation-core fuzz targets each run 10,000 counted executions by default
(`VALIDATION_FUZZ_TIME` overrides this count). The MXL round-trip and raw ZIP
assessment-observer targets each run 10,000 executions (`MXL_FUZZ_TIME`). The Linux
test job also executes
`GOARCH=386 go test -count=1 ./...` to exercise host-width contracts.
The full race stage uses an explicit finite 15-minute per-package timeout; its
CI job retains a 20-minute cap. This allows host-timing headroom for the complete
uncached race suite without skipping tests or changing assessment budgets.

Package obligation inventories also have standard-library Python checks,
required in the Linux CI jobs:

```bash
python3 scripts/verify-package-inventory.py --strict-evidence
python3 scripts/test-package-inventory.py
python3 scripts/test-package-inventory-maintenance.py
```

These verify preserved original records, exact test routes, canonical LF/CRLF
hash equivalence and rejection of real mutations. Existing Linux quality and
release jobs require them without adding a CI job or changing the seven Go
stages. Python is not a library runtime dependency. The required-xmllint job
explicitly includes the original-byte container/sounds differential test.

### Updating inspected test hashes

The inventory deliberately rejects changed original sources and stale inspected
test bytes. After a reviewed edit to one of the routed Go test files, preview the
current test-file hash changes with:

```bash
python3 scripts/refresh-package-inventory.py
```

The default is a read-only dry run. Inspect `changed_test_files` and
`changed_test_routes`, then explicitly apply the same narrow update and verify:

```bash
python3 scripts/refresh-package-inventory.py --write
python3 scripts/verify-package-inventory.py --strict-evidence
python3 scripts/test-package-inventory.py
python3 scripts/test-package-inventory-maintenance.py
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
Canonical record hashes use UTF-8 JSON with sorted keys, `ensure_ascii=False`
and separators `(',', ':')`. Source-file SHA-256 and `bytes` cover the full file
after replacing CRLF with LF only. Review the affected whole-file hash/size
changes alongside the per-ID diffs. Original denominators remain fixed; a new
inventory scope needs a separately reviewed format/denominator change. Run the
strict verifier and both Python control suites after the reviewed reconciliation.

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
