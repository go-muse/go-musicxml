# Mandatory assessed-loading pipeline

`DecodeAssessed` and `DecodeAssessedWithDictionaries` now always require the
existing `ProfileSourceSemantics` followed by direct `ProfileModelSemantics`.
This is a breaking wrapper-contract change. It does not widen either profile,
change its catalog digest, or establish complete MusicXML conformance.

Ordinary `Decode`, typed decode helpers and `DecodeWithOptions` remain transport
operations. `InspectXML`, `AssessModel`, their dictionary variants and the
resource/MXL inspection and assessment APIs retain their explicit profiles and
data-selection contracts. There is no assessed package decoder in this change.

## Loading and report contract

The loaders retain one source/parser owner. Original source observation precedes
model namespace filtering, attribute projection and unknown-element skipping;
conversion uses that same stream. A recoverable mapping failure drains the same
healthy parser without resuming the failed reflection decoder. There is no reader
replay, second XML parse, or encoding the Go model back into XML for assessment.

The acceptance sequence is:

1. Validate context, profile, decode limits, budgets and dictionary configuration
   before reading input or calling a dictionary source.
2. Import one dictionary set and assess the original source with
   `ProfileSourceSemantics`, while converting through the existing mapping path.
   Source EOF, source acceptance, successful conversion and no earlier operation
   error are required to advance.
3. Assess the actual converted Go object with `ProfileModelSemantics`. The walker
   uses current field/index facts directly; it does not invent source bytes,
   lexical values, namespace declarations or discarded attributes.
4. Return the document only if both reports are satisfied, conversion succeeded
   and no operation or close error occurred.

`DecodeResult` separates four outputs:

- `Source`: the original-source report, including the best partial report on
  interruption when available.
- `Conversion`: mapping state and bounded findings. `ConversionConverted` is not
  an exact lexical or numeric round-trip guarantee.
- `Model`: the independent current-model report. `Model.Requested` is false when
  an earlier operation failed or source/conversion acceptance was not reached;
  this is not a pass.
  Once attempted, the report is retained even if model assessment fails.
- `Document`: nonnil only on complete successful loading. It is nil for source or
  model violation, incomplete required assessment, conversion failure, operation
  failure or close failure.

The model stage matters even after satisfied source assessment. For example, an
exact positive source decimal may underflow to zero in an existing `float64`
field. Conversion can succeed while the resulting current model violates its
selected contract. Model locations remain field/index locations; a model-stage
failure does not rewrite the satisfied source report.

Reports and typed error causes remain independently useful. Normative violation
can match `ErrInvalidDocument`; incomplete required assessment retains
`ErrIncompleteAssessment` / `validation.IncompleteError`. Syntax, I/O,
cancellation, resource, conversion and dictionary causes remain inspectable with
`errors.Is` / `errors.As`. A proved violation and incomplete work may coexist;
diagnostic truncation cannot turn either into acceptance.

## Options and migration

Both loaders accept nil assessment options, an empty assessment value, or the
explicit `ProfileSourceSemantics`. They always select `ProfileModelSemantics`
for the model stage; it is not caller-replaceable. Any other profile, including
`ProfileSourceXSDPreview`, `ProfileSourceLocal`, `ProfileSourceTimeline`,
`ProfileSourceContext`, narrower internal-DTD/dictionary profiles, model/package
profiles and unknown IDs, fails configuration before I/O.

For mandatory assessed loading, the zero-value options are sufficient:

```go
result, err := musicxml.DecodeAssessed(ctx, reader, musicxml.DecodeAssessmentOptions{})
if err != nil {
    // result.Source, result.Conversion and result.Model retain available evidence.
    return err
}
document := result.Document
```

The previous nil-assessment request for permissive conversion becomes ordinary
`Decode(reader)`, or `DecodeWithOptions` when preserving explicit transport limits:

```go
document, err := musicxml.DecodeWithOptions(reader, decodeOptions)
```

