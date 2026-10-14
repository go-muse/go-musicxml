# Bounded MusicXML 4.0 barline location

## Source and accepted scope

The pinned [barline clause](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L3225-L3246)
requires location agreement. Its first/last musical-data placement language is
`should`, not the predicate implemented here. The official
[MIDI-compatible-part tutorial](https://www.w3.org/2021/06/musicxml40/tutorial/midi-compatible-part/)
permits attributes before a left barline and demonstrates a trailing forward
restoring an earlier voice endpoint. Resolved
[MusicXML issue 28](https://github.com/w3c-cg/musicxml/issues/28) establishes the
current-measure end interpretation after the last note.

This is the bounded implementation of `CHECK-barline-location`, using the
reviewed terminal-note/rest and restoring-forward interpretation. It does not
claim a universal measure-duration reconstruction algorithm.

- Left compares the exact unshifted cursor with structural origin zero.
- Right, including known omission of location, compares with a certified end.
- Middle requires a positive cursor strictly before that end.
- Independently contained current locations supply lower bounds. A strictly
  later lower bound disproves right or proves positive middle without needing
  a complete endpoint.
- Empty, grace-only, forward-only and new-maximum trailing-forward measures
  retain unavailable endpoint evidence. Middle at zero still fails.
- Meter, `implicit`, printed measure number, first/last XML-child placement,
  display/effect offsets and figured-bass duration do not define the boundary.
- There is no special `rest@measure` certificate. An ordinary terminal rest
  uses its encoded duration in the same way as an ordinary terminal note.

## Private evidence and closure

The selected timeline supplies canonical part/measure identity, exact origins,
normalized durations, principal-note relations and causal unavailable evidence.
The predicate enumerates the exact containing music-data stream and rebuilds a
private contained lower bound, explicitly excluding every barline as a witness.
It does not read or update the shared `frameBounds` maximum for this proof and
never changes `UnprovedFrame`, cross-measure ordering or repeat `partEnd`.

A terminal ordinary-note certificate requires complete relevant enumeration,
known note branches and positive duration/units, no negative or unresolved
movement excursion, and no earlier note/chord/cursor extent beyond its end.
Trailing metadata, grace and valid chord tones do not advance the cursor.
An overlong chord blocks this dependent certificate and preserves the selected
chord predicate's failure reference; it is not a second barline violation.
A restoring-forward certificate additionally requires an earlier ordinary note
endpoint at the same destination and independent contained coverage of every
backup/forward departure and destination. A forward cannot establish a new
maximum by itself.

Unknown declarations or missing timing facts are not zero-time assumptions.
Invalid relevant scalar and structural causes survive. Unrelated bad pitch or
color does not poison otherwise valid timing. Independent left or lower-bound
proofs need not wait for irrelevant suffix extent facts.

All loops and exact comparisons charge the session account and poll
cancellation. No pass or violation is emitted after an incomplete end proof.
Already proved failures survive diagnostic truncation and dependency-cache
resource exhaustion.

## Composition and compatibility

`validation.NewBarlineLocationPlan` accepts `ContextPlanSpec` with the exact
selected timeline rule. It is a normative mandatory node predicate with the
exact barline type selector and document timeline dependency. The existing
context binding table and semantic revisions are unchanged.

`addBarlineLocationAssessment` only appends the reviewed rule/plan definition.
New explicit completion profiles may select it. Frozen source/model timeline
and context profiles do not select it and retain their old digests.

## Executed test receipt

Author worktree base: `8a3423bd3e93cee3b1db4bc72ddc0d2bae07e3c4`.
Official toolchain: `/workspace/shared/pr37-recovery-tools/env.sh`.

1. Initial API/compile RED: the first adapter fixture failed to build because
   `addBarlineLocationAssessment` did not yet exist. This was not a behavioral
   failure.
2. Separate behavioral RED: a compiling evaluator stub returned Unknown for
   every selected barline. The 20-case source/model × partwise/timewise matrix
   failed its expected passes/failures with
   `barline-unimplemented-behavioral-red`.
3. The restored implementation passed that 80-leaf matrix.
4. Expanded tests exercise metadata, grace/chords, causal invalid durations,
   unknown enumeration/location, negative excursions and unrelated invalid
   fields on all four axes. Positive matrix results are checked using exact
   Pass-count differences against the identical frozen timeline catalog, not
   merely absence of diagnostics.
5. Unit tests cover private-evidence nonmutation, omitted timing facts,
   barline-witness exclusion, independent proof retention, relevant causal gaps,
   exact and one-under work/buffer allowances, cancellation at every observed
   poll and known-failure retention after diagnostics/cache exhaustion.
6. A defensive malformed-known-Rational test first produced a nil-pointer
   panic on a hand-built private service. The hardened consumer rejects this
   impossible-through-adapters record as unavailable instead. Normal timeline
   values come only from initialized cursor/arithmetic evidence.

Focused commands:

```
go test . ./validation -run '^TestBarlineLocation' -count=1
go test . -run 'Frozen' -count=1
```

Independent review and final integrated-profile execution are separate from
this author receipt; no independent acceptance or publication is implied here.
