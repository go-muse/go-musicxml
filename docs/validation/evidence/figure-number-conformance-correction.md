# Figure-number conformance correction

## Authority and disposition

This correction addresses the original `CHECK-figure-number-numeric` and
`MX40-PROSE-figure-number-numeric` records. Their original IDs, source text and
historical planning/acceptance records are retained. The former selected rule
`MX40-CONTEXT-figure-number-numeric@1` is withdrawn from the built-in context
profiles as an invented additional lexical requirement, not marked implemented
as a total numeric grammar.

The MusicXML 4.0 [figure-number reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/figure-number/)
assigns string content through `style-text`. The [full issue 94 discussion](https://github.com/w3c-cg/musicxml/issues/94)
considered changing this to positive/nonnegative integers. After discussion of
historical notation, the [author's decision](https://github.com/w3c-cg/musicxml/issues/94#issuecomment-290221733)
explicitly retained unrestricted text. The following
[concrete proposal](https://github.com/w3c-cg/musicxml/issues/94#issuecomment-290224542)
retained both the `style-text` declaration and the descriptive numeric sentence.
[PR 202](https://github.com/w3c-cg/musicxml/pull/202), merged as
`c7cc6db0073b4638b2a294ef518abd91b14a3e7e`, left that declaration intact while
extending the surrounding figured-bass representation. The pinned MusicXML 4.0
schema still uses it.

Consequently, the built-in profiles cannot penalize otherwise valid style-text
for falling outside a guessed numeric notation subset. This does not establish
numeric meaning for every string. It removes the extra lexical obligation and
continues independent XSD declaration, attribute and content checks and exact
raw text projection. No new optional validation obligation or notation grammar
has been added.

## Legacy API compatibility

`validation.NewFigureNumberNumericPlan` remains exported and is deprecated.
Its original exact-declaration binding, nonempty ASCII-digit Pass and other-text
Unsupported behavior remain available only to explicitly constructed custom
catalogs. Its mandatory/normative binding contract is unchanged for compatibility;
selecting it imposes a caller-chosen overconstraint beyond MusicXML conformance.
The implementation comments and API documentation identify that limitation.
There is no new application-policy API or silent reinterpretation of its old
results. The historically named
`TestContextFigureNumberRetainsUnreviewedGrammarGate` now exercises an explicitly
constructed legacy catalog, not either built-in context profile.

## Deliberate profile migration

`ProfileSourceContext` and `ProfileModelContext` remove exactly the selected
figure-number rule and its support entry, update the closure wording, and change
their catalog digest from
`84279f72d5bda73adca5e340d52a9ba8532df43de036c910eae6b64ad1c47866` to
`c3e4d66f82ce776a6f8cfdd62295c124e352bc2ade1b0218ef70c8837c7f1506`.
The source/model required-rule counts change from 56/58 to 55/57. Counts are
selected operations, not MusicXML coverage denominators. The six earlier
XSD/local/timeline profile definitions, digests and captured reports remain
unchanged.

The source-composition guard's complete Profile JSON hashes also migrate:
- Source context: `f2547f7b087b4551e47f1a4d1f1b8799cf955ec3f2cb2f56da96257dae6e41fc`
  to `43a78f538defd0fbbc4e5331eecf41688204f12742a5900e49672128cd320155`.
- Model context: `37a63c9325973e67ae03328df45be262101fd83b216562641770e7ddf6ca93bb`
  to `ea6744501e0555255420471e8b8d5a0e4fb9371d915332777bfffdac77fd2511`.

Its historical map is retained. `migrateFigureNumberProfileHash` checks these
exact before/after pairs while the other fourteen profile hashes remain exact.

The historical file `testdata/validation/frozen-eight-profiles-batch8.json` is
unchanged, SHA-256
`cabd7e95094b7cf696fc6bc12816b5ed65ec3e7fdab58a854209599ef23260b1`.
`TestAuxiliaryFrozenEightProfiles` applies the explicit, bounded migration in
`migrateFrozenFigureNumberReport` to only its two context rows before comparing
all report fields and JSON shape. The retained Opus fixture loses precisely one
NotApplicable result (42 to 41), because the withdrawn rule selected no score
node there. Other counts, findings, contextual facts and report metadata compare
unchanged. The helper checks the old exact digest, rule revision and closure text,
and pins the corrected digest; it does not accept arbitrary catalog changes.

For actual figures, nondecimal text formerly produced Unsupported and made
otherwise valid context assessment incomplete. Such text now receives its
ordinary independent schema/context assessment without that penalty. Decimal
figures also lose the retired rule's Pass count; no fabricated Pass replaces a
removed obligation. Strict assessed decoding consequently accepts the valid
nondecimal fixtures. This is an intentional outcome change, not frozen-profile
or whole-MusicXML completion.

## Test-first evidence

Unpublished local base: `1535ec1`. Unpublished local RED commit: `23f0027`.
The new `TestFigureNumberUnrestrictedText*` tests failed on that base with the
retired Unsupported finding, selected/support rule entries and strict-decoding
incompleteness. The correction changes only catalog selection and its description;
the numeric-subset evaluator's behavior is unchanged.

Regression routes:

- `TestFigureNumberUnrestrictedTextConformance`: source/current-model ×
  partwise/timewise, decimal digits, leading zeros, signs, decimal-point text,
  fractions, Roman numerals, Arabic/fullwidth digits, arbitrary text, empty and
  whitespace content. Every case checks completed satisfied assessment, absence
  of the withdrawn requirement and exact retained contextual text.
- `TestFigureNumberUnrestrictedTextPreservesXSDConstraints`: an invalid style
  attribute still produces an independent XSD violation in all four adapters.
- `TestFigureNumberUnrestrictedTextPreservesSourceDeclaration`: a foreign-
  namespace figure-number remains invalid in both source score organizations.
- `TestFigureNumberUnrestrictedTextStrictDecode`: valid Roman-numeral figures
  remain available through strict source-assessed decoding in both organizations.
- `TestContextFigureNumberRetainsUnreviewedGrammarGate`,
  `TestContextFigureNumberRequiresExactDeclaration` and
  `TestLocalRound1FigureNumberSufficientNumericSubset`: deprecated explicit-plan
  compatibility, exact declaration scoping and historical positive subset.
- `TestAuxiliaryFrozenEightProfiles` and
  `TestIndependentContextFrozenProfileDigests`: exact bounded context migration
  and unchanged earlier profiles.

Final local execution results are recorded with the commit receipt. No remote
publication, remote CI, performance or race claim is made by this source decision.
