# Independent review: signed offsets and cross-measure order

Date: 2026-10-12. Inspected production baseline: `cff2984`, rechecked against
`8e63f61`. This is a source-evidence/design review, not implementation closure.

## Confirmed counterexample

`TestTimelineFinalReviewCrossMeasureOffsetCannotClaimKnownChronology` preserves
an exact local sound coordinate `(measure ordinal 1, -1 quarter)` but obtains a
known tempo of 60 at the preceding measure's established note end `q=1`.
The prior measure has divisions 1, tempo 60, and one duration-1 note; the next
measure starts with tempo 120 and local sound offset -1. The provider has not
established the distance between those measure origins. Its ordinal-first
comparison nevertheless excludes the later-serialized negative-position change.

This test does not assert that the preceding measure's duration equals its
maximum note endpoint, nominal meter, or final cursor. It asserts that an exact
signed coordinate in one frame is insufficient to establish its chronological
relation to a coordinate in another frame. Different unconstrained origin
distances yield different applicability answers. Returning unavailable in the
absence of the needed relation is different from rejecting the document.

Affected paths: `readSoundAt` publishes the shifted position through
`readInstrumentChanges` and `Playback.readSound`; `stateComparePosition` and the
ordinary state indexes order different measure ordinals without a displacement
qualification. Tempo, dynamics, pizzicato, instrument, MIDI and device effective
queries can therefore acquire unsupported chronological certainty. Raw sound
and listening positions can remain exact local facts. Numeral key lookup uses
its independent unshifted origin and is not changed by this finding.

## Primary-source findings

The checked sources are the pinned MusicXML 4.0 schema annotations and official
4.0 references, not an importer convention or a later draft.

- [Offset](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/offset/),
  pinned schema lines 3937–3946, locates the **current musical location** within
  the current measure, including its end. It defines a signed displacement from
  that origin and separates visual from sound/listening application. It does
  not say the displaced destination is confined to that measure.
- [Sound](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/sound/),
  pinned schema lines 4123–4168, makes the offset the intended musical position
  of the sound change. Its local offset replaces a containing direction offset.
  Clamping that destination to its source measure or silently giving measure
  ordinal priority would not implement the stated displacement.
- [Listening](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/listening/)
  describes the corresponding continuing-state change from its specified
  performance point. Its origin/destination distinction is the same.
- [Duration](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/duration/)
  defines positive divisions and cursor movement for ordinary notes, backup and
  forward. It supplies no universal measure-length formula.
- [Measure](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/measure-partwise/)
  has no numeric actual-duration field. Its labels and graphical width do not
  supply an elapsed-time distance between measure origins.
- [Backup](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/backup/)
  and [forward](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/forward/)
  recommend avoiding boundary crossing. They do not impose a mandatory
  no-crossing constraint on sound offsets or prove every cursor path is inside
  its measure.
