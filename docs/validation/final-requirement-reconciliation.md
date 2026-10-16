# Final requirement reconciliation: baseline and PR 42 evidence

Date: 16 October 2026. This is an author-produced development reconciliation,
not independent design/QA acceptance or an integrated-tree execution certificate.
Requirement classifications retain merged baseline commit
`0b5a2f24b89ef628d2d09b361cbcbe390b9f01b9`, tree
`1e78190afb839ba0cd5b91bc5fb8bff6a102638d`. Regression routes added by PR 42 are
explicitly labeled `pr42-added`; they are not attributed to that baseline.

The additive [machine-readable ledger](final-requirement-reconciliation.json)
retains every original requirement and attached test contract: exactly 95 and
95. Original records, statuses, IDs, acceptance clauses and five-axis case texts
are unchanged. Historical `planned` and `decision_required` statuses are not
proof that current implementation is missing. Conversely, a related test name,
a successful corpus run or an Unknown result is not proof of closure.

## What the rows establish

Each requirement has its original pointer and unmodified record, applicability,
targets, implementation, code paths, interpretation/decision sources, individual
prior ownership pointers, directly matching functional crosswalk rows, execution
disposition, residual and closure criterion.

Each test contract retains positive, negative, boundary, missing-fact and
interaction axes separately. All 127 concrete regression routes are used by at
least one axis. They carry input/outcome descriptions and unique source anchors
bound to the named test or its directly referenced fixtures. There are 123
baseline routes and four PR 42 additions:

- `TestAssessedDecodeSkippedSubtreeXMLContracts` and
  `TestAssessedDecodeSkippedDistinctAttributesRemainWellFormed` in
  `assessed_skipped_xml_contract_test.go` distinguish lexical/expanded-name
  duplicates from whitespace errors, with ordinary and strict decode controls.
- `TestFinalReconciliationRejectsConsistentlyUnlinkedMatrix` in
  `internal/finalreconciliation/review_mutations_test.go` checks missing source
  graph edges even when the ledger and source fingerprints change consistently.
- `TestFinalReconciliationRejectsHistoricalExecutionPromotion` in
  `internal/finalreconciliation/ledger_test.go` guards this ledger's historical
  execution boundary. It does not prove general profile readiness.

There are 78 implemented-awaiting-current-verification rows, 13 explicit bounded
limitations, two unresolved evidence/execution decisions and two
optional-not-elected rows. No row claims final-tree verified completion or a
newly established missing runtime algorithm.

The five axes have 91 mapped, 228 partial, 140 decision-only, ten not-elected
and six missing-evidence dispositions. These are evidence classifications,
not runtime pass/fail totals. In particular:

- A partial mapping retains the exact original illustrative text and identifies
  the clauses that its related assertions do not prove. It does not assert an
  absent implementation or absent current regression.
- RESULT-AXES/negative retains a baseline gap: Unknown-only report behavior and
  strict unavailable-witness rejection are asserted separately, without jointly
  excluding normative failure. The PR-added
  `TestFinalConfigurationSelectedNilDictionaryPublicReports` supplies a later
  combined public contract, documented in
  [the configuration matrix](final-configuration-contracts.md).
- XML-SKIP/interaction retains the unasserted observer-delivery ordering clause;
  malformed skipped-XML detection alone does not prove unfiltered fact delivery.
- VERIFY-PROFILE/missing_fact is partial because its PR-added promotion guard
  protects this ledger, rather than implementing general coverage accounting.
- The 28 original issue requirements concern interpretation decisions. Their
  140 axes retain issue-specific examples and the scoped interpretation; a
  generic schema regression is not independent proof of each example.
- The IPA negative axis is limited by authority: the selected positive witness
  corpus supplies no complete normative negative complement. Returning Unknown
  does not certify the broader historical axis.

## Original inventories and changed interpretations

All 28 original issue IDs and their original records are retained individually.
Their dispositions separate formal XSD authority, bounded source-backed
interpretation and genuine uncertainty. They do not turn the original
`hard_error_basis=false` gate into independent rejection authority.

All 29 candidate IDs retain their historical disposition pointer and local,
timeline, context or functional crosswalk disposition. This keeps later barline,
chord, numeral, repeat, figure-number and intent corrections from being
replaced by older batch labels. Linked context/prose/XSD checks, contracts,
overlaps, 2,218 XSD instances, 42 expanded source contracts and both functional
crosswalk inventories are retained by their actual IDs. They overlap and are
not added to the 95-requirement denominator.

## Open obligations and exclusions

The 291 translation identities are individually partitioned into 177 committed
reviewed triplets and 114 missing-portable-evidence records. The latter are the
exact complement of committed record IDs. Their historical fingerprint does not
provide portable reviewed triplet bytes. `TRANSLATION-114-PORTABILITY` remains
open with no waiver or recovery authorization. Recovery and retranslation remain
canceled; resolving this residual requires a separately approved disposition.

B5-PERF-01 remains open for separately authorized matched measurement and a
reviewed improvement or explicit reviewed residual-cost disposition. Logical
budget controls, old benchmarks and CI duration do not close it. This work
performs no benchmark or performance tuning.

Bundled dictionary generation and explicit selection were added outside the
baseline by published PR 42 commits `8e9e081`, `5beff1b` and `1a62c10`; see
[bundled data](../../internal/dictionarydata/README.md). The public-API
configuration matrix is a separate PR 42 artifact. Neither addition
retroactively verifies the baseline requirements. Integrated-tree acceptance
receipts are recorded in [PR 42](https://github.com/go-muse/go-musicxml/pull/42).
Independent design/QA, mass real-corpus acceptance and performance assessment
remain outside this ledger.

## Integrity controls and author verification

The narrow test-only package `internal/finalreconciliation` checks:

- Exact original records, five axes and linked inventory identity sets
- Nonempty mandatory requirement/test edges and reciprocal links in the source
  graph itself, independently of ledger/source equality
- All eight preserved source paths and fingerprints, both retained obligation
  IDs and their open states, and required implementation/decision/history links
- Unique same-file anchors inside the named test or directly referenced
  fixtures, at least one anchor in the test body, and no unused routes
- Pinned per-row classification/residual/closure policy, per-axis assessment,
  basis/gap/routes, and case descriptions, anchors and evidence layers

These pins detect unreviewed policy drift. They do not interpret arbitrary
prose or establish runtime conformance. The Unknown-only closure mutation now
changes only the closure criterion and is rejected by its reviewed policy pin;
it does not rely on the separate ban on `implemented-and-verified`.

The round-one mutations first reproduced accepted residual retirement, axis
promotion, missing sources/links/obligations, consistently unlinked source
records, unrelated anchors and removal of a named test body. Narrow repairs
then rejected those same mutations. Focused author verification uses the
repository Go 1.27.2 environment and the shared build lock:

```sh
go test -count=1 ./internal/finalreconciliation
```

The integrity/mutation suite passed on linux/amd64. Focused runtime checks cover
only the corrected budget, result, nil and skipped-XML routes; the allocation
test's 32-bit branch remains a mapped assertion, not a 386 execution receipt.
No full root suite, seven-stage local acceptance, external oracle, race, fuzz,
hosted CI or 386 run is claimed by this reconciliation.
