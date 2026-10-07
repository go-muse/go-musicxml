# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Renamed the `release-check` Make target and `scripts/release-check.sh` to
  `check-all` and `scripts/check-all.sh`.
- `check-all` compares worktree snapshots around `go mod tidy` and
  `go generate` instead of requiring a clean, committed worktree, prints a
  header before each stage, and accepts stage names to run checks
  individually.
- CI jobs and the Make check targets run stages of `scripts/check-all.sh`
  instead of keeping their own copies of the checks.
- Documented the numeric precision and range limits of the generated model
  and the default linked-document depth limit used by `ResolveOpus`.

### Fixed

- Ignore foreign-namespace lookalikes and namespace declarations when decoding
  MusicXML fields and MXL container metadata, preserving supported XML/XLink
  attributes.
- Avoid overflowing the read ceiling for explicitly configured MaxInt64 MXL
  byte limits, which could silently empty resources.
- Normalize dot segments in archive-root-relative opus links and reject
  directory references instead of resolving them to regular files.
- Reject unrepresentable XML text and root-file metadata before serialization
  or whitespace normalization can replace characters, lose text, or leave
  unreadable archives.
- Apply XSD whitespace normalization in generated fixed-attribute comparisons.
- Check XSD URI lexical forms beyond Go URL parsing, including percent escapes
  in queries and opaque URIs, fragment delimiters, and escaped authorities.
- Encode finite XSD decimal values without exponent notation, including direct
  built-in fields, named derived types, attributes, and simple content.
- Accept legal leading signs for unsigned XSD integer values during XML
  transport and validation, including negative zero for nonnegative values.
- Decode explicitly declared UTF-16BE/LE documents without a byte-order mark.
- Decode UTF-16 documents by their byte-order mark even when the encoding
  declaration names another encoding, instead of re-decoding the text as the
  declared one.
- Normalize XSD whitespace in MXL root-file metadata and opus links before
  resolving paths, while preserving literal resource names and bytes.
- Use the XSD Unicode name and decimal-digit classes in validation patterns.
- Include terminal opus elements when checking the maximum document depth.

## [0.1.0] - 2026-08-15

### Added

- Generated Go model for the official MusicXML 4.0 score and opus XSDs.
- Partwise, timewise, and opus XML decoding and encoding.
- Typed `DecodeScorePartwise`, `DecodeScoreTimewise`, and `DecodeOpusDocument`
  helpers for callers that know the expected MusicXML root type.
- Typed MXL decoding helpers and safe `AsScorePartwise`, `AsScoreTimewise`, and
  `AsOpusDocument` accessors for polymorphic documents.
- Configurable XML-depth and MXL byte limits through `DecodeOptions` and
  `MXLOptions`.
- UTF-8, UTF-16BE/LE, and ISO-8859-1 XML decoding.
- Compressed `.mxl` decoding and encoding with safe ZIP-path and size checks.
- Lossless preservation of MXL rootfile metadata and related resources.
- Typed opus-link resolution, cycle handling, and atomic synchronization.
- Explicit XSD validation with structured issue paths.
- XSD `default` and `fixed` helpers.
- Score, measure, note, and ordered-content construction helpers.
- Official examples, a 150-document interoperability corpus, and fuzz tests.
- Stable end-to-end round trips that compare decoded models and require the
  first and second encodings to be byte-identical.
- GitHub CI across Linux, macOS, and Windows, with race and fuzz checks.
- External libxml2 XSD validation of re-encoded corpus documents in Linux CI.
- Explicit concurrent transport and validation regression coverage.
- A release workflow that validates tags against `main` and publishes release
  notes from this changelog.

### Fixed

- Preserve repeated `key`, `lyric`, `time`, and other grouped child elements
  in typed ordered `Content` slices instead of lossy parallel slices.
- Generate XML fields in schema order and validate content models strictly in
  document order.
- Reject excessive XML nesting before recursive model decoding can exhaust the
  goroutine stack.
- Validate all XSD date/time lexical forms and XML 1.0 Unicode name types.
- Accept the optional UTF-8 byte-order mark.
- Give the MXL fuzz target bounded expansion limits, small focused seeds, and
  a deterministic 10,000-execution CI budget.
- Ignore unsupported child elements consistently during transport, including
  inside generated ordered content, while dropping them on re-encoding.
- Reject configured XML nesting limits above the safe package maximum.
- Apply XSD whitespace normalization to XML whitespace characters only.
- Keep generated ordered `Content` slices valid when callers use
  `encoding/xml` directly and unknown elements are encountered.
- Reject cyclic or excessively deep programmatically constructed opus models
  before encoding, validation, or opus resolution can exhaust the goroutine
  stack.

### Changed

- Added the unambiguous `MusicXMLVersion` constant.
- Removed the unreleased deprecated `Version` alias.
- Namespaced root elements are rejected because MusicXML 4.0 roots are
  unqualified.
- GitHub Actions are pinned to immutable commit SHAs.
- External XSD validation forbids network access, and release metadata checks
  require canonical semantic-version tags and an exact install command.
- Module, generation, formatting, test, and release checks now cover Linux,
  macOS, and Windows; external XSD conformance is enforced in Linux CI.
- The MXL fuzz target includes the realistic compressed corpus fixture and
  accepts inputs up to 64 KiB.

[Unreleased]: https://github.com/go-muse/go-musicxml/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/go-muse/go-musicxml/releases/tag/v0.1.0
