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

- Go 1.26 or later
- MusicXML 4.0 input for schema validation

## Installation

```bash
go get github.com/go-muse/go-musicxml@latest
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
| Read or write compressed MusicXML | `DecodeMXL`, `DecodeMXLWithOptions`, `EncodeMXL` |
| Preserve MXL resources | `DecodeMXLPackage`, `DecodeMXLPackageWithOptions`, `EncodeMXLPackage` |
| Validate against MusicXML 4.0 XSD | `Validate`, root `Validate` methods |
| Construct scores and opus documents | `NewScorePartwise`, `NewScoreTimewise`, `NewOpusDocument` |
| Construct common notes | `NewPitchedNote`, `NewRestNote` |
| Resolve linked opus documents | `MXLPackage.ResolveOpus` |
| Persist resolved-opus edits | `MXLPackage.SyncResolvedOpus` |

The full generated data model is available through the package documentation.
See [`API.md`](API.md) for invariants and compatibility rules.

## Transport and validation

`Decode` and `Encode` are transport operations. They do not validate
automatically, so callers can inspect, repair, and re-encode incomplete
documents. Call `Validate` explicitly when XSD conformance is required.

XML decoding supports UTF-8 (with or without a BOM), UTF-16BE/LE, and
ISO-8859-1. MusicXML root elements are unqualified because the official
MusicXML 4.0 schema has no target namespace. Decoding rejects documents deeper
than 256 simultaneously open XML elements by default; `DecodeOptions` can set
a different ceiling up to the package maximum of 4096.

Optional XSD attributes remain pointers after decoding. Generated
`Effective...` methods expose schema defaults without changing omission
semantics, and `...MatchesFixed` methods check fixed attributes.

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

## Reliability

- The interoperability test corpus contains 150 real MusicXML Test Suite
  files, including XML in multiple encodings and compressed `.mxl`.
- Every decodable fixture is decoded, validated when expected to be valid,
  encoded, decoded again, and compared as a typed Go model.
- A second encoding must match the first byte-for-byte, proving that XML and
  MXL serialization reach a deterministic fixed point.
- On Ubuntu, CI also validates every re-encoded, originally valid corpus
  document with libxml2 against the bundled MusicXML 4.0 XSD.
- CI runs tests on Linux, macOS, and Windows, plus the race detector and three
  fuzz targets.

## Limitations

- The generated model and schema validator target MusicXML 4.0.
- XML extensions not represented by the MusicXML 4.0 model are ignored during
  decoding and are not preserved when re-encoded.
- Encoding normalizes lexical details such as whitespace, attribute order,
  character encoding, and ZIP metadata; the first output need not match the
  original bytes.
- The API is pre-1.0 and may change between minor releases.

## Development

```bash
make check
make release-check
```

`make check` runs formatting, tests, and `go vet`. `make release-check` also
checks module tidiness and generated files, runs the race detector, and
performs short fuzzing passes. Release checks require a clean, committed
worktree. CI and the release workflow additionally install `xmllint` for an
independent XSD conformance pass.

## License

The library code is licensed under the [MIT License](LICENSE). Bundled schemas
and test fixtures retain their upstream terms; see
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
