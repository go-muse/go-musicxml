# XSD 1.0 prerequisite scope and reuse ledger

This is the bounded existing-validator prerequisite work following PR 30. It
keeps the six pinned MusicXML 4.0 schemas and XSD 1.0 interpretation. It does not
introduce the planned public assessment API, compiled catalog, session, source
adapter, direct-model adapter, resource-limit policy or broader advertised
profile. Those remain explicit later owners. Planning statuses remain planned;
reusable implementation evidence is not whole-stage readiness.

## Baseline and obligation ownership

PR 30 supplies exact integer/decimal values, inherited whitespace and patterns,
complex simple-content restrictions, built-in list lengths, selected aggregate
values and XSD 1.0 union-member expansion. Earlier repairs supply mandatory XML
checks, simple-element attribute permissibility, schema-instance name recognition,
nil contracts, and effective empty-element defaults/fixed values. Those paths
remain regression obligations; older historical code-map descriptions do not
reopen repairs already present on merged main.

| Obligation | This prerequisite | Explicit remaining owner |
| --- | --- | --- |
| REQ-DEF-OPS-SCALARS | Primitive-domain identity, supported boolean/URI equality, pinned datatype-name repertoire; exact date/number/whitespace/pattern reuse evidence | Full unverified primitive algebra and external-specification closure; session cache/budget integration |
| REQ-DEF-NUM-EXACT | Preserve exact immutable integer/decimal comparison and existing operation-cost tests | Context arithmetic, shared accounts, source conversion and model evidence |
| REQ-DEF-XSI-TYPE / XSI-NIL | Subject namespace scope, assembly-local type lookup, derivation/block/final/abstract metadata and actual-type assessment composed with declaration nillability | Shared binder/capability profile and source adapter |
| REQ-DEF-XSI-SCHEMALOC / NONAMESPACE | Actual list(anyURI)/anyURI datatype assessment under the approved fixed-schema disposition | Web discovery/resolution and namespace/location pairing (batch 8); hints never select/fetch a schema here |
| REQ-DEF-ATTR-COMPLEX / OPS-IDENTITY | Preserve physical presence and XSD 1.0 identity eligibility; explicitly disposition proposed default/list/union expansion | Effective fact production, document-qualified ambiguity/causal outcomes, later typed musical scopes |
| REQ-DEF-OPS-GRAMMAR | Valid-schema endpoint matching, nullable/repeated occurrence semantics, deterministic witnesses and polynomial-work evidence | General schema-component validity/UPA and unimplemented wildcard semantics |
| REQ-DEF-BUDGET-MATCH | Algorithmic cost proof and test counters, no loops proportional to huge nullable occurrence bounds | Shared limits, defaults, accounting, cancellation and observable exhaustion in the session/report batch |

## Required primitive surface

The six schema inputs are musicxml.xsd, opus.xsd, sounds.xsd, container.xsd,
xlink.xsd and xml.xsd. Their ordinary builtin references use string-derived
values, decimal/integer-derived values, date and anyURI. No pinned schema uses an
explicit list, element/attribute wildcard, or anyType/anySimpleType element.
Their unions contain only string and exact numeric members. Production date is
yyyy-mm-dd with the no-timezone pattern; no production temporal enumeration,
fixed or ordering facet requires a generic date algebra.

Valid xsi:type substitution does not permit unrelated primitive reinterpretation.
The ordinary atomic derivation closure is the ten string-family types, decimal
plus thirteen integer-family types, date and anyURI, 26 types in total. Boolean
lexical assessment is additionally required by xsi:nil and expanded QName by
xsi:type. The standard hints use anyURI and a list of anyURI. The 17 named plan
templates are an ancestor inventory, and the existing 46-entry builtin dispatch
test expressly is not a guarantee of all builtin value/facet semantics.

Primitive tags protect reusable comparisons from conflating different primitive
spaces. Integer restrictions share decimal value identity; string restrictions
share string identity. Boolean has two actual values despite four spellings.
URI equality uses its own normalized character value, without percent-decoding,
case folding, dot-segment resolution, base application or network use. Patterns
continue to inspect the selected lexical view, and union member selection is
never repeated using canonicalized text.

