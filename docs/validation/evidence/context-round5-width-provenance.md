# Round 5 width-conflict provenance

Review: [4251682431](https://github.com/go-muse/go-musicxml/pull/36#discussion_r4251682431).

The conflict `layout.width-outcome` row is a direct observation of unavailable
encoded boundary evidence. Its absent raw width scalar does not make that
evidence inherited geometry or an application choice. The conflict call now
selects `ContextExplicit` through a private explicit-provenance publishing
helper. Raw remains Absent; Value, Reason, Cause, scopes, ownership and admission
are unchanged. Ordinary publication and ordinary width comparison retain their
existing provenance selection. No exported API, normative scope, default budget
or source policy changes.

## Test-first evidence

- `TestLayoutRound5WidthConflictProvenance`: 32 public cases across source/model,
  partwise/timewise, initial/later position, both part orders, plain yes/no and
  yes/no/malformed conflicts. These exercise 80 conflict rows.
- `TestLayoutRound3ConflictingAdmittedBreaksEveryOrder`: 36 admitted evidence
  cases (Unknown/Unsupported/Invalid, initial/later, all six orders), now also
  checking 108 conflict rows for exact provenance, absent Raw and preserved
  state/reason/cause.
- The final test matrix was rerun against untouched production at published
  `9803bb699ac3eb7b7fbe2cab226a225892e4e1bf`: exit 1, only provenance assertions
  fail. Authentic RED log SHA256:
  `9553a3d5d0e985b634a31081d6ec8f2dcc5a46fe838f903cdf08f470e49d3fbf`.
- Same matrix with the correction: exit 0. Focused GREEN log SHA256:
  `5a13c6d4c1d5971e1e7c9520456446b13a08e23ca534f9cf3c63db19690532c0`.
  Earlier test development corrected an overly strict Unknown-reason assertion
  and a Go Code/string type mismatch; neither changed production semantics.
- All layout regressions passed in both root and validation packages.

Independent design critique preferred this call-site-specific correction over
globally relabeling all unavailable values. The review's positiveInteger parity
observation was explicitly note-only and outside this requested correction.

Independent semantic/resource QA accepted the frozen correction. Forty
ordinary/explicit publisher combinations compare every payload field and exact
work/fact/retention accounting with direct publication, including independently
owned causes. Five ordinary width controls preserve consistency, disagreement
and incomplete-prerequisite behavior. Exact/under byte admission, fact/work
quotas, sticky errors and every cancellation poll leave no partial publication
or retention leak. These controls are checked in as
`validation/context_layout_round5_independent_test.go`.

The independent full layout regression run passed (root 7.593s, validation
0.161s); its race variant also passed (root 63.351s, validation 1.969s), using
Go 1.27.2 linux/amd64. The reviewer fixture's malformed `maybe` alone does not
force the conflict branch in every public adapter; the public three-part
yes/no/malformed case and admitted-state matrix reproduce the actual branch
without changing its selection policy.

The Round 3 measured limits remain the documented workload evidence, not a new
measurement claim for this metadata correction. Official samples still require
explicit work and buffer budgets; B5-PERF-01 remains deferred. Final whole-tree
seven-stage verification and exact-head nine-job CI are recorded in the PR.