To retain a narrower source-only assessment, use `InspectXML` with its explicit
profile and inspect `report.Satisfied()`. Its ordinary normative violations still
return a report without an operation error. A separate conversion needs its own
reader; do not reuse a consumed reader. Narrow inspection plus conversion does
not provide the mandatory loader's source-and-model acceptance guarantee.

Finite overrides remain available without selecting a different profile:

```go
options := musicxml.DecodeAssessmentOptions{
    Decode: decodeOptions,
    Assessment: &musicxml.AssessmentOptions{
        Budgets: budgets,
    },
}
result, err := musicxml.DecodeAssessed(ctx, reader, options)
```

`DecodeOptions` is unchanged. Existing depth/security limits still apply alongside
assessment limits. Invalid limits, budgets and contexts fail preflight. Adding
`Model validation.Report` to `DecodeResult` requires external unkeyed struct
literals to become keyed literals.

## Required dictionary evidence

Default assessed loading uses `BundledDictionarySource()` with these exact
selections (the constants are in `validation`):

| Manifest | Required relation |
| --- | --- |
| `DictionaryManifestSMuFL14Canonical` (`smufl-1.4-canonical-v1`) | `DictionaryRelationCanonicalName` (`canonical-name`) |
| `DictionaryManifestIPARepresentation2015Unicode13` (`ipa-2015-unicode13-representation-v1`) | `DictionaryRelationIPARepresentation` (`ipa-symbol-representation`) |

The IPA selection is the current representation manifest, not the legacy partial
positive manifest. Bundling supplies pinned evidence; it does not turn an
unproved IPA input into a negative judgment or establish universal IPA coverage.
Standard-sound recognition and dictionary-backed rendering advice are not added
to the score semantic profiles.

`DecodeAssessedWithDictionaries` uses the same defaults when `Assessment` is nil
or its `Dictionaries.Source` is nil and it has no selections (nil and empty slices
both qualify). A typed-nil `Source` is an explicit override and does not activate
defaults. Any explicit override must include both required manifest/relation
pairs. Other supported selections may be added; unknown, duplicate or incomplete
selections fail before input reads or callbacks.
A custom source with empty selections is an incomplete configuration.

For an explicit source, provide the required selections together:

```go
options := musicxml.DecodeDictionaryAssessmentOptions{
    Assessment: &musicxml.DictionaryAssessmentOptions{
        Dictionaries: musicxml.DictionaryOptions{
            Source: dictionarySource,
            Selections: []validation.DictionarySelection{
                {
                    Manifest: validation.DictionaryManifestSMuFL14Canonical,
                    Relations: []validation.DictionaryRelation{
                        validation.DictionaryRelationCanonicalName,
                    },
                },
                {
                    Manifest: validation.DictionaryManifestIPARepresentation2015Unicode13,
                    Relations: []validation.DictionaryRelation{
                        validation.DictionaryRelationIPARepresentation,
                    },
                },
            },
        },
    },
}
result, err := musicxml.DecodeAssessedWithDictionaries(ctx, reader, options)
```

The explicit source is honored, including a nil, typed-nil or unavailable source
with explicit selections. No failure triggers bundled fallback, and no component
outside the explicit selections is implicitly requested. Missing evidence remains
unavailable wherever selected checks require it; import or close errors withhold
the document even if unrelated report obligations are satisfied. The caller's
input reader is never closed. Every usable dictionary body transferred to the
importer is closed under the existing ownership contract.

