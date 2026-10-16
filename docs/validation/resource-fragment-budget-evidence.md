# Fragment lookup and source-origin transport budgets

This receipt covers PR 41 review items 4291055418, 4291055432 and 4291055313,
including the reopened capacity observation in comment 4293498146.
It records bounded resource-accounting corrections and the measured distinction
between original and DOCTYPE-free official sample bytes. Default budgets,
public fact layouts, catalog identities and semantic interpretations are unchanged.

## Once-owned document-order index

The previous shorthand/`element(id)` implementation nested every XSD and DTD ID
scan inside the element loop. A 10,000-element, 1,000-ID absent lookup reaches the
default 20,000,000 rule-step limit before completing one query.

The snapshot now constructs one private `NodeRef` to document-order index after
admitting its full lease. Keys borrow strings from the snapshot-owned element
array. The map is neither exposed nor remapped for adoption, and `release` clears
it with the other snapshot storage. Its allowance is 512 bytes plus 128 bytes per
element. A compile-time assertion covers twice the fixed key, value and control
word sizes, including spare/growing map storage on native and 32-bit layouts.

Each lookup scans each selected ID authority once, then compares indexed document
orders for matching values. NodeID magnitude and schema uniqueness never choose
the winner. The unchanged whole-domain gates return Unknown before matching if
XSD identity, selected DTD identity, or element enumeration is incomplete.
Multiple declarations on one owner still select that owner; duplicate owners
still select the first occurrence in document order.

The absent query `absent-id` costs exactly `1 + 10 * ID-count` rule steps after
index construction: 1,001 for 100 IDs and 10,001 for 1,000 IDs, independent of
whether the document contains 1,000 or 10,000 elements. Matching owners also
admit their document-identity hashing work. Repeated lookups and adoptions reuse
the index; both graph traversal and final link decisions remain metered.

`TestReviewFragmentIDLinearWork`,
`TestReviewFragmentIndexJoinedOrderAndUnknownDomains`,
`TestReviewFragmentIndexExactWorkAndCancellation`,
`TestReviewFragmentIndexSnapshotAdmissionAndReuse`, and
`TestReviewFragmentIndexSnapshotPostAdmissionCancellation` cover the core path.
`TestReviewFragmentGraphMediumScoreDefaultRuleBudget` exercises 2,000 measures
with 2,000 IDs and three `#m1` links through the public graph entry point. The
target `m1` is the final measure in document order, rather than an early return.
The 86,052-byte control uses empty measures and raises only the buffer allowance to
512 MiB, isolating the unchanged default 20,000,000 rule-step ceiling. It asserts
three retained link contexts, four distinct adoptions and exactly three package
Pass outcomes, with complete enumeration and a satisfied bundle. The same control
on unpublished local baseline `9f49784` (tree-equivalent to published
`159dd6e`) refuses rule steps with 19,999,999 used and 3 requested against the
20,000,000 limit. Adding pitched notes to every measure is a larger intrinsic
assessment: that separate variant
still reaches the shared work limit during intrinsic snapshot finalization,
before package decisions. This correction does not promise that every
2,000-measure composition fits the default work budget.

## Closed source-origin copy sizing

All selected origin rows remain in both the canonical intrinsic report and the
selected snapshot projection. Both independent copies are charged while session
storage still overlaps. The previous generic 32-byte allowance at every inline
field repeatedly counted nested record headers. The new closed-record path
counts a containing record once and separately admits its owned allocations:

| Storage made by `resourceCopyInto` | Admission |
| --- | --- |
| Root record or nonnil pointer pointee | Concrete fixed type size plus 32 bytes |
| Inline struct, scalar or nil pointer | Already included in its containing record |
| Nonnil slice backing storage | Exact length times fixed element size, plus 32 bytes |
| Each nonempty cloned string occurrence | Twice its byte length plus 16 bytes |
| Each remapped NodeRef/Location identity | Additional twice destination length plus 16 bytes |

The original identity clone remains charged when a longer adoption identity
replaces it. Repeated source pointers/strings are counted separately; the copier
does not preserve mutable aliases. Slice source spare capacity is not copied.
All additions and count products are checked before storage admission. Traversal
and bounded string-copy work remain charged and cancellation-aware.

The closed payload inventory is:

- Subject document identity; optional Attribute QName; Kind
- Specified and AncestryComplete evidence, including state, reason and optional
  causal decision with its rule and subject identity
- Physical, Declaration and Use evidence, including each Location's target,
  document identity, entry, path and optional physical position/coordinate space
- Exact-length Ancestry backing storage; each step's entity string and both
  declaration/use evidence sites, with their independent positions and causes

