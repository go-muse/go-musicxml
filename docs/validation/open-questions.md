# Open questions and proposed dispositions

This English review guide summarizes all **28 stable issue IDs** in
[the preserved issue registry](registry/issues.json). The dispositions below are
research proposals, **not approved project policy**, implemented behavior, or new
MusicXML requirements. Every issue record has `hard_error_basis: false`: an issue
is not itself authority to reject a document. Clear formal constraints may still
be enforced on their own evidence.

The issues mix editorial discrepancies, interpretation questions, and unfinished
external dependencies. They are not all blockers to starting implementation.
An issue-to-rule link normally means shared source context, not that every clause
on that page is disputed; see [reconciliation](registry/reconciliation.json).
Before implementing a disputed predicate, record its clause-level interpretation,
scope, evidence, and disposition. Where evidence remains insufficient, preserve
that uncertainty rather than inventing a hard error. The source registry retains
the exact locators and original bilingual descriptions; this guide does not alter
it.

## Editorial and structural discrepancies

### MX40-ISSUE-beam-six-eight

The `beam-level` annotation describes six levels, while its facet and the `beam`
and `beam-value` descriptions support eight. **Proposed disposition:** use the
explicit XSD range 1–8; do not turn the outdated phrase into a maximum of six.
Sources: [beam-level][mx43], [beam-value][mx1607], [beam][mx4718].

### MX40-ISSUE-cue-duration

The `full-note` annotation says cue notes lack `duration`; the formal `note`
branches and its prose require duration for cue notes and omit it for grace
notes. **Proposed disposition:** follow the structural contract, supported by the
`note` prose, while retaining the discrepancy.
Sources: [full-note][mx6492], [note][mx5162].

### MX40-ISSUE-container-schema-exists

The compressed-file tutorial says there is no `container.xsd`, whereas the 4.0
release notes announce it and the schema exists. **Proposed disposition:** treat
the tutorial claim as outdated; include `container.xsd` in the 4.0 scope.
Sources: [compressed-file tutorial][compressed], [4.0 release notes][history40],
[container schema][container4].

### MX40-ISSUE-container-sounds-wording

The `container` type description calls it the root of a sounds file, while the
global `container` declaration describes `META-INF/container.xml`.
**Proposed disposition:** follow the explicit structure and matching root
annotation; do not allow `sounds` as the container-document root.
Sources: [container type][container108], [container root][container140].

### MX40-ISSUE-container-schema-uri

The introductory `container.xsd` prose gives a `sounds.xsd` URI for validation.
**Proposed disposition:** record a *probable* editorial mistake and use the
pinned `container.xsd`, without substituting the sounds schema.
Source: [container schema introduction][container4].

### MX40-ISSUE-xml-declaration-required

The tutorial describes an XML declaration as required for every XML document.
**Proposed disposition:** do not introduce a rejection for an absent declaration
from this statement; it does not follow from the MusicXML XSD. The dependent XML
specification was not separately audited by this prose pass.
Source: [Hello World tutorial][hello].

### MX40-ISSUE-every-note-duration

The tutorial generalizes its example to duration on every note, overlooking
grace notes. **Proposed disposition:** apply the actual `note` branches, without
a universal duration-required rule that also covers grace notes.
Sources: [Hello World tutorial][hello], [note][mx5162].

### MX40-ISSUE-duration-integer

The tutorial calls duration an integer, while XSD and the reference allow
decimal-based `positive-divisions`. **Proposed disposition:** accept fractional
duration values within the formal domain; treat integer values as a compatibility
recommendation.
Sources: [MIDI-compatible tutorial][midi], [divisions][mx90], [duration][mx6472].

### MX40-ISSUE-old-note-type-list

The tutorial's list runs from `256th` to `long`; the 4.0 schema also includes
`1024th`, `512th`, and `maxima`. **Proposed disposition:** use the complete 4.0 XSD
enumeration, not the older illustrative list.
Sources: [notation tutorial][notation], [note-type-value][mx1744].

### MX40-ISSUE-numeral-step-typo

The `kind-value` advice refers to empty `text` on a nonexistent `numeral-step`;
the schema has `numeral-root`. **Proposed disposition:** retain the editorial
ambiguity. `numeral-root` is the *probable* intended name, not a confirmed
correction; do not invent a new element or field.
Sources: [kind-value][mx1039], [numeral-root][mx3915].

### MX40-ISSUE-assess-type-default

The HTML `assess` table describes a default when `type` is omitted, even though
XSD requires that attribute. **Proposed disposition:** interpret the contextual
assessment default as absence of an `assess` override, not permission for an
`assess` element without its required `type`.
Sources: [assess reference][assess], [assess type][mx4699].

