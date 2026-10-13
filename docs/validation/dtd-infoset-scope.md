# Bounded internal-DTD source-infoset prerequisite

This is an additive prerequisite within the final approved assessment program.
The split is necessary because effective attributes, entity-expanded structure
and source origins change the facts consumed by later identity, dictionary and
external-resource checks. It does not retire those downstream obligations or
replace the original nine-batch target with an omission ceiling.

## Profile and compatibility

`musicxml-4.0/source-internal-dtd` selects only original-source MusicXML
score-partwise, score-timewise and Opus assessment. It runs the six shared XSD
operations over the effective stream and sealed typed source-origin providers.
The old source/XSD/local/timeline/context/package profiles and plain decoding
retain their accepted contracts and digests. The new profile's XSD operations
have distinct revisions. `DecodeAssessed` converts the same effective stream it
assesses; it does not reread input or independently expand entities.

The profile is deliberately separate from general validating-DTD or XML processor
conformance. A satisfied report applies only to its selected bounded operations.
Unsupported required evidence prevents acceptance. Current-model input cannot
reconstruct declarations or source history. Source-origin resource snapshot
export is explicitly unsupported and preserves the separately returned report.

The approved additive `ContextResult.SourceOrigin` field has `omitempty`; old
JSON is unchanged. External unkeyed literals need conversion to keyed literals.
Other public fact, location, report and option layouts are unchanged.

## Authorities and semantic boundaries

