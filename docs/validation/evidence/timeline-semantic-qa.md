# Batch 6 semantic integration QA

Date: 2026-10-12. Independently tested production baseline:
`5ba0bd1` (`docs: retain full timeline scope and original provenance`).
The changes associated with this report add tests and evidence only.

## Result

No new production defect was established by this semantic integration pass.
The earlier sound-placement defects below were independently reproduced and
remain blockers on this original baseline until the separate fix is integrated.
The 16 new top-level tests completed 139 reported test/subtest results, with no
failures or skips. This count includes parent subtests; it is not a clause,
leaf-case, or full MusicXML coverage denominator.

The tests do not close the full batch-6 ownership ledger. In particular, the
sound-offset integration and additional typed-state/default/provenance work
were being handled separately, and this baseline is not their integrated head.

## What was exercised

Both actual original-source observation and direct current-model assessment:

- Partwise/timewise hierarchy, legal repeated timewise part IDREFs, repeated
  nonnumeric measure labels, canonical owner identity and cross-part key
  isolation. Invalid declared formatting on a score-part child does not erase
  the independently established owner.
- A key that occurs later in XML but earlier in score order, and the converse;
  late same-position publication; invisible, zero-fifths, empty nontraditional,
  and explicit nontraditional keys.
- Staff-local key isolation, unresolved omitted multistaff assignment, and a
  newer all-staves key superseding an older invalid staff-local publication.
  The historical scalar failure remains independently reported.
- Principal 4 with tones 1 and 3, tone 5 failure, cross-staff/mixed-voice chords,
  nearest principal across differing voices, cue/ordinary mixed branches,
  grace groups, timed tone after grace, orphan and interrupted anchors.
- Same-region changed-divisions rewind. The hand-computed sequence is
  D1; N4; D2; N4; B2; N2, with positions 0, 4, 4, 6, 5. No disputed
  boundary-crossing normalization is required by this positive case.
- Incomplete root, part, or measure enumeration in both hierarchy forms.
  Closed absence can fail, while incomplete enumeration cannot fabricate it;
  an own local numeral-key remains independent.
- Invalid effective key, own numeral-key, or duration retaining the exact XSD
  cause instead of acquiring a duplicate invented prose failure.
- Local numeral-key proof despite an unsupported required provider. The
  unavailable fallback propagates the provider's unsupported reason.
- Genuine work-account exhaustion immediately before finalization. Identical
  controls differing only in consumer selection establish one retained pass
  for the valid local key and none for an unresolved fallback. This proves
  survival across a finalization resource interruption; it does not pretend
  to isolate a particular internal provider instruction as the stopping site.
- Provider completion with unavailable optional musical facts, consistent with
  the explicit profile contract. Provider Pass certifies finalization, not that
  all positions or all effective-state queries are known.
- A proven prefix chord violation surviving a later narrow divisions gap,
  alongside an independent successful local numeral-key.

Runtime-level tests additionally check exact 1/3 normalization, stable original
principal duration, absence of invented voice 1, unobserved intervening-note
anchor loss, the newer all-staves tonal value, distinct measure occurrences,
score-order lookup after backup, and high-water state carry rather than the
final lexical cursor.

Seven same-byte `xmllint` controls verify the schema validity assumptions for
repeated labels/IDREFs, mixed cue chords, the distinction between semantic
requirements and XSD, and the deliberately invalid cancel-location field.
`xmllint` is only the XSD oracle, not a timeline semantics oracle.

## Known defects independently rechecked before integration

The earlier read-only runtime-review overlay still fails exactly these two
sound cases on this baseline. These are known in-progress fixes owned by the
sound-offset work, not new findings or reasons to broaden the divisions gate.

1. Standalone sound placement: divisions 2, a note with duration 4, followed by
   sound/play with local offset 2. Its cursor is 2 quarters and the sound origin
   should be 3. The baseline publishes 2, silently ignoring the local offset.
2. Direction-nested sound: a direction containing sound/play should publish one
   positioned playback record. The baseline publishes zero records.

The retained incomplete-measure, safe-region rewind, and proven-prefix cases in
that same overlay pass. The fresh failing output was retained separately before
lead integration; no production code was changed to obtain these results.

## Executed checks

Environment was loaded from the shared Go toolchain setup. It supplied
Go 1.27.2 and actual libxml2/xmllint 2.9.14, with
`MUSICXML_REQUIRE_XMLLINT=1`.

- Baseline `go test -count=1 ./...`: passed.
- After tests, `go test -count=1 ./...`: passed, including real external XSD
  tests and the new seven-case same-byte fixture controls.
- `go test -json -count=1 -run '^TestTimelineSemanticQA' ./...`: passed;
  16 top-level tests, 139 pass events, no skips or failures.
- `go test -race -count=1 -run '^TestTimelineSemanticQA' ./...`: passed.
- `go vet ./...`: passed.
- `GOARCH=386 go test -count=1 -run '^TestTimelineSemanticQA' ./...`: compiled
  but could not execute the 32-bit binaries (`exec format error`). The current
  x86_64 environment has no working 32-bit execution route or qemu-i386.
  This is an unexecuted architecture check, not a passing runtime check.

## Evidence boundaries

Earlier draft test attempts exposed harness/fixture mistakes, not production
regressions: the source observer requires document ID `source`; key `mode` is a
free string and therefore cannot be used as an invalid enumeration control; an
unsupported dependency preserves an unsupported reason rather than inventing
an invalid-prerequisite cause. Corrected tests use the actual declared domains.
Those transient red logs must not be presented as bugs fixed by production code.

No importer convention was promoted to normative semantics, no should-level
boundary advice became a hard error, and no blanket divisions unsupported
policy was introduced. The existing research gate remains untouched. The tests
were not used to update planning statuses or claim full-scope completion.
