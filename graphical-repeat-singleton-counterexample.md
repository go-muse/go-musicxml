# Bounded singleton graphical-repeat counterexamples

This extension adds one independent sufficient counterexample to the existing
MusicXML 4.0 beat-repeat and measure-repeat content consumers. It does not negate
the older sufficient representation-equality proof, change its accepted cases,
or claim general musical equivalence, playback realization, or full repeat
semantics.

## Necessary property

In complete, independently established source and target windows, suppose the
only source event is one ordinary pitched event and the only target event is
one ordinary pitched event or rest. Both have the same explicit staff, voice,
resolved instrument, relative onset, intended duration, and one known common
encoded chromatic/octave transformation. If their exact canonical encoded pitches
differ, or the target is a rest, the sole target event cannot supply the source's
required pitched event. Complete enumeration excludes another candidate.

The source obligations are the released MusicXML 4.0
[beat-repeat](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/beat-repeat/)
and [measure-repeat](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/measure-repeat/)
requirements to encode the repeated music. Canonical arithmetic follows encoded
[pitch](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/pitch/),
[alter](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/alter/),
[duration](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/duration/),
and [transpose](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/transpose/).
The singleton theorem is an implementation inference from these clauses, not a
standardized universal equality algorithm. It says nothing about acoustic
frequency, instrument tuning, renderer policy, or independent author intent.

## Deliberately closed initial implementation

The new private fallback runs after existing positive and empty-target proofs,
and only on an unresolved start row with complete note enumeration, an explicit
numbered staff, no except-voice qualification, a proved complete range, and an
explicit stop. Beat windows remain in one measure with a positive exact beat;
measure-repeat currently requires a one-measure pattern.

Its own direct music-data roster includes events lacking timeline facts. It
independently checks complete child and attribute axes and exact declaration
bindings. The allowed part domain consists of ordinary notes and attributes
containing divisions, staves, one staff-qualified transpose, or supported
measure-style declarations. A small set of nonperformance note spelling fields
is accepted. Unsupported data causes unavailability, never exclusion by omission.

Every note in the canonical part needs an explicit resolved instrument, voice,
and staff; a positive exact duration; and an independently contained onset and
end in its own measure. A terminal note end alone does not prove containment.
An explicit later in-measure position can establish the needed bound. No meter
capacity or fabricated part-end position is used. Source-order nonoverlap is
checked before binary beat-window selection.

Exactly one explicit staff-qualified transpose publication and no relevant gap
are required. Its chromatic/octave contribution must be known, with no doubling,
and the publication must precede both events. Additional publications decline
this first theorem even if a later, more general proof could establish equal
transformation. Absent transpose is not silently interpreted as zero.

The complete attribute whitelist excludes attack, release, time-only, hidden
notes, and other note-level modifiers. The complete child/event whitelist excludes
ties, grace, cue, chord, playback instructions, sound changes, listening data,
technical/ornament data, scordatura, capo, and unresolved tuning transformations.
Initial MIDI/play mappings also withhold this theorem. Multiple events in either
selected window, differing onset/duration, different identity context, missing
closure, or unknown/invalid prerequisites never become a new false result.
Unrelated staff events can conservatively withhold the initial whole-part gate.

`performancePitch` supplies exact rational C0=0 arithmetic. Known absent alter
uses its established natural projection. Enharmonic spellings, fractional
alterations, and octave-edge spellings with equal canonical pitch are never
rejected by this theorem. Changed transposition producing equal pitch is outside
its closed domain. Equal written notes under explicit transpose remain whatever
the unchanged positive proof previously established; this fallback adds no
positive proof.

## Resource and evidence contract

The additional event roster is lazily admitted and released. Independent part
records, instrument references, note/measure slices, and pending results have
prepaid scratch bounds. Variable strings, declaration checks, history scans,
window searches, exact arithmetic, and result publication are charged and
cancellable. The fallback stages results and publishes only after all work and
final admission succeed, before the provider is finalized and before consumers
use its new proofs. Existing positive and empty-target results are not rolled
back or erased. An error still aborts provider completion according to the
existing assessment contract; earlier independently recorded conformance
findings remain intact. Public projections independently own causal references.

No public API, capability, renderer, tuning dictionary, external-intent provider,
or navigation traversal has been introduced.

## Regression evidence

- `TestSingletonRepeatIndependentCounterexamples`: C4→D4 and C4→rest, both repeat
  consumers, source/current-model × partwise/timewise. The new tests were observed
  RED with the old transposed-equivalence Unknown before implementation; GREEN
  requires both Known(false) context and the mandatory consumer's failure.
- Canonical-pitch, transformation, ordinary-domain, encoded-duration-unit, and
  containment controls retain Unknown outside the theorem and never reject equal
  enharmonic pitch.
- `TestIndependentSingletonRepeatResourceBoundary`: exact/one-under work and
  independently located live-storage boundaries, every cancellation poll, no
  partial negative publication, and complete scratch release.
- `TestIndependentSingletonRepeatOwnedPrerequisiteCauses` and incomplete-domain
  tests preserve prerequisite causes and closed-domain boundaries.
- Existing sufficient equality, multivoice multiset order, empty-target,
  trailing-forward, and resource assertions remain unchanged.

General repeat-content equivalence remains unresolved beyond the independently
proved positive and negative cases. This extension does not close that domain.
