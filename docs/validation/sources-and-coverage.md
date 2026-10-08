# Sources and coverage

This is the source-accounting record for a **proposed** MusicXML 4.0 validation
architecture. The accompanying catalog is research evidence, not an implemented
validator, an approved conformance profile, or a test-coverage report. Counts and
verification results below describe the recorded research run of 8 October 2026.

## Version and provenance

- Publication: [MusicXML 4.0, Final Community Group Report, 1 June 2021][report].
  It is not a W3C Recommendation or a W3C Standards Track specification.
- Official release: [v4.0][release], pinned to commit
  `799e2defb2ece0ae7bafe08dcbcac25b2c631d53`.
- Formal scope: the six schemas in the dated [File Listings][listings]:
  `musicxml.xsd`, `opus.xsd`, `container.xsd`, `sounds.xsd`, `xml.xsd`, and
  `xlink.xsd`. Current editor's drafts and later MusicXML versions are not sources
  of 4.0 requirements in this catalog.
- XSD language references: [XML Schema 1.0 Structures, Second Edition][structures]
  and [Datatypes, Second Edition][datatypes]. The catalog's language summaries do
  not replace these specifications.

[Source provenance](registry/xsd/source-provenance.json) records raw-file,
dated-listing HTML, and operative-tree SHA-256 hashes, exact public URLs, and the
method used for each schema. The verification is deliberately narrower than a
fresh byte-for-byte download of all six pinned Git files:

1. Five schema copies matched the raw SHA-256 hashes retained from an earlier
   official-tag audit: `musicxml.xsd`, `opus.xsd`, `container.xsd`, `xml.xsd`, and
   `xlink.xsd`.
2. All six operative schema trees independently matched the dated W3C listings.
   This comparison excluded annotations and comments because rendered HTML can
   change escaping and whitespace. Prose was reviewed separately.
3. `sounds.xsd` came from a vendored copy and has no corresponding prior-run raw
   hash record. Its reported verification is the complete operative-tree match,
   not a newly verified raw Git object.
4. The recorded release check saw commit prefix `799e2de`; the full pin used
   earlier audit provenance and a successfully opened full-commit raw URL. The
   provenance explicitly records no new full Git-object verification.

The HTML study separately obtained the [official archive at the pinned
commit][archive]. Its [provenance](registry/prose/github-archive-provenance.json)
records 5,634,909 bytes and SHA-256
`57d8e41fae85e1c5b3747c8bb036bf7b56629904b2550f0c1892385e735c3b52`.
That later archive retrieval does not retroactively strengthen the recorded
schema-byte verification method.

### Upstream locators and the repository copy of opus

Registry `source_url`, `source_line`, XPath, and annotation hashes refer to the
pinned upstream representation identified by that record. They do not promise
the same line numbers or annotation bytes in a repository's vendored copy.
A follow-up comparison at repository commit
`e486735cd6e4537e839ff67704b7768a9df3bdb3` found:

- Pinned [upstream opus.xsd](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/opus.xsd)
  has SHA-256 `e8256607b9255075f16c1d70ed53f8384ab6e3b9dc4c1f81291cdd1d56b0474a`.
- The [repository copy](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/schema/musicxml-4.0/opus.xsd)
  has SHA-256 `e490ee703d40a27616a36ca3c3f482649e6dbc9a46870c647f4557d2193f66bd`.
- Named opus occurrence line locations and three annotation text hashes differ
  against the local copy. The difference includes annotation wording as
  well as formatting. Operative schema trees agree when annotations, comments,
  and insignificant whitespace are excluded.

The upstream locators remain valid. All 642 annotation hashes match the pinned
upstream inputs; 639 match the vendored schemas at that repository commit.
Use the record's upstream URL for evidence review rather than substituting the
local file at the recorded line. No schema is changed by this documentation update.
HTML sources instead use dated-page/section locators and archive provenance;
they need not have an XSD XPath or source-line field.

## Formal schema inventory

The [source coverage](registry/xsd/source-coverage.json) and
[combined reconciliation](registry/coverage-reconciliation.json) account for:

| Schema | All XSD nodes | Operative occurrences | Documentation blocks | Explicit properties | QName reference tokens |
| --- | ---: | ---: | ---: | ---: | ---: |
| `musicxml.xsd` | 4,052 | 2,826 | 613 | 3,801 | 1,463 |
| `opus.xsd` | 48 | 30 | 9 | 48 | 17 |
| `container.xsd` | 21 | 11 | 5 | 19 | 5 |
| `sounds.xsd` | 46 | 30 | 8 | 46 | 15 |
| `xml.xsd` | 31 | 19 | 6 | 19 | 9 |
| `xlink.xsd` | 25 | 23 | 1 | 25 | 6 |
| **Total** | **4,223** | **2,939** | **642** | **3,958** | **1,515** |

