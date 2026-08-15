# Contributing

Contributions are welcome.

## Prerequisites

- Go 1.26 or later
- Git
- Make for the convenience targets, or the equivalent Go commands
- Optional: `xmllint` for running the external XSD conformance test locally

## Local checks

Run the standard checks before opening a pull request:

```bash
make check
```

For release-level verification:

```bash
make release-check
```

The latter requires a clean, committed worktree. It also checks `go mod tidy`
and reproducible generation, uses the race detector, and runs short fuzzing
passes. The external XSD test runs locally when `xmllint` is available; Linux
CI installs it and requires that test to execute. The underlying
`scripts/release-check.sh` is also used by the release workflow on Linux,
macOS, and Windows.

## Generated code

Files named `zz_generated_*.go` are generated from the XSD files in
`schema/musicxml-4.0`.

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
