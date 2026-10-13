# Round 2 graph boundary corrections

This record covers review comments
[4259638309](https://github.com/go-muse/go-musicxml/pull/37#discussion_r4259638309)
and [4259638320](https://github.com/go-muse/go-musicxml/pull/37#discussion_r4259638320).
The inspected baseline is `861aec472c61ae284c25908684ad680dd32d4060`, tree
`2faaa347078bc1be8ee27ec526e75d265bc2ed4a`, the same tree as published
`a9a53a135fd9214cc5a60818add27dc6108bf2e4`. This is local correction evidence,
not a new publication, CI result, or review-thread closure.

## Unreachable request cache

The removed key included source pin, profile, package, effective base, template
index, and href. A reusable resource has one canonical assessment per pin/profile;
`enqueue` admits one expansion per assessment/package/base; `walkResources`
visits each template index once in that expansion. Partial, non-reusable resources
are not enqueued. Consequently a production `walkLink` call cannot revisit an
earlier stored request key. A repeated href at another template is a different
key, and an alias adoption does not walk the owning expansion again.

A logging-only overlay on the unchanged baseline, run with
`go test -count=1 -run 'Graph|Link|Depth|Package|MXL|Resource|Review|Recovery' -v .`,
observed 211 edge keys with no duplicate keys, 162 cache insertions, and 13 boundary
lookups. The only successful lookup was the manually populated
`TestReviewGraphKnownRequestCacheRequiresExactContext`. The selected tests passed.
That artificial test and the cache were removed together.

`TestReviewGraphRepeatedEdgesRetainOnlyLiveIndexes` uses three real resolver
requests with the same pin and retrieval context. Before the implementation
changed, it failed with 1,566 retained bytes per subsequent edge instead of 512:
the unnecessary charge was `1024 + 2*len("target.musicxml") = 1054` bytes.
After correction, only the live 512-byte linked-target index remains, excluding
the independently accounted file-metadata index. All three bodies are still
read, charged and closed once. Two canonical resources produce four adoptions,
three linked-document charges, and three distinct wrong-kind edge findings.
Existing same-entity and exact archive-context reuse tests remain in place;
external URI text still cannot establish identity before a resolver callback.

## Inventory evidence at the depth boundary

The initial test-first public regression exercised both `InspectMXL` and
`AssessMXLPackage`: `main.opus -> one.opus -> two.opus -> missing.opus`, with
`MaxDepth=3`. Both baseline runs failed with depth exhaustion and lost the
authoritative-absence finding. Both corrected runs return nil operation error,
complete enumeration, and one `MX40-PACKAGE-link-target` failure with reason
`linked-resource-authoritatively-absent`.

The no-I/O inventory lookup is shared by ordinary member loading and boundary
handling. It uses exact package and path identity, checks cancellation and an
existing terminal cause first, and charges bounded lookup work. Only a complete
inventory proves absence. Missing package context remains Unknown; incomplete
inventory retains its Unknown reason; Invalid inventory preserves its exact
state, reason and causal reference. Core's existing rule that Unknown evidence
cannot carry an invalid cause is unchanged. Primary selection and archive-primary
indirection behavior are unchanged.

The final 31 new cases cover:

- Raw/current public absence, existing unadmitted member refusal, and an admitted
  same-context cycle at the identical depth boundary.
- Raw/current zero resolver callbacks, exact aggregate selected-byte counts,
  unattempted raw member reads, uncopied current members, exact/one-under
  linked-document limits, and zero live buffered/depth gauges after cleanup.
- Unknown and Invalid inventory evidence, including independent ownership of the
  causal reference after edge ingress.
- Missing package, other-package inventory, encoded member identity,
  known-incomplete inventory, cancellation, terminal session state, and exhausted
  rule-work allowance.
- Canonical alias reuse with independent per-edge diagnostic multiplicity and
  retained-byte accounting.

The expanded regression set was also run with an overlay restoring all four
changed production files to the baseline. It reproduced the public absence
failures, lost prerequisite evidence, terminal-precedence failures, and 1,566-byte
alias slope. This later overlay run supplements the initial executed red tests;
it is not represented as an earlier test-first execution.

## Local verification

Go 1.27.2 and real libxml2 2.9.14 `xmllint` were used, with
`MUSICXML_REQUIRE_XMLLINT=1`. On the correction worktree:

- All 31 new cases pass:
  `go test -count=1 -run '^TestReviewGraph(BoundaryArchiveInventory|RepeatedEdgesRetain|ArchiveInventoryPrerequisite)' -v .`
- The broader graph/link/depth/package/MXL/resource/review/recovery selection
  passes with `-count=1`.

Full integrated acceptance and exact-head CI remain separate requirements.
