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

The following review follow-ups are still explicitly **deferred**:

- **Machine-readable implementation evidence:** add a reviewed `implemented_by`
  or `evidence` contract to the planning format and its integrity checker.
  Link partial requirement/test implementations to PRs, commits, source files
  and test names; check that referenced local artifacts exist. Retain the
  meaning of `status: planned` and keep incomplete strict-source obligations
  visible rather than marking the entire contract implemented.

- **Namespace-declaration provenance:** repair the pre-existing
  `xmlns:p="xmlns" p:rubbish="x"` ambiguity for `step`, `staves` and complex
  `measure`; retain the [raw regression fixture](../../testdata/validation/namespace-declaration-lookalike.musicxml)
  and require both internal and external validators to reject it. The existing
  [`wellFormedXMLTokenReader`](../../xml_wellformedness.go) sees raw start tags
  before namespace expansion: a declaration is
  `xml.Name{Space: "xmlns", Local: "p"}`, while the ordinary lookalike is
  `xml.Name{Space: "p", Local: "rubbish"}`. Carry per-attribute declaration flags
  from those raw tokens into the `validationNode` built by
  `parseValidationDocument`, then have `validationNamespaceDeclaration` consult
  that provenance instead of inferring it from the expanded `"xmlns"` sentinel.
  Keep this repair inside the existing parser path without a public API change;
  preserve actual namespace declarations and once-only namespace expansion.

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
acceptance is preserved here, and unknown `xsi` names are rejected. The existing
`xsi:nil` handling is unchanged. Full value/type/nillability contracts for all
four names (`REQ-DEF-XSI-*`, `DEF02-03/04`) remain open, including the known
complex-type rejection of `xsi:type` and schema-location hints. Permitting a
standard name is not evidence that its value semantics have been checked.

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

A residual namespace-provenance gap is explicitly deferred: `encoding/xml`
represents namespace declarations with the sentinel namespace `xmlns`. An ordinary
attribute bound to the literal relative namespace URI `xmlns` (for example,
`xmlns:p="xmlns" p:rubbish="x"`) has the same retained shape and is currently
mistaken for a declaration. This affects the existing complex checker as well as
the simple-element check. The concrete raw-token provenance repair is tracked
under [XML-reading follow-ups](#xml-reading-follow-ups). The checked-in
[raw reproducer](../../testdata/validation/namespace-declaration-lookalike.musicxml)
covers `step`, `staves` and complex `measure`. The external-XSD test requires its
rejection; it is deliberately not included among the 35 shared internal/oracle
cases. A future source-reader repair must reject it internally too. This slice
does not claim complete namespace-attribute conformance or resolve that
parser-level ambiguity.

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
  evidence and remaining namespace/xsi limitations. Complex-type attributes such
  as `measure/@implicit="maybe"` were already rejected at the baseline and remain
  rejected.
- **XML well-formedness** (repaired by [PR #14](https://github.com/go-muse/go-musicxml/pull/14)):
  duplicate attributes and a DOCTYPE inside the root were accepted at the
  pinned review baseline. The existing decoding/internal parsing paths now
  reject them through [`xml_wellformedness.go`](../../xml_wellformedness.go),
  including skipped subtrees. See the [repair update](#xml-reading-repair-update)
  for current coverage; future strict-source integration remains open.
- **xsi semantics:** `xsi:noNamespaceSchemaLocation` is rejected as an unallowed
  attribute by the current source path. The proposed fix is the standard
  attribute contract, not blanket acceptance of every xsi value. `xsi:type`
  and `xsi:nil` also require their own semantic checks.
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
