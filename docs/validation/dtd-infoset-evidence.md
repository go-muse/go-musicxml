# Internal-DTD prerequisite verification evidence

This document distinguishes historical test-first evidence, local implementation
verification and exact published-head CI. The dated receipts below apply only to
their named commits and trees. They do not verify subsequent Round 1 corrections
or establish final acceptance of the wider assessment program.

## Logical implementation sequence

1. Bounded declaration store: lexical keys, first binding, read/cutoff/standalone
   state, separate entity domains and declaration metadata.
2. Internal input frames: context-aware replacement, original-byte identity,
   independent finite nesting, pre-admitted work/retention and cleanup.
3. Sealed source-origin contract: exact source/assembly selection, ownership,
   typed component availability, coverage seal and unsupported snapshot export.
4. Internal parameter declarations and read-default verification: declaration
   ancestry, frame boundaries, context-correct unused/default checks and retained
   uncertainty.
5. Explicit public profile: effective defaults/normalization before namespace
   expansion, one assessed/conversion stream and occurrence-origin projection.
6. Independent semantic correction and final verification: reference declaration
   origin across PE callers, public/resource probes, real xmllint differential,
   counted fuzzing and precise scope/compatibility documentation.

These are dependency-related milestones, not one commit per test. The first
three published checkpoint commits are `c0defa499b68222a2bec6d2939cb7032916c3067`,
`72a9e0d3fd6e0278c9fca97beeeb400a68582a55` and
`8a8e956989796cfce7c00ba725701b8e0d3381b4`; the last has tree
`b25342bb4e3a16a635c6d8a8c9c68403e58ad886`. That checkpoint was not advertised
as a complete PR or full-suite acceptance. Local author commits and authenticated
GitData commits have different metadata; publication must verify identical trees
and exact canonical parent chains, not assume equal commit IDs.

## Test-first and independent findings

- The first public regression used the literal profile ID so it compiled before
  the exported constant existed. It failed because the explicitly selected
  profile was unavailable. This is behavioral RED, not a missing-symbol claim.
- Non-CDATA normalization tests first failed against a pass-through stub on
  trimming/collapse and cancellation. The earlier missing-method compile failure
  is separately recorded as API-scaffolding evidence. Boundary controls include
  numeric-reference whitespace, Unicode nonspace, 4096-byte chunk/UTF-8 splits,
  exact/one-short work and buffer admission, and zero retained bytes after Close.
- Integration tests failed before effective attribute application: default
  attributes were absent, non-CDATA whitespace unchanged and a same-tag defaulted
  xlink binding unresolved. Public source acceptance and assessed conversion now
  use the same effective stream. Explicit attributes suppress defaults with
  physical specifiedness preserved.
- Independent tests found a real standalone-reference origin defect. A directly
  declared alias invoked from a PE-origin default inherited the caller's region
  and incorrectly accepted a reference to a PE-declared entity. The original
  public-scanner fixture failed before the one-line semantic correction. Direct,
  nested, numeric-constructed and PE-declared aliases are tested with standalone
  yes/no controls. Reference declaration origin is independent of caller ancestry.
- An intermediate test expected complete success for an undeclared general
  entity after a read PE. That expectation was corrected to the explicit
  `dtd-entity-declared-validity-unassessed` capability result; a validity condition
  was not invented as a well-formedness error to make the test pass.

## Executable regression map

| Contribution | Representative committed regressions |
| --- | --- |
| declaration/read domain | `xml_source_dtd_store_test.go`, `xml_source_dtd_store_parameter_test.go` |
| lexical context and frame boundaries | `TestSourceInternalEntityContexts`, `TestSourceInternalEntityBoundaries`, `TestDTDAuditParameterCannotSupplyPartialDeclaration` |
| original bytes and encodings | `TestSourceInternalEntityOriginalEncodingDigest`, `TestSourceInternalEntityDataAndTerminalReadError` |
| finite resource/cancellation/ownership | `TestSourceEntityFrameAdmissionAndWork`, `TestSourceEntityFrameRetentionBeforeAllocation`, `TestSourceEntityFrameCancellationAtEveryDepth`, `TestDTDAuditParameterBudgetOwnership` |
| defaults and normalization | `TestSourceDTDUnusedDefaultWellFormedness`, `TestSourceDTDDefaultValidationDoesNotRetainDiscardedValues`, `TestDTDAttributesNormalizeBeforeNamespaceExpansion`, `TestDTDTokenizedNormalizationExactBufferAdmission` |
| exact reference origin | `TestDTDAuditStandaloneDirectAliasThroughParameterDefault`, `TestDTDStandaloneReferenceOriginDoesNotFollowCaller` |
| public profile/legacy isolation | `TestSourceInternalDTDProfilePublicAcceptance`, `TestSourceInternalDTDModePreflightAndLegacyIsolation`, `TestSourceInternalDTDDecodeUsesSameEffectiveStream` |
| honest unavailable declaration closure | `TestSourceInternalDTDUnverifiedDeclarationClosureIsIncomplete`, `TestSourceInternalEntityUnusedCycleAndLegacyCompatibility` |
| typed source-origin contract | `validation/source_origin_test.go`: exact providers/assembly, owned sites, duplicate/foreign/closed subjects, bounded ancestry, missing components, coverage, cancellation, retained terminal prefix, snapshot rejection |
| independent XML WFC oracle | `TestSourceInternalDTDAgainstXMLLint`: 12 internal-only cases, real `xmllint --nonet --noent --noout`, no schema/DTD-validity oracle conflation |
| mutational coverage | `FuzzSourceInternalDTDFrames`, run in the existing counted fuzz stage alongside the legacy source reader |

