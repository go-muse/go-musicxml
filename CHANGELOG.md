# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Organize loading and checking around three scenarios (see API.md,
  "Scenarios"): `Decode`/`DecodeMXL` without validation; strict loading with
  `DecodeAssessed` and the new `DecodeMXLAssessed`; and model checks with
  `AssessModel`, on the primary document for an MXL package. With unexhausted
  budgets, `AssessModel` on a document from `DecodeAssessed` gives the same
  report as its model stage.
- Add `DecodeAssessmentOptions.Lenient` and `DecodeResult.Accepted`. The
  lenient mode returns the converted document with both reports when an
  assessment finds violations or is incomplete, with a nil error; operational
  failures still return an error.
- Add `DecodeMXLAssessed`, `DecodeMXLAssessmentOptions` and `DecodeMXLResult`.
  It assesses the original archive with `ProfileMXLSourceSemantics`, converts
  it and assesses the primary document with `ProfileModelSemantics`, with one
  shared budget account and a lenient mode.
- Add `AssessmentOptions.Dictionaries`, used directly by `InspectXML`,
  `AssessModel` and `DecodeAssessed` and as `Graph.Dictionaries` by the
  resource and MXL entry points.
- Add `ValidationIssue.Rule`, the ID of the violated rule.
- Add `ErrInvalidConfiguration` (identical to
  `validation.ErrInvalidConfiguration`), which `ErrInvalidDecodeOptions`,
  `ErrInvalidEncodeOptions` and `ErrInvalidMXLOptions` now match.
- Add `DictionarySourceFunc`, `validation.ResourceResolverFunc`,
  `validation.Known`, `validation.Unknown` and `validation.Report.String`.
- Bundle the official MusicXML 4.0 DTD modules in `schema/musicxml-4.0` and
  resolve DOCTYPE external subsets from them, never from the network: the
  official catalog's public identifiers of MusicXML 1.0 through 4.0 and the
  official `musicxml.org/dtds/` system identifiers select the pinned 4.0 files.
  Ordinary decoding uses a resolved subset only for one-character general
  entities, such as `&eacute;`, and defaulted namespace declarations, such as
  `xmlns:xlink`, normalized as attribute values; other DTD defaults are not
  added to the model. Files that
  decoded before decode to the same tokens, except that a DTD namespace default
  now applies even where an ancestor binds the same prefix to another
  namespace.
- Add `DTDSource`, `DTDSourceFunc`, `DTDRequest`, `DTDOptions`, `DTDError`,
  `ErrDTDNotFound` and `ErrDTDUnavailable`, with `DTD` fields on
  `DecodeOptions`, `MXLOptions` and `AssessmentOptions`, so callers can supply
  their own DTD bytes ahead of the bundled DTDs. `DecodeOptions.DTD` takes
  precedence over `AssessmentOptions.DTD` in assessed decoding, and
  `MXLAssessmentOptions.Graph.DTD` over `MXLAssessmentOptions.MXL.DTD`.
- Add `validation.CoordinateSpaceOriginalBytes` and
  `validation.CoordinateSpaceExternalDTD`. Source origins and entity declaration
  facts locate a declaration read from an external DTD entity by its system
  identifier in `Location.Entry`.

- Add `DecodeContext`, `DecodeMXLContext` and `DecodeMXLPackageContext`. They
  decode like the `WithOptions` variants with cooperative cancellation: the
  context is checked between reads, while MXL entries are expanded and before
  a result is returned, and cancellation errors match `ctx.Err()`. A blocked
  `Read` is not interrupted. Existing decoders keep using
  `context.Background()` and are unchanged.
- Add `EncodeWithOptions` and `EncodeOptions` for an XML declaration and
  indented output with the same cycle, nesting and XML-text checks as
  `Encode`. Zero options write exactly the bytes of `Encode`. Invalid options
  match the new `ErrInvalidEncodeOptions`.
- Add `MXLPackage.ResolveOpusWithOptions`, which applies the caller's
  `DecodeOptions` to linked documents. `ResolveOpus` keeps the default limit.
