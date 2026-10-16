# Final configuration and public-entry-point contracts

## Scope and decision

This is the retained, bounded configuration coverage record for the final PR,
starting from merged main `0b5a2f24b89ef628d2d09b361cbcbe390b9f01b9`
(tree `1e78190afb839ba0cd5b91bc5fb8bff6a102638d`). It inventories existing
substantive tests before adding missing public-wrapper cases. It is not a
real-corpus run, performance assessment, independent acceptance review, or a
claim that every document can be completely assessed within the defaults.

The supported use cases are expressible with the existing target-specific
options. The coverage additions passed against unchanged production code; no
configuration defect was demonstrated, no baseline failure is claimed, and no
API redesign, constructor reduction, profile widening or default increase is
needed for this item. Bundled pinned-data delivery is an additive source
implementation, not a change to selection or missing-capability semantics.

## Entry-point / profile / options matrix

Names below are exported Go constants, not strings callers must reconstruct.
The profile is always explicit when assessment is requested. A profile describes
its bounded interpretation; its existence does not certify universal MusicXML
conformance. All examples/tests named in this record are executable Go tests.

| Input and purpose | Public entry point and options | Compatible explicit selection | Existing executable route |
| --- | --- | --- | --- |
| Original MusicXML XML, inspect without a model | `InspectXML`, `AssessmentOptions` | A source profile, e.g. `ProfileSourceXSDPreview`, `ProfileSourceLocal`, `ProfileSourceLinks`, `ProfileSourceTimeline`, `ProfileSourceContext`, or `ProfileSourceSemantics` | `ExampleInspectXML`; `TestSemanticCompositionPublicProfiles` covers partwise/timewise source and model semantic selection |
| Original XML, convert and optionally assess | `DecodeAssessed`, `DecodeAssessmentOptions` | `Assessment == nil` requests permissive conversion; nonnil `Assessment.Profile` must be a source profile compatible with the root | `ExampleDecodeAssessed`; `TestDecodeAssessedPermissiveAndStrictSourceAreSeparate`; `TestDecodeAssessedSourceAndConversionDomains` |
| Current Go model | `AssessModel`, `AssessmentOptions` | `ProfileModelXSD`, `ProfileModelLocal`, `ProfileModelLinks`, `ProfileModelTimeline`, `ProfileModelContext`, `ProfileModelDictionaries`, or `ProfileModelSemantics` | `ExampleAssessModel`; `TestAssessModelCurrentTargetAndRootMetadata`; `TestSemanticCompositionPublicProfiles` |
| Original XML with selected dictionaries | `InspectXMLWithDictionaries`, `DictionaryAssessmentOptions` | `Assessment.Profile`: `ProfileSourceDictionaries`, `ProfileSourceInternalDTDDictionaries`, or `ProfileSourceSemantics`; source sounds variants below | `TestDictionaryAssessmentExplicitProfilesAndConversion`; `TestSourceDeclarationCompositionProfiles`; `TestSemanticCompositionPublicProfiles` |
| Convert original XML with selected dictionaries | `DecodeAssessedWithDictionaries`, `DecodeDictionaryAssessmentOptions` | Nil `Assessment` is unassessed; otherwise the same compatible source dictionary/semantic profiles, with a supported document root | `TestDictionaryAssessmentExplicitProfilesAndConversion`; `TestIPARepresentationPublicSourceModelAndAssessedReader` |
| Current model with selected dictionaries | `AssessModelWithDictionaries`, `DictionaryAssessmentOptions` | `Assessment.Profile`: `ProfileModelDictionaries` or `ProfileModelSemantics` | `TestDictionaryAssessmentExplicitProfilesAndConversion`; `TestDictionaryCanonicalSourceModelAndIndependentFamily` |
| Linked original XML / current model | `InspectXMLWithResources` / `AssessModelWithResources`, `LinkedAssessmentOptions` | `Graph.Profile = ProfileResourceGraph` | `ExampleInspectXMLWithResources`; `TestResourceGraphPrimaryReports` |
| Linked XML / model with selected dictionaries or composed semantics | `InspectXMLWithResourcesAndDictionaries` / `AssessModelWithResourcesAndDictionaries`, `LinkedDictionaryAssessmentOptions` | `Resources.Graph.Profile`: `ProfileResourceGraphDictionaries`, `ProfileResourceGraphInternalDTDDictionaries`, or `ProfileResourceGraphSemantics` | `TestDictionaryGraphEntryPointsKeepRepresentationAndProfiles`; `TestDTDPackageCompositionEntryPoints`; `TestSemanticPackageCompositionEntryPoints` |
| Original raw MXL bytes | `InspectMXL`, `MXLAssessmentOptions` | `Graph.Profile = ProfileMXLSource` | `ExampleInspectMXL`; `TestPackageGraphRawPrimaryAndSourceContainer` |
| Current `MXLPackage` | `AssessMXLPackage`, `MXLAssessmentOptions` | `Graph.Profile = ProfileMXLModel` | `TestPackageGraphCurrentDirectModelAndNoInputMutation` |
| Original raw MXL with selected dictionaries or composed semantics | `InspectMXLWithDictionaries`, `MXLDictionaryAssessmentOptions` | `Resources.Graph.Profile`: `ProfileMXLSourceDictionaries`, `ProfileMXLSourceInternalDTDDictionaries`, or `ProfileMXLSourceSemantics` | The three `*CompositionEntryPoints` / `*EntryPointsKeepRepresentationAndProfiles` tests above, `raw MXL` cases |
| Current `MXLPackage` with selected dictionaries or composed semantics | `AssessMXLPackageWithDictionaries`, `MXLDictionaryAssessmentOptions` | `Resources.Graph.Profile`: `ProfileMXLModelDictionaries`, `ProfileMXLModelInternalDTDDictionaries`, or `ProfileMXLModelSemantics` | The same three entry-point tests, `current MXL` cases |