### MX40-ISSUE-midi-instrument-parent-wording

Some prose calls `midi-instrument` a child of `score-instrument`; the formal
structure and `score-part` description place the initial assignment in
`score-part`. **Proposed disposition:** use the XSD content model; the imprecise
introductory wording does not permit otherwise invalid nesting.
Sources: [midi-instrument][mx2676], [score-instrument][mx6048],
[score-part][mx6072].

### MX40-ISSUE-legacy-tutorial-header-root-list

The tutorial gives an older count of header children and says every chord symbol
has `root`; 4.0 includes `defaults`/`credit` and the `root`/`numeral`/`function`
choice. **Proposed disposition:** do not narrow the full 4.0 structural contract
using descriptions of particular examples.
Sources: [file structure][structure], [chord-symbol tutorial][chords],
[harmony group][mx6408], [score header][mx6549].

### MX40-ISSUE-pull-off-attribute-descriptions-swapped

The HTML descriptions for `pull-off`'s `type` and `number` are swapped.
**Proposed disposition:** follow the unambiguous XSD/`hammer-on-pull-off` model:
`type` uses start/stop and `number` uses `number-level`. Do not generate predicates
from the mistaken table descriptions.
Sources: [pull-off reference][pull-off], [hammer-on-pull-off][mx4906].

### MX40-ISSUE-pizzicato-nonexistent-element

The note's `pizzicato` description refers to a nonexistent `pizzicato` element;
the general mode is the `pizzicato` attribute on `sound`. **Proposed disposition:**
do not add an element. Use the `sound` prose to distinguish note-level and general
scope.
Sources: [note reference][note], [note type][mx5162], [sound][mx4125].

### MX40-ISSUE-time-beat-spelling

The HTML `time` prose says `beat`/`beat-type`, while the structural name is
`beats`/`beat-type`. **Proposed disposition:** treat this as editorial shorthand
or a mistake; use the XSD's expanded names.
Sources: [time reference][time], [time type][mx3180].

## Interpretation and scope questions

### MX40-ISSUE-part-id-uniqueness-scope

The partwise example's statement about a unique ID for every part does not
establish how repeated timewise `part` references are scoped.
**Proposed disposition:** `score-part` ID uniqueness is document-wide; timewise
`part` IDREF values may recur across measures. Additional uniqueness among sibling
parts requires a separately supported rule.
Sources: [Hello World tutorial][hello], [part attributes][mx2407],
[score-timewise][mx6614].

### MX40-ISSUE-left-barline-attributes

The reference recommends a left barline first, except for `print`/`bookmark`/`link`,
but the tutorial puts it after `attributes`. **Proposed disposition:** do not
implement a hard XML-first-child check. A musical zero-position reading is
possible, but interpretation of the recommendation and required location remains
to be resolved explicitly.
Sources: [MIDI-compatible tutorial][midi], [barline][mx3227].

### MX40-ISSUE-chord-preceding-duration

The reference uses “preceding note” and the first preceding non-chord note for
time movement; the tutorial puts the longest note first.
**Proposed disposition:** the duration bound against the base note is supported.
A pairwise non-increasing order for every chord tone is *not proven*. Do not reject
a shorter tone followed by a longer tone when both fit the base solely on that
unresolved reading.
Sources: [chord documentation][mx6497], [MIDI-compatible tutorial][midi].

### MX40-ISSUE-initial-effective-context

XSD permits omission of divisions, key, time, and clef, without universal initial
defaults for all of them. **Proposed disposition:** do not invent C major, 4/4,
treble clef, or divisions = 1 as format requirements. Distinguish unknown effective
context from a failed constraint.
Sources: [attributes][mx2815], [divisions context][mx2822], [defaults][mx5865].

### MX40-ISSUE-span-pairing-scope

Start/stop/continue and overlap identifiers are described, but no universal MUST
requiring every span type to form fully balanced pairs was established.
**Proposed disposition:** do not strengthen “most” or “should” into universal
balance. Account for repeats, let-ring, cross-system spans, and stop-before-start
ordering when defining individual predicates.
Sources: [number-level][mx287], [slur][mx5465], [tied][mx5661].

### MX40-ISSUE-measure-duration-completeness

No universal rule was found requiring each measure or voice to equal the time
signature's duration. **Proposed disposition:** pickup, implicit, non-controlling,
and senza-misura contexts prevent that blanket rule; any completeness heuristic
belongs in an explicit application profile, not an asserted MusicXML MUST.
Sources: [measure attributes][mx2385], [time/senza-misura][mx3191],
[duration][mx6472].

### MX40-ISSUE-zero-counts-and-ratios