At the leaf level, `NodeRef` contains `DocumentID` and scalar `Node`; `QName`
contains `Space` and `Local`. Each `Evidence` contains `State`, `Value`, `Reason`
and optional `Cause`; every cause owns `Rule.ID`, scalar `Rule.Revision` and its
subject `NodeRef`. Every position owns `CoordinateSpace` and scalar `ByteOffset`,
`Line` and `Column`. The containing fixed records include all scalar values,
string/slice headers and nil/non-nil pointer slots. Zero, nonnil-empty,
maximal-ancestry and spare-capacity source shapes are checked; copies retain
exact length/capacity, including nonnil-empty slice identity.

The canonical report and selected-context collection are still separate owners.
Getters and repeated adoptions still create independent copies. Unknown component
states, original positions and arbitrary strings equal to a document identity
retain their previous interpretation. No origin row is filtered out.

The maximal-ancestry, all-string and independent concrete-storage checks are
`TestResourceRecordMeasurementCoversClosedCloneShapes`,
`TestResourceRecordMeasurementCountsEveryClonedString`, and
`TestResourceRecordMeasurementDoesNotBoxFields`. Existing source-origin transport
regressions additionally cover exact/one-short overlapping snapshot admission,
every post-admission cancellation point, mutation isolation, repeated adoption,
longer identity remapping, rejected transfer and account reuse.

The 500-measure public fixture in `TestReviewDefaultBudgetOriginGraph` is 67,043
bytes with 4,008 source-origin rows. Before the correction, snapshot admission
requests 27,141,969 additional bytes while 41,103,183 are already live, exceeding
the 67,108,864-byte default. The corrected path retains every row and admits the
same fixture with default budgets.

## Default-buffer capacity is fixture-specific

The source-origin sizing correction is bounded accounting, not a memory-layout
redesign. Every selected origin remains in the canonical snapshot report and in
the selected projection as independent deep copies. The copies overlap live
session storage at admission. This real retained storage still sharply limits
the DTD source graph compositions under the unchanged 64 MiB (67,108,864-byte)
logical live-buffer default; it is independent of the primary-input byte limit.

The following adjacent cases were reproduced on Go 1.27.2 linux/amd64 from
unpublished local `35b3865`, whose tree is identical to published
`2b2e3d196402ad5dff75f55ad154d679efdca2d0`. All budget fields are zero/default.
`reviewBudgetScore(n)` is the exact generator in `resource_budget_review_test.go`:
one part `P1` named `Part`, sequential decimal measure numbers 1 through n, and
one C4 quarter note with duration 1 per measure, with no inserted whitespace or
DOCTYPE. The entry point is `InspectXMLWithResourcesAndDictionaries`, selecting
`ProfileResourceGraphInternalDTDDictionaries`, with no dictionary overrides,
resource dependencies, resolver or identity override.

| Measures | Exact source bytes | Default-budget result |
| --- | --- | --- |
| 500 | 67,043 | Complete, satisfied; all 4,008 source-origin rows retained |
| 531 | 71,197 | Complete, satisfied |
| 532 | 71,331 | Buffered-byte refusal: used 44,066,863, requested 23,115,219; incomplete, unsatisfied |

`TestReviewOriginGraphDefaultBufferBoundary` pins the adjacent 531/532 cases on
amd64, alongside the unchanged public 500-measure regression. Concrete record
sizes are architecture-dependent; these exact capacity checks skip other
architectures. They characterize this generated family and revision, not all
MusicXML documents of the same byte size, number of measures or number of nodes.

