# Declaration-dependent XSD and standalone source composition

This scope extends the accepted bounded internal-DTD reader into existing
normative source consumers. It does not add a general validating DTD processor.

## Explicit profiles

The new, separately compiled catalog contains:

| Profile | Selected behavior |
| --- | --- |
| `ProfileSourceInternalDTDLocal` | Effective internal-DTD source facts, declaration-dependent XSD, source origins, and existing local clauses/typed references |
| `ProfileSourceInternalDTDLinks` | The local composition plus existing nonfetching local link, attachment, schema-hint and Opus projections |
| `ProfileSourceInternalDTDDictionaries` | The links composition plus explicitly supplied dictionary capabilities and existing consumers |

All three support score-partwise, score-timewise and Opus through their exact
MusicXML/Opus assemblies. Score-only clauses remain inapplicable to Opus. They
do not inspect linked score content. Container, Sounds, timeline/context and
package profiles are not implicitly composed.

The ordinary decode path, existing profiles and existing catalog identities
remain unchanged. Profile-name prefixes never elect parser capabilities.
The dictionary composition uses the existing caller-selected importer and one
shared account; there is no automatic dictionary loading or new data edition.
The existing partial IPA and open-string sounds contracts remain unchanged.

## Document entity membership

The reviewed XSD 1.0 boundary checks `ENTITY`, `ENTITIES` and their restriction
descendants against effective general-entity declarations in the containing
source document. Existing lexical, whitespace and facet constraints still apply;
the builtin `ENTITIES` list must be nonempty. Dynamic actual types must first
pass the existing QName, declaration and derivation checks. In particular,
`ENTITIES` cannot simply replace a declared `xs:string` type.

An effective unparsed binding proves membership. An effective parsed binding
disproves it. An absent name is a violation only when the relevant declaration
enumeration is complete; otherwise it remains unavailable. Parameter entities,
ignored duplicate declarations and declarations suppressed by the reader's
unread-parameter-entity cutoff cannot become positive witnesses. A selected
reader that has completed the prolog without a DOCTYPE can establish a complete
empty domain. Omitted declaration input cannot.

`NDATA` determines whether the general entity is unparsed. Missing or duplicate
notation declarations are a separate XML validity question; they do not erase
the known entity binding or create a new XSD membership failure. No external
entity payload or subset is fetched. Accepted standalone and cutoff processing
is reused without reinterpretation.

Declaration enumeration is independent of document EOF, replacement-content
well-formedness and complete attribute enumeration. The existing XSD binding
guard still refuses to infer absent `xsi:type` or `xsi:nil` from an incomplete
attribute set. Consequently, a useful partial declaration witness does not
necessarily make an entire source occurrence assessable or a report satisfied.

Sources: [XSD 1.0 String Valid](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-datatype-valid),
[ENTITY and ENTITIES](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#ENTITY),
[XML 1.0 entity and processor rules](https://www.w3.org/TR/2008/REC-xml-20081126/),
and [unparsed entity information items](https://www.w3.org/TR/2004/REC-xml-infoset-20040204/#infoitem.unparsed).

## Literal and constructed-type boundaries

Schema enumeration/fixed operands are converted in their schema value context;
an unrelated instance DTD must not decide their pure conversion or poison a
schema-keyed cache. Applied effective instance values still receive membership
assessment. `CompileSchema` continues not to certify general schema-component
validity, including every context-dependent value-constraint obligation.

Arbitrary ENTITY-bearing constructed lists/unions retain an explicit unsupported
boundary. [W3C issue R-200](https://www.w3.org/2001/05/xmlschema-rec-comments.html#pfiENTITYlists)
records the XSD 1.0 wording gap, while the completed
[bug 10662](https://www.w3.org/Bugs/Public/show_bug.cgi?id=10662) expressly concerns
XSD 1.1. This implementation does not silently import that later interpretation.
Existing ordered-member selection and technical-uncertainty handling remain in
force. This is partial capability support, not complete constructed-type closure.

## Owned input and provenance

`NewXSDPlanWithSourceDeclarations` elects the new sealed source-only mode.
Selected XSD operations sharing one assembly/run must agree on their mode;
an unsupported or unselected plan cannot enable runtime facts. Old `NewXSDPlan`
digest bytes remain unchanged. The new catalog is separate because catalog
identity also includes its other rules and profiles.

`Session.SourceEntityDeclarations` accepts one document-qualified snapshot after
root `Open`, before attributes, children or closing events. It copies effective
bindings and independently available declaration locations. Declaration sequence
IDs are document-local identities, not schema declarations or fabricated nodes.
The session owns the storage until its borrowed XSD run has closed, including
prefix finalization after scanner cleanup. Missing positions remain unavailable.

New source-presence local plans use supplied effective source attributes,
including DTD defaults and entity markup. XSD-only defaults do not establish
supplied presence. The new declaration-aware catalog rejects selected, available incompatible legacy
physical-presence plans rather than silently changing old constructors.

Resource projections retain a separate physical-presence axis. A DTD default can
produce an effective value while physical `Presence` and `Raw` are absent.
`Session.SourceAttributeLexical` can admit the actually retained XML-normalized
physical spelling after the matching attribute and origin events. Unretained or
general-entity-transformed original spelling remains unavailable; it is never
reconstructed from the effective value. Schema-hint pairing consumes effective
values independently, so defaulted hints are not lost when physical raw evidence
is absent. None of these projections grants retrieval authority.

All ingress copying, indexes, temporary overlap and lookup work use the same
finite account. Cancellation is polled during bounded scans. Constructor/event
errors, unavailable evidence, normative failures and resource refusal retain
their distinct outcomes and causes. Existing source reader ownership is unchanged.

## Contribution and retained owners

The direct obligation is `REQ-DEF-OPS-SCALARS` and the reached declaration gap in
[XSD prerequisites](xsd-prerequisite-scope.md). The composition contributes to
existing attribute/local, typed-reference, link/schema-hint and dictionary
consumers; it does not create another set of requirements. Original identifiers,
denominators and historical completion records are preserved.

General DTD validity, DTD ID domains, external declarations, wider promised XSD
contracts, complete IPA, remaining contextual/intent predicates and
`B5-PERF-01` remain separately owned. Integer/decimal and DOCTYPE placement rows
are not reinterpreted as new declaration-validation duties.

Source-origin resource snapshots remain explicitly unsupported. Package export
requires separate profile pins, ownership/accounting and rebasing of subject,
physical, declaration, use and ancestry locations across multiple adoptions.
That transport contract is retained with external package closure; standalone
composition does not advertise universal MXL/graph support.

Execution and independent acceptance are recorded separately in
[the evidence document](source-declarations-evidence.md). Scope statements alone
are not test execution or final acceptance.
