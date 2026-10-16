# Explicit DTD and dictionary package composition

These additive profiles compose existing bounded operations. They do not claim
complete MusicXML, XML DTD validation, ZIP, IPA, playback or rendering closure.
Composition preserves existing profile identities except for the separately
documented figure-number, IPA representation and mimetype local-header
corrections; see
[integration identity qualification](#integration-identity-qualification).
Preserved profile metadata does not promise unchanged outcomes for corrected
predicates; see the [legacy behavior note](local-link-context-scope.md#legacy-behavior-corrections).

## Explicit entry points and fixed interpretations

| Entry point | Package profile |
| --- | --- |
| `InspectXMLWithResourcesAndDictionaries`, `AssessModelWithResourcesAndDictionaries` | `ProfileResourceGraphInternalDTDDictionaries` |
| `InspectMXLWithDictionaries` | `ProfileMXLSourceInternalDTDDictionaries` |
| `AssessMXLPackageWithDictionaries` | `ProfileMXLModelInternalDTDDictionaries` |

Choose `Resources.Graph.Profile` explicitly. Existing dictionary profiles retain
their selected scope, subject to the separately documented corrections. Empty,
unknown and cross-entry-point selections, invalid
budgets, dictionaries, dependencies and MXL limits fail before caller/provider
I/O. Ordinary non-dictionary entry points do not acquire a new mode.

Every new package catalog pins source MusicXML/Opus to the existing standalone
`ProfileSourceInternalDTDDictionaries`, source Container to
`ProfileSourceInternalDTDContainer`, source Sounds to
`ProfileSourceInternalDTDSoundsDictionaries` and current MusicXML/Opus models to
`ProfileModelDictionaries`, together with the exact engine catalog
digest and target. Raw MXL omits model rows. A successful source snapshot export
is transport, not proof that every provenance component is available.

Public musical link/rootfile traversal still requests musical sources. The
Sounds row supports explicit compatible snapshot admission, as tested through
private adapter seams; it does not provide a public auxiliary-file selector.
Standalone `InspectXMLWithDictionaries` accepts the new Sounds profile, and
`InspectXML` accepts the new Container profile. Neither creates a Document model
or decode root. Filenames never select auxiliary profiles.

## Source evidence

Only actually assessed source XML receives bounded internal-DTD processing. Only
actually-read internal declarations establish declaration evidence. External DTDs,
entities, schemas and dictionaries are never fetched automatically. Missing
capabilities and reached unsupported declarations remain incomplete.

Container paths/media types are effective XSD-assessed tokens. A defaulted path
can route an archive entry while physical `FullPathRaw` is Absent. Physical
attributes retain available XML-normalized pre-DTD-tokenization spellings;
unretained general-entity spellings are Unknown. Entity-generated attributes
have Absent physical Raw. Missing required attributes retain their independent
Invalid-with-cause evidence. Both enclosing content proofs are necessary for a
complete ordered rootfile inventory.

Sounds mapping fields remain effective supplied values, including explicit DTD
defaults. Library IDs retain xs:string whitespace, ensemble number retains its
xs:NMTOKEN domain, and ID uniqueness/content checks remain independent. Reverse
and forward preferences and alternatives remain distinct; no implicit primary
mapping is chosen. Known nonstandard IDs remain allowed, and missing dictionary
data cannot suppress independent XSD violations.

Auxiliary source hints independently preserve physical Raw/presence and effective
pairs. A default/entity-produced hint can provide effective values with Absent
physical fields. Odd `schemaLocation` token counts remain a discovery-capability
gap, not an added list(anyURI) XSD violation. Hints never select a schema.

## Ownership, identity and boundaries

One finite Account and one verified dictionary set serve every child. Primary
reader bytes are charged once and the caller reader stays open. Provider bodies
are closed once by their owner. Retained archive bytes are not recharged as input;
lexical, declaration, snapshot and adoption allocations stay on the same account.
Failed transfers leave snapshots caller-owned; successful transfers move sole
release ownership to the package session.

Cache identity retains pin, exact profile and actual dictionary fingerprint;
core transfer identity also pins catalog digest and target. Repeated adoptions
receive distinct document identities and independently rebound nested origin
locations, nodes, ancestry and causes. Context indexes are the actual emitted
indexes, including origin rows. A current primary model is assessed directly and
never receives reconstructed source origins, hints, original Container or ZIP
history. Retained source children are separate evidence.

Composition retains the package/ZIP/path/media/indirection/association/role gates
and legacy partial IPA limits; the separate corrections below still apply.
Technical traversal of nested MXL does not settle unresolved
outer indirection obligations, and successful source assessment does not imply
that independent package obligations are satisfied.

## Integration identity qualification

Relative to merged PR40 `5295080`, the separately documented
[figure-number correction](evidence/figure-number-conformance-correction.md)
migrates `ProfileSourceContext` and `ProfileModelContext`. The
[IPA representation migration](ipa-representation.md) changes
`ProfileSourceDictionaries`, `ProfileSourceSoundsDictionaries` and
`ProfileModelDictionaries`, as well as dictionary-containing compositions.
`TestSourceDeclarationCompositionPreservesProfiles` retains its original baseline
and applies only these explicit standalone migrations. `ProfileSourceXSDPreview`,
`ProfileSourceLocal`, `ProfileSourceLinks` and `ProfileSourceInternalDTD` retain
their complete Profile JSON.

The separately reviewed mimetype local-header correction changes rule and
closure text, and therefore the six ordinary/dictionary package profile/catalog
identities. Those are intentional semantic-correction identities, not changes
introduced by DTD composition. Earlier pre-composition captures are historical
local checkpoints and are not substitutes for the published PR40 baseline. The
three new standalone DTD compositions also have intentional shorthand/IPA
identity changes before publication. Historical receipts and pre-correction
hashes remain dated evidence; the [migration ledger](ipa-representation.md)
distinguishes those checkpoints from public before/after identities.

The separately selected [IPA representation capability](ipa-representation.md)
adds bounded affirmative whole-input witnesses through these existing adapters.
Unproved representations and missing capabilities remain unavailable; no
phonotactics, pronunciation intent or universal IPA conformance is asserted.