Bundled data uses committed local assets and the existing bounded verifier and
importer. It needs no runtime files or network access. Using the bundled source,
including through these defaults, makes all five embedded components reachable
regardless of selections. Binary distributions containing those data must include
the five notice/license files in
[the bundled-data distribution instructions](../../THIRD_PARTY_NOTICES.md#embedded-copies-of-the-pinned-dictionary-data).
The code's MIT license does not replace those data terms.

## Budgets and capacity

One finite account covers dictionary import, source assessment and model
assessment. The single imported dictionary set remains available across both
stages. Retained source-report context stays accounted for while the model stage
runs; the model stage does not receive a fresh allowance. The source graph and
model candidate make single-pass input different from constant-memory processing.
No default resource limit is raised. Previous source-only budget receipts do not
promise capacity for the combined pipeline.

### Low-level retained-report ownership

`validation.Session.FinishWithRetainedReport` finalizes the session and transfers
the already admitted `Report.Context` projection to an optional `ReportLease`,
without copying it or making a resource snapshot. No context means no lease is
needed. The loaders keep this source lease through direct model assessment; it is
a cross-package accounting seam, not another user option or a changed snapshot
API. If its bounded handle cannot be admitted, the report is incomplete and
dependent assessment cannot continue.

Low-level callers must close a nonnil lease even when finalization returns an
error. Do not copy the lease or use it concurrently. `ReportLease.Close` is safe
for nil/already-closed leases; it releases live accounting without invalidating
the returned report or refunding cumulative quotas. Caller mutations and extra
report copies remain outside this lease's admitted storage contract. See the
[ownership API](../../API.md#retained-report-ownership).

### Official-score capacity control

The unchanged original `testdata/official/MozaChloSample.musicxml` is 223,068 bytes
of UTF-16BE input, with SHA-256
`4323af5d7daa23ee3ae4fc27ff51274e98055a775824c49d3f4312f7586bc23e`.
Mandatory default loading reaches the 64 MiB (67,108,864-byte) buffered-byte
ceiling before source EOF. It retains `SyntaxIncomplete`, `InputComplete=false`
and incomplete conversion, with `Model.Requested=false` and no document. The
error retains both resource-limit and incomplete-assessment recognition. This is
a capacity change from the earlier narrower source-only loading contract, not a
MusicXML violation or a malformed-input claim.

The [migration control](../../source_dtd_no_fetch_review_test.go) keeps this
default-budget refusal distinct from a verified configured-capacity control.
With only `MaxBufferedBytes: 96 << 20` changed, the same original fixture completes
source input and conversion, but returns `ErrIncompleteAssessment` for unread
external-DTD evidence. It returns neither a document nor a requested model report;
this control has no resource-limit or invalid-document error. It is exercised by
`TestSourceInternalDTDOfficialDOCTYPEsRemainIncomplete` and
`TestSourceAssessmentAgainstSchema` for the exact MozaChlo fixture.

The 96 MiB (100,663,296-byte) allowance is fixture-specific, not a new default, a
measured minimum, a production recommendation or evidence that the external DTD
was read. Earlier standalone source-semantics observations remain in the
[historical capacity receipt](resource-fragment-budget-evidence.md#official-utf-16be-sample-and-standalone-composition).
Logical live accounting is not heap usage or a general input-byte capacity
guarantee; the original bytes, BOM and DOCTYPE are preserved in both controls.

## Authority and acceptance boundaries

These are the existing bounded semantic profiles. Required `Unknown`, unsupported
or otherwise incomplete evidence remains an error and withholds the document.
External DTD subsets and entities are never fetched or assumed harmless. Bounded
processing of actually read internal declarations does not add general DTD
validation. Source and current-model evidence remain distinct.

Successful loading establishes only the selected intrinsic source and model
contracts. It does not establish universal MusicXML conformance, arbitrary
repeat/playback/layout behavior, author intent, linked-resource validity or MXL
package acceptance. Use the separate explicit resource/package APIs for their
bounded obligations. See the
[semantic composition scope](semantic-composition-scope.md) and
[public API](../../API.md#mandatory-assessed-loading).

Historical profile-identity tables, design proposals, measurements and execution
receipts remain evidence for their stated revisions. They do not verify this
later loading migration; verification must use the changed tree.