- Add `ErrUnsupportedXMLFeature`, matched by decode errors for well-formed
  input that relies on an unimplemented XML feature such as DTD entity
  expansion or an unavailable encoding. These errors still match
  `ErrIncompleteAssessment`.
- Add `validation.Rational.Cmp` and `Equal` methods on `validation.Rational`,
  `Decimal` and `Integer`. These types compile with `==` but compare internal
  pointers.
- Add `validation.SchemaXSDNamespace` and untyped constants for the string
  vocabulary of the `validation.Schema*` metadata aliases.
- Add `TestPublicAPISnapshot` and the golden files in `testdata/api/`, which
  record the exported API of the root and `validation` packages, including
  generated struct fields with their XML tags, comparability and method sets.
- Document the compatibility policy for generated model types, comparable
  option structs and caller-implemented interfaces in `API.md`, API levels in
  the `validation` package documentation, and API conventions and the
  snapshot workflow in `CONTRIBUTING.md`.

- Add `validation.Session.FinishWithRetainedReport` and `ReportLease.Close` for
  explicit ownership of already admitted report context across assessment stages.
  The assessed loaders use this bounded accounting seam; report data remains
  usable after close and existing resource-snapshot APIs are unchanged.

- Add `BundledDictionarySource()` for the existing pinned SMuFL, MusicXML 4.0
  standard-sounds and IPA-derived components. Explicit offline generation
  prepares committed local assets embedded at build time; callers need no data
  files or runtime fetch. Explicit selections, profiles, explicit nil-source
  behavior, importer accounting and bounded evidence semantics remain unchanged;
  see the [assessed-loading migration][assessed-loading] for its wrapper defaults.
  Referencing the bundled source or either assessed loader embeds all five
  components regardless of options, including the separately licensed IPA data. Binary distributions
  containing those data must include the five notice/license files listed in
  [the distribution instructions][bundled-data-notices].

- Add explicit encoded semantic source/model and graph/raw/current-MXL profiles
  through existing assessment APIs. They compose exact intrinsic interpretations,
  bounded barline/repeat evidence, selected dictionary capabilities and the
  registered XML Framework/shorthand/element fragment baseline. Source DTD and
  current-model identity remain distinct; no automatic data fetch or external
  musical-intent authority is added.

- Add an explicitly selected IPA2015/Unicode13 representation manifest with
  295 complete encoding atoms, 158 genuine canonical relations and a separately
  pinned minimal policy. Preserve the legacy 197-row manifest and its outputs.
  Reuse the existing typed payload and adapters, permit five fixed selections,
  and publish a whole-input raw span under a new context property. Unproved
  inputs remain unavailable; this is not phonotactics or universal conformance.
  Deliberately revise the IPA consumer/rule and dictionary-containing catalogs.
  New-manifest malformed bundles remain configuration errors; legacy and caller
  callback error classifications are unchanged.

- Add explicit internal-DTD/dictionary graph and raw/current-MXL compositions in
  separate catalogs. Identity preservation excludes the separately documented
  figure-number, IPA and mimetype local-header migrations. Add source
  Container and Sounds compositions with exact auxiliary origin routing,
  physical Container Raw versus effective values, effective Sounds mappings,
  and nonfetching source hints. Current primary models retain model-only
  evidence; retained source snapshots preserve independently rebound origins.
  Existing dictionary APIs require the exact compatible profile and retain all
  package, indirection, unread-external and incomplete-capability gates.

### Fixed

- `UnsupportedRootError` and `MXLLinkError` no longer panic when `Error` or
  `Unwrap` is called on a nil receiver, matching the package's other error
  types.
- Correct `API.md` statements about XSD choices (only repeated choices use
  wrapper structs) and enumerations (anonymous enumerations have no
  constants), and list the remaining differences of direct `encoding/xml` use.

- Avoid boxing nonnumeric reflected state fields solely to recognize immutable
  numeric wrappers. Preserve exact measurement, traversal, work charges and
  cancellation/error behavior; see [the allocation receipt](docs/validation/state-measure-boxing.md).

