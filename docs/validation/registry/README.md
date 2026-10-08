# MusicXML 4.0 research registry

This is the evidence base for the [proposed validation architecture](../architecture.md).
It contains research contracts and interpretations, not executable validation code.

## Canonical data

- [catalog.json](catalog.json): 2,560 records with original fields preserved, comprising 2,218 XSD
  contracts and 342 prose records. Each entry preserves its original ID and record.
- [reconciliation.json](reconciliation.json): XSD/prose links, overlap categories,
  and deduplication cautions. Context links are not predicate equivalence.
- [issues.json](issues.json): 28 editorial, ambiguity, and dependency
  records. See the [English issue guide](../open-questions.md).
- [rule-definition-examples.json](rule-definition-examples.json): six illustrative
  declarations, not a complete runtime catalog or frozen API.
- [coverage-reconciliation.json](coverage-reconciliation.json): research counts,
  reconciliation results, and separate coverage axes.

The `_ru` fields and all original evidence are retained. Additive `_en` fields
cover all 51 prose constraints, 28 issues, and six illustrative rules. The other
291 prose records do not yet have English descriptions/conditions. Translation
does not change IDs, source locators, classifications, predicates, or dispositions.
Research
metadata such as `product_tests_created_or_run: false` describes the research
phase, not any later repository CI run.

## XSD evidence

- [occurrence-registry.json](xsd/occurrence-registry.json) preserves all 2,939
  operative schema occurrences, including their explicit attributes and locators.
- [property-glossary.json](xsd/property-glossary.json) describes the 19 property
  names. The 3,958 explicit property IDs are occurrence ID + `@` + attribute name;
  their values are in each occurrence's `attributes` object.
- [occurrence-requirement-map.json](xsd/occurrence-requirement-map.json) maps source
  occurrences to schema contracts. To reconstruct it, invert `source_occurrence_ids`
  in XSD catalog records, excluding `language_semantics` and `builtin_datatype`.
- [reference-graph.json](xsd/reference-graph.json) records all 1,515 resolved QName tokens.
- [language-semantics.json](xsd/language-semantics.json) includes XSD language
  requirements and the separate semantics of the four standard xsi attributes.
- [scope-and-applicability.json](xsd/scope-and-applicability.json) records target
  boundaries and required evidence.
- [source-coverage.json](xsd/source-coverage.json) and
  [source-provenance.json](xsd/source-provenance.json) record source counts,
  fingerprints, verification methods, and their limitations.
- [independent-verification.json](xsd/independent-verification.json) and
  [schema-compilation-check.json](xsd/schema-compilation-check.json) record research
  checks, not product conformance or rule-test coverage.

## Prose evidence

- [coverage-ledger.json](prose/coverage-ledger.json) preserves all 974 page records
  and 642 annotation records, their dispositions, hashes, and aggregate counts.
- [html-attribute-coverage.json](prose/html-attribute-coverage.json) records
  attribute-description coverage.
- [archive-equivalence.json](prose/archive-equivalence.json),
  [github-archive-provenance.json](prose/github-archive-provenance.json), and
  [verification.json](prose/verification.json) record the pinned archive and checks.

## Repository packaging

[packaging-provenance.json](packaging-provenance.json) identifies omitted or
repackaged original files and preserves their SHA-256 digests. Duplicate CSV
views and Russian narrative reports are omitted. English documentation provides
the reading guide and source-coverage caveats.

The separate explicit-property table is replaced by occurrence attributes and
the property glossary. The derived effective-contract export is omitted: all
242 complex contracts and 155 scalar domains are already in the catalog. Its
additional group expansions, root reachability closures, and identity-site
projections can be derived from the occurrence inventory and reference graph;
the exact serialized projection is not retained.

The 14,569-entry HTML block hash index remains archival research evidence. Its
aggregate count and original containing-file checksum are retained, along with
all page and annotation coverage. The omitted individual block hashes cannot
be reconstructed from these summaries.

[manifest.json](manifest.json) hashes the files actually published in this
registry, excluding the manifest itself. Its hashes describe these repository
files. They are distinct from upstream-source hashes and original research-file
hashes. No raw HTML cache, downloaded archive, schema copy, temporary script,
or product source code is included here.
