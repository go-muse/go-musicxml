# Timeline and effective-state assessment ownership

Status: **full-scope implementation target; integration and acceptance evidence
not yet reconciled**. Ownership baseline: `a1f2246`. Pinned research registry:
`3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c`.

The machine-readable [ownership ledger](timeline-assessment-ownership.json)
is the complete retained evidence contract. It carries 121 selected research
records, all 20 split/shared context checks, and the resulting union of 135
original records with all three Russian fields and reviewed English translations.
The earlier count of 82 identifies draft translations, not implemented clauses,
closed requirements, or a coverage denominator. No record is closed by this page.

All 95 engineering requirements, 28 issue gates, 29 candidate dispositions and
42 source contracts retain their complete previous ownership records. Their
original planning statuses, acceptance criteria and five-axis test recipes are
unchanged. The 342-record selection index preserves the outside-scope research
IDs as well. Inventory counts and planned fixtures are not executable coverage.

## Full batch-6 target

Exact musical timeline and full dependency-appropriate effective state belong
here. The chord-duration bound and numeral-key requirement are mandatory
milestones, **not a scope ceiling**. Time, clef, transpose, instrument, staff,
key and divisions work cannot be blanket-deferred behind those two consumers.

Both source and current-model inputs feed the same session-owned service,
shared XSD/typed identity evidence, and sealed consumers. A configuration-time
exact declaration binding is different from matching runtime local names. Model
assessment uses actual current field/Content order and exact current binary
numeric values, without encoding/reparsing or invented XML lexemes. Original
source decimals remain exact source values; decoding does not prove numeric
identity between these targets.

Required implementation targets are:

- Exact immutable rational quarter-note coordinates, including ordinary 1/3,
  normalized durations, part-local measure cursors and signed backup/forward.
- Document-qualified event and measure occurrences, verified score-part
  identity, preserved document order and independent musical order. Measure
  number is a label, not a numeric coordinate or unique primary key.
- Nearest nonchord principal relation and principal onset/duration. Ordinary
  and cue duration-bearing principals advance; chord tones do not advance;
  grace absence of duration never becomes a fabricated zero-duration scalar.
- Sparse exact staff and voice scopes, without staff-count enumeration or
  host-width truncation. Clef number defaults to 1; absent key/time/transpose
  number applies to all staves. An omitted multistaff assignment does not
  automatically choose staff 1, and an omitted voice does not become voice 1.
- Typed divisions, key, time, clef, staves/staff-details, transpose and instrument
  publications with source, explicit/default/inherited presence, exact scope,
  position, revision, history closure and dependency-local gaps.
- Exact offset origins and local-versus-direction precedence with separate
  visual, sound, listening and note-local timing. An offset does not move the
  score cursor. Harmony offsets cannot silently disappear from key selection.

The intended sealed bindings are `NewTimelinePlan`, `NewChordDurationPlan` and
`NewNumeralKeyPlan`. Their appearance here is a target, not proof that a current
implementation meets every clause. The ledger intentionally has empty new
runtime-binding/test-evidence arrays until exact-head integration is reviewed.

## Typed state responsibilities and remaining consumers

| State | Mandatory batch-6 contribution | Additional retained owner |
| --- | --- | --- |
| Divisions | Exact positive explicit units, musical-order history and exact normalization; sound/barline navigation divisions remain distinct | Batch 7 repeat re-entry/navigation; the narrow reconstruction evidence gate below remains open |
| Key | Traditional/nontraditional structured values, numbered/all-staves scope, prior-key/cancel provenance and encoded-key evidence; no invisible-key erasure | Batch 7 rendering and broader harmonic interpretation; batch 9 external glyph closure |
| Time | Preserve all encoded beats/beat-type pairs, additive strings, interchangeable and senza-misura, plus separately documented display defaults | Batch 7 detailed beat/slash/rendering consumers; no invented numeric grammar or meter-fill rule |
| Clef | Sign, line, octave-change, number scope, additional and active-field evidence; simultaneous clefs are retained | Batch 7 system rendering; no invented primary winner, treble default or staff-line clamp |
| Staff | Exact count and staff identity, staff-details/type/lines/tuning/capo state and documented defaults | Batch 7 rendering/system spacing/created-part consequences |
| Transpose | Separate chromatic, diatonic, octave-change, doubling and staff scope; add the octave once; preserve absent components | Batch 7 authoritative written-pitch intent and created-part realization; actual-score transpose remains separate |
| Instrument | Part-wide count, typed score-instrument ownership, plural note-local selections, initial definitions and positioned instrument-change/MIDI/play records | Batch 7 playback/repeat realization; batch 8 links/sounds; batch 9 dictionaries; property-update uncertainty stays explicit |

The initial staves/instruments count default is distinct from a repeated
per-attributes reset. Preserve earlier explicit counts after later omission;
zero remains the formal XSD value. Count 1 selects no score-instrument ID.
A previous note's instrument selection is not an inherited assignment.

State is selected at the greatest applicable established musical position, not
by last XML encounter. A later all-staves change must be considered even after
an older staff-local change. Equal-position incompatible publishers preserve
projection-specific ambiguity; inheritance is not a competing new publication.
Absence requires supported closed enumeration with sufficient part/staff/time
facts. An invalid update cannot simply be dropped to reveal an older good state.
Independent families and already established failures survive unrelated holes.

## Open source-evidence and interpretation boundaries

Follow the documented MusicXML semantics. The mid-measure divisions and rewind
work remains a **documented-semantics evidence/research gate**, rather than a
request for blanket policy approval. Ordinary changed-units rewinds wholly
inside one proven region are mandatory supported implementation work. Boundary
crossing/departure selection and late changed publishers retain narrowly scoped
causal incompleteness only where exact facts cannot be established. Preserve the
already proved prefix and independent state. No whole-family or whole-measure
unsupported shortcut is authorized by this boundary.

Onset/departure unit-selection conventions and all-solutions reconstruction
semantics are **not approved** by the instruction to follow documentation.
Importer lexical-unit conventions cannot override normative source evidence,
and should-level language cannot become a must-reject rule. The machine gate
`B6-DIVISIONS-REWIND` retains those distinctions. Primary-source research and
independent exact examples must settle the remaining cases; request a new user
decision only if that research leaves a substantive unresolved choice.

Other narrow interpretation boundaries remain explicit:

