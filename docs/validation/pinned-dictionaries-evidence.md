# Pinned dictionary verification checkpoint

Date: **14 October 2026**. Canonical base:
`a8e7a10cdf7e34fad979a758c1ba481c74f579fe`, tree
`c54f0a190e24687f162d1851bf46dd08147de473`.
Current documented local integration checkpoint:
`548734f80fef2c5de9997914a8ab47d1c3baac65`, tree
`7dae9557dde0f27d247c9eb0daadec43ff199935`.

**These are scoped, dated observations, not final integrated acceptance.**
The first integrated ordinary full-suite run passed at earlier `1209584`.
Consumer, ingress and graph reviews have separately bounded accepted results.
Independent integrated resource/profile-budget review also accepted its scoped
contracts at `229e4fd`; final frozen-tree seven-stage checks, eleven counted fuzz targets, inventory
controls, actual 386 execution and exact published-head CI remain pending.
There is no PR/release or published-head CI acceptance receipt for this slice at
this checkpoint. Local and future published commits must be mapped by exact
tree and parent lineage rather than assumed to have identical commit IDs.

The [scope and contribution crosswalk](pinned-dictionaries-scope.md) records
original IDs, exact subjects, remaining interpretation gates and owners. No
original planning/status/denominator/source record was changed to manufacture
coverage. A successful importer or selected family test does not retire the
original full IPA, external-spec, language-portability or final-program gates.

## Read-only baseline and compatibility comparisons

Before implementation, the audit captured canonical `a8e7a10` with a read-only Go
source overlay and Go 1.27.2. It retained the actual inputs and full parsed report
objects rather than only a list of expected pass/fail labels:

- All **16 existing profiles**, including exact catalog digests, required rules,
  rule metadata, schedules, support and package intrinsic bindings.
- **35 source/current-model/auxiliary reports**, with JSON shape, operation
  errors and satisfaction outcomes, plus all finite default budgets.
- **Four graph/MXL bundles** covering original XML, current model, original MXL
  and current MXL, including resource reports and adoption contexts.
- Exact schema-use/type/identity dumps and fifteen original-source actual-type
  witnesses, supporting the 27 declared SMuFL uses, ten assessed SMuFL components
  and 35 dynamically reachable string declaration inventory.

The author consumer/graph comparisons preserved these inputs. The independent
consumer review repeated the comparison at frozen consumer commit
`b42ef5dbba276159423bacd7016af7882b8df920`, tree
`04768738675f5c27538dd466ff7031d3280c2477`:

- Profile/schedule/support/intrinsic metadata, all 35 reports, default budgets,
  source inputs and dynamic-type witnesses were **parsed-exact equal**.
- All four bundles were equal after **only a bijective replacement of the
  existing random 32-hex per-run resource namespace nonce**. Occurrence suffixes,
  ordering, multiplicity, paths and catalog digests were preserved. Raw bundle
  JSON is not claimed byte-identical; the initial raw comparison correctly
  reported those nonce differences before the separate normalized comparison.

