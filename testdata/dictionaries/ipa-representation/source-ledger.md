# Bounded IPA 2015 / Unicode 13 representation witness

Reviewed 2026-10-14. This is project-authored derivative data and source accounting,
not a reproduction of a chart, historical publication, full Unicode corpus or font.
The separate adapted-data and Unicode notices in NOTICES.txt apply. Independently
authored implementation code has the project's ordinary code license.

## Exact admitted facts

The 453 typed rows comprise 295 complete encoding atoms (273 singletons and 22
noncanonical letter constructions) and 158 genuine Unicode 13 recursive canonical
decompositions. There are no literal example rows. The 170 original symbol facts
are preserved as inputs, with 103 additional singleton representations. The old 197
row positive-v1 component and its transformed serialization remain untouched.

symbol-role-witness.tsv gives every complete atom, its semantic role and evidence
identifier. The 170 `legacy:` references resolve to the unchanged
[positive ledger](../ipa-positive/positive-ledger.json). source-row-ledger.json
records all 125 new singleton/construction identities and their source witnesses.
new-singleton-witness.tsv and construction-witness.tsv are compact audit views.
canonical-closure-witness.tsv records all 158 genuine mappings; they are support
facts only, and do not independently authorize arbitrary strings.

The normalized alphabet has 278 leaves including four half marks, 53 nonzero CCC
values, and 436 assigned raw input scalars after adding decomposition keys.
assigned-input-witness.tsv includes exact DerivedAge13 line witnesses. No Hangul
Jamo leaf belongs to this alphabet, so algorithmic Hangul decomposition cannot
contribute an omitted canonical equivalent. Unicode compatibility mappings used
to identify a modifier's parent are evidence only; the runtime never applies
compatibility normalization, case folding or a host Unicode normalizer.

The roles are 125 direct letters, 74 modifier letters, 22 constructed letters,
47 independent combining marks and 27 other signs/boundaries. In particular,
U+02DE is a sign, not a half-tie base. The 248 starter atoms have 225 distinct
normalized starter values and at most two candidates each.

## Source definitions and scope

