# Batch 6 acceptance checkpoint (not final)

Frozen production code: `22ae68a731ddc26e15f988fb59232e2dd9d572e6`, tree
`440f9d3cabffd535efb127bbbaeecb22e3185a27`. Independent semantic acceptance is
complete for this code checkpoint, as is independent resource/API acceptance.
Aggregate/CI gates are separately recorded below; historical runs retain their pins.
This report does not claim final batch acceptance, completion of all repository
stages, or any exact-head CI result. The public profiles remain bounded and opt-in.

## Executed checkpoint

- At `b6623ae`, all 185 distinct functions then declared by the execution ledger passed
  across `./...`, together with `TestTimelineOwnership*`, using an exact anchored
  function-name union and `-count=1`. This is a selected suite, not a full-suite
  pass. Evidence: `reconcile-declared-b6623ae.log`.
- At `d381b50`, the exact selected union of 21 new association/Placement/resource
  functions plus `TestTimelineFinalReviewDisplacedKeySchemaControls` passed.
  `MUSICXML_REQUIRE_XMLLINT=1` required the real source-schema controls to execute.
  Evidence: `reconcile-displacement-d381b50.log`. This excludes the outstanding
  public key-projection and cursor-unit regressions.
- At `d381b50`, all 206 distinct functions in the refreshed execution ledger
  plus `TestTimelineOwnership*` passed across `./...` using an exact anchored
  union and `-count=1`. Evidence: `reconcile-declared-d381b50.log`. The pending
  groups below are excluded from this selected pass.
- At `8237597`, all `TestTimelineFinalReview*` functions and
  `TestTimelineUnplacedDivisionsPreserveOnlyIndependentSameValue` pass across
  `./...`; required real xmllint fixture controls execute. Evidence:
  `reconcile-final-review-8237597.log`. This is a selected local run, not final
  independent review or the whole repository.
- At `8237597`, all 229 distinct declared functions plus ownership checks pass
  across `./...` using an exact anchored union. Evidence:
  `reconcile-declared-8237597.log`. The open long-identity work-meter regression
  is not included in this pass.
- At frozen production `22ae68a` plus independent test commit `38b77a3`, all
  243 declared functions plus ownership checks pass across `./...` with an exact
  anchored union and `MUSICXML_REQUIRE_XMLLINT=1`. Evidence:
  `reconcile-declared-22ae68a.log`. The additional eight functions independently
  test divisions-certificate and normalization gate/closure/restoration lifecycle,
  admission, causal ownership, primitive/full parity and cancellation.
- At production `22ae68a`, the lead's initial seven-stage run passed module tidy,
  reproducible generation, formatting, full tests, vet, full race and all seven
  counted fuzz targets. Evidence: `production-22ae68a-all7.log`. The sourced
  environment exports `MUSICXML_REQUIRE_XMLLINT=1`; the separately supplied
  `REQUIRE_XMLLINT=1` variable is unused. This earlier code checkpoint predates
  final independent tests and documentation, so the exact final head must rerun
  all seven stages. Actual 386 execution still belongs to CI.
- Runtime profile metadata and actual binding revisions match the compiled engine.
  All original source, translation, selection and later-owner integrity checks pass.
- The 121 selected clauses, 20 split/shared contracts, 135 original source records,
  95 requirements, 28 issue gates and 29 candidate dispositions remain preserved.
  Original pending translation/status fields are not promoted by executable evidence.
- Earlier checkpoint runs exposed stale lifecycle and optimized-work assertions.
  These were reported, corrected separately and rerun; they are not retroactive
  passes. The current selected run includes the corrected assertions.

See the [full scope](../TIMELINE_ASSESSMENT_SCOPE.md) and
[machine ledger](../timeline-assessment-ownership.json) for exact file/function
qualification. Shared assessed source/model fixtures are distinguished from
public MusicXML adapter tests. Integrity tests do not count as musical evidence.

## Preserved task-level RED → GREEN history

These are authentic retained regression and implementation commits, checked to
exist locally. They are representative task evidence, not a claim that one pair
executes every original five-axis contract. Independent review snapshots below
are separate from the worker implementation history.

