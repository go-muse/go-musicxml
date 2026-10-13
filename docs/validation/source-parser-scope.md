# Source XML reader scope

The root package has one private source scanner shared by legacy decode and the
new source-assessment orchestration. It does not call another XML parser over
captured bytes, fabricate an XML declaration element, re-encode a model, fetch a
DTD, or select a schema from document hints. `encoding/xml.NewTokenDecoder`
consumes already-scanned tokens for namespace expansion and model unmarshalling.
The scanner is an original implementation informed by the XML grammar and the
existing Go token API; no Go parser source was copied into this component.

## Syntax and compatibility

The markup repertoire is XML 1.0 Fifth Edition, with Namespaces in XML 1.0 Third
Edition constraints. It is separate from the older XML name repertoire required
by the pinned XSD 1.0 datatype rules. Fifth Edition supplementary markup names
are accepted. Lexical QName shape, prefix bindings, reserved XML/XMLNS bindings,
raw start/end name matching and expanded-attribute uniqueness are checked before
any model filtering or skipping. Namespace names are compared literally. URI
reference syntax is not validated, as expressly permitted for processors by
Namespaces section 8; relative namespace names remain accepted.

The reader checks declaration grammar, document boundaries, exactly one root,
comments/PIs/CDATA, character references and the XML character repertoire in
all lexical contexts. A syntactic version `1.x` is processed using XML 1.0 rules.
UTF-8, UTF-16BE/LE, UTF-16 signatures and the previously supported Latin1 encoding
aliases remain available. Unsupported encodings are typed capability failures.
Identified encodings with malformed code units are syntax failures.

Legacy public functions and the `DecodeOptions` layout are unchanged. Decoding
remains permissive about well-formed unknown MusicXML elements and attributes.
The following formerly accepted input is now rejected as genuine XML defects:
undeclared prefixes, illegal reserved bindings, malformed declarations or DTD
grammar, and contradictory BOM/encoding declarations. A BOM selects decoding;
it does not legitimize an incompatible declaration. Out-of-root literal text
now has a typed `xml.SyntaxError` cause. Literal attribute tabs and line endings
now normalize to spaces, while numeric references to whitespace keep their
referenced characters. This can change decoded string values for such input.

## DTD boundary

This section records the legacy and existing source-profile boundary. The
explicitly selected `ProfileSourceInternalDTD` has the separately documented
[bounded internal-DTD contract](dtd-infoset-scope.md); it does not change the
profiles described below or authorize external loading.

The no-fetch reader checks the local grammar of DOCTYPE, ELEMENT, ATTLIST,
ENTITY, NOTATION, comments and PIs. Empty and comment/PI-only internal subsets
are supported. A DOCTYPE/root-name mismatch is a DTD validity matter and is not
rejected by the XML syntax layer. External identifiers remain inert.

DTD defaults, tokenized attribute normalization, general-entity replacement,
entity declaration ordering/recursion and external-subset processing are not
implemented. Legacy decode still ignores unused declarations and does not apply
default attributes. Source assessment receives explicit unsupported evidence
instead of claiming those resulting infoset facts are complete. Uninterpreted
ATTLIST/ENTITY declaration families also make XML well-formedness assessment
incomplete: local grammar closure does not establish all declaration WFCs.
An unread external identifier alone is distinguished from this internal-declaration
WFC limitation, while still exposing possible infoset dependencies as unsupported.

Consequently, the source preview cannot satisfy an ordinary score that includes
an external DOCTYPE. This includes all three committed official score samples;
strict assessed decoding rejects their source assessment despite successful
model conversion. The two committed opus examples contain no DOCTYPE. Neither a
known MusicXML public identifier nor xmllint's XSD-only acceptance proves that
an unread external subset is infoset-neutral. No ignore-DTD policy or implicit
DTD loading is provided in this batch.

Parameter-entity replacement needed to continue the internal-subset grammar,
non-predefined entity expansion, and namespace bindings that could depend on
unread DTD default attributes stop with typed capability errors. Unavailable
replacement text is never invented as empty text. Obvious lexical violations
remain syntax failures. This boundary is not a claim to implement a fully
conforming general-purpose non-validating XML processor.

The DTD grammar has explicit implementation ceilings of 1 MiB for names and
1024 nested content-model groups, with lower account limits taking precedence.
These ceilings yield capability/resource errors, not false malformed-XML claims.

## Observation and source coordinates

Raw-prefix tokens and matching lexical metadata are delivered synchronously.
A single source token decoder expands names. The source observer runs before
namespace projection, attribute filtering and every skip. Namespace-declaration
flags come from raw lexical names; a foreign URI literally named `xmlns` does
not turn an ordinary attribute into a declaration. Metadata and token slices
are borrowed until the next token call; retaining consumers copy them first.

Element/attribute spans use zero-based original-byte offsets including BOMs.
Lines and columns are one-based decoded source-character coordinates; a CRLF
pair counts as one line ending, and a supplementary character counts as one
column even when represented by a UTF-16 surrogate pair. If either coordinate
exceeds the host-sized integer range, both are omitted as `0/0` for the rest of
the source; original-byte offsets remain available. Attribute-value spans
exclude their quote delimiters. Synthetic end tokens for empty tags have an
explicit flag and a zero-width position after the original tag.

