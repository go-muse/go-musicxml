# Six end-to-end validation contract traces

Status: **DESIGN EXAMPLES**, not production validator results. Read the
[Go contracts](go-contracts.md) first. Each trace covers input, adapter facts,
shared rule/dependency execution and observable result. Example rule labels
below are candidate semantic names, not frozen runtime IDs. Research requirement
IDs and source links are pinned evidence.

Fragments below occupy their normal position in an otherwise conforming
MusicXML 4.0 document; they are not standalone schema-root fixtures. A scoped
profile can isolate the stated obligation for a feasibility test. A complete
MusicXML profile may additionally find other problems and must not inherit a
“pass” from this narrow example. Unless noted, input reaches EOF, budgets suffice,
and all unrelated selected mandatory obligations pass.

## 1. A local prose rule with source and model facts

Input inside a direction-type:

```xml
<accordion-registration/>
```

The equivalent current Go object is `AccordionRegistration{}`. Its three pointer
fields `AccordionHigh`, `AccordionMiddle`, and `AccordionLow` are nil.

1. XML adapter emits open/close for the registration with a known-complete empty
   child set. The direct model adapter enumerates those three absent pointers
   and emits the same current child-set evidence; neither calls `Encode`.
2. The compiled `count-children-at-least` plan selects `accordion-high`,
   `accordion-middle`, and `accordion-low`, with minimum 1. Applicability is
   proven because the registration exists.
3. One mandatory local predicate fails: observed count 0, expected at least 1.
   Its provenance is `MX40-PROSE-accordion-at-least-one`. The XSD allows all three
   children to be optional; it does not supply this additional prose condition.
4. The report is `violated`, complete for this scoped profile. Strict decode
   withholds the otherwise representable model and returns an assessment error.
   Permissive decode remains permitted; a later explicit model assessment finds
   the same local failure.

Representative formatted diagnostic:

> /score-partwise/part[1]/measure[1]/direction[1]/direction-type[1]/accordion-registration[1]:
> At least one accordion register is required. Expected: high, middle, or low;
> actual: none. These children are individually optional in XSD, but their
> combined presence is required by the prose. Source: the pinned accordion
> registration annotation. Rule: candidate accordion-at-least-one, revision 1.

The actual formatter includes the verified [source URL][accordion], not merely
“the pinned annotation.” A model location uses its field/index path without
fabricated XML lines.

Boundary checks: adding `AccordionHigh: &Empty{}` passes; a registration that
is not present is not applicable; a partial child set is unknown, not empty;
a known absent required child in some other structure remains a normative
absence failure, not a generic model-capability excuse.

## 2. Child order: source grammar versus current model structure

Input inside a note:

```xml
<pitch><octave>4</octave><step>C</step></pitch>
```

1. Source facts contain child order `[octave, step]` with `Order=source-order`.
   The selector identifies `musicxml` assembly type `pitch`; matching only the
   local name across assemblies would be insufficient.
2. The shared grammar plan requires `step`, optional `alter`, then `octave`.
   Its sequence matcher fails even though each child has an individually valid
   value and the independent child counts look plausible.
3. The source report is `violated` with a grammar diagnostic at `pitch` and the
   unexpected first child. Strict decode rejects it; the original stream is not
   replaced with the model's export order.
4. A permissively decoded or newly constructed `Pitch{Step: StepC, Octave: 4}`
   is a different target. The model walker reports the defined field sequence
   `step, octave` with `Order=model-field-order`, and current value facts.
   The same grammar operator can assess that current structure successfully.
   The model report says `Target=model`; it makes no historical source-order
   claim and cannot retroactively clear the source diagnostic.

Source: [pitch type and sequence][pitch]. The semantic rule is a compiled schema
component contract rather than a new prose requirement.

Boundary checks: `step, alter, octave` passes; a repeated source `octave` fails
occurrence/grammar even if decoding collapses it to one Go field; an ordered
`Content` slice must retain its current order rather than sorting it; missing
original non-pointer scalar presence cannot be inferred from a Go zero value.
Nested choice/repetition still needs full particle semantics, not this one
sequence as a universal matcher test.

## 3. Generic IDREF plus a typed, scoped target

Relevant source facts in one score are:

```xml
<part-list>
  <score-part id="P1">
    <part-name>Clarinet</part-name>
    <score-instrument id="I1"><instrument-name>Clarinet</instrument-name></score-instrument>
  </score-part>
</part-list>
<part id="I1"><measure number="1"/></part>
```

