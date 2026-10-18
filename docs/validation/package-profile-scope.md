# Bounded package profile catalog

These are additive metadata bindings for MusicXML 4.0, pinned to registry commit
`3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c` and the published schema release
`799e2defb2ece0ae7bafe08dcbcac25b2c631d53`. They do not authorize retrieval or
implement a ZIP parser, resolver or graph orchestration. The catalog fixes the
interpretation consumed by that separately bounded machinery.

## Identity and intrinsic interpretation

`ProfileResourceGraph` (`musicxml-4.0/resource-graph`), `ProfileMXLSource`
(`musicxml-4.0/mxl-source`) and `ProfileMXLModel` (`musicxml-4.0/mxl-model`) all
have `TargetPackage` and no XML roots. Their rules use closed package selectors,
not a synthetic Container assembly or a relaxed XML catalog invariant.

Every profile requires these source intrinsic rows:

| Assembly | Target | Profile |
| --- | --- | --- |
| MusicXML | source | `ProfileSourceLinks` |
| Opus | source | `ProfileSourceLinks` |
| Container | source | `ProfileSourceContainer` |
| Sounds | source | `ProfileSourceSounds` |

ResourceGraph and MXLModel also permit MusicXML/Opus current-model rows pinned to
`ProfileModelLinks`. MXLSource has only the four source rows. Each row carries the
actual compiled intrinsic catalog digest. The canonical package digest covers
these rows, all selected rule metadata and the exact closed check/selector
bindings. Table order does not affect the digest. No auxiliary model roots exist.

Standalone source/model entry points reject these package profile IDs. All eight
older profiles remain exact regression controls, including complete reports and
serialized field shape. The catalog does not mutate or extend their definitions.

## Selected checks and provenance

All three profiles bind the same 17 sealed checks. Runtime applicability follows
the represented boundary and exact typed subject. For example, raw mimetype
checks are not claims about the original bytes of a current package; a raw nested
MXL boundary inside that same graph receives the raw checks.

Runtime rule keys are `MX40-PACKAGE-` plus the check name, revision 1. Research
record IDs are provenance, not a count of independent executable obligations.

| Check | Subject | Force and bounded condition | Original provenance |
| --- | --- | --- | --- |
| `container-path` | package | Mandatory exact `META-INF/container.xml` entry; independent container XSD report | `MX40-PROSE-mxl-container-path` |
| `mimetype-stored` | entry | Existing raw mimetype is STORE and unencrypted; independent local/central evidence | `MX40-PROSE-mxl-mimetype-stored` |
| `mimetype-no-extra` | entry | Mandatory local header clause; independently associated zero local extra passes even with central-only extras | `MX40-PROSE-mxl-mimetype-no-extra` |
| `mimetype-no-prefix` | entry | Reviewed leading ASCII/NUL padding and five standard Unicode BOMs; other families remain unknown | `MX40-PROSE-mxl-mimetype-no-prefix` |
| `first-root-musicxml` | rootfile | Mandatory actual musical kind for the first encoded rootfile | `MX40-PROSE-mxl-first-root-musicxml` |
| `rootfile-root` | rootfile | MusicXML rootfiles have unqualified score-partwise/timewise/opus roots; known nonmusical alternates are separate | `MX40-PROSE-mxl-rootfile-document-root` |
| `first-media-type` | rootfile | Reviewed primary media-type condition with absent/present-empty distinction and bounded known type set | `MX40-PROSE-mxl-first-media-type`, `MX40-PROSE-mxl-media-type-default` |
| `score-before-parts` | package | Mandatory only when exact scoped roles establish score plus individual parts; complete minimal packages can prove vacuity | `MX40-PROSE-mxl-score-before-parts` |
| `link-target` | edge | Exact selected link family, adopted target kind and supported fragment evidence | `MX40-PROSE-opus-link-target`, `MX40-PROSE-opus-score-target`, `MX40-PROSE-part-link-archive-root`, `MX40-PROSE-compressed-relative-links`; score work-opus schema annotation |
| `instrument-association` | association | Exact instrument-to-part-file evidence after the existing local IDREF prerequisite | `MX40-PROSE-instrument-link-target` |
| `group-match` | edge | Optional exact-string correspondence advice | `MX40-PROSE-group-link-match` |
| `mimetype-advice` | package | Optional presence, first placement and exact US-ASCII MIME content | `MX40-PROSE-mxl-mimetype-first` |
| `entry-name-advice` | entry | Optional raw-byte UTF-8 interoperability advice. Path policy terminates on the first unsafe/non-UTF-8 name; that member may retain advice, but later members are unavailable | `MX40-PROSE-mxl-entry-name-utf8` |
| `compression-observation` | entry | Capability context, not a normative DEFLATE-only rejection | `MX40-PROSE-mxl-deflate`, `MX40-PROSE-mxl-zip-format` |
| `schema-hint-pairs` | resource | Capability policy for separately selected discovery pairing, never an XSD datatype predicate | `REQ-DEF-XSI-SCHEMALOC`, `REQ-DEF-XSI-NONAMESPACE` |
| `image-scaling` | image | Optional one-dimension proportional/two-dimension independent advice, with typed compatible metadata | `MX40-PROSE-image-scaling` |
| `file-metadata-advice` | metadata | Optional .mxl/.musicxml/legacy .xml and MIME conventions; suffix does not prove kind | `MX40-PROSE-musicxml-file-types` |