- Conflicting same-position publications have no generic lexical/specificity
  winner. An encoded-key-existence projection can need less than unique tonal
  content, but invalid key data is not a valid witness.
- A chord after backup/forward or an unobserved intervening note must not borrow
  an arbitrary older same-voice anchor. Missing principal evidence is not an
  invented orphan-chord error. Cross-staff chords and varying voice labels do
  not themselves establish a new prohibition.
- Effective-key lookup for a harmony must disclose its actual offset/position
  contract. A valid own numeral-key can independently satisfy the local
  alternative even when timeline service or fallback evidence is unavailable.
- Instrument property omission, direction/sound/listening offsets and visual
  placement need exact clause-specific dispositions before derived-state claims.

The original 28 issue gates keep `hard_error_basis=false`. Their associations
are contextual evidence, not independent rejection authority. No universal
initial C major, 4/4, treble clef or divisions=1 is added. No universal duration,
measure-fill, chord pairwise-length, note-type-duration equality or inferred
instrument-intent requirement is added.

## Mandatory consumers and joint ownership

Chord duration is compared with the established principal in common exact units:
principal 4, tones 1 then 3 passes; tone 5 fails. Later tone 3 is not bounded by
tone 1. Grace branches and interrupted anchors retain their actual evidence.

Numeral requires either a valid own numeral-key or an applicable explicitly
encoded effective key. Known complete absence can fail. Missing timing, scope,
unclosed history or invalid prerequisite retains incomplete/causal evidence.
Fifths 0 and nontraditional keys are not silently replaced with C major or
excluded merely because a richer pitch derivation is unavailable.

Barline-location, beat-repeat-content, measure-repeat-content and
written-transposition remain joint batch-6/batch-7 candidates. Batch 6 supplies
actual positions, occurrences, exact normalized events and effective transpose.
Batch 7 retains boundary interpretation, repeat correspondence/event equivalence
or authoritative written-pitch intent. A successful provider does not close the
joint consumer. The detailed rows below retain every clause's own contract.

All twenty split/shared checks remain explicit, including cancel-location,
beat/measure-repeat state, slash-stop, attack/release units, measure-label domain,
parallel-print mismatch and shared transposition-additive. An advisory parent
record cannot hide its separately stated default/context subclause. Non-timeline
split checks retain named batch-7, batch-8 or batch-9 owners rather than being
quietly dropped or claimed by the timeline profile.

## Frozen compatibility and planned profiles

The new explicit profile targets are `musicxml-4.0/model-timeline` and
`musicxml-4.0/source-timeline`. This ledger records them as planned. The lead's
actual API/catalog documentation and executable tests must establish their
availability, exact required/transitive bindings, roots and closure.

All previous profiles remain frozen: source-xsd-preview has 12 required keys,
model-xsd 14, model-local 36 and source-local 34. Preserve revisions, support,
targets, roots, normative closure, catalog digest and behavior. A separate
compiled timeline catalog can reuse immutable builders without broadening the
old catalogs. No new implicit default is permitted.

Legacy `Validate` remains direct current-model XSD-only with depth 4096 and its
existing MaxInt64 compatibility policy for other quotas. Explicit assessment
retains finite budgets. Opus score-only inapplicability must be proved if opus
is selected; it supplies no linked-score assessment. Unread external DTDs remain
incomplete. No DTD stripping/trust/fetching policy, package resolver, schema
hint loading, network access or external dictionary closure is introduced.

## Accounting, diagnostics and acceptance evidence

One existing account owns source/model/session/kernel/timeline work. Charge
extraction, exact arithmetic and intermediate magnitudes, GCD, copies, indexes,
history sorting/lookups, publications, provenance and diagnostics before work
or allocation. Sparse scope never requires a slice of arbitrary staves length.
Cleanup releases live gauges, never cumulative work. Repeated consumer queries
must not rebuild the service. No mutable rational or borrowed store escapes its
lifetime. Resource exhaustion is incomplete, never a musical violation.

Provider service availability is not proof that every optional state query is
known. Per-query gaps remain scoped. Missing capability cannot be represented as
absence or not-applicable. Unsupported handling does not close an obligation.
Normative failures, completeness, model representation, syntax and conversion
remain independent; diagnostics do not determine predicate counts.

Before a clause or batch contribution is marked implemented, record the exact
runtime binding/revision and implementation commit, independent hand-computed
source/current-model expectations, all applicable positive/negative/boundary/
missing-fact/interaction axes, causal interruption behavior and remaining owners.
The integrity tests in `timeline_ownership_test.go` check traceability only.
All final repository stages, exact-head CI, actual 386 execution and required
real xmllint checks remain separate acceptance evidence; XSD oracle success
cannot prove musical semantics.

## Remaining performance obligation

B5-PERF-01 remains **outstanding**, owned by the **shared validation
kernel/session workstream**, with final reconciliation in **batch 9**. Its
closure requires matched remeasurement and a reviewed improvement or explicit
reviewed residual-cost disposition. Incremental timeline benchmarks and catalog
isolation do not close it. Measure new timeline overhead separately; preserve
causal evidence, bounded resources and authoritative witnesses while optimizing.

## Translation and source evidence

Every original Russian description, short explanation and condition is copied
unchanged from the pinned registry beside the exact source ID, URL and original interpretation/status fields.
Schema-annotation sources retain XPath, line and text hash; HTML sources retain
section label, document hash, retrieval URL and original anchor-verification status. The reviewed English
triplets express textual equivalence to those Russian paraphrases; they are not
a new W3C transcription or normative approval. The original pending translation
statuses remain pending. Two earlier draft conditions had over-specific wording
removed: clef system printing and voice full-duration scope. The deprecated
none-clef terminology caution remains visible in the review detail.

The ledger retains immutable fingerprints for the design/inventory/translation
inputs, including the engine and normative reviews. Their proposals do not
change this document's open evidence/interpretation gates. Source subclauses for instrument
state and navigation divisions retain separate evidence IDs; no registry ID is
invented for them.

## Clause-by-clause retained ownership

All entries below remain planned in this ledger. Their complete original checks,
capabilities, five-axis recipes, translated fields and source locators are in the
linked JSON. “Later” describes additional ownership, not permission to defer the
batch-6 contribution. A source clause may support several providers/consumers;
research-record counts therefore do not equal runtime predicate counts.

### Timeline