There are also 642 annotation wrapper nodes: **4,223 = 2,939 + 642 + 642**.
All 2,939 operative occurrences map to a primary catalog contract; no operative
kind, explicit property, or QName reference was left unresolved in the recorded
reconciliation. Type/group reuse is retained as dependencies rather than counted
as new source occurrences.

The inventory includes 472 global components, 155 simple types and 242 complex
types (including 15 anonymous types), 718 effective particle sites, 677 facet
occurrences, 1,453 effective attribute uses, and 19 ID/IDREF declaration-or-reference
sites. These are different inventory measures and must not be added together as
independent rules. There are five document roots:

| Assembly | Roots | Imported modules |
| --- | --- | --- |
| `musicxml.xsd` | `score-partwise`, `score-timewise` | `xml.xsd`, `xlink.xsd` |
| `opus.xsd` | `opus` | `xml.xsd`, `xlink.xsd` |
| `container.xsd` | `container` | None |
| `sounds.xsd` | `sounds` | None |

All five roots have no namespace. The four subject schemas are separate
assemblies; similarly named declarations must not be merged across them.
`xml.xsd` and `xlink.xsd` contribute namespaced declarations, not extra document
roots. Importing either module does not permit all its attributes on every
element.

## Published prose corpus and exclusions

The [coverage ledger](registry/prose/coverage-ledger.json) accounts for **974 discovered
dated HTML URLs**, with no unretrieved URL in the scoped corpus. Of these, 260 were
retrieved directly from the dated W3C publication and 714 from the pinned official
archive. All 260 overlapping pages had matching main text in the
[archive comparison](registry/prose/archive-equivalence.json); this is not a
claim of byte-identical HTML or direct W3C retrieval of the other 714 pages.

| Final page disposition | Pages |
| --- | ---: |
| Narrative or navigation reviewed | 29 |
| Reference text and attribute/value descriptions reviewed | 624 |
| Illustrative examples or example indexes screened as non-normative | 306 |
| Non-prose implementation/dictionary listings excluded | 10 |
| Prior-version history excluded from 4.0 norms | 5 |
| **Total** | **974** |

The 10 excluded listings cover XSLT, XML catalog, and sounds data rather than new
prose requirements. Example narrative was screened, but example XML and images
do not establish universal constraints. Earlier-version histories were not used
as 4.0 norms. A final disposition means that a page was reviewed or explicitly
excluded; simply having retrieved it was not treated as proof of reading.

The review records 14,569 HTML sections; all 642 schema documentation blocks;
37 schema comment blocks; 11 tutorial pages including the index; 444 MusicXML
element pages and 159 datatype pages excluding their indexes; 370 unique attribute
descriptions; 414 datatype value-description rows; 203 reference introductory
paragraphs not automatically matched to annotations; and 9 additional explanatory
paragraphs from examples. These measures overlap and are not additional page
totals. The repository's [coverage ledger](registry/prose/coverage-ledger.json)
retains all 974 page records and 642 annotation records with their dispositions;
see also [attribute coverage](registry/prose/html-attribute-coverage.json) and
[prose verification](registry/prose/verification.json). The original 14,569-block
HTML hash index is omitted from Git. Its aggregate counts and the original full
ledger's SHA-256 are retained, but its detailed block evidence remains in the
original research archive and cannot be reconstructed from these page summaries.

## What the catalog counts

The [combined catalog](registry/catalog.json) has **2,560 records**:

- 2,218 XSD-side contracts: 2,171 explicit contracts, 30 XSD-language topic
  records, and 17 builtin-datatype records.
- 342 prose records: 51 constraints, 58 recommendations, 143 interpretations,
  73 defaults, 14 application-dependent cases, and 3 descriptive records.
- The **28 issue records** are separate; see [open questions](open-questions.md).

These are not 2,560 independent mandatory predicates or future error codes. A
contract may aggregate several clauses; derived views may refer to the same
source constraints; enumeration alternatives form a value domain rather than
separate requirements to equal each value. Prose classification does not establish
runtime severity, and an interpretation or default need not describe an invalid
document at all.

