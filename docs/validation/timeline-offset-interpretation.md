# Non-cursor offsets and analysis-key context

## Status and source boundary

The named timeline profiles use a reviewed, clause-derived interpretation for
non-cursor offset normalization and numeral-key context. MusicXML 4.0 does not
literally state a complete algorithm for either a harmony offset crossing a key
change or a scalar offset crossing a divisions change. This document states the
interpretation and its limits explicitly; it is not a claim that an importer or
a discussion amended the format.

Primary sources are the pinned MusicXML 4.0 schema annotations for `attributes`,
`direction`, `harmony`, `numeral`, `offset`, `sound`, `listening` and `divisions`,
corroborated by their official element references:

- [Attributes](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/attributes/): changes are effective in musical score order, not solely serialization order.
- [Offset](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/offset/): retain the current score location, graphical displacement, and explicit playback/listening effect as distinct facts.
- [Harmony](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/harmony/) and [numeral](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/numeral/): positioning data is distinct from the encoded key or local analysis key.
- [Sound](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/sound/) and [listening](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/listening/): an own offset overrides the containing direction offset.
- [Divisions](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/divisions/): musical divisions per quarter supply the scalar's units.

Importer examples and community discussion are corroborating implementation
history only. They are not rejection authority. In particular, another importer's
rounding, integer coercion, staff selection or negative-position repair is not
adopted.

## Distinct facts

1. Preserve the unshifted reconstructed score origin with canonical part,
   measure occurrence, exact quarter coordinate and staff/voice evidence.
2. Preserve a separate nominal offset/display-time coordinate. It is not a
   final graphical x-position: default-x and later layout precedence still
   belong to the graphical consumer.
3. Query a numeral's applicable encoded key at its unshifted part/staff origin.
   A visual offset or its sound flag does not move that analysis context under
   this interpretation. A valid own numeral-key remains independently sufficient.
4. Sound/listening use the original containing origin. An own local offset
   replaces, rather than adds to, a parent direction offset. Without an own
   offset, the parent displacement affects these channels only with sound=yes;
   the documented absent flag means no.
5. Normalize one signed non-cursor scalar with divisions established at its
   origin after relevant musical-order publications are complete. Do not use a
   stale lexical register merely because the event was encountered first.
   Offset conversion never advances the serialization cursor.

Neither missing nor malformed display conversion destroys independently known
analysis-key origin evidence. Invalid source data may still have an independent
XSD finding. Truly unavailable origin, staff, identity, history or units retain
their precise causal availability; they do not become defaults or success.

## Local coordinates and cross-frame applicability

An exact signed coordinate is relative to its measure origin. It is not proof
of the elapsed distance to another measure, and a displaced publisher cannot
obtain chronological precedence just from its serialized measure ordinal.
Same-frame rational comparison remains exact. Two independently contained score
positions retain the canonical structural order of their measures, including
the documented end-of-measure current location; this is score-order scope, not
a claim that absolute elapsed times are strictly separated.

The [cross-frame source review](evidence/timeline-cross-frame-review.md) records
the selected sufficient-proof boundary. An observed, established unshifted
current-location origin can bound a displaced coordinate inside its measure.
An ordinary-note prefix can establish such an observed origin; a terminal note
end alone cannot. Backup/forward outside an already proved interval suspends
that prefix proof. No nominal-meter, maximum-note-end, final-cursor, graphical
barline or shifted-position rule invents a measure length or certifies its own
containment.

Effective continuing-state queries must consider whether a displaced candidate
can apply and whether it can supersede an established statement. Proven later
or unrelated-property candidates do not erase independently known state.
Unsettled competing relations retain local unavailable evidence and causes;
they do not reject the document, clamp/wrap the offset, or affect the independent
unshifted numeral-key query. Concrete implemented proof cases and limitations
are recorded in the ownership ledger separately from this source contract.

## Independently worked expectations

- No encoded key before q=2; harmony origin q=1 with visual offset +2: the
  numeral-key fallback has known absent key at q=1. A later key at the displayed
  position does not satisfy this origin-context requirement.
- Key begins at q=2; harmony origin q=3 with visual offset -2: its origin-context
  key remains available even though the nominal graphic coordinate is q=1.
- D=2 before q=1 and D=4 from q=1; origin q=3/2, scalar offset -3: origin-unit
  conversion gives q=3/4. No destination-unit fixed point or piecewise integral
  is introduced.
- D=2 at q=0; a duration-2 note reaches q=1; a harmony at q=1 is serialized before
  a same-position attributes change to D=4. Its offset 2 normalizes to 1/2 and
  its nominal display coordinate is q=3/2. Finalized score-order state, rather
  than encounter order, supplies the units.

These are derived fixtures, not official worked examples. Exact executable
file/function names, target qualification and implementation snapshot are in
[timeline-assessment-ownership.json](timeline-assessment-ownership.json).

## What this does not settle

The scalar non-cursor interpretation does not settle how a backup/forward
cursor movement crossing a divisions boundary establishes its own departure,
destination or event sequence. `B6-DIVISIONS-REWIND` remains independently
scoped: ordinary movements wholly within one proved region are supported;
unproved crossing/departure or late changed cursor publishers retain narrow
causal gaps and preserve already known prefixes. No must-reject rule is inferred
from a should-level boundary recommendation.

It also does not select an application-specific winner among simultaneous note
associations, execute repeats, choose playback passes or determine rendered
layout. Instrument property-update semantics are separate from these origin
and unit facts.
