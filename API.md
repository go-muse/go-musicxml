# API policy

This document describes the public contract for the `v0.1` release line.

## Compatibility

`go-musicxml` follows semantic import versioning. The module is still below
`v1`, so later minor releases may contain intentional breaking changes. Patch
releases in the `v0.1.x` line should preserve source compatibility unless a
security or data-integrity defect requires otherwise.

`MusicXMLVersion` is the supported schema version.

## Documents

`Document` is a sealed interface. Its supported implementations are:

- `*ScorePartwise`
- `*ScoreTimewise`
- `*OpusDocument`

`ScoreDocument` narrows that set to the two score roots. Sealing prevents a
value that `Encode`, `Validate`, or MXL transport cannot handle from satisfying
the interface accidentally.

`AsScorePartwise`, `AsScoreTimewise`, and `AsOpusDocument` safely narrow a
polymorphic `Document` without a panicking type assertion.

## Generated model

Most exported types mirror MusicXML 4.0 XSD complex and simple types.

- Optional scalar attributes and elements use pointers.
- Repeated elements use slices.
- XSD enumerations use named string types and constants.
- XSD choices use wrapper structs where exactly one field must be set.
- Types whose child order is semantically significant expose `Content` and
  generated `Add...` methods. Their generated `...Contents` slice types append
  recognized elements and discard unknown elements during unmarshalling, so
  every retained entry contains exactly one child variant.
- `Effective...` methods return XSD defaults without mutating the raw field.
- `...MatchesFixed` methods test explicit values against XSD `fixed`
  constraints after applying the attribute type's XML-whitespace policy.

Numeric field types are unchanged by XML lexical normalization. Finite decimals
are written without exponent notation, but the model still uses `float64`
precision. Integers must fit their generated Go type. Exact arbitrary-precision
XSD numbers are outside the model's representational range.

Directly assigning generated fields is supported. Constructors and `Add...`
methods are conveniences, not a separate object model.

## Transport

`Decode` accepts exactly one unqualified `score-partwise`, `score-timewise`, or
`opus` root and rejects non-whitespace content outside it. It does not call
`Validate`. `DecodeWithOptions` changes the XML nesting ceiling; the zero-value
options use the documented safe default.

XML reading rejects duplicate attributes (including collisions after namespace
expansion) and repeated DOCTYPE declarations or declarations inside/after the
root, even in skipped unknown or foreign-namespace subtrees. A single prolog
DOCTYPE is accepted without loading external DTDs; an XML declaration is not
required. If present, the XML declaration must be first, before whitespace,
comments, processing instructions or DOCTYPE; an encoding byte order mark is
allowed before it. Repeated declarations and all other case variants of the
exact processing-instruction target `xml` are rejected. Ordinary targets such
as `xml-stylesheet` remain allowed. These checks also apply to MXL container
metadata and documents parsed while resolving opus links. They do not add DTD
validation or strict MusicXML validation. Empty prefixed namespace declarations
such as `xmlns:p=""` are rejected under Namespaces in XML 1.0; resetting the
default namespace with `xmlns=""` remains allowed.

Duplicate-attribute, DOCTYPE-placement, XML-declaration-placement,
reserved-XML-target and empty-prefix-declaration errors wrap
`*xml.SyntaxError`, available through `errors.As`. Its `Line` is the innermost
XML decoder's detection position after reading the offending token; for a
multiline token this is its end line, not the attribute's start line. Existing
resource-limit and encoding errors retain their separate error contracts.

These are bounded XML-reading checks, not complete prolog or XML-declaration
pseudo-attribute grammar validation. The declaration's contents and DTD syntax
remain subject to the existing `encoding/xml` behavior; no DTD is parsed or
fetched by this additional token check.

When the expected root type is known, `DecodeScorePartwise`,
`DecodeScoreTimewise`, and `DecodeOpusDocument` return the corresponding
concrete pointer type. They return `ErrUnsupportedRoot` for any other root.
Each typed helper has a corresponding `WithOptions` variant.

