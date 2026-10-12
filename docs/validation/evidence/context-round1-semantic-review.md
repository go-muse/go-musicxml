# Round 1 independent semantic acceptance

## Result and scope

**Semantic result: accepted for the production candidate identified below, subject to the explicitly separate integration gates.** All 69 original review threads were read and assessed against the repaired implementation, tests, primary sources and stated boundaries. The independent findings in this review were corrected and rechecked. This is bounded semantic acceptance, not an assertion that all MusicXML prose is implemented or that the PR is ready to merge.

The lead still owns the final seven-stage aggregate run, actual supported-host 386 execution, the final official-sample resource matrix and documentation, the PR-body explanation of the context fuzz registration, and remote CI for the exact published head. A passing test result from an earlier head does not close those gates.

## Exact identities

- Reviewed production candidate: `3e778bf36e60fca4644b65f1c448a32e9073a128`.
- Candidate Git tree: `c5184bf93fb48ba6b50eb46cf47345f7af311d59`.
- Original Round 1 review head: `d9e5fa1b6fef198a4e6042e5bbc6204dae1d99b9`; production-equivalent repair baseline: `b54786a95a1203db98397bfb4ee046d9e1835355`.
- Original 69-thread snapshot SHA-256: `4c850a80b58318ab8200856cd22da736506b728c3b4c93b84413934eeca0ce7d`.
- Layout dispatch integrated commit: `3e778bf` (author commit `7c20854`).
- Independent typed-evidence/provenance corrections: `d310d87`; state selected-publication reuse: `63dc6a8`; current evidence-route and catalog corrections: `3c0e573`; span category wildcard correction: `6d6a9d7`.
- `validation/context_layout_dispatch.go` SHA-256: `87c37a585f2c2f20c1fe40db3cb12e71718170ab17aed0ac438076da60f0e49e`.
- `validation/context_layout_runtime.go` SHA-256: `44c77da040dee139f98b6d2fffe8faf1fea523105ece2545a13e8889e57062af`.
- `validation/timeline_state.go` SHA-256: `45f29b5d6e7e94d1f750b42448071a7244a29111ca00040bdd4302c650ac69e7`.

The candidate was clean when copied into an isolated test snapshot. Tests below do not depend on concurrent edits in the integration worktree. A subsequent documentation-only commit should refer to this production identity and separately verify its own final aggregate/CI result.

## Evidence classes and preservation

Public `InspectXML` and `AssessModel` tests run against actual adapters and both score hierarchies. Current-model evidence includes a deliberate post-decode NaN geometry mutation; it is not merely a claim about the source string. Source and model representations retain their existing differences.

Defensive malformed-Known atoms, unavailable declaration/category evidence, quota/cancellation injection and established timeline services assembled directly in tests are supplementary provider evidence. They do not establish that the public kernel emits those synthetic states. In particular:

- Incomplete public partwise/timewise hierarchies can hit the accepted identity gate before span grouping. Separate established-domain tests exercise the provider gap propagation defect.
- An omitted required duration in an unrelated part still makes canonical identity unavailable; this case was not relabeled a successful repeat-isolation result. A facet-invalid duration with known identity is a distinct control.
- The empty-measure witness fixture already failed correctly on the baseline. Its added regression coverage is not a claim that the baseline could never produce a consumer failure.
- Top-level `ContextResult.Value` is unused for compound-only payloads and was excluded from recursive zero-field defect claims.

All 307 check records and 291 retained original dispositions remain present. The retained dispositions compare equal to the original planning JSON. Current executable mappings were corrected without promoting historical statuses. Those counts are an inventory, not a conformance denominator. The earlier six profiles retain their fixed digests and selected-rule counts. `B5-PERF-01`, accepted Batch 6 association/closure limits, and exact missing-required-attribute ownership remain open or bounded as before.

## Primary-source conclusions