1. Both adapters can enumerate the current declaration `I1` on `score-instrument`
   and the `part/@id` reference `I1`, with locations and document/part scopes.
2. The ID provider finalizes a unique document-wide index after EOF or complete
   model traversal. Generic IDREF existence passes: `I1` exists.
3. The dependent plan for `MX40-PROSE-part-id-target` requires a `score-part` in
   this document's `part-list`. It fails because the resolved target is a
   `score-instrument`. The diagnostic names the required/actual target kinds
   and includes the declaration location. It does not report an IDREF-missing
   error or pretend the XSD generic reference condition failed.
4. The normative scoped report is `violated`. Both source and model can provide
   enough evidence; no external resolver is involved.

Source: [part-attributes annotation][part-target].

Variants distinguish the dependencies:

- `id="P1"`: both generic reference and typed target pass.
- `id="missing"`: at finalization the generic rule fails once; the typed rule is
  `blocked` with a document-qualified `DecisionRef` to that exact failure. Conformance is violated,
  assessment incomplete under the contract's conservative blocked policy.
- Two declarations of `I1`: uniqueness fails with both locations; resolution is
  ambiguous and dependent target rules are blocked, not arbitrary first-match.
- A forward reference: remains pending until complete scope, not a premature
  missing-ID failure. An interrupted input leaves it unknown.
- Repeated timewise `part` IDREF occurrences are references, not duplicate ID
  declarations. Do not impose an unsupported sibling-uniqueness rule.

## 4. Local reference success and unresolved linked-file evidence

Relevant source inside `score-part` (XLink binding is in scope):

```xml
<part-link xlink:href="parts/clarinet.musicxml">
  <instrument-link id="I1"/>
</part-link>
```

The same `score-part` declares `score-instrument id="I1"`. The Go representation
uses `PartLink.Href`, `PartLink.InstrumentLink`, and `InstrumentLink.ID` directly.

1. Shared ID/typed-scope operators establish the local sub-contract: `I1` is the
   correct score instrument in the owning score part. They pass.
2. The selected external obligation records `Role=predicate` plus
   `Requires=external`; needing external data does not create a separate role.
   The loader request carries kind `linked-document`, base/href, required
   profile/revision and source location. It is not an automatic schema fetch.
3. With no resolver/provider, the external sub-contract is `unsupported` and
   the aggregate is `undetermined`, incomplete, even though local reference
   checks pass. Strict decode returns an incomplete-assessment error; the
   permissive default does not begin retrieving the link.
4. With a caller-supplied local resolver, the root loader owns the returned
   stream, enforces shared budgets, parses the pinned resource once, closes it,
   and returns typed facts plus its child assessment. Core has no XML parser
   or network dependency. Only enough complete evidence for the formalized
   linked-file obligation can produce a pass.

Provenance: `MX40-PROSE-instrument-link-target`,
[instrument-link annotation][instrument-link] and [part-link declaration][part-link].
This trace deliberately does **not** assert an unreviewed rule that the linked
file must contain an identical ID spelling. Formalizing which facts prove the
part-file association is a later rule task. If that operator has not been
formalized, even a successfully loaded file leaves it `unsupported`.

Boundary checks: authoritative absence from a complete local package is distinct
from a timeout/unavailable service; the latter is unknown. A successful raw read
is not proof of content conformance. Wrong target type, malformed linked XML,
stale/wrong dictionary revision, URI cycles and aggregate budget exhaustion
retain their respective causes. A local failure blocks unnecessary external
lookup. Document caches include identity and revision/digest, not href alone.

## 5. Valid XSD integer beyond the Go field's range

Input inside `attributes`:

```xml
<staves>18446744073709551616</staves>
<instruments>1</instruments>
```

1. The source observer records the exact staves lexeme before model conversion.
   The scalar operator applies the `xs:nonNegativeInteger` lexical/value
   contract using budgeted arbitrary precision. The value is 2^64 and is within
   that XSD value space. A finite Go integer range is not an XSD facet.
2. Current `Attributes.Staves` is `*uint64`; model mapping cannot represent
   18446744073709551616. Conversion records `unrepresentable`, retaining the
   actual field/path and value. It creates no false XSD invalid-integer finding.
3. Orchestration discards the partial model, stops model mapping, and continues
   **the same parser** through `instruments`, remaining document and EOF. No
   second reader, rewind, retained-byte replay or re-encoded model is used.