Additional source-only choices are intentional. `InspectXML` accepts
`ProfileSourceInternalDTD`, `ProfileSourceInternalDTDLocal`,
`ProfileSourceInternalDTDLinks`, `ProfileSourceInternalDTDDictionaries`,
`ProfileSourceContainer`, `ProfileSourceInternalDTDContainer`,
`ProfileSourceSounds`, `ProfileSourceSoundsDictionaries` and
`ProfileSourceInternalDTDSoundsDictionaries`. The dictionary wrapper accepts
`ProfileSourceSoundsDictionaries` and
`ProfileSourceInternalDTDSoundsDictionaries` for explicit sounds XML. There is
no `SoundsDocument` or container model conversion; source-profile admission
does not manufacture such a model. Auxiliary and DTD behavior remains covered by
`TestAuxiliarySourceAssemblies` and `TestDTDPackageCompositionAuxiliaryProfiles`.
These two auxiliary routes are inventory references, not part of this record's
focused execution receipt.

Plain standalone assessment can select a dictionary-containing profile without
supplying data. To supply data, use its dictionary wrapper. A graph profile is
never a standalone source/model profile, and a raw-MXL profile is never a
current-MXL profile. Graph profiles bind exact intrinsic source/model/container
profiles; the options do not accept arbitrary cross-target combinations.
Current-model APIs walk the current model directly and do not encode it to
reconstruct original XML or ZIP evidence. Original reader inputs stay open;
usable bodies returned by selected providers transfer ownership and are closed.

## Options that remain independent

- `AssessmentOptions.Budgets` (or `Graph.Budgets`) creates one root account.
  Children, dictionary imports, retained reports and linked/package work share
  that account. A child does not reset the limits.
- Every zero budget field selects its documented finite default. Every negative
  budget field is a configuration error. Defaults are safety ceilings, not a
  workload capacity, latency or memory-use promise. `MaxBufferedBytes` measures
  admitted logical retained representations and model-candidate storage; it is
  neither allocated bytes nor peak heap/RSS.
- `DecodeAssessmentOptions.Decode.MaxXMLDepth` remains independent of the shared
  assessment depth limit. The tighter applicable limit wins. Zero decode depth
  means 256; the maximum configurable decode depth is 4096.
- `MXLAssessmentOptions.MXL` supplies independent archive, metadata, document,
  resource, expanded-total and per-document XML-depth caps. Zero fields select
  `DefaultMXLOptions()`. These do not replace or enlarge `Graph.Budgets`.
  Package assessment's expanded-total cap includes selected container, mimetype
  and primary entries; `DecodeMXLPackage` has its separately documented scope.
  Current packages lack original raw ZIP evidence; a raw archive limit is not
  proof about a reconstructed archive.
