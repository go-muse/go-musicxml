# Bounded MusicXML 4.0 barline location

## Source and accepted scope

The pinned [barline clause](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L3225-L3246)
requires agreement with placement among the musical data. Its first/last
XML-child language is `should`, so placement recommendations alone do not
establish mandatory violations. Both ordered placement and musical cursor
position constrain this bounded predicate. The official
[MIDI-compatible-part tutorial](https://www.w3.org/2021/06/musicxml40/tutorial/midi-compatible-part/)
permits attributes before a left barline and demonstrates a trailing forward
restoring an earlier voice endpoint. The
[barline reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/barline/)
also makes ordered placement explicit. MusicXML issue 28 concerns direction
`offset` after the last note; it does not establish barline semantics and is not
used as authority for this predicate.

This is the bounded implementation of `CHECK-barline-location`, using the
reviewed terminal-note/rest and restoring-forward interpretation. It does not
claim a universal measure-duration reconstruction algorithm.

- Left passes only at exact unshifted cursor zero with no earlier
  note/backup/forward in the ordered music-data stream. Returning to zero after
  earlier movement is Unknown (`barline-left-placement-unestablished`), not a
  fabricated placement violation. A known nonzero cursor still fails.
- Right, including known omission of location, compares with a certified end.
  A trailing right barline after a shorter final voice remains Unknown
  (`barline-measure-end-unequal-voices`) rather than failing because an earlier
  voice extended farther.
- Middle requires a positive cursor strictly before that end.
- Independently contained current locations supply lower bounds. A strictly
  later lower bound proves positive middle without needing a complete endpoint.
  It disproves right only when a known note/backup/forward follows the queried
  barline in the ordered stream.
- Empty, grace-only, forward-only and new-maximum trailing-forward measures
  retain unavailable endpoint evidence. Middle at zero still fails.
- Meter, `implicit`, printed measure number, strict first/last XML-child
  recommendations, display/effect offsets and figured-bass duration do not
  independently define the boundary.
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

The movement-order scan uses exact declaration bindings, requires ordered
complete enumeration, and verifies the queried barline belongs to that stream.
It allocates no retained scratch storage. A left-prefix check stops at the
barline, so unknown suffix declarations cannot erase a proved beginning.

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

Original author worktree base (unpublished local):
`8a3423bd3e93cee3b1db4bc72ddc0d2bae07e3c4`.
Official toolchain: Go 1.27.2.

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

## PR 41 ordered-placement correction (2026-10-16)

Review comment 4291055338 identified two cursor-only misclassifications. A
four-quarter first voice followed by a two-quarter second voice and XML-last
right barline produced Fail; a left barline after a backup to zero produced
Pass. The first outcome is now Unknown with unequal-voice evidence. The second
is Unknown with unestablished left placement. This preserves the distinction
between mandatory location agreement and recommended first/last XML ordering.

Behavioral RED on unpublished local base `9f49784` used the real
`ProfileSourceSemantics` and `ProfileModelSemantics` entry points for partwise
and timewise inputs. The new 40-leaf ordered-placement matrix had 20 failing
leaves: explicit/default right, metadata after right, left after backup, and
left after forward/backup. The other 20 control leaves already passed. The RED
test commit is unpublished local `77a1830`. A separate defensive test showed
that merely removing the shortcut still let a terminal cursor below a certified
end reach the final comparison. That path now also preserves unequal-voice
Unknown.

On unpublished local runtime commit `264ade8`, Go 1.27.2 passed:

```
go test . ./validation -run '^TestBarlineLocation' -count=1
```

The source/model adapter package passed in 1.498 seconds and the private
validation package in 0.031 seconds. A prior compiler process was killed by the
environment; that infrastructure failure is not counted as behavioral RED.

The correction additionally tests movement-side declarations and enumeration,
exact and one-under rule-step limits, every observed cancellation poll,
zero-scratch accounting, input nonmutation, and preservation of causal gaps.
Existing boundary accounting, chord prerequisites, independent middle proofs,
unrelated invalid pitch/color, and frozen-profile tests remain in the focused
acceptance set. No shared timeline bounds, repeat endpoints, or rule identities
are strengthened by this private placement proof.

The catalog now cites the direct MusicXML 4.0 barline reference in place of
issue 28 and describes the ordered-placement guard explicitly. That corrected
source metadata intentionally changes the two new semantic catalog identities
and their three package-composition identities. Legacy timeline/context profile
metadata and rule revisions are unchanged.
[`TestSemanticProfilesExactFinalIdentity`](../../../semantic_profile_identity_test.go)
pins all five final catalog and complete Profile JSON identities independently
of the earlier integrated acceptance receipt.

After the citation correction, the following focused command also passed on
2026-10-16 with Go 1.27.2 (root package 2.770 seconds, validation 0.042 seconds):

```
go test . ./validation -run '(^TestBarlineLocation|^TestSemanticCompositionPublicProfiles$|^TestSemanticCompositionEffectiveDTDBarline$|^TestSemanticPackageCompositionEntryPoints$|^TestSourceDeclarationCompositionPreservesProfiles$|Frozen)' -count=1
```

This covers semantic source/model and package entry points, effective-DTD
barline behavior, published-profile compatibility, and every frozen-profile
guard selected by `Frozen`. It is focused author verification, not a claim of
full-suite, race, hosted-CI, or independent-review acceptance.