- The [MIDI-compatible tutorial](https://www.w3.org/2021/06/musicxml40/tutorial/midi-compatible-part/)
  gives fully worked counter/voice examples and describes a forward movement to
  a known example's measure end. These examples corroborate the cursor model;
  they do not state a universal final-cursor/max-endpoint/meter-fill theorem.
- [Rest](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/rest/)
  marks an explicitly complete-measure rest. This is a possible future source
  of exact boundary evidence after its ordinary onset, duration, scope and
  completeness obligations are established. It is not proof that every measure
  has such a rest or that all note endpoints define its boundary.
- [Barline](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/barline/)
  requires its location to match its placement. The current provider preserves
  raw location and cursor facts without closing that joint boundary consumer.
  An unassessed `location=right` must not be silently promoted to a verified
  elapsed-time boundary merely to repair this offset calculation.

## Minimal rigorous provider alternatives

These are derived sufficient proofs, not additional format validity rules.

### A. Same-frame exact arithmetic plus interval membership

Keep each event's unshifted origin and shifted local coordinate independently.
Same-frame rational ordering remains exact even when the shifted value is
negative or extends beyond the observed music.

Where an unshifted origin `a` is independently proved to belong to a measure,
any shifted coordinate `s` with `0 <= s <= a` lies between the structural start
and that anchor. A strict `s < a` also avoids assuming the anchor itself is
strictly before the right boundary. This uses an observed anchor as a bound;
it does not equate a maximum observed anchor to measure duration.

An absent/zero offset stays at its proved unshifted origin. A negative offset
from `a=2` to `s=1` is contained by that origin. A positive shift from 1 to 2
may be bounded by another independently established in-measure origin at 3,
even if it was encountered earlier or later in XML. A positive coordinate
beyond every proved anchor lacks a proven upper containment bound. A negative
coordinate is before the source measure's start. Neither case is rounded,
clamped, wrapped, or treated as a new measure-duration rule.

Only origins with established membership can be these witnesses: a displaced
sound position cannot certify its own containment, and an unsupported cursor
boundary crossing must not be used as an in-measure witness.

A narrow ordinary-prefix derivation can supply such an origin without assuming
a measure length. Begin at the structural start. The pinned duration annotation
(lines 6472–6474) defines ordinary non-chord cursor movement; the chord annotation
(line 6499) describes that movement as the musical position within a measure;
the offset annotation (line 3939) explicitly retains the current location in the
current measure, including its end. Along a complete ordinary-note prefix with
known exact duration units, an **observed subsequent unshifted current-location
origin** may therefore establish a membership bound. This is a derived provider
proof, not an added mandatory clause or a theorem equating duration with meter.
An origin at a backup/forward departure may be observed before processing that
movement, provided the incoming prefix is still proved. A terminal note end
alone is not an observed subsequent origin.

Backup/forward movement must not extend this bound from its destination merely
because it is serialized inside the measure. The source only recommends avoiding
boundary crossing. A movement with both endpoints inside an already proved
interval preserves membership; a destination outside that interval suspends
this prefix proof. Exact recovery at structural zero can resume it. An origin
reached through unknown units, invalid duration, or an unproved movement cannot
bootstrap a new bound. Neither chord/grace endpoints, attack/release endpoints,
figured-bass chain sums, shifted coordinates, nor unassessed right-barline
locations are independent cursor membership witnesses.

### B. Bounded cross-frame order

Two independently proved-contained positions retain the existing canonical
**score-order** precedence of their respective measures. This represents
structural before/after-barline scope; it is not a claim that their absolute
elapsed times are strictly separated. The adopted
[MusicXML 4.0 issue 28](https://github.com/w3c-cg/musicxml/issues/28) clarification
specifically retains an end-of-measure current location in the old measure.
There is no primary-source evidence here requiring ordinary old-measure-end and
next-measure-start state statements to become ambiguous. An unrelated floating
publication must not remove this independently established score-order result.

The stricter elapsed-time proof below applies when one or both positions lack
that containment proof. In particular an unproved displaced candidate cannot
claim structural precedence merely from its original measure label.

Conceptually retain unknown measure-origin times `T_m`. A local position is
`T_m + x`, not the tuple's lexicographic rank. Same-part structural measure
order and proved membership bounds can establish some relations without
knowing all lengths. If the origin distance from earlier frame `m` to later
frame `n` has proved lower bound `L`, then

`(T_n + y) - (T_m + x) >= L + y - x`.

A strictly positive right-hand side proves the earlier-frame event precedes
the later-frame event. A zero lower bound proves only non-strict order unless
additional facts exclude equality. Unknown upper bounds cannot establish the
opposite relation merely by flipping a comparison. Exact established boundary
spans could strengthen these proofs later; arbitrary meter or max-note span
substitution cannot.

Even the conservative first implementation can preserve common cases using
same-frame exact comparison, proven anchor containment, and source-frame start
relations, while marking only genuinely competing cross-frame candidates as
unavailable. Do not globally invalidate all timeline families or all positions
because one offset lacks an inter-frame relation.

### C. Query-specific sufficiency

An effective query needs both whether a candidate could apply by the query and
whether it could supersede the selected property statement. A candidate that
is proved after the query is irrelevant. A candidate proved before a later
unconditional same-property publication is superseded. A candidate of another
property is irrelevant. If competing statements have the same projected value,
a separately proved consensus may retain that value and its actual provenance.
Otherwise preserve local ambiguity and its exact source/cause.

Do not mark every same-frame query known automatically: another frame's displaced
publisher can still compete. Conversely do not discard a same-frame result
merely because some unrelated cross-frame offset exists. For unproved displaced candidates, a non-strict elapsed bound cannot supply a
strict supersession proof. This caution does not reopen the established
canonical precedence of two independently contained positions. Same-frame
same-position conflicts still require the existing no-invented-winner discipline.

## Dependent proof boundaries

A known local coordinate presupposes a uniquely established duration-unit
conversion. Marking an already-computed destination as outside the proved frame
does not repair an earlier unsupported unit choice. The existing cursor-unit
interpretation remains in force: no departure-only or piecewise convention is
invented for a movement that may cross differing divisions. This differs from
the separately documented origin-unit interpretation for a non-cursor offset.

The independent excursion regressions cover a backup after a changed division
value at the new measure start; a backup with unchanged current-start units that
may reach a prior mid-measure differently valued region; and a forward outside
the proved prefix that may reach a later-frame changed region. Uniform-value
controls retain exact arithmetic. Existing earlier complete prefixes remain
independent. A per-measure detector alone cannot establish that no remote region
can intervene. Nor is checking only differently valued publisher points inside
a tentative interval sufficient: in reverse travel the crossed boundary may
publish the departure value, while the earlier differently valued region began
before the tentative landing point. A conservative proof must cover applicable
regions, unavailable publications and the independent placement of its bounds.
The backup/forward recommendations remain advice, not a newly enforced error.

Direction/figured-bass first-following-note associations and grace donor queries
also depend on cross-frame order. An unproved cursor candidate cannot borrow
canonical order from its containing measure label. Exact same-frame comparisons
and two proved-contained frame positions retain their existing proofs. Both
candidate indexes and fallback scans must consider potentially competing
unproved candidates, including earlier-frame positive excursions and later-frame
negative excursions; a nominal ordinal prefilter is not enough.

Finally, an unavailable signed sound displacement has no established destination
interval merely because its unshifted source was in a known measure. Its
property-specific gap must retain that missing confinement and the actual
scalar/units cause. A contained earlier query cannot ignore it by ordinal, and
a nominally later publication cannot repair it without an independent
supersession relation. This differs from an unknown ordinary cursor with proved
within-frame membership. Only the selected sound displacement matters: an
ignored visual direction offset (`sound=no` by default), or a parent offset
replaced by an own sound offset, must not manufacture a displaced sound gap.

## Evidence still required for acceptance

The RED test establishes the missing chronology gate. A fix needs explicit
positive controls for negative offsets remaining inside a proved measure,
positive offsets bounded by another origin, source-order-independent witnesses,
negative later-frame and positive earlier-frame possible crossing, same-frame
ordering, unaffected property preservation, later proven supersession, causal
ownership and resource accounting. A successful local fact is not a claim that
repeat traversal, rendered playback, or every inter-measure span is known.
