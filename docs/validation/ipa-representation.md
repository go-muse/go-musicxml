# Additive IPA representation capability

This separately selected, bounded affirmative capability adds no public struct
fields, generic provider, options flag or automatic loading. It does not claim
phonotactics, pronunciation intent, rendering correctness, a complete negative
complement or universal IPA conformance.

## Selection and data

Select `DictionaryManifestIPARepresentation2015Unicode13` with
`DictionaryRelationIPARepresentation` through the existing dictionary entry
points, or provide its exact existing `DictionaryImport.IPA` typed payload.
The component is `DictionaryComponentIPARepresentation2015Unicode13`, its kind
is `DictionaryKindIPARepresentation`, and all five fixed components may coexist.
An absent source remains an explicit unavailable capability.
`BundledDictionarySource()` supplies the pinned derived bundle from embedded
local data through the same bounded importer. It does not fetch cited sources
or upgrade derived lineage to newly verified original-publication provenance.

The manifest has 295 complete symbol encoding atoms (273 singletons and 22 exact
noncanonical constructions), no literal example rows, and 158 genuine recursive
Unicode13 canonical relations. The compiled policy contains only 53 relevant
nonzero CCC values, two paired-half-tie relations and fixed role/projection rules.
The immutable support domain is derived from the verified rows; it has 436 raw
scalars and 278 normalized leaves including four half marks. No full UCD, age
table, source publication, font, PDF or host normalizer is a runtime dependency.

The [source ledger](../../testdata/dictionaries/ipa-representation/source-ledger.md)
provides all source definitions, row witnesses, inference strengths and separate
notices. In particular R05/R06/R14/R17 remain explicit compositional inferences,
not exact DoNotEmit mappings or canonical equivalences. The unsupported
historical candidates are outside the affirmative scope, not mandatory unfinished
positives or known-negative conclusions.

## Recognition and output

- Guard each unchanged input scalar against the sealed support domain, expand
  its full canonical decomposition, and retain its exact original byte interval.
- Stable-sort combining segments by CCC without crossing starters or reordering
  equal-CCC marks. Leading independent marks and empty concatenations are valid.
- Search every complete atom candidate for each starter. Required marks must
  prefix each equal-CCC run after separately accounted half marks; residual marks
  must be independently admitted. This permits a lower-CCC overlay before the
  cedilla of a complete represented c-with-cedilla, without authorizing a free
  cedilla or another letter with one.
- Admit only directly established, correctly oriented adjacent half pairs,
  with complete represented bases and independent single-base decorations.
  Multiple disjoint pairs are supported. Repeated halves, shared-middle pairs,
  multiple halves in one segment, reversed/mismatched or unmatched halves and
  endpoints on signs remain unavailable. A whole tie on either endpoint or the
  immediately preceding segment also leaves mixed overlap unavailable.
- Keep U+035C/U+0361 as independent complete mark atoms in the pinned
  concatenation language. Isolated, repeated and sign-adjacent whole marks have
  representation witnesses; these do not certify tie endpoints, rendering or
  phonotactics. The endpoint, repetition and orientation restrictions above
  concern presentation-form half ties; mixed whole/half overlap remains unavailable.
- Publish one `DictionarySpan` over the entire known string, preserving raw
  bytes, including a genuinely empty string and known text without a witness or
  capability. Unavailable/non-string scalar evidence has no invented raw span;
  its evidence state and cause remain intact.

The new `ContextDictionaryIPARepresentation` property identifies the new relation.
The whole span has category `whole-representation`, empty `RowID` and empty
`Equivalent`. Only a successful complete witness makes both `Result` and
`WholeSequence` known-true and satisfies this repertoire clause. Unproved input
is unavailable, never false by lookup absence. This does not establish overall
MusicXML conformance or suppress any independent failure.

The old `ContextDictionaryIPA` projections remain unchanged even when both
manifests are selected. The legacy 197-row data still leave `WholeSequence`
unavailable. If the new manifest is unselected, no new projection is emitted and
the legacy node decision remains in force. Selected but missing data emit an
explicit unavailable projection.

