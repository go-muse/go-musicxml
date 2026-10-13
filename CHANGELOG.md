# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Reduce local-resource snapshot measurement/copy allocations and account owned
  byte slices contiguously; original MozartTrio and MozaChlo source graphs now
  fit the existing default buffer while retaining unread-DTD incompleteness.
  Snapshot-admission failure remains terminal/incomplete and transferred
  snapshot getters explicitly report package ownership.

- Refine the unreleased resource context API: subject declarations now carry
  typed availability/cause evidence, odd schema-hint pairing is known false,
  and snapshot headers identify selection of the assembly-local link plan.
  DTD-wide hint gaps occupy one root row while observed descendant hints remain.

- Add explicit `InspectXMLWithResources` and `AssessModelWithResources` entry
  points with caller-selected resolver dependencies, immutable pin verification,
  exact base/member expansion contexts and one finite shared account. Preserve
  the best independent intrinsic report through later admission failure.
- Add sealed package-role member discovery and metadata-only observed-file
  advice. Exact file observations retain pin/provenance consistency without
  creating parsed resource, ID, role or closure capabilities. Primary retrieval
  names remain distinct from canonical identity and original source history;
  raw non-UTF8 byte names retain optional advice without safe-path admission.

- Add `InspectMXL` and `AssessMXLPackage` for bounded original/current MXL
  assessment with separate container and musical intrinsic reports, exact local
  header evidence, finite package-local graphs, nested archive scopes and shared
  quotas. Current models are assessed directly without encoding or recovering
  discarded source history; outer compressed-rootfile interpretation remains
  explicit.

- Add explicit package-target `ProfileResourceGraph`, `ProfileMXLSource` and
  `ProfileMXLModel` metadata with sealed package/root/link checks, distinct
  capability policy and optional advice, and fixed intrinsic catalog digest
  tables. Preserve all eight earlier profiles and source/model entry-point
  selection. Current-model discarded schema hints remain an explicit selected
  discovery limitation; current packages do not fabricate original ZIP evidence.

- Add source-only ordered container metadata projections through
  `NewContainerMetadataContextPlan` and `ContextResult.Container`. Exact source
  identities, raw/effective token fields, required-attribute causes and guarded
  inventory/order remain distinct. This reuses the assessed source session and
  does not assert package graph rules or alter the eight earlier profile reports.

- Add source-only container and sounds XSD profiles with separately generated
  exact assembly bindings, preserving all eight earlier profiles and digests.
  Sounds adds owned directional mapping context with explicit absence, exact
  string/NMTOKEN domains and causal invalid fields. No Document roots, dictionary
  lookup, unique-primary rule or ordinary Decode behavior changes are introduced.
  `ContextResult` gains the optional `SoundMapping` payload; use keyed literals.

- Add separate sealed package/resource catalogs, one-account typed graph sessions,
  canonical assessment bundles and fresh reference-remapped adoptions. Preserve
  known failures through terminal admission refusal and optional scoped image,
  filename/media-type and mimetype observations without rendering or I/O in core.

- Add alternate terminal `validation.Session.FinishWithResourceSnapshot` with
  immutable intrinsic resource facts, retained ownership and unconditional best
  report preservation on snapshot admission failure. Ordinary `Finish` and
  existing profile semantics remain unchanged.

- Add explicit nonfetching `ProfileSourceLinks` and `ProfileModelLinks`, typed
  owned local resource projections, exact link/bookmark attachment, XLink
  defaults, image/group/instrument metadata and schema-hint discovery pairs.
  Reuse existing XSD/local and Opus prerequisites; preserve all eight older
  profiles/digests and distinguish optional discovery gaps from datatype errors.

- Add explicit `ProfileModelContext` and `ProfileSourceContext`, preserving all
  frozen XSD/local/timeline profiles and XSD-only `Validate`. The new bounded
  providers expose owned repeat/navigation, type-specific span, local default,
  layout and performance context, with exact prerequisite causes and separate
  optional unknowns. Mandatory consumers cover numeric time-only ordering and
  narrowly established bend/repeat content antecedents.
- Add the sealed context plan constructors and `Report.Context` projections,
  including assembly-specific local Opus version and pagination facts without
  following links. External packages/resolvers, dictionaries, disputed normative
  policies and B5-PERF-01 retain their later owners; no universal rendering,
  playback or MusicXML conformance claim is introduced.