- CH15: International Phonetic Association, IPA Chart revised 2015; exact Doulos
  and Kiel originals are pinned in source-pins.json. The selected edition is
  [IPA 2015](https://www.internationalphoneticassociation.org/sites/default/files/IPA_Doulos_2015.pdf).
- MAP12: [IPA 2012 symbol list](https://www.internationalphoneticassociation.org/IPAcharts/common_files/pdfs/pdfs_IPA_charts_archive/IPA_2012_symbollist.pdf),
  notably the ASCII-g alternative on p.2 and compound-tone mappings on p.10.
- MAP26: [later Doulos encoding aid](https://www.internationalphoneticassociation.org/IPAcharts/common_files/pdfs/pdfs_IPA_charts_E/IPA_Doulos_sl.pdf).
  Its identities are matched to the selected historical alphabet; it does not
  upgrade membership or Unicode version. Its pin/notice is in the legacy ledger.
- K89: International Phonetic Association, Report on the 1989 Kiel Convention,
  JIPA 19(2),67–80, DOI10.1017/S0025100300003868.
  [Primary publication](https://www.cambridge.org/core/services/aop-cambridge-core/content/view/C10DB13CAB1BAC6187438C74F3FF01EC/S0025100300003868a.pdf/report-on-the-1989-kiel-convention-international-phonetic-association1.pdf).
  Pages71, 74 establish the retained superscript provision; pp.75–77 establish
  ordinary word space and either-side Chao stems/extended contours.
- HB99: Handbook of the International Phonetic Association, 1999, pp.14–17,
  [primary-text extract](https://commons.princeton.edu/eng266-s25/wp-content/uploads/sites/433/2025/02/IPA-Introduction-Handbook.pdf).
  The pinned extract covers pp.3–17, not the whole book. It establishes tone,
  positional-diacritic, rhotic and overlay identities.
- SUP20: [L2/20-252R](https://www.unicode.org/L2/L2020/20252r-mod-ipa-a.pdf),
  2020-11-08; IPA Council letter pp.10–12. Later confirmation of the earlier
  superscript provision, including implicit current consonants. It does not
  authorize post-13 encodings or every Unicode modifier character.
- RET04: [L2/04-046](https://www.unicode.org/L2/L2004/04046-retroflex-hook.pdf),
  2004-02-01, pp.5–7, exact rhotic/retroflex identities.
- TIL03: [WG2 N2632](https://scripts.sil.org/cms/sites/nrsi/download/middletildeproposaltowg2/MiddleTildeProposalToWG2.pdf),
  2003-09-30, pp.4–5,8–9, current tilde-overlay constructions.
- SCOT19: [WG2 N5036R / L2/19-075R](https://www.unicode.org/wg2/docs/n5036r-scots-phonetics.pdf),
  2019-05-05, p.1, the Unicode 13 turned-r-with-tilde identity under the historical
  overlay provision.
- ABOVE24: [L2/24-080](https://www.unicode.org/L2/L2024/24080-ipa-diacritics-above.pdf),
  2024-03-31, p.2, corroborates historical positional practice. Newer proposed
  scalars and extIPA identities are not imported.
- Combining ejective: [L2/20-253R](https://www.unicode.org/L2/L2020/20253r-mod-ipa-b.pdf),
  2020-11-08, pp.1–2. U+0315 is one independent sign; no 83-row enclosing-string
  grammar or superscript scope interpretation is introduced.
- Compound tone accents: [L2/04-348](https://www.unicode.org/L2/L2004/04348-phonetic.pdf),
  pp.3–6; HB99 p.14 and MAP12 p.10 independently establish the three additions.
- [L2/20-125R](https://www.unicode.org/L2/L2020/20125r-ipa-retroflex.pdf):
  pp.4 and13, context for current identities and construction encodings.
  P22 has its strongest direct witness in SUP20 Council p.11 and NamesList13
  A78E. The resident source digest is recorded.
- DoNotEmit17: [Unicode 17 DoNotEmit.txt](https://www.unicode.org/Public/17.0.0/ucd/DoNotEmit.txt).
  This later encoding guidance corroborates 17 exact hook spellings, not
  Unicode 13 normalization and not admission of newer target scalars.
- U13: [Unicode 13 chapter7](https://www.unicode.org/versions/Unicode13.0.0/ch07.pdf),
  §§7.8–7.9; [UnicodeData13](https://www.unicode.org/Public/13.0.0/ucd/UnicodeData.txt),
  [DerivedAge13](https://www.unicode.org/Public/13.0.0/ucd/DerivedAge.txt),
  [NamesList13](https://www.unicode.org/Public/13.0.0/ucd/NamesList.txt), and final
  [UAX15 revision 50](https://www.unicode.org/reports/tr15/tr15-50.html).
  These establish assigned scalar identities, CCC, recursive canonical
  decomposition, stable equal-CCC ordering and the two half-tie representations.

source-pins.json hashes only the 15 resident originals independently verified in
this review. Citation-only sources do not acquire invented raw hashes. None of
these original publications is redistributed by this fixture.

## Four explicitly inferred constructions

R05,R06,R14,R17 have weaker, compositional representation evidence. They are the
exact turned-r/s retroflex-hook spellings and their modifier counterparts:
NamesList13 supplies the current letter identities, U13 §7.9 supplies the
shape-building U+0322 role, and the established superscript provision supplies
modifier identity. They are not claimed as exact DoNotEmit mappings and are
never recorded as canonical equivalences. Only the22 complete named atoms are
admitted; free U+0322/U+0327 and arbitrary base-plus-hook/cedilla remain unproved.
R22 is the separately evidenced superscript-cedilla construction.

## Fixed recognition and half-tie boundary

Every raw scalar must belong to the sealed 436-scalar support domain. One-step
full recursive NFD expansion preserves each raw scalar's UTF-8 interval. Stable
CCC segment sorting never crosses a starter. A complete atom's required marks
must prefix each equal-CCC run after separately accounting for half marks;
residual marks must be independently admitted. This recognizes lower-CCC
interleaving without accepting free construction components or reordering
unequal equal-CCC signs. Leading independent marks and empty concatenations
have witnesses.

U13 §7.9 pp.327–328,333–334 and NamesList13 establish FE20/FE21 for 0361 and
FE27/FE28 for 035C. Exactly one correctly oriented pair may span two immediately
adjacent represented bases; independent single-base diacritics can decorate
both. Multiple disjoint pairs are allowed. Repeated halves, multiple halves in
one segment, shared-middle pairs, mismatched orientations, skipped bases,
unmatched endpoints and endpoints on signs remain unavailable.

A half-pair endpoint may not contain a whole tie (035C/0361), and the segment
immediately preceding the left endpoint may not contain one. This conservative
mixed-tie boundary avoids unproved overlap. It does not change canonical
identity, turn the pairs into canonical decompositions, or establish rendering
scope for mixed/repeated ties. Whole ties remain independent existing atoms
outside this boundary.

For known string scalar input, the public representation result is one unchanged
whole-input byte span, including empty or unproved text. Unavailable/non-string
scalar evidence retains its state and cause without an invented raw interval. A successful complete witness alone makes
Result and WholeSequence known-true; absence remains unavailable, never a
fabricated negative complement. This is a bounded affirmative representation
capability, not phonotactics, pronunciation intent or universal IPA conformance.

## Reproducible pins and review

manifest-pins.json records raw, transformed and minimal-policy identities.
The new transformed format is the existing length-prefixed dictionary encoding
with one additional policy-SHA256 string immediately after its five manifest
strings and before the row count. Only this new manifest uses that extension.
The exact policy bytes bind the 53 CCC entries, role policy, two half-pair
relations, mixed/repeated exclusions and whole-input projection contract.

Independent review reproduced every count and all 15 source pins without running
the design derivation, compared 81,488 finite construction/mark oracle segments
with zero mismatches, verified 17 exact hook correspondences, and reconstructed
the final 453-row serialization and pins independently. These checks support the
bounded design; implementation tests separately cover ownership, raw provenance,
work/storage limits, cancellation and consumer integration. They are not a
universal representation or phonological proof.
