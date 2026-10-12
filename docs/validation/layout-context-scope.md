# Bounded layout context

The layout provider projects exact source/current-model facts. It does not
engrave a score, quantize coordinates, choose application fonts, dereference
images, or follow linked scores. A finished provider is not a claim that every
optional rendering fact is known. Earlier profile behavior and original planning
statuses are unchanged.

## Scalar and formatting facts

`ContextResult.Value` carries the effective atomic field. `Layout.Raw`,
`Applicability`, `Origin`, `Scope`, `Unit`, and `Provenance` retain distinct
encoded/default/inherited evidence. These metadata also describe local scalar
projections; their use is not restricted to geometric fields. Derived exact
ratios use `Layout.Conversion`. Page/system references identify structural
occurrences, never printed labels. `Basis` records the documented coordinate
relation, not a computed unencoded position.

Formatting is component-local. Explicit child attributes win. Consecutive
children of a direction-type, and consecutive credit text/symbols, carry
nonpositional formatting. Cross-wrapper direction inheritance remains uncertain
for an absent component that might have been inherited. Default/relative
coordinates are not formatting inheritance. Harmony descendants inherit
applicable print-style components; per-minute font overrides metronome font by
component. Empty explicit xml:lang clears language. Conflicting genuinely
inherited XML language and lyric-language preserve the reviewed precedence gap.

Lyric selectors use exact normalized strings: `01` is different from `1`.
Omitted name/number dimensions are unrestricted; present dimensions are
conjoined. Per-component consensus requires complete eligible publishers.
Conflicts and unknown eligible selectors remain unavailable for that component;
there is no last-wins or most-specific-wins cascade. Local explicit formatting
still wins. Relevant contributing locations are retained.

## Layout lifetime and visibility

Print page, system, staff, and measure layouts expire at their corresponding
scope. An omitted field falls back to score defaults. Exact supports statements
can establish encoded break completeness. Without that proof, uncertainty is
local to potentially expired overrides and rendered memberships, rather than a
blanket loss of encoded distances/defaults.

The first visible part publishing system-layout is separate from the first
occupied staff layout slot. A hidden staff retaining print-spacing can remain a
predecessor slot; a proved cutaway with no spacing is excluded. Unknown visibility
or spacing retains uncertainty. Staff candidates are sparse observed boundaries,
not an array expanded to the encoded number of staves. Default staff-layout
number omission means staff 1 in every part; it does not make each part's first
staff the first staff of the system.

Page margins use physical-page parity; printed page-number is a label. Negative
system margins reduce the page inset. System divider positions retain their
current-to-previous-system boundary and exact half-distance relation. A proven
measure-width/system-width mismatch has application-dependent rendering; it is
not a new MusicXML conformance defect. A zero scaling denominator similarly
makes physical conversion unavailable without inventing an XSD failure.

Names in print take effect at the current or next new system. A missing future
boundary and unknown intervening break coverage remain distinct. The narrow Opus
provider reads only local score/new-page hints and never visits href targets.

## Verification boundaries

Concrete source and current-model fixtures cover both score hierarchies.
Malformed original lexical values are not claimed to survive model decoding.
Pure evidence tests exercise missing/invalid/cancellation paths independently of
source spelling. Real xmllint parity checks the same fixture bytes against the
pinned schema; it is an XSD oracle, not an oracle for prose rendering semantics.
See the per-check implementation map for exact functions, tests, remaining
application/interpretation dependencies, and ownership. Test/fact counts are not
conformance coverage denominators.

Font attribute ownership alone does not select the word-font domain. The
word-font fallback is limited to exact word-bearing declarations (words,
rehearsal, credit-words, display-text, footnote, part/group names and
abbreviations, and per-minute). Symbol/notation domains and font publishers
retain independent application choices when their applicable font is not proved.
A dynamics child's placement is ignored inside direction even when the
direction placement is omitted; that omission leaves the effective placement
application-dependent while preserving the child's encoded raw value.
