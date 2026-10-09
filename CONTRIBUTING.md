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
for the same stages, and the CI jobs run them from the same script.

## Generated code

Files named `zz_generated_*.go` are generated from XSD inputs. Production files
use `schema/musicxml-4.0`; the test-only `zz_generated_nil_validation_test.go`
uses `testdata/validation/nil-contract.xsd` for nil, element value-constraint and
content-category contracts. `zz_generated_integer_validation_test.go` uses
`testdata/validation/integer-contract.xsd` for exact integer value-space contracts.
Both test schemas use the local empty catalog.

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