- `ResourceDependency` is a closed, independent selection of documents, image
  metadata, instrument associations or package roles. Empty selection invokes
  no resolver. Selected nil/typed-nil resolvers leave the corresponding facts
  unavailable. Retrieval/canonical identities grant no network or file access.
- Dictionary selections name exact manifests and nonempty supported relations.
  Empty selections invoke no source. Nil/typed-nil sources leave selected
  capabilities unavailable. Selection, byte identity and assessment profile
  remain separate choices; no implicit latest edition or remote fetch exists.

The 14 finite budget dimensions and accounting modes are defined in
[`validation/budgets.go`](../../validation/budgets.go), with all-field tests in
[`validation/budgets_test.go`](../../validation/budgets_test.go).

## Selected dictionary matrix

| Manifest constant | Explicit relation constants | Interpretation boundary |
| --- | --- | --- |
| `DictionaryManifestSMuFL14Canonical` | `DictionaryRelationCanonicalName`, `DictionaryRelationPrimaryCodepoint` | Exact canonical membership and primary codepoint; independent MusicXML lexical/family constraints remain active |
| `DictionaryManifestSMuFL14Ranges` | `DictionaryRelationRangeMembership` | Exact independently selected ranges; no implicit selection by canonical-name request |
| `DictionaryManifestStandardSounds40` | `DictionaryRelationStandardSound` | Standard-ID recognition; nonmembership does not invalidate an allowed custom sound string |
| `DictionaryManifestIPAPositive2015Unicode13` | `DictionaryRelationIPASymbol`, `DictionaryRelationIPAAttestedExample`, `DictionaryRelationIPACanonicalEquivalent` | Preserved partial positive evidence; no full-input grammar or negative complement |
| `DictionaryManifestIPARepresentation2015Unicode13` | `DictionaryRelationIPARepresentation` | Separately selected affirmative whole-input repertoire witness; no negative complement, phonotactics or pronunciation claim |

All constants in this table belong to `validation`. Consumers still need an
appropriate dictionary or semantic profile. The semantic compositions include
normative canonical/IPA consumers, but do not add optional standard-sound
recognition or dictionary-backed rendering advice merely because data is
available. Use the dictionary profile when those observations are wanted.

`TestDictionaryGraphKnownSetReachesEveryPrimaryRepresentation` demonstrates one
verified caller source across linked source/model and raw/current MXL entry
points. `TestIPARepresentationPublicSourceModelAndAssessedReader` and
`TestIPARepresentationGraphAliasesAndMXLUseSameWitness` exercise the separately
selected representation witness. `TestDictionarySoundsCustomAllowedAndIPAGate`
retains the open-string sounds and partial IPA boundaries. These are reused,
not duplicated with a second corpus or synthetic substitute.

The runnable `ExampleBundledDictionarySource` in
`bundled_dictionary_example_test.go` demonstrates the additive bundled source.
`TestBundledDictionaryPublicAssessmentPreservesSelection` in
`bundled_dictionary_test.go` is its source/model/assessed-decode ×
explicit/unselected/nil-source integration route. The same file checks all
five pinned components without filesystem access
(`TestBundledDictionarySourcePinnedBytesWithoutFiles`), exact requests,
independent body ownership, and retained importer/accounting behavior.
`TestBundledDictionaryGraphAndMXLParity` in `bundled_dictionary_graph_test.go`
adds post-review graph source/model and raw/current-MXL comparisons against a
caller-provided source, including all five components and explicit, unselected,
nil-source and external-byte-limit cases.

These bundled-source routes were not present or executed in the focused test
tree identified below and are not counted in its receipt. The separate initial
PR 42 acceptance receipt certifies only the routes present at head `195c5eb`;
it does not cover the post-review parity addition. Selecting the bundled
source must preserve the nil-source and explicit-selection contracts checked
here.

## Existing coverage reused and exact missing cases added

