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
activate any selection, replace a nil source, or supply files for the caller.

Upstream refresh is separate reviewed maintenance: obtain the intended inputs
explicitly, review provenance and licensing, and update the affected fixed pins,
transform/policy identities and migration evidence as needed before regenerating.
There is no download or "latest" mode in the generator.

The original SMuFL and MusicXML source components and the two reviewed derived
IPA bundles retain their distinct authority. IPA remains bounded positive-witness
evidence, not a complete grammar or pronunciation validator. Standard Sounds
nonmembership does not invalidate custom open-string identifiers.

The generated `data` directory retains the source notices and Unicode permission
text. These data have separate upstream/adapted-data terms, including CC BY-SA 4.0
for the IPA-derived data; the implementation code license does not replace them.
See `THIRD_PARTY_NOTICES.md` at the repository root. Preserve applicable notices
and license links when distributing the data, including in compiled binaries.