[Reconciliation](registry/reconciliation.json) labels prose/XSD overlap as 19
full, 43 partial, and 280 none. Those are contract-context links, not a completed
predicate-level equivalence proof. An issue link usually means shared source
context; it does not automatically invalidate every rule from that page or type.
Stable source, contract, prose, and issue IDs and all original fields remain
unchanged. Additive English descriptions/conditions cover the 51 constraints;
English descriptions/dispositions cover all 28 issues; titles, explanations, and
diagnostics cover the six illustrative rules. The other 291 prose records retain
their original descriptions pending translation. These additions do not change
normative classification, source evidence, or predicate semantics.

## What was verified, and what remains outside the claim

- The recorded [independent static check](registry/xsd/independent-verification.json)
  ran 22,162 source-to-registry checks with zero failures. The
  [schema assembly sanity check](registry/xsd/schema-compilation-check.json)
  compiled all six schemas with local import resolution using libxml2. It ran no
  document-instance or product tests.
- The [cross-registry check](registry/coverage-reconciliation.json) reconciled
  unique rule IDs, supplied prose-to-XSD and prose-to-issue links, and all 642
  documentation IDs and text hashes. This verifies recorded relationships, not
  the universal correctness of the extracted interpretations.
- Published-source inventory and scoped reading are accounted for, with the
  exclusions above. Runtime predicates, a concrete Go-model capability mapping,
  and a rule-by-rule product test ledger are not implemented by this proposal.
  No implemented coverage percentage or claim about existing repository behavior
  follows from these counts.
- Full transitive external normative closure is **not claimed**. XML, namespace,
  XSD datatype, XLink, XML Base/ID, ZIP/JAR/DEFLATE, SMuFL, and IPA dependencies
  still need scoped treatment wherever a future conformance claim depends on
  them. In particular, a prefix check is not canonical SMuFL dictionary
  validation, and a Unicode-block check is not full IPA validation.
- XML parsing prerequisites, network/entity policy, archive path safety, and
  resource limits need explicit implementation boundaries. `container.xsd` does
  not validate a ZIP archive. Security policy and application heuristics must not
  be presented as new MusicXML norms. See
  [scope and applicability](registry/xsd/scope-and-applicability.json).

## Attribution and hash boundaries

The [publication's attribution][report] credits the Contributors to the MusicXML
Specification (2004–2021), published by the W3C Music Notation Community Group
under the [W3C Community Final Specification Agreement][fsa]. Source URLs and
locators are retained for attribution and review. This documentation contribution
does not copy the schema files, HTML corpus, or archive into the repository; its
research records contain structured facts and paraphrases, not a replacement
distribution of those sources. This attribution does not relicense the upstream
specification under the repository's license.

Keep three integrity scopes distinct:

1. Original-source hashes in
   [XSD provenance](registry/xsd/source-provenance.json),
   [archive provenance](registry/prose/github-archive-provenance.json), and the
   prose ledgers identify the research inputs and their representations.
2. [Packaging provenance](registry/packaging-provenance.json) records original
   research-file digests, exclusions, and transformations. The English README is
   regenerated; Russian narrative reports and duplicate CSV views are omitted.
   The explicit-property table is omitted because its 3,958 property IDs and values
   remain in the occurrence records, with the labels retained in a property
   glossary. The separate effective-contracts file is omitted: the catalog
   retains the 242 complex-type and 155 scalar-type contracts, while additional
   group-expansion, root-reachability, and 19 identity-site projections are
   derivable from the occurrence/reference graph, not copied verbatim into the
   catalog. The XSD coverage file refers to the retained prose annotation records
   rather than repeating their handoff data; their IDs and hashes were verified
   to match. The detailed HTML block index is omitted as described above.
3. [The regenerated registry manifest](registry/manifest.json) records current
   packaged registry files, separately from original research-file and source
   hashes. Its hashes attest to packaging integrity only; they neither replace
   source hashes nor strengthen source-verification or implementation-coverage
   claims.

[report]: https://www.w3.org/2021/06/musicxml40/
[release]: https://github.com/w3c-cg/musicxml/releases/tag/v4.0
[listings]: https://www.w3.org/2021/06/musicxml40/listings/
[archive]: https://codeload.github.com/w3c-cg/musicxml/tar.gz/799e2defb2ece0ae7bafe08dcbcac25b2c631d53
[structures]: https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/
[datatypes]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/
[fsa]: https://www.w3.org/community/about/agreements/final/
