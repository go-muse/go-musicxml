# Pinned dictionary source fixtures

These original source bytes are test inputs, not automatically loaded runtime
resources. They retain their upstream terms; see [NOTICES.txt](NOTICES.txt) and
[the project notices](../../THIRD_PARTY_NOTICES.md).

- SMuFL 1.4 glyphnames and ranges: upstream commit
  `753f551ca6f96bf24ba38aa2d8a317e706238c51`, files under `metadata/`.
- MusicXML 4.0 standard sounds: upstream commit
  `799e2defb2ece0ae7bafe08dcbcac25b2c631d53`, `schema/sounds.xml`.

`TestPinnedMusicDictionarySourceBytes` verifies exact length and SHA-256,
including original line endings. Git preserves this directory without text
conversion. Sounds retains its external DOCTYPE and copyright comment; the
fixed dictionary transformation does not fetch or validate that DTD. SMuFL
classes, optional font glyphs, font programs and artwork are not selected.

The `ipa-positive` and `ipa-representation` directories contain separately
licensed project-authored derived ledgers, not original publications. The
representation bundle is an explicit test input with 453 records and a separately
pinned minimal policy. Its source and license witnesses are in
[the source ledger](ipa-representation/source-ledger.md); no runtime auto-load or
source fetch is introduced.
