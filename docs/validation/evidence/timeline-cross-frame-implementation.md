# Cross-frame continuing-state implementation checkpoint

Baseline: `c168fb1`; independent REDs `310dbae`, `1d8a2ce`, `8c7cceb`,
`c4a852c`. Worktree is preserved independently of the batch-6 lead checkout.

The baseline negative later-frame sound and positive earlier-frame overflow
regressions were reproduced with `go test ./validation -run
'TestTimelineFinalReview(CrossMeasure|PositiveOverflow)' -count=1` (both failed).
The independent initial-device RED was retained as `b8357a7`; the default/cause
RED as `e19fa24`. The full validation suite initially exposed the independently
specified floating-dynamics/default consensus failure before the default loader
was integrated. Those focused tests are now GREEN.

## Proof and query boundaries

- `musicalPosition.UnprovedFrame` separates exact local coordinates from
  canonical measure membership. Neither signed displacement nor a cursor
  excursion self-certifies containment.
- `timelineState.frameBounds` stores a maximum independently observed current
  location, never a measure duration. A complete ordinary exact-note prefix may
  witness its next observed unshifted origin. A terminal End, chord's reused
  principal onset, attack/release or other derived endpoint is not a new bound.
- Backup/forward departures are observed before movement; destinations preserve
  prefix membership only inside the already proved interval. Excursions suspend
  witness extension; exact return to structural zero recovers it. The monotone
  no-rewind prefix has an explicit comparison-elision proof; rewound maxima use
  metered exact comparisons. Chord origins cannot decrease an existing maximum.
- Two established member positions retain canonical score-order precedence.
  A floating cross-frame position instead needs a joint rational inequality.
  An origin-distance lower bound never becomes an actual elapsed span, and
  equality of that lower bound never proves strict dominance.
- Ordinary contained histories retain per-scope indexed lookup. Floating
  candidates invoke a bounded exceptional partial-order scan. A possible
  publication is excluded only if proved after the query, or dominated by a
  definitely applicable unconditional statement proved strictly later.
- Known value requires agreement among every nondominated projected possibility
  and a definite baseline. Initial instrument/MIDI definitions and documented
  dynamics/count defaults are available lazily as actual baselines. Omitted
  properties are excluded by the existing sparse statement views.
- Point gaps follow the same partial applicability/repair relation. Conditional
  publishers cannot repair a gap or revive a gap already repaired. Strongest
  actual causes survive synthetic order uncertainty, and result causes are
  independently owned. Provenance includes all surviving actual sources;
  `InheritanceUnproved` discloses ambiguous latest-source/query relation.
- Scratch candidate capacity is checked and admitted before allocation, released
  on every exit. Every exceptional pair and rational operation shares existing
  work/scalar/buffer budgets. No limit is increased. Exceptional worst-case work
  is quadratic; exhaustion remains incomplete assessment.

## Verification at this checkpoint

- `go test ./validation -count=1`: GREEN after baseline loader integration.
- Focused independent cross-frame/default/causal tests: GREEN.
- `TestTimelineCrossFrameJointFloatingOrder`, point-gap conditional repair, and
  initial-instrument consensus controls: GREEN.
- All 16 source/model 1,000-direction/figure ordinary and diagnostic-buffer
  variants: GREEN, without raised production limits.
- `TestIndependentTimelinePracticalDefaults`, including exact model float64
  0.1 durations/divisions 3 and 1,000 notes: GREEN.

## Explicit remaining work

This checkpoint does not close downstream direction/figure/grace cross-frame
associations for newly marked cursor excursions, or causal unplaced signed-offset
publisher gaps whose destination has no established confinement. Independent
resource/cancellation tests and final integration checks remain pending. The
existing B5-PERF-01 kernel/session obligation remains open.

## Well-founded cursor-unit checkpoint

Independent REDs `ba52bd6`, `b614c1a`, `5aced4b`, and `8c6d53e` require
cross-frame unit evidence, directional unavailable-position bounds, rejection of
self-supported recovered-prefix bounds, and preserved opposite-side controls.
Their focused groups are GREEN with the least-proof reconstruction below.

