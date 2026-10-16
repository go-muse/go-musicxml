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
five selected components described above, together with copies of their notices
and Unicode permission text. `BundledDictionarySource` makes those local data
available through explicit dictionary selections; their upstream and adapted-data
licenses continue to apply when the data are embedded in a binary. Preserve the
applicable notices and license links when distributing those data. The generation
step changes neither source bytes nor authority: the two IPA components remain
reviewed derived bundles, not the original publications or a complete grammar.
