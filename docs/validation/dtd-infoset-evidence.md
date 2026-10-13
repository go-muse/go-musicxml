# Internal-DTD prerequisite verification evidence

This document distinguishes historical test-first evidence from acceptance of a
published final tree. It does not claim that the final whole-PR stages or CI have
run merely because focused tests passed. Exact final-tree results belong in the
PR's dated verification receipt after those runs finish.

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

Independent runtime review accepted production commit
`8142aa712e24a8d9b5d165c8b917ddabdbe5add1`, tree
`0103ca3aeb3c57bf7cff57d895dd879df11c39da`, on 2026-10-13. It reproduced and
verified the standalone fix, ran source/DTD and validation origin tests, four
frozen-profile guards and 1,000 counted internal-DTD fuzz executions. Additional
test-only delivery `a136905ad3114e603ac25c85703708844953fdbe` is integrated as
`221ecc9ee39bb8c659cb932e205b3e81d935fb28`: exact repeated-default attribute
accounting, six resource refusal kinds, cancellation/cleanup, independent owned
reports, namespace/default cases and unsupported snapshot/report preservation.
The public unused-recursion and predefined-redeclaration controls additionally
verify completed input with `SyntaxIncomplete` and `Satisfied()==false`. Ordinary
report incompleteness can accompany a nil `InspectXML` error; the tests do not
equate nil error with acceptance. This is a dated production-pin acceptance,
not a substitute for final packaged-tree local/CI verification.

Focused source/core tests and independent adversarial probes support the bounded
implementation. Final acceptance additionally requires all seven local stages,
the exact published tree, all nine CI jobs, executed 386 tests and real xmllint.
Local 386 binaries compile in this execution environment but cannot execute
(`exec format error`); compile-only evidence must not be called ACTUAL386.

The full race stage retains its detector and test coverage; capture JSON timing
on the next required run so performance attribution is measured rather than
guessed. No default budget increase, timeout increase or skipped regression is
part of this prerequisite. Parallel tooling/race optimization is a separately
owned local lane and is not implicitly accepted by these results.

The scope's external/unused-declaration/provenance limitations remain explicit.
Source support does not close XSD ENTITY membership, DTD ID domains, general
validating-DTD semantics, external retrieval, dictionary consumers, original
source history of current models, or the final performance obligation.
