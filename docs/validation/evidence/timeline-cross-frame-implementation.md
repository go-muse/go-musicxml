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