## Immutable identities

| Identity | Bytes | SHA-256 |
|---|---:|---|
| New raw component | 58,745 | `8ab091cfe63c248ce7351fecce9b670af11c949f76d7a5d0f18c2ed43d44f473` |
| New transformed records | 22,430 | `f3efd17200088616bf170aedc4f2cf618a8db4c73d8852de03b72b86c7dc2737` |
| Minimal policy JSON | 3,359 | `abf354d8847b1eb4233ac2d7e835d025ebd9d99dee903737aa0e921004a65bd2` |
| Unchanged legacy raw | 26,435 | `ee4d39dbf8e999d12ae4e0680fc1b55a9704b81a4f049a110bcf5a21c1da2901` |
| Unchanged legacy transformed | 9,836 | `47b0a8812835e56e0f8b6527cca13b6d55bb4b3ee231186868e8ec8590804ec6` |

The new raw schema-2 prefix is 797 bytes; the 453 records copy exactly 8,007 UTF-8
bytes, and the longest field is 18 bytes. The transformed encoder adds the
length-prefixed exact policy hash after its five manifest strings and before the
row count, only for the new manifest. The policy revision is
`ipa2015-unicode13-canonical-representation-v1`. Static-policy tests verify its
hash, every compiled CCC value, role counts, half identities and exclusion
behavior. No legacy data encoding or pin is changed.

## Errors, work and lifecycle

Importer-owned malformed new bundles are `ConfigurationError`, including wrong
length/hash, structural fields/counts/ordering or malformed encodings. They never
produce a usable imported set. A caller's own Open/Read/Close error keeps the
existing unavailable-import classification, even if it is a ConfigurationError.
Legacy component failures retain their original classification. Cancellation,
deadline and resource failures remain terminal; joined causes are preserved,
including cancellation from Close after a malformed body length.

The copied typed data and fixed derived index have independent admitted leases.
The index's 32,768-byte conservative reservation is checked against its actual
structure in tests. Runtime normalization checks arithmetic and AllocationSize
before reserving two 32-byte-per-leaf buffers plus 4,096 bytes of fixed
control/counting workspace. Raw scans, lookups, expansion writes, stable sort
counts/prefix/scatter, alternative candidates and pair transitions consume work.
Proof scratch is released on success, quota refusal or cancellation. Output
copies use the existing projection accounting; reports retain no dictionary
borrow. Closed owners preserve indexes for existing borrowers until the last
release and reject new borrowers.

## Deliberate profile migration

The `DictionaryIPA` operation name stays unchanged, but its plan policy and
`MX40-DICTIONARY-musicxml/ipa-positive` rule move from revision 1 to revision 2.
Only this operation adds the fifth pin. Non-IPA dictionary operations retain
revision 1 and their four-pin plan identities. Dictionary-containing catalog
fingerprints deliberately change with the new policy and precise closure text.
Profiles sharing such a catalog also change their `CatalogDigest`, even when
their selected rules do not include IPA. This IPA change does not itself alter
catalogs without this operation. Other functional corrections in this PR,
including local-header mimetype scope, have separate identity changes.

### Published package baseline