- Add explicit `ProfileModelTimeline` and `ProfileSourceTimeline` through a
  separate engine, preserving frozen XSD/local profiles and legacy XSD-only
  `Validate`. The bounded provider retains exact musical chronology and typed
  effective state; selected consumers check principal-note chord duration bounds
  and explicit numeral-key or encoded key at the unshifted harmony score origin.
- Add sealed `NewTimelinePlan`, `NewChordDurationPlan` and `NewNumeralKeyPlan`,
  finalized note/offset/grace/figured-bass/direction context and sparse encoded
  instrument/MIDI statements. Unknown dependencies retain local causal evidence;
  provider completion is not a claim that every fact is known. Publish the full
  clause/requirement ownership ledger without promoting frozen source statuses
  or claiming complete repeat, layout, playback or MusicXML conformance.
- Keep B5-PERF-01 outstanding under shared validation kernel/session ownership
  and final batch-9 reconciliation. New timeline resource improvements do not
  discharge the earlier matched-performance obligation. Final integrated
  acceptance and actual 386 execution remain separately evidenced gates.
- Add immutable `validation.Rational` with bounded canonical fractions, separate
  from XSD decimals, and shared-session metered exact arithmetic for musical
  positions. Arithmetic reserves conservative work and scratch before big-number
  operations and keeps resource exhaustion distinct from invalidity.

- Add direct `AssessModel` with explicit `ProfileModelXSD` and
  `ProfileModelLocal`, plus `ProfileSourceLocal` for the same bounded local
  clauses on original input. Preserve the existing generated `Assess` type.
  Model reports use current field/index evidence without original XML positions;
  finite `float64` decimals mean their exact binary rational value.
- Add generated representation metadata, bounded iterative traversal,
  model-representation facts and sealed local/typed-reference plans sharing
  the authoritative XSD kernel and actual-ID owner resolution. The selected
  scope has 22 local runtime bindings for 19 whole candidate clauses plus one
  local instrument-link split; date and MIDI existence share existing
  consequences. These counts are not conformance coverage.
