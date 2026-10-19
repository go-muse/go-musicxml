# Direct model and bounded local assessment scope

Status: **bounded implementation under round-1 correction verification; batch-5 reviewer acceptance pending**.
Base: `511f55b4fa01dee1dd7542414ccd1eb5ad7ec6bf`.

This document records the frozen scope and its bounded implementation. Its
obligation manifest is
[model-assessment-ownership.json](model-assessment-ownership.json): all 95
engineering requirements, 28 unchanged issue gates, 29 individual candidate
dispositions and 42 retained source contracts. Original planning statuses and
acceptance/five-axis contracts are preserved. New runtime bindings and inspected
executable-test references are recorded separately. Original planned test IDs,
generic/parameterized axes and research counts are not executed coverage. The
nine-batch program remains a packaging target, not permission to omit a requirement.

## Reviewed decisions

Independent scope, adapter, kernel and architectural critiques identified the
following design corrections, now reflected in the implementation:

- The proposed function `Assess` collides with the existing generated type
  `Assess`. The additive root API is `AssessModel(context.Context,
  Document, AssessmentOptions) (validation.Report, error)`.
- Source-only `Scalar.Lexeme` and `OrderSource` cannot stand in for current
  model evidence. An explicit current-value lane connects the authoritative
  `internal/xsd` kernel and target-aware session bridge.
- Current generated field order and valid `Content` order are sufficient for
  current-structure grammar. They never reconstruct historical XML order.
- A generated variant can represent repeated same-name schema positions.
  Generated Go representation metadata supplies fields; shared grammar witnesses
  bind exact declaration uses, rather than independently manufactured use IDs.
- Fixed root names come from concrete generated root types. `XMLName` is ignored
  transport metadata, preserving the established public Encode contract.
- Local representation problems use bounded causal evidence: the optional
  once-per-node `ModelRepresentation` event precedes closure, and a sealed
  required model plan consumes it. Missing evidence is not implicit success.
  Existing `NodeEnd` fields are unchanged.
- An invalid middle `Content` entry is a hole, not a truncated prefix. It cannot
  be compacted away and fed to prefix-witness recovery. Independent known axes
  continue; affected child order/binding remains explicitly unavailable.
- Typed references must consume the authoritative XSD actual-ID owner lookup.
  A separate score-instrument-only index would miss wrong-kind collisions.
  Do not gate every reference on one document-wide all-ID success verdict.

## Frozen target profiles and boundaries

The public explicit profiles are:

- `ProfileModelXSD` (`musicxml-4.0/model-xsd`): current generated score-partwise, score-timewise and
  opus structure/value XSD assessment, with required model representation checks.
  This is also the private semantic scope of the legacy XSD-only `Validate`
  facade after direct migration.
- `ProfileModelLocal` (`musicxml-4.0/model-local`): the same three generated roots, shared current-model
  XSD and representation checks, plus the frozen local musical clauses below.
  Score-only clauses are provably inapplicable to opus; linked opus/score content
  is outside this profile's claim.
- `ProfileSourceLocal` (`musicxml-4.0/source-local`): original single-pass source XSD on those three
  roots plus the same local clauses and semantic rule revisions. Source syntax
  remains separate from format conformance.

These are bounded target-specific claims, not unqualified MusicXML validity.
They are available through explicit selection, but final combined review and
acceptance evidence remain pending. The existing `musicxml-4.0/source-xsd-preview` retains its exact
scope, IDs and source behavior. These local/XSD profiles remain explicit
inspection/assessment choices. The later assessed-loading defaults are the
separate semantic profiles described below.

All source profiles remain unable to accept unread external-DOCTYPE subsets,
including the three committed official score examples. No DTD trust, stripping,
fetching, ignore-DTD policy or source-infoset assumption is introduced. The two
DOCTYPE-free opus examples remain a different source-input case. Schema hints
never select a new schema or perform I/O. Container, sounds and packages remain
outside this batch.

The selected schema is pinned MusicXML 4.0; `version` remains its actual XSD
value/default contract, not an invented enumeration containing only `4.0`.

### Catalog inventory and selection