- The official [tied reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/tied/) explicitly permits a visual tie attached to one note using ordered start/stop marks. The implementation therefore preserves an isolated exact local pair, but does not greedily extract it from a competing chain.
- The [dashes reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/dashes/) and [official formatting example](https://www.w3.org/2021/06/musicxml40/musicxml-reference/examples/dashes-element/) support dash-length/space-length applicability. They do not justify claiming an omitted line-type attribute default.
- The [percussion tutorial](https://www.w3.org/2021/06/musicxml40/tutorial/percussion/) establishes the treble-based display reference, including G4 on staff line 2, independently of the clef glyph's encoded line.
- The [sound reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/sound/) supplies jump defaults, but does not settle a universal identity between nested repeat passes and D.C./D.S. epochs. Mixed jump/repeat traversal consequently retains an unavailable bounded prefix.
- The [bend reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/bend/) describes the same-note bend/release pair. The negative consequent can satisfy the implication without inventing a broader cross-note gesture association.
- The [figure-number reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/figure-number/) requires numeric content. Nonempty ASCII digits are accepted as a sufficient subset; the remainder retains the unreviewed grammar gate. The mandatory class is unchanged.

## Independent verification

Native toolchain: Go 1.27.2, with the real xmllint requirement enabled. Commands used `-p 1` to bound build parallelism.

1. **Final full package run:** `go test -p 1 ./... -count=1`. Passed all packages on the exact candidate (root package 65.895 s; validation package 18.068 s).
2. **Final focused corrections:** `go test -p 1 ./validation -run 'Test(LayoutDispatch|IndependentStateSelectedReuse|ContextStateSelectedPublicationProjectedOnce|CrossReview(Span|Beam|Vertical))' -count=1`. Passed on the exact candidate.
3. **Public span, preservation and ownership:** `go test -p 1 . -run 'Test(SpanRound1|ContextSpan|IndependentContextFrozenProfileDigests|ContextOwnership|ContextTranslations)' -count=1`. Passed after the span wildcard correction; these tests are also included in the final full package run.
4. **Selected-state differential/race:** `go test -p 1 -race ./validation -run 'TestIndependentStateSelectedReuse|TestContextStateSelectedPublicationProjectedOnce|TestTimelineState' -count=1`. Passed in the isolated snapshot with the exact `timeline_state.go` hash above. This focused race run is not the final aggregate race gate.
5. **Frozen profiles after selected-state reuse:** `go test -p 1 . -run TestIndependentContextFrozenProfileDigests -count=1 -v`. Passed, preserving all six fixed digests and required-rule counts.
6. **Public constructor/provenance probes:** all four source/model × partwise/timewise routes confirmed explicit payload states and corrected documented-default/application-choice provenance. Only intentional top-level union Value fields remain unused.
7. **Current implementation-map audit:** every simple current implementation symbol resolves, including renamed `performanceLocalRoles`; all named test functions checked resolve. Root-alter current routing and figure-number catalog text match the actual implementation.

### Ordered public layout parity

The independent parity driver compared all ordered public Layout payload fields before and after dispatch, rather than sorting rows or selecting a few values. It covers 24 actual adapter/hierarchy cases: inherited formatting, scalar defaults, invalid geometry and selectors, multiple-print ambiguity, staff/page/system relations, percussion coordinates, grace notation, harmony and figured-bass. A model case mutates geometry to NaN after decoding.

The comparison explicitly serializes `Integer`, `Decimal` and `Rational` through their exact `String` representation. Ordinary JSON of structs with unexported numeric storage would otherwise erase meaningful values and could create a false parity result. Every other exported nested field, cause, origin, applicability, unit and provenance is retained in order.

The output before dispatch (`63dc6a8`) and on the final candidate is byte-for-byte identical: 24 JSONL records, both SHA-256 `1245026092fb754fd94d1e45567d6edad6abe65f3d529233bbeedaf7192f5264`.

The committed `TestLayoutDispatchMatchesExhaustiveRoleReference` and `TestLayoutDispatchCombinedRolesPreserveOrderAndIndependentAxes` additionally compare complete ordered output against the former exhaustive control flow. Tests cover unavailable axes, parent-only directive output, duplicate/wrong-axis role lists, finite constant capacity, exact/one-under work limits and nonsticky reader refusal. Manual branch comparison found no omitted publisher.

### Selected-publication differential

`validation/semantic_state_selected_crossreview_test.go` retains the pre-change query as an independent test-only oracle. All 864 three-candidate permutations preserve values, exact causal references, source-order provenance and inheritance: equal/different Known values, Unknown, Unsupported, Invalid and Absent; mixed exact-staff/all-staff groups; owned and borrowed results. Mutation of a returned causal reference does not alias stored history.

The corrected query uses fewer work units without changing causal precedence. Exact budgets pass at 4263 steps for the borrowed fixture and 13849 for the owned fixture; one step less returns `ErrResourceLimit`. All 130 borrowed and 118 owned cancellation polls return cancellation. The selected value is projected once, while distinct candidates still project and compare normally.

An initial independent harness build used the wrong integer width for `RuleKey.Revision`. This was a test-harness error corrected before execution, not a production regression or a claimed baseline RED.

## Additional independent findings closed

- Layout constructors exposed zero evidence states on unprojected axes. Each constructor now supplies explicit states, preserving existing causal metadata; clone-time normalization was deliberately avoided.
- Non-duration events and chord notes need different treatment for an absent internal End field. The context projection retains Absent for non-duration events, explicit unavailable chord-end evidence, and any already-established endpoint. Grace make-time keeps an explicit unrealized-position boundary rather than inventing a realized position.
- Layout scalar default provenance now distinguishes documented staff/lyric defaults and the harmony application choice from encoded values.
- Current implementation maps no longer claim the removed performance root-location field or removed `performanceLocal` function. The separate performance print-object field and historical inventory routes remain intact.
- Figure-number catalog text now describes the sufficient numeric subset instead of claiming every style-text value is unsupported.

## Per-thread disposition

The numbering below is the original zero-based review snapshot order. “Corrected” means the bounded code/test disposition is accepted here; external documentation and CI actions remain separately identified.

## 0 · PRRT_kwDOT4c3vs6rgi3G

The per-node constants have been reduced through shared role dispatch, typed layout copies, sparse indexes and selected-value reuse. Final official-sample budget statements and resource measurements are integration gates; neither synthetic capacity nor original baseline minima are a substitute for the final sample matrix.

## 1 · PRRT_kwDOT4c3vs6rgi3R

Lazy timing payload and shared role dispatch inspected. Both reductions preserve exact declaration matching; family-cost/resource measurements belong to resource acceptance.

## 2 · PRRT_kwDOT4c3vs6rgi3W

Typed flat LayoutContext cloning inspected: owned strings and DecisionRefs are cloned; immutable numeric wrappers retain byte accounting. Per-property publication granularity remains explicit.

## 3 · PRRT_kwDOT4c3vs6rgi3c

ActualType-only interning is correctly rejected. The kernel selected-use table depends on actual assessed binding completeness and required-slot presence; exact owner/schema/assembly and verified selector equality remain prerequisites.

## 4 · PRRT_kwDOT4c3vs6rgi3e

Finite advisory vocabulary is precomputed; no per-call concatenation in the property validator.

## 5 · PRRT_kwDOT4c3vs6rgi3k

Canonical binding table is hashed at compile time and shared in new context-plan digests. Earlier six profile digests are independently frozen.

## 6 · PRRT_kwDOT4c3vs6rgi3q

Negative bend-alter proves the implication consequent without proving a larger gesture. Nonnegative ambiguous release remains unavailable. Exact same-technical antecedent is retained.

## 7 · PRRT_kwDOT4c3vs6rgi3x

Nonempty ASCII digits are a sufficient numeric subset. Other strings remain Unsupported; no new lexical rejection or advisory demotion. Mandatory classification is retained.

## 8 · PRRT_kwDOT4c3vs6rgi32

At this Round 1 snapshot, public empty repeat consumer failures were tested with independently witnessed cursor coverage. Unwitnessed terminal End remained Unknown. The measure witness test already passed baseline and was added coverage, not a newly fixed impossible result. The subsequent [empty-measure onset follow-up](context-empty-measure-onset-review.md) supersedes this measure-specific limitation with a separate scored-onset counterexample; terminal End ordering itself remains unchanged.

## 9 · PRRT_kwDOT4c3vs6rgi38

Same-measure beat closure extends to part end. Cross-measure correspondence remains explicitly unavailable; no meter-sized measure capacity is invented.

## 10 · PRRT_kwDOT4c3vs6rgi4B

PR-body explanation is a lead-owned external deliverable. Repository read-only review cannot claim that explanation was posted.

## 11 · PRRT_kwDOT4c3vs6rgi4I

Nonsticky reader quota error is propagated before the no-role continue. A seven-step internal fixture exercises the actual failure site.

## 12 · PRRT_kwDOT4c3vs6rgi4O

Temporary repeat note snapshots and windows use admitted releasable scratch. Rationals remain session-owned; release of backing storage is not misrepresented as zero arithmetic retention.

## 13 · PRRT_kwDOT4c3vs6rgi4S

Part-scoped coverage preserves unknown membership interference. Public omitted-duration P2 remains unavailable through canonical identity closure; it is not falsely advertised as a green isolation case.

## 14 · PRRT_kwDOT4c3vs6rgi4Y

Irrelevant repeat Beat/PatternMeasures and stop ContentEquivalent are explicitly Absent.

## 15 · PRRT_kwDOT4c3vs6rgi4j

Restricted pitch/rhythm domain admits benign engraving, dot and time-modification while preserving direct sound-tie, ornament/technical, grace and unavailable-domain gaps.

## 16 · PRRT_kwDOT4c3vs6rgi4o

Stop/origin/closure and scoped rosters are indexed once. Common scopes reuse borrowed rosters; arbitrary voice/unplaced cases and output-sized membership remain metered conservative paths.

## 17 · PRRT_kwDOT4c3vs6rgi4w

Interchangeable time evidence prevents a fabricated known beat unit.

## 18 · PRRT_kwDOT4c3vs6rgi42

Dead test-only measure comparator is removed and the scale test runs the production roster comparator.

## 19 · PRRT_kwDOT4c3vs6rgi4_

Repeat child iterations charge before optional matching and redundant completeness reads are removed.

## 20 · PRRT_kwDOT4c3vs6rgi5E

Dead known-other-part branch is removed after exact partitioning.

## 21 · PRRT_kwDOT4c3vs6rgi5K

Static performance defaults use shared exact owner roles rather than an all-role probe per node.

## 22 · PRRT_kwDOT4c3vs6rgi5R

Movement advice indexes divisions and gap histories by exact part/measure, preserving source-order gap precedence.

## 23 · PRRT_kwDOT4c3vs6rgi5a

MIDI spatial advice uses parent-sound grouping without part-wide scans per sound.

## 24 · PRRT_kwDOT4c3vs6rgi5j

Swing unknown publication is a repairable point gap; a later proved publication can recover the state.

## 25 · PRRT_kwDOT4c3vs6rgi5o

A genuinely available service with missing event yields event-specific Unknown. Actual unlisted-part public adapters retain causal upstream identity Invalid; synthetic known-service tests are distinguished.

## 26 · PRRT_kwDOT4c3vs6rgi5v

Absent encoded note type intentionally excludes the bounded encoded-type swing projection. It does not claim acoustic nonswingability from duration.

## 27 · PRRT_kwDOT4c3vs6rgi5z

Bend declaration unavailability is retained instead of NotApplicable. Known false antecedent is restricted to the established pair, not arbitrary negative bends.

## 28 · PRRT_kwDOT4c3vs6rgi56

Unconditional swing publication uses nil TimeOnly, matching the history contract.

## 29 · PRRT_kwDOT4c3vs6rgi6C

Known nonpositive swing components become unavailable rather than an arithmetic configuration failure; authentic scalar-invalid values remain causal.

## 30 · PRRT_kwDOT4c3vs6rgi6H

Performance writers preserve first interruption; guarded storage and sparse-index error tests cover cancellation/quota.

## 31 · PRRT_kwDOT4c3vs6rgi6M

Shared key/time/transpose/staff-details, swing, position conversion and release-child helpers replace duplicated projection logic.

## 32 · PRRT_kwDOT4c3vs6rgi6T

A mixed repeat/jump pass domain no longer resets indefinitely. Public D.C.-inside-repeat traversal ends in finite Unknown without claiming a normative epoch policy.

## 33 · PRRT_kwDOT4c3vs6rgi6a

Ending boundary roles obtain source-supported ordering around repeat controls. Ending realization remains Unknown; earlier proved ordinary repeat prefixes survive.

## 34 · PRRT_kwDOT4c3vs6rgi6g

Known listening identity keys use a charged map. Unavailable pairs remain conservatively compared with charge before early inequality.

## 35 · PRRT_kwDOT4c3vs6rgi6o

Mixed local-repeat ToCoda no longer borrows a local frame pass. Source does not establish a universal repeat/epoch selector.

## 36 · PRRT_kwDOT4c3vs6rgi6t

Direction-type graphical marker advice excludes barline glyphs, whose activation is a separate playback relation.

## 37 · PRRT_kwDOT4c3vs6rgi60

Unused Passes is removed; MarkerActive separates marker activation from Guard. Prefix InitialSyncMode and irrelevant fields use explicit evidence.

## 38 · PRRT_kwDOT4c3vs6rgi6_

Provider-level borrowed instrument index avoids per-part speculative retained facts; contained-prefix comparison uses suffix minima, and displaced fallback remains metered.

## 39 · PRRT_kwDOT4c3vs6rgi7E

Shared normalized positive-decimal scanner and distinctness/position helpers retain complete grammar validation, duplicate policy, and cancellation cadence.

## 40 · PRRT_kwDOT4c3vs6rgi7J

Local provenance is selected at semantic call sites. Additional layout-static omissions found independently were corrected in d310d87: staff-number and lyric-placement omission are documented defaults; absent harmony arrangement is an application choice.

## 41 · PRRT_kwDOT4c3vs6rgi7P

Staff-line and staff-line-position labels are separated; scalar is no longer read on the attribute path.

## 42 · PRRT_kwDOT4c3vs6rgi7V

Malformed admitted Known fret/divisions atoms become unavailable. These are defensive provider fixtures, not claims that kernel adapters emit mistyped atoms.

## 43 · PRRT_kwDOT4c3vs6rgi7e

Color lexical validation precedes parsing/indexing. Defensive malformed Known test cases no longer panic; kernel-invalid values retain their prior cause.

## 44 · PRRT_kwDOT4c3vs6rgi7i

Root-alter location moves to local constants and figured-bass printout fields are projected. Current implementation-map ownership was corrected in 3c0e573 to local.root-alter-location, with the separate RootAlterPrintObject performance contribution retained. Historical inventory routing is unchanged.

## 45 · PRRT_kwDOT4c3vs6rgi7t

Official dashes semantics/example support dash/space applicability without fabricating an omitted line-type attribute or documented default.

## 46 · PRRT_kwDOT4c3vs6rgi75

Glyph-only loose missing-required bridge is deleted; exact shared required-owner bridge remains authoritative.

## 47 · PRRT_kwDOT4c3vs6rgi8H

Local publish paths propagate reader interruption before projection; unused binding/helper removal is complete.

## 48 · PRRT_kwDOT4c3vs6rgi8Q

Harmony root association requires complete enumeration; score-part child result is reused and temporary font string admission is held through owned publication then released.

## 49 · PRRT_kwDOT4c3vs6rgi8Z

Lyric unavailable publication and ancestor lookup preserve first reader error; interruption never becomes a normal evidence payload.

## 50 · PRRT_kwDOT4c3vs6rgi8g

Official tied reference expressly supports isolated same-note start→stop visual ties. Competing identity runs remain Unknown; ordinary stop→start chains retain known associations.

## 51 · PRRT_kwDOT4c3vs6rgi8l

Malformed Known vertical identity is Unknown, not fabricated Known(false). Defensive fixture reachability is distinguished from actual public scalar input.

## 52 · PRRT_kwDOT4c3vs6rgi8r

Known grace/cue vertical categories are separated. Unknown category remains possible interference with every compatible category; 6d6a9d7 preserves known identity axes while propagating the exact category cause, including beam runs.

## 53 · PRRT_kwDOT4c3vs6rgi82

Unnamed lyrics use an explicit absent identity, not verse 1; duplicate unnamed siblings are ambiguous and independent numbered lines remain distinct.

## 54 · PRRT_kwDOT4c3vs6rgi9C

Same-note recommendation is restricted to relevant roles and exact identity; beam begin/continue/end levels do not create irrelevant start/stop advice.

## 55 · PRRT_kwDOT4c3vs6rgi9L

Singleton association depends on known local role; unused invalid number retains causal evidence in Number. Non-temporal positions remain explicitly Absent.

## 56 · PRRT_kwDOT4c3vs6rgi9R

Exact reverse dispatch avoids all-family probes. Gap/association/member loops are charged, and new borrowed identity indexes release scratch.

## 57 · PRRT_kwDOT4c3vs6rgi9b

Unknown-domain musical gaps reach all possible part runs, excluding separately enumerated non-temporal domains. Public incomplete hierarchy is stopped earlier by accepted identity; synthetic established-domain tests exercise the provider defect separately.

## 58 · PRRT_kwDOT4c3vs6rgi9j

Staff override lifetime is tracked separately from system publisher certainty and repaired only for fresh explicit selectors.

## 59 · PRRT_kwDOT4c3vs6rgi9n

Undecidable visible system publisher cannot silently select score defaults.

## 60 · PRRT_kwDOT4c3vs6rgi9u

Unavailable multiple-print selection is propagated; observed name-display updates do not disappear.

## 61 · PRRT_kwDOT4c3vs6rgi9z

Parallel break consensus precedes frame mutation; disagreement retains parity/membership and stale override uncertainty.

## 62 · PRRT_kwDOT4c3vs6rgi9-

Known absent supports@attribute is ignored as irrelevant.

## 63 · PRRT_kwDOT4c3vs6rgi-G

Staff selector gaps are part-local and reset at known system boundaries. Sparse selectors retain explicit Number evidence.

## 64 · PRRT_kwDOT4c3vs6rgi-L

Skipped potential width membership marks the accumulator incomplete. Unavailable divider selection cannot fall through to known default geometry.

## 65 · PRRT_kwDOT4c3vs6rgi-R

Named row omissions are corrected. The independent public sweep also found additional zero-state Layout axes; d310d87 explicitly initializes genuinely unprojected axes in each constructor while retaining supplied causal evidence. No blanket clone normalization was added.

## 66 · PRRT_kwDOT4c3vs6rgi-j

Nil timeline is guarded; blank-page parity requires initialized nonnegative integer evidence.

## 67 · PRRT_kwDOT4c3vs6rgi-m

Official percussion tutorial supports the G4/line2 display reference independently of encoded glyph line. Word-font fallback origin belongs to the actual consumer when no component is inherited.

## 68 · PRRT_kwDOT4c3vs6rgi-p

Scope/state consumers reuse one immutable admitted hierarchy-block cache, and interrupted partial state indexes are not cached.

## Retained normative and execution limits

- No universal span balance, LIFO association, staff/voice partition or automatic lyric verse 1 is introduced. Unknown competitors preserve compatible-run interference.
- Self-contained singleton association does not require unused metadata, but invalid metadata retains its own exact cause.
- Repeat equivalence remains a restricted pitch/rhythm proof. Sound ties, arbitrary performance notation, grace timing, unproved membership and unwitnessed coverage are not silently treated as engraving or absence.
- Beat part-end support is measure-local. Unshifted cursor evidence and encoded movements are distinct from display offsets, nominal meter and unproved measure capacity.
- Ending execution and mixed repeat/jump pass policy remain unavailable where not established. No absent repeat count, after-jump rule or reset policy is invented.
- Layout is scoped encoded/default geometry and applicability, not an engraver. Playback projections are not a physical performance model; absent encoded type is only the stated encoded-type exclusion.
- Source/model, exact scalar, causal XSD ownership and accepted Batch 6 chronology boundaries remain intact. No old profile is broadened, budget default raised, or later-owner gate closed.
- This reviewer did not perform GitHub mutations, merge/ready/auto-merge operations, the final seven-stage aggregate workflow or remote CI. Native 386 execution is unavailable on this host; compilation-only evidence must never be described as execution.
- The final official-sample/default-budget conclusions must come from the resource acceptance matrix for the published production code. A synthetic or family-only measurement is not a full-profile default-capacity claim.