| Task | Representative regression → implementation commits |
| --- | --- |
| Exact rational arithmetic and cursor | `e505cf8` → `7af4d12`; `1457e0f` → `e7dd337` |
| Sealed authoritative schema/provider contracts | `809e8c6` → `b7572d2`; `b825826` → `93faee8` |
| Typed state extraction, precedence and closure | `216f48a` → `3c059fc`; `bc78096` → `c1fd58a`; `e49e8bb` → `f8f0c7f` |
| Shared runtime and mandatory consumers | `a51000b` → `ba62e19`; `7378975` → `81efd1c`; `e3dc69c` → `089baab` |
| Full context origins and sparse statement providers | `933cb38` → `5a376ff`; `9734580` → `4700435`; `06d49c6` → `4e1049b`; `8103c5f`, `af0750f` |
| Independent semantic corrections | `f6797d3`/`35a2c7e` → `8e63f61`; `e061338` → `0f8f58d`; `d5a49ec` → `2338065`; `62b6887`/`d45f9d0` → `6c155bf` |
| Bounded practical ownership and lookup cost | `6c88619` → `35a8a4f`/`ff6ec4b`; `cab3b54` → `5576c8e`/`46cd579`/`ab8da72`; `1940479` → `c0ac72f` |
| Cross-frame continuing-state proof and consensus | `310dbae`/`1d8a2ce`/`f4338b5`/`f962eb3` → `f0b11ee` (integrated checkpoint `b6623ae`) |
| Cross-frame downstream association and nearer chord barrier | `acd674c`/`0abef6b` → `dbb5f8f`; worker `6415978` → `f000fe1` (integrated `1ed3400`) |
| Unavailable association Placement | Worker `525445b` → `44ffd16`; integrated `3408d7a` → `c7dda33` |
| Attribute and service-adapter Placement forwarding | `4643b97` → `aea97d8`; `775165b`/`2b1731b` → `1f7797b` (forwarding correction; later unit/projection fixes are recorded below) |
| Well-founded unit reconstruction, causes and directional bounds | `22a6c77`/`5f52cb0`/`87196b2` → `23bd0a0`; `d381975`/`b029b8c`/`8e9ccef` → `c52c9ce`; `9227daf` → `3124dcc` |
| Complete-history normalization and equal-value restoration | `743e74c` → `6c633a0`; `13ce21c`/`12a9fe1` → `8237597` (independently accepted at production `22ae68a`) |
| Invariant key-presence projection | `f34dc1e` → `121cbcf`, independently accepted at `d4071ae` |
| Canonical-identity work before comparison | `56a3b8d` → `94a56df`; `2224b66` → `4a3fe68`; `2921866` → `4543c44`; `65ecf3f`/`1d21eb8` → `22ae68a` |
| Full ownership and acceptance reconciliation | `5ba0bd1` → `f1958a9`; final code semantic report `bfb4b6a`, resource report `8fb81fc`, 243-case reconciliation `2f7cb90` |


Cross-frame continuing-state implementation reached the separately reviewed
`b6623ae` checkpoint; downstream associations subsequently reached the separately
reviewed worker `44ffd16` checkpoint. Final test/document integration and CI remain separately gated
below. Its [implementation report](timeline-cross-frame-implementation.md)
preserves the initial counterexamples, exact sufficient-proof boundary and
bounded exceptional-query cost. A RED commit is not promoted to a pass merely
because a later unrelated test command completed.

## Independent review snapshots

- Final independent semantic code acceptance is pinned to `22ae68a` and its tree
  above. A fresh preserved checkout passed full `go test ./... -count=1`, focused
  all-package race and `go vet ./...`, with required real xmllint 2.9.14. All 30
  final-review families pass without exclusions; that family count is not a
  coverage denominator. The [final semantic report](timeline-final-semantic-review.md)
  records the exact commands, logs and reviewed proof matrix. Subsequent code
  changes require a refresh. This semantic acceptance does not substitute for
  the separate resource report or exact-final-head aggregate/CI verification.

- Final resource/API acceptance at production `22ae68a`, with independent guards
  `38b77a3` and report `8fb81fc`, passes all public independent defaults, full
  validation tests, focused race, vet, root/validation 386 compilation and all
  60 warmed matched benchmark samples. Certificate and normalization admission,
  ownership, cancellation and cleanup controls pass. No concrete resource/API
  blocker remains in that reviewed code; actual 386 execution remains a CI gate.
