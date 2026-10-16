# Document-only pitch validation scope

Date: 2026-10-14. This disposition preserves the original planning records and
their IDs; it does not remove encoded MusicXML pitch or transposition checks.

The functional completion scope is XML syntax, MusicXML/XSD source facts, and
semantic requirements that can be assessed from the encoded MusicXML. Comparing
that document with an independently desired score or exporter intention is a
separate application feature. No external pitch-intent provider API is included.

## Source requirements and observable facts

MusicXML 4.0 [pitch](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/pitch/)
encodes step, optional chromatic alteration, and octave. The
[tutorial](https://www.w3.org/2021/06/musicxml40/tutorial/hello-world/)
explains explicit alteration for non-natural pitches. An omitted alteration is
a valid natural encoding; a validator cannot infer that its author intended a
different note from a key signature or graphical accidental alone.

[Transpose](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/transpose/)
describes the interval from written to sounding pitch, including its staff scope.
The encoded interval, scalar domains, structural requirements and effective
state remain assessable. An instrument name alone does not establish a missing
interval or prove whether the exporter intended written or concert pitch.

The same encoded document can be consistent with its author's intention or
differ from an independently desired score. That unencoded distinction cannot
be resolved by a document-only conformance predicate. Missing external intent
must not be manufactured into either a violation or a verified match.

## Original-row disposition

- `CHECK-written-transposition` and `TEST-written-transposition` describe a
  conditional comparison whose inputs include authoritative exporter intent and
  an independently supplied interval. Their generated planning row does not
  establish a mandatory MusicXML API or an observable document-only antecedent.
- `CHECK-pitch-alter-explicit` and `TEST-pitch-alter-explicit` similarly describe
  an intended-pitch comparison. The original closure expressly calls for defining
  and authorizing an optional evidence contract and retaining document-only
  incompleteness. That optional contract is outside this functional completion.
  It does not add a selected requirement or make a document-only profile
  unsatisfied merely because external intent was not supplied.

The original rows, source references, test descriptions and historical statuses
remain unchanged. This additive disposition separates their external comparison
proposal from the actual encoded-value requirements. It is not a claim that the
optional comparison was implemented, nor a rejection of the MusicXML prose.

Existing source/model XSD checks, exact alteration values, effective transpose
state, chord/tie/repeat pitch evidence and their unavailable/invalid causes remain
in place. The optional experimental authority implementation was explicitly
reverted before publication, with its local history and test evidence preserved.