- Reuse private per-invocation dictionary hash scratch buffers instead of
  allocating them for every encoded field. Preserve hash bytes, write chunks,
  budget charges, cancellation checks, dictionary identities and public APIs.
  See [the bounded measurement receipt](docs/validation/dictionary-hash-scratch.md).

- Reduce bounded-assessment storage amplification and repeated work: compact
  private node facts, share exact session-owned evidence and final role caches,
  admit DFS scratch by live capacity, build exact-size XSD witness vectors,
  release temporary attribute indexes, and meter actual state projections.
  Public loading behavior, selected checks and default limits are unchanged.
  Retained-container growth now also charges actual copy work against
  `MaxRuleSteps`; a previously just-sufficient explicit allowance can refuse
  even for the same input (reviewer XSD/local controls observed +0.6–1.1%).
  Compiled type/use identities each add one compile record and 64 logical
  bytes, so custom schemas near fixed compile/catalog limits can also refuse.
  The temporary schema attribute index now admits its header/first group
  and conservative entry storage before allocating, rather than undercounting
  small maps; near-limit explicit buffer allowances may refuse sooner.
  Callers with tuned explicit limits should revalidate them and allow headroom.
  Resource/cancellation checkpoints can change as more required work is reached;
  large-model default assessments still have explicit resource ceilings. See
  [the corpus correction scope and residual limitations](docs/validation/corpus-resource-fixes.md).

