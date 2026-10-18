# Pinned dictionary scope and retained obligations

Status: **implemented additive slice under integration review**, 14 October
2026. The canonical pre-change base is
`a8e7a10cdf7e34fad979a758c1ba481c74f579fe` (tree
`c54f0a190e24687f162d1851bf46dd08147de473`). The historical integration is
unpublished local checkpoint
`548734f80fef2c5de9997914a8ab47d1c3baac65`; later corrections and final checks
are separate gates. No published-head acceptance, complete IPA validator,
complete MusicXML conformance or final-program acceptance is claimed. See the
[published lineage map][published-lineage] for inspectable containing commits;
historical local checkpoints are not public checkout targets. See [verification
evidence](pinned-dictionaries-evidence.md) for the exact scope of completed
checks and [API.md](../../API.md#opt-in-pinned-dictionaries) for use.

## Additive representation update

The legacy positive-v1 behavior described below remains unchanged. The separately
selected [IPA representation manifest](ipa-representation.md) now adds bounded
affirmative whole-input witnesses, without a negative complement, phonotactics
or universal IPA conformance. It reuses the existing typed IPA payload and
adapters. Its exact policy/rule and dictionary-containing catalog migrations are
recorded separately; no original planning record or historical result is changed.

## Preservation and selected closure

This page is an additive contribution crosswalk. It does not change the original
95 engineering records, their statuses, source pointers or test contracts in
[implementation-plan.json](plan/implementation-plan.json). The original 307
context records, 291 dispositions, 51 prose contracts, 28 issues and 20 split
records retain their denominators and historical meaning. A planned test ID is
not an executed test; a reviewed implementation contribution does not silently
retire an original requirement. Original `hard_error_basis=false` issue records
are not converted into independent normative predicates.

The slice adds fixed, verified, immutable dictionary capabilities; exact
source/current-model consumers; and propagation through selected resource graphs
and nested MXL packages. The source-sounds intrinsic row admits compatible
explicitly supplied fact-stream snapshots; the root dictionary package catalog
constructor is private. Standalone `InspectXML` with `Dictionaries` supports
`ProfileSourceSoundsDictionaries`, but public graph/MXL entry points do not discover,
resolve or assess sounds children. The slice selects SMuFL 1.4 canonical names
and exact ranges, MusicXML 4.0 standard-sound recognition, unchanged partial
IPA positive-v1 facts and a separately selected bounded representation witness.
It adds no general downloadable dictionary framework, font inspection, expression
evaluation or application sound mapping.
The proposed generic `ExternalDictionary`/`ExternalProvider` interfaces in
[go-contracts.md](go-contracts.md) remain proposals, not the shipped API.

The minimal source/model profile closure is XSD/local-link prerequisites plus
these dictionary operations and narrow defaults/advice. Full timeline, layout,
repeat and unrelated context families are not prerequisites. New graph/MXL
profiles bind exact dictionary intrinsic profiles/catalog digests. The IPA
representation addition deliberately migrates dictionary-containing catalogs;
its exact migration is separate from the earlier preserved-profile capture.
Source-only container stays `ProfileSourceContainer`;
the source-sounds catalog row uses `ProfileSourceSoundsDictionaries`. Raw MXL
retains source-only intrinsic bindings; current MXL does not reconstruct
discarded source evidence.
The separate internal-DTD/source-origin pipeline is not composed implicitly.

## Original-obligation contribution crosswalk

The pointer suffixes below refer to the immutable original files under `plan/`.
For each row, the named remaining owner is an original requirement or retained
workstream, not an assertion that a new person has accepted ownership. Test names
identify committed coverage routes; current execution receipts and unresolved
review gates are separate in the evidence page.

| Original ID / contract | Applicability and implemented contribution | Representative coverage route | Remaining limit and owner |
| --- | --- | --- | --- |
| `REQ-DEF-EXTERNAL-SMUFL` / `TEST-DEF-EXTERNAL-SMUFL` (`implementation-plan.json#/requirements/33`) | Profiles selecting exact assessed SMuFL subjects use immutable SMuFL 1.4 pins and canonical membership, with authoritative absence only from that complete canonical relation. | `TestDictionaryReviewedMusicDigestsAndOwnedSortedRecords`; `TestDictionaryCanonicalSourceModelAndIndependentFamily` | Exact-head acceptance remains with dictionary integration; font alternates and general rendering intent are outside this relation. |
| `CHECK-smufl-canonical-name` / `TEST-smufl-canonical-name` (`prose-contracts.json#/checks/1`) | Present selected attribute or element scalar with genuine assessed type/scalar evidence. One canonical conclusion per scalar occurrence; exact case, normalized NMTOKEN value, no codepoint-string/alias shortcut. | `TestDictionaryAllTwentySevenDeclaredUses`; `TestDictionaryAllTenAssessedSMuFLTypes`; `TestDictionaryAllThirtyFiveDynamicStringCarriers` | Missing capability stays unavailable; discarded model `xsi:type` stays unavailable. Owner: `REQ-DEF-EXTERNAL-SMUFL` and `REQ-DEF-ADAPT-CAPABILITIES`. |
| `CHECK-smufl-accidental-family` / `TEST-smufl-accidental-family` (`#/checks/2`) | Present assessed accidental-family scalar: retain shared XSD collapse/pattern independently and add canonical membership. The `acc`, `medRenFla`, `medRenNatura`, `medRenShar`, `kievanAccidental` domain is not the SMuFL accidentals class. | `TestDictionarySixFamilyFailuresAndBadNominationStayCausal`; `TestDictionaryAllTenAssessedSMuFLTypes` | No implicit classes relation; absent dictionary cannot hide an XSD failure. Owner: `REQ-DEF-EXTERNAL-SMUFL`. |
| `CHECK-smufl-coda-family` / `TEST-smufl-coda-family` (`#/checks/3`) | Present coda-family scalar retains its XSD facet and independent canonical relation. | Same family/type routes above | No family inferred from dictionary descriptions. Owner: `REQ-DEF-EXTERNAL-SMUFL`. |
| `CHECK-smufl-lyrics-family` / `TEST-smufl-lyrics-family` (`#/checks/4`) | Present lyrics-family scalar, including elision SMuFL even if nonempty display text takes precedence. | `TestDictionaryAllTwentySevenDeclaredUses`; `TestDictionaryInactiveRenderingAttributesStillCanonical` | Display precedence does not erase scalar obligations. Owner: `REQ-DEF-EXTERNAL-SMUFL`; preserve `CHECK-context-elision-precedence`. |
| `CHECK-smufl-pictogram-family` / `TEST-smufl-pictogram-family` (`#/checks/5`) | Present pictogram-family scalar retains XSD family checking plus exact canonical membership. | `TestDictionaryAllTwentySevenDeclaredUses`; `TestDictionaryAllTenAssessedSMuFLTypes` | No visual/semantic inference from the glyph name. Owner: `REQ-DEF-EXTERNAL-SMUFL`. |
| `CHECK-smufl-segno-family` / `TEST-smufl-segno-family` (`#/checks/6`) | Present segno-family scalar retains independent XSD and canonical results. | Same family/type routes above | No notation/playback agreement is inferred. Owner: `REQ-DEF-EXTERNAL-SMUFL`. |
| `CHECK-smufl-wavy-family` / `TEST-smufl-wavy-family` (`#/checks/7`) | Present wavy-line scalar retains `wiggle…` or `guitar…VibratoStroke` family semantics and beam exclusion, independently of the canonical set. | `TestDictionarySixFamilyFailuresAndBadNominationStayCausal`; `TestDictionaryAllTenAssessedSMuFLTypes` | No broader ornament semantic classifier. Owner: `REQ-DEF-EXTERNAL-SMUFL`. |
| `REQ-DEF-ISSUE-SMUFL-VERSION-DICTIONARY` / `MX40-ISSUE-smufl-version-dictionary` | Explicitly choose SMuFL 1.4, exact canonical names and pinned transform/policy; distinguish optional font glyphs, primary/alternate codepoints, classes and family facets. | `TestDictionaryManifestsAreClosedAndOwned`; `TestDictionaryComponentVerifierRejectsZeroWrongLengthAndCanceled` | Historical issue status and `hard_error_basis=false` remain unchanged; future edition changes need new reviewed identities. Owner: the same issue requirement. |
| `REQ-DEF-EXTERNAL-IPA` / `TEST-DEF-EXTERNAL-IPA` (`implementation-plan.json#/requirements/34`); `CHECK-ipa-2015-unicode13` / `TEST-ipa-2015-unicode13` (`prose-contracts.json#/checks/15`) | Every present exact `play/ipa`: legacy positive-v1 retains partial symbol/example/canonical facts. Separately selected representation-v1 may establish a bounded affirmative whole-input witness, including known empty strings. | `TestDictionaryIPAPositiveNeverClosesSequence`; `TestDictionaryENTITYAndSemanticUseRemainIndependent`; `TestDictionaryIPADerivedBundleDoesNotAcquireOriginalSourceOrNegativeAuthority`; `TestIPARepresentationSelectedConsumerWholeInput`; `TestIPARepresentationLegacyManifestIdentityUntouched` | Unproved representations remain unavailable; no negative complement, phonotactics or universal conformance claim. Owner: `REQ-DEF-EXTERNAL-IPA`; see the legacy limits and additive policy below. |
| `REQ-DEF-ISSUE-IPA-CHARACTER-REPERTOIRE` / `MX40-ISSUE-ipa-character-repertoire` | Select the named partial-positive or bounded affirmative-representation policy without presenting a Unicode block or later mapping aid as a complete repertoire. | IPA tests above; unchanged 197-row legacy ledger and separately reviewed 453-row representation ledger | Keep original issue status and `hard_error_basis=false`; full normative interpretation remains with this issue and `REQ-DEF-EXTERNAL-IPA`. |
| `CHECK-context-sounds-standard-identifiers` / `TEST-context-sounds-standard-identifiers` (`context-dispositions.json#/checks/242`) | Exact score-instrument and instrument-change `instrument-sound`, plus source-only sounds `sound/@id`: expose standard=true/false from 894 IDs. | `TestDictionarySoundsCustomAllowedAndIPAGate`; `TestDictionaryGraphAdmitsExplicitSourceSoundsProfile` | False means nonstandard, never invalid MusicXML. No `SoundsDocument`, public graph/MXL sounds traversal or closed extension enum. The source-sounds graph test calls the internal helper directly. Public sounds-resource traversal remains with `REQ-DEF-PACKAGE-ADAPTERS` and `REQ-DEF-ADAPT-SOURCE`; standard-sounds dictionary context still owns the selected scalar relation; general mapping remains `CHECK-context-sounds-application-mapping`. |
| `CHECK-context-stopped-smufl-default` / `TEST-context-stopped-smufl-default` (`#/checks/194`) | Exact technical/stopped: proven absent attribute gives `brassMuteClosed`; explicit present scalar is independently canonical-assessed. | `TestDictionaryDefaultsWithoutDataAndRangeAdvice`; `TestDictionaryAllTwentySevenDeclaredUses` | Default remains known without data; present invalid/unknown does not become absence. No brass rendering proof. Owner: this context obligation. |
| `CHECK-context-open-smufl-default` / `TEST-context-open-smufl-default` (`#/checks/195`) | Exact technical/open: same presence rules, default `brassMuteOpen`. | Same defaults/direct-use routes | No default on unrelated same-type elements or inferred performance meaning. Owner: this context obligation. |
| `CHECK-context-half-muted-smufl-default` / `TEST-context-half-muted-smufl-default` (`#/checks/196`) | Exact technical/half-muted: same presence rules, default `brassMuteHalfClosed`. | Same defaults/direct-use routes | Preserve local default independently of dictionary availability. Owner: this context obligation. |
| `CHECK-context-tremolo-smufl-condition` / `TEST-context-tremolo-smufl-condition` (`#/checks/204`) | Exact ornaments/tremolo: XSD default type=single; known unmeasured activates `buzzRoll` default and tremolos-range context. Present attributes remain canonical-assessed even when inactive. | `TestDictionaryDefaultsWithoutDataAndRangeAdvice`; `TestDictionaryInactiveRenderingAttributesStillCanonical` | Range context is not an extra mandatory predicate or general tremolo/performance proof. Owner: this context obligation and retained performance workstream. |
| `CHECK-context-named-notehead-text` / `TEST-context-named-notehead-text` (`#/checks/36`) | Exact note/notehead with applicable glyph: canonical and range evidence supports note-name text advice for E150–E1AF / EEE0–EEFF. Known non-other with no SMuFL is inapplicable. | `TestDictionaryDefaultsWithoutDataAndRangeAdvice` | Missing glyph/data leaves advice unknown; never a conformance failure or geometry proof. Owner: this advisory obligation. |
| `CHECK-context-symbol-defaults-and-name` / `TEST-context-symbol-defaults-and-name` (`#/checks/290`) | Exact direction-type/symbol: justify=left, enclosure=none on proven absence; reuse the single canonical-name consumer. | `TestDictionarySymbolDefaultsAndUnavailableAdviceSourceModel` | No duplicate canonical rejection or fabricated source presence. Owner: this context obligation. |
| `CHECK-context-symbol-semantic-markup` / `TEST-context-symbol-semantic-markup` (`#/checks/109`) | Preserve separate advice for exact direction symbols and explicitly unavailable specialized-markup relation. | `TestDictionarySymbolDefaultsAndUnavailableAdviceSourceModel` | Canonical/codepoint data cannot select appropriate semantic markup. Owner: this advisory/semantic-relation obligation. |
| `CHECK-context-glyph-match-type` / `TEST-context-glyph-match-type` (`#/checks/145`) | Exact appearance/glyph retains source-listed positive examples through existing `glyphSemanticTarget`/`glyphTypeMatch` and independent canonical membership. | `TestDictionaryGlyphLimitedPositivesAndUnavailableAdviceSourceModel`; `TestDictionaryAllTwentySevenDeclaredUses` covers canonical input | General glyph/type agreement remains unknown; open glyph-type values stay open. Owner: this advisory obligation; preserve `CHECK-context-extensible-glyph-type`. |
| `CHECK-context-arrow-glyph-consistency` / `TEST-context-arrow-glyph-consistency` (`#/checks/161`) | Exact technical/arrow retains direction/style/circular/arrowhead structure, independent SMuFL membership and unavailable description agreement. | `TestDictionaryArrowLocalDescriptionAndUnavailableAdviceSourceModel` | No English-description heuristic or head-versus-whole-arrow inference proves agreement. Owner: this advisory/semantic-relation obligation. |
| `CHECK-context-mordent-long-default` / `TEST-context-mordent-long-default` (`#/checks/181`) | Exact ornaments/mordent and inverted-mordent: default long=no on proven absence; retain distinct MusicXML/SMuFL terminology. | `TestDictionaryDefaultsWithoutDataAndRangeAdvice`; `TestDictionaryInvertedMordentLocalLongDefaultSourceModel` | These two uses share one obligation; no invented glyph mapping. Owner: this context obligation. |

The four explicit symbol/glyph/arrow/inverted-mordent source/model test groups
were added after the original consumer review at test-only author
unpublished local checkpoint `4245de51cb8e84b0fe1a2736bc17dead772aae67`. They
passed on their first run; this is a coverage addition, not a claimed production
bug or RED/green correction. Parent static review accepted that supplement; its
historical integration was unpublished local checkpoint
`548734f80fef2c5de9997914a8ab47d1c3baac65`. Final whole-tree acceptance remains
part of the evidence page's outstanding gates.

The eleven default/advisory consumer records above represent ten distinct original
obligations because mordent and inverted-mordent are separate uses. Standard
sounds is a further dictionary-context obligation, not one of those eleven.
The crosswalk preserves `CHECK-context-extensible-glyph-type` (#/checks/33),
`CHECK-context-elision-precedence` (#/checks/167),
`CHECK-context-notehead-fill-default` (#/checks/188) and
`CHECK-context-font-family-order` (#/checks/230); custom geometry and font
availability are not proved here. `CHECK-context-sounds-application-mapping`
(#/checks/278) is explicitly excluded from execution by this dictionary slice,
not deleted from the program.

### Shared engineering contracts and residual owners

- `REQ-DEF-EXTERNAL-RESOLVER`, `REQ-DEF-ADAPT-CAPABILITIES`,
  `REQ-DEF-CAT-OPERATORS`, `REQ-DEF-CAT-METADATA`, `REQ-DEF-CAT-ROLES`,
  `REQ-DEF-CAT-OVERLAP` and `REQ-DEF-CAT-MATRIX`: closed typed capabilities,
  exact identity, genuine prerequisites, separate normative/context force and
  this additive source/model/consumer crosswalk. Their unrelated clauses remain
  with the original owners.
- `REQ-DEF-PROFILE-SCOPE`, `REQ-DEF-PACKAGE-ADAPTERS`,
  `REQ-DEF-SCOPE-LOCAL-LINK`, `REQ-DEF-ADAPT-SOURCE`, `REQ-DEF-ADAPT-MODEL`:
  explicit profiles, exact assembly/representation bindings and occurrence
  propagation. Existing source/model/package limitations still apply.
- `REQ-DEF-RESULT-AXES`, `REQ-DEF-CONTEXT-OUTCOMES`,
  `REQ-DEF-DIAG-METADATA`, `REQ-DEF-DIAG-CATEGORIES`, `REQ-DEF-DIAG-COMPAT`,
  `REQ-DEF-DIAG-BOUNDS`: causal unavailable evidence, preserved independent
  violations, owned projections and bounded diagnostics. A nil operation error
  is not satisfaction; advisory false and standard=false are not normative fail.
- `REQ-DEF-SEC-NOFETCH`, `REQ-DEF-SEC-CANCEL` and
  `REQ-DEF-BUDGET-ACCOUNT`, `REQ-DEF-BUDGET-INPUT`,
  `REQ-DEF-BUDGET-UNPACKED`, `REQ-DEF-BUDGET-BUFFER`,
  `REQ-DEF-BUDGET-DEPTH`, `REQ-DEF-BUDGET-EVENTS`,
  `REQ-DEF-BUDGET-ATTRS`, `REQ-DEF-BUDGET-SCALAR`, `REQ-DEF-BUDGET-IDS`,
  `REQ-DEF-BUDGET-MATCH`, `REQ-DEF-BUDGET-CONTEXT`,
  `REQ-DEF-BUDGET-LINKS`, `REQ-DEF-BUDGET-DIAGS`,
  `REQ-DEF-BUDGET-MODEL`: one finite root
  account, owning-boundary charges, bounded component/lookup/copy work and
  unchanged intrinsic/graph limits. This is no claim that every independent
  budget or parser obligation in the broader program is closed.
- `REQ-DEF-DEC-API`, `REQ-DEF-DEC-CATALOG`, `REQ-DEF-DEC-PROFILE`:
  additive API/profile identities and explicit unkeyed `ContextResult` migration.
  `REQ-DEF-VERIFY-CI`, `REQ-DEF-VERIFY-DIFFERENTIAL`,
  `REQ-DEF-VERIFY-PROFILE`: independent data/runtime review, five-axis cases and
  exact-head checks remain the acceptance owners; historical fixtures alone do
  not satisfy them.

Relevant historical capability IDs remain `CAP-585d1beaa0f60e9e`
(pinned-external-dictionary), `CAP-d6c06de01b97749b`
(resolver-revision-verification), `CAP-578f7bb0cf99fb8b` (smufl_context) and
`CAP-86802f64c9a02732` (sounds_dictionary_context). Their original planned/unbound
records have not been rewritten as executable services.

## Exact source/model consumer inventory

This compact inventory reproduces the selected declaration identities from the
read-only base audit. Runtime bindings are in
[`dictionary_bindings.go`](../../validation/dictionary_bindings.go) and are
validated against the exact schema; local names alone are never selectors.
MusicXML schema digest:
`52a61387995feb5c632a3660836a6ddaaf1e101d2a12c2f87d5f0bfb5c3191af`.
Sounds schema digest:
`13712e67b078d2de3db7e55cab04d2ef87aa49eb176e5a86e4f9f520cd7bc4ce`.

There are 27 direct SMuFL uses: 23 attributes and four element declarations,
including both distinct credit-symbol positions. The remaining four rows are
one IPA and three standard-sound uses. `S/M` means source/current-model;
`S` alone is source-only. These counts describe declaration routes, not an extra
rule execution for every row.

| Kind / exact owner and scalar | Declaration occurrence | Targets |
| --- | --- | --- |
| smufl: `accidental/@smufl` | `/types/1/attributes/2` | S/M |
| smufl: `accidental-mark/@smufl` | `/types/2/attributes/0` | S/M |
| smufl: `accidental-text/@smufl` | `/types/3/attributes/0` | S/M |
| smufl: `appearance/glyph` | `/types/8/records/46/children/3/element` | S/M |
| smufl: `arrow/@smufl` | `/types/10/attributes/10` | S/M |
| smufl: `coda/@smufl` | `/types/44/attributes/0` | S/M |
| smufl: `credit/credit-symbol` | `/types/47/records/325/children/1/element` | S/M |
| smufl: `credit/credit-symbol` | `/types/47/records/335/children/1/element` | S/M |
| smufl: `direction-type/symbol` | `/types/58/records/426/children/1/element` | S/M |
| smufl: `effect/@smufl` | `/types/64/attributes/0` | S/M |
| smufl: `elision/@smufl` | `/types/66/attributes/0` | S/M |
| smufl: `empty-placement-smufl/@smufl` | `/types/71/attributes/10` | S/M |
| smufl: `glass/@smufl` | `/types/104/attributes/0` | S/M |
| sounds: `instrument-change/instrument-sound` | `/types/138/records/903/children/0/element` | S/M |
| smufl: `key-accidental/@smufl` | `/types/143/attributes/0` | S/M |
| smufl: `membrane/@smufl` | `/types/170/attributes/0` | S/M |
| smufl: `metal/@smufl` | `/types/172/attributes/0` | S/M |
| smufl: `notehead/@smufl` | `/types/201/attributes/7` | S/M |
| smufl: `other-direction/@smufl` | `/types/219/attributes/12` | S/M |
| smufl: `other-notation/@smufl` | `/types/221/attributes/13` | S/M |
| smufl: `other-placement-text/@smufl` | `/types/222/attributes/10` | S/M |
| smufl: `other-text/@smufl` | `/types/224/attributes/0` | S/M |
| smufl: `pitched/@smufl` | `/types/242/attributes/0` | S/M |
| ipa: `play/ipa` | `/types/245/records/1691/children/0/element` | S/M |
| sounds: `score-instrument/instrument-sound` | `/types/262/records/1789/children/0/element` | S/M |
| smufl: `segno/@smufl` | `/types/264/attributes/0` | S/M |
| smufl: `timpani/@smufl` | `/types/335/attributes/0` | S/M |
| smufl: `tremolo/@smufl` | `/types/339/attributes/11` | S/M |
| smufl: `wavy-line/@smufl` | `/types/359/attributes/2` | S/M |
| smufl: `wood/@smufl` | `/types/363/attributes/0` | S/M |
| sounds: `sound/@id` | `/types/3/attributes/0` | S |

The ten exact assessed SMuFL scalar components are `smufl-glyph-name`,
`smufl-accidental-glyph-name`, `smufl-coda-glyph-name`,
`smufl-lyrics-glyph-name`, `smufl-pictogram-glyph-name`,
`smufl-segno-glyph-name`, `smufl-wavy-line-glyph-name`, `glyph`,
`formatted-symbol` and `formatted-symbol-id`. Source `xsi:type` can reach them
through these 35 additional exact `xs:string` element declarations:

| Declared owner / element | Declaration occurrence |
| --- | --- |
| `beat-repeat/except-voice` | `/types/27/records/231/children/1/element` |
| `credit/credit-type` | `/types/47/records/314/children/0/element` |
| `direction/voice` | `/types/57/records/407/children/0/element` |
| `encoding/software` | `/types/78/records/571/children/2/element` |
| `encoding/encoding-description` | `/types/78/records/571/children/3/element` |
| `forward/voice` | `/types/100/records/703/children/0/element` |
| `hole/hole-type` | `/types/130/records/864/children/0/element` |
| `hole/hole-shape` | `/types/130/records/864/children/2/element` |
| `identification/source` | `/types/135/records/884/children/3/element` |
| `instrument-change/instrument-sound` | `/types/138/records/903/children/0/element` |
| `interchangeable/beats` | `/types/140/records/920/children/0/element` |
| `interchangeable/beat-type` | `/types/140/records/920/children/1/element` |
| `metronome/metronome-relation` | `/types/174/records/1134/children/0/element` |
| `midi-instrument/midi-name` | `/types/183/records/1175/children/1/element` |
| `note/voice` | `/types/196/records/1345/children/0/element` |
| `part-link/group-link` | `/types/230/records/1580/children/1/element` |
| `play/ipa` | `/types/245/records/1691/children/0/element` |
| `player/player-name` | `/types/246/records/1702/children/0/element` |
| `score-instrument/instrument-name` | `/types/262/records/1784/children/0/element` |
| `score-instrument/instrument-abbreviation` | `/types/262/records/1784/children/1/element` |
| `score-instrument/instrument-sound` | `/types/262/records/1789/children/0/element` |
| `score-part/group` | `/types/263/records/1801/children/6/element` |
| `slash/except-voice` | `/types/269/records/1841/children/1/element` |
| `swing/swing-style` | `/types/309/records/2005/children/1/element` |
| `time/beats` | `/types/329/records/2159/children/0/element` |
| `time/beat-type` | `/types/329/records/2159/children/1/element` |
| `time/senza-misura` | `/types/329/records/2157/children/1/element` |
| `virtual-instrument/virtual-library` | `/types/357/records/2282/children/0/element` |
| `virtual-instrument/virtual-name` | `/types/357/records/2282/children/1/element` |
| `work/work-number` | `/types/365/records/2307/children/0/element` |
| `work/work-title` | `/types/365/records/2307/children/1/element` |
| `/records/2325/movement-number` | `/records/2325/records/2327/children/1/element` |
| `/records/2325/movement-title` | `/records/2325/records/2327/children/2/element` |
| `/records/2381/movement-number` | `/records/2381/records/2383/children/1/element` |
| `/records/2381/movement-title` | `/records/2381/records/2383/children/2/element` |

`TestDictionaryAllThirtyFiveDynamicStringCarriers` exercises these source routes;
`TestDictionaryAllTenAssessedSMuFLTypes` exercises the exact actual types.
Selection uses authoritative assessed declaration/type and scalar results;
attributes use the exact assessed attribute-use identity, including inherited or
dynamically nominated complex owners. Invalid nominations preserve their XSD
cause. An invalid scalar blocks only genuinely dependent dictionary conclusions.
The current model retains declared current fields, not discarded source
`xsi:type`; it is never encoded/reparsed to manufacture that history.

IPA and standard-sound semantic roles remain selected at their exact declarations
even when an admitted actual type also adds an independent SMuFL role. Legal
`xsi:type=xs:ENTITY` on these string carriers reaches the existing unavailable
unparsed-entity declaration-domain prerequisite; dictionaries do not bypass it.
Ordinary string lookup does not require generic temporal/binary/QName equality,
wildcard or UPA closure. Those prerequisites and DTD/default/constructed-identity
reconciliation remain separately owned in the
[XSD prerequisite ledger](xsd-prerequisite-scope.md).

## Fixed data, transformations and lineage

The authoritative compiled metadata lives in
[`dictionary_manifest.go`](../../validation/dictionary_manifest.go). The committed
source fixtures also serve as pinned offline generation inputs for the embedded assets.
`BundledDictionarySource()` supplies those exact bytes through the existing
importer. Inspection and standalone model/package assessment keep explicit
component/profile selection; the assessed loaders fix their semantic profiles
and default to required canonical SMuFL and IPA representation evidence. See the
[assessed-loading migration](assessed-loading-pipeline.md). All raw and transformed
identities are independently checked; a correct source certificate does not
authorize altered typed rows.

| Manifest | Raw component bytes / SHA-256 | Typed bytes / SHA-256 | Exact selected relation |
| --- | --- | --- | --- |
| `smufl-1.4-canonical-v1` | 301683 / `1d05352599a20983d1c901635dc75d76f063c0987a7bee65f145325fc3e0d29f` | 137769 / `00be19819ecac90e2463287a7f76d29e2333c6d1d71c95fb9ffdda4d4ecd1bcf` | 2,932 canonical names and their primary codepoints |
| `smufl-1.4-ranges-v1` | 98796 / `a22be610edadf823de387f4c11716beee0ba2a11534c2bae1897b23fb6869531` | 214363 / `f3cde9fd6b9deb403c315efcf6096b0738c83bfd407bddddfc1f9ae3f78aa54a` | 131 inclusive ranges and their enumerated names; 3,063 serialized rows |
| `musicxml-4.0-standard-sounds-v1` | 33703 / `a73d52b7215ea567dd54c5f2ea3e9d4f7ea167a40d5770a8fe0e3cb458858b35` | 30909 / `b6d3d12d9e33c47bb793fe7769190476bc8def765469b699a477aa51771edfaf` | 894 standard identifiers; nonstandard strings remain allowed |
| `ipa-2015-unicode13-positive-v1` | 26435 / `ee4d39dbf8e999d12ae4e0680fc1b55a9704b81a4f049a110bcf5a21c1da2901` | 9836 / `47b0a8812835e56e0f8b6527cca13b6d55bb4b3ee231186868e8ec8590804ec6` | Reviewed derived bundle: 170 symbols, 18 exact examples, nine exact canonical-equivalence inputs |

SMuFL pins [glyphnames.json][glyphnames] and [ranges.json][ranges] at commit
`753f551ca6f96bf24ba38aa2d8a317e706238c51`. Canonical names are exact keys, not
case-folded aliases, codepoint notation or font-specific optional glyphs.
`alternateCodepoint` is another encoding of one name, not an alias name.
The 85 researched classes are not a selected runtime component and cannot
replace MusicXML's six prefix-family facets. `gClef8vbCClef` is the canonical
spelling; the differing `gClef8vbCclef` sample spelling is not silently corrected.
`gClefSmall` and `flag8thUpStraight` are not canonical-map keys.

The reviewed range advice uses noteNameNoteheads E150–E1AF (96 listed glyphs),
noteNameNoteheadsSupplement EEE0–EEFF (27), and tremolos E220–E23F (21).
Canonical availability is a separate prerequisite for range conclusions.
`brassMuteClosed` E5E5, `brassMuteHalfClosed` E5E6, `brassMuteOpen` E5E7 and
`buzzRoll` E22A are fixed canonical names; local default selection remains
independent from availability of their dictionary evidence.

The standard-sounds component is [the exact MusicXML 4.0 sounds.xml][sounds]
at `799e2defb2ece0ae7bafe08dcbcac25b2c631d53`. Its external DOCTYPE and original
bytes remain intact. The private, digest-gated literal-ID transform is not
arbitrary XML/DTD validation and performs no external fetch. `instrument-sound`
retains `xs:string` whitespace; auxiliary `sound/@id` retains its separate `xs:ID`
normalization. No `any`/`solo`/`ensemble` library text or unrelated `sound`
element is selected, and a caller extension cannot modify the canonical set.

The IPA [positive ledger](../../testdata/dictionaries/ipa-positive/positive-ledger.json)
preserves original chart/mapping/Unicode pins and row locators. Runtime
certification verifies the compact **derived bundle**, not the IPA PDF or Unicode
files in that run. Typed records without a raw certificate establish only
reviewed-derived lineage. With a certificate, `ComponentVerified` plus
`SourceKind` keeps original-component verification distinct from derived-bundle
verification. Neither proves transport authorization, full IPA validity or XML
conformance. Hashes cover importer/transform and interpretation revisions;
actual selection, availability/reasons and lineage participate in package-session
snapshot capability identities. The graph cache mirrors this key defensively;
its fingerprint cannot vary within a run.

### Legacy positive-v1 limits and the additive witness

The unchanged 197-row positive-v1 policy retains these four limitations. The
separate [453-row representation policy](ipa-representation.md) establishes
explicit empty/SP/leading-mark witnesses, finite Unicode13 canonical closure
and source-established constructions for its affirmative language. It does not
turn these historical limitations into negatives, revise original issue status,
or claim phonotactics or universal repertoire conformance:

1. Complete, exact 2015 repertoire mapping versus historical, additional and
   extIPA symbols. The 2026 Doulos list is an encoding aid, not a 2015 normative
   snapshot; its additional-symbol section is not implicitly imported.
2. Reviewed canonical-equivalent encodings beyond the nine exact pairs, without
   compatibility folding. No general NFC/NFD closure or NFKC/NFKD rewrite is
   claimed. For example, `g` is not silently equated with U+0261, and a modifier
   letter is not folded into its base letter.
3. Explicit empty-string, SP/TAB/LF/CR/NBSP, bracket and slash dispositions.
   Present empty content remains applicable and unknown. No nonempty requirement,
   automatic trimming or invented leading-mark/combining-count prohibition exists.
4. A justified sequence interpretation separate from phonological plausibility
   and intended pronunciation. Attested examples and all-positive atoms do not
   satisfy the whole-sequence predicate. Stricter application policy would need
   its own named scope and cannot become a MusicXML normative defect by default.

Unlisted symbols/encodings are unknown, never authoritative negative evidence.
Individual spans preserve exact UTF-8 text and byte offsets. Full repertoire,
sequence and intended-pronunciation obligations cannot be closed by extending a
finite positive table or increasing its test count.

## Ownership, failures and package propagation

The [API contract](../../API.md#account-lifecycle-and-concurrent-use) specifies
same-account construction/borrowing, pre-allocation accounting, terminal causes
and owner/session lifetimes. Core sets own copied strings/slices and no caller
map/buffer. `Account` is concurrency-safe and each `Session` is single-goroutine; independent
sessions may share one account/set concurrently. Concurrent owner `Close` rejects
new borrows while already admitted borrowers can finish. Storage
is freed once after the last borrower; owned reports need no live set.

One root transport/import/index set is shared by all selected graph children,
with no second external-byte charge at certificate verification and no child
budget reset. Temporary source, decoded records and final copies are admitted
for simultaneous live storage. Scalar caps also apply to derived defaults and
nested input/effective/span/equivalent text before publication. Every
transferred usable body closes once even on read/metadata/close failure. Wrong
request/pin/kind errors on the four legacy components and ordinary provider
errors become capability gaps and may accompany a partial result. For the new
representation manifest only, importer-owned malformed structure/length/hash
errors preserve ConfigurationError and cannot return a usable set; they do not
suppress independent XSD/family failures. Actual cancellation/resource exhaustion
is terminal, with original causes and earlier proved violations preserved.

The graph retains dictionary context in selected intrinsic snapshots and remaps
all occurrence-qualified subjects and nested causes on adoption. Two aliases of
one canonical resource have two occurrence views; the shared verified lookup
store does not make their document identities interchangeable. Source, current
model, raw MXL and current MXL outcomes remain separately scoped, including
incomplete children. An unselected dictionary source is never invoked.

### Observed default-budget limit

The historical review at unpublished local checkpoint `229e4fd`
selected all four exact components for a repeated-rest fixture with one
canonical accidental attribute per note. Standalone source/model assessment
completed at 320 notes and reached the unchanged 20,000,000-rule-step cap at
321. Graph and raw/current MXL completed operational enumeration at 316 and
reached that cap at 317. Current-model bundles still retained an independent
schema-hint unknown even when their intrinsic assessment completed. These are
fixture-specific observations, not universal note-count capacities, bundle
acceptance or B5-PERF-01 closure. See the [measured
conditions](pinned-dictionaries-evidence.md#default-budget-observations).

The historical PR 40 Round 1 correction recheck observed 368/369 for standalone
source/model and 363/364 for graph/MXL with those same finite limits. These observations have
not been remeasured after the corpus resource corrections and are not
current-head capacity claims. See the paired
[correction measurements](pinned-dictionaries-evidence.md#round1-correction-measurements-2026-10-14)
for exact work counters, fixture qualifications and unchanged capability gaps.

## Source licenses and distribution boundary

- SMuFL 1.4 metadata retains its 2021 contributor copyright, edition attribution
  and [W3C Community Final Specification Agreement notice][smufl-license].
  MusicXML sounds retains its 2004–2021 contributor notice and the [FSA][fsa].
  Imported metadata is not relabeled MIT.
- IPA-derived ledger/runtime data are separately distributed as CC BY-SA 4.0
  with historical chart CC BY-SA 3.0 attribution and the later 2026 mapping aid's
  CC BY-SA 4.0 attribution preserved. The documented engineering disposition
  keeps adapted data distinct from independently authored MIT implementation
  code. It is not a legal guarantee for every downstream combination.
- Unicode-derived fields retain the adjacent Unicode License V3 notice. That
  is the preserved current 1991–2026 snapshot, not a claimed original 2020 license
  snapshot; the historical UCD copyright statements are also retained.
- The [project notices](../../THIRD_PARTY_NOTICES.md),
  [music-data notices](../../testdata/dictionaries/NOTICES.txt),
  [legacy IPA notices](../../testdata/dictionaries/ipa-positive/NOTICES.txt) and
  [representation notices][representation-notices] accompany the source data
  and must be preserved with generated equivalents. No chart artwork, PDF, font,
  outline or Unicode specification text is bundled.

Any binary referencing `DecodeAssessed`, `DecodeMXLAssessed`, `InspectXML`,
`AssessModel`, `InspectXMLWithResources`, `AssessModelWithResources`,
`InspectMXL` or `AssessMXLPackage` embeds
all five component assets, including both CC BY-SA 4.0 IPA bundles, even with
custom-source options. Direct use of `BundledDictionarySource` also retains all
five. Selection limits runtime import and assessment, not the binary's asset
references. Distribute these binaries with all five files from
`internal/dictionarydata/data`:

- `NOTICES.txt`
- `ipa-positive/NOTICES.txt`
- `ipa-positive/LICENSE-UNICODE-V3.txt`
- `ipa-representation/NOTICES.txt`
- `ipa-representation/LICENSE-UNICODE-V3.txt`

Their embed patterns preserve the notices in `go mod vendor` output; a binary
distributor must still ship them in the distribution or associated
documentation. The [bundled-data distribution instructions][bundled-notices]
give the complete paths and commit-pinned links to the full IPA
modification/provenance records, which remain in `testdata/dictionaries`
rather than the runtime asset directory. Actual release packaging and notice
inclusion remain acceptance checks.

Independent positive-data review accepted the scoped provenance/notice
classification with these caveats. It did not grant publication permission,
accept a new agreement or complete runtime/release review.

## Remaining program ownership and stopping condition

This slice does not close `REQ-DEF-EXTERNAL-SPECS` or
`REQ-DEF-ISSUE-EXTERNAL-SPEC-CLOSURE`; declaration-domain/ENTITY, general
XML/XSD/XLink/XML Base/ID and ZIP/JAR/DEFLATE closure remain with their original
prerequisite/package owners. Full glyph/type, specialized-markup, arrow and
mordent semantic relations remain with the specific crosswalk owners above.
B5-PERF-01 remains a separate kernel/session performance obligation.

`REQ-DEF-LANG-291` and research portability are not silently retired. The prior
177 reviewed English triplets remain evidence; the separate historical 114-triplet
artifact retains its recorded fingerprint and current portability gap. This
slice does not retranslate them or restart canceled lost-folder recovery.

The selected dictionary slice can be accepted only after its exact infrastructure,
consumers and all selected package representations pass independent semantic and
resource/security review plus exact-head repository checks, with the retained
IPA/semantic gates stated explicitly. That acceptance would still not be final
program readiness. Original record updates, if later authorized, require separate
per-ID reconciliation; do not replace the original denominators with the subset
implemented here.

[glyphnames]: https://raw.githubusercontent.com/w3c-cg/smufl/753f551ca6f96bf24ba38aa2d8a317e706238c51/metadata/glyphnames.json
[ranges]: https://raw.githubusercontent.com/w3c-cg/smufl/753f551ca6f96bf24ba38aa2d8a317e706238c51/metadata/ranges.json
[sounds]: https://raw.githubusercontent.com/w3c-cg/musicxml/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/sounds.xml
[smufl-license]: https://w3c-cg.github.io/smufl/releases/1.4/preamble/license.html
[fsa]: https://www.w3.org/community/about/agreements/final/

[published-lineage]: pinned-dictionaries-evidence.md#published-implementation-lineage

[representation-notices]: ../../testdata/dictionaries/ipa-representation/NOTICES.txt
[bundled-notices]: ../../THIRD_PARTY_NOTICES.md#embedded-copies-of-the-pinned-dictionary-data