4. `InspectXML` can return conformant and complete for the relevant fully
   implemented source profile. `DecodeAssessed` can carry that same successful
   source report while returning a representability error, conversion state
   `unrepresentable`, and a nil document. It does not newly match
   `ErrInvalidDocument` solely because of Go overflow.

Source: [staves declaration][staves] and the existing Go field in
[`zz_generated_types.go`](../../zz_generated_types.go). Exact scalar handling is
subject to resource budgets, not unbounded allocation promises.

Boundary checks: 18446744073709551615 is representable in `uint64`; `-1` is a
normative domain failure; a lexeme exceeding `MaxScalarBytes` is a resource
limit/incomplete result, not an XSD range failure. If a malformed XML tail follows
the overflow, preserve both the conversion cause and syntax cause; syntax is
malformed and no complete conformant-source claim survives. If canceled while
draining, retain earlier findings and mark incomplete.

This is a required future behavior. Current main still rejects the large
integer in the internal scalar checker, as recorded in the existing-code map.
A temporary feasibility probe can demonstrate token-stream continuation without
constituting a production decoder change.

## 6. Context provider and a document-level condition

Document defaults and a later measure contain:

```xml
<defaults><concert-score/></defaults>
<!-- later, inside a measure -->
<attributes><transpose><chromatic>2</chromatic></transpose></attributes>
```

1. Either adapter emits present `concert-score` and exact current/source
   chromatic value 2, preserving locations. A context-provider plan emits
   `ContextUpdate{Key: document concert-score, Fact: known true}` with origin
   and provider rule key. The session copies this output into its context store.
2. The predicate for `MX40-PROSE-concert-score-transpose` depends on that fact
   and exact `diatonic`/`chromatic` evidence. It queries `ContextView` for this
   document, not a process-global setting, and fails because chromatic is nonzero.
3. Diagnostic locations include the transpose and the concert-score that makes
   the condition apply. The source/current-model scoped report is violated and
   complete if no other selected obligation is unfinished.
4. No concert-score in a known-complete defaults/header scope makes this
   particular conditional predicate not applicable. An unknown/unsupported
   concert-score fact does not. Zero chromatic with diatonic omitted/zero and permitted octave-change or
   double is not rejected. `part-transpose` inside `for-part` is not incorrectly
   selected as `transpose`.

Source: [concert-score annotation][concert-score]. This is a bounded context
example; it does not pretend to implement a complete musical timeline.

A selected `MX40-PROSE-numeral-requires-key` plan illustrates the stricter boundary:
its provider needs effective key at the numeral's musical position and staff.
An explicit local numeral-key can satisfy its corresponding alternative; otherwise
`ContextKey.At` carries the document-qualified subject; the key also includes
part and staff/voice as relevant.
If backup/forward, divisions or ordering cannot be interpreted, effective key is
unknown/unsupported and the obligation is incomplete. Do not pick the last key
encountered lexically, borrow another staff's key, or invent C major. A provider
that proves no effective key in a complete, supported context can establish
absence; that is different from merely failing to find context.
Source: [numeral annotation][numeral].

## Review and later test matrix

These traces establish the interfaces' intended semantics, not implementation
coverage. A future executable slice needs positive, negative and boundary cases
for both relevant targets, plus explicit unsupported-capability cases.

| Contract | Required verification |
| --- | --- |
| Once-only input | Non-seekable/counting reader; unknown subtree, namespace collision, tail errors, mid-element conversion failure. |
| Direct model adapter | No Encode/parser calls; optional pointer absence, zero required scalars, ordered content, invalid choice, cycles and depth. |
| Rule reuse | Same compiled rule key/plan across source and model; target-specific evidence explains differing outcomes. |
| Identity/context | Forward reference, duplicate ID, wrong kind/scope, provider output, cross-document/staff isolation, invalid prerequisites. |
| External evidence | No resolver, authoritative absence, transient failure, wrong revision, body close on every path, bounded cached linked parse. |
| Results | Zero report, advisory only, known violation plus incompleteness, diagnostic truncation, cancellation and causal error compatibility. |

Temporary contract compilation and focused feasibility probes are design evidence
only. They do not replace the library's regression suite or establish a new
validator, full source conformance, full normative closure, or a stable public API.

[accordion]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L3290
[pitch]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L5402
[part-target]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2407
[instrument-link]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L5928
[part-link]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L5992
[staves]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2840
[concert-score]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L5871
[numeral]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L3889
