# Bounded local resource context

The additive `ProfileSourceLinks` and `ProfileModelLinks` compose the existing
XSD/local predicates with nonfetching resource projections. Existing eight
profiles, catalog digests, options, defaults and legacy MXL APIs are unchanged.
This increment does not implement a package or resolver API. Its finalized
resource records are suitable inputs to a separately selected resource snapshot
and graph consumer.

## Typed evidence and ownership

`ContextResult.Resource` has exactly one named branch: link, image, schema hints,
group or instrument. Its owner, exact assembly-qualified subject declaration,
local score-part, source/model location and all causal references remain explicit.
Copies own slices, strings, positions and causal records. Immutable numeric
wrappers retain their normal ownership contract. The optional JSON member is
omitted for existing context records, preserving historical serialized reports.

Every `ResourceAttribute` separates physical presence, raw XML-normalized lexical
evidence and the shared XSD's effective scalar. Direct models do not fabricate
source spelling. Known absent XLink show/actuate use the actual attribute-group
replace/onRequest defaults; invalid or explicitly empty enumerations retain XSD
causes. URI text comes from the exact already-assessed anyURI slot. This local
projection does not change the general Atomic bridge or URI datatype semantics.
A literal `#` split preserves fragment absence versus a present empty fragment;
it does not interpret escapes, XML IDs, XPointer or a remote resource.

The exact link families distinguish score link, part-link, score-work opus,
Opus score, Opus opus-link and bookmark. No family is inferred from file suffixes.
The accepted local Opus version and pagination provider is reused without
reimplementing its defaults. No timeline or broad contextual provider is selected.

## Attachment subset

Without `element`, link/bookmark attachment is the exact following sibling,
including another marker. `position` is irrelevant to this attachment branch;
its independent XSD domain still applies. With `element`, the anchor is the next
sibling that is neither an exact link nor bookmark. Missing targets are context
absence and never an additional mandatory failure.

A present selector supports unprefixed NCName spellings in the no-namespace
MusicXML descendant domain only. This does not reinterpret arbitrary NMTOKENs as
QNames. There is no absent-position default. Global descendant ordinal and XPath
per-parent ordinal readings must identify the same unique node, or both have no
target. Other cases retain the interpretation gap. Enumeration and order gaps
retain their own evidence/cause; current-model order is not original source order.

The implementation builds document-order indexes by name and by
(name, per-parent ordinal), once. Each selection uses bounded binary searches,
not a new sibling/descendant scan. The exact integer is magnitude-checked before
host-width conversion. Huge positions do not allocate sparse arrays.

## Other local branches

Image/credit-image source and MIME type requirements reuse XSD. Source suffixes
never override MIME evidence. One supplied dimension gives proportional scaling
advice; two give independent X/Y scaling advice. No intrinsic geometry or rendering
requirement is invented. Group/group-link preserve exact strings, including
whitespace; they are separate from part-group number matching. Instrument-link
uses the already selected typed, same-score-part reference result, including its
exact invalid cause. It proves no cross-file correspondence.

`NewSchemaHintContextPlan` supports each exact MusicXML, Opus, container or sounds
assembly independently. It tokenizes only XML whitespace into discovery pairs,
retains an unmatched final token, and never fetches or selects schemas. Existing
fixed-schema zero/odd/even list(anyURI) acceptance remains unchanged. A pairing
gap is optional intrinsic context: successful extraction does not assert usable
schema discovery. A separately selected discovery consumer must assess that gap.
Model xsi history is unavailable, not known absent.

## Evidence and regression provenance

`link_context_test.go` adapts only the schema-hint/composition ideas from the
genuine unpublished batch-2 `qa_composed_contract_test.go` and
`qa_composition_test.go`. Their original files are preserved unchanged. The new
current-API tests separately prove old odd-list datatype acceptance, typed new
pairing incompleteness, and independently invalid hints alongside unavailable
ENTITY context. The obsolete APIs and unrelated export harness are not replayed.

Tests cover source/current-model, partwise/timewise, credit and nested Opus,
wrong-assembly/foreign homonyms, direct-model edits, immediate marker siblings,
ordinal agreement/disagreement, huge positions, missing/invalid/empty evidence,
owned copies/remapping, bounded work, cancellation and retained-byte cleanup.
Counts are predicate instances and projections, not conformance coverage ratios.

## Bounded XLink behavior intent

`ResourceLinkContext.PresentationIntent` preserves `new`, `replace` and `embed`
recommendations. `ActuationIntent` preserves `onRequest` and `onLoad` intentions.
For either axis, `other` and `none` leave application behavior Unknown; they do
not forbid or authorize retrieval. Invalid fields retain their prerequisite cause.
The independently retained Show/Actuate fields still distinguish encoded values
from exact schema omission defaults. This follows XLink 1.0 section5.6 and the
pinned MusicXML/Opus link-attributes defaults, without claiming full XLink
application conformance or performing any renderer action.
