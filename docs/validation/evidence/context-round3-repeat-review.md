# Round 3 repeat coverage and encoded terminal extent

Baseline: `0821afd31cc0c783db53b351096ea9e76bd060dc`. This evidence addresses
[container coverage](https://github.com/go-muse/go-musicxml/pull/36#discussion_r4250901507)
and [unused origins / trailing forward](https://github.com/go-muse/go-musicxml/pull/36#discussion_r4250901511).
It clarifies the existing bounded beat-repeat interpretation; it does not change
the accepted timeline frame or endpoint policy.

## Eventless model container regression

The direct-model fixture is P1 with C in measure 1, measure-repeat start 1 plus
C in measure 2, one content choice containing both Note and Forward in measure
3, and stop in measure 4. Model traversal reports `model-choice-cardinality`
and emits no child for the invalid choice. Both hierarchy forms previously
published `NoteScopesComplete = Known(true)` for the two represented notes and
`ContentEquivalent = Unsupported(identity-scope-incomplete)`.

The coverage index now visits exact semantic root/part/measure containers as
well as represented events and note descendants. It borrows the accepted
selected timeline's exact hierarchy declarations and canonical part resolver;
no raw part ID or element local name establishes identity. Wrong declaration
owners, assemblies and structural parents cannot become containers. Unknown
container declarations retain conservative uncertainty. Known-part queries merge
only their part and unknown-membership gaps; unknown-part rows use a cached
all-parts aggregate because their public Notes roster includes every note.
Equal-strength causes keep source order. This aggregate requires no per-row
scan of all parts.

Both public evidence fields now retain Invalid with the original representation
rule/subject cause. The public consumer still reports its existing document
prerequisite incompleteness; this change neither bypasses scheduling nor
fabricates another normative failure. The two evidence causes are independent
caller-owned snapshots. A source document cannot represent the same invalid
in-memory choice, so that exact regression is model-only; reachable source
controls retain source/model and partwise/timewise coverage.

The unused origins map and its population loops are removed. The conservative
per-entry scratch allowance drops from 768 to 640 bytes; construction still
admits its maps before work and releases scratch on every failure.

## Source basis and bounded terminal-range argument

The pinned `schema/musicxml-4.0/musicxml.xsd` annotations provide the source:

- Beat-repeat, lines 2907–2911: repeated beats require actual repeated music in
  the file; a stop marks the first beat no longer displayed; a stop can be
  omitted for repetition through part end. See the [official
  beat-repeat reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/beat-repeat/).
- Duration, lines 6470–6474: intended score duration advances musical position
  for ordinary non-chord notes, forward and backup. See [duration](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/duration/).
- Forward, lines 4867–4869: the cursor movement coordinates voices and staves;
  avoiding measure/divisions boundaries is advisory. See [forward](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/forward/).
- Slash group, line 6306: slash-type/dots supply the beat for the display; absent
  explicit beat uses the existing bounded time-signature interpretation.

The bounded same-measure argument is about a complete encoded coordinate
sequence. Complete canonical part closure, complete note enumeration, available
unshifted origins/durations and the existing note-equivalence gates are required.
For a start in the final structural measure, maximum encoded non-backup extent
provides the terminal numeric comparison bound. Retaining the maximum allows
multiple voices: a later backup must not erase an earlier voice's extent. Source
and target beat windows use exact offsets in this same measure only. If a
non-rest source beat has encoded notes but a complete target beat has none, the
encoded repeat-content requirement fails. This does not establish acoustic
silence, endpoint containment or a meter-derived measure capacity.

The MusicXML text does not separately formalize part-end reconstruction for
boundary-crossing cursor data or infer the author's intended meaning of voice
padding. This is a documented bounded encoded-extent interpretation, not a
universal rule about such intent. Excluding forward would be asymmetric with
the source-defined cursor movement. Requiring containment of all terminal
endpoints would instead change the existing comparison contract for ordinary
notes as well. Neither policy change is made.

`partEnd` is private and consumed only by `contextRepeatBeatProof`. The latter
requires the identical structural measure and compares rational offsets; it
neither observes frame origins nor calls cross-frame position ordering. Adding
a positive duration now truthfully marks the synthesized endpoint
`UnprovedFrame=true` for notes and forwards alike. An independently observed
origin can prove that coordinate contained through the unchanged timeline API;
a synthetic endpoint does not create that witness.

## Exact fixtures and verification boundary

`TestRound3RepeatAbsentStopTrailingForwardExtent` uses divisions 1, optional
4/4, C, quarter beat-repeat start, C C C, then optional forward duration 2, all
in the final measure with no stop. All source/model × partwise/timewise cases
retain Known(true) without forward and Known(false) with it, with the exact
`MX40-CONTEXT-beat-repeat-content` owner on the missing-content failure.

`TestRound3RepeatTerminalRangeBoundaries` compares ordinary terminal notes,
trailing forward with and without an independent destination witness, explicit
stop, backup and a padded second voice, nonintegral extent, and an empty final
measure. `TestRound3RepeatTerminalExtentProjections` separately checks note
3+1=4 and forward 4+2=6 with/without frame witnesses, plus backup 4−4 whose
previous maximum remains 4. `TestRound3RepeatTerminalExtentUnavailable` keeps
cross-measure, incomplete, empty, unknown and Invalid extent cases unavailable
with causal fidelity.

`TestRound3RepeatEverySemanticContainer`,
`TestRound3RepeatKnownPartEventlessContainerIsolation`,
`TestRound3RepeatContainerExactBindings`,
`TestRound3RepeatUnknownRosterCoverageAndCausalOrder`,
`TestRound3RepeatContainerResources` and
`TestRound3RepeatContainerAndAllScopeLinearWork` cover exact selection,
source-order causes, document/part isolation, exact and one-under work/bytes,
every cancellation poll, atomic index publication, scratch release and linear
100/1000-container construction with constant-work aggregate lookup. These
focused fixtures are not unrestricted throughput evidence or a full-suite pass.

The exact container-only resource fixture costs 388 steps (partwise) / 419
(timewise) and 4480 bytes, with all 29/30 cancellation polls tested. The
100/1000-eventless-container fixtures cost 10864/108064 steps and
128640/1280640 bytes. Known canonical P1/P2 scope isolation is separately
exercised with a finalized authoritative synthetic identity table and an
injected provider enumeration gap: repeated P2 containers share one coverage
entry, P1 remains independent, and the unknown-row aggregate retains the gap.
This lower-level fixture does not claim that a malformed public model can
preserve canonical identity completeness.

Focused verification uses the configured repository toolchain and serialized
package execution:

- `go test -p 1 ./validation -run 'Repeat|EmptyOnset|TimelineCrossFrame|TimelineDisplaced|TimelineFinalReview.*(Frame|Cursor|Divisions)' -count=1`
- `go test -p 1 -race ./validation -run '^TestRound3Repeat|^TestRound1RepeatProofIndexesCancellationAndGauge$|^TestIndependentEmptyOnsetProviderAdmissionAndAllPolls$' -count=1`

Both passed. No full suite or GitHub mutation was performed for this isolated
fix. Integrated acceptance and exact-head CI remain separate gates.