Each rule carries offline-readable title, explanation, diagnostic message,
original IDs and pinned primary source locators. The principal authorities are
[container.xsd](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/container.xsd),
[score link annotations](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L4541),
[part/instrument annotations](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L5926),
[Opus link annotations](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/opus.xsd#L76),
[image attributes](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2326),
and [XSD 1.0 discovery](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#schema-loc).
The [dated compressed MXL tutorial](https://www.w3.org/2021/06/musicxml40/tutorial/compressed-mxl-files/)
provides publication context; its historical statement denying a container XSD
does not override the actual pinned published container schema.

## Mimetype header scope, reviewed 14 October 2026

The adopted early-byte identification mechanism fixes the local payload position;
central-directory extras cannot move it. The
[dated source review](evidence/mimetype-local-header-review-20261014.md) follows
MusicXML's own tutorial and adoption history through the exact EPUB 3.1 and
OpenDocument 1.0 predecessors. This contextual evidence supersedes the older
central-only interpretation gate; it is not a new user-selected ZIP profile or
an express MusicXML erratum.

A proved local extra is a failure, including when later transport is incomplete.
A proved zero local extra with independently associated headers passes this one
predicate regardless of central extra length. Unavailable local or association
evidence remains unavailable. Separate raw ZIP checks still govern central
record bounds, identity, flags/methods, entry layout, CRC and supported features.
An incompletely framed mimetype central extra block produces a bounded transport
capability error after identity checks. It stops package/graph enumeration and
child assessment even when the low-level entry inventory is retained: no
Container or primary-resource assessment is promised after that
refusal. Already-established local-header no-extra Pass/Fail remains independent,
and the package cannot be satisfied. This behavior is covered by
`TestPackageGraphMimetypeCentralExtraFramingIsIndependent` and
`TestMXLAssessmentMimetypeCentralExtraFramingPreservesAssociation`. The narrow
framing guard does not interpret unknown extra kinds, change unrelated-entry
admission, or expand ZIP support.

## Current-state revision

Revision 2 of `MX40-PACKAGE-schema-hint-pairs` assesses a current model resource
as it would be encoded, so schema-hint pairing is not applicable to it. Every
package catalog shares the rule, so every catalog digest moves, including the
source and graph profiles whose source-resource outcomes are unchanged.
`current_state_package_migration_test.go` maps each earlier capture to its new
identity and pins the complete Profile JSON of all twelve profiles.

The revision follows a rule-by-rule audit of a freshly decoded package and of a
package with extra resources. Each rule's fact is (a) derivable from the current
model or `MXLPackage` fields, (b) fixed by `EncodeMXLPackage` for every value it
accepts, or (c) dependent on original bytes:

| Rule `MX40-PACKAGE-…` | Current package | Class |
|---|---|---|
| `container-path`, `mimetype-stored`, `mimetype-no-extra`, `mimetype-no-prefix`, `mimetype-advice`, `entry-name-advice`, `compression-observation` | not applicable: raw-only scopes | (b): a stored mimetype entry without an extra field is written first, then `META-INF/container.xml` built from `RootFiles`; entry names are valid UTF-8 paths; Store for mimetype, Deflate for the rest |
| `first-root-musicxml`, `first-media-type`, `link-target`, `group-match`, `file-metadata-advice` | evaluated from the current fields | (a) |
| `rootfile-root`, `score-before-parts`, `instrument-association`, `image-scaling` | evaluated from the current fields; in the audited cases, which had no resolver, unknown where `InspectMXL` on the same bytes is unknown (nested `.mxl` indirection, role, association and image providers) | (a), an open interpretation or a provider; never (c) |
| `schema-hint-pairs` | revision 1: unknown for every model resource; revision 2: not applicable to model resources, source pairing for retained raw resources | (a)+(b): the generated model types cannot represent `xsi:schemaLocation` or `xsi:noNamespaceSchemaLocation`, and the encoders write `xml.Header` plus `Encode`, without hints or a DOCTYPE; only "did the original have hints" is (c), which is not asked of a current model |

`TestAssessMXLPackageCurrentStateMatchesEncodedArchive` and
`TestAssessMXLPackageCurrentStateCorpus` check the result: package findings and
acceptance agree with `InspectMXL` on the bytes `EncodeMXLPackage` writes,
after edits and for every official and test-suite file accepted as an archive.

| Package catalog group | Before | After |
|---|---|---|
| Ordinary graph/raw-MXL/current-MXL | `554a97e25d6494c2e0550519fd4f6d2a439ca2f31e01a82baa7e28a00ddd2088` | `bcc5d047e4de9f02a61cfdd86f57297f9655006ea0fb6f04ef38628d05dcc28f` |
| Dictionary graph/raw-MXL/current-MXL | `f80db03b147d43021db39d1762880c2ef8e83967c1002c37b92229eb0feabd44` | `bbb60bcd61359b41fda394d95e29d93c42c8c0526b101e98f375b698204d6658` |
| Internal-DTD dictionary graph/raw-MXL/current-MXL | `69d813b09c15ea5083d382fe8864a41179e93cf52c4b04bfb140493070df7648` | `135bb292eac14befeb45ac8c96e0f950028dffa39a7a152d4b2c5759aa8a792b` |
| Semantic graph/raw-MXL/current-MXL | `e7be6720d91cdc81becf07f5312b6c86039f27290ccb72139643186118cc4a89` | `ab43413ab5e82f27b9ffd406d542e006c2f87d6ebdcc30ea912d667100f34533` |

## Boundaries that affect readiness

- Current models cannot prove discarded original xsi hints absent or complete.
  When this scope was recorded, schema discovery therefore left every
  current-model graph/package bundle incomplete. Revision 2 of
  `MX40-PACKAGE-schema-hint-pairs` assesses a current model as it would be
  encoded instead: the model cannot represent xsi hints and the encoders write
  none, so the pairing is not applicable to it and neither fails nor blocks
  completion. The discarded history is still not
  reconstructed; it is simply not the question asked of a current model. Retained
  raw resources keep source pairing. Source odd-length
  hints remain valid `list(anyURI)` values when their members are valid.
  Known-false pairing yields `OutcomeFail` under `CategoryCapability`,
  `ClassPolicy` and `RolePredicate`, retaining `schema-location-unmatched-token`
  and the exact provider/subject cause. It blocks `Satisfied()` through
  `BlockingProblems` without adding normative `Counts.Fail` or establishing
  `ConformanceViolated`.
  Known-false pairing alone does not make assessment incomplete. No hint fetches
  or selects a schema.
- Original ZIP compression, encryption, extras, entry order and original container
  lexical presence are outside a current MXL boundary's claim. No encode/reparse
  step can establish those facts. Separately reached raw nested packages still
  carry original-source obligations.
- Unreviewed leading padding, complete media type-set closure, outer
  rootfile-to-compressed-package substitution and unknown
  role/association interpretations remain explicit incomplete cases when reached.
  Technical nested traversal alone does not settle outer indirection semantics.
- Full ZIP/JAR/DEFLATE normative closure and external dictionaries remain open
  obligations assigned to the final external-closure batch. This bounded profile
  does not discharge them or remove them from the obligation inventory.
  Ordinary archive opening, a decoded child, a successful root-kind check, a
  filename, or matching cross-file ID strings never certify whole conformance.
  Minimal-package no-links vacuity additionally requires the snapshot's explicit
  selection witness for an `ImplementationAvailable` sealed local-link plan and
  complete intrinsic scope. That witness alone asserts neither projection nor
  scope completion. Absent or unsupported implementation support cannot establish
  it; empty unselected projections cannot certify the absence of separate parts.
- There is no instance-wide `version=4.0` rule, invented role inference, universal
  XPointer, required renderer or implicit HTTP/filesystem access. Auxiliary
  source assemblies, local link facts and core budgets retain their own limits.

Catalog tests verify exact selection, role/class/category separation, complete
intrinsic tables, defensive metadata ownership, canonical digest sensitivity,
wrong-profile rejection and the full frozen eight-profile regression. Graph,
raw/current-MXL orchestration and typed predicate outcomes require their own
runtime acceptance tests; these metadata tests do not stand in for them.

Raw entry-name UTF-8 byte validity and the ZIP EFS flag are separate observations.
A clear EFS flag does not make valid UTF-8 bytes invalid. The transport path
policy still rejects unsafe or non-UTF-8 member names and stops inventorying at
that first member. Its retained advice is not evidence about later entries and
does not make the incomplete archive inventory complete.
