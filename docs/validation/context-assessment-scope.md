# Bounded contextual assessment

The opt-in `musicxml-4.0/source-context` and `musicxml-4.0/model-context`
profiles extend the frozen timeline profiles. They preserve the source/model,
XSD, local-rule, typed-reference and timeline contracts. Ordinary `Decode`,
`Validate`, and earlier assessment profiles are unchanged.

## Meaning of the result

`Report.Context` contains caller-owned snapshots. It is produced before session
cleanup, under the session's work, fact and retention budgets. Editing a returned
record cannot change its producer or another assessment. Like the rest of
`Report`, the exported fields are editable; they are not a signed certificate.

The selected services project local defaults and applicability, type-specific
span correspondence, graphical-repeat scopes, bounded navigation, layout
applicability and encoded performance/timing context. A provider's `Pass` means
its extraction and projection completed. It does not assert that every optional
field is known. Every field retains its own evidence state and causal source.
Source readers use observed input; direct-model readers use current fields and
presence. Model assessment does not serialize and reparse the model.

Defaults apply only to known absence. Invalid prerequisites retain a causal
`DecisionRef`; unavailable or unsupported prerequisites do not become absence.
The selected timeline dependency schedules the exact accepted provider, but an
unavailable timeline field does not suppress independent local facts. A missing
required attribute can retain its containing XSD decision only when the compiled
use is proved against the exact declaration owner, assembly and schema digest.
Inherited required uses without that exact ownership proof remain unsupported;
this bounded causal repair does not reinterpret model zero values as source
absence.

## Mandatory consumers

The context profiles select additional narrow predicates:

- Present `time-only` values are compared as exact positive integers. Descending
  adjacent values fail independently of repeat reachability. Equal adjacent
  values retain the unresolved duplicate policy and remain incomplete. Integer
  comparison does not depend on machine width or expand playback traversals.
- A release in an established supported bend gesture must have negative
  `bend-alter`. A lone or ambiguous release does not establish that antecedent.
- Graphical beat and measure repeats consume only proved musical-content
  correspondence. They do not synthesize missing notes. Unresolved equivalence
  is incomplete rather than a guessed failure or success.
- Nonempty ASCII-digit `figure-number` content passes a sufficient numeric subset,
  supported by the source requirement and official example. Other spellings retain
  an explicit unsupported grammar gate; no full lexical domain or rejection rule
  is invented. XSD `style-text` validity alone is not numeric proof.

A missing optional node does not instantiate its node-local predicate.
Resource exhaustion is incomplete assessment, never a MusicXML defect.

## Deliberate boundaries

Span families have separate identities and applicability. There is no universal
balance test, LIFO pairing rule, or staff/voice partition. Encoded document order
and musical endpoint order are separate. Refused same-note tied interpretations
withhold their exact identity run regardless of document placement. Duplicate
wholly unnamed lyric lines withhold that unnamed identity within the musical
part; explicit name/number identities remain separately scoped. These are
conservative correspondence limits, with no invented verse default or line reset.
[Span scope repair evidence](evidence/span-round3-scope.md) records source rationale,
causal boundaries, and controls. Graphical repetition is separate from
playback navigation. Unproved repeat/jump policy retains unavailable evidence;
there is no invented absent `times=2` or `after-jump=no` rule.

Layout records expose source-defined scope, default provenance and applicability,
not an engraver or application-selected physical geometry. Performance records
expose encoded/default facts, not a physical player. Written pitch/transposition
intent cannot be inferred from a part name, key signature or accidental display.
Absent optional authoritative intent is preserved as a bounded exclusion.

MXL/package processing, linked resources, sound containers, dictionaries and
transitive external normative closure remain outside these profiles. Opus
assessment projects each local Opus version (omission defaults to 1.0) and score
pagination hints, including nested identities; omitted new-page is application-dependent, and no
linked score is fetched. The accepted timeline's narrow research
boundaries remain intact. `B5-PERF-01` remains assigned to the final kernel/session
reconciliation; this batch does not close it.

Historical planning records retain their original statuses. New executable
contracts, source interpretation evidence and actual tests must be reconciled
separately. Counts of rules, projected facts or fixtures are not a MusicXML
coverage denominator.

## Graphical beat-repeat terminal extent

An absent stop can be bounded by complete encoded part closure only when the
start and the final encoded extent are in the same structural measure. This
existing bounded interpretation uses the maximum unshifted event origin plus
positive non-backup duration: ordinary notes and forwards contribute
symmetrically, while a backup does not add its duration and cannot erase an
already observed extent. Display/effect offsets and nominal meter do not extend
this bound. A synthesized duration endpoint retains `UnprovedFrame`.

This is an encoded-content comparison bound, not proof of measure capacity,
frame containment, sounding silence, or the author's intent when padding a
voice. A trailing forward can therefore expose an empty repeated beat and yield
`ContentEquivalent = Known(false)` and the encoded-repeat-content consumer's
failure, even without independent containment of the forward destination.
Ordinary terminal note endpoints have the same bounded treatment. Cross-measure
part-end ranges, incomplete part closure, unavailable extent and nonintegral
beat windows retain unavailable evidence. No required stop, meter-fill rule or
forward-exclusion policy is introduced. The [source argument and exact
fixtures](evidence/context-round3-repeat-review.md) distinguish this comparison
from the unchanged timeline endpoint/frame contract.

## Practical default limits

The repository's official MusicXML 4.0 samples `MozartTrio.musicxml` (262 notes)
and `MozaChloSample.musicxml` (278 notes) do **not** complete either context
profile under the default budgets. Both stop at the 20,000,000-step limit;
raising work alone still reaches the default 64 MiB buffered-byte limit.
Explicit finite `MaxRuleSteps: 1 << 34` and `MaxBufferedBytes: 512 << 20`
complete the measured source/current-model cases. Source measurements remove
only the unsupported DOCTYPE; MozaChlo's UTF-16BE BOM and encoding declaration
are preserved. Unmodified DOCTYPE-bearing source remains unsupported by the
source assessment contract.

The corrected Mozart model run requires 40,569,928 steps and a measured
94,263,641-byte buffer minimum; MozaChlo requires 46,302,398 steps and
103,134,552 bytes. [Matched Round 3 resource acceptance](evidence/context-round3-resource-acceptance.md)
records baseline/final results, exact finite configuration, source treatment,
public outcomes, one-under controls and post-Finish cleanup. Corrected passive
navigation handling reaches additional encoded occurrences and retains their
owned state snapshots; the official-sample work/fact increase is reconciled
against those actual occurrences rather than hidden by larger defaults. These two official
files and synthetic fixtures do not establish replicated-score or universal
throughput guarantees.

A clean 100-note fixture completes under unchanged default budgets after the
context indexing corrections. The measured 1000-note fixture now stops at the default 64 MiB buffered-byte
limit before exhausting default work (source: 12,994,218 steps; model:
13,699,811 steps). Raising work alone does not remove that retention limit. Explicit
finite limits of `MaxRuleSteps: 1 << 34` and `MaxBufferedBytes: 512 << 20` allow
that measured fixture to finish. This is bounded-profile evidence, not an
unrestricted score-size guarantee. See [current resource acceptance evidence](evidence/context-round3-resource-acceptance.md)
for exact counts, budget outcomes and remaining output-size costs.
