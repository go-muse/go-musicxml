# MusicXML 4.0 validation architecture proposal

Status: **PROPOSED**. Research snapshot: 8 October 2026.

This document proposes an architecture for validating source MusicXML and the Go model against a versioned body of requirements. It records the outcome of the research and architecture phase. It does not implement a validator, change Go packages, add product tests, freeze a public API, or select a final package layout. Names such as `RuleDefinition`, `conformance`, and `assessment_complete` describe proposed responsibilities and data, not committed Go declarations.

## Decisions proposed now and work deferred

Proposed decisions:

- Keep mandatory XML well-formedness checks in one parse; source MusicXML validation is opt-in.
- Use a shared versioned rule catalog and engine with overlapping source/model applicability.
- Validate current Go objects directly, without Encode → parse; report unavailable evidence explicitly.
- Include human-readable explanations and source URLs in offline diagnostics.
- Reuse existing schema, grammar, scalar, and identity components only after checking their contracts; preserve no-network and resource boundaries.

Deferred to implementation design and review:

- Public Go API, package placement, frozen rule IDs, exact operator syntax, and the sentinel-error migration policy.
- Concrete model capability mapping, complete predicate/test coverage, and contextual or external-dependency closure for each claimed profile.
- Staged musical-time and package/link support; prerequisites of selected rules still have to be satisfied.
- Future registry revision selection and generation workflows. The current research evidence is pinned in the separate registry repository; this library keeps package-facing design documents.

The [existing-code map](existing-code-map.md) identifies reuse candidates, verified gaps, model limits, error compatibility, and a bounded candidate backlog.

## Recommended design

Use one versioned, declarative requirements catalog and one validation engine, with two fact adapters: source XML and the Go model. XML syntax validation remains a mandatory part of a single parse. Strict validation of source MusicXML is enabled by a separate flag. Validation of a created or edited model is invoked independently and never serializes the model back to XML.

Source and model applicability overlap. They are not two disjoint filters applied in sequence: octave ranges, permitted combinations of child objects, ID references, and many prose requirements apply to both representations. An invalid numeric spelling, discarded order, or ignored attribute cannot be recovered from a normalized model.

The research registries provide a sufficient foundation for this architecture. They are not an executable validator. Predicates still need to be formalized, the fact mapping for the actual Go model needs to be confirmed, and normative external dependencies need to be closed for each claimed profile.

## Evidence and completeness boundary

