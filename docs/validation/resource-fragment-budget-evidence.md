# Fragment lookup and source-origin transport budgets

This receipt covers PR 41 review items 4291055418, 4291055432 and 4291055313.
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
on baseline `9f49784` refuses rule steps with 19,999,999 used and 3 requested
against the 20,000,000 limit. Adding pitched notes to every measure is a larger intrinsic assessment: that separate variant
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

## Official UTF-16BE sample and standalone composition

The original `testdata/official/MozaChloSample.musicxml` is 223,068 bytes, with
SHA-256 `4323af5d7daa23ee3ae4fc27ff51274e98055a775824c49d3f4312f7586bc23e`.
Its XML declaration and external DOCTYPE are UTF-16BE after the BOM. The
measurement control removes the encoded DOCTYPE byte interval `[114, 358)`;
it preserves all other bytes, the BOM and encoding declaration. That produces
222,824 bytes with SHA-256
`ac6257b749d2ab2e7dc36bc9e8ede76eb83f83d3b4453624b1edb30d05868cbf`.

The following observations use official Go 1.27.2 on linux/amd64. The DOCTYPE-free
rows were independently reproduced on the exact first-review baseline
`9f49784b62334a70b9ed41f4aaa963ff9c559476`. The original-byte refusal and
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

Native focused validation and public-entry-point regressions pass with official
Go 1.27.2. The closed-record and index layout assertions also compile for
`GOARCH=386`; the host cannot execute that architecture, so this is compile-only
evidence. No full-suite, race or timing/capacity-readiness claim is made here.
