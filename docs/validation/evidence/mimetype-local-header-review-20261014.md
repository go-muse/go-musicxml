# Mimetype local-header scope review

14 October 2026. This source-supported contextual interpretation supersedes the
older central-only no-extra gate. It does not relabel historical planning rows,
source hashes, test executions or overall package acceptance.

## Authority and mechanism

The [MusicXML 4.0 compressed-file tutorial](https://www.w3.org/2021/06/musicxml40/tutorial/compressed-mxl-files/)
explains identification from the beginning of the archive and expressly identifies
EPUB as using the same mechanism. The originating
[MusicXML issue 185](https://github.com/w3c-cg/musicxml/issues/185) links EPUB 3.1;
[PR 206](https://github.com/w3c-cg/musicxml/pull/206), merged 28 April 2017 at
`7907a80bd410858f23c88fcee4405fea4fcd5754`, adopts that mechanism. MusicXML's
[3.1 packaging history](https://www.w3.org/2021/06/musicxml40/version-history/31/#packaging)
links the change to the same issue. The
[pinned container annotation](https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/container.xsd#L46)
retains the no-extra condition in MusicXML 4.0.

The exact predecessor,
[EPUB OCF 3.1, 25 January 2017](https://www.w3.org/submissions/2017/SUBM-epub-ocf-20170125/),
§4.3 and Appendix C, specifies the initial local signature, filename at offset 30
and MIME payload at offset 38. A local extra field moves the payload; a later
central-directory extra does not. OCF's cited
[OpenDocument 1.0 specification](https://docs.oasis-open.org/office/v1.0/OpenDocument-v1.0-os.pdf),
§17.4, printed pages 685–686, explicitly explains this fixed-position rationale.

Corroborating historical implementation:
[EPUBCheck v4.0.2, EpubCheck.java lines 266–311](https://github.com/w3c/epubcheck/blob/3de6ddb86dedf6c8f09c3f2211d158cbb27bfc4a/src/main/java/com/adobe/epubcheck/api/EpubCheck.java#L266-L311)
checks the local extra-length field at offset 28. That version predates the
MusicXML adoption. The
[EPUB editor's 7 January 2019 explanation](https://github.com/w3c/epub-specs/issues/1206#issuecomment-452051710)
also identifies preservation of the fixed magic positions as the purpose.

This is a high-confidence whole-source interpretation, not an express MusicXML
erratum containing the word “local.” It does not import EPUB's mandatory first
entry, exact content, trailing-whitespace restriction or other packaging rules.
MusicXML's own recommendations and mandatory conditions remain distinct. The 18
MusicXML 4.0 official sample archives inspected during the source review all had
zero local and central mimetype extras, so they do not distinguish central-only
scope and are not claimed as precedent for that case.

## Exact implementation contract

For an independently identified raw `mimetype` entry:

- Nonzero proved local extra length fails, even if later central or transport
  evidence is missing or invalid.
- Zero proved local extra length and independently associated headers pass the
  no-extra predicate. Central-only extras do not create an interpretation gap.
- Missing local framing, local length or entry association cannot prove success.
- Current-model boundaries still cannot reconstruct discarded original bytes.
- Other entries are outside this conditional MusicXML no-extra predicate.

No passing local predicate certifies central structural validity, successful
transport, encryption/compression compliance, CRC, entry obligations or complete
MusicXML conformance. Existing independent checks and source/model limits remain.

### Separate central-extra framing capability

The [exact Info-ZIP 11 March 1997 APPNOTE](https://infozip.sourceforge.net/doc/appnote-19970311-iz.zip)
distinguishes local and central records in General Format A/C. Explanation of
fields D, extra field, lines 291–347 of `appnote.iz`, describes repeated blocks
with a two-byte header ID, two-byte data size and the indicated following data.
The inspected member has SHA-256
`499fcebdc91e97cd2eee72501b03a3f1638ce656c7a934d380ba075c772117bb`.
Its wording is advisory; this review does not turn it into a new mandatory
MusicXML failure.

The pre-change observer deliberately tolerated unframed uninterpreted extra
payloads. Merely dropping the old central-only gate would newly allow an archive
with truncated central mimetype extra blocks to satisfy the bounded profile.
The narrow correction keeps that case capability-incomplete: a partial block
header or advertised payload exceeding the retained extra bytes gives
`unframed mimetype central extra field`. The finite existing scan still checks
ZIP64 as before and charges/polls at the same block boundaries. Complete unknown
blocks remain uninterpreted; unrelated/local extra admission is unchanged.

This capability error is deferred until the existing entry range and identity
checks finish. Thus a proved local-zero no-extra predicate can pass while the
overall archive is incomplete, and a proved local-extra violation still fails.
If a separate structural failure prevents association, missing association is
still a legitimate reason why local-zero success cannot be established.

## Regression evidence

New core tests exercise neither/local-only/central-only/both, unobserved local or
central length, unproved association/framing, unrelated entries, and a known
local failure surviving unavailable central evidence and cancellation. New public
raw-MXL tests exercise the same header matrix, independent malformed central
records, association/CRC failures and missing directories. Separate extra-block
fixtures prove capability incompleteness without losing local predicate results
or exact retained association. Observer cleanup assertions cover buffered/depth
account release.

The semantic tests were executed against unchanged production code before the
fix and failed on central-only Unknown (and the core's irrelevant central-length
gate). The separate framing tests failed before that capability guard was added.
The repository's current acceptance still requires applicable checks on the
final integration head; these local regression routes do not replace full gates.

## Public profile identity migration

The [published package baseline ledger](../ipa-representation.md#published-package-baseline)
records all six ordinary and dictionary package profiles from public main
`5295080`. The local-header rule/closure correction deliberately changes the
ordinary package catalog digest; dictionary package identity also incorporates
its intrinsic dictionary migration. These are catalog digests, not complete
Profile JSON hashes. `TestPackageProfilesPublishedBaselineMigration` checks the
complete captured public baseline and each exact before/after catalog value.
