# Direction association: typed origin provider

## Clause and source boundary

This provider implements the ordinary typed evidence portion of
`MX40-PROSE-direction-note-association`. Its source is the pinned direction
annotation at `schema/musicxml-4.0/musicxml.xsd`, lines 3462–3466, corroborated
by the [MusicXML 4.0 direction reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/direction/).
The annotation describes optional attachment to the first following note in
score order, excluding a different voice. It does not require every direction
to have an associated note, and does not specify a tie-break between notes at
the same established musical position.

The [voice reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/voice/)
describes a voice as a temporal sequence within a part. The
[staff reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/staff/)
describes staff assignment, but the association clause does not impose an
additional equal-staff restriction. Accordingly a voice may be followed across
staves; malformed or missing staff evidence does not destroy a voice-only
association. Staff evidence remains available independently for consumers that
actually need it.

The [offset reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/offset/)
distinguishes graphical displacement from effects on sound and listening. No
clause makes a graphically shifted position the search origin for attachment.
The runtime must preserve its unshifted cursor coordinate before computing an
offset. This provider never changes a cursor or playback position.

## Contract

`timelineEventFact.Origin` preserves the unshifted coordinate independently of
`Position`. `timelineService.resolveDirectionOrigins` runs after all measure
facts are published and stores one `directionTimingOrigin` per exactly bound
direction. Runtime orchestration supplies all normalized measure blocks and
must populate `Origin`; it is deliberately not reconstructed from offsets.

Each record contains the direction identity, raw part/staff/voice evidence,
cursor origin, existing display position, optional associated note identity,
and the associated note's position/staff/voice. Sound/listening positions remain
their own independently addressable facts. There is no claim that a display
offset coordinate is the final rendered horizontal position.

The resolver searches only the established canonical part, using measure
occurrence identity, measure ordinal and exact rational musical position. It
does not sort printed measure numbers, infer runtime roles from names or paths,
or treat XML/model enumeration as a musical tie-break. Explicit voice strings
match exactly, including the valid empty `xs:string`. An omitted direction
voice adds no voice restriction and does not invent voice `1`; an explicit
voice with missing candidate voice retains unavailable evidence.

Notes at the direction's cursor position are eligible. The nearest unique
compatible note is known even if source order differs due to backup. Notes are
not excluded merely for containing grace or chord markers: the direction
annotation names notes generally. Distinct eligible notes at an equal earliest
position remain unknown, including unresolved simultaneous/grace/chord groups.
This is the remaining narrow tie-order gate, not a blanket grace/chord exclusion.

The search may continue into later measures of the same fully enumerated part.
If the complete search has no compatible following note, the association is
absent. It is not a validation failure. Invalid or unavailable prerequisites
retain their state and exact causal decision. A gap only matters if it could
precede or tie the selected note; a later exact-position voice gap or later
measure gap cannot invalidate an earlier proved note. A note with unavailable
position can affect its own measure, but cannot affect an earlier measure.

## Ownership and bounded work

The provider consumes shared declaration/scalar/timeline evidence; it has no
parallel XSD interpretation or string-based selector machinery. Every output
map record charges one context fact and retained fixed storage before
publication. Iteration, exact comparisons and string comparison lengths are
work-metered and cancellable. `ownOrigin` copies causal pointers in the newly
assembled record before publication, without copying previously admitted
immutable numeric values. No partially built record is published on failure.

## Verification and remaining integration

`validation/timeline_direction_origins_test.go` exercises both source-order and
model-content-order provider inputs, exact score order, independent malformed
display offsets, voice selection, cross-staff selection, absent and empty voice,
grace/chord eligibility, genuine equal-time ambiguity, cross-measure lookup,
complete absence, exact declaration roles, document isolation, causal ownership,
local gap relevance, work/storage/context budgets, cancellation and duplicates.
These are internal provider tests, not a claim of new public consumer coverage.

The orchestration owner adds `Origin` population and the final all-measures
resolver call, then verifies both public adapters. Batch 7 still owns any
rendering, performance transformation, decision to actually attach directions,
and application-specific equal-time ordering. No ordinary known association is
deferred to that consumer work. No registry ownership status is promoted by this
document alone.