- Tighten source-origin snapshot accounting without raising defaults. Capacity
  remains limited by two owned origin copies and overlapping session storage:
  the historical amd64 boundary of 531 measures (71,197 bytes) admitted and
  532 (71,331 bytes) refused was superseded by compact retained facts. The
  initial PR 43 checkpoint `c0d73972` admits 568 measures (76,155 bytes),
  then refuses 569 (76,289 bytes): 42,495,274 live bytes plus a 24,717,911-byte
  request. Review corrections further reduce repeated role-cache storage: the
  same fixture now completes at 576 measures (77,227 bytes) and refuses 577
  (77,361 bytes), at 42,053,858 live bytes plus a 25,064,439-byte request.
  The earlier reported boundary was 494 measures (about 66 KB).
  The review reported the 500-measure case at
  about 62.9 of 67.1 MB, with comparisons of about 114 KB for standalone DTD/dictionary
  assessment and 168 KB for the legacy graph. These are fixture-specific
  observations, not general document-size guarantees. The 86,052-byte,
  2,000-empty-measure fragment fixture still refuses the default buffer;
  its 512 MiB test isolates the default rule-step limit only. See the
  [capacity receipt](docs/validation/resource-fragment-budget-evidence.md#default-buffer-capacity-is-fixture-specific).

- Correct `MX40-PACKAGE-mimetype-no-extra` for `ProfileResourceGraph`,
  `ProfileMXLSource`, `ProfileMXLModel` and their dictionary variants: a proved
  zero local extra now passes with a framed central-only extra, instead of
  remaining Unknown. Nonzero local extra still fails. Malformed central extra
  framing now stops package/graph enumeration and child assessment with a
  transport-capability error, while preserving an already-established local
  Pass/Fail. No Container or primary-resource check is promised after that
  refusal. See the [package scope](docs/validation/package-profile-scope.md) and
  [six-profile public catalog migration](docs/validation/ipa-representation.md#published-package-baseline).

- Correct MIME classification in those package/graph profiles. Distinct valid
  unknown parameters, such as `application/vnd.recordare.musicxml+xml; foo=bar`,
  now preserve the recognized base type: first-media-type and file-metadata
  checks can pass instead of reporting `musicxml-media-parameter-unresolved`.
  Equal repeated parameters and trailing semicolons now remain Unknown with
  `media-type-duplicate-parameter` and `media-type-syntax-unresolved`, respectively,
  so formerly satisfied packages can become undetermined. Conflicting repeated
  parameters also use the duplicate reason. For compressed rootfiles with a
  charset parameter, metadata advice now reports `nonstandard-file-media-type`
  instead of `nonstandard-file-suffix`. The parser remains authoritative for
  affirmative classification; media declarations do not certify file contents.

- Resolve `ProfileSourceLinks` / `ProfileModelLinks` link and bookmark ordinals
  across matching descendants with different parents. These cases now have
  Known attachment instead of Unknown `link-descendant-ordinal-interpretation`.
  The local-link rule revision and CatalogDigest are unchanged; pin the library
  revision when reproducing historical output. See the
  [behavior correction note](docs/validation/local-link-context-scope.md).

- Stop formatting carry between `direction-type` wrappers in
  `ProfileSourceContext` / `ProfileModelContext`. Each wrapper now uses its own
  values and defaults, including `application-format-choice` where appropriate,
  instead of inherited wrapper values or `direction-cross-wrapper-format-precedence`.
  The affected rule revisions are unchanged; the context digest migration below
  identifies the separate figure-number removal.

- Return Known numeral-root suppression advice for a harmony chord's own
  `kind=none` in `ProfileSourceContext` / `ProfileModelContext`, instead of
  Unsupported advice. Stacked chord branches retain separate associations.
  This correction likewise does not revise the affected context rule key.

- Preserve original malformed-XML failures for `ProfileResourceGraph`,
  `ProfileMXLSource` and `ProfileMXLModel` when a primary, nested rootfile or
  external child fails before its assembly is known. The error retains the
  XML syntax cause and `SyntaxMalformed` failure report without adding the
  former `invalid configuration: resource transfer: established intrinsic
  assembly required` error. The bundle remains unsatisfied.

- Remove the invented figure-number numeric lexical requirement from the built-in
  source/model context profiles. MusicXML deliberately retains unrestricted
  style-text; nondecimal figures no longer make assessment incomplete, while
  independent XSD failures remain. Keep the public numeric-subset plan as
  deprecated legacy compatibility. Record the deliberate context-profile
  rule/digest/outcome migration without changing historical snapshots.

- Classify dictionary provider, response, transport and verification failures as
  runtime dictionary unavailability, exposing `ErrDictionaryUnavailable` and
  `DictionaryImportError` with preserved original causes. Configuration misuse
  retains its separate contract; cancellation/resource failures remain terminal.

- Pin the reviewed IPA mapping-aid omissions and clarify historical license-page
  byte observations and effective license URLs without changing runtime data.
  Distributed dictionary attributions are nonprovisional notices.

- Classify EOF inside unfinished XML markup as a syntax error in the shared
  source reader, preserving the cause through ordinary and assessed decode and
  rejecting truncated processing instructions after an otherwise complete root.

- Meter ZIP reads, decompression, CRC and actual buffer relocation in bounded
  4096-byte work units, preserving exact byte quotas and simultaneous live-buffer
  peaks without changing defaults.

- Frame ZIP end records before classifying unsupported envelope features; do not
  mistake central-comment locator bytes for ZIP64 records. Preserve independent
  non-framing header flags and legacy unsafe-path/duplicate-name error identities.

- Document symbolic-link refusal as an assessment transport limitation and UTF8Name
  as observed byte validity with independently retained EFS flags.

- Preserve known-false schema-hint pairing as a blocking capability-policy failure
  with its exact context cause, without making it a normative XSD violation.
  Snapshot link-selection witnesses require an available sealed local-link plan
  for the established assembly; selection remains separate from scope completion.

### Added

- Add explicit source-only DTD/local, DTD/link and DTD/dictionary compositions
  with owned document ENTITY/ENTITIES declaration evidence. Separate schema
  operands from instance membership, and physical attribute provenance from
  effective DTD values. Add sealed source-mode constructors and bounded fact
  ingress while preserving existing profile/catalog identities. Unresolved
  constructed carriers and package/source-origin transport remain explicit
  limitations. See the [scope](docs/validation/source-declarations-scope.md) and
  [verification checkpoint](docs/validation/source-declarations-evidence.md).

- Add opt-in pinned dictionary assessment for original XML, current models,
  selected resource graphs and original/current MXL packages. Six explicit new
  profiles share one verified immutable dictionary set and finite root account;
  old profiles, option shapes and default behavior remain unchanged. The
  source-sounds intrinsic row admits compatible explicitly supplied snapshots;
  public graph/MXL wrappers do not discover or assess sounds children. Standalone
  dictionary XML inspection supports explicit sounds input.

- Add exact SMuFL 1.4 canonical-name evidence alongside independent MusicXML
  family facets, narrow rendering/default advice, open-string MusicXML 4.0
  standard-sound recognition and explicitly partial IPA 2015/Unicode 13 positive
  evidence. Full IPA repertoire/sequence validity and general glyph semantics
  remain unavailable; no dictionary, font or DTD is fetched implicitly.

- Add sealed component certificates, typed `DictionarySet` construction and
  same-account `Engine.BeginWithDictionaries` leases with concurrent-safe owner
  close, bounded caller-supplied transport and owned occurrence projections.
  Raw-source certification remains distinct from reviewed-derived IPA lineage.

- Add `ContextResult.Dictionary` with `omitempty`. External unkeyed literals of
  `ContextResult` must migrate to keyed literals; old nil-payload report JSON is
  unchanged. Dictionary data retain their separately documented upstream licenses.
  See the [scope](docs/validation/pinned-dictionaries-scope.md) and
  [verification checkpoint](docs/validation/pinned-dictionaries-evidence.md);
  these additions do not claim final integrated or release acceptance.

- `ErrEntityNestingLimit` identifies the bounded internal-DTD named-entity
  nesting refusal while preserving its typed resource error.

- Add developmental `ProfileSourceInternalDTD` for bounded original-source
  internal declarations, entity replacement, attribute defaults and non-CDATA
  normalization. Expose typed occurrence origins through
  `SourceOriginContextResult`, `SourceOriginStep`, `SourceOriginKind`,
  `NewSourceOriginPlan`, `Engine.SourceOriginProvider`, `Session.SourceOrigin`
  and `Session.CompleteSourceOrigins`. External DTDs/entities remain unfetched;
  this profile does not establish general XML/DTD or MusicXML conformance.

- Reduce local-resource snapshot measurement/copy allocations and account owned
  byte slices contiguously; original MozartTrio and MozaChlo source graphs now
  fit the existing default buffer while retaining unread-DTD incompleteness.
  Snapshot-admission failure remains terminal/incomplete and transferred
  snapshot getters explicitly report package ownership.

- Refine the unreleased resource context API: subject declarations now carry
  typed availability/cause evidence, odd schema-hint pairing is known false,
  and snapshot headers identify selection of the assembly-local link plan.
  DTD-wide hint gaps occupy one root row while observed descendant hints remain.

- Tighten package failure and ownership contracts: require established intrinsic
  assemblies, keep the root as its exact context owner, and reject contradictory
  failure metadata or duplicate transferred assessments. Post-terminal failure
  summaries retain bounded original facts without admitting new work.

- Add a processing-policy error marker for adapter refusals, preserving existing
  error identities and distinguishing policy incompleteness from normative failure.

- Preserve the first omitted blocking package diagnostic in a fixed prepaid
  summary independent of shared diagnostic quotas. Expose read-only package
  terminal causes and retain original intrinsic terminal classifications.

- Add one bounded declared-MIME classifier shared by package checks and adapters,
  with transient parsing scratch, case-insensitive registered types and explicit
  unknown parameter policy. Binary members do not receive MusicXML suffix advice.

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
  discarded source history; outer compressed-rootfile and linked-target
  interpretation remains explicit. Incoming archive-to-primary substitutions
  retain their own uncertainty without discarding inner XML reports.

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
  `ContextResult` gains three optional payloads: `SoundMapping`, `Resource` and
  `Container`. Migrate unkeyed composite literals to keyed literals.

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

- Add single-pass original-source inspection with `InspectXML` and explicit
  `ProfileSourceXSDPreview`, original-byte locations and bounded retained facts.
  `DecodeAssessed` uses the same parser ownership but now fixes the semantic
  source/model pipeline described below; it does not accept the preview profile.
  The source-XSD preview is not a complete production MusicXML profile. Unread external DOCTYPE
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

- An empty `Profile` now selects the full semantic profile, with the bundled
  required dictionaries, instead of returning a configuration error:
  `ProfileSourceSemantics` for `InspectXML`, `ProfileModelSemantics` for
  `AssessModel`, `ProfileResourceGraphSemantics` for `InspectXMLWithResources`
  and `AssessModelWithResources`, `ProfileMXLSourceSemantics` for `InspectMXL`
  and `ProfileMXLModelSemantics` for `AssessMXLPackage`. Binaries referencing
  these functions now embed the bundled dictionary data and must ship its
  notices.
- `ValidationIssue` from `DecodeAssessed` follows `Validate`: `Constraint` is
  the reason code, `Message` the offending value and `Rule` the rule ID.
  Previously `Constraint` held the rule ID and `Message` the reason code.
  `ValidationIssue.String` shows the constraint when the message is empty.
- New trailing fields break unkeyed composite literals of `AssessmentOptions`,
  `DecodeAssessmentOptions`, `DecodeResult` and `ValidationIssue`; keyed
  literals and `==` are unaffected.

- Source profiles that read DTD declarations (the `ProfileSourceInternalDTD`
  family, `ProfileSourceSemantics` and the internal-DTD and semantic package
  profiles) now read a recognized DOCTYPE external subset, with parameter
  entities from the internal subset in scope. A caller-supplied subset is
  parsed under the assessment's budgets; the bundled closure is checked
  against `MaxScalarBytes` and `MaxDepth` when imported. Attribute-default
  well-formedness constraints, including declaration order across both
  subsets, apply to every external definition, and external defects are
  `*DTDError`s. Declarations generated by parameter entities are located
  where their text is written. Measured for this change, default
  `DecodeAssessed` accepts 140 of the 154 score and opus files in
  `testdata/official` and the MusicXML test suite instead of 2; larger files can
  now reach resource limits where they previously stopped at unread-DTD
  evidence. Their profile closures and catalog digests change accordingly, and
  the longer closure texts move the documented amd64 graph buffer witnesses by
  202 live and 104 requested bytes;
  profiles that do not read DTDs keep their digests and still report external
  subsets as unread.
- With a resolved external subset, ordinary decoding reports a namespace prefix
  that neither the document nor the DTD binds as an `*xml.SyntaxError`, as
  without a DOCTYPE, instead of `ErrUnsupportedXMLFeature`.
- New trailing `DTD` fields break unkeyed composite literals of
  `DecodeOptions`, `MXLOptions` and `AssessmentOptions`; keyed literals and `==`
  are unaffected.

- **Breaking relative to the unreleased development API, not v0.1.0:**
  `DecodeAssessed` and `DecodeAssessedWithDictionaries` always require
  the existing `ProfileSourceSemantics` followed by direct
  `ProfileModelSemantics`. Nil/empty assessment options use these fixed defaults;
  narrower, wrong-target and unknown profiles fail before I/O. `DecodeResult`
  adds an independent `Model` report; source, conversion, model, operation or
  close failure withholds `Document`. Replace external unkeyed `DecodeResult`
  literals with keyed literals. Neither loader can accept an unread external
  DOCTYPE subset, including with default options. Callers needing the old
  nil-assessment conversion behavior must use `Decode`/`DecodeWithOptions`; these
  are transport operations, not validation substitutes. On the asserted
  linux/amd64 control, the 223,068-byte original MozaChlo fixture reaches the
  64 MiB default buffer limit before EOF. The reviewer also reported a refusal
  for 94,916-byte MozartTrio after EOF and conversion. These are
  architecture-dependent capacity receipts; the 386 run does not assert them.
  Resource refusal remains incomplete assessment, not normative invalidity.
- The assessed loaders default to bundled canonical SMuFL and current IPA
  representation evidence. Explicit dictionary overrides must include both
  required relations and never silently fall back. A single finite account and
  imported dictionary set cover source and model stages; no reader replay, model
  re-encoding, external DTD fetch, profile widening or budget increase is added.
  Inspection, standalone model assessment and package APIs retain their existing
  explicit contracts. See the [migration guide](docs/validation/assessed-loading-pipeline.md),
  including the external-DOCTYPE compatibility impact. Earlier receipts and
  profile identity records remain historical evidence for their stated revisions.
- Any binary referencing either assessed loader now embeds all five bundled
  dictionary components, even with custom-source options. Distributors must ship
  the five [notice/license files][bundled-data-notices], including the CC BY-SA
  4.0 IPA-derived data and Unicode License v3 notices.
- Each default load re-imports and verifies both required bundled components
  before reading the document; there is no cross-call verified-set cache. At
  reviewed head `7e7126e`, the reviewer reported about 3.08 ms and 1.55 MB allocated
  for import alone, plus logical charges of 3,279,308 rule steps and 360,428
  `MaxExternalBytes`. These are reviewer-reported observations, not independent
  benchmarks or portable minima. Previously sufficient budgets can now refuse
  before source reads. See the [cost and budget disclosure][loading-cost].

- Add optional `ContextResult.SourceOrigin` for selected source-origin providers.
  External unkeyed `ContextResult` literals must migrate to keyed literals.
  Its `omitempty` tag preserves existing profile JSON when the payload is absent.

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

### Deprecated

These wrappers keep their behaviour, apart from the `ValidationIssue` fields
that `DecodeAssessedWithDictionaries` shares with `DecodeAssessed`, and will be
removed in the next minor release. An empty profile still fails there. The
replacement gives the same result for every profile the wrapper accepts:

| Deprecated | Replacement |
| --- | --- |
| `InspectXMLWithDictionaries(ctx, r, DictionaryAssessmentOptions{Assessment: a, Dictionaries: d})` | `InspectXML(ctx, r, a)` with `a.Dictionaries = &d` |
| `AssessModelWithDictionaries(ctx, doc, DictionaryAssessmentOptions{Assessment: a, Dictionaries: d})` | `AssessModel(ctx, doc, a)` with `a.Dictionaries = &d` |
| `DecodeAssessedWithDictionaries(ctx, r, DecodeDictionaryAssessmentOptions{Decode: o, Assessment: &DictionaryAssessmentOptions{Assessment: a, Dictionaries: d}})` | `DecodeAssessed(ctx, r, DecodeAssessmentOptions{Decode: o, Assessment: &a})` with `a.Dictionaries = &d` |
| `InspectXMLWithResourcesAndDictionaries(ctx, r, LinkedDictionaryAssessmentOptions{Resources: l, Dictionaries: d})` | `InspectXMLWithResources(ctx, r, l)` with `l.Graph.Dictionaries = &d` |
| `AssessModelWithResourcesAndDictionaries(ctx, doc, LinkedDictionaryAssessmentOptions{Resources: l, Dictionaries: d})` | `AssessModelWithResources(ctx, doc, l)` with `l.Graph.Dictionaries = &d` |
| `InspectMXLWithDictionaries(ctx, r, MXLDictionaryAssessmentOptions{Resources: m, Dictionaries: d})` | `InspectMXL(ctx, r, m)` with `m.Graph.Dictionaries = &d` |
| `AssessMXLPackageWithDictionaries(ctx, p, MXLDictionaryAssessmentOptions{Resources: m, Dictionaries: d})` | `AssessMXLPackage(ctx, p, m)` with `m.Graph.Dictionaries = &d` |

The option types `DictionaryAssessmentOptions`,
`DecodeDictionaryAssessmentOptions`, `LinkedDictionaryAssessmentOptions` and
`MXLDictionaryAssessmentOptions` are deprecated with them.

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

[bundled-data-notices]: THIRD_PARTY_NOTICES.md#embedded-copies-of-the-pinned-dictionary-data

[assessed-loading]: docs/validation/assessed-loading-pipeline.md
[loading-cost]: docs/validation/assessed-loading-pipeline.md#per-call-import-cost-and-pre-input-budget-floor