The research uses the dated [MusicXML 4.0 publication of 1 June 2021](https://www.w3.org/2021/06/musicxml40/) and release commit `799e2defb2ece0ae7bafe08dcbcac25b2c631d53`. This is a Final Community Group Report, not a W3C Recommendation. The current editor's draft does not replace the pinned 4.0 publication.

| Reconciled material | Result |
| --- | --- |
| Six official XSD files | All 4,223 nodes accounted for: 2,939 operative, 642 annotation, and 642 documentation nodes |
| Explicit XSD properties and references | 3,958 properties; 1,515 QName tokens resolved |
| XSD contracts | 2,218 records: 2,171 schema contracts, 30 XSD 1.0 semantic topics, and 17 built-in types |
| Types and document roots | 155 simple types, 242 complex types, 48 attribute groups, 27 model groups, and 5 roots |
| Prose in the publication | All 642 documentation blocks and 37 comments, plus the entire 974-page HTML corpus, accounted for with explicit dispositions |
| Prose registry | 342 records: 51 constraints, 58 recommendations, 143 interpretations, 73 defaults, 14 application-dependent records, and 3 descriptive records |
| Prose overlap with XSD | 19 full overlaps, 43 partial overlaps, and 280 with no overlap; all cited XSD IDs resolve |
| Issue register | 28 editorial contradictions, ambiguities, and external dependencies |

Of the 974 HTML pages, 260 were obtained directly from the dated site and 714 from the official archive at the same commit. The main text matched for all 260 overlapping pages. Every page has an explicit disposition: 624 reference pages were read as text, 29 narrative or navigation pages were read, 306 example pages were checked as illustrations, and 10 non-narrative listings and 5 historical pages were explicitly excluded. There are no unread pages within this corpus. Images and XML examples are not treated as proof of a universal normative rule.

The operative trees of all six XSD files were confirmed to match the dated W3C listings. Five raw hashes also matched the earlier pinned audit; this does not claim a new byte-for-byte Git audit of all six files. The annotation comparison for `musicxml.xsd` against the dated listing matched 613 of 613 annotations. The exact provenance is retained in the supporting research registries.

This is completeness of accounting for and reading the published MusicXML 4.0 sources within the stated boundary. It is not a complete transitive audit of XML, XSD, XLink, ZIP/JAR, SMuFL, or IPA; proof that no semantic error remains; or implementation or test coverage. A previous large corpus run, regardless of the number of documents processed, does not replace rule coverage.

The combined catalog contains **2,218 XSD contracts plus 342 prose records, totaling 2,560 research records**. That is not a count of independent atomic mandatory rules or diagnostics. Composite contracts reuse the same constraints, and most prose records describe semantics, defaults, or recommendations. The 974-page and 642-annotation corpus is scoped complete; closure of external normative dependencies remains incomplete, with 28 tracked issues.

## Three logical layers

| Layer | Input | Contract |
| --- | --- | --- |
| XML syntax | Source document bytes | Always checks well-formedness and namespaces during parsing; a fatal error prevents further reliable processing |
| Strict source MusicXML | Unfiltered XML facts and document context | Optionally checks XSD and applicable, confirmed mandatory prose requirements of the pinned profile |
| Go model | Current object state | Checks applicable requirements using available facts and separately reports model-contract violations; does not assert validity of the original XML |

Ordinary `Decode` should retain its permissive default behavior. Turning off the strict flag must not disable XML syntax checks, namespace checks, or protective limits. Separate model validation is useful after creating or editing a model and after permissive `Decode`; it does not require source validation first.

Strict `Decode` must construct the model and validate the source together. Source facts are checked before information is lost, including simple attributes, unknown elements, and foreign namespaces. A strict decode can succeed only after EOF, completion of document-wide checks, and successful conversion to the model. If XML conforms to the standard but an exact number cannot be represented by the current Go model, that is a model-representability error, not a fabricated XSD violation. A schema-valid but unrepresentable document must remain distinguishable from a schema-invalid one. Source conformance can be assessed with exact numeric values within explicit resource budgets even if Go-model conversion fails. If an implementation cannot assess the required value space, report that capability limit and incomplete assessment; exceeding a Go type's range is not evidence of an XSD violation.

Results should expose independent notions of `conformance` and `assessment_complete`. Finding no violation when a mandatory check is unfinished or unsupported does not mean “valid.” Success always refers to a stated version, profile, and target.

### One XML parse

The processing responsibilities are:

1. Read input while enforcing limits.
2. Parse XML syntax and namespaces.
3. Observe unfiltered source facts.
4. Feed a source-validation session when strict mode is enabled.
5. Perform the ordinary mapping into the model.

This describes responsibilities, not a required sequence of particular Go wrappers. No stage rereads XML “first for errors, then for the model.” Even a subtree skipped by the decoder must undergo syntax checks, limits, and source observation. Namespace expansion happens exactly once. Checking trailing content after the root and reaching EOF also belong to that parse.

Local rules execute on events. Document indexes are finalized at end-of-document. Musical-time analysis may buffer the necessary measure or part facts and make passes over those facts; this is not a second XML parse. A single parse does not imply constant memory or a single pass over every derived fact.

### Model validation without Encode

The model adapter traverses objects and emits typed facts directly. `Encode` → XML text → parse is prohibited as a model-validation strategy. A separate check of whether a model is exportable may use the same adapter contract without creating an XML document.

The adapter must explicitly describe what is actually preserved: presence, selected branch, order, expanded names, exact values, and original lexical forms. If the model stores repeated alternatives in unrelated slices, their original cross-type order cannot be recovered. Validation may check the current model structure and the export order it defines; it cannot thereby declare the original source order correct.

A source-evidence snapshot from permissive `Decode` could optionally be stored alongside a model, but edits make it stale. It must not silently participate in validation of the new state as current evidence. This optional capability is not required by the proposed architecture.

## Catalog and rule granularity

Distinguish three entities: a source, a normative contract, and an executable predicate. An XSD occurrence, a prose entry, and a runtime error do not have a one-to-one relationship.

Research IDs are already stable within the pinned snapshot: `MX40-XSD-*` identifies occurrences, `@property` identifies their properties, `MX40-REQ-*` identifies contracts, `MX40-PROSE-*` identifies semantic records, and `MX40-ISSUE-*` identifies issues. Preserve them as provenance IDs. Future diagnostics need separate, stable semantic rule IDs: moving a line or changing a generator must not change the code for the same error. A semantic change requires a new revision or a new ID with an explicit migration.

### An executable rule definition

A future `RuleDefinition` should describe:

- A stable ID and revision; the MusicXML version, profile, and source-set fingerprint.
- A localizable short title, explanation, and message template. The research registry retains its original bilingual fields unchanged.
- A normative class: mandatory, recommendation, interpretation, default, or application-dependent; technical or model-policy restrictions need a separate classification.
- Provenance: XSD, prose, or both; every source ID; verified URLs and anchors, or XPath and section locators.
- Source and model target applicability, preconditions, and required adapter capabilities.
- A selector and scope: element, sibling group, measure, part, document, package, or external dictionary.
- A declarative predicate with parameters, or a reference to a shared contextual operator.
- Dependencies on other facts and rules, the outcome when context is unavailable, and reporting policy.
- An issue relationship only where the issue genuinely affects that requirement.

An expression or predicate must not be arbitrary executable code taken from a source. Prefer a small set of inspectable operators: presence/cardinality, scalar domain, sequence/choice grammar, conditional implication, uniqueness/reference, and relations over context. When necessary, add a new shared operator once with defined semantics. This does not yet select Go structures, DSL syntax, or a generator.

Execution/reporting role and required dependencies are separate axes. A record can define a mandatory predicate, provide a context fact, or support an advisory, while independently requiring an external dictionary, linked document, or caller-supplied intent. "Needs external facts" is a capability/dependency statement, not a mutually exclusive fourth role and not a fixed rule outcome. Record both axes explicitly during formalization.

Interpretations and defaults produce facts for mandatory checks; they do not automatically produce errors. For example, the default beam number of `1` participates in checking that one note's beam numbers are distinct. An inactive attribute that the specification says to ignore does not become an error merely because it is present.

### Avoiding duplicate validation

Define XSD types, groups, and facets once and reference them from declarations. A complete prose restatement adds explanation and provenance to an existing condition without creating a second failing predicate. Split a partial overlap into its formal component and its additional semantic condition. For example, generic IDREF validation checks that an ID exists; prose separately checks the target's type and part.

Do not automatically merge every record connected through `related_xsd_requirement_ids`. Some links refer to an entire complex contract rather than an exactly equivalent component. The combined registry therefore preserves original IDs and explicit full/partial/context relationships. Before formalization, preserve evidence rather than inventing equivalence. The 19 full-overlap prose records must not create a second error for the same formal requirement; their additional semantics and defaults remain available.

The prose registry's `related_issue_ids` can denote shared-source context. They do not automatically disable every rule in a paragraph. For example, an editorial error about the container schema does not block a clear prohibition on compressing the `mimetype` entry.

### Required operator families

| Family | Essential requirements |
| --- | --- |
| Scalars and facets | Lexical and value spaces, whitespace, exact numbers, typed enumeration and fixed values, XSD patterns, and restriction layers |
| Union | Alternative order and member-specific whitespace; do not normalize the whole union in advance using one policy |
| Content grammar | Nested sequence/choice, nullable branches, and bounds on the whole particle; independent child counters are insufficient |
| Attributes | Permitted expanded names, required attributes, explicit presence, default/fixed values, and simple scalar-attribute validation |
| Identity | Document-wide ID uniqueness and IDREF resolution, followed by typed references and prose-defined scope |
| Local prose logic | At least one accordion register; second Bézier controls only for slur continuation; distinct beam numbers |
| Context | `concert-score`/`for-part`/`transpose`, effective key, event order and durations, and part/staff/voice scope |
| External dependencies | Pinned SMuFL/IPA dictionaries, package metadata, and linked documents; missing data produces unknown/unsupported outcomes |

The six XSD files contain no `xs:key`, `xs:keyref`, or `xs:unique`; ID/IDREF requirements still apply. There are no `xs:any` or `xs:anyAttribute` wildcards, so imports do not authorize arbitrary foreign nodes. There is no `xs:list`, but the standard `xsi:schemaLocation` attribute depends on a URI list. The catalog remains closed over the selected schema assemblies: identical no-namespace type names in musicxml, opus, and sounds do not identify one shared component.

Implement XML and XSD semantics against pinned specifications, not a superficially similar Go parser or regular-expression API. In particular, [XSD 1.0 Datatypes](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/) distinguishes lexical and value spaces, facet layers, and union-member order.

### Illustrative operator names

The six [rule examples](https://github.com/go-muse/go-musicxml-registry/blob/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/rule-definition-examples.json) use these names. This mapping explains their relationship to the families above; it does not define a finished DSL or approve executable implementations.

| Example name | Family and intended role |
| --- | --- |
| `scalar_domain` | Scalars and facets: the selected exact value domain and source lexical check |
| `count_union_at_least` | Local prose logic: presence/cardinality across a set of child names |
| `all_distinct` | Identity/local relations: uniqueness of selected values within the declared scope |
| `effective_attribute` | Attribute/default fact used by another predicate; not an independent violation |
| `reference_target_kind` | Identity/context: an existing reference must identify the required target kind and scope |
| `ascending_order` | Local relation over a typed sequence; equal adjacent values retain the example's unresolved policy |
| `typed_comma_separated_positive_integers` | Scalar parsing fact needed before the order relation; not proof of full rule success |
| `standard_xsi_attribute` | Standard schema-instance attribute contract, including attribute-specific semantics |

## Context and assessment reliability

Source facts retain the lexical form needed after mandatory XML normalization, as well as names, presence, and occurrence order. Raw bytes are necessary only for requirements that actually depend on them, such as a BOM in the contents of `mimetype`. Model facts contain the current value and information about its representation. Absence and a default value remain distinct; calculating an effective value for analysis does not mutate the model or source.

Context services in the engine must be explicit: a document ID index, typed part/instrument/player indexes, part/measure timelines, effective attribute state, linked-document/package indexes, and external versioned dictionaries. Every check need not have all of these services. Source and model adapters may supply the same contextual facts in different ways.

For `MX40-PROSE-instrument-link-target`, a local sub-contract may check the IDREF target's kind and scope using available document facts. Establishing the external part file's existence and content requires a separate resolver and a declared linked-document scope. A local success is only a partial result and must not be reported as a pass for the whole requirement while those external obligations remain unresolved. The resolver does not imply automatic network access; it can receive authorized or caller-supplied documents. If the rule is demonstrably inapplicable, report that separately from missing resolution capability.

Musical order is not XML order. `backup`, `forward`, `chord`, and changes to `divisions` require a defined temporal model. Numeric and temporal comparisons must not silently round through `float64` if that changes the normative conclusion. Insufficient representation requires an explicit precision or context limitation. Do not invent initial C major, 4/4, treble clef, or `divisions=1`.

A rule's outcome is one of: pass, violation, not-applicable, unknown, unsupported, or blocked-by-invalid-prerequisite. An unknown precondition is not false. `not-applicable` is justified only when applicability is known to be false for the assessed document and scope. Missing facts for a possibly applicable obligation produce `unknown`, or `unsupported` when the implementation lacks the necessary capability; neither means `pass`. Outcomes are determined for each document, scope, and available facts. Do not assign a static outcome to a set of rules merely because those rules can depend on external context. If a scalar is already invalid, dependent musical calculations must not generate cascades of invented errors. Independent checks continue while structure and resource budgets permit.

Document-scoped requirements may be unavailable when validating only a subtree. Such validation must report its scope and missing context rather than promise whole-document validity. A cyclic Go object graph, an invalid internal representation such as a nil slice element, or numeric unrepresentability belongs to the model contract. If nil means a required XML element or attribute is absent, however, the required/minOccurs violation remains a normative XSD defect on the model target; it must not be hidden among internal errors.

## Self-contained diagnostics

A diagnostic should carry the rule ID and revision, target, structural path, specific cause, expected and actual values, and related locations. The human-readable formatter automatically adds a short explanation and source from the embedded catalog. Users must not have to look up an ID in source code to understand an error.

Illustrative future output:

> /score-partwise/part[1]/measure[3]/direction[1]/direction-type[1]/accordion-registration: no register was specified. Expected at least one of accordion-high, accordion-middle, or accordion-low; found 0. MusicXML requires at least one of these child elements, although the XSD permits all of them to be absent. Source: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L3290 . Rule: MX40-PROSE-accordion-at-least-one.

This illustrates a format; it is not a validator-run result or a frozen runtime rule ID. Duplicate ID or beam-number diagnostics should also point to the first conflicting node. Expected and actual values need safe length limits so that a huge attribute string cannot inflate the report without bound.

A location contains a target kind and path. Byte offsets, lines, and columns are included only when available and correctly related to the source encoding; a newly constructed model has no invented source lines. MXL locations also identify the archive entry, while model locations may use a field/index path. Message text is localizable; IDs and structural paths remain programmatic data.

Short explanations, message templates, source URLs, and version information must be available offline at runtime. The full long-form prose catalog, editorial issue history, and raw evidence may be distributed separately. Each diagnostic can refer to one `RuleDefinition` without copying the entire rule text into every object. Ordinary formatted output must still show the explanation and URL automatically, even without access to project files or a network connection.

XML fatal errors, MusicXML violations, model invariants, unsupported capabilities, and resource/security limits must be distinguishable in both machine-readable results and human-readable text. Recommendation warnings are reported separately and do not make a normatively valid document invalid. IDs aid programs; they do not replace a human explanation.

## Namespaces versions and security boundaries

The profile has five no-namespace roots: `score-partwise`, `score-timewise`, `opus`, `container`, and `sounds`. A container in a different namespace is a different compatibility profile. Strict source validation must see the original expanded names before permissive namespace filtering. `xml:*` and `xlink:*` are allowed only where the specific contract permits them; importing their schemas does not authorize the whole attribute family everywhere.

The four standard xsi attributes have their own semantics: `xsi:type`, `xsi:nil`, `xsi:schemaLocation`, and `xsi:noNamespaceSchemaLocation`. They must not be blanket-rejected as unknown MusicXML attributes. `xsi:type` requires QName handling and valid derivation; `xsi:nil` depends on nillability; location attributes are hints. None authorizes replacing the pinned schema or automatically retrieving schemas over the network.

XML syntax includes a single root, prolog grammar, correct nesting, characters, encodings, references, and attribute uniqueness. Namespaces additionally require correct bindings and uniqueness of expanded attribute names. [XML 1.0](https://www.w3.org/TR/REC-xml/#sec-prolog-dtd) and [Namespaces in XML](https://www.w3.org/TR/xml-names/#uniqAttrs) provide separate foundations for these checks. A DOCTYPE inside the root is invalid. A valid DOCTYPE in the prolog is not inherently a MusicXML error; the [official example](https://www.w3.org/2021/06/musicxml40/tutorial/hello-world/) includes one. An imprecise tutorial sentence does not justify requiring an XML declaration.

Network schemas, DTDs, entities, and XInclude must not activate automatically. The safe default is pinned local schemas, no external network retrieval, and explicitly defined DTD/entity support. If policy does not support a valid XML feature, report unsupported/security-policy rather than claiming that MusicXML forbids it.

Budgets are needed for input and decompressed bytes, depth, tokens/attributes/events, scalar length, ID-table size, content-matching complexity, buffered context, linked documents, and diagnostics, together with cancellation. XSD `unbounded` does not mean an unlimited application budget. Exceeding a limit means an incomplete assessment, not a musical-format violation. Model traversal must also limit depth and volume and detect cycles.

The strict profile is pinned to MusicXML 4.0; unknown extensions do not silently become valid. Ordinary permissive `Decode` remains the compatibility path. The `version` attribute has its own XSD domain and default; do not invent an enumeration containing only `4.0`. If the caller selects a profile from the declared version, an unknown version means an unsupported profile. If the caller explicitly validates against 4.0, the report identifies that schema. An absent `version` is not automatically equivalent to 4.0.

MXL ZIP packages and linked opus graphs require additional facts and scopes unavailable from an ordinary XML or model adapter. The shared catalog may include their requirements, but these three logical layers do not require turning XML validation into a ZIP validator. The relevant profile needs a separate package/link adapter using the same diagnostic mechanism. Full ZIP/JAR validation is not promised before the external audit is complete.

## Issue dispositions and the next phase

The 28 tracked issues do not require 28 user decisions before any implementation can begin. Retain three disposition strategies:

1. **Clear editorial error.** Apply the unambiguous XSD contract and supporting prose while preserving the conflict: eight beam levels, the actual note branches, the container schema, and `pull-off` types. Do not introduce elements from `beat`, `numeral-step`, or `pizzicato` typos.
2. **Ambiguous strengthening of a requirement.** Do not introduce hard errors for blanket balancing of every span, complete filling of every measure, pairwise-decreasing chord durations, or a rigid left-barline position without proof. In the `time-only` example, “ascending” has not been turned into an unproven prohibition on repeated numbers: decreasing values violate the order, while the policy for equal adjacent values remains open. A separately and explicitly named advisory or profile is possible.
3. **External dictionary or incomplete environment.** SMuFL prefix patterns can be checked now; canonical glyph existence requires a pinned dictionary. The IPA repertoire, complete ZIP semantics, and the disputed media-type set remain explicitly unresolved. Dependent outcomes must be unknown or unsupported, not pass.

The existing validator provides useful schema data, particle matching, scalar checks, and ID indexes. Reuse still needs contract-by-contract verification: confirmed gaps include attributes on simple-typed elements, duplicate attributes, DOCTYPE placement, xsi handling, and unbounded XSD integer value spaces. These are documented precisely in the [existing-code map](existing-code-map.md); they do not imply that ordinary complex-type attributes are unchecked. The proposed direct model adapter replaces the current Encode → parse route while allowing verified helpers to be reused.

Dependencies between the catalog, engine, and adapters must remain acyclic. The engine need not know public Go models or the XML decoder. A separate internal package is an option when needed to remove a cycle, not a requirement to reorganize the entire repository. Concrete placement is deferred until a bounded mapping review of the existing code.

After approval of the architecture, the next phase should first define runtime rule IDs, the actual model's capabilities, and executable predicates, then use the catalog to drive TDD. No product tests were written or run in this research phase. A future coverage ledger should connect:

- Every source to a disposition.
- Every mandatory requirement to a predicate.
- Every predicate to source/model applicability and positive, negative, and boundary cases.
- Every uncheckable rule to a reason and a closure criterion.

Grammar needs branch and repetition coverage; enumerations and facets need boundary coverage; defaults and fixed values need presence coverage; contextual requirements need scope coverage; and diagnostics need complete explanations and source URLs. Corpus runs supplement regression testing; they are not the denominator for this coverage.

Readiness must be defined per profile: every mandatory applicable requirement has a check or an explicit, supported limitation; unresolved obligations are not masked by success; violations are not duplicated; explanations are available offline; source XML is parsed once; and model validation does not use serialization.

## External research registry

Supporting research is stored in the separate
[go-musicxml-registry snapshot](https://github.com/go-muse/go-musicxml-registry/tree/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c),
pinned to `3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c`. The links below point to
that exact revision's `docs/validation/registry/` directory, not this library's tree:

- [`catalog.json`](https://github.com/go-muse/go-musicxml-registry/blob/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/catalog.json): the combined research catalog of 2,560 records, preserving IDs and original semantics.
- [`reconciliation.json`](https://github.com/go-muse/go-musicxml-registry/blob/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/reconciliation.json): checked XSD/prose relationships and a policy against duplicate validation without unproven merging.
- [`coverage-reconciliation.json`](https://github.com/go-muse/go-musicxml-registry/blob/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/coverage-reconciliation.json): final reconciliation of sources, references, and completeness boundaries.
- [`rule-definition-examples.json`](https://github.com/go-muse/go-musicxml-registry/blob/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/rule-definition-examples.json): six illustrative declarative rules; not code or a complete runtime catalog.
- [`issues.json`](https://github.com/go-muse/go-musicxml-registry/blob/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/issues.json): 28 issues with sources and recommended dispositions.
- [`xsd/`](https://github.com/go-muse/go-musicxml-registry/tree/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/xsd/) and [`prose/`](https://github.com/go-muse/go-musicxml-registry/tree/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/prose/): supporting source and coverage registries and provenance, without a raw HTML cache.
- [`manifest.json`](https://github.com/go-muse/go-musicxml-registry/blob/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c/docs/validation/registry/manifest.json): the supporting registry inventory and file SHA-256 hashes.

The supporting data preserves all original fields, including Russian research paraphrases and source locators. Additive English fields cover the 51 constraint records, 28 issues, and six illustrative rules. The remaining 291 prose records retain their original descriptions pending translation; no translation replaces the original evidence. Research IDs, evidence relations, issue dispositions, and examples are preserved as research data; they do not freeze executable predicates or a public validation API.

These registries are an auditable foundation for the next phase. Their research counts do not claim that a future validator is ready.