The input hash covers original bytes before decoding. Bytes read can exceed the
current token's consumed position because fixed-size buffering reads ahead.
Only a clean EOF establishes a complete digest. A failed run's digest is a prefix
digest with a byte count, never a whole-document fingerprint. Input completion,
digest completion and syntax-assessment completion are separate: an internal DTD
can reach EOF with a complete digest but unresolved XML WFCs.

## Resource ownership

Strict source runs supply their shared `validation.Account`; legacy decode and
unassessed transport retain their existing depth-only configurable policy.
A nil account does not silently create assessment profiles or new scalar/input
limits. Cancellation is still checked when a context is supplied.

With an account, original input bytes are charged at reader refill. Exact-limit
EOF is allowed, and a one-byte overflow probe detects excess input. The scanner
charges raw-token and raw-attribute-attempt work to Events. Session fact events
are separate work. Session ingress remains the owner of admitted Nodes,
Attributes and the live semantic Depth gauge; the parser independently guards
its local depth without charging that same nesting twice.

Scanner token builders, namespace/scoping state, input buffers and temporary DTD
name/content-model buffers reserve buffered storage before growing. Per-item
scalar limits apply during lexical accumulation, not after a giant token has
already been returned. Closing markup is not charged as comment/PI/CDATA value
content. Persistent namespace key capacity is retained and charged until reader
cleanup, even after a scope unbinds a prefix. Session/observer copies have their
own retention charges. Logical allowance accounting is not a heap profiler.

Close releases only scanner-owned live storage and never closes the caller's
reader. Cumulative byte/event work is not refunded. Context checks occur before
reads, throughout long token scans and before namespace processing. Arbitrary
blocking `io.Reader.Read` calls still require caller-provided cancellable I/O.

## Verification

The existing encoding, namespace, declaration, document-boundary, skipped-subtree,
MXL and linked-resource matrices exercise the shared scanner. Focused source tests
add original-byte UTF-8/BOM/UTF-16 positions, literal/reference normalization,
malformed token classes, exact and excess quotas, all-reservation cleanup, data
plus EOF/I/O error, DTD capability distinctions and raw-prefix URI lookalikes.
The counted `FuzzSourceXMLReader` target checks fragment-independent acceptance,
progress and cleanup under finite budgets. These tests do not claim complete XML
or MusicXML production conformance or replace independent normative review.

## Primary references

- [XML 1.0 Fifth Edition](https://www.w3.org/TR/2008/REC-xml-20081126/): document and declaration productions, characters/names, encoding, attribute normalization and processor duties
- [Namespaces in XML 1.0 Third Edition](https://www.w3.org/TR/REC-xml-names/): QName/binding/uniqueness rules and section 8's URI-reference-checking exception
- [Go encoding/xml token API](https://pkg.go.dev/encoding/xml#NewTokenDecoder): token-stream transport; its namespace behavior is not itself proof of full namespace constraint checking

## Legacy Decode performance

`BenchmarkDecode` measures the committed official UTF-8 and UTF-16 scores,
including model construction and mandatory XML checks but excluding fixture I/O
and opt-in assessment. The cursor and scanner reuse bounded scratch storage,
scan eligible ASCII spans within one charged refill, and avoid UTF-16 per-unit
allocations; delimiters, CR/CRLF, non-ASCII and invalid controls retain the normal
lexical path. Cancellation and quota admission precede each bounded span.

Five interleaved one-second benchmark sets on Go 1.27.2, linux/amd64, AMD EPYC
9V74 with `GOMAXPROCS=1` measured these fixture-specific medians:

| Fixture | Previous main reader | Initial PR reader | Optimized reader before coordinate guard |
| --- | ---: | ---: | ---: |
| MozartTrio (UTF-8) | 6.045 ms | 12.204 ms | 7.871 ms |
| MozaChloSample (UTF-16BE) | 9.972 ms | 16.461 ms | 12.253 ms |

That optimized checkpoint still cost approximately 30% / 23% more time than the
previous main reader on these inputs, while preserving the stronger checks and
provenance. Allocations per operation are 40,182 versus 39,601 previously for
UTF-8, and 50,073 versus 160,921 for UTF-16. Allocation bytes remain approximately
25% / 12% higher. This is an explicitly disclosed compatibility cost, not a
zero-overhead claim or a portable throughput guarantee. Timing is not asserted
in CI. Reproduce the current benchmark with:

```sh
GOMAXPROCS=1 go test -run '^$' -bench '^BenchmarkDecode$' -benchmem -benchtime 1s -count 5 .
```

The paired comparison used the identical benchmark in isolated baseline,
initial-head and optimized binaries; heavy concurrent verification was paused.

The final host-width coordinate guard was measured separately in five
alternating-order one-second pairs on the same host: the optimized pre-guard
reader versus the final guarded reader measured 7.439 / 7.598 ms for UTF-8
(+2.13%) and 11.077 / 11.569 ms for UTF-16 (+4.44%). Median allocation counts
were unchanged (40,181 / 50,073). These separate timing windows must not be
combined into a new claimed baseline percentage. The final guard preserves
exact-or-omitted source coordinates on both 32-bit and 64-bit hosts.
