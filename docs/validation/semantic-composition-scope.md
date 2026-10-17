# Encoded semantic composition

This composition is explicitly selectable for inspection/assessment and fixed in
the assessed loaders. It joins the implemented MusicXML 4.0 checks without
claiming universal conformance or reconstructing a desired score. It uses the existing
assessment and dictionary entry points; no new provider or options structure is
introduced. Inspection and standalone model/package assessment require explicit
selection. The later [assessed-loading migration](assessed-loading-pipeline.md)
fixes these existing source/model profiles and required bundled evidence in both
assessed decoders; it does not change these profile identities or enlarge their
normative scope.

## Exact interpretations

- `ProfileSourceSemantics` selects source XSD, local, exact timeline and contextual
  checks, actually read internal declarations, owned source origins, canonical
  SMuFL membership and supported IPA evidence. Physical attributes and effective
  defaulted/normalized values remain distinct. The
  [external-DOCTYPE source limit](../../API.md#original-source-inspection-and-assessed-loading)
  applies: an unread external subset leaves selected source checks unavailable.
- `ProfileModelSemantics` selects current encoded model facts. It does not invent
  discarded DTD, lexical, namespace, original-byte or source-hint information.
- `ProfileResourceGraphSemantics`, `ProfileMXLSourceSemantics` and
  `ProfileMXLModelSemantics` pin those exact intrinsic profiles and catalog digests
  for MusicXML/Opus source/model resources. Original MXL and current-package
  evidence retain their separate scope. Retained source children are assessed as
  source even when their primary package document is a current model.

The source-only Container interpretation retains bounded DTD/effective metadata
and independently owned physical origins. The existing explicit Sounds auxiliary
interpretation is retained in the catalog table; public graph/MXL wrappers do not
introduce Sounds discovery. Standard-sound recognition and dictionary-backed
rendering advice are not added to the score profiles.

All entry points reject incompatible profile/target combinations before caller
I/O. One root imports only selected dictionary components, owns the set, and
shares its account and actual capability identity across intrinsic assessments
and cache reuse. Inspection and standalone model/package assessment keep
caller-chosen dictionary sources and selections, including the embedded
`BundledDictionarySource`. The assessed decoders default to canonical SMuFL and
IPA representation evidence and require both relations in an explicit override.
There is no schema or external DTD fetch. Existing resolver authority and
resource ownership contracts still apply.

## XML fragments and musical proofs

The exact MusicXML and Opus link plans elect the registered XML-media fragment
baseline: complete XPointer Framework syntax, shorthand, and `element()` child
addressing. URI decoding occurs once. Child ordinals count elements, including
unprojected leaves, rather than text/comments. Source shorthand joins effective
DTD and XSD identity domains; current models have only current-state XSD identity.
First document-order resolution remains separate from duplicate-ID validity.
Incomplete supported evidence blocks fallback; unsupported schemes are skipped.
There is no XPath interpreter, external declaration retrieval, or compressed
primary substitution. Existing indirection uncertainty remains independent.

Barline checks use independently established cursor/end evidence. Empty measures,
unknown movers and unsupported endpoint shapes remain unavailable rather than
being filled from meter. Graphical-repeat checks preserve their bounded positive
and negative proof domains, including the
[closed singleton-repeat theorem](evidence/graphical-repeat-singleton-counterexample.md);
they do not render playback. Unrestricted figure-number
text is not subjected to the removed invented numeric grammar. No external
performer/exporter intent API is selected.

## Limits and acceptance

An unavailable dictionary or source capability remains explicit where a selected
occurrence needs it. The legacy IPA positive manifest remains partial. The
separately selected new IPA representation relation has independently reviewed
witness and policy evidence; this composition does not enlarge its declared
affirmative scope. Independent XSD or encoded semantic failures survive incomplete
package/resource evidence.

These are new profile/catalog identities. Existing published profiles are not
silently elected or widened by selecting a similarly spelled ID. Separate reviewed
corrections to old predicates (including figure text and local-header mimetype
extras) have their own documented identity migrations. Resource accounting is
finite; composition adds real retained/work costs and does not promise unchanged
near-limit outcomes.

Current evidence includes public standalone source/model tests, source/defaulted
attribute controls, all four graph/MXL entry paths, wrong-entry-point no-I/O
controls, direct `element()` success/absence, and preserved intrinsic barline
failures. The
[14 October 2026 receipt](source-declarations-evidence.md#dated-integrated-receipt)
records seven local stages, eleven fuzz targets and eleven hosted CI checks for
tree `fa90c0b32ac027f45bbadcdabcd9a4b1c80fd8af`. It remains separate from
earlier segment receipts and from verification of subsequent review corrections.

[`TestSemanticProfilesExactFinalIdentity`](../../semantic_profile_identity_test.go)
pins the two standalone and three package profiles after the barline citation
correction, including exact CatalogDigest and complete Profile JSON hashes.
These identity assertions are separate from the dated execution receipt above.
