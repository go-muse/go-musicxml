# API policy

This document describes the public contract for the `v0.1` release line.

## Compatibility

`go-musicxml` follows semantic import versioning. The module is still below
`v1`, so later minor releases may contain intentional breaking changes. Patch
releases in the `v0.1.x` line should preserve source compatibility unless a
security or data-integrity defect requires otherwise.

`MusicXMLVersion` is the supported schema version. `Version` is retained as a
deprecated compatibility alias.

## Documents

`Document` is a sealed interface. Its supported implementations are:

- `*ScorePartwise`
- `*ScoreTimewise`
- `*OpusDocument`

`ScoreDocument` narrows that set to the two score roots. Sealing prevents a
value that `Encode`, `Validate`, or MXL transport cannot handle from satisfying
the interface accidentally.

## Generated model

Most exported types mirror MusicXML 4.0 XSD complex and simple types.

- Optional scalar attributes and elements use pointers.
- Repeated elements use slices.
- XSD enumerations use named string types and constants.
- XSD choices use wrapper structs where exactly one field must be set.
- Types whose child order is semantically significant expose `Content` and
  generated `Add...` methods.
- `Effective...` methods return XSD defaults without mutating the raw field.
- `...MatchesFixed` methods test explicit values against XSD `fixed`
  constraints.

Directly assigning generated fields is supported. Constructors and `Add...`
methods are conveniences, not a separate object model.

## Transport

`Decode` accepts exactly one unqualified `score-partwise`, `score-timewise`, or
`opus` root and rejects non-whitespace content outside it. It does not call
`Validate`.

When the expected root type is known, `DecodeScorePartwise`,
`DecodeScoreTimewise`, and `DecodeOpusDocument` return the corresponding
concrete pointer type. They return `ErrUnsupportedRoot` for any other root.

`Encode` writes one root element without an XML declaration and does not call
`Validate`. MXL encoding adds an XML declaration to stored MusicXML documents.

Encoding preserves the typed model, not original XML formatting. Unknown XML
extensions are not part of the compatibility guarantee.

## Validation

`Validate` returns:

- `nil` for a valid document;
- `*ValidationError` for XSD violations;
- a sentinel argument or document-type error when validation cannot start.

Use `errors.Is(err, ErrInvalidDocument)` for the category and
`errors.As(err, *ValidationError)` to inspect every `ValidationIssue`.
Issue paths use indexed XML-style paths.

## MXL packages

`MXLPackage.RootFiles[0]` identifies `Document`. `Resources` contains every
other regular file except `mimetype` and `META-INF/container.xml`.

Resource order and bytes are preserved; ZIP compression metadata is not.
Encoding validates archive paths and rejects collisions with reserved or
primary paths.

`ResolveOpus` builds a memoized graph. Repeated links share targets, and cycles
are supported. `SyncResolvedOpus` accepts only the graph created for the same,
unchanged package. It commits linked-resource updates atomically.

## Errors

Public sentinel errors are intended for `errors.Is`. `UnsupportedRootError`,
`MXLLinkError`, and `ValidationError` provide structured context and are
intended for `errors.As`.