- Publish the bounded model/local scope and executable-evidence ledger with
  unchanged planning contracts and remaining batches 6–9. Batch-5 combined
  review and aggregate checks passed; [PR #34](https://github.com/go-muse/go-musicxml/pull/34)
  merged as `146400c`, with all nine [postmerge CI jobs](https://github.com/go-muse/go-musicxml/actions/runs/38202157871)
  passing on that exact main commit. This historical checkpoint does not close
  batch-6 acceptance or B5-PERF-01, or claim complete MusicXML conformance.

- Add opt-in single-pass `InspectXML` and `DecodeAssessed` with explicit
  `ProfileSourceXSDPreview`, original-byte locations, bounded retained facts and
  independent source-conformance/model-conversion outcomes. This is a source XSD
  preview, not a complete production MusicXML profile. Unread external DOCTYPE
  subsets prevent source acceptance, including all three committed official score
  samples; strict assessed decoding returns no document for those files. The two
  committed DOCTYPE-free opus examples are within the preview's assessable scope.
- Reuse one XML-neutral authoritative XSD kernel through immutable schema
  snapshots and sealed binding/declaration/attribute/scalar/content/identity
  plans. Composite predicates count once while retaining multiple issue details.
- Add counted fragmented-input source-scanner fuzzing and require raw-source XSD
  oracle tests. Linux CI now executes all packages with `GOARCH=386`.

- Add a bounded `validation` fact-stream foundation with immutable compiled
  catalogs, eight sealed plan families, exact values, finite shared budgets,
  causal context/identity outcomes and independently inspectable reports.
  A separate pinned inspection inventory preserves all 2,560 research records;
  it does not enable a production MusicXML profile or source/model adapter.
- Export `ErrIncompleteAssessment` with identical root/core sentinel identity
  and typed incomplete causes, preserving pure and joined legacy XSD outcomes
  independently of the later direct-model `Validate` migration below.
- Execute `GOARCH=386 go test -count=1 ./validation` in the Linux CI test job
  for actual 32-bit host-width coverage. Add the validation-core fuzz targets
  `FuzzReportFieldIsBoundedEscapedUTF8` and `FuzzSessionLifecycle` to `check-all`,
  each with 10,000 counted executions by default (`VALIDATION_FUZZ_TIME` overrides
  this count).

### Changed

- Migrate legacy `Validate` and root `Validate` methods from Encode/reparse to
  direct current-model XSD assessment. Keep XSD-only scope, ignored `XMLName`,
  nil/unsupported document recognition and legacy cycle/depth sentinels with
  depth 4096. Preserve the legacy size/work contract: the new assessment default
  quotas do not apply to this facade (other quotas use `math.MaxInt64`, with
  overflow and host-allocation checks retained). Memory/work are not bounded by
  practical defaults; use explicit finite `AssessModel` budgets for untrusted
  models. Representation findings
  retain legacy invalid-document recognition; pure unsupported/resource outcomes
  remain incomplete. Decimal edge outcomes intentionally follow current exact
  values rather than serialized spellings. `Decode` and `Encode` stay explicit
  transport operations.
- A proven model representation failure can now join `ErrIncompleteAssessment`
  when dependent XSD work is blocked, even without an independent interruption.
  For example, a NaN measure width matches both `ErrInvalidDocument` and
  `ErrIncompleteAssessment`; the former remains a definitive rejection. This is
  a legacy sentinel behavior change from serialized-value validation. Check
  invalidity before treating incomplete as a reason to retry. Reports retain
  their separate causal and completion axes, including omitted diagnostics.
- Disclose the direct `Validate` performance regression: matched warmed
  MozaChloSample measurements on Go 1.27.2 linux/amd64 have 3.73× the previous
  Encode/reparse time, 2.85× allocated bytes and 1.47× allocation count. Compact
  optional-attribute absence records reduce initial direct-path allocated bytes
  by 26.3%, but do not restore baseline performance. See API budget/performance
  guidance for the fixture, environment, absolute values and limitations.
- Keep the developmental source XSD preview's catalog and behavior separate.
  Source-local adds only the selected musical clauses and does not accept unread
  external DTD subsets, strip DOCTYPEs, fetch resources or claim linked content.

- Route legacy XML reading through the same original-byte scanner. Reject
  undeclared prefixes, illegal reserved namespace bindings, malformed local
  declaration/DTD grammar and contradictory BOM/encoding declarations. Normalize
  literal attribute whitespace separately from character references. DTD
  replacement/default semantics remain unsupported; no external DTD is fetched.
- Add official UTF-8/UTF-16 `Decode` benchmarks and reduce the new scanner's
  transport overhead. Same-host paired medians before the final coordinate
  overflow guard were about 30% / 23% above the previous reader (down from about
  102% / 65%); UTF-16 allocations were about 69% lower. A separate paired run
  measured the final guard at another 2.1% / 4.4% time with unchanged allocations.
  These are fixture/toolchain/host-specific measurements, not throughput
  guarantees; stronger mandatory checks and original-source provenance remain.
- Document the source preview's practical 64 MiB retention cap with measured
  official-fixture boundaries; serialized bytes and node limits do not guarantee
  assessment fits the live-buffer limit. Default limits are unchanged.

- Migrate legacy incomplete-XSD error text from
  `musicxml: incomplete XSD assessment: …` to
  `validation: incomplete assessment: incomplete XSD assessment: …`.
  Recognize this result with `errors.Is`/`errors.As`, not string-prefix matching.
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

- Keep pass and not-applicable assessment outcomes out of stored diagnostics
  and their quotas while retaining counts and compact, separately budgeted
  dependency state. Reports store only fail/unknown/unsupported/blocked findings
  and their rule metadata; successful assessment cannot hide later diagnostics
  or mark diagnostic storage truncated. No all-decisions trace option is added.
- Cache each document rule-prerequisite aggregate lazily after its final phase,
  avoiding repeated full-document scans by its consumers without claiming to
  remove rules-times-nodes dispatch or all retained fact storage.
- Treat valid non-string identity keys as per-rule unsupported evidence and
  keep declaration indexes incomplete rather than inventing missing-reference
  failures. Malformed atomic facts remain terminal contract errors.
- Classify terminal resource/cancellation/deadline, malformed-source and
  capability failures with deterministic precedence and dedicated reason codes,
  preserving joined error causes and leaving policy as an explicit rule category.
- Resolve schema-instance type QNames within captured instance namespace scopes,
  enforce supported type derivation/block/final/abstract contracts, and compose
  actual-type assessment with nil and element value constraints. Missing required
  metadata or ENTITY declaration facts return technical errors without inventing
  document violations; independently proven issues remain inspectable.
- Separate declaration-based identity-table entries from successful actual-ID
  bindings, counting distinct containing elements. Retain dynamic-ID resolution,
  root ownership and local validation gates without mutating source nodes.
- Validate standard schema-location hint datatypes without fetching or selecting
  schemas. Odd list lengths alone remain valid at this fixed-schema layer;
  namespace/location pairing belongs to separately scoped schema discovery.
- Apply the pinned XSD 1.0 datatype-name character repertoire independently of
  XML reading, including Name/NCName/NMTOKEN, list and QName lexical paths.
- Preserve distinct primitive domains in reusable scalar comparisons. Compare
  booleans by truth value, including list/union enumeration and fixed values,
  and apply the XSD whitespace contract to URI fixed values without resolving
  or canonicalizing URLs. Other unverified primitive equality remains limited.
- Match nullable repeated particles and valid repeated-group partitions without
  greedy false rejections. Memoize child-grammar endpoints, reconstruct one
  deterministic declaration witness, and reject malformed/cyclic particle
  metadata without unbounded recursive matching.
- Apply `length`, `minLength` and `maxLength` to item counts for the built-in
  XSD list types `NMTOKENS`, `IDREFS` and `ENTITIES`, including inherited
  restrictions, attributes and complex simple content.

- Compare list and union enumeration and fixed values in the bounded value
  domains of strings, exact integers/decimals and ordered lists of those values.
  Preserve the first successful union member's value and distinguish list shape
  and numeric/string domains. Other primitive families retain lexical equality.

- Expand union-variety members at XSD 1.0 union-composition boundaries, preserving
  member order and atomic/list member restrictions. Direct union restrictions
  and restrictions on unions used as list item types remain operative.

- Treat XSD patterns declared in one restriction as alternatives, while retaining
  intersection with inherited restriction groups. Every local alternative is
  checked for malformed or unsupported patterns, even after a match or miss;
  successful groups still run bounds, length and digit facets. Single-pattern
  diagnostics and the existing regex translator remain unchanged. This is a
  synthetic-schema operator repair; the pinned schemas have no sibling patterns.

- Execute effective XSD `whiteSpace` facets before inherited scalar checks in
  the existing validator. Ordered unions supply the first successful member's
  normalized text; enumeration literals use their declaring restriction's base.
  Atomic string-derived fixed values now use the effective whitespace policy,
  including complex simple content. Source text and schema literals stay intact;
  scalar diagnostics retain source spelling. Pattern alternatives are repaired
  above, alongside bounded aggregate equality; other primitive typed equality
  remains separate work.

- Retain and apply supported scalar restriction facets on complex simple-content
  types, including optional inline scalar bases and inherited restrictions.
  Keep failed or cyclic effective-type resolution from later accepting a partial
  cached type. This is a synthetic-schema operator repair; the pinned MusicXML
  schemas declare no complex simple-content restrictions.

- Apply decimal `totalDigits` and `fractionDigits` to exact values in the existing
  validator, accepting redundant whole/fractional zero padding while retaining
  XSD 1.0 coefficient and scale limits. This synthetic-schema-tested operator
  repair leaves typed transport and the pinned MusicXML schemas unchanged.

- Compare decimal-derived bounds, enumerations and fixed values exactly in the
  existing validator, including alternate equal spellings and values outside
  floating-point precision/range. Typed transport is unchanged.

- Validate integer value spaces and integer-derived bounds, enumeration/fixed
  equality and digit facets exactly, including values beyond Go integer ranges.
  Existing typed Decode representation limits are unchanged.

- Reject references and CDATA outside the XML root across Decode, MXL reads and
  the internal XML parser, including references to whitespace and empty CDATA.
  Legal in-root text, supported encodings and literal boundary whitespace remain
  supported; lexical-origin tracking uses bounded streaming state.

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