| Contract / axis | Precise route and substantive case | Disposition |
| --- | --- | --- |
| Valid explicit selections / positive | Entry-point matrix above; dictionary entry-point test checks profile, target, resource count and enumeration; semantic and DTD composition entry-point tests check exact intrinsic selection and absence of invented model source origins | Reused |
| Standalone preflight / negative | `TestSourceAssessmentPreflightDoesNotRead`: empty/unknown/model profile, negative budget, nil/canceled context and invalid decode depth with no reader call; `TestAssessModelPreflight`: incompatible source selection, invalid budget and nil/typed-nil documents | Reused |
| Source/model dictionary target / negative | `TestSemanticCompositionRejectsWrongTargetBeforeDictionaryIO`: valid selected manifest plus wrong target rejects before dictionary calls or input bytes; `TestDictionaryAssessmentRejectsBeforeInput`: empty, incompatible and model profiles | Reused |
| Linked dictionary configuration / negative | `TestDictionaryGraphConfigurationBeforeAnyIO`: old incompatible graph profile, unknown manifest, invalid dependency and invalid MXL depth preserve zero reader/provider calls | Reused |
| Remaining public-wrapper preflight / negative | `TestFinalConfigurationUncoveredEntrypointsRejectProfilesBeforeIO`: eight explicit entry-point subtests, each with empty, unknown and known-but-incompatible profile; call counters cover reader, dictionary and resolver; assessed decode exposes neither model nor requested source report | New, 24 profile cases |
| Zero/default and negative budgets / boundary | `TestBudgetDefaultsFiniteCopiedAndEachFieldNegativeRejected`: all 14 zero fields normalize to exact finite defaults, copied values cannot mutate them, each negative field fails, each positive override is retained; `TestEveryBudgetKindExactLimitModeAndFailureAtomicity`: exact limit and one-over with unchanged usage on failure | Reused; no duplicate threshold search |
| Shared account / interaction | `TestSharedBudgetAccountConcurrentAggregateQuota`: shared aggregate quota; `TestBudgetOverflowFailsWithoutMutation`, `TestAllocationSizeChecksHostIntWithoutNarrowingLimits`, `TestDiagnosticPairChargeIsAtomic`: arithmetic, host conversion and joint diagnostic accounting | Reused; host-size test is not a claim of actual 386 execution |
| Decode / MXL caps versus account / boundary and interaction | `TestDecodeAssessedUsesTighterDepthLimit`: both depth orderings; `TestPackageGraphRawExactByteOwnership`: exact and one-under archive, metadata, document, expanded-total, input-account and external-account caps independently; `TestPackageGraphNestedAggregateByteCaps`: aggregate expansion across nested packages; `TestDictionaryGraphPreservesMXLXMLDepth`: dictionary wrapper keeps independent depth refusal | Reused |
| Closed resource selections / negative | `TestResourceOptionsRejectBeforeAnyIO`: unknown/duplicate dependencies, invalid identities and budget; `TestResourceOptionsEmptyAndTypedNilResolver`: selected typed nil normalizes to unavailable | Reused internal option tests; new public resolver case below |
| Closed dictionary selections / negative | `TestDictionaryImportSelectionBeforeCallbacks`: unknown/duplicate manifest, empty/unknown/duplicate relation, excess selection count with zero source calls | Reused |
| Unselected / missing capability | `TestDictionaryImportEmptyAndMissingSources`, `TestDictionaryGraphUnselectedSourceNeverCalled`, `TestResourceGraphNoResolverForUnselectedOrInvalidHref`: unselected providers are not invoked, including schema hints | Reused |
| Selected nil/typed-nil dictionary / missing fact and interaction | `TestFinalConfigurationSelectedNilDictionaryPublicReports`: same actual SMuFL occurrence through source inspection, current model and strict decode; unavailable unverified occurrence, complete input, incomplete assessment, no normative failure; strict decode has converted state but returns incomplete error and discards the model | New public-wrapper coverage, two provider forms across three entry points |
| Selected nil/typed-nil resolver / missing fact | `TestFinalConfigurationSelectedNilResolverDictionaryGraph`: selected document link keeps `resolver-unavailable`/Unknown, one primary resource, no fabricated violation or terminal error | New public dictionary-graph coverage, two provider forms |
| Provider failures, ownership and cancellation / interaction | `TestDictionaryImportProviderFaultOwnership`: usable returned bodies close on provider faults; `TestDictionaryImportTerminalCancellationAndBudgets`: terminal stop and finite limits; `TestDictionaryAssessmentOrdinaryImportFailurePreservesXSD` and `TestDictionaryGraphProviderErrorKeepsIndependentFamilyFailure`: provider error does not erase an independent normative failure | Reused |
| Error versus report completeness / missing fact and negative | `TestDecodeAssessedPermissiveAndStrictSourceAreSeparate`, `TestSourceAssessmentDTDCompletenessAxes`: nil inspection error may accompany violation or unsupported/incomplete evidence; `TestDecodeAssessedSourceAndConversionDomains`: conversion domain separate from source conformance | Reused, plus selected-nil tests above |
| Bounded diagnostics / boundary and interaction | `TestDecodeAssessedExplainsTruncatedSourceIssues`, `TestSourceAcceptanceSummaryKeepsCompositeCountsDistinctFromDetails`, `TestDecodeAssessedTruncatedSummaryPreservesJoinedCauses`: counts, stored details and joined causes remain distinct | Reused |
| Profile stability / interaction | `TestSemanticProfilesExactFinalIdentity`, `TestDictionaryProfilesPublishedBaselineMigration`, `TestPackageProfilesPublishedBaselineMigration`, `TestResourceGraphIntrinsicProfileBundlePreservesLegacyChoices`, `TestPackageProfilesDoNotMutateFrozenEight`: exact complete profile identities, explicit historical migrations and preserved old selection | Reused; a migration is not described as an unchanged digest |

