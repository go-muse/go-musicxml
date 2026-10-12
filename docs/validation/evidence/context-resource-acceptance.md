# Context resource acceptance and practical limits

The contextual profiles use the existing finite shared budgets. They do not
promise that arbitrary-size scores finish under the defaults. Resource exhaustion
is incomplete assessment, not a MusicXML violation. B5-PERF-01 remains assigned to
the final kernel/session reconciliation and is not discharged here.

## Independent measurements

Final production checkpoint `59dcdff` includes the independently reproduced common-path scan fixes
and standalone projection accounting correction. A clean 100-note fixture now
completes on source and current-model targets under unchanged defaults. The model
uses 14,944,682 rule steps and projects 2,174 records. The initial independent
checkpoint required 30,807,123 steps and exceeded the default 20,000,000-step cap,
while projecting fewer records (1,863). These are shared-account operation counts,
not wall-clock performance guarantees.

The 1,000-note fixture still exceeds default work limits. With explicit finite
`MaxRuleSteps: 1 << 34` and `MaxBufferedBytes: 512 << 20`, it completes at this
checkpoint with 145,289,971 model steps, 32,084 context facts and 21,074 projected
records. Raising work alone does not suffice: retaining the default 64 MiB buffer
cap interrupts assessment during projection at 442 records (67,106,090 bytes
used; the next 4,672-byte request exceeds the cap). The returned resource-limit error
identifies the exhausted budget; no successful-readiness claim follows from the
retained partial report.

The tests `TestIndependentContextOrdinaryHundredNotesDefaultWork`,
`TestIndependentContextFullSharedAccountCost`, and
`TestIndependentContextDefaultResourceMatrix` retain these distinctions. Exact
final-tree verification and final measurements are recorded with the pull request.

## Corrections and residual costs

- Exact compiled role membership and per-authoritative-binding attribute indexes
  replace repeated schema-role and absent-attribute scans. Actual declaration and
  assembly identity remain authoritative; negative cache entries cannot be
  published after interrupted enumeration.
- Per-part invariant performance advice is built once. Repeat boundary/marker and
  note-scope indexes prevent unrelated parts from rescanning one another. The
  independent whole-call growth guards now include ordered measure equality
  (11.04×), sequential measures (10.09×), sequential beat windows (11.33×),
  voice exclusions (10.00×), and the added uniform-meter collector work (9.99×)
  for ten times the ordinary input. The last guard explicitly subtracts the
  accepted timeline-query baseline rather than claiming that older work is fixed.
- A standalone span/layout/timing clone does not append to a state-history array.
  It reserves the conservative closed-graph measure once; report slice growth is
  already independently reserved. The shared timeline accounting is unchanged.
  An empty timing payload is 9,560 bytes on the measured 64-bit host, while its
  conservative clone reserve is 29,566 bytes (previously 59,132). This is not a
  claim of exact heap accounting.
- Full membership lists can inherently have output proportional to the number
  of ranges times their members. Unproved partial/overlapping positions may need
  conservative within-part comparisons. These cases remain bounded by quotas.
  Projection construction also retains conservative temporary query allowances.

Publication is tested transactionally at exact and one-under limits, through
cancellation polls, and with mutable nested graphs for all five payload families.
Large counts remain sparse; no loop expands an arbitrary staff/count scalar.
Actual 386 execution is a required CI gate: local cross-compilation alone does
not count as architecture execution.
