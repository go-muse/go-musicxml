# Local source residual decisions

Date: 2026-10-14. This addendum preserves the original obligation IDs and
planning records. It records the reviewed narrow M12/M14 implementations and
the M02/M14 source-domain dispositions; it does not turn unknown semantic facts
into passes or claim a published upstream erratum.

Authority: MusicXML 4.0, [pinned upstream revision
799e2defb2ece0ae7bafe08dcbcac25b2c631d53](https://github.com/w3c-cg/musicxml/tree/799e2defb2ece0ae7bafe08dcbcac25b2c631d53).
The checked-in `schema/musicxml-4.0/musicxml.xsd` SHA-256 is
`bfe37ed25a9ec00e6f2591d53df260b84efe12aed209ba3ac0a76f9287665a99`.
Later draft behavior is not substituted for this version.

## M12: numeral suppression advice

Original IDs: `CHECK-context-harmony-none-hide-root`,
`MX40-ISSUE-numeral-step-typo`.

The [none kind-value documentation](https://www.w3.org/2021/06/musicxml40/musicxml-reference/data-types/kind-value/)
recommends an empty text override on root-step or the nonexistent numeral-step.
The real [numeral-root](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/numeral-root/)
owns that override. The introduction of numeral-root in
[commit 0e62aa2, 2021-03-04](https://github.com/w3c-cg/musicxml/commit/0e62aa2b2b1ea5ff98aaea9e4948a5c05a0ac9ca)
and the subsequent extension of the root-only advice in
[commit 0b9e87e, 2021-04-30](https://github.com/w3c-cg/musicxml/commit/0b9e87ec71189d4a72be38b527861d45dd87e974)
establish the typo's editing path. The author's
[worked numeral-root example](https://github.com/w3c-cg/musicxml/issues/295#issuecomment-786331248)
confirms the concrete spelling.

Disposition: the advice resolves to `numeral-root/@text`, paired with its own
kind in each stacked harmony-chord. It is observable guidance, not a conformance
predicate or a test of whether the author followed the recommendation. Empty,
nonempty, and absent text therefore leave the same none-kind antecedent true.
The function branch retains its existing unavailable association; no function
text suppression semantics are added.

Verification:
- `TestLocalHarmonyNoneNumeralAdviceAndChordScope`: ten cases across source/model
  and partwise/timewise, including explicit empty/whitespace text, false kind
  conditions, stacked branch changes, and function controls.
- `TestLocalHarmonyNoneInvalidKindIsIndependentlyCausal`: invalid scalar kind
  does not contaminate the following explicit numeral advice, on all four paths.
- `TestLocalHarmonyNoneUnavailableDeclarationDoesNotReusePriorChord`: admitted
  absent/unknown/unsupported/invalid declaration gaps cannot preserve a prior
  root, even when parent enumeration is known complete. Causes survive; a later
  explicit root/numeral starts an independently known association.
- `TestLocalHarmonyNoneStackedParentCompletenessRemainsPrerequisite`: unavailable
  parent completeness keeps its own cause. Existing Round1 and Round3 causal
  controls remain intact.

## M14: direction-type formatting scope

Original IDs: `CHECK-context-direction-format-inheritance`,
`CHECK-context-direction-type-format-scope`.

[Direction](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/direction/)
describes visual sequencing and nonpositional formatting carry between child
components. [Direction-type](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/direction-type/)
limits print suggestions to "the individual &lt;direction-type&gt;, not to the
overall &lt;direction&gt;." The DTD repeats that scope. In upstream issue 163, the
[initial words/symbol/words example](https://github.com/w3c-cg/musicxml/issues/163#issuecomment-271031987)
places all three children inside one wrapper; the
[author's final clarification](https://github.com/w3c-cg/musicxml/issues/163#issuecomment-276446846)
retains inheritance from the immediately preceding component rather than a
previous component with the same tag.

Disposition: carry nonpositional fields within one wrapper, including mixed
children; start fresh at each new direction-type. Applicable independent
defaults then resolve normally. Per-field explicit overrides and coordinate
noninheritance remain unchanged.

`TestLayoutDirectionWrapperFormattingReset`,
`TestLayoutDirectionIntraWrapperComponentsStayIndependent`, and
`TestLayoutDirectionIntraWrapperInvalidCauseAndOverride` run source/model ×
partwise/timewise. They exercise defaults, application-dependent absence,
element-specific language/enclosure, mixed-child carry, four noninherited
coordinates, partial overrides, inherited invalid causes, independent invalid
defaults, and explicit known recovery.

The retained `TestLayoutMatrixInheritedTextDefaults/missing_fact` case now
expects `ltr` and `normal` after the wrapper reset. These are independently
specified by pinned XSD [text-direction, line 555](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L555)
and [letter-spacing, line 2037](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2037),
and retain the pre-existing default resolution branches. No case was removed.

## M02: deliberately unrestricted figure-number text

Original ID: `CHECK-figure-number-numeric`.

The [figure-number documentation](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/figure-number/)
describes numeric meaning but declares string content through style-text.
Upstream history specifically considered a positive/nonnegative integer
restriction in [issue 94](https://github.com/w3c-cg/musicxml/issues/94#issuecomment-284049700).
The author's final decision was to "keep the prefix, figure-number, and suffix
elements as unrestricted text."
[Decision](https://github.com/w3c-cg/musicxml/issues/94#issuecomment-290221733).
The [concrete proposal](https://github.com/w3c-cg/musicxml/issues/94#issuecomment-290224542)
retains both style-text and the descriptive numeric sentence;
[PR 202](https://github.com/w3c-cg/musicxml/pull/202) merged this design in 2017.

Disposition: a total numeric lexical grammar is not an omitted implementation
of an established MusicXML datatype. The accepted nonempty ASCII digit subset
remains sufficient positive evidence. Other style-text remains semantically
unassessed by this implementation; unrestricted text does not prove every string
numeric, and no new sign, fraction, Roman numeral, Unicode digit, whitespace, or
empty-string rejection is justified by an invented integer grammar.

No runtime policy changes accompany this disposition. Existing
`TestContextFigureNumberRetainsUnreviewedGrammarGate`,
`TestContextFigureNumberAbsentDoesNotCreateGrammarObligation`,
`TestContextFigureNumberRequiresExactDeclaration`, and
`TestLocalRound1FigureNumberSufficientNumericSubset` preserve the bounded result.
If the completion contract requires a total semantic answer for arbitrary text,
that requires an explicit additional notation/profile policy; this document
does not silently mark that promise implemented.

## M14: lyric language domain and font selectors

Original IDs: `CHECK-context-lyric-language-target`,
`CHECK-context-lyric-font-target`.

In the pinned XSD, all xml:lang declarations are:
- [text-formatting attribute group, line 2247](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2247),
  used only by accidental-text, formatted-text, and formatted-text-id, each
  simpleContent;
- [attributes/directive, line 2890](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2890),
  anonymous simpleContent;
- [text-element-data, line 5645](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L5645),
  simpleContent;
- [lyric-language, line 5948](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L5948),
  empty content.

There are no anyAttribute wildcards. None of these owners can legally contain
a lyric or its text. Thus a lyric's ancestor xml:lang competing with the score's
lyric-language default is unreachable in a schema-valid MusicXML 4.0 score.
[XML 1.0 Fifth Edition §2.12](https://www.w3.org/TR/2008/REC-xml-20081126/#sec-lang-tag)
defines language inheritance and clearing in XML generally; it does not add
undeclared MusicXML attributes. This domain proof replaces the claim of a
valid-tree ancestor/default precedence gate, not the runtime handling of invalid
or externally admitted evidence.

Direct `lyric/text/@xml:lang`, including empty clearing, remains representable.
The accepted exact name/number conjunction, wildcard omitted dimensions,
per-component publisher consensus, and local overrides remain unchanged.
[Lyric-font](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/lyric-font/)
and [lyric-language](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/lyric-language/)
do not prescribe a specificity or document-order tie-break for conflicting
matched publishers. That separate source-policy boundary remains explicit.

Existing `TestLayoutLyricPublishersFiveAxes`,
`TestLayoutMatrixLyricSelectorComponents`, and
`TestLayoutMatrixEmptyLanguageClears` retain the actual reachable cases.
This documentation does not alter lyric selector or language precedence code.
An independent `xmllint --nonet` check with the pinned local XML catalog accepts
direct text language and rejects attempted xml:lang on lyric, note, measure,
part, and score ancestors in both partwise and timewise fixtures.

## Unchanged source boundaries

M01 `CHECK-time-only-ascending` equality policy and M06
`CHECK-normal-type-when-different`, `CHECK-context-tuplet-normal-fallback`, and
`CHECK-context-normal-dots-reference` omitted-group/dot policy are unaffected.
No original obligation or accepted regression is removed by this addendum.
