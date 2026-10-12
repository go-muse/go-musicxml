# Normal-dot omitted-group boundary

Reviewed 2026-10-12 against the pinned MusicXML 4.0 XSD, lines 5683–5710,
and the official 4.0 reference:

- https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/time-modification/
- https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/normal-type/
- https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/normal-dot/
- https://www.w3.org/2021/06/musicxml40/musicxml-reference/examples/tuplet-dot-element/

The optional grammar group contains normal-type followed by zero or more
normal-dot elements. The normal-type prose describes the group when the normal
note type differs from the current type. It does not separately define the
semantic dot-count fallback when that entire group is absent. The official
example uses a dotted eighth current note and explicitly supplies normal-type
of eighth with no normal-dot to describe its normal side.

Narrow decision: raw count of observed normal-dot elements can be zero, but
that must not be confused with a proved effective normal-side dot count.
An explicit normal-type with no normal-dot supports zero. An absent group
combined with a dotted current note retains unknown effective normal dots;
there is no new invented current-dot inheritance or unconditional zero rule.
Unknown current dot evidence remains unavailable. A known undotted current
note and absent group can retain zero where the interpretations agree.
Tuplet per-field fallback propagates this uncertainty only to the affected dot
field; independent count/type fields remain available. This is a bounded
projection interpretation, not a new conformance rejection or closure of the
broader source question.

Searches of official reference and primary issue sources did not locate a more
explicit 4.0 omitted-group dot rule. No third-party implementation was used as
normative authority.
