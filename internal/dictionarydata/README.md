# Pinned runtime dictionary assets

This package embeds byte-identical copies of the five existing component pins in
`validation/dictionary_manifest.go`. The source inputs and provenance remain in
`testdata/dictionaries`. No manifest, transform, policy, profile or catalog identity
changes merely because the bytes are embedded.

From the repository root, run:

```
go generate bundled_dictionary_generate.go
```

The generator reads only committed local inputs. It checks regular-file lengths
before bounded reads and validates all five source digests before writing any
output, then copies the components and their license notices verbatim. Repeated
generation is byte-stable, including original line endings. The existing
`go generate ./...` check also includes this directive. An explicit alternative
is `go run ./internal/dictionarygen/cmd/dictionarygen -root /path/to/repository`.

Ordinary `go build` and runtime use consume committed embedded assets and invoke
neither generation nor downloading. `BundledDictionarySource` gives each caller
an independent context-aware reader over immutable shared strings; the normal
accounted importer still verifies and parses the component. The source does not
activate any selection or supply files for the caller by itself. The assessed
loaders select canonical SMuFL and IPA representation evidence when dictionary
options have a nil source and no selections; inspection/assessment APIs retain
explicit selection.

Upstream refresh is separate reviewed maintenance. Preserve existing edition
inputs, manifest/component identities and fixed pins; add a new edition identity
or a new `-vN` revision instead of rewriting a published pin in place. Follow the
[refresh checklist](../../CONTRIBUTING.md#pinned-dictionary-generation) for input
provenance, licensing, generator/embed registration and migration verification.
There is no download or "latest" mode in the generator.

The original SMuFL and MusicXML source components and the two reviewed derived
IPA bundles retain their distinct authority. IPA remains bounded positive-witness
evidence, not a complete grammar or pronunciation validator. Standard Sounds
nonmembership does not invalidate custom open-string identifiers.

The generated `data` directory retains the source notices and Unicode permission
text. They are explicitly embedded so `go mod vendor` preserves these exact files:

- `data/NOTICES.txt`
- `data/ipa-positive/NOTICES.txt`
- `data/ipa-positive/LICENSE-UNICODE-V3.txt`
- `data/ipa-representation/NOTICES.txt`
- `data/ipa-representation/LICENSE-UNICODE-V3.txt`

These data have separate upstream/adapted-data terms, including CC BY-SA 4.0 for
the IPA-derived data; the implementation code license does not replace them.
Any binary referencing `DecodeAssessed` or `DecodeAssessedWithDictionaries` embeds
all five components, even when calls pass custom sources or selections. Direct
use of `BundledDictionarySource` also retains all five. Binary distributors must
ship the five files above alongside the binary, retaining their attribution and
license links. The private notice
filesystem can be removed by the linker; embedding it does not guarantee that
notices are accessible from a compiled binary. See `THIRD_PARTY_NOTICES.md` at the
repository root for distribution guidance.

The copied notices are byte-identical source records. Their references to an
accompanying full positive ledger concern the original source distribution,
not this reduced runtime directory. The full reviewed provenance is available
in the pinned [positive ledger][ipa-positive-ledger] and
[representation source ledger][ipa-representation-ledger] at merged main commit
`0b5a2f24b89ef628d2d09b361cbcbe390b9f01b9`. These records are byte-identical to
those cited by the bundled-source implementation's initial PR 42 head; the
main-history pin keeps them independent of later PR-branch changes. The
[project distribution guidance][bundled-notices] also links the representation
source pins, row ledger and manifest pins at the same merged-main commit.

[ipa-positive-ledger]: https://github.com/go-muse/go-musicxml/blob/0b5a2f24b89ef628d2d09b361cbcbe390b9f01b9/testdata/dictionaries/ipa-positive/positive-ledger.json
[ipa-representation-ledger]: https://github.com/go-muse/go-musicxml/blob/0b5a2f24b89ef628d2d09b361cbcbe390b9f01b9/testdata/dictionaries/ipa-representation/source-ledger.md
[bundled-notices]: ../../THIRD_PARTY_NOTICES.md#embedded-copies-of-the-pinned-dictionary-data
