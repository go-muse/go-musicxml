# XSD 1.0 attribute wildcard extension union

## Normative scope

The pinned [XSD 1.0 Structures Second Edition, 3.4.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#Complex_Type_Definition_details)
constructs an extension's attribute wildcard by unioning the base and complete
local wildcard namespace constraints. When both exist, the local complete
wildcard supplies `processContents`; an absent local wildcard inherits the base
wildcard, including its mode. A restriction instead uses its own complete
wildcard. The [3.10.6 union rules](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cos-aw-union)
include finite set union, absorption by `any`, and explicitly inexpressible
negation/set combinations. Namespace admission remains separate from declaration
lookup and value validation.

The previous `MergeAnyAttribute` simply selected the derived wildcard, producing
a definitive forbidden-attribute issue for an attribute in the base namespace.
The test-first commit `c48afd1` reproduces that failure against integrated base
`ed3774adf265ecf6cb912c414333c518bf68a989`. Its failing cases include simple and
complex content, inheritance, base `##any`, process modes, and an independently
preserved content witness.

## Implemented boundary

- Exact finite namespace-list unions, including `##local`, preserve first-seen
  order and deduplicate exact token spellings. Case, URI escape spellings and
  non-XML whitespace inside a token are not normalized into another namespace.
- An explicit or default `##any` absorbs an otherwise well-formed wildcard.
- The derived complete wildcard's `processContents` is retained exactly. A
  missing mode still means strict; skip is never substituted for lax or strict.
- Both simple-content and complex-content extensions share this implementation.
  Restrictions do not acquire the base wildcard, including after an earlier
  extension. Later extension inherits the effective restricted result.
- The defining target namespace is still unavailable. Explicit finite members
  can establish admission alongside `##targetNamespace`, but that unresolved
  member cannot establish exclusion. Combinations involving `##other` other
  than provable `##any` absorption are unavailable; equal token spellings do
  not prove equal defining namespaces. Inexpressible or invalid compositions
  never become a normative instance failure or an all-namespace success.
- An unavailable composition is run-local effective-type state. Only reached
  undeclared attributes depend on it. Declared attribute facts and independent
  failures remain assessable, and extension propagates unavailable state.
  Restriction may replace that wildcard with its own local constraint.

Accepted pre-existing limitation: the generator's string-valued namespace field
cannot distinguish an omitted namespace attribute from an explicit
`namespace=""`. Both become the legacy default `##any`, although XSD 1.0 defines
the explicit empty list as the empty namespace set. A nonempty whitespace spelling
(`namespace=" "`) retains that empty set in direct admission and unions. The
characterization tests `TestAnyAttributeEmptyNamespacePresenceLimitation` and
`TestAttributeWildcardEmptyNamespaceMetadataLimitation` pin this distinction;
they do not claim generic XSD closure. MusicXML 4.0 has no `anyAttribute`, so this
limitation affects no built-in profile. Preserving explicit-empty presence in the
generator remains outside this bounded union correction.

No schema DTO field, public alias, generated metadata layout, compiled snapshot
format or digest encoder changes. The new state belongs only to
`EffectiveComplex`. The existing record allowance also covers its boolean in
the existing trailing boolean padding. Schema and catalog identities are not
changed for an unused feature field. The newly charged union work and retained
metadata can nevertheless change outcomes close to configured work or retention
limits; unchanged digest identity is not a promise of unchanged resource cost.

The internal legacy metadata-only `MergeAnyAttribute` has no unavailable-result
channel. It now returns exact representable unions, but retains its historical
derived-wildcard fallback for unavailable compositions. A repository-wide
caller audit finds only its definition, tests and the unused private
`mergeValidationAnyAttribute` alias. Neither authoritative `EffectiveComplex`
extension path calls that wrapper: both carry the separate unavailable state to
`ValidateComplex` and its private attribute validator. Public current-model,
source and compiled validation therefore do not take that fallback.

This is not complete target-namespace provenance, wildcard declaration lookup,
attribute-group wildcard intersection, schema-component validity, or general
value-space equality support. Lax/strict admission remains explicitly
incomplete where lookup is needed.

## Tests and independent oracle

`internal/xsd/attribute_wildcard_union_test.go` covers finite/default/any/absent
constraints, both content forms, restriction followed by extension, all 16
base/derived processing-mode pairs (including default strict), inherited modes,
input ownership, exact namespace identities, current-model binding, unrelated
declared-attribute failure, and unchanged declared-content witness selection.
The unexpressible/unknown controls require no fabricated instance issues.

`testdata/validation/attribute-wildcard-union-contract.xsd` is generated through
the ordinary XSD generator into the new test-only metadata file. Its 54
same-byte cases exercise compatibility source assessment, compiled Run assessment
and real `xmllint --nonet --noout --schema`. Another seven oracle cases
independently distinguish lax/strict unknown/declared valid/declared invalid
attributes, including strict processing of a namespace inherited from the base.
The controlled kernel deliberately reports unavailable for declaration-dependent
cases rather than claiming either oracle outcome.

A separate real-xmllint schema-error control constructs `not(urn:owner)` union
`{absent}`. It requires exit code 5 and the inexpressibility diagnostic, distinct
from exit code 3 for a schema-invalid instance. This is direct evidence for the
pinned 3.10.6 rule 5.3 boundary, not an instance rejection expectation.

On 2026-10-14, the official local Go 1.27.2 toolchain and real libxml2/xmllint
2.9.14 passed the focused generated/source/compiled/oracle matrix with
`MUSICXML_REQUIRE_XMLLINT=1`; none of those oracle tests skipped. Full internal
xsd and xsdgen tests and the focused race selection also passed. These statements
are scoped to the local commands below, not final integrated-head CI acceptance.

## Resources

Token scans, URI checking, exact-token hashing, deduplication, copying and output
construction are metered. Scratch token slices and dedup maps are admitted before
allocation and released at operation exit; retained union records and strings
remain owned by the run through Close. Builder capacity is conservatively
admitted before `Grow`. The immutable compiled schema is never mutated.

`attribute_wildcard_union_meter_test.go` replays deterministic exact work and
peak-retention budgets, one-short limits with original error identity, every
cancellation checkpoint, long URI and duplicate-token scans, operation-scope
cleanup and zero retention after Close. The initial finite fixture measured
1,125 work units / 13,472 logical peak bytes and 504 cancellation checkpoints;
the unavailable fixture measured 176 / 5,389 and 124. These are fixture-specific
logical resource observations, not heap or performance claims.

Reproduction commands (with the repository's official toolchain selected):

```sh
go test ./internal/xsd -run TestAttributeWildcard -count=1 -v
go test ./internal/xsd ./internal/xsdgen -count=1
go test . -run 'Test(GeneratedAttributeWildcard|AttributeWildcard)' -count=1 -v
go test -race ./internal/xsd ./internal/xsdgen -run 'Test(AttributeWildcard|TruthfulnessWildcard|TruthfulnessAttributeWildcard|RunWildcard|GenerateValidationSchema)' -count=1
go generate validation_attribute_wildcard_union_test.go
```

Existing obligation inventories, source records and historical acceptance claims
are preserved. The existing required real-oracle CI selection now names both
new oracle tests. A test-first guard verifies their selection, LF/CRLF behavior,
omission detection, and fail-versus-skip behavior with xmllint absent. Job count,
timeouts and race settings are unchanged. All-package tests passed on the local
implementation head before this narrow CI selector amendment.

The combined functional batch still owns final all-stage,
architecture and exact-head acceptance.
