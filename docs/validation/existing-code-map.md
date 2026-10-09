# Existing code and proposed validation changes

Status: **PROPOSED**. This mapping is based on repository commit
`e486735cd6e4537e839ff67704b7768a9df3bdb3`, the unchanged product-code base of
this documentation proposal. It identifies reuse candidates and observed limits;
it is not an implementation plan with a fixed API or an exhaustive validator audit.

## XML-reading repair update

The duplicate-attribute and DOCTYPE-placement gaps recorded below are now
repaired in the existing decoding and internal XML-parsing paths by
[`xml_wellformedness.go`](../../xml_wellformedness.go), with
[regressions](../../xml_wellformedness_test.go) covering skipped subtrees and
MXL paths. This is the existing-reader slice of STAGE-DEF-01. Integration with
the proposed strict source adapter remains open; the pinned historical review
and broader deferred contracts below are not a claim of completed validation.

## XML-reading follow-ups

The XML-declaration-ordering follow-up from [PR #14 review](https://github.com/go-muse/go-musicxml/pull/14#pullrequestreview-5459364076)
is now repaired in the existing readers by
[`xml_wellformedness.go`](../../xml_wellformedness.go). A declaration must be
first after any encoding signature; whitespace, comments, processing
instructions, DOCTYPE and root content may not precede it. Case variants of the
exact reserved PI target `xml` are rejected, while `xml-stylesheet` and other
ordinary targets remain accepted. These rules follow XML 1.0
[productions 22-23](https://www.w3.org/TR/REC-xml/#sec-prolog-dtd) and
[production 17](https://www.w3.org/TR/REC-xml/#sec-pi).

Executable evidence is in [`xml_declaration_test.go`](../../xml_declaration_test.go)
(`TestXMLDeclarationDecodePaths`, `TestXMLDeclarationContainer`,
`TestXMLDeclarationValidationParser`, `TestXMLDeclarationLinkedResources`,
`TestXMLDeclarationReservedTargets`) and the declaration cases in
[`xml_wellformedness_errors_test.go`](../../xml_wellformedness_errors_test.go).
Public decode paths retain supported encodings and cover skipped subtrees,
MXL root/container documents, deferred linked-document parsing and error
wrapping/positions. The internal Encode/reparse validation helper retains its
existing plain UTF-8 input contract. Full declaration pseudo-attribute grammar,
DTD syntax, broader XML/namespace conformance and strict-source integration
remain outside this repair.

The **non-XML whitespace outside the root** follow-up from
[PR #22 review](https://github.com/go-muse/go-musicxml/pull/22#discussion_r4229843345)
is now repaired at the four character-data checks in `readRoot`,
`readDocumentTail`, `parseValidationDocument` and `readValidationTail` in
[`decode.go`](../../decode.go) and [`validation.go`](../../validation.go).
They trim only XML `S` (space, tab, carriage return and line feed), rather than
Unicode whitespace, preserving the existing before/after-root errors. Their
`xmlWhitespace` cutset in [`xml_whitespace.go`](../../xml_whitespace.go) is also the shared
source for `parseXMLUnsignedInteger` and `isValidationWhitespace`; the latter
continues to govern XSD normalization and element-content checks.
[XML 1.0 document, S, prolog and Misc productions 1, 3, 22 and 27](https://www.w3.org/TR/REC-xml/#sec-prolog-dtd)
require rejection of literal non-XML whitespace such as NBSP and NEL at those
boundaries. This mandatory XML-reading repair is separate from the element-only
XSD content repair below and does not add XSD assessment to Decode.

Executable evidence is in
[`xml_document_whitespace_test.go`](../../xml_document_whitespace_test.go):
`TestXMLDocumentWhitespaceDecodePaths`, `TestXMLDocumentWhitespaceContainer`,
`TestXMLDocumentWhitespaceValidationParser` and
`TestXMLDocumentWhitespaceLinkedResources` extend the shared
[XML-reading matrix](../../xml_reading_matrix_test.go) across all three document
roots, options, MXL root/container documents and deferred linked resources.
The literal-character cases cover all four XML `S` characters and all 19
XML-valid non-`S` Unicode whitespace characters before the root, after the root
and after a declaration. The shared `xmlDeclarationEncodingVariants` helper
uses genuine UTF-8/UTF-16 bytes and representable Latin-1; non-ASCII Latin-1
requires a first declaration. ASCII-only fixtures retain Latin-1 variants even
without a first declaration, and omitted declarations stay omitted.
`TestXMLDeclarationEncodingVariants` independently checks fixture bytes and
representability; `TestIsValidationWhitespace` checks the shared XML `S` set.
`TestXMLDocumentWhitespaceErrors` retains ordinary-character errors
and empty/XML-`S`-only input behavior. `TestXMLDocumentWhitespacePreservesText`
checks exact Unicode text and model round trips, while legal comments/PIs and
declaration ordering retain their existing matrix coverage.
`TestXMLDocumentWhitespaceAgainstXMLLint` compares the same raw source bytes
with `xmllint --nonet --noout`, required in Linux CI, without an XSD or DTD
validity check. The internal validation parser retains its plain UTF-8 contract.

This remains a bounded existing-reader slice of `STAGE-DEF-01`. The later
[document-boundary lexical repair](#document-boundary-lexical-repair-update)
closes the separate reference/CDATA gap that a decoded-character predicate
cannot distinguish. Full declaration/DTD/namespace conformance and strict-source
integration remain open; planning JSON completion semantics are unchanged.

**Typed numeric source whitespace remains a separate deferred follow-up**, as
recorded in [PR #23 review](https://github.com/go-muse/go-musicxml/pull/23#discussion_r4230227524).
`xmlDecimal.UnmarshalText` and `parseXMLUnsignedText` in
[`xml_numeric.go`](../../xml_numeric.go) still use `strings.TrimSpace`, and
ordinary integer decoding in `encoding/xml` likewise trims Unicode whitespace.
A Go 1.27.2 probe confirms that NBSP around `divisions`, NEL around `octave`,
and NBSP, NEL or EM SPACE padding in `staves` can be lost during typed Decode.
These characters are legal XML element text but are not XML `S` in XSD numeric
lexical forms. The original-source validator and `xmllint --nonet --schema` reject those
numeric values, while Decode/Encode canonicalizes them and public `Validate`
accepts the resulting model because it cannot recover the original text.
This document-boundary repair does not change numeric transport or freeze that
leniency as normative acceptance tests. A future transport-compatibility decision
and strict-source lexical validation remain separate work.

The following review follow-up is still explicitly **deferred**:

- **Machine-readable implementation evidence:** add a reviewed `implemented_by`
  or `evidence` contract to the planning format and its integrity checker.
  Link partial requirement/test implementations to PRs, commits, source files
  and test names; check that referenced local artifacts exist. Retain the
  meaning of `status: planned` and keep incomplete strict-source obligations
  visible rather than marking the entire contract implemented.

Until that evidence contract is designed, the existing-reader evidence for
`REQ-DEF-XML-ATTR`, `REQ-DEF-XML-DOCTYPE`, `REQ-DEF-XML-SKIP` and their matching
`TEST-DEF-XML-*` contracts is [PR #14](https://github.com/go-muse/go-musicxml/pull/14),
[`xml_wellformedness_test.go`](../../xml_wellformedness_test.go)
(`TestXMLWellFormednessDecodePaths`, `TestXMLWellFormednessContainer`,
`TestXMLWellFormednessValidationParser`, `TestXMLWellFormednessLinkedResources`,
`TestXMLWellFormednessDepthBoundary`, `TestXMLDoctypeDoesNotFetchExternalDTD`),
and [`xml_wellformedness_errors_test.go`](../../xml_wellformedness_errors_test.go)
(`TestXMLWellFormednessErrorPositions`). These are executable evidence for the
current-reader slice, not a change to the planning JSON's completion semantics.

## Document-boundary lexical repair update

The existing readers now reject references and CDATA sections outside the root,
including references to XML whitespace and empty CDATA. XML 1.0
[document, prolog and Misc productions 1, 22 and 27](https://www.w3.org/TR/REC-xml/#sec-prolog-dtd)
permit literal XML `S`, comments and processing instructions at these boundaries,
with an XML declaration and DOCTYPE only in their permitted prolog positions.
Decoded whitespace is insufficient evidence: references and CDATA are different
lexical constructs. Both remain supported inside the root.

[`xml_lexical.go`](../../xml_lexical.go) observes the innermost decoder's byte
stream after encoding conversion. It retains only the latest `<` and `&` offsets
and tests them against each `CharData` token's half-open `InputOffset` span.
The decoder's lookahead `<` is excluded from the preceding literal text, while
CDATA's own markup remains within its token even when the section is empty.
An `io.ByteReader` prevents decoder buffering above the observer; buffering below
it remains bounded. Latin-1 conversion installs a fresh observer at the current
decoder offset, so upstream read-ahead and source-byte lengths cannot corrupt
post-conversion offsets. No document or token copy, second parse, entity expansion
or resource fetching is added.

[`wellFormedXMLTokenReader`](../../xml_wellformedness.go) applies this origin check
only outside its existing element scopes, below namespace/model filtering. New
failures wrap `*xml.SyntaxError` and report the innermost decoder's detection line
at the end of the offending token. Literal-character boundary errors retain
their existing messages; namespace/depth checks and deferred MXL parsing retain
their existing contracts. The internal validation parser still takes plain UTF-8;
public readers retain their existing UTF-8, UTF-16 and Latin-1 support.

Executable evidence is in
[`xml_document_lexical_test.go`](../../xml_document_lexical_test.go): the
`TestXMLDocumentLexicalDecodePaths`, `Container`, `ValidationParser` and
`LinkedResources` tests reuse the shared XML-reading matrix. Negative cases cover
all four XML `S` characters as decimal and hexadecimal references, predefined
entities, empty/whitespace/text CDATA, declaration/DOCTYPE/tail boundaries and
self-closing roots. Controls preserve literal whitespace, markup-looking text in
comments/PIs/DTD literals, attribute references, and legal known/unknown/foreign
content. `TestXMLDocumentLexicalErrors` checks syntax-error lines and no-root
inputs; `PreservesText` checks exact text and model round trips; `Streaming` and
`BoundedReadAhead` cover buffer boundaries, charset offset divergence, supplementary
characters, one-byte/data-plus-EOF readers and incremental root delivery.

`TestXMLDocumentLexicalAgainstXMLLint` compares identical original bytes through
the shared `xmllint --nonet --noout` driver, required in Linux CI. This is XML
well-formedness evidence, not XSD or DTD validity and not Decode/Encode parity.

This is another bounded current-reader prerequisite under `STAGE-DEF-01` and
`DEF01-03`/`DEF01-04`; it does not complete the stage or a strict-source adapter.
Production schemas, generated models/metadata and public API signatures are
unchanged. Numeric transport whitespace, full declaration/DTD/namespace grammar,
new validation profiles and machine-readable implementation evidence remain
separate work. Planning JSON retains `status: planned`.

## Exact integer value-space repair update

The existing validator now separates all 13 XSD integer-family domains from Go
conversion. The five unbounded families accept arbitrary finite ASCII-digit
lexemes; the eight bounded families retain their normative XSD ranges. The
[integer helper](../../validation_integer.go) uses immutable sign/digit views
with leading zeros removed for comparison and a single zero value. Parsing and
comparison scan linearly with constant auxiliary space; existing normalization,
source storage and diagnostics still allocate in proportion to their input.
There is no big-number arithmetic, exponent expansion or newly chosen numeric
limit. Signed unsigned-type spellings preserve existing library compatibility.

Genuinely integer-derived restrictions now use exact inclusive/exclusive bounds,
value-space enumeration and significant-digit counts. Integer fixed values use
exact equality on attributes, simple elements and resolved complex simple-content
elements. Lexical patterns still receive the original XSD-normalized spelling;
source nodes and generated schema literals remain unchanged. Integer identity is
selected from builtin ancestry, never from an integral-looking decimal value.
The normative basis is XSD 1.0 [integer and derived domains][integer-domain],
[value facets][integer-facets] and [element fixed constraints][integer-fixed].

[`validation_integer_test.go`](../../validation_integer_test.go) covers machine
endpoints and their neighbors, signs/zero/leading zeros, ASCII digits, XML-only
whitespace, 2^53 neighbors, positive and negative values beyond floating-point
range, every bound kind, layered restrictions, enumeration/fixed/default values,
patterns, nil/child interactions, lists and union member acceptance. Generated
metadata comes from [the synthetic fixture](../../testdata/validation/integer-contract.xsd).
Independent `math/big` checks cover exact ordering, including 4,096-digit values;
allocation checks cover the standalone digit-view operations.

`TestIntegerValueSpacesAgainstSchema` sends identical original source bytes to
the internal validator and `xmllint --nonet --schema`; Linux CI requires it.
Its bounded differential subset includes `staves=18446744073709551616` against
the pinned MusicXML schema. libxml2 2.9.14 has observed precision limits beyond
24 significant digits and disagrees on certain signed/whitespace spellings and
numeric element-fixed equality. Those cases remain explicit internal
normative/compatibility tests and are excluded from that oracle subset, not
changed to match it. Full large-value support is not inferred from the oracle.

`TestIntegerSourceAndModelConversion` separately proves raw-source acceptance of
`staves=2^64` and unchanged typed Decode range failure, including its underlying
`strconv.ErrRange` and absence of a `ValidationError`. Public `Validate` still
uses Encode/reparse and cannot recover discarded source information; no new
source-assessment or conversion-report API is introduced.

This is a bounded existing-validator slice of `STAGE-DEF-03` (`DEF03-01/04` and
integer comparison portions of `DEF03-02`), not completion of that stage.
Decimal comparisons and digit facets are covered by the later repairs below;
typed union/list aggregate equality, mixed-base simple-content restrictions,
wider facet semantics, contextual arithmetic,
session parse-once caching, shared budgets and incomplete/conversion reports
remain open. No new profile, adapter, public API or resource policy is selected;
planning JSON retains `status: planned`.

[integer-domain]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#integer
[integer-facets]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#rf-totalDigits
[integer-fixed]: https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt

## Exact atomic decimal comparison repair update

The existing validator now compares atomic decimal-derived values exactly for
all four inclusive/exclusive bound facets, enumeration, and fixed constraints
on attributes, simple elements and resolved complex simple-content elements.
The [decimal helper](../../validation_decimal.go) uses immutable sign, whole-digit
and fractional-digit views. Leading whole zeros and trailing fractional zeros
are omitted from the views, and every signed zero compares equally. Lexical
acceptance remains the existing XSD decimal contract. Comparisons do not convert
to `float64`, expand exponents, allocate scaled integers or perform arithmetic;
they scan linearly with constant auxiliary space. Existing source storage,
normalization and diagnostics still allocate proportionally to their input.
No numeric limit or shared-session budget policy is selected here.

Only builtin ancestry `decimal` selects the new comparison path. Integer,
float/double, string and aggregate union/list equality retain their previous
paths; decimal list items and union members benefit from their own atomic bound
checks without establishing aggregate typed equality. Patterns still see the
unchanged normalized source spelling. Fixed-value comparison reuses effective
simple-content ancestry resolution; generated schema literals and source text
are never rewritten. The basis is XSD 1.0 [decimal value identity][decimal-domain],
[bound and enumeration facets][decimal-facets] and
[typed element fixed constraints][integer-fixed].

[`validation_decimal_test.go`](../../validation_decimal_test.go) contains
replayable original-source regressions independent of the new helper: close
fractional neighbors, alternate equal spellings, signed zeros, all bound kinds,
layered restrictions, fixed/default values, patterns, nil/child interactions,
attribute parity and decimal members of lists/unions. Arbitrary-width positive
and negative bound cases include 4,096-digit whole/fractional parts. Generated
metadata comes from [the synthetic fixture](../../testdata/validation/decimal-contract.xsd).
[`validation_decimal_operations_test.go`](../../validation_decimal_operations_test.go)
compares against independent `math/big.Rat` values, checks lexical compatibility,
invalid bound metadata, dispatch scope, immutable source/schema spellings and
constant-space helper behavior on a one-MiB input.

`TestDecimalComparisonsAgainstSchema` sends identical source bytes to the
internal validator and `xmllint --nonet --schema`, including actual MusicXML
`positive-divisions` values; the Linux CI selector requires it. The external
subset excludes values outside libxml2 2.9.x's supported decimal precision and
its lexical comparison of explicit decimal element-fixed values. Those cases
remain internal normative tests, not evidence that the oracle supports the
entire decimal domain. `TestDecimalSourceAndModelConversion` separately proves
raw acceptance of huge/tiny positive divisions and the unchanged typed Decode
range error for a huge finite value. Public `Validate` still uses Encode/reparse
and cannot recover source precision discarded by model conversion.

This is another bounded existing-validator portion of `STAGE-DEF-03`
(`REQ-DEF-NUM-EXACT`, `DEF03-02/04`). Decimal digit facets are covered by the
later repair below. Typed union/list aggregate equality, mixed-base simple-content
restrictions and wider facet semantics,
contextual arithmetic, parse-once session caching, resource budgets,
incomplete/conversion reports and strict-source/direct-model adapters remain
separate work. No public API, model-decimal interpretation, profile or planning
completion semantics changes; all relevant planning statuses remain `planned`.

[decimal-domain]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#decimal
[decimal-facets]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#rf-maxInclusive

## Exact decimal digit-facet repair update

The existing validator now evaluates atomic decimal `totalDigits` and
`fractionDigits` from the immutable decimal value views, rather than counting
source spelling. Leading whole zeros and trailing fractional zeros do not alter
the result. Leading fractional zeros and trailing whole zeros remain relevant:
`.001` requires `totalDigits >= 3`, and `1000` requires `totalDigits >= 4`.
Every signed zero has scale zero and a total count of one.

This follows XSD 1.0 Second Edition [totalDigits][decimal-total-digits] and
[fractionDigits][decimal-fraction-digits]: `totalDigits` restricts both the
coefficient magnitude and scale, while `fractionDigits` bounds scale alone.
The existing normalized views permit constant-space, linear-time counting without
float conversion, arithmetic, exponent expansion or a new numeric limit.
Patterns still see the original normalized spelling; source values and generated
schema literals are not rewritten. Existing issue paths, constraint names and
failure order are preserved, with digit-count messages now reporting value counts.
Integer behavior and nondecimal fallback remain on their existing paths.

[`validation_decimal_digits_test.go`](../../validation_decimal_digits_test.go)
provides raw-source positive, negative and boundary regressions with
[generated synthetic metadata](../../testdata/validation/decimal-contract.xsd).
It covers named/inline/layered restrictions, attributes and effective complex
simple-content extensions, defaults/fixed values, nil and child interactions,
lexical patterns, and decimal list items/union members. The same source bytes run
against `xmllint --nonet --schema` in `TestDecimalDigitFacetsAgainstSchema`,
required by Linux CI. Its ordinary-width subset excludes unsupported huge values
and libxml2's lexical element-fixed equality limitation. Separate internal tests
use a reduced-rational arithmetic oracle, 4,096-digit boundary values, immutable
source/schema checks, exact diagnostics and a one-MiB zero-allocation success case.

The pinned MusicXML schemas declare neither facet; this is a verified reusable
scalar prerequisite under `STAGE-DEF-03` / `REQ-DEF-NUM-EXACT`, not a new MusicXML
profile or completion of that stage. The later effective-whitespace and
[pattern-group repair](#same-level-pattern-alternatives-repair-update) close those
bounded scalar gaps. Mixed-base simple-content restrictions, broader regex
semantics, schema-component validity/metadata-width limits, typed aggregate
equality, contextual arithmetic, transport and current-model interpretation,
session caching/budgets, incomplete/conversion reports and the proposed adapters
remain separate work. Planning statuses remain `planned`.

[decimal-total-digits]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#rf-totalDigits
[decimal-fraction-digits]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#rf-fractionDigits

## Complex simple-content restriction metadata repair update

The existing generator and validator now retain and apply local scalar
restriction metadata declared inside complex `simpleContent` restrictions.
The parent complex type remains distinct from the optional inline `simpleType`:
without that inline type, local facets restrict the parent's effective scalar;
with it, they restrict the inline scalar. These are the simple-content-parent
cases of XSD 1.0 [Structures section 3.4.2][simple-content-mapping]. Named and
anonymous restrictions, further restrictions and extensions retain their scalar
layers rather than flattening or replacing inherited facets.

[`complex.go`](../../internal/xsdgen/complex.go) resolves the optional inline
scalar. [`generate_validation.go`](../../internal/xsdgen/generate_validation.go)
shares lexical enumeration/pattern/facet rendering and does not impose Go
constant naming on the outer restriction's enumeration values.
[`effectiveComplex`](../../validation.go) binds a per-context copy of the local
layer to its scalar base; generated definitions and source values stay unchanged.
Existing scalar validation, fixed-value equality, default substitution and
identity recording consume that same effective scalar once. Ordinary inherited
attributes, nil handling and child-element rejection retain their existing paths.
Pending/failed effective-type cache entries now remain failures, including on
repeated lookup, so malformed cycles and unsupported content cannot become
partially resolved types.

The bridge applies the existing enumeration, single-pattern, bound, length and
digit-facet operators, including exact integer/decimal value comparisons. It
preserves supported facet values, but does **not** claim complete facet semantics:
the subsequent [effective-whitespace repair](#effective-scalar-whitespace-repair-update)
executes `whiteSpace`, and the [pattern-group repair](#same-level-pattern-alternatives-repair-update)
executes same-level alternatives. Broader regex coverage, schema-component
validity, fixed-facet metadata and facet-metadata width remain separate work.
The metadata-only tests here establish preservation; the later sections supply
independent runtime evidence for those two operators.
Complex restrictions of mixed/emptiable bases remain explicitly unavailable;
their separate content-category mapping is not implemented here. Existing
wildcard merging and aggregate list/union equality are also unchanged.
A separate QA probe confirmed that absent defaulted IDREF attributes are not
recorded by the existing attribute path; this predates the bridge and remains
an attribute-default materialization follow-up.

Executable evidence is in
[`validation_simple_content_test.go`](../../validation_simple_content_test.go),
[`validation_simple_content_metadata_test.go`](../../validation_simple_content_metadata_test.go)
and the [generator tests](../../internal/xsdgen/generate_simple_content_validation_test.go),
using [reproducible synthetic metadata](../../testdata/validation/simple-content-contract.xsd).
Original-source tests exercise local and inherited failures, optional inline
scalar facets, integer/decimal exactness, Unicode/token and list lengths,
required/prohibited/fixed attributes, defaults/fixed values, nil and child
boundaries, ID/IDREF behavior, sibling isolation and source/schema immutability.
`TestSimpleContentRestrictionsAgainstSchema` sends identical XML bytes to
`xmllint --nonet --schema` and is required by Linux CI. The external subset
excludes arbitrary-width precision probes, libxml2's explicit element-fixed
lexical comparison, and its observed missing-IDREF/duplicate-ID leniency;
those remain internal expectations rather than oracle-backed claims.

All six pinned production XSDs contain zero complex simple-content restrictions.
Production schemas, generated public models and public APIs are unchanged; only
the existing synthetic nil fixture gains its empty restriction layer on
regeneration. This is a bounded reusable-scalar prerequisite for
`REQ-DEF-OPS-SCALARS` / `DEF04-06`, supporting the exact numeric work, not completion
of `STAGE-DEF-03` or `STAGE-DEF-04`. Catalog binding, adapters, budgets, reports,
profile selection, `xsi:type` and schema-hint decisions remain open. Planning
JSON statuses retain `planned`.

[simple-content-mapping]: https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#Complex_Type_Definition_details

## Effective scalar whitespace repair update

The existing validator now applies the most-derived atomic/list `whiteSpace`
policy before inherited lexical facets. Named and inline restrictions retain
`preserve`, `replace` and `collapse`; normalization uses XML `S` only, keeping
NBSP and NEL as data. The immutable lexical view is separate from source text
and exact numeric values. Patterns, enumerations, builtin datatype and numeric
bound diagnostics retain the original source spelling, including inherited
failures; digit/length messages continue to report measured counts.

[`validation_whitespace.go`](../../validation_whitespace.go) resolves policy and
genuine atomic string ancestry with a per-context cache. Missing, invalid-policy
and cyclic dependencies fail closed, including repeated lookups. This is not a
full schema-component validity checker or a change to generated fixed helpers.
Union members retain independent policies and declaration order; the first
member that validates supplies its normalized lexical view to outer restrictions.
An outer facet failure does not backtrack to another union member. List lexical
facets receive collapsed text; aggregate typed equality remains unimplemented.
Enumeration literals use direct atomic/list normalization or ordered union-member
assessment without reassessing every inherited restriction. Union literal views
are cached per context by declaring restriction and literal index, including
explicit success/failure for empty values. Cache growth is bounded by reached
schema enumeration entries, never by XML instance values. Layered atomic/list,
union and nested-union allocation regressions in
[`validation_whitespace_cost_test.go`](../../validation_whitespace_cost_test.go)
prevent exponential repeated base assessment; they do not set a public work budget.

Enumeration candidates are interpreted using the base type of the restriction
that declares them. They do not inherit a same-level or descendant's stronger
policy: a padded enumeration declared over `xs:string` can become unreachable
when the subject is collapsed. Fixed comparisons instead apply the declaration's
effective type policy to both sides, only for genuinely atomic string-derived
types. The existing exact integer/decimal comparators and raw mixed-content
fixed comparisons retain their domains. Boolean, date/time, QName, URI and
aggregate value-space equality remain separate work.

Normative basis: XSD 1.0 [whiteSpace][scalar-whitespace],
[normalization during validation][scalar-normalization],
[enumeration base values][scalar-enumeration],
[ordered union members][scalar-union] and [element fixed constraints][scalar-fixed].
The complex simple-content bridge above supplies the same effective scalar to
this path without a second identity-registration pass. Source nodes, attributes,
constraint literals, generated schemas and public models remain unchanged.

[`validation_whitespace_test.go`](../../validation_whitespace_test.go), using
[generated test-only metadata](../../testdata/validation/whitespace-contract.xsd),
covers strengthened policies versus inherited patterns/length, both enumeration
traps and base-collapse controls, named/inline/complex scalar paths, attribute
references, CDATA/split text, empty/XML-`S`-only values, NBSP/NEL, ordered and
failed union members, list lexical normalization, scalar fixed/default/nil/child
boundaries, exact diagnostics, sibling isolation and source/schema immutability.
`TestWhitespaceMetadataFailures` checks missing, malformed and cyclic metadata;
`TestWhitespaceFixedComparisonScope` keeps comparison dispatch bounded.

`TestEffectiveWhitespaceAgainstSchema` sends identical original XML bytes to
`xmllint --nonet --schema` and is required in Linux CI. libxml2 2.9.14 locally
rejects six explicit scalar element-fixed spellings equal in value space;
only these cases are excluded from the oracle subset and retain normative
internal acceptance tests. Attribute-fixed normalization agrees externally.

This is a bounded existing-validator part of `REQ-DEF-OPS-SCALARS` / `DEF04-06`,
not completion of `STAGE-DEF-04` or a new source/model API. Production schemas,
public models and planning statuses are unchanged. The later
[pattern-group repair](#same-level-pattern-alternatives-repair-update) supplies
same-level alternatives. Broader regex/facet support, mixed-parent mapping,
absent-defaulted-IDREF augmentation, catalog/profile/budget choices and
strict-source adapter integration remain open.

[scalar-whitespace]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#rf-whiteSpace
[scalar-normalization]: https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#whiteSpace
[scalar-enumeration]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#rf-enumeration
[scalar-union]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#dt-union
[scalar-fixed]: https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt

## Same-level pattern alternatives repair update

The existing scalar restriction path in [`validation.go`](../../validation.go)
now treats each local `Patterns` slice as an OR group. Base validation still
runs independently, retaining intersection across restriction layers rather than
flattening groups. This follows XSD 1.0 [multiple patterns in one restriction][pattern-groups].
Every reached local alternative is evaluated before accepting or rejecting the
group, so an earlier match or miss cannot conceal malformed or unsupported
pattern metadata. An actual schema error still stops assessment immediately;
this is not a schema-component validity pass over unreachable restrictions.
A successful group continues into bounds, length and digit checks.

The subject remains the effective normalized lexical view from the whitespace
repair above, including collapsed list text and the first successful union
member's view. Numeric equality does not canonicalize the pattern subject:
`01.20` and `+1.2` can satisfy a group while equal-valued `1.2` fails it. Failed
groups report their alternatives together and retain original display spelling.
The exact single-pattern diagnostic and no-pattern behavior are unchanged.
The [existing regex translator](../../validation_pattern.go) is unchanged;
unsupported constructs such as Unicode block escapes are not newly supported.

[`validation_pattern_groups_test.go`](../../validation_pattern_groups_test.go)
uses [reproducibly generated synthetic metadata](../../testdata/validation/pattern-contract.xsd)
for first/last alternatives, empty patterns, inherited intersections, own/inherited
and strengthened whitespace, numeric spelling, named/inline/complex scalar and
attribute paths, list/union lexical subjects, defaults/fixed values, nil/children,
and continued bounds/length/digit checks. Internal permutations cover malformed
and translator-unsupported patterns before and after matches and misses; repeated
assessment verifies exact diagnostics and source/metadata immutability.
`TestPatternGroupsAgainstSchema` assesses the same original XML bytes internally
and with `xmllint --nonet --schema`, and Linux CI requires it. Of 135 source cases,
134 are oracle-comparable; the sole exclusion is libxml2 2.9.14's numeric
**element**-fixed lexical comparison. The analogous attribute-fixed alternative
agrees externally. The excluded source retains its internal value-space contract.

All six pinned production schemas have no sibling multi-pattern group;
`musicxml.xsd` has 11 single pattern facets. This is a bounded reusable-operator
repair under `REQ-DEF-OPS-SCALARS` / `DEF04-06`, not completion of a stage or
profile. Production schemas/models, public APIs and planning statuses are
unchanged. Broader regex support, aggregate and other primitive typed equality,
mixed/emptiable-base mapping, wildcard derivation, schema-component validity,
fixed-facet metadata/facet-width limits, absent-defaulted-IDREF augmentation,
`xsi:type`/schema hints and catalog/adapter/budget/profile decisions remain open.

[pattern-groups]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#src-multiple-patterns

## Ordinary-attribute repair update

The ordinary-attribute gap on simple- and builtin-typed elements is now repaired
at the existing validator's element boundary in
[`validation.go`](../../validation.go). Named and inline simple types and builtin
simple types reject undeclared attributes by expanded name. Namespace declarations
remain permitted. The check runs before nil-content short-circuiting, leaves
`xs:anyType` unchanged, and does not run again when a complex simple-content type
validates its scalar base. Existing complex attributes, inherited attributes,
fixed/default behavior and identity recording remain on their current path.

This is a bounded existing-validator slice of `REQ-DEF-ATTR-SIMPLE` and
`REQ-DEF-ATTR-COMPLEX` (`DEF02-01/02`), not completion of `STAGE-DEF-02`.
[XSD 1.0 cvc-type 3.1.1](https://www.w3.org/TR/xmlschema-1/#cvc-type) permits the
four standard schema-instance attribute names on simple elements. Their previous
acceptance is preserved here, and unknown `xsi` names are rejected. At that repair,
`xsi:nil` handling was unchanged; the [nil repair below](#xsinil-repair-update)
now checks its bounded existing-validator contract. Full schema-instance
integration (`REQ-DEF-XSI-*`, `DEF02-03/04`) remains open, including the known
complex-type rejection of `xsi:type`. The later
[schema-location name repair](#schema-location-hint-name-repair-update) removes
complex-type rejection of the two hint names, without checking their values.
Permitting a standard name is not evidence that its value semantics have been
checked.

Executable evidence is in
[`validation_attributes_test.go`](../../validation_attributes_test.go):
`TestValidateElementAttributeContracts`, `TestValidateSimpleElementTypeForms`,
`TestValidateSimpleElementAttributeAndContentIssues`,
`TestValidateAnyTypeAttributesUnchanged`,
`TestValidateComplexSimpleContentIdentityOnce`,
`TestValidateUnknownElementTypeRemainsSchemaIssue`,
`TestDecodeDoesNotValidateSimpleElementAttributes`, and
`TestElementAttributeContractsAgainstSchema`. The internal checker and required
Linux external-XSD job use the same raw source cases against the pinned score
and opus schemas. Valid standard-xsi cases are compatibility checks, not tests
of the deferred semantic contracts. Full source validation is still not exposed:
`Decode` may discard these attributes, and public `Validate` still uses
Encode/reparse and cannot reconstruct them. Strict-source integration and
machine-readable partial-implementation evidence remain separate work.

## Namespace-declaration provenance repair update

The namespace-provenance follow-up is now repaired in the existing internal
parser and validator. `encoding/xml` represents namespace declarations with the
sentinel namespace `xmlns`; an ordinary attribute bound to the literal relative
namespace URI `xmlns` can have the same expanded name. The
[`wellFormedXMLTokenReader`](../../xml_wellformedness.go) now records a declaration
flag for each attribute from its raw lexical name, before namespace expansion.
`parseValidationDocument` copies those flags together with the expanded attributes
into each `validationNode`. `validationNamespaceDeclaration` reads that retained
provenance, so only actual declarations bypass ordinary attribute checks. Names
are still expanded exactly once, and no second namespace resolver is introduced.

The retained [raw reproducer](../../testdata/validation/namespace-declaration-lookalike.musicxml)
now produces three internal `attribute` issues, on `measure`, `staves` and `step`,
and the external XSD test rejects the identical source. Shared raw cases in
[`validation_attributes_test.go`](../../validation_attributes_test.go) also cover
lexical attribute order, declarations and ordinary attributes with the same
expanded name, `p:xmlns`, real declarations and complex simple-content.
`TestValidateNamespaceDeclarationLookalike` checks the exact three issue paths.
[`validation_namespace_provenance_test.go`](../../validation_namespace_provenance_test.go)
covers retained flags after nested and self-closing elements, default namespace
resets, prefix rebinding and restoration, explicit `xml` declarations, required
ordinary attributes in the literal `xmlns` namespace, and unchanged public
Decode behavior across supported encodings.

This closes the recorded declaration/lookalike ambiguity without changing the
public API. The internal Encode/reparse helper retains its plain UTF-8 input
contract; public Decode still discards unsupported attributes and public Validate
cannot reconstruct them. Full XML/namespace conformance, strict-source adapters,
full schema-instance semantics and machine-readable partial-implementation
evidence remain separate work. The planning JSON's `status: planned` semantics
are unchanged; these regressions do not complete `STAGE-DEF-01` or `STAGE-DEF-02`.

## Schema-location hint name repair update

The existing internal validator now permits the exact expanded names
`{http://www.w3.org/2001/XMLSchema-instance}schemaLocation` and
`{http://www.w3.org/2001/XMLSchema-instance}noNamespaceSchemaLocation` on complex
types, including complex simple-content. Simple types already permit them.
This follows [XSD 1.0 cvc-complex-type clause 3][xsi-name-rule]; it does not
implement the other schema-instance contracts. Unknown `xsi` names, unqualified
lookalikes and foreign-namespace lookalikes still follow ordinary attribute
validation. Both attribute paths use `validationStandardXSIName`; its explicit
`allowType` parameter preserves the existing difference between simple and
complex paths. Allowing the `xsi:type` name on simple elements does not resolve
or apply that type. That name repair left `xsi:type`, `xsi:nil` and `xs:anyType` behavior
unchanged; the later [nil repair](#xsinil-repair-update) adds nil checks.

This is only a **name-permissibility slice** of `REQ-DEF-XSI-SCHEMALOC` and
`REQ-DEF-XSI-NONAMESPACE` (`DEF02-03/04`). Hint values are not validated: no
lexical, list or namespace/location pair check is added. No schema is fetched
or selected from a hint; the existing caller-selected score/opus schema stays
in effect. Public Decode still drops unsupported source attributes, and public
Validate still assesses Encode/reparse output. No strict-source API is added.
The planning JSON requirements and their `status: planned` remain unchanged.

Executable evidence is in
[`validation_schema_hints_test.go`](../../validation_schema_hints_test.go).
`validationSchemaHintCases` adds identical raw-source fixtures to both
`TestValidateElementAttributeContracts` and the required Linux oracle test
`TestElementAttributeContractsAgainstSchema`. Cases include partwise/timewise
and opus roots, complex content, complex simple-content, named/builtin simple
types, alternate and inherited/rebound prefixes, exact-name negative controls,
and ordinary required/enumeration/datatype/fixed/content failures beside hints.
`TestSchemaLocationHintsDoNotFetch` uses a live HTTP request counter with valid
and invalid documents; `TestSchemaLocationHintsPreservePublicModelValidation`
checks the existing public model boundary.
[`validation_xsi_names_test.go`](../../validation_xsi_names_test.go) checks both
helper modes across exact/case-variant/unknown names and matching/unqualified/
foreign namespace URIs, plus the preserved `xsi:type` name allowance on builtin,
inline simple, complex and complex simple-content types.

### Deferred hint-value reconciliation

[XSD 1.0 section 3.2.7][xsi-declarations] defines `schemaLocation` as a list of
`anyURI`, and `noNamespaceSchemaLocation` as `anyURI`.
[Section 4.3.2][xsi-location-pairs] separately describes namespace/location pairs
for schema discovery. The plan's `TEST-DEF-XSI-SCHEMALOC` expects an odd number
of tokens to fail the pair contract. Before implementing that contract, establish
which normative assessment layer owns that rejection and distinguish it from
the built-in list datatype and this library's fixed-schema, no-fetch policy.

A stage-selection probe with xmllint/libxml2 2.9.14, `--nonet`, and explicit
pinned `opus.xsd` accepted odd token counts and candidate values `%` and
`http://[invalid` for the standard hints. **That leniency is not evidence that
these values are normatively valid.** It shows that this oracle invocation
cannot establish the deferred lexical/pair outcomes. The name-allowance tests
therefore use ordinary valid URI hints and do not freeze malformed-value
acceptance as a compatibility requirement. Implementing hint lexical/pair
semantics, reconciling the oracle evidence with the cited normative rules,
full `xsi:type` handling, its interaction with nil, strict-source integration
and checked machine-readable implementation evidence remain separate work.

[xsi-name-rule]: https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-complex-type
[xsi-declarations]: https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#xsi.schemaLocation
[xsi-location-pairs]: https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#schema-loc

## xsi:nil repair update

The existing internal validator now checks `xsi:nil` by expanded name against
[XSD 1.0 Element Locally Valid, clauses 3.1 and 3.2][nil-rule] and the
[boolean lexical contract][nil-boolean]. Only `true`, `1`, `false` and `0` are
accepted after XML whitespace normalization. Unicode whitespace does not become
XML whitespace. A nonnillable declaration forbids the attribute even when its
value is `false` or `0`.

On a nillable declaration, true nil requires no element or character content,
including whitespace-only text, and forbids a fixed value constraint. Comments
and processing instructions do not count as character content. Scalar values
and required child particles are suppressed only for true nil; complex
attributes still pass through the existing effective-attribute validator,
including inherited required, prohibited, fixed and datatype constraints and
ID/IDREF tracking. False nil uses ordinary content and attribute validation.
Diagnostics for the nil attribute use its expanded-name path, independently
of the source prefix.

One pre-existing assessment limit remains explicit. When a true-nilled element
illegally contains children, the validator reports the parent's nil-content
failure and still checks its attributes, but does not assess those descendants
or record their identities. A reference elsewhere to an ID only inside that
rejected subtree can therefore produce an additional unresolved-IDREF issue.
The empty-content default/fixed gap that was deferred by the nil repair is
addressed separately in the [element value-constraint update below](#empty-element-value-constraint-repair-update).
Its application branch is cvc-elt 5.1/5.1.2; clause 5.1.1 specifically concerns
an `xsi:type`-selected local type and remains outside these repairs.

Executable evidence is in [`validation_nil_test.go`](../../validation_nil_test.go):
`TestValidateNilContracts`, `TestValidateNilIdentityTracking`,
`TestValidateMusicXMLNilContracts`, `TestNilContractsAgainstSchema`, and
`TestMusicXMLNilPreservesPublicModelValidation`. The external test uses identical
original XML bytes for its internal/oracle parity cases with real
`xmllint --nonet`; Linux CI requires it to execute. Positive nil cases use the synthetic
[`nil-contract.xsd`](../../testdata/validation/nil-contract.xsd), because the
pinned MusicXML score and opus declarations are all nonnillable. The fixture's
runtime metadata is generated into the test-only
[`zz_generated_nil_validation_test.go`](../../zz_generated_nil_validation_test.go)
using the existing schema generator and a local empty catalog. `go generate`
and the `check-all` generation stage guard schema/metadata drift; no handwritten
full-schema mirror remains. Positive cases also compose nil with the standard
schema-location hint names on simple and complex elements. Real MusicXML
negative cases prevent synthetic-schema support from implying new MusicXML
nil support. Separate internal assertions cover known libxml2 2.9.14 limitations: it accepts
unresolved IDREF(S) and whitespace-only IDREFS, and rejects empty CDATA on
nilled elements even though that section contributes no character information
items ([XML Infoset section 2.6][nil-characters]). IDREFS requires a
[nonempty sequence][nil-idrefs]. These oracle disagreements are not treated as
normative outcomes.

This is an existing-validator slice of `REQ-DEF-XSI-NIL` and
`CHECK-XSD-LANG-NIL`. The planning contracts clarify that false nil also needs
a nillable declaration, but retain `status: planned`: source adapters, dynamic
`xsi:type` resolution and its nil interactions, rule-level implementation
bindings, and model capability mapping remain future work. Hint-value semantics
are unchanged. `Decode` still discards unsupported source attributes; public
`Validate` still encodes and reparses the typed model and cannot recover the
original nil attribute. No public API, schema fetch, or generated-model change
is introduced.

[nil-rule]: https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-elt
[nil-boolean]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#boolean
[nil-characters]: https://www.w3.org/TR/xml-infoset/#infoitem.character
[nil-idrefs]: https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#IDREFS

## Empty-element value-constraint repair update

The existing internal validator now applies an element declaration's stored
default or fixed value when the original element has no character or element
children and is not true-nilled. This is the bounded empty-content branch of
[XSD 1.0 cvc-elt 5.1/5.1.2][nil-rule], read with the
[approved E1-56 correction][element-default-erratum]. Whitespace-only content
does not trigger defaulting. Comments, processing instructions and empty CDATA
contribute no character content. Absent elements are never created.

After declaration resolution and the original-source nil checks, validation
uses a temporary node with a fresh text builder and the existing attributes.
The stored schema lexeme follows the ordinary scalar/facet and complex-type
paths; required/prohibited/fixed attributes and IDREF(S) recording still apply.
The source node is not modified, repeated validation remains stable, and no
default is materialized into the caller's Go model. True nil still suppresses
defaults and still rejects a fixed constraint.

Executable evidence is in
[`validation_element_values_test.go`](../../validation_element_values_test.go).
The matrix covers builtin/named/inline simple types, inherited simple-content,
mixed content with an emptiable particle, global/referenced and local declarations, empty-string
constraints, lexical-pattern preservation, nil/whitespace/content boundaries,
ordinary attributes, and public partwise/timewise/opus model stability. Both the
internal validator and `TestElementValueConstraintsAgainstSchema` receive the
same original bytes. Linux CI requires that oracle test with `xmllint --nonet`.
The extended synthetic XSD still generates the test-only validation metadata;
`go generate` guards drift. Identity tests independently check resolved and
unresolved default/fixed IDREF(S), since libxml2 does not reliably reject missing
targets. No invalid ID-valued default declarations are introduced.

`TestElementValueConstraintEmptyCDATA` separately checks zero-character CDATA:
xmllint/libxml2 2.9.14 incorrectly leaves a defaulted integer empty in that case.
That oracle disagreement is not used as a normative rejection.

This is a narrow existing-validator prerequisite for `REQ-DEF-OPS-SCALARS`
(`DEF04-06`), not completion of `STAGE-DEF-04` or the schema-instance stage.
Planning JSON remains `status: planned`. The pinned MusicXML score and opus
schemas have no element default/fixed declarations, so this synthetic-schema
capability adds no newly covered production declaration. No public API, generated
production model, strict-source adapter, schema fetching, or schema assembly is
added. `Validate` still encodes/reparses the model and cannot recover discarded
source facts.

General typed fixed-value equality remains unchanged. The fixed mixed-content
child gap is addressed in the [separate repair below](#fixed-element-child-content-repair-update).
QName/NOTATION schema-context handling, general schema
canonical-value processing, attribute-default materialization, `xsi:type`, hint
values, and assessment of descendants beneath an invalid nilled element remain
unchanged. This
repair does not claim complete default/fixed or XSD validation.

[element-default-erratum]: https://www.w3.org/2004/03/xmlschema-errata.html#e1-56

## Fixed-element child-content repair update

The existing internal validator now rejects element children under a resolved,
fixed-constrained element when it is not true-nilled. This implements the
separate structural condition in [XSD 1.0 cvc-elt 5.2.2.1][nil-rule], including
mixed content and `xs:anyType`; matching direct character content does not
excuse a child element. It is independent of the fixed-value equality condition
in 5.2.2.2. The [approved E1-56 correction][element-default-erratum] leaves this
prohibition unchanged.

The check runs at the declaration boundary, after reference resolution and nil
assessment. A child-content failure gets one `fixed` issue at the parent path,
with structural failure taking precedence over a textual fixed-value mismatch.
Ordinary type validation continues: required, prohibited, fixed and datatype
attribute checks, matched-child assessment, and existing ID/IDREF tracking are
not bypassed. Empty-element defaulting, true-nil handling and child-free textual
fixed comparisons keep their existing behavior; no source nodes are changed.

Executable evidence is in
[`validation_fixed_children_test.go`](../../validation_fixed_children_test.go):
`TestValidateFixedElementChildren` covers matching and mismatching mixed content,
child-only content, empty fixed strings, inherited mixed types, global/referenced
and local declarations, builtin and complex simple-content, `xs:anyType`, nil
boundaries, comment/PI/CDATA controls, and ordinary attribute/child errors.
Default-constrained and unconstrained mixed children remain permitted.
`TestFixedElementChildrenPreserveAssessment` checks parent and descendant identity
recording, unresolved references, and repeated validation without mutation.
The fixture metadata is regenerated from `testdata/validation/nil-contract.xsd`.

`TestFixedElementChildrenAgainstSchema` feeds the same original bytes to the
internal validator and required Linux `xmllint --nonet` for its explicitly
comparable subset. libxml2 2.9.14 incorrectly accepts the decisive mixed
counterexample `<fixed-mixed required="1">kept<child>7</child></fixed-mixed>`.
It also incorrectly rejects an empty CDATA section under a mixed empty-string
fixed declaration. These cases remain normative unit regressions outside the
parity subset; tests do not require an oracle to retain either bug. Oracle
rejection for a separate attribute or child datatype error does not establish
that it implements the fixed-child rule.

This is another bounded existing-validator prerequisite for
`REQ-DEF-OPS-SCALARS` (`DEF04-06`), not completion of `STAGE-DEF-04`. Planning
JSON remains `status: planned`. The pinned production schemas still have no
element default/fixed declarations; this repair makes no new production
coverage claim. General typed equality, canonicalization, QName/NOTATION schema
context, attribute-default materialization, `xsi:type`, hint values, invalid
nilled descendant assessment, and new source/model adapters remain deferred.
No public API, production model/schema, schema fetch, or schema assembly is added.
Public `Validate` retains its Encode/reparse model boundary.

## Element-content XML whitespace repair update

The existing internal validator now uses XML whitespace rather than Go's broader
Unicode whitespace classification when checking nonmixed complex content.
[XSD 1.0 cvc-complex-type 2.3][xsi-name-rule] permits only the four characters in
[XML 1.0 production S](https://www.w3.org/TR/REC-xml/#NT-S): space, tab, carriage
return and line feed. Nonbreaking space, NEL and Unicode space separators are
legal XML characters but are not permitted as element-only character content.
Comments and processing instructions remain irrelevant to that check; CDATA
and character references contribute their decoded characters.

The check retains its existing parent-path `element-only` diagnostic. It does
not return early: ordinary attributes, child grammar, matched child values and
ID/IDREF assessment continue. Mixed content, simple content, nil handling and
source-node immutability remain unchanged. Tests use original XML bytes against
the pinned partwise/timewise and opus schemas as well as the existing synthetic
inherited-type fixture. Executable evidence is in
[`validation_content_whitespace_test.go`](../../validation_content_whitespace_test.go):
`TestValidateElementOnlyXMLWhitespace`,
`TestElementOnlyXMLWhitespacePreservesAssessment`, and
`TestElementOnlyXMLWhitespacePreservesPublicModels`.
`TestElementOnlyXMLWhitespaceAgainstSchema` supplies the same original bytes
to the internal validator and required Linux `xmllint --nonet`, using local
catalogs without schema fetching.

libxml2 2.9.14 rejects XML-whitespace-only and empty CDATA in element-only
content even though the former contributes only permitted character codes and
the latter contributes no characters. Those normative cases remain in the
internal regressions and are explicitly excluded from the comparable oracle
subset; tests do not require the oracle to retain that bug. Non-XML character
data in CDATA remains in the rejection parity cases.

This is a bounded existing-validator repair for `CHECK-XSD-LANG-TEXT` and
`TEST-XSD-LANG-TEXT`, and a prerequisite for the shared-operator work in
`DEF04-06`. It does not complete those contracts, `STAGE-DEF-04` or `STAGE-xsd`;
the planning JSON retains `status: planned`. That repair also rejects Unicode
whitespace in the existing empty-type path. The later
[particleless empty-content repair](#particleless-empty-content-repair-update)
closes its XML-whitespace gap under cvc-complex-type 2.1; full schema content
category normalization remains separate work.

No production schemas, generated metadata/models, public API or validation
profile are changed. Public Decode may discard inter-element source text, and
public `Validate` still assesses Encode/reparse output. Strict-source and direct
model adapters, full text-category classification, typed fixed-value equality,
schema-instance type/hint values and other deferred contracts remain open.

## Particleless empty-content repair update

The existing internal validator now rejects all character content for a resolved,
nonsimple, nonmixed complex type whose effective particle is absent, including
space, tab, carriage return and line feed. [XSD 1.0 cvc-complex-type 2.1][xsi-name-rule]
requires no character or element information-item children for empty content.
This differs from element-only content, which permits XML `S`. Comments,
processing instructions and empty CDATA contribute no characters and remain
valid. Character references and nonempty CDATA contribute their decoded text.

[`validateComplex`](../../validation.go) checks this already-represented empty
category after attribute, true-nil and simple-content handling. It retains the
legacy parent-path `element-only` diagnostic and `character data is not allowed`
message for character failures, including the existing nonwhitespace failures.
Unexpected children retain their `content-model` diagnostics. Attribute failures
and ID/IDREF assessment still run, and source nodes remain unchanged.

The scope is deliberately **absent effective particles**, not particle
emptiability. Direct and restricted particleless types, empty-to-empty extension,
and restriction from an optional particle to empty are covered. A base extended
with an actual particle, a nullable particle with no instance children, mixed
content, simple content, true nil and `xs:anyType` keep their existing contracts.
The generator still retains explicit empty sequence/all/optional-empty-choice
particles and loses some source distinctions when expanding groups; normalizing
those into the XSD content categories requires separate generator/runtime work.
No recursive or nullable-particle heuristic is introduced. Mixed-derivation
normalization is also outside this repair.

Executable evidence is in
[`validation_empty_content_test.go`](../../validation_empty_content_test.go):
`TestValidateEmptyComplexContent`, `TestEmptyComplexContentCategoryControls`,
`TestEmptyComplexContentPreservesAssessment` and
`TestEmptyComplexContentPreservesPublicModels`. The synthetic
[`nil-contract.xsd`](../../testdata/validation/nil-contract.xsd) supplies generated
metadata for the inheritance/category boundary cases; production partwise,
timewise and opus cases use the pinned original schemas. Source cases cover
literal, reference and CDATA whitespace, empty/comment/PI controls, nil values,
attributes, identity references and existing unexpected-child paths.

`TestEmptyComplexContentAgainstSchema` validates identical original bytes with
the internal checker and required Linux `xmllint --nonet --schema`, using local
catalogs. libxml2 2.9.14 incorrectly rejects empty CDATA in genuinely empty complex
content despite its lack of character information items. Those normative positive
cases remain internal regressions outside the parity subset; whitespace-bearing
CDATA rejection remains in that subset. No source is decoded and re-encoded
before an oracle comparison.

This is a bounded existing-validator prerequisite for `CHECK-XSD-LANG-TEXT`,
`TEST-XSD-LANG-TEXT` and `DEF04-06`, not completion of those contracts,
`STAGE-DEF-04` or `STAGE-xsd`. Planning JSON remains `status: planned`. Production
schemas, generated production models/metadata, public APIs and validation profiles
are unchanged. Decode remains permissive for source-XSD character-content
violations; public `Validate` assesses Encode/reparse output and cannot recover
text discarded by Decode. Strict-source/direct-model adapters, full content
category normalization and the other deferred validation contracts remain open.

## Reuse and adaptation

| Existing component | Proposed use | Required adaptation or verification |
| --- | --- | --- |
| [`internal/xsdgen/generate_validation.go`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/internal/xsdgen/generate_validation.go), generated score and opus schemas | Reuse schema extraction and generated contract data where verified | Preserve source IDs and version fingerprints; map generated facts to the shared catalog without creating a second independent XSD rule set |
| [`matchParticle` and related matchers](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/validation.go#L1747-L1952) | Candidate shared content-grammar operators | Verify sequence/choice, repetitions, nullable branches, and resource bounds against the catalog |
| [`validateSimple`, `validateBuiltin`, and facet helpers](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/validation.go#L1080-L1745) | Candidate scalar-domain operators | Verify exact value spaces, union normalization, pattern semantics and facet layers; remove confusion between bounded Go representation and unbounded XSD integer domains |
| [`validateAttributes`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/validation.go#L889-L984) | Reuse permitted/required/default/fixed attribute logic | Cover attributes on simple-typed elements as well as complex types; implement the separate contracts for all four standard xsi attributes |
| [`recordIdentity` and `validateIdentityReferences`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/validation.go#L1018-L1078) | Reuse document-wide ID indexing and reference resolution | Add prose-defined target kinds and part/instrument/player scope; ID existence alone is insufficient |
| [`newDocumentDecoder`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/decode.go#L147-L183), [`xml_decoder.go`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/xml_decoder.go), [`xml_namespace.go`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/xml_namespace.go) | Feed source facts and model decoding from the same XML parse | Observe unfiltered expanded names and attributes before permissive filtering; enforce the missing XML checks and finish at EOF |
| Current [`Validate`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/validation.go#L70-L125) | Preserve an explicit model-validation entry point while adapting its implementation | Replace its Encode → parse route with a direct model-fact adapter. This requirement does not imply discarding every existing matcher or scalar helper |
| [`mxl.go`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/mxl.go#L173-L329), [`options.go`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/options.go), [`document_depth.go`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/document_depth.go) | Retain useful package checks, XML/MXL budgets, and model traversal guards | Map existing path, mimetype, container/rootfile, depth, and cycle checks to appropriate categories; do not claim full ZIP/JAR semantics |

The target is one shared rule implementation with source and model adapters.
Reuse is contract-by-contract. Neither a wholesale rewrite nor the sufficiency
of the current XSD engine is assumed.

## Confirmed limits of the current implementation

Focused review probes at the pinned code base showed the following. They
identify the original implementation work; completed repairs are called out
explicitly below, while other limits remain open.

- **Attributes on simple-typed elements** (ordinary-attribute slice repaired by
  [PR #16](https://github.com/go-muse/go-musicxml/pull/16)): unrecognized attributes
  on `staves` and `step`, such as `rubbish="x"`, passed the internal
  source-validation path at the pinned review baseline. At that baseline,
  [`validateType`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/validation.go#L569-L620)
  checked text and children in its simple/builtin branches, with attribute
  validation reached only through the complex-type branch. The existing validator
  now rejects these ordinary attributes through `validateSimpleElementAttributes`;
  see the [repair update](#ordinary-attribute-repair-update) for executable
  evidence and remaining xsi limitations; the later
  [provenance repair](#namespace-declaration-provenance-repair-update) closes the
  recorded namespace-declaration lookalike gap. Complex-type attributes such
  as `measure/@implicit="maybe"` were already rejected at the baseline and remain
  rejected.
- **XML well-formedness** (repaired by [PR #14](https://github.com/go-muse/go-musicxml/pull/14)):
  duplicate attributes and a DOCTYPE inside the root were accepted at the
  pinned review baseline. The existing decoding/internal parsing paths now
  reject them through [`xml_wellformedness.go`](../../xml_wellformedness.go),
  including skipped subtrees. See the [repair update](#xml-reading-repair-update)
  for current coverage; future strict-source integration remains open.
- **xsi semantics:** The pinned baseline rejected schema-location hints on
  complex types as unallowed attributes. The existing validator now permits
  their exact standard names; see the [name repair](#schema-location-hint-name-repair-update).
  Hint lexical/list/pair semantics are still unchecked and require the standard
  attribute contracts. The [nil repair](#xsinil-repair-update) covers nil in the
  current internal validator; `xsi:type` and its nil interaction remain open.
- **Integer value space:** `staves=18446744073709551616` is accepted by libxml2
  against the pinned schema but rejected by the current scalar checker.
  Its [`ParseInt`/unsigned parsing branches](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/validation.go#L1410-L1440)
  use machine-width limits even for unbounded XSD integer families. A Go-model
  representation limit must not be reported as a normative XSD violation.
- **Additional prose conditions:** a `part/@id` pointing at an ID of the wrong
  kind, an empty `accordion-registration`, and duplicate beam numbers on one
  note pass current validation. They demonstrate concrete needs beyond generic
  IDREF and structural XSD checks.

## Model evidence that can and cannot be supplied

The adapter must describe capabilities per field and rule, not promise that
all source information survives except for a short list of lexical details.

| Model feature | Evidence available or lost |
| --- | --- |
| Optional pointer fields | Usually preserve omitted versus present values; `Effective...` methods expose defaults without mutating omission |
| Required non-pointer scalars | May lose source presence: a missing `octave` becomes the zero value after Decode and can pass current model validation |
| Singular fields | May lose source multiplicity: repeated `octave` elements collapse during Decode and can pass current model validation |
| Ordered `Content` slices | Preserve the represented interleaving, subject to decoder filtering; this is not evidence for discarded nodes |
| Fixed struct-field sequences | Describe current model/export order, not necessarily historical source order |
| Integers | Exact only within the concrete Go type's range; original integer lexical form is not preserved |
| Decimal fields | `float64` can lose exact decimal values and lexical forms |
| Foreign/unknown nodes, duplicate attributes, declarations and directives | Discarded or normalized input cannot be reconstructed from the current model |

A direct model check evaluates the current object state. It must not claim
that a missing or repeated original scalar was absent/present exactly once.
Where a current model actually represents an absent required child or attribute,
that remains a normative required/minOccurs violation, not merely an internal
model error. Insufficient evidence for an applicable obligation must be surfaced
as incomplete/unknown/unsupported rather than silently treated as a pass.

## Existing tests and security behavior

[`TestEncodedCorpusConformsToMusicXMLSchema`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/external_validation_test.go#L47-L87)
checks 146 fixtures after **Decode → Encode**, then supplies the exported bytes
to `xmllint --nonet`. This is useful evidence about generated output. It is not
proof that arbitrary original source XML is fully validated by the library.
Corpus regression and rule-by-rule source/model coverage answer different
questions.

The present `encoding/xml` route does not automatically fetch external DTDs,
entities, or schemas, and does not expand XInclude. Retain this no-network
invariant when adding schema-location handling, linked documents, or external
dictionaries. Existing XML depth, five MXL byte budgets, model depth, and opus
cycle checks are useful safeguards. Broader diagnostic/scalar/attribute budgets
and cancellation remain proposed work.

## Error compatibility proposal

Preserve existing sentinel identities and observable `errors.Is`/`errors.As`
behavior where their meanings still apply, while attaching richer diagnostic
categories. This is a proposed compatibility policy for review, not a completed
API commitment. A future migration must test existing callers and document any
necessary change explicitly.

- [`ValidationError.Unwrap`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/validation.go#L61-L64)
  currently exposes `ErrInvalidDocument`. Preserve that recognition for genuine
  validation failures while making individual diagnoses more precise.
- [`ErrXMLTooDeep`, `ErrDocumentTooDeep`, and `ErrMXLTooLarge`](https://github.com/go-muse/go-musicxml/blob/e486735cd6e4537e839ff67704b7768a9df3bdb3/errors.go)
  represent resource limits; they do not by themselves prove malformed XML or
  a normatively invalid model. Existing wrapping relationships may need to
  coexist with a more accurate new category.
- `ErrDocumentCycle` identifies a model-graph problem. `UnsupportedRootError`
  identifies an unsupported root/profile and is not necessarily malformed XML.
- The current `representation` issue can include an Encode failure. A direct
  adapter should distinguish model invariants, numeric representability, and
  normative violations rather than treating that catch-all as the final design.
- Unknown, unsupported, and blocked-by-invalid-prerequisite outcomes need explicit
  reporting. An incomplete assessment must not become a successful conformance
  claim merely because no violation was found.

## First implementation candidates

A first increment can start with local presence/value relations and typed IDREFs.
The 51 constraint records partition into five fully overlapping XSD records,
eight dictionary-dependent records (seven SMuFL, one IPA), nine MXL records,
and **29 remaining candidate records**. These sets are disjoint in this snapshot.
The remaining applicability is 27 `both`, one `source`, and one `exporter`.
This is a planning filter, not 29 proven independent executable predicates.

A proposed review profile is a selection from this inventory, not a replacement
for it. `MX40-PROSE-bend-release-negative` is already present in the
[catalog](https://github.com/go-muse/go-musicxml-registry/blob/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/catalog.json).
Its omission from a reviewer's candidate list does not indicate a missing catalog
record or justify creating a duplicate. Selection still requires its bend-sequence
context and exact value conditions.

Useful early candidates are `accordion-at-least-one`, `beam-number-distinct`,
local typed instrument/part/player targets, `key-octave-cancel-exists`,
`concert-score-transpose`, and `for-part-requires-concert`. For
`instrument-link-target`, keep the local IDREF type/scope sub-contract separate
from checking the linked part file: that external existence/content obligation
requires a resolver. Passing the local sub-contract does not establish a pass
for the whole requirement. Each still needs an
exact selector, prerequisites, target capabilities, and positive/negative/boundary
cases before becoming executable.

`written-transposition` needs instrument/intent context. `chord-duration-bound`
requires a defined principal-note/duration relation, and `numeral-requires-key`
needs effective key state in musical order and staff context. A comprehensive
timeline can be staged later, but a selected rule's mandatory dependencies
cannot be omitted or guessed.

Renderer/exporter labels are not automatic exclusion from validation work.
Defaults and interpretations can supply context facts; exporter recommendations
can be optional advisories; some obligations need external intent or application
state. Assign the execution/reporting role per record during formalization,
and record dependencies/capabilities separately. A mandatory predicate or context
fact may itself require external information. Missing required facts produce
unknown/unsupported assessment, not automatic not-applicable or pass; applicability
and outcomes depend on the actual document and scope.