`ContextResult.Dictionary` is omitted when nil. Owned-payload and old-JSON tests
cover that additive field, while [API.md](../../API.md#evidence-and-compatibility)
explicitly documents the source migration required for external unkeyed literals.
These baseline comparisons are historical checkpoint evidence, not a substitute
for rerunning compatibility checks on the eventual integrated/final head.

## Independent source-data review

The earlier 35 successfully acquired research artifacts were rechecked against
their recorded byte lengths and SHA-256 without replacing their original records.
SMuFL 1.4 and MusicXML 4.0 raw/typed identities were independently reconstructed
with a standard-library Go verifier. The exact runtime pins are in
[`dictionary_manifest.go`](../../validation/dictionary_manifest.go); fixture tests
retain source bytes, line endings and the sounds external DOCTYPE.

The independent IPA reviewer accepted **only the explicitly partial
positive-data/provenance scope**. A separate Go verifier, not the author generator,
passed **71,993 assertions** over the immutable data snapshot. It verified all
170 symbol records, 18 exact examples, nine canonical-equivalence inputs, source
pins, typed serialization, preserved notices and all 25 documented mapping-aid
omissions. Chart visuals and explicit written mapping fields were reviewed;
PDF glyph/font codes were not treated as Unicode identities. The typed binary
was independently reproduced as 9,836 bytes with SHA-256
`47b0a8812835e56e0f8b6527cca13b6d55bb4b3ee231186868e8ec8590804ec6`.

The review retained the separable CC-BY-SA-4.0 data disposition, historical chart
CC-BY-SA-3.0 attribution and Unicode notice caveats. It did not certify a complete
repertoire/sequence grammar, original-source acquisition during a runtime run,
legal sufficiency for every downstream distribution, or final release packaging.
Unlisted input stays unknown. See the [four retained IPA gates](pinned-dictionaries-scope.md#four-retained-ipa-gates)
and [source licenses](pinned-dictionaries-scope.md#source-licenses-and-distribution-boundary).

## Test-first implementation and scoped reviews

### Core and root importer

Recorded RED/green checkpoints cover API scaffolding separately from behavioral
failures. They include dictionary lifecycle and manifest contracts; empty-set
scratch admission; preservation of ordinary Begin's cancellation precedence;
root body transfer/read/close failures; and terminal failure after an earlier
component succeeds. A missing-symbol compile failure is not presented as a
behavioral defect reproduction.

Independent ingress/resource review accepted core
`22364f5f06dcc687945ea2741e4628cec1df6cce` plus exact correction
`2ff8df03c0e459195083ee8a2579d15ddb359629`, and importer
`aa067dd805461a30ca9015328d3793664305ba8b` within that lane. Core and importer
focused tests, hostile-data overlays and race runs passed. The final isolated
corrected-core overlay race run passed in 2.481 seconds. It did not contain the
later integrated wrappers/consumers/graphs or establish whole-tree acceptance.

The independent finding was host-width addition of dictionary comparison lengths
before widening. An allocation-free int32 arithmetic probe reproduced the fault;
`2ff8df0` widens operands separately and checks the int64 sum, retaining cancellation
precedence. This probe is not an actual 386 execution. Other controls independently
mutated valid certified typed data, certificate accounts/components, exact request
fields, component ordering and cancellation points. A certificate never bypassed
the independent transformed-data identity check.

The verified contract includes same-account borrow/Close linearization, active
borrowers surviving owner close, exact one-time storage release, failed-Begin
cleanup, owned getters and no input aliases. Root imports call only selected
components once, close every transferred usable body once, preserve ordinary
unavailable components and stop terminal cancellation/resource failures. The
pinned sounds transform neither strips nor fetches its external DTD.

### Consumer semantic/API review

The final independent consumer lane accepted frozen `b42ef5d` (full ID and tree
above), covering **67 top-level focused tests**, including **nine independent
test groups**, with exit 0 on the root and `validation` packages. Its read-only
archive included original consumer work, independent primary-codepoint/resource
supplements and the scalar-output correction. The lane did not include the
lead's separate core host-width correction, importer/graph wrappers, full race,
fuzz, actual 386 or CI; those remain separate evidence boundaries.

The independent reviewer found a derived-scalar cap bypass: with
`MaxScalarBytes=14`, an absent half-muted SMuFL attribute exposed the 19-byte
`brassMuteHalfClosed` default. Because the outer generic value was absent, the
compound payload's retained-size accounting had not enforced that scalar limit.
The preserved RED is followed by a fix checking `Input`, `Effective`, IPA span
text and canonical-equivalent text before cloning. Independent source/model
controls verify **19-byte exact admission succeeds and 18-byte one-under fails**
terminally without an oversized visible projection. Fixed metadata is not
misclassified as user scalar data. The integration equivalent is `229e4fd`.

The independent coverage includes the exact binding census; all 27 direct uses,
35 dynamic carriers and ten actual types; six independent family-failure matrices;
failed nominations and causal scalar results; assembly/declaration homonyms;
empty/absent/whitespace/case/codepoint boundaries; exact range advice; partial IPA
and ENTITY boundaries; separate primary-codepoint selection; retained known
nonmembership on later resource refusal; owned nested payloads and occurrence
remapping; and distinct actual-capability cache identities. This does not claim
an exhaustive per-context source/model fixture matrix.

A later test-only author supplement,
`4245de51cb8e84b0fe1a2736bc17dead772aae67` (tree
`5fe464536c6a74b2a00e4093e8048aa7ce2cd234`), adds four explicit dictionary-profile
groups and **52 source/model × dictionary-availability leaf cases** for symbol
defaults/unavailable specialized-markup advice, glyph limited positives/open types,
arrow structure/unavailable description agreement, and inverted-mordent defaults.
All passed on the first run: this is an honest coverage addition, not RED evidence
or a production-fix claim. The subsequent focused dictionary plus legacy-glyph
run passed root/validation in 2.233/0.196 seconds. The supplement postdates the
67-test independent consumer freeze. Parent static review accepted the addition,
integrated as `548734f`; final whole-tree verification is still required.

### Graph propagation and integrated checkpoints

Graph author tests pass the exact intrinsic-profile table, original/current
representations, one import through aliases and nested packages, source-only
sounds, role/depth retention, context/cause remapping, actual capability cache
identity, unavailable dictionaries with independent family failures, terminal
cleanup and absence of implicit internal-DTD composition. Focused graph race
checks also passed. Parent static/follow-up review accepted the graph correction
`42b4322`, including typed-nil input rejection before provider work. These scoped
checks do not imply final end-to-end acceptance.

At integrated `1209584ee56930b9475c27f1820025ecc1bf42ae`, the first complete
`go test -count=1 ./...` run passed all packages. The recorded root package time
was 84.796 seconds and validation 16.127 seconds; these are shared-host observations,
not controlled performance claims. Later integrated corrections are:

- `42b4322`: reject typed-nil dictionary graph input before provider work.
- `8f5a06b`: independent primary-codepoint evidence, causal/resource boundary
  regressions and actual-capability snapshot identity controls.
- `229e4fd`: scalar admission for derived/nested dictionary projections.

That earlier full test result does **not** certify these later changes. Independent
integrated resource/profile-budget review subsequently accepted its exercised
scope at `229e4fd`, with no implementation defect found. It covers all seven public
wrappers with four real imported components, ownership through foreign-account
refusal/owner close, XML/nested-MXL aliases, fourth-component terminal cleanup,
unavailable canonical evidence alongside independent family failure, typed-nil
preflight and 51 post-import same-account quota/refusal controls. Targeted race
confirmation passed; it did not repeat the adjacent-boundary binary search under
race. The initial reviewer harness failures are explicitly recorded as fixture,
accounting and bundle-expectation mistakes, not product RED evidence.

The lead advanced during the race command to `548734f`, whose sole change is the
new rendering test file; production bytes were unchanged. Logs retain the command's
starting head rather than retroactively claiming all execution began at the later
head. Final frozen-head whole-suite checks remain pending. Local 386 attempts returned `exec format error`; successful
cross-compilation of root/validation test binaries is compile-only evidence.
Actual 386 execution must be established by the appropriate final Linux CI run.

### Default-budget observations

An independent Go overlay at integrated `229e4fd`, using Go 1.27.2 Linux/amd64,
`GOMAXPROCS=2` and `GOFLAGS=-p=1`, imported all four exact components (460,617 raw component bytes) into the same
finite account used by each assessment. The fixture is `resourceScore` containing
repeated rest notes, each with duration=1, type=quarter and
`accidental smufl="accidentalSharp"` text `sharp`. It has no IPA element; loading
the IPA component does not assert an IPA obligation is exercised. No DOCTYPE
stripping or budget increase was used. This is a schema-valid synthetic fixture,
not an official corpus score or a general supported-capacity measurement.

`TestIndependentDictionaryAdjacentDefaultBoundaries` binary-searched and then
repeated both adjacent cases for each representation. The independent results
are:

| Representation | Last operationally complete notes | First rule-step refusal | Completion meaning below the boundary |
| --- | ---: | ---: | --- |
| Standalone source | 320 | 321 | Input/assessment complete and report satisfied |
| Standalone current model | 320 | 321 | Assessment complete and report satisfied |
| Source resource graph | 316 | 317 | Enumeration complete and bundle satisfied |
| Current-model resource graph | 316 | 317 | Enumeration complete; bundle still unsatisfied by existing schema-hint unknown |
| Original raw MXL | 316 | 317 | Enumeration complete and bundle satisfied |
| Current MXL | 316 | 317 | Enumeration complete; bundle still unsatisfied by existing schema-hint unknown |

All refusals reached the **unchanged 20,000,000-rule-step limit** and remained
incomplete/resource outcomes, not invented normative failures. At 320 notes the
standalone source was only 37,327 bytes, illustrating why input size alone is
not a useful cost guarantee. One-note source/model runs already consumed about
5.08 million steps with all four imports. Component selection and the entire
profile/representation contribute to work; these counts do not isolate kernel,
importer, lookup or model costs, and are not a controlled optimization comparison.
Cleanup checks found zero remaining live-buffer/depth gauges after finalization.

The current-model graph/MXL limitation is the existing
`schema-discovery-pairing-unavailable` package capability result, not dictionary
nonmembership. Do not call these complete bundles satisfied. The fixture-specific
320/321 and 316/317 boundaries are historical observations at `229e4fd`, not
universal note ceilings, guaranteed capacities, raised defaults, full profile
acceptance or B5-PERF-01 closure. The scoped integrated resource-review receipt accepts these exercised contracts;
broader frozen-head acceptance remains a separate gate.

## Executable routes and five-axis interpretation

These routes are additive evidence, not replacements for original planned test
contracts. Every obligation still needs its positive, negative, boundary,
missing-fact and interaction axes considered against its actual semantics.
For partial IPA, a negative control must not invent a normative repertoire
failure: it establishes unknown or a distinct genuine XSD/model failure.

| Boundary | Representative committed routes |
| --- | --- |
| Raw pins, exact transform and hostile typed input | `TestPinnedMusicDictionarySourceBytes`; `TestDictionaryReviewedMusicDigestsAndOwnedSortedRecords`; `TestDictionaryCertificateCannotAuthorizeAlteredRecordsOrForeignAccounts`; `TestDictionaryHostileTypedIngressIsBoundedBeforeCopy` |
| Known/missing/unsupported identity and exact relation selection | `TestDictionaryConstructorRejectsContradictoryAndDuplicateImports`; `TestDictionaryCapabilityFingerprintPreservesActualSelectionStateAndLineage`; `TestDictionaryUnknownRelationsAndInputAliasesRemainIndependent`; `TestDictionaryIndependentPrimaryCodepointRelation` |
| Account/lifetime/concurrency | `TestDictionaryCloseDefersStoreReleaseUntilAllBorrowersFinish`; `TestDictionaryBeginRequiresSameAccountAndReleasesFailedBorrow`; `TestDictionaryConcurrentBeginAndCloseHaveOneLifetime`; `TestDictionaryConcurrentImmutableReadsKeepStoreUntilLastSession` |
| Exact/one-under budgets and cleanup | `TestDictionaryTypedConstructionExactWorkRetentionAndScalarBudgets`; `TestDictionaryPartialConstructorFailurePreservesCallerAndCumulativeWork`; `TestDictionaryVerifierExactBudgetsCorruptionAndFinalCancellation`; `TestDictionaryProjectionScalarCapsCoverDerivedValues`; `TestDictionaryDefaultOutputScalarBudgetIsTerminal` |
| Bounded source transport and terminal-after-success | `TestDictionaryComponentBodyExactLengthAndProbe`; `TestDictionaryComponentBodyReviewOwnership`; `TestDictionaryImportProviderFaultOwnership`; `TestDictionaryImportSuccessfulComponentBeforeTerminalStop` |
| Exact source/model scalar subjects and independent XSD | `TestDictionaryAllTwentySevenDeclaredUses`; `TestDictionaryAllTenAssessedSMuFLTypes`; `TestDictionaryAllThirtyFiveDynamicStringCarriers`; `TestDictionaryDynamicAttributeOwnerAndDiscardedNomination`; `TestDictionarySixFamilyFailuresAndBadNominationStayCausal` |
| Defaults, activation and advisory ranges | `TestDictionaryDefaultsWithoutDataAndRangeAdvice`; `TestDictionaryInactiveRenderingAttributesStillCanonical`; independent `TestIndependentDictionaryRangeAdviceBoundaries` |
| Explicit narrow rendering semantics | `TestDictionarySymbolDefaultsAndUnavailableAdviceSourceModel`; `TestDictionaryGlyphLimitedPositivesAndUnavailableAdviceSourceModel`; `TestDictionaryArrowLocalDescriptionAndUnavailableAdviceSourceModel`; `TestDictionaryInvertedMordentLocalLongDefaultSourceModel` |
| Partial IPA and open standard sounds | `TestDictionaryIPAPositiveNeverClosesSequence`; `TestDictionaryENTITYAndSemanticUseRemainIndependent`; `TestDictionarySoundsCustomAllowedAndIPAGate`; `TestDictionaryImportDerivedBundleSourceKind` |
| Previous proof survives later terminal refusal | `TestDictionaryKnownNonmembershipSurvivesSecondaryLookupRefusal`; `TestDictionaryKnownNonmembershipSurvivesProjectionRefusal`; `TestDictionaryAssessmentOrdinaryImportFailurePreservesXSD`; `TestDictionaryGraphProviderErrorKeepsIndependentFamilyFailure` |
| Owned report/snapshot/cache occurrence evidence | `TestDictionaryProjectionOwnsNestedEvidence`; `TestDictionarySnapshotGetAndOccurrenceRemapOwnNestedCauses`; `TestDictionaryPackageSnapshotsDoNotAliasActualCapabilities`; `TestDictionaryGraphAdoptionsRemapNestedCausalEvidence` |
| Selected graph/MXL and no hidden I/O | `TestDictionaryGraphProfilesPinExactIntrinsicTable`; `TestDictionaryGraphKnownSetReachesEveryPrimaryRepresentation`; `TestDictionaryGraphNestedAliasPackagesShareOneVerifiedSet`; `TestDictionaryGraphUnselectedSourceNeverCalled`; `TestDictionaryGraphDoesNotEnableInternalDTD`; `TestDictionaryGraphTypedNilBeforeProvider` |
| Old JSON/profile isolation | `TestDictionaryNilAndUnusedAttachmentPreserveOriginalReportJSON`; `TestDictionaryProjectionOmittedFromOldJSON`; original frozen-profile tests plus the external 16-profile/35-report/four-bundle captures above |

The original five-axis contracts remain in `plan/implementation-plan.json`,
`plan/prose-contracts.json` and `plan/context-dispositions.json`. This table does
not claim every case has an independent oracle or exhaustive input coverage.
The scope crosswalk distinguishes the later explicit rendering supplement from
older canonical-slot coverage and the earlier independent-review freeze.

## Retained external receipts

The following receipt names refer to retained local review artifacts, not files
published in this repository. Their hashes identify the reviewed snapshot and
prevent a later log from silently replacing this checkpoint. They are not direct
repository download links or a claim that final CI can retrieve a local artifact.

| Receipt | SHA-256 |
| --- | --- |
| `base consumer-ledger.json` | `1cdf7eaea060d35c534396a32a1df1c9e7763f56b0e387aa0fb6a638e18b116e` |
| `IPA INDEPENDENT_REVIEW.md` | `1a7d8f97c011b71d3aef3520bdfc41c00d6bb11868788ae838b62620f0e36edb` |
| `IPA verification-go.txt` | `c8cdb4848cf854b5e39db710ba1fddd966a80492006f39b15fa6affa102ad466` |
| `ingress REVIEW.md` | `c695d6f2bcd9ef69ba81b7d806a0ac457c2b0fb4eb4eb9c3d32fc755aef9c312` |
| `ingress core-fixed-overlay-race.log` | `b695c59a92fbd6825335e2ab6095e6627f8d2d5213be73311c8246cb4ddc15a3` |
| `consumer READOUT.md` | `b372783465da178be1c3991f5f5fdd54b11b2f262edc9b199bef9dfc95241b9c` |
| `consumer acceptance.json` | `dfb62814312f646822a56522c601e6fd0a17c50d8181ace59ffbf28bf80ab723` |
| `consumer final-baseline-comparison.json` | `b8fc367051dd1d71d3ade30fe3c56c7f234c96fdf59aa4e512331676af4a4d4b` |
| `consumer final-independent-focused.log` | `f2ceab22b64fb6bdc84500a49a410541b46a23b479b1c156380408beb401d218` |
| `1209584 first-full.log` | `73b53725a6a5a2c0911aa2088d4d15acf564fa6df91041018c0ce1cba4872d82` |
| `graph 11-integrated-graph-race.log` | `57d79daa6ac0c531bc08b8706ab554db53e0170aae228991090b036f6683d8bb` |
| `graph 12-integrated-graph-386.log (exec-format refusal)` | `80037887427de8a93290b1e2d8a3e429fcd6688ea45372caeef75d00362b1b09` |
| `consumer 24-local-rendering-source-model-first.log` | `29661e591ce5714ab58489dbc7505ad485f3d527be4fb9acab9a9dfa68d87439` |
| `consumer 25-supplemental-and-existing-green.log` | `c9d02a0f78bb880459b64399ca08cff8f0b77ff1b78797acea9b5ce2aa94715f` |
| `integrated resource independent-integrated-review.log` | `38b9f0745bcc948fdc3f20c2d86581b91e3d6489884aa3b22c944e100f499b8f` |
| `integrated resource independent-account-refusals.log` | `5dc0d56923ffcfed6d4f9e78c0520b100146eafdd32e416511d4187495e92e4d` |
| `integrated resource integrated-focused-race.log` | `5b16b9971f8175e05b47a18092df14f322e0bb53b90c724dea15c90f56dfa2b5` |
| `integrated resource REVIEW.md` | `090b791c2be1b4c924635800d8e128506b0ac414d85c74dad6d4fc5c810d5ce6` |

## Outstanding acceptance gates

Follow [CONTRIBUTING.md](../../CONTRIBUTING.md) on the final frozen integration
head: module tidy, reproducible generation, formatting, full ordinary tests, vet,
full race and all eleven counted fuzz targets. Retain their default budgets and
counts. Run strict package inventory verification and its Go control suites;
only reviewed current routed-test hash refreshes are permitted. Original source
records, record hashes/statuses, denominators, source pointers and test-route
identities must remain intact.

Final hosted verification must cover the existing platform jobs, real xmllint,
actual `GOARCH=386` execution and both exhaustive race shards with their aggregate
gate. Preserve exact commit/tree/run evidence and distinguish local author IDs
from authenticated published equivalents. Further corrections require their own
exact-head reruns and independent review where applicable. None of these gates
is waived by this documentation or a passing focused suite.
