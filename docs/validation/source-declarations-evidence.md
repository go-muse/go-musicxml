# Source declaration and composition evidence

## Baseline and current gate

Canonical baseline is merged PR40 commit
`5295080b2030be6d616c714e7c8bcad47185c742`, tree
`b463a3e22b6a65e6ef1d14356c4e9a79d8d7dbf7`.
[Its main CI](https://github.com/go-muse/go-musicxml/actions/runs/38519205492)
passed all eleven checks. This is baseline evidence, not acceptance of this change.

Current implementation is undergoing independent semantic/resource review.
Final seven-stage local verification and exact-head hosted CI remain pending.
No whole-batch acceptance is claimed by the focused results below.

## Authentic regression checkpoints

- Public composition tests first compiled on the baseline and failed because
  the three new profiles were unavailable. The assembled candidate now passes
  those tests, reached ENTITY controls, defaulted local constraints and physical
  versus effective link/schema-hint projections.
- Core API tests first failed on the absent additive API. A separate executable
  semantic regression then reproduced missing declaration membership. Further
  interruption tests caught and corrected a false proof from a zero intermediate
  result and a downgrade of effective default/fixed membership failures.
- Scanner lexical tests executed before implementation and failed on missing
  pre-normalization provenance. The unelected control passed. Declaration bridge
  capability and subsequent typed ownership/order controls were checked separately.
- Consumer tests first exposed absent APIs, then exercised independent physical
  and effective evidence, owned copies, exact/one-short budgets, cancellation and
  existing local/link regressions. The custom-mode compatibility guard has its
  own executable acceptance-versus-refusal regression.

`TestSourceDeclarationCompositionPreservesProfiles` pins the complete serialized
metadata of sixteen existing source/model profiles, captured before changes.
This includes required keys, support, closure text and catalog identity; it is
not merely a comparison of profile names.

Author-focused race tests passed. Local 386 cross-compilation succeeded, but this
host cannot execute the resulting binaries. Actual 386 execution remains a
required hosted CI gate; cross-compilation is not reported as execution.

The full original-obligation and source boundaries remain those in
[the scope document](source-declarations-scope.md). Unresolved constructed-type,
external-declaration, package-transport and broader conformance claims are not
promoted by these regression checkpoints.