- Record a potential excursion at its observed departure, before descendants
  can extend the frame bound. Complete assessed positive-unit consensus permits
  the ordinary exact result without a rebuild. No-excursion histories do not
  allocate a unit inventory or repeat duration arithmetic.
- For mixed/unknown units, stop every unresolved excursion in a fresh service.
  Inventory raw divisions publishers independently of staged coordinates; lost
  positions never erase publishers. Only the safe service's positions and
  bounds can exclude a potentially crossed region. A movement also requires a
  known safe departure and matching effective departure units.
- Forward travel excludes independently proved earlier or unreachable later
  changes. Backup travel excludes a differing earlier region only when an
  independently placed same-unit repair is strictly after that region and no
  later than the tentative destination. Every other potentially intervening
  publisher remains relevant. Opposite-direction contained changes are local.
- Remove a gate only after this independent certificate; rebuild and repeat.
  Each progressing pass removes at least one member of the finite admitted gate
  map. A dependent return and bound cannot certify their ancestor excursion.
  Existing local causal gates remain unavailable. Actual relevant invalid or
  unavailable unit causes are carried into durations and descendant positions.
- Displacement scope survives loss of exact coordinates in event/note/playback
  facts and state-family gaps. An assessed nonnegative movement and a complete
  positive-unit-domain proof retain its independently known departure as an
  upper (backup) or lower (forward) bound. Subsequent movement weakens endpoints
  as necessary. A contained return can restore ordinary prefix membership.
- Raw publisher scratch is pre-counted, host-checked and admitted before
  allocation, with an idempotent release. Named fixed allowances cover excursion
  append peaks, maps, and staged service objects. All scans, candidate pairs,
  arithmetic and rebuilds share the existing limits and cancellation polling.
  The original service is replaced only after successful completion. Old and
  staged retained allocations remain charged until session cleanup, a deliberate
  conservative exceptional-case cost; no production budget is raised.

At this checkpoint, `go test ./validation -count=1 -skip
'TestTimelineFinalReviewUnprovedCursorCannotCertifyCrossFrameFollowingNote'`
is GREEN. The one skipped association group belongs to the parallel association
implementation and is not claimed fixed by this state/runtime checkpoint.

The follow-up causal controls `6c6bac5`, `8f41082`, and
`TestTimelineExcursionGatePreservesDescendantUnitUnavailability` exposed three
additional REDs. Locked local gates now collect stronger reachable causes while
remaining locked; global gates also stop descendant lexical normalization; and a
new positioned divisions statement must actually repair the effective history
before it restores duration units. These groups and the complete validation
suite (apart from the separately owned association group above) are GREEN.

## Complete-history normalization closure

Independent RED `ae1c680` showed that a sequential stopped stage still needed
closure: a later negative excursion's displaced divisions publisher can affect
an earlier normalized note. Before any stage supplies certificate premises,
retained known normalizations are checked against its complete effective history.
An unsupported normalization is stopped and reconstructed; descendants retain
displaced placement metadata, including already-unavailable new-frame units.
Scope-filtered independently repaired gaps do not widen an ordinary cursor.

Each event can acquire one normalization gate, undergo at most three strict
availability-rank upgrades, and restore once from an independently retained
origin and known units in the preceding closed stage. Each excursion can acquire
one direction qualifier and be removed once. Every progressing pass consumes
one of these finite transitions. No stopped descendant bound certifies its own
restoration. A defensive contract check rejects any purported independent proof
that weakens under refinement. Note-only/no-excursion histories avoid this path.

`TestTimelineFinalReviewLaterDisplacedDivisionsCannotLeaveEarlierNormalizedNotes`
and the existing causal/directional/circularity controls are GREEN. At this
checkpoint `5023dbe` remains explicitly pending the lead's disjoint raw same-value
divisions certificate, needed because generic unavailable-position gaps discard
an otherwise known equal scalar. The new closure does not claim that positive
restoration control fixed before the certificate is integrated.
