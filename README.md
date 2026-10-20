# go-musicxml

[![CI](https://github.com/go-muse/go-musicxml/actions/workflows/ci.yml/badge.svg)](https://github.com/go-muse/go-musicxml/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-muse/go-musicxml.svg)](https://pkg.go.dev/github.com/go-muse/go-musicxml)
[![GitHub Release](https://img.shields.io/github/v/release/go-muse/go-musicxml)](https://github.com/go-muse/go-musicxml/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`go-musicxml` is a typed, pure-Go implementation of the MusicXML 4.0 data
model. It reads, writes, validates, and constructs partwise scores, timewise
scores, opus documents, and compressed `.mxl` packages.

The runtime package uses only the Go standard library. Its generated types and
validation rules come from the bundled official MusicXML 4.0 XSD files.

## Caution

This project was developed with substantial AI assistance. It is pre-1.0, so
evaluate it against your own MusicXML files before production use. The test
suite includes round trips, independent XSD conformance checks, fuzzing, and
race detection.

## Requirements

- Go 1.27 or later
- MusicXML 4.0 input for schema validation

## Installation

```bash
go get github.com/go-muse/go-musicxml@v0.1.0
```

## Quick start

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/go-muse/go-musicxml"
)

func main() {
	input, err := os.Open("score.musicxml")
	if err != nil {
		log.Fatal(err)
	}
	defer input.Close()

	score, err := musicxml.DecodeScorePartwise(input)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(
		"MusicXML %s: %d parts\n",
		score.EffectiveVersion(),
		len(score.Part),
	)

	score.MovementTitle = musicxml.Ptr("Edited with go-musicxml")

	if err := score.Validate(); err != nil {
		log.Fatal(err)
	}

	output, err := os.Create("edited.musicxml")
	if err != nil {
		log.Fatal(err)
	}
	defer output.Close()

	if err := musicxml.Encode(output, score); err != nil {
		log.Fatal(err)
	}
}
```

This example decodes a MusicXML document, accesses and edits its typed model,
validates the result, and writes it back. Runnable examples also cover score
construction and compressed `.mxl` round trips in
[`example_test.go`](example_test.go).

## Main API

| Task | API |
| --- | --- |
| Read XML | `Decode`, typed `Decode...` helpers, and their `WithOptions` variants |
| Write XML | `Encode` |
| Read or write compressed MusicXML | `DecodeMXL`, typed `DecodeMXL...` helpers, `As...` accessors, and `EncodeMXL` |
| Preserve MXL resources | `DecodeMXLPackage`, `DecodeMXLPackageWithOptions`, `EncodeMXLPackage` |
| Validate current model against MusicXML 4.0 XSD | `Validate`, root `Validate` methods |
| Load only a valid structure (scenario 2: validation lines 2 and 3) | `DecodeAssessed`, `DecodeMXLAssessed`; `Lenient` returns the document with its reports |
| Check a current model with the semantic profile (scenario 3: line 3) | `AssessModel` with empty options; `AssessMXLPackage` with empty options for the current state of a whole MXL package |
| Find the validation line of a rule | `RuleLine`, `AllowedLineOverlap`, `ValidationLine` |
| Assess current model with an explicit profile/report | `AssessModel`, `ProfileModelXSD`, `ProfileModelLocal`, `ProfileModelTimeline`, `ProfileModelContext` |
| Inspect original source | `InspectXML` (empty profile: `ProfileSourceSemantics`), `ProfileSourceXSDPreview`, `ProfileSourceLocal`, `ProfileSourceTimeline`, `ProfileSourceContext` |
| Assess original source after bounded internal DTD processing | `ProfileSourceInternalDTD` |
| Compose DTD declarations with standalone local/link/dictionary assessment | `ProfileSourceInternalDTDLocal`, `ProfileSourceInternalDTDLinks`, `ProfileSourceInternalDTDDictionaries` |
| Assess local links without fetching | `ProfileSourceLinks`, `ProfileModelLinks` |
| Inspect or assess explicitly supplied pinned dictionaries | `SourceOptions.Dictionaries` or `ModelOptions.Dictionaries`, or `Graph.Dictionaries` of the resource and MXL options |
| Assess original auxiliary XML | `ProfileSourceContainer`, `ProfileSourceSounds` |
| Assess a selected resource graph | `InspectXMLWithResources`, `AssessModelWithResources`, `ProfileResourceGraph` |
| Assess original or current MXL packages | `InspectMXL`, `AssessMXLPackage`, `ProfileMXLSource`, `ProfileMXLModel` |
| Construct scores and opus documents | `NewScorePartwise`, `NewScoreTimewise`, `NewOpusDocument` |
| Construct common notes | `NewPitchedNote`, `NewRestNote` |
| Resolve linked opus documents | `MXLPackage.ResolveOpus` |
| Persist resolved-opus edits | `MXLPackage.SyncResolvedOpus` |

The full generated data model is available through the package documentation.
See [`API.md`](API.md) for invariants and compatibility rules.

## Transport and validation

`Decode` and `Encode` are transport operations. They do not validate
automatically, so callers can inspect, repair, and re-encode incomplete
documents. Call `Validate` explicitly for bounded XSD-only assessment of the
current model. It now walks Go fields directly without encoding or parsing XML;
its successful result makes no claim about the original input.
Encoding rejects invalid UTF-8 and XML 1.0 characters before writing, so text
is never silently replaced with U+FFFD.

Three scenarios cover loading and checking, over three validation lines; see
[Scenarios](API.md#scenarios) and [Validation lines](API.md#validation-lines).
`DecodeAssessed` and `DecodeMXLAssessed` run line 2, `ProfileSourceLine`
(`ProfileMXLSourceLine` for MXL), while they read: the requirements the loaded
structure cannot show. After conversion they run line 3, direct
`ProfileModelSemantics` (`ProfileMXLModelSemantics`) assessment. `AssessModel`
with empty options is that line-3 assessment on its own.
Nil or empty assessment options select this fixed pipeline, with bundled canonical
SMuFL and IPA representation evidence for line 3. `DecodeResult` exposes
`SourceLine`, `Conversion` and `ModelLine` separately; by default any failure or incomplete
required assessment withholds `Document`, while `Lenient` returns it with
`Accepted` false. A DOCTYPE external subset is read from the bundled official
MusicXML 4.0 DTDs, which also resolve the identifiers of MusicXML 1.0 through 3.1
as the official catalog does, or from a caller `DTDSource`; nothing is fetched.
An unrecognized external subset still cannot be accepted, and raising budgets does
not close that gap.
Use ordinary `Decode` for transport-only loading, without a validation guarantee.
See the [migration guide][assessed-loading] for compatibility, dictionary
import and budget costs and bundled-data distribution requirements.

XML decoding supports UTF-8 (with or without a BOM), UTF-16BE/LE (with a BOM,
or without one when the XML declaration names the byte order), and
ISO-8859-1. A UTF-16 BOM takes precedence over the encoding declaration.
MusicXML root elements are unqualified because the official
MusicXML 4.0 schema has no target namespace. The package decoders ignore
foreign-namespace child elements and attributes rather than treating matching
local names as MusicXML fields. Decoding rejects documents deeper
than 256 simultaneously open XML elements by default; `DecodeOptions` can set
a different ceiling up to the package maximum of 4096.

Optional XSD attributes remain pointers after decoding. Generated
`Effective...` methods expose schema defaults without changing omission
semantics, and `...MatchesFixed` methods check fixed attributes.

Ordered `Content` fields use generated slice types that discard unknown child
elements before appending them. This keeps the same invariant for both the
package decoders and direct `encoding/xml` use: every retained entry represents
exactly one recognized child.

### Explicit assessment profiles

`AssessModel(ctx, document, ModelOptions{Profile: ProfileModelLocal})`
checks the current model's XSD/representation contract and a bounded selection of
local musical clauses, including accordion register presence, beam-number
uniqueness and typed same-part references. Select `ProfileModelXSD` for the
XSD/representation scope without those local clauses. All three document roots
are supported; score-only clauses are inapplicable to opus. Linked content is
not assessed.

Select `ProfileModelTimeline` or `ProfileSourceTimeline` to add exact musical
positions, typed effective-state histories and the selected chord-duration and
numeral-key checks. Chord tones are bounded by their principal note; numeral
fallback uses an explicitly encoded key at the harmony's unshifted score origin.
The provider also retains bounded note/offset, grace, figured-bass, direction,
staff and instrument context. A completed provider does not imply every
ambiguous fact is known. The separate timeline engine leaves the existing
XSD/local profiles and legacy `Validate` unchanged. See the
[timeline scope and evidence](docs/validation/TIMELINE_ASSESSMENT_SCOPE.md).

Select `ProfileModelContext` or `ProfileSourceContext` for the bounded
repeat/span, local-default, layout and encoded-performance extension. Owned
`Report.Context` snapshots retain per-field availability and provenance; a
completed provider does not mean every optional fact is known. Selected
contextual predicates retain unresolved time-only duplicate ordering,
repeat-content and gesture requirements explicitly. Figure-number text retains
its deliberately open string domain without an additional numeric grammar gate. See the [context scope](docs/validation/context-assessment-scope.md).

The repository's official MusicXML 4.0 samples `MozartTrio.musicxml` (262 notes)
and `MozaChloSample.musicxml` (278 notes) do **not** complete either context
profile under the default budgets. Both stop at the 20,000,000-step limit;
raising work alone still reaches the default 64 MiB buffered-byte limit.
Explicit finite `MaxRuleSteps: 1 << 34` and `MaxBufferedBytes: 512 << 20`
complete the measured source/current-model cases. Source measurements remove
only the DOCTYPE; MozaChlo's UTF-16BE BOM and encoding declaration are
preserved. The unmodified DOCTYPE-bearing originals are now assessed with their
bundled external DTD; see [External DTD subsets](API.md#external-dtd-subsets).
See [matched resource evidence](docs/validation/evidence/context-round3-resource-acceptance.md)
for exact minima and outcomes. Exhaustion is incomplete assessment, not a
MusicXML violation or a readiness success.

Always inspect `Report.Satisfied()`: a nil error alone is not acceptance.
Ordinary violations live in the report; cancellation/resource interruptions
return their cause and a partial report. Model locations are field/index paths
without invented XML positions. Finite decimal fields are assessed as the
shortest decimal that reads back to the same `float64`, so `float64(0.1)` is
decimal `1/10`.
Optional nil and present-zero fields remain distinct; `XMLName` is ignored
transport metadata.

The separate developmental `ProfileSourceInternalDTD` processes bounded internal
declarations, entity replacements and effective attributes with typed occurrence
origins, including a bundled or caller-supplied external subset. External DTDs
are not fetched, and the other existing profiles do not opt into this processing. See its [scope and retained obligations](docs/validation/dtd-infoset-scope.md).

The explicit [standalone compositions](docs/validation/source-declarations-scope.md)
add declaration-dependent ENTITY/ENTITIES checks and reuse local, nonfetching
link and selected dictionary consumers. DTD defaults and normalized effective
values remain distinct from physical source occurrences. Dictionary components
remain explicitly selected; callers can use the bundled pinned source without
handling data files. These profiles do not themselves assess package obligations
or general DTD validity.
Explicit package compositions transport independently owned source origins.

The additive [DTD/dictionary package compositions](docs/validation/dtd-package-composition-scope.md)
can be selected explicitly through the existing dictionary graph and MXL APIs.
They assess retained source children with bounded internal declarations and
owned source origins, while current primary models keep their existing model
profile. Effective Container paths stay distinct from physical Raw; explicit
Sounds input retains effective mapping fields. No auxiliary filename discovery
or external DTD/schema/dictionary fetching is introduced.

The [encoded semantic compositions][semantic-compositions] are explicitly
selectable for inspection/assessment and fixed in the assessed loaders. They join
source/current-model XSD, local, timeline and contextual checks with bounded
barline/repeat proofs and registered XML Framework/shorthand/element fragments.
Matching graph and raw/current-MXL profiles pin those intrinsic interpretations.
Inspection and standalone model/package assessment keep explicit dictionary
selection; the assessed loaders use the fixed defaults described above. Missing
evidence and original package/indirection limits remain visible. These profiles
add no external musical intent provider and make no universal-conformance claim.

For original-input inspection, use `InspectXML` with an explicit
`ProfileSourceContext`, `ProfileSourceTimeline`, `ProfileSourceLocal` (XSD
plus the same local clauses), or the unchanged `ProfileSourceXSDPreview` (XSD
only). These narrower profiles are not accepted by either assessed loader.
Profiles that do not read DTD declarations still report any external DOCTYPE
subset as unread. Strict assessed decoding reads the bundled official DTD and
accepts `Chant.musicxml` with default budgets; `MozartTrio.musicxml` and
`MozaChloSample.musicxml` are accepted with larger explicit budgets. Of the 154
score and opus files in `testdata/official` and the MusicXML test suite, default
assessed loading accepted 140 when measured for this change. There is no DTD
fetching, stripping or ignore policy: the bundled DTDs are selected only by the
official public and system identifiers, and any other DTD comes from a caller
`DTDSource`.

These intrinsic profiles do not establish complete MusicXML conformance, full
repeat/playback/layout semantics, author intent, or package and linked-resource
validity. Separate additive graph/package profiles assess bounded selected
resource obligations as described below. See the [scope and evidence][model-scope]
and [API contract](API.md#direct-current-model-assessment). Inspection and
standalone model/package assessment still require explicit profiles; the fixed
assessed-loading pipeline is the exception. A runnable [direct-model example][model-example]
shows explicit profile selection and the model report axes.

All assessment profiles share finite resource budgets; input byte size alone
does not predict their cost. B5-PERF-01 remains an open shared validation
kernel/session performance obligation, with final reconciliation in batch 9;
timeline improvements do not close it. See the [API limits](API.md#exact-timeline-and-effective-state-profiles).

### Pinned dictionary evidence

The dictionary inspection and assessment entry points select explicit source,
source-sounds, model, resource-graph or MXL profiles. The assessed loaders instead
require canonical SMuFL and current IPA representation relations by default;
explicit overrides must include both and never fall back to bundled data after
an unavailable source or import failure. Standalone XML inspection accepts sounds
input; public graph/MXL wrappers do not discover or assess sounds children.
Callers select fixed components and use
[`BundledDictionarySource()`](bundled_dictionary_example_test.go) for the embedded
pinned data, or supply their own `DictionarySource`. No data files or
network access are needed for the bundled source. Explicit offline `go generate`
prepares committed assets; ordinary builds embed those local assets without
running generation. Selection and the existing bounded importer remain explicit.
One verified immutable set and finite account cover the root and selected
linked/nested-package children.
Catalogs without the IPA dictionary operation remain unchanged; its additive
policy deliberately updates dictionary-containing catalog fingerprints.

The selected scope includes exact SMuFL 1.4 canonical membership with independent
MusicXML name-family checks, narrow defaults/range advice and recognition of the
894 MusicXML 4.0 standard sound IDs. Custom sound strings remain allowed. IPA
2015/Unicode 13 positive-v1 data remain explicitly partial. A separately selected
representation-v1 manifest can establish an affirmative whole-input repertoire
witness with stable canonical ordering and bounded, source-established half
ties. Unproved input stays unavailable; no phonotactics, pronunciation or
universal representation conformance is claimed. The old 197-row manifest is
unchanged. See the [representation policy and migration](docs/validation/ipa-representation.md).
Internal-DTD composition requires its separate profile; general glyph/rendering
semantics remain outside this scope.

Default budgets can still stop modest inputs. In a specific repeated-note fixture
with all four dictionary components imported, standalone source/model assessment
completes at 368 notes and exhausts the unchanged 20,000,000-step limit at 369;
graph and raw/current MXL complete operational enumeration at 363 and refuse
364. When this was measured, current-model bundles also kept a schema-hint
unknown, which revision 2 of the pairing rule removed. These are fixture-specific observations, not general capacities or
B5-PERF-01 closure. See the [measured conditions][dictionary-budget-evidence].

[dictionary-budget-evidence]: docs/validation/pinned-dictionaries-evidence.md#default-budget-observations

See the [API and migration contract](API.md#opt-in-pinned-dictionaries),
[exact obligations and retained gates](docs/validation/pinned-dictionaries-scope.md)
and [dated verification checkpoint](docs/validation/pinned-dictionaries-evidence.md).
The new optional `ContextResult.Dictionary` field requires external unkeyed
literals to become keyed literals. Dictionary data retain separate FSA, IPA and
Unicode terms in [third-party notices](THIRD_PARTY_NOTICES.md); the MIT code license
does not replace them.

## Bounded packages and resource graphs

Use `InspectXMLWithResources` or `AssessModelWithResources` with explicit
`ProfileResourceGraph`, or `InspectMXL` / `AssessMXLPackage` with their source /
current MXL profiles. The returned bundle preserves a package report and separate
canonical intrinsic reports. Check both the error and `bundle.Satisfied()`.
Original headers/container bytes and current package fields have different,
explicit scopes; current models are never serialized to manufacture source proof.
A current model is assessed as it would be encoded: it carries no xsi schema
hints, so schema-hint pairing does not apply to it. An unedited package decoded
from an archive that `InspectMXL` accepts satisfies `AssessMXLPackage`, unless
conversion lost a fact the source supplied, such as an internal-DTD attribute
default. Original hints are not reconstructed; `InspectMXL` reports them.

External retrieval and image/association/role metadata require an explicitly
selected dependency and caller-provided resolver. Empty selections perform no
external I/O. The resolver owns transport authorization; no built-in network,
filesystem, schema-hint, DTD or entity fetching is introduced. One finite account
covers the complete graph, including aliases and nested packages; legacy MXL byte
caps also apply. Unavailable providers, incomplete inventories and unsupported
ZIP/media/indirection interpretations remain explicit gaps. See the [package scope](docs/validation/package-profile-scope.md)
and [graph API](API.md#selected-resource-graph-entry-points) for exact contracts.
Runnable [source graph and MXL examples](resource_assessment_example_test.go)
show explicit profile selection and bundle acceptance checks.

Package-role replies apply to a sealed, exact member scope; unread entries remain
unknown even if the selected provider table is complete. Optional file metadata
may survive a parse or admission error, but cannot certify an XML target, role,
ID or graph closure. Original non-UTF8 entry bytes may retain encoding advice
without becoming safe member paths. Primary retrieval-name advice uses a supplied
URI rather than the canonical cache identity or reconstructed model history.

Default budgets are finite. Shared input, external bytes and working storage each
default to 64 MiB, with 20,000,000 rule steps and depth 1024; the separate MXL byte
caps do not raise them. A satisfied bundle covers its named bounded profile,
not full ZIP/JAR/DEFLATE, media-type or compressed-rootfile interpretation closure.

## Compressed MXL and opus

`DecodeMXLPackage` retains alternate root files, images, PDFs, linked scores,
and other resources. Unchanged linked XML resources remain byte-for-byte
unchanged after `ResolveOpus` and `SyncResolvedOpus`; edited documents are
re-encoded as UTF-8 MusicXML.

MXL decoding rejects unsafe paths, duplicate entries, invalid container
metadata, and oversized archives. The default limits restrict compressed input
to 64 MiB, metadata to 1 MiB, the primary document and each resource to
256 MiB, and all resources together to 512 MiB. `MXLOptions` exposes every byte
limit and the XML nesting limit for callers with a stricter threat model.

When the expected archive root is known, use `DecodeMXLScorePartwise`,
`DecodeMXLScoreTimewise`, or `DecodeMXLOpusDocument` and their `WithOptions`
variants. Polymorphic `Document` values can be inspected without a panicking
type assertion through `AsScorePartwise`, `AsScoreTimewise`, and
`AsOpusDocument`.

## Reliability

- The interoperability test corpus contains 150 real MusicXML Test Suite
  files, including XML in multiple encodings and compressed `.mxl`.
- Every decodable fixture is decoded, validated when expected to be valid,
  encoded, decoded again, and compared as a typed Go model.
- A second encoding must match the first byte-for-byte, proving that XML and
  MXL serialization reach a deterministic fixed point.
- A dedicated Linux CI job validates every re-encoded, originally valid corpus
  document with `xmllint` against the bundled MusicXML 4.0 XSD without network
  access.
- CI runs tests, module and generation reproducibility checks, and formatting
  checks on Linux, macOS, and Windows, plus the race detector and ten counted fuzz
  targets.

## Limitations

- The generated model and schema validator target MusicXML 4.0.
- Numeric values use the generated Go numeric types. Decimal values have
  `float64` precision, and integer values must fit their Go representation;
  arbitrary-precision XSD values are not preserved.
- `ResolveOpus` decodes linked resources with the default XML depth limit of
  256. `MXLOptions.MaxXMLDepth` applies to the primary document, not later
  linked-resource resolution.
- XML extensions not represented by the MusicXML 4.0 model are ignored during
  decoding and are not preserved when re-encoded.
- Encoding and legacy `Validate` reject cyclic opus models and models deeper
  than 4096 nested elements. `AssessModel` reports model/resource outcomes with
  configurable finite budgets and a default depth of 1024.
- Encoding normalizes lexical details such as whitespace, attribute order,
  character encoding, and ZIP metadata; the first output need not match the
  original bytes.
- The API is pre-1.0 and may change between minor releases.

## Validation design

See [`docs/validation`](docs/validation/README.md) for the proposed validation
architecture, MusicXML 4.0 requirements catalog, source coverage, and open
questions. The research data lives separately in the
[pinned registry snapshot](https://github.com/go-muse/go-musicxml-registry/tree/3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c).
The broader proposals remain future work. The foundation, source preview and
bounded direct-model/local implementation have separate scope/evidence ledgers;
research counts and planned test contracts are not executed coverage.

## Development

```bash
make check
make check-all
```

`make check` runs formatting, tests, and `go vet`. `make check-all` also
checks module tidiness and generated files, runs the race detector, and
performs short fuzzing passes. Both work on a worktree with uncommitted
changes. When `xmllint` is available, the test suite also runs its external
XSD conformance pass; Linux CI installs and requires it. CI and the release
workflow run the same checks on Linux, macOS, and Windows; only the release
workflow verifies the tag, the changelog entry, and the installation command
in this README.

## License

The library code is licensed under the [MIT License](LICENSE). Bundled schemas,
test fixtures and runtime dictionary data retain their own terms. Any binary
referencing `DecodeAssessed`, `DecodeMXLAssessed`, `InspectXML`, `AssessModel`,
`InspectXMLWithResources`, `AssessModelWithResources`, `InspectMXL` or
`AssessMXLPackage` embeds all five
dictionary components, even with custom-source options. Direct use of
`BundledDictionarySource` also retains all five. These include W3C FSA, CC BY-SA
4.0 and Unicode License v3 data. Distribute these binaries with the five
notice/license files listed in [`THIRD_PARTY_NOTICES.md`][bundled-data-notices].

[bundled-data-notices]: THIRD_PARTY_NOTICES.md#embedded-copies-of-the-pinned-dictionary-data

[assessed-loading]: docs/validation/assessed-loading-pipeline.md
[semantic-compositions]: docs/validation/semantic-composition-scope.md
[model-scope]: docs/validation/model-assessment-scope.md
[model-example]: model_assessment_example_test.go