- Historical resource checkpoint `ab8da72c1c538946945d4c06cb8139360714b1e7` accepted all eight
  public direction and all eight public figured-bass variants, including default
  source/model budgets, plus independent arithmetic, cancellation, ownership and
  ordinary-trajectory controls. The [resource report](timeline-resource-independent-checkpoint.md)
  records matched medians/ranges and their limits. This is incremental timeline
  cost, not closure of the earlier legacy-validation performance regression.
- Semantic component review at `047baa91ba0fd29e1315d549a3e53ff12788ce70` accepted
  conditional instrument/MIDI, positioned target gaps, figure-prefix, grace
  locality/score-order and conditional playback causal repair. Major cross-frame
  state/default/provenance controls are green at `b6623ae`.
- Association worker `44ffd16` is independently accepted at review merge `1b13767`:
  unavailable-position displacement/bounds controls, original direction/figure/
  grace regressions, and unavailable sound-offset actual cause all pass. Evidence:
  `final-semantic-associations-44ffd16-green.log`. This does not accept later
  unit-region staging or mandatory public key projection.
- Key-presence implementation `121cbcf` is independently accepted at `d4071ae`:
  valid-baseline presence remains known while missing/invalid/hidden/scope cases
  retain their proper uncertainty and actual sources.
- Causal/directional unit implementation `58f1a43` plus `e6a0ed4` is independently
  accepted at `e89fe4e`, including locked prior Invalid causes, actual new-frame
  repair and excluded-future positivity/bounds. Evidence:
  `final-semantic-unit-58f1a43-e6a0ed4.log`. This earlier component snapshot does
  not accept the later complete-history closure or raw-divisions certificates.
- Resource refresh `904c7cf` passes all 32 public practical direction/figure/grace
  variants. At production `1f7797b`, independent tests `e7cf607` pass separate
  ownership of 16 publisher families, 880 observed cancellation polls, literal
  first-gap admission, focused private/race checks, validation/internal-xsd vet
  and validation-package 386 compilation. The [resource report](timeline-resource-independent-checkpoint.md)
  records the exact scopes/logs. Compilation is not actual 386 execution; its
  matched benchmark remains at `ab8da72`.

Older semantic/resource reports retain their earlier-head scope and are not
final acceptance evidence. The historical parent whole-repository run at `1f7797b`
failed the public displaced-key consumer, cross-frame unit-proof and local-unit
placement-bound groups (`pre-unit-certificate-whole-repository.log`). A later
unrelated pass cannot erase those recorded failures.

## Remaining acceptance work and precise boundaries

No unresolved semantic counterexample remains in the independently reviewed
bounded matrix at `22ae68a`. The earlier complete-history, safe restoration,
projection and canonical-identity regressions have actual implementation and
executed evidence above. No earlier RED is retroactively relabeled as a pass.

- The exact final documentation/test integration requires all seven local
  repository stages, followed by all nine exact-head CI jobs when publication
  is authorized. The final head does not exist until that integration completes.
- Original source-dependent donor/group ambiguities remain precise evidence
  boundaries. A named test or a truthful Unsupported result does not execute a
  full five-axis clause recipe or independently close a mandatory clause.

The ledger maps implemented contributions and precise remaining source-dependent
cases to the original clause IDs. Unsupported never closes a mandatory clause
by itself. Rendering/span consumers remain with batch 7, including graphical Bezier selection; batch 6 retains exact
origins and units. Later batches 8–9 retain their original external/link/dictionary
and final-reconciliation ownership.

## Acceptance still required

The profile snapshot and 243-case named-test binding are reconciled. Independent
semantic/resource acceptance and the initial seven-stage run above remain pinned
to their tested production code. All
seven repository-local stages on the final documentation/test integration and
all nine exact-head CI jobs remain pending until explicit results are supplied. Cross-compilation is not actual 386 execution;
the latter remains a CI requirement. Real xmllint evidence concerns XSD controls,
not musical semantics.

B5-PERF-01 remains outstanding under the shared validation kernel/session
workstream, with final reconciliation in batch 9. No new timeline optimization,
passing provenance test, scoped benchmark or incomplete result closes it.
