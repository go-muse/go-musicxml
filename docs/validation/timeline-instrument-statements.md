# Effective encoded instrument statements

## Reviewed provider contract

Batch 6 models identity-qualified encoded statements, separately from a realized
synthesizer configuration. Its sparse top-level update interpretation is derived
from MusicXML's change semantics and independent property meanings. The standard
does not literally say every omitted child inherits; this document states the
bounded interpretation rather than claiming a new express format rule.

The initial declaration and every positioned change retain their raw fields.
Known omission of a top-level property makes no new statement. A property's
latest applicable established statement remains available with its actual
source, position and inherited status. Initial absence means no encoded
statement, not an application-selected playback default.

The pinned source basis is MusicXML 4.0 schema `score-instrument` (6046–6070),
`instrument-change` (3685–3691), `virtual-instrument-data` (6221–6244),
`midi-instrument` (2674–2721), `midi-device` (2664–2671), `play` (2747–2766)
and `sound` (4123 onward). Official references corroborate the same boundaries:

- [Instrument change](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/instrument-change/) changes one verified score-instrument identity.
- [MIDI instrument](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/midi-instrument/) supplies independent optional playback fields; its formal initial parent is score-part.
- The [pan/elevation example](https://www.w3.org/2021/06/musicxml40/musicxml-reference/examples/pan-and-elevation-elements/) encodes spatial updates without restating channel or program.
- [MIDI device](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/midi-device/) expressly makes an omitted id apply to every score-instrument in that score-part.
- [Play](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/play/) distinguishes continuing sound instructions from current-note instructions.

Neither importer behavior nor optional schema occurrence alone proves a reset or
merge rule. No new validity predicate follows from this provider interpretation.

## Property granularity

1. Instrument-sound is an independent statement. Explicit empty xs:string is
   a new empty statement; an absent child is no update.
2. Solo/ensemble is one exclusive performance statement. Explicit solo replaces
   ensemble and explicit ensemble replaces solo. An empty ensemble states an
   unspecified size, so it does not inherit an earlier numeric size.
3. Virtual-instrument is one atomic descriptor statement. If the element is
   omitted, there is no descriptor update. If present, its current library/name
   fields replace the prior descriptor as a whole, including raw absence. A new
   library must not acquire a patch name copied from an earlier descriptor.
   An empty descriptor is an encoded empty descriptor, not a proved hardware
   reset.
4. MIDI channel, name, bank, program, unpitched note, volume, pan and elevation
   select independently in musical score order. The provider neither emits MIDI
   commands nor infers physical controller state across instruments or devices.
5. Device id absence is symbolic all-instruments scope within its canonical
   score-part. It is distinct from unknown or invalid id evidence. Do not expand
   an arbitrary encoded count into an array. Later all-instruments and earlier
   identity-specific statements compete by position, with no invented same-time
   specificity winner.
6. Play retains its typed technique list and current-note/following-note extent.
   The explicit midi-device all-instruments omission rule does not automatically
   apply to play. Repeated heterogeneous techniques, arbitrary other-play types
   and note-local playback overrides do not acquire a generic last-write-wins
   algebra merely because their records are retained.

Counts, declarations and note selection remain separate. Count one identifies
no instrument ID. Note instrument references are plural and note-local; a
previous note's choice is never implicitly inherited.

## Availability and provenance

The [sound reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/sound/)
and pinned `sound` annotation (schema line 4141), together with `time-only`
(lines 581–587), restrict the sound element to the specified repeat passes.
This applies to its instrument-change, MIDI and device updates, not just its
scalar tempo or navigation attributes. The provider retains each raw encoded
value and its original `TimeOnly` evidence independently. Without established
repeat-pass evidence, an effective property stated by a conditional publisher
is unavailable; the provider does not assume that the first or current pass
matches. A known omitted property still makes no update, so a guard cannot
erase an unrelated earlier statement merely through omission.

Invalid field or condition evidence retains its original cause ahead of a
synthetic repeat-pass-unavailable result. A later unguarded statement for the
same property can restore known effective evidence; a different property's
update cannot. Same-position competition preserves the established conflict
and causal-precedence rules rather than inventing lexical order. These are
bounded provider evidence rules, not repeat execution, pass-number ordering
validation or a new musical rejection predicate.

A malformed, unsupported or hidden possible update is not omission. It retains
the original cause for the affected property. A later update to another property
does not repair it; a proved later update to that property can restore the
property after its relevant gap interval. Different properties at the same
position can both be known, while incompatible statements for the same property
remain ambiguous.

An unresolved instrument target with an established musical position introduces
point-local uncertainty for its stated properties in the canonical part. It
cannot invalidate a proved earlier prefix or properties omitted by that record.
A strictly later known statement for the same property may repair the result;
an equal-position statement is not silently treated as later. The unresolved
record retains its original target cause, raw fields, repeat condition and
position. Only a genuinely unplaced publisher needs bounded measure-interval
uncertainty. This is the provider's score-order evidence boundary, not an excuse
to discard the original XSD identity failure or manufacture a target identity.

A property query names its actual selected source and inherited state. A
compound convenience projection may aggregate sources, but may not label every
field as if it were published by the latest unrelated change. Copies, indexes,
provenance and exact comparisons remain session-accounted and cancellable.

## Required distinguishing cases and later owners

- Prior ensemble 12, then empty ensemble: unspecified current size
- Prior ensemble, then solo: current solo rather than two active choices
- Prior library A/patch X, then library B only: no manufactured B/X descriptor
- Explicit empty sound string versus no sound child: different statements
- Invalid program, then pan-only update: program cause retained, pan independent
- Older instrument-specific device, then later id-less device: later scoped
  all-instruments statement participates without arbitrary count enumeration
- Guarded program-only update: program needs repeat-pass evidence, while an
  omitted channel preserves its independent earlier statement
- Guarded invalid value: preserve its actual cause; a later unconditional update
  repairs only the same property
- Known-position invalid target: preserve earlier property state, retain the
  exact identity cause from that position, and permit strictly later same-field
  repair without poisoning omitted fields

Exact executable cases and the inspected implementation snapshot are recorded in
[timeline-assessment-ownership.json](timeline-assessment-ownership.json).
Documentation alone does not establish implementation or adapter coverage.

Batch 7 retains actual patch/reset realization, interactions between MIDI and
virtual descriptions, technique composition, repeats and playback passes.
Batches 8–9 retain package/library availability and external sound dictionaries.
Those owners are not closed by the encoded-statement provider. The separate
shared validation kernel/session obligation B5-PERF-01 remains outstanding for
final batch 9 reconciliation.
