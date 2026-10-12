# Grace neighbors and figured-bass origin evidence

## Grace: derived sufficient proof, not a performance scheduler

The [grace reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/grace/)
describes individual percentages of previous/following note time and make-time
in divisions. It does not specify a universal donor tie-break or an omitted
percentage default. The [voice reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/voice/)
describes a linear musical sequence within a part. Together these support the
provider's explicitly derived sufficient proof:

- Complete explicit voice streams are independent of staff. A wholly unvoiced
  complete part is usable only when its musical sequence is proved linear;
  no voice 1 is invented.
- A grace run skips its grace peers and refers to proven ordinary neighbors.
  Each grace retains its own percentage, exact fraction, donor duration and
  stolen-duration amount. No aggregate allocation or redistribution is implied.
- Neighbor identity may cross complete measures or forward gaps. Identity is
  distinct from continuous elapsed time between the events.
- Missing percentages remain absent. Grace duration/end is never fabricated.

Raw `NoteOrigins` records stay immutable. Separately finalized `GraceNeighbors`
records contain the supported donor association and calculation. An unresolved
neighbor slot in the raw record is not the final association answer.

Chord donors with competing identities/durations, overlapping same-voice events,
mixed explicit/absent voice, missing kinds/positions and unclosed domains retain
precise local unavailable evidence. These are batch-6 provider evidence
boundaries, not an excuse to defer every ordinary grace association to playback.
Batch 7 retains realized performance redistribution and engraving decisions.

## Figured bass: independent anchor, duration and chain

The [figured-bass reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/figured-bass/)
anchors a figure to the first following ordinary note in score order, excluding
grace and chord notes. It specifies top-to-bottom figure order and optional
duration for changes under a note. No staff/voice filter or same-measure ceiling
is stated. The [duration reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/duration/)
does not make figured-bass duration a shared note-cursor mover.

The provider therefore retains separately:

1. The unique following regular-note identity and exact anchor position across
   complete canonical part/measure domains. Equal-position competing notes are
   unresolved; lexical order is not a tie-break.
2. Raw optional duration, finalized cursor and anchor divisions, selected units,
   normalized duration and independent anchor-relative end. Duration uses the
   finalized units at the documented anchor position. Cursor units are retained
   as provenance, not an equality gate. Absent duration does not inherit note
   duration.
3. For a proved consecutive shared-anchor group, its group start and cumulative
   exact chain onset/end with the explicit interpretation label
   `consecutive-figured-bass-duration-chain`. These do not replace anchor facts.

Musical-order divisions must be finalized before these calculations; an earlier
lexical register is insufficient when a later serialized attributes record is
at the same score position. A missing prior duration cannot invent a chain step.
Interleaved/nonconsecutive group membership remains a local reconstruction
boundary. End coordinates stay in the established anchor measure frame; no
measure-length wrapping is guessed. Chain positions are derived sums of the
established anchor-based durations, not a second unit-origin interpretation or
a fully rendered schedule. A later divisions value at a derived chain position
does not rewrite the element's documented anchor-based duration.

The duration unit origin follows directly from three published clauses: the
figure takes its score position from the following regular note; duration is in
divisions; and [attributes changes apply in score order](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/attributes/).
Consequently different or missing cursor units do not block a known anchor-unit
duration. Missing or unavailable anchor units retain their own causal evidence.
`TestFiguredBassDurationUsesFinalizedAnchorUnits` covers differing known units,
missing cursor units, invalid anchor units and omitted duration.
`TestFiguredBassUnavailableAnchorCauseAndRawAbsence` separately checks an
unavailable anchor identity without inventing duration from raw absence.

## Clarification evidence and limits

The common consecutive-duration interpretation is discussed in
[issue 645](https://github.com/w3c-cg/musicxml/issues/645), including
[the onset example](https://github.com/w3c-cg/musicxml/issues/645#issuecomment-4293539090)
and [its duration interpretation](https://github.com/w3c-cg/musicxml/issues/645#issuecomment-4293761129).
This is corroboration for a labeled derived context fact, not a new MusicXML 4.0
validity rule. The provider does not infer a sum-of-figure-durations <= attached
note-duration restriction; [the discussion rejects that proposed bound](https://github.com/w3c-cg/musicxml/issues/645#issuecomment-4300322734).
[General overlap interpretation remains disputed](https://github.com/w3c-cg/musicxml/issues/645#issuecomment-4311290373).
[A suggested same-measure restriction](https://github.com/w3c-cg/musicxml/issues/645#issuecomment-4332370420)
is not adopted MusicXML 4.0 text and does not justify withholding an ordinary
unique anchor in the next measure.

The pinned XSD annotations and official 4.0 element definitions remain the
format authority. Importer examples are corroborating controls only. No importer
fallback duration, inherited voice label, rendering attachment convention or
new must-reject condition is adopted.

## Integration and evidence

Finalize all event facts, state publishers and note-local origins first. Run
`resolveGraceNeighbors` and `resolveFiguredBassOriginsAll` over all normalized
measure blocks before declaring the service final. The single-block figure
wrapper is for isolated fixtures, not a production same-measure boundary.
Transactional output, metered scratch/index allocation, exact comparison,
cancellation and causal ownership are part of the provider contract.

Concrete file/function scopes, source/current-model qualification and the tested
snapshot are in [the ownership ledger](timeline-assessment-ownership.json).
Shared-fact provider budgets do not establish that the public-adapter stress
fixtures pass. Final integrated checks and independent resource review remain
separate acceptance evidence.