The public baseline is [main at `5295080`](https://github.com/go-muse/go-musicxml/commit/5295080b2030be6d616c714e7c8bcad47185c742).
These are `CatalogDigest` values, not hashes of complete Profile JSON. Each row
covers all three graph/raw-MXL/current-MXL profiles. The ordinary package change
comes from the local-header mimetype correction; the dictionary package change
also incorporates the selected intrinsic dictionary migration.

| Published catalog group | Main `5295080` | Corrected catalog |
|---|---|---|
| Ordinary graph/raw-MXL/current-MXL | `083b1b94f8b581825b23b81e3c7e39c238741f04f897ba71f7aa9354ea789d97` | `554a97e25d6494c2e0550519fd4f6d2a439ca2f31e01a82baa7e28a00ddd2088` |
| Dictionary graph/raw-MXL/current-MXL | `14e5cdb4bc6b9fd2b7315f0059255fd8fc14fd43217e46238b1b64fd503b1333` | `f80db03b147d43021db39d1762880c2ef8e83967c1002c37b92229eb0feabd44` |

`TestPackageProfilesPublishedBaselineMigration` verifies all six mappings against
an immutable capture of complete profiles executed from that public commit.
The fixture includes its source commit and preserves the old closure/rule data;
its byte hash, profile count, unique IDs, target and before/after catalog values
are asserted independently of the current implementation.

### Published source/model dictionary baseline

The same public main commit `5295080` supplies the baseline for
`ProfileSourceDictionaries`, `ProfileModelDictionaries` and
`ProfileSourceSoundsDictionaries`. Their shared `CatalogDigest` migration is:

| Published catalog group | Main `5295080` | Corrected catalog |
|---|---|---|
| Source/model/sounds dictionaries | `21c731b5d19ebf918bff42fb3a15cac2fae6053b5b8006faf3f32b6108a8db21` | `d8199be864c2baa081aca5887ca0ac188052ba22f6a690ea9bb3b436dc054c57` |

`TestDictionaryProfilesPublishedBaselineMigration` checks all three identities
against an immutable complete-profile capture executed from that public commit.
Its pinned bytes, source commit, unique profile IDs, targets and exact old/current
catalog values distinguish this public migration from the local history below.

### Supplemental unpublished checkpoints

The following comparisons retain engineering history from unpublished local
integration checkpoint `47cd65c`; they are not public release baselines. In
particular `147d3418…` already included earlier functional changes and must not
be substituted for the public dictionary package baseline above.

| Intermediate catalog group | Before IPA integration | After IPA integration |
|---|---|---|
| Source internal-DTD local/links/dictionaries | `9f9d4a3ef31008332be016a7fe5b86cc10b2a764cf13426735683455ec07e5d0` | `b2c6ad587726a0b64f7315f0d053a867e0d1d85b81f87217267d967fe5f9092c` |
| Dictionary graph/raw-MXL/current-MXL | `147d3418013fcd8f41dd5335ea0e7b068cb43da421b0febac895ce7536fb1a9b` | `f80db03b147d43021db39d1762880c2ef8e83967c1002c37b92229eb0feabd44` |
| DTD/dictionary graph/raw-MXL/current-MXL | `4ecfcba585d635aa3a67ae6748985bb63c2b0482e6257cff7ccf39220acad319` | `9d55aa68bf6b7fa45a27417ab9800d4e1f7b9744af810004de53cabf7b7be3b2` |

The separately compiled DTD/sounds dictionary catalog remains
`4d3745b2a8b4a04862dce514a19d1c744770059e8321a99e9c5e083df63bda06`.
`dictionary_ipa_representation_migration_test.go` preserves every historical
whole-Profile JSON capture and enumerates the nine intentional hash migrations;
the existing compatibility guards call that exact helper instead of rewriting
their old constants. The same test proves legacy projection JSON is identical
with the new manifest selected or unselected. Historical snapshots, measured
four-component budget observations and planning statuses are not rewritten.
Broader semantic-composition catalog accounting belongs to its integration.

## Verification scope

The source reviewer independently reproduced the finite rows, roles, support
closure and all 15 source pins, compared 81,488 finite construction/mark cases,
and reconstructed the final serialized identities. Repository tests separately
cover all atoms, constructed/leading/canonical/half cases, 2,801 independent
finite concatenation-oracle strings, raw intervals and equal-CCC order, exact
work/storage edges, cancellation and owner/borrow cleanup. Consumer tests cover
source, direct model, assessed reader, graph aliases and raw/current/DTD MXL.

Independent resource review reproduced and retained two regression tests before
their fixes: Close-time cancellation on malformed new-body length, and invented
raw spans for unavailable/non-string scalar evidence. The unchanged overlays
passed after both fixes, including 455 direct-node cancellation checkpoints.
Focused concurrent lifecycle race checks passed. The review environment compiles
386 binaries but cannot execute them (`exec format error`); this is not a claim
of 386 execution success. Full integrated validation remains a separate gate.
