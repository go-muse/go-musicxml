# Third-party notices

The project license applies to the `go-musicxml` library code. The following
bundled material retains its upstream terms and notices.

## MusicXML 4.0 schemas

Files under `schema/musicxml-4.0` originate from the MusicXML 4.0
specification.

Copyright © 2004-2021 the Contributors to the MusicXML Specification,
published by the W3C Music Notation Community Group under the
[W3C Community Final Specification Agreement](https://www.w3.org/community/about/agreements/final/).
The source files contain the complete notices.

## MusicXML Test Suite

`testdata/corpus/musicxml-test-suite` is a snapshot of the
[MusicXML Test Suite](https://github.com/w3c-cg/musicxmlTestSuite) at commit
`b2e6a1627b8574c9714e1fd0a8a5b1921e10f8f3`.

Copyright © 2016-2026 Michael Scott Asato Cuthbert.
The snapshot is distributed under the MIT License; its `LICENSE` file is
preserved in the fixture directory.

## MusicXML examples

Files under `testdata/official` come from the
[MusicXML example set](https://www.musicxml.com/music-in-musicxml/example-set/)
and the
[MusicXML 4.0 opus reference](https://www.w3.org/2021/06/musicxml40/opus-reference/examples/).
Embedded copyright and rights statements are preserved.

## Pinned SMuFL 1.4 and MusicXML 4.0 dictionary data

The original glyph-name and range fixtures under
`testdata/dictionaries/smufl-1.4` derive from Standard Music Font Layout
(SMuFL), version 1.4, copyright © 2021 its contributors. The original
`testdata/dictionaries/musicxml-4.0/sounds.xml` derives from MusicXML,
version 4.0, copyright © 2004–2021 its contributors. These specifications
are published by the W3C Music Notation Community Group under the
[W3C Community Final Specification Agreement](https://www.w3.org/community/about/agreements/final/).

Their source bytes, generated dictionary identities and relation data retain
these upstream attributions and terms. Exact source commits, source URLs and
transformation boundaries are in
[testdata/dictionaries/NOTICES.txt](testdata/dictionaries/NOTICES.txt).
The library's MIT code license does not replace these data terms. No font
program or glyph artwork is included.

## Partial IPA positive-evidence data

The data under `testdata/dictionaries/ipa-positive` are separately licensed
under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/).
They derive from selected symbols and examples on the International Phonetic
Association's 2015 chart (originally CC BY-SA 3.0), explicit encoding assistance
from its 2026 Doulos symbol list (CC BY-SA 4.0), and Unicode 13 data.
The original source attributions, modification description, license links and
Unicode permission notice are preserved in
[testdata/dictionaries/ipa-positive/NOTICES.txt](testdata/dictionaries/ipa-positive/NOTICES.txt)
and the adjacent Unicode license file. The full positive ledger and source
pins accompany the compact runtime representation. These data are not covered
by the library code's MIT license. Preserve their notices when distributing
these data or generated equivalents. No chart artwork or font is bundled.

## Additive IPA representation data

The separate data under `testdata/dictionaries/ipa-representation` reuse the
legacy symbol lineage, add reviewed singleton/construction identities and
Unicode13 canonical relations, and retain the adapted-data CC BY-SA 4.0
attribution and modification notices. The 53-value CCC table in
`validation/dictionary_ipa_representation_policy.go` is minimal Unicode-derived
data with the separate Unicode permission notice. See the
[representation notice](testdata/dictionaries/ipa-representation/NOTICES.txt),
[source ledger](testdata/dictionaries/ipa-representation/source-ledger.md) and
adjacent Unicode license snapshot. Implementation code remains separately
licensed. No full Unicode corpus, historical publication, chart artwork or font
is included by this addition.

## Embedded copies of the pinned dictionary data

`internal/dictionarydata/data` contains byte-identical generated copies of the
five supported components described above, together with copies of their
notices and Unicode permission text. Using `BundledDictionarySource` makes all
five component assets reachable, including both CC BY-SA 4.0 IPA bundles,
regardless of `Selections`. Selections control import and assessment, not
which assets the bundled implementation references. These data retain their
upstream and adapted-data terms; the library code's MIT license does not
replace them.

Distribute binaries containing the bundled data with all five of these files,
including their attribution, change descriptions, disclaimers and license
links:

- `internal/dictionarydata/data/NOTICES.txt`
- `internal/dictionarydata/data/ipa-positive/NOTICES.txt`
- `internal/dictionarydata/data/ipa-positive/LICENSE-UNICODE-V3.txt`
- `internal/dictionarydata/data/ipa-representation/NOTICES.txt`
- `internal/dictionarydata/data/ipa-representation/LICENSE-UNICODE-V3.txt`

The first notice covers SMuFL 1.4 and MusicXML 4.0 under the W3C FSA. The two
IPA notices preserve the adapted-data CC BY-SA 4.0 attribution and change
notes; the two Unicode files preserve the Unicode License v3 permission text.
These files are included in the embed patterns so `go mod vendor` retains them
with the data. That source-distribution safeguard does not arrange notice
delivery alongside a compiled binary: its distributor must include the files
in the distribution or associated documentation. Programs using only
caller-provided or nil sources do not need the bundled source for assessment;
no particular linker behavior or binary-size saving is guaranteed.

The copied notices remain byte-identical to the source fixtures. Their
references to accompanying ledgers describe the full source distribution. The
compact runtime asset directory does not contain those detailed modification
and provenance records. For the embedded IPA positive bundle, see the
[full positive ledger][ipa-positive-ledger] at
`testdata/dictionaries/ipa-positive/positive-ledger.json`. For the representation
bundle, see the full [source ledger][ipa-representation-ledger] at
`testdata/dictionaries/ipa-representation/source-ledger.md`, its
[source pins][ipa-source-pins], [row ledger][ipa-row-ledger] and
[manifest pins][ipa-manifest-pins]. These links pin the provenance records for
the bundled bytes to merged main commit
`0b5a2f24b89ef628d2d09b361cbcbe390b9f01b9`. All five files have the same blob
identities as the bundled-source implementation's initial PR 42 head. This
main-history pin keeps the provenance independent of later PR-branch changes.
Future data refreshes must retain separately reviewed exact-pin provenance.

Generation changes neither source bytes nor authority: the two IPA components
remain reviewed derived bundles, not the original publications or a complete
grammar.

[ipa-positive-ledger]: https://github.com/go-muse/go-musicxml/blob/0b5a2f24b89ef628d2d09b361cbcbe390b9f01b9/testdata/dictionaries/ipa-positive/positive-ledger.json
[ipa-representation-ledger]: https://github.com/go-muse/go-musicxml/blob/0b5a2f24b89ef628d2d09b361cbcbe390b9f01b9/testdata/dictionaries/ipa-representation/source-ledger.md
[ipa-source-pins]: https://github.com/go-muse/go-musicxml/blob/0b5a2f24b89ef628d2d09b361cbcbe390b9f01b9/testdata/dictionaries/ipa-representation/source-pins.json
[ipa-row-ledger]: https://github.com/go-muse/go-musicxml/blob/0b5a2f24b89ef628d2d09b361cbcbe390b9f01b9/testdata/dictionaries/ipa-representation/source-row-ledger.json
[ipa-manifest-pins]: https://github.com/go-muse/go-musicxml/blob/0b5a2f24b89ef628d2d09b361cbcbe390b9f01b9/testdata/dictionaries/ipa-representation/manifest-pins.json
