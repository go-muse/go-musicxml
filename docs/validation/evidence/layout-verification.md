# Layout context verification

The implementation map covers all 75 assigned layout records and seven shared
text/language records. Original inventory planning statuses are preserved.
Every named test function was resolved against the real test sources. Pure
fact-protocol helpers are listed separately from source/current-model tests.

## Accepted execution

Environment: native workspace, Go 1.27.2, required real xmllint. All family
implementations used for integration were real code; no linking stubs or
scaffolds were present. Other workers' integration files were not included in
layout-owned commits.

- `go test ./...`: PASS after final Part identity and state/advisory integration.
- `go test . ./validation -run '^TestLayout' -count=1`: PASS.
- `go test -race . ./validation -run '^TestLayout' -count=1`: PASS before the final
  additive Part field.
- Final Part ownership, cross-part, visibility, and percussion regressions also
  passed under the race detector.
- Both complete root and validation test packages compile with `GOARCH=386`.
  Executing either i386 binary is unavailable in this sandbox (`exec format
  error`). This is a compilation result, not a 386 runtime pass.

Source and current-model adapters run independently across partwise/timewise
fixtures. Model checks assess the decoded/current structure directly without an
Encode/reparse path. Opus runs independent source/current-model fixtures; score
hierarchy variants do not apply to the separate Opus assembly.

## Regression evidence

Authentic failing-before/fixed-after fixtures covered layout lifetimes, geometry
precedence, margins, divider geometry, cancellation admission, blank-page parity,
ignored dynamics placement, font domains, parallel musical print positions,
zero-staff publishers, spacing versus visibility uncertainty, percussion advice,
and the missing per-part identity of timewise layout rows. The last regression
was first observed with the old payload and then checked with the typed Part
field. Unknown part causes and ownership are retained; unrelated rows explicitly
carry absent Part evidence.

Application font choices, exact unencoded geometry, instrument intent, and
rendered/extracted-part outcomes remain explicit per-record dependencies. The
implementation does not convert advisory guidance or uncertain rendering into
new normative failures.