`nonNegativeInteger` allows zero for `staves`, `actual-notes`, and `normal-notes`;
scaling `tenths` also permits zero, without a prose prohibition in every case.
**Proposed disposition:** do not silently strengthen these domains to positive
values. A computation with a zero denominator needs an unsupported/undefined
outcome rather than an invented format rule.
Sources: [staves][mx2840], [time-modification][mx5685], [scaling][mx4471].

## Package and external-dependency questions

### MX40-ISSUE-mxl-methods-compatibility

The prose describes DEFLATE for container files and requires stored `mimetype`,
but does not separately enumerate every allowed method for other entries.
**Proposed disposition:** do not claim full ZIP validation from these paragraphs;
the external ZIP/JAR corpus needs review for that claim.
Sources: [container introduction][container4], [compressed-file tutorial][compressed].

### MX40-ISSUE-mxl-first-media-type-set

The first rootfile cannot use a non-MusicXML media type, but this condition does
not specify the complete MusicXML media-type set or resolve nested compressed
roots. **Proposed disposition:** check a direct XML root separately; clarify the
permitted media-type set rather than inventing equality to one MIME value.
Sources: [rootfile][container128], [compressed-file tutorial][compressed].

### MX40-ISSUE-smufl-version-dictionary

Canonical glyph constraints depend on the external SMuFL dictionary. The 4.0
release mentions SMuFL 1.4, but the full dictionary was not reviewed here.
**Proposed disposition:** canonical-name existence remains unknown without a
pinned dictionary; the XSD prefix constraints can still be checked.
Sources: [SMuFL glyph names][mx413], [SMuFL notehead names][mx1766],
[4.0 release notes][history40].

### MX40-ISSUE-ipa-character-repertoire

IPA 2015 in Unicode 13.0 constrains content, but the exact allowed repertoire and
combining characters were not enumerated in this pass.
**Proposed disposition:** keep the external corpus work pending; an arbitrary
Unicode-block check cannot support a claim of complete IPA validation.
Sources: [IPA pronunciation][mx2755], [4.0 release notes][history40].

### MX40-ISSUE-external-spec-closure

Complete semantics for XML, XSD datatypes, XLink, XML Base/ID, ZIP/JAR/DEFLATE,
SMuFL, and IPA live in external specifications. **Proposed disposition:** do not
claim complete transitive normative closure. The prose researcher did not fetch
and review that entire external corpus; the XSD-side language summaries do not
close this gap. Scope any additional review to the capabilities actually claimed.
Sources: [XML module][xml5], [XLink module][xlink4],
[container introduction][container4].

[hello]: https://www.w3.org/2021/06/musicxml40/tutorial/hello-world/
[compressed]: https://www.w3.org/2021/06/musicxml40/tutorial/compressed-mxl-files/
[history40]: https://www.w3.org/2021/06/musicxml40/version-history/40/
[midi]: https://www.w3.org/2021/06/musicxml40/tutorial/midi-compatible-part/
[notation]: https://www.w3.org/2021/06/musicxml40/tutorial/notation-basics/
[structure]: https://www.w3.org/2021/06/musicxml40/tutorial/structure-of-musicxml-files/
[chords]: https://www.w3.org/2021/06/musicxml40/tutorial/chord-symbols-and-diagrams/
[assess]: https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/assess/
[pull-off]: https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/pull-off/
[note]: https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/note/
[time]: https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/time/
[container4]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/container.xsd#L4
[container108]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/container.xsd#L108
[container128]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/container.xsd#L128
[container140]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/container.xsd#L140
[xml5]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/xml.xsd#L5
[xlink4]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/xlink.xsd#L4
[mx43]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L43
[mx90]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L90
[mx287]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L287
[mx413]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L413
[mx1039]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L1039
[mx1607]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L1607
[mx1744]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L1744
[mx1766]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L1766
[mx2385]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2385
[mx2407]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2407
[mx2676]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2676
[mx2755]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2755
[mx2815]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2815
[mx2822]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2822
[mx2840]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L2840
[mx3180]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L3180
[mx3191]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L3191
[mx3227]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L3227
[mx3915]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L3915
[mx4125]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L4125
[mx4471]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L4471
[mx4699]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L4699
[mx4718]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L4718
[mx4906]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L4906
[mx5162]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L5162
[mx5465]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L5465
[mx5661]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L5661
[mx5685]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L5685
[mx5865]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L5865
[mx6048]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L6048
[mx6072]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L6072
[mx6408]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L6408
[mx6472]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L6472
[mx6492]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L6492
[mx6497]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L6497
[mx6549]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L6549
[mx6614]: https://github.com/w3c-cg/musicxml/blob/799e2defb2ece0ae7bafe08dcbcac25b2c631d53/schema/musicxml.xsd#L6614