The normative baseline is [XML 1.0 Fifth Edition](https://www.w3.org/TR/2008/REC-xml-20081126/),
especially sections 2.8–2.11, 3.3, 4.1–4.6 and 5.1;
[Namespaces 1.0 Third Edition](https://www.w3.org/TR/2009/REC-xml-names-20091208/),
sections 2.3, 3 and 6; and [XML Infoset Second Edition](https://www.w3.org/TR/2004/REC-xml-infoset-20040204/),
sections 2.1–2.3. These define the selected behavior, not a claim that every
processor requirement in those specifications has been implemented.

- General and parameter entities have separate declaration domains. First
  declarations bind; repeated ATTLISTs merge by lexical element/attribute names.
  Ignored bindings are still lexically checked and applicable read defaults are
  checked in attribute context even if suppressed or never used.
- Whole-declaration internal parameter-entity expansion is supported. A frame
  cannot borrow the caller's delimiter, quote, partial declaration or element
  boundary. Parameter-entity declaration origin is distinct from transport: XML
  calls these external markup declarations even when all bytes are internal.
- Read declarations before an unread PE remain usable. Later effective bindings
  obey XML section 5.1's cutoff and standalone exception. Unread declarations do
  not become known absence. Document EOF, syntax closure, declaration closure,
  default availability and expansion completeness remain distinct.
- General replacement uses scanner input frames. Replacement quotes remain data
  in a containing attribute; character-reference whitespace is distinct from
  literal whitespace. Physical line-end and attribute normalization occur once.
  The final non-CDATA step trims/collapses U+0020 only.
- Applicable defaults, including absent `#FIXED`, are supplied before namespace
  expansion. A physical lexical attribute suppresses its matching default.
  Defaulted namespace declarations apply on their own tag; expanded-name
  collisions are still rejected. Required/default/fixed DTD validity predicates
  are not silently converted into XML syntax failures.
- Entity-declaration well-formedness versus validity conditions are kept separate.
  In particular, a reference stored in a directly declared alias retains that
  declaration origin when invoked through a PE-origin default. Caller ancestry
  cannot make a standalone violation disappear.

## Provenance and ownership

Each admitted element occurrence and effective attribute slot has an owned typed
origin row. `Specified` describes original physical presence, whereas ordinary
attribute presence describes effective presence. Declaration and use sites are
separate from the occurrence's physical site. Synthetic occurrences have no
fabricated original position. Missing sites and incomplete ancestry remain
explicitly unavailable; they do not become byte zero or known absence.

The adapter's public evidence is occurrence-level, not a segmented source map of
every character in an effective scalar. Physically encoded attributes containing
entity references remain physically specified. Entity-created markup is
synthetic. PE-created declaration positions can remain unavailable. Diagnostics
must not label transformed value bytes as precise original scalar bytes. Full
per-segment and downstream declaration-domain provenance remain open work.

Providers are exact source/assembly plans with admitted-subject checks and one
row per present slot. Final enumeration is sealed only after successful input
completion. Owned getter copies, causal evidence and old JSON compatibility are
part of the contract. Ancestry transport is bounded at 64; unavailable ancestry
is reported, never silently truncated into complete evidence.

## Finite resources and no transport

There is no network/filesystem resolver, ambient schema/entity loading, or new
transport permission. An external declaration can be observed without reading
its target. Official samples with external MusicXML DOCTYPEs still have missing
external evidence; this slice does not claim their source readiness.

One existing account covers the scanner, declaration store, defaults, observer
and session. Input hashes/bytes cover original bytes once. Internal replacement
is not input or external traffic. Event work includes repeated replacement UTF-8
bytes, tokenized-normalization bytes and reference/frame invocations before processing; sharing a declaration
does not make repeated expansion free. Retained declarations, temporary copies,
old/new growth peaks, frame state and owned facts reserve buffered bytes before
allocation and release only their own charges. Nodes and effective attributes
retain established ingress charges. The independent 64-frame replacement limit
and XML depth both apply. Defaults remain finite and unchanged.

Cancellation is checked within bounded scans, normalization and replacement
processing. It cannot interrupt an arbitrary blocking caller `Read`; callers
must provide cancellable I/O where needed. The library does not close the caller's
reader. Syntax, unsupported evidence, resource refusal and cancellation retain
their own causes and do not fabricate one another's outcomes.

## Original obligations: contribution, not closure

The complete final-program audit retains all 95 engineering rows, 307 contextual
checks, 291 dispositions, 51 prose rows, 28 issue gates and 20 split records.
This slice contributes only the following prerequisite cells. Original records,
denominators and accepted historical statuses are not rewritten.

| Original obligation | Delivered contribution | Retained owner/dependency |
| --- | --- | --- |
| `REQ-DEF-XML-DOCTYPE` / `TEST-DEF-XML-DOCTYPE` | Selected read internal declarations, bounded frames, defaults, normalization and honest origin/closure | External subset/entities, complete DTD/XML processor duties and old-profile limits remain final external closure |
| `CHECK-context-xml-reserved-semantics` | Effective namespace attributes assembled before namespace expansion, source-only provenance | General XML Base, xml:id, XInclude and external rules remain final external closure |
| `REQ-DEF-XSI-SCHEMALOC`, `REQ-DEF-XSI-NONAMESPACE` | Effective attributes reach the existing shared source XSD machinery without invented physical history | Resolver/graph hint consumers are not newly composed; current-model discarded history remains Unknown |
| `REQ-DEF-OPS-SCALARS`, `REQ-DEF-NUM-DOMAIN`, `REQ-DEF-NUM-EXACT` | Declaration storage is a prerequisite for later declaration-dependent domains | XSD ENTITY membership, wider scalar/equality promises are not claimed closed |
| `REQ-DEF-ATTR-SIMPLE`, `REQ-DEF-ATTR-COMPLEX`, `REQ-DEF-OPS-IDENTITY` | Effective DTD attributes are assessed through existing exact XSD operations | Generic XSD default/constructed identity, DTD ID domains and wider identity closure remain separately owned |
| `REQ-DEF-EXTERNAL-RESOLVER`, `CHECK-context-link-element-position` | No-fetch and original-byte/occurrence identity preserved | External transport, target interpretation, XPointer and package integration remain later consumers |
| `B5-PERF-01` | Bounded accounting regressions only | Comparative throughput, memory, scaling and final performance acceptance remain independently owned |

Still explicit: unused recursive declarations and unverified predefined-entity
redeclarations can prevent full well-formedness closure; external data is never
assumed read; broader declaration validity is not a parser success claim.
Dictionaries (SMuFL, IPA, sounds), residual musical/intent clauses, ZIP/JAR/MIME
and package/link normative gates, generic promised kernel contracts and final
requirement reconciliation remain in the approved program. The historical
114-translation artifact has only a recorded fingerprint; its portability gap
is not solved by this implementation or by redoing completed translations.

See [execution and review evidence](dtd-infoset-evidence.md) for the distinction
between implemented behavior, focused regression evidence and final acceptance.