Old profile catalog/digest fixtures are rerun to detect accidental replacement
of accepted contracts. Parser-only probe success is not evidence that the public
profile accepted the same input; separate public tests exercise those results.
Independent review also checks early-cancel/export failure report preservation,
same-tag namespace/default identity, shared-account cleanup and no hidden I/O.

## Acceptance and limitations

### Historical independent review and published mapping

Independent runtime review on 2026-10-13 used unpublished local production
checkpoint `8142aa712e24a8d9b5d165c8b917ddabdbe5add1` (local tree
`0103ca3aeb3c57bf7cff57d895dd879df11c39da`). It reproduced and verified the
standalone fix, ran source/DTD and validation origin tests, four frozen-profile
guards and 1,000 counted internal-DTD fuzz executions. Additional test-only
local delivery `a136905ad3114e603ac25c85703708844953fdbe` was integrated at
local checkpoint `221ecc9ee39bb8c659cb932e205b3e81d935fb28`.

These local IDs are historical provenance, not published acceptance anchors.
Their integrated production state and probes are available at published
[commit 2a00ad3][integrated-commit], tree
`e2be9e7a394e2e444b87da84b15a1035bade26ac`. A local-to-published comparison
verified that the only difference from production checkpoint `8142aa7` is the
additional `source_dtd_audit_integration_test.go`; the production files are
unchanged. This maps the reviewed production bytes to a reachable canonical
commit without claiming that the intermediate production tree is identical to
the published integration tree.

The integrated probes cover exact repeated-default attribute accounting, six
resource refusal kinds, cancellation/cleanup, independent owned reports,
namespace/default cases and unsupported snapshot/report preservation. This
historical bounded review does not substitute for later whole-tree verification
or resolve Round 1 review findings.

### Completed local implementation verification

All seven local stages passed on the implementation tree
`7d8f0f1ad5c90b7c931d7b5f184cd39b16d81c93`, published as
[commit dcd148c][implementation-commit]: module tidy, generation, formatting,
ordinary tests, vet, full race and all eleven counted fuzz targets. The local run
used Go 1.27.2 and real xmllint 2.9.14 and finished at 2026-10-13 19:04:53 UTC
with exit 0. Local author commit `f3fe71dbeb24b32a6c2b75298c500bfa9a3783ef`
was verified to have that same implementation tree. The new internal-DTD fuzz
target completed its required 10,000 counted executions. Public unused-recursion
and predefined-redeclaration controls added at this checkpoint verified completed
input with `SyntaxIncomplete` and `Satisfied()==false`. Ordinary report
incompleteness can accompany a nil `InspectXML` error; the tests do not equate nil
error with acceptance.

The full race command retained every test and the detector, and per-test JSON
was captured in the local verification artifact `final-race.jsonl` (SHA-256
`76a0f44567e4b2d0f3435ad423c99dc150f5a475eadc1b3e3484ce20c8ba4c3d`).
The package-level pass records report 646.884 seconds for the root package and
163.336 seconds for `validation`. The complete local-stage log is
`final-seven.log` (SHA-256
`786cf3d56de20f46aee899ebae34f290a41a82de1b15e18203c6b02c3bc3f31c`).
These retained local artifacts are not published repository files. Concurrent
independent tooling-lane load was recorded, so these timings are not a controlled
performance comparison. No default budget increase, timeout increase or skipped
regression is part of this prerequisite. Parallel tooling/race optimization
remains separately owned and is not accepted by these results.