The model-XSD profile requires 14 rule keys: six shared XSD operations for each
of the musicxml and opus assemblies, plus one required model-representation rule
per assembly. Model-local requires those 14 plus 22 local rule keys (36 total).
Source-local requires the 12 shared XSD keys plus those same 22 local keys
(34 total), without model-representation rules. The isolated developmental
source-XSD preview retains its original 12 `MX40-SOURCE-XSD-*` keys.
`TestBoundedCatalogExactProfilesAndSharedRules` checks those exact inventories,
revisions, support entries, roots and catalog separation.

The new XSD keys are `MX40-XSD-{musicxml,opus}-{binding,declaration,attributes,scalar,content,identity}`,
all at revision 1. Model representation uses
`MX40-MODEL-REPRESENTATION-{musicxml,opus}`, revision 1. These assembly-selected
composites are not one predicate for every research record. Runtime counts are
selected normative predicate instances; model blockers are a separate axis.
`Report.Profile` carries the target, registry commit, catalog digest, required
keys, support and normative closure.

Use `AssessModel` with an explicit model profile and `InspectXML` with an explicit
source profile. Unknown or wrong-target selection fails their preflight. An
empty profile, which failed here when this scope was recorded, now selects
`ProfileModelSemantics` or `ProfileSourceSemantics` (see
[Scenarios](../../API.md#scenarios)).
The [later assessed-loading migration](assessed-loading-pipeline.md) instead fixes
validation line 2, `ProfileSourceLine` (`ProfileMXLSourceLine` for MXL), then
line 3, direct `ProfileModelSemantics` (`ProfileMXLModelSemantics`), in the
assessed loaders. Nil/empty assessment options
select that pipeline; narrower source-local options are rejected. Line 3 is
this scope: the model checks of strict loading are exactly `AssessModel` with
`ProfileModelSemantics`. A present empty accordion registration is XSD-valid but
locally invalid: `Validate` and the preview retain the XSD result, while the
mandatory assessed loader withholds the model and model-local reports the added
violation. See `TestBoundedLocalStrictAccordionIntegration` and the runnable
[AssessModel example](../../model_assessment_example_test.go).

## Selected musical obligations

Nineteen whole candidate clauses and one local subclause are selected. These
research-record dispositions are not twenty independent new runtime predicates:

1. Accordion registration contains at least one selected register child.
2. Beam effective numbers (default 1 only after absence) are distinct per note.
3. Any second Bézier control requires slur with type continue; tied never qualifies.
4. A for-part element requires this document's defaults/concert-score marker.
5. Key-octave cancel=yes requires a cancel child in the same key.
6. Metronome relation is exactly empty or equals; no trimming or case folding.
7. Present sound dacapo is yes.
8. Present sound forward-repeat is yes.
9. Each sound pedal value is yes/no or the shared union's exact decimal in 0..100.
10. With concert-score, attributes/transpose has chromatic zero and absent/zero
    diatonic; octave-change/double remain allowed and for-part/part-transpose is excluded.
11. Date/no-timezone reuses the shared XSD scalar condition and adds provenance,
    not another failing predicate.
12. Part IDREF targets a unique score-part in this document's part-list.
13. Present midi-device IDREF targets a score-instrument in the matching score-part.
14. Midi-instrument IDREF has that same target kind and matching part scope.
15. Instrument-change IDREF has that target kind/scope.
16. Note/instrument IDREF has that target kind/scope; multiple instrument children are allowed.
17. Present play IDREF has that target kind/scope; omission keeps its broader scope.
18. Present assess/player targets a player in the matching score-part; omission
    does not make assess's required type optional.
19. MIDI channel/bank/program corresponding-instrument existence reuses the same
    typed assignment evidence and consequence, without another missing-ID error.
20. Instrument-link's **local** IDREF targets a score-instrument in its owning
    score-part. Linked-file existence/content/association is separately owned by
    batch 8 and is not claimed by this local-only clause.

Every relation uses exact assembly-qualified declaration/component evidence and
shared validated scalar/default results. Generic missing/ambiguous/invalid IDREF
blocks the typed consequence with its causal identity. Unique wrong-kind or
wrong-part targets fail the added clause. Part identity comes from the containing
part's successful typed reference, never its ordinal/name or guessed intent.
Repeated timewise part references remain legal.

Nine whole candidates are outside these bounded profiles: barline location,
beat-repeat content, bend release negativity, chord/base duration bound,
measure-repeat content, numeral effective key, pitch alteration intent,
time-only numerical ascending order, and written transposition. Their exact
owners, evidence and closure conditions remain in the manifest. Time-only's
numerical/equal-adjacent policy is local semantic work in batch 7; it is not
falsely described as requiring a repeat graph. Underlying XSD checks remain.
Exclusion defines this profile's claim; it is not a runtime pass or proof that
an omitted obligation is inapplicable.

### Executable local bindings

The following exact runtime IDs are revision 1 on both local profiles. Their
selectors come from authoritative assembly-qualified compiled declaration/use
or type descriptors. The table does not promote the original research status or
claim that every original five-axis recipe is independently instantiated.
Detailed test file/function/case scopes are in the ownership JSON.

| Selected clause | Runtime rule ID(s) |
| --- | --- |
| Accordion presence | `MX40-LOCAL-accordion-at-least-one` |
| Beam uniqueness | `MX40-LOCAL-beam-number-distinct` |
| Second Bézier control | `MX40-LOCAL-bezier-second-control/slur`, `MX40-LOCAL-bezier-second-control/tied` |
| for-part concert marker | `MX40-LOCAL-for-part-requires-concert` |
| Key-octave cancellation | `MX40-LOCAL-key-octave-cancel-exists` |
| Metronome relation | `MX40-LOCAL-metronome-relation-value` |
| Sound flags | `MX40-LOCAL-sound-dacapo-yes`, `MX40-LOCAL-sound-forward-repeat-yes` |
| Pedal range | `MX40-LOCAL-sound-pedal-range/damper-pedal`, `/soft-pedal`, `/sostenuto-pedal` under the same prefix |
| Concert-score transpose | `MX40-LOCAL-concert-score-transpose` |
| Part targets | `MX40-LOCAL-part-id-target/use-1`, `MX40-LOCAL-part-id-target/use-2` |
| Instrument/player targets | `MX40-LOCAL-midi-device-target`, `MX40-LOCAL-midi-instrument-target`, `MX40-LOCAL-instrument-change-target`, `MX40-LOCAL-instrument-note-target`, `MX40-LOCAL-play-instrument-target`, `MX40-LOCAL-assess-player-target` |
| Instrument-link local target | `MX40-LOCAL-instrument-link-target/local` |
| Date/no-timezone provenance | `MX40-XSD-musicxml-scalar`, no additional local predicate |
| MIDI corresponding-instrument existence | `MX40-LOCAL-midi-instrument-target`, shared evidence/consequence |

The two part use IDs distinguish the partwise/timewise declaration uses. Bézier
owners and the three pedal attributes have distinct bindings, explaining why
19 whole candidate records plus one local split produce 22 local rules rather
than twenty independent new predicates. Date/prose provenance is attached to the
shared XSD scalar operation. MIDI channel/bank/program existence is discharged by
the required typed assignment in these profiles; it does not create a second
missing-ID failure. The conditional-only shared constructor is separately tested.
None of 22, 36, 34, 29 or 2,560 is a complete-conformance coverage denominator.

## Current-model evidence contract

The adapter never calls Encode, marshal methods, an XML parser or a user
predicate callback. It uses a generated closed Go field/variant representation
map and an iterative bounded traversal. Shared pointers have independent
occurrences; only repeated pointers on the active path are cycles.

Required nonpointer fields are present current values, including zero/empty.
Optional nil pointers are absent; nonnil zero pointers are present. Valid
Content preserves current interleaving. Other fields preserve the generated
current field order. No assertion recovers discarded source attributes, missing
required source scalars, repeated singular fields, namespaces, xsi controls,
unknown elements or original spelling. Model locations are field/index paths
with occurrence IDs and no XML positions.

String-backed scalar fields supply their actual current string to the shared
schema domain machinery, keeping public Lexeme unavailable. Integer fields
supply exact signed/unsigned values. Finite float64 decimal fields supply the
shortest decimal that reads back to the same float64, the value a decoder read
from that text: 0.1 is decimal 1/10. Numeric bounds, enumeration, fixed values
and digit facets use that value. Strict loading rejects, in the source, any
value whose shortest float64 form differs (MX40-SOURCE-REPRESENTABLE-*). Numeric patterns cannot use
fabricated canonical source spellings and remain a reached capability limit.
NaN, infinity and invalid text are model representation/value problems.

Every model profile names this contract in its normative closure; the catalog
builder appends it to composed model profiles whose closure a composition
replaced. The contract therefore changed the catalog digest of every catalog
that holds a model profile, the source profiles sharing those catalogs, and
the four package catalogs whose intrinsic bindings name model profiles. The
tests migrate earlier captures through one documented step,
`migrateDecimalContractCatalog` and `migrateDecimalContractProfileHash`,
applied after the earlier migrations, which keep their own endpoints.

Known scalar absence on an element-only model is distinct from unknown scalar
content. Numeric zero must never trigger empty-element defaulting. General
empty-model element value-constraint interpretation not needed by the pinned
MusicXML/opus metadata remains explicitly unsupported rather than silently
choosing a new policy. Known absent XSD attribute defaults remain separate
effective facts and do not mutate the object. DTD attribute defaults differ:
strict loading supplies them in the effective source infoset, so the converted
object holds them like written values, and Encode writes them out.

The core records real model-representation decisions before publishing causal
invalid evidence. A bad attribute affects that slot, a bad scalar affects that
scalar, and an invalid Content wrapper affects child sequence evidence. It does
not turn unrelated known facts into missing data or fabricate grammar defects.

### Field and obligation capability matrix

`TestModelMetadataCoversEveryReachableField` checks every generated reachable
field and choice variant against the closed metadata map; generator tests check
that the map comes from the same render plans. Metadata describes Go
representation, not an independently guessed schema use ID. Shared grammar
witnesses bind exact declaration uses, including repeated/group/anonymous cases.

| Generated representation | Evidence available to selected obligations | Evidence not supplied |
| --- | --- | --- |
| Optional pointer | Current nil/present state; present zero/empty is preserved; absence may enable an XSD attribute default | Historical source presence or spelling; whether strict loading supplied the value from a DTD default |
| Required nonpointer scalar/struct | Current represented occurrence and value, including zero/empty | Whether the original source had a required child or repeated singleton |
| Repeated slice | Current count and per-index occurrences | Discarded unknown source occurrences |
| Valid ordered `Content` | Current selected branch and interleaving | Historical order after lossy decoding |
| Fixed generated fields | Current generated field order | Original cross-field XML order |
| Signed/unsigned numeric field | Exact current Go integer and XSD value-space checks | Source integers beyond the Go type; strict loading rejects them in the source |
| Finite `float64` decimal field | Shortest decimal reading of the current value for bounds, equality, fixed/enumeration and digits | Original lexical form; strict loading rejects source decimals the model cannot hold unchanged |
| String-derived field (`YYYYMMDD`, `YesNoNumber`, etc.) | Current string assessed through the resolved shared whitespace/domain/union plan | Prior validation or original XML lexeme |
| Root concrete type | Fixed root identity and assembly | Authority from `XMLName`; that field is ignored |
| Noncyclic shared pointer | Distinct occurrence each time, charged again | Pointer identity as document identity |
| Invalid choice/cycle/value | Scoped representation blocker and causal invalid evidence | Fabricated children, default values or XML syntax defects |
| Namespace/xsi/unknown source data absent from public fields | Only current representable static declaration evidence | Lost namespace declarations, xsi:type/nil/hints, unknown attributes, DTD infoset |

A missing applicable capability stays unknown/unsupported; exclusion from the
profile is a separately documented scope decision. It cannot be inferred from a
missing field. Numeric lexical patterns and general empty-model element value
constraints remain reached limitations. Existing wider primitive equality,
constructed identity, wildcard/general schema and DTD limitations remain with
the source/XSD ledgers and batch 9 unless explicitly covered here.

## Compatibility, resources and diagnostics

`AssessModel` returns ordinary normative/model failures in its report with nil
operation error; callers inspect Satisfied. Incomplete infrastructure returns
its real cause and best partial report. Model syntax is NotAssessed.
`Validate` stays XSD-only and uses the same direct adapter, preserving
ErrNilDocument, unsupported-document recognition, ValidationError and genuine
ErrInvalidDocument findings. Existing cycle/depth recognition is preserved only
in that legacy facade. Its private compatibility policy sets every quota to
`math.MaxInt64` except depth 4096, avoiding new default document, scalar, work and
diagnostic ceilings on `Validate`. Shared-account overflow checks and host
allocation guards remain active; this is not unlimited memory or a throughput
guarantee. Memory and work grow with the model. `AssessModel` and source assessed
APIs retain their finite defaults; use explicit assessed budgets for untrusted
input. Model/resource/unsupported results are not source malformed XML.

Legacy `Validate` can intentionally match both `ErrInvalidDocument` and
`ErrIncompleteAssessment` even when blocked XSD work depends on a known model
failure, such as a NaN width. Invalidity remains definitive: callers must not
interpret the incomplete sentinel alone as meaning no invalidity was proved.
A composite XSD result can also contain an independent unavailable capability
behind the same causal `Blocked` summary; counts and retained findings do not
prove that every underlying limitation is dependent. The facade therefore does
not suppress incompleteness from that summary. A complete ordinary XSD violation
still produces invalidity without incompleteness. `AssessModel` keeps its report
axes and nil operation error for these ordinary findings; diagnostic omission
never changes counts or completion.

One shared account owns the whole run. Session ingress charges actual nodes,
attributes and depth once. Walker work includes every visited field, choice and
slice entry, even when no node is emitted. Walker owns bounded frame, active
pointer, path, problem and exact-value temporary storage; session/kernel own
facts, compiled witnesses, scalar/index execution and report storage. All scans
and growth check cancellation and allowance before work/allocation. No cumulative
quota is refunded; every run is finished once and live gauges are released.
No artificial input-byte charge is invented for the caller's existing graph.
These accounting mechanisms also serve legacy `Validate`, but its private quota
policy above deliberately differs from the finite assessed defaults.

Model-representation preparation at `CloseNode` uses an immutable compiled ordered
subset, rather than reclassifying every selected rule for each node; full rule
assessment still visits the selected schedule. The distinct-child operator
selects and materializes each effective child value once per evaluation. Exact
equality comparisons remain pairwise and metered. Temporary scratch growth is
charged before allocation and released on success, limits and cancellation;
known duplicates and related-location encounter order retain their semantics.
The generic stress controls do not broaden MusicXML beam counts or claim linear
total work.

The shared kernel retains ordered per-binding attribute-result pointers and
shares only immutable unconstrained-absence facts in a run-local effective-schema
indexed cache. Exact declaration-owner occurrence identity is part of that
cache's applicability: another named alias owner falls back to independent
assessment. Present, defaulted and fixed-value records remain subject-specific.
The internal `Attributes` iterator yields value copies in input-then-schema order
without allocating a materialized snapshot; consumers meter each visit. Cache
fields, arrays, records and per-binding pointers are admitted before allocation.
Incomplete enumeration and interrupted prefixes never become known complete
absence, and iteration cannot expose facts after the owning run is closed.
These storage changes preserve the original source/current-model evidence
contract and do not close broader default/fixed identity obligations.

Diagnostics retain exact rule revision, target, model/source location, English
explanation, provenance, actual/expected and related owner evidence. Truncation
cannot erase normative counts, model blockers, causal state or legacy cycle/depth
recognition. The original research/status inventory remains an inspection artifact.

## Frozen logical implementation and acceptance tasks

Each production task uses authentic failing focused tests before its correction,
then focused regression checks. Commits remain logically separated:

- B5-01: this exact scope, candidate/remaining-owner manifest and design reconciliation
- B5-02: generated representation-map metadata and exhaustive generator coverage
- B5-03: bounded model-representation event, sealed plan, copy/lifecycle/causal contract
- B5-04: exact current scalar domain and value-facet entry paths in internal/xsd
- B5-05: current structure/attribute/default/identity integration and hole safety
- B5-06: immutable schema-use descriptors and authoritative bounded ID resolution
- B5-07: iterative direct walker, exact current values, shape/cycle/resource controls
- B5-08: sealed local relation plans with default/scalar prerequisites and five-axis tests
- B5-09: typed-reference/scope plans and per-reference causal/missing/ambiguity tests
- B5-10: frozen source/model catalogs, exact selectors and shared/deduplicated clauses
- B5-11: public AssessModel, explicit local source selection and strict accordion integration
- B5-12: direct XSD-only Validate migration and documented compatibility tests
- B5-13: target capability/ownership bindings, API/examples, source regression and fuzz guards
- B5-14: independent combined-diff QA, all local stages, actual 386, real xmllint,
  one draft PR, exact remote head/CI verification and the retained reviewer loop

### Execution evidence and pending acceptance

Focused adapter, current-value/kernel, representation, local relation,
typed-reference, catalog and public API suites have passed at their implementation
revisions. The evidence below identifies inspected committed tests, not planned
test IDs or an assertion that all original five-axis recipes are complete.
`TestModelAssessmentOwnershipPreservesProgram` verifies the unchanged original
95 requirements, 28 gates, all 29 candidate contracts and 42 source contracts;
`TestModelAssessmentOwnershipExecutableEvidence` resolves listed executable names
against Go source and runtime bindings against actual required profile keys.
These integrity tests do not prove that a named test ran or covers every clause.

| Evidence | Scope |
| --- | --- |
| `TestBoundedLocalProductionClausesBothTargets`, `TestBoundedTypedReferencesBothTargets` | Concrete production source/model positives and violations for selected local clauses and reference kinds/scopes |
| `TestLocalSchemaClauseMatrixBothTargets`, `TestLocalSchemaDefaultsAndInvalidSlotCausality` | Shared exact-declaration synthetic boundary/missing-fact/default/causal controls; not every production occurrence |
| `TestTypedReferenceAuthoritativeResolutionBothTargets`, `TestTypedReferenceIncompleteAndUnselectedOwner`, `TestTypedReferenceOptionalAndRepeatedScopes` | Authoritative owner lookup, forward/missing/invalid/ambiguous references, incomplete providers and repeated/optional scope controls |
| `TestBoundedDateReusesScalarConsequenceBothTargets`, `TestTypedReferenceConditionalUsesSameConsequence` | Date and MIDI shared consequences without duplicate predicates |
| `TestDirectModelScalarExactCurrentValues`, `TestCurrentNumericFacetsAndPattern`, `TestCurrentStringUnionAndFixed` | Exact current numeric/string value bridge and explicit lexical limitations |
| `TestModelRepresentationTruncationPreservesBlockersAndCauses`, `TestLocalFailureSurvivesDiagnosticTruncationAndReleasesAccount` | Required blockers/causes and normative counts survive storage truncation; live resources release |
| `TestAssessModelFiniteResourcesAndIndependentAxes`, `TestDirectModelWalkerBudgetsCancellationAndDepth` | Public resource categories, bounded traversal, cancellation, cycle/depth and partial reports |
| `TestLocalSchemaConjunctFailureSurvivesInvalidSibling`, `TestLocalMarkerValuesIndependentConjuncts` | Known local counterexample survives another unavailable/invalid conjunct |
| `TestLocalPublicSessionFailureSurvivesPostProofStorageStop`, `TestTypedReferenceNestedOwnerProofIsNotCountedForConsumer` | Optional diagnostic/cache storage interruption preserves established failures and exact provider attribution; resource axes stay incomplete |
| `TestBoundedProfilesConcurrentIndependentReports`, `ExampleAssessModel` | Independent unchanged-model reads and public external-package usage |
| `TestAssessModelOfficialCorpusAndSourceDTDRemainSeparate` | Five unchanged official fixtures satisfy both current-model profiles; three external-DTD score sources remain incomplete, two DOCTYPE-free opus sources satisfy source-local |
| `TestLegacyValidationBudgetPolicy`, `TestValidateLegacyLargeScore`, `TestValidateLegacyLargeCurrentText` | Private legacy quotas/overflow guards, a 3000-measure model and >1 MiB valid text; explicit/default assessed limits remain effective |
| `TestValidateDependentRepresentationIncompleteness`, `TestModelBlockedCompositeCanRetainIndependentLimitations` | Intentional joined legacy sentinels, truncation invariance and a synthetic composite whose causal summary also hides an independent lexical-pattern limitation |
| `TestCloseUsesCompiledRepresentationSchedule` | Synthetic source/model catalogs, copied configuration, stable causal order and preparation independent of 1/64/512 unrelated rules |
| `TestLocalDistinctMaterializesEachEffectiveValueOnce`, `TestLocalDistinctSelectionWorkScalesWithChildren` | Generic both-target effective-value materialization once per child; selection/lookup cost separated from still-pairwise exact equality comparisons |
| `TestLocalDistinctScratchExactLimitAndCleanup`, `TestLocalDistinctCancellationReleasesScratch`, `TestLocalDistinctMixedFamiliesAndEncounterOrder` | Exact scratch boundaries, owned-charge cleanup, cancellation and retained known-duplicate/related-location semantics in synthetic controls |
| `TestRunAbsentAttributesRetainCompactPerNodeFacts`, `TestRunAbsentAttributesKeepOrderedIndependentFacts` | Synthetic both-lane compact retention, ordered absence, independent present/default/fixed values, incomplete-enumeration and cleanup controls |
| `TestRunAbsentAttributesPreserveActualTypeUse`, `TestRunAbsentAttributeCacheKeepsAliasedNamedUses` | Source actual-type/inherited owners and source/model named-alias owner identity stay exact; no new model xsi capability |
| `TestRunAbsentAttributesStopAtProvedPrefix`, `TestRunAbsentAttributeCacheAdmission` | Cold/warm source interruption preserves only proved prefixes; cache/record/node-pointer admission failures retain no unadmitted facts |
| `TestRunAbsentAttributeIteratorLifecycle`, `TestRunAbsentAttributesConcurrentIndependentRuns` | Invalid/stale/closed iterator controls and independent source/model runs sharing immutable compiled schema with run-local caches |
| `FuzzAssessCurrentModel` | Direct caller-created values/shapes under finite budgets, determinism/non-mutation and model target axes; focused counted run passed 10,018 executions against a 10,000 minimum |

Independent core review at `681f1cb2e429566085202f75a796d3c9d53e69f7`
reproduced and verified fixes for unavailable-union fixed comparison, per-slot
attribute completeness and lost causal evidence. Its all-package tests with real
xmllint, focused core race and vet passed. That core-only review does not accept
the later adapter/local/catalog/API integration. Subsequent independent combined review verified the allocation, local-conjunct
and post-proof storage-interruption corrections. All-package tests with required
xmllint passed at local snapshot `ae2babc57967a029689c35d191febc3a64b425c9`;
focused affected-package race and full vet passed at `8adb1fe`. The final
documentation/ownership scan found only the corrected fuzz-target total.

An earlier integrated revision, `1a6af41e93fa7d17507729f3a142e3bd84f757aa`,
passed all seven local `check-all` stages. Its separately required real-xmllint
selection executed 27 oracle roots and 8,548 subtests with no skips or failures.
Those results predate later corrections and this documentation reconciliation;
they are preliminary evidence, not final-head acceptance or conformance counts.

The integrated local snapshot `86d7578f835840399a016bbcfacbfd3ba9523ea2`
subsequently passed all seven stages and 150,028 counted fuzz executions, including
the direct-model target. These revision IDs identify local pre-publication test
snapshots; the PR acceptance record maps their identical trees to published
commits when connector publication recreates commit IDs.

The final initial-publication local snapshot
`8b9b0438b4b42b4eb6cb31f6bf8b0f9931cb53d6`, tree
`2f32955c56c011d45ab6a3042cb81863d61ff476`, passed all seven stages,
150,000 counted fuzz executions and the required 27-root/8,548-subtest xmllint
selection with zero skips/failures. Its equivalent published head
`60c7a7702016630d68263f827716c77b5380d50f` subsequently passed all nine jobs in
[CI run 38170367439](https://github.com/go-muse/go-musicxml/actions/runs/38170367439),
including **actual all-package 386 execution** in the Ubuntu test job and the
required real-xmllint job. Local 386 compilation alone was not the runtime proof.

Round-1 review of that green head still identified legacy quota and performance
regressions. The private legacy quota correction, intentional dependent
invalid/incomplete contract, compiled representation preparation and distinct-value
materialization controls above address specific findings. The compact
absent-attribute correction and its eight focused kernel tests are also
integrated. Final matched measurements exist in [API.md](../../API.md) and
[CHANGELOG.md](../../CHANGELOG.md); no universal per-node cost is claimed.
The pre-publication gates below are a historical snapshot. The corrected head
`4c739e275bc68566ad7d20a4c8e2171ea11e6ad7` passed all nine jobs in
[CI run 38174422528](https://github.com/go-muse/go-musicxml/actions/runs/38174422528),
including actual 386 execution and required real xmllint. The reviewer accepted
the residual performance tradeoff for this batch subject to retaining the owned
follow-up below. Initial-head CI does not accept later corrections or close the
reviewer gate.

Historical pre-publication pending gates (superseded by the exact-head record
above where stated; future corrections require their own checks):
- All seven `check-all` stages (`mod`, `generate`, `format`, `test`, `vet`, `race`,
  `fuzz`) on the final round-1 corrected revision, with reproducible generation/
  planning and required original-byte xmllint roots actually executed
- Actual `GOARCH=386 go test -count=1 ./...` on the corrected published head.
  This local host still rejects execution with `exec format error`; initial-head
  CI success is evidence only for its verified head/tree
- Corrected published-head/tree verification, all nine required CI jobs,
  measured performance reconciliation and the retained reviewer loop. Corrected-
  head run/artifact links and reviewer acceptance remain pending

The public ownership manifest carries these pending evidence fields explicitly.
Official corpus and fuzz results supplement clause-specific tests; neither
supplies a conformance denominator or replaces final acceptance.

## Remaining program

Batch 6 exact timeline/effective state; batch 7 broader
contexts/repeats/spans/intent and gated local ordering; batch 8 MXL, links,
container/sounds, bounded resolution and schema discovery; batch 9 external
normative and DTD/XSD/value/identity closure, remaining faithful translations and
final reconciliation. None is silently discharged by this bounded profile.

### B5-PERF-01: residual direct-path validation cost

Status: **outstanding engineering obligation**, separate from normative coverage
and acceptance of this batch's measured tradeoff. Owner: **shared validation
kernel/session workstream**, with **batch 9 final reconciliation** responsible
for an explicit disposition; earlier dependency-appropriate work may address it.
The retained cost areas are `Run.Bind` per-node state, child witnesses,
`recordXSDResult`, `OpenNode` header copies and `cacheDecision`.

The matched warmed `BenchmarkValidate` evidence on `MozaChloSample.musicxml`
(Go 1.27.2, linux/amd64, AMD EPYC 9V74, GOMAXPROCS=4, five alternating
750 ms samples) compares base `511f55b` with corrected production `4c96f9b`
(published equivalent `6c453ce`; later round-1 changes are documentation only):
median **27.02 ms / 11,705,376 allocated bytes/op / 123,632 allocations/op**
versus **100.73 ms / 33,374,378 allocated bytes/op / 181,857 allocations/op**,
or **3.73× / 2.85× / 1.47×**. These are fixture-specific warmed costs, not
retained-budget or peak-memory figures. [API.md](../../API.md) retains the ranges
and methodological caveats; the [review discussion](https://github.com/go-muse/go-musicxml/pull/34#discussion_r4243223274)
records the conditional batch-5 acceptance and follow-up request.

Closure requires matched remeasurement and a reviewed improvement or explicit
reviewed residual-cost disposition, preserving exact current-value, source/model,
causal and resource-accounting contracts. Preserve this measurement history when
recording that disposition. A streaming/fact-lifetime redesign is a possible
investigation, not a prescribed architecture or a silent policy change. Neither
this engineering item nor its documentation-integrity test adds normative
runtime coverage or discharges any batch 6–9 obligation.