The new tests live in
[`final_configuration_contract_test.go`](../../final_configuration_contract_test.go)
in external package `musicxml_test`; they use only public API. They complement
existing private-boundary tests rather than replacing them.

## Reading the result

For inspection/current-model calls, evaluate both the returned operation error
and `Report.Satisfied()` (or `AssessmentBundle.Satisfied()`). A nil operation
error is not completeness or conformance. Inspect `InputComplete`,
`AssessmentComplete`, `Conformance`, outcome counts, `Terminal`, and the bundle's
resource/package reports and enumeration state as appropriate. Unknown,
Unsupported, Blocked and Fail remain distinct. Diagnostic counts describe rule
evaluations and need not equal the number of stored findings.

Strict assessed decoding additionally enforces source acceptance and discards
the model on any acceptance, conversion, dictionary-operation or terminal error.
`ConversionConverted` alone is therefore not a usable-model guarantee. Source
validity and model representability are independent: use typed errors and
`errors.Is`/`errors.As`, including joined causes. Budget exhaustion is incomplete
resource failure, not proof that the music is invalid.

## Focused execution receipt

Execution date: 2026-10-16. Production base: the merged commit above. The
focused test-only tree is `aaa6a79884ec84fd304b12ddd7cf73b7faa9556b`, retained
by unpublished local commit `6c573307030c13f9b18081613fabd16203518792`.
That identical tree was published on the PR 42 branch as
[`daec7bcf517aed450c50ea90c28fff0886373063`][published-configuration-tests]
on 2026-10-16. Use that published commit as the portable checkout target; the
local commit identifies the original focused-test provenance. The new test
file's SHA-256 is
`5df6edf9b4be328d706d81542debff425f47957674f7f63e6a615ad34077d2e7`.
This documentation was added afterward and is not part of that measured tree.

Environment: official project recovery toolchain, Go `1.27.2`, Linux/amd64,
`GOMAXPROCS=2`, `GOFLAGS=-p=1`, `GOTOOLCHAIN=local`. Go execution was serialized
with the shared build lock; no full suite, threshold search, benchmark, fuzz or
race run was started for this item.

| Command scope | Outcome | Meaning |
| --- | --- | --- |
| `go test . -run '^TestFinalConfiguration' -count=1 -v` | Exit 0; 3 top-level tests; reported package duration 0.089 s | First run of the additions passed on unchanged baseline production code; no fabricated RED |
| Exact root-package selection below | Exit 0; 48 top-level tests and 5 runnable examples; 0.896 s | Reused substantive routes plus new public cases |
| Exact validation-package selection below | Exit 0; 11 top-level tests; 0.007 s | Finite budget/default/negative/accounting and closed dictionary metadata contracts |

Durations are ordinary test command output, not performance measurements or
acceptance thresholds. The full first-run log SHA-256 is
`5e798ba08fc183dd95cfe336a4f36898a4c11eef3549f99fe837b854fb617842`;
the combined reused-route log SHA-256 is
`21251037535daf849fccbd67fecd4eb9d5d6a6c59722a37aae07e0a4469d7fbc`.
Local log hashes identify retained development receipts; the complete commands
below make the selected test routes portable without depending on those logs.

