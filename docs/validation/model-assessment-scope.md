# Direct model and bounded local assessment scope

Status: **reviewed implementation plan; batch-5 execution evidence pending**.
Base: `511f55b4fa01dee1dd7542414ccd1eb5ad7ec6bf`.

This document freezes the batch before implementation. Its obligation manifest is
[model-assessment-ownership.json](model-assessment-ownership.json): all 95
engineering requirements, 28 unchanged issue gates, 29 individual candidate
dispositions and 42 retained source contracts. Original planning statuses and
acceptance/five-axis contracts are preserved. Planned bindings and test lists
are not executed coverage. The nine-batch program remains a packaging target,
not permission to omit a requirement.

## Reviewed decisions

Independent scope, adapter, kernel and architectural critiques identified the
following required design corrections before product coding:

- The proposed function `Assess` collides with the existing generated type
  `Assess`. The additive root API will be `AssessModel(context.Context,
  Document, AssessmentOptions) (validation.Report, error)`.
- Source-only `Scalar.Lexeme` and `OrderSource` cannot stand in for current
  model evidence. Add an explicit current-value lane to the authoritative
  `internal/xsd` kernel and target-aware session bridge.
- Current generated field order and valid `Content` order are sufficient for
  current-structure grammar. They never reconstruct historical XML order.
- A generated variant can represent repeated same-name schema positions.
  Generate Go representation metadata; let the shared grammar witnesses bind
  exact declaration uses. Never independently manufacture compiled use IDs.
- Fixed root names come from concrete generated root types. `XMLName` is ignored
  transport metadata, preserving the established public Encode contract.
- Local representation problems need bounded, causal model evidence. Add an
  optional once-per-node `ModelRepresentation` event before closure and a sealed
  required model plan. Missing evidence is not implicit success. Avoid changing
  existing `NodeEnd` fields merely to carry this new event.
- An invalid middle `Content` entry is a hole, not a truncated prefix. It cannot
  be compacted away and fed to prefix-witness recovery. Independent known axes
  continue; affected child order/binding remains explicitly unavailable.
- Typed references must consume the authoritative XSD actual-ID owner lookup.
  A separate score-instrument-only index would miss wrong-kind collisions.
  Do not gate every reference on one document-wide all-ID success verdict.

## Frozen target profiles and boundaries

The intended explicit profiles are:

- `musicxml-4.0/model-xsd`: current generated score-partwise, score-timewise and
  opus structure/value XSD assessment, with required model representation checks.
  This is also the private semantic scope of the legacy XSD-only `Validate`
  facade after direct migration.
- `musicxml-4.0/model-local`: the same three generated roots, shared current-model
  XSD and representation checks, plus the frozen local musical clauses below.
  Score-only clauses are provably inapplicable to opus; linked opus/score content
  is outside this profile's claim.
- `musicxml-4.0/source-local`: original single-pass source XSD on those three
  roots plus the same local clauses and semantic rule revisions. Source syntax
  remains separate from format conformance.

These are bounded target-specific claims, not unqualified MusicXML validity.
They are activated only after the frozen obligations and their actual evidence
pass review. The existing `musicxml-4.0/source-xsd-preview` retains its exact
scope, IDs and source behavior. No profile becomes an implicit default.

All source profiles remain unable to accept unread external-DOCTYPE subsets,
including the three committed official score examples. No DTD trust, stripping,
fetching, ignore-DTD policy or source-infoset assumption is introduced. The two
DOCTYPE-free opus examples remain a different source-input case. Schema hints
never select a new schema or perform I/O. Container, sounds and packages remain
outside this batch.

The selected schema is pinned MusicXML 4.0; `version` remains its actual XSD
value/default contract, not an invented enumeration containing only `4.0`.

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
exact binary rational via big.Rat.SetFloat64, as the agreed contract specifies;
0.1 is not silently replaced by decimal 1/10. Numeric bounds, enumeration,
fixed values and digit facets use that exact value. Numeric patterns cannot use
fabricated canonical source spellings and remain a reached capability limit.
NaN, infinity and invalid text are model representation/value problems.

Known scalar absence on an element-only model is distinct from unknown scalar
content. Numeric zero must never trigger empty-element defaulting. General
empty-model element value-constraint interpretation not needed by the pinned
MusicXML/opus metadata remains explicitly unsupported rather than silently
choosing a new policy. Known absent attribute defaults remain separate effective
facts and do not mutate the object.

The core records real model-representation decisions before publishing causal
invalid evidence. A bad attribute affects that slot, a bad scalar affects that
scalar, and an invalid Content wrapper affects child sequence evidence. It does
not turn unrelated known facts into missing data or fabricate grammar defects.

## Compatibility, resources and diagnostics

`AssessModel` returns ordinary normative/model failures in its report with nil
operation error; callers inspect Satisfied. Incomplete infrastructure returns
its real cause and best partial report. Model syntax is NotAssessed.
`Validate` stays XSD-only and uses the same direct adapter, preserving
ErrNilDocument, unsupported-document recognition, ValidationError and genuine
ErrInvalidDocument findings. Existing cycle/depth recognition is preserved only
in that legacy facade. Legacy depth 4096 remains distinct from default assessed
limits. Model/resource/unsupported results are not source malformed XML.

One shared account owns the whole run. Session ingress charges actual nodes,
attributes and depth once. Walker work includes every visited field, choice and
slice entry, even when no node is emitted. Walker owns bounded frame, active
pointer, path, problem and exact-value temporary storage; session/kernel own
facts, compiled witnesses, scalar/index execution and report storage. All scans
and growth check cancellation and allowance before work/allocation. No cumulative
quota is refunded; every run is finished once and live gauges are released.
No artificial input-byte charge is invented for the caller's existing graph.

Diagnostics retain exact rule revision, target, model/source location, English
explanation, provenance, actual/expected and related owner evidence. Truncation
cannot erase normative counts, model blockers, causal state or legacy cycle/depth
recognition. The original research/status inventory remains an inspection artifact.

## Ordered logical implementation and acceptance tasks

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

Baseline all-package tests passed on Go 1.27.2 before implementation. No future
stage is claimed passed here. Final acceptance requires all seven check-all
stages, reproducible generation/planning, race and counted fuzzing, required
same-byte external oracle roots, actual all-package 386 execution, and all nine
exact-head CI jobs. Official corpus results supplement clause-specific tests;
they are not a denominator for conformance coverage.

Remaining program: batch 6 exact timeline/effective state; batch 7 broader
contexts/repeats/spans/intent and gated local ordering; batch 8 MXL, links,
container/sounds, bounded resolution and schema discovery; batch 9 external
normative and DTD/XSD/value/identity closure, remaining faithful translations and
final reconciliation. None is silently discharged by this bounded profile.