- `MX40-PROSE-backup-no-boundary-cross`: Supply relevant timeline facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: It is recommended that backup not cross a measure boundary or a divisions change within a measure. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-backup-positive`: Supply relevant timeline facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: The duration of backup must remain positive. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-chord-anchor`: Retain the nearest preceding nonchord principal, its onset and duration; cross-staff and cue/grace branches remain explicit. Interrupted anchors retain the narrow gate. Remaining: Batch 7 repeat navigation, span pairing, detailed performance transformations and intent; batch 9 final normative reconciliation.
- `MX40-PROSE-chord-duration-bound`: Compare every duration-bearing chord tone with the established nonchord principal in common exact units. Base 4 then tones 1 and 3 passes; tone 5 fails; no pairwise non-increasing rule. Remaining: Batch 7 repeat navigation, span pairing, detailed performance transformations and intent; batch 9 final normative reconciliation.
- `MX40-PROSE-divisions-tuplet-units`: Build bounded immutable exact rational positions and durations, event/measure/part identities, preserved document order, scope-local chronology, principal-note and chord evidence. No meter-fill predicate. Remaining: Batch 7 repeat navigation, span pairing, detailed performance transformations and intent; batch 9 final normative reconciliation.
- `MX40-PROSE-duration-cursor`: Build bounded immutable exact rational positions and durations, event/measure/part identities, preserved document order, scope-local chronology, principal-note and chord evidence. No meter-fill predicate. Remaining: Batch 7 repeat navigation, span pairing, detailed performance transformations and intent; batch 9 final normative reconciliation.
- `MX40-PROSE-duration-not-notated-equality`: Build bounded immutable exact rational positions and durations, event/measure/part identities, preserved document order, scope-local chronology, principal-note and chord evidence. No meter-fill predicate. Remaining: Batch 7 repeat navigation, span pairing, detailed performance transformations and intent; batch 9 final normative reconciliation.
- `MX40-PROSE-forward-no-boundary-cross`: Supply relevant timeline facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: It is recommended that forward not cross a measure boundary or a divisions change. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-forward-positive`: Supply relevant timeline facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: The duration of forward must remain positive. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-fractional-divisions`: Supply relevant timeline facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: Integer divisions values are preferred for MIDI compatibility and to reduce rounding. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-grace-cue-duration`: Reuse existing schema/local/typed-reference result and exact causal identity; do not replace, duplicate or broaden frozen predicate. Remaining: Existing XSD/local/typed-reference owners retain this exact prerequisite: grace, including grace+cue, has no duration; cue without grace has duration; cue branches do not contain tie. Batch 6 reuses the established result and cause; no duplicate failure or new completion claim.
- `MX40-PROSE-measure-identity`: Use distinct measure occurrences and retain number as a label; no numeric parsing or universal uniqueness check. Remaining: Batch 7 retains optional guidance for this exact recommendation: Matching measure numbers are recommended for partwise measures in different parts that share a left barline. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-noncontrolling-measure`: Preserve noncontrolling measure identity/flag without inventing global cross-part alignment or nominal lengths. Remaining: Batch 7 repeat navigation, span pairing, detailed performance transformations and intent; batch 9 final normative reconciliation.
- `MX40-PROSE-one-divisions-per-part`: Supply relevant timeline facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: For broad compatibility, a single divisions element at the start of the part is recommended. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-part-id-target`: Reuse existing schema/local/typed-reference result and exact causal identity; do not replace, duplicate or broaden frozen predicate. Remaining: Existing XSD/local/typed-reference owners retain this exact prerequisite: part id must refer specifically to a score-part in part-list. Batch 6 reuses the established result and cause; no duplicate failure or new completion claim.
- `MX40-PROSE-part-list-order`: Build bounded immutable exact rational positions and durations, event/measure/part identities, preserved document order, scope-local chronology, principal-note and chord evidence. No meter-fill predicate. Remaining: Batch 7 repeat navigation, span pairing, detailed performance transformations and intent; batch 9 final normative reconciliation.
- `MX40-PROSE-voice-time-sequence`: Build bounded immutable exact rational positions and durations, event/measure/part identities, preserved document order, scope-local chronology, principal-note and chord evidence. No meter-fill predicate. Remaining: Batch 7 repeat navigation, span pairing, detailed performance transformations and intent; batch 9 final normative reconciliation.

### State

- `MX40-PROSE-attribute-score-order`: Publish updates at their established score positions; retain document order independently; apply history in musical order and preserve same-position ambiguity. Remaining: Only expressly split layout, playback, created-part and repeat/span consumers remain with batch 7. Core time/clef/transpose/instrument providers may not be blanket-deferred.

### Staff

- `MX40-PROSE-alternate-staff-playback`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 6 owns the typed state/origin/scope portion of: An alternate staff presents the preceding staff’s music in a different form and is not added to playback. Batch 7 retains rendering, spacing and created-part realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-capo-tuning`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 6 owns the typed state/origin/scope portion of: capo raises the open non-capo staff-tuning by the specified number of semitones. Batch 7 retains rendering, spacing and created-part realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-fret-zero-open`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 6 owns the typed state/origin/scope portion of: An open string has fret=0; the first fret is 1. Batch 7 retains rendering, spacing and created-part realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-hidden-staff-lines`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 6 owns the typed state/origin/scope portion of: Hidden lines count for clef line and display-step/display-octave, but not when counting lines for system or staff layout. Batch 7 retains rendering, spacing and created-part realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-show-frets-default`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 6 owns the typed state/origin/scope portion of: The default way to display frets is with numbers. Batch 7 retains rendering, spacing and created-part realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-staff-assignment`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 7 staff rendering, system spacing and created-part/extraction consequences after explicit per-clause split.
- `MX40-PROSE-staff-cutaway`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 6 owns the typed state/origin/scope portion of: print-object=no with print-spacing=yes preserves the vertical space of the hidden staff. Batch 7 retains rendering, spacing and created-part realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-staff-layout-scope`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 6 owns the typed state/origin/scope portion of: staff-layout number defaults to 1; in defaults it applies to every system, while in print it applies only to the current system. Batch 7 retains rendering, spacing and created-part realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-staff-line-numbering`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 7 staff rendering, system spacing and created-part/extraction consequences after explicit per-clause split.
- `MX40-PROSE-staff-numbering`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 7 staff rendering, system spacing and created-part/extraction consequences after explicit per-clause split.
- `MX40-PROSE-staff-size-scaling`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 6 owns the typed state/origin/scope portion of: staff-size specifies the percentage of interline space; scaling separately specifies the percentage size of notation. Batch 7 retains rendering, spacing and created-part realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-staff-tuning-line-order`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 6 owns the typed state/origin/scope portion of: staff-tuning line numbers lines from bottom to top. Batch 7 retains rendering, spacing and created-part realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-staff-type-default`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 7 staff rendering, system spacing and created-part/extraction consequences after explicit per-clause split.
- `MX40-PROSE-string-numbering`: Retain exact staff count, numbered scopes, staff type/lines/tuning/capo/details history and explicit/default provenance; preserve zero and arbitrary-width source counts symbolically. Remaining: Batch 6 owns the typed state/origin/scope portion of: Strings are numbered in descending pitch: 1 denotes the highest full-length string. Batch 7 retains rendering, spacing and created-part realization. No ordinary typed-state portion is deferred by that consumer split.

