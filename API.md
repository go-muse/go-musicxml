# API policy

This document describes the public contract for the `v0.1` release line.

## Compatibility

`go-musicxml` follows semantic import versioning. The module is still below
`v1`, so later minor releases may contain intentional breaking changes. Patch
releases in the `v0.1.x` line should preserve source compatibility unless a
security or data-integrity defect requires otherwise.

`MusicXMLVersion` is the supported schema version.

`TestPublicAPISnapshot` records every exported identifier of the root,
`validation` and `validation/inventory` packages, including signatures, struct
fields with their XML tags, methods, and the fields of any internal type that
an exported alias re-exports. The golden files live in
`testdata/api/`. An intentional API change updates them in the same change
with `go test -run TestPublicAPISnapshot -update-api .`, so the diff shows
each addition, removal, or changed declaration for review.

Error message text is not part of the API; match errors with `errors.Is` and
`errors.As`.

### Interfaces implemented by callers

`DictionarySource` and `validation.ResourceResolver` are implemented by
callers, so their method sets are frozen. New capabilities will be offered as
separate optional interfaces discovered with type assertions. A resolver that
receives a request branch it does not recognize should return the request ID
with `State: validation.EvidenceUnsupported`, a `Reason`, and no payload.

`DictionarySourceFunc`, `validation.ResourceResolverFunc` and `DTDSourceFunc`
adapt plain functions to these interfaces. A nil `DictionarySourceFunc` is a
typed-nil source: the importer supplies nothing from it, so its selections stay
unavailable, and it is an explicit override rather than a request for the
bundled defaults. `validation.Known` and
`validation.Unknown` build `Evidence` values, and `validation.Report.String`
returns the text that `Report.Format` writes.

### Comparable structs

