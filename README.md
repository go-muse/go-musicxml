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
| Assess current model with explicit profile/report | `AssessModel`, `ProfileModelXSD`, `ProfileModelLocal`, `ProfileModelTimeline`, `ProfileModelContext` |
| Assess original source or strictly decode it | `InspectXML`, `DecodeAssessed`, `ProfileSourceXSDPreview`, `ProfileSourceLocal`, `ProfileSourceTimeline`, `ProfileSourceContext` |
| Assess local links without fetching | `ProfileSourceLinks`, `ProfileModelLinks` |
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

`AssessModel(ctx, document, AssessmentOptions{Profile: ProfileModelLocal})`
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
contextual predicates retain unresolved numeric, repeat-content and gesture
requirements explicitly. See the [context scope](docs/validation/context-assessment-scope.md).

The repository's official MusicXML 4.0 samples `MozartTrio.musicxml` (262 notes)
and `MozaChloSample.musicxml` (278 notes) do **not** complete either context
profile under the default budgets. Both stop at the 20,000,000-step limit;
raising work alone still reaches the default 64 MiB buffered-byte limit.
Explicit finite `MaxRuleSteps: 1 << 34` and `MaxBufferedBytes: 512 << 20`
complete the measured source/current-model cases. Source measurements remove
only the unsupported DOCTYPE; MozaChlo's UTF-16BE BOM and encoding declaration
are preserved. Unmodified DOCTYPE-bearing source remains unsupported by the
source assessment contract.
See [matched resource evidence](docs/validation/evidence/context-round3-resource-acceptance.md)
for exact minima and outcomes. Exhaustion is incomplete assessment, not a
MusicXML violation or a readiness success.

Always inspect `Report.Satisfied()`: a nil error alone is not acceptance.
Ordinary violations live in the report; cancellation/resource interruptions
return their cause and a partial report. Model locations are field/index paths
without invented XML positions. Finite decimal fields are assessed as their
exact current binary rational value, so `float64(0.1)` is not decimal `1/10`.
Optional nil and present-zero fields remain distinct; `XMLName` is ignored
transport metadata.

For original input, use `InspectXML` or strict `DecodeAssessed` with explicit
`ProfileSourceContext`, `ProfileSourceTimeline`, `ProfileSourceLocal` (XSD plus the same local clauses), or the unchanged
`ProfileSourceXSDPreview` (XSD only). Unread external DOCTYPE subsets still make
all source profiles incomplete, including `Chant.musicxml`, `MozartTrio.musicxml`
and `MozaChloSample.musicxml`. Strict assessed decoding returns no document for
those original files. There is no new DTD trust, stripping, fetching or ignore
policy. The two DOCTYPE-free official opus examples are a separate assessable
source case.

These intrinsic profiles do not establish complete MusicXML conformance, full
repeat/playback/layout semantics, author intent, or package and linked-resource
validity. Separate additive graph/package profiles assess bounded selected
resource obligations as described below. See the [scope and evidence](docs/validation/model-assessment-scope.md)
and [API contract](API.md#direct-current-model-assessment); no profile is selected
implicitly. A runnable [direct-model example](model_assessment_example_test.go)
shows explicit profile selection and the model report axes.

All assessment profiles share finite resource budgets; input byte size alone
does not predict their cost. B5-PERF-01 remains an open shared validation
kernel/session performance obligation, with final reconciliation in batch 9;
timeline improvements do not close it. See the [API limits](API.md#exact-timeline-and-effective-state-profiles).

## Bounded packages and resource graphs

Use `InspectXMLWithResources` or `AssessModelWithResources` with explicit
`ProfileResourceGraph`, or `InspectMXL` / `AssessMXLPackage` with their source /
current MXL profiles. The returned bundle preserves a package report and separate
canonical intrinsic reports. Check both the error and `bundle.Satisfied()`.
Original headers/container bytes and current package fields have different,
explicit scopes; current models are never serialized to manufacture source proof.
The shipped graph/package profiles select schema-discovery evidence that current
models discard. Consequently `AssessModelWithResources` and `AssessMXLPackage`
bundles remain incomplete and cannot be `Satisfied()`, even when their intrinsic
current-model reports are satisfied. The discarded hint history stays Unknown.

External retrieval and image/association/role metadata require an explicitly
selected dependency and caller-provided resolver. Empty selections perform no
external I/O. The resolver owns transport authorization; no built-in network,
filesystem, schema-hint, DTD or entity fetching is introduced. One finite account
covers the complete graph, including aliases and nested packages; legacy MXL byte
caps also apply. Unavailable providers, incomplete inventories and unsupported
ZIP/media/indirection interpretations remain explicit gaps. Current models cannot
recover discarded original schema hints and can therefore remain incomplete even
without outgoing links. See the [package scope](docs/validation/package-profile-scope.md)
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
default to 64 MiB, with 20,000,000 work steps and depth 1024; the separate MXL byte
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

The library code is licensed under the [MIT License](LICENSE). Bundled schemas
and test fixtures retain their upstream terms; see
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