### Key

- `MX40-PROSE-cancel-before-barline-midmeasure`: Supply relevant key facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: For a key change within a measure, before-barline should be interpreted as left. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-cancel-key-value`: Retain cancellation value and previous effective key provenance. Do not invent cancel elements; cancellation rendering remains batch 7. Remaining: Batch 6 owns the typed state/origin/scope portion of: cancel repeats the fifths value of the cancelled key signature; on a change to C major or A minor, cancellation is implied without a separate cancel element. Batch 7 retains key/cancel printing and broader harmonic realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-harmony-none`: Publish traditional and nontraditional key structures, scope and history; consume explicit local numeral-key or a provably applicable encoded effective key. Remaining: Batch 6 owns the typed state/origin/scope portion of: kind=none explicitly denotes no harmony; root, numeral and function then have no harmonic meaning. Batch 7 retains key/cancel printing and broader harmonic realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-harmony-root-choice`: Reuse existing schema/local/typed-reference result and exact causal identity; do not replace, duplicate or broaden frozen predicate. Remaining: Existing XSD/local/typed-reference owners retain this exact prerequisite: Each harmony-chord selects root, numeral, or the deprecated function to avoid duplicating the basis. Batch 6 reuses the established result and cause; no duplicate failure or new completion claim.
- `MX40-PROSE-key-octave-cancel-exists`: Reuse existing schema/local/typed-reference result and exact causal identity; do not replace, duplicate or broaden frozen predicate. Remaining: Existing XSD/local/typed-reference owners retain this exact prerequisite: key-octave cancel=yes is prohibited without a corresponding cancel in the parent key. Batch 6 reuses the established result and cause; no duplicate failure or new completion claim.
- `MX40-PROSE-key-staff-scope`: Retain traditional/nontraditional key structure and numbered/all-staves scope; later all-staves updates supersede older local updates only by musical time. No C-major default. Remaining: Batch 7 rendering and broader harmonic interpretation/intent; batch 9 remaining external glyph closure.
- `MX40-PROSE-key-system-print`: Publish traditional and nontraditional key structures, scope and history; consume explicit local numeral-key or a provably applicable encoded effective key. Remaining: Batch 6 owns the typed state/origin/scope portion of: A key signature appears at the start of a system unless print-object is no. Batch 7 retains key/cancel printing and broader harmonic realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-minor-mode-default`: Publish traditional and nontraditional key structures, scope and history; consume explicit local numeral-key or a provably applicable encoded effective key. Remaining: Batch 6 owns the typed state/origin/scope portion of: Unqualified minor in mode or numeral-mode is interpreted as natural minor. Batch 7 retains key/cancel printing and broader harmonic realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-numeral-requires-key`: A valid own numeral-key independently satisfies the alternative. Otherwise query a closed, applicable encoded key at the exact harmony part/staff/position; absence proof and missing evidence are distinct. Remaining: Batch 7 rendering and broader harmonic interpretation/intent; batch 9 remaining external glyph closure.

### Time

- `MX40-PROSE-senza-misura`: Preserve staff-scoped time structures, composite/additive beats, interchangeable forms and senza-misura, and documented symbol/separator/print defaults. Remaining: Batch 7 slash display, detailed beat interpretation and notation/rendering consumers that require extra reviewed grammar.
- `MX40-PROSE-time-separator-default`: Preserve staff-scoped time structures, composite/additive beats, interchangeable forms and senza-misura, and documented symbol/separator/print defaults. Remaining: Batch 7 slash display, detailed beat interpretation and notation/rendering consumers that require extra reviewed grammar.
- `MX40-PROSE-time-staff-scope`: Preserve staff-scoped time structures, composite/additive beats, interchangeable forms and senza-misura, and documented symbol/separator/print defaults. Remaining: Batch 7 slash display, detailed beat interpretation and notation/rendering consumers that require extra reviewed grammar.
- `MX40-PROSE-time-symbol-default`: Preserve staff-scoped time structures, composite/additive beats, interchangeable forms and senza-misura, and documented symbol/separator/print defaults. Remaining: Batch 7 slash display, detailed beat interpretation and notation/rendering consumers that require extra reviewed grammar.

### Clef

- `MX40-PROSE-clef-conditions`: Publish number=1 scope and additional/after-barline applicability separately from sign/line/octave values. Retain simultaneous clefs; system rendering remains batch 7. Remaining: Batch 6 owns the typed state/origin/scope portion of: clef number defaults to 1; additional=yes disables the meaning of line, and size applies to the additional clef. Batch 7 retains system rendering and display realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-clef-line-range`: Publish clef sign/line/octave-change per exact staff; apply documented number=1, additional and mid-measure conditions only after review; keep display origin separate from sounding pitch. Remaining: Batch 7 system-start printing and layout rendering; no universal treble fallback.
- `MX40-PROSE-deprecated-none-clef`: Supply relevant clef facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: To hide the clef sign, print-object is recommended instead of the deprecated sign=none. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-display-percussion-clef`: Publish clef sign/line/octave-change per exact staff; apply documented number=1, additional and mid-measure conditions only after review; keep display origin separate from sounding pitch. Remaining: Batch 6 owns the typed state/origin/scope portion of: display-step and display-octave position a rest or unpitched note without assigning pitch; under a percussion clef, the treble reference G4 on the second line is used. Batch 7 retains system rendering and display realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-percussion-clef-unpitched`: Supply relevant clef facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: Under a percussion clef, always using unpitched instead of pitch is recommended. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-unpitched-default-position`: Publish clef sign/line/octave-change per exact staff; apply documented number=1, additional and mid-measure conditions only after review; keep display origin separate from sounding pitch. Remaining: Batch 6 owns the typed state/origin/scope portion of: Without display-step/display-octave, an unpitched note is placed on the middle staff line. Batch 7 retains system rendering and display realization. No ordinary typed-state portion is deferred by that consumer split.

### Transpose

- `MX40-PROSE-concert-score-transpose`: Reuse existing schema/local/typed-reference result and exact causal identity; do not replace, duplicate or broaden frozen predicate. Remaining: Existing XSD/local/typed-reference owners retain this exact prerequisite: A document with concert-score cannot contain transpose with a nonzero diatonic or chromatic value. Batch 6 reuses the established result and cause; no duplicate failure or new completion claim.
- `MX40-PROSE-diatonic-plus-octave`: Publish explicit chromatic/diatonic/octave/doubling facts and staff scope; derive exact displacement where all necessary numeric inputs are known. Remaining: Batch 7 written-pitch intent and created-part consumers; joint written-transposition remains open until authoritative instrument/intent evidence exists.
- `MX40-PROSE-double-transpose-default`: Retain an additional octave-doubling line, upper for above=yes and otherwise lower under the specific presence/default clause; never replace the primary line. Remaining: Batch 7 written-pitch intent and created-part consumers; joint written-transposition remains open until authoritative instrument/intent evidence exists.
- `MX40-PROSE-for-part-requires-concert`: Reuse existing schema/local/typed-reference result and exact causal identity; do not replace, duplicate or broaden frozen predicate. Remaining: Existing XSD/local/typed-reference owners retain this exact prerequisite: for-part is permitted only in a document with defaults/concert-score. Batch 6 reuses the established result and cause; no duplicate failure or new completion claim.
- `MX40-PROSE-for-part-staff-scope`: Retain explicitly declared created-part staff scope separately from actual-score transpose. Batch 7 owns created-part realization. Remaining: Batch 6 owns the typed state/origin/scope portion of: Without number, for-part applies to all staves of the part being created. Batch 7 retains created-part/written-pitch realization. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-octave-change-interval`: Supply relevant transpose facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: For intervals of an octave or more, using octave-change is recommended; for smaller intervals, not using it is recommended. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-transpose-additive`: Publish explicit chromatic/diatonic/octave/doubling facts and staff scope; derive exact displacement where all necessary numeric inputs are known. Remaining: Batch 7 written-pitch intent and created-part consumers; joint written-transposition remains open until authoritative instrument/intent evidence exists.
- `MX40-PROSE-written-transposition`: Publish effective actual-score transpose state and provenance. Batch 7 owns authoritative transposing-instrument/written-pitch intent and the consequent requirement. Remaining: Reviewed intent and effective transposition scope remain unavailable. Batch 7 written-pitch intent and created-part consumers; joint written-transposition remains open until authoritative instrument/intent evidence exists.

### Instrument

- `MX40-PROSE-instrument-change-target`: Reuse existing schema/local/typed-reference result and exact causal identity; do not replace, duplicate or broaden frozen predicate. Remaining: Existing XSD/local/typed-reference owners retain this exact prerequisite: instrument-change id refers to the affected score-instrument. Batch 6 reuses the established result and cause; no duplicate failure or new completion claim.
- `MX40-PROSE-instrument-note-target`: Reuse existing schema/local/typed-reference result and exact causal identity; do not replace, duplicate or broaden frozen predicate. Remaining: Existing XSD/local/typed-reference owners retain this exact prerequisite: note/instrument id identifies a score-instrument in score-part. Batch 6 reuses the established result and cause; no duplicate failure or new completion claim.
- `MX40-PROSE-instruments-default`: Publish initial instruments=1 count only on proved absence and preserve later inheritance. Count never chooses an instrument ID or limits note selections. Remaining: Batch 7 actual playback realization/repeat-pass semantics and inferred author intent; batch 8 linked parts/sounds resources, batch 9 dictionary closure.
- `MX40-PROSE-midi-instrument-target`: Reuse existing schema/local/typed-reference result and exact causal identity; do not replace, duplicate or broaden frozen predicate. Remaining: Existing XSD/local/typed-reference owners retain this exact prerequisite: midi-instrument id refers to the affected score-instrument. Batch 6 reuses the established result and cause; no duplicate failure or new completion claim.
- `MX40-PROSE-midi-requires-score-instrument`: Reuse existing schema/local/typed-reference result and exact causal identity; do not replace, duplicate or broaden frozen predicate. Remaining: Existing XSD/local/typed-reference owners retain this exact prerequisite: If MIDI channels, banks, or programs are specified, a corresponding score-instrument is required. Batch 6 reuses the established result and cause; no duplicate failure or new completion claim.
- `MX40-PROSE-multi-instrument-note`: Retain each verified note instrument selection independently. Omission advice remains advisory; never inherit the previous note selection or select the first declaration. Remaining: Batch 7 retains optional guidance for this exact recommendation: If a part has multiple score-instrument elements, an instrument element is recommended for each note. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-no-default-tempo`: Retain exact instruments count, typed instrument owners, explicit note assignments and time-scoped sound instrument/play changes, with local versus continuing scope and per-field provenance. Remaining: Batch 6 owns the typed state/origin/scope portion of: MusicXML specifies no default tempo; the standard forte velocity is 90. Batch 7 retains repeat-aware playback realization. Batch 8 retains linked resources and batch 9 external dictionaries.
- `MX40-PROSE-play-instrument-target`: Reuse existing schema/local/typed-reference result and exact causal identity; do not replace, duplicate or broaden frozen predicate. Remaining: Existing XSD/local/typed-reference owners retain this exact prerequisite: A specified play id refers to the particular score-instrument to which playback applies. Batch 6 reuses the established result and cause; no duplicate failure or new completion claim.
- `MX40-PROSE-play-scope`: Retain note-local play separately from continuing positioned sound/play state and verified instrument scope; batch 7 realizes playback. Remaining: Batch 6 owns the typed state/origin/scope portion of: play inside note affects only that note; inside sound it affects subsequent notes in musical order. Batch 7 retains repeat-aware playback realization. Batch 8 retains linked resources and batch 9 external dictionaries.
- `MX40-PROSE-sound-instrument-grouping`: Retain identity-qualified independent instrument updates and original order; grouping advice creates no adjacency or ordering error. Remaining: Batch 7 retains optional guidance for this exact recommendation: It is recommended that multiple instrument-change, midi-device, midi-instrument, and play elements be grouped by instrument id. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-sound-pizzicato-scope`: Retain exact instruments count, typed instrument owners, explicit note assignments and time-scoped sound instrument/play changes, with local versus continuing scope and per-field provenance. Remaining: Batch 6 owns the typed state/origin/scope portion of: sound pizzicato changes all subsequent notes: yes enables pizzicato and no restores arco. Batch 7 retains repeat-aware playback realization. Batch 8 retains linked resources and batch 9 external dictionaries.
- `MX40-PROSE-sound-tempo-dynamics`: Retain exact instruments count, typed instrument owners, explicit note assignments and time-scoped sound instrument/play changes, with local versus continuing scope and per-field provenance. Remaining: Batch 6 owns the typed state/origin/scope portion of: tempo is measured in quarter notes per minute; dynamics is a percentage of MIDI forte=90. Batch 7 retains repeat-aware playback realization. Batch 8 retains linked resources and batch 9 external dictionaries.

### Offset

- `MX40-PROSE-bezier-precedence`: Provide exact signed positions for direction/sound/listening and note-local attack/release/bend offsets; preserve precedence and visual-versus-performance channels without altering the score cursor. Remaining: Batch 6 owns the typed state/origin/scope portion of: bezier-x takes precedence over bezier-offset, and x2 similarly takes precedence over offset2; the values are not added. Batch 7 retains layout coordinates, playback transformation and unresolved note association. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-direction-note-association`: Provide exact signed positions for direction/sound/listening and note-local attack/release/bend offsets; preserve precedence and visual-versus-performance channels without altering the score cursor. Remaining: Batch 6 owns the typed state/origin/scope portion of: When a direction is associated with a note, use the first following note in musical order that is not in another voice. Batch 7 retains layout coordinates, playback transformation and unresolved note association. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-directive-position-precedence`: Provide exact signed positions for direction/sound/listening and note-local attack/release/bend offsets; preserve precedence and visual-versus-performance channels without altering the score cursor. Remaining: Batch 6 owns the typed state/origin/scope portion of: default-x, justify or halign overrides directive positioning. Batch 7 retains layout coordinates, playback transformation and unresolved note association. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-figured-bass-anchor`: Provide exact signed positions for direction/sound/listening and note-local attack/release/bend offsets; preserve precedence and visual-versus-performance channels without altering the score cursor. Remaining: Batch 6 owns the typed state/origin/scope portion of: figured-bass is anchored to the first following ordinary note, excluding grace and chord; figures run from top to bottom. Batch 7 retains layout coordinates, playback transformation and unresolved note association. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-grace-time-semantics`: Provide exact signed positions for direction/sound/listening and note-local attack/release/bend offsets; preserve precedence and visual-versus-performance channels without altering the score cursor. Remaining: Batch 6 owns the typed state/origin/scope portion of: steal-time-previous and steal-time-following are percentages of the neighboring note’s time; make-time adds time in divisions. Batch 7 retains layout coordinates, playback transformation and unresolved note association. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-listening-offset`: Retain listening offset relative to its note/direction origin, with local override and known zero versus absent provenance; no score cursor movement. Remaining: Batch 6 owns the typed state/origin/scope portion of: The listening offset overrides the direction offset; the change applies subsequently in performance time. Batch 7 retains layout coordinates, playback transformation and unresolved note association. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-offset-scope-precedence`: Preserve exact signed offset, divisions/origin and visual/sounding selector separately; default-x does not erase unrelated playback timing. Remaining: Batch 6 owns the typed state/origin/scope portion of: offset is measured from the current position in the current measure; sound defaults to no. Batch 7 retains layout coordinates, playback transformation and unresolved note association. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-release-offset-origin`: Provide exact signed positions for direction/sound/listening and note-local attack/release/bend offsets; preserve precedence and visual-versus-performance channels without altering the score cursor. Remaining: Batch 6 owns the typed state/origin/scope portion of: A release offset is measured from the current note; bend first-beat and last-beat continue to be measured from the note’s original position. Batch 7 retains layout coordinates, playback transformation and unresolved note association. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-sound-offset-precedence`: Retain local sound offset override of parent direction offset and distinct playback position; this never moves the score cursor. Remaining: Batch 6 owns the typed state/origin/scope portion of: A sound offset overrides a direction offset; midi-instrument pan and elevation take precedence over the deprecated sound attributes. Batch 7 retains layout coordinates, playback transformation and unresolved note association. No ordinary typed-state portion is deferred by that consumer split.
- `MX40-PROSE-tie-attack-release`: Supply relevant offset facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: release is discouraged on a note with tie start, and attack is discouraged on a note with tie stop. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.

### Repeat Span

- `MX40-PROSE-accidental-defaults`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: The editorial and cautionary attributes of accidental default to no. accidental describes the printed sign separately from pitch/alter. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-arpeggio-default`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: Without direction, an arpeggio proceeds from the lowest note to the highest; number associates joint or separate arpeggiation. unbroken is an optional cross-staff hint; its absence is not an error. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-barline-boundary-order`: Supply relevant repeat_span facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: It is recommended that a left barline be first and a right barline be last, except for print, bookmark, and link. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-barline-location`: Supply real score positions and measure occurrence/closure evidence. Batch 7 retains semantic boundary/location interpretation; no XML-first/last-child rule. Remaining: Musical-position meaning and zero-duration metadata interpretation remain unapproved. Batch 7 owns repeat/span correspondence, semantic event equivalence, layout/intent decisions and full consumer closure. Joint rows retain [6,7] until explicitly reconciled.
- `MX40-PROSE-beam-group-identity`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: beam number is the beam level, not an ID for an overlapping group; groups are distinguished by voice and grace/cue. For 1024th, there are up to eight levels; the contradiction with the older beam-level wording is noted separately. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-beat-repeat-content`: Supply exact positioned/normalized events and staff/voice ranges. Batch 7 owns endpoint correspondence and semantic equivalence; notation never supplies missing music. Remaining: Duration-normalized voice/staff repeat ranges and event-equivalence remain unavailable. Batch 7 owns repeat/span correspondence, semantic event equivalence, layout/intent decisions and full consumer closure. Joint rows retain [6,7] until explicitly reconciled.
- `MX40-PROSE-beat-repeat-stop`: Supply relevant repeat_span facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: Specifying the start and end of beat-repeat is recommended, except for repeats continuing to the part’s end. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-bend-release-negative`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: bend-alter for a release following a bend is negative, rather than zero. The value is a change in semitones; microtonal fractions are permitted. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-cue-silent`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: The presence of cue makes the note silent regardless of its full/cue graphical size. A cue size without a cue element does not by itself silence the note. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-dot-placement-condition`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: dot placement is ignored if the note is in a space between staff lines. The number of dot elements determines the notated extension of duration, but it does not have to agree with the intended duration. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-double-note-tremolo-duration`: Supply relevant repeat_span facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: For a double-note tremolo, the recommended duration of each note is half the duration represented by type, with time-modification actual=2/normal=1. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-except-voice`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: Notes in except-voice are displayed in ordinary notation in addition to the slashes that are always shown. Do not treat this as disabling slash notation for the entire part. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-measure-numbering-implicit`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: implicit=yes suppresses the measure number regardless of measure-numbering; otherwise text is used first, followed by number. The default reference staff is 1; multiple-rest attributes apply when numbering is not none. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-measure-repeat-content`: Supply measure occurrences and exact normalized event ranges. Batch 7 owns correspondence and semantic equivalence; notation never supplies missing music. Remaining: Musical repeat ranges and event-equivalence remain unavailable. Batch 7 owns repeat/span correspondence, semantic event equivalence, layout/intent decisions and full consumer closure. Joint rows retain [6,7] until explicitly reconciled.
- `MX40-PROSE-measure-repeat-stop`: Supply relevant repeat_span facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: Specifying start and stop for measure-repeat is recommended, except for repeats continuing to the part’s end. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-measure-rest`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: rest measure=yes denotes a rest lasting the entire measure. This does not imply a requirement for the graphical type=whole or for a standard fixed length in pickup or senza-misura cases. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-measure-style-all-staves`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: Without number, measure-style applies to all staves of the part. An explicit number selects one staff. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-midi-device-target`: Reuse existing schema/local/typed-reference result and exact causal identity; do not replace, duplicate or broaden frozen predicate. Remaining: Existing XSD/local/typed-reference owners retain this exact prerequisite: A specified midi-device id refers to the score-instrument for that assignment. Batch 6 reuses the established result and cause; no duplicate failure or new completion claim.
- `MX40-PROSE-non-arpeggiate-endpoints`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: non-arpeggiate is placed on the upper and lower notes of the bracket, rather than on every note of the chord. The sign is graphical; it is not an independent playback change. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-normal-dots-reference`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: normal-dot specifies the dots of the tuplet's normal type, rather than the dots of the current note. type/dot and normal-type/normal-dot must not be conflated when calculating the ratio. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-normal-type-when-different`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: If the normal note type differs from the current one, it is described by normal-type and normal-dot. The type may be omitted when it is the same. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-note-type-size-default`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: The size of a regular note is full; grace+cue is grace-cue; grace alone or cue alone is cue. Do not conflate the visual-size default with the actual duration. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-octave-middle-c`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: Octave number 4 denotes the octave beginning with middle C. XSD separately restricts the range to 0 through 9. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-parallel-print-position`: Supply relevant repeat_span facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: In multi-part music, placing new-system and new-page at matching positions in the parts is recommended. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-part-symbol-default`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: The default symbol for a multi-staff grouping is brace; by default, a partially spanning bracket also changes the shared barlines. top-staff and bottom-staff limit the span. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-pitch-alter-explicit`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: The alteration needed for the actual pitch is encoded in pitch/alter, even if already specified in the key signature. A natural note may omit alter; alter cannot be required for every pitch. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-semitone-alteration`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: alter and related values specify semitones; fractions are used for microtones. Values must not be restricted to only -2 through 2 or to integers. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-slash-beat-default`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: If slash-type and slash-dot are absent, the beat for repeat signs is determined by the current time. Do not invent a default meter when time is absent. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-slash-start-stop`: Supply relevant repeat_span facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: For slash notation, start and stop are recommended, except when it continues to the part’s end. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-sound-fine-duration`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: fine follows the last note or rest: a number specifies the actual final duration, while yes leaves it unchanged. Do not require this duration to agree across parts or voices without a separate norm. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-start-stop-continue-order`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: continue denotes an intermediate point of an extended sign; start and stop retain their musical meaning. Do not reject a document solely because stop precedes start in the XML. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-start-stop-score-order`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: start and stop denote the musical beginning and end, rather than their order in the XML. stop may be encountered before start when traversing multiple staves. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-staves-default`: Publish initial staves=1 only on proved absence; preserve explicit zero/large counts and inherited values after later omission; never enumerate arbitrary counts. Remaining: Batch 7 owns repeat/span correspondence, semantic event equivalence, layout/intent decisions and full consumer closure. Joint rows retain [6,7] until explicitly reconciled.
- `MX40-PROSE-stop-before-start`: Supply relevant repeat_span facts where useful; split context/default subclauses must receive their own explicit binding. Recommendation alone never adds normative failure. Remaining: Batch 7 retains optional guidance for this exact recommendation: Reading must allow stop to precede its corresponding start in the XML. Its mandatory default/context subclauses, where present, remain separately owned by the split/shared checks; underlying XSD domains retain their authoritative owners.
- `MX40-PROSE-swing-applicability`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: swing does not apply to grace notes, notes without type, or notes whose duration differs from the nominal duration for type. swing-type defaults to eighth; attack and release adjust playback after swing has been applied. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-tablature-rhythm-required`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: Tablature retains rhythmic data even when its display is hidden. The ordinary XSD note branches apply, including the exception for grace duration. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-time-modification-cumulative`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: time-modification specifies the cumulative sounding ratio of tuplets and double-note tremolo relative to type/dots. Detailed display and nesting require tuplet; the written duration may intentionally differ. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-time-only-ascending`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: time-only specifies positive pass numbers separated by commas in ascending order. XSD covers the lexical pattern, but not numerical ordering. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-tuplet-actual-fallback`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: Unspecified tuplet-actual fields are derived from time-modification. Do not require all sounding fields to be explicitly repeated. Batch 6 still owes the exact chronology/scope prerequisites recorded here.
- `MX40-PROSE-tuplet-normal-fallback`: Supply exact positions, measure boundaries/occurrences, typed part/staff/voice scopes, note branches and normalized durations; preserve consumer-specific missing facts. Remaining: Batch 7 retains the downstream clause: Unspecified tuplet-normal fields are derived from time-modification. Do not impose XSD-required fields where they are optional. Batch 6 still owes the exact chronology/scope prerequisites recorded here.