```sh
go test . -run '^(TestFinalConfiguration.*|TestSourceAssessmentPreflightDoesNotRead|TestDecodeAssessedPermissiveAndStrictSourceAreSeparate|TestSourceAssessmentDTDCompletenessAxes|TestDecodeAssessedSourceAndConversionDomains|TestDecodeAssessedUsesTighterDepthLimit|TestAssessModelPreflight|TestAssessModelCurrentTargetAndRootMetadata|TestResourceOptionsRejectBeforeAnyIO|TestResourceOptionsEmptyAndTypedNilResolver|TestResourceGraphPrimaryReports|TestResourceGraphNoResolverForUnselectedOrInvalidHref|TestPackageGraphRawPrimaryAndSourceContainer|TestPackageGraphCurrentDirectModelAndNoInputMutation|TestPackageGraphRawExactByteOwnership|TestPackageGraphNestedAggregateByteCaps|TestDictionaryAssessmentExplicitProfilesAndConversion|TestDictionaryAssessmentRejectsBeforeInput|TestDictionaryAssessmentOrdinaryImportFailurePreservesXSD|TestDictionaryImportSelectionBeforeCallbacks|TestDictionaryImportEmptyAndMissingSources|TestDictionaryImportProviderFaultOwnership|TestDictionaryImportTerminalCancellationAndBudgets|TestDictionaryGraphEntryPointsKeepRepresentationAndProfiles|TestDictionaryGraphConfigurationBeforeAnyIO|TestDictionaryGraphUnselectedSourceNeverCalled|TestDictionaryGraphKnownSetReachesEveryPrimaryRepresentation|TestDictionaryGraphProviderErrorKeepsIndependentFamilyFailure|TestDictionaryGraphPreservesMXLXMLDepth|TestSemanticCompositionPublicProfiles|TestSemanticCompositionRejectsWrongTargetBeforeDictionaryIO|TestSemanticPackageCompositionEntryPoints|TestSourceDeclarationCompositionProfiles|TestDTDPackageCompositionEntryPoints|TestDecodeAssessedExplainsTruncatedSourceIssues|TestSourceAcceptanceSummaryKeepsCompositeCountsDistinctFromDetails|TestDecodeAssessedTruncatedSummaryPreservesJoinedCauses|TestSemanticProfilesExactFinalIdentity|TestDictionaryProfilesPublishedBaselineMigration|TestPackageProfilesPublishedBaselineMigration|TestResourceGraphIntrinsicProfileBundlePreservesLegacyChoices|TestPackageProfilesDoNotMutateFrozenEight|TestDictionaryCanonicalSourceModelAndIndependentFamily|TestDictionarySoundsCustomAllowedAndIPAGate|TestIPARepresentationPublicSourceModelAndAssessedReader|TestIPARepresentationGraphAliasesAndMXLUseSameWitness|ExampleInspectXML|ExampleDecodeAssessed|ExampleAssessModel|ExampleInspectXMLWithResources|ExampleInspectMXL)$' -count=1 -v
go test ./validation -run '^(TestBudgetDefaultsFiniteCopiedAndEachFieldNegativeRejected|TestEveryBudgetKindExactLimitModeAndFailureAtomicity|TestBudgetInvalidCallsNeverMutate|TestBudgetOverflowFailsWithoutMutation|TestBudgetUninitializedAccountsFailClosed|TestSharedBudgetAccountConcurrentAggregateQuota|TestAllocationSizeChecksHostIntWithoutNarrowingLimits|TestDiagnosticPairChargeIsAtomic|TestDictionaryManifestsAreClosedAndOwned|TestDictionaryComponentVerifierRejectsZeroWrongLengthAndCanceled|TestDictionaryConstructorRejectsContradictoryAndDuplicateImports)$' -count=1 -v
```

All coverage rows above were exercised in these selections except the two
explicitly marked auxiliary inventory references and the separately integrated
bundled-source routes. A test name alone is not execution evidence for another
configuration or a later tree. Each published candidate requires affected
routes and the unchanged repository/hosted checks on its exact tree. No actual
386 run, complete external-oracle run, corpus acceptance, performance
acceptance, independent review or full-program completion is claimed by this
focused receipt. The separate [PR 42 receipt][pr42-initial-receipt], dated
2026-10-16, records broader development checks for initial head
`195c5ebc82acc22bddcdde46c9394fc060659c2a` only; it does not certify subsequent
review corrections.

[published-configuration-tests]: https://github.com/go-muse/go-musicxml/commit/daec7bcf517aed450c50ea90c28fff0886373063
[pr42-initial-receipt]: https://github.com/go-muse/go-musicxml/pull/42#issuecomment-6216656081