The [reviewer's comparison](https://github.com/go-muse/go-musicxml/pull/41#discussion_r4293498146)
reported an earlier boundary of 494 measures (about 66 KB), the corrected
500-measure case at 62.9 MB of the 67.1 MB allowance, and comparison capacities of
about 114 KB for standalone `ProfileSourceInternalDTDDictionaries` and 168 KB
for its "legacy graph" comparator. The review did not supply its probe source;
those comparison capacities are reviewer-reported, not independently established
boundaries for the generator above. They must not be substituted for a general
source-byte guarantee. MB/KB in those rounded figures are decimal units; the
actual default remains 64 MiB.

The exact `reviewBudgetFragmentScore()` generator is different: 2,000 empty
measures with ascending `number` and descending IDs `m2000` through `m1`, then
three `#m1` links in the final measure, with an XLink namespace on the root.
It contains 86,052 bytes. `reviewBudgetFragmentOptions(0)` selects the same DTD
graph profile, retrieval URI `https://budget.test/score.musicxml` and
`ResourceDocuments`. At all defaults, snapshot admission refuses with
57,607,574 live bytes and a 34,741,160-byte request against 67,108,864 bytes.
`TestReviewFragmentGraphMediumScoreDefaultBufferRefusal` preserves the typed
buffer refusal, zero adoptions, incomplete enumeration, unsatisfied bundle and
retained failed-child report. The companion
`TestReviewFragmentGraphMediumScoreDefaultRuleBudget` uses precisely the same
source and options except `MaxBufferedBytes: 512 << 20`; its pass isolates the
20,000,000-rule-step ceiling. That allowance is neither a measured minimum nor a
production recommendation. A work-budget pass with lifted buffer is not an
all-default admission pass. Refusal is incomplete processing, not XML invalidity.

For these source-origin graph profiles, choose explicit finite budgets based on
the actual input and selected capabilities. Source bytes, origin count, record
contents, selected projections, independently owned copies, overlapping storage
and other limits all affect admission. No default is raised, no origin row is
removed, and no sharing or performance redesign is claimed here.

## Official UTF-16BE sample and standalone composition

The original `testdata/official/MozaChloSample.musicxml` is 223,068 bytes, with
SHA-256 `4323af5d7daa23ee3ae4fc27ff51274e98055a775824c49d3f4312f7586bc23e`.
Its XML declaration and external DOCTYPE are UTF-16BE after the BOM. The
measurement control removes the encoded DOCTYPE byte interval `[114, 358)`;
it preserves all other bytes, the BOM and encoding declaration. That produces
222,824 bytes with SHA-256
`ac6257b749d2ab2e7dc36bc9e8ede76eb83f83d3b4453624b1edb30d05868cbf`.

The following observations use official Go 1.27.2 on linux/amd64. The DOCTYPE-free
rows were independently reproduced on unpublished local baseline
`9f49784b62334a70b9ed41f4aaa963ff9c559476`, whose tree is identical to published
first-review head `159dd6e400dd8ba39733288e4dc590713a8154bc`. The original-byte refusal and
96 MiB allowance were verified with the narrowed corrections, which do not
change standalone source assessment:

| Input and profile | Observed result |
| --- | --- |
| Original bytes, SourceSemantics, default buffer | Buffered-byte refusal: used 67,108,454, requested 448, limit 67,108,864 |
| Original bytes, SourceSemantics, 96 MiB buffer only | Input completes; assessment remains incomplete due to unread external DTD |
| DOCTYPE-free control, SourceInternalDTDDictionaries, defaults | Assessment complete; observed buffer peak 44,036,266 bytes |
| DOCTYPE-free control, SourceSemantics, defaults | Rule-step refusal: used 20,000,000, requested 76; observed buffer peak 67,011,966 bytes |

Observed peaks sample `Account.Usage(BudgetBufferedBytes)` at context checkpoints.
They measure logical live admission; they are not exact minimum successful
allowances or heap/RSS measurements. The semantic control's peak is measured
only until its independent rule-step refusal. The 96 MiB allowance applies only
to the stated original fixture/profile/options; it neither enlarges defaults nor
establishes external-DTD validity. Removing the DOCTYPE changes the input and is
not a supported workaround for assessing the original bytes.

The exact stripped-input buffer failure described in review item 4291055313 was
not reproduced on this baseline. The original and correctly stripped byte paths
have the distinct outcomes above. The existing context/timeline rule-step limit
is independent of the snapshot/index corrections.

`TestReviewDefaultBudgetOfficialSourceComposition` pins the UTF-16 transformation,
byte counts and distinct typed resource limits.
`TestReviewOfficialOriginalDOCTYPEBufferAndEvidence` checks the original-byte
finite allowance without turning unread-DTD evidence into a validation pass.

## Focused verification

The round-two capacity characterization changes only tests and documentation.
Reproduce its public-entry-point cases with:

```sh
GOMAXPROCS=1 GOGC=20 go test -count=1 -v \
  -run 'TestReview(DefaultBudgetOriginGraph|OriginGraphDefaultBufferBoundary|FragmentGraphMediumScoreDefault(BufferRefusal|RuleBudget))$' .
```

This command checks the original 500-measure pass, both adjacent 531/532
admissions, and both budget configurations of the 2,000-measure fragment source.
It neither searches a broad capacity curve nor runs a performance benchmark.

Native focused validation and public-entry-point regressions pass with official
Go 1.27.2. The closed-record and index layout assertions also compile for
`GOARCH=386`; the host cannot execute that architecture, so this is compile-only
evidence. No full-suite, race or timing/capacity-readiness claim is made here.