`Encode` writes one root element without an XML declaration and does not call
`Validate`. It rejects invalid UTF-8 and characters forbidden by XML 1.0 before
writing output, rather than silently replacing them with U+FFFD. `Validate`
reports these failures as representation issues. MXL encoding adds an XML
declaration to stored MusicXML documents.

Encoding preserves the typed model, not original XML formatting. Unknown XML
extensions are not part of the compatibility guarantee. The package decoders
ignore foreign-namespace children and attributes, including names that match
unqualified MusicXML fields; supported XML and XLink attributes are retained.
`Decode` ignores unknown child elements inside a supported root, including
inside generated ordered `Content`, and `Encode` does not reproduce them.

Direct `encoding/xml` unmarshalling of generated types applies the same
ordered-content filtering. It does not apply the package decoder's character
encoding support, configurable XML-depth limit, duplicate-attribute/DOCTYPE/
XML-declaration checks, or foreign-namespace filtering for ordinary struct
fields, nor the encoder's XML-text preflight. Go's
`encoding/xml` matches unqualified struct tags by local name, so `Decode` and
its typed variants remain the recommended document entry points.

`Encode`, `Validate`, and `ResolveOpus` reject cyclic opus models and document
nesting deeper than 4096 elements before walking the model recursively.

## Validation

`Validate` returns:

- `nil` for a valid document;
- `*ValidationError` for XSD violations;
- a sentinel argument or document-type error when validation cannot start.

Use `errors.Is(err, ErrInvalidDocument)` for the category and
`errors.As(err, *ValidationError)` to inspect every `ValidationIssue`.
Issue paths use indexed XML-style paths.

URI values are checked using XSD `anyURI` lexical rules, including XLink
escaping of spaces and non-ASCII characters. Validation leaves the stored
string unchanged and does not check resource existence or accessibility.

## MXL packages

`MXLPackage.RootFiles[0]` identifies `Document`. `Resources` contains every
other regular file except `mimetype` and `META-INF/container.xml`.

Resource order and bytes are preserved; ZIP compression metadata is not.
Container elements and attributes are matched by their expanded XML names;
namespace declarations and foreign-namespace lookalikes cannot select root
files. Root-file paths and media types are interpreted using XSD token
whitespace normalization. Opus hrefs use XSD anyURI whitespace normalization
before URI parsing; literal ZIP resource names and percent-encoded spaces are
unchanged. Encoding validates archive paths and rejects collisions with
reserved or primary paths. Root-file paths and media types must also be
representable as XML 1.0 text, and are checked before whitespace
normalization. Opus links normalize dot segments in both relative and
archive-root-relative paths; references to directories cannot bind to a
same-named regular file.

`DecodeMXLWithOptions` and `DecodeMXLPackageWithOptions` expose the archive,
metadata, primary-document, per-resource, aggregate-resource, and XML-depth
limits. Zero-value fields select the package defaults. XML depth defaults to
256 and may be configured up to 4096; larger values are rejected because they
cannot be decoded safely on every supported platform.

`DecodeMXLScorePartwise`, `DecodeMXLScoreTimewise`, and
`DecodeMXLOpusDocument`, together with their `WithOptions` variants, return a
concrete root type when the expected MXL document kind is known. The `As...`
accessors cover `MXLPackage.Document` and other polymorphic document values.

`ResolveOpus` decodes linked XML resources with the default XML depth limit of
256, independently of the `MXLOptions.MaxXMLDepth` used to read the primary
document. There is currently no separate linked-document depth option.

`ResolveOpus` builds a memoized graph. Repeated links share targets, and
opus-link cycles between archive documents are supported. `SyncResolvedOpus`
accepts only the graph created for the same, unchanged package. It commits
linked-resource updates atomically.

## Errors

Public sentinel errors are intended for `errors.Is`, including
`ErrDocumentTooDeep` and `ErrDocumentCycle` for unsafe programmatically built
models. `UnsupportedRootError`, `MXLLinkError`, and `ValidationError` provide
structured context and are intended for `errors.As`.
