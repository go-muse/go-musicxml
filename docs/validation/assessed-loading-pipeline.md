# Mandatory assessed-loading pipeline

Strict loading is scenario 2 of the [scenarios](../../API.md#scenarios): it
returns a valid MusicXML 4.0 structure or an error, over three
[validation lines](../../API.md#validation-lines). Line 1 is the XML decoder.
Line 2, `ProfileSourceLine`, runs while `DecodeAssessed` reads the document and
checks what the converted structure cannot show: XSD on the original bytes and
their DTD infoset, source provenance, schema hints and the numeric values the
Go model cannot hold. Line 3, direct `ProfileModelSemantics`, checks the
converted structure. `DecodeMXLAssessed` runs line 2 on the original archive
with `ProfileMXLSourceLine` (archive bytes and schema-hint pairing, rootfile
documents with `ProfileSourceLine`, no resource links) and then line 3 on the
current state of the whole converted package with `ProfileMXLModelSemantics`,
as `AssessMXLPackage` does. Lines 2 and 3 do not check the same thing, except
for the overlap `AllowedLineOverlap` lists. Until the lines were split, both
loaders ran the full `ProfileSourceSemantics` (`ProfileMXLSourceSemantics`),
which repeated 62 of the 64 rules of the model stage. The former
`DecodeAssessedWithDictionaries` was removed; its dictionary options are
`AssessmentOptions.Dictionaries`.
This breaks the earlier unreleased development API; these entry points did not
exist in v0.1.0. It does not widen the XML loader's profiles or change their
catalog digests, and it does not establish complete MusicXML conformance. The
MXL package profiles changed identity later, with revision 2 of the schema-hint
pairing rule.

Ordinary `Decode`, typed decode helpers and `DecodeWithOptions` remain transport
operations. `InspectXML`, `AssessModel` and the resource/MXL inspection and
assessment APIs retain their explicit-profile and data-selection contracts; an
empty profile selects their semantic profile (see
[Scenarios](../../API.md#scenarios)). `DecodeMXLAssessed` is the assessed
package decoder.

## Loading and report contract

The loaders retain one source/parser owner. Original source observation precedes
model namespace filtering, attribute projection and unknown-element skipping;
conversion uses that same stream. A recoverable mapping failure drains the same
healthy parser without resuming the failed reflection decoder. There is no reader
replay, second XML parse, or encoding the Go model back into XML for assessment.

The acceptance sequence is:

1. Validate context, profile, decode limits, budgets and dictionary configuration
   before reading input or calling a dictionary source.
2. Import one dictionary set for line 3, then run line 2, `ProfileSourceLine`,
   on the original source while converting through the existing mapping path.
   Line 2 reads no dictionaries. Source EOF, line-2 acceptance, successful
   conversion and no earlier stream error are required to advance.
3. Run line 3, `ProfileModelSemantics`, on the actual converted Go object. The
   walker uses current field/index facts directly; it does not invent source
   bytes, lexical values, namespace declarations or discarded attributes. A
   dictionary import that failed but returned a set does not stop line 3, so
   its report shows the unavailable evidence.
4. Return the document only if both reports are satisfied, conversion succeeded
   and no operation, import or close error occurred.

`DecodeResult` separates four outputs:

- `SourceLine`: the line-2 report of the original source, including the best
  partial report on interruption when available.
- `Conversion`: mapping state and bounded findings. `ConversionConverted` is not
  an exact lexical or numeric round-trip guarantee.
- `ModelLine`: the independent line-3 report of the converted model, when
  initialized. `ModelLine.Requested` can be false after an earlier failure or a failure to start
  the model session, including cancellation after satisfied source assessment and
  successful conversion. It cannot distinguish those cases and is never a pass.
  Once a model report exists, it is retained even if model assessment fails.
- `Document`: nonnil only on complete successful loading. It is nil for source or
  model violation, incomplete required assessment, conversion failure, operation
  failure or close failure.

A source value the model cannot hold unchanged, such as an exact positive
decimal that would underflow to zero in a `float64` field, is rejected in the
source by the representability rule (`MX40-SOURCE-REPRESENTABLE-*`), so the
model stage sees the values the source assessed. The model stage remains the
assessment of the converted structure, the same as `AssessModel`. Model
locations remain field/index locations; a model-stage failure does not rewrite
the satisfied source report.

Reports and typed error causes remain independently useful. Normative violation
can match `ErrInvalidDocument`; incomplete required assessment retains
`ErrIncompleteAssessment` / `validation.IncompleteError`. Syntax, I/O,
cancellation, resource, conversion and dictionary causes remain inspectable with
`errors.Is` / `errors.As`. A proved violation and incomplete work may coexist;
diagnostic truncation cannot turn either into acceptance.

## Options and migration

`DecodeAssessed` accepts nil assessment options, an empty assessment value, or
the explicit `ProfileSourceLine`, and always selects `ProfileModelSemantics`
for line 3. `DecodeMXLAssessed` accepts an empty `Graph.Profile` or
`ProfileMXLSourceLine`, and always selects `ProfileMXLModelSemantics` for
line 3. The former `ProfileSourceSemantics` and `ProfileMXLSourceSemantics`
options are configuration errors: they remain diagnostic profiles of
`InspectXML` and `InspectMXL`. Neither model profile is
caller-replaceable. Any other profile, including
`ProfileSourceXSDPreview`, `ProfileSourceLocal`, `ProfileSourceTimeline`,
`ProfileSourceContext`, narrower internal-DTD/dictionary profiles, model/package
profiles and unknown IDs, fails configuration before I/O.

**External DOCTYPE subsets:** both assessed loaders read a DOCTYPE external
subset from the bundled official MusicXML 4.0 DTDs, selected by the official
catalog's public identifiers (MusicXML 1.0 through 4.0) or official system
identifiers, or from a caller `DTDSource` (`DecodeOptions.DTD`). No subset is
fetched. An unrecognized external subset still cannot be accepted, including
with nil or empty assessment options, and raising budgets does not remove that
authority limit. See [External DTD subsets](../../API.md#external-dtd-subsets).

With the bundled DTDs, default assessed loading accepted 140 of the 154 score and
opus files in `testdata/official` and the MusicXML test suite when measured for
this change, including the 148 test-suite files' common external DOCTYPE;
`TestAssessedLoadingCorpusMeasurement` keeps that floor. The remaining files
report genuine violations, unestablished semantic context or resource limits,
not unread-DTD evidence. The history below records the state before the bundled DTDs.

In the [PR 46 corpus review][review-corpus] of head `7e7126e9405e0731e6cefa713e801515b4874703`,
the reviewer reported that default loading changed from 153 of 157 repository
fixtures on the prior main branch to only the two DOCTYPE-free official opus
fixtures. The 148 test-suite XML files and `Chant.musicxml` were incomplete for
unread external-DTD evidence; the two larger official scores also reached a
resource limit. This is a reviewer-reported compatibility observation, not an
independent corpus run or a universal-conformance result.

Zero-value options configure the required pipeline; they do not guarantee that
a particular input is within its evidence or resource limits:

```go
result, err := musicxml.DecodeAssessed(ctx, reader, musicxml.DecodeAssessmentOptions{})
if err != nil {
    // result.SourceLine, result.Conversion and result.ModelLine retain available evidence.
    return err
}
document := result.Document
```

Callers who relied on nil assessment to load external-DOCTYPE files must use
ordinary `Decode(reader)` or `DecodeWithOptions` if they want the previous
transport/conversion behavior. These APIs do not validate the source and are not
a substitute for successful assessed loading. They retain XML/security checks,
but provide no source-and-model conformance guarantee:

```go
document, err := musicxml.DecodeWithOptions(reader, decodeOptions)
```

To retain a narrower source-only assessment, use `InspectXML` with its explicit
profile and inspect `report.Satisfied()`. For dictionary-dependent profiles, use
`InspectXML` with explicit `Dictionaries.Source` and `Selections`; with an
explicit profile and nil `Dictionaries`, `InspectXML` supplies no dictionary
evidence. This includes the source
dictionary, internal-DTD dictionary and source-sounds dictionary profiles.
Ordinary normative violations still return a report without an operation error.
A separate conversion needs its own reader; do not reuse a consumed reader.
Narrow inspection plus conversion does not provide the mandatory loader's
source-and-model acceptance guarantee or close unread external-DTD evidence.

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
assessment limits. Invalid limits, budgets and contexts fail preflight. The
`DecodeResult` fields are `SourceLine` and `ModelLine`, named after the
validation lines; the earlier `Source` and `Model` fields were renamed.

## Required dictionary evidence

Only validation line 3 reads dictionaries. Default assessed loading uses
`BundledDictionarySource()` with these exact selections (the constants are in
`validation`):

| Manifest | Required relation |
| --- | --- |
| `DictionaryManifestSMuFL14Canonical` (`smufl-1.4-canonical-v1`) | `DictionaryRelationCanonicalName` (`canonical-name`) |
| `DictionaryManifestIPARepresentation2015Unicode13` (`ipa-2015-unicode13-representation-v1`) | `DictionaryRelationIPARepresentation` (`ipa-symbol-representation`) |

The IPA selection is the current representation manifest, not the legacy partial
positive manifest. A previous positive-IPA-only override must add the required
representation manifest/relation, along with canonical SMuFL; positive evidence
does not satisfy the representation selection requirement. Bundling supplies
pinned evidence; it does not turn an unproved IPA input into a negative judgment
or establish universal IPA coverage.
Standard-sound recognition and dictionary-backed rendering advice are not added
to the score semantic profiles.

`DecodeAssessed` uses the same defaults when `Assessment` or
`Assessment.Dictionaries` is nil, or when the dictionaries' `Source` is nil and
they have no selections (nil and empty slices both qualify). A typed-nil
`Source` is an explicit override and does not activate
defaults. Any explicit override must include both required manifest/relation
pairs. Other supported selections may be added; unknown, duplicate or incomplete
selections fail before input reads or callbacks.
A custom source with empty selections is an incomplete configuration.

For an explicit source, provide the required selections together:

```go
options := musicxml.DecodeAssessmentOptions{
    Assessment: &musicxml.AssessmentOptions{
        Dictionaries: &musicxml.DictionaryOptions{
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
result, err := musicxml.DecodeAssessed(ctx, reader, options)
```

The explicit source is honored, including a nil, typed-nil or unavailable source
with explicit selections. No failure triggers bundled fallback, and no component
outside the explicit selections is implicitly requested. Missing evidence remains
unavailable wherever selected checks require it; import or close errors withhold
the document even if unrelated report obligations are satisfied. The caller's
input reader is never closed. Every usable dictionary body transferred to the
importer is closed under the existing ownership contract.

Bundled data uses committed local assets and the existing bounded verifier and
importer. It needs no runtime files or network access. Any binary referencing
`DecodeAssessed` or another entry point listed under [Scenarios](../../API.md#scenarios)
embeds all five components,
even when every call supplies a custom source or different selections. Direct
use of `BundledDictionarySource` also retains all five. Options control runtime
import and assessment; they do not remove the embedded assets. Distribute these
binaries with the five notice/license files in the
[bundled-data distribution instructions][bundled-notices]. They include CC BY-SA
4.0 IPA-derived data and Unicode License v3 notices; the code's MIT license does
not replace those data terms.

## Budgets and capacity

One finite account covers dictionary import and both validation lines.
`DecodeAssessed` imports one dictionary set, before it reads the input, and
passes it to line 3 only; `DecodeMXLAssessed` imports it once, when line 3
starts, because the line-3 graph owns its set. Retained source-report context stays accounted for while the model stage
runs; the model stage does not receive a fresh allowance. The source graph and
model candidate make single-pass input different from constant-memory processing.
No default resource limit is raised. Previous source-only budget receipts do not
promise capacity for the combined pipeline.

Splitting the lines removed the 62 rules the source stage used to repeat.
Minimal budgets of `DecodeAssessed`, found by binary search through the public
API on darwin/arm64:

| File | Before the split | After the split |
| --- | --- | --- |
| `01a-Pitches-Pitches.xml`, 31 KB | 21.69 million rule steps, 53.2 MiB buffer | 13.68 million, 34.2 MiB |
| `41c-StaffGroups.xml`, 42 KB | 24.90 million, 72.4 MiB | 15.59 million, 46.5 MiB |

Both now load within the default budgets (20 million rule steps, 64 MiB);
`TestStrictLoadingFitsDefaultBudgets` keeps that. These are logical account
figures for these files, not heap measurements or general capacity.

### Per-call import cost and pre-input budget floor

Each invocation imports, verifies and parses a fresh account-bound dictionary set
before reading the document. The default path imports canonical SMuFL and the
IPA representation bundle even if the document has no dictionary-dependent
content. The set is reused by the two stages of that invocation, then closed;
there is no cross-call verified-set cache. Embedded bytes are shared, but parsed
sets and their verification work are not. Repeated loading of small documents
therefore pays this fixed cost each time.

The [PR 46 performance review][review-import] reported the following for reviewed
head `7e7126e9405e0731e6cefa713e801515b4874703`:

| Reviewer probe | Reported per-call result |
| --- | --- |
| Bundled import alone | About 3.08 ms, 1.55 MB allocated, 8,641 allocations |
| One-note default load, prior main | 14,935 ns, about 20 KB allocated |
| One-note default load, reviewed head | 6,393,642 ns, about 8.6 MB allocated |

These are reviewer-reported timing/allocation observations, not independently
reproduced benchmarks, latency guarantees or portable capacities. The cited
comment does not specify a complete benchmark environment. The old default did
not perform these required assessments, so this is a compatibility-cost
comparison rather than equivalent validation work. No caching redesign is
introduced by this migration.

The [same review's logical-budget probe][review-budget] reported 3,279,308 rule
steps for one required import, about 16.4% of the default 20,000,000-step account.
The 301,683 canonical SMuFL bytes plus 58,745 IPA representation bytes also charge
360,428 `MaxExternalBytes` before the document is read. This existing import-byte
accounting applies to embedded data despite no network or filesystem fetch; the
charge is not a claim that an external service was contacted.

| Reviewer's one-note probe | Standalone source semantics | Mandatory loader |
| --- | ---: | ---: |
| Minimum `MaxRuleSteps` in that probe | 109,716 | 3,486,332 |
| Minimum configured `MaxExternalBytes` in that probe | 1 | 360,428 |
| Minimum `MaxBufferedBytes` in that probe | 675,607 | 1,372,253 |

These are reviewer-measured logical boundaries at the named head, not new defaults
or independently measured general minima. The standalone comparison does not
include the mandatory loader's import and direct-model stage. Previously
sufficient explicit budgets can now fail before any input read, leaving
`Source.Requested=false`, `Model.Requested=false` and no document. Recheck tuned
budgets for the entire operation; an import refusal is resource incompleteness,
not invalid MusicXML.

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
Before the validation lines were split, on the asserted linux/amd64 migration
control, mandatory default loading reached the 64 MiB (67,108,864-byte)
buffered-byte ceiling before source EOF. It retained `SyntaxIncomplete`,
`InputComplete=false` and incomplete conversion, with the model unrequested and
no document. The error retains
both resource-limit and incomplete-assessment recognition. This is a capacity
change from the earlier narrower source-only loading contract, not a MusicXML
violation or a malformed-input claim. This exact refusal was asserted only on
amd64; the [reviewer also reproduced it on darwin/arm64 and
darwin/amd64][review-architecture]. The 386 CI run did not assert this
boundary, and size-based accounting makes it architecture-dependent.

The smaller original `testdata/official/MozartTrio.musicxml` is 94,916 bytes.
The [PR 46 reviewer][review-mozart] also observed a default 64 MiB refusal for
that fixture: 67,108,764 bytes live plus a 452-byte request exceeded 67,108,864.
In that observation source EOF and conversion were complete, but source
finalization retained a resource-limit terminal and the loader returned no
document. It is not a portable boundary. Input byte size alone cannot place the
capacity threshold above 200 KB.

With the bundled external DTD, both larger originals do their full source and
model work. Since the lines were split, line 2 accepts both within the default
budgets and line 3 reaches the default 20 million rule-step limit. The focused
regression
[TestSourceInternalDTDOfficialDOCTYPEsResolveBundledDTD][official-capacity-test]
keeps these amd64 default refusals, which return no document, and then accepts
both with `MaxBufferedBytes: 256 << 20` and `MaxRuleSteps: 100_000_000`. `Chant.musicxml` is accepted with default budgets.
`TestSourceAssessmentAgainstSchema` exercises the same controls. The 386 run
checks the configured acceptance, not the default frontiers.

The configured allowance is fixture-specific, not a new default, a measured
minimum or a production recommendation. Earlier standalone source-semantics
observations remain in the [historical capacity receipt][capacity-history].
Logical live accounting is not heap usage or a general input-byte capacity
guarantee; the original bytes, BOM and DOCTYPE are preserved in both controls.

## Authority and acceptance boundaries

These are the existing bounded semantic profiles. Required `Unknown`, unsupported
or otherwise incomplete evidence remains an error and withholds the document.
External DTD subsets and entities are never fetched. A subset is read only from
the bundled official MusicXML DTDs or a caller DTD source; bounded processing of
actually read declarations does not add general DTD validation. Source and current-model evidence remain distinct.

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

[review-corpus]: https://github.com/go-muse/go-musicxml/pull/46#discussion_r4302240891
[review-import]: https://github.com/go-muse/go-musicxml/pull/46#discussion_r4302240900
[review-budget]: https://github.com/go-muse/go-musicxml/pull/46#discussion_r4302240905
[review-architecture]: https://github.com/go-muse/go-musicxml/pull/46#discussion_r4302240930
[review-mozart]: https://github.com/go-muse/go-musicxml/pull/46#discussion_r4302240934
[bundled-notices]: ../../THIRD_PARTY_NOTICES.md#embedded-copies-of-the-pinned-dictionary-data
[capacity-history]: resource-fragment-budget-evidence.md#official-utf-16be-sample-and-standalone-composition

[official-capacity-test]: ../../source_dtd_no_fetch_review_test.go#L67
