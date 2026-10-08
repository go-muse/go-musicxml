# Deferred validation work after PR 11

Date of the originating inventory: **8 October 2026**.

This is the published English task specification for the deferred-work inventory
following [PR 11's squash merge at `f1f77ae`][pr11], its two reviews, and the
responses to those reviews. It preserves all 15 sections and all 52 stable
`DEFxx-yy` source keys used by the [implementation plan][plan] and
[source snapshot][snapshot]. Sections 11–15 include paragraph-level obligations,
not just list bullets. `DEFxx-yy` keys are editorial traceability labels assigned
to those original bullets and paragraphs, not MusicXML clause IDs or frozen
runtime rule IDs. The research registry remains pinned at
[`3e33e4c`][registry]. The later [Go contract proposal at `ba32b92`][contracts]
provides implementation-design context; it does not change the meaning of this
originating inventory.

At the originating checkpoint, PR 11 had merged as one squash commit and all
nine checks in [the recorded main CI run][ci] had passed. That completed the
documentation stage, not the implementation described below. Registry relocation,
English fields for 51 constraints, 28 questions and six examples, the existing-code
map, and the last four architectural clarifications were already complete. They
are not reopened as deferred work here.

**Status and authority.** This inventory specifies future implementation work and
unresolved decisions. It neither proves implementation or test execution nor
starts implementation, assigns deadlines, freezes a public API, or creates public
issues or code PRs. Links to the code map establish observed implementation limits;
links to the Go contracts establish project design proposals; links to the question
registry establish unresolved research context. These are not, by themselves,
MusicXML normative clauses. Normative checks must retain their exact XML, XSD,
MusicXML prose or external-specification evidence from the pinned registry and
linked sources. API shape, resource policy, compatibility, delivery mechanisms and
implementation order are project decisions. Optional proposals remain optional.

## 1 Mandatory XML layer

Sections 1–3 are three concrete repair blocks that can be prepared independently
of the complete new engine. Each requires narrow regression tests first, followed
by the repair and compatibility checks. These defects were documented, not fixed,
by the documentation PR.

### DEF01-01

Reject duplicate attributes on one element, including collisions after expanding
namespace prefixes.

Basis: [confirmed implementation limits][limits].

### DEF01-02

Reject a DOCTYPE in an invalid position, particularly inside the root or after it;
preserve a permitted DOCTYPE in the prolog.

Basis: [confirmed implementation limits][limits].

### DEF01-03

Perform these checks in both ordinary `Decode` and strict mode, including inside
skipped unknown or foreign-namespace subtrees.

Basis: [confirmed implementation limits][limits] and
[the proposed single-parse adapter contract][single-parse].

### DEF01-04

Cover positive cases, invalid placements, and interaction with existing namespace
and depth checks. Do not introduce a mandatory XML declaration.

Basis: [existing tests and security behavior][existing-tests] and
[the XML-declaration question][xml-declaration].

## 2 XSD and schema-instance attributes

### DEF02-01

Close the gap in attribute-permissibility checking on simple-typed and
builtin-typed elements, such as `staves` and `step`.

Basis: [confirmed implementation limits][limits].

### DEF02-02

Preserve the working attribute checks for complex types: required, permitted,
fixed, enumeration and datatype constraints.

Basis: [existing reuse and adaptation contracts][reuse].

### DEF02-03

Implement separate contracts for `xsi:type`, `xsi:nil`, `xsi:schemaLocation` and
`xsi:noNamespaceSchemaLocation`: QName and derivation checks for `xsi:type`,
nillability for `xsi:nil`, and the respective value-type checks for the
schema-location attributes.

Basis: [confirmed implementation limits][limits] and
[the pinned research registry][registry].

### DEF02-04

Do not accept every `xsi` value without checking it, and do not load schemas from
location hints.

Basis: [confirmed implementation limits][limits] and
[existing no-fetch behavior][existing-tests].

## 3 Numeric domains and Go representability

### DEF03-01

Remove the incorrect identification of unbounded XSD integer families with
`int64` or `uint64`.

Basis: [confirmed implementation limits][limits].

### DEF03-02

Define exact operations on integer and decimal values, implementation limits,
and resource budgets.

Basis: [existing scalar-helper reuse requirements][reuse] and
[the proposed value contracts][values]. Exact normative value spaces and chosen
implementation budgets are different kinds of obligation.

### DEF03-03

Distinguish an XSD violation, inability to complete assessment with the supported
capabilities, and inability to convert the input into the Go model.

Basis: [confirmed implementation limits][limits] and
[the proposed conversion contract][single-parse].

### DEF03-04

Cover machine-type boundaries, `staves=2^64` (`18446744073709551616`), lexical
forms, range restrictions, and exactness of comparisons. A valid source must not
be declared XSD-invalid solely because of its Go representation type.

Basis: [the documented integer-domain counterexample][limits].

## 4 Formalize requirements and build TDD coverage

Sections 4–10 describe implementation of the agreed architectural direction.
Their detailed contracts, profile choices and API decisions still require the
review and verification specified below.

### DEF04-01

Turn the research contracts into versioned executable predicates with a stable
runtime ID, revision, provenance, selector, scope, parameters, applicability, and
required adapter capabilities.

Basis: [the proposed catalog contracts][catalog] and
[the pinned research registry][registry]. These are formalization tasks, not a
claim that research IDs are already executable runtime IDs.

### DEF04-02

Record execution and reporting role separately from dependencies. A predicate,
context fact or advisory may also require an external dictionary, file, or
author intent.

Basis: [the proposed catalog and dependency contracts][catalog] and
[the candidate-role guidance][candidates].

### DEF04-03

Define shared-operator semantics, dependency order, and suppression of cascading
errors. Do not freeze the current illustrative names as a finished DSL without
designing it.

Basis: [the proposed catalog and operator contracts][catalog].

### DEF04-04

Split full and partial XSD/prose overlaps into shared conditions and additional
conditions. Do not emit two diagnostics for the same violation or treat every
reference as proof of equivalence.

Basis: [the proposed catalog contracts][catalog] and
[the pinned reconciliation evidence][reconciliation].

### DEF04-05

Maintain a matrix from source to condition to predicate to source/model target,
then to positive, negative and boundary tests. For a condition that cannot be
checked, state the reason and the criterion for closing that gap.

Basis: [the proposed catalog contracts][catalog] and
[the source/model evidence limits][model-evidence].

### DEF04-06

Verify reusable matchers, scalar/facet helpers, schemas and the ID index against
their contracts: sequence/choice, nullable branches and repetition, union-member
order, whitespace, patterns, value spaces, defaults/fixed values, and reference
scopes.

Basis: [existing reuse and adaptation contracts][reuse].

The 2,560 research records do not equal 2,560 independent rules. For a claimed
profile, every mandatory applicable obligation must have a check or an explicitly
stated assessment limit.

## 5 Implement two adapters and one shared engine

### DEF05-01

Add opt-in strict source validation while retaining permissive `Decode` by default.

Basis: [the proposed entry-point contracts][entry-points].

### DEF05-02

Parse XML once. Source validation must observe facts before information loss and
namespace filtering, including skipped subtrees. The combined source-validation
and decode operation can complete only after EOF, document-wide checks, and
successful model construction.

Basis: [the proposed single-parse and conversion contract][single-parse]. This
condition concerns the combined decode operation; it does not require a separate
source-only inspection entry point to build a Go model.

### DEF05-03

Replace the present `Validate` route through Encode → parse with direct traversal
of the Go model. Reuse verified shared contracts rather than creating a second,
independent XSD implementation.

Basis: [existing reuse and adaptation contracts][reuse] and
[the direct-model proposal][direct-model].

### DEF05-04

Map capabilities of concrete fields: presence, branch, order, exact value and
lexeme. Account for information lost by required scalar fields, repeated elements
collapsed into singular fields, `float64`, and integer-range limits.

Basis: [the field-level model evidence map][model-evidence].

### DEF05-05

Check current model invariants, cycles and depth without attributing the original
XML's history to the model. A representable absence of a required field remains
a normative violation.

Basis: [the model evidence map][model-evidence] and
[the direct-model proposal][direct-model]. Model graph invariants and normative
required-field constraints must keep their respective authorities.

## 6 Approve the first profile and add context in stages

### DEF06-01

Review all 29 candidate prose records; their final allocation is not yet approved.
The reviewer's proposed list had 28: `bend-release-negative` already exists in the
catalog and needs inclusion in the plan, not a duplicate record.

Basis: [the complete candidate inventory and its scope][candidates] and
[the pinned catalog][registry-catalog].

### DEF06-02

Start with local relations and typed IDREFs: accordion, beam number, key-octave
cancel, concert-score/for-part, and checks of the target's kind and scope.

Basis: [the proposed first implementation candidates][candidates].

### DEF06-03

Define part/staff/voice scope and document indexes. For `instrument-link`, separate
the local IDREF check from checking the existence and content of an external part
file; the latter requires a resolver.

Basis: [the local and linked-file candidate contracts][candidates] and
[the proposed bounded resolver][external].

### DEF06-04

For the selected profile, add musical-time and effective-state services:
chord/base-note relation, divisions, backup/forward, effective key, and repeat/span
context. Do not invent initial C major, 4/4, a clef, or `divisions=1`.

Basis: [the proposed context dependencies][catalog] and
[the initial-effective-context question][initial-context].

### DEF06-05

Dynamically distinguish `pass`, `violation`, `not-applicable`, `unknown`,
`unsupported` and `blocked`. Unknown intent for `written-transposition` does not
mean inapplicability. A deferred rule need not be unsupported on every document.

Basis: [the candidate applicability guidance][candidates] and
[the proposed result contracts][results].

### DEF06-06

State source/model/subtree/package profile boundaries and completeness explicitly,
including version selection.

Basis: [the proposed profile and catalog contracts][catalog] and
[entry-point scope][entry-points].

## 7 Package context and external normative dependencies

### DEF07-01

Create separate package/link adapters for MXL and opus, reusing existing path,
`mimetype`, container/rootfile and budget checks.

Basis: [existing reuse and adaptation contracts][reuse] and
[the proposed external-resolution boundary][external].

### DEF07-02

Define a resolver for authorized documents or documents supplied by the caller,
without automatic network traversal.

Basis: [the proposed explicit bounded resolver][external].

### DEF07-03

For capabilities actually claimed, pin and verify the SMuFL/IPA dictionaries,
ZIP/JAR/DEFLATE, and the relevant external XML/XSD/XLink/XML Base/ID contracts.

Basis: [the external-dependency questions][external-questions] and
[the proposed resolver boundary][external]. The required review is scoped to the
claimed capabilities, not an assertion that all external specifications have
already been audited.

### DEF07-04

Until a dependency is closed, report the assessment limit. Do not present prefix
checks or Unicode-block checks as complete dictionary validation.

Basis: [the SMuFL question][smufl-question],
[the IPA question][ipa-question], and
[the external-specification closure question][closure-question].

## 8 Diagnostics and API compatibility

### DEF08-01

Implement independent `conformance` and `assessment_complete` results. Incomplete
assessment must not appear to be successful validation merely because
`err == nil`.

Basis: [the proposed result and error contracts][results].

### DEF08-02

Include a stable ID/revision, target/path, reason, expected/actual values, related
locations, a short explanation, and a source URL, available without network access.

Basis: [the proposed diagnostic contracts][results].

### DEF08-03

Distinguish fatal XML errors, MusicXML violations, model invariants, unsupported
capabilities, and resource/security limits. Report recommendations separately.

Basis: [the proposed diagnostic categories][results] and
[the existing error-compatibility proposal][error-compatibility].

### DEF08-04

Approve and test compatibility for sentinel errors, `errors.Is`/`errors.As`, and
the existing `ValidationError`. The proposal to retain legacy error recognition
is not yet a final API.

Basis: [the error-compatibility proposal][error-compatibility] and
[the proposed error families and legacy wrappers][legacy-wrappers].

### DEF08-05

Bound diagnostic volume and value sizes, support localization, and do not invent
source line numbers for newly constructed models.

Basis: [the proposed diagnostic contracts][results] and
[the model evidence limits][model-evidence].

## 9 Security and resource budgets

### DEF09-01

Preserve the absence of automatic loading of schemas, DTDs, entities and XInclude.
Explicitly define support for valid XML features that are not implemented.

Basis: [existing security behavior][existing-tests] and
[the proposed bounded external-resolution contract][external].

### DEF09-02

In addition to existing depth/MXL limits, add budgets for input and decompressed
bytes, tokens/attributes/events, scalar length, the ID index, matching complexity,
accumulated context, linked documents, number of diagnostics, and model-traversal
volume.

Basis: [existing security behavior][existing-tests] and
[the proposed budget contracts][external]. These are implementation resource
policies, not extra MusicXML rejection conditions.

### DEF09-03

Add cancellation. Exhausting a budget means incomplete assessment, not a musical
violation.

Basis: [the proposed cancellation and entry-point contracts][entry-points] and
[the result categories][results].

## 10 Implementation verification and readiness

### DEF10-01

For every change, pass the relevant unit/regression tests and standard CI:
generation, formatting, vet, tests on three operating systems, race, fuzz, and
external XSD conformance.

Basis: [existing test behavior][existing-tests] and
[the recorded baseline CI run][ci]. This is a future per-change verification
obligation, not evidence that the planned implementation tests already exist.

### DEF10-02

Add differential checks of the original XML facts, not only
Decode → Encode → xmllint. Use independent reference implementations or other
independent oracles where applicable.

Basis: [the documented limitations of current corpus testing][existing-tests].

### DEF10-03

Test grammar branches and repetitions, domain boundaries, presence/default/fixed
behavior, contextual scopes, namespace cases, incomplete outcomes, and diagnostic
self-sufficiency.

Basis: [existing reusable contracts][reuse],
[model evidence limits][model-evidence], and
[the proposed result contracts][results].

### DEF10-04

Use corpus runs as supplementary regression evidence. Determine readiness from
the selected profile's obligations and the coverage matrix rather than replacing
that criterion with a count of processed files.

Basis: [the documented corpus-testing boundary][existing-tests] and
[the bounded first-implementation proposal][first-seam].

## 11 API and rule-delivery decisions

### DEF11-01

Concrete Go types, package placement without dependency cycles, operator and
compilation formats, freezing runtime IDs, error migration, and first-profile
boundaries remain open. Choose how to obtain a compact runtime catalog from the
pinned external registry, how to version it, and how to verify pin updates.
Automatic synchronization is not implemented and must not be assumed.

Basis: [the Go contract proposal][contracts],
[its catalog and operator contracts][catalog], and
[the pinned external registry][registry]. These are project implementation
and delivery decisions, not additional MusicXML clauses. Proposed signatures
and planning identifiers do not settle the final public API.

## 12 Dispositions for all 28 open questions

### DEF12-01

The [open-question registry][questions] contains research proposals, not 28
completed decisions and not 28 mandatory blockers to starting implementation.
The disposition work covers every stable question in that registry:

1. [MX40-ISSUE-beam-six-eight][issue-beam-six-eight]
2. [MX40-ISSUE-cue-duration][issue-cue-duration]
3. [MX40-ISSUE-container-schema-exists][issue-container-schema-exists]
4. [MX40-ISSUE-container-sounds-wording][issue-container-sounds-wording]
5. [MX40-ISSUE-container-schema-uri][issue-container-schema-uri]
6. [MX40-ISSUE-xml-declaration-required][xml-declaration]
7. [MX40-ISSUE-every-note-duration][issue-every-note-duration]
8. [MX40-ISSUE-duration-integer][issue-duration-integer]
9. [MX40-ISSUE-old-note-type-list][issue-old-note-type-list]
10. [MX40-ISSUE-numeral-step-typo][issue-numeral-step-typo]
11. [MX40-ISSUE-assess-type-default][issue-assess-type-default]
12. [MX40-ISSUE-midi-instrument-parent-wording][issue-midi-instrument-parent-wording]
13. [MX40-ISSUE-legacy-tutorial-header-root-list][issue-legacy-tutorial-header-root-list]
14. [MX40-ISSUE-pull-off-attribute-descriptions-swapped][issue-pull-off-attribute-descriptions-swapped]
15. [MX40-ISSUE-pizzicato-nonexistent-element][issue-pizzicato-nonexistent-element]
16. [MX40-ISSUE-time-beat-spelling][issue-time-beat-spelling]
17. [MX40-ISSUE-part-id-uniqueness-scope][issue-part-id-uniqueness-scope]
18. [MX40-ISSUE-left-barline-attributes][issue-left-barline-attributes]
19. [MX40-ISSUE-chord-preceding-duration][issue-chord-preceding-duration]
20. [MX40-ISSUE-initial-effective-context][initial-context]
21. [MX40-ISSUE-span-pairing-scope][issue-span-pairing-scope]
22. [MX40-ISSUE-measure-duration-completeness][issue-measure-duration-completeness]
23. [MX40-ISSUE-zero-counts-and-ratios][issue-zero-counts-and-ratios]
24. [MX40-ISSUE-mxl-methods-compatibility][issue-mxl-methods-compatibility]
25. [MX40-ISSUE-mxl-first-media-type-set][issue-mxl-first-media-type-set]
26. [MX40-ISSUE-smufl-version-dictionary][smufl-question]
27. [MX40-ISSUE-ipa-character-repertoire][ipa-question]
28. [MX40-ISSUE-external-spec-closure][closure-question]

This list fixes the scope of the disposition obligation; it does not approve
any linked proposed disposition.

### DEF12-02

Before implementing an affected disputed predicate, record its clause-level
interpretation, scope, evidence and disposition. Principal unresolved areas are
left barline, chord duration, part-uniqueness scope, initial effective context,
span pairing, measure completeness, zero counts/ratios, MXL methods/media types,
SMuFL, IPA, and transitive external closure. Handle editorial discrepancies using
exact XSD contracts and evidence. Do not turn a disputed interpretation or the
issue itself into a basis for hard rejection.

Basis: [the full open-question guide and its clause-level sources][questions]
and [the pinned issue registry][registry-issues]. Clear formal constraints retain
their own evidence; an issue on a shared page does not automatically dispute every
clause on that page.

## 13 Remaining English explanations

### DEF13-01

Translate the remaining **291 prose records** as predicates and context facts
are formalized, preserving the Russian original, ID and provenance. Refine
translation priority together with profile selection; no date for completing
all translations has been assigned.

Basis: [the pinned source catalog][registry-catalog]. The obligation concerns the
remaining research-field translations, not retranslating the completed English
fields for 51 constraints, 28 questions and six examples. The plan's English
role/fact summaries do not establish completion of these translations.

## 14 Optional separate opus replacement

### DEF14-01

Consider a small separate PR using byte-for-byte upstream `opus.xsd`, preserving
attribution, comparing operative trees, and providing reproducible generation and
tests. The locator explanation already added is sufficient for the documentation
PR. The schema replacement has not been performed.

Basis: [the pinned research registry][registry] and
[the existing schema reuse boundary][reuse]. This remains an optional independent
proposal; it is not a prerequisite for the validation architecture.

## 15 Optional short quotations and additional evidence

### DEF15-01

Decide whether short quotations are needed for candidates and diagnostics,
including their size, attribution, licensing and inclusion in the runtime catalog.
A quotation supplements a comprehensible error explanation; it does not replace
one.

Basis: [the proposed diagnostic contracts][results] and
[the pinned source provenance][registry]. Quotations remain optional.

### DEF15-02

An optional source-evidence snapshot alongside the model is likewise not required
for the architecture. If needed, define invalidation after editing and prohibit
using a stale snapshot as current facts.

Basis: [the source/model evidence boundary][model-evidence] and
[the direct-model proposal][direct-model]. This conditional proposal does not
require snapshot storage or permit reconstruction of discarded source history.

## Suggested order

1. Prepare TDD reproductions and close the three concrete defect blocks.
2. Formalize contracts, capability mapping, the first profile and the coverage
   matrix.
3. Implement shared components, the single-pass strict source adapter and the
   direct model adapter.
4. Expand the profile with context, packages and dictionaries while retaining
   honest outcomes and readiness criteria.
5. Translate alongside rule work; decide optional changes separately.

This is a suggested ordering of the preserved scope. It does not assign dates,
create public issues or new code PRs, or start execution tasks.

[pr11]: https://github.com/go-muse/go-musicxml/tree/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation
[ci]: https://github.com/go-muse/go-musicxml/actions/runs/37758208909
[plan]: plan/implementation-plan.json
[snapshot]: plan/external-snapshot.json
[registry]: https://github.com/go-muse/go-musicxml-registry/tree/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c
[registry-catalog]: https://github.com/go-muse/go-musicxml-registry/blob/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/catalog.json
[registry-issues]: https://github.com/go-muse/go-musicxml-registry/blob/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/issues.json
[reconciliation]: https://github.com/go-muse/go-musicxml-registry/blob/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/reconciliation.json
[contracts]: https://github.com/go-muse/go-musicxml/blob/ba32b92300a54485f000105615fbe4af10325b8e/docs/validation/go-contracts.md
[entry-points]: https://github.com/go-muse/go-musicxml/blob/ba32b92300a54485f000105615fbe4af10325b8e/docs/validation/go-contracts.md#2-proposed-entry-points-and-return-rules
[values]: https://github.com/go-muse/go-musicxml/blob/ba32b92300a54485f000105615fbe4af10325b8e/docs/validation/go-contracts.md#3-shared-types-identity-evidence-and-values
[single-parse]: https://github.com/go-muse/go-musicxml/blob/ba32b92300a54485f000105615fbe4af10325b8e/docs/validation/go-contracts.md#single-xml-parse-and-conversion-failure
[direct-model]: https://github.com/go-muse/go-musicxml/blob/ba32b92300a54485f000105615fbe4af10325b8e/docs/validation/go-contracts.md#direct-model-mapping-and-honest-applicability
[catalog]: https://github.com/go-muse/go-musicxml/blob/ba32b92300a54485f000105615fbe4af10325b8e/docs/validation/go-contracts.md#5-catalog-operators-and-dependencies
[external]: https://github.com/go-muse/go-musicxml/blob/ba32b92300a54485f000105615fbe4af10325b8e/docs/validation/go-contracts.md#6-external-resolution-is-explicit-bounded-and-partial
[results]: https://github.com/go-muse/go-musicxml/blob/ba32b92300a54485f000105615fbe4af10325b8e/docs/validation/go-contracts.md#7-results-diagnostics-and-error-compatibility
[legacy-wrappers]: https://github.com/go-muse/go-musicxml/blob/ba32b92300a54485f000105615fbe4af10325b8e/docs/validation/go-contracts.md#error-families-and-legacy-wrappers
[first-seam]: https://github.com/go-muse/go-musicxml/blob/ba32b92300a54485f000105615fbe4af10325b8e/docs/validation/go-contracts.md#8-existing-code-reuse-and-first-implementation-seam
[limits]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/existing-code-map.md#confirmed-limits-of-the-current-implementation
[reuse]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/existing-code-map.md#reuse-and-adaptation
[existing-tests]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/existing-code-map.md#existing-tests-and-security-behavior
[model-evidence]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/existing-code-map.md#model-evidence-that-can-and-cannot-be-supplied
[candidates]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/existing-code-map.md#first-implementation-candidates
[error-compatibility]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/existing-code-map.md#error-compatibility-proposal
[questions]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md
[external-questions]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#package-and-external-dependency-questions
[issue-beam-six-eight]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-beam-six-eight
[issue-cue-duration]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-cue-duration
[issue-container-schema-exists]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-container-schema-exists
[issue-container-sounds-wording]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-container-sounds-wording
[issue-container-schema-uri]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-container-schema-uri
[xml-declaration]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-xml-declaration-required
[issue-every-note-duration]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-every-note-duration
[issue-duration-integer]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-duration-integer
[issue-old-note-type-list]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-old-note-type-list
[issue-numeral-step-typo]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-numeral-step-typo
[issue-assess-type-default]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-assess-type-default
[issue-midi-instrument-parent-wording]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-midi-instrument-parent-wording
[issue-legacy-tutorial-header-root-list]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-legacy-tutorial-header-root-list
[issue-pull-off-attribute-descriptions-swapped]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-pull-off-attribute-descriptions-swapped
[issue-pizzicato-nonexistent-element]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-pizzicato-nonexistent-element
[issue-time-beat-spelling]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-time-beat-spelling
[issue-part-id-uniqueness-scope]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-part-id-uniqueness-scope
[issue-left-barline-attributes]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-left-barline-attributes
[issue-chord-preceding-duration]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-chord-preceding-duration
[initial-context]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-initial-effective-context
[issue-span-pairing-scope]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-span-pairing-scope
[issue-measure-duration-completeness]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-measure-duration-completeness
[issue-zero-counts-and-ratios]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-zero-counts-and-ratios
[issue-mxl-methods-compatibility]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-mxl-methods-compatibility
[issue-mxl-first-media-type-set]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-mxl-first-media-type-set
[smufl-question]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-smufl-version-dictionary
[ipa-question]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-ipa-character-repertoire
[closure-question]: https://github.com/go-muse/go-musicxml/blob/f1f77ae2820a4ce1d97d617c315b4a6d6a08903b/docs/validation/open-questions.md#mx40-issue-external-spec-closure
