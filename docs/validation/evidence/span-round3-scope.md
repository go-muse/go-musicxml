# Span scope repair: PR36 Round 3

The baseline for these findings is `0821afd31cc0c783db53b351096ea9e76bd060dc`.
These changes narrow unavailable correspondence; they add no validity predicate,
playback rule, ordering heuristic, or default lyric line.

## 5. Local tied stubs require enumerated competitor scope

The existing one-note visual-tie interpretation is intentionally limited to an
isolated exact identity. Part-level and unknown-musical-domain enumeration gaps
must therefore be merged before the local-pair prepass. A complete note cannot
prove that an unenumerated measure contains no competing boundary.

The admitted-stream test supplies an established canonical part identity and
separately checks partwise and timewise hierarchy gaps, `ChildrenComplete` and
`Order`, and Unknown/Invalid/Unsupported states with their exact causal reference.
A partwise gap leaves the other known part independent; a timewise measure gap
with no single part domain reaches both musical parts.

Actual source/current-model adapters retain their earlier
`identity-scope-incomplete` gate when hierarchy enumeration is unavailable. The
older `local-tie-stub-survives` test contradicted the competitor dependency and is
renamed to `local-tie-stub-needs-domain-enumeration`, with that exact public
causal boundary asserted. Self-contained `let-ring` membership remains Known.
These two test methods do not claim the same prerequisite was available.

## 6. Refused same-note interpretations are order-independent

The [MusicXML 4.0 tied reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/tied/)
and the pinned `tied` type in `schema/musicxml-4.0/musicxml.xsd` distinguish
notation from tie sound and describe a same-note start/stop spelling for one-note
visual ties. The accepted implementation deliberately declines to identify that
spelling as a stub when its exact identity also has competing remote boundaries.
The [number-level reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/data-types/number-level/)
also distinguishes document order from musical order. Neither is a general
license to discard an unresolved pair and greedily recover the remaining chain.

The correction preflights the complete exact tied identity run before publishing
any candidate pairs. An unresolved same-note start followed by stop makes that
run Unknown with `span-tied-local-interpretation-ambiguous`, regardless of where
it appears. It does not establish that another candidate is independent merely
because its positions are Known. The implementation makes no new independence
theorem from positions, missing boundaries, or unencoded sound evidence.
Different numbers, actual part domains, and independently established implied
pitch identities remain separate. Ordinary same-note stop/start chains and
isolated visual stubs retain their previous behavior.

Tests cover all six permutations of a same-note pair and two remote boundaries,
known/unavailable positions, separate numbers/parts/implied pitches, and authentic
source/current-model fixtures in partwise/timewise hierarchies.

## 7. Wholly unnamed lyric uniqueness is scoped to the musical part

The pinned `lyric` type and [official lyric reference](https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/lyric/)
make both `number` and `name` optional without a default. The
[official tutorial](https://www.w3.org/2021/06/musicxml40/tutorial/midi-compatible-part/)
uses those attributes to distinguish multiple verses. They do not specify a
selection or reset rule for multiple wholly unnamed lyric siblings. No primary
discussion establishing such a rule was found during this repair. Informal
application-specific name/number matching is not adopted as normative evidence.

A wholly unnamed lyric remains its own absent-number/absent-name identity,
separate from verse `1` and from an explicitly empty name. Duplicate wholly
unnamed lyrics on any note, including notes with no extend, conservatively
withhold only that identity in the same musical part. The result is identical
whether the duplicate note occurs before, between, or after the extend endpoints.
There is no inferred line reset at an extend start, note, or measure boundary.
An unproved part domain can compete with unnamed runs in all musical parts.
Unavailable lyric identity evidence can also be unnamed when neither attribute
independently excludes absence; its exact evidence and cause propagate instead
of becoming a synthetic duplicate diagnosis. A separately Known string number
or name excludes the wholly unnamed identity even when the other attribute is
unavailable. This includes an explicitly empty name, but not a mistyped Known
atomic value. The affected endpoint's full identity remains unavailable with its
own causal evidence; the independent exclusion does not repair that identity.
Unavailable lyrics contribute their own evidence to the unnamed candidate rather
than a shared endpoint-local wildcard bucket, which can contain independently
excluded siblings. Inserting excluded named siblings before or after the actual
possible candidate cannot change the candidate's evidence state or cause.

This is a bounded correspondence policy, not a declaration that such lyrics are
invalid. Independently explicit name/number identities retain the existing exact
attribute key and endpoint-local duplicate check. An unnamed duplicate does not
contaminate those explicit identities or an independently known other part.
Tests retain that distinction, including number `01`, explicit empty name,
invalid empty number with a matching source/model causal finding, and admitted
unknown-domain competition.

## Resource contract and validation

The new tied preflight scans each identity run once. Its temporary note index
releases on success, budget refusal, and cancellation. The unnamed lyric-domain
index is built during the existing lyric traversal, is queried once per unnamed
endpoint, and shares the provider's released scratch storage. Lookups and string
keys are charged; neither path rescans the whole score for each endpoint.

Focused tests include exact/one-under buffer caps, linear-work checks, and
cancellation cleanup at 100 and 1000 records. Full-suite and release-stage
validation remain the parent task's coordinated responsibility.