Same-family temporal, binary, floating-point and ordinary QName/NOTATION equality
remain unverified lexical limitations outside this selected surface. Ur-types
remain opaque compatibility values, not additional primitive domains. General
QName enumeration/fixed support would require separate schema-literal and
instance namespace contexts; xsi:type QName resolution alone does not provide it.
ENTITY is reachable as a string descendant, but lexical NCName validity alone
cannot establish an unparsed entity declaration. Reached missing declaration
facts must remain an assessment limitation, never a pass or an invented XSD
violation. No DTD resource is fetched.

## Schema-location assessment-layer disposition

Standard xsi:schemaLocation has a facet-free list(anyURI) type;
xsi:noNamespaceSchemaLocation has type anyURI. Zero, odd and even item counts
are not independently invalid in fixed-schema assessment. Namespace/location
pairing belongs to Web schema-discovery processing, which this validator does
not perform. Hints never select a different assembly or trigger resource access.
The plan's earlier unconditional odd-token negative is explicitly corrected;
the discovery/resolver owner retains pairing and its tests as a separate
obligation. This follows the approved three-stage structure rather than adding
an extra MusicXML rule.

Sources: [XSD 1.0 standard schema-location attributes](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#xsi.schemaLocation)
and [Web schema discovery](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#schema-loc).

## Identity version disposition

An effective schema default is distinct from membership in the normative
ID/IDREF table. This batch does not add the proposed absent-default/fixed-attribute
or arbitrary constructed-list/union identity rejection rules to XSD 1.0.

XSD 1.0 section 3.15.5 uses declaration-type eligibility. XSD 1.1 explicitly
changes these rules: Appendix G.1.7 identifies checks on default values and the
extension through list construction, union and extension as changes from 1.0.
The associated W3C bug is labeled 1.1-only. This is a correction to the proposed
implementation task based on the selected specification version, not an omitted
obligation or a new product policy.

Retain ordinary ID/IDREF/IDREFS and applicable restriction behavior, forward
resolution, repeated references, document isolation and physical absence.
Retain existing effective element-default behavior with its E1-56 evidence;
absent-attribute contribution is not a reason to revert that earlier repair.
Keep actual scalar primitive identity and declaration-based ID-table eligibility
separate during dynamic type selection. Do not let trial union members create
identity side effects. Later typed musical-reference and incomplete-traversal
rules remain distinct obligations.

Audit observations are deliberately labeled: libxml2 accepts unresolved explicit
IDREF fixtures and therefore cannot establish those negative outcomes. JAXP
rejects several constructed-type cases but that observed behavior does not
supersede the pinned 1.0 text. Both tested 1.0 engines accept the absent
attribute-default/fixed-reference cases; they are not labeled oracle bugs.

Sources:
- [XSD 1.0 identity table](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#sic-id)
- [XSD 1.0 attribute default contribution](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#sic-attrDefault)
- [XSD 1.1 changes, Appendix G.1.7](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/)
- [W3C constructed-type identity bug 10662](https://www.w3.org/Bugs/Public/show_bug.cgi?id=10662)

## Readiness boundary

A root/fixture parity test proves only its bound contracts. Production container
and sounds validators, executable catalog binding, direct field capabilities,
source parse-once conversion/draining, shared budgets and profile completeness
remain with their assigned batches. No new profile is enabled by these repairs.
The external CI selector must actually execute every new parity root; focused
internal checks alone do not establish that. Each demonstrated external-oracle
divergence remains separately identified rather than weakening the normative
internal expectation.

## Reused test-fixture correction

The earlier nil fixture's integer default `07` had pattern `0[0-9]`, excluding
its canonical value spelling `7`. That is invalid schema metadata under
[e-props-correct.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#e-props-correct)
and [cos-valid-default](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-valid-default).
The fixture now uses `0?[0-9]`; explicit `7` is valid and explicit `+7` still
fails the lexical pattern. Empty-element defaults and source nonmutation remain
covered without treating an invalid schema as a normative positive. JAXP rejects
the original schema and compiles the corrected complete fixture; libxml2's
acceptance of the earlier schema did not establish schema validity.

## Source-stage and API boundaries

The legacy raw parser still accepts an undeclared attribute prefix through
`encoding/xml`; it can also reject some newer XML markup names before XSD
assessment. The source-adapter owner must reconcile the selected XML/namespace
version and enforce mandatory namespace constraints with original-source tests
and correct stage attribution. Existing duplicate-expanded-attribute and
namespace-provenance repairs do not prove that remaining case complete.

Technical metadata/fact failures are private run-local errors. Incomplete-only
assessment does not match `ErrInvalidDocument`; independent proven issues may
coexist in a joined error and remain inspectable with `errors.As`. This bounded
error distinction supplies a prerequisite for the later report/session engine,
not its public outcomes, resource limits or readiness policy.

## Executable bindings and acceptance

These are concrete prerequisite bindings, not replacement statuses for the
planned executable catalog or profile-readiness records.

| Contract slice | Bound tests and implementation evidence |
| --- | --- |
| Primitive identity, boolean/URI equality | `TestValidatePrimitiveValues`, `TestPrimitiveValueDomains`, `TestPrimitiveValueImmutability`, `TestPrimitiveURIValuesDoNotFetch`; [primitive tests](../../validation_primitive_value_test.go) |
| Pinned name repertoire | `TestValidateDatatypeNameBoundaries`, `TestValidateDatatypeNameSources`, `TestDatatypeNamesImmutable`; [name tests](../../validation_datatype_name_test.go) |
| Actual type, derivation, nil and constraints | `TestValidateXSIType`, `TestXSITypeGeneratedMetadata`, `TestXSITypeSelectedAssembly`, `TestXSITypeCacheBounds`; [source tests](../../validation_xsi_type_test.go), [metadata tests](../../validation_xsi_metadata_test.go), [generator tests](../../internal/xsdgen/generate_derivation_validation_test.go) |
| Hint datatype and no-fetch | `TestValidateSchemaHintDatatypes`, `TestSchemaLocationHintsDoNotFetch`; [hint tests](../../validation_schema_hint_datatype_test.go), [no-fetch counter](../../validation_schema_hints_test.go) |
| Grammar endpoints and bounded work | `TestValidateGrammarParticles`, `TestGrammarExhaustiveShortLanguages`, algorithmic work/large-occurrence controls; [grammar tests](../../validation_grammar_test.go), [work tests](../../validation_grammar_work_test.go) |
| Technical boundary and composition | `TestGrammarMetadataAssessmentBoundary`, `TestXSIQNameUsesDatatypeRepertoire`, `TestReferencedNilElementDeclarationIsIncomplete`, `TestFixedUnresolvedComplexTypeIsIncomplete`; [integration](../../validation_prerequisite_integration_test.go), [metadata failures](../../validation_element_metadata_test.go) |
| Explicit identity reuse and version boundary | `TestXSITypeIdentityUsesDeclaration`, `TestIncompleteIdentityTraversalDoesNotProveAbsentTarget`, existing `TestValidateNilIdentityTracking`, `TestValidateNilIdentityTypeForms`, `TestElementValueConstraintIdentityTracking`, `TestValidateComplexSimpleContentIdentityOnce` |

Every new external parity root is named explicitly in the
[Linux CI selector](../../.github/workflows/ci.yml). The selector must execute
`TestPrimitiveValuesAgainstSchema`, `TestDatatypeNamesAgainstSchema`,
`TestDatatypeNamesProductionAgainstSchema`, `TestGrammarParticlesAgainstSchema`,
`TestXSITypeAgainstSchema` and `TestSchemaHintDatatypesAgainstSchema` on the
original fixture bytes. Optional local absence of xmllint is an explicit skip;
required CI absence is a hard failure. Named oracle divergences do not weaken
internal normative expectations or turn missing facts into success.

Whole-change acceptance also includes reproducible generation, module/format
checks, full tests and vet, race detection, the standard counted fuzz targets,
and unchanged pinned-registry planning integrity. Public source/model capability,
resource and profile acceptance remains with the later owners above.