Several option and value structs are comparable today, including
`DecodeOptions`, `EncodeOptions`, `MXLOptions`, `AssessmentOptions`,
`DecodeAssessmentOptions`, `PartDefinition`, `MXLRootFile`, and
`ValidationIssue`. New fields added to these structs are kept comparable
(scalars or pointers), so `==` on them keeps compiling. Comparability of
other structs, in particular generated model types, is not guaranteed; see
[Generated model compatibility](#generated-model-compatibility).

`validation.Integer`, `validation.Decimal`, and `validation.Rational` are not
comparable: `==` on them, or on structs that contain them such as
`validation.Atomic` and `validation.ContextPosition`, does not compile, and they
cannot be map keys. Compare them with their `Equal` and `Cmp` methods, and
compare containing structs field by field.

## Migrating to v0.2

v0.2 removes duplicates and tightens types; the project has no compatibility
promise before v1. Each row names what changed and what to use instead.

| Before v0.2 | v0.2 |
| --- | --- |
| A typed-nil reader or writer, such as a nil `*os.File`, panicked or failed with a read error | Every reader and writer entry point returns `ErrNilReader` or `ErrNilWriter`; `validation.Report.Format` returns `ErrInvalidConfiguration` |
| `(*validation.DictionarySet)(nil).Close()` returned `ErrInvalidConfiguration` | It returns nil; a zero set is still rejected |
| `==` on `validation.Integer`, `Decimal`, `Rational` compiled and compared pointers | It does not compile; use `Equal` or `Cmp`. The same holds for the structs that contain them: `Atomic`, `AttributeFact`, `ChildValueRequirement`, `ContainerContextResult`, `ContextPosition` (and its alias `SpanPosition`), `LayoutContextResult`, `NavigationInstrumentState`, `NavigationListeningStatement`, `NavigationPassFrame`, `NavigationPerformance`, `NodeEnd`, `PackageImageAdvice`, `ResourceAttribute`, `ResourceImageContext`, `ResourceInstrumentContext`, `ResourceLinkContext`, `Scalar`, `SoundMappingContextResult`, `TimingAdjustment`, `TimingBend`, `TimingDonor`, `TimingPercent` and `TimingTuning` |
| Generated fields of schema type `xs:positiveInteger`, `xs:nonNegativeInteger`, `xs:integer` were `uint64`/`int64` | They are `PositiveInteger`, `NonNegativeInteger`, `Integer`; convert with `PositiveInteger(n)` and `uint64(v)`, or `Integer(n)` and `int64(v)` |
| 25 generated structs, such as `Note`, `Attributes` and `Forward`, had `MarshalXML`/`UnmarshalXML` | They have none; encoding/xml handles them through the field types |
| `PlayContent.Ipa`, `Play.AddIpa` | `PlayContent.IPA`, `Play.AddIPA` |
| `validation.Session.Open`, `Session.Close` | `Session.OpenNode`, `Session.CloseNode`; `SessionError.Operation` reports the new names |
| `validation.ContextConsumerSpec`, `validation.TimelineConsumerSpec` | `validation.ConsumerSpec` |
| `validation.UnsupportedRootError` | `validation.ExcludedRootError` |
| `validation.MusicXML40Inventory`, `validation.Inventory`, `validation.InventoryX` | `inventory.MusicXML40`, `inventory.Snapshot`, `inventory.X` in package `validation/inventory` |
| `validation.Schema*` were aliases of internal types | They are types of package `validation` with the same fields |
| `validation.SchemaName` | `validation.QName` |
| The MusicXML-specific plan constructors, the `Source*` methods of `Engine` and `Session`, `CompiledPackageCatalog.Begin`, the methods of `PackageSession` and `AllocationSize` | Not exported; see [Adapter entry points](#adapter-entry-points). The built-in profiles and entry points such as `InspectXML`, `InspectMXL` and `AssessMXLPackage` cover them |

## Scenarios

Three scenarios cover loading and checking MusicXML. Each has one function per
input kind, and each can be called with zero options. Default budgets bound the
work and memory of every assessment. The semantic profiles are costly, so
larger scores can need larger `Budgets`; a refused budget is an operational
error, even in the lenient mode.

| Scenario | What it does | Functions | Validation lines |
| --- | --- | --- | --- |
| 1. Load without validation | Reads well-formed XML into the Go model. XSD and MusicXML rules are not checked. | `Decode`, `DecodeMXL`, `DecodeMXLPackage`, the typed decoders and their `WithOptions`/`Context` variants | 1 |
| 2. Strict loading | Returns a valid MusicXML 4.0 structure or an error. By default the document is returned only when everything is accepted; in the lenient mode it is always returned with the reports. | `DecodeAssessed` for XML, `DecodeMXLAssessed` for MXL | 1, 2 while reading, 3 after conversion |
| 3. Check a model | Checks a structure built or edited in Go: exactly the line-3 assessment of scenario 2, with the bundled required dictionaries. | `AssessModel`; `AssessMXLPackage` for an MXL package | 3 |

### Validation lines

The three scenarios share three validation lines, named by the
`ValidationLine` type:

1. **Decoder** (`LineDecoder`): XML and document correctness inside the
   decoder, such as well-formedness, encoding, entities and the DOCTYPE. It
   has no rules.
2. **Source** (`LineSource`): the MusicXML 4.0 and XSD requirements that the
   loaded structure cannot show, because decoding and conversion lose or
   change the data they read. `ProfileSourceLine` checks XSD on the original
   bytes and their DTD infoset (undeclared elements and attributes, lexical
   forms, order, DTD-supplied values), source provenance, schema hints, and
   the numeric values the Go model cannot hold (see
   [Model representability](#model-representability)).
   `ProfileMXLSourceLine` adds the original archive bytes (container path,
   mimetype entry, entry names, compression), the first rootfile's media type,
   which conversion refuses when it is not MusicXML, and schema-hint pairing,
   and
   assesses the first rootfile document, the one conversion decodes, with
   `ProfileSourceLine`; it follows no resource links and leaves later
   rootfiles, which the package keeps as bytes, to line 3.
3. **Model** (`LineModel`): the checks on the loaded structure, the bulk of
   the MusicXML rules: `ProfileModelSemantics` for a document,
   `ProfileMXLModelSemantics` for the current state of a whole MXL package,
   including the linked files the package keeps as bytes.

Lines 2 and 3 do not check the same thing. `RuleLine` reports the line that
owns each rule revision of every public profile, and `AllowedLineOverlap`
lists the only accepted overlap with its reasons:

- the six XSD operations, which line 2 runs whole on the original bytes,
  because cutting out the subchecks the structure shows would save little
  (about 3% of `DecodeAssessed`, by the design estimate);
- the archive-byte checks, which `ProfileMXLModelSemantics` applies only to
  the files a converted package keeps as bytes, such as a nested raw MXL;
- the first rootfile's media type: line 2 decides it before conversion, which
  refuses a non-MusicXML first rootfile, so that strict loading reports a
  violation instead of a conversion error; line 3 decides it from the current
  `RootFiles`, which keep it as written;
- the source checks of those retained files: line 3 assesses them with source
  profiles, because for them the bytes are the structure. Line 2 assesses only
  what conversion consumes, the container and the first rootfile, so no file
  gets these checks on both lines.

Tests fail when a rule has no line, a line profile requires another line's
rule, or the overlap changes.

Only line 3 reads dictionaries. Both loaders obtain them when line 3 starts;
the default bundled set is imported once per process and shared. When line 3
runs, a dictionary failure shows in `ModelLine`; in every case the document is
withheld. Splitting the lines cut the cost of strict loading by about a third,
and the shared dictionary set and real-layout context accounting by another
fifth to a quarter: `DecodeAssessed` needs about 9.7 million rule steps and 27 MiB of
buffer for the 31 KB `01a-Pitches-Pitches.xml` and 11.4 million steps and 35
MiB for the 42 KB `41c-StaffGroups.xml` of the MusicXML test suite, both within
the default budgets (20 million steps, 64 MiB), where it needed 21.7 and 24.9
million steps before the split ([measurements][loading-budgets]).

`InspectXML`, `InspectMXL`, `InspectXMLWithResources`,
`AssessModelWithResources` and `Validate` are diagnostic tools outside the
scenarios. With an empty profile `InspectXML` still runs
`ProfileSourceSemantics`, every source rule, including the line-3 rules that
strict loading checks on the model; `InspectXML` with `ProfileSourceLine` and
`InspectMXL` with `ProfileMXLSourceLine` give the line-2 reports of strict
loading.

Scenario 2 is line 2 plus scenario 3: `AssessModel` on a document that
`DecodeAssessed` returned gives the same report as `DecodeResult.ModelLine`,
and `AssessMXLPackage` on the package that `DecodeMXLAssessed` returned, with
the same `Assessment` options and an empty `Graph.Profile`, gives the same
outcomes as `DecodeMXLResult.ModelLine`.
This holds while the budgets are not exhausted: the loader's model stage shares
its account with the source report, so a tight diagnostic or buffer budget can
truncate or refuse the two differently.

**Current state of an MXL package.** `AssessMXLPackage` assesses a whole
package, possibly edited in Go, as `EncodeMXLPackage` would write it:

- The primary `Document` gets the model assessment. The Go model cannot
  represent `xsi:schemaLocation` or `xsi:noNamespaceSchemaLocation`, and the
  encoders write neither, so schema-hint pairing
  (`MX40-PACKAGE-schema-hint-pairs`, revision 2) does not apply to it. Hints of
  the original source are not reconstructed; `InspectMXL` reports them.
- Rootfiles, links and other package rules are evaluated from the current
  fields.
- Retained raw resources that are reached, such as later rootfiles and link
  targets, keep their source assessment from their bytes, including their
  schema hints. Unreferenced resources are recorded as entries only, as in
  `InspectMXL`.
- Raw ZIP checks (mimetype storage, extra fields and leading bytes, the
  container path, compression) do not apply: `EncodeMXLPackage` always writes a
  stored mimetype entry first, without an extra field, followed by
  `META-INF/container.xml`, which it builds from `RootFiles` and which is not
  assessed as a document.

An unedited package decoded from an archive that `InspectMXL` accepts
satisfies `AssessMXLPackage`, unless conversion lost a fact the source supplied:
decoding does not apply internal-DTD attribute defaults, for example, so a part
whose `id` only such a default supplies has none in the model, and the model
stage of `DecodeMXLAssessed` refuses it. After edits, the package findings and
acceptance agree with `InspectMXL` on the bytes `EncodeMXLPackage` writes; only
the archive assessment also covers the container document. Role,
instrument-association and image facts still need a resolver, as for the
original archive.

```go
// Scenario 2: strict loading.
result, err := musicxml.DecodeAssessed(ctx, reader, musicxml.DecodeAssessmentOptions{})
if err != nil {
	return err // rejected, or an operational failure
}
score, _ := musicxml.AsScorePartwise(result.Document)

// Scenario 2, lenient: the document comes back with its reports.
result, err = musicxml.DecodeAssessed(ctx, reader, musicxml.DecodeAssessmentOptions{Lenient: true})
if err != nil {
	return err // operational failure only
}
if !result.Accepted {
	fmt.Println(result.SourceLine.String(), result.ModelLine.String())
}

// Scenario 3: check a model built or edited in Go.
report, err := musicxml.AssessModel(ctx, score, musicxml.AssessmentOptions{})
```

**Lenient mode.** `DecodeAssessmentOptions.Lenient` and
`DecodeMXLAssessmentOptions.Lenient` return the converted document even when
an assessment finds violations or is incomplete. In that case `err` is nil,
`Accepted` is false, and line 3 runs even after line-2 findings, so that both
reports are complete. Operational failures still return an error and
no document: malformed XML, I/O, cancellation, resource limits, conversion
failure, dictionary import failure and invalid configuration. In the default
strict mode the document is returned exactly when `Accepted` is true.

**Options.** Everything else is a field of the options:

- `AssessmentOptions.Profile` selects a narrower profile. An empty `Profile`
  selects the full semantic profile of each entry point:
  - `ProfileSourceSemantics` for `InspectXML`;
  - `ProfileModelSemantics` for `AssessModel`;
  - `ProfileResourceGraphSemantics` for `InspectXMLWithResources` and
    `AssessModelWithResources`;
  - `ProfileMXLSourceSemantics` for `InspectMXL`;
  - `ProfileMXLModelSemantics` for `AssessMXLPackage`.

  These defaults use the bundled required dictionaries, also when
  `Dictionaries` has a nil `Source` and no selections. Before this release an
  empty profile was a configuration error for these functions.
  `DecodeAssessed` and `DecodeMXLAssessed` accept only an empty profile or
  their line-2 profile, `ProfileSourceLine` or `ProfileMXLSourceLine`.
- `AssessmentOptions.Dictionaries` selects pinned dictionary capabilities. It
  replaces the bundled required data of an empty profile, and with an explicit
  profile it requires a dictionary or semantic one.
- `AssessmentOptions.Budgets`, `AssessmentOptions.DTD`, and the resource fields
  of `LinkedAssessmentOptions` and `MXLAssessmentOptions` keep their meaning.

**Advanced tools.**

- `InspectXML` and `InspectMXL` report on an original source without building a
  model.
- `AssessModel` on an MXL package's `Document` checks that document alone,
  without the package rules and the other resources that `AssessMXLPackage`
  covers.
- `InspectXMLWithResources` and `AssessModelWithResources` assess a graph of
  linked documents. They return a `validation.AssessmentBundle`, not a single
  report, and take their dictionaries from `Graph.Dictionaries`.
- `Validate` is the fast XSD-only check of a model and is unchanged. It is not
  scenario 3: it checks the XSD model profile only, without the MusicXML
  semantic rules.

**Strict MXL loading.** `DecodeMXLAssessed` works in four steps:

1. It checks every option, then reads the archive once into a buffer and
   indexes it, metered as in `InspectMXL`: reads, buffer growth and the ZIP
   inventory are admitted on the shared account before they happen. A reader
   failure or a corrupt archive matches `ErrInvalidMXL`, as in `DecodeMXL`,
   and `SourceLine` is then the zero bundle. An archive layout the assessment
   does not process, such as one with a preamble, is reported in `SourceLine`, as by
   `InspectMXL`; its terminated assessment returns an error that matches
   `ErrIncompleteAssessment`, in both modes.
2. It runs validation line 2 on the original archive with
   `ProfileMXLSourceLine`: the archive-byte checks, the original first
   rootfile media type and schema-hint pairing, with the first rootfile
   document assessed by `ProfileSourceLine`. It follows no resource links and
   asks the caller's resolver for nothing, whatever `Dependencies` select, and
   leaves later rootfiles to line 3.
3. In the default strict mode only after line 2 accepts the archive, and in
   the lenient mode after any line-2 stage without an operational failure, it
   converts the buffered bytes as `DecodeMXLPackageContext` does, except that
   the root document and `META-INF/container.xml` are decoded from the DTD
   infoset that line 2 assessed, as `DecodeAssessed` does: DTD defaults reach
   the model and `RootFiles`, and internal-subset entities expand. DTDs
   resolve from `Graph.DTD` first, as in line 2.
4. It runs validation line 3 on the current state of the converted package as
   `AssessMXLPackage` does with `ProfileMXLModelSemantics`, with the same
   resources, resolver and dependencies, and returns that bundle as
   `DecodeMXLResult.ModelLine`. Line 3 follows the resource links and assesses
   the linked files the package keeps as bytes.

Both assessments share one account bounded by
`Assessment.Graph.Budgets`. The archive bytes stay charged to it until
conversion finishes, and the source bundle until the call returns, so the later
stages are bounded by the combined live storage. A rejected source bundle is
classified from its authoritative counts (see `AssessmentBundle.Failures`):
failed rule evaluations match `ErrInvalidDocument` even when their diagnostics
were truncated, and anything else that keeps the bundle from being satisfied
matches `ErrIncompleteAssessment`. Conversion parses the buffered bytes again,
so the converted package is bounded by the MXL limits, as for
`DecodeMXLPackage`.

**Removed functions.** The functions below and their option types
`DictionaryAssessmentOptions`, `DecodeDictionaryAssessmentOptions`,
`LinkedDictionaryAssessmentOptions` and `MXLDictionaryAssessmentOptions` were
removed: they duplicated the entry points above. The replacement gives the same
result for every profile the removed function accepted.

| Removed | Replacement |
| --- | --- |
| `InspectXMLWithDictionaries(ctx, r, DictionaryAssessmentOptions{Assessment: a, Dictionaries: d})` | `InspectXML(ctx, r, a)` with `a.Dictionaries = &d` |
| `AssessModelWithDictionaries(ctx, doc, DictionaryAssessmentOptions{Assessment: a, Dictionaries: d})` | `AssessModel(ctx, doc, a)` with `a.Dictionaries = &d` |
| `DecodeAssessedWithDictionaries(ctx, r, DecodeDictionaryAssessmentOptions{Decode: o, Assessment: &DictionaryAssessmentOptions{Assessment: a, Dictionaries: d}})` | `DecodeAssessed(ctx, r, DecodeAssessmentOptions{Decode: o, Assessment: &a})` with `a.Dictionaries = &d` |
| `InspectXMLWithResourcesAndDictionaries(ctx, r, LinkedDictionaryAssessmentOptions{Resources: l, Dictionaries: d})` | `InspectXMLWithResources(ctx, r, l)` with `l.Graph.Dictionaries = &d` |
| `AssessModelWithResourcesAndDictionaries(ctx, doc, LinkedDictionaryAssessmentOptions{Resources: l, Dictionaries: d})` | `AssessModelWithResources(ctx, doc, l)` with `l.Graph.Dictionaries = &d` |
| `InspectMXLWithDictionaries(ctx, r, MXLDictionaryAssessmentOptions{Resources: m, Dictionaries: d})` | `InspectMXL(ctx, r, m)` with `m.Graph.Dictionaries = &d` |
| `AssessMXLPackageWithDictionaries(ctx, p, MXLDictionaryAssessmentOptions{Resources: m, Dictionaries: d})` | `AssessMXLPackage(ctx, p, m)` with `m.Graph.Dictionaries = &d` |

**Binary contents.** Any binary that references `DecodeAssessed`,
`DecodeMXLAssessed`, `InspectXML`, `AssessModel`, `InspectXMLWithResources`,
`AssessModelWithResources`, `InspectMXL` or `AssessMXLPackage` embeds the
bundled dictionary data. Distribute it with the notices listed under
[bundled pinned dictionary data](#bundled-pinned-dictionary-data).
`Decode`, `DecodeMXL`, `Validate` and `Encode` do not embed it.

## Documents

`Document` is a sealed interface. Its supported implementations are:

- `*ScorePartwise`
- `*ScoreTimewise`
- `*OpusDocument`

`ScoreDocument` narrows that set to the two score roots. Sealing prevents a
value that `Encode`, `Validate`, `AssessModel`, or MXL transport cannot handle
from satisfying the interface accidentally.

`AsScorePartwise`, `AsScoreTimewise`, and `AsOpusDocument` safely narrow a
polymorphic `Document` without a panicking type assertion.

## Generated model

Most exported types mirror MusicXML 4.0 XSD complex and simple types. Their doc
comments, and those of element fields, include the schema's `xs:documentation`
text under the attribution at the top of each generated file; documentation of
groups and attribute groups is not repeated at every use.

- Optional scalar attributes and elements use pointers.
- Repeated elements use slices.
- Named XSD enumerations use named string types and constants. A restriction
  enumeration also has `IsKnown()`, an exact match against its constants, and
  `Values()`, a new slice of them in schema order, for example
  `YesNo("").Values()`. Union types with enumerated members, such as
  `NumberOrNormal`, have constants for those members only and no helpers.
- Anonymous enumerations, the XLink `show`, `actuate` and `type` attributes and
  `xml:space`, stay plain strings with untyped constants: `XLinkShowNew`,
  `XLinkActuateOnRequest`, `XLinkTypeSimple`, `XMLSpacePreserve` and the
  others. Union types such as `FontSize` are named strings without constants.
- Repeated XSD choices use `...Content` wrapper structs where exactly one
  field must be set. `Value()` returns the selected variant, a pointer, and
  `ElementName()` its XML element name; both are zero unless exactly one variant
  is set. Variants can share a Go type, for example most `DynamicsContent`
  variants are `*Empty`, so switch on `ElementName()` or on the type of
  `Value()` with a default branch. A non-repeated choice is flattened into mutually
  exclusive optional fields of the owning type, for example `Note.Pitch`,
  `Note.Unpitched`, and `Note.Rest`; `Validate` reports more than one set.
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
XSD numbers are outside the model's representational range: strict loading
rejects a value the model cannot hold unchanged (see
[Model representability](#model-representability)). Ordinary decoding reads an
empty decimal element or attribute as 0.

A field whose schema type is the built-in `xs:positiveInteger`,
`xs:nonNegativeInteger` or `xs:integer` has the generated type
`PositiveInteger`, `NonNegativeInteger` or `Integer` (underlying `uint64`,
`uint64` and `int64`). These types decode every XSD lexical form that fits,
such as `+1` and `-0`; the range facets, such as positiveInteger's exclusion
of zero, are checked by validation. Simple types derived from these built-ins,
such as `StaffNumber`, keep their own names.

Directly assigning generated fields is supported. Constructors and `Add...`
methods are conveniences, not a separate object model. Generated `Add...`
methods store the pointer they receive. The hand-written `AddMeasure` methods
store a shallow copy of their argument: reassigning its fields or slice
headers afterwards does not reach the stored measure, while changes made
through its shared slices and pointers do. Populate a measure before adding it
or edit the stored element. Constructors leave `XMLName` empty, while decoding fills
it; `Encode` ignores `XMLName`.

### Generated model compatibility

The generated types follow the bundled schema. When a later MusicXML schema
is adopted, these changes are treated as additive:

- new optional fields, attributes, and generated methods such as
  `Effective...` and `...MatchesFixed`;
- new `...Content` variants and new types;
- new enumeration constants.

These properties are not guaranteed and may change with the schema:

- comparability with `==` and use as map keys;
- unkeyed composite literals of generated structs;
- exhaustive handling of `...Content` variants or enumeration values without
  a default branch;
- which generated types implement `xml.Marshaler`, `xml.Unmarshaler`,
  `xml.MarshalerAttr`, or `xml.UnmarshalerAttr`. Some types have whole-struct
  marshal methods only because they contain decimal or derived integer fields;
- embedding a generated type in a caller-defined struct to extend its XML.
  Promoted marshal methods take over the outer struct and drop its own fields
  and `XMLName`, so embed nothing or wrap the value in a named field.

Schema changes that alter a field's shape, such as optional to repeated
(pointer to slice), required to optional (value to pointer), an element of
type `empty` gaining its own type, a plain type becoming ordered `Content`,
or a string becoming an enumeration, are breaking. They are released only in
a minor version with migration notes. `TestPublicAPISnapshot` makes every
such change, including a field whose XML tag is reassigned from an attribute
to an element, visible in review.

## Transport

`Decode` accepts exactly one unqualified `score-partwise`, `score-timewise`, or
`opus` root and rejects non-whitespace content outside it. Boundary whitespace
must be literal XML `S`: space, tab, carriage return or line feed. References
and CDATA sections are forbidden outside the root, even if they decode to
whitespace or no characters; they remain supported inside elements. It does not
call `Validate`. `DecodeWithOptions` changes the XML nesting ceiling; the zero-value
options use the documented safe default.

XML reading rejects duplicate attributes (including collisions after namespace
expansion) and repeated DOCTYPE declarations or declarations inside/after the
root, even in skipped unknown or foreign-namespace subtrees. A single prolog
DOCTYPE is accepted; external DTDs are never fetched (see
[External DTD subsets](#external-dtd-subsets)). An XML declaration is not
required. If present, the XML declaration must be first, before whitespace,
comments, processing instructions or DOCTYPE; an encoding byte order mark is
allowed before it. Repeated declarations and all other case variants of the
exact processing-instruction target `xml` are rejected. Ordinary targets such
as `xml-stylesheet` remain allowed. These checks also apply to MXL container
metadata and documents parsed while resolving opus links. They do not add DTD
validation or strict MusicXML validation. Empty prefixed namespace declarations
such as `xmlns:p=""` are rejected under Namespaces in XML 1.0; resetting the
default namespace with `xmlns=""` remains allowed.

The shared source scanner checks XML 1.0 declaration and local DTD grammar,
namespace closure and original encoding before model filtering. XML defects wrap
`*xml.SyntaxError`; resource and unavailable-encoding/DTD semantics retain their
separate typed causes. Well-formed input that relies on a deliberately
unimplemented feature, such as DTD entity expansion or an encoding without a
decoder, matches `ErrUnsupportedXMLFeature`; for compatibility it also matches
`ErrIncompleteAssessment`. `xml.SyntaxError.Line` identifies a decoded-source
recognition position, not an attribute-start location. Source assessment findings
supply explicit original-byte positions where available.

Truncated processing instructions and XML declarations are syntax errors,
including an unfinished processing instruction after a complete root. These
errors do not expose a decoded document or become successful end-of-input.

Literal attribute tabs and line endings normalize to spaces, while numeric
references preserve their referenced whitespace. Contradictory BOM/encoding
declarations, undeclared prefixes, malformed declaration/DTD grammar and illegal
reserved namespace bindings are now rejected. Ordinary decoding does not apply
DTD attribute defaults or internal-subset entity replacement text; it uses a
resolved external subset only as described under
[External DTD subsets](#external-dtd-subsets). Unsupported declaration
WFC/infoset dependencies are explicit assessment limitations. See the
[source parser scope](docs/validation/source-parser-scope.md) for the precise
syntax, compatibility, encoding and DTD boundary.

When the expected root type is known, `DecodeScorePartwise`,
`DecodeScoreTimewise`, and `DecodeOpusDocument` return the corresponding
concrete pointer type. They return `ErrUnsupportedRoot` for any other root.
Each typed helper has a corresponding `WithOptions` variant.

`DecodeContext(ctx, reader, options)` decodes like `DecodeWithOptions` with
cooperative cancellation: it checks `ctx` before each read from the reader and
while parsing already buffered input, and stops at the next check after `ctx`
is done. A canceled call returns an error that matches `ctx.Err()`; a nil
`ctx` is a `*validation.ConfigurationError`. A `Read` call that blocks cannot
be interrupted, so give a reader that can block its own deadline, use
context-aware I/O, or close it to unblock the call. The existing functions keep
using `context.Background()`. There are no typed context variants; narrow the
result with `AsScorePartwise`, `AsScoreTimewise`, or `AsOpusDocument`.

`Encode` writes one root element without an XML declaration and does not call
`Validate`. `EncodeWithOptions` performs the same checks and can add the
standard XML declaration (`EncodeOptions.XMLDeclaration`) and indentation
(`EncodeOptions.Indent`, spaces and tabs only). Zero options write exactly the
bytes of `Encode`; invalid options match `ErrInvalidEncodeOptions`. It rejects invalid UTF-8 and characters forbidden by XML 1.0 before
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

Further differences of direct `encoding/xml` use:

- `xml.Marshal` of a cyclic opus model recurses until the Go runtime aborts
  with a fatal stack overflow, which cannot be recovered. `Encode` returns
  `ErrDocumentCycle` instead.
- Only the three root types carry an `XMLName`. `xml.Marshal` of another
  generated value, such as a `Note`, uses the Go type name as the element
  name; use `xml.Encoder.EncodeElement` with an explicit start element.
- `xml.Unmarshal` accepts a namespace-qualified root and fills plain fields,
  but ordered `Content` matches only unqualified names and stays empty.
  `Decode` rejects such a root with `ErrUnsupportedRoot`.
- The `XMLName` tag fixes the root element name, so a containing struct field
  tagged with a different element name fails to marshal.

`EncodeWithOptions` writes an XML declaration or indented output with the
same checks as `Encode`; prefer it to configuring an `xml.Encoder` directly.

### External DTD subsets

The official MusicXML 4.0 DTD modules are bundled in `schema/musicxml-4.0`
next to the XSD files and pinned by SHA-256. A DOCTYPE external subset is
resolved, never fetched:

- A public identifier listed in the official
  `schema/musicxml-4.0/catalog.xml` selects its bundled file. As in that
  catalog, the identifiers of MusicXML 1.0 through 3.1 resolve to the
  backward-compatible 4.0 files. The MIDI DTD is not bundled.
- Otherwise an official system identifier `http(s)://www.musicxml.org/dtds/`
  (or `musicxml.org/dtds/`) selects the file of the same name, which is the
  current 4.0 version.
- `DTDOptions.Source` is asked first. Returning `ErrDTDNotFound` for the
  DOCTYPE external subset selects the bundled closure, including its modules.
  When the source supplies the external subset, each external parameter entity
  it references is requested from the source and then from the bundled files;
  `DTDRequest.BaseSystemID` names the external entity that was being read when
  its declaration was parsed, for resolving a relative system identifier (XML
  4.2.2). Any other source error is reported as
  a `*DTDError`, which matches `ErrDTDUnavailable`, as do caller bytes that are
  not a well-formed UTF-8 external subset, including a text declaration whose
  version is not `1.` followed by digits or whose encoding name is malformed
  or not UTF-8. A `*DTDError` never matches `*xml.SyntaxError`. One caller
  entity may not exceed 16 MiB. A typed-nil source is ignored.
- Any other identifier leaves the external subset unread, as before.

The source is configured through `DecodeOptions.DTD` for ordinary and assessed
decoding, `MXLOptions.DTD` for MXL decoding and `AssessmentOptions.DTD` for
assessments. Where two of them apply, the narrower one wins:
`DecodeAssessmentOptions.Decode.DTD` over `AssessmentOptions.DTD`, and
`MXLAssessmentOptions.Graph.DTD` over `MXLAssessmentOptions.MXL.DTD`. The
bundled closure is parsed once per process; an external subset supplied by the
source is parsed for every document that names it, under the assessment's
budgets when one reads it.

Ordinary decoding (`Decode`, the typed decoders, MXL decoding and opus link
resolution) uses a resolved external subset for two things only: general
entities whose replacement text is one character, such as the ISO Latin
`&eacute;` of the MusicXML DTDs, and defaulted namespace declarations, such as
the `xmlns:xlink` default that lets an opus omit its declaration. A namespace
default is normalized as an attribute value (XML 3.3.3): character references
and references to internal entities of the external subset are replaced, and a
default that is not a well-formed attribute value is not applied. Other DTD
attribute defaults are not added to the model. A document that decoded before
decodes to the same tokens, with one exception: a defaulted namespace
declaration now applies to an element that omits it even where an ancestor
binds the same prefix to another namespace, as XML 3.3.2 requires. With a
resolved external subset, a prefix that neither the document nor the DTD binds
is reported as an `*xml.SyntaxError`, as it is without a DOCTYPE, instead of
`ErrUnsupportedXMLFeature`.

Ordinary decoding does not use the external subset when the internal subset
contains entity declarations, attribute-list declarations or parameter-entity
references, because it does not interpret them, and does not expand its
entities in a document declared `standalone="yes"` (XML 4.1, Entity Declared).
It ignores identifiers longer than 4096 bytes. In these cases the previous
behaviour remains. A caller source error is reported only when one of these
features actually needs the DTD.

Source profiles that read DTD declarations, including `ProfileSourceLine`
used by `DecodeAssessed` and `ProfileSourceSemantics`, read the resolved external subset after the internal
subset with XML first-binding precedence. Parameter entities declared in the
internal subset are in scope while the external subset is read, so they can
select its conditional sections or replace its parameter entities. An external
subset that the source supplies, or that internal parameter entities can
customize, is parsed into the document's declarations under the assessment's
budgets, including `MaxScalarBytes` for every literal and name and `MaxDepth`
for conditional-section and content-model nesting; `MaxBufferedBytes` holds the
normalized text of each caller entity while it is read, as well as the
declarations, frames and replacement maps. The bundled closure is
prepared once; importing it charges the document's buffered bytes and checks
its longest string against `MaxScalarBytes` and its deepest nesting against
`MaxDepth`.

The well-formedness constraints on attribute defaults apply to the external
subset as to the internal one, to every read definition, including duplicates
that do not bind: a general entity must be declared before a direct reference
in a default, the internal subset counting as earlier (XML 2.8, 4.1), and may
be neither external nor unparsed. A defect in an external declaration is a
`*DTDError`; one in the internal subset remains an `*xml.SyntaxError`. Its
attribute defaults and entities form the effective source infoset
and, as for internal-subset declarations, reach the model converted by assessed
loading. There they become ordinary field values: the model cannot tell a DTD
default from a written value, and `Encode` writes it out. Source origins locate a declaration from an external entity by that
entity's system identifier in `validation.Location.Entry`, with
`validation.CoordinateSpaceExternalDTD` line, column and byte coordinates in its
text after the byte order mark is removed and line ends are normalized, any text
declaration included. A declaration generated by a parameter
entity is located where its text is written, which can be the literal of that
parameter entity or of one it includes in another module; for example the
`default-x` attribute of `note` is located in the `%position;` literal of
`common.mod`. A declaration whose text comes from an internal-subset parameter
entity, or from past the 256th segment of a parameter entity's replacement map
(each character reference starts a segment), and a declaration site in an
entity whose identifier is longer than 4096 bytes, are unknown evidence, so
such an assessment is incomplete. Profiles that do not read DTD declarations, such as
`ProfileSourceXSDPreview` and `ProfileMXLSource`, keep reporting the external
subset as unread whatever the options say.

`Encode` and `ResolveOpus` reject cyclic opus models and document nesting deeper
than 4096 elements before recursive traversal. `Validate` preserves that legacy
depth/cycle recognition through the bounded iterative model walker.
`AssessModel` uses its own configurable finite assessment budgets; its default
`MaxDepth` is 1024.

## Validation

`Validate` and the three root `Validate` methods now traverse the current model
directly using the shared XSD kernel. They do not call `Encode`, marshal methods
or an XML parser. Their scope stays XSD-only: the new local musical clauses are
selected only by an explicit local profile. `Decode` and `Encode` still do not
validate automatically.

`Validate` returns:

- `nil` when the selected XSD and model-representation checks accept the current
  document;
- an error containing `*ValidationError` for proven XSD violations or legacy model
  representation failures, possibly joined with a technical error;
- a sentinel argument or document-type error when validation cannot start;
- a technical error when required metadata or contextual facts are unavailable.
  Pure capability/resource incompleteness does not match `ErrInvalidDocument`,
  apart from the preserved legacy depth exception below; incompleteness is not
  evidence that the document is valid or invalid.

Use `errors.Is(err, ErrInvalidDocument)` for the legacy category and
`errors.As` with a `*ValidationError` target to inspect retained `ValidationIssue`
details. Individual diagnostic previews are bounded and are not an exhaustive trace.
Every operation that reports issues fills them the same way: `Constraint` is the
failure reason code, `Message` the offending value when the rule reports one,
and `Rule` the ID of the violated rule. `DecodeAssessed` previously put the rule
ID in `Constraint` and the reason code in `Message`. `String` shows the
constraint when the message is empty.
Legacy issue locations retain XML-style structural display paths; representation
messages also identify Go fields. These paths do not describe original XML.
`ErrNilDocument`, `ErrUnsupportedDocument`, `ErrDocumentCycle` and
`ErrDocumentTooDeep` remain recognizable where applicable. Depth and cycle errors
still match `ErrInvalidDocument` in this facade. The legacy depth ceiling remains
4096. Other assessment default quotas do not apply to this compatibility facade:
document/scalar/work/diagnostic ceilings use `math.MaxInt64`, while checked
accounting overflow and host-sized allocation guards remain active. This restores
the former size/work contract, not a memory or throughput guarantee. Memory and
work grow with the model. Use `AssessModel` with explicit finite budgets for
untrusted models and independent normative, model and resource outcomes; its
finite defaults are unchanged.

A finite decimal field means the shortest decimal that reads back to the same
`float64`, the value a decoder read from that text, including at bounds,
enumeration, fixed and digit facets. For example, current `float64(0.1)` is
decimal `1/10`, and `0.1 + 0.2` computed in Go is `0.30000000000000004`.
Large finite values are not assessed through an encoder's spelling.
Unsupported numeric lexical patterns remain incomplete, without inventing a
source lexeme. No serialization fallback is retained.

### Model representability

The generated model holds decimals as `float64` and integers in fixed-width Go
types. Strict loading checks, in the source, that each numeric value the model
holds survives conversion unchanged: rule `MX40-SOURCE-REPRESENTABLE-musicxml`
(and `-opus`), a model-category rule of `ProfileSourceLine`, validation line 2.
It fails for:

- a decimal whose shortest `float64` form is a different number: more than 17
  significant digits, as in `1.00000000000000000001`, an underflow to zero or
  an overflow;
- an integer outside its Go type, such as a `xs:positiveInteger` beyond
  `uint64`.

This is a limit of the model, not of MusicXML: the source stays conformant,
and the report names the value's path. The failure rejects the document like
a violation, with a `*ValidationError` cause (`ErrInvalidDocument`). An integer
beyond its Go type also makes conversion fail, so `DecodeResult.Conversion`
reports it as unrepresentable. Union and list values are held as strings and
are not checked.

Model-category failures are not part of `Report.Counts`: `Report.ModelFailures`
counts them, and `validation.ResourceFailureSummary.ModelFailures` keeps that
count when a package keeps only the summary of a failed resource. Both reject
the report or bundle like a failed normative rule.

`ErrIncompleteAssessment` identifies technical incompleteness, including its presence in a joined invalid/incomplete
result. It is the identical sentinel exported by the `validation` subpackage.
`errors.As` can also retrieve its `*validation.IncompleteError`. A legacy
model/XSD failure may be joined with either independent incompleteness or
dependent schema work blocked by that same proven failure. For example, a NaN
measure width definitely fails model representation, while its XSD attribute
assessment cannot establish a decimal value. `Validate` then matches both
`ErrInvalidDocument` and `ErrIncompleteAssessment`, whereas `AssessModel` has a
nil operation error and a report containing the model failure and causally
blocked XSD result. This joined legacy sentinel behavior differs from the former
serialized-value path. Test invalidity first when deciding whether to reject or
retry; incomplete alone does not mean that a definitive invalidity is absent.
The facade does not reinterpret the report as complete because a blocked result
has a known cause: a composite can also contain independent capability limits.
Diagnostic truncation cannot change these report axes.
Incomplete-error text and diagnostic wording may change across the direct-walker
migration. Use `errors.Is`/`errors.As` rather than matching an error-string prefix;
the public sentinel and typed cause are the compatibility contract.

Reached `ENTITY`/`ENTITIES` assessment cannot establish unparsed-entity
declarations at this layer. Lexically valid values therefore produce technical
incompleteness; lexical violations remain reportable and can reject a value
before declaration facts are needed. Independent violations elsewhere are still
reported alongside a reached technical error. Validation never fetches a DTD.

URI values are checked using XSD `anyURI` lexical rules, including XLink
escaping of spaces and non-ASCII characters. Validation leaves the stored
string unchanged and does not check resource existence or accessibility.

## Direct current-model assessment

```go
func AssessModel(ctx context.Context, document Document, options AssessmentOptions) (validation.Report, error)
```

The name is deliberately `AssessModel`: `Assess` is already an exported generated
MusicXML model type and remains unchanged. `AssessmentOptions` retains its
`Profile` and `Budgets` fields. `AssessModel` accepts `*ScorePartwise`,
`*ScoreTimewise` and `*OpusDocument`; it rejects nil/typed-nil documents and
unsupported concrete implementations. A nonnil context is required; an empty
profile selects `ProfileModelSemantics` (see [Scenarios](#scenarios)). Unknown
or source-target profiles, invalid budgets and already-canceled contexts fail
before model traversal.

| Constant | Profile ID | Declared scope |
| --- | --- | --- |
| `ProfileModelXSD` | `musicxml-4.0/model-xsd` | Current generated structure/value XSD plus required model representation checks |
| `ProfileModelLocal` | `musicxml-4.0/model-local` | The same model checks plus the explicitly selected local musical and typed-reference clauses |
| `ProfileSourceLocal` | `musicxml-4.0/source-local` | Original single-pass source XSD plus those same local clauses; selected with `InspectXML` |
| `ProfileModelTimeline` | `musicxml-4.0/model-timeline` | Model-local scope plus the bounded exact timeline/effective-state provider and selected chord/numeral consumers |
| `ProfileSourceTimeline` | `musicxml-4.0/source-timeline` | Source-local scope plus the same timeline provider and consumers on original input |
| `ProfileModelContext` | `musicxml-4.0/model-context` | Frozen model-timeline scope plus bounded typed context projections and selected contextual predicates |
| `ProfileSourceContext` | `musicxml-4.0/source-context` | The corresponding context extension on original input |

These are bounded implementations under final verification, not a claim of
whole-MusicXML conformance or completed batch acceptance. The complete selected
clause list, exclusions, test evidence and remaining owners are in the
[model assessment scope](docs/validation/model-assessment-scope.md) and
[timeline scope](docs/validation/TIMELINE_ASSESSMENT_SCOPE.md). Each profile
supports the three roots above; score-only local predicates are inapplicable to
opus and never assess linked opus/score content. Version selection is the pinned
MusicXML 4.0 schema, not an invented requirement that the `version` attribute
equal `4.0`. `InspectXML` and `AssessModel` have no implicit default profile.
The [assessed loaders](#mandatory-assessed-loading) instead fix the source and
model semantic profiles. The separately compiled
`ProfileSourceXSDPreview` remains developmental with its existing scope and IDs.

The direct adapter uses generated field/choice metadata and the same authoritative
XSD grammar, scalar, attribute/default and identity machinery as source. It never
serializes, parses XML, invokes user marshal methods or mutates the model.
The caller must not mutate a document during assessment. Independent assessments
may read the same unchanged document concurrently.

Current evidence has precise limits:

- Nil optional pointers are absent; nonnil pointers are present even at zero or
  empty. Required nonpointer fields are present current values, including zero.
- Valid `Content` entries preserve current interleaving; ordinary fields use their
  generated order. Empty/multiple-branch entries and active-path cycles are model
  problems. A hole never becomes a complete compacted child sequence.
- Root identity comes from the concrete generated type. `XMLName` is ignored
  transport metadata, including inconsistent or invalid text in that field.
- Strings supply their current text to shared schema-domain/whitespace/union
  checks. Integers supply exact current signed/unsigned values. Every finite
  `float64` decimal supplies the shortest decimal that reads back to the same
  `float64`. In particular, `0.1` means decimal `1/10`. NaN, infinity and
  invalid XML text are model representation problems.
- Scalar source lexemes remain unavailable. Numeric lexical patterns and other
  reached unsupported value/default/identity capabilities remain incomplete.
  Numeric zero does not trigger an empty-element default; absent attribute
  defaults remain separate effective facts and never overwrite fields.
- The current model cannot recover discarded attributes/elements, repeated
  singleton occurrences, original required-scalar presence, namespaces, xsi
  controls, numeric precision or original order/spelling. A model pass cannot
  retroactively validate a permissively decoded source.

For a started model assessment, `Report.Target` is `TargetModel` and
`Report.Syntax` is `SyntaxNotAssessed`; a preflight failure can return a zero report.
Locations use Go field/index paths, such as
`ScorePartwise.Part[0].Measure[0].Content[0].Note.Pitch.Step`, with occurrence IDs
and no XML byte/line coordinates. Long paths use bounded parent-occurrence anchors;
paths and node IDs are run-local display/occurrence evidence, not stable edit IDs.
Shared pointers outside the active path create independent occurrences.

Ordinary normative and model-representation findings return a report with a nil
operation error. Always use `Report.Satisfied()`. A model blocker can prevent
acceptance without proving an XSD violation. Configuration, cancellation,
resource or traversal interruptions retain their typed cause and best partial
report. Diagnostic truncation cannot erase counts or blockers; known violations
can coexist with incomplete assessment. Reports own retained metadata after the
call returns.

One finite budget account covers the iterative walker, schema/session work,
indexes, exact-value scratch and diagnostics. `MaxEvents` includes examined
fields/choice entries even when they emit no node; session ingress owns node,
attribute and depth charges. Scalar and live-buffer limits apply before expensive
work/retention. No input-byte charge is invented for the existing object graph,
and the 64 MiB default live-buffer limit is not a model-size or heap guarantee.
No schema, DTD or linked resource is fetched.

Historical XSD-only sizing retained in published baseline `d336f089`
(Go 1.27.2 linux/amd64): the following recorded measurements
use `ProfileModelXSD` or `ProfileSourceXSDPreview`, with a shared account and
verified success at the exact minimum allowance and resource failure one
byte/step below it. These are fixture-specific costs after compact
optional-attribute absence storage, not universal per-node constants or
local/timeline-profile measurements:

| Input and target | Nodes | Minimum live-buffer allowance (bytes) | Rule steps |
| --- | ---: | ---: | ---: |
| Baseline `MozartTrio.musicxml` model | 2,516 | 16,321,980 | 2,784,531 |
| Same model with each part's measures repeated twice | 4,913 | 31,644,150 | 5,441,724 |
| Baseline `MozaChloSample.musicxml` model | 3,185 | 20,338,110 | 3,482,930 |
| Source MozartTrio, DOCTYPE-free measurement control | 2,516 | 18,209,624 | 2,893,122 |

Those recorded current-model fixtures cost about 6.4–6.5 KB and
1,094–1,108 rule steps per node; the source control is about 7.2 KB and
1,150 steps per node. The historical estimates imply roughly 10,000 model
nodes or 9,000 source nodes in this family, before other limits; the step
allowance alone
would admit roughly 18,000 or 17,000 respectively. These are historical
calculations, not current-head capacities. Real scores can exceed these
defaults. These estimates are neither guaranteed capacities nor heap/total
allocation measurements: attributes, text, types, failures and selected local
rules change the cost. Choose informed finite overrides and handle resource
outcomes. The legacy `Validate` facade does not inherit these default ceilings.

The corpus resource corrections change these costs. Retained node-map and
sequence/stack/child-vector growth now charges one rule step per copied entry
plus one per growth operation, before allocation/copying. This is newly metered
storage-management work, even for the same completed input, rather than only
later progress after a prior refusal. A tuned `MaxRuleSteps` that just completed
on `d336f089` can now refuse: the retained regression uses 953,719 steps for
`01a-Pitches-Pitches.xml` with `ProfileModelXSD`, and 556,569 steps for the
100-measure generated `ProfileSourceXSDPreview` fixture. Both still complete
with the default work allowance and the same explicit buffer allowance.
Callers using tuned explicit step limits must revalidate them and allow headroom.
The reviewer measured approximately 0.6–1.1% more steps for its XSD/local
controls; this is a bounded observation, not a universal percentage.

The temporary schema attribute-name index now reserves a modeled fixed
512-byte header/first-group allowance plus 128 bytes per entry, releasing it
when publication returns. Published baseline `d336f089` instead retained
96 bytes per attribute cumulatively until session cleanup. The current lease
replaces that cumulative retention; it can be larger for early elements before
the baseline's accumulated retention exceeds it.

The initial PR 43 checkpoint `c0d73972` had already made the 96-byte-per-entry
charge transient, but that lease could under-admit small Go maps. Relative to
that intermediate checkpoint only, the corrected lease is larger by
512 + 32 × attribute-count logical bytes. A limit tuned to that transient peak
can refuse sooner. This delta does not describe the full change from the
published baseline and does not add permanent retained storage.

Compiled type/use identities also each admit one additional compile record and
64 logical bytes. The fixed compile limits remain 65,536 records and 64 MiB;
near-limit custom schemas or catalog configurations may therefore be refused
where the baseline accepted them. The extra charge represents a separately
retained immutable identity, not a new semantic validation requirement.

The source control removes DOCTYPE only to isolate cost in an experiment; it is
different input, not a supported workaround for assessing the original file.
The original external-DTD score sources remain incomplete. The shared kernel
keeps ordered per-occurrence facts; unconstrained absent attribute descriptors
are shared only within a run and exact type owner. Present/default/fixed results
remain independently assessed. The session still retains a fact graph, so this
optimization does not claim constant memory or full conformance.

The direct legacy path has a substantial performance cost versus the preceding
Encode/reparse implementation. Five alternating warmed samples of the unchanged
`BenchmarkValidate` on `MozaChloSample.musicxml` (Go 1.27.2, linux/amd64, AMD EPYC
9V74, `GOMAXPROCS=4`, 750 ms per sample, fixture decode/I/O excluded) measured:

| Implementation | Median time (range) | Median allocated bytes/op | Median allocations/op |
| --- | ---: | ---: | ---: |
| Previous Encode/reparse (`511f55b`) | 27.02 ms (25.35–28.00) | 11,705,376 | 123,632 |
| Direct shared assessment after compaction | 100.73 ms (100.13–108.67) | 33,374,378 | 181,857 |

This is **3.73× time, 2.85× allocated bytes and 1.47× allocations** in this
measurement, not performance parity. Compaction reduces allocated bytes by
26.3% from the initial direct implementation's 45,257,229-byte median, while
allocation count rises slightly. The initial direct-path timing samples had different
host contention, so that series is not a matched timing comparison. The direct
path retains schema/session facts and exact current-value/report semantics;
these measurements disclose the remaining cost rather than promising a
throughput or memory improvement. Cold initialization adds further cost.

### Exact timeline and effective-state profiles

Select `ProfileModelTimeline` with `AssessModel`, or `ProfileSourceTimeline`
with `InspectXML`. These narrower profiles cannot replace the fixed semantic
profiles in `DecodeAssessed` or `DecodeMXLAssessed`.
These profiles use a separately compiled engine and add exactly three required
bindings to their corresponding local profile: `MX40-TIMELINE-effective-state@1`,
`MX40-TIMELINE-chord-duration-bound@1`, and
`MX40-TIMELINE-numeral-requires-key@1`. Model/source have 39/37 required keys,
respectively, with no additional transitive-only keys. The existing XSD/local
catalogs and their digests remain frozen. Legacy `Validate` remains XSD-only;
ordinary `Decode` and `Encode` do not select a timeline profile.

The session-owned provider retains exact rational musical positions and typed
divisions, key, time, clef, staff, transpose and instrument histories. It includes
independent display/sound/listening offsets, note attack/release/bend origins,
proved grace donors, figured-bass anchors and anchor-based durations, direction
association, barline/navigation statements, and sparse sound/instrument/MIDI
statements. These are bounded context facts, not additional rejection rules.
A provider pass means indexing and finalization completed; it does not assert
that every optional or ambiguous fact is known. This release does not expose a
general public timeline-query or playback API.

The selected consumers check each duration-bearing chord tone against its
established principal note, not the preceding chord tone, and require a valid
local numeral-key or an applicable explicitly encoded key at the harmony's
unshifted score origin and staff. Display offsets cannot borrow a future key.
No initial C major, meter, clef, tempo or divisions value is invented. An own
valid numeral-key remains an independent alternative when fallback timeline
evidence is unavailable.
For the duration-only chord bound, the principal is the established preceding
non-chord `<note>` element, including a duration-bearing rest. The bound does
not decide pitch-based chord eligibility or musical/rendering intent; a rest
principal uses the same shorter/equal/longer duration comparison.

When harmony staff is omitted in a multi-staff part, encoded-key presence can
still be proved if the AllStaves-only baseline and every observed staff-scoped
publication/gap partition agree. This does not select a staff or a concrete key,
and it never enumerates the encoded staff count. The baseline is retained even
when numbered publications appear to cover all staves. If the projection can
vary with staff, it remains Unknown with `timeline-staff-unresolved`; malformed
or unavailable prerequisite evidence retains its actual causal reason. General
concrete-key queries still require resolved staff evidence.
A projection may prove encoded-key presence, or an unchanged divisions value,
without identifying a unique latest publisher. This requires an independently
established baseline and independently valid same-result alternatives; missing,
invalid or hidden candidates remain causal. Actual source witnesses survive,
and uncertain inheritance is retained rather than replaced by fabricated
provenance.


Exact established regions remain usable when another region is incomplete.
Unknown same-time conflicts, missing staff/voice/anchor evidence, and unsettled
cursor movement across divisions boundaries retain their specific causes;
they are not silently defaulted or turned into new musical violations.
Signed offsets remain exact local coordinates; continuing-state selection across
measure frames also needs established containment/order evidence. No measure
length is invented from meter or a terminal note endpoint.
Cursor normalization is checked against complete unit history. Staged proofs use
only independently established prior-stage origins/units; derived descendants
cannot certify an unresolved ancestor or its frame bound. Final facts are
replaced only after successful closure.

Grace donor identity does not establish continuous playback time. Figured-bass
consecutive-chain positions are explicitly derived context, not a sum-duration
restriction. Instrument state means latest applicable encoded statements, with
exclusive solo/ensemble and atomic virtual descriptors, not a reconstructed
synthesizer configuration. A sound `time-only` guard is retained on raw updates;
without repeat-pass evidence, only its stated effective properties become
unavailable, and omitted properties make no update. The reviewed
[offset](docs/validation/timeline-offset-interpretation.md),
[note-origin](docs/validation/timeline-note-origin-interpretation.md), and
[instrument](docs/validation/timeline-instrument-statements.md) contracts state
the exact source interpretations and remaining boundaries.

The profiles retain all target-specific XSD, representation and unread-DTD
limits. Full repeat/span, layout, performance realization, author intent,
packages, linked content and external dictionaries are outside their closure.
Opus selection assesses that document only. Predicate counts and the two
consumer milestones are not complete MusicXML coverage or a ceiling on the
provider's typed-state responsibilities. Final acceptance evidence is tracked
in the [full ownership ledger](docs/validation/timeline-assessment-ownership.json).

Timeline arithmetic, indexes and retained context share the session's finite
budgets; a small input can still exceed work or memory limits.
Contained state histories use indexed lookup; exceptional ambiguous cross-frame
candidate comparisons can be quadratic under those same limits. Exceptional
reconstruction can retain both old and staged services until session cleanup;
that conservative cost remains charged, with no automatic budget increase.
The earlier XSD-only costs are not timeline capacity promises. A matched
same-head budget probe at published `632a969` decoded the unchanged
`MozartTrio.musicxml` once, then assessed its **2,516 model nodes** under each
profile. Exact minimum buffer/step limits pass; one byte/step below fails with
the corresponding typed resource error. Both profiles also pass defaults.

| Resource | `ProfileModelLocal` | `ProfileModelTimeline` | Timeline increment | Increment per node |
| --- | ---: | ---: | ---: | ---: |
| Minimum `MaxBufferedBytes` (logical live bytes) | 18,155,049 | 21,077,650 | 2,922,601 (+16.1%) | 1,161.61 bytes |
| Minimum `MaxRuleSteps` | 2,848,163 | 4,193,534 | 1,345,371 (+47.2%) | 534.73 steps |
| Cumulative context facts consumed | 0 | 1,199 | 1,199 | 0.476550 facts |

The respective local/timeline totals are **7,215.84 / 8,377.44 logical bytes**
and **1,132.02 / 1,666.75 steps per node** for this fixture. The timeline requires
`MaxContextFacts=1199`; 1198 fails. Local consumes no context facts here: 1 is
merely its smallest expressible positive limit, because 0 selects the default.
A local session with zero remaining context allowance also succeeds. These
per-node averages are sizing observations, not universal coefficients: scalar
size, types, attributes, diagnostics, state histories and ambiguous cross-frame
reconstruction change the work, retained scratch and publication counts.

`MaxBufferedBytes` accounts for conservatively admitted logical live retained
data and scratch, including overlapping old/staged services. The minimum above
is found by successful-run boundary search; it is not the account's usage after
`Finish`, when this fixture's live gauge is zero. Cumulative allocated bytes
(`B/op` or a `TotalAlloc` difference) also count temporary allocations released
within a run; neither metric is process RSS or a Go heap ceiling. Compiled
metadata, runtime caches, allocator overhead and the caller's model are not a
promise made by the session's live-buffer allowance. Defaults remain **64 MiB
live buffer, 20 million rule steps and one million context facts**. Size finite
overrides for the selected profile and handle resource incompleteness.

Matched warmed **1,000-note** results at production `22ae68a` (the same production
code as `632a969`) disclose the incremental timeline cost:

| Target | Local median time | Timeline median time | Time ratio | Allocated bytes/op, local → timeline |
| --- | ---: | ---: | ---: | ---: |
| Source | 211.05 ms | 292.55 ms | 1.39× | 52,677,464 → 78,759,784 (1.50×) |
| Model | 211.73 ms | 263.81 ms | 1.25× | 48,341,472 → 74,423,784 (1.54×) |

These are five separate warmed runs per case, Go 1.27.2 linux/amd64,
`GOMAXPROCS=4`, 200 ms per sample, with finite timing-only limits of 512 MiB
buffer and 2^34 steps. Source includes parsing; model reuses a decoded model.
The [independent resource checkpoint](docs/validation/evidence/timeline-resource-independent-checkpoint.md)
records ranges, allocation counts and controls. Shared-host timing noise remains;
the medians do not establish universal throughput or default-budget capacity.

The [PR 35 review](https://github.com/go-muse/go-musicxml/pull/35#discussion_r4246488604)
additionally reported these matched local/timeline figures at `632a969`:

| Model fixture | Local time | Timeline time | Cumulative allocation, local → timeline |
| --- | ---: | ---: | ---: |
| `MozaChloSample.musicxml` | 68 ms | 76 ms | Not reported |
| `MozartTrio.musicxml`, measures repeated 10× | 0.61 s | 0.71 s | 269 MB → 344 MB |
| Same, 50× | 3.88 s | 4.50 s | 1.44 GB → 1.83 GB |

The replicated cases used lifted budgets. These timing/allocation values are
reviewer-reported, not independently remeasured here; their exact budget values,
sampling method and MB/GB convention were not supplied. The independently
reproduced budget boundaries, finite probe limits, raw output and procedure are
in the [PR 35 budget evidence](docs/validation/evidence/pr35-timeline-budget-costs.md).
**B5-PERF-01 remains outstanding** with the shared validation kernel/session
workstream and final batch-9 reconciliation. Incremental timeline measurements
and initialization improvements do not close the older matched-performance
obligation.

### Bounded context profiles

`ProfileModelContext` (`musicxml-4.0/model-context`) and `ProfileSourceContext`
(`musicxml-4.0/source-context`) compose the frozen timeline profiles with six
sealed families: local defaults/applicability, type-specific spans, graphical
repeats, bounded navigation, layout and encoded performance/timing. Earlier
profile IDs, selected rules, digests and ordinary validation remain unchanged.

`validation.Report.Context` exposes independently owned `ContextResult` values.
A closed `ContextProperty` identifies a scalar evidence value or a typed
`SpanContextResult`, `RepeatContextResult`, `NavigationContextResult`,
`LayoutContextResult` or `TimingContextResult`. `TimingContextResult.Kind` is one
of the `validation.TimingKind…` constants, such as `TimingKindNote` or
`TimingKindRootAlter`. Local scalar defaults also use layout-shaped
scope/applicability metadata. A provider's completion does not
claim that each optional field is known. Mandatory consumers separately assess
exact `time-only` ordering, supported bend-release signs and proved graphical
repeat content. Figure-number content retains MusicXML's deliberately unrestricted
`style-text` domain. The built-in context profiles do not select an additional
numeric lexical predicate and do not become incomplete merely because the text
is outside ASCII decimal digits. This does not assert numeric meaning for every
string; independent XSD declaration and attribute constraints still apply.

Removing the former figure-number overconstraint deliberately changes these two
context profiles' selected rules, catalog digest and affected report outcomes.
The six XSD/local/timeline profile contracts remain unchanged. Historical
snapshots are retained; the [explicit migration receipt](docs/validation/evidence/figure-number-conformance-correction.md)
describes the correction rather than claiming frozen context-profile equality.

`NavigationContextResult.MarkerActive` records activation of an encoded barline
marker; `Guard` records pass-selection evidence. Neither field is a realized
playback count. Traversal extent remains separately described by `Complete`;
ambiguous mixed jump/repeat control preserves an unavailable bounded prefix.
Ordinary sound/listening updates retain stable encoded order at a shared
position; they do not create a competing navigation-control boundary. For an
ordinary sound update, an absent pass selector has a Known true `Guard`,
independently of traversal completeness. Listening statements retain their own
activation evidence; jump-specific selector defaults remain separate.

Span correspondence preserves exact part and identity boundaries. Refused
same-note tied interpretations withhold their exact identity run in every
document order. Duplicate wholly unnamed lyric lines withhold that unnamed
identity within the part; no default verse or reset boundary is inferred.
Graphical beat-repeat comparison without an explicit stop uses the documented
same-final-measure encoded cursor extent. A trailing forward contributes to that
extent just like other positive non-backup advancement; this comparison does
not establish measure capacity or endpoint containment. See the
[bounded terminal-range contract](docs/validation/context-assessment-scope.md#graphical-beat-repeat-terminal-extent).

The built-in profiles bind these sealed contracts through the module's
internal plan constructors (see [Adapter entry points](#adapter-entry-points)).
The legacy figure-number plan, which accepted only the nonempty ASCII-digit
subset, is no longer selectable: no built-in profile uses it, and its
constructor is internal since v0.2. Its tests remain.
The Opus layout plan is separately
bound to the Opus assembly and projects only local score pagination hints;
missing new-page remains application-dependent and no linked score is opened. `ContextPlanSpec` binds the exact selected timeline
provider where needed; `ConsumerSpec` binds an exact type-specific
provider. There are no caller expressions, evaluator callbacks or unmetered
post-Finish queries. Configuration is schema-bound and revisioned separately
from the accepted timeline contract.

Projection is admitted under the same context-fact, work and buffered-byte
budgets before cleanup. Reports are editable caller snapshots, not immutable
certificates. Unavailable optional geometry, author intent and physical playback
realization remain per-field evidence or explicit exclusions; they are not new
conformance failures. Repeated/ambiguous ranges remain bounded by the selected
finite budgets. The matched `BenchmarkContextProfiles` compares warmed timeline
and context profiles with documented larger finite budgets; it does not close
`B5-PERF-01` or promise default-budget throughput.

The repository's official MusicXML 4.0 samples `MozartTrio.musicxml` (262 notes)
and `MozaChloSample.musicxml` (278 notes) do **not** complete either context
profile under the default budgets. Both stop at the 20,000,000-step limit;
raising work alone still reaches the default 64 MiB buffered-byte limit.
Explicit finite `MaxRuleSteps: 1 << 34` and `MaxBufferedBytes: 512 << 20`
complete the measured source/current-model cases. Source measurements remove
only the unsupported DOCTYPE; MozaChlo's UTF-16BE BOM and encoding declaration
are preserved. Unmodified DOCTYPE-bearing source remains unsupported by the
source assessment contract.

The corrected Mozart model run uses 40,569,928 shared steps, 13,808 facts and
11,105 records; its measured minimum buffered limit is 94,263,641 bytes.
The MozaChlo model run uses 46,302,398 steps, 15,488 facts and 12,337 records,
with a 103,134,552-byte minimum. These are fixture-specific results, not a
score-size guarantee. Correcting false passive-control ambiguity reaches more
encoded navigation occurrences and retains their owned state snapshots; the
higher official-sample cost is included in these measurements.
See [matched resource evidence](docs/validation/evidence/context-round3-resource-acceptance.md)
for source minima, exact/one-under controls and cleanup.

See [scope and limits](docs/validation/context-assessment-scope.md) for the
source/model contract and deliberately retained normative boundaries.


## Fact-stream validation foundation

The `validation` subpackage exports `NewCatalog`, eight generic sealed typed-plan
constructors, the sealed shared-schema `NewXSDPlan`, bounded local relation and
typed-reference constructors, `NewModelRepresentationPlan`, `NewEngine`, and
explicit-profile fact sessions using `OpenNode`, `Attribute`, optional
`CompleteAttributes`/`ModelRepresentation`, `CloseNode` and `Finish`. The optional
attribute-closure event seals explicit start-tag enumeration before children;
it enables truthful prefix assessment when an ancestor never closes. Catalogs are immutable; sessions copy retained
facts and return independently owned reports. Exact selectors distinguish schema
assemblies and declaration/use occurrences. There is no default MusicXML profile,
resolver or package-assessment entry point.

### Adapter entry points

The MusicXML-specific plan constructors, the source-ingress methods of
`Engine` and `Session`, package sessions and `AllocationSize` serve the root
package's own adapters. Since v0.2 they are not exported: package `validation`
registers them as typed hooks in the module-internal package
`internal/validationlink`, and the root package looks them up at
initialization. Their data types, such as `ContextPlanSpec`, `ConsumerSpec`,
`TimelinePlanSpec`, `SourceOriginContextResult` and the `Package*` observation
types, stay exported. Some of them appear in reports and results. Others,
such as the spec types, `DictionaryOperation`, `PackageIdentity`,
`PackageEdgeObservation` and `SourceEntityDeclarations`, appear only in the
hook signatures that the root package spells out, so they have no use outside
the module, and neither do `NewPackageCatalog`, `ResourceAdoption` and the
snapshot from `Session.FinishWithResourceSnapshot` without a package session. Where this
document names a hidden constructor or method, it describes how the built-in
profiles use it. The hidden entry points are:

- the plan constructors `NewBarlineLocationPlan`, `NewBendReleaseSignPlan`,
  `NewChordDurationPlan`, `NewContainerMetadataContextPlan`,
  `NewDictionaryPlan`, `NewFigureNumberNumericPlan`,
  `NewGraphicalRepeatContentPlan`, `NewLayoutContextPlan`,
  `NewLocalContextPlan`, `NewLocalLinkContextPlan`,
  `NewNavigationContextPlan`, `NewNumeralKeyPlan`,
  `NewOpusLayoutContextPlan`, `NewPerformanceContextPlan`,
  `NewRepeatContextPlan`, `NewSchemaHintContextPlan`,
  `NewSoundsMappingContextPlan`, `NewSourceContainerMetadataContextPlan`,
  `NewSourceLocalLinkContextPlan`, `NewSourceOriginPlan`,
  `NewSourceSchemaHintContextPlan`, `NewSpanContextPlan`,
  `NewTimeOnlyOrderPlan`, `NewTimelinePlan`, `NewXMLFragmentLinkContextPlan`
  and `NewXSDPlanWithSourceDeclarations`;
- `Engine.SourceAttributeLexicalProvider`, `Engine.SourceDTDIDsSelected` and
  `Engine.SourceOriginProvider`;
- `Session.CompleteSourceDTDIDs`, `Session.CompleteSourceOrigins`,
  `Session.SourceAttributeLexical`, `Session.SourceDTDIDAttribute`,
  `Session.SourceDTDIDDomain`, `Session.SourceDTDIDsSelected`,
  `Session.SourceEntityDeclarations` and `Session.SourceOrigin`;
- `CompiledPackageCatalog.Begin` and every method of `PackageSession`, which
  remains an opaque type;
- `AllocationSize`.
The root package provides the explicit source and direct-model adapters described
here. The [exported example](validation/example_test.go) uses synthetic facts.

`CompiledSchema.Uses()` returns copied `SchemaUse` descriptors with exact
declaration, owner and declared-type identities; the `Known` flags distinguish
unavailable metadata from anonymous types. It does not establish runtime support.
`NewXSDPlan` accepts source/current-model facts without inventing source history.

The additive local constructors are `NewDistinctChildAttributePlan`,
`NewScalarOneOfPlan`, `NewPresentAttributeValuePlan`,
`NewAttributeNumberOrTokensPlan`, `NewPresenceImpliesAttributePlan`,
`NewAttributeImpliesRelatedChildPlan`, `NewDocumentChildPresencePlan` and
`NewDocumentMarkerValuesPlan` (with `ChildValueRequirement`).
`NewTypedReferencePlan(ReferencePlanSpec)` uses authoritative XSD ID resolution;
`ReferenceOwner` binds the exact provider rule/revision defining matching scope.
These sealed constructors require bounded exact selectors/parameters and copy
retained inputs. They are not a callback ABI or a linked-resource resolver.

`NewTimelinePlan(TimelinePlanSpec)` requires the exact compiled MusicXML schema
and both part-reference owners. `NewChordDurationPlan(ConsumerSpec)` and
`NewNumeralKeyPlan(ConsumerSpec)` require the matching schema and exact
provider rule/revision; their rule definitions must also declare that document
dependency. Sealed role/type validation rejects similarly named but incompatible
schema declarations. These plans do not add generic interpretation to unrelated
context keys or expose mutable session state.

`Session.ModelRepresentation` accepts one bounded `ModelRepresentationFact` for
the currently open model occurrence before `CloseNode`. `ModelProblem` identifies a
scalar, exact attribute, children or node axis through the `ModelAxis` constants.
The session copies the examination and establishes a real required model decision
before exposing causal invalid evidence. The corresponding
`NewModelRepresentationPlan` counts model blockers separately from normative
failures; missing examination evidence cannot pass.

A profile explicitly names all required and transitive rules, static support,
supported roots and normative closure. `CompiledCatalog.Schedule` exposes each
rule once in final-evaluation order, after derived context-provider/identity
phase ordering; these edges do not add rules or semantic dependencies. Catalog
digests cover canonical metadata and typed plan parameters.
`inventory.MusicXML40` (package `validation/inventory`) is a separate pinned inspection snapshot of 2,560 research records, not executable rule coverage.

`Report.Satisfied()` evaluates independent syntax, input/assessment completion,
conformance and blocker axes. A nil operation error alone is not acceptance.
Evidence-only limitations and normative findings are report outcomes; terminal
failures preserve typed incomplete/resource/cancellation causes. Finalize exactly
once with `Finish` or the selected retained-report/resource-snapshot variant,
even after an event error, to release or explicitly transfer owned live resources. Required
model/policy/context blockers do not become normative violations, and diagnostic
truncation does not erase counts or blockers. Formatting is bounded and offline.

`Report.Findings` stores only fail, unknown, unsupported and blocked diagnostics;
`Report.Rules` contains metadata only for those stored diagnostics. Pass and
not-applicable outcomes still contribute to applicable `Counts` and are retained
compactly for dependency execution, but never consume diagnostic quotas or set
`DiagnosticsTruncated`. There is no all-decisions trace option. The compact
per-rule success/not-applicable tables and document-prerequisite aggregates have
separate live-buffer charges. A `RelationDocument` rule-prerequisite aggregate is
computed lazily once after its prerequisite's final evaluation phase and reused
by consumers. This does not remove rules-times-nodes dispatch or make the retained
fact session a constant-memory stream.

The identity plans accept upstream-domain-validated string keys. A well-formed
known atomic value in another family produces a per-rule unsupported outcome,
not a terminal event error. An unsupported declaration makes its index incomplete
so an unresolved reference cannot fabricate a missing-target violation; malformed
atomic payloads remain terminal fact-contract failures.

Terminal summaries use deterministic precedence across joined causes:
resource-limit, cancellation and deadline exhaustion are resource failures;
otherwise a valid source completion asserting malformed syntax with an explicit
cause is syntax; unsupported root, invalid session, invalid configuration and
unknown infrastructure failures are capability
failures. Each has its own reason code, listed in the
[terminal disposition table](docs/validation/foundation-scope.md#terminal-disposition).
Invalid or contradictory completion assertions remain capability contract or
lifecycle failures rather than proving malformed XML. Policy remains an explicit
rule category. Selecting one terminal summary does
not discard the underlying causes available through `errors.Is`/`errors.As`.

`Budgets` has finite zero defaults; a shared `Account` cannot be reset or widened
by child sessions. All limits, charging owners, exact-value constructor caps,
plan boundaries and remaining obligations are documented in the
[foundation scope](docs/validation/foundation-scope.md). Numeric constructor
resource limits are not XSD facets. `validation.Rational` / `NewRational` add
an immutable reduced fraction for musical positions, including nonterminating
values such as `1/3`; they do not widen `Decimal` or XSD `Atomic`. The zero wrapper
is uninitialized. `Value` returns a copy, `String` returns a canonical fraction
(or an integer), and `ByteSize` counts significant magnitude plus text. Constructor
inputs, intermediate magnitude and canonical output are statically bounded by
`MaxExactValueBytes`; overly large unreduced inputs may be refused even if they
would cancel. Internal session arithmetic additionally precharges conservative
quadratic work and live scratch before normalization, cross-products or rendering.
Work counts architecture-independent logical bulk units (32-bit limbs and
four-byte text chunks), not individual bits or literal machine instructions;
large operands retain quadratic costs. Its output is retained until `Finish`;
resource rejection means incomplete
assessment, not musical invalidity. Cancellation is checked before and after
synchronous `math/big` calls, which cannot be preempted in flight.

`DecodeOptions` and generated public field
representations remain unchanged. The additive model-representation event is
required only when its corresponding sealed plan is selected; older generic
fact sessions and the source preview retain their contracts.

### Retained report ownership

`Session.FinishWithRetainedReport(ctx, end)` returns `(Report, *ReportLease, error)`
and finalizes the session like `Finish`. It transfers the already admitted
`Report.Context` projection to an optional lease before intrinsic cleanup, plus a
bounded ownership-handle charge. It does not copy the report, admit its projection
a second time or create a resource snapshot. Reports without context need no
lease. If the handle cannot be admitted, the best report is returned incomplete
with no lease; dependent assessment must not continue.

Close every nonnil lease, including one returned with an error. `ReportLease.Close()`
needs no context, is safe for nil/already-closed leases, and releases the live
context/control allowance exactly once without refunding cumulative work or
diagnostic quotas. A lease has one owner: do not copy it or use it concurrently.
Returned report data remains caller-owned and usable after `Close`; caller
mutations and extra report copies are outside the admitted projection accounting.

`PackageSession.FinishWithRetainedBundle(ctx, end)` returns
`(AssessmentBundle, *ReportLease, error)` for package and graph sessions.
`Finish` releases every package and resource reservation although the bundle
still owns its reports, so this variant measures the published bundle once
after `Finish`, with the strings, slices and pointed-to values of every report
and record, and charges that size plus the handle to a lease. If the
measurement or its admission is refused, the bundle is returned with that
error and no lease; dependent work must not continue. The lease follows the
rules above. `DecodeMXLAssessed` uses it to keep its source bundle charged
during the later stages.

`AssessmentBundle.Failures()` summarizes why a bundle is not satisfied, from the
authoritative conformance and counts of its package report, its required
resource reports and its failure summary, or the failed report the summary
retained, without copying those reports:

- `Violated`: one of them is `ConformanceViolated` or counts a failed rule
  evaluation (`Counts.Fail + ModelFailures`), even when its diagnostics were
  truncated;
- `Incomplete`: anything else keeps the bundle from being satisfied, such as an
  incomplete, terminated or undetermined report without a violation,
  incomplete enumeration or a retained failure summary;
- `Failed`: the failed normative and model-category evaluations they count
  (`Counts.Fail + ModelFailures`);
- `Truncated`: a violated report or failure summary omitted stored
  diagnostics;
- `Findings`: copies of the stored failed normative and model findings.

A satisfied bundle has none. The strict loaders build their rejection errors
from this summary.

The assessed loaders use this cross-package ownership seam to keep source context
charged while the direct-model stage runs, then close the lease when transferring
the result to the caller. It adds no loading option. Ordinary `Finish` and
`FinishWithResourceSnapshot` retain their existing contracts.

## Original-source inspection and assessed loading

**Source input limits:** a DOCTYPE external subset is read from the bundled
official MusicXML DTDs or a caller DTD source, as described under
[External DTD subsets](#external-dtd-subsets). An unrecognized external subset
still prevents source acceptance, including in the mandatory assessed-loading
pipeline. With the bundled DTDs, `DecodeAssessed` accepts the committed official
`Chant.musicxml` with default budgets. The larger `MozartTrio.musicxml` and
UTF-16 `MozaChloSample.musicxml` now complete their full source and model
assessment and need more than the default 64 MiB buffer and 20,000,000 rule
steps; with 256 MiB and 100,000,000 steps both are accepted. Their default
buffer refusals are architecture-dependent receipts, not portable input-size
limits; 386 does not assert them. See the [capacity controls][loading-capacity]
for provenance. Of the 154 score and opus files in `testdata/official` and the
MusicXML test suite, default assessed loading accepted 140 when measured for
this change, and `TestAssessedLoadingCorpusMeasurement` keeps that floor; the
rest report genuine violations, unestablished semantic context or resource
limits, none of them unread-DTD evidence. The source-XSD preview remains developmental and
DTD-independent and still reports any external subset as unread. Removing a
DOCTYPE changes the input and is not equivalent assessment of the original
file. `Decode` and `DecodeWithOptions` remain transport/conversion only; those
APIs do not validate the source or replace successful assessed loading.

`InspectXML(ctx, reader, AssessmentOptions{Profile: ProfileSourceXSDPreview})`
assesses original XML without creating a Go document. Use `ProfileSourceLocal`
to add the frozen local clauses, or `ProfileSourceTimeline` to add the bounded
timeline provider and consumers described above. An empty/unknown profile,
invalid budgets, or an already canceled context fail before the first input read.
A nil operation error does not imply acceptance: inspect `Report.Satisfied()`.
Ordinary normative failures return a report with no operational error. Syntax,
I/O, cancellation, resource and fatal capability failures preserve their causes
with the best partial report.

### Mandatory assessed loading

`DecodeAssessed(ctx, reader, DecodeAssessmentOptions{...})` returns a
`DecodeResult` with independent `SourceLine`, `Conversion`, `ModelLine` and
`Document` fields. This intentionally breaks the earlier unreleased
optional-assessment contract; these APIs were not present in v0.1.0.
Both assessed loaders always require validation line 2, `ProfileSourceLine`
(`ProfileMXLSourceLine` for MXL), and, after its acceptance and successful
conversion, line 3, `ProfileModelSemantics` (`ProfileMXLModelSemantics`).
Nil `Assessment`, empty options and an empty `Assessment.Profile` select these
fixed defaults; the explicit line-2 profile is also accepted. Any other
profile, including narrower source profiles, wrong-target profiles or unknown
IDs, is a configuration error before source reads or dictionary callbacks.
`Assessment.Budgets` remains configurable and existing `DecodeOptions` safety
limits are preserved. `InspectXML` and `AssessModel` keep their explicit-profile
contracts. Ordinary `Decode` remains transport-only.

The result includes `ModelLine validation.Report`. An earlier operation failure,
unsatisfied/incomplete source report or conversion failure leaves it unrequested.
A dictionary import that refuses after line 2 (cancellation, resource limit,
invalid dictionary data) or a model-session startup failure, including
cancellation after source acceptance, can also leave `ModelLine.Requested ==
false` while `SourceLine` and `Conversion` are complete; that field cannot
distinguish these cases. Once initialized, model assessment walks the actual Go object directly and
preserves its available report, including on failure. `Document` is
returned only when both reports are satisfied, conversion succeeds and there is
no operation or close error. All other outcomes retain the available reports and
return no document. See the [migration guide](docs/validation/assessed-loading-pipeline.md)
for defaults, dictionary overrides and examples.

The observer sees every original element, physical attribute, namespace binding,
child order and direct character segment before Go-model filtering or skipping.
The XML parser has one owner. After a recoverable model mapping error, that same
healthy parser is drained; the failed reflection decoder is never resumed.
There is no reader replay, second XML parse or model Encode→parse in this path.
The implementation retains a budgeted fact graph; single-pass input does not
mean constant-memory processing. One finite budget account covers source
assessment, the dictionary set and direct model assessment, with the source
report's retained context accounted for while the model stage runs. The
dictionary set serves line 3 only and is obtained when line 3 starts: a
document that line 2 rejects in the strict mode never opens a dictionary
source. The caller retains ownership of the reader.

The default dictionaries, the required selections of `BundledDictionarySource()`,
are imported and verified once per process. Each call reads that verified set
and is charged its retained storage on `MaxBufferedBytes` while holding it, but
no import work: a few rule steps and no `MaxExternalBytes`. Any other source or
selection is imported on every call, at the caller's expense (about 3.3 million
rule steps for the required bundled pair at an earlier head). See the [dictionary
import cost][loading-cost] for details and the measured budgets.

`AssessmentOptions.OmitContext` leaves `Report.Context` empty in both reports
and keeps every decision, count and finding, which roughly halves the buffer
strict loading needs.

The default 64 MiB `MaxBufferedBytes` is a safety cap, not a promise that a
64 MiB file or one million nodes can be assessed. The historical `d336f089`
budget-sizing
table above measured the DOCTYPE-free MozartTrio source control at 18,209,624
bytes and 2,893,122 rule steps for 2,516 nodes: about 7.2 KB and 1,150 steps per
node, or roughly 9,000 such nodes under the default buffer allowance. Those
baseline-derived estimates are not current-head capacities. They are
logical accounting estimates, not heap/RSS measurements or a universal element
limit. Ordinary larger scores can exceed the default. Set informed finite
overrides in `AssessmentOptions.Budgets`; all other limits still apply. Earlier
source-scope measurements remain historical evidence for their stated revision,
before the shared absent-attribute optimization.

Composition costs differ from that XSD-only table. In the historical
linux/amd64 receipt below, the original 223,068-byte UTF-16BE
`MozaChloSample.musicxml` with `ProfileSourceSemantics` exhausts the default
64 MiB buffer. Its external subset is now read from the bundled official DTD, so
the assessment does its full work: with `MaxBufferedBytes: 256 << 20` and
`MaxRuleSteps: 100_000_000` this particular fixture completes and is satisfied.
This is a measured finite allowance for that exact input and profile, not a new
default or a minimum/general capacity guarantee.
Removing the encoded DOCTYPE produces a different 222,824-byte control.
`ProfileSourceInternalDTDDictionaries` assesses that control within defaults
(observed live-account peak 44,036,266 bytes). `ProfileSourceSemantics` instead
reaches the existing 20,000,000 rule-step limit, with an observed buffer peak of
67,011,966 bytes before that refusal. These peaks are sampled logical account
usage, not exact minimum budgets or heap measurements. The
[composition and snapshot budget receipt](docs/validation/resource-fragment-budget-evidence.md)
records the byte transformation, limits, ownership accounting and regressions.

`ConversionConverted` means mapping succeeded under the existing Go model
contract, including its `float64` precision and field representation. It is not
an exact lexical or numeric round-trip guarantee. `ConversionUnrepresentable`
and `ConversionError` preserve machine-range errors without inventing XSD
invalidity. Strict error chains may include both an actual normative
`ValidationError` and `validation.IncompleteError` when both axes apply.

`ProfileSourceXSDPreview` is a developmental, source-only profile for
`score-partwise`, `score-timewise` and `opus`. Six sealed XSD operations cover
binding, element declarations, attributes, scalars, content grammar and document
identity, reusing the authoritative shared XSD implementation. Each selected
operation counts once per applicable subject, regardless of how many diagnostic
details it emits. A proved failure and incomplete remaining work can coexist.
A small diagnostic quota cannot turn failure/incompleteness into acceptance.
Repeated cause-free `dtd-infoset-unavailable` outcomes are displayed as one
bounded summary per full rule key and document, with the number of summarized
rule evaluations and the first representative occurrence. All instance counts
and blockers remain exact. Those summaries are admitted after ordinary details,
so they cannot consume a fitting later failure's only diagnostic slot. Other
unsupported reasons and causal blockers are not coalesced. `ValidationError.Issues`
contains only quota-admitted normative details and may be empty; a truncated
source error additionally explains the failed-evaluation count and truncation.

The preview remains a separate developmental profile. It excludes
MusicXML prose, direct current-model evidence, local typed musical relations,
timeline/repeats/context, packages, external documents and dictionaries.
Reached unverified XSD/DTD capabilities remain unavailable; catalog compilation
is an immutable bounded snapshot, not general XSD schema validation or UPA
closure. The [source assessment scope](docs/validation/source-assessment-scope.md)
defines its exact support, ownership, verification and remaining obligations.
`ProfileSourceLocal` adds only the [selected local clauses](docs/validation/model-assessment-scope.md#selected-musical-obligations)
and retains all source-parser/XSD/DTD limits. Empty accordion registration, for
example, passes the XSD-only preview and legacy `Validate`, but fails either local
profile. The mandatory assessed loaders also withhold that otherwise converted
document, through line 3.
These existing profiles introduce no DTD stripping, fetching, trust or ignore-DTD option.

### Selected internal-DTD source profile

`ProfileSourceInternalDTD` (`musicxml-4.0/source-internal-dtd`) is a separate
developmental profile selected explicitly with `InspectXML`. The mandatory
assessed loaders select `ProfileSourceLine`, derived from the
`ProfileSourceSemantics` composition, which includes bounded
internal-declaration processing, rather than this narrower profile.
It supports the three score/Opus roots and reuses the six sealed XSD operations
after bounded processing of actually read internal declarations. It applies
supported internal entity replacements, attribute defaults and declared
non-CDATA normalization before namespace expansion, assessment and conversion.
It does not compose the local/timeline/context/package profiles. Existing
inspection profiles, plain `Decode` and standalone current-model behavior are
unchanged with respect to DTD processing. Nil-assessment `DecodeAssessed` now
uses its mandatory line-2 profile, `ProfileSourceLine`, which applies the same
bounded DTD processing. External subsets/entities are never
fetched; a subset that neither the bundled official MusicXML DTDs nor a caller
DTD source provides remains incomplete evidence. This is not a validating DTD processor or general XML
processor-conformance claim.

The shared scanner also corrects an earlier malformed-tail acceptance bug:
truncated processing instructions/declarations now retain their XML syntax
cause across existing decoding modes. This does not enable DTD expansion in
those modes.

The new `ContextResult.SourceOrigin` pointer carries owned typed occurrence
evidence, selected only by source-origin providers. `Specified` distinguishes
an original physical occurrence from an effective default or entity-generated
markup; it does not replace `AttributeFact.Presence`. Physical, declaration and
use sites have independent availability. Synthetic locations do not invent a
physical byte offset. The adapter reports start-tag/attribute occurrence origins,
not per-character value-segment provenance. A physical attribute containing an
entity reference remains physically specified. Missing precise transformed-value
coordinates must not be interpreted as original value coordinates.
`Specified` is not the XML Infoset `[specified]` property: an attribute in
entity-generated markup is not physically specified here. Nested general-entity
use sites and default use sites on entity-generated tags cannot currently be
represented as physical coordinates; they remain Unknown and prevent origin
provider satisfaction. This bounded vocabulary does not expose
replacement-relative positions. A missing declaration in an unread external/PE domain is Unknown,
not known absent. Sealing origins also requires complete attribute enumeration.

This additive public struct field requires migration of external **unkeyed**
`ContextResult` literals; use keyed fields. Its `omitempty` tag preserves old
profile JSON when no source-origin payload is present. Existing profile digests
and revisions are frozen; the new profile uses distinct operation revisions.
The root adapter selects the source-origin plan, discovers its exact assembly
provider, submits each origin and seals occurrence enumeration through the
hooks described in [Adapter entry points](#adapter-entry-points). The session
checks admitted occurrence ownership, duplicate slots, assembly, completeness,
owned copies and the shared account. Current-model and resource-snapshot export
do not support this source-only evidence; a failed export preserves the intrinsic
report and never implies a valid resource adoption.

Internal expansion requires the existing non-nil assessment account. Original
input bytes and hashes exclude replacements and defaults. `MaxEvents`
additionally meters replacement UTF-8 bytes, tokenized-normalization bytes and
invocations; buffered declarations, stacks, effective attributes and report
copies share `MaxBufferedBytes`. Effective nodes and attributes retain normal
ingress charges. A separate finite 64-named-entity nesting ceiling applies
alongside XML depth; the accounted anonymous default-value frame does not consume
a named-entity slot. `errors.Is(err, ErrEntityNestingLimit)` identifies this
specific ceiling while `errors.As` still exposes its `validation.ResourceError`.
Refusal is resource incompleteness, not XML invalidity. No default budgets are
raised. See the [precise scope, limitations and requirement
contributions](docs/validation/dtd-infoset-scope.md) and
[verification evidence](docs/validation/dtd-infoset-evidence.md).

Provenance has a material capacity cost. In the paired `MozartTrio.musicxml`
measurement (94,916 original bytes, 3,687 occurrence rows), retained bytes before
`Finish` were 22,588,921 with this profile versus 11,632,335 with source/XSD
preview while the external DTD was unread. With the bundled external DTD read,
this profile retains about 27 MiB before `Finish` and completes satisfied within
the default 64 MiB limit, while a 24,000,000-byte buffer is refused as a
resource limit; the preview still reports the unread subset. These fixture-specific
figures are not a general maximum document size; paths, ancestry and copied
evidence affect admission.

Positions use zero-based original-byte offsets, including BOMs, and one-based
decoded-character lines/columns. CRLF is one line break. If either coordinate
exceeds the host integer range, both are omitted as `0/0` for the rest of the
source; original-byte offsets remain available. Physical attribute
positions survive skipped content and foreign namespace lookalikes. Overlong
display paths are omitted while occurrence identity/positions remain available.
Fact identity ceilings (1024 UTF-8 bytes per local name/prefix, 4096 per namespace
URI and 65536 in-scope bindings) cause incompleteness, not malformed XML.
Clean input EOF can coexist with incomplete XML well-formedness assessment when
DTD declaration semantics remain unavailable. Context checks cannot interrupt a
blocking arbitrary `io.Reader.Read`; use cancellable I/O where needed.

### Declaration-dependent standalone compositions

Three additive source profiles combine the bounded internal-DTD infoset with
existing normative consumers for score-partwise, score-timewise and Opus:

- `ProfileSourceInternalDTDLocal`: declaration-dependent XSD and existing local
  clauses/typed references, with source-origin evidence.
- `ProfileSourceInternalDTDLinks`: the local composition plus nonfetching local
  links, attachments and schema-hint projections.
- `ProfileSourceInternalDTDDictionaries`: the links composition plus explicitly
  supplied dictionary capabilities through the existing dictionary entry points.

These profiles use a separate catalog. Existing profile digests and rule
semantics are unchanged. Additional internal diagnostic fields are accounted
for at their actual retained size, so a buffered-byte limit set exactly at an
older diagnostic boundary can now refuse slightly earlier. They do not select
auxiliary Sounds/Container processing,
timeline/context rules or automatic package composition. Explicit source-origin
snapshot export is supported by the owned transport contract below. See the
[exact scope](docs/validation/source-declarations-scope.md) and
[verification evidence](docs/validation/source-declarations-evidence.md).

Reached XSD `ENTITY`/`ENTITIES` builtin and restriction carriers use effective
first-bound general-entity declarations from the source document. An unparsed
binding proves membership; a parsed binding disproves it. Missing names are
negative only with complete relevant declaration enumeration. Missing facts,
unread declarations and unresolved XSD 1.0 constructed carriers remain
unavailable. This does not perform general DTD validation or retrieve entity
payloads. Schema-literal conversion remains independent of instance membership;
applied effective instance values retain their own membership obligation.

The internal declaration-aware XSD plan (`NewXSDPlanWithSourceDeclarations`
before v0.2) serves the internal-DTD profiles. Every selected
XSD operation sharing an assembly/run must agree on the mode, and unsupported
plans cannot enable runtime facts. The root adapter submits
`SourceEntityDeclarations` containing owned-input `SourceEntityDeclaration` rows
immediately after root `OpenNode`, before any
attribute, child or closing event. Rows contain only effective first general
bindings, with positive unique document-local sequence IDs and independently
available original-byte origins. The session copies them under the shared
account; callers may release their input after return.

`NewSourceAttributeValuePlan` and `NewSourcePresenceImpliesAttributePlan` use
supplied effective source presence, including DTD defaults and entity markup.
XSD-only defaults do not establish supplied presence. New declaration-aware
catalogs reject incompatible legacy physical-presence local plans; existing
constructors and old catalogs are not reinterpreted.

`NewSourceLocalLinkContextPlan` and `NewSourceSchemaHintContextPlan` preserve
physical `ResourceAttribute.Presence` and `Raw` separately from effective
`Value`. The root adapter discovers the exact selected origin provider and
submits actually retained XML-normalized physical spelling through internal
hooks, after
the matching attribute and origin events while the owner is open. DTD defaults
and entity-generated markup have absent physical spelling. Unretained or
general-entity-transformed original spelling is unavailable rather than
reconstructed. Effective defaulted hints can still produce discovery pairs;
neither their presence nor their values authorize external retrieval.

These additions introduce new types/functions/methods without changing existing
exported option or report struct layouts. Each session retains its existing
single-goroutine lifecycle; independent sessions own separate declaration facts.

## Opt-in pinned dictionaries

Dictionaries are selected with `AssessmentOptions.Dictionaries`, directly or as
`Graph.Dictionaries` of the resource options (see [Scenarios](#scenarios)). An
empty profile uses the bundled required data; an explicit profile without
dictionaries keeps its dictionary-free behaviour. This does not change
`Validate`, ordinary `Decode`, `validation.RunOptions`, or any existing
profile's selected rules and catalog digest.
The full [selected scope](docs/validation/pinned-dictionaries-scope.md) separates
implemented relations from unresolved obligations; the dated
[verification record](docs/validation/pinned-dictionaries-evidence.md) is not a
claim of final integrated acceptance.

### Options and profile composition

`DictionaryOptions` has `Selections []validation.DictionarySelection` and
`Source DictionarySource`. Each selection supplies one fixed `Manifest` and a
nonempty, duplicate-free subset of its supported `Relations`. At most five
manifests may be selected, one per fixed component. Unknown
manifests/relations, duplicate selections and incompatible profiles are
configuration errors, rejected before callbacks. Selecting ranges does not
implicitly select or open the canonical-name component.

| Entry point with `Dictionaries` set | Explicit profile | Result |
| --- | --- | --- |
| `InspectXML` | `ProfileSourceDictionaries`, `ProfileSourceSoundsDictionaries` or a semantic or internal-DTD dictionary profile | `validation.Report` |
| `AssessModel` | `ProfileModelDictionaries` or `ProfileModelSemantics` | `validation.Report` |
| `DecodeAssessed` (via `Assessment.Dictionaries`) | empty or `ProfileSourceLine`; explicit dictionaries must include both required relations and serve line 3 | `DecodeResult` |
| `DecodeMXLAssessed` (via `Assessment.Graph.Dictionaries`) | empty or `ProfileMXLSourceLine`; explicit dictionaries must include both required relations | `DecodeMXLResult` |
| `InspectXMLWithResources`, `AssessModelWithResources` (via `Graph.Dictionaries`) | `ProfileResourceGraphDictionaries` or its internal-DTD or semantic composition | `validation.AssessmentBundle` |
| `InspectMXL` (via `Graph.Dictionaries`) | `ProfileMXLSourceDictionaries` or its compositions; `ProfileMXLSourceLine` reads no dictionaries and rejects them | `validation.AssessmentBundle` |
| `AssessMXLPackage` (via `Graph.Dictionaries`) | `ProfileMXLModelDictionaries` or its compositions | `validation.AssessmentBundle` |

All entry points also return an operation error. `AssessmentOptions` carries
`Dictionaries *DictionaryOptions`; `LinkedAssessmentOptions` and
`MXLAssessmentOptions` carry it in `Graph`. In `DecodeAssessed` a nil
`Assessment` uses the mandatory source/model pipeline with the bundled
defaults. A `Dictionaries` value with nil `Source` and no selections also
selects defaults:
`DictionaryManifestSMuFL14Canonical` with `DictionaryRelationCanonicalName`, and
`DictionaryManifestIPARepresentation2015Unicode13` with
`DictionaryRelationIPARepresentation`. These constants belong to `validation`.

Any other dictionary options must explicitly include both required relations;
other supported selections may be added. Missing required relations, including a
custom source with empty selections, fail configuration before any input read or
callback. An explicit source is honored, including nil/typed-nil or unavailable
sources with explicit selections; import failures never trigger bundled fallback
or additional implicit component requests. Any source, model, conversion, dictionary-operation,
close or strict acceptance error discards the document while preserving available
source and model reports. Low-level inspection and package APIs keep their
existing explicit-selection and missing-capability behavior.

The original dictionary profile IDs are:

- `ProfileSourceDictionaries`: `musicxml-4.0/source-links-dictionaries`
- `ProfileModelDictionaries`: `musicxml-4.0/model-links-dictionaries`
- `ProfileSourceSoundsDictionaries`: `musicxml-4.0/source-sounds-dictionaries`
- `ProfileResourceGraphDictionaries`: `musicxml-4.0/resource-graph-dictionaries`
- `ProfileMXLSourceDictionaries`: `musicxml-4.0/mxl-source-dictionaries`
- `ProfileMXLModelDictionaries`: `musicxml-4.0/mxl-model-dictionaries`

Source/model intrinsic profiles extend the bounded XSD/local-link closure with
exact dictionary consumers and narrow local defaults/range advice. They do not
require full timeline or context profiles. Sounds extends source-only sounds
XSD, directional mapping and nonfetching discovery; there is no `SoundsDocument`.
The graph/MXL catalogs pin source MusicXML/Opus to `ProfileSourceDictionaries`,
current-model MusicXML/Opus to `ProfileModelDictionaries`, source sounds to
`ProfileSourceSoundsDictionaries`, and source container to unchanged
`ProfileSourceContainer`, each with its exact catalog digest. Raw-MXL profiles
have source intrinsic rows only. Current MXL assesses its primary model directly
and retains distinct original-source evidence for selected stored children. The
source-sounds intrinsic catalog row can admit compatible explicitly supplied
fact-stream snapshots; the root dictionary package catalog constructor is
private. Public resource-graph and MXL dictionary assessments do not discover,
resolve or assess sounds children. Standalone `InspectXML` with `Dictionaries`
accepts `ProfileSourceSoundsDictionaries` for explicit sounds input.

One root imports each selected component at most once and shares its verified
set and finite account with selected linked XML and nested packages. Package
snapshot identity includes the actual capability fingerprint, not merely a
requested edition. The per-run graph cache mirrors this identity defensively;
its dictionary fingerprint is fixed for the entire run. Reused canonical content
still has independently owned, document-qualified adoption views, including
nested dictionary causes and spans. Existing archive limits, resource selections
and indirection limits still apply. No child resets or widens budgets. These
profiles do not compose `ProfileSourceInternalDTD` or export source-origin
snapshots; original-source limitations, including unread external DTDs, remain
active.

### Explicit internal-DTD and dictionary package composition

The resource and MXL entry points with `Graph.Dictionaries` set also accept
these exact additive profiles:

- `InspectXMLWithResources` and `AssessModelWithResources`:
  `ProfileResourceGraphInternalDTDDictionaries`
  (`musicxml-4.0/resource-graph-internal-dtd-dictionaries`).
- `InspectMXL`: `ProfileMXLSourceInternalDTDDictionaries`
  (`musicxml-4.0/mxl-source-internal-dtd-dictionaries`).
- `AssessMXLPackage`: `ProfileMXLModelInternalDTDDictionaries`
  (`musicxml-4.0/mxl-model-internal-dtd-dictionaries`).

Select the profile explicitly in `Graph.Profile`; an empty profile selects the
entry point's semantic profile instead. Unknown and cross-entry-point choices
fail before reader or provider I/O. The existing
non-DTD profiles and their catalogs remain unchanged. These new catalogs pin
source MusicXML/Opus to `ProfileSourceInternalDTDDictionaries`, current models to
unchanged `ProfileModelDictionaries`, source Container to
`ProfileSourceInternalDTDContainer` (`musicxml-4.0/source-internal-dtd-container`)
and source Sounds to `ProfileSourceInternalDTDSoundsDictionaries`
(`musicxml-4.0/source-internal-dtd-sounds-dictionaries`), each with the exact
engine-returned catalog digest. Raw MXL accepts source rows only.

`InspectXML` accepts the new Container profile and, with `Dictionaries`, the
new Sounds profile. Neither adds a Document model or decode root.
Sounds remains an explicit standalone input or compatible snapshot admission:
public graph/MXL traversal still selects musical children and does not discover
Sounds from filenames or infer auxiliary inputs.

Only actually assessed source XML receives bounded internal-DTD processing and
actually-read declaration evidence. Container `FullPath` and `MediaType` are
effective values: a DTD-defaulted path can select an entry with `FullPathRaw`
Absent. Physical tokenized attributes preserve retained pre-tokenization Raw;
unretained entity replacement spellings remain Unknown. Effective Sounds ID,
primary and number fields retain DTD defaults without inventing implicit mapping
choices. Auxiliary schema hints retain effective pairs separately from physical
Raw/presence; odd pairing remains a discovery gap, not an XSD datatype violation.
No external DTD, entity, hinted schema or dictionary is automatically retrieved.

The same root account and verified dictionary set cover lexical/declaration
capture, intrinsic snapshots and repeated adoption. Origins are independently
owned and rebound to each adoption, without upgrading incomplete provenance.
The current primary model remains directly assessed with no invented source
origins, original Container or ZIP history; retained source children are separate.
Existing path, archive, indirection, unproved-IPA-representation, unsupported-capability and
unread-external gates remain active. See
[the composition scope](docs/validation/dtd-package-composition-scope.md).

### Bundled pinned dictionary data

Set `DictionaryOptions.Source` to `BundledDictionarySource()` to use the five
bundled components: SMuFL 1.4 canonical glyph names and ranges, MusicXML 4.0
standard sounds, and the separate IPA 2015/Unicode 13 positive and representation
bundles. Keep `Selections` explicit for inspection and standalone model,
linked-document and MXL dictionary entry points; no caller data files are needed.
The assessed loaders alone select bundled canonical SMuFL and IPA representation
evidence when `Source` is nil and there are no selections.

The existing bounded importer verifies raw and transformed identities and owns
its parsed data. Embedding changes delivery only: profiles, catalog identities,
relations, finite budgets and source/current-model authority are unchanged.
For explicit dictionary options, nil sources still leave selected capabilities
unavailable and empty selections do not implicitly open the bundled source. The
assessed-loader defaults are the specific exception above; a custom source with
empty selections is rejected there. Caller-provided sources remain supported.

Any binary referencing one of the entry points listed under
[Scenarios](#scenarios) embeds all five component assets, including both CC BY-SA 4.0 IPA bundles,
regardless of options or custom/nil dictionary sources. Direct use of
`BundledDictionarySource` also retains all five. Selections control runtime
import and assessment; they do not prune the binary's asset references. The
data retain their W3C FSA, CC BY-SA 4.0 and Unicode License v3 terms,
separately from the MIT code license. Distribute binaries containing these
data with all five notice/license files listed in [the bundled-data
distribution instructions][bundled-data-notices]. Those instructions also link
the detailed IPA modification and provenance records. No particular linker
behavior or binary-size saving is guaranteed for programs that reference
neither assessed loader nor the bundled source.

Ordinary compilation embeds committed local files with `go:embed`; it neither
runs generation nor fetches dictionary data. Explicit `go generate ./...`
reproduces those files offline from committed exact-pin inputs. Updating an
upstream snapshot is a separate maintenance action requiring deliberate pin,
provenance and compatibility changes. SMuFL 1.4 is the supported catalog snapshot,
not an exclusive version requirement imposed by MusicXML 4.0.

Bundled standard-sound recognition does not reject legal custom sound names.
Bundled IPA data retains its bounded affirmative-witness semantics: unavailable
representations do not become normative negatives, and derived bundle lineage
is distinct from reading the original publications during an assessment.

### Caller-supplied transport and sealed data

`DictionarySource.Open(context.Context, DictionaryComponentRequest)` returns
`(DictionaryComponentResult, error)`. The request fixes the manifest and exact
`DictionaryComponentPin`; the result must echo it exactly. `EvidenceKnown`
requires a usable `io.ReadCloser` body and no reason. `EvidenceUnknown` or
`EvidenceUnsupported` requires a bounded nonempty reason and no body. Empty
selections invoke no source. Nil and typed-nil sources leave selected capabilities
unavailable. Do not mutate supplied options or input bytes during their use.

Every usable nonnil returned body transfers ownership, including a body returned
with an error, and is closed exactly once. Caller-owned primary readers stay
open. A read or close error prevents accepting a component even if a returned
prefix matches the pin. The importer bounds reads to the exact length plus an
extra-byte probe, verifies original bytes before private fixed-corpus parsing,
and independently verifies the transformed typed identity. The pinned
`sounds.xml` retains its external DOCTYPE; literal-ID extraction is an exact-pin
transformation, not a full XML/DTD conformance certificate.

The library never fetches a URL, opens a file, follows redirects, reads a font,
or loads a DTD for dictionaries. Manifest URLs are provenance, not authorization.
Caller sources own transport authorization and cancellation of blocked I/O;
byte verification cannot sandbox I/O a source already performed. Dictionary
transport is separate from `ResourceResolver` and its four dependency classes.

Core consumers use immutable typed data; they never parse JSON/XML or run
provider-supplied expressions. `LookupDictionaryManifest` returns owned fixed
metadata, without loading or activating anything. The internal dictionary plan binds a
closed `DictionaryOperation` to the reviewed compiled MusicXML or sounds schema;
it does not select data. Canonical-name and IPA operations are mandatory node
predicates; standard sounds and rendering are context providers.

For fact-stream clients, `validation.NewDictionarySet(ctx, account, selections,
imports)` copies and verifies `DictionaryImport` records. A known import has
exactly one appropriate `SMuFLDictionaryData`, `StandardSoundsDictionaryData` or
`IPAPositiveDictionaryData` payload, no reason, and optionally a certificate.
Up to five fixed manifests may be selected together. The new
`DictionaryManifestIPARepresentation2015Unicode13` reuses the existing IPA
payload with 295 symbols, no examples and 158 genuine canonical relations;
no public struct layout, provider or options field changes. Missing imports
remain unavailable. Unavailable imports contain neither data nor certificate.
Extra/unselected imports, contradictory states and duplicate records are rejected;
callers cannot assert their own completeness or authoritative hash.

`VerifyDictionaryComponent(ctx, account, componentID, residentBytes)` returns a
sealed, account-bound `DictionaryComponentCertificate` after exact length/hash
verification and final cancellation checking. Every failure returns the zero
certificate. It retains no input and needs no `Close`. It proves that component's
bytes were verified on that account, not permission to obtain them. The set
independently hashes copied typed records even with a valid certificate. Zero,
wrong-component and foreign-account certificates cannot establish known data.
Without a certificate, matching typed records establish only reviewed-derived
lineage; they do not claim a raw source was read in this run.

`DictionaryContextResult.ComponentVerified` says whether the selected component
was certified in this run. `SourceKind` distinguishes an `original-component`
manifest from a `reviewed-derived-bundle`. For IPA, even `ComponentVerified=true`
means only that the compact derived bundle was verified, never that the original
IPA PDF or Unicode source files were read in the run. `DataSHA256` and `Policy`
identify the compiled reviewed transformation and interpretation.

### Account, lifecycle and concurrent use

A set belongs to exactly one initialized `validation.Account`. Use
`Engine.BeginWithDictionaries(ctx, RunOptions{Account: account, ...}, set)` with
that identical account and zero `RunOptions.Budgets`. Nil set delegates to
ordinary `Begin`. The root wrappers create, share and close their account's set;
advanced callers own that lifecycle explicitly.

`Account` is concurrency-safe; each `Session` remains single-goroutine. Independent
sessions may borrow one immutable set concurrently. Do not copy `DictionarySet`
after first use. Borrow admission linearizes with concurrent `Close`: closing
rejects new borrows immediately but does not wait for or invalidate already
admitted sessions. `Close` is idempotent for an initialized set; nil/uninitialized
sets are errors. Storage is released exactly once when the owner is closed and
the last session finishes. Failed session initialization releases its borrow.
Finalize each admitted session exactly once with `Finish` or the selected
`FinishWithRetainedReport` / `FinishWithResourceSnapshot` path, including after an
event failure. Close any retained-report lease even on failure. Returned reports
are owned values and retain no dictionary borrow. `Fingerprint` returns a fixed
`[32]byte` identity and rejects a closed/uninitialized owner.

The root transport boundary charges external bytes once; resident-byte certificate
verification adds hashing/scratch work, not another transport charge. One account
covers parsing, copies, sorted indexes, lookup comparisons, contexts, snapshots,
result retention and overlapping temporary buffers. Admission precedes allocation;
work and live-buffer costs remain distinct. Per-item scalar caps also apply to
derived defaults, nested `Input`/`Effective` values, IPA span text and canonical
equivalent text before publication. Cleanup releases owned live leases
only, never cumulative byte/work charges or someone else's storage. Cancellation
and resource failures are terminal and preserve their causes. Ordinary missing,
unsupported components remain per-capability gaps; legacy component mismatches
also retain that behavior. Malformed newly selected representation bundles are
configuration errors, as detailed below. An operation
error can accompany a usable report with independent XSD/family failures retained.
Always check both the returned error and `Report.Satisfied()` or
`AssessmentBundle.Satisfied()`.

Runtime import faults are identifiable with
`errors.Is(err, ErrDictionaryUnavailable)`; `errors.As` can match the
`*DictionaryImportError` wrapper.
`DictionaryImportError` exposes `Manifest`, a fixed phase `Reason`, and the
original `Err` through `Unwrap`. Provider, read and close causes remain
inspectable through `errors.Is`/`errors.As`; response, length and digest faults
also use this runtime classification for the four original manifests. For the
new IPA representation manifest only, importer-owned structural, length, digest
and decoder faults remain `validation.ConfigurationError` and cannot expose a
usable set. Caller callback errors keep their original unavailable-import
behavior, including callbacks returning `ConfigurationError`. Options, profile
and account misuse stay
`validation.ConfigurationError`. Direct misuse of
`validation.VerifyDictionaryComponent` keeps that configuration contract;
the root importer's own verification mismatch is translated only for the
original manifests. If a callback
itself returns a `ConfigurationError`, that original cause remains inspectable.
Actual cancellation, deadline and resource failures, including joined close
failures, remain terminal rather than being relabeled unavailable.

### Default-budget boundary

Selecting dictionaries does not raise the finite defaults. The following are
historical PR 40 Round 1 observations, with the exact revision mapping in the
linked dated receipt; they have not been remeasured after the corpus resource
corrections and are not current-head note-count boundaries. In the repeated-rest/
accidental fixture measured for the Round 1 correction, all four components are
imported before assessment. Standalone source/model completed at 368 notes and
refused at 369; graph and raw/current MXL completed operational enumeration at
363 and refused at 364 on the unchanged 20,000,000-rule-step cap. When this was
measured, current-model bundles also kept their schema-hint unknown, which
revision 2 of the pairing rule removed.
The earlier published `a772914` observations were 320/321 and 316/317;
[the dated evidence][dictionary-budget-evidence] preserves both measurements.
These are fixture-specific observations, not supported note-count capacities.
Exhaustion is incomplete assessment, not musical invalidity or B5-PERF-01 closure.

Import work is deliberately conservative. Selecting every relation of one
component costs 2,635,787 steps for canonical names/codepoints, 1,735,474 for
ranges, 435,365 for sounds, or 210,493 for IPA. Selecting all four in one run
costs 5,016,969 steps; shared setup means that this is not the sum of four
independent runs. `TestDictionaryImportPinnedWorkCosts` records the
exact normal-read fixture and selection. Short or empty provider reads add work.
The decoder's `6*rawBytes+1024` reservation is a logical envelope for scanning,
escape/UTF-8 checks, copies, token and row operations, not a measured pass count.

[dictionary-budget-evidence]: docs/validation/pinned-dictionaries-evidence.md#default-budget-observations

### Evidence and compatibility

`ContextResult` gains `Dictionary *DictionaryContextResult` with JSON
`omitempty`. External **unkeyed `ContextResult` literals must migrate to keyed
literals** because this adds a public struct field; it is not source-compatible
for positional construction. Existing keyed literals need no new field, and nil
payloads preserve old report JSON. Existing option structs and `RunOptions` do
not gain fields. Dictionary projections, nested evidence, attribute selectors,
spans and causes are independently owned through report/snapshot getters and
occurrence remapping.

The properties `ContextDictionaryCanonical`, `ContextDictionarySounds`,
`ContextDictionaryIPA`, `ContextDictionaryIPARepresentation`,
`ContextDictionaryRendering` and `ContextDictionaryAdvice` separate exact
relations. `Input` retains assessed scalar evidence; `Attribute=nil` denotes
element content. `Codepoint` remains independently gated by `primary-codepoint`;
it cannot substitute for required `canonical-name` membership.

The original IPA manifest and its three `ContextDictionaryIPA` projections are
unchanged: positive atoms, examples and canonical equivalents leave
`WholeSequence` unavailable. The new, explicitly selected
`DictionaryRelationIPARepresentation` instead publishes one unchanged whole-input
UTF-8 span for known string input, including empty/unproved text, under the
separate property. Unavailable/non-string scalar evidence retains its state
and cause without an invented raw span. Its
`Category` is `whole-representation`, its `RowID` and `Equivalent` are empty,
and only an affirmative whole witness makes both `Result` and `WholeSequence`
known-true. Unproved input stays unknown, never false. No new projection appears
when this manifest is unselected; a selected missing capability is explicit.

The new policy supports stable Unicode13 canonical decomposition/ordering,
complete encoding atoms and source-established disjoint adjacent half-tie pairs
with independent single-base decorations. Repetition, orientation, endpoint and
shared-middle restrictions apply to presentation-form half ties. U+035C/U+0361
remain independent complete mark atoms in the pinned concatenation language:
isolated, repeated and sign-adjacent whole marks have representation witnesses,
without certifying tie endpoints or rendering. Mixed whole/half overlap remains
unavailable. The policy neither rewrites raw input nor uses compatibility
normalization or a host Unicode version. This is a bounded witness, not pronunciation,
phonotactics, rendering or universal IPA conformance. See the
[exact additive policy and migration](docs/validation/ipa-representation.md).

SMuFL canonical membership is exact after independent XSD normalization and
scalar assessment. It does not replace the six MusicXML prefix-family facets,
and missing data cannot suppress their known failures. Present inactive-rendering
attributes remain assessed. Standard-sound nonmembership means nonstandard, not
invalid MusicXML; strings remain open. The separate source/data licenses and full
remaining IPA/semantic gates are documented in the selected scope.

## MXL packages

`MXLPackage.RootFiles[0]` identifies `Document`. `Resources` contains every
other regular file except `mimetype` and `META-INF/container.xml`.

Resource order and bytes are preserved; ZIP compression metadata is not.
Container elements and attributes are matched by their expanded XML names;
namespace declarations and foreign-namespace lookalikes cannot select root
files. Root-file paths and media types are interpreted using XSD token
whitespace normalization. `MXLRootFile.MediaType` is nil when the
`media-type` attribute is absent, which means the MusicXML media type; a
present attribute is kept, even when it is empty, and encoded as written. Opus hrefs use XSD anyURI whitespace normalization
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
document. `ResolveOpusWithOptions(DecodeOptions)` applies the caller's limit to
every linked document; `ResolveOpus` is equivalent to calling it with zero
options.

`DecodeMXLContext` and `DecodeMXLPackageContext` take a `context.Context`
with the same cooperative cancellation. They check it before each read of the
archive, while each entry is expanded, while the primary document is decoded
and before returning a result, so a cancellation during the last entry is not
reported as success. Cancellation errors match `ctx.Err()` and do not match
`ErrInvalidMXL`; a blocked `Read` is not interrupted.

`ResolveOpus` builds a memoized graph. Repeated links share targets, and
opus-link cycles between archive documents are supported. `SyncResolvedOpus`
accepts only the graph created for the same, unchanged package. It commits
linked-resource updates atomically.

## Errors

Public sentinel errors are intended for `errors.Is`, including
`ErrDocumentTooDeep` and `ErrDocumentCycle` for unsafe programmatically built
models. `UnsupportedRootError`, `MXLLinkError`, and `ValidationError` provide
structured context and are intended for `errors.As`.

Errors defined outside `errors.go`:

- `ErrIncompleteAssessment` shares its identity with
  `validation.ErrIncompleteAssessment` and marks unfinished assessment, not an
  XSD violation.
- `ErrUnsupportedXMLFeature` marks well-formed input that relies on an
  unimplemented XML feature; it also matches `ErrIncompleteAssessment`.
- `ErrEntityNestingLimit` reports internal-DTD entity expansion beyond the
  nesting limit.
- `ErrDictionaryUnavailable` and `*DictionaryImportError` report dictionary
  import failures; the typed error unwraps to the provider's error.
- `*ConversionError` reports a model-mapping failure in assessed loading;
  its `Cause` holds the mapper error.
- `ErrDTDNotFound` is returned by a `DTDSource` that declines an identifier;
  `ErrDTDUnavailable` and `*DTDError` report a DTD source failure or unreadable
  caller DTD bytes.
- `ErrInvalidDecodeOptions`, `ErrInvalidEncodeOptions`, and
  `ErrInvalidMXLOptions` report invalid transport options. Invalid
  assessment options are reported as `*validation.ConfigurationError`.
  `ErrInvalidConfiguration`, which is `validation.ErrInvalidConfiguration`,
  matches all four.

All exported error types tolerate a nil receiver in `Error` and `Unwrap`.

## Auxiliary source assemblies

`InspectXML` also accepts `ProfileSourceContainer` and `ProfileSourceSounds`.
Each profile admits only its no-namespace root (`container` or `sounds`) and
uses separately generated, pinned XSD metadata with exact assembly-qualified
bindings. They do not add `Document` implementations or change `Decode`,
`DecodeAssessed` conversion, `AssessModel`, `Validate`, or existing profile
identities and digests. `AssessmentOptions` and `validation.Report` retain their
existing fields. A foreign or wrong root is a profile-admission error; ordinary
in-scope schema violations appear in the report with a nil operation error.

The container profile selects the internal `NewContainerMetadataContextPlan` in
addition to its XSD operations. Its exact source-only provider publishes
`ContextPropertyContainerMetadata` records with an optional
`ContextResult.Container` payload of type `ContainerContextResult`:

- `Role` distinguishes `ContainerRoot`, `ContainerRootfiles` and `ContainerRootfile`
- `Container` and `Parent` preserve exact source identities; the root has no parent
- `FullPathRaw` and `MediaTypeRaw` preserve XML-normalized source lexemes, while
  `FullPath` and `MediaType` retain assessed `xs:token` values
- Optional omission differs from a known empty string. Missing required
  `full-path` has absent raw evidence and invalid effective evidence with the
  exact XSD prerequisite cause
- `Content` proves complete, grammar-valid ordered child-role inventory for that
  occurrence. A complete rootfile inventory requires both the root and rootfiles
  guards; malformed or skipped children never turn a known subset into completeness
- `Ordinal` is one-based within a rootfiles parent and known only when that
  parent's inventory is complete. Non-rootfile attribute and ordinal fields are absent

These projections come from the same assessed source session, without reparsing
XML or exporting a generic fact tree. They do not prove ZIP properties, entry
existence, rootfile target kind, media-type classification or score/part ordering.
A consumer must also honor the containing assessment's input/syntax completeness.
The sounds profile additionally selects
the internal `NewSoundsMappingContextPlan`. Its sealed source-only provider adds
`ContextPropertySoundMapping` records with a `ContextResult.SoundMapping`
payload of type `SoundMappingContextResult`:

- `Role` distinguishes sound, any, solo and ensemble in the sounds assembly
- `Direction` distinguishes `SoundMappingToMusicXML` (sound reverse preference)
  from `SoundMappingFromMusicXML` (mapping-child forward preference)
- `Sound` and `SoundID` identify the owning occurrence and its assessed ID value;
  document-wide uniqueness remains an independent XSD identity obligation
- `LibraryID` preserves exact `xs:string` content, including whitespace and empty
  values; `Number` preserves the optional ensemble `xs:NMTOKEN` domain
- `Primary` preserves explicit yes/no, omission and causal invalid/unavailable
  evidence. There is no omission default or unique-primary requirement
- `Content` separately records the owning sound's grammar/enumeration evidence

Every alternative is a separate owned record in original source order. Known
fields remain available alongside invalid independent fields or content.
Provider completion means projection finished, not that every value is known or
that an application choice has been made. No sound dictionary lookup, numerical
ensemble restriction, library access, playback, or cross-file correspondence is
performed. Existing source DTD, capability, cancellation and finite budget limits
still apply. See [auxiliary source scope](docs/validation/auxiliary-source-scope.md).

`ContextResult` gains three optional typed payload fields: `SoundMapping`,
`Resource` and `Container`. Clients using unkeyed composite literals for that
struct must migrate to keyed literals. This additive context-family extension
does not change the `Report` or options struct shapes.

## Nonfetching local resource context

`ProfileSourceLinks` and `ProfileModelLinks` select XSD/local prerequisites plus
bounded local resource projections, without timeline, package parsing, URI
resolution or I/O. The internal `NewLocalLinkContextPlan` binds exact MusicXML or
Opus uses; the internal `NewSchemaHintContextPlan` binds standard xsi hint discovery
for one exact supported assembly. Existing eight profiles and option shapes are
unchanged.

`validation.ContextResult.Resource` is an optional, owned
`ResourceContextResult`, with exactly one branch: `ResourceLinkContext`,
`ResourceImageContext`, `ResourceSchemaHintsContext`, `ResourceGroupContext` or
`ResourceInstrumentContext`. `ResourceAttribute` exposes independent `Presence`,
`Raw` and effective validated `Value` evidence. `SchemaHintPair` retains namespace,
location and source origin. `ResourceLinkKind` distinguishes the score-link,
part-link, opus-score, opus-link, score-work-opus and bookmark families.

`ContextResourceLink`, `ContextResourceImage`, `ContextResourceSchemaHints`,
`ContextResourceGroup` and `ContextResourceInstrument` identify those branches.
`ResourceScoreLink`, `ResourcePartLink`, `ResourceOpusScore`, `ResourceOpusLink`,
`ResourceScoreWorkOpus` and `ResourceBookmark` are the closed link kinds.

`ResourceContextResult.SubjectDeclaration` is `Evidence[DeclarationID]`; consumers
read `.Value` only when `.State` is known, and retain unavailable reason/cause.
Known absence is different from unknown, unsupported and invalid evidence.
Missing attachment targets do not fail a new mandatory predicate. Schema-hint
pairing is known false for an observed unmatched token and known true for zero
or evenly paired tokens. `Raw`/`NoNamespace.Presence` distinguish absent from
explicitly empty hints. Intrinsic extraction can still succeed; this does not
change the accepted XSD list(anyURI) datatype. A separately selected package
consumer records known-false pairing as `OutcomeFail` under
`CategoryCapability`, `ClassPolicy` and `RolePredicate`. It retains
`schema-location-unmatched-token` and the exact context provider/subject cause.
That policy failure blocks `Satisfied()` through `BlockingProblems`; it does not
increment normative `Counts.Fail` or establish `ConformanceViolated`. Known-false
pairing alone does not make assessment incomplete. Direct models expose unavailable
xsi source history, which remains Unknown. XLink onLoad is metadata and grants no
retrieval authority.
See [local resource scope](docs/validation/local-link-context-scope.md) for the
bounded descendant-selection, image advice and exact typed-reference contracts.

### Owned intrinsic resource snapshots

`validation.Session.FinishWithResourceSnapshot` is an additive alternate terminal
finalizer. It runs the selected intrinsic phases before admitting an immutable
`ResourceSnapshot`, and releases session storage exactly once. Its first return
retains intrinsic findings. If snapshot admission fails, that returned `Report`
is also marked incomplete with a terminal summary; absent a known violation, its
conformance becomes undetermined. The returned operation error carries the cause.
A snapshot retains the canonical report, exact root/profile/assembly, selected
resource projections and schema-determined ID states under the same budget
account until `Close` or successful transfer to package assessment. It never
parses, fetches, retains a model, or exposes a post-finalization fact view.
Selected source-origin contexts are copied into both the canonical report and
selected snapshot projections. Physical, declaration, use and ancestry locations
retain their independent evidence states. Package adoption rebases only matching
typed document identities, preserving original byte/line/column, Entry and Path;
unrelated external references are unchanged. Repeated adoptions and getter copies
do not share mutable origin storage. Structural `ScopeComplete` does not establish
origin-provider completeness. The prior explicit
`source-origin-resource-snapshot-unsupported` export refusal is replaced by this
bounded capability; missing origins may still remain unavailable in a snapshot.
Initial copying is cancellation-aware after overlapping storage admission, and a
post-admission failure releases that lease while preserving the intrinsic report.
Exact package intrinsic profile/digest admission remains independently required;
export support alone does not enable a DTD graph/package profile.

Default-buffer capacity is limited for source-origin graph compositions: every
selected origin row has two independently owned snapshot copies while session
storage still overlaps. On Go 1.27.2 linux/amd64, the single-note-per-measure
`reviewBudgetScore` fixture with `ProfileResourceGraphInternalDTDDictionaries`
now admits 610 measures (81,783 source bytes) with defaults; 611 measures
(81,917 bytes) refuse the 64 MiB live-buffer allowance. The earlier published
531/532, PR 43 568/569 and 576/577 boundaries are historical; all of these
cases now complete. Exact/one-short snapshot controls pin 62,617,598 bytes for
569 measures and 67,175,452 bytes for 611 measures.
The review reported the
500-measure case at approximately 62.9 MB of the 67.1 MB allowance, a
previous boundary of 494 measures (about 66 KB), and comparison boundaries of
about 114 KB for standalone `ProfileSourceInternalDTDDictionaries` and 168 KB
for its "legacy graph" comparator. These are fixture/profile-specific
observations, not a general document-byte guarantee or heap/RSS limit.

The separate 2,000-empty-measure, three-`#m1` fixture is only 86,052 source bytes
but still refuses snapshot admission at default budgets: 54,486,816 live bytes
plus a 34,741,264-byte request exceed 67,108,864 bytes. Its regression that uses
512 MiB of buffer isolates the unchanged 20,000,000-step limit; it does not prove
all-default capacity, nor is 512 MiB a measured minimum or recommendation.
Choose explicit finite limits for the actual profile and input. See the
[fixture definitions, provenance and boundary checks](docs/validation/resource-fragment-budget-evidence.md#default-buffer-capacity-is-fixture-specific).
The [corpus resource correction receipt](docs/validation/corpus-resource-fixes.md)
distinguishes the current boundary from the earlier published measurements.

`ResourceSnapshotHeader.LinkProjectionSelected` records selection of an
`ImplementationAvailable` sealed local-link plan for the established assembly.
It does not assert projection or scope completion. Absent or unsupported
implementation support does not establish that witness. An empty projection from
a profile without the witness proves no link absence.

Snapshots are single-goroutine objects. Serialize getters, ownership transfer and
`Close`. Standalone `Close` is idempotent. Closed header/count access returns zero;
closed content access returns a session error. Transferred content access
reports that ownership belongs to the package, rather than reporting a closed
or uninitialized snapshot. Getters produce defensive,
caller-owned API copies of already admitted immutable data; retaining adapters
must account for their own working copies. Existing `Finish`, options, profile
identities and ordinary assessment behavior are unchanged.

### Sealed package and resource contracts

`validation.NewPackageCatalog` accepts closed `PackageRuleBinding` selectors and
constructor-produced `PackagePlan` operations separately from XML catalogs.
Package rules carry an empty XML selector, package scope and exclusive package
target. Package profiles carry no XML roots and include a fixed assembly/target
intrinsic profile/digest table. Existing `NewCatalog` XML invariants and catalog
digests remain unchanged. The package catalog is metadata, not a parser or a
runtime expression language.

`ResourceRequest` and `ResourceResult` have exactly one corresponding document,
image, association or role branch. Core validators check bounded shape and exact
request identity; only root orchestration reads and verifies streams.
`ResourceDocumentRequest.RequestedRevision` is available to low-level callers,
but root orchestration leaves it empty because the target canonical identity and
revision are not known before resolving. A retrieval URI is not a revision pin.
`ResourceResult.RetrievalURI` is the optional external retrieval base for a
returned document. Empty leaves that base unavailable. When nonempty, it must
parse as an absolute URI; its fragment is ignored for retrieval context. Root
orchestration rejects malformed or relative values with
`validation.ErrInvalidConfiguration` after the resolver callback returns and
before reading the body. This checks returned evidence; it does not sandbox the
resolver's already-performed transport.
Every returned nonnil body transfers ownership even on error or rejected
metadata.
A newly returned alias body still consumes actual external byte/work quotas;
parse reuse does not establish transport-byte reuse. Caller resolver code owns
transport and redirect authorization; validation cannot control I/O it already
performed. Source pins use verified SHA-256 digests. Current state uses opaque
per-run snapshot identity, never an invented source hash.

Returned bodies grow in owned contiguous buffers. Growth admits the old and new
capacities simultaneously before releasing the old buffer; spare retained
capacity can approach twice the body length. Even when external-byte and work
allowances suffice, a 33 MiB body cannot fit the default 64 MiB buffered-byte
limit: growth alone gives an approximately 32 MiB body ceiling before other live
storage and scratch. `MaxExternalBytes` is not a promise that a body of that
size fits the independent work and live-buffer budgets. Use explicit finite
overrides when the selected workload needs them.

Association replies bind complete source/target pins, exact local subjects and
normalized IDs. Partial mapping misses are unknown; known negative membership
requires authoritative exact relation coverage. Optional target IDs must have
verified kind/owner when supplied, but there is no required cross-file ID equality
or bijection. Role results echo the whole bounded package request scope. Image
metadata names an explicit common dimension unit and never infers MusicXML tenths
or physical density.

### Named package and resource-graph profiles

The additive `ProfileResourceGraph`, `ProfileMXLSource` and `ProfileMXLModel`
identities select separate bounded graph/package scopes. Every graph report has
`TargetPackage`, package subject selectors and an empty XML `SupportedRoots`
list. None is accepted as a standalone `InspectXML` or `AssessModel` profile,
and none grants resolver or filesystem/network authority.

- `ProfileResourceGraph` describes a finite selected resource graph rooted in
  source XML or a current model. Plain document boundaries do not assert ZIP
  facts. Authorized nested MXL boundaries receive applicable package checks.
- `ProfileMXLSource` describes original raw-MXL evidence and source intrinsic
  reports. A decoded current model cannot substitute for original XML proof.
- `ProfileMXLModel` describes the current primary `Document`, represented package
  fields and separately selected retained source resources. Original headers,
  compression/encryption, physical order and original container lexical presence
  are outside its current-boundary claim. They are never recovered by encoding
  the package. Raw checks remain active for separately reached raw-MXL boundaries.

All three select the same sealed, representation-conditional check inventory.
Mandatory package/root/link/association conditions stay separate from optional
mimetype, group, image and filename/MIME advice, compression observation and
schema-discovery capability policy. Existing intrinsic XSD/local predicates are
reused rather than copied into package reports. Per-report counts remain scoped;
the catalog does not claim one rule per research record or full conformance.

The fixed intrinsic table is part of the package catalog digest. Every profile
pins source MusicXML/Opus to `ProfileSourceLinks`, source Container to
`ProfileSourceContainer` and source Sounds to `ProfileSourceSounds`.
`ProfileResourceGraph` and `ProfileMXLModel` additionally pin current-model
MusicXML/Opus to `ProfileModelLinks`. Raw-MXL source assessment accepts no current
model row. There are no Container/Sounds `Document` model roots. Digests derive
from the actual compiled intrinsic catalogs, including their selected providers;
they are not copied constants that can drift from auxiliary metadata changes.

Revision 2 of the schema-hint pairing rule assesses a current model as it would
be encoded: the model cannot represent xsi hints and the encoders write none, so
the pairing does not apply to it and neither fails nor blocks completion. The
original hints are not reconstructed. Before revision 2 every
current-model graph/package bundle stayed incomplete on this rule. Source
discovery separately preserves namespace/location pairs and an unmatched
trailing URI. It never changes the accepted odd/even
`list(anyURI)` datatype contract or automatically loads/selects schemas.

Read the profile's `NormativeClosure` for its exact bounded claim. Central-only
mimetype extra fields, other leading padding families, the complete media-type
set, outer compressed-rootfile and linked-target indirection, unproved
association/role evidence and full ZIP/JAR/DEFLATE closure remain explicit
interpretation/capability gaps when reached. Technical nested traversal does not
settle normative indirection.
The pinned MusicXML 4.0 schema is not a universal instance `version=4.0`
restriction. All eight earlier profile identities, digests and report shapes are
unchanged. See [package profile scope](docs/validation/package-profile-scope.md)
for the sealed check inventory and provenance.

The internal package-session hook (`CompiledPackageCatalog.Begin` before
v0.2) creates a typed `PackageSession` over one shared account. `OpenPackage` establishes acyclic encapsulation with independent entry
namespaces; linked cycles use fresh adoptions of canonical intrinsic assessments.
`AddResource` transfers only a core-created snapshot on the identical account.
It requires an established assembly matching the fixed target/profile/catalog
interpretation, regardless of whether the intrinsic report is satisfied. An
unknown assembly is refused explicitly; `RecordResourceFailure` preserves the
untransferred original failure rather than inventing an assembly from table rows.
`AdoptResource` requires explicit package scope, a fresh document ID for each
linked occurrence, and an explicit single root marker.
The root must own its exact expansion context: a root adoption is refused if an
earlier linked adoption already owns that same assessment/package/base/member
context. A different context remains independently adoptable. Successful transfer
invalidates the old snapshot owner. Live indexed reads admit work and temporary
copy storage; returned copies belong to the caller. Package completion releases
all live leases, while immutable `AssessmentBundle` getters remain available.
`PackageSession.TerminalCause` reads the accumulated terminal error without
charging work or admitting another event, including after completion.
Adapters can preserve `validation.ErrProcessingPolicy` alongside existing legacy
error identities for genuine technical processing refusals. The terminal report
uses category `policy` and reason `processing-policy`, remaining incomplete rather
than claiming normative invalidity. Resource limits, cancellation, deadlines and
established malformed-source evidence retain their existing higher precedence;
ordinary transport failures and unsupported formats must not acquire this marker.
`AssessmentBundle.OmittedPackageDiagnostics` preserves the first omitted blocking
package rule/subject/boundary/outcome/category/reason and a saturating omission
count when intrinsic children exhaust shared diagnostic quotas. Its fixed 6 KiB
reserve is charged at Begin, and capture allocates nothing after exhaustion. It
is separate from Findings and does not expand MaxDiagnostics; full locations,
causal detail and rule messages may be omitted. Counts remain authoritative.

The bundle retains one package report and one report per canonical intrinsic
assessment. Required child failures cannot be flattened away or multiplied by
edge count. `RecordResourceFailure` is write-once and preserves the best original
intrinsic report of an untransferred child when export or later package admission
fails.
If ordinary report retention is refused, its reserved explicitly truncated
identity/count summary still preserves independently known results. An unavailable
full content pin is marked as such and cannot establish canonical resource identity
or successful package completion.
Failure-only ingress accepts nonnil canceled contexts and prior terminal errors
without charging new work or attempting a full report copy after termination.
It validates bounded conformance/count/target metadata and pin consistency before
storing anything, and refuses an already transferred exact assessment. Invalid
failure metadata cannot replace an existing blocker. Before termination, ordinary
full retention may still succeed within the shared quotas.

`ObserveEdge` consumes a once URI-decoded fragment component. Empty fragments
address the whole document. Published legacy profiles retain schema-ID NCName
shorthand and their schema-uniqueness gate. The explicitly DTD-composed source
profiles instead join effective first-bound DTD ID attributes with schema IDs
and select the first matching owner element in document order. Intrinsic XSD
identity failures remain independent. These new profiles intentionally
migrate their closure and catalog identities; this is not a frozen-digest claim.
Each observation requires explicit `Indirection` evidence. Known false asserts
that the adopted XML is the directly selected target; known true records an
archive-to-primary substitution. Omitted evidence is invalid ingress, not an
implicit direct-target claim. An indirect target retains its adopted inner XML
and intrinsic report, while outer target, applicable group-link and association
predicates remain Unknown with `nested-link-indirection-unresolved`. Its outer
fragment is not resolved against the inner primary's ID index, and the root
adapter does not request an association proof for that substituted target.
Indirection belongs to the incoming occurrence: direct XML members, inner
outgoing links and aliases retain their own independently established evidence.
Complete exact ID absence fails, incomplete ID scopes stay unknown, and other
fragment grammars stay unsupported in these bounded shorthand profiles. DTD
shorthand requires both declaration-domain and occurrence-enumeration seals,
plus complete XSD identity and element order; a later known candidate cannot
exclude an earlier unknown one. Source `SourceDTDIDDomain`,
`SourceDTDIDAttribute`, and `CompleteSourceDTDIDs` ingress matches each actual
admitted non-namespace attribute before XSD normalization. Declaration lexical
names, first-binding identity, actual DTD-normalized value, default kind and
independently available declaration/use sites stay owned with the snapshot.
Legal explicit `ID #REQUIRED` and `ID #IMPLIED` declarations are supported;
illegal ID value/fixed defaults retain limited typing evidence and leave the
selection authority incomplete, without inventing DTD validation findings.
No external declarations are fetched and current models never manufacture
historical DTD absence or IDs.

The internal engine query for DTD ID selection is profile-wide and is used
before parsing the root to decide whether the scanner should capture DTD ID
facts. The session query is the narrower, admitted-root assembly query: it returns false before an admitted root, for a current-model target, or
when that assembly has no selected DTD ID consumer. A true result reports
selection only; it is no certificate of completed enumeration, valid declarations
or available DTD identity evidence.

The resource-graph profiles elect the internal XML-fragment link plan
(`NewXMLFragmentLinkContextPlan(schema, sourceInfoset)` before v0.2) for the
registered XML-media fragment baseline. This adds the
XPointer Framework and mandatory `element()` scheme, and pins that interpretation
in the catalog digest. `sourceInfoset=true` requires the selected source
provenance/declaration capabilities and joins DTD and XSD IDs; `false` uses the
selected ordinary source or current-model XSD domain only. The snapshot owns a
complete, bounded index of all admitted elements, their actual parents and
occurrence order, including elements without schema IDs or context rows.

The Framework validates its entire syntax before interpreting parts. It reverses
only its `^^`, `^(` and `^)` escapes; URI percent decoding has already happened
once. Parts are attempted left to right. Unsupported schemes are skipped;
`xmlns()`, prefixed schemes and arbitrary `xpointer()`/XPath are not elected.
A supported unavailable result cannot be skipped. Malformed `element()` data
and complete out-of-range paths are empty parts and permit fallback; malformed
Framework syntax is separately reported. Exhausting supported empty parts fails
the pointer; an entirely unsupported scheme sequence retains an unsupported
capability result.

`element(ID)` uses joined first-owner selection. `/1` starts at the document
root, and subsequent positive, nonzero-prefixed integers select element children
only; text, comments and processing instructions never affect ordinals. Arbitrary
width ordinals are validated under the shared budget and safely bounded against
the finite admitted tree without integer truncation. Missing tree/ID closure
keeps resolution unknown. `ResourceAdoption.ResolveXMLFragment` exposes an owned,
adoption-remapped selected result without exposing retained trees or DTD tables;
`selected=false` preserves the legacy adapter route. All pointer input, temporary
parser storage, lookup work and copying share the root account. Direct XML target
evidence remains mandatory: an outer MXL fragment is never applied to its inner
primary XML.

The selected baseline follows [RFC 7303 sections 5 and 9.6.1](https://www.rfc-editor.org/rfc/rfc7303#section-5),
[XPointer Framework sections 3–4](https://www.w3.org/TR/2003/REC-xptr-framework-20030325/),
and [element() section 3](https://www.w3.org/TR/2003/REC-xptr-element-20030325/#language).
This election adds no document `xml:base` or `xml:id` consumers: the fixed
MusicXML/Opus/Container/Sounds assemblies do not admit those attributes.

Unknown/absent selected targets still charge
linked-document admission. Each known edge uses its own nonroot adoption distinct
from its source. A target may belong to another explicitly admitted package
boundary; a known empty fragment target must be its exact nonzero adopted root.
`MaxLinkedDocuments` counts nonroot adoptions, not just external resolver calls.
The first linked raw-MXL package can require three admissions: container, primary
and incoming edge. An exhausted counter is checked before I/O, but positive
remaining capacity does not guarantee that all internal adoptions will fit; their
number depends on the returned content.

`ObserveImageMetadata` accepts an exact image request/result for a selected
intrinsic template. Bundle image advice exposes exact dimensionless aspect ratios
and agreement with two specified dimensions. Pixel dimensions require explicit
pixel-aspect-ratio evidence; physical dimensions use their stated common unit.
No DPI, rendering result, or pixels-to-tenths conversion is inferred. Missing
optional metadata remains nonblocking advice. `ObserveFileMetadata` supplies
bounded scoped filename/media-type recommendations; suffixes never prove content
validity. Binary members are outside this MusicXML filename/media advice scope.
`validation.ClassifyResourceMediaType(ctx, account, value)` shares bounded declared
MIME applicability across package checks and adapters. It accepts registered
MusicXML base types case-insensitively. Syntactically admitted distinct unknown
parameters do not change that base identity (RFC 2045 sections 5–5.1); this includes
charset on the compressed type without selecting a decoder. Repeated parameter
names are case-insensitively erroneous even with equal values (RFC 6838 section
4.3), and remain Unknown along with malformed declarations and unreviewed base
types. RFC 2231 raw names such as `foo`, `foo*`, and `foo*0` remain distinct for
this duplicate check; no decoded-name equivalence is inferred. The bounded ASCII
declaration classifier does not elect internationalized header syntax. It never
verifies content or a resource pin. Callers preserve absent and invalid evidence before invoking it;
present-empty is unresolved. Scalar/step costs and temporary parse scratch use
the supplied account; cancellation/quota errors remain errors. Parser and raw-name
index scratch is conservatively reserved together, so exact historical near-limit
MIME budget outcomes can change without raising defaults. Bundle getters also
preserve supplied mimetype prefix observations; incomplete retained prefixes cannot
prove content mismatch.

`ObserveRootfile.Source` is a well-formed adapter-asserted provenance reference,
not a core-verified membership claim about an adopted container snapshot. The raw
adapter remains responsible for parsing the original container and projecting its
actual rootfile occurrences; existing container and scalar obligations are unchanged.

A `ResourceCurrentMXL` boundary assesses representable logical rootfile/resource
relations. It excludes the original ZIP container-entry representation and raw
header claims, which cannot be reconstructed from a model. A `ResourceRawMXL`
boundary keeps those obligations. `PackageIdentity.PinUnavailable` is restricted
to an observed raw MXL prefix with no whole-body digest. It never enters
canonical pin tables and unconditionally keeps the bundle incomplete, while
preserving already known local violations.

Resource expansion is explicitly scoped by canonical assessment, package
boundary, `BaseURI`, and `ArchiveEntry`. Profile/digest identity is part of the
canonical assessment; resolver configuration is fixed by the enclosing run.
`ExpansionOwner` returns the first adoption for that exact context. Only that
owner may publish outgoing edges, association replies and selected image
metadata. Aliases keep fresh document IDs and linked-document charges, including
cycle/back references. `SealResourceExpansion` requires complete intrinsic scope
and exact coverage of all outgoing templates before accepting a complete
assertion. Each owner must be sealed before its package inventory. A retained
child snapshot may still support an independently known target-kind finding
after an intrinsic error; it does not authorize continued child expansion after
that error. Unsealed, failed or partial expansions keep the whole bundle
incomplete; aliases do not convert in-progress work into vacuity.
`AdoptionContext` exposes this immutable context and completion evidence.

Image requests carry the same explicit base URI, package and archive-entry scope,
which must match their exact source expansion. Canonical identity is never used
as a retrieval base. Known numeric image fields individually consume the scalar
limit even when another field is unavailable and no ratio can be calculated.

### Package role member scope

`SealResourceMembers` closes admitted member discovery separately from outgoing
expansion and final package enumeration. A complete role request requires that
seal and coverage of every admitted member context. Each `ResourceRoleMember`
keeps the immutable pin, literal archive entry and explicit base evidence;
identical bytes at two member paths are not one role occurrence. Exact member
records and the resource pin list are echoed by the provider. Legacy pin-only
scope cannot resolve an ambiguous physical-member association.

A nested role request has a boundary-local member path, package identity and
resource pin, but no enclosing archive-entry chain. Its `BaseURI` alone is not a
self-contained nested retrieval address; a provider requiring that chain cannot
recover it from the request. Diagnostic parent/entry provenance does not extend
the provider request.

The final package `Roles` seal records completion of the selected observation
pass. It does not prove authoritative or complete role classification; separate
`ScopeComplete` and exact score/individual-part membership evidence govern that
claim. A relevant request includes every admitted member, including
`META-INF/container.xml`. Complete negative coverage requires explicit
score=false and individual-part=false evidence for that container member too.

Role requests are dispatched only with selected `ResourceRoles`, a supplied
resolver, a usable package inventory and a selected musical document. Plain
XML/model roots do not themselves create package-role requests. Nested raw
packages reached from them may do so. Neither root kind nor suffix classifies a
document as a score or individual part.

Without a role provider decision, `MX40-PACKAGE-score-before-parts` (revision
2) decides from the MusicXML link semantics, after `container.xsd`: when a
MusicXML file has both a score and separate part files, the score is the file
in the first rootfile, and the part files can be linked from it. In a package
whose entries, rootfiles and edges are completely enumerated:

- a `<part-link>` target is an individual part file and its source a score;
- a first rootfile that links part files is the score, so the rule passes;
  part links only from other documents keep it undecided;
- without part links, an opus `<score>` target is a score, opus-link and
  `work/opus` targets are opus documents (`MX40-PACKAGE-link-target` checks
  their root kind), and a later rootfile with an established non-MusicXML
  media type, such as a PDF rendition, is not MusicXML: the declared media
  type is authoritative, as for `MX40-PACKAGE-rootfile-root`. When every
  other entry, ignoring directory entries, is so classified, separate part
  files are excluded and the rule does not apply; an unclassified entry, such
  as an unlinked second MusicXML file or an image, keeps it undecided.

The structure decides only where a role provider stated nothing: a known
provider fact is never overridden, even when it decides nothing. Unread or unpinned entries
keep the requested role universe incomplete; the literal `mimetype` metadata
record is the only physical-entry exclusion. Complete provider coverage of a
smaller selected table cannot establish absence of roles outside that table.
This includes directory entries and binary or unread members without admitted
role contexts. Such a package cannot produce a satisfied bundle under the
shipped profile, even if the provider classifies every requested member. This
conservative capability limit does not mean that those physical entries make an
MXL invalid.

## Original and current MXL assessment

`InspectMXL(ctx, reader, MXLAssessmentOptions)` requires
`Graph.Profile = ProfileMXLSource`;
`AssessMXLPackage(ctx, package, MXLAssessmentOptions)` requires
`Graph.Profile = ProfileMXLModel`. Neither chooses a profile implicitly.
Both return `(validation.AssessmentBundle, error)`. These additive
entry points preserve all legacy decode, encode, resolve and transport behavior.
The caller's reader stays open; no built-in filesystem or network resolver is
introduced.

The raw observer supports ordinary single-volume, non-ZIP64 STORE/DEFLATE
archives with unsigned or signed data descriptors. Unsupported features are
processing limitations, not normative MusicXML violations. Symbolic-link-mode
entries are refused by transport policy even though no paths are extracted or
links followed; this intentionally differs from legacy decoding. Unsafe paths
and duplicate names retain `ErrMXLInvalidPath` and `ErrMXLDuplicateEntry` through
`errors.Is`. Descriptor/encryption flag disagreements are framing errors;
independent EFS and compression-level flags are retained, while unsupported bits
in either header remain explicit capability limitations.

Raw assessment retains independent observed local/central header facts, original
mimetype prefixes and complete or explicitly partial entry inventories. The
original container is assessed once with `ProfileSourceContainer`; ordered
rootfiles come from that exact immutable source snapshot. Root and rootfiles
content proofs both gate complete enumeration. Later established non-MusicXML
alternate renditions are not fed to the musical XML parser. Primary target kind,
media type and intrinsic child conformance remain separate evidence.

Current package assessment walks `Document` directly with `ProfileModelLinks`.
An empty `RootFiles` uses the encoder's current-state `score.musicxml` default.
A nil `MediaType` is absent and means the MusicXML media type; a non-nil one,
even empty, is decided as written, as the encoder writes it. This does not
recover the original container, source order or headers.
Selected `Resources` bytes receive admitted independent copies before parsing.
Callers must not mutate any inputs during assessment. Current field locations
identify rootfile occurrences without inventing source XML nodes.

Every package-local member identity incorporates the exact enclosing package pin
and literal path unambiguously. Nested archives keep separate entry scopes and
share the root account. Raw nested technical traversal preserves its inner source
reports and the outer rootfile/linked-target indirection interpretation gaps.
Neither a matching nor a different inner XML root settles the outer direct-target
predicate. Parse reuse is
canonical resource/profile-specific; graph expansion additionally includes the
actual package and effective retrieval context. The primary is seeded before
archive-local backreferences, and a repeated reference is a distinct adoption.

All five `MXLOptions` byte caps remain independent of graph budgets. Raw
archive, metadata, document and resource limits apply to their represented byte
streams; `MaxResourcesBytes` is aggregate across all selected expanded entry
bytes, selected current resource copies and bounded external bodies in package
scope. For assessment, this aggregate includes selected `mimetype`, container
and primary entry bytes. Legacy `DecodeMXLPackage` excludes those three entries
and caps only additional resources with `MaxResourcesBytes`; the aggregate has
different scope at these entry points. Nested compressed entry bytes and their
inner expansion are distinct represented bytes. Current models have no
compressed archive or serialized primary XML bytes to measure; no encoding is
performed to fabricate those measurements. Current rootfile string storage is
separately bounded by the metadata cap. Newly returned external aliases count
again even when their verified intrinsic parse is cached. Caller and resolver
I/O cancellation remains cooperative; all owned leases are released on
completion, including technical errors and cancellation.

Zero-valued graph budget fields select finite defaults, including 64 MiB each for
primary input, external represented bytes and buffered working storage,
20,000,000 rule steps and depth 1024. ZIP transport, CRC and buffer relocation
charge bounded work units of at most 4096 bytes, with empty reads and EOF probes
also charged; byte quotas remain exact and growing buffers reserve old and new
storage simultaneously. The independent MXL byte defaults are
64 MiB archive, 1 MiB metadata, 256 MiB document/per-resource and 512 MiB aggregate
selected resources. The stricter applicable limit wins; those legacy byte caps
do not enlarge the shared graph account. Set finite overrides explicitly when
needed. For example, set `Graph.Budgets.MaxBufferedBytes` to `128 << 20`
(134,217,728 bytes) for a finite buffer-only increase; other limits stay at their
defaults. Raising a budget does not recover unread-DTD or discarded-model
evidence. Exhaustion is incomplete processing, not a MusicXML violation or proof
of conformance.

`MXL.MaxXMLDepth` independently limits represented XML element depth, including
scalar leaves, for container/primary/local XML and selected linked XML reached
from an actual MXL context. Zero uses the existing MXL default of 256; values
above 4096 or below zero are rejected before input is read. The shared
`Graph.Budgets.MaxDepth` remains active separately for XML/model and graph/package
depth. A smaller XML cap does not restrict the number of shallow graph hops.
Plain XML siblings remain governed by their graph settings. Cached intrinsic
assessments retain a bounded peak-depth witness, checked against each receiving
MXL context before adoption; reuse never bypasses the cap or reparses canonical
bytes. Depth refusal is incomplete processing and preserves known reports.

## Selected resource graph entry points

`InspectXMLWithResources(ctx, reader, LinkedAssessmentOptions)` assesses original
MusicXML/Opus source; `AssessModelWithResources(ctx, document,
LinkedAssessmentOptions)` assesses the current model directly. Both require
`Graph.Profile = ProfileResourceGraph` and return
`(validation.AssessmentBundle, error)`. MXL entry points require their corresponding
`ProfileMXLSource` or `ProfileMXLModel` explicitly. `Graph.Budgets` creates one
finite account shared by all parsing, snapshots, graph work, metadata and packages.
No child gets a fresh budget. Ordinary normative findings can have a nil operation
error; callers must inspect `bundle.Satisfied()` as well as the error.

`Dependencies` is a closed, duplicate-free selection of `ResourceDocuments`,
`ResourceImages`, `ResourceAssociations` and `ResourceRoles`. An empty selection
invokes no external resolver. A nil or typed-nil `Resolver` leaves selected
external dependencies unavailable. Invalid options are rejected before input or
resolver I/O. Package-local traversal uses only the supplied package resources;
XLink `show`/`actuate` intent does not authorize external retrieval. Schema hints,
DTD declarations, external entities and XInclude never start hidden loads.

The caller-provided `validation.ResourceResolver` owns transport, custom-scheme
and redirect authorization, including cooperative cancellation of blocked I/O.
The library verifies exact request/result identities, resource pins and actual
returned bytes. Those checks do not sandbox I/O already performed by the resolver.
Transferred document bodies close exactly once, including rejected responses and
read errors; caller-owned primary readers remain open.

`Identity.CanonicalID` is a cache identity, not a retrieval base. An empty identity
becomes run-local. `Identity.RetrievalURI`, when supplied, is an absolute URI and
is the only supplied primary external retrieval base. Actual source digests cannot
be overridden by metadata; current models receive fresh opaque snapshot identities.
Optional primary filename advice uses the actual supplied retrieval URI of the
admitted source/current boundary, never its canonical identity. Current-state
advice does not reconstruct an original source filename or MIME declaration.
The first verified canonical representation pin is immutable for the run. A
changed returned revision cannot silently replace the same requested resource.
Each newly returned body is read, charged and verified even when its intrinsic
parse can be reused. A raw-MXL alias that reuses an existing package releases its
new body immediately instead of retaining another borrowed archive. Distinct
base/package/archive contexts expand independently;
cycle aliases preserve separate adopted document identities and linked quotas.

`PackageReport`, `ResourceReport`, `Adoption` and `AdoptionContext` expose separately
scoped results. Counts are not a flattened total over references. `Failure` exposes
a terminal summary, and `FailureReport` exposes its complete original report when
retention was admitted while the package session was healthy. After ordinary
package admission terminates, even newly freed storage does not reopen report
measurement or copying: failure ingress retains only the prepaid bounded summary.
A truncated fallback is explicit and cannot satisfy the bundle. Published getters
return independent owned values and support concurrent reads; live sessions,
snapshots and adoptions require serialized single-owner use.
Additional selected XML resources, including later XML rootfiles and archive-local
XML edges, obey both document and resource byte caps. A metered fixed four-byte
prefix classifies local selected bytes before growing body allocation. ZIP content
uses archive and resource caps; other content uses its selected XML cap. Names
and declared media types do not select the representation. The prefix counts once
in shared byte usage, including when later classification rejects the body.
Graph depth follows the actual incoming link path through package boundaries
instead of restarting at a nested primary. Package containment depth remains
independently checked. Same-context backreferences
reuse the admitted expansion rather than manufacturing a normative cycle error.
At the graph-depth boundary, a new target expansion is refused before resolver
I/O or archive-member selection. Reuse requires a same-entity or already admitted
archive expansion context; matching retrieval URI text alone does not establish
canonical identity. A missing member in a complete local archive inventory still
produces an authoritative-absence finding without I/O; an incomplete inventory
leaves the target unknown. Every reused incoming edge still receives its own
adoption and linked-document charge.

Optional file advice preserves exact presentation provenance. Original rootfile
name/media fields retain absent versus present-empty evidence; current rootfiles
retain current-field provenance. A selected local member or resolver body receives
metadata-only observation after its complete byte digest is verified, before XML
parsing or snapshot export. Core `ObserveFileMetadata` requires either an admitted
physical `Entry` or an exact selected `Source`/`Template`/`Href` occurrence,
with those modes mutually exclusive; existing boundary/adoption observations
retain their exact optional context. For the same canonical identity, revision
and representation, observed file bytes must agree with earlier and later
admitted pins, without granting parsed capabilities. Root orchestration emits one
observation per exact occurrence/context, so later adoption does not duplicate
its advice. This observation cannot admit a parsed resource, adoption, ID, role,
root validity or graph closure. Consequently known advice may
survive an independent parsing/export error without certifying that file as valid.
Unselected auxiliary container/sounds parsing does not acquire musical-file advice.

File metadata is scoped to the admitted package plus exact member occurrence or
selected source-link template and effective retrieval context. Distinct aliases
remain distinct even when their verified parse is reused. A name derived from an
HTTP(S)/file retrieval URI is an escaped last path segment, not a claim about a
physical filename; escaped slashes are never decoded into new member boundaries.
Opaque/custom retrieval URIs may leave the optional name unavailable. Malformed
or unreviewed supplied external MIME remains Unknown; absent MIME stays absent
rather than receiving a guessed default. Optional advice never supplies a document
classification rule.

`PackageEntryObservation.UTF8Name` reports whether the observed name bytes are
valid UTF-8. It does not prove the writer's declared encoding. The EFS bit remains
separately available in `Local.Flags` and `Central.Flags`; the bounded name advice
does not impose a mandatory EFS rule.

An original ZIP entry with a non-UTF8 byte name may retain `RawName` header
observation independently of transport path policy. Raw names are bounded exact
bytes, exclusive with textual `Name`, and do not enter safe member lookup or
rootfile, file-metadata or role-completeness capabilities. Encoding advice uses
an ASCII hexadecimal diagnostic label; no legacy decoding or filename
normalization is inferred. The existing unsafe-path operation error still stops
selection, while an independently observed encoding recommendation remains visible.


## Encoded semantic compositions

`ProfileSourceSemantics` and `ProfileModelSemantics` select the bounded combined
encoded MusicXML interpretation through existing `InspectXML`, `AssessModel` and
corresponding dictionary entry points. `ProfileModelSemantics` is validation
line 3 of both [mandatory assessed loaders](#mandatory-assessed-loading); their
line 2, `ProfileSourceLine`, keeps the XSD, provenance and schema-hint rules of
`ProfileSourceSemantics` and adds the representability rule, which the
diagnostic source profiles no longer carry.
Source selects actually read internal declarations and owned origins; model
selects current-state facts. Inspection and standalone model/package assessment
keep explicit dictionary selection through `DictionaryOptions`; the assessed
loaders provide the required bundled defaults. Neither profile itself loads data
automatically or selects standard-sound recognition and dictionary-backed
rendering advice for score nodes. The current model holds no `xsi`
attributes, so `ProfileModelSemantics` no longer requires the schema-hint
provider, which always found none there.

For resources, use the existing dictionary graph/MXL entry points with the exact
compatible `ProfileResourceGraphSemantics`, `ProfileMXLSourceSemantics` or
`ProfileMXLModelSemantics`. Each catalog pins the actual source/model intrinsic
profile and digest; unknown assemblies/targets are rejected. Source Container
metadata remains physical/effective-aware. Existing explicit Sounds auxiliary
support does not introduce public Sounds traversal or filename discovery.

These profiles elect bounded barline/repeat proofs and the registered XML
Framework, shorthand and `element()` interpretation. They preserve direct-target
indirection gates, complete-domain requirements, current-model source gaps and
independent intrinsic failures. They do not imply general XPath, renderer,
external desired-score authority or universal conformance. See the
[scope and acceptance boundary](docs/validation/semantic-composition-scope.md).

[bundled-data-notices]: THIRD_PARTY_NOTICES.md#embedded-copies-of-the-pinned-dictionary-data

[loading-capacity]: docs/validation/assessed-loading-pipeline.md#official-score-capacity-control
[loading-cost]: docs/validation/assessed-loading-pipeline.md#dictionary-import-cost
[loading-budgets]: docs/validation/assessed-loading-pipeline.md#budgets-and-capacity