### Completed published-head CI verification

The later CI-selector correction at [commit 2c9d37b][ci-commit], tree
`6b830c5acc0f8983440198d529e2b6dda9168222`, added the internal-DTD differential
to the required real-xmllint CI selection and its LF/CRLF/counting regression;
it changed no production source. All nine jobs passed on that exact head in
[run 38453204915, attempt 2][ci-run]. The saved job logs verify actual 386
execution (`GOARCH=386 go test -count=1 ./...`), required real xmllint including
`TestSourceInternalDTDAgainstXMLLint`, all eleven counted fuzz targets and the
full race stage. Local 386 binaries compiled but could not execute in the local
environment (`exec format error`); actual 386 execution is established by this
CI run, not by compile-only evidence.

The local seven-stage receipt belongs to `dcd148c`; the nine-job CI receipt belongs
to `2c9d37b`. Neither receipt verifies subsequent Round 1 corrections. Those
corrections require their own final-tree checks and exact-head CI before any new
acceptance claim.

The scope's external/unused-declaration/provenance limitations remain explicit.
Source support does not close XSD ENTITY membership, DTD ID domains, general
validating-DTD semantics, external retrieval, dictionary consumers, original
source history of current models, or the final performance obligation.

[integrated-commit]: https://github.com/go-muse/go-musicxml/commit/2a00ad3294c44ec505edfd5312b5b6b642d5f4b2
[implementation-commit]: https://github.com/go-muse/go-musicxml/commit/dcd148c1c80e2f179373571fa51f1537f08c218c
[ci-commit]: https://github.com/go-muse/go-musicxml/commit/2c9d37be780287bd64d9f781f8ae77d615d4c501
[ci-run]: https://github.com/go-muse/go-musicxml/actions/runs/38453204915/attempts/2

## Round 1 focused capacity and oracle evidence

The repeatable `TestSourceOriginReviewOfficialCapacity` uses unchanged
`testdata/official/MozartTrio.musicxml` (94,916 bytes; SHA-256
`8d491c8db4d19c886ec157e2c26fdda91ee8faa801e24b31aaf8cf092d73c63f`),
the same source profiles, and default/24,000,000-byte retained budgets. Run:

```bash
go test . -run '^TestSourceOriginReviewOfficialCapacity$' -count=1 -v
```

The baseline is published `2c9d37be780287bd64d9f781f8ae77d615d4c501`.
The integrated correction measurement used local checkpoint
`f7896b5` (unpublished local commit metadata, not a remote commit link), with
this test added without production changes. Before `Finish`, internal-DTD
retention changed from 24,982,035 to 22,588,921 accounted bytes and rule steps
from 3,726,713 to 2,345,211. Preview remained 11,632,335 bytes / 44,913 steps.
Both default-budget runs completed input; both reports remained incomplete
because the external DTD was unread. The identical 24,000,000-byte limit
refused the baseline but admitted the correction. All retained account charges
returned to zero after finalization. These are capacity measurements, not
controlled CPU/RSS benchmarks or final whole-PR acceptance.

Later change: the internal-DTD profile now reads the bundled official MusicXML
external DTD. The same test now expects that profile to complete satisfied at
the default budget (about 27 MiB retained before `Finish`) and to be refused as a
resource limit at 24,000,000 bytes, while the preview keeps its unread-DTD
evidence. The measurements above describe the earlier unread-DTD state.

The expanded xmllint table separates explicit observed oracle classes from the
bounded reader's source-supported disposition. It records nonfatal validity
errors, validity/internal-error cascades and deliberate standalone/PE
interpretation differences; exit status alone is never a well-formedness proof.
The run logs the installed libxml2 version and uses `--nonet --noent --noout`
in an empty temporary directory. Its curated external-negative cases admit only
`SYSTEM "missing.ent"`; other literal external identifiers are rejected before
starting the process. This is a closed test corpus, not a general-purpose
sandbox for arbitrary XML.

Public no-fetch controls exercise both `InspectXML` and `DecodeAssessed` with
SYSTEM/PUBLIC identifiers, an HTTP access counter, a local content-use sentinel,
and original official sample bytes (including the UTF-16 BOM). Strict decode
retains incomplete source evidence and withholds acceptance. Frame fuzzing now
checks closed terminal classes, original read-prefix hashes, admitted input-byte
charges, EOF frame cleanup and final account cleanup; its mutation controls
reject injected arbitrary errors and corrupted byte/digest/frame summaries.
Round 1 final seven-stage, independent acceptance and exact-head CI remain
separate required gates after these focused checks.
