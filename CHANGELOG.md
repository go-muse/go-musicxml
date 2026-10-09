# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Require Go 1.27 or later.
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
- Fuzz smoke budgets are execution counts for every target. A duration budget
  can fail spuriously with `context deadline exceeded` on Go 1.26
  (go.dev/issue/75804).

### Fixed

- Reject all character data, including XML whitespace, in the internal validator's
  particleless empty complex types. Preserve comments, processing instructions,
  empty CDATA, nil handling and attribute checks. Nullable particles still permit
  XML whitespace; full empty-group normalization remains separate. Public
  `Validate` cannot recover source text discarded during Decode.

- Reject non-XML Unicode whitespace before and after the document root,
  including after an XML declaration, across Decode and MXL reads and the
  internal XML parser. XML whitespace, legal comments and processing
  instructions, and Unicode text inside elements remain supported.

- Reject non-XML Unicode whitespace in element-only complex content in the
  internal XML validator. Only space, tab, carriage return and line feed are
  permitted between children; mixed and simple content retain their existing
  rules. Public `Validate` still assesses the encoded model and cannot recover
  source text discarded during Decode.

- Reject child elements under a fixed-constrained, nonnilled element in the
  internal XML validator, including mixed content whose text matches the fixed
  value. Ordinary type, attribute and identity checks still run. This is a
  synthetic-schema repair; the pinned MusicXML schemas have no element value
  constraints, and public `Validate` still assesses the encoded model.

- Apply element default/fixed values to genuinely empty, nonnilled elements in
  the internal XML validator. Whitespace-only content and true nil do not
  trigger defaulting; source nodes and caller models remain unchanged. The
  pinned MusicXML schemas have no element value constraints, and public
  `Validate` still assesses the encoded model.

- Enforce the `xsi:nil` boolean, nillability, empty-content and fixed-value
  contracts in the internal XML validator. Nilled complex elements still check
  their attributes and identity references. Any `xsi:nil` is rejected on the
  nonnillable MusicXML declarations, including `false` and `0`. Public
  `Validate` still assesses the encoded model; no strict-source API is added.

- Permit the exact standard `xsi:schemaLocation` and
  `xsi:noNamespaceSchemaLocation` names on complex-typed elements in the
  internal XML validator, as already permitted on simple-typed elements.
  Hints do not fetch or replace the pinned schema. Their value semantics,
  `xsi:type` validation and strict-source integration remain
  deferred; public `Validate` still assesses the encoded model.

- Preserve lexical namespace-declaration provenance in the internal XML
  validator, so ordinary attributes bound to the literal URI `xmlns` cannot
  bypass attribute checks. Actual declarations and public Decode behavior
  remain unchanged; public Validate still assesses the encoded model.

- Check ordinary attribute permissibility on simple- and builtin-typed elements
  in the internal XML validator, including `step` and `staves`, without
  rechecking the attributes of complex simple-content bases. Public `Decode`
  remains permissive, and `Validate` still assesses the model after encoding;
  full schema-instance attribute semantics remain deferred.

- Reject misplaced or repeated XML declarations and reserved case variants of
  the exact `xml` processing-instruction target across document and MXL reads.
  First or omitted declarations, encoding signatures and ordinary targets such
  as `xml-stylesheet` remain supported.
- Reject duplicate XML attributes, including namespace-expanded collisions,
  and repeated or misplaced DOCTYPE declarations before model filtering,
  including skipped subtrees, MXL metadata, and linked documents. Legal
  prolog DOCTYPE declarations remain accepted without fetching their DTDs.
- Report these XML guard failures as `*xml.SyntaxError` with the underlying
  decoder's detection line, preserved through existing MXL error wrappers.
- Reject empty prefixed namespace declarations (`xmlns:p=""`) explicitly,
  while preserving legal default namespace resets (`xmlns=""`).

- Resolve namespace prefixes exactly once through XML token adapters, including
  BOM-less UTF-16, so foreign namespace URIs cannot become XML/XLink prefixes.
- Preserve URI segment boundaries while resolving MXL links, including empty
  segments and escaped slashes, to avoid selecting a different archive file.
- Accept RFC 2732 IPv6 literals with zero-padded decimal IPv4 tails during
  XSD anyURI validation.
- Reject unsupported fixed-attribute unions with mixed whitespace policies or
  non-string value spaces instead of generating an incorrect comparison.
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
