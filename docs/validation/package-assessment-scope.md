# Batch 8 package and linked-resource scope/evidence

Historical packaging snapshot, clarified on 13 October 2026. The observed private
integration head was `388dbe06aaedda02b7d1f07a9ebe35ee6a5fd75f` (tree
`6703aad89730a6f681065253ac32d1e3a5e8babd`). It was an unpublished development
candidate, referenced by the separate engineering archive, not a checkout target
in the published repository. The moving-tree proposal, dirty-worktree records,
row caveats and pending gates below describe that historical observation. They
are not the current acceptance status and are not relabeled as executed here.

For a repository-resolvable publication reference, the PR baseline is
`7b5460af2e2028dc27079353e5f44f2901054d19` (tree
`232598ae2bc28651f8de267770595f5c264470aa`). That reference does not repin or
certify the historical executions. Later review measurements are recorded in
[package assessment evidence](package-assessment-evidence.md); final exact-head
local/remote acceptance must be established separately.

A later incremental checkpoint was published on 13 October 2026 at 10:41 UTC:
`e8db43747d46f4af42c0796e4fc71159b95e729e` (tree
`ce61d65026931569aef1141398c8f4e1e671c638`). Publication was confirmed by the
10:42:04 UTC PR readback. [CI run 38385055748](https://github.com/go-muse/go-musicxml/actions/runs/38385055748)
was in progress at the separate 10:44 UTC observation.
This dated checkpoint is not final acceptance and does not transfer the archived
candidate's execution results to the published tree.

A subsequent readback confirmed that run 38385055748 completed with failure. The
stale all-assembly discovery test expected Unknown for an observed odd hint list;
[commit 6b2bc2953d40ddcbc79f97f8b919548b4b95e1a6](https://github.com/go-muse/go-musicxml/commit/6b2bc2953d40ddcbc79f97f8b919548b4b95e1a6)
corrected that expectation to the reviewed Known(false) pairing contract and
updated only its current test-file hashes. The 10:44 UTC in-progress observation
above remains historical; this terminal outcome neither rewrites it nor certifies
any later tree. The failed run is not acceptance evidence.

## Preservation without a duplicate corpus

Repository text hashes and byte counts use canonical LF (CRLF → LF only), matching the prior cross-platform evidence convention. Apart from the explicitly reviewed [36-record fixture-reference correction](evidence/auxiliary-fixture-reference-review.json), every other byte and original JSON field value remain unchanged; external archive hashes stay exact raw-byte hashes. The ownership manifest hashes the existing original documents and routes every retained obligation by its original ID and exact JSON pointer. Original status strings, source-language fields, source provenance, test contracts, overlaps, capabilities and accepted prior contributions remain in those complete documents. The external 8.9 MB annotated audit and original snapshot remain preserved by exact artifact hash; neither is copied into the repository.

- 95 requirements, 307 context checks, 291 original dispositions, 51 constraints, 28 issue gates and 20 inherited splits remain retained.
- The selected work is 44 engineering rows, 24 context rows, 10 constraints and 5 issue gates.
- 175 selected XSD instances means 36 new auxiliary instances (11 container, 25 sounds) plus 139 reused/transitive instances. It does not mean 211 instances.
- All 42 inherited source contracts, 29 first-profile dispositions, both B6 source subclauses and B5-PERF-01 remain routed.
- Original research decision_required, planned and pending fields are not approval or implementation flags. Completed 177+114 translations are reused; no selected first-time translation is created.

## Evidence precision

The separate row evidence map gives all five axes for each selected row, with actual test function/file/hash routes and explicit target limits. It records assertions visible in test source, not passed tests. Empty test routes and open gaps are intentionally visible. No source-only assertion proves current-model coverage; no ZIP observer byte-retention test proves the semantic classifier result.

- Group matching uses `resource_graph_group_test.go`: exact strings, mismatch, missing/invalid target and source/model scope interactions.
- XLink intent uses `link_intent_test.go`: show × actuate × source/model matrix, including defaults, other/none and invalid values.
- Sounds rows use `auxiliary_sounds_context_test.go`: mapping direction, invalid field causes, alternatives, missing primary, NMTOKEN and source-only boundaries. Container tests are not substituted.
- Original-byte container/sounds XSD fixture axes remain in `auxiliary-source-ownership.json`; the same bytes go to runtime and real xmllint. The existing trace is historical task evidence until rerun on the final tree.

## Targets and retained support boundaries

| Surface | Exact available evidence | Boundary |
| --- | --- | --- |
| Source links | Original source order, normalized/raw carriers, source locations and discovery hints | No automatic fetch; external DOCTYPE remains incomplete |
| Current-model links | Current fields and content order; per-run snapshot identity | No original hints, lexical spelling, source coordinates or invented digest |
| Auxiliary source | Independent container/sounds schemas and typed source projections | No generated auxiliary Document model root |
| Raw MXL | Original ZIP observations, container source, ordered rootfiles, raw nested package boundaries | Bounded feature subset; outer compressed-root interpretation remains unresolved |
| Current MXL | Direct primary model plus owned current resource bytes and logical rootfiles | No reconstructed original headers, physical order, original container source or lexical MIME presence |
| Resource graph | Selected resolver branches, canonical parse and context-specific expansion, typed association/image/group/intent evidence | Exact role protocol, file metadata and four-surface semantic routes are present; final acceptance remains pending and unknown authority is never truth |

## Historical scoped independent acceptance

The archived record for private candidate 388dbe06aaedda02b7d1f07a9ebe35ee6a5fd75f (tree 6703aad89730a6f681065253ac32d1e3a5e8babd) has independent 300-case semantic acceptance, complete validation, focused root/core race, validation vet, unchanged eight-profile captures and refreshed finite-default sizing. The exact reports, commands, logs, test sources and hashes are pinned separately in the evidence archive. That audit identified no active implementation omission at its observation. These scoped passes do not replace whole-tree stages, real-xmllint, actual 386 or remote CI.

## Acceptance gates pending at the historical observation

- seven local stages plus nine exact-head remote jobs: pending final tree; actual 386 and real xmllint required. Owner: lead release/CI verification.

These are batch 8 completion gates. They must not be relabeled as batch 9 residuals. Earlier focused passes, a 386 compile, or old batch 7 workload measurements cannot certify the final tree.

## Selected context and constraint routes

| Original ID | Bounded contribution | Target/evidence caveat |
| --- | --- | --- |
| `CHECK-context-image-scaling` | Expose encoded dimensions and optional exact compatible intrinsic ratio; missing metadata remains optional unknown. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-link-element-position` | Exact next sibling when element absent; next non-marker anchor and supported agreement-only descendant selection when element supplied. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-compressed-relative-links` | Exact score link ZIP-root base, separate from containing Opus path and effective external retrieval base. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-part-link-archive-root` | Exact part-link ZIP-root base, separately proved linked target kind and association. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-group-link-match` | Optional exact-string target score-part/group comparison after reached target context, not graphical part-group. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-mxl-entry-name-utf8` | Observed raw-name UTF-8 validity feeds advice separately from EFS. The transport path policy stops at the first unsafe/non-UTF-8 name; that member may retain advice while later entries remain unavailable. This is not a normative UTF-8-only rule. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-mxl-deflate` | Expose ordinary entry method as observed capability; retain DEFLATE exclusivity and complete external closure gate. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-mxl-mimetype-first` | Optional presence, first physical entry, exact US-ASCII MIME content and legacy omission advice, separate from mandatory existing-entry checks. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-musicxml-file-types` | Supplied filename and media-type conventions, including .musicxml/.mxl/legacy .xml; no content inference. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-mxl-rootfile-relative-path` | Token-collapsed full-path is exact literal ZIP-root entry identity, not percent-decoded href or sorted name. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-mxl-media-type-default` | Known omitted media-type implies MusicXML; present empty token remains distinct and unavailable classification, not absence. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-opus-nesting-linking` | Ordered inline nesting plus explicitly selected linked graph, finite acyclic encapsulation/cycle-aware traversal as processing policy. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-opus-version-default` | Reuse accepted exact local version projection at nested occurrences without replacing original status. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-sounds-primary-direction` | sound/@primary is reverse library-to-MusicXML preference; alternatives and absence retained. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-sounds-any-primary` | any string mapping has forward preference; primary absence stays absent and alternatives remain plural. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-sounds-ensemble-primary` | ensemble text/number and forward primary; number stays NMTOKEN, not numeric ensemble size. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-sounds-solo-primary` | solo exact string and forward preference without exactly-one-primary strengthening. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-mxl-media-location-open` | Arbitrary permitted ZIP resource locations; no fixed parts/images/media folder requirement. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-xlink-actuate-behavior` | Project explicit/default onRequest, onLoad, other and none with bounded advisory trigger meaning. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-xlink-show-behavior` | Project new/replace/embed advice; other/none keep display choice application-dependent. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-opus-link-target` | Selected successfully resolved direct XML target must be an unqualified opus; invalid/inaccessible child and wrong-kind remain distinct. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-opus-score-target` | Selected direct XML target must be an unqualified score-partwise/timewise; intrinsic conformance and root kind remain independent. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-xlink-show-default` | Known absent exact shared use gets replace; explicit and invalid values retain provenance and raw/effective separation. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-context-image-resource-type` | Image source/type requiredness reused from XSD; type is media information, no suffix/sniffing or invented enum. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-instrument-link-target` | Reuse accepted local predicate and require exact scoped instrument membership in linked part resource. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-mxl-zip-format` | Bounded reviewed ZIP processing evidence only; no whole ZIP/JAR compliance certificate. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-mxl-container-path` | Exact META-INF/container.xml presence plus independently required original container source report. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-mxl-mimetype-stored` | Existing raw mimetype requires STORE and no encryption; independently retained local/central observations and early failures. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-mxl-mimetype-no-extra` | Known local extra is a violation, both absent is proved subset success, central-only remains explicit interpretation gap. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-mxl-mimetype-no-prefix` | Reviewed ASCII/NUL padding and all five standard Unicode BOMs; trailing content does not invent leading-prefix failure. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-mxl-first-root-musicxml` | First encoded rootfile identifies actual musical primary; XML/child validity separate, nested substitution unresolved. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-mxl-score-before-parts` | Exact authoritative role evidence with complete scope; minimal single-score inventory can prove antecedent false; unclassified roles cannot. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-mxl-rootfile-document-root` | Referenced MusicXML XML root is unqualified score-partwise/timewise/opus; known PDF/audio alternate not parsed as MusicXML. | Five-axis source routes recorded; final exact-head execution pending. |
| `CHECK-mxl-first-media-type` | Bounded case-insensitive registered direct MIME plus reviewed charset syntax, established non-MusicXML fails, unknown/empty/compressed interpretation unavailable. | Five-axis source routes recorded; final exact-head execution pending. |

## All selected engineering requirements

| Original ID | Exact batch 8 contribution |
| --- | --- |
| `REQ-DEF-XSI-SCHEMALOC` | Selected discovery-pair consumer in all four source assemblies plus explicit model unavailability; old zero/odd/even datatype acceptance unchanged. |
| `REQ-DEF-XSI-NONAMESPACE` | No-namespace anyURI hint as data only, no second pair rule/fetch/schema selection. |
| `REQ-DEF-CAT-METADATA` | Original IDs/provenance map to exact sealed package selectors and source-qualified intrinsic declarations. |
| `REQ-DEF-CAT-ROLES` | Normative predicates, capability discovery, context facts and optional advice have distinct class/role/category. |
| `REQ-DEF-CAT-OPERATORS` | Closed typed snapshot/package operations and causal prerequisites; no raw string DSL or executable resolver payload. |
| `REQ-DEF-CAT-OVERLAP` | Reuse XSD/local IDREF/shared defaults exactly once; package association/target/hint consumers add only their independent subclauses. |
| `REQ-DEF-CAT-MATRIX` | Preserve every original five-axis test and explicit target limits; concrete per-row evidence plus final-head acceptance required. |
| `REQ-DEF-PROFILE-CANDIDATES` | All 29 first-profile candidate dispositions retained; package-dependent portions routed without duplicating merged local obligations. |
| `REQ-DEF-LOCAL-RELATIONS` | Retain accepted exact local relation identity/causes and extend only exact linked association after local prerequisites. |
| `REQ-DEF-SCOPE-LOCAL-LINK` | Document/part/local typed target, adopted linked resource and package boundary remain distinct scopes. |
| `REQ-DEF-CONTEXT-OUTCOMES` | Known false antecedent differs from unknown/unsupported/invalid. Unsealed/partial expansion cannot certify. |
| `REQ-DEF-PROFILE-SCOPE` | Source/model graph and raw/current MXL scopes distinct; no current metadata reconstruction or full conformance claim. |
| `REQ-DEF-PACKAGE-ADAPTERS` | Raw/current/nested MXL, strict original container, source-only sounds, typed local link and resource graph adapters. |
| `REQ-DEF-EXTERNAL-RESOLVER` | Explicit selected dependencies, verified returned bodies/pins/authority, one account and one closure owner; complete external normative closure remains 9. |
| `REQ-DEF-RESULT-AXES` | Conformance, completion and operation error independent in package and canonical intrinsic reports; required failures never flattened. |
| `REQ-DEF-DIAG-METADATA` | Offline rule/source/subject/reason/cause metadata, exact profile/digest and scoped entry/document positions. |
| `REQ-DEF-DIAG-CATEGORIES` | Syntax/representation/technical policy/resource/capability/normative/advisory separate across raw loader and graph. |
| `REQ-DEF-DIAG-COMPAT` | Legacy errors.Is/As, ErrMXLTooLarge/ErrInvalidMXL and MXLLinkError compatibility; new resource incomplete never silently becomes normative invalid. |
| `REQ-DEF-DIAG-BOUNDS` | No fabricated source coordinates for current model/package or unrelated cause borrowing; bounded related locations and fallback summary. |
| `REQ-DEF-SEC-NOFETCH` | No automatic HTTP/filesystem/DTD/entity/XInclude/schema-hint retrieval; only selected resolver branches after exact local prerequisites. |
| `REQ-DEF-BUDGET-ACCOUNT` | One validated root account before I/O, shared all adapters/children/caches, finite unchanged defaults and explicit byte-owner modes. |
| `REQ-DEF-BUDGET-INPUT` | Compressed/raw caller input charges InputBytes once; caller reader remains open. |
| `REQ-DEF-BUDGET-UNPACKED` | All decompressed container/mimetype/XML bytes and transferred bodies charge ExternalBytes; preserve five independent MXL caps. |
| `REQ-DEF-BUDGET-BUFFER` | Admit compressed retention, inventories, buffers, context snapshots, working copies and canonical caches before allocation; zero gauges after finish. |
| `REQ-DEF-BUDGET-DEPTH` | Shared finite active XML/model depth plus aggregate link/encapsulation chain, cycle-safe without new normative depth bound. |
| `REQ-DEF-BUDGET-EVENTS` | Source/model/auxiliary child node counts under the same account; no reset per document. |
| `REQ-DEF-BUDGET-ATTRS` | All child source/model attributes under same account, including auxiliary sources. |
| `REQ-DEF-BUDGET-IDS` | Exact schema-ID index, all duplicate owners, resource/adoption identity and bounded lookup storage. |
| `REQ-DEF-BUDGET-MATCH` | Every graph/index/URI/metadata copy and provider/core scan accounted and cancellation-polled. |
| `REQ-DEF-BUDGET-CONTEXT` | Resource/container/sounds/link/association/image/intent facts and publication all quota-admitted. |
| `REQ-DEF-BUDGET-LINKS` | Each selected occurrence including cache/cycle hits and unresolved targets charges linked admission; canonical parse once and base-specific expansion. |
| `REQ-DEF-BUDGET-DIAGS` | Shared count/bytes, known failure survives truncation/snapshot export refusal, no report erasure. |
| `REQ-DEF-SEC-CANCEL` | Cancellation before/between bounded work, all transferred streams closed exactly once with joined causes; no force-cancel goroutine for arbitrary blocked I/O. |
| `REQ-DEF-VERIFY-CI` | Final exact-tree seven stages, actual cross-platform CI, 386 execution, real xmllint and all fuzz targets; focused earlier passes insufficient. |
| `REQ-DEF-VERIFY-DIFFERENTIAL` | Original-byte container/sounds vs real pinned-schema xmllint; ZIP/association/roles use independent exact evidence, not XML oracle or Encode. |
| `REQ-DEF-VERIFY-PROFILE` | Full preserved obligation matrix with tested limitations; frozen eight reports/profiles, target-specific readiness and finite-default workloads. |
| `REQ-DEF-DEC-API` | New option structs/entry points; no field additions to frozen AssessmentOptions/DecodeOptions/MXLOptions/Report. Document additive ContextResult shape compatibility. |
| `REQ-DEF-DEC-CATALOG` | Sealed separate package catalog/profile IDs, actual intrinsic digest pinning, no relaxed XML root/selector invariants. |
| `REQ-DEF-ISSUE-CONTAINER-SCHEMA-EXISTS` | Proposed disposition: treat the tutorial statement as outdated; container.xsd is part of version 4.0. Before implementing the affected disputed clause, approve its interpretation, scope, evidence and selector; update that clause test contract. This contextual issue association alone neither blocks other clauses nor authorizes hard rejection. Final issue/status reconciliation remains explicit where this row carries a research gate. |
| `REQ-DEF-ISSUE-CONTAINER-SOUNDS-WORDING` | Proposed disposition: do not allow sounds as the container-document root; the explicit structure and the second annotation agree. Before implementing the affected disputed clause, approve its interpretation, scope, evidence and selector; update that clause test contract. This contextual issue association alone neither blocks other clauses nor authorizes hard rejection. Final issue/status reconciliation remains explicit where this row carries a research gate. |
| `REQ-DEF-ISSUE-CONTAINER-SCHEMA-URI` | Proposed disposition: record a probable editorial error; use the pinned container.xsd without substituting the sounds schema. Before implementing the affected disputed clause, approve its interpretation, scope, evidence and selector; update that clause test contract. This contextual issue association alone neither blocks other clauses nor authorizes hard rejection. Final issue/status reconciliation remains explicit where this row carries a research gate. |
| `REQ-DEF-ISSUE-MXL-METHODS-COMPATIBILITY` | Proposed disposition: do not claim complete ZIP validation from these paragraphs; the external ZIP/JAR corpus needs review. Before implementing the affected disputed clause, approve its interpretation, scope, evidence and selector; update that clause test contract. This contextual issue association alone neither blocks other clauses nor authorizes hard rejection. Final issue/status reconciliation remains explicit where this row carries a research gate. |
| `REQ-DEF-ISSUE-MXL-FIRST-MEDIA-TYPE-SET` | Proposed disposition: check a direct XML root separately; clarify the set of permitted media types rather than inventing equality to a single MIME value without explanation. Before implementing the affected disputed clause, approve its interpretation, scope, evidence and selector; update that clause test contract. This contextual issue association alone neither blocks other clauses nor authorizes hard rejection. Final issue/status reconciliation remains explicit where this row carries a research gate. |
| `REQ-DEF-LANG-291` | Reuse completed 177+114 linguistic triplets; preserve original planned fields and all Russian/source provenance; zero new first-time selected translations. |

## Five original issue gates

- `MX40-ISSUE-container-schema-exists`: editorial disposition supported by pinned formal assembly; original decision_required and hard_error_basis:false preserved. Owner: batch8 formal assembly; batch9 final research reconciliation.
- `MX40-ISSUE-container-sounds-wording`: editorial disposition supported by pinned formal assembly; original decision_required and hard_error_basis:false preserved. Owner: batch8 formal assembly; batch9 final research reconciliation.
- `MX40-ISSUE-container-schema-uri`: editorial disposition supported by pinned formal assembly; original decision_required and hard_error_basis:false preserved. Owner: batch8 formal assembly; batch9 final research reconciliation.
- `MX40-ISSUE-mxl-methods-compatibility`: bounded implementation with exact unresolved residual; no full issue closure. Owner: batch8 bounded subset; batch9 full external closure.
- `MX40-ISSUE-mxl-first-media-type-set`: bounded implementation with exact unresolved residual; no full issue closure. Owner: batch8 bounded subset; batch9 full external closure.

## Exact later owners and residuals

- `CHECK-mxl-zip-format`, `CHECK-context-mxl-deflate`, `MX40-ISSUE-mxl-methods-compatibility`: batch9 external ZIP/JAR/DEFLATE normative closure. Exhaustive external feature/method/JAR interpretation. Bounded raw observer is implemented separately; no unconditional required Unknown for every ordinary supported archive.
- `CHECK-mxl-mimetype-no-extra`: batch9 external header interpretation. Central-only extra fields after independently proved zero local extra. Known local extra and safe both-zero subset remain batch8 duties.
- `CHECK-mxl-mimetype-no-prefix`: batch9 broader byte-prefix interpretation. Padding/encoding families outside reviewed ASCII SP/HT/LF/VT/FF/CR, NUL and five standard BOMs. Required bounded classifier implementation remains batch8; selected-resource ZIP/XML prefix classification is a separate adapter contract.
- `CHECK-mxl-first-media-type`, `MX40-ISSUE-mxl-first-media-type-set`: batch9 complete media-type set/parameter interpretation. Unreviewed aliases, parameters and complete permitted set; not an excuse to omit supported direct types or established non-MusicXML negative.
- `CHECK-mxl-first-root-musicxml`, `CHECK-mxl-rootfile-document-root`: batch9 outer compressed-root substitution interpretation. Technical nested archive traversal does not prove outer direct-rootfile normative validity; retained Unknown is required when this branch is reached.
- `CHECK-context-link-element-position`, `CHECK-context-xml-reserved-semantics`, `REQ-DEF-EXTERNAL-RESOLVER`: batch9 general XPointer/XML Base/DTD/entity/XInclude closure. Beyond reviewed exact fragment/local-attachment and bounded URI semantics. No automatic external fetch or missing position=1 interpretation.
- `CHECK-context-sounds-standard-identifiers`, `CHECK-context-stopped-smufl-default`, `CHECK-context-open-smufl-default`, `CHECK-context-half-muted-smufl-default`, `CHECK-context-tremolo-smufl-condition`: batch9 canonical sounds/SMuFL/IPA dictionaries and extensions. Accepted local default/name projections and four source sounds mapping directions remain separate; open string domains are not closed enums.
- `CHECK-instrument-link-target`, `CHECK-mxl-score-before-parts`, `B6-SOURCE-SOUND-INSTRUMENT-STATE`: batch8 typed provider capability; batch9 external correspondence/interpretation closure. Absent authoritative instance mappings/roles remain unavailable. Exact usable role-provider root wiring is active batch8 work; identical IDs/names never infer correspondence.
- `REQ-DEF-XSI-SCHEMALOC`, `REQ-DEF-XSI-NONAMESPACE`, `REQ-DEF-XML-DOCTYPE`: retained source/model representation limitations; batch9 final capability reconciliation. Source external DOCTYPE incompleteness and discarded current-model original hints remain explicit. Model source history is never reconstructed.
- `B5-PERF-01`: batch9-kernel-session-final-reconciliation. Finite unchanged default limits, broad kernel/session performance and unproved workload scaling. Old batch7 timings do not certify new graph/package usability; new bounded workload evidence is independent.

## Inherited split obligations

All 20 split records are routed to their unchanged batch 7 ownership objects, not condensed into a new completed flag. Their batch 6 contributions, batch 7 consumers, exact remaining subclauses and later owner lists remain authoritative. Both B6 source-only subclause records remain intact; instrument-state linkage adds only exact association and does not claim playback or dictionary closure.

## Historical freeze and acceptance procedure

1. Freeze the exact final commit and tree after the integrated semantic/resource evidence is reviewed; no active code omission is identified by this inventory.
2. Regenerate this additive route proposal and run the inventory verifier; recheck changed test hashes. Do not rerun or rewrite completed linguistic/original records.
3. Retain the independent source/model/raw/current/nested semantic and resource acceptance pinned at production 388dbe0. Any later production change needs an affected-scope recheck, with exact command, exit status, head/tree, environment and hashes.
4. Preserve exact eight-profile equality and refreshed finite-default usability evidence at 388dbe0 independently of B5-PERF-01; the full final suite must still cover unchanged legacy/API behavior.
5. Execute CONTRIBUTING mod, generate, format, test, vet, race and fuzz stages; require actual real-xmllint oracle and 386 execution plus all nine remote exact-head CI jobs.
6. Only the lead may record final acceptance after all gates settle. Provenance/route verification alone never changes readiness.
