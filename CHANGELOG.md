# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