## Split/shared check ownership

- `CHECK-context-cancel-location-default` (`MX40-PROSE-cancel-before-barline-midmeasure`): Batch 6 exact timeline/state contribution through validation.NewTimelinePlan; Each present key cancel element. Remaining: Batch 6 exact timing/state subclauses plus expressly retained later consumer scope; see source inventory.
- `CHECK-context-beat-repeat-state` (`MX40-PROSE-beat-repeat-stop`): Batch 6 exact timeline/state contribution through validation.NewTimelinePlan; Each present beat-repeat start or stop. Remaining: Batch 6 exact timing/state subclauses plus expressly retained later consumer scope; see source inventory.
- `CHECK-context-measure-repeat-state` (`MX40-PROSE-measure-repeat-stop`): Batch 6 exact timeline/state contribution through validation.NewTimelinePlan; Each present measure-repeat start or stop. Remaining: Batch 6 exact timing/state subclauses plus expressly retained later consumer scope; see source inventory.
- `CHECK-context-symbol-defaults-and-name` (`MX40-PROSE-symbol-semantic-markup`): Retained later context provider: Each symbol used in direction text. Remaining: Batch 7 broader context/rendering/span services, with any external dictionary closure retained at batch 9; not closed by timeline.
- `CHECK-context-assess-target-scope` (`MX40-PROSE-assess-default`): Retained later context provider: Each present note assessment instruction. Remaining: Batch 7 broader context/rendering/span services, with any external dictionary closure retained at batch 9; not closed by timeline.
- `CHECK-context-slash-stop-boundary` (`MX40-PROSE-slash-start-stop`): Batch 6 exact timeline/state contribution through validation.NewTimelinePlan; Each slash notation stop in its style scope. Remaining: Batch 6 exact timing/state subclauses plus expressly retained later consumer scope; see source inventory.
- `CHECK-context-xlink-show-default` (`MX40-PROSE-xlink-show-behavior`): Retained later context provider: XLink show attributes instantiated through the cited attribute group. Remaining: Batch 8 link/resource evidence, batch 9 external closure; do not silently add to timeline profile.
- `CHECK-context-system-number-position-scope` (`MX40-PROSE-system-number-single-part`): Retained later context provider: System numbers using only-top, only-bottom, also-top, or also-bottom. Remaining: Batch 7 broader context/rendering/span services, with any external dictionary closure retained at batch 9; not closed by timeline.
- `CHECK-context-system-mark-part-scope` (`MX40-PROSE-system-mark-single-part`): Retained later context provider: System marks with system=none. Remaining: Batch 7 broader context/rendering/span services, with any external dictionary closure retained at batch 9; not closed by timeline.
- `CHECK-context-relative-position-basis` (`MX40-PROSE-position-resolution`): Retained later context provider: Positioned notation with relative coordinates. Remaining: Batch 7 broader context/rendering/span services, with any external dictionary closure retained at batch 9; not closed by timeline.
- `CHECK-context-image-resource-type` (`MX40-PROSE-image-scaling`): Retained later context provider: Each image element. Remaining: Batch 8 link/resource evidence, batch 9 external closure; do not silently add to timeline profile.
- `CHECK-context-parallel-print-mismatch-outcome` (`MX40-PROSE-parallel-print-position`): Batch 6 exact timeline/state contribution through validation.NewTimelinePlan; Cross-part page or system breaks in multi-part music. Remaining: Batch 6 exact timing/state subclauses plus expressly retained later consumer scope; see source inventory.
- `CHECK-context-arrowhead-geometry` (`MX40-PROSE-arrow-glyph-consistency`): Retained later context provider: Arrow notation using the arrowhead branch. Remaining: Batch 7 broader context/rendering/span services, with any external dictionary closure retained at batch 9; not closed by timeline.
- `CHECK-context-attack-release-units` (`MX40-PROSE-tie-attack-release`): Batch 6 exact timeline/state contribution through validation.NewTimelinePlan; Present note attack or release timing adjustments. Remaining: Batch 6 exact timing/state subclauses plus expressly retained later consumer scope; see source inventory.
- `CHECK-context-single-endpoint-completeness` (`MX40-PROSE-same-note-single-order`): Retained later context provider: Span-like notation using a type domain that includes single. Remaining: Batch 7 broader context/rendering/span services, with any external dictionary closure retained at batch 9; not closed by timeline.
- `CHECK-context-metadata-role-domain` (`MX40-PROSE-encoder-type`, `MX40-PROSE-creator-type`): Retained later context provider: Encoder and creator metadata type fields. Remaining: Batch 7 broader context/rendering/span services, with any external dictionary closure retained at batch 9; not closed by timeline.
- `CHECK-context-measure-label-domain` (`MX40-PROSE-measure-identity`): Batch 6 exact timeline/state contribution through validation.NewTimelinePlan; Measure labels participating in the cross-part consistency recommendation. Remaining: Batch 6 exact timing/state subclauses plus expressly retained later consumer scope; see source inventory.
- `CHECK-context-lyric-hyphen-and-syllabic` (`MX40-PROSE-lyric-literal-hyphen`): Retained later context provider: Lyric content with literal hyphens or explicit syllabic markup. Remaining: Batch 7 broader context/rendering/span services, with any external dictionary closure retained at batch 9; not closed by timeline.
- `CHECK-context-fermata-defaults` (`MX40-PROSE-fermata-upright`, `MX40-PROSE-empty-fermata-normal`): Retained later context provider: Each present fermata notation. Remaining: Batch 7 broader context/rendering/span services, with any external dictionary closure retained at batch 9; not closed by timeline.
- `CHECK-context-transposition-additive` (`MX40-PROSE-transpose-additive`, `MX40-PROSE-diatonic-plus-octave`): Batch 6 exact timeline/state contribution through validation.NewTimelinePlan; Each applicable transpose state and written note pitch in its staff scope. Remaining: Batch 6 exact timing/state subclauses plus expressly retained later consumer scope; see source inventory.
