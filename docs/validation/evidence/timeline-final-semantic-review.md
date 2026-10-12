# Independent semantic acceptance: batch 6 code checkpoint

Date: 2026-10-12. Production commit tested:
`22ae68a731ddc26e15f988fb59232e2dd9d572e6`, tree
`440f9d3cabffd535efb127bbbaeecb22e3185a27`.

The independent acceptance checkout is a preserved worktree created from the
production branch, not an accumulation of selectively copied production patches.
The older `timeline-semantic-qa.md` remains evidence for its explicitly named
`5ba0bd1` baseline; it is not promoted to this acceptance.

## Result

Accepted for the named code checkpoint. All of the following passed from the
preserved checkout, with Go 1.27.2, real libxml2/xmllint 2.9.14, and the repository
flag `MUSICXML_REQUIRE_XMLLINT=1` supplied by `go-musicxml-env.sh`:

- `go test ./... -count=1` (full suite, no exclusions).
- `go test -race ./... -run 'Timeline|Grace|FiguredBass|Direction' -count=1`
  (focused race suite, no excluded matching test).
- `go vet ./...`.

Execution logs are retained in `/workspace/shared/batch6-timeline-evidence/`:
`final-semantic-acceptance-22ae68a-full.log` and
`final-semantic-acceptance-22ae68a-race-vet.log`. Earlier component checkpoints
remain separately named and are not substituted for this exact-head execution.
Any subsequent production-code change requires renewed acceptance; this report
adds documentation only.

No unresolved semantic counterexample remains from this review's regression
matrix at the named code checkpoint. This is acceptance of the documented
bounded provider and its consumers, not a claim that arbitrary playback,
repeat traversal, measure extent, or every MusicXML clause is reconstructed.
The 30 top-level `TestTimelineFinalReview` functions are regression families,
not a clause-coverage denominator.

## Independent defects reproduced and checked after correction

- Conditional sound instrument/MIDI/device statements retain their raw values
  and guards. Only stated properties become conditional; omissions preserve
  earlier properties. Conditional statements cannot repair an earlier invalid
  same-property gap or resurrect one already repaired unconditionally.
- Positioned invalid target and hidden-publisher gaps start at their actual
  point. Equal positions do not manufacture a winner or a strict repair.
  Unavailable displaced positions retain independent bounds and actual causes.
- Figured-bass duration uses the finalized documented following-note anchor's
  units. Cursor units remain provenance; derived chain sums do not create an
  extra unit origin. A later unrelated figure cannot erase an earlier chain.
- Grace donors use proved score order rather than global XML monotonicity.
  Later irrelevant voice gaps remain local. Cross-frame direction, figure and
  grace candidates use the same partial-order proof, including unavailable
  displaced candidates and chord donor barriers.
- Signed local coordinates are distinct from cross-frame chronology. Two
  independently contained frames retain canonical score precedence. Displaced
  candidates compete unless independently excluded or strictly superseded;
  default/initial values participate in consensus and actual causes survive.
- Cursor-unit reconstruction does not invent departure or piecewise units
  across possible differing regions. Stopped stages retain raw publishers,
  independent origins and scoped directional bounds. Descendant witnesses
  cannot certify their own ancestor excursion. Complete-history normalization
  invalidates earlier notes reached by later displaced divisions, and an
  independently restored safe excursion can restore affected normalization.
- Valid unplaced key updates preserve established encoded-key presence without
  choosing a concrete effective key. Equal unplaced divisions preserve an
  independently established scalar baseline. Neither certificate establishes
  a missing baseline or excuses invalid/hidden publishers. Public queries
  retain owned witnesses and honest inheritance uncertainty.

The final matrix includes positive controls for exact uniform units, contained
returns, independently repaired prior regions, excluded future invalid units,
separate properties/parts, documented defaults and local numeral-key proof.
It includes actual original-source observation and decoded current-model
assessment for the mandatory numeral consumer in both partwise and timewise
forms. These integer fixtures make no arbitrary source-decimal/model-binary
identity claim. Real pinned-schema xmllint controls validate the relevant
counterexample encodings.

## Source and proof boundary

The pinned MusicXML 4.0 annotations remain authoritative. The
[attributes clause](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/attributes/)
governs score-order updates; the
[duration clause](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/duration/)
and [divisions clause](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/divisions/)
govern exact normalization. The
[offset clause](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/offset/)
and [sound clause](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/sound/)
keep placement and playback dependencies separate. The
[figured-bass clause](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/figured-bass/),
[grace clause](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/grace/)
and [numeral clause](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/numeral/)
supply the corresponding association and consumer requirements.

`timeline-cross-frame-review.md` records the detailed derivation and rejected
alternatives. No meter-fill rule, maximum note endpoint, terminal cursor, or
importer convention is used as an invented measure length. Backup/forward
recommendations remain advice, not newly enforced mandatory failures. Provider
completion does not turn unavailable optional facts into known facts.
