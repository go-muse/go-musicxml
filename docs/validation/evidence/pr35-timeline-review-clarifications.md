# PR 35: bounded clause and ownership clarifications

Reviewed baseline: `632a9690be60586daf68996914b1fdcb353c4a38`.
Date: 2026-10-12. These clarifications do not add a new normative rejection rule.

## Rest principals and the selected duration bound

[Reviewer question](https://github.com/go-muse/go-musicxml/pull/35#discussion_r4246488620).

The selected predicate uses the established preceding non-chord `note` element,
including a duration-bearing rest. The
[MusicXML 4.0 chord reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/chord/)
identifies that element relation when explaining cursor movement and the duration
bound. The [note grammar](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/note/)
permits pitch, unpitched and rest alternatives after the optional chord marker.
The [duration reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/duration/)
conditions note movement on absence of the chord child, without a pitch/rest
exception. The [official tutorial](https://www.w3.org/2021/06/musicxml40/tutorial/midi-compatible-part/)
illustrates pitched chords and unequal durations, but supplies no exhaustive
pitch-based membership restriction.

The description's musical chord-tone language motivates the review question.
It does not, in these sources, establish a rest-specific exception to the
numeric predicate. Schema validity alone also does not settle prose semantics.
The bounded interpretation therefore keeps the known duration comparison and
states its limit: passing it does not certify conventional musical meaning,
rendering, or interoperability of a rest-containing group. No search past a
rest for an older pitched anchor, new malformed-rest-chord error, or fabricated
Unknown/NotApplicable gate is introduced. Existing grace, interrupted-anchor,
unavailable-duration and unit-evidence rules remain intact.

Independent baseline probes used 72 exact fixtures: partwise/timewise,
rest/cue-rest/pitch/unpitched principals, pitch/rest/unpitched chord members,
and durations 2/4/8 against principal 4. All 72 passed the bundled official XSD
with real offline xmllint and its XML catalog. All 144 source/model assessments
matched the expected selected duration outcome. This is neither a musical
renderer test nor an assertion that the schema alone proves the interpretation.
The maintained `TestTimelineRestPrincipalDurationCharacterization` covers the
shorter/equal/longer controls, the 1-then-3 principal-bound control, rest-marked
members, both score forms and both public adapters. These characterize existing
behavior; they are not presented as a failing-regression implementation change.

## Closed state-record reflection

[Reviewer maintainability note](https://github.com/go-muse/go-musicxml/pull/35#discussion_r4246488610).

Reflection remains restricted to the closed private state-record graph; the
implementation now documents its non-nil pointer allow-list and ownership
purpose next to `stateMeasure`: `DecisionRef` for causes, `Evidence[string]` for
optional sound time-only conditions, and `musicalPosition` for gap/placement
witnesses. Those records are recursively copied after admission. Immutable
Integer, Decimal and Rational wrappers are measured as terminal values and may
share their private numeric storage. New pointer-bearing records require an
explicit accounting/ownership review. This documentation choice does not claim
compile-time enforcement or a performance improvement. Existing resource,
mutation, cancellation and cleanup tests remain the executable safeguards.
